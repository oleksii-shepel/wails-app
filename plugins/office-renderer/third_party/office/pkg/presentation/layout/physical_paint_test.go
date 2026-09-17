package layout

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestPhysicalPaintPlanOwnsOrderClipAndEffectOverflow(t *testing.T) {
	first := &ir.Shape{Kind: ir.ShapeText, Geometry: "ellipse", Bounds: ir.Rect{W: 1270000, H: 1270000}}
	second := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{W: 1270000, H: 1270000}, Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Glow{Radius: 10}}}}
	got := MaterializePhysicalVisualTree([]*ir.Shape{first, second})
	if len(got) != 2 {
		t.Fatalf("physical visuals = %d, want 2", len(got))
	}
	if got[0].Drawing.Paint.ZIndex != 1 || got[1].Drawing.Paint.ZIndex != 2 {
		t.Fatalf("z order = %d,%d, want 1,2", got[0].Drawing.Paint.ZIndex, got[1].Drawing.Paint.ZIndex)
	}
	if got[0].Drawing.Paint.Clip.Kind != "ellipse" || !got[0].Drawing.Paint.Clip.ClipContent {
		t.Fatalf("ellipse clip = %+v", got[0].Drawing.Paint.Clip)
	}
	if !got[1].Drawing.Paint.OverflowVisible || got[1].Drawing.Paint.Overflow.Empty() {
		t.Fatalf("glow paint overflow = %+v", got[1].Drawing.Paint)
	}
}
