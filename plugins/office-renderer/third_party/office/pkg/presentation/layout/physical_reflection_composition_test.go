package layout

import (
	"math"
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"testing"
)

func TestPhysicalReflectionOverflowOwnsDistanceScaleAndBlur(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 12700 * 100, H: 12700 * 50}, Effects: drawing.EffectList{Effects: []drawing.Effect{
		drawing.Reflection{Direction: 0, Distance: 6, Blur: 2, ScaleY: .5},
	}}}
	effects := ResolveEffectVisualPlan(shape.Effects)
	got := resolvePhysicalOverflowPlan(shape, effects)
	if math.Abs(got.BottomPt-35) > 1e-9 || got.LeftPt != 4 || got.RightPt != 4 {
		t.Fatalf("reflection overflow = %+v, want left/right=4 bottom=35", got)
	}
}

func TestPhysicalReflectionOverflowRespectsDirection(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 12700 * 80, H: 12700 * 30}, Effects: drawing.EffectList{Effects: []drawing.Effect{
		drawing.Reflection{Direction: 90, Distance: 5, ScaleX: .5},
	}}}
	got := resolvePhysicalOverflowPlan(shape, ResolveEffectVisualPlan(shape.Effects))
	if math.Abs(got.RightPt-45) > 1e-9 || got.LeftPt != 0 || got.BottomPt != 0 {
		t.Fatalf("right reflection overflow = %+v, want right=45", got)
	}
}

func TestPhysicalReflectionOverflowRotatesWithShape(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 12700 * 80, H: 12700 * 30}, RotationDeg: 90, Effects: drawing.EffectList{Effects: []drawing.Effect{
		drawing.Reflection{Direction: 0, Distance: 5, RotateWithShape: true},
	}}}
	got := resolvePhysicalOverflowPlan(shape, ResolveEffectVisualPlan(shape.Effects))
	if math.Abs(got.RightPt-85) > 1e-9 || got.BottomPt != 0 {
		t.Fatalf("rotated reflection overflow = %+v, want right=85", got)
	}
}

func TestReflectionVisualPlanPreservesPhysicalTransformSemantics(t *testing.T) {
	got := ResolveEffectCompositionPlan(drawing.EffectList{Effects: []drawing.Effect{
		drawing.Reflection{Blur: 3, Size: .75, ScaleX: .8, ScaleY: .6, SkewX: 10, SkewY: -5, RotateWithShape: true},
	}}).Reflection
	if got == nil || got.BlurPt != 3 || got.Size != .75 || got.ScaleX != .8 || got.ScaleY != .6 || got.SkewXDeg != 10 || got.SkewYDeg != -5 || !got.RotateWithShape {
		t.Fatalf("reflection plan lost transform semantics: %#v", got)
	}
}
