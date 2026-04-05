// Package mount manages virtual filesystem mount points.
// It routes VFS operations to the correct underlying FileSystem based on path prefix.
package mount

import (
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	"wails-app/vfs"
)

// Manager routes VFS operations to the correct FileSystem based on mount point.
// Mounts are resolved deepest-first so that /a/b/c takes priority over /a/b.
//
// Manager satisfies the vfs.Mounter interface.
type Manager struct {
	mu     sync.RWMutex
	mounts []entry // ordered deepest-first
	root   vfs.FileSystem
	nextID int
}

type entry struct {
	mountPoint string
	fs         vfs.FileSystem
	volume     vfs.Volume
	readOnly   bool
}

// NewManager creates a Manager with root as the "/" filesystem.
func NewManager(root vfs.FileSystem) *Manager {
	return &Manager{
		root:   root,
		mounts: make([]entry, 0),
	}
}

// Mount registers fs at mountPoint.
// Implements vfs.Mounter.
func (m *Manager) Mount(mountPoint string, fs vfs.FileSystem, readOnly bool, name string, vtype vfs.VolumeType) (vfs.Volume, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mountPoint = clean(mountPoint)

	for _, e := range m.mounts {
		if e.mountPoint == mountPoint {
			return nil, fmt.Errorf("mount: %q is already mounted", mountPoint)
		}
	}

	vol := &volume{
		id:       fmt.Sprintf("vol-%d", m.nextID),
		name:     name,
		vtype:    vtype,
		root:     mountPoint,
		fs:       fs,
		mgr:      m,
		readOnly: readOnly,
	}
	m.nextID++

	e := entry{
		mountPoint: mountPoint,
		fs:         fs,
		volume:     vol,
		readOnly:   readOnly,
	}

	// Keep sorted by descending length (deepest first).
	inserted := false
	for i, existing := range m.mounts {
		if len(mountPoint) > len(existing.mountPoint) {
			m.mounts = append(m.mounts[:i], append([]entry{e}, m.mounts[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		m.mounts = append(m.mounts, e)
	}

	return vol, nil
}

// Unmount removes the mount at mountPoint and closes the FS if it is an io.Closer.
// Implements vfs.Mounter.
func (m *Manager) Unmount(mountPoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mountPoint = clean(mountPoint)

	for i, e := range m.mounts {
		if e.mountPoint == mountPoint {
			if closer, ok := e.fs.(io.Closer); ok {
				_ = closer.Close()
			}
			m.mounts = append(m.mounts[:i], m.mounts[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("mount: %q not found", mountPoint)
}

// Resolve finds the deepest mount covering path and returns its FileSystem,
// the path translated into that FS's namespace, and the Volume (nil = root FS).
// Implements vfs.Mounter.
func (m *Manager) Resolve(path string) (vfs.FileSystem, string, vfs.Volume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path = clean(path)

	for _, e := range m.mounts {
		mp := e.mountPoint
		if path == mp || strings.HasPrefix(path, mp+"/") {
			rel := strings.TrimPrefix(path, mp)
			if rel == "" {
				rel = "/"
			}
			return e.fs, rel, e.volume, nil
		}
	}

	return m.root, path, nil, nil
}

// ListMounts returns a snapshot of all current mounts.
// Implements vfs.Mounter.
func (m *Manager) ListMounts() []vfs.MountedEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]vfs.MountedEntry, len(m.mounts))
	for i, e := range m.mounts {
		result[i] = vfs.MountedEntry{
			MountPoint: e.mountPoint,
			FS:         e.fs,
			Volume:     e.volume,
			ReadOnly:   e.readOnly,
		}
	}
	return result
}

// clean normalises a mount point to an absolute clean slash-separated path.
func clean(p string) string {
	if p == "" {
		return "/"
	}
	return path.Clean("/" + strings.TrimPrefix(p, "/"))
}

// volume implements vfs.Volume.
type volume struct {
	id       string
	name     string
	vtype    vfs.VolumeType
	root     string
	fs       vfs.FileSystem
	mgr      *Manager
	readOnly bool
}

func (v *volume) ID() string           { return v.id }
func (v *volume) Name() string         { return v.name }
func (v *volume) Type() vfs.VolumeType { return v.vtype }
func (v *volume) Root() string         { return v.root }
func (v *volume) IsReadOnly() bool     { return v.readOnly }
func (v *volume) Unmount() error       { return v.mgr.Unmount(v.root) }

var _ vfs.Volume = (*volume)(nil)
var _ vfs.Mounter = (*Manager)(nil)
