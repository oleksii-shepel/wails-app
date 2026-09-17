package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLConsumesPhysicalSlideAuxiliaryComposition(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	start := strings.Index(s, "func renderSlide(")
	endRel := strings.Index(s[start:], "func renderPhysicalSlideComments(")
	if start < 0 || endRel < 0 {
		t.Fatal("renderSlide boundaries not found")
	}
	body := s[start : start+endRel]
	for _, required := range []string{"physicalSlide.Auxiliary.Notes", "physicalSlide.Auxiliary.Comments", "renderPhysicalVisuals(b, physicalSlide.Auxiliary.Notes.Content"} {
		if !strings.Contains(body, required) {
			t.Fatalf("renderer must consume physical auxiliary plan %q", required)
		}
	}
	for _, forbidden := range []string{"slide.Notes", "slide.Comments", "renderVisualShapes(b, slide.Notes.Shapes"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("renderer retained auxiliary composition ownership %q", forbidden)
		}
	}
}
