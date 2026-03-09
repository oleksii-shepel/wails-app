package vfs

import (
	"context"
	"io"
	"io/fs"
)

// FileInfo extends fs.FileInfo with additional metadata
type FileInfo interface {
	fs.FileInfo
	Path() string     // Full VFS path
	RealPath() string // Underlying real path (if any)
	Volume() Volume   // Source volume
}

// Volume represents a mounted filesystem
type Volume interface {
	ID() string
	Name() string
	Type() VolumeType
	Root() string
	IsReadOnly() bool
	Unmount() error
}

type VolumeType string

const (
	VolumeLocal   VolumeType = "local"
	VolumeArchive VolumeType = "archive"
	VolumeRemote  VolumeType = "remote"
	VolumeMemory  VolumeType = "memory"
)

// FileSystem is the core VFS interface
type FileSystem interface {
	// Core operations
	Open(ctx context.Context, path string) (File, error)
	Stat(ctx context.Context, path string) (FileInfo, error)
	ReadDir(ctx context.Context, path string) ([]FileInfo, error)

	// Write operations (may return ReadOnly for read-only filesystems)
	Create(ctx context.Context, path string) (File, error)
	Mkdir(ctx context.Context, path string, perm fs.FileMode) error
	Remove(ctx context.Context, path string) error
	Rename(ctx context.Context, oldPath, newPath string) error

	// Utility
	Exists(ctx context.Context, path string) bool
	Join(elem ...string) string
	Split(path string) (dir, file string)
	IsAbs(path string) bool
	RealPath(vfsPath string) (string, error) // Returns underlying path if applicable
}

// File represents an open file in the VFS
type File interface {
	io.Reader
	io.Writer
	io.Seeker
	io.Closer

	Stat() (FileInfo, error)
	Sync() error
}

// Mountable extends FileSystem with mount capability
type Mountable interface {
	FileSystem
	Mount(mountPoint string, fs FileSystem) error
	Unmount(mountPoint string) error
	Resolve(path string) (FileSystem, string, error) // Resolve path to specific FS
}
