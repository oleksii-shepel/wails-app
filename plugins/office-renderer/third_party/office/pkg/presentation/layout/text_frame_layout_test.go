package layout

import (
	"math"
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestLayoutTextFrameOwnsContentHeightAndCenterOffset(t *testing.T) {
	shape := &ir.Shape{
		Bounds:             ir.Rect{W: 3657600, H: 1270000}, // 288pt x 100pt
		TextAnchor:         "ctr",
		TextInsetTopEMU:    45720,
		TextInsetBottomEMU: 45720,
		Paragraphs:         []*ir.Paragraph{{Runs: []*ir.Run{{Text: "one line", FontSize: 1200}}}},
	}
	frame := ResolveTextFrameMetrics(shape)
	got := LayoutTextFrame(shape, frame)
	if !got.Measured {
		t.Fatal("text frame was not measured")
	}
	if got.ContentHeightPt <= 0 || got.RequiredHeightPt <= got.ContentHeightPt {
		t.Fatalf("invalid vertical metrics: %+v", got)
	}
	wantOffset := (got.AvailableHeightPt - got.ContentHeightPt) / 2
	if math.Abs(got.VerticalOffsetPt-wantOffset) > 0.001 {
		t.Fatalf("vertical offset = %.4f, want %.4f", got.VerticalOffsetPt, wantOffset)
	}
}

func TestLayoutTextFrameReportsOverflow(t *testing.T) {
	shape := &ir.Shape{
		Bounds:     ir.Rect{W: 3657600, H: 127000}, // 288pt x 10pt
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "one line", FontSize: 1200}}}},
	}
	got := LayoutTextFrame(shape, ResolveTextFrameMetrics(shape))
	if !got.Measured || got.OverflowPt <= 0 {
		t.Fatalf("expected measured overflow, got %+v", got)
	}
}

func TestLayoutTextFrameDeclinesSpecialRuns(t *testing.T) {
	shape := &ir.Shape{
		Bounds:     ir.Rect{W: 3657600, H: 1270000},
		Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{MathML: "<math/>"}}}},
	}
	if got := LayoutTextFrame(shape, ResolveTextFrameMetrics(shape)); got.Measured {
		t.Fatalf("special run must stay on semantic fallback: %+v", got)
	}
}
