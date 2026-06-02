package browserpane

import (
	"encoding/json"
	"fmt"
	"net/url"
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
	GetDocumentBackground() (string, error)
	PostMessage(string) error
	Dispose() error
}

type Service struct {
	mu        sync.Mutex
	panes     map[string]pane
	emitEvent func(string, interface{})
	lastURL   map[string]string
}

func NewService(emitEvent func(string, interface{})) *Service {
	return &Service{
		panes:     make(map[string]pane),
		emitEvent: emitEvent,
		lastURL:   make(map[string]string),
	}
}

func (s *Service) Sync(update Update) error {
	update.SurfaceID = strings.TrimSpace(update.SurfaceID)
	update.URL = strings.TrimSpace(update.URL)
	if update.SurfaceID == "" {
		return fmt.Errorf("browser pane surface id is required")
	}

	if !isSupportedBrowserPaneURL(update.URL) {
		update.URL = ""
		update.Visible = false
		update.Width = 0
		update.Height = 0
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
		target, err = newPlatformPane(update, s.emitResourceUpdate, func(message string) {
			s.handlePaneMessage(update.SurfaceID, message)
		})
		if err != nil {
			s.mu.Unlock()
			return err
		}
		s.panes[update.SurfaceID] = target
	}
	urlChanged := update.URL != "" && s.lastURL[update.SurfaceID] != update.URL
	if urlChanged {
		s.lastURL[update.SurfaceID] = update.URL
	}
	s.mu.Unlock()

	if urlChanged {
		s.emitLifecycleEvent(update.SurfaceID, "started", update.URL)
	}

	return target.Sync(update)
}

func isSupportedBrowserPaneURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}

	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return false
	}

	return strings.TrimSpace(parsed.Host) != ""
}

func (s *Service) emitResourceUpdate(path string) {
	if s.emitEvent == nil || strings.TrimSpace(path) == "" {
		return
	}
	s.emitEvent("web:resourcesChanged", map[string]string{"path": strings.TrimSpace(path)})
}

func (s *Service) handlePaneMessage(surfaceID string, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	var envelope struct {
		Type  string `json:"type"`
		State string `json:"state"`
		URL   string `json:"url"`
	}
	if err := json.Unmarshal([]byte(message), &envelope); err == nil && envelope.Type == "lifecycle" {
		s.emitLifecycleEvent(surfaceID, envelope.State, envelope.URL)
		return
	}

	s.emitPaneMessage(surfaceID, message)
}

func (s *Service) emitPaneMessage(surfaceID string, message string) {
	if s.emitEvent == nil || strings.TrimSpace(surfaceID) == "" {
		return
	}
	s.emitEvent("browser-pane:message", map[string]string{
		"surfaceId": strings.TrimSpace(surfaceID),
		"message":   message,
	})
}

func (s *Service) emitLifecycleEvent(surfaceID string, state string, url string) {
	if s.emitEvent == nil || strings.TrimSpace(surfaceID) == "" || strings.TrimSpace(state) == "" {
		return
	}
	s.emitEvent("browser-pane:lifecycle", map[string]string{
		"surfaceId": strings.TrimSpace(surfaceID),
		"state":     strings.TrimSpace(state),
		"url":       strings.TrimSpace(url),
	})
}

func (s *Service) Dispose(surfaceID string) error {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" {
		return nil
	}

	s.mu.Lock()
	target := s.panes[surfaceID]
	delete(s.panes, surfaceID)
	delete(s.lastURL, surfaceID)
	s.mu.Unlock()

	if target == nil {
		return nil
	}
	return target.Dispose()
}

// CaptureScreenshot returns a base64-encoded PNG screenshot of the browser pane.
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

func (s *Service) GetDocumentBackground(surfaceID string) (string, error) {
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
	return target.GetDocumentBackground()
}

func (s *Service) PostMessage(surfaceID string, message string) error {
	surfaceID = strings.TrimSpace(surfaceID)
	if surfaceID == "" {
		return fmt.Errorf("browser pane surface id is required")
	}

	s.mu.Lock()
	target := s.panes[surfaceID]
	s.mu.Unlock()

	if target == nil {
		return fmt.Errorf("browser pane %q not found", surfaceID)
	}
	return target.PostMessage(message)
}

func (s *Service) Shutdown() error {
	s.mu.Lock()
	panes := make([]pane, 0, len(s.panes))
	for key, target := range s.panes {
		delete(s.panes, key)
		delete(s.lastURL, key)
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
