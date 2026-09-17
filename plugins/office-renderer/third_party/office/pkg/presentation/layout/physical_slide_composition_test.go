package layout

import (
	ir "office/pkg/presentation/ir"
	"testing"
)

func TestMaterializePhysicalSlideOwnsBackgroundAndSharedPaintOrder(t *testing.T) {
	content := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{W: 127000, H: 63500}}
	footer := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{W: 127000, H: 63500}}
	number := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{W: 127000, H: 63500}}
	slide := &ir.Slide{BgColor: "112233", Shapes: []*ir.Shape{content}, Footer: footer, SlideNumber: number}
	plan := MaterializePhysicalSlide(slide)
	if plan.Background.Color != "112233" || plan.Background.ZIndex != 0 {
		t.Fatalf("background plan = %+v", plan.Background)
	}
	if len(plan.Content) != 1 || plan.Content[0].Drawing.Paint.ZIndex != 1 {
		t.Fatalf("content paint order not materialized: %+v", plan.Content)
	}
	if len(plan.Decorations) != 1 || plan.Decorations[0].Drawing.Paint.ZIndex != 2 {
		t.Fatalf("decoration paint order not shared with content: %+v", plan.Decorations)
	}
	if plan.SlideNumber == nil || plan.SlideNumber.Drawing.Paint.ZIndex != 3 {
		t.Fatalf("slide number paint order not shared: %+v", plan.SlideNumber)
	}
}
