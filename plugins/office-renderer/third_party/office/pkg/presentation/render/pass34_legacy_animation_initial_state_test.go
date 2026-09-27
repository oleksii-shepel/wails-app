package render

import (
	"strings"
	"testing"

	"office/pkg/presentation/ir"
)

func TestPass34PlaybackSerializesAnimationOnlyOverlayInitiallyHidden(t *testing.T) {
	pres := &ir.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*ir.Slide{{
		Shapes: []*ir.Shape{
			{ID: 1, Kind: ir.ShapeText, Bounds: ir.Rect{W: 1000000, H: 1000000}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "base"}}}}},
			{ID: 2, Kind: ir.ShapeText, AnimationOnly: true, AnimStep: 1, Bounds: ir.Rect{W: 1000000, H: 1000000}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "highlight overlay"}}}}},
		},
		AnimationTimeline: []ir.AnimationEvent{{ShapeID: 2, Step: 1, NodeType: "shape", Behavior: "cut"}},
	}}}

	out, _, err := HTMLWithOptions(pres, HTMLOptions{EnablePlayback: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `data-animation-only="1" data-anim-visible="0"`) {
		t.Fatalf("animation-only playback shape is not initially hidden")
	}
	if !strings.Contains(out, `[data-animation-only="1"][data-anim-visible="0"]`) {
		t.Fatalf("animation-only initial-state CSS missing")
	}
}

func TestPass34StaticHTMLStillOmitsAnimationOnlyOverlay(t *testing.T) {
	pres := &ir.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*ir.Slide{{Shapes: []*ir.Shape{
		{ID: 1, Kind: ir.ShapeText, Bounds: ir.Rect{W: 1000000, H: 1000000}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "base"}}}}},
		{ID: 2, Kind: ir.ShapeText, AnimationOnly: true, AnimStep: 1, Bounds: ir.Rect{W: 1000000, H: 1000000}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "highlight overlay"}}}}},
	}}}}
	out, err := HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "highlight overlay") {
		t.Fatalf("static HTML rendered animation-only overlay")
	}
}
