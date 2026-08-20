package runtime

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"wails-app/internal/pathref"
	internalplatform "wails-app/internal/platform"
)

type paneDirectoryChangedEvent struct {
	PaneID string `json:"paneId"`
	Path   string `json:"path"`
}

type PaneWatchManager struct {
	emitEvent    func(string, interface{})
	resolvePath  func(string) (string, error)
	toPublicPath func(string) string
	mu           sync.Mutex
	watcher      *fsnotify.Watcher
	panePaths    map[string]string
	panePublic   map[string]string
	pathRef      map[string]int
	paneDebounce map[string]*time.Timer
	closed       bool
}

func NewPaneWatchManager(emitEvent func(string, interface{}), resolvePath func(string) (string, error), toPublicPath func(string) string) *PaneWatchManager {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return &PaneWatchManager{
			emitEvent:    emitEvent,
			resolvePath:  resolvePath,
			toPublicPath: toPublicPath,
			panePaths:    make(map[string]string),
			panePublic:   make(map[string]string),
			pathRef:      make(map[string]int),
			paneDebounce: make(map[string]*time.Timer),
		}
	}

	manager := &PaneWatchManager{
		emitEvent:    emitEvent,
		resolvePath:  resolvePath,
		toPublicPath: toPublicPath,
		watcher:      w,
		panePaths:    make(map[string]string),
		panePublic:   make(map[string]string),
		pathRef:      make(map[string]int),
		paneDebounce: make(map[string]*time.Timer),
	}
	go manager.loop()
	return manager
}

func (m *PaneWatchManager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	for paneID, timer := range m.paneDebounce {
		timer.Stop()
		delete(m.paneDebounce, paneID)
	}
	w := m.watcher
	m.watcher = nil
	m.mu.Unlock()

	if w != nil {
		_ = w.Close()
	}
}

func (m *PaneWatchManager) SetPaneDirectory(paneID, path string) error {
	watchPath, err := m.normalizeWatchPath(path)
	if err != nil {
		return err
	}
	publicPath := m.toEventPath(watchPath)

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	prev := m.panePaths[paneID]
	if prev == watchPath && m.panePublic[paneID] == publicPath {
		return nil
	}
	if prev != "" {
		m.removeWatchLocked(prev)
	}
	if watchPath == "" {
		delete(m.panePaths, paneID)
		delete(m.panePublic, paneID)
		return nil
	}

	m.panePaths[paneID] = watchPath
	m.panePublic[paneID] = publicPath
	return m.addWatchLocked(watchPath)
}

func (m *PaneWatchManager) loop() {
	for {
		m.mu.Lock()
		w := m.watcher
		closed := m.closed || w == nil
		m.mu.Unlock()
		if closed {
			return
		}

		select {
		case evt, ok := <-w.Events:
			if !ok {
				return
			}
			if evt.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) == 0 {
				continue
			}
			affectedDir := normalizeNativeWatchPath(filepath.Dir(evt.Name))
			if affectedDir == "" {
				continue
			}
			m.scheduleForAffectedDir(affectedDir)
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}

func (m *PaneWatchManager) scheduleForAffectedDir(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}

	for paneID, panePath := range m.panePaths {
		if panePath != dir {
			continue
		}
		if existing := m.paneDebounce[paneID]; existing != nil {
			existing.Stop()
		}
		id := paneID
		path := m.panePublic[paneID]
		m.paneDebounce[paneID] = time.AfterFunc(200*time.Millisecond, func() {
			if m.emitEvent != nil {
				m.emitEvent("fs:directoryChanged", paneDirectoryChangedEvent{
					PaneID: id,
					Path:   path,
				})
			}
		})
	}
}

func (m *PaneWatchManager) addWatchLocked(path string) error {
	if m.pathRef[path] > 0 {
		m.pathRef[path]++
		return nil
	}
	if m.watcher != nil {
		if err := m.watcher.Add(path); err != nil {
			return err
		}
	}
	m.pathRef[path] = 1
	return nil
}

func (m *PaneWatchManager) removeWatchLocked(path string) {
	count := m.pathRef[path]
	if count <= 1 {
		delete(m.pathRef, path)
		if m.watcher != nil {
			_ = m.watcher.Remove(path)
		}
		return
	}
	m.pathRef[path] = count - 1
}

func (m *PaneWatchManager) normalizeWatchPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", nil
	}
	if pathref.IsRegistry(trimmed) || pathref.IsAlias(trimmed) {
		return "", nil
	}
	if m.resolvePath != nil {
		resolved, err := m.resolvePath(trimmed)
		if err != nil {
			return "", err
		}
		trimmed = resolved
	}
	return normalizeNativeWatchPath(trimmed), nil
}

func (m *PaneWatchManager) toEventPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if m.toPublicPath != nil {
		if publicPath := strings.TrimSpace(m.toPublicPath(trimmed)); publicPath != "" {
			return pathref.Normalize(publicPath)
		}
	}
	return normalizeNativeWatchPath(trimmed)
}

func normalizeNativeWatchPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if pathref.IsRegistry(trimmed) || pathref.IsAlias(trimmed) {
		return ""
	}
	return internalplatform.ToSlash(filepath.Clean(trimmed))
}
