package layout

import (
	"math"
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestBaselineShiftExpandsPhysicalLineOnAuthoredSide(t *testing.T) {
	metrics := ParagraphMetrics{AvailableWidthPt: 500, FirstLineAvailableWidthPt: 500, LineHeightScale: 1}
	base := &ir.Run{Text: "base", FontFamily: "Arial", FontSize: 2000}
	raised := &ir.Run{Text: "up", FontFamily: "Arial", FontSize: 2000, BaselineShiftPct: 30}
	p := &ir.Paragraph{Runs: []*ir.Run{base, raised}}

	got := LayoutParagraphLines(p, metrics, 1, "", 0)
	if len(got.Lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(got.Lines))
	}
	ba, _ := paragraphPhysicalBaselineExtentsPt(20, "Arial", metrics)
	if got.Lines[0].AscentPt <= ba {
		t.Fatalf("ascent = %.3f, want > unshifted %.3f", got.Lines[0].AscentPt, ba)
	}
	if math.Abs(got.Lines[0].AscentPt-(ba+6)) > .001 {
		t.Fatalf("ascent = %.3f, want %.3f", got.Lines[0].AscentPt, ba+6)
	}
}

func TestNegativeBaselineShiftExpandsPhysicalDescent(t *testing.T) {
	metrics := ParagraphMetrics{AvailableWidthPt: 500, FirstLineAvailableWidthPt: 500, LineHeightScale: 1}
	lowered := &ir.Run{Text: "down", FontFamily: "Arial", FontSize: 2000, BaselineShiftPct: -25}
	p := &ir.Paragraph{Runs: []*ir.Run{lowered}}

	got := LayoutParagraphLines(p, metrics, 1, "", 0)
	if len(got.Lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(got.Lines))
	}
	_, bd := paragraphPhysicalBaselineExtentsPt(20, "Arial", metrics)
	if math.Abs(got.Lines[0].DescentPt-(bd+5)) > .001 {
		t.Fatalf("descent = %.3f, want %.3f", got.Lines[0].DescentPt, bd+5)
	}
}

func TestBaselineShiftDoesNotMutateAuthoredRunFontSize(t *testing.T) {
	run := &ir.Run{Text: "x", FontFamily: "Arial", FontSize: 1800, BaselineShiftPct: 35}
	p := &ir.Paragraph{Runs: []*ir.Run{run}}
	_ = LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 100, FirstLineAvailableWidthPt: 100, LineHeightScale: 1}, 1, "", 0)
	if run.FontSize != 1800 || run.BaselineShiftPct != 35 {
		t.Fatalf("layout mutated authored run: %+v", run)
	}
}
