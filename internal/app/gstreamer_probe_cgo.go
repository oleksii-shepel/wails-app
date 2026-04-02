//go:build cgo

package app

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/pbutils"
)

func probeVideoWithGStreamer(nativePath string) (*mediaProbeOutput, error) {
	if err := ensureGStreamerRuntime(); err != nil {
		return nil, err
	}
	gstInitOnce.Do(func() {
		gst.Init(nil)
	})

	discoverer, err := pbutils.NewDiscoverer(gst.ClockTime(15 * time.Second))
	if err != nil {
		return nil, fmt.Errorf("failed to create gstreamer discoverer: %w", err)
	}

	info, err := discoverer.DiscoverURI(fileURIForNativePath(nativePath))
	if err != nil {
		return nil, fmt.Errorf("failed to discover media: %w", err)
	}

	probe := &mediaProbeOutput{}
	probe.Format.Duration = formatClockTimeSeconds(info.GetDuration())
	probe.Format.FormatName = discovererFormatName(info, nativePath)

	for _, video := range info.GetVideoStreams() {
		probe.Streams = append(probe.Streams, struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
		}{
			CodecType: "video",
			CodecName: codecNameFromCapsString(video.GetCaps().String(), "video"),
		})
	}
	for _, audio := range info.GetAudioStreams() {
		probe.Streams = append(probe.Streams, struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
		}{
			CodecType: "audio",
			CodecName: codecNameFromCapsString(audio.GetCaps().String(), "audio"),
		})
	}

	return probe, nil
}

func fileURIForNativePath(nativePath string) string {
	u := &url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(nativePath),
	}
	return u.String()
}

func formatClockTimeSeconds(value gst.ClockTime) string {
	if value <= 0 {
		return ""
	}
	seconds := float64(value) / float64(time.Second)
	return strconv.FormatFloat(seconds, 'f', 3, 64)
}

func discovererFormatName(info *pbutils.DiscovererInfo, nativePath string) string {
	for _, stream := range info.GetContainerStreams() {
		if stream == nil || stream.GetCaps() == nil {
			continue
		}
		if formatName := formatNameFromCapsString(stream.GetCaps().String()); formatName != "" {
			return formatName
		}
	}

	if stream := info.GetStreamInfo(); stream != nil && stream.GetCaps() != nil {
		if formatName := formatNameFromCapsString(stream.GetCaps().String()); formatName != "" {
			return formatName
		}
	}

	return strings.TrimPrefix(strings.ToLower(filepath.Ext(nativePath)), ".")
}

func formatNameFromCapsString(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(value, "video/quicktime"):
		if strings.Contains(value, "variant=(string)iso") {
			return "mp4"
		}
		return "mov"
	case strings.Contains(value, "video/x-matroska"):
		if strings.Contains(value, "webm") {
			return "webm"
		}
		return "mkv"
	case strings.Contains(value, "application/ogg"):
		return "ogv"
	case strings.Contains(value, "video/x-msvideo"):
		return "avi"
	case strings.Contains(value, "video/mpegts"):
		return "ts"
	case strings.Contains(value, "video/mpeg"):
		return "mpeg"
	case strings.Contains(value, "video/x-flv"):
		return "flv"
	case strings.Contains(value, "video/x-ms-asf"):
		return "asf"
	default:
		return ""
	}
}

func codecNameFromCapsString(raw string, streamType string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch streamType {
	case "video":
		switch {
		case strings.Contains(value, "video/x-h264"):
			return "h264"
		case strings.Contains(value, "video/x-h265"), strings.Contains(value, "video/x-hevc"):
			return "hevc"
		case strings.Contains(value, "video/x-av1"):
			return "av1"
		case strings.Contains(value, "video/x-vp9"):
			return "vp9"
		case strings.Contains(value, "video/x-vp8"):
			return "vp8"
		case strings.Contains(value, "image/jpeg"):
			return "mjpeg"
		case strings.Contains(value, "video/mpeg"):
			switch {
			case strings.Contains(value, "mpegversion=(int)4"):
				return "mpeg4"
			case strings.Contains(value, "mpegversion=(int)2"):
				return "mpeg2video"
			case strings.Contains(value, "mpegversion=(int)1"):
				return "mpeg1video"
			}
		case strings.Contains(value, "video/x-divx"), strings.Contains(value, "video/x-xvid"):
			return "mpeg4"
		case strings.Contains(value, "video/x-wmv"):
			return "wmv3"
		case strings.Contains(value, "video/x-huffyuv"):
			return "huffyuv"
		}
	case "audio":
		switch {
		case strings.Contains(value, "audio/mpeg"):
			switch {
			case strings.Contains(value, "mpegversion=(int)4"):
				return "aac"
			case strings.Contains(value, "layer=(int)3"):
				return "mp3"
			}
		case strings.Contains(value, "audio/x-ac3"):
			return "ac3"
		case strings.Contains(value, "audio/x-eac3"):
			return "eac3"
		case strings.Contains(value, "audio/x-opus"):
			return "opus"
		case strings.Contains(value, "audio/x-vorbis"):
			return "vorbis"
		case strings.Contains(value, "audio/x-flac"):
			return "flac"
		case strings.Contains(value, "audio/x-alac"):
			return "alac"
		case strings.Contains(value, "audio/x-wavpack"):
			return "wavpack"
		case strings.Contains(value, "audio/x-dts"):
			return "dts"
		case strings.Contains(value, "audio/x-private1-ac3"):
			return "ac3"
		}
	}
	return ""
}
