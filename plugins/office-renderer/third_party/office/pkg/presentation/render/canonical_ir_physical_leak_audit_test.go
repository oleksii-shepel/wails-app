package render

import (
	"os"
	"strings"
	"testing"
)

// TestRendererCanonicalIRPhysicalLeakAudit keeps the final renderer boundary
// honest. Canonical shapes remain valid semantic/content carriers (text,
// hyperlinks, media bytes, names and animation metadata), but physical
// composition must arrive through layout-owned plans.
func TestRendererCanonicalIRPhysicalLeakAudit(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	forbidden := []string{
		"shape.Bounds",
		"shape.RotationDeg",
		"shape.FlipH",
		"shape.FlipV",
		"shape.Effects",
		"shape.LineWidthPt",
		"shape.LineColor",
		"shape.FillColor",
		"shape.PictureTile",
		"shape.GeometryData",
		"shape.GeometryAdjustments",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("renderer still reads canonical physical/composition state %q", token)
		}
	}
}

func TestRectFallbackConsumesResolvedShapeVisualPlan(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func rectShapeStyles(")
	if start < 0 {
		t.Fatal("rectShapeStyles not found")
	}
	end := strings.Index(src[start:], "\nfunc presentationEffectPlanStyles")
	if end < 0 {
		t.Fatal("rectShapeStyles boundary not found")
	}
	body := src[start : start+end]
	for _, required := range []string{"visual.HasVisibleFill", "visual.Stroke"} {
		if !strings.Contains(body, required) {
			t.Fatalf("rect fallback does not consume resolved visual contract %q", required)
		}
	}
	if strings.Contains(body, "shape.") {
		t.Fatal("rect fallback must not consult canonical shape paint state")
	}
}
