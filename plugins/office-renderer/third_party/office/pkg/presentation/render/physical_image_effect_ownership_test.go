package render

import (
	"os"
	"strings"
	"testing"
)

func TestImageRenderingConsumesPhysicalBlipEffectPlan(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	start := strings.Index(s, "func renderImageShape(")
	end := strings.Index(s[start:], "\nfunc renderMediaShape(")
	if start < 0 || end < 0 {
		t.Fatal("renderImageShape boundaries not found")
	}
	body := s[start : start+end]
	if strings.Contains(body, "shape.ImageEffects") {
		t.Fatal("renderImageShape must not reinterpret canonical ImageEffects")
	}
	for _, required := range []string{"imageVisual.Effects", "imageEffectPlanFilterDefinition", "imageEffectPlanNames"} {
		if !strings.Contains(body, required) {
			t.Fatalf("renderImageShape must consume physical image effect plan via %q", required)
		}
	}
}
