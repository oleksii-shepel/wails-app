//go:build !cgo

package app

import (
	"context"
	"fmt"
)

func buildGStreamerSegmentedVideoFragment(ctx context.Context, nativePath string, startSeconds float64, durationSeconds float64) ([]byte, []byte, error) {
	return nil, nil, fmt.Errorf("gstreamer segmented mp4 generation is currently implemented only on windows")
}
