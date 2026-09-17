package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLConsumesPhysicalSlideComposition(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	start := strings.Index(s, "func renderSlide(")
	end := strings.Index(s[start:], "func renderPhysicalSlideComments(")
	if start < 0 || end < 0 {
		t.Fatal("renderSlide boundaries not found")
	}
	body := s[start : start+end]
	if !strings.Contains(body, "physicalSlide *playout.PhysicalSlidePlan") {
		t.Fatal("renderer must consume the finalized layout-owned physical slide plan")
	}
	for _, forbidden := range []string{"slide.BgColor", "slide.BgFill", "slide.BgPictureTile", "renderVisualShapes(b, slide.Shapes"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("renderer retained slide composition ownership %q", forbidden)
		}
	}
}
