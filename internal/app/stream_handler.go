package app

import (
	"context"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wails-app/internal/pathref"
	vfslib "wails-app/vfs"
	"wails-app/vfs/archive/cache"
)

const streamPath = "/wails-stream"

func NewStreamHandler(app *App, archCache *cache.Cache) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != streamPath {
			http.NotFound(w, r)
			return
		}

		if r.Method == http.MethodOptions {
			setCORSHeaders(w)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		serveStreamRequest(w, r, app, archCache)
	})
}

// NewStreamMiddleware returns an http.Handler that intercepts requests to
// /wails-stream and pipes file content directly to the browser.
//
// Two modes:
//
//  1. Disk file  — GET /wails-stream?path=drive://c/foo/bar.mkv
//     Uses os.Open + http.ServeContent → full range-request support (video seek).
//
//  2. Archive entry — GET /wails-stream?fsPath=drive://c/foo.zip&entry=dir/file.mp4&nested=inner.tar.gz
//     Opens archive via cache, pipes entry with io.Copy.
//     nested is optional comma-separated chain of intermediate archive paths.
//
// All other requests are forwarded to next.
func NewStreamMiddleware(app *App, archCache *cache.Cache, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != streamPath {
			next.ServeHTTP(w, r)
			return
		}
		serveStreamRequest(w, r, app, archCache)
	})
}

func serveStreamRequest(w http.ResponseWriter, r *http.Request, app *App, archCache *cache.Cache) {
	q := r.URL.Query()

	// ── Archive entry ─────────────────────────────────────────────────────
	fsPath := q.Get("fsPath")
	entry := q.Get("entry")
	if fsPath != "" && entry != "" {
		serveArchiveEntry(w, r, archCache, fsPath, entry, q.Get("nested"))
		return
	}

	// ── Disk / VFS path ───────────────────────────────────────────────────
	path := q.Get("path")
	if path == "" {
		http.Error(w, "missing path parameter", http.StatusBadRequest)
		return
	}
	servePath(w, r, app, path)
}

func servePath(w http.ResponseWriter, r *http.Request, app *App, publicPath string) {
	publicPath = pathref.Normalize(publicPath)
	if shouldServeViaVFS(app, publicPath) {
		serveVFSFile(w, r, app.State.vfs, publicPath)
		return
	}
	serveDiskFile(w, r, publicPath)
}

func shouldServeViaVFS(app *App, publicPath string) bool {
	if app == nil || app.State == nil || app.State.vfs == nil {
		return false
	}
	if pathref.IsAlias(publicPath) {
		return app.State.vfs.IsMounted(pathref.ToVFS(publicPath))
	}
	return app.State.vfs.IsMounted(publicPath)
}

func serveVFSFile(w http.ResponseWriter, r *http.Request, vfs *vfslib.Unified, publicPath string) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	internalPath := publicPath
	if pathref.IsAlias(publicPath) {
		internalPath = pathref.ToVFS(publicPath)
	}

	f, err := vfs.Open(ctx, internalPath)
	if err != nil {
		http.Error(w, "cannot open file", http.StatusNotFound)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		http.Error(w, "not a file", http.StatusBadRequest)
		return
	}

	setCORSHeaders(w)
	setContentType(w, fi.Name())
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

// serveDiskFile resolves the public path (drive:// scheme) to a native OS path
// and lets http.ServeContent handle range requests.
func serveDiskFile(w http.ResponseWriter, r *http.Request, publicPath string) {
	nativePath := pathref.ToNativePath(publicPath)
	// Reject any path traversal attempts
	if strings.Contains(nativePath, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	f, err := os.Open(nativePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "not found", http.StatusNotFound)
		} else {
			http.Error(w, "cannot open file", http.StatusInternalServerError)
		}
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		http.Error(w, "not a file", http.StatusBadRequest)
		return
	}

	setCORSHeaders(w)
	setContentType(w, nativePath)
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

