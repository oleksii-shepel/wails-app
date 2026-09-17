package render

import (
	"os"
	"strings"
	"testing"
)

func TestShapeEffectStylesConsumePhysicalPlan(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	const signature = "func presentationEffectPlanStyles(effects playout.EffectVisualPlan) map[string]string {"
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("physical effect style function %q not found", signature)
	}

	rest := src[start:]
	end := strings.Index(rest, "\nfunc presentationTextEffectPlanStyles(")
	if end < 0 {
		t.Fatal("end of physical effect style function not found")
	}
	body := rest[:end]

	for _, forbidden := range []string{
		"shape.Effects",
		"drawingcss.EffectList",
		"drawing.EffectList",
		"ResolveEffectVisualPlan(",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("renderer still interprets canonical effect state %q", forbidden)
		}
	}

	for _, required := range []string{
		"playout.EffectVisualPlan",
		"effects.Composition",
		"effects.Shadows",
		"effects.Glows",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("renderer does not consume physical effect plan %q", required)
		}
	}
}


func TestClass14RendererDoesNotReparseSourceEffectList(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, forbidden := range []string{
		"shape.Effects.Effects",
		"sourceShape.Effects.Effects",
		"SourceShape.Effects.Effects",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("renderer reparses canonical source effects via %q", forbidden)
		}
	}
}
