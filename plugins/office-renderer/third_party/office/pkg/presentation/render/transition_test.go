package render_test

import (
	"strings"
	"testing"

	"office/pkg/presentation/parsers/ooxml/resolve"
	"office/pkg/presentation/render"
)

func TestSlideTransitionDataAttributes(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Transition: &resolve.SlideTransition{
					Type:     "fade",
					Spd:      "med",
					AdvClick: true,
					AdvTmMs:  4000,
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-transition="fade"`) {
		t.Errorf("expected data-transition=\"fade\" in output:\n%s", out)
	}
	if !strings.Contains(out, `data-transition-spd="med"`) {
		t.Errorf("expected data-transition-spd=\"med\" in output:\n%s", out)
	}
	if !strings.Contains(out, `data-transition-adv-tm="4000"`) {
		t.Errorf("expected data-transition-adv-tm=\"4000\" in output:\n%s", out)
	}
	if strings.Contains(out, `data-transition-adv-click="0"`) {
		t.Errorf("unexpected data-transition-adv-click=\"0\" when AdvClick=true:\n%s", out)
	}
}

func TestSlideTransitionNoAutoAdvanceEmitsAdvClickZero(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Transition: &resolve.SlideTransition{
					Type:     "push",
					Spd:      "slow",
					AdvClick: false,
					AdvTmMs:  0,
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-transition="push"`) {
		t.Errorf("expected data-transition=\"push\":\n%s", out)
	}
	if !strings.Contains(out, `data-transition-adv-click="0"`) {
		t.Errorf("expected data-transition-adv-click=\"0\":\n%s", out)
	}
	if !strings.Contains(out, `<div class="slide" id="slide-1" data-transition="push" data-transition-spd="slow" data-transition-adv-click="0">`) {
		t.Errorf("expected slide markup without auto-advance attr:\n%s", out)
	}
}

func TestSlideTransitionPlaybackBootstrap(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:     resolve.ShapeText,
						AnimStep: 2,
						Bounds:   resolve.Rect{X: 0, Y: 0, W: 1000000, H: 500000},
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: "animated"}}},
						},
					},
				},
			},
			{
				Index: 1,
				Transition: &resolve.SlideTransition{
					Type:     "fade",
					Spd:      "med",
					AdvClick: true,
					AdvTmMs:  2500,
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		`data-max-anim-step="2"`,
		`body.ppt-player-ready`,
		`document.body.classList.add("ppt-player-ready")`,
		`node.setAttribute("data-anim-visible", required <= step ? "1" : "0")`,
		`window.setTimeout(function () {`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestSlideNoTransitionEmitsNoDataAttrs(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{Index: 0},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, `data-transition`) {
		t.Errorf("unexpected data-transition attribute when no transition:\n%s", out)
	}
	if strings.Contains(out, `ppt-player-ready`) {
		t.Errorf("unexpected playback bootstrap for static slide:\n%s", out)
	}
}

func TestShapeAnimStepDataAttribute(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:     resolve.ShapeText,
						AnimStep: 2,
						Bounds:   resolve.Rect{X: 0, Y: 0, W: 1000000, H: 500000},
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: "animated"}}},
						},
					},
					{
						Kind:   resolve.ShapeText,
						Bounds: resolve.Rect{X: 0, Y: 600000, W: 1000000, H: 500000},
						Paragraphs: []*resolve.Paragraph{
							{Runs: []*resolve.Run{{Text: "always visible"}}},
						},
					},
				},
			},
		},
	}
	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-anim-step="2"`) {
		t.Errorf("expected data-anim-step=\"2\" in output:\n%s", out)
	}
	if strings.Count(out, ` data-anim-step="`) != 1 {
		t.Errorf("expected exactly 1 data-anim-step attr, output:\n%s", out)
	}
}
func TestHTMLActionLinksEnablePlaybackNavigation(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{
			{
				Index: 0,
				Shapes: []*resolve.Shape{
					{
						Kind:       resolve.ShapeText,
						ActionLink: "nextSlide",
						Bounds:     resolve.Rect{W: 1000000, H: 500000},
						Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Next"}}}},
					},
					{
						Kind:       resolve.ShapeText,
						ActionLink: "slide3",
						Bounds:     resolve.Rect{Y: 600000, W: 1000000, H: 500000},
						Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Third"}}}},
					},
				},
			},
			{Index: 1},
			{Index: 2},
		},
	}

	out, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}

	for _, want := range []string{
		`data-ppt-action="nextSlide"`,
		`href="#action-nextSlide"`,
		`data-ppt-action="slide3"`,
		`href="#slide-3"`,
		`slide.querySelector("[data-anim-motion],[data-anim-trigger],[data-ppt-action]")`,
		`function activateAction(action)`,
		`case "firstSlide":`,
		`case "lastSlide":`,
		`case "nextSlide":`,
		`case "prevSlide":`,
		`case "endShow":`,
		`action.match(/^slide(\d+)$/)`,
		`var actionNode = event.target.closest("[data-ppt-action]")`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in output:\n%s", want, out)
		}
	}
}
