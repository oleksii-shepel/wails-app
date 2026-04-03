//go:build !gstreamer

package app

import (
	"context"
	"fmt"
)

func buildGStreamerFragmentedMP4(ctx context.Context, nativePath string, startSeconds float64, windowSeconds float64, playbackMode VideoPlaybackMode) ([]byte, error) {
	return nil, fmt.Errorf("gstreamer transcoding requires cgo")
}

func extractVideoFrameWithGStreamer(ctx context.Context, inputPath string, targetFormat string, frameSeconds float64) ([]byte, error) {
	return nil, fmt.Errorf("gstreamer frame extraction requires cgo")
}
