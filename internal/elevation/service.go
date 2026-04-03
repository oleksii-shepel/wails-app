package elevation

import (
	"fmt"
	"os"
	"sync"
	"time"

	contracts "wails-app/internal/contracts"
)

type Request struct {
	Op          string `json:"op"`
	Path        string `json:"path,omitempty"`
	ShowHidden  bool   `json:"showHidden,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	Content     string `json:"content,omitempty"`
	Recursive   bool   `json:"recursive,omitempty"`
	UseTrash    *bool  `json:"useTrash,omitempty"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
}

type Response struct {
	Success    bool                        `json:"success"`
	Error      string                      `json:"error,omitempty"`
	DirListing *contracts.DirectoryListing `json:"dirListing,omitempty"`
	File       *contracts.FileContent      `json:"file,omitempty"`
	Operation  *contracts.OperationResult  `json:"operation,omitempty"`
}

type SessionStatus struct {
	Active    bool   `json:"active"`
	PID       int    `json:"pid,omitempty"`
	StartedAt int64  `json:"startedAt,omitempty"`
	Port      int    `json:"port,omitempty"`
	OpCount   int    `json:"opCount"`
	Error     string `json:"error,omitempty"`
}

type Client interface {
	Alive() bool
	Dispatch(Request) (Response, error)
	Status() SessionStatus
	Stop()
}

type Service struct {
	mu             sync.Mutex
	wait           map[string]chan bool
	client         Client
	emitEvent      func(string, interface{})
	resolveExePath func() (string, error)
	startClient    func(exePath string) (Client, error)
}

func NewService(emitEvent func(string, interface{}), startClient func(exePath string) (Client, error)) *Service {
	return &Service{
		wait:           make(map[string]chan bool),
		emitEvent:      emitEvent,
		resolveExePath: os.Executable,
		startClient:    startClient,
	}
}

func (s *Service) StartSession() (*SessionStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil && s.client.Alive() {
		status := s.client.Status()
		s.emit("elevation:session-started", status)
		return &status, nil
	}

	exePath, err := s.resolveExePath()
	if err != nil {
		return nil, fmt.Errorf("resolve executable: %w", err)
	}

	s.emit("elevation:uac-requested", map[string]string{
		"phase":     "requested",
		"operation": "session-start",
		"target":    "elevated-service",
		"message":   "Administrative privileges requested to start elevated service session.",
	})

	client, err := s.startClient(exePath)
	if err != nil {
		return nil, err
	}
	s.client = client
	status := client.Status()
	s.emit("elevation:session-started", status)
	return &status, nil
}

func (s *Service) StopSession() {
	s.mu.Lock()
	client := s.client
	s.client = nil
	s.mu.Unlock()

	if client != nil {
		client.Stop()
	}
	s.emit("elevation:session-stopped", map[string]bool{"active": false})
}

func (s *Service) Status() SessionStatus {
	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client == nil {
		return SessionStatus{}
	}
	return client.Status()
}

func (s *Service) Client() Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

func (s *Service) Confirm(requestID string, approve bool) {
	s.mu.Lock()
	ch, ok := s.wait[requestID]
	if ok {
		delete(s.wait, requestID)
	}
	s.mu.Unlock()

	if ok {
		select {
		case ch <- approve:
		default:
		}
	}
}

func (s *Service) WaitForApproval(requestID string, timeout time.Duration) (bool, error) {
	ch := make(chan bool, 1)
	s.mu.Lock()
	s.wait[requestID] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.wait, requestID)
		s.mu.Unlock()
	}()

	select {
	case approve := <-ch:
		return approve, nil
	case <-time.After(timeout):
		return false, fmt.Errorf("elevation approval timed out")
	}
}

func (s *Service) emit(name string, data interface{}) {
	if s.emitEvent != nil {
		s.emitEvent(name, data)
	}
}