// serveArchiveEntry opens the archive (or uses the LRU cache), then pipes the
// requested entry. Because archive entries are not seekable, range requests are
// not supported — but the full content is piped without buffering in JS.
func serveArchiveEntry(w http.ResponseWriter, r *http.Request, archCache *cache.Cache, fsPath, entryPath, nestedRaw string) {
	nativeFsPath := pathref.ToNativePath(fsPath)
	if strings.Contains(nativeFsPath, "..") || strings.Contains(entryPath, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	var nestedChain []string
	if nestedRaw != "" {
		nestedChain = strings.Split(nestedRaw, ",")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	arch, err := archCache.Get(ctx, nativeFsPath, nestedChain, "")
	if err != nil {
		http.Error(w, "cannot open archive: "+err.Error(), http.StatusInternalServerError)
		return
	}

	f, err := arch.Open(ctx, entryPath)
	if err != nil {
		http.Error(w, "cannot open entry: "+err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()

	setCORSHeaders(w)
	setContentType(w, entryPath)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, f) //nolint:errcheck — client disconnect is not an error
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
}

func setContentType(w http.ResponseWriter, name string) {
	ext := strings.ToLower(filepath.Ext(name))
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		ct = extMIME(ext)
	}
	if ct != "" {
		w.Header().Set("Content-Type", ct)
	}
}

// extMIME covers formats not in the stdlib mime database.
func extMIME(ext string) string {
	m := map[string]string{
		".mkv":  "video/x-matroska",
		".avi":  "video/x-msvideo",
		".flv":  "video/x-flv",
		".wmv":  "video/x-ms-wmv",
		".ts":   "video/mp2t",
		".mts":  "video/mp2t",
		".m3u8": "application/x-mpegURL",
		".mxf":  "application/mxf",
		".webp": "image/webp",
		".avif": "image/avif",
		".heic": "image/heic",
		".heif": "image/heif",
	}
	return m[ext]
}

// StreamURLForFile builds the /wails-stream URL for a disk path.
// Called from the frontend via Wails binding.
func StreamURLForFile(baseURL, publicPath string) string {
	return joinStreamBaseURL(baseURL) + streamPath + "?path=" + urlEncode(publicPath)
}

// StreamURLForEntry builds the /wails-stream URL for an archive entry.
func StreamURLForEntry(baseURL, fsPath string, nestedChain []string, entryPath string) string {
	u := joinStreamBaseURL(baseURL) + streamPath + "?fsPath=" + urlEncode(fsPath) + "&entry=" + urlEncode(entryPath)
	if len(nestedChain) > 0 {
		u += "&nested=" + urlEncode(strings.Join(nestedChain, ","))
	}
	return u
}

func joinStreamBaseURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/")
}

func urlEncode(s string) string {
	// Manual percent-encoding for path characters. We only need to encode the
	// characters that would break query-string parsing.
	var b strings.Builder
	for _, c := range []byte(s) {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '-', c == '_', c == '.', c == '~',
			c == '/', c == ':', c == '@':
			b.WriteByte(c)
		default:
			b.WriteString("%" + hexByte(c))
		}
	}
	return b.String()
}

func hexByte(b byte) string {
	const hex = "0123456789ABCDEF"
	return string([]byte{hex[b>>4], hex[b&0xf]})
}

// GetStreamURL is a Wails-bound method that returns the stream URL for any path.
// Exposed so the frontend can call App.GetStreamURL(path) to get a ready-to-use src.
func (a *App) GetStreamURL(publicPath string) string {
	return StreamURLForFile(a.StreamBaseURL(), publicPath)
}

// GetArchiveStreamURL is a Wails-bound method for archive entries.
func (a *App) GetArchiveStreamURL(fsPath string, nestedChain []string, entryPath string) string {
	return StreamURLForEntry(a.StreamBaseURL(), fsPath, nestedChain, entryPath)
}
