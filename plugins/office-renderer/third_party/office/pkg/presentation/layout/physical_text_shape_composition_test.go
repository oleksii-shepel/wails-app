package layout

import (
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestPhysicalTextPlanOwnsOuterShapeComposition(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{X: 12700, Y: 25400, W: 381000, H: 152400}, RotationDeg: 17, FlipH: true}
	items := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(items) != 1 || items[0].Text == nil {
		t.Fatalf("expected one physical text plan")
	}
	plan := items[0].Text
	if plan.FrameBounds.LeftPt != 1 || plan.FrameBounds.TopPt != 2 || plan.FrameBounds.WidthPt != 30 || plan.FrameBounds.HeightPt != 12 {
		t.Fatalf("text frame bounds = %+v", plan.FrameBounds)
	}
	if plan.Transform.RotationDeg != 17 || !plan.Transform.FlipH {
		t.Fatalf("text transform = %+v", plan.Transform)
	}
	if plan.Paint.ZIndex != 1 {
		t.Fatalf("text paint z-index = %d, want 1", plan.Paint.ZIndex)
	}
}
