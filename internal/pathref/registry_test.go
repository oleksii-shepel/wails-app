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
