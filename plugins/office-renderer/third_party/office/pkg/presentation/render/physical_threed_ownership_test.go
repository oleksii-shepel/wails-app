package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLThreeDRenderingConsumesPhysicalPlanOnly(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"ThreeDSource",
		"drawingcss.ThreeDLightOverlays",
		"extractShapeThreeD(",
		"*drawing.ThreeD) {",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("HTML renderer retained canonical 3-D interpretation surface %q", forbidden)
		}
	}
	for _, required := range []string{
		"visual.Effects.Composition.ThreeD",
		"*playout.ThreeDVisualPlan",
		"LightingOverlays",
		"ExtrusionDepthPt",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("HTML renderer must consume physical 3-D plan %q", required)
		}
	}
}
