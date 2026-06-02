//go:build cgo && (windows || darwin || linux)

package app

import (
	"bytes"
	"context"
	"fmt"

	"github.com/asticode/go-astiav"
)

type segmentedVideoRange struct {
	StartSeconds    float64
	DurationSeconds float64
}

func buildFixedSegmentRanges(durationSeconds float64) []segmentedVideoRange {
	totalSegments := int(durationSeconds / segmentedPreviewSegmentSeconds)
	if durationSeconds > float64(totalSegments)*segmentedPreviewSegmentSeconds {
		totalSegments++
	}
	if totalSegments <= 0 {
		totalSegments = 1
	}

	ranges := make([]segmentedVideoRange, 0, totalSegments)
	for index := 0; index < totalSegments; index++ {
		startSeconds := float64(index) * segmentedPreviewSegmentSeconds
		segmentDuration := segmentedPreviewSegmentSeconds
		if remaining := durationSeconds - startSeconds; remaining > 0 && remaining < segmentDuration {
			segmentDuration = remaining
		}
		if segmentDuration <= 0 {
			segmentDuration = segmentedPreviewSegmentSeconds
		}

		ranges = append(ranges, segmentedVideoRange{
			StartSeconds:    startSeconds,
			DurationSeconds: segmentDuration,
		})
	}

	return ranges
}

func buildKeyframeAlignedSegmentRanges(ctx context.Context, nativePath string, durationSeconds float64) ([]segmentedVideoRange, error) {
	keyframes, err := collectVideoKeyframeTimes(ctx, nativePath)
	if err != nil {
		return nil, err
	}
	return buildSegmentRangesFromKeyframes(keyframes, durationSeconds), nil
}

func buildSegmentRangesFromKeyframes(keyframes []float64, durationSeconds float64) []segmentedVideoRange {
	if durationSeconds <= 0 {
		return buildFixedSegmentRanges(durationSeconds)
	}

	starts := make([]float64, 0, len(keyframes)+1)
	starts = append(starts, 0)
	for _, keyframeTime := range keyframes {
		if keyframeTime <= 0.001 || keyframeTime >= durationSeconds-0.001 {
			continue
		}
		if keyframeTime-starts[len(starts)-1] <= 0.001 {
			continue
		}
		starts = append(starts, keyframeTime)
	}

	ranges := make([]segmentedVideoRange, 0, len(starts))
	for index, startSeconds := range starts {
		endSeconds := durationSeconds
		if index+1 < len(starts) {
			endSeconds = starts[index+1]
		}
		segmentDuration := endSeconds - startSeconds
		if segmentDuration <= 0.001 {
			continue
		}

		ranges = append(ranges, segmentedVideoRange{
			StartSeconds:    startSeconds,
			DurationSeconds: segmentDuration,
		})
	}

	if len(ranges) == 0 {
		return buildFixedSegmentRanges(durationSeconds)
	}

	return ranges
}

func collectVideoKeyframeTimes(ctx context.Context, nativePath string) ([]float64, error) {
	inputFormatContext := astiav.AllocFormatContext()
	if inputFormatContext == nil {
		return nil, fmt.Errorf("failed allocating input format context")
	}
	defer inputFormatContext.Free()

	if err := inputFormatContext.OpenInput(nativePath, nil, nil); err != nil {
		return nil, fmt.Errorf("failed opening input: %w", err)
	}
	defer inputFormatContext.CloseInput()

	if err := inputFormatContext.FindStreamInfo(nil); err != nil {
		return nil, fmt.Errorf("failed finding stream info: %w", err)
	}

	videoStreamIndex := -1
	for index := 0; index < inputFormatContext.NbStreams(); index++ {
		s := inputFormatContext.Streams()[index]
		if s.CodecParameters().MediaType() == astiav.MediaTypeVideo && !s.DispositionFlags().Has(astiavDispositionFlagAttachedPic) {
			videoStreamIndex = index
			break
		}
	}
	if videoStreamIndex < 0 {
		return nil, fmt.Errorf("no video stream found")
	}

	pkt := astiav.AllocPacket()
	if pkt == nil {
		return nil, fmt.Errorf("failed allocating packet")
	}
	defer pkt.Free()

	keyframes := make([]float64, 0, 16)
	videoStream := inputFormatContext.Streams()[videoStreamIndex]

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if err := inputFormatContext.ReadFrame(pkt); err != nil {
			break
		}

		if pkt.StreamIndex() != videoStreamIndex || !pkt.Flags().Has(astiavPacketFlagKey) {
			pkt.Unref()
			continue
		}

		keyframeTime, ok := packetTimestampSeconds(pkt, videoStream)
		pkt.Unref()
		if !ok {
			continue
		}

		if len(keyframes) == 0 || keyframeTime-keyframes[len(keyframes)-1] > 0.001 {
			keyframes = append(keyframes, keyframeTime)
		}
	}

	if len(keyframes) == 0 {
		return []float64{0}, nil
	}
	if keyframes[0] > 0.001 {
		return append([]float64{0}, keyframes...), nil
	}

	keyframes[0] = 0
	return keyframes, nil
}

func packetTimestampSeconds(pkt *astiav.Packet, stream *astiav.Stream) (float64, bool) {
	timestamp := pkt.Pts()
	if timestamp == astiavNoPtsValue {
		timestamp = pkt.Dts()
	}
	if timestamp == astiavNoPtsValue {
		return 0, false
	}

	timeBase := stream.TimeBase()
	if timeBase.Den() == 0 {
		return 0, false
	}

	return float64(timestamp) * float64(timeBase.Num()) / float64(timeBase.Den()), true
}

