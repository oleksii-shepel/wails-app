package render

import (
	"strings"
	"testing"

	"office/pkg/officeart/ir"
)

func TestHTMLShapeRendersEmbeddedImageData(t *testing.T) {
	shape := &ir.Shape{
		ID:   1,
		Kind: ir.ShapePicture,
		Image: &ir.Image{
			MIMEType: "image/png",
			Data:     []byte{1, 2, 3},
		},
	}
	html, err := HTMLShape(shape)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "data:image/png;base64,AQID") {
		t.Fatalf("html = %q", html)
	}
}

func TestHTMLShapeRendersOfficeArtImageCrop(t *testing.T) {
	shape := &ir.Shape{
		ID:   2,
		Kind: ir.ShapePicture,
		Transform: ir.Transform{
			Rect: ir.Rect{W: 1270000, H: 1270000},
		},
		Image: &ir.Image{
			URL: "image.png",
			Crop: ir.Crop{
				Left:   0.10,
				Top:    0.20,
				Right:  0.30,
				Bottom: 0.10,
			},
		},
	}

	html, err := HTMLShape(shape)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"overflow:hidden",
		"width:166.666667%",
		"height:142.857143%",
		"left:-16.666667%",
		"top:-28.571429%",
		"max-width:none",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q: %s", want, html)
		}
	}
}

func TestImageStyleFallsBackForDegenerateCrop(t *testing.T) {
	got := imageStyle(ir.Crop{Left: 0.75, Right: 0.25})
	if got != "width:100%;height:100%;display:block" {
		t.Fatalf("style = %q", got)
	}
}
