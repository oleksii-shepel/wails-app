package drawing

import "testing"

func TestPass31CustomPathLocalPaintFlagsOverrideShapeStyle(t *testing.T) {
	geom := &Geometry{Type: GeometryTypeCustom, Paths: []Path{
		{PaintFlagsSet: true, StrokeEnabled: true, FillEnabled: false, Commands: []PathCommand{{Type: "M", Points: []Point{{0, 0}}}, {Type: "L", Points: []Point{{1, 1}}}}},
		{PaintFlagsSet: true, StrokeEnabled: false, FillEnabled: true, Commands: []PathCommand{{Type: "M", Points: []Point{{0, 0}}}, {Type: "L", Points: []Point{{1, 0}}}, {Type: "Z"}}},
	}}
	line := &LineStyle{Color: "000000", Width: 2}
	primitives, err := BuildShapePrimitives(ShapeGeometrySpec{Geometry: geom, Width: 100, Height: 100, FillColor: "FF0000", Stroke: line, StrokeColor: "000000", StrokeWidth: 2})
	if err != nil || len(primitives) != 2 {
		t.Fatalf("primitives len=%d err=%v", len(primitives), err)
	}
	if primitives[0].Style.FillColor != "" || primitives[0].Style.StrokeColor != "000000" {
		t.Fatalf("stroke-only style = fill:%q stroke:%q", primitives[0].Style.FillColor, primitives[0].Style.StrokeColor)
	}
	if primitives[1].Style.FillColor != "FF0000" || primitives[1].Style.StrokeColor != "" || primitives[1].Style.StrokeWidth != 0 {
		t.Fatalf("fill-only style = fill:%q stroke:%q width:%v", primitives[1].Style.FillColor, primitives[1].Style.StrokeColor, primitives[1].Style.StrokeWidth)
	}
}
