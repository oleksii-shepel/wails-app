package zip

import (
	"archive/zip"
	"context"
	"encoding/binary"

	"vfs"
	"vfs/archive"
)

const (
	magicPK0304 = 0x504B0304 // Local file header
	magicPK0506 = 0x504B0506 // Empty archive
)

type Handler struct{}

func (h *Handler) Name() string { return "zip" }

func (h *Handler) Extensions() []string { return []string{".zip"} }

func (h *Handler) Magic() []byte { return []byte{0x50, 0x4B, 0x03, 0x04} }

func (h *Handler) CanRead() bool   { return true }
func (h *Handler) CanWrite() bool  { return true }
func (h *Handler) CanCreate() bool { return true }
func (h *Handler) CanStream() bool { return true }

func (h *Handler) Detect(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	sig := binary.LittleEndian.Uint32(data[:4])
	return sig == magicPK0304 || sig == magicPK0506
}

func (h *Handler) Open(ctx context.Context, path string, opts archive.OpenOptions) (archive.Archive, error) {
	if len(opts.NestedChain) > 0 {
		return h.openNested(ctx, path, opts.NestedChain)
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}

	return &Archive{
		reader:   r,
		path:     path,
		readOnly: opts.ReadOnly,
	}, nil
}

// Archive implements archive.Archive and vfs.FileSystem
type Archive struct {
	reader   *zip.ReadCloser
	path     string
	readOnly bool
	modified bool // Track if archive needs rewrite
}

func (a *Archive) Open(ctx context.Context, path string) (vfs.File, error) {
	// Implementation
}

func (a *Archive) Stat(ctx context.Context, path string) (vfs.FileInfo, error) {
	// Implementation using zip.FileHeader
}

func (a *Archive) ReadDir(ctx context.Context, path string) ([]vfs.FileInfo, error) {
	// Virtual directory listing - one level only
}

func (a *Archive) Extract(ctx context.Context, srcPath, dstPath string) error {
	// Extract single file
}

func (a *Archive) ExtractSelected(ctx context.Context, entries []string, dstPath string, opts archive.ExtractOptions) error {
	// Batch extraction with progress
}

func (a *Archive) Add(ctx context.Context, srcPath, dstPath string) error {
	if a.readOnly {
		return vfs.ErrReadOnly
	}
	// Queue for rewrite or use temp file
}

func (a *Archive) Remove(ctx context.Context, path string) error {
	if a.readOnly {
		return vfs.ErrReadOnly
	}
	// Mark for removal, rewrite on close
}

func (a *Archive) Close() error {
	// If modified, rewrite archive
	return a.reader.Close()
}

// Implement vfs.FileSystem interface methods...
// Create, Mkdir, Remove, Rename return ErrReadOnly or implement via rewrite

var _ archive.Handler = (*Handler)(nil)
var _ archive.Archive = (*Archive)(nil)
var _ vfs.FileSystem = (*Archive)(nil)
