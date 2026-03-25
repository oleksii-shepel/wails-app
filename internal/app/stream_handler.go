package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wails-app/internal/pathref"
	vfslib "wails-app/vfs"
	"wails-app/vfs/archive/cache"
)

const streamPath = "/wails-stream"

type VideoPlaybackMode string

const (
	VideoPlaybackModeDirect    VideoPlaybackMode = "direct"
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

type ffprobeOutput struct {
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

	// ── Converted disk image ───────────────────────────────────────────────
	path := q.Get("path")
	convertFormat := q.Get("convert")
	if path != "" && convertFormat != "" {
		serveConvertedPath(w, r, app, path, convertFormat)
		return
	}
	if path != "" && transcodeFormat != "" {
		serveTranscodedPath(w, r, path, transcodeFormat, playbackMode, startSeconds)
		return
	}

	// ── Archive entry ─────────────────────────────────────────────────────
	fsPath := q.Get("fsPath")
	entry := q.Get("entry")
	if fsPath != "" && entry != "" && transcodeFormat != "" {
		serveTranscodedArchiveEntry(w, r, archCache, fsPath, entry, q.Get("nested"), transcodeFormat, playbackMode, startSeconds)
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

func serveTranscodedPath(w http.ResponseWriter, r *http.Request, publicPath string, targetFormat string, playbackMode string, startSeconds float64) {
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

	stdout, stderr, cmd, err := startFFmpegTranscode(r.Context(), buildFFmpegInputArgs(nativePath, startSeconds), normalizePlaybackMode(playbackMode))
	if err != nil {
		handleTranscodeStartError(w, err)
		return
	}
	defer stdout.Close()

	streamTranscodedOutput(w, r, cmd, stdout, stderr)
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

	stdout, stderr, cmd, err := startFFmpegTranscode(r.Context(), []string{"-i", "pipe:0"}, normalizePlaybackMode(playbackMode))
	if err != nil {
		handleTranscodeStartError(w, err)
		return
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		stdout.Close()
		http.Error(w, "failed to prepare transcode input", http.StatusInternalServerError)
		return
	}

	go func() {
		defer stdin.Close()
		_, _ = io.Copy(stdin, entryFile)
	}()

	defer stdout.Close()
	streamTranscodedOutput(w, r, cmd, stdout, stderr)
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

func startFFmpegTranscode(ctx context.Context, inputArgs []string, playbackMode VideoPlaybackMode) (io.ReadCloser, *bytes.Buffer, *exec.Cmd, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, nil, nil, err
	}

	args := append([]string{
		"-hide_banner",
		"-loglevel", "error",
		"-fflags", "+nobuffer",
		"-flags", "+low_delay",
		"-muxdelay", "0",
		"-muxpreload", "0",
	}, inputArgs...)
	args = append(args, "-map", "0:v:0", "-map", "0:a?:0", "-sn", "-dn")
	if playbackMode == VideoPlaybackModeRemux {
		args = append(args,
			"-c:v", "copy",
			"-c:a", "copy",
			"-bsf:a", "aac_adtstoasc",
		)
	} else {
		args = append(args,
			"-probesize", "32k",
			"-analyzeduration", "0",
			"-c:v", "libx264",
			"-preset", "ultrafast",
			"-tune", "zerolatency",
			"-crf", "23",
			"-pix_fmt", "yuv420p",
			"-g", "30",
			"-keyint_min", "30",
			"-sc_threshold", "0",
			"-c:a", "aac",
			"-b:a", "128k",
		)
	}
	args = append(args,
		"-movflags", "frag_keyframe+empty_moov+default_base_moof+faststart",
		"-f", "mp4",
		"pipe:1",
	)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		stdout.Close()
		return nil, nil, nil, err
	}

	return stdout, stderr, cmd, nil
}

func handleTranscodeStartError(w http.ResponseWriter, err error) {
	if execErr, ok := err.(*exec.Error); ok && execErr.Err != nil {
		http.Error(w, "ffmpeg is not installed or not available on PATH", http.StatusServiceUnavailable)
		return
	}
	http.Error(w, "failed to start ffmpeg transcoder", http.StatusInternalServerError)
}

func streamTranscodedOutput(w http.ResponseWriter, r *http.Request, cmd *exec.Cmd, stdout io.ReadCloser, stderr *bytes.Buffer) {
	setCORSHeaders(w)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "video/mp4")
	w.WriteHeader(http.StatusOK)

	_, copyErr := io.Copy(w, stdout)
	waitErr := cmd.Wait()
	if copyErr == nil && waitErr == nil {
		return
	}

	if r.Context().Err() != nil {
		return
	}
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

func buildFFmpegInputArgs(inputPath string, startSeconds float64) []string {
	args := make([]string, 0, 4)
	if startSeconds > 0 {
		args = append(args, "-ss", strconv.FormatFloat(startSeconds, 'f', 3, 64))
	}
	args = append(args, "-i", inputPath)
	return args
}

func normalizePlaybackMode(raw string) VideoPlaybackMode {
	if strings.EqualFold(raw, string(VideoPlaybackModeRemux)) {
		return VideoPlaybackModeRemux
	}
	return VideoPlaybackModeTranscode
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
	return a.GetVideoPlaybackPlan(publicPath).URL
}

func (a *App) GetArchiveVideoPreviewURL(fsPath string, nestedChain []string, entryPath string) string {
	return a.GetArchiveVideoPlaybackPlan(fsPath, nestedChain, entryPath).URL
}

func (a *App) GetVideoPlaybackPlan(publicPath string) VideoPlaybackPlan {
	return buildDiskVideoPlaybackPlan(a.StreamBaseURL(), publicPath)
}

func (a *App) GetArchiveVideoPlaybackPlan(fsPath string, nestedChain []string, entryPath string) VideoPlaybackPlan {
	return buildArchiveVideoPlaybackPlan(a.StreamBaseURL(), fsPath, nestedChain, entryPath)
}

func buildDiskVideoPlaybackPlan(baseURL, publicPath string) VideoPlaybackPlan {
	plan := VideoPlaybackPlan{
		Mode:            VideoPlaybackModeDirect,
		Format:          strings.TrimPrefix(strings.ToLower(filepath.Ext(publicPath)), "."),
		ZeroDelay:       false,
		Seekable:        true,
		UseNativePlayer: false,
		URL:             StreamURLForFile(baseURL, publicPath),
	}
	if directPreviewExtensions[strings.ToLower(filepath.Ext(publicPath))] {
		return plan
	}

	probe, err := probeVideo(pathref.ToNativePath(pathref.Normalize(publicPath)))
	if err == nil {
		plan.Format = firstNonEmpty(plan.Format, primaryFormatName(probe.Format.FormatName))
		plan.VideoCodec, plan.AudioCodec = extractProbeCodecs(probe)
		plan.DurationSeconds = parseProbeDuration(probe.Format.Duration)
	}

	plan.Mode = decidePlaybackMode(publicPath, plan.VideoCodec, plan.AudioCodec)
	plan.ZeroDelay = plan.Mode != VideoPlaybackModeDirect
	plan.UseNativePlayer = plan.Mode != VideoPlaybackModeDirect
	plan.Seekable = true
	if plan.Mode == VideoPlaybackModeDirect {
		return plan
	}
	plan.URL = FFmpegPreviewURLForFile(baseURL, publicPath, "mp4", string(plan.Mode), 0)
	return plan
}

func buildArchiveVideoPlaybackPlan(baseURL, fsPath string, nestedChain []string, entryPath string) VideoPlaybackPlan {
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
		UseNativePlayer: true,
		URL:             FFmpegPreviewURLForEntry(baseURL, fsPath, nestedChain, entryPath, "mp4", string(mode), 0),
	}
}

func decidePlaybackMode(name, videoCodec, audioCodec string) VideoPlaybackMode {
	ext := strings.ToLower(filepath.Ext(name))
	if directPreviewExtensions[ext] {
		return VideoPlaybackModeDirect
	}
	if remuxableVideoCodecs[strings.ToLower(videoCodec)] && remuxableAudioCodecs[strings.ToLower(audioCodec)] {
		switch ext {
		case ".mov", ".qt", ".flv", ".ts", ".mts", ".m2ts":
			return VideoPlaybackModeRemux
		}
	}
	if transcodePreferredVideoCodecs[strings.ToLower(videoCodec)] {
		return VideoPlaybackModeTranscode
	}
	if shouldTranscodeVideoPreview(name) {
		return VideoPlaybackModeTranscode
	}
	return VideoPlaybackModeDirect
}

func probeVideo(nativePath string) (*ffprobeOutput, error) {
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return nil, err
	}
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_format", "-show_streams", nativePath)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var parsed ffprobeOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func extractProbeCodecs(probe *ffprobeOutput) (string, string) {
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

func FFmpegPreviewURLForFile(baseURL, publicPath, targetFormat, playbackMode string, startSeconds float64) string {
	u := joinStreamBaseURL(baseURL) + streamPath + "?path=" + urlEncode(publicPath) + "&transcode=" + urlEncode(targetFormat)
	if playbackMode != "" {
		u += "&mode=" + urlEncode(playbackMode)
	}
	if startSeconds > 0 {
		u += "&start=" + urlEncode(strconv.FormatFloat(startSeconds, 'f', 3, 64))
	}
	return u
}

func FFmpegPreviewURLForEntry(baseURL, fsPath string, nestedChain []string, entryPath string, targetFormat, playbackMode string, startSeconds float64) string {
	u := joinStreamBaseURL(baseURL) + streamPath + "?fsPath=" + urlEncode(fsPath) + "&entry=" + urlEncode(entryPath) + "&transcode=" + urlEncode(targetFormat)
	if playbackMode != "" {
		u += "&mode=" + urlEncode(playbackMode)
	}
	if startSeconds > 0 {
		u += "&start=" + urlEncode(strconv.FormatFloat(startSeconds, 'f', 3, 64))
	}
	if len(nestedChain) > 0 {
		u += "&nested=" + urlEncode(strings.Join(nestedChain, ","))
	}
	return u
}
