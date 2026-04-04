package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"

	"github.com/asticode/go-astiav"
)

func isSegmentableStream(s *astiav.Stream) bool {
	if s == nil {
		return false
	}
	if s.DispositionFlags().Has(astiav.DispositionFlagAttachedPic) {
		return false
	}
	mediaType := s.CodecParameters().MediaType()
	return mediaType == astiav.MediaTypeVideo || mediaType == astiav.MediaTypeAudio
}

func timestampSeconds(pts int64, tb astiav.Rational) float64 {
	if pts == astiav.NoPtsValue || tb.Num() == 0 || tb.Den() == 0 {
		return 0
	}
	return float64(pts) * float64(tb.Num()) / float64(tb.Den())
}

func secondsToTimestamp(seconds float64, tb astiav.Rational) int64 {
	if seconds <= 0 || tb.Num() == 0 || tb.Den() == 0 {
		return 0
	}
	return int64(math.Round(seconds * float64(tb.Den()) / float64(tb.Num())))
}

func isSupportedSegmentVideoCodec(codecID astiav.CodecID) bool {
	switch codecID {
	case astiav.CodecIDH264, astiav.CodecIDHevc, astiav.CodecIDVp8, astiav.CodecIDVp9, astiav.CodecIDAv1:
		return true
	default:
		return false
	}
}

func isSupportedSegmentAudioCodec(codecID astiav.CodecID) bool {
	switch codecID {
	case astiav.CodecIDAac, astiav.CodecIDOpus, astiav.CodecIDVorbis, astiav.CodecIDFlac, astiav.CodecIDMp3:
		return true
	default:
		return false
	}
}

// needsTranscode checks if the input streams contain codecs that the segmented fMP4 path should re-encode.
func needsTranscode(inputFormatContext *astiav.FormatContext) bool {
	for _, s := range inputFormatContext.Streams() {
		if !isSegmentableStream(s) {
			continue
		}
		cp := s.CodecParameters()
		switch cp.MediaType() {
		case astiav.MediaTypeVideo:
			if !isSupportedSegmentVideoCodec(cp.CodecID()) {
				return true
			}
		case astiav.MediaTypeAudio:
			if !isSupportedSegmentAudioCodec(cp.CodecID()) {
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

	// Always transcode segments to guarantee each segment starts with a
	// keyframe. Remuxing (packet copy) after a backward seek can leave the
	// first frames as P/B frames referring to a dropped keyframe, causing
	// decode glitches and timestamp overlap at segment boundaries.
	return buildTranscodedFragment(ctx, inputFormatContext, startSeconds, durationSeconds)
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
		if !isSegmentableStream(inStream) {
			streamMapping[inStream.Index()] = -1
			continue
		}
		cp := inStream.CodecParameters()
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

	endSeconds := 0.0
	if durationSeconds > 0 {
		endSeconds = startSeconds + durationSeconds
	}
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
		packetTimeSeconds := timestampSeconds(pkt.Pts(), inStream.TimeBase())
		if pkt.Pts() == astiav.NoPtsValue {
			packetTimeSeconds = timestampSeconds(pkt.Dts(), inStream.TimeBase())
		}

		if packetTimeSeconds < startSeconds {
			pkt.Unref()
			continue
		}
		if endSeconds > 0 && packetTimeSeconds >= endSeconds {
			pkt.Unref()
			if inStream.CodecParameters().MediaType() == astiav.MediaTypeVideo {
				break
			}
			continue
		}

		pkt.SetStreamIndex(outIdx)
		pkt.RescaleTs(inStream.TimeBase(), outStream.TimeBase())
		startTimestamp := secondsToTimestamp(startSeconds, outStream.TimeBase())
		if pkt.Dts() != astiav.NoPtsValue {
			pkt.SetDts(max64(0, pkt.Dts()-startTimestamp))
		}
		if pkt.Pts() != astiav.NoPtsValue {
			pkt.SetPts(max64(0, pkt.Pts()-startTimestamp))
		}
		if pkt.Dts() != astiav.NoPtsValue && pkt.Pts() != astiav.NoPtsValue && pkt.Pts() < pkt.Dts() {
			pkt.SetPts(pkt.Dts())
		}
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
