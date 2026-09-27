package sax

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestDegenerateCustomOpenStrokeCanonicalizesAsConnector(t *testing.T) {
	shape := &ir.Shape{
		Kind:      ir.ShapeText,
		Geometry:  "custom",
		Bounds:    ir.Rect{X: 10, Y: 20, W: 0, H: 300},
		LineColor: "000000",
		GeometryData: &drawing.Geometry{
			Type: drawing.GeometryTypeCustom,
			Paths: []drawing.Path{{Commands: []drawing.PathCommand{
				{Type: "M", Points: []drawing.Point{{X: 0, Y: 1}}},
				{Type: "L", Points: []drawing.Point{{X: 0, Y: 0}}},
				{Type: "L", Points: []drawing.Point{{X: 0, Y: 0}}},
			}}},
		},
	}
	if !saxDegenerateCustomGeometryIsConnector(shape) {
		t.Fatal("degenerate open custom stroke was not recognized as connector")
	}
}

func TestDegenerateCustomClosedShapeDoesNotCanonicalizeAsConnector(t *testing.T) {
	shape := &ir.Shape{
		Kind:      ir.ShapeText,
		Geometry:  "custom",
		Bounds:    ir.Rect{W: 0, H: 300},
		LineColor: "000000",
		GeometryData: &drawing.Geometry{
			Type: drawing.GeometryTypeCustom,
			Paths: []drawing.Path{{Commands: []drawing.PathCommand{
				{Type: "M", Points: []drawing.Point{{X: 0, Y: 1}}},
				{Type: "L", Points: []drawing.Point{{X: 0, Y: 0}}},
				{Type: "Z"},
			}}},
		},
	}
	if saxDegenerateCustomGeometryIsConnector(shape) {
		t.Fatal("closed degenerate custom shape was misclassified as connector")
	}
}
