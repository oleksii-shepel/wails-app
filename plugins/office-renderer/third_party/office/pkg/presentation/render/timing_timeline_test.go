package render

import (
	"strings"
	"testing"

	"office/pkg/presentation/ir"
)

func TestHTMLSerializesRichAnimationTimeline(t *testing.T) {
	pres := &ir.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*ir.Slide{{
		Index:             0,
		Shapes:            []*ir.Shape{{ID: 7, Kind: ir.ShapeText, Bounds: ir.Rect{W: 914400, H: 685800}}},
		AnimationTimeline: []ir.AnimationEvent{{ShapeID: 7, Step: 1, NodeType: "clickEffect", Behavior: "animScale", Trigger: "onClick", DelayMs: 150, DurationMs: 600, RepeatCount: 2, AutoReverse: true, Fill: "hold"}},
	}}}
	out, err := HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`data-has-anim-timeline="1"`, `data-shape-id="7"`, `class="ppt-animation-timeline"`, `"DurationMs":600`, `runTimeline(slide, "step", currentStep)`, `node.animate(keyframes`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in HTML", want)
		}
	}
}
