package render

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	playout "office/pkg/presentation/layout"
)

func TestSVGPhysicalPathGradientMaterializesRectangularBoundary(t *testing.T) {
	g := &playout.GradientVisualPlan{
		Kind: drawing.GradientTypePath, PathBoundary: "rect",
		CenterX: .5, CenterY: .5, HasCenter: true,
		Stops: []playout.GradientStopVisualPlan{
			{Position: 0, Color: "000000", Alpha: 1},
			{Position: 1, Color: "FFFFFF", Alpha: 1},
		},
	}
	got := svgPhysicalGradientDefinition(g, "g")
	if !strings.HasPrefix(got, `<pattern id="g"`) {
		t.Fatalf("rect ShadeShape should materialize as a rectangular path-gradient pattern: %s", got)
	}
	if strings.Contains(got, `<radialGradient`) {
		t.Fatalf("rect ShadeShape must not fall back to radial geometry: %s", got)
	}
	if !strings.Contains(got, `x="0.000000" y="0.000000" width="1.000000" height="1.000000" fill="#FFFFFF"`) {
		t.Fatalf("outer contour must use boundary stop: %s", got)
	}
	if !strings.Contains(got, `fill="#000000"`) {
		t.Fatalf("inner contour must converge to focus stop: %s", got)
	}
}

func TestSVGPhysicalPathGradientPreservesRectangularCenterPlateau(t *testing.T) {
	g := &playout.GradientVisualPlan{
		Kind: drawing.GradientTypePath, PathBoundary: "rect",
		CenterX: .5, CenterY: .5, CenterWidth: .2, CenterHeight: .4, HasCenter: true,
		Stops: []playout.GradientStopVisualPlan{{Position: 0, Color: "102030", Alpha: 1}, {Position: 1, Color: "F0E0D0", Alpha: 1}},
	}
	got := svgPhysicalGradientDefinition(g, "g")
	if !strings.Contains(got, `x="0.400000" y="0.300000" width="0.200000" height="0.400000" fill="#102030"`) {
		t.Fatalf("center rectangle was not preserved in materialized path gradient: %s", got)
	}
}
