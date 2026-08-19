package app

import (
	"context"
	"path/filepath"
	"strings"

	"wails-app/internal/pathref"
	"wails-app/vfs/archive/cache"
)

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

func (a *App) GetVideoPlaybackPlan(publicPath string) VideoPlaybackPlan {
	return buildDiskVideoPlaybackPlan(a.StreamBaseURL(), publicPath)
}

func (a *App) GetArchiveVideoPlaybackPlan(fsPath string, nestedChain []string, entryPath string) VideoPlaybackPlan {
	return buildArchiveVideoPlaybackPlan(a.StreamBaseURL(), a.ArchCache(), fsPath, nestedChain, entryPath)
}

func buildDiskVideoPlaybackPlan(baseURL, publicPath string) VideoPlaybackPlan {
	if pathref.IsWeb(pathref.Normalize(publicPath)) {
		return VideoPlaybackPlan{
			Mode:            VideoPlaybackModeDirect,
			Format:          strings.TrimPrefix(strings.ToLower(filepath.Ext(publicPath)), "."),
			Seekable:        true,
			UseNativePlayer: false,
			URL:             StreamURLForFile(baseURL, publicPath),
		}
	}

	probe, _ := probeVideo(pathref.ToNativePath(pathref.Normalize(publicPath)))
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(publicPath)), ".")
	var videoCodec, audioCodec string
	var durationSeconds float64
	if probe != nil {
		format = firstNonEmpty(format, primaryFormatName(probe.Format.FormatName))
		videoCodec, audioCodec = extractProbeCodecs(probe)
		durationSeconds = resolveVideoDurationSeconds(context.Background(), pathref.ToNativePath(pathref.Normalize(publicPath)), probe)
	}

	return VideoPlaybackPlan{
		Mode:            VideoPlaybackModeDirect,
		Format:          format,
		VideoCodec:      videoCodec,
		AudioCodec:      audioCodec,
		DurationSeconds: durationSeconds,
		Seekable:        true,
		UseNativePlayer: false,
		URL:             StreamURLForFile(baseURL, publicPath),
	}
}

func buildArchiveVideoPlaybackPlan(baseURL string, archCache *cache.Cache, fsPath string, nestedChain []string, entryPath string) VideoPlaybackPlan {
	_ = archCache

	return VideoPlaybackPlan{
		Mode:            VideoPlaybackModeDirect,
		Format:          strings.TrimPrefix(strings.ToLower(filepath.Ext(entryPath)), "."),
		Seekable:        false,
		UseNativePlayer: false,
		URL:             StreamURLForEntry(baseURL, fsPath, nestedChain, entryPath),
	}
}

func decidePlaybackMode(name, videoCodec, audioCodec string) VideoPlaybackMode {
	ext := strings.ToLower(filepath.Ext(name))
	if directPreviewExtensions[ext] && supportsDirectVideoCodec(videoCodec) {
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

func decideDiskPlaybackMode(name, videoCodec, audioCodec string) VideoPlaybackMode {
	vc := strings.ToLower(videoCodec)
	ac := strings.ToLower(audioCodec)
	if remuxableVideoCodecs[vc] && remuxableAudioCodecs[ac] {
		return VideoPlaybackModeRemux
	}
	if transcodePreferredVideoCodecs[vc] || shouldTranscodeVideoPreview(name) {
		return VideoPlaybackModeTranscode
	}
	return VideoPlaybackModeRemux
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

func supportsDirectVideoCodec(videoCodec string) bool {
	return !transcodePreferredVideoCodecs[strings.ToLower(videoCodec)]
}

func supportsSegmentedVideoCodec(videoCodec string) bool {
	return supportsDirectVideoCodec(videoCodec)
}

func shouldPreferDirectDiskPlayback(name, videoCodec string) bool {
	if !supportsDirectVideoCodec(videoCodec) {
		return false
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".m4v":
		return true
	default:
		return false
	}
}

func shouldTranscodeVideoPreview(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".avi", ".divx", ".xvid", ".flv", ".mov", ".qt", ".mpg", ".mpeg", ".vob", ".m2v", ".m2p", ".ts", ".mts", ".m2ts", ".mjpeg", ".mjpg":
		return true
	default:
		return false
	}
}