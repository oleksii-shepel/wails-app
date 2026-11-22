package cloudutil

import (
	"io/fs"
	"path"
	"strings"
	"time"

	"wails-app/vfs"
	vfsarchive "wails-app/vfs/archive"
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

func isArchiveExt(name string) bool {
	return vfsarchive.IsArchiveExt(name)
}
