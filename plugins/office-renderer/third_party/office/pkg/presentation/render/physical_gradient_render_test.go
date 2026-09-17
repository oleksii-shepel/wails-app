package render

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	playout "office/pkg/presentation/layout"
)

func TestSVGPhysicalGradientDefinitionUsesMaterializedAngleAndAlpha(t *testing.T) {
	def := svgPhysicalGradientDefinition(&playout.GradientVisualPlan{
		Kind:     drawing.GradientTypeLinear,
		AngleDeg: 0,
		Stops: []playout.GradientStopVisualPlan{
			{Position: 0, Color: "#112233", Alpha: 1},
			{Position: 1, Color: "#445566", Alpha: 0.5},
		},
	}, "g")
	for _, want := range []string{`<linearGradient id="g"`, `x1="0.0000%"`, `x2="100.0000%"`, `stop-color="#112233"`, `stop-opacity="0.5000"`} {
		if !strings.Contains(def, want) {
			t.Fatalf("gradient definition missing %q: %s", want, def)
		}
	}
}

func TestPhysicalGradientRendererNormalizesRawHexStopColors(t *testing.T) {
	gradient := playout.GradientVisualPlan{
		Kind:  drawing.GradientTypeLinear,
		Stops: []playout.GradientStopVisualPlan{{Position: 0, Color: "2DA2BF", Alpha: 1}},
	}
	got := svgPhysicalGradientDefinition(&gradient, "g")
	if !strings.Contains(got, `stop-color="#2DA2BF"`) {
		t.Fatalf("gradient stop color was not normalized for SVG: %s", got)
	}
}

func TestClass5PhysicalGradientRendererPreservesZeroAlphaStop(t *testing.T) {
	gradient := playout.GradientVisualPlan{
		Kind: drawing.GradientTypeLinear,
		Stops: []playout.GradientStopVisualPlan{
			{Position: 0, Color: "112233", Alpha: 1},
			{Position: 1, Color: "445566", Alpha: 0},
		},
	}
	got := svgPhysicalGradientDefinition(&gradient, "g")
	if !strings.Contains(got, `offset="100.0000%" stop-color="#445566" stop-opacity="0.0000"`) {
		t.Fatalf("transparent gradient stop was not preserved: %s", got)
	}
}
