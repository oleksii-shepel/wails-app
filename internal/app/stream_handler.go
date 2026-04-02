package app

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"wails-app/internal/pathref"
	vfslib "wails-app/vfs"
	"wails-app/vfs/archive/cache"
)

const streamPath = "/wails-stream"
const defaultPreviewWindowSeconds = 90.0
const defaultThumbnailSize = 256
const defaultVideoFramePreviewSeconds = 1.0
const segmentedPreviewSegmentSeconds = 2.0
const segmentedVideoCacheTTL = 5 * time.Minute
const segmentedVideoMaxResidentSegments = 24

type VideoPlaybackMode string

const (
	VideoPlaybackModeDirect    VideoPlaybackMode = "direct"
	VideoPlaybackModeSegmented VideoPlaybackMode = "segmented"
	VideoPlaybackModeRemux     VideoPlaybackMode = "remux"
	VideoPlaybackModeTranscode VideoPlaybackMode = "transcode"
)

type VideoPlaybackPlan struct {
	Mode            VideoPlaybackMode `json:"mode"`
	Format          string            `json:"format"`
	VideoCodec      string            `json:"videoCodec,omitempty"`
	AudioCodec      string            `json:"audioCodec,omitempty"`
	DurationSeconds float64           `json:"durationSeconds"`
	ZeroDelay       bool              `json:"zeroDelay"`
	Seekable        bool              `json:"seekable"`
	UseNativePlayer bool              `json:"useNativePlayer"`
	URL             string            `json:"url"`
}

type segmentedVideoManifest struct {
	Mode            string                       `json:"mode"`
	Format          string                       `json:"format"`
	DurationSeconds float64                      `json:"durationSeconds"`
	SegmentDuration float64                      `json:"segmentDuration"`
	TotalSegments   int                          `json:"totalSegments"`
	InitSegmentURL  string                       `json:"initSegmentUrl"`
	Segments        []segmentedVideoManifestItem `json:"segments"`
}

type segmentedVideoManifestItem struct {
	Index           int     `json:"index"`
	StartTime       float64 `json:"startTime"`
	DurationSeconds float64 `json:"durationSeconds"`
	URL             string  `json:"url"`
}

type segmentedVideoCacheEntry struct {
	CacheKey        string  `json:"cacheKey"`
	Format          string  `json:"format"`
	DurationSeconds float64 `json:"durationSeconds"`
	SegmentDuration float64 `json:"segmentDuration"`
	TotalSegments   int     `json:"totalSegments"`
}

type segmentedVideoPlaybackCacheEntry struct {
	mu          sync.Mutex
	manifest    segmentedVideoCacheEntry
	nativePath  string
	initSegment []byte
	segments    map[int][]byte
	segmentSeen map[int]time.Time
	inflight    map[int]*segmentedVideoSegmentBuild
	createdAt   time.Time
	lastAccess  time.Time
	cleanup     func() // called when evicted; used for temp file cleanup
}

type segmentedVideoPlaybackBuild struct {
	done    chan struct{}
	entry   *segmentedVideoPlaybackCacheEntry
	err     error
	cancel  context.CancelFunc
	waiters int
}

type segmentedVideoSegmentBuild struct {
	done    chan struct{}
	init    []byte
	segment []byte
	err     error
	cancel  context.CancelFunc
	waiters int
}

type segmentedVideoPlaybackCache struct {
	mu       sync.Mutex
	entries  map[string]*segmentedVideoPlaybackCacheEntry
	inflight map[string]*segmentedVideoPlaybackBuild
}

var segmentedPlaybackCache = newSegmentedVideoPlaybackCache()

type mediaProbeOutput struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
	} `json:"streams"`
}

var directPreviewExtensions = map[string]bool{
	".mp4":  true,
	".m4v":  true,
	".mkv":  true,
	".webm": true,
	".ogv":  true,
	".mxf":  true,
}

var remuxableVideoCodecs = map[string]bool{
	"h264": true,
	"hevc": true,
	"h265": true,
	"av1":  true,
}

var remuxableAudioCodecs = map[string]bool{
	"":    true,
	"aac": true,
}

var transcodePreferredVideoCodecs = map[string]bool{
	"mpeg2video": true,
	"vc1":        true,
	"mjpeg":      true,
	"mpeg4":      true,
	"msmpeg4v3":  true,
	"wmv3":       true,
	"vp6":        true,
	"vp6f":       true,
}

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
		serveTranscodedArchiveEntry(w, r, archCache, fsPath, entry, q.Get("nested"), transcodeFormat, playbackMode, startSeconds, windowSeconds)
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
		handleTranscodeStartError(w, err)
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
		handleTranscodeStartError(w, err)
		return
	}

	setCORSHeaders(w)
	setContentType(w, "frame."+targetFormat)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "frame."+targetFormat, time.Time{}, bytes.NewReader(frame))
}

func serveSegmentManifest(w http.ResponseWriter, r *http.Request, publicPath string) {
	entry, err := ensureSegmentedVideoCacheEntry(r.Context(), publicPath)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	manifest := buildSegmentedVideoManifest(requestBaseURL(r), entry)
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(manifest)
}

func serveSegmentManifestByKey(w http.ResponseWriter, r *http.Request, cacheKey string) {
	entry, ok := segmentedPlaybackCache.Get(cacheKey)
	if !ok {
		http.Error(w, "segment cache entry not found or expired", http.StatusNotFound)
		return
	}
	manifest := buildSegmentedVideoManifest(requestBaseURL(r), entry.manifest)
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(manifest)
}

func serveSegmentFile(w http.ResponseWriter, r *http.Request, cacheKey string, fileName string) {
	content, modTime, ok, err := segmentedPlaybackCache.File(r.Context(), cacheKey, fileName)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	setCORSHeaders(w)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeContent(w, r, fileName, modTime, bytes.NewReader(content))
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

	output, err := buildGStreamerFragmentedMP4(r.Context(), nativePath, startSeconds, windowSeconds, normalizePlaybackMode(playbackMode))
	if err != nil {
		handleTranscodeStartError(w, err)
		return
	}

	if durationSeconds := probeFileDurationSeconds(nativePath); durationSeconds > 0 {
		w.Header().Set("X-Video-Duration-Seconds", strconv.FormatFloat(durationSeconds, 'f', 3, 64))
	}

	setCORSHeaders(w)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeContent(w, r, filepath.Base(nativePath)+".mp4", time.Time{}, bytes.NewReader(output))
}

func serveTranscodedArchiveEntry(w http.ResponseWriter, r *http.Request, archCache *cache.Cache, fsPath, entryPath, nestedRaw string, targetFormat string, playbackMode string, startSeconds float64, windowSeconds float64) {
	if strings.ToLower(targetFormat) != "mp4" {
		http.Error(w, "unsupported transcode target", http.StatusBadRequest)
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

	tempPattern := "wails-transcode-*" + filepath.Ext(entryPath)
	tempFile, err := os.CreateTemp("", tempPattern)
	if err != nil {
		http.Error(w, "failed to prepare transcode input", http.StatusInternalServerError)
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
		http.Error(w, "failed to finalize transcode input", http.StatusInternalServerError)
		return
	}

	output, err := buildGStreamerFragmentedMP4(r.Context(), tempPath, startSeconds, windowSeconds, normalizePlaybackMode(playbackMode))
	if err != nil {
		handleTranscodeStartError(w, err)
		return
	}

	if durationSeconds := probeFileDurationSeconds(tempPath); durationSeconds > 0 {
		w.Header().Set("X-Video-Duration-Seconds", strconv.FormatFloat(durationSeconds, 'f', 3, 64))
	}

	setCORSHeaders(w)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeContent(w, r, filepath.Base(entryPath)+".mp4", time.Time{}, bytes.NewReader(output))
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

func extractVideoFrame(ctx context.Context, inputPath string, targetFormat string, frameSeconds float64) ([]byte, error) {
	return extractVideoFrameWithGStreamer(ctx, inputPath, targetFormat, frameSeconds)
}

func handleTranscodeStartError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "gstreamer media pipeline failed"
	}
	if strings.Contains(strings.ToLower(message), "requires cgo") {
		status = http.StatusServiceUnavailable
	}
	http.Error(w, message, status)
}

