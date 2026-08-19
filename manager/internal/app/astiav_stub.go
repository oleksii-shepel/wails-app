package app

import (
	"context"
	"fmt"
	"io"
)

const astiavUnavailableMessage = "video transcoding and frame extraction are not available in this build"

type segmentedVideoRange struct {
	StartSeconds    float64
	DurationSeconds float64
}

func extractVideoFrame(ctx context.Context, inputPath string, targetFormat string, frameSeconds float64) ([]byte, error) {
	return nil, fmt.Errorf("%s", astiavUnavailableMessage)
}

func probeVideo(nativePath string) (*ffprobeOutput, error) {
	return nil, fmt.Errorf("%s", astiavUnavailableMessage)
}

func startAstiavTranscode(ctx context.Context, nativePath string, startSeconds float64, windowSeconds float64, playbackMode VideoPlaybackMode) (io.ReadCloser, error) {
	return nil, fmt.Errorf("%s", astiavUnavailableMessage)
}

func startAstiavTranscodeReader(ctx context.Context, r io.Reader, playbackMode VideoPlaybackMode) (io.ReadCloser, error) {
	return nil, fmt.Errorf("%s", astiavUnavailableMessage)
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
	return nil, fmt.Errorf("%s", astiavUnavailableMessage)
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
	return nil, fmt.Errorf("%s", astiavUnavailableMessage)
}

func estimateMediaDurationSeconds(ctx context.Context, nativePath string) (float64, error) {
	return 0, fmt.Errorf("%s", astiavUnavailableMessage)
}

func buildSegmentedVideoFragment(ctx context.Context, nativePath string, startSeconds float64, durationSeconds float64) ([]byte, []byte, error) {
	return nil, nil, fmt.Errorf("%s", astiavUnavailableMessage)
}