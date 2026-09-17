package layout

import (
	"math"
	ir "office/pkg/presentation/ir"
	"testing"
)

func TestAbsoluteLineSpacingOwnsPhysicalLineBoxHeight(t *testing.T) {
	p := &ir.Paragraph{HasLineSpacing: true, LineSpacingPt: 1800, Runs: []*ir.Run{{Text: "one two", FontSize: 1200, FontFamily: "Arial"}}}
	m := ResolveParagraphMetrics(p, 200, 12, 1)
	if math.Abs(m.LineHeightPt-18) > .001 {
		t.Fatalf("absolute line height = %.3f, want 18pt", m.LineHeightPt)
	}
	got := LayoutParagraphLines(p, m, 1, "", 0)
	if len(got.Lines) != 1 || math.Abs(got.Lines[0].HeightPt-18) > .001 {
		t.Fatalf("physical line = %#v, want one 18pt line", got.Lines)
	}
}

func TestAbsoluteLineSpacingParticipatesInFrameVerticalPlacement(t *testing.T) {
	p := &ir.Paragraph{HasLineSpacing: true, LineSpacingPt: 2000, Runs: []*ir.Run{{Text: "A", FontSize: 1000}}}
	shape := &ir.Shape{Bounds: ir.Rect{W: 1270000, H: 1270000}, Paragraphs: []*ir.Paragraph{p}}
	frame := TextFrameMetrics{FontScale: 1, LineScale: 1, VerticalJustify: "center"}
	pp := ResolveParagraphVisualPlan(p, 100, 1, 1)
	got := LayoutTextFrameFromParagraphPlans(shape, frame, []ParagraphVisualPlan{pp})
	if math.Abs(got.ContentHeightPt-20) > .001 {
		t.Fatalf("content height = %.3f, want 20pt", got.ContentHeightPt)
	}
	if got.VerticalOffsetPt <= 0 {
		t.Fatalf("centered absolute line spacing should produce positive offset: %#v", got)
	}
}
