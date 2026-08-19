package app

import "testing"

func TestBuildSegmentRangesFromKeyframes(t *testing.T) {
	ranges := buildSegmentRangesFromKeyframes([]float64{0, 5.873, 11.745, 18.952}, 21.955)
	if len(ranges) != 4 {
		t.Fatalf("expected 4 ranges, got %d", len(ranges))
	}

	assertRange := func(index int, wantStart float64, wantDuration float64) {
		t.Helper()
		got := ranges[index]
		if diff := absFloat(got.StartSeconds - wantStart); diff > 0.001 {
			t.Fatalf("range %d start mismatch: got %.3f want %.3f", index, got.StartSeconds, wantStart)
		}
		if diff := absFloat(got.DurationSeconds - wantDuration); diff > 0.001 {
			t.Fatalf("range %d duration mismatch: got %.3f want %.3f", index, got.DurationSeconds, wantDuration)
		}
	}

	assertRange(0, 0, 5.873)
	assertRange(1, 5.873, 5.872)
	assertRange(2, 11.745, 7.207)
	assertRange(3, 18.952, 3.003)
}

func TestBuildSegmentRangesFromKeyframesPrependsZeroStart(t *testing.T) {
	ranges := buildSegmentRangesFromKeyframes([]float64{5.873, 11.745}, 12)
	if len(ranges) != 3 {
		t.Fatalf("expected 3 ranges, got %d", len(ranges))
	}
	if ranges[0].StartSeconds != 0 {
		t.Fatalf("expected first range to start at zero, got %.3f", ranges[0].StartSeconds)
	}
}

func TestShouldUseKeyframeAlignedSegments(t *testing.T) {
	cases := map[string]bool{
		"sample.mp4":  false,
		"sample.m4v":  false,
		"sample.webm": false,
		"sample.mkv":  true,
		"sample.flv":  true,
		"sample.avi":  true,
		"sample.divx": true,
		"sample.xvid": true,
		"sample.mov":  true,
		"sample.ts":   true,
		"sample.mpg":  true,
		"sample.wmv":  true,
		"sample.asf":  true,
		"sample.ogv":  true,
		"sample.mxf":  true,
	}

	for path, want := range cases {
		if got := shouldUseKeyframeAlignedSegments(path); got != want {
			t.Fatalf("shouldUseKeyframeAlignedSegments(%q) = %v, want %v", path, got, want)
		}
	}
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}