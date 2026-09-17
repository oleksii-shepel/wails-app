package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLConsumesPhysicalSlidePlaybackComposition(t *testing.T) {
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
	if !strings.Contains(body, "physicalSlide.Playback") {
		t.Fatal("renderer must consume layout-owned slide playback plan")
	}
	for _, forbidden := range []string{"slide.Transition", "slide.AnimationTimeline", "slideMaxAnimStep(slide)", "animationTargetShapeIDs(slide)"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("renderer retained slide playback ownership %q", forbidden)
		}
	}
}
