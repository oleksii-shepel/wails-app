package resolve

import (
	"testing"

	"office/pkg/ooxml/drawing"
)

func TestSyncPresentationOfficeArtShape(t *testing.T) {
	shape := &Shape{
		ID: 9, OriginalShapeID: 99, Name: "shape", Kind: ShapeText,
		Bounds:   Rect{X: 10, Y: 20, W: 400, H: 200},
		Geometry: "ellipse",
		Fill:     drawing.SolidFill{Color: "00FF00", Alpha: 1},
	}
	got := syncPresentationOfficeArtShape(shape)
	if got == nil || got.ID != 9 || got.Geometry.Preset != "ellipse" || got.Source.ShapeID != 99 {
		t.Fatalf("officeart=%#v", got)
	}
}
