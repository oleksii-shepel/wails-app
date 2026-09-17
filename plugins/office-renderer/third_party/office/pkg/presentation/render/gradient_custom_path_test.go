package render

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	playout "office/pkg/presentation/layout"
)

func TestSVGShadeShapeUsesActualCustomPathBoundary(t *testing.T) {
	path := &drawing.Path{Commands: []drawing.PathCommand{
		{Type: "M", Points: []drawing.Point{{X: 50, Y: 0}}},
		{Type: "L", Points: []drawing.Point{{X: 100, Y: 100}}},
		{Type: "L", Points: []drawing.Point{{X: 0, Y: 100}}},
		{Type: "Z"},
	}}
	g := &playout.GradientVisualPlan{
		Kind: drawing.GradientTypePath, PathBoundary: "triangle",
		PathBoundaryPath: path, PathBoundaryWidthPt: 100, PathBoundaryHeightPt: 100,
		CenterX: .5, CenterY: .5, HasCenter: true,
		Stops: []playout.GradientStopVisualPlan{{Position: 0, Color: "000000", Alpha: 1}, {Position: 1, Color: "FFFFFF", Alpha: 1}},
	}
	got := svgPhysicalGradientDefinition(g, "shade")
	if !strings.Contains(got, `<pattern id="shade"`) || !strings.Contains(got, `M 50.0000 0.0000`) {
		t.Fatalf("custom ShadeShape must materialize the actual boundary: %s", got)
	}
	if strings.Contains(got, `<radialGradient`) {
		t.Fatalf("custom ShadeShape must not degrade to radial geometry: %s", got)
	}
}

func TestSVGShadeShapeMaterializesNonZeroCustomCenterRectangle(t *testing.T) {
	path := &drawing.Path{Commands: []drawing.PathCommand{
		{Type: "M", Points: []drawing.Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []drawing.Point{{X: 100, Y: 0}}},
		{Type: "L", Points: []drawing.Point{{X: 50, Y: 100}}},
		{Type: "Z"},
	}}
	g := &playout.GradientVisualPlan{Kind: drawing.GradientTypePath, PathBoundary: "custom", PathBoundaryPath: path, PathBoundaryWidthPt: 100, PathBoundaryHeightPt: 100, CenterX: .5, CenterY: .5, CenterWidth: .2, CenterHeight: .4, HasCenter: true, Stops: []playout.GradientStopVisualPlan{{Position: 0, Color: "000000", Alpha: 1}, {Position: 1, Color: "FFFFFF", Alpha: 1}}}
	got := svgShapePathGradientDefinition(g, "shade")
	if !strings.Contains(got, `<pattern id="shade"`) {
		t.Fatalf("non-zero custom center rectangle must materialize a path gradient: %s", got)
	}
	// Final contour maps the 100x100 boundary into the authored 20x40 box
	// centered at (50,50): scale(.2,.4) + translate(40,30).
	if !strings.Contains(got, `matrix(0.20000000 0 0 0.40000000 40.000000 30.000000)`) {
		t.Fatalf("final contour does not map to authored center rectangle: %s", got)
	}
	if strings.Contains(got, `<radialGradient`) {
		t.Fatalf("custom center-rectangle ShadeShape must not degrade to radial geometry: %s", got)
	}
}
