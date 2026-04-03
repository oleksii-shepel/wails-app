//go:build !gstreamer

package app

import "fmt"

func probeVideoWithGStreamer(nativePath string) (*mediaProbeOutput, error) {
	return nil, fmt.Errorf("gstreamer probing requires cgo")
}
