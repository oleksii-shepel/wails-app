package main

import (
	"context"
	"sync"
)

// DirStreamService owns directory stream lifecycle and event emission.
type DirStreamService struct {
	mu                    sync.Mutex
	streams               map[string]context.CancelFunc
	emitEvent             func(string, interface{})
	runElevatedWithEvents func(op, target string, req elevatedRequest) (elevatedResponse, error)
}

func NewDirStreamService(
	emitEvent func(string, interface{}),
	runElevatedWithEvents func(op, target string, req elevatedRequest) (elevatedResponse, error),
) *DirStreamService {
	return &DirStreamService{
		streams:               make(map[string]context.CancelFunc),
		emitEvent:             emitEvent,
		runElevatedWithEvents: runElevatedWithEvents,
	}
}

func (s *DirStreamService) Emit(name string, data interface{}) {
	if s.emitEvent != nil {
		s.emitEvent(name, data)
	}
}

func (s *DirStreamService) Register(streamID string, cancel context.CancelFunc) {
	s.mu.Lock()
	s.streams[streamID] = cancel
	s.mu.Unlock()
}

func (s *DirStreamService) Unregister(streamID string) {
	s.mu.Lock()
	delete(s.streams, streamID)
	s.mu.Unlock()
}

func (s *DirStreamService) Cancel(streamID string) {
	if streamID == "" {
		return
	}
	s.mu.Lock()
	cancel := s.streams[streamID]
	delete(s.streams, streamID)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *DirStreamService) CancelAll() {
	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.streams))
	for id, cancel := range s.streams {
		delete(s.streams, id)
		if cancel != nil {
			cancels = append(cancels, cancel)
		}
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}
