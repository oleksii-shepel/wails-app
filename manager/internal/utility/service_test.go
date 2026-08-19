package utility

import (
	"context"
	"fmt"
	"io/fs"
	"testing"

	vfslib "wails-app/vfs"
)

type stubMemoryResolver struct {
	called bool
}

func (s *stubMemoryResolver) ResolvePublicPath(path string) (string, bool, error) {
	s.called = true
	return fmt.Sprintf("resolver:%s", path), true, nil
}

func (s *stubMemoryResolver) ToPublicPath(path string) string {
	return path
}

type stubMounter struct {
	resolvedPath string
	resolveCalls int
}

func (s *stubMounter) Mount(mountPoint string, fs vfslib.FileSystem, readOnly bool, name string, vtype vfslib.VolumeType) (vfslib.Volume, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubMounter) Unmount(mountPoint string) error {
	return fmt.Errorf("not implemented")
}

func (s *stubMounter) Resolve(path string) (vfslib.FileSystem, string, vfslib.Volume, error) {
	s.resolveCalls++
	if path == "/memory/demo.txt" {
		return &stubFileSystem{realPath: s.resolvedPath}, "demo.txt", &stubVolume{}, nil
	}
	return &stubFileSystem{}, "", nil, fmt.Errorf("unmounted path: %s", path)
}

func (s *stubMounter) ListMounts() []vfslib.MountedEntry {
	return nil
}

type stubFileSystem struct {
	realPath string
}

func (s *stubFileSystem) Open(ctx context.Context, path string) (vfslib.File, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubFileSystem) Stat(ctx context.Context, path string) (vfslib.FileInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubFileSystem) ReadDir(ctx context.Context, path string) ([]vfslib.FileInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubFileSystem) Exists(ctx context.Context, path string) bool {
	return false
}

func (s *stubFileSystem) Create(ctx context.Context, path string) (vfslib.File, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *stubFileSystem) Mkdir(ctx context.Context, path string, perm fs.FileMode) error {
	return fmt.Errorf("not implemented")
}

func (s *stubFileSystem) Remove(ctx context.Context, path string) error {
	return fmt.Errorf("not implemented")
}

func (s *stubFileSystem) Rename(ctx context.Context, oldPath, newPath string) error {
	return fmt.Errorf("not implemented")
}

func (s *stubFileSystem) Join(elem ...string) string {
	return ""
}

func (s *stubFileSystem) Split(path string) (dir, file string) {
	return "", ""
}

func (s *stubFileSystem) IsAbs(path string) bool {
	return false
}

func (s *stubFileSystem) RealPath(vfsPath string) (string, error) {
	return s.realPath, nil
}

type stubVolume struct{}

func (s *stubVolume) ID() string              { return "stub" }
func (s *stubVolume) Name() string            { return "stub" }
func (s *stubVolume) Type() vfslib.VolumeType { return vfslib.VolumeMemory }
func (s *stubVolume) Root() string            { return "/memory" }
func (s *stubVolume) IsReadOnly() bool        { return true }
func (s *stubVolume) Unmount() error          { return nil }

func TestResolvePublicPathPrefersVFSForMountedAlias(t *testing.T) {
	resolver := &stubMemoryResolver{}
	mounter := &stubMounter{resolvedPath: "C:/Temp/demo.txt"}
	service := NewService(ServiceConfig{MemoryResolver: resolver})
	service.SetVFS(vfslib.NewUnified(mounter))

	got, err := service.resolvePublicPath("memory://demo.txt")
	if err != nil {
		t.Fatalf("resolvePublicPath returned error: %v", err)
	}

	if got != "C:/Temp/demo.txt" {
		t.Fatalf("resolvePublicPath mismatch\nwant: %q\n got: %q", "C:/Temp/demo.txt", got)
	}
	if resolver.called {
		t.Fatal("memory resolver should not be called for mounted alias paths")
	}
	if mounter.resolveCalls == 0 {
		t.Fatal("expected VFS resolver to be used")
	}
}

func TestResolvePublicPathRejectsVirtualOnlyAlias(t *testing.T) {
	service := NewService(ServiceConfig{})
	service.SetVFS(vfslib.NewUnified(&stubMounter{resolvedPath: ""}))

	_, err := service.resolvePublicPath("memory://demo.txt")
	if err == nil {
		t.Fatal("expected error for virtual-only path")
	}
	if got := err.Error(); got != "path \"memory://demo.txt\" is virtual-only and not available to the OS" {
		t.Fatalf("unexpected error: %q", got)
	}
}