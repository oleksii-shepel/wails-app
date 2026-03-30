// Package osfs adapts the real OS filesystem to the vfs.FileSystem interface.
package osfs

import (
	"context"
	"io/fs"
	"os"
	"runtime"
	"path/filepath"
	"strings"

	"wails-app/vfs"
)

// FS is a vfs.FileSystem backed by the real OS filesystem.
type FS struct{ root string }

// New creates an FS rooted at osRoot. Pass "/" to expose the whole machine.
func New(osRoot string) *FS {
	return &FS{root: filepath.Clean(osRoot)}
}

// abs translates a VFS path to an absolute OS path, preventing path traversal.
func (f *FS) abs(p string) string {
	clean := filepath.FromSlash(filepath.Clean("/" + p))
	if runtime.GOOS == "windows" && (f.root == filepath.Clean("/") || f.root == `\`) {
		trimmed := strings.TrimPrefix(filepath.ToSlash(clean), "/")
		if len(trimmed) >= 2 && isWindowsDriveSegment(trimmed) {
			drivePath := strings.ToUpper(trimmed[:1]) + ":"
			remainder := strings.TrimPrefix(trimmed[1:], "/")
			if remainder == "" {
				return filepath.Clean(drivePath + `/`)
			}
			return filepath.FromSlash(drivePath + `/` + remainder)
		}
	}
	return filepath.Join(f.root, clean)
}

func (f *FS) Open(_ context.Context, path string) (vfs.File, error) {
	fh, err := os.Open(f.abs(path))
	if err != nil {
		return nil, mapErr(err)
	}
	return &file{fh}, nil
}

func (f *FS) Stat(_ context.Context, path string) (vfs.FileInfo, error) {
	info, err := os.Lstat(f.abs(path))
	if err != nil {
		return nil, mapErr(err)
	}
	return &fileInfo{FileInfo: info, vfsPath: filepath.ToSlash(path), osPath: f.abs(path)}, nil
}

func (f *FS) ReadDir(_ context.Context, path string) ([]vfs.FileInfo, error) {
	entries, err := os.ReadDir(f.abs(path))
	if err != nil {
		return nil, mapErr(err)
	}
	result := make([]vfs.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		childVFS := filepath.ToSlash(filepath.Join(path, e.Name()))
		result = append(result, &fileInfo{
			FileInfo: info,
			vfsPath:  childVFS,
			osPath:   filepath.Join(f.abs(path), e.Name()),
		})
	}
	return result, nil
}

func (f *FS) Exists(_ context.Context, path string) bool {
	_, err := os.Lstat(f.abs(path))
	return err == nil
}

func (f *FS) Create(_ context.Context, path string) (vfs.File, error) {
	osPath := f.abs(path)
	if err := os.MkdirAll(filepath.Dir(osPath), 0755); err != nil {
		return nil, err
	}
	fh, err := os.Create(osPath)
	if err != nil {
		return nil, err
	}
	return &file{fh}, nil
}

func (f *FS) Mkdir(_ context.Context, path string, perm fs.FileMode) error {
	return os.MkdirAll(f.abs(path), perm)
}

func (f *FS) Remove(_ context.Context, path string) error {
	return os.RemoveAll(f.abs(path))
}

func (f *FS) Rename(_ context.Context, old, new string) error {
	return os.Rename(f.abs(old), f.abs(new))
}

func (f *FS) Join(elem ...string) string { return filepath.ToSlash(filepath.Join(elem...)) }
func (f *FS) Split(p string) (string, string) {
	d, n := filepath.Split(p)
	return filepath.ToSlash(d), n
}
func (f *FS) IsAbs(p string) bool               { return filepath.IsAbs(p) }
func (f *FS) RealPath(p string) (string, error) { return f.abs(p), nil }

var _ vfs.FileSystem = (*FS)(nil)

// ── fileInfo ──────────────────────────────────────────────────────────────────

type fileInfo struct {
	os.FileInfo
	vfsPath string
	osPath  string
}

func (fi *fileInfo) Path() string         { return fi.vfsPath }
func (fi *fileInfo) RealPath() string     { return fi.osPath }
func (fi *fileInfo) Volume() vfs.Volume   { return nil }
func (fi *fileInfo) IsArchive() bool      { return !fi.IsDir() && isArchiveExt(fi.Name()) }
func (fi *fileInfo) IsHidden() bool       { return strings.HasPrefix(fi.Name(), ".") }
func (fi *fileInfo) IsSymlink() bool      { return fi.FileInfo.Mode()&os.ModeSymlink != 0 }
func (fi *fileInfo) IsNested() bool       { return false }
func (fi *fileInfo) MountChain() []string { return nil }

var _ vfs.FileInfo = (*fileInfo)(nil)

// ── file ──────────────────────────────────────────────────────────────────────

type file struct{ *os.File }

func (f *file) Stat() (vfs.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &fileInfo{FileInfo: info, osPath: f.File.Name(), vfsPath: filepath.ToSlash(f.File.Name())}, nil
}
func (f *file) Sync() error { return f.File.Sync() }

var _ vfs.File = (*file)(nil)

// ── helpers ───────────────────────────────────────────────────────────────────

func isArchiveExt(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".rar", ".7z"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func mapErr(err error) error {
	if os.IsNotExist(err) {
		return vfs.ErrNotFound
	}
	if os.IsPermission(err) {
		return vfs.ErrReadOnly
	}
	return err
}

func isWindowsDriveSegment(path string) bool {
	if len(path) < 1 {
		return false
	}
	drive := path[0]
	return ((drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')) &&
		(len(path) == 1 || path[1] == '/')
}
