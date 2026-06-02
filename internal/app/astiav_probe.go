//go:build cgo && (windows || darwin || linux)

package app

import (
	"fmt"

	"github.com/asticode/go-astiav"
)

func probeVideo(nativePath string) (*ffprobeOutput, error) {
	fmtCtx := astiav.AllocFormatContext()
	if fmtCtx == nil {
		return nil, fmt.Errorf("failed allocating format context")
	}
	defer fmtCtx.Free()

	if err := fmtCtx.OpenInput(nativePath, nil, nil); err != nil {
		return nil, fmt.Errorf("failed opening input: %w", err)
	}
	defer fmtCtx.CloseInput()

	if err := fmtCtx.FindStreamInfo(nil); err != nil {
		return nil, fmt.Errorf("failed finding stream info: %w", err)
	}

	out := &ffprobeOutput{}
	if fmtCtx.InputFormat() != nil {
		out.Format.FormatName = fmtCtx.InputFormat().Name()
	}

	duration := float64(fmtCtx.Duration()) / float64(astiavTimeBase)
	out.Format.Duration = fmt.Sprintf("%f", duration)

	for i := 0; i < fmtCtx.NbStreams(); i++ {
		stream := fmtCtx.Streams()[i]
		codecParams := stream.CodecParameters()

		// Skip attached pictures (album art, cover thumbnails) — they are not playable video.
		if codecParams.MediaType() == astiav.MediaTypeVideo && stream.DispositionFlags().Has(astiavDispositionFlagAttachedPic) {
			continue
		}

		codecType := ""
		switch codecParams.MediaType() {
		case astiav.MediaTypeVideo:
			codecType = "video"
		case astiav.MediaTypeAudio:
			codecType = "audio"
		case astiav.MediaTypeSubtitle:
			codecType = "subtitle"
		}

		codecName := ""
		codec := astiav.FindDecoder(codecParams.CodecID())
		if codec != nil {
			codecName = codec.Name()
		}

		out.Streams = append(out.Streams, struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
		}{
			CodecType: codecType,
			CodecName: codecName,
		})
	}

	return out, nil
}
