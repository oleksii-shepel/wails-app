package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// ElevationSessionService owns the lifecycle of the persistent elevated IPC
// session and the in-app approval waiters associated with it.
type ElevationSessionService struct {
	mu             sync.Mutex
	wait           map[string]chan bool
	svc            *ElevatedServiceClient
	emitEvent      func(string, interface{})
	resolveExePath func() (string, error)
}

func NewElevationSessionService(emitEvent func(string, interface{})) *ElevationSessionService {
	return &ElevationSessionService{
		wait:           make(map[string]chan bool),
		emitEvent:      emitEvent,
		resolveExePath: os.Executable,
	}
}

func (s *ElevationSessionService) StartSession() (*ElevatedSessionStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.svc != nil && s.svc.Alive() {
		status := s.svc.Status()
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

	svc, err := StartElevatedServiceClient(exePath)
	if err != nil {
		return nil, err
	}
	s.svc = svc
	status := svc.Status()
	s.emit("elevation:session-started", status)
	return &status, nil
}

func (s *ElevationSessionService) StopSession() {
	s.mu.Lock()
	svc := s.svc
	s.svc = nil
	s.mu.Unlock()

	if svc != nil {
		svc.Stop()
	}
	s.emit("elevation:session-stopped", map[string]bool{"active": false})
}

func (s *ElevationSessionService) Status() ElevatedSessionStatus {
	s.mu.Lock()
	svc := s.svc
	s.mu.Unlock()
	if svc == nil {
		return ElevatedSessionStatus{}
	}
	return svc.Status()
}

func (s *ElevationSessionService) Client() *ElevatedServiceClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.svc
}

func (s *ElevationSessionService) Confirm(requestID string, approve bool) {
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

func (s *ElevationSessionService) WaitForApproval(requestID string, timeout time.Duration) (bool, error) {
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

func (s *ElevationSessionService) emit(name string, data interface{}) {
	if s.emitEvent != nil {
		s.emitEvent(name, data)
	}
}
