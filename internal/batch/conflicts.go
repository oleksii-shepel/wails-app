package batch

import (
	"fmt"
	"sync"
	"sync/atomic"

	contracts "wails-app/internal/contracts"
)

// ConflictManager coordinates paused batch operations waiting on a user
// conflict resolution from the frontend.
type ConflictManager struct {
	mu      sync.Mutex
	pending map[string]chan contracts.ConflictResolutionRequest
	counter int64
}

func NewConflictManager() *ConflictManager {
	return &ConflictManager{pending: make(map[string]chan contracts.ConflictResolutionRequest)}
}

func (m *ConflictManager) NextID(opID string) string {
	return fmt.Sprintf("%s-conflict-%d", opID, atomic.AddInt64(&m.counter, 1))
}

func (m *ConflictManager) Register(id string) <-chan contracts.ConflictResolutionRequest {
	response := make(chan contracts.ConflictResolutionRequest, 1)
	m.mu.Lock()
	m.pending[id] = response
	m.mu.Unlock()
	return response
}

func (m *ConflictManager) Resolve(id string, req contracts.ConflictResolutionRequest) bool {
	m.mu.Lock()
	response, ok := m.pending[id]
	if ok {
		delete(m.pending, id)
	}
	m.mu.Unlock()
	if !ok {
		return false
	}
	response <- req
	return true
}

func (m *ConflictManager) CancelAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, response := range m.pending {
		response <- contracts.ConflictResolutionRequest{Resolution: contracts.ConflictCancel}
		delete(m.pending, id)
	}
}
