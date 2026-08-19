package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	htmlpkg "html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wails-app/internal/pathref"
	vfslib "wails-app/vfs"
	"wails-app/vfs/archive/cache"
)

const streamPath = "/wails-stream"

const defaultPreviewWindowSeconds = 90.0

const defaultThumbnailSize = 256

const defaultVideoFramePreviewSeconds = 1.0

const remoteHTMLRewriteLimit = 8 * 1024 * 1024

var remoteStreamClient = &http.Client{}

// NewUnifiedHandler creates a server that handles:
// - Video streaming via /wails-stream (legacy)
// - Archive entry streaming (legacy)
// - Range requests for seeking
func NewUnifiedHandler(app *App, archCache *cache.Cache) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle OPTIONS requests for CORS
		if r.Method == http.MethodOptions {
			setCORSHeaders(w)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Route to legacy streaming
		if r.URL.Path == streamPath {
			serveStreamRequest(w, r, app, archCache)
			return
		}

		// Other paths return 404
		http.NotFound(w, r)
	})
}

func serveStreamRequest(w http.ResponseWriter, r *http.Request, app *App, archCache *cache.Cache) {
	q := r.URL.Query()
	transcodeFormat := strings.ToLower(q.Get("transcode"))
	playbackMode := strings.ToLower(q.Get("mode"))
	startSeconds := parseStartSeconds(q.Get("start"))
	windowSeconds := parseWindowSeconds(q.Get("window"))

	// ── Converted disk image ───────────────────────────────────────────────
	path := q.Get("path")
	if path != "" && q.Get("segmentManifest") == "1" {
		serveSegmentManifest(w, r, path)
		return
	}
	if segmentKey := q.Get("segmentKey"); segmentKey != "" {
		if q.Get("segmentManifest") == "1" {
			serveSegmentManifestByKey(w, r, segmentKey)
			return
		}
		if q.Get("segmentInit") == "1" {
			serveSegmentFile(w, r, segmentKey, "init.mp4")
			return
		}
		if rawIndex := q.Get("segmentIndex"); rawIndex != "" {
			segmentIndex, err := strconv.Atoi(rawIndex)
			if err != nil || segmentIndex < 0 {
				http.Error(w, "invalid segment index", http.StatusBadRequest)
				return
			}
			serveSegmentFile(w, r, segmentKey, segmentedVideoFileName(segmentIndex))
			return
		}
	}
	convertFormat := q.Get("convert")
	thumbnailFormat := q.Get("thumb")
	thumbnailWidth := parseThumbnailDimension(q.Get("thumbWidth"))
	thumbnailHeight := parseThumbnailDimension(q.Get("thumbHeight"))
	frameFormat := strings.ToLower(q.Get("frame"))
	frameSeconds := parseStartSeconds(q.Get("frameTime"))
	if path != "" && thumbnailFormat != "" {
		serveThumbnailPath(w, r, app, path, thumbnailFormat, thumbnailWidth, thumbnailHeight)
		return
	}
	if path != "" && frameFormat != "" {
		serveVideoFramePath(w, r, path, frameFormat, frameSeconds)
		return
	}
	if path != "" && convertFormat != "" {
		serveConvertedPath(w, r, app, path, convertFormat)
		return
	}
	if path != "" && transcodeFormat != "" {
		serveTranscodedPath(w, r, path, transcodeFormat, playbackMode, startSeconds, windowSeconds)
		return
	}

	// ── Archive entry ─────────────────────────────────────────────────────
	fsPath := q.Get("fsPath")
	entry := q.Get("entry")
	if fsPath != "" && entry != "" && transcodeFormat != "" {
		serveTranscodedArchiveEntry(w, r, archCache, fsPath, entry, q.Get("nested"), transcodeFormat, playbackMode, startSeconds)
		return
	}
	if fsPath != "" && entry != "" && thumbnailFormat != "" {
		serveThumbnailArchiveEntry(w, r, app, archCache, fsPath, entry, q.Get("nested"), thumbnailFormat, thumbnailWidth, thumbnailHeight)
		return
	}
	if fsPath != "" && entry != "" && frameFormat != "" {
		serveVideoFrameArchiveEntry(w, r, archCache, fsPath, entry, q.Get("nested"), frameFormat, frameSeconds)
		return
	}
	if fsPath != "" && entry != "" {
		serveArchiveEntry(w, r, archCache, fsPath, entry, q.Get("nested"))
		return
	}

	// ── Disk / VFS path ───────────────────────────────────────────────────
	path = q.Get("path")
	if path == "" {
		http.Error(w, "missing path parameter", http.StatusBadRequest)
		return
	}
	servePath(w, r, app, path)
}

