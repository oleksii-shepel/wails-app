package layout

import (
	"math"
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"testing"
)

func TestPhysicalGradientCounterRotatesWhenNotRotatingWithShape(t *testing.T) {
	visual := ShapeVisualPlan{Fill: FillVisualPlan{Gradient: &GradientVisualPlan{Kind: drawing.GradientTypeLinear, AngleDeg: 90, RotateWithShape: false}}}
	materializePhysicalGradientOrientation(&visual, PhysicalTransformPlan{RotationDeg: 30}, nil)
	if got := visual.Fill.Gradient.AngleDeg; got != 60 {
		t.Fatalf("physical gradient angle = %v, want 60", got)
	}
}

func TestPhysicalGradientKeepsAngleWhenRotatingWithShape(t *testing.T) {
	visual := ShapeVisualPlan{Fill: FillVisualPlan{Gradient: &GradientVisualPlan{Kind: drawing.GradientTypeLinear, AngleDeg: 90, RotateWithShape: true}}}
	materializePhysicalGradientOrientation(&visual, PhysicalTransformPlan{RotationDeg: 30}, nil)
	if got := visual.Fill.Gradient.AngleDeg; got != 90 {
		t.Fatalf("physical gradient angle = %v, want 90", got)
	}
}

func TestPhysicalGradientShadeScaleUsesShapeAspectRatio(t *testing.T) {
	visual := ShapeVisualPlan{Fill: FillVisualPlan{Gradient: &GradientVisualPlan{Kind: drawing.GradientTypeLinear, AngleDeg: 45, RotateWithShape: true, ScaleAngleWithAspect: true}}}
	shape := &ir.Shape{Bounds: ir.Rect{W: 200, H: 100}}
	materializePhysicalGradientOrientation(&visual, PhysicalTransformPlan{}, shape)
	want := 26.56505117707799
	if got := visual.Fill.Gradient.AngleDeg; math.Abs(got-want) > 1e-9 {
		t.Fatalf("shadeScale angle = %v, want %v", got, want)
	}
}

func TestPhysicalGradientShadeScaleSquarePreservesAngle(t *testing.T) {
	visual := ShapeVisualPlan{Fill: FillVisualPlan{Gradient: &GradientVisualPlan{Kind: drawing.GradientTypeLinear, AngleDeg: 37, RotateWithShape: true, ScaleAngleWithAspect: true}}}
	shape := &ir.Shape{Bounds: ir.Rect{W: 100, H: 100}}
	materializePhysicalGradientOrientation(&visual, PhysicalTransformPlan{}, shape)
	if got := visual.Fill.Gradient.AngleDeg; math.Abs(got-37) > 1e-9 {
		t.Fatalf("square shadeScale angle = %v, want 37", got)
	}
}

func TestClass5GradientPlanPreservesTransparentStopsAndOrdersOffsets(t *testing.T) {
	fill := drawing.GradientFill{GradientType: drawing.GradientTypeLinear, Stops: []drawing.GradientStop{
		{Position: 1, Color: "FFFFFF", Alpha: 0},
		{Position: 0, Color: "000000", Alpha: 1},
		{Position: .5, Color: "808080", Alpha: .4},
	}}
	got := resolveGradientVisualPlan(fill)
	if len(got.Stops) != 3 {
		t.Fatalf("stops = %#v", got.Stops)
	}
	if got.Stops[0].Position != 0 || got.Stops[1].Position != .5 || got.Stops[2].Position != 1 {
		t.Fatalf("gradient offsets not canonicalized: %#v", got.Stops)
	}
	if got.Stops[2].Alpha != 0 {
		t.Fatalf("transparent terminal stop alpha = %v, want 0", got.Stops[2].Alpha)
	}
}

func TestClass5GradientPlanTreatsAllZeroLegacyAlphaAsOpaqueDefault(t *testing.T) {
	fill := drawing.GradientFill{GradientType: drawing.GradientTypeLinear, Stops: []drawing.GradientStop{
		{Position: 0, Color: "000000"},
		{Position: 1, Color: "FFFFFF"},
	}}
	got := resolveGradientVisualPlan(fill)
	for i, stop := range got.Stops {
		if stop.Alpha != 1 {
			t.Fatalf("stop %d alpha = %v, want compatibility default 1", i, stop.Alpha)
		}
	}
}

func TestClass5PhysicalGradientMaterializesAuthoredFlipIntoLinearVector(t *testing.T) {
	visual := ShapeVisualPlan{Fill: FillVisualPlan{Gradient: &GradientVisualPlan{Kind: drawing.GradientTypeLinear, AngleDeg: 0, RotateWithShape: true, FlipH: true}}}
	materializePhysicalGradientOrientation(&visual, PhysicalTransformPlan{}, nil)
	g := visual.Fill.Gradient
	if !g.HasLinearVector {
		t.Fatal("missing physical linear vector")
	}
	if math.Abs(g.LinearX1-1) > 1e-9 || math.Abs(g.LinearX2) > 1e-9 || math.Abs(g.LinearY1-.5) > 1e-9 || math.Abs(g.LinearY2-.5) > 1e-9 {
		t.Fatalf("flipped vector=(%v,%v)->(%v,%v), want (1,.5)->(0,.5)", g.LinearX1, g.LinearY1, g.LinearX2, g.LinearY2)
	}
}

func TestClass5PhysicalGradientCombinesShadeScaleRotationAndFlipBeforeRendering(t *testing.T) {
	visual := ShapeVisualPlan{Fill: FillVisualPlan{Gradient: &GradientVisualPlan{Kind: drawing.GradientTypeLinear, AngleDeg: 45, RotateWithShape: false, ScaleAngleWithAspect: true, FlipV: true}}}
	shape := &ir.Shape{Bounds: ir.Rect{W: 200, H: 100}}
	materializePhysicalGradientOrientation(&visual, PhysicalTransformPlan{RotationDeg: 15}, shape)
	g := visual.Fill.Gradient
	wantAngle := 26.56505117707799 - 15
	if math.Abs(g.AngleDeg-wantAngle) > 1e-9 {
		t.Fatalf("angle=%v want %v", g.AngleDeg, wantAngle)
	}
	if !g.HasLinearVector {
		t.Fatal("missing physical linear vector")
	}
	// Vertical authored flip mirrors Y endpoints after all physical angle work.
	if !(g.LinearY1 > .5 && g.LinearY2 < .5) {
		t.Fatalf("vertical flip not materialized: %+v", g)
	}
}