func parseStartSeconds(raw string) float64 {
	if raw == "" {
		return 0
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return seconds
}

func parseWindowSeconds(raw string) float64 {
	if raw == "" {
		return 0
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || seconds <= 0 {
		return 0
	}
	return seconds
}

func parseThumbnailDimension(raw string) int {
	if raw == "" {
		return defaultThumbnailSize
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultThumbnailSize
	}
	if value > 2048 {
		return 2048
	}
	return value
}

func normalizePlaybackMode(raw string) VideoPlaybackMode {
	if strings.EqualFold(raw, string(VideoPlaybackModeSegmented)) {
		return VideoPlaybackModeSegmented
	}
	if strings.EqualFold(raw, string(VideoPlaybackModeRemux)) {
		return VideoPlaybackModeRemux
	}
	return VideoPlaybackModeTranscode
}

func requestBaseURL(r *http.Request) string {
	if r == nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func segmentedVideoFileName(index int) string {
	return fmt.Sprintf("segment_%05d.m4s", index+1)
}

func segmentManifestURLForFile(baseURL, publicPath string) string {
	return joinStreamBaseURL(baseURL) + streamPath + "?path=" + urlEncode(publicPath) + "&segmentManifest=1"
}

func segmentManifestURLForKey(baseURL, cacheKey string) string {
	return joinStreamBaseURL(baseURL) + streamPath + "?segmentKey=" + urlEncode(cacheKey) + "&segmentManifest=1"
}

func segmentInitURL(baseURL, cacheKey string) string {
	return joinStreamBaseURL(baseURL) + streamPath + "?segmentKey=" + urlEncode(cacheKey) + "&segmentInit=1"
}

func segmentFileURL(baseURL, cacheKey string, index int) string {
	return joinStreamBaseURL(baseURL) + streamPath + "?segmentKey=" + urlEncode(cacheKey) + "&segmentIndex=" + urlEncode(strconv.Itoa(index))
}

func buildSegmentedVideoManifest(baseURL string, entry segmentedVideoCacheEntry) segmentedVideoManifest {
	segments := make([]segmentedVideoManifestItem, 0, entry.TotalSegments)
	for index := 0; index < entry.TotalSegments; index++ {
		startTime := float64(index) * entry.SegmentDuration
		segmentDuration := entry.SegmentDuration
		if remaining := entry.DurationSeconds - startTime; remaining < segmentDuration {
			segmentDuration = remaining
		}
		if segmentDuration < 0 {
			segmentDuration = 0
		}
		segments = append(segments, segmentedVideoManifestItem{
			Index:           index,
			StartTime:       startTime,
			DurationSeconds: segmentDuration,
			URL:             segmentFileURL(baseURL, entry.CacheKey, index),
		})
	}

	return segmentedVideoManifest{
		Mode:            string(VideoPlaybackModeSegmented),
		Format:          entry.Format,
		DurationSeconds: entry.DurationSeconds,
		SegmentDuration: entry.SegmentDuration,
		TotalSegments:   entry.TotalSegments,
		InitSegmentURL:  segmentInitURL(baseURL, entry.CacheKey),
		Segments:        segments,
	}
}

func ensureSegmentedVideoCacheEntry(ctx context.Context, publicPath string) (segmentedVideoCacheEntry, error) {
	publicPath = pathref.Normalize(publicPath)
	nativePath := pathref.ToNativePath(publicPath)
	if strings.Contains(nativePath, "..") {
		return segmentedVideoCacheEntry{}, fmt.Errorf("invalid path")
	}

	fi, err := os.Stat(nativePath)
	if err != nil {
		return segmentedVideoCacheEntry{}, err
	}

	cacheKey := segmentedVideoCacheKey(nativePath, fi)
	entry, err := segmentedPlaybackCache.Ensure(ctx, cacheKey, func(buildCtx context.Context) (*segmentedVideoPlaybackCacheEntry, error) {
		return buildSegmentedVideoCache(buildCtx, nativePath, cacheKey)
	})
	if err != nil {
		return segmentedVideoCacheEntry{}, err
	}
	return entry.manifest, nil
}

func archiveSegmentedVideoCacheKey(fsPath string, nestedChain []string, entryPath string) string {
	parts := make([]string, 0, 2+len(nestedChain))
	parts = append(parts, fsPath)
	parts = append(parts, nestedChain...)
	parts = append(parts, entryPath)
	hash := sha1.Sum([]byte(strings.Join(parts, "|")))
	return "arch-" + fmt.Sprintf("%x", hash)
}

func ensureArchiveSegmentedVideoCacheEntry(ctx context.Context, archCache *cache.Cache, fsPath string, nestedChain []string, entryPath string) (segmentedVideoCacheEntry, error) {
	if archCache == nil {
		return segmentedVideoCacheEntry{}, fmt.Errorf("archive cache unavailable")
	}
	cacheKey := archiveSegmentedVideoCacheKey(fsPath, nestedChain, entryPath)
	entry, err := segmentedPlaybackCache.Ensure(ctx, cacheKey, func(buildCtx context.Context) (*segmentedVideoPlaybackCacheEntry, error) {
		return buildArchiveSegmentedVideoCache(buildCtx, archCache, fsPath, nestedChain, entryPath, cacheKey)
	})
	if err != nil {
		return segmentedVideoCacheEntry{}, err
	}
	return entry.manifest, nil
}

func buildArchiveSegmentedVideoCache(ctx context.Context, archCache *cache.Cache, fsPath string, nestedChain []string, entryPath string, cacheKey string) (*segmentedVideoPlaybackCacheEntry, error) {
	nativeFsPath := pathref.ToNativePath(fsPath)
	if strings.Contains(nativeFsPath, "..") || strings.Contains(entryPath, "..") {
		return nil, fmt.Errorf("invalid path")
	}

	extractCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	arch, err := archCache.Get(extractCtx, nativeFsPath, nestedChain, "")
	if err != nil {
		return nil, fmt.Errorf("cannot open archive: %w", err)
	}

	entryFile, err := arch.Open(extractCtx, entryPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open entry: %w", err)
	}
	defer entryFile.Close()

	ext := filepath.Ext(entryPath)
	tmp, err := os.CreateTemp("", "wails-seg-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, entryFile); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return nil, fmt.Errorf("cannot extract entry: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("cannot finalize temp file: %w", err)
	}

	result, err := buildSegmentedVideoCache(ctx, tmpPath, cacheKey)
	if err != nil {
		os.Remove(tmpPath)
		return nil, err
	}
	result.cleanup = func() { os.Remove(tmpPath) } //nolint:errcheck
	return result, nil
}

func buildSegmentedVideoCache(ctx context.Context, nativePath string, cacheKey string) (*segmentedVideoPlaybackCacheEntry, error) {
	probe, err := probeVideo(nativePath)
	if err != nil {
		return nil, err
	}
	durationSeconds := parseProbeDuration(probe.Format.Duration)
	totalSegments := int(durationSeconds / segmentedPreviewSegmentSeconds)
	if durationSeconds > float64(totalSegments)*segmentedPreviewSegmentSeconds {
		totalSegments++
	}
	if totalSegments <= 0 {
		totalSegments = 1
	}

	manifest := segmentedVideoCacheEntry{
		CacheKey:        cacheKey,
		Format:          "mp4",
		DurationSeconds: durationSeconds,
		SegmentDuration: segmentedPreviewSegmentSeconds,
		TotalSegments:   totalSegments,
	}

	firstSegmentDuration := segmentedPreviewSegmentSeconds
	if durationSeconds > 0 && durationSeconds < firstSegmentDuration {
		firstSegmentDuration = durationSeconds
	}
	if firstSegmentDuration <= 0 {
		firstSegmentDuration = segmentedPreviewSegmentSeconds
	}

	initSegment, firstSegment, err := buildSegmentedVideoFragment(ctx, nativePath, 0, firstSegmentDuration)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &segmentedVideoPlaybackCacheEntry{
		manifest:    manifest,
		nativePath:  nativePath,
		initSegment: initSegment,
		segments: map[int][]byte{
			0: firstSegment,
		},
		segmentSeen: map[int]time.Time{
			0: now,
		},
		inflight:   make(map[int]*segmentedVideoSegmentBuild),
		createdAt:  now,
		lastAccess: now,
	}, nil
}

func buildSegmentedVideoFragment(ctx context.Context, nativePath string, startSeconds float64, durationSeconds float64) ([]byte, []byte, error) {
	return buildGStreamerSegmentedVideoFragment(ctx, nativePath, startSeconds, durationSeconds)
}

func requiresAccurateSegmentSeek(nativePath string) bool {
	switch strings.ToLower(filepath.Ext(nativePath)) {
	case ".avi", ".divx", ".xvid":
		return true
	default:
		return false
	}
}

func splitFragmentedMP4(data []byte) ([]byte, []byte, error) {
	if len(data) < 8 {
		return nil, nil, fmt.Errorf("failed to build segmented preview: fragmented mp4 output too small")
	}

	splitOffset := -1
	for offset := 0; offset+8 <= len(data); {
		boxSize := int(uint32(data[offset])<<24 | uint32(data[offset+1])<<16 | uint32(data[offset+2])<<8 | uint32(data[offset+3]))
		headerSize := 8
		if boxSize == 1 {
			if offset+16 > len(data) {
				return nil, nil, fmt.Errorf("failed to build segmented preview: invalid extended mp4 box")
			}
			largeSize := uint64(data[offset+8])<<56 | uint64(data[offset+9])<<48 | uint64(data[offset+10])<<40 | uint64(data[offset+11])<<32 |
				uint64(data[offset+12])<<24 | uint64(data[offset+13])<<16 | uint64(data[offset+14])<<8 | uint64(data[offset+15])
			if largeSize > uint64(len(data)-offset) || largeSize < 16 {
				return nil, nil, fmt.Errorf("failed to build segmented preview: invalid extended mp4 size")
			}
			boxSize = int(largeSize)
			headerSize = 16
		} else if boxSize == 0 {
			boxSize = len(data) - offset
		}
		if boxSize < headerSize || offset+boxSize > len(data) {
			return nil, nil, fmt.Errorf("failed to build segmented preview: invalid mp4 box size")
		}

		boxType := string(data[offset+4 : offset+8])
		if boxType == "sidx" || boxType == "styp" || boxType == "moof" || boxType == "mdat" {
			splitOffset = offset
			break
		}
		offset += boxSize
	}

	if splitOffset <= 0 || splitOffset >= len(data) {
		return nil, nil, fmt.Errorf("failed to build segmented preview: could not split initialization segment")
	}

	initSegment := append([]byte(nil), data[:splitOffset]...)
	mediaSegment := append([]byte(nil), data[splitOffset:]...)
	if len(mediaSegment) == 0 {
		return nil, nil, fmt.Errorf("failed to build segmented preview: empty media segment")
	}
	return initSegment, mediaSegment, nil
}

func segmentedVideoCacheKey(nativePath string, fi os.FileInfo) string {
	hash := sha1.Sum([]byte(strings.Join([]string{nativePath, strconv.FormatInt(fi.Size(), 10), strconv.FormatInt(fi.ModTime().UnixNano(), 10)}, "|")))
	return fmt.Sprintf("%x", hash)
}

func newSegmentedVideoPlaybackCache() *segmentedVideoPlaybackCache {
	return &segmentedVideoPlaybackCache{
		entries:  make(map[string]*segmentedVideoPlaybackCacheEntry),
		inflight: make(map[string]*segmentedVideoPlaybackBuild),
	}
}

func (c *segmentedVideoPlaybackCache) Ensure(ctx context.Context, cacheKey string, build func(context.Context) (*segmentedVideoPlaybackCacheEntry, error)) (*segmentedVideoPlaybackCacheEntry, error) {
	for {
		c.mu.Lock()
		c.evictExpiredLocked(time.Now())
		if entry, ok := c.entries[cacheKey]; ok {
			entry.lastAccess = time.Now()
			c.mu.Unlock()
			return entry, nil
		}
		pending, ok := c.inflight[cacheKey]
		if !ok {
			buildCtx, cancel := context.WithCancel(context.Background())
			pending = &segmentedVideoPlaybackBuild{done: make(chan struct{}), cancel: cancel}
			c.inflight[cacheKey] = pending
			go c.runPlaybackBuild(cacheKey, pending, buildCtx, build)
		}
		pending.waiters++
		c.mu.Unlock()

		select {
		case <-pending.done:
			if pending.err != nil {
				return nil, pending.err
			}
			if pending.entry != nil {
				return pending.entry, nil
			}
		case <-ctx.Done():
			c.releasePlaybackBuildWaiter(cacheKey, pending)
			return nil, ctx.Err()
		}
	}
}

func (c *segmentedVideoPlaybackCache) runPlaybackBuild(cacheKey string, pending *segmentedVideoPlaybackBuild, buildCtx context.Context, build func(context.Context) (*segmentedVideoPlaybackCacheEntry, error)) {
	entry, err := build(buildCtx)

	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil && entry != nil {
		entry.lastAccess = time.Now()
		c.entries[cacheKey] = entry
	}
	pending.entry = entry
	pending.err = err
	if current, ok := c.inflight[cacheKey]; ok && current == pending {
		delete(c.inflight, cacheKey)
	}
	close(pending.done)
}

func (c *segmentedVideoPlaybackCache) releasePlaybackBuildWaiter(cacheKey string, pending *segmentedVideoPlaybackBuild) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.inflight[cacheKey]
	if !ok || current != pending {
		return
	}
	if pending.waiters > 0 {
		pending.waiters--
	}
	if pending.waiters == 0 && pending.cancel != nil {
		pending.cancel()
	}
}

func (c *segmentedVideoPlaybackCache) File(ctx context.Context, cacheKey string, fileName string) ([]byte, time.Time, bool, error) {
	now := time.Now()
	c.mu.Lock()
	c.evictExpiredLocked(now)
	entry, ok := c.entries[cacheKey]
	if ok {
		entry.lastAccess = now
	}
	c.mu.Unlock()
	if !ok {
		return nil, time.Time{}, false, nil
	}

	content, err := entry.file(ctx, fileName)
	if err != nil {
		return nil, time.Time{}, false, err
	}
	if len(content) == 0 {
		return nil, time.Time{}, false, nil
	}

	c.mu.Lock()
	entry.lastAccess = time.Now()
	c.mu.Unlock()
	return content, entry.createdAt, true, nil
}

func (c *segmentedVideoPlaybackCache) Clear() {
	c.mu.Lock()
	for _, entry := range c.entries {
		if entry.cleanup != nil {
			go entry.cleanup()
		}
	}
	clear(c.entries)
	clear(c.inflight)
	c.mu.Unlock()
}

func (c *segmentedVideoPlaybackCache) Get(cacheKey string) (*segmentedVideoPlaybackCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[cacheKey]
	if ok {
		entry.lastAccess = time.Now()
	}
	return entry, ok
}

func (c *segmentedVideoPlaybackCache) evictExpiredLocked(now time.Time) {
	for cacheKey, entry := range c.entries {
		if now.Sub(entry.lastAccess) > segmentedVideoCacheTTL {
			delete(c.entries, cacheKey)
			if entry.cleanup != nil {
				go entry.cleanup()
			}
		}
	}
}

func cleanupSegmentedVideoPlaybackCache() {
	segmentedPlaybackCache.Clear()
}

func (e *segmentedVideoPlaybackCacheEntry) file(ctx context.Context, fileName string) ([]byte, error) {
	if fileName == "init.mp4" {
		initSegment, _, err := e.ensureSegment(ctx, 0)
		return initSegment, err
	}

	segmentIndex, ok := parseSegmentedVideoFileName(fileName)
	if !ok {
		return nil, nil
	}
	_, segment, err := e.ensureSegment(ctx, segmentIndex)
	return segment, err
}

func (e *segmentedVideoPlaybackCacheEntry) ensureSegment(ctx context.Context, segmentIndex int) ([]byte, []byte, error) {
	if segmentIndex < 0 || segmentIndex >= e.manifest.TotalSegments {
		return nil, nil, nil
	}

	for {
		now := time.Now()
		e.mu.Lock()
		if len(e.initSegment) > 0 {
			if segment, ok := e.segments[segmentIndex]; ok {
				e.segmentSeen[segmentIndex] = now
				e.lastAccess = now
				initSegment := append([]byte(nil), e.initSegment...)
				mediaSegment := append([]byte(nil), segment...)
				e.mu.Unlock()
				return initSegment, mediaSegment, nil
			}
		}
		pending, ok := e.inflight[segmentIndex]
		if !ok {
			buildCtx, cancel := context.WithCancel(context.Background())
			pending = &segmentedVideoSegmentBuild{done: make(chan struct{}), cancel: cancel}
			e.inflight[segmentIndex] = pending

			startSeconds := float64(segmentIndex) * e.manifest.SegmentDuration
			durationSeconds := e.manifest.SegmentDuration
			if remaining := e.manifest.DurationSeconds - startSeconds; remaining > 0 && remaining < durationSeconds {
				durationSeconds = remaining
			}
			if durationSeconds <= 0 {
				durationSeconds = e.manifest.SegmentDuration
			}

			go e.runSegmentBuild(segmentIndex, pending, buildCtx, startSeconds, durationSeconds)
		}
		pending.waiters++
		e.mu.Unlock()

		select {
		case <-pending.done:
			if pending.err != nil {
				return nil, nil, pending.err
			}
			return pending.init, pending.segment, nil
		case <-ctx.Done():
			e.releaseSegmentBuildWaiter(segmentIndex, pending)
			return nil, nil, ctx.Err()
		}
	}
}

func (e *segmentedVideoPlaybackCacheEntry) runSegmentBuild(segmentIndex int, pending *segmentedVideoSegmentBuild, buildCtx context.Context, startSeconds float64, durationSeconds float64) {
	initSegment, mediaSegment, err := buildSegmentedVideoFragment(buildCtx, e.nativePath, startSeconds, durationSeconds)

	e.mu.Lock()
	defer e.mu.Unlock()
	if err == nil {
		now := time.Now()
		if len(e.initSegment) == 0 {
			e.initSegment = initSegment
		}
		e.segments[segmentIndex] = mediaSegment
		e.segmentSeen[segmentIndex] = now
		e.lastAccess = now
		e.pruneSegmentsLocked()
	}
	pending.init = initSegment
	pending.segment = mediaSegment
	pending.err = err
	if current, ok := e.inflight[segmentIndex]; ok && current == pending {
		delete(e.inflight, segmentIndex)
	}
	close(pending.done)
}

func (e *segmentedVideoPlaybackCacheEntry) releaseSegmentBuildWaiter(segmentIndex int, pending *segmentedVideoSegmentBuild) {
	e.mu.Lock()
	defer e.mu.Unlock()
	current, ok := e.inflight[segmentIndex]
	if !ok || current != pending {
		return
	}
	if pending.waiters > 0 {
		pending.waiters--
	}
	if pending.waiters == 0 && pending.cancel != nil {
		pending.cancel()
	}
}

func (e *segmentedVideoPlaybackCacheEntry) pruneSegmentsLocked() {
	if len(e.segments) <= segmentedVideoMaxResidentSegments {
		return
	}

	type residentSegment struct {
		index int
		seen  time.Time
	}
	resident := make([]residentSegment, 0, len(e.segments))
	for index, seenAt := range e.segmentSeen {
		resident = append(resident, residentSegment{index: index, seen: seenAt})
	}
	sort.Slice(resident, func(left int, right int) bool {
		return resident[left].seen.Before(resident[right].seen)
	})
	for len(resident) > segmentedVideoMaxResidentSegments {
		oldest := resident[0]
		delete(e.segments, oldest.index)
		delete(e.segmentSeen, oldest.index)
		resident = resident[1:]
	}
}

func parseSegmentedVideoFileName(fileName string) (int, bool) {
	if !strings.HasPrefix(fileName, "segment_") || !strings.HasSuffix(fileName, ".m4s") {
		return 0, false
	}
	rawIndex := strings.TrimSuffix(strings.TrimPrefix(fileName, "segment_"), ".m4s")
	segmentNumber, err := strconv.Atoi(rawIndex)
	if err != nil || segmentNumber <= 0 {
		return 0, false
	}
	return segmentNumber - 1, true
}

func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range, Accept-Ranges")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Content-Type")
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
		".mkv":   "video/x-matroska",
		".avi":   "video/x-msvideo",
		".divx":  "video/x-msvideo",
		".xvid":  "video/x-msvideo",
		".flv":   "video/x-flv",
		".wmv":   "video/x-ms-wmv",
		".mpg":   "video/mpeg",
		".mpeg":  "video/mpeg",
		".vob":   "video/mpeg",
		".m2v":   "video/mpeg",
		".m2p":   "video/mpeg",
		".ts":    "video/mp2t",
		".mts":   "video/mp2t",
		".m2ts":  "video/mp2t",
		".mjpeg": "video/x-motion-jpeg",
		".mjpg":  "video/x-motion-jpeg",
		".qt":    "video/quicktime",
		".m3u8":  "application/x-mpegURL",
		".mxf":   "application/mxf",
		".webp":  "image/webp",
		".avif":  "image/avif",
		".heic":  "image/heic",
		".heif":  "image/heif",
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

// ConvertedStreamURLForFile builds the /wails-stream URL for a disk path that should be converted on demand.
func ConvertedStreamURLForFile(baseURL, publicPath, targetFormat string) string {
	return joinStreamBaseURL(baseURL) + streamPath + "?path=" + urlEncode(publicPath) + "&convert=" + urlEncode(targetFormat)
}

func ThumbnailStreamURLForFile(baseURL, publicPath, targetFormat string, maxWidth int, maxHeight int) string {
	return joinStreamBaseURL(baseURL) + streamPath + "?path=" + urlEncode(publicPath) + "&thumb=" + urlEncode(targetFormat) + "&thumbWidth=" + strconv.Itoa(maxWidth) + "&thumbHeight=" + strconv.Itoa(maxHeight)
}

func ThumbnailStreamURLForEntry(baseURL, fsPath string, nestedChain []string, entryPath string, targetFormat string, maxWidth int, maxHeight int) string {
	u := joinStreamBaseURL(baseURL) + streamPath + "?fsPath=" + urlEncode(fsPath) + "&entry=" + urlEncode(entryPath) + "&thumb=" + urlEncode(targetFormat) + "&thumbWidth=" + strconv.Itoa(maxWidth) + "&thumbHeight=" + strconv.Itoa(maxHeight)
	if len(nestedChain) > 0 {
		u += "&nested=" + urlEncode(strings.Join(nestedChain, ","))
	}
	return u
}

func VideoFramePreviewURLForFile(baseURL, publicPath, targetFormat string, frameSeconds float64) string {
	u := joinStreamBaseURL(baseURL) + streamPath + "?path=" + urlEncode(publicPath) + "&frame=" + urlEncode(targetFormat)
	if frameSeconds > 0 {
		u += "&frameTime=" + strconv.FormatFloat(frameSeconds, 'f', 3, 64)
	}
	return u
}

func VideoFramePreviewURLForEntry(baseURL, fsPath string, nestedChain []string, entryPath string, targetFormat string, frameSeconds float64) string {
	u := joinStreamBaseURL(baseURL) + streamPath + "?fsPath=" + urlEncode(fsPath) + "&entry=" + urlEncode(entryPath) + "&frame=" + urlEncode(targetFormat)
	if frameSeconds > 0 {
		u += "&frameTime=" + strconv.FormatFloat(frameSeconds, 'f', 3, 64)
	}
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

func (a *App) GetVideoPreviewURL(publicPath string) string {
	return VideoFramePreviewURLForFile(a.StreamBaseURL(), publicPath, "png", defaultVideoFramePreviewSeconds)
}

func (a *App) GetArchiveVideoPreviewURL(fsPath string, nestedChain []string, entryPath string) string {
	return VideoFramePreviewURLForEntry(a.StreamBaseURL(), fsPath, nestedChain, entryPath, "png", defaultVideoFramePreviewSeconds)
}

func (a *App) GetVideoPlaybackPlan(publicPath string) VideoPlaybackPlan {
	return buildDiskVideoPlaybackPlan(a.StreamBaseURL(), publicPath)
}

func (a *App) GetArchiveVideoPlaybackPlan(fsPath string, nestedChain []string, entryPath string) VideoPlaybackPlan {
	return buildArchiveVideoPlaybackPlan(a.StreamBaseURL(), a.ArchCache(), fsPath, nestedChain, entryPath)
}

func buildDiskVideoPlaybackPlan(baseURL, publicPath string) VideoPlaybackPlan {
	probe, _ := probeVideo(pathref.ToNativePath(pathref.Normalize(publicPath)))
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(publicPath)), ".")
	var videoCodec, audioCodec string
	var durationSeconds float64
	if probe != nil {
		format = firstNonEmpty(format, primaryFormatName(probe.Format.FormatName))
		videoCodec, audioCodec = extractProbeCodecs(probe)
		durationSeconds = parseProbeDuration(probe.Format.Duration)
	}
	mode := decideDiskPlaybackMode(publicPath, videoCodec, audioCodec)
	return VideoPlaybackPlan{
		Mode:            mode,
		Format:          format,
		VideoCodec:      videoCodec,
		AudioCodec:      audioCodec,
		DurationSeconds: durationSeconds,
		ZeroDelay:       true,
		Seekable:        true,
		UseNativePlayer: false,
		URL:             TranscodedPreviewURLForFile(baseURL, publicPath, "mp4", string(mode), 0, defaultPreviewWindowSeconds, durationSeconds),
	}
}

