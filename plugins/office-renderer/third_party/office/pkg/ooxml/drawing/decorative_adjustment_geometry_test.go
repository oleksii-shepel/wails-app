package drawing

import (
	"math"
	"testing"
)

func TestStarAdjustmentControlsInnerRadiusAndStaysBounded(t *testing.T) {
	tight, _ := (&PresetGeometry{Name: "star5", Adjustments: []float64{0.2}}).GetPath(200, 100)
	open, _ := (&PresetGeometry{Name: "star5", Adjustments: []float64{0.8}}).GetPath(200, 100)
	// command 1 is the first inner vertex; larger inner radius must move farther from centre.
	tc, oc := tight.Commands[1].Points[0], open.Commands[1].Points[0]
	center := Point{X: 100, Y: 50}
	td := math.Hypot(tc.X-center.X, tc.Y-center.Y)
	od := math.Hypot(oc.X-center.X, oc.Y-center.Y)
	if !(od > td) {
		t.Fatalf("inner radius did not grow: tight=%g open=%g", td, od)
	}
	extreme, _ := (&PresetGeometry{Name: "star32", Adjustments: []float64{9}}).GetPath(200, 100)
	assertPathInside(t, extreme, 200, 100)
}

func TestRibbonVariantsOwnAuthoredDepthAndAreDistinct(t *testing.T) {
	shallow, _ := (&PresetGeometry{Name: "ribbon", Adjustments: []float64{0.1, 0.2}}).GetPath(200, 100)
	deep, _ := (&PresetGeometry{Name: "ribbon", Adjustments: []float64{0.4, 0.2}}).GetPath(200, 100)
	if shallow.Commands[4].Points[0].Y == deep.Commands[4].Points[0].Y {
		t.Fatal("ribbon depth adjustment ignored")
	}
	inverted, _ := (&PresetGeometry{Name: "ribbon2", Adjustments: []float64{0.2, 0.2}}).GetPath(200, 100)
	normal, _ := (&PresetGeometry{Name: "ribbon", Adjustments: []float64{0.2, 0.2}}).GetPath(200, 100)
	if inverted.Commands[4].Points[0].Y == normal.Commands[4].Points[0].Y {
		t.Fatal("ribbon2 collapsed to ribbon geometry")
	}
}

func TestEllipseRibbonUsesCurvedGeometryAndAdjustment(t *testing.T) {
	p, _ := (&PresetGeometry{Name: "ellipseRibbon", Adjustments: []float64{0.3, 0.2}}).GetPath(200, 100)
	if len(p.Commands) < 3 || p.Commands[1].Type != "C" || p.Commands[2].Type != "C" {
		t.Fatalf("ellipse ribbon is not curved: %#v", p.Commands)
	}
}

func TestScrollVariantsHaveRolledEdgesAndStayFinite(t *testing.T) {
	for _, name := range []string{"horizontalScroll", "verticalScroll"} {
		p, err := (&PresetGeometry{Name: name, Adjustments: []float64{0.9}}).GetPath(200, 100)
		if err != nil {
			t.Fatal(err)
		}
		foundCurve := false
		for _, c := range p.Commands {
			if c.Type == "C" {
				foundCurve = true
			}
			for _, pt := range c.Points {
				if math.IsNaN(pt.X) || math.IsNaN(pt.Y) || math.IsInf(pt.X, 0) || math.IsInf(pt.Y, 0) {
					t.Fatalf("%s invalid point %+v", name, pt)
				}
			}
		}
		if !foundCurve {
			t.Fatalf("%s has no rolled edge", name)
		}
	}
}

func assertPathInside(t *testing.T, p Path, w, h float64) {
	t.Helper()
	for _, c := range p.Commands {
		for _, pt := range c.Points {
			if pt.X < -1e-9 || pt.X > w+1e-9 || pt.Y < -1e-9 || pt.Y > h+1e-9 {
				t.Fatalf("point outside bounds: %+v", pt)
			}
		}
	}
}
