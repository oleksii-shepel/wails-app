package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLDoesNotOwnOuterTextShapeComposition(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func renderTextShape(")
	endRel := strings.Index(src[start:], "\nfunc renderPhysicalWordArt(")
	if start < 0 || endRel < 0 {
		t.Fatal("renderTextShape boundaries not found")
	}
	body := src[start : start+endRel]
	for _, forbidden := range []string{
		"boundsPercent(shape.Bounds",
		"shape.RotationDeg",
		"shape.FlipH",
		"shape.FlipV",
		"effectStyleProps(ctx, shape)",
		"appendPhysicalPaintStyles(styles, ctx, shape)",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("renderer must consume physical text-shape composition; found %q", forbidden)
		}
	}
	for _, required := range []string{
		"item.Text",
		"physicalBoundsPercent(textPlan.FrameBounds",
		"textPlan.Transform.RotationDeg",
		"presentationTextShapeEffectPlanStyles(textPlan.Effects)",
		"textPlan.Paint.ZIndex",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("renderer must consume precomputed text-shape composition %q", required)
		}
	}
}