func serveConvertedPath(w http.ResponseWriter, r *http.Request, app *App, publicPath string, targetFormat string) {
	if app == nil || app.State == nil || app.State.Converter == nil {
		http.Error(w, "converter unavailable", http.StatusServiceUnavailable)
		return
	}

	publicPath = pathref.Normalize(publicPath)
	nativePath := pathref.ToNativePath(publicPath)
	if strings.Contains(nativePath, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	converted, err := app.State.Converter.ConvertFormat(nativePath, targetFormat)
	if err != nil {
		http.Error(w, "conversion failed: "+err.Error(), http.StatusUnsupportedMediaType)
		return
	}

	setCORSHeaders(w)
	setContentType(w, "preview."+targetFormat)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "preview."+targetFormat, time.Time{}, bytes.NewReader(converted))
}

func serveThumbnailPath(w http.ResponseWriter, r *http.Request, app *App, publicPath string, targetFormat string, maxWidth int, maxHeight int) {
	if app == nil || app.State == nil || app.State.Converter == nil {
		http.Error(w, "converter unavailable", http.StatusServiceUnavailable)
		return
	}

	publicPath = pathref.Normalize(publicPath)
	nativePath := pathref.ToNativePath(publicPath)
	if strings.Contains(nativePath, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	thumbnail, err := app.State.Converter.CreateThumbnail(nativePath, targetFormat, maxWidth, maxHeight)
	if err != nil {
		http.Error(w, "thumbnail failed: "+err.Error(), http.StatusUnsupportedMediaType)
		return
	}

	setCORSHeaders(w)
	setContentType(w, "thumbnail."+targetFormat)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "thumbnail."+targetFormat, time.Time{}, bytes.NewReader(thumbnail))
}

func serveThumbnailArchiveEntry(w http.ResponseWriter, r *http.Request, app *App, archCache *cache.Cache, fsPath string, entryPath string, nestedRaw string, targetFormat string, maxWidth int, maxHeight int) {
	if app == nil || app.State == nil || app.State.Converter == nil {
		http.Error(w, "converter unavailable", http.StatusServiceUnavailable)
		return
	}

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

	entryFile, err := arch.Open(ctx, entryPath)
	if err != nil {
		http.Error(w, "cannot open entry: "+err.Error(), http.StatusNotFound)
		return
	}
	defer entryFile.Close()

	tempPattern := "wails-thumb-*" + filepath.Ext(entryPath)
	tempFile, err := os.CreateTemp("", tempPattern)
	if err != nil {
		http.Error(w, "failed to prepare thumbnail input", http.StatusInternalServerError)
		return
	}
	tempPath := tempFile.Name()
	defer func() {
		tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	if _, err := io.Copy(tempFile, entryFile); err != nil {
		http.Error(w, "failed to read archive entry", http.StatusInternalServerError)
		return
	}
	if err := tempFile.Close(); err != nil {
		http.Error(w, "failed to finalize thumbnail input", http.StatusInternalServerError)
		return
	}

	thumbnail, err := app.State.Converter.CreateThumbnail(tempPath, targetFormat, maxWidth, maxHeight)
	if err != nil {
		http.Error(w, "thumbnail failed: "+err.Error(), http.StatusUnsupportedMediaType)
		return
	}

	setCORSHeaders(w)
	setContentType(w, "thumbnail."+targetFormat)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "thumbnail."+targetFormat, time.Time{}, bytes.NewReader(thumbnail))
}

