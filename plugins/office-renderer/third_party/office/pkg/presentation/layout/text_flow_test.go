package layout

import (
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
)

func TestResolveTextFlowMetricsOwnsColumnsAndFlowWidth(t *testing.T) {
	shape := &ir.Shape{
		Bounds: ir.Rect{W: 4572000}, // 360pt
		TextBodyProps: drawing.TextBodyProperties{
			Columns:          2,
			ColumnSpacingEMU: 457200, // 36pt
		},
	}
	frame := TextFrameMetrics{InsetLeftPt: 7.2, InsetRightPt: 7.2}
	got := ResolveTextFlowMetrics(shape, frame)
	if got.Columns != 2 || math.Abs(got.ColumnGapPt-36) > 0.001 {
		t.Fatalf("columns = %d gap = %.4f, want 2 / 36pt", got.Columns, got.ColumnGapPt)
	}
	want := (360.0 - 14.4 - 36.0) / 2
	if math.Abs(got.ColumnWidthPt-want) > 0.001 || math.Abs(got.LineExtentPt-want) > 0.001 {
		t.Fatalf("column/line width = %.4f / %.4f, want %.4f", got.ColumnWidthPt, got.LineExtentPt, want)
	}
}

func TestResolveTextFlowMetricsOwnsVerticalRotationAndNoWrap(t *testing.T) {
	shape := &ir.Shape{
		Bounds: ir.Rect{W: 1828800},
		TextBodyProps: drawing.TextBodyProperties{
			Rot:       900000,
			Vert:      "vert",
			Wrap:      "none",
			AnchorCtr: true,
		},
	}
	got := ResolveTextFlowMetrics(shape, TextFrameMetrics{})
	if math.Abs(got.RotationDeg-15) > 0.001 {
		t.Fatalf("rotation = %.4f, want 15", got.RotationDeg)
	}
	if got.WritingMode != "vertical-rl" || !got.AnchorCenter || !got.NoWrap {
		t.Fatalf("flow = %+v", got)
	}
}

func TestResolveTextFlowMetricsQuarterTurnUsesRotatedPhysicalExtents(t *testing.T) {
	shape := &ir.Shape{
		Bounds:        ir.Rect{W: 12700 * 200, H: 12700 * 100},
		HasTextInsets: true,
		TextBodyProps: drawing.TextBodyProperties{Rot: 90 * 60000},
	}
	got := ResolveTextFlowMetrics(shape, ResolveTextFrameMetrics(shape))
	if got.LineExtentPt != 100 || got.ProgressionExtentPt != 200 {
		t.Fatalf("quarter-turn extents = %.2f x %.2f, want 100 x 200", got.LineExtentPt, got.ProgressionExtentPt)
	}
}

func TestAnchorCtrOwnsRotationOriginNotParagraphAlignment(t *testing.T) {
	shape := &ir.Shape{TextBodyProps: drawing.TextBodyProperties{AnchorCtr: true}}
	got := ResolveTextFlowMetrics(shape, ResolveTextFrameMetrics(shape))
	if !got.AnchorCenter || got.TransformOrigin != "center center" {
		t.Fatalf("anchorCtr flow = %+v", got)
	}
}


func TestClass12QuarterTurnWrapsAgainstPhysicalHeight(t *testing.T) {
	shape := &ir.Shape{
		Bounds: ir.Rect{W: 2540000, H: 1270000}, // 200pt x 100pt
		TextBodyProps: drawing.TextBodyProperties{Rot: 90 * 60000},
	}
	frame := ResolveTextFrameMetrics(shape)
	flow := ResolveTextFlowMetrics(shape, frame)
	want := TextFrameContentHeightPt(shape, frame)
	if math.Abs(flow.LineExtentPt-want) > .01 {
		t.Fatalf("quarter-turn line extent = %.3f, want physical height %.3f", flow.LineExtentPt, want)
	}
}
