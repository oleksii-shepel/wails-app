package mount

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"vfs"
)

// Entry represents a mount point
type Entry struct {
	MountPoint string
	FS         vfs.FileSystem
	Volume     vfs.Volume
	ReadOnly   bool
}

// Manager handles filesystem mounting
type Manager struct {
	mu     sync.RWMutex
	mounts []Entry // Ordered by mount point depth (deepest first)
	root   vfs.FileSystem
	nextID int
}

func NewManager(root vfs.FileSystem) *Manager {
	return &Manager{
		root:   root,
		mounts: make([]Entry, 0),
	}
}

func (m *Manager) Mount(mountPoint string, fs vfs.FileSystem, opts MountOptions) (vfs.Volume, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Normalize mount point
	mountPoint = filepath.Clean("/" + mountPoint)

	// Check for conflicts
	for _, e := range m.mounts {
		if e.MountPoint == mountPoint {
			return nil, fmt.Errorf("mount point %q already in use", mountPoint)
		}
		if strings.HasPrefix(mountPoint, e.MountPoint+"/") ||
			strings.HasPrefix(e.MountPoint, mountPoint+"/") {
			return nil, fmt.Errorf("mount point %q conflicts with %q", mountPoint, e.MountPoint)
		}
	}

	// Create volume
	vol := &volume{
		id:    fmt.Sprintf("vol-%d", m.nextID),
		name:  opts.Name,
		vtype: opts.Type,
		root:  mountPoint,
		fs:    fs,
		mgr:   m,
	}
	m.nextID++

	entry := Entry{
		MountPoint: mountPoint,
		FS:         fs,
		Volume:     vol,
		ReadOnly:   opts.ReadOnly,
	}

	// Insert in order (deepest first for correct resolution)
	inserted := false
	for i, e := range m.mounts {
		if len(mountPoint) > len(e.MountPoint) {
			// Insert before (deeper mount points first)
			m.mounts = append(m.mounts[:i], append([]Entry{entry}, m.mounts[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		m.mounts = append(m.mounts, entry)
	}

	return vol, nil
}

func (m *Manager) Unmount(mountPoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mountPoint = filepath.Clean("/" + mountPoint)

	for i, e := range m.mounts {
		if e.MountPoint == mountPoint {
			// Cleanup
			if closer, ok := e.FS.(io.Closer); ok {
				closer.Close()
			}
			m.mounts = append(m.mounts[:i], m.mounts[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("mount point %q not found", mountPoint)
}

func (m *Manager) Resolve(path string) (vfs.FileSystem, string, vfs.Volume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path = filepath.Clean("/" + path)

	// Find deepest matching mount point
	for _, e := range m.mounts {
		if path == e.MountPoint || strings.HasPrefix(path, e.MountPoint+"/") {
			// Translate path
			rel := strings.TrimPrefix(path, e.MountPoint)
			if rel == "" {
				rel = "/"
			}
			return e.FS, rel, e.Volume, nil
		}
	}

	// Fall back to root filesystem
	return m.root, path, nil, nil
}

func (m *Manager) ListMounts() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Entry, len(m.mounts))
	copy(result, m.mounts)
	return result
}

type MountOptions struct {
	Name     string
	Type     vfs.VolumeType
	ReadOnly bool
}
