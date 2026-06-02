package plugin

import (
	vfslib "wails-app/vfs"
)

// VFSPlugin allows plugins to provide new filesystem backends.
// Built-in providers (osfs, archive, memory, registry, remote) are registered
// through the same interface.
type VFSPlugin interface {
	Plugin
	// Schemes returns the URI schemes this provider handles (e.g., ["smb", "cifs"]).
	Schemes() []string
	// NewFS creates a new filesystem instance from config.
	NewFS(config map[string]any) (vfslib.FileSystem, error)
}
