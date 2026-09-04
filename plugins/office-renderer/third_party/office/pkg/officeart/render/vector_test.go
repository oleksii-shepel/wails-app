package render

import (
	"strings"
	"testing"

	"office/pkg/officeart/ir"
)

func TestHTMLShapeRendersSharedConnectorWithArrows(t *testing.T) {
	shape := &ir.Shape{
		ID:   44,
		Kind: ir.ShapeConnector,
		Transform: ir.Transform{
			Rect: ir.Rect{X: 12700, Y: 25400, W: 1270000, H: 635000},
		},
		Line: ir.Line{
			Visible: true,
			Color:   "112233",
			Alpha:   1,
			WidthPt: 2,
			Dash:    "dash",
			StartArrow: ir.Arrow{
				Type: "triangle", Width: "lg", Length: "sm",
			},
			EndArrow: ir.Arrow{
				Type: "stealth", Width: "sm", Length: "lg",
			},
		},
		Connector: &ir.Connector{
			Start: &ir.Point{X: 12700, Y: 25400},
			End:   &ir.Point{X: 1282700, Y: 660400},
		},
	}

	out, err := HTMLShape(shape)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`class="officeart-vector officeart-connector"`,
		`x1="0.000000" y1="0.000000" x2="100.000000" y2="50.000000"`,
		`stroke="#112233"`,
		`stroke-width="2"`,
		`marker-start="url(#officeart-arrow-44-start)"`,
		`marker-end="url(#officeart-arrow-44-end)"`,
		`M 0 0 L 10 5 L 0 10 Z`,
		`M 0 0 L 10 5 L 0 10 L 3 5 Z`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "border-width") {
		t.Fatalf("connector must not render a rectangular CSS border: %s", out)
	}
}

func TestHTMLShapeRendersConnectorPathAsPolyline(t *testing.T) {
	shape := &ir.Shape{
		ID:   45,
		Kind: ir.ShapeConnector,
		Transform: ir.Transform{
			Rect: ir.Rect{W: 1270000, H: 1270000},
		},
		Line: ir.Line{Visible: true, Color: "000000", Alpha: 1, WidthPt: 1},
		Connector: &ir.Connector{
			Path: []ir.Point{
				{X: 0, Y: 0},
				{X: 635000, Y: 254000},
				{X: 1270000, Y: 1270000},
			},
		},
	}

	out, err := HTMLShape(shape)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `points="0.000000,0.000000 50.000000,20.000000 100.000000,100.000000"`) {
		t.Fatalf("expected connector polyline: %s", out)
	}
}

func TestHTMLShapeRendersCustomGeometryPaths(t *testing.T) {
	shape := &ir.Shape{
		ID:   46,
		Kind: ir.ShapeAuto,
		Transform: ir.Transform{
			Rect: ir.Rect{W: 1270000, H: 1270000},
		},
		Geometry: ir.Geometry{
			Kind:    ir.GeometryCustom,
			ViewBox: ir.Rect{W: 100, H: 100},
			Paths: []ir.Path{
				{
					Fill:   true,
					Stroke: true,
					Commands: []ir.PathCommand{
						{Op: "M", Points: []ir.PointF{{X: 0, Y: 100}}},
						{Op: "L", Points: []ir.PointF{{X: 50, Y: 0}}},
						{Op: "L", Points: []ir.PointF{{X: 100, Y: 100}}},
						{Op: "Z"},
					},
				},
			},
		},
		Fill: ir.Fill{Kind: ir.FillSolid, Color: "FF0000", Alpha: 1},
		Line: ir.Line{Visible: true, Color: "0000FF", Alpha: 1, WidthPt: 1.5},
	}

	out, err := HTMLShape(shape)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`class="officeart-vector officeart-geometry"`,
		`viewBox="0 0 100 100"`,
		`d="M 0 100 L 50 0 L 100 100 Z"`,
		`fill="#FF0000"`,
		`stroke="#0000FF"`,
		`stroke-width="1.5"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "border-width") {
		t.Fatalf("custom geometry must not render a rectangular CSS border: %s", out)
	}
}

func TestShapeStyleKeepsHorizontalConnectorRenderable(t *testing.T) {
	style := ShapeStyle(&ir.Shape{
		Kind: ir.ShapeConnector,
		Transform: ir.Transform{
			Rect: ir.Rect{W: 1270000, H: 0},
		},
	})
	if style["width"] != "100.0000pt" || style["height"] != "1.0000pt" {
		t.Fatalf("connector size = %q x %q", style["width"], style["height"])
	}
}