func buildArchiveVideoPlaybackPlan(baseURL string, archCache *cache.Cache, fsPath string, nestedChain []string, entryPath string) VideoPlaybackPlan {
	mode := decidePlaybackMode(entryPath, "", "")
	if mode == VideoPlaybackModeDirect {
		return VideoPlaybackPlan{
			Mode:            VideoPlaybackModeDirect,
			Format:          strings.TrimPrefix(strings.ToLower(filepath.Ext(entryPath)), "."),
			Seekable:        false,
			UseNativePlayer: false,
			URL:             StreamURLForEntry(baseURL, fsPath, nestedChain, entryPath),
		}
	}
	return VideoPlaybackPlan{
		Mode:            mode,
		Format:          strings.TrimPrefix(strings.ToLower(filepath.Ext(entryPath)), "."),
		ZeroDelay:       true,
		Seekable:        false,
		UseNativePlayer: false,
		URL:             TranscodedPreviewURLForEntry(baseURL, fsPath, nestedChain, entryPath, "mp4", string(mode), 0, 0, 0),
	}
}

func decidePlaybackMode(name, videoCodec, audioCodec string) VideoPlaybackMode {
	ext := strings.ToLower(filepath.Ext(name))
	if directPreviewExtensions[ext] {
		return VideoPlaybackModeDirect
	}
	_ = videoCodec
	_ = audioCodec
	return VideoPlaybackModeTranscode
}

