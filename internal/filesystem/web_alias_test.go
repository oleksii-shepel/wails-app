package filesystem

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	contracts "wails-app/internal/contracts"
	"wails-app/internal/webresourcecache"
)

func TestWebAliasListDirectoryDiscoversLinkedEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head><title>Index of /</title></head><body><h1>Index of /</h1><a href="/docs/page.html">Docs</a><a href="/assets/app.js">App</a><a href="/home?hl=en-US">Home</a><script src="/assets/app.js"></script></body></html>`))
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

func TestWebAliasListDirectoryIgnoresRegularWebsitePages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Google News</title></head><body><nav><a href="/home?hl=en-US&gl=US">Home</a><a href="/topics/CAAqJggKIiBDQkFTRWdvSUwyMHZNRFZxYUdjU0FtVnVLQUFQAQ">Topic</a></nav></body></html>`))
	}))
	defer server.Close()

	service := newWebAliasService()
	listing, err := service.listDirectory(server.URL)
	if err != nil {
		t.Fatalf("listDirectory returned error: %v", err)
	}

	if len(listing.Entries) != 0 {
		t.Fatalf("expected 0 entries for non-directory page, got %d", len(listing.Entries))
	}
}

func TestWebAliasListDirectoryUsesCachedBrowserResourcesForRegularPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Google News</title></head><body><div>news</div></body></html>`))
	}))
	defer server.Close()

	webresourcecache.Store(server.URL, []webresourcecache.Resource{
		{URL: server.URL + "/static/app.js", Size: 1024, InitiatorType: "script", MimeType: "application/javascript"},
		{URL: server.URL + "/_/scs/mail-static/_/js/k=abc", Size: 4096, InitiatorType: "script", MimeType: "application/javascript"},
		{URL: server.URL + "/home?hl=en-US", Size: 2048, InitiatorType: "fetch"},
	})

	service := newWebAliasService()
	listing, err := service.listDirectory(server.URL)
	if err != nil {
		t.Fatalf("listDirectory returned error: %v", err)
	}

	if len(listing.Entries) != 2 {
		t.Fatalf("expected 2 cached resource entries, got %d", len(listing.Entries))
	}
	seen := map[string]int64{}
	for _, entry := range listing.Entries {
		seen[entry.Path] = entry.Size
	}
	if seen[server.URL+"/static/app.js"] != 1024 {
		t.Fatalf("unexpected cached resource size for app.js: %d", seen[server.URL+"/static/app.js"])
	}
	if seen[server.URL+"/_/scs/mail-static/_/js/k=abc"] != 4096 {
		t.Fatalf("unexpected cached resource size for extensionless resource: %d", seen[server.URL+"/_/scs/mail-static/_/js/k=abc"])
	}
}

func TestWebAliasListDirectoryShortCircuitsRegularWebsitePages(t *testing.T) {
	var fullRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Header.Get("Range") != "" {
			_, _ = w.Write([]byte(`<!doctype html><html><head><title>Gmail</title></head><body>mail</body></html>`))
			return
		}

		fullRequests.Add(1)
		time.Sleep(250 * time.Millisecond)
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Gmail</title></head><body>mail</body></html>`))
	}))
	defer server.Close()

	service := newWebAliasService()
	started := time.Now()
	listing, err := service.listDirectory(server.URL)
	if err != nil {
		t.Fatalf("listDirectory returned error: %v", err)
	}

	if len(listing.Entries) != 0 {
		t.Fatalf("expected 0 entries for non-directory page, got %d", len(listing.Entries))
	}
	if fullRequests.Load() != 0 {
		t.Fatalf("expected no full GET requests, got %d", fullRequests.Load())
	}
	if elapsed := time.Since(started); elapsed >= 200*time.Millisecond {
		t.Fatalf("expected regular website probe to return quickly, took %v", elapsed)
	}
}

func TestWebAliasGetFileInfoTreatsRegularHTMLAsFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Docs</title></head><body>docs</body></html>`))
	}))
	defer server.Close()

	service := newWebAliasService()
	info, err := service.getFileInfo(server.URL + "/docs")
	if err != nil {
		t.Fatalf("getFileInfo returned error: %v", err)
	}

	if info.Type != contracts.FileTypeFile {
		t.Fatalf("expected file type, got %q", info.Type)
	}
	if info.Path != server.URL+"/docs" {
		t.Fatalf("unexpected info path: %q", info.Path)
	}
}

func TestWebAliasGetFileInfoTreatsDirectoryListingHTMLAsDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Index of /docs</title></head><body><h1>Index of /docs</h1></body></html>`))
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
}

func TestWebAliasGetFileInfoKeepsQueryResourceAsFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", "42")
		_, _ = w.Write([]byte("012345678901234567890123456789012345678901"))
	}))
	defer server.Close()

	service := newWebAliasService()
	path := server.URL + "/10836211492?random=1775580549518"
	service.rememberMetadata(path, webAliasMetadata{size: 42})

	info, err := service.getFileInfo(path)
	if err != nil {
		t.Fatalf("getFileInfo returned error: %v", err)
	}
	if info.Type != contracts.FileTypeFile {
		t.Fatalf("expected query resource to stay file, got %q", info.Type)
	}
	if info.Size != 42 {
		t.Fatalf("unexpected query resource size: %d", info.Size)
	}
}

func TestWebAliasListDirectoryUsesCachedFileSizes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head><title>Index of /</title></head><body><a href="/assets/app.js">App</a></body></html>`))
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

func TestWebAliasListDirectoryResolvesRelativeLinksFromSlashlessDirectoryAlias(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/MPEG-4", "/MPEG-4/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head><title>Index of /MPEG-4</title></head><body><h1>Index of /MPEG-4</h1><a href="MPEG4%20by%20philips.mp4">MPEG4 by philips.mp4</a></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := newWebAliasService()
	listing, err := service.listDirectory(server.URL + "/MPEG-4")
	if err != nil {
		t.Fatalf("listDirectory returned error: %v", err)
	}

	if len(listing.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(listing.Entries))
	}
	if listing.Entries[0].Path != server.URL+"/MPEG-4/MPEG4%20by%20philips.mp4" {
		t.Fatalf("unexpected entry path: %q", listing.Entries[0].Path)
	}
	if listing.Entries[0].Name != "MPEG4 by philips.mp4" {
		t.Fatalf("unexpected entry name: %q", listing.Entries[0].Name)
	}
}
