package layout

import (
	"math"
	"office/pkg/ooxml/drawing"
	"testing"
)

func TestResolveGradientVisualPlanPreservesConcentricCenterRectangle(t *testing.T) {
	fill := drawing.GradientFill{
		GradientType: drawing.GradientTypePath,
		FillToRect:   &drawing.Rect{X: .2, Y: .3, Width: .4, Height: .2},
	}
	got := resolveGradientVisualPlan(fill)
	if !got.HasCenter {
		t.Fatal("expected center rectangle")
	}
	if math.Abs(got.CenterX-.4) > 1e-9 || math.Abs(got.CenterY-.4) > 1e-9 {
		t.Fatalf("center=(%v,%v), want (.4,.4)", got.CenterX, got.CenterY)
	}
	if math.Abs(got.CenterWidth-.4) > 1e-9 || math.Abs(got.CenterHeight-.2) > 1e-9 {
		t.Fatalf("center size=(%v,%v), want (.4,.2)", got.CenterWidth, got.CenterHeight)
	}
}

func TestClass5GradientFillToRectangleIsClampedToShapeSpace(t *testing.T) {
	fill := drawing.GradientFill{GradientType: drawing.GradientTypePath, FillToRect: &drawing.Rect{X: -.2, Y: .8, Width: 1.5, Height: .5}}
	got := resolveGradientVisualPlan(fill)
	if !got.HasCenter {
		t.Fatal("expected center rectangle")
	}
	if math.Abs(got.CenterX-.5) > 1e-9 || math.Abs(got.CenterY-.9) > 1e-9 || math.Abs(got.CenterWidth-1) > 1e-9 || math.Abs(got.CenterHeight-.2) > 1e-9 {
		t.Fatalf("clamped center=(%v,%v %vx%v), want (.5,.9 1x.2)", got.CenterX, got.CenterY, got.CenterWidth, got.CenterHeight)
	}
}
