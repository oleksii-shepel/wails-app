package layout

import (
	"math"
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestTextFlowUsesVisibleSlideWidthWhenFrameExtendsPastRightEdge(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{X: 9 * 914400, Y: 0, W: 2 * 914400, H: 914400}}
	frame := TextFrameMetrics{}
	flow := TextFlowMetrics{LineExtentPt: 144, ColumnWidthPt: 144, Columns: 1}
	got := constrainTextFlowToSlide(shape, frame, flow, PhysicalMaterializeOptions{SlideWidthPt: 720, SlideHeightPt: 540})
	if math.Abs(got.LineExtentPt-72) > .001 {
		t.Fatalf("line extent = %.3fpt, want visible 72pt", got.LineExtentPt)
	}
}

func TestTextFlowKeepsAuthoredWidthWhenFrameIsInsideSlide(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{X: 914400, Y: 0, W: 2 * 914400, H: 914400}}
	flow := TextFlowMetrics{LineExtentPt: 144, ColumnWidthPt: 144, Columns: 1}
	got := constrainTextFlowToSlide(shape, TextFrameMetrics{}, flow, PhysicalMaterializeOptions{SlideWidthPt: 720, SlideHeightPt: 540})
	if math.Abs(got.LineExtentPt-144) > .001 {
		t.Fatalf("line extent = %.3fpt, want authored 144pt", got.LineExtentPt)
	}
}

func TestLeadingTabOnHangingContinuationReturnsToTextEdge(t *testing.T) {
	p := &ir.Paragraph{HasMarginLeft: true, MarginLeftEMU: 26 * 12700, HasTextIndent: true, TextIndentEMU: -26 * 12700, Runs: []*ir.Run{{Text: "\t(single complex pole pair)", FontFamily: "Times New Roman", FontSize: 2800}}}
	// The run measures ~295pt at 28pt, and the leading tab advances to the 26pt
	// text edge, so the line needs ~321pt to stay unbroken and show that the tab
	// itself never contributes a break.
	m := ResolveParagraphMetrics(p, 340, 28, 1)
	got := LayoutParagraphLines(p, m, 1, "", 0)
	if len(got.Lines) != 1 {
		t.Fatalf("leading continuation tab produced %d lines, want 1", len(got.Lines))
	}
	if len(got.Lines[0].Fragments) == 0 || got.Lines[0].Fragments[0].Text != "\t" || got.Lines[0].Fragments[0].WidthPt > 26.01 {
		t.Fatalf("leading tab = %+v, want <= 26pt return-to-text-edge advance", got.Lines[0].Fragments)
	}
}
