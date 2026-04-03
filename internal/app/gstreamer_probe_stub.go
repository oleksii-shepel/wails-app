//go:build !cgo

package app

func probeVideoWithGStreamer(nativePath string) (*mediaProbeOutput, error) {
	return nil, nil
}
