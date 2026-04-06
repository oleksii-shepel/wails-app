package webdav

import "testing"

func TestServerURLRoot(t *testing.T) {
	fs := &FS{baseURL: "https://example.test:443/Users"}

	if got := fs.serverURL("/"); got != "https://example.test:443/Users/" {
		t.Fatalf("serverURL(/) = %q", got)
	}
	if got := fs.serverURL(""); got != "https://example.test:443/Users/" {
		t.Fatalf("serverURL(\"\") = %q", got)
	}
}

func TestServerURLPreservesPathSeparators(t *testing.T) {
	fs := &FS{baseURL: "https://example.test:443/Users"}

	got := fs.serverURL("/folder one/nested/file#.txt")
	want := "https://example.test:443/Users/folder%20one/nested/file%23.txt"
	if got != want {
		t.Fatalf("serverURL returned %q, want %q", got, want)
	}
}
