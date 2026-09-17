package layout

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestMaterializePhysicalVisualTreeCarriesGenericDrawingComposition(t *testing.T) {
	shape := &ir.Shape{
		Kind:        ir.ShapeText,
		Bounds:      ir.Rect{X: 12 * 12700, Y: 18 * 12700, W: 120 * 12700, H: 60 * 12700},
		Geometry:    "roundRect",
		RotationDeg: 17,
		FlipH:       true,
		FillColor:   "#336699",
		LineColor:   "#112233",
		LineWidthPt: 2,
	}
	items := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(items) != 1 || items[0].Drawing == nil {
		t.Fatalf("generic drawing plan missing: %+v", items)
	}
	got := items[0].Drawing
	if got.FrameBounds.LeftPt != 12 || got.FrameBounds.TopPt != 18 || got.FrameBounds.WidthPt != 120 || got.FrameBounds.HeightPt != 60 {
		t.Fatalf("generic drawing frame = %+v", got.FrameBounds)
	}
	if got.Transform.RotationDeg != 17 || !got.Transform.FlipH {
		t.Fatalf("generic drawing transform = %+v", got.Transform)
	}
	if got.Visual.Geometry.Preset != "roundRect" {
		t.Fatalf("generic drawing geometry = %+v", got.Visual.Geometry)
	}
	if got.Paint.ZIndex != 1 {
		t.Fatalf("generic drawing paint = %+v", got.Paint)
	}
}
