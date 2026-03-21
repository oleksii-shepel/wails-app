package filesystem

import (
	"fmt"
	"path/filepath"
	"testing"
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

func TestResolvePublicPathKeepsDrivePathsNative(t *testing.T) {
	resolver := &stubMemoryResolver{}
	service := NewService(ServiceConfig{MemoryResolver: resolver})

	got, err := service.resolvePublicPath("drive://c/New Folder")
	if err != nil {
		t.Fatalf("resolvePublicPath returned error: %v", err)
	}

	want := filepath.FromSlash("C:/New Folder")
	if got != want {
		t.Fatalf("resolvePublicPath mismatch\nwant: %q\n got: %q", want, got)
	}
	if resolver.called {
		t.Fatal("memory resolver should not be called for drive paths")
	}
}

func TestResolvePublicPathUsesMemoryResolverForMemoryScheme(t *testing.T) {
	resolver := &stubMemoryResolver{}
	service := NewService(ServiceConfig{MemoryResolver: resolver})

	got, err := service.resolvePublicPath("memory://New Folder")
	if err != nil {
		t.Fatalf("resolvePublicPath returned error: %v", err)
	}

	if !resolver.called {
		t.Fatal("memory resolver should be called for memory paths")
	}
	if got == filepath.FromSlash("memory://New Folder") {
		t.Fatalf("expected memory resolver output, got unchanged path %q", got)
	}
}
