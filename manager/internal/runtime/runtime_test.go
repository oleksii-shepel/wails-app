package runtime

import (
	"context"
	"os"
	"testing"
	"time"

	contracts "wails-app/internal/contracts"
	vfslib "wails-app/vfs"
)

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
type testEvent struct {
	name string
	data interface{}
}

func newTestDirStreamService() (*DirStreamService, *[]testEvent) {
	var events []testEvent
	s := NewDirStreamService(
		func(name string, data interface{}) { events = append(events, testEvent{name, data}) },
		nil, nil, nil, nil,
	)
	return s, &events
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// DirStreamService lifecycle
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestDirStreamStartRequiresPath(t *testing.T) {
	s, _ := newTestDirStreamService()
	_, err := s.Start("left", "", false)
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestDirStreamStartReturnsID(t *testing.T) {
	s, _ := newTestDirStreamService()
	id, err := s.Start("left", "/tmp", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty stream ID")
	}
	s.Cancel(id)
}

func TestDirStreamRegisterUnregister(t *testing.T) {
	s, _ := newTestDirStreamService()
	_, cancel := context.WithCancel(context.Background())
	s.Register("stream-1", cancel)

	if s.streams["stream-1"] == nil {
		t.Fatal("expected stream to be registered")
	}

	s.Unregister("stream-1")
	if _, ok := s.streams["stream-1"]; ok {
		t.Fatal("expected stream to be unregistered")
	}

	// Ensure no panic on cancel of missing stream
	s.Cancel("missing")
}

func TestDirStreamCancelAll(t *testing.T) {
	s, _ := newTestDirStreamService()
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	s.Register("a", cancel1)
	s.Register("b", cancel2)

	s.CancelAll()

	select {
	case <-ctx1.Done():
		// expected
	case <-time.After(time.Second):
		t.Fatal("expected ctx1 to be cancelled")
	}
	select {
	case <-ctx2.Done():
		// expected
	case <-time.After(time.Second):
		t.Fatal("expected ctx2 to be cancelled")
	}

	if len(s.streams) != 0 {
		t.Fatalf("expected all streams cleared, got %d", len(s.streams))
	}
}

func TestDirStreamEmitNoOp(t *testing.T) {
	s := NewDirStreamService(nil, nil, nil, nil, nil)
	// Should not panic
	s.Emit("test", "data")
}

func TestDirStreamSetVFS(t *testing.T) {
	s, _ := newTestDirStreamService()
	s.SetVFS(
		func(path string) (*contracts.DirectoryListing, error) { return nil, nil },
		func(path string) bool { return true },
	)
	if s.vfsListDir == nil || s.isVFSPath == nil {
		t.Fatal("expected VFS handlers to be set")
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
type testVFSInfo struct {
	name     string
	path     string
	size     int64
	modTime  time.Time
	isDir    bool
	isArch   bool
	isSym    bool
	isHidden bool
}

func (t testVFSInfo) Name() string       { return t.name }
func (t testVFSInfo) Size() int64        { return t.size }
func (t testVFSInfo) Mode() os.FileMode  { return 0 }
func (t testVFSInfo) ModTime() time.Time { return t.modTime }
func (t testVFSInfo) IsDir() bool        { return t.isDir }
func (t testVFSInfo) Sys() interface{}   { return nil }
func (t testVFSInfo) Path() string       { return t.path }
func (t testVFSInfo) RealPath() string   { return t.path }
func (t testVFSInfo) Volume() vfslib.Volume { return nil }
func (t testVFSInfo) IsArchive() bool    { return t.isArch }
func (t testVFSInfo) IsHidden() bool     { return t.isHidden }
func (t testVFSInfo) IsSymlink() bool    { return t.isSym }
func (t testVFSInfo) IsNested() bool     { return false }
func (t testVFSInfo) MountChain() []string { return nil }

func TestConvertVFSInfo(t *testing.T) {
	info := testVFSInfo{
		name: "file.txt", path: "/a/file.txt", size: 42,
		modTime: time.Unix(1000, 0), isDir: false, isArch: false,
		isSym: false, isHidden: false,
	}
	fi := convertVFSInfo(info)
	if fi.Name != "file.txt" || fi.Path != "/a/file.txt" || fi.Size != 42 {
		t.Fatalf("basic fields mismatch: %+v", fi)
	}
	if fi.Type != contracts.FileTypeFile {
		t.Fatalf("expected file type, got %s", fi.Type)
	}
	if fi.IsNested {
		t.Fatal("expected not nested")
	}
}

func TestConvertVFSInfoDirectory(t *testing.T) {
	info := testVFSInfo{name: "dir", path: "/a/dir", isDir: true}
	fi := convertVFSInfo(info)
	if fi.Type != contracts.FileTypeDirectory {
		t.Fatalf("expected directory type, got %s", fi.Type)
	}
}

func TestConvertVFSInfoArchive(t *testing.T) {
	info := testVFSInfo{name: "archive.zip", path: "/a/archive.zip", isArch: true}
	fi := convertVFSInfo(info)
	if fi.Type != contracts.FileTypeArchive {
		t.Fatalf("expected archive type, got %s", fi.Type)
	}
	if !fi.IsNested {
		t.Fatal("expected archive to be marked nested")
	}
	if len(fi.NestedChain) != 1 || fi.NestedChain[0] != "/a/archive.zip" {
		t.Fatalf("unexpected nested chain: %v", fi.NestedChain)
	}
}

func TestConvertVFSInfoSymlink(t *testing.T) {
	info := testVFSInfo{name: "link", path: "/a/link", isSym: true}
	fi := convertVFSInfo(info)
	if fi.Type != contracts.FileTypeSymlink {
		t.Fatalf("expected symlink type, got %s", fi.Type)
	}
}

func TestConvertVFSInfoWithChain(t *testing.T) {
	info := testVFSInfo{name: "nested.zip", path: "/a/nested.zip", isArch: true}
	fi := convertVFSInfoWithChain(info, []string{"/outer.zip"})
	if len(fi.NestedChain) != 2 || fi.NestedChain[0] != "/outer.zip" || fi.NestedChain[1] != "/a/nested.zip" {
		t.Fatalf("unexpected nested chain: %v", fi.NestedChain)
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
type testVolume struct{}

func (testVolume) ID() string             { return "v1" }
func (testVolume) Name() string           { return "test" }
func (testVolume) Type() vfslib.VolumeType { return vfslib.VolumeLocal }
func (testVolume) Root() string           { return "/" }
func (testVolume) IsReadOnly() bool       { return false }
func (testVolume) Unmount() error         { return nil }

type testVFSInfoWithVolume struct {
	testVFSInfo
	vol vfslib.Volume
}

func (t testVFSInfoWithVolume) Volume() vfslib.Volume { return t.vol }

func TestConvertVFSInfoWithVolume(t *testing.T) {
	info := testVFSInfoWithVolume{
		testVFSInfo: testVFSInfo{name: "f.txt", path: "/f.txt", size: 10},
		vol:         testVolume{},
	}
	fi := convertVFSInfo(info)
	if fi.Name != "f.txt" {
		t.Fatal("name mismatch")
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// joinPublicPath
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestJoinPublicPath(t *testing.T) {
	tests := []struct {
		base string
		name string
		want string
	}{
		{"/home", "docs", "/home/docs"},
		{"/home/", "docs", "/home/docs"},
		{"/", "docs", "/docs"},
		{"", "docs", "/docs"},
		{"/home", "a/b", "/home/a/b"},
	}
	for _, tc := range tests {
		got := joinPublicPath(tc.base, tc.name)
		if got != tc.want {
			t.Fatalf("joinPublicPath(%q, %q) = %q, want %q", tc.base, tc.name, got, tc.want)
		}
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// archiveMountPoint
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestArchiveMountPoint(t *testing.T) {
	got := archiveMountPoint("/tmp/archive.zip", nil)
	if got != "/archives/tmp/archive.zip" {
		t.Fatalf("unexpected mount point: %s", got)
	}

	got = archiveMountPoint("/tmp/archive.zip", []string{"inner", "deep"})
	if got != "/archives/tmp/archive.zip/inner/deep" {
		t.Fatalf("unexpected nested mount point: %s", got)
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
// normalizeNativeWatchPath
// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

func TestNormalizeNativeWatchPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user", "/home/user"},
		{"/home/user/", "/home/user"},
		{"  /home/user  ", "/home/user"},
		{"", ""},
		{"registry://some/key", ""},
	}
	for _, tc := range tests {
		got := normalizeNativeWatchPath(tc.input)
		if got != tc.want {
			t.Fatalf("normalizeNativeWatchPath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
type FileInfo = vfslib.FileInfo

func TestPaneWatchManagerNormalizeWatchPath(t *testing.T) {
	m := NewPaneWatchManager(nil, nil, nil)
	defer m.Close()

	got, err := m.normalizeWatchPath("/home")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/home" {
		t.Fatalf("expected /home, got %s", got)
	}

	got, _ = m.normalizeWatchPath("")
	if got != "" {
		t.Fatalf("expected empty for empty input, got %s", got)
	}

	got, _ = m.normalizeWatchPath("registry://HKLM/Software")
	if got != "" {
		t.Fatalf("expected empty for registry path, got %s", got)
	}
}

func TestPaneWatchManagerToEventPath(t *testing.T) {
	m := NewPaneWatchManager(nil, nil, func(s string) string { return "public://" + s })
	defer m.Close()

	got := m.toEventPath("/home")
	if got != "public:///home" {
		t.Fatalf("expected public:///home, got %s", got)
	}

	got = m.toEventPath("")
	if got != "" {
		t.Fatalf("expected empty, got %s", got)
	}
}

func TestPaneWatchManagerSetPaneDirectory(t *testing.T) {
	var events []testEvent
	watchPath := t.TempDir()
	m := NewPaneWatchManager(
		func(name string, data interface{}) { events = append(events, testEvent{name, data}) },
		func(s string) (string, error) { return s, nil },
		nil,
	)
	defer m.Close()

	err := m.SetPaneDirectory("left", watchPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	normalizedPath := normalizeNativeWatchPath(watchPath)
	if m.panePaths["left"] != normalizedPath {
		t.Fatalf("expected pane path %q, got %q", normalizedPath, m.panePaths["left"])
	}

	// Setting same path again should be no-op.
	err = m.SetPaneDirectory("left", watchPath)
	if err != nil {
		t.Fatalf("unexpected error on repeat: %v", err)
	}

	// Clear path.
	err = m.SetPaneDirectory("left", "")
	if err != nil {
		t.Fatalf("unexpected error on clear: %v", err)
	}
	if _, ok := m.panePaths["left"]; ok {
		t.Fatal("expected pane path to be cleared")
	}
}

func TestPaneWatchManagerCloseIdempotent(t *testing.T) {
	m := NewPaneWatchManager(nil, nil, nil)
	m.Close()
	m.Close() // should not panic
}

func TestPaneWatchManagerAddRemoveWatchRefCount(t *testing.T) {
	m := NewPaneWatchManager(nil, nil, nil)
	defer m.Close()

	watchPath := normalizeNativeWatchPath(t.TempDir())

	if err := m.addWatchLocked(watchPath); err != nil {
		t.Fatalf("unexpected error adding first watch: %v", err)
	}
	if m.pathRef[watchPath] != 1 {
		t.Fatalf("expected ref count 1, got %d", m.pathRef[watchPath])
	}

	if err := m.addWatchLocked(watchPath); err != nil {
		t.Fatalf("unexpected error adding second watch: %v", err)
	}
	if m.pathRef[watchPath] != 2 {
		t.Fatalf("expected ref count 2, got %d", m.pathRef[watchPath])
	}

	m.removeWatchLocked(watchPath)
	if m.pathRef[watchPath] != 1 {
		t.Fatalf("expected ref count 1 after remove, got %d", m.pathRef[watchPath])
	}

	m.removeWatchLocked(watchPath)
	if _, ok := m.pathRef[watchPath]; ok {
		t.Fatal("expected path ref to be deleted")
	}
}