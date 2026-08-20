package vfs

import (
	"context"
	"errors"
	"io"
	"io/fs"
)

// Sentinel errors
var (
	ErrReadOnly = errors.New("filesystem is read-only")
	ErrNotFound = errors.New("path not found")
	ErrNotDir   = errors.New("path is not a directory")
	ErrNotFile  = errors.New("path is not a file")
	ErrExists   = errors.New("path already exists")
)

// FileInfo extends fs.FileInfo with VFS-specific metadata.
type FileInfo interface {
	fs.FileInfo
	Path() string     // Full VFS path
	RealPath() string // Underlying real path (if applicable)
	Volume() Volume   // Source volume (nil for nested archive entries)
	IsArchive() bool  // True if the entry can be mounted as a sub-filesystem
	IsHidden() bool
	IsSymlink() bool
	IsNested() bool       // Lives inside an archive chain
	MountChain() []string // Archive chain leading here
}

// Volume represents a mounted filesystem.
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
	VolumeLocal    VolumeType = "local"
	VolumeArchive  VolumeType = "archive"
	VolumeRemote   VolumeType = "remote"
	VolumeRegistry VolumeType = "registry"
)

// FileSystem is the core VFS interface every adapter must implement.
// Write operations return ErrReadOnly on read-only filesystems rather than panicking.
type FileSystem interface {
	// Read operations
	Open(ctx context.Context, path string) (File, error)
	Stat(ctx context.Context, path string) (FileInfo, error)
	ReadDir(ctx context.Context, path string) ([]FileInfo, error)
	Exists(ctx context.Context, path string) bool

	// Write operations — may return ErrReadOnly
	Create(ctx context.Context, path string) (File, error)
	Mkdir(ctx context.Context, path string, perm fs.FileMode) error
	Remove(ctx context.Context, path string) error
	Rename(ctx context.Context, oldPath, newPath string) error

	// Path helpers
	Join(elem ...string) string
	Split(path string) (dir, file string)
	IsAbs(path string) bool

	// RealPath translates a VFS path to the underlying OS path, if applicable.
	// Returns ("", nil) for purely virtual filesystems (in-memory archives, etc.).
	RealPath(vfsPath string) (string, error)
}

// ProviderCapabilities describes what a mounted adapter actually implements.
type ProviderCapabilities struct {
	Pagination   bool
	RandomAccess bool
	TotalCount   bool
	ServerSort   bool
	ServerFilter bool
	RangeRead    bool
	Watch        bool
	Rename       bool
	AtomicMove   bool
	Trash        bool
}

type CapabilityFileSystem interface {
	FileSystem
	ProviderCapabilities() ProviderCapabilities
}

type DirectoryPage struct {
	Entries    []FileInfo
	NextToken  string
	TotalCount int
	HasTotal   bool
}

type PagedFileSystem interface {
	FileSystem
	ReadDirPage(ctx context.Context, path, token string, limit int) (DirectoryPage, error)
}

// File represents an open file handle inside the VFS.
type File interface {
	io.Reader
	io.Writer
	io.Seeker
	io.Closer
	Stat() (FileInfo, error)
	Sync() error
}
