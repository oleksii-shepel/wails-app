package filesystem

import (
	"path/filepath"
	"testing"
)

func TestResolvePublicPathKeepsDrivePathsNative(t *testing.T) {
	service := NewService(ServiceConfig{})

	got, err := service.resolvePublicPath("filesystem://c/New Folder")
	if err != nil {
		t.Fatalf("resolvePublicPath returned error: %v", err)
	}

	want := filepath.FromSlash("C:/New Folder")
	if got != want {
		t.Fatalf("resolvePublicPath mismatch\nwant: %q\n got: %q", want, got)
	}
}
