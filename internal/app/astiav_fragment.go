package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/asticode/go-astiav"
)

// needsTranscode checks if the input streams contain codecs that browsers can't play.
func needsTranscode(inputFormatContext *astiav.FormatContext) bool {
	for _, s := range inputFormatContext.Streams() {
		cp := s.CodecParameters()
		switch cp.MediaType() {
		case astiav.MediaTypeVideo:
			codecName := strings.ToLower(cp.CodecID().String())
			if !strings.Contains(codecName, "h264") && !strings.Contains(codecName, "h265") &&
				!strings.Contains(codecName, "hevc") && !strings.Contains(codecName, "vp8") &&
				!strings.Contains(codecName, "vp9") && !strings.Contains(codecName, "av1") {
				return true
			}
		case astiav.MediaTypeAudio:
			codecName := strings.ToLower(cp.CodecID().String())
			if !strings.Contains(codecName, "aac") && !strings.Contains(codecName, "opus") &&
				!strings.Contains(codecName, "vorbis") && !strings.Contains(codecName, "flac") &&
				!strings.Contains(codecName, "mp3") {
				return true
			}
		}
	}
	return false
}

func buildSegmentedVideoFragment(ctx context.Context, nativePath string, startSeconds float64, durationSeconds float64) ([]byte, []byte, error) {
	inputFormatContext := astiav.AllocFormatContext()
	if inputFormatContext == nil {
		return nil, nil, fmt.Errorf("failed allocating input format context")
	}
	defer inputFormatContext.Free()

	if err := inputFormatContext.OpenInput(nativePath, nil, nil); err != nil {
		return nil, nil, fmt.Errorf("failed opening input: %w", err)
	}
	defer inputFormatContext.CloseInput()

	if err := inputFormatContext.FindStreamInfo(nil); err != nil {
		return nil, nil, fmt.Errorf("failed finding stream info: %w", err)
	}

	if needsTranscode(inputFormatContext) {
		return buildTranscodedFragment(ctx, inputFormatContext, startSeconds, durationSeconds)
	}
	return buildRemuxedFragment(ctx, inputFormatContext, startSeconds, durationSeconds)
}

func buildTranscodedFragment(ctx context.Context, inputFormatContext *astiav.FormatContext, startSeconds float64, durationSeconds float64) ([]byte, []byte, error) {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		transcodeLoopPreOpened(ctx, inputFormatContext, startSeconds, durationSeconds, pw)
	}()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(pr); err != nil {
		return nil, nil, fmt.Errorf("transcode failed: %w", err)
	}

	return splitFragmentedMP4(buf.Bytes())
}

func buildRemuxedFragment(ctx context.Context, inputFormatContext *astiav.FormatContext, startSeconds float64, durationSeconds float64) ([]byte, []byte, error) {
	if startSeconds > 0 {
		seekTarget := int64(startSeconds * float64(astiav.TimeBase))
		if err := inputFormatContext.SeekFrame(-1, seekTarget, astiav.NewSeekFlags(astiav.SeekFlagBackward)); err != nil {
			// Non-fatal
		}
	}

	outputFormatContext, err := astiav.AllocOutputFormatContext(nil, "mp4", "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed allocating output format context: %w", err)
	}
	defer outputFormatContext.Free()

	var buf bytes.Buffer
	ioContext, err := astiav.AllocIOContext(4096, true, nil, nil, func(b []byte) (int, error) {
		return buf.Write(b)
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed allocating custom IO context: %w", err)
	}
	defer ioContext.Free()
	outputFormatContext.SetPb(ioContext)

	streamMapping := make(map[int]int)
	outputStreamIdx := 0
	for _, inStream := range inputFormatContext.Streams() {
		cp := inStream.CodecParameters()
		if cp.MediaType() != astiav.MediaTypeVideo && cp.MediaType() != astiav.MediaTypeAudio {
			streamMapping[inStream.Index()] = -1
			continue
		}
		outStream := outputFormatContext.NewStream(nil)
		if outStream == nil {
			return nil, nil, fmt.Errorf("failed allocating output stream")
		}
		if err := cp.Copy(outStream.CodecParameters()); err != nil {
			return nil, nil, fmt.Errorf("failed copying codec parameters: %w", err)
		}
		outStream.CodecParameters().SetCodecTag(0)
		streamMapping[inStream.Index()] = outputStreamIdx
		outputStreamIdx++
	}

	options := astiav.NewDictionary()
	defer options.Free()
	options.Set("movflags", "frag_keyframe+empty_moov+default_base_moof", 0)

	if err := outputFormatContext.WriteHeader(options); err != nil {
		return nil, nil, fmt.Errorf("failed writing header: %w", err)
	}

	pkt := astiav.AllocPacket()
	if pkt == nil {
		return nil, nil, fmt.Errorf("failed allocating packet")
	}
	defer pkt.Free()

	var startPts int64 = -1
	for {
		select {
		case <-ctx.Done():
			break
		default:
		}

		if err := inputFormatContext.ReadFrame(pkt); err != nil {
			break
		}

		outIdx, ok := streamMapping[pkt.StreamIndex()]
		if !ok || outIdx < 0 {
			pkt.Unref()
			continue
		}

		inStream := inputFormatContext.Streams()[pkt.StreamIndex()]
		outStream := outputFormatContext.Streams()[outIdx]

		if inStream.CodecParameters().MediaType() == astiav.MediaTypeVideo {
			if startPts == -1 {
				startPts = pkt.Pts()
			}
			if durationSeconds > 0 && startPts != -1 {
				timeDiff := float64(pkt.Pts()-startPts) * float64(inStream.TimeBase().Num()) / float64(inStream.TimeBase().Den())
				if timeDiff > durationSeconds {
					pkt.Unref()
					break
				}
			}
		}

		pkt.SetStreamIndex(outIdx)
		pkt.RescaleTs(inStream.TimeBase(), outStream.TimeBase())
		pkt.SetPos(-1)

		if err := outputFormatContext.WriteInterleavedFrame(pkt); err != nil {
			pkt.Unref()
			return nil, nil, fmt.Errorf("failed writing frame: %w", err)
		}
		pkt.Unref()
	}

	if err := outputFormatContext.WriteTrailer(); err != nil {
		return nil, nil, fmt.Errorf("failed writing trailer: %w", err)
	}

	return splitFragmentedMP4(buf.Bytes())
}
