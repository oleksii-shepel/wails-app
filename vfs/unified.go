package vfs

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// Mounter is the interface Unified requires from a mount manager.
// mount.Manager satisfies this via the adapter in vfs/mount.
type Mounter interface {
	Mount(mountPoint string, fs FileSystem, readOnly bool, name string, vtype VolumeType) (Volume, error)
	Unmount(mountPoint string) error
	Resolve(path string) (FileSystem, string, Volume, error)
	ListMounts() []MountedEntry
}

// MountedEntry describes one active mount point.
type MountedEntry struct {
	MountPoint string
	FS         FileSystem
	Volume     Volume
	ReadOnly   bool
}

// Unified is the top-level VFS the application uses.
// Construct it with NewUnified, then call Open/Stat/ReadDir/etc. on it.
//
// To support archive formats, blank-import their packages in main.go:
//
//	import _ "wails-app/vfs/archive/zip"
//	import _ "wails-app/vfs/archive/tar"
type Unified struct {
	mounts Mounter
}

// NewUnified creates a Unified VFS backed by the given Mounter.
func NewUnified(mounts Mounter) *Unified {
	return &Unified{mounts: mounts}
}

func (u *Unified) Mount(mountPoint string, fs FileSystem, readOnly bool, name string, vtype VolumeType) (Volume, error) {
	return u.mounts.Mount(mountPoint, fs, readOnly, name, vtype)
}

func (u *Unified) Unmount(mountPoint string) error {
	return u.mounts.Unmount(mountPoint)
}

func (u *Unified) Open(ctx context.Context, path string) (File, error) {
	fsys, rel, _, err := u.mounts.Resolve(path)
	if err != nil {
		return nil, err
	}
	return fsys.Open(ctx, rel)
}

func (u *Unified) Stat(ctx context.Context, path string) (FileInfo, error) {
	fsys, rel, _, err := u.mounts.Resolve(path)
	if err != nil {
		return nil, err
	}
	return fsys.Stat(ctx, rel)
}

func (u *Unified) ReadDir(ctx context.Context, path string) ([]FileInfo, error) {
	fsys, rel, _, err := u.mounts.Resolve(path)
	if err != nil {
		return nil, err
	}
	entries, err := fsys.ReadDir(ctx, rel)
	if err != nil {
		return nil, err
	}
	// Inject synthetic directory entries for sub-mounts under path.
	cleanPath := filepath.ToSlash(filepath.Clean("/" + path))
	for _, m := range u.mounts.ListMounts() {
		if filepath.ToSlash(filepath.Dir(m.MountPoint)) == cleanPath {
			entries = append(entries, &mountPointInfo{
				name: filepath.Base(m.MountPoint),
				path: m.MountPoint,
				vol:  m.Volume,
			})
		}
	}
	return entries, nil
}

func (u *Unified) Exists(ctx context.Context, path string) bool {
	_, err := u.Stat(ctx, path)
	return err == nil
}

func (u *Unified) Create(ctx context.Context, path string) (File, error) {
	fsys, rel, _, err := u.mounts.Resolve(path)
	if err != nil {
		return nil, err
	}
	return fsys.Create(ctx, rel)
}

func (u *Unified) Mkdir(ctx context.Context, path string, perm fs.FileMode) error {
	fsys, rel, _, err := u.mounts.Resolve(path)
	if err != nil {
		return err
	}
	return fsys.Mkdir(ctx, rel, perm)
}

func (u *Unified) Remove(ctx context.Context, path string) error {
	fsys, rel, _, err := u.mounts.Resolve(path)
	if err != nil {
		return err
	}
	return fsys.Remove(ctx, rel)
}

func (u *Unified) Rename(ctx context.Context, oldPath, newPath string) error {
	fsOld, relOld, volOld, err := u.mounts.Resolve(oldPath)
	if err != nil {
		return err
	}
	_, relNew, volNew, err := u.mounts.Resolve(newPath)
	if err != nil {
		return err
	}
	if volOld != volNew {
		return fmt.Errorf("vfs: cannot rename across filesystems")
	}
	return fsOld.Rename(ctx, relOld, relNew)
}

// IsMounted reports whether path is currently served by a non-root (non-osfs)
// mount — i.e. the path resolves inside an archive or other special FS.
// Returns false for plain OS paths that resolve through the root osfs.
func (u *Unified) IsMounted(path string) bool {
	_, _, vol, err := u.mounts.Resolve(path)
	return err == nil && vol != nil
}

func (u *Unified) Join(elem ...string) string { return strings.Join(elem, "/") }
func (u *Unified) Split(path string) (string, string) {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "", path
	}
	return path[:i], path[i+1:]
}
func (u *Unified) IsAbs(path string) bool { return strings.HasPrefix(path, "/") }
func (u *Unified) RealPath(path string) (string, error) {
	fsys, rel, _, err := u.mounts.Resolve(path)
	if err != nil {
		return "", err
	}
	return fsys.RealPath(rel)
}

// mountPointInfo is a synthetic FileInfo injected for each active sub-mount.
type mountPointInfo struct {
	name string
	path string
	vol  Volume
}

func (m *mountPointInfo) Name() string         { return m.name }
func (m *mountPointInfo) Size() int64          { return 0 }
func (m *mountPointInfo) Mode() fs.FileMode    { return fs.ModeDir | 0555 }
func (m *mountPointInfo) ModTime() time.Time   { return time.Time{} }
func (m *mountPointInfo) IsDir() bool          { return true }
func (m *mountPointInfo) Sys() any             { return nil }
func (m *mountPointInfo) Path() string         { return m.path }
func (m *mountPointInfo) RealPath() string     { return "" }
func (m *mountPointInfo) Volume() Volume       { return m.vol }
func (m *mountPointInfo) IsArchive() bool      { return false }
func (m *mountPointInfo) IsHidden() bool       { return false }
func (m *mountPointInfo) IsSymlink() bool      { return false }
func (m *mountPointInfo) IsNested() bool       { return false }
func (m *mountPointInfo) MountChain() []string { return nil }

var _ FileInfo = (*mountPointInfo)(nil)
