package browserpane

import (
	"fmt"
	"strings"
	"sync"
)

type Update struct {
	SurfaceID       string
	URL             string
	Theme           string
	BackgroundColor string
	Visible         bool
	X               int
	Y               int
	Width           int
	Height          int
}

type pane interface {
	Sync(Update) error
	CaptureScreenshot() (string, error)
	Dispose() error
}

type Service struct {
	mu    sync.Mutex
	panes map[string]pane
}

func NewService() *Service {
	return &Service{
		panes: make(map[string]pane),
	}
}

func (s *Service) Sync(update Update) error {
	update.SurfaceID = strings.TrimSpace(update.SurfaceID)
	update.URL = strings.TrimSpace(update.URL)
	if update.SurfaceID == "" {
		return fmt.Errorf("browser pane surface id is required")
	}

	if !update.Visible {
		s.mu.Lock()
		target := s.panes[update.SurfaceID]
		s.mu.Unlock()
		if target == nil {
			return nil
		}
		return target.Sync(update)
	}

	if update.URL == "" {
		return fmt.Errorf("browser pane url is required when visible")
	}
	if update.Width <= 0 || update.Height <= 0 {
		return fmt.Errorf("browser pane bounds must be positive")
	}

	s.mu.Lock()
	target := s.panes[update.SurfaceID]
	if target == nil {
		var err error
		target, err = newPlatformPane(update)
		if err != nil {
			s.mu.Unlock()
			return err
		}
		s.panes[update.SurfaceID] = target
	}
	s.mu.Unlock()

	return target.Sync(update)
}

func (s *Service) Dispose(surfaceID string) error {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" {
		return nil
	}

	s.mu.Lock()
	target := s.panes[surfaceID]
	delete(s.panes, surfaceID)
	s.mu.Unlock()

	if target == nil {
		return nil
	}
	return target.Dispose()
}

// CaptureScreenshot returns a base64-encoded JPEG screenshot of the browser pane.
func (s *Service) CaptureScreenshot(surfaceID string) (string, error) {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" {
		return "", fmt.Errorf("browser pane surface id is required")
	}

	s.mu.Lock()
	target := s.panes[surfaceID]
	s.mu.Unlock()

	if target == nil {
		return "", fmt.Errorf("browser pane %q not found", surfaceID)
	}
	return target.CaptureScreenshot()
}

func (s *Service) Shutdown() error {
	s.mu.Lock()
	panes := make([]pane, 0, len(s.panes))
	for key, target := range s.panes {
		delete(s.panes, key)
		panes = append(panes, target)
	}
	s.mu.Unlock()

	var firstErr error
	for _, target := range panes {
		if err := target.Dispose(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
