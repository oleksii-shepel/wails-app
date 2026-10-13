package render

import (
	"strings"
	"testing"

	officeartir "office/pkg/officeart/ir"
	"office/pkg/presentation/ir"
)

func TestHTMLRendersPhysicalWordArtAsVectorTextPath(t *testing.T) {
	pres := &ir.Presentation{
		SlideWidthEMU: 9144000, SlideHeightEMU: 6858000,
		Slides: []*ir.Slide{{Index: 0, Shapes: []*ir.Shape{{
			Kind: ir.ShapeText, Bounds: ir.Rect{X: 914400, Y: 914400, W: 3657600, H: 1371600},
			FillColor: "3366CC", LineColor: "112233", LineWidthPt: 1,
			OfficeArt: &officeartir.Shape{GeometryText: &officeartir.GeometryText{
				Text: "Legacy WordArt", FontFamily: "Arial Black", FontSizePt: 30,
				Warp: "textArchUpCurve", Align: 1, Bold: true, Stretch: true,
			}},
		}}}},
	}
	out, err := HTML(pres)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`class="shape__wordart"`, `<textPath`, `Q 50 5 95 78`, `Legacy WordArt`,
		`font-family:`, `font-weight:700`,
		// Legacy WordArt is laid out against its shape's bounding rectangle, so
		// the glyphs are scaled to the box rather than drawn at their size.
		`transform="translate(0 `, `scale(1 `, `textLength="109.00"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("WordArt HTML missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, `class="shape__content"`) {
		t.Fatalf("vector WordArt fell through to rectangular paragraph rendering: %s", out)
	}
}
