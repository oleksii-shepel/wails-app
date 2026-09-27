package render

import (
	"office/pkg/ooxml/drawing"
	"strings"
	"testing"
)

func TestSVGPathDataPreservesEveryCompactedCubicSegment(t *testing.T) {
	p := drawing.Path{Commands: []drawing.PathCommand{{Type: "C", Points: []drawing.Point{
		{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}, {X: 7, Y: 8}, {X: 9, Y: 10}, {X: 11, Y: 12},
	}}}}
	got := svgPathData(p)
	if strings.Count(got, "C ") != 2 || !strings.Contains(got, "11.0000 12.0000") {
		t.Fatalf("compacted cubic path lost a segment: %q", got)
	}
}

func TestSVGPathDataPreservesEveryCompactedQuadraticSegment(t *testing.T) {
	p := drawing.Path{Commands: []drawing.PathCommand{{Type: "Q", Points: []drawing.Point{
		{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}, {X: 7, Y: 8},
	}}}}
	got := svgPathData(p)
	if strings.Count(got, "Q ") != 2 || !strings.Contains(got, "7.0000 8.0000") {
		t.Fatalf("compacted quadratic path lost a segment: %q", got)
	}
}
