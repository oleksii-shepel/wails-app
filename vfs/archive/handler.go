package archive

import (
	"context"

	"wails-app/vfs"
)

// Handler is the interface every archive format plugin must implement.
// Register implementations with Register() from an init() function.
type Handler interface {
	// Identity & capability
	Name() string         // Short name: "zip", "tar.gz", "7z"
	Extensions() []string // Dot-prefixed: [".zip"], [".tar.gz", ".tgz"]
	Magic() []byte        // Header bytes for content-based detection (nil = extension only)
	CanRead() bool
	CanWrite() bool  // Supports in-place modification (rewrite)
	CanCreate() bool // Supports creating new archives from scratch
	CanStream() bool // Can read entries without seeking (good for tarballs)

	// Detect reports whether the first bytes of a file match this format.
	Detect(data []byte) bool

	// Open returns an Archive ready to use. The caller must call Archive.Close().
	Open(ctx context.Context, path string, opts OpenOptions) (Archive, error)
}

// OpenOptions controls how an archive is opened.
type OpenOptions struct {
	ReadOnly    bool
	Password    string   // For encrypted formats (zip AES, 7z, rar)
	NestedChain []string // Ordered list of entry paths to resolve before opening.
	// Example: outer.zip contains inner.tar.gz; NestedChain = ["inner.tar.gz"]
}

// Archive represents an opened archive and exposes it as a vfs.FileSystem.
// The caller is responsible for calling Close().
type Archive interface {
	vfs.FileSystem // Archives are navigable as read (and sometimes write) filesystems.

	// Archive-specific
	Comment() string // Global archive comment (zip supports this; others return "")
	IsSolid() bool   // True for solid-compression formats (7z, some rar)
	EntryCount() int // Total number of entries

	// Targeted extraction — more efficient than Open+Read for large archives.
	Extract(ctx context.Context, srcEntryPath, dstOSPath string) error
	ExtractSelected(ctx context.Context, entries []string, dstOSPath string, opts ExtractOptions) (*ExtractResult, error)

	Close() error
}

// ExtractOptions controls how entries are extracted to disk.
type ExtractOptions struct {
	// StripPrefix removes a leading path component from every entry before writing.
	// Example: StripPrefix="a/b" turns "a/b/c.txt" into "c.txt" at the destination.
	StripPrefix  string
	FlattenPaths bool // Drop all directory structure; write everything flat.
	PreserveTime bool // Restore mtime/atime from archive metadata.
	Progress     ProgressFunc
	// OnConflict is called when dstPath already exists.
	// Return the final path to write to, or ("", true, nil) to skip.
	OnConflict ConflictFunc
}

type ProgressFunc func(current, total int64, currentFile string)

// ExtractResult summarises the outcome of an ExtractSelected call.
type ExtractResult struct {
	Extracted int          // number of entries successfully written
	Bytes     int64        // total uncompressed bytes written
	Failed    int          // number of entries that could not be extracted
	Errors    []EntryError // per-entry errors (len == Failed)
}

// EntryError pairs an archive entry path with the error that occurred.
type EntryError struct {
	Entry string
	Err   error
}

// ConflictFunc is called when an extraction target already exists on disk.
// Returning ("", true, nil) skips the entry.
// Returning (path, false, nil) writes to path (which may differ from dst for renames).
type ConflictFunc func(src, dst string) (resolvedDst string, skip bool, err error)