func serveVideoFramePath(w http.ResponseWriter, r *http.Request, publicPath string, targetFormat string, frameSeconds float64) {
	publicPath = pathref.Normalize(publicPath)
	nativePath := pathref.ToNativePath(publicPath)
	if strings.Contains(nativePath, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	frame, err := extractVideoFrame(r.Context(), nativePath, targetFormat, frameSeconds)
	if err != nil {
		http.Error(w, "failed to extract video frame: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setCORSHeaders(w)
	setContentType(w, "frame."+targetFormat)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "frame."+targetFormat, time.Time{}, bytes.NewReader(frame))
}

func serveVideoFrameArchiveEntry(w http.ResponseWriter, r *http.Request, archCache *cache.Cache, fsPath string, entryPath string, nestedRaw string, targetFormat string, frameSeconds float64) {
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

	entryFile, err := arch.Open(ctx, entryPath)
	if err != nil {
		http.Error(w, "cannot open entry: "+err.Error(), http.StatusNotFound)
		return
	}
	defer entryFile.Close()

	tempPattern := "wails-vframe-*" + filepath.Ext(entryPath)
	tempFile, err := os.CreateTemp("", tempPattern)
	if err != nil {
		http.Error(w, "failed to prepare frame input", http.StatusInternalServerError)
		return
	}
	tempPath := tempFile.Name()
	defer func() {
		tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	if _, err := io.Copy(tempFile, entryFile); err != nil {
		http.Error(w, "failed to read archive entry", http.StatusInternalServerError)
		return
	}
	if err := tempFile.Close(); err != nil {
		http.Error(w, "failed to finalize frame input", http.StatusInternalServerError)
		return
	}

	frame, err := extractVideoFrame(r.Context(), tempPath, targetFormat, frameSeconds)
	if err != nil {
		http.Error(w, "failed to extract video frame from archive: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setCORSHeaders(w)
	setContentType(w, "frame."+targetFormat)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "frame."+targetFormat, time.Time{}, bytes.NewReader(frame))
}

func serveTranscodedPath(w http.ResponseWriter, r *http.Request, publicPath string, targetFormat string, playbackMode string, startSeconds float64, windowSeconds float64) {
	if strings.ToLower(targetFormat) != "mp4" {
		http.Error(w, "unsupported transcode target", http.StatusBadRequest)
		return
	}

	nativePath := pathref.ToNativePath(pathref.Normalize(publicPath))
	if strings.Contains(nativePath, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if _, err := os.Stat(nativePath); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "cannot open file", http.StatusInternalServerError)
		return
	}

	stdout, err := startAstiavTranscode(r.Context(), nativePath, startSeconds, windowSeconds, normalizePlaybackMode(playbackMode))
	if err != nil {
		http.Error(w, "failed to start transcoder: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer stdout.Close()

	if durationSeconds := probeFileDurationSeconds(nativePath); durationSeconds > 0 {
		w.Header().Set("X-Video-Duration-Seconds", strconv.FormatFloat(durationSeconds, 'f', 3, 64))
	}

	streamTranscodedOutput(w, r, stdout)
}

func serveTranscodedArchiveEntry(w http.ResponseWriter, r *http.Request, archCache *cache.Cache, fsPath, entryPath, nestedRaw string, targetFormat string, playbackMode string, startSeconds float64) {
	if strings.ToLower(targetFormat) != "mp4" {
		http.Error(w, "unsupported transcode target", http.StatusBadRequest)
		return
	}
	if startSeconds > 0 {
		http.Error(w, "seeking is not available for archive video previews", http.StatusBadRequest)
		return
	}

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

	entryFile, err := arch.Open(ctx, entryPath)
	if err != nil {
		http.Error(w, "cannot open entry: "+err.Error(), http.StatusNotFound)
		return
	}
	defer entryFile.Close()

	stdout, err := startAstiavTranscodeReader(r.Context(), entryFile, normalizePlaybackMode(playbackMode))
	if err != nil {
		http.Error(w, "failed to start reader transcoder: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer stdout.Close()

	streamTranscodedOutput(w, r, stdout)
}

func servePath(w http.ResponseWriter, r *http.Request, app *App, publicPath string) {
	publicPath = pathref.Normalize(publicPath)
	if pathref.IsWeb(publicPath) {
		serveRemotePath(w, r, publicPath)
		return
	}
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

	fi, err := vfs.Stat(ctx, internalPath)
	if err != nil || fi.IsDir() {
		http.Error(w, "not a file", http.StatusBadRequest)
		return
	}

	f, err := vfs.Open(ctx, internalPath)
	if err != nil {
		http.Error(w, "cannot open file", http.StatusNotFound)
		return
	}
	defer f.Close()

	setCORSHeaders(w)
	setContentType(w, fi.Name())
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Accept-Ranges", "bytes")
	if _, err := f.Seek(0, io.SeekStart); err == nil {
		http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
		return
	}
	serveNonSeekableVFSFile(w, r, f, fi)
}

func serveNonSeekableVFSFile(w http.ResponseWriter, r *http.Request, f io.Reader, fi vfslib.FileInfo) {
	size := fi.Size()
	if size < 0 {
		size = 0
	}

	rangeHeader := strings.TrimSpace(r.Header.Get("Range"))
	if rangeHeader != "" && size > 0 {
		start, end, ok := parseSingleByteRange(rangeHeader, size)
		if !ok {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}

		if start > 0 {
			if _, err := io.CopyN(io.Discard, f, start); err != nil {
				http.Error(w, "cannot read file", http.StatusInternalServerError)
				return
			}
		}

		length := end - start + 1
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		w.WriteHeader(http.StatusPartialContent)
		if r.Method == http.MethodHead {
			return
		}
		if _, err := io.CopyN(w, f, length); err != nil && !errors.Is(err, io.EOF) {
			return
		}
		return
	}

	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	if _, err := io.Copy(w, f); err != nil {
		return
	}
}

func parseSingleByteRange(header string, size int64) (int64, int64, bool) {
	if size <= 0 {
		return 0, 0, false
	}
	if !strings.HasPrefix(strings.ToLower(header), "bytes=") {
		return 0, 0, false
	}

	spec := strings.TrimSpace(header[len("bytes="):])
	if spec == "" || strings.Contains(spec, ",") {
		return 0, 0, false
	}

	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return 0, 0, false
	}

	startRaw := strings.TrimSpace(spec[:dash])
	endRaw := strings.TrimSpace(spec[dash+1:])

	switch {
	case startRaw == "" && endRaw == "":
		return 0, 0, false
	case startRaw == "":
		suffix, err := strconv.ParseInt(endRaw, 10, 64)
		if err != nil || suffix <= 0 {
			return 0, 0, false
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, true
	default:
		start, err := strconv.ParseInt(startRaw, 10, 64)
		if err != nil || start < 0 || start >= size {
			return 0, 0, false
		}
		if endRaw == "" {
			return start, size - 1, true
		}
		end, err := strconv.ParseInt(endRaw, 10, 64)
		if err != nil || end < start {
			return 0, 0, false
		}
		if end >= size {
			end = size - 1
		}
		return start, end, true
	}
}

// serveDiskFile resolves the public path (filesystem:// scheme) to a native OS path
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

func serveRemotePath(w http.ResponseWriter, r *http.Request, publicPath string) {
	targetURL, err := normalizeRemoteStreamURL(publicPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, nil)
	if err != nil {
		http.Error(w, "invalid remote request", http.StatusBadRequest)
		return
	}
	req.Header.Set("Accept", firstNonEmpty(r.Header.Get("Accept"), "*/*"))
	req.Header.Set("Accept-Language", firstNonEmpty(r.Header.Get("Accept-Language"), "en"))
	req.Header.Set("User-Agent", firstNonEmpty(r.Header.Get("User-Agent"), "WailsFileManager/1.0"))
	for _, name := range []string{"Range", "If-None-Match", "If-Modified-Since"} {
		if value := strings.TrimSpace(r.Header.Get(name)); value != "" {
			req.Header.Set(name, value)
		}
	}

	resp, err := remoteStreamClient.Do(req)
	if err != nil {
		http.Error(w, "cannot open remote resource", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if shouldRewriteRemoteHTML(resp, r.Method) {
		serveRewrittenRemoteHTML(w, r, resp)
		return
	}

	writeRemoteResponse(w, r, resp, false, nil)
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

func shouldRewriteRemoteHTML(resp *http.Response, method string) bool {
	if resp == nil || strings.EqualFold(method, http.MethodHead) {
		return false
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return false
	}
	if resp.ContentLength > remoteHTMLRewriteLimit {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		mediaType = strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	}
	switch strings.ToLower(mediaType) {
	case "text/html", "application/xhtml+xml":
		return true
	default:
		return false
	}
}

func serveRewrittenRemoteHTML(w http.ResponseWriter, r *http.Request, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "cannot read remote document", http.StatusBadGateway)
		return
	}

	baseHref := ""
	if resp.Request != nil && resp.Request.URL != nil {
		baseHref = resp.Request.URL.String()
		if resp.Request.URL.Path == "" {
			baseHref += "/"
		}
	}
	rewritten := injectHTMLBaseTag(body, baseHref)
	writeRemoteResponse(w, r, resp, true, rewritten)
}

func writeRemoteResponse(w http.ResponseWriter, r *http.Request, resp *http.Response, rewritten bool, body []byte) {
	setCORSHeaders(w)
	copyRemoteHeaders(w.Header(), resp.Header, rewritten)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	if strings.EqualFold(r.Method, http.MethodHead) {
		return
	}
	if rewritten {
		_, _ = w.Write(body)
		return
	}
	_, _ = io.Copy(w, resp.Body)
}

func copyRemoteHeaders(dst http.Header, src http.Header, rewritten bool) {
	skip := map[string]struct{}{
		"connection":                          {},
		"keep-alive":                          {},
		"proxy-authenticate":                  {},
		"proxy-authorization":                 {},
		"te":                                  {},
		"trailer":                             {},
		"transfer-encoding":                   {},
		"upgrade":                             {},
		"x-frame-options":                     {},
		"content-security-policy":             {},
		"content-security-policy-report-only": {},
		"cross-origin-embedder-policy":        {},
		"cross-origin-opener-policy":          {},
		"cross-origin-resource-policy":        {},
		"alt-svc":                             {},
	}
	if rewritten {
		skip["content-length"] = struct{}{}
		skip["content-encoding"] = struct{}{}
		skip["etag"] = struct{}{}
	}

	for key, values := range src {
		if _, blocked := skip[strings.ToLower(key)]; blocked {
			continue
		}
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func normalizeRemoteStreamURL(publicPath string) (string, error) {
	normalized := pathref.Normalize(publicPath)
	if !pathref.IsWeb(normalized) {
		return "", fmt.Errorf("unsupported remote scheme")
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("invalid remote URL")
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	parsed.Path = strings.ReplaceAll(parsed.Path, "\\", "/")
	return parsed.String(), nil
}

func injectHTMLBaseTag(document []byte, baseHref string) []byte {
	if len(document) == 0 || strings.TrimSpace(baseHref) == "" {
		return document
	}

	lowerDocument := strings.ToLower(string(document))
	if strings.Contains(lowerDocument, "<base ") {
		return document
	}

	baseTag := `<base href="` + htmlpkg.EscapeString(baseHref) + `">`
	if headStart := strings.Index(lowerDocument, "<head"); headStart >= 0 {
		if headEnd := strings.Index(lowerDocument[headStart:], ">"); headEnd >= 0 {
			insertAt := headStart + headEnd + 1
			return append(document[:insertAt], append([]byte(baseTag), document[insertAt:]...)...)
		}
	}
	if htmlStart := strings.Index(lowerDocument, "<html"); htmlStart >= 0 {
		if htmlEnd := strings.Index(lowerDocument[htmlStart:], ">"); htmlEnd >= 0 {
			insertAt := htmlStart + htmlEnd + 1
			headBlock := []byte("<head>" + baseTag + "</head>")
			return append(document[:insertAt], append(headBlock, document[insertAt:]...)...)
		}
	}
	return append([]byte("<head>"+baseTag+"</head>"), document...)
}

func streamTranscodedOutput(w http.ResponseWriter, r *http.Request, stdout io.ReadCloser) {
	firstChunk := make([]byte, 32*1024)
	n, readErr := stdout.Read(firstChunk)
	if n == 0 {
		if r.Context().Err() != nil {
			return
		}

		if readErr != nil && readErr != io.EOF {
			message := readErr.Error()
			http.Error(w, message, http.StatusInternalServerError)
			return
		}

		http.Error(w, "media transcode produced no output", http.StatusInternalServerError)
		return
	}

	setCORSHeaders(w)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "video/mp4")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(firstChunk[:n]); err != nil {
		return
	}

	_, _ = io.Copy(w, stdout)
}