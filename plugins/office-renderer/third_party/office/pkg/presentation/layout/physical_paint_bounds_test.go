package layout

import (
	"math"
	"testing"

	"office/pkg/presentation/ir"
)

func TestPhysicalPaintBoundsIncludeEffectOverflow(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{X: 12700, Y: 25400, W: 1270000, H: 635000}}
	overflow := PhysicalOverflowPlan{LeftPt: 10, TopPt: 20, RightPt: 30, BottomPt: 40}
	got := resolvePhysicalPaintBounds(shape, overflow, PhysicalTransformPlan{})
	if got.LeftPt != -9 || got.TopPt != -18 || got.WidthPt != 140 || got.HeightPt != 110 {
		t.Fatalf("paint bounds = %+v, want left=-9 top=-18 width=140 height=110", got)
	}
}

func TestPhysicalPaintBoundsRotateExpandedEffectEnvelope(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 1270000, H: 635000}}
	overflow := PhysicalOverflowPlan{LeftPt: 10, TopPt: 20, RightPt: 30, BottomPt: 40}
	got := resolvePhysicalPaintBounds(shape, overflow, PhysicalTransformPlan{RotationDeg: 90})
	if math.Abs(got.WidthPt-110) > 1e-9 || math.Abs(got.HeightPt-140) > 1e-9 {
		t.Fatalf("rotated paint bounds = %+v, want width=110 height=140", got)
	}
}

func TestMaterializePhysicalVisualTreeCarriesPaintBounds(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{X: 12700, Y: 25400, W: 1270000, H: 635000}}
	got := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(got) != 1 || got[0].Drawing.Paint.Bounds.Empty() {
		t.Fatalf("physical paint bounds missing: %+v", got)
	}
	if got[0].Drawing.Paint.Bounds.LeftPt != 1 || got[0].Drawing.Paint.Bounds.TopPt != 2 || got[0].Drawing.Paint.Bounds.WidthPt != 100 || got[0].Drawing.Paint.Bounds.HeightPt != 50 {
		t.Fatalf("physical paint bounds = %+v", got[0].Drawing.Paint.Bounds)
	}
}
