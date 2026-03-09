package vfs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"vfs/archive"

	"vfs/mount"
)

// UnifiedVFS combines mount management with VFS operations
type UnifiedVFS struct {
	mounts *mount.Manager
}

func New(root FileSystem) *UnifiedVFS {
	return &UnifiedVFS{
		mounts: mount.NewManager(root),
	}
}

func (u *UnifiedVFS) Mount(mountPoint string, fs FileSystem, opts mount.MountOptions) (Volume, error) {
	return u.mounts.Mount(mountPoint, fs, opts)
}

func (u *UnifiedVFS) Unmount(mountPoint string) error {
	return u.mounts.Unmount(mountPoint)
}

// VFS operations with automatic routing
func (u *UnifiedVFS) Open(ctx context.Context, path string) (File, error) {
	fs, rel, _, err := u.mounts.Resolve(path)
	if err != nil {
		return nil, err
	}
	return fs.Open(ctx, rel)
}

func (u *UnifiedVFS) Stat(ctx context.Context, path string) (FileInfo, error) {
	fs, rel, _, err := u.mounts.Resolve(path)
	if err != nil {
		return nil, err
	}
	return fs.Stat(ctx, rel)
}

func (u *UnifiedVFS) ReadDir(ctx context.Context, path string) ([]FileInfo, error) {
	fs, rel, vol, err := u.mounts.Resolve(path)
	if err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(ctx, rel)
	if err != nil {
		return nil, err
	}

	// Inject mount points as virtual directories
	if rel == "/" || rel == "" {
		for _, m := range u.mounts.ListMounts() {
			if filepath.Dir(m.MountPoint) == path ||
				(path == "/" && strings.Count(m.MountPoint, "/") == 1) {
				// Create virtual entry for mount point
				entries = append(entries, &mountPointInfo{
					name: filepath.Base(m.MountPoint),
					path: m.MountPoint,
					vol:  m.Volume,
				})
			}
		}
	}

	return entries, nil
}

// ... implement all other FileSystem methods similarly

// Special handling for archives - auto-mount on access
func (u *UnifiedVFS) OpenArchive(ctx context.Context, path string, nested []string) (FileSystem, error) {
	// Resolve to real path first
	realPath, err := u.RealPath(path)
	if err != nil {
		return nil, err
	}

	// Get archive handler
	handler, ok := archive.DefaultRegistry.GetByPath(realPath)
	if !ok {
		return nil, fmt.Errorf("no handler for archive: %s", path)
	}

	// Open archive
	arch, err := handler.Open(ctx, realPath, archive.OpenOptions{
		NestedChain: nested,
	})
	if err != nil {
		return nil, err
	}

	// Auto-mount at virtual path
	mountPoint := filepath.Join("/archives", path)
	if len(nested) > 0 {
		mountPoint = filepath.Join(mountPoint, strings.Join(nested, "/"))
	}

	vol, err := u.Mount(mountPoint, arch, mount.MountOptions{
		Name:     filepath.Base(path),
		Type:     VolumeArchive,
		ReadOnly: true,
	})
	if err != nil {
		arch.Close()
		return nil, err
	}

	return &mountedArchive{
		Archive:    arch,
		volume:     vol,
		mountPoint: mountPoint,
		vfs:        u,
	}, nil
}