func decideDiskPlaybackMode(name, videoCodec, audioCodec string) VideoPlaybackMode {
	_ = name
	_ = videoCodec
	_ = audioCodec
	return VideoPlaybackModeTranscode
}

func shouldAttemptSegmentedDiskPlayback(name, videoCodec, audioCodec string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".m4v", ".mkv", ".webm", ".mov", ".qt",
		".avi", ".divx", ".xvid",
		".flv", ".ts", ".mts", ".m2ts",
		".mpg", ".mpeg", ".vob", ".m2v", ".m2p",
		".wmv", ".asf", ".ogv", ".mxf",
		".mjpeg", ".mjpg":
		return true
	}
	return false
}

func probeVideo(nativePath string) (*mediaProbeOutput, error) {
	return probeVideoWithGStreamer(nativePath)
}

func extractProbeCodecs(probe *mediaProbeOutput) (string, string) {
	if probe == nil {
		return "", ""
	}
	var videoCodec string
	var audioCodec string
	for _, stream := range probe.Streams {
		switch stream.CodecType {
		case "video":
			if videoCodec == "" {
				videoCodec = strings.ToLower(stream.CodecName)
			}
		case "audio":
			if audioCodec == "" {
				audioCodec = strings.ToLower(stream.CodecName)
			}
		}
	}
	return videoCodec, audioCodec
}

