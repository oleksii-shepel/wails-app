package layout

import (
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestResolveFillVisualPlanMaterializesGradientSemantics(t *testing.T) {
	fill := drawing.GradientFill{
		GradientType:    drawing.GradientTypeLinear,
		Angle:           37.5,
		Flip:            "xy",
		RotateWithShape: true,
		Stops: []drawing.GradientStop{
			{Position: -0.2, Color: "FF0000", Alpha: .25},
			{Position: .4, Color: "00FF00", Alpha: .5},
			{Position: 1.2, Color: "0000FF", Alpha: 1},
		},
	}
	got := ResolveFillVisualPlan(fill, "", nil)
	if got.Gradient == nil {
		t.Fatal("gradient physical plan = nil")
	}
	if got.Gradient.Kind != drawing.GradientTypeLinear || got.Gradient.AngleDeg != 37.5 || !got.Gradient.FlipH || !got.Gradient.FlipV || !got.Gradient.RotateWithShape {
		t.Fatalf("gradient physical plan = %+v", got.Gradient)
	}
	if len(got.Gradient.Stops) != 3 || got.Gradient.Stops[0].Position != 0 || got.Gradient.Stops[2].Position != 1 {
		t.Fatalf("normalized stops = %+v", got.Gradient.Stops)
	}
}

func TestResolveShapeVisualPlanMaterializesPhysicalEffects(t *testing.T) {
	shape := &ir.Shape{Effects: drawing.EffectList{Effects: []drawing.Effect{
		drawing.Shadow{Color: "112233", Alpha: .4, Blur: 3, Distance: 10, Angle: 90, RotateWithShape: true},
		drawing.InnerShadow{Color: "445566", Alpha: .5, Blur: 2, Distance: 4, Angle: 0},
		drawing.Glow{Color: "778899", Alpha: .3, Radius: 5},
		drawing.Blur{Radius: 6, Grow: true},
		drawing.SoftEdge{Radius: 2.5},
	}}}
	got := ResolveShapeVisualPlan(shape).Effects
	if len(got.Shadows) != 2 || len(got.Glows) != 1 || len(got.Blurs) != 1 || len(got.SoftEdges) != 1 {
		t.Fatalf("physical effects = %+v", got)
	}
	if math.Abs(got.Shadows[0].OffsetXPt) > 1e-9 || math.Abs(got.Shadows[0].OffsetYPt+10) > 1e-9 {
		t.Fatalf("outer shadow offset = (%v,%v)", got.Shadows[0].OffsetXPt, got.Shadows[0].OffsetYPt)
	}
	if !got.Shadows[1].Inner || math.Abs(got.Shadows[1].OffsetXPt-4) > 1e-9 {
		t.Fatalf("inner shadow = %+v", got.Shadows[1])
	}
}
