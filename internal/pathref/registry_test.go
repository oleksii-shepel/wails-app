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

func TestToVFSAddsDefaultSMBPort(t *testing.T) {
	got := ToVFS("smb://fileserver/share/docs")
	want := "/remote/smb/fileserver:445/share/docs"
	if got != want {
		t.Fatalf("ToVFS(smb) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestParentKeepsHTTPAliasesNavigable(t *testing.T) {
	got, ok := Parent("https://example.com/app/assets/main.js")
	if !ok {
		t.Fatalf("Parent(http alias) did not match")
	}

	want := "https://example.com/app/assets"
	if got != want {
		t.Fatalf("Parent(http alias) mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestIsWebRecognizesHTTPAliases(t *testing.T) {
	if !IsWeb("http://example.com") {
		t.Fatalf("expected http alias to be recognized as web")
	}
	if !IsWeb("https://example.com") {
		t.Fatalf("expected https alias to be recognized as web")
	}
	if IsWeb("smb://example.com/share") {
		t.Fatalf("did not expect smb alias to be recognized as web")
	}
}
