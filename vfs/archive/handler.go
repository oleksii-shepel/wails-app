package archive

import (
	"context"
	"vfs"
)

// Handler is the interface for archive format implementations
type Handler interface {
	// Identity
	Name() string         // "zip", "tar.gz", "7z"
	Extensions() []string // [".zip"], [".tar", ".tar.gz"]
	Magic() []byte        // File signature for detection

	// Capabilities
	CanRead() bool
	CanWrite() bool  // Can modify archives in-place
	CanCreate() bool // Can create new archives
	CanStream() bool // Supports streaming without full extraction

	// Operations on archive files
	Open(ctx context.Context, path string, options OpenOptions) (Archive, error)
	Detect(data []byte) bool // Detect format from header bytes
}

type OpenOptions struct {
	ReadOnly    bool
	Password    string   // For encrypted archives
	NestedChain []string // For nested archives: ["outer.zip", "inner.tar.gz"]
}

// Archive represents an opened archive
type Archive interface {
	vfs.FileSystem // Archives act as read-only filesystems

	// Archive-specific operations
	Extract(ctx context.Context, srcPath, dstPath string) error
	ExtractSelected(ctx context.Context, entries []string, dstPath string, opts ExtractOptions) error
	Add(ctx context.Context, srcPath, dstPath string) error // Add file to archive
	Remove(ctx context.Context, path string) error          // Remove from archive
	Close() error

	// Metadata
	Comment() string
	IsSolid() bool // Solid compression (7z/rar)
	EntryCount() int
}

type ExtractOptions struct {
	StripPrefix  string
	FlattenPaths bool
	PreserveTime bool
	Progress     ProgressCallback
}

type ProgressCallback func(current, total int64, currentFile string)
