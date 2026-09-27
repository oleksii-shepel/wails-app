package render

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
)

func TestSVGPathDataPreservesEveryVertexInCompactedLineCommand(t *testing.T) {
	path := drawing.Path{Commands: []drawing.PathCommand{
		{Type: "M", Points: []drawing.Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []drawing.Point{{X: 20, Y: 0}, {X: 0, Y: 10}, {X: 20, Y: 10}}},
		{Type: "Z"},
	}}
	got := svgPathData(path)
	want := "M 0.0000 0.0000 L 20.0000 0.0000 0.0000 10.0000 20.0000 10.0000 Z"
	if got != want {
		t.Fatalf("svg path = %q, want %q", got, want)
	}
}

func TestSVGPathDataPreservesImplicitLineVerticesAfterMove(t *testing.T) {
	path := drawing.Path{Commands: []drawing.PathCommand{
		{Type: "M", Points: []drawing.Point{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}}},
	}}
	got := svgPathData(path)
	if !strings.Contains(got, "M 1.0000 2.0000 3.0000 4.0000 5.0000 6.0000") {
		t.Fatalf("multi-point move lost implicit line vertices: %q", got)
	}
}
