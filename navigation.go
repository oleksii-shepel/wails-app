package main

import (
	"sync"
	"time"
)

const defaultMaxHistory = 200

// paneHistory holds the back/forward stacks for a single pane.
type paneHistory struct {
	back    []string // oldest → newest (current is back[len-1])
	forward []string // oldest → newest (next is forward[len-1])
}

// NavigationManager manages per-pane navigation history.
type NavigationManager struct {
	mu      sync.Mutex
	panes   map[string]*paneHistory
	maxSize int
}

func NewNavigationManager() *NavigationManager {
	return &NavigationManager{
		panes:   make(map[string]*paneHistory),
		maxSize: defaultMaxHistory,
	}
}

func (nm *NavigationManager) pane(id string) *paneHistory {
	if _, ok := nm.panes[id]; !ok {
		nm.panes[id] = &paneHistory{}
	}
	return nm.panes[id]
}

// Navigate pushes a new path, clearing the forward stack.
func (nm *NavigationManager) Navigate(paneID, path string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	p := nm.pane(paneID)

	// Don't push if same as current location
	if len(p.back) > 0 && p.back[len(p.back)-1] == path {
		return
	}

	p.back = append(p.back, path)
	// Trim to maxSize
	if len(p.back) > nm.maxSize {
		p.back = p.back[len(p.back)-nm.maxSize:]
	}
	p.forward = nil // clear forward stack on new navigation
}

// GoBack moves back one step, returns new current path or "" if at start.
func (nm *NavigationManager) GoBack(paneID string) string {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	p := nm.pane(paneID)
	if len(p.back) < 2 {
		return ""
	}

	// Pop current off back, push to forward
	current := p.back[len(p.back)-1]
	p.back = p.back[:len(p.back)-1]
	p.forward = append(p.forward, current)

	return p.back[len(p.back)-1]
}

// GoForward moves forward one step, returns new current path or "" if at end.
func (nm *NavigationManager) GoForward(paneID string) string {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	p := nm.pane(paneID)
	if len(p.forward) == 0 {
		return ""
	}

	// Pop from forward, push to back
	next := p.forward[len(p.forward)-1]
	p.forward = p.forward[:len(p.forward)-1]
	p.back = append(p.back, next)

	return next
}

// CanGoBack returns true if back navigation is possible for the pane.
func (nm *NavigationManager) CanGoBack(paneID string) bool {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	return len(nm.pane(paneID).back) >= 2
}

// CanGoForward returns true if forward navigation is possible for the pane.
func (nm *NavigationManager) CanGoForward(paneID string) bool {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	return len(nm.pane(paneID).forward) > 0
}

// CurrentPath returns the current path for a pane.
func (nm *NavigationManager) CurrentPath(paneID string) string {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	p := nm.pane(paneID)
	if len(p.back) == 0 {
		return ""
	}
	return p.back[len(p.back)-1]
}

// GetHistory returns the full history for a pane as a NavigationHistory struct.
func (nm *NavigationManager) GetHistory(paneID string) NavigationHistory {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	p := nm.pane(paneID)
	entries := make([]HistoryEntry, 0, len(p.back))
	now := time.Now().Unix()

	for i, path := range p.back {
		entries = append(entries, HistoryEntry{
			Path:      path,
			Timestamp: now - int64(len(p.back)-1-i), // approximate ordering
			PaneID:    paneID,
		})
	}

	return NavigationHistory{
		Entries:    entries,
		MaxEntries: nm.maxSize,
	}
}

// ClearHistory clears history for a specific pane.
func (nm *NavigationManager) ClearHistory(paneID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.panes[paneID] = &paneHistory{}
}