func parseProbeDuration(raw string) float64 {
	if raw == "" {
		return 0
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return seconds
}

func primaryFormatName(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	return strings.ToLower(strings.TrimSpace(parts[0]))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func shouldTranscodeVideoPreview(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".avi", ".divx", ".xvid", ".flv", ".mov", ".qt", ".mpg", ".mpeg", ".vob", ".m2v", ".m2p", ".ts", ".mts", ".m2ts", ".mjpeg", ".mjpg":
		return true
	default:
		return false
	}
}

func TranscodedPreviewURLForFile(baseURL, publicPath, targetFormat, playbackMode string, startSeconds float64, windowSeconds float64, durationSeconds float64) string {
	u := joinStreamBaseURL(baseURL) + streamPath + "?path=" + urlEncode(publicPath) + "&transcode=" + urlEncode(targetFormat)
	if playbackMode != "" {
		u += "&mode=" + urlEncode(playbackMode)
	}
	if startSeconds > 0 {
		u += "&start=" + urlEncode(strconv.FormatFloat(startSeconds, 'f', 3, 64))
	}
	if windowSeconds > 0 {
		u += "&window=" + urlEncode(strconv.FormatFloat(windowSeconds, 'f', 3, 64))
	}
	if durationSeconds > 0 {
		u += "&duration=" + urlEncode(strconv.FormatFloat(durationSeconds, 'f', 3, 64))
	}
	return u
}

func TranscodedPreviewURLForEntry(baseURL, fsPath string, nestedChain []string, entryPath string, targetFormat, playbackMode string, startSeconds float64, windowSeconds float64, durationSeconds float64) string {
	u := joinStreamBaseURL(baseURL) + streamPath + "?fsPath=" + urlEncode(fsPath) + "&entry=" + urlEncode(entryPath) + "&transcode=" + urlEncode(targetFormat)
	if playbackMode != "" {
		u += "&mode=" + urlEncode(playbackMode)
	}
	if startSeconds > 0 {
		u += "&start=" + urlEncode(strconv.FormatFloat(startSeconds, 'f', 3, 64))
	}
	if windowSeconds > 0 {
		u += "&window=" + urlEncode(strconv.FormatFloat(windowSeconds, 'f', 3, 64))
	}
	if durationSeconds > 0 {
		u += "&duration=" + urlEncode(strconv.FormatFloat(durationSeconds, 'f', 3, 64))
	}
	if len(nestedChain) > 0 {
		u += "&nested=" + urlEncode(strings.Join(nestedChain, ","))
	}
	return u
}

func probeFileDurationSeconds(nativePath string) float64 {
	probe, err := probeVideo(nativePath)
	if err != nil {
		return 0
	}
	return parseProbeDuration(probe.Format.Duration)
}
