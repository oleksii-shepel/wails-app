package layout

import (
	"math"
	ir "office/pkg/presentation/ir"
	"testing"
)

func TestPercentageParagraphSpacingUsesPhysicalBoundaryLineHeight(t *testing.T) {
	p := &ir.Paragraph{
		SpaceBeforePct: 50000,
		SpaceAfterPct:  100000,
		Runs: []*ir.Run{
			{Text: "small ", FontSize: 1000, FontFamily: "Arial"},
			{Text: "BIG", FontSize: 3000, FontFamily: "Arial"},
		},
	}
	got := ResolveParagraphVisualPlan(p, 500, 1, 1)
	if !got.LinesMeasured || len(got.Lines) != 1 {
		t.Fatalf("lines = %#v", got.Lines)
	}
	h := got.Lines[0].HeightPt
	if math.Abs(got.Metrics.SpaceBeforePt-h*.5) > .001 {
		t.Fatalf("before = %.3f, want %.3f", got.Metrics.SpaceBeforePt, h*.5)
	}
	if math.Abs(got.Metrics.SpaceAfterPt-h) > .001 {
		t.Fatalf("after = %.3f, want %.3f", got.Metrics.SpaceAfterPt, h)
	}
}

func TestPercentageParagraphSpacingUsesFirstAndLastWrappedLineIndependently(t *testing.T) {
	p := &ir.Paragraph{
		SpaceBeforePct: 100000,
		SpaceAfterPct:  100000,
		Runs: []*ir.Run{
			{Text: "BIG ", FontSize: 3000, FontFamily: "Arial"},
			{Text: "small small small", FontSize: 1000, FontFamily: "Arial"},
		},
	}
	got := ResolveParagraphVisualPlan(p, 70, 1, 1)
	if !got.LinesMeasured || len(got.Lines) < 2 {
		t.Fatalf("expected wrapped lines, got %#v", got.Lines)
	}
	first, last := got.Lines[0].HeightPt, got.Lines[len(got.Lines)-1].HeightPt
	if first <= last {
		t.Fatalf("fixture must have taller first line: first %.3f last %.3f", first, last)
	}
	if math.Abs(got.Metrics.SpaceBeforePt-first) > .001 {
		t.Fatalf("before = %.3f, want first %.3f", got.Metrics.SpaceBeforePt, first)
	}
	if math.Abs(got.Metrics.SpaceAfterPt-last) > .001 {
		t.Fatalf("after = %.3f, want last %.3f", got.Metrics.SpaceAfterPt, last)
	}
}
