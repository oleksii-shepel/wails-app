package navigation

import "sync"

const defaultMaxHistory = 200

type paneHistory struct {
	back    []string
	forward []string
}

// Manager stores per-pane navigation history independently of the Wails binding layer.
type Manager struct {
	mu      sync.Mutex
	panes   map[string]*paneHistory
	maxSize int
}

func NewManager() *Manager {
	return &Manager{
		panes:   make(map[string]*paneHistory),
		maxSize: defaultMaxHistory,
	}
}

func (m *Manager) pane(id string) *paneHistory {
	if _, ok := m.panes[id]; !ok {
		m.panes[id] = &paneHistory{}
	}
	return m.panes[id]
}

// Navigate pushes a new path and clears the forward stack.
func (m *Manager) Navigate(paneID, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pane := m.pane(paneID)
	if len(pane.back) > 0 && pane.back[len(pane.back)-1] == path {
		return
	}

	pane.back = append(pane.back, path)
	if len(pane.back) > m.maxSize {
		pane.back = pane.back[len(pane.back)-m.maxSize:]
	}
	pane.forward = nil
}

// GoBack moves back one step and returns the new current path.
func (m *Manager) GoBack(paneID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	pane := m.pane(paneID)
	if len(pane.back) < 2 {
		return ""
	}

	current := pane.back[len(pane.back)-1]
	pane.back = pane.back[:len(pane.back)-1]
	pane.forward = append(pane.forward, current)

	return pane.back[len(pane.back)-1]
}

// GoForward moves forward one step and returns the new current path.
func (m *Manager) GoForward(paneID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	pane := m.pane(paneID)
	if len(pane.forward) == 0 {
		return ""
	}

	next := pane.forward[len(pane.forward)-1]
	pane.forward = pane.forward[:len(pane.forward)-1]
	pane.back = append(pane.back, next)

	return next
}

func (m *Manager) CanGoBack(paneID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pane(paneID).back) >= 2
}

func (m *Manager) CanGoForward(paneID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pane(paneID).forward) > 0
}

func (m *Manager) CurrentPath(paneID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	pane := m.pane(paneID)
	if len(pane.back) == 0 {
		return ""
	}
	return pane.back[len(pane.back)-1]
}

func (m *Manager) ClearHistory(paneID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.panes[paneID] = &paneHistory{}
}
