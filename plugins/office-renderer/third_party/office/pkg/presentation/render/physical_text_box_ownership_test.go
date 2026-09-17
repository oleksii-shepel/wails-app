package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLTextRendererDoesNotRecomposeGeometryInsetsOrAutofitScale(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, forbidden := range []string{
		"visual.Geometry.ContentInsetTop",
		"visual.Geometry.ContentInsetRight",
		"visual.Geometry.ContentInsetBottom",
		"visual.Geometry.ContentInsetLeft",
		"textAutofitFontScale(",
		"textAutofitLineScale(",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("HTML renderer retained text-box composition surface %q", forbidden)
		}
	}
}

func TestOrdinaryTextShapeConsumesMaterializedParagraphPlans(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func renderTextShape(")
	end := strings.Index(src[start:], "\nfunc ")
	if start < 0 || end < 0 {
		t.Fatal("renderTextShape source not found")
	}
	body := src[start : start+end]
	if !strings.Contains(body, "textPlan.Paragraphs") || !strings.Contains(body, "renderParagraphWithPlan") {
		t.Fatal("ordinary text renderer must consume materialized paragraph plans")
	}
	if strings.Contains(body, "ResolveParagraphVisualPlan") {
		t.Fatal("ordinary text renderer must not resolve paragraph layout")
	}
}
