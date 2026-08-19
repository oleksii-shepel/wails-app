package memory

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"wails-app/vfs"
	"wails-app/vfs/osfs"
)

var connectionInfoFn = GetConnectionInfo

func DefaultRootProvider() string {
	if root := os.Getenv("WAILS_MEMFS_ROOT"); strings.TrimSpace(root) != "" {
		return root
	}
	if root := os.Getenv("MEMFS_HIDDEN_PATH"); strings.TrimSpace(root) != "" {
		return root
	}
	// GetConnectionInfo works on all platforms: Windows uses a named pipe,
	// Unix uses a domain socket. Failure is non-fatal — fall through to the
	// store-backed default.
	if info, err := connectionInfoFn(); err == nil {
		if root := strings.TrimSpace(info.HiddenPath); root != "" {
			return root
		}
	}
	return ""
}

type FS struct {
	resolver *PathResolver
}

func New(appName string, configuredRoot func() string) (*FS, error) {
	resolver, err := NewPathResolver(appName, configuredRoot)
	if err != nil {
		return nil, err
	}
	return &FS{resolver: resolver}, nil
}

func (f *FS) ResolvePublicPath(path string) (string, bool, error) {
	return f.resolver.ResolvePublicPath(path)
}

func (f *FS) ToPublicPath(path string) string {
	return f.resolver.ToPublicPath(path)
}

func (f *FS) BackingRoot() string {
	return f.resolver.BackingRoot()
}

func (f *FS) backend() *osfs.FS {
	return osfs.New(filepath.FromSlash(f.resolver.BackingRoot()))
}

func (f *FS) Open(ctx context.Context, path string) (vfs.File, error) {
	return f.backend().Open(ctx, path)
}

func (f *FS) Stat(ctx context.Context, path string) (vfs.FileInfo, error) {
	return f.backend().Stat(ctx, path)
}

func (f *FS) ReadDir(ctx context.Context, path string) ([]vfs.FileInfo, error) {
	return f.backend().ReadDir(ctx, path)
}

func (f *FS) Exists(ctx context.Context, path string) bool {
	return f.backend().Exists(ctx, path)
}

func (f *FS) Create(ctx context.Context, path string) (vfs.File, error) {
	return f.backend().Create(ctx, path)
}

func (f *FS) Mkdir(ctx context.Context, path string, perm fs.FileMode) error {
	return f.backend().Mkdir(ctx, path, perm)
}

func (f *FS) Remove(ctx context.Context, path string) error {
	return f.backend().Remove(ctx, path)
}

func (f *FS) Rename(ctx context.Context, oldPath, newPath string) error {
	return f.backend().Rename(ctx, oldPath, newPath)
}

func (f *FS) Join(elem ...string) string {
	return f.backend().Join(elem...)
}

func (f *FS) Split(path string) (dir, file string) {
	return f.backend().Split(path)
}

func (f *FS) IsAbs(path string) bool {
	return f.backend().IsAbs(path)
}

func (f *FS) RealPath(vfsPath string) (string, error) {
	return f.backend().RealPath(vfsPath)
}

var _ vfs.FileSystem = (*FS)(nil)