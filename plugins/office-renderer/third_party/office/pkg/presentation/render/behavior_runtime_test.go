package render

import (
	"strings"
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestStaticTransitionDoesNotInstallBehaviorRuntime(t *testing.T) {
	pres := &ir.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*ir.Slide{
		{Index: 0, Transition: &ir.SlideTransition{Type: "fade", AdvClick: true}},
		{Index: 1},
	}}
	out, err := HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, `body.ppt-player-ready`) {
		t.Fatal("transition-only static HTML installed behavior runtime")
	}
}

func TestAnimationAndActionsInstallBehaviorRuntimeWithoutForcingPlayback(t *testing.T) {
	pres := &ir.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*ir.Slide{{
		Index:  0,
		Shapes: []*ir.Shape{{Kind: ir.ShapeText, AnimStep: 1, ActionLink: "nextSlide", Bounds: ir.Rect{W: 1000, H: 1000}}},
	}}}
	out, err := HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`body.ppt-player-ready`, `runTimeline(slide, "step", currentStep)`, `data-ppt-action="nextSlide"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing behavior runtime marker %q", want)
		}
	}
}
