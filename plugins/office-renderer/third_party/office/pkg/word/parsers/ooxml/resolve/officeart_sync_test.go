package resolve

import (
	"testing"

	dml "office/pkg/ooxml/drawing"
)

func TestSyncWordOfficeArtShape(t *testing.T) {
	shape := &Shape{
		Kind: "rect", X: 10, Y: 20, Width: 300, Height: 200,
		RotationDeg: 15, FlipH: true,
		Geometry: &dml.PresetGeometry{Name: "roundRect", Adjustments: []float64{0.2}},
		Fill:     dml.SolidFill{Color: "FF0000", Alpha: 1},
	}
	got := syncWordOfficeArtShape(shape)
	if got == nil || got.Transform.Rect.X != 10 || got.Geometry.Preset != "roundRect" || got.Fill.Color != "FF0000" {
		t.Fatalf("officeart=%#v", got)
	}
}