func packetEndTimestampSeconds(pkt *astiav.Packet, stream *astiav.Stream) (float64, bool) {
	startSeconds, ok := packetTimestampSeconds(pkt, stream)
	if !ok {
		return 0, false
	}

	packetDuration := pkt.Duration()
	if packetDuration <= 0 {
		return startSeconds, true
	}

	timeBase := stream.TimeBase()
	if timeBase.Den() == 0 {
		return startSeconds, true
	}

	durationSeconds := float64(packetDuration) * float64(timeBase.Num()) / float64(timeBase.Den())
	return startSeconds + durationSeconds, true
}

func estimateMediaDurationSeconds(ctx context.Context, nativePath string) (float64, error) {
	inputFormatContext := astiav.AllocFormatContext()
	if inputFormatContext == nil {
		return 0, fmt.Errorf("failed allocating input format context")
	}
	defer inputFormatContext.Free()

	if err := inputFormatContext.OpenInput(nativePath, nil, nil); err != nil {
		return 0, fmt.Errorf("failed opening input: %w", err)
	}
	defer inputFormatContext.CloseInput()

	if err := inputFormatContext.FindStreamInfo(nil); err != nil {
		return 0, fmt.Errorf("failed finding stream info: %w", err)
	}

	pkt := astiav.AllocPacket()
	if pkt == nil {
		return 0, fmt.Errorf("failed allocating packet")
	}
	defer pkt.Free()

	streams := inputFormatContext.Streams()
	maxEndSeconds := 0.0

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}

		if err := inputFormatContext.ReadFrame(pkt); err != nil {
			break
		}

		streamIndex := pkt.StreamIndex()
		if streamIndex < 0 || streamIndex >= len(streams) {
			pkt.Unref()
			continue
		}

		endSeconds, ok := packetEndTimestampSeconds(pkt, streams[streamIndex])
		pkt.Unref()
		if !ok {
			continue
		}
		if endSeconds > maxEndSeconds {
			maxEndSeconds = endSeconds
		}
	}

	if maxEndSeconds <= 0 {
		return 0, fmt.Errorf("failed estimating media duration: no packet timestamps")
	}

	return maxEndSeconds, nil
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

	if startSeconds > 0 {
		seekTarget := int64(startSeconds * float64(astiavTimeBase))
		if err := inputFormatContext.SeekFrame(-1, seekTarget, astiav.NewSeekFlags(astiavSeekFlagBackward)); err != nil {
			// Non-fatal seek error
		}
	}

	outputFormatContext, err := astiav.AllocOutputFormatContext(nil, "mp4", "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed allocating output format context: %w", err)
	}
	defer outputFormatContext.Free()

	var buf bytes.Buffer
	ioContext, err := astiav.AllocIOContext(
		4096,
		true,
		nil,
		nil,
		func(b []byte) (int, error) {
			return buf.Write(b)
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed allocating custom IO context: %w", err)
	}
	defer ioContext.Free()

	outputFormatContext.SetPb(ioContext)

	streamMapping := make(map[int]int)
	outputStreamIdx := 0
	selectedVideoStreamIndex := -1

	streamsArr := inputFormatContext.Streams()
	for i := 0; i < inputFormatContext.NbStreams(); i++ {
		inStream := streamsArr[i]
		codecParams := inStream.CodecParameters()
		if !shouldIncludePreviewStream(inStream, &selectedVideoStreamIndex) {
			streamMapping[i] = -1
			continue
		}

		outStream := outputFormatContext.NewStream(nil)
		if outStream == nil {
			return nil, nil, fmt.Errorf("failed allocating output stream")
		}

		if err := codecParams.Copy(outStream.CodecParameters()); err != nil {
			return nil, nil, fmt.Errorf("failed copying codec parameters: %w", err)
		}
		outStream.CodecParameters().SetCodecTag(0)

		streamMapping[i] = outputStreamIdx
		outputStreamIdx++
	}

	options := astiav.NewDictionary()
	defer options.Free()
	options.Set("movflags", "frag_keyframe+empty_moov+default_base_moof+delay_moov", 0)

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
		if err := inputFormatContext.ReadFrame(pkt); err != nil {
			break
		}

		inStreamIdx := pkt.StreamIndex()
		outStreamIdx, ok := streamMapping[inStreamIdx]
		if !ok || outStreamIdx < 0 {
			pkt.Unref()
			continue
		}

		inStream := inputFormatContext.Streams()[inStreamIdx]
		outStream := outputFormatContext.Streams()[outStreamIdx]

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

		pkt.SetStreamIndex(outStreamIdx)
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

func shouldIncludePreviewStream(stream *astiav.Stream, selectedVideoStreamIndex *int) bool {
	codecParams := stream.CodecParameters()
	switch codecParams.MediaType() {
	case astiav.MediaTypeAudio:
		return true
	case astiav.MediaTypeVideo:
		if stream.DispositionFlags().Has(astiavDispositionFlagAttachedPic) {
			return false
		}
		if *selectedVideoStreamIndex >= 0 {
			return false
		}
		*selectedVideoStreamIndex = stream.Index()
		return true
	default:
		return false
	}
}
