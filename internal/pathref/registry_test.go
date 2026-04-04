package pathref

import (
	"path/filepath"
	"testing"
)

func TestToNativePathDriveRootUsesPlatformSeparator(t *testing.T) {
	got := ToNativePath("filesystem://c")
	want := filepath.FromSlash("C:/")
	if got != want {
		t.Fatalf("ToNativePath(drive root) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestToNativePathDriveChildUsesPlatformSeparator(t *testing.T) {
	got := ToNativePath("filesystem://c/New Folder")
	want := filepath.FromSlash("C:/New Folder")
	if got != want {
		t.Fatalf("ToNativePath(drive child) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestToVFSAddsDefaultFTPPort(t *testing.T) {
	got := ToVFS("ftp://samples.ffmpeg.org/pub")
	want := "/remote/ftp/samples.ffmpeg.org:21/pub"
	if got != want {
		t.Fatalf("ToVFS(ftp) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestToVFSAddsDefaultSFTPPort(t *testing.T) {
	got := ToVFS("sftp://example.com/home")
	want := "/remote/sftp/example.com:22/home"
	if got != want {
		t.Fatalf("ToVFS(sftp) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestToVFSAddsDefaultSMBPort(t *testing.T) {
	got := ToVFS("smb://fileserver/share/docs")
	want := "/remote/smb/fileserver:445/share/docs"
	if got != want {
		t.Fatalf("ToVFS(smb) mismatch\nwant: %q\n got: %q", want, got)
	}
}
