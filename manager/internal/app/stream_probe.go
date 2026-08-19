package app

import (
	"context"
	"strconv"
	"strings"
)

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

func resolveVideoDurationSeconds(ctx context.Context, nativePath string, probe *ffprobeOutput) float64 {
	if probe != nil {
		if durationSeconds := parseProbeDuration(probe.Format.Duration); durationSeconds > 0 {
			return durationSeconds
		}
	}

	durationSeconds, err := estimateMediaDurationSeconds(ctx, nativePath)
	if err != nil || durationSeconds <= 0 {
		return 0
	}
	return durationSeconds
}

func probeFileDurationSeconds(nativePath string) float64 {
	probe, err := probeVideo(nativePath)
	if err != nil {
		return 0
	}
	return resolveVideoDurationSeconds(context.Background(), nativePath, probe)
}