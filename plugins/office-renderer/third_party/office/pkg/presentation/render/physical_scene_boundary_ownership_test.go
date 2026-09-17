package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLSlideRendererConsumesFinalPhysicalScene(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func renderSlide(")
	end := strings.Index(src[start:], "func renderPhysicalSlideComments(")
	if start < 0 || end < 0 {
		t.Fatal("could not isolate renderSlide")
	}
	body := src[start : start+end]
	if !strings.Contains(body, "physicalSlide *playout.PhysicalSlidePlan") {
		t.Fatal("renderSlide must accept the finalized physical slide scene")
	}
	for _, forbidden := range []string{"*ir.Slide", "MaterializePhysicalSlide(", "slide.Shapes", "slide.Transition", "slide.AnimationTimeline", "slide.SectionName", "slide.Index"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("renderSlide retained canonical slide composition dependency %q", forbidden)
		}
	}
}

func TestHTMLPresentationRuntimeDecisionConsumesPhysicalPlaybackSummary(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{"physicalPresentationNeedsPlayback(physicalSlides)", "physicalPresentationNeedsBehaviorRuntime(physicalSlides)", "playback.HasInteractiveBehavior"} {
		if !strings.Contains(src, required) {
			t.Fatalf("renderer missing physical scene boundary %q", required)
		}
	}
	for _, forbidden := range []string{"func presentationNeedsPlayback(pres *ir.Presentation)", "func presentationNeedsBehaviorRuntime(pres *ir.Presentation)", "func slideHasInteractiveAnimations("} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("renderer retained canonical presentation behavior scan %q", forbidden)
		}
	}
}
