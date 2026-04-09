package cloudutil

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"sync"
	"time"

	"wails-app/vfs"
)

// CleanPath normalizes a VFS-relative remote path into slash-separated form.
func CleanPath(p string) string {
	trimmed := strings.TrimLeft(strings.ReplaceAll(p, `\`, `/`), "/")
	if trimmed == "" || trimmed == "." {
		return ""
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

// Info is a small read-only VFS file info implementation for cloud adapters.
type Info struct {
	NameValue    string
	SizeValue    int64
	ModifiedTime time.Time
	Dir          bool
}

func (fi *Info) Name() string { return fi.NameValue }
func (fi *Info) Size() int64  { return fi.SizeValue }
func (fi *Info) Mode() fs.FileMode {
	if fi.Dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (fi *Info) ModTime() time.Time   { return fi.ModifiedTime }
func (fi *Info) IsDir() bool          { return fi.Dir }
func (fi *Info) Sys() any             { return nil }
func (fi *Info) Path() string         { return fi.NameValue }
func (fi *Info) RealPath() string     { return "" }
func (fi *Info) Volume() vfs.Volume   { return nil }
func (fi *Info) IsArchive() bool      { return !fi.Dir && isArchiveExt(fi.NameValue) }
func (fi *Info) IsHidden() bool       { return strings.HasPrefix(fi.NameValue, ".") }
func (fi *Info) IsSymlink() bool      { return false }
func (fi *Info) IsNested() bool       { return false }
func (fi *Info) MountChain() []string { return nil }

var _ vfs.FileInfo = (*Info)(nil)

// Loader downloads file content on first access.
type Loader func(context.Context) ([]byte, error)

// ReadOnlyFile is a small in-memory, read-only file handle for cloud adapters.
type ReadOnlyFile struct {
	ctx      context.Context
	info     vfs.FileInfo
	loader   Loader
	loadOnce sync.Once
	loadErr  error
	data     []byte
	pos      int64
}

// NewReadOnlyFile returns a lazily-loaded in-memory file handle.
func NewReadOnlyFile(ctx context.Context, info vfs.FileInfo, loader Loader) *ReadOnlyFile {
	return &ReadOnlyFile{
		ctx:    ctx,
		info:   info,
		loader: loader,
	}
}

func (f *ReadOnlyFile) ensureLoaded() error {
	f.loadOnce.Do(func() {
		f.data, f.loadErr = f.loader(f.ctx)
	})
	return f.loadErr
}

func (f *ReadOnlyFile) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if err := f.ensureLoaded(); err != nil {
		return 0, err
	}
	if f.pos >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.pos:])
	f.pos += int64(n)
	return n, nil
}

func (f *ReadOnlyFile) Write(_ []byte) (int, error) {
	return 0, vfs.ErrReadOnly
}

func (f *ReadOnlyFile) Seek(offset int64, whence int) (int64, error) {
	if err := f.ensureLoaded(); err != nil {
		return 0, err
	}

	next := offset
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		next = f.pos + offset
	case io.SeekEnd:
		next = int64(len(f.data)) + offset
	default:
		return 0, fmt.Errorf("cloudutil: invalid seek whence %d", whence)
	}
	if next < 0 {
		return 0, fmt.Errorf("cloudutil: negative seek position")
	}
	f.pos = next
	return f.pos, nil
}

func (f *ReadOnlyFile) Close() error {
	return nil
}

func (f *ReadOnlyFile) Stat() (vfs.FileInfo, error) {
	return f.info, nil
}

func (f *ReadOnlyFile) Sync() error {
	return vfs.ErrReadOnly
}

var _ vfs.File = (*ReadOnlyFile)(nil)

func isArchiveExt(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".rar", ".7z"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
