package layout

import (
	"math"
	"testing"
)

func TestLineSpansForBandPreservesBothSidesOfDrawing(t *testing.T) {
	spans := lineSpansForBand(0, 100, 0, 10, []lineExclusion{{top: 0, bottom: 20, left: 30, right: 60}})
	if len(spans) != 2 {
		t.Fatalf("spans = %#v, want two text regions", spans)
	}
	if !almostSame(spans[0].Left, 0) || !almostSame(spans[0].Right, 30) || !almostSame(spans[1].Left, 60) || !almostSame(spans[1].Right, 100) {
		t.Fatalf("spans = %#v, want [0,30] and [60,100]", spans)
	}
}

func TestLineSpansForBandIntersectsMultipleFloats(t *testing.T) {
	exclusions := []lineExclusion{{top: 0, bottom: 20, left: 20, right: 35}, {top: 0, bottom: 20, left: 65, right: 80}}
	spans := lineSpansForBand(0, 100, 0, 10, exclusions)
	want := []lineSpan{{0, 20}, {35, 65}, {80, 100}}
	if len(spans) != len(want) {
		t.Fatalf("spans = %#v, want %#v", spans, want)
	}
	for i := range want {
		if !almostSame(spans[i].Left, want[i].Left) || !almostSame(spans[i].Right, want[i].Right) {
			t.Fatalf("span %d = %#v, want %#v", i, spans[i], want[i])
		}
	}
}

func TestLineSpansHonorsWrapSideLargest(t *testing.T) {
	spans := lineSpansForBand(0, 100, 0, 10, []lineExclusion{{top: 0, bottom: 20, left: 15, right: 40, wrapSide: "largest"}})
	if len(spans) != 1 || !almostSame(spans[0].Left, 40) || !almostSame(spans[0].Right, 100) {
		t.Fatalf("spans = %#v, want [40,100]", spans)
	}
}

func TestExactPolygonIntervalsAtBandCapturesThinRotatedTip(t *testing.T) {
	polygon := [][2]float64{{0, 0.01}, {50, 5}, {100, 0.01}, {50, -5}}
	spans := exactPolygonIntervalsAtBand(polygon, 0, 10)
	if len(spans) == 0 {
		t.Fatal("no polygon projection")
	}
	if spans[0].Left > 0.01 || spans[len(spans)-1].Right < 99.99 {
		t.Fatalf("projection = %#v, want complete tip", spans)
	}
}

func TestExactPolygonIntervalsAtBandPreservesConcaveGap(t *testing.T) {
	polygon := [][2]float64{{10, 0}, {30, 0}, {30, 8}, {70, 8}, {70, 0}, {90, 0}, {90, 20}, {10, 20}}
	spans := exactPolygonIntervalsAtBand(polygon, .5, 5)
	if len(spans) != 2 {
		t.Fatalf("polygon spans = %#v, want two lobes", spans)
	}
	if spans[0].Right > 30.001 || spans[1].Left < 69.999 {
		t.Fatalf("polygon spans = %#v, center gap lost", spans)
	}
}

func TestLineSpansApplyTightWrapPaddingAfterContourProjection(t *testing.T) {
	polygon := [][2]float64{{20, 0}, {40, 0}, {40, 20}, {20, 20}}
	e := lineExclusion{top: 0, bottom: 20, left: 20, right: 40, polygon: polygon, padLeft: 3, padRight: 7}
	spans := lineSpansForBand(0, 100, 5, 10, []lineExclusion{e})
	if len(spans) != 2 {
		t.Fatalf("spans = %#v", spans)
	}
	if math.Abs(spans[0].Right-17) > .001 || math.Abs(spans[1].Left-47) > .001 {
		t.Fatalf("spans = %#v, want blocked 17..47", spans)
	}
}

func TestParagraphLinePlanUsesFirstAndContinuationFrames(t *testing.T) {
	plan := planParagraphLineBands(10, 90, 0, 100, 50, []float64{10, 12, 8}, 10, nil)
	if len(plan.Bands) != 3 {
		t.Fatalf("bands=%d", len(plan.Bands))
	}
	want := []struct{ top, bottom, left, right float64 }{{50, 60, 10, 90}, {60, 72, 0, 100}, {72, 80, 0, 100}}
	for i, w := range want {
		b := plan.Bands[i]
		if !almostSame(b.Top, w.top) || !almostSame(b.Bottom, w.bottom) || len(b.Spans) != 1 || !almostSame(b.Spans[0].Left, w.left) || !almostSame(b.Spans[0].Right, w.right) {
			t.Fatalf("band %d = %#v", i, b)
		}
	}
	if !almostSame(plan.TotalHeight(), 30) {
		t.Fatalf("height=%v", plan.TotalHeight())
	}
}

func TestParagraphLinePlanPreservesMultipleRegionsForPagination(t *testing.T) {
	e := lineExclusion{top: 0, bottom: 30, left: 30, right: 60}
	plan := planParagraphLineBands(0, 100, 0, 100, 0, []float64{10, 10, 10, 10}, 10, []lineExclusion{e})
	if len(plan.Bands) != 4 {
		t.Fatalf("bands=%d", len(plan.Bands))
	}
	for i := 0; i < 3; i++ {
		if len(plan.Bands[i].Spans) != 2 {
			t.Fatalf("band %d spans=%#v, want both sides", i, plan.Bands[i].Spans)
		}
	}
	if len(plan.Bands[3].Spans) != 1 || !almostSame(plan.Bands[3].Spans[0].Width(), 100) {
		t.Fatalf("last band=%#v", plan.Bands[3])
	}
}

func TestParagraphLinePlanFullWidthExclusionProducesBlockedBand(t *testing.T) {
	plan := planParagraphLineBands(0, 100, 0, 100, 0, []float64{10, 10}, 10, []lineExclusion{{top: 0, bottom: 10, fullWidth: true}})
	if len(plan.Bands[0].Spans) != 0 {
		t.Fatalf("blocked band=%#v", plan.Bands[0])
	}
	if len(plan.Bands[1].Spans) != 1 || !almostSame(plan.Bands[1].Spans[0].Width(), 100) {
		t.Fatalf("second band=%#v", plan.Bands[1])
	}
}

func TestResolveLineBandMarksOneSidedFloatConstraint(t *testing.T) {
	band := resolveLineBand(0, 100, 0, 10, []lineExclusion{{
		left: 0, right: 35, top: 0, bottom: 20, wrapSide: "largest",
	}})
	if !band.Constrained {
		t.Fatal("one-sided float exclusion must mark the physical line constrained")
	}
	if len(band.Spans) != 1 || math.Abs(band.Spans[0].Left-35) > 0.01 || math.Abs(band.Spans[0].Right-100) > 0.01 {
		t.Fatalf("spans = %#v, want one-sided 35..100", band.Spans)
	}

	plain := resolveLineBand(0, 100, 0, 10, nil)
	if plain.Constrained {
		t.Fatal("unexcluded line must remain browser-flow eligible")
	}
}
