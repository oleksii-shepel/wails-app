package render

import (
	"strings"
	"testing"

	"office/pkg/presentation/ir"
)

func TestPass31StaticHTMLSuppressesAnimationOnlyOverlay(t *testing.T) {
	pres := &ir.Presentation{SlideWidthEMU: 9144000, SlideHeightEMU: 6858000, Slides: []*ir.Slide{{Shapes: []*ir.Shape{
		{ID: 1, Kind: ir.ShapeText, Bounds: ir.Rect{W: 1000000, H: 1000000}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "base"}}}}},
		{ID: 2, Kind: ir.ShapeText, AnimationOnly: true, AnimStep: 1, Bounds: ir.Rect{W: 1000000, H: 1000000}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "overlay-only"}}}}},
	}}}}
	out, err := HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "overlay-only") || !strings.Contains(out, "base") {
		t.Fatalf("static output did not suppress animation overlay")
	}
}
