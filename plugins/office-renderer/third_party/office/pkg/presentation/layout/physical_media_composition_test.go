package layout

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestPhysicalMediaPlanOwnsFrameTransformEffectsAndPaint(t *testing.T) {
	shape := &ir.Shape{
		Kind:        ir.ShapeMedia,
		Bounds:      ir.Rect{X: 12700, Y: 25400, W: 127000, H: 63500},
		RotationDeg: 27, FlipH: true,
		Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Glow{Radius: 3}}},
	}
	visuals := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(visuals) != 1 || visuals[0].Media == nil {
		t.Fatal("media physical plan missing")
	}
	got := visuals[0].Media
	if got.FrameBounds.LeftPt != 1 || got.FrameBounds.TopPt != 2 || got.FrameBounds.WidthPt != 10 || got.FrameBounds.HeightPt != 5 {
		t.Fatalf("media frame = %+v", got.FrameBounds)
	}
	if got.Transform.RotationDeg != 27 || !got.Transform.FlipH || got.Transform.FlipV {
		t.Fatalf("media transform = %+v", got.Transform)
	}
	if !got.Effects.HasEffects {
		t.Fatal("media effects were not materialized")
	}
	if got.Paint.ZIndex != 1 || !got.Paint.OverflowVisible {
		t.Fatalf("media paint = %+v", got.Paint)
	}
}

func TestPhysicalImagePlanOwnsContainerFrameAndPaint(t *testing.T) {
	shape := &ir.Shape{
		Kind:    ir.ShapeImage,
		Bounds:  ir.Rect{X: 25400, Y: 12700, W: 254000, H: 127000},
		Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Glow{Radius: 2}}},
	}
	visuals := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(visuals) != 1 || visuals[0].Image == nil {
		t.Fatal("image physical plan missing")
	}
	got := visuals[0].Image
	if got.Composition.FrameBounds.LeftPt != 2 || got.Composition.FrameBounds.TopPt != 1 || got.Composition.FrameBounds.WidthPt != 20 || got.Composition.FrameBounds.HeightPt != 10 {
		t.Fatalf("image container = %+v", got.Composition.FrameBounds)
	}
	if got.Composition.Paint.ZIndex != 1 || !got.Composition.Paint.OverflowVisible {
		t.Fatalf("image paint = %+v", got.Composition.Paint)
	}
}
