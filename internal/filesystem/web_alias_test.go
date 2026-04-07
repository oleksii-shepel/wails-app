package filesystem

import (
	"net/http"
	"net/http/httptest"
	"testing"

	contracts "wails-app/internal/contracts"
)

func TestWebAliasListDirectoryDiscoversLinkedEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head></head><body><a href="/docs/page.html">Docs</a><a href="/assets/app.js">App</a><script src="/assets/app.js"></script></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newWebAliasService()
	listing, err := service.listDirectory(server.URL + "/index.html")
	if err != nil {
		t.Fatalf("listDirectory returned error: %v", err)
	}

	if listing.Path != server.URL+"/index.html" {
		t.Fatalf("unexpected listing path: %q", listing.Path)
	}
	if len(listing.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(listing.Entries))
	}

	expected := map[string]contracts.FileType{
		server.URL + "/assets/app.js":  contracts.FileTypeFile,
		server.URL + "/docs/page.html": contracts.FileTypeDirectory,
	}
	for _, entry := range listing.Entries {
		wantType, ok := expected[entry.Path]
		if !ok {
			t.Fatalf("unexpected entry path: %q", entry.Path)
		}
		if entry.Type != wantType {
			t.Fatalf("unexpected entry type for %q: want %q, got %q", entry.Path, wantType, entry.Type)
		}
	}
}

func TestWebAliasGetFileInfoTreatsHTMLAsDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>docs</body></html>"))
	}))
	defer server.Close()

	service := newWebAliasService()
	info, err := service.getFileInfo(server.URL + "/docs")
	if err != nil {
		t.Fatalf("getFileInfo returned error: %v", err)
	}

	if info.Type != contracts.FileTypeDirectory {
		t.Fatalf("expected directory type, got %q", info.Type)
	}
	if info.Path != server.URL+"/docs" {
		t.Fatalf("unexpected info path: %q", info.Path)
	}
}

func TestWebAliasListDirectoryUsesCachedFileSizes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><body><a href="/assets/app.js">App</a></body></html>`))
		case "/assets/app.js":
			payload := []byte("console.log('hello');")
			w.Header().Set("Content-Type", "application/javascript")
			w.Header().Set("Content-Length", "21")
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newWebAliasService()

	listing, err := service.listDirectory(server.URL + "/index.html")
	if err != nil {
		t.Fatalf("initial listDirectory returned error: %v", err)
	}
	if len(listing.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(listing.Entries))
	}
	if listing.Entries[0].Size != 0 {
		t.Fatalf("expected uncached size to be 0, got %d", listing.Entries[0].Size)
	}

	if _, err := service.readFile(server.URL+"/assets/app.js", "utf8"); err != nil {
		t.Fatalf("readFile returned error: %v", err)
	}

	listing, err = service.listDirectory(server.URL + "/index.html")
	if err != nil {
		t.Fatalf("second listDirectory returned error: %v", err)
	}
	if listing.Entries[0].Size != 21 {
		t.Fatalf("expected cached size 21, got %d", listing.Entries[0].Size)
	}
}
