package app

import (
	"strconv"
	"strings"
)

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

func FFmpegPreviewURLForFile(baseURL, publicPath, targetFormat, playbackMode string, startSeconds float64, windowSeconds float64, durationSeconds float64) string {
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

func FFmpegPreviewURLForEntry(baseURL, fsPath string, nestedChain []string, entryPath string, targetFormat, playbackMode string, startSeconds float64, windowSeconds float64, durationSeconds float64) string {
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