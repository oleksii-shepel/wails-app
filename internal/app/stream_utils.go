package app

import (
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

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
