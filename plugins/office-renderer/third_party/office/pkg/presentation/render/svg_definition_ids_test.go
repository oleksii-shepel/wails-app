package render_test

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
	"office/pkg/presentation/render"
)

func TestHTMLNamespacesInlineSVGDefinitionsPerShape(t *testing.T) {
	pres := &ir.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*ir.Slide{{
			Shapes: []*ir.Shape{
				{Kind: ir.ShapeText, Bounds: ir.Rect{W: 914400, H: 685800}, Fill: pictureFillForSVGDefinitionTest()},
				{Kind: ir.ShapeText, Bounds: ir.Rect{X: 914400, W: 914400, H: 685800}, Fill: pictureFillForSVGDefinitionTest()},
			},
		}},
	}
	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, id := range []string{"shape-1-fill", "shape-2-fill"} {
		if strings.Count(html, `id="`+id+`"`) != 1 || strings.Count(html, `url(#`+id+`)`) != 1 {
			t.Fatalf("expected exactly one definition and reference for %q\n%s", id, html)
		}
	}
	if strings.Contains(html, `id="shape-fill"`) || strings.Contains(html, `url(#shape-fill)`) {
		t.Fatalf("inline SVG definitions must not use a document-global shared ID\n%s", html)
	}
}

func pictureFillForSVGDefinitionTest() drawing.Fill {
	return drawing.PictureFill{ImageData: []byte{0x89, 0x50, 0x4E, 0x47}, ContentType: "image/png"}
}
