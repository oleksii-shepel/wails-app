package ir

import (
	"testing"
)

func TestMapVisualRectUsesCanonicalChildCoordinateSpace(t *testing.T) {
	group := VisualTransform{
		Offset: Point{X: 1000, Y: 2000}, Extent: VisualSize{W: 4000, H: 2000},
		ChildOffset: Point{X: 10, Y: 20}, ChildExtent: VisualSize{W: 200, H: 100},
	}
	got := MapVisualRect(Rect{X: 10, Y: 20, W: 50, H: 25}, group)
	want := Rect{X: 1000, Y: 2000, W: 1000, H: 500}
	if got != want {
		t.Fatalf("MapVisualRect() = %+v, want %+v", got, want)
	}
}

func TestApplyVisualTransformComposesReflectionRotationAndConnectorPath(t *testing.T) {
	shape := &Shape{
		Bounds: Rect{X: 0, Y: 0, W: 100, H: 50}, RotationDeg: 20,
		ConnectorStart: &Point{X: 0, Y: 0}, ConnectorEnd: &Point{X: 100, Y: 50},
		ConnectorPath: []Point{{X: 25, Y: 10}, {X: 75, Y: 40}},
	}
	group := VisualTransform{
		Offset: Point{X: 0, Y: 0}, Extent: VisualSize{W: 100, H: 50},
		ChildExtent: VisualSize{W: 100, H: 50}, Rotation: 90, FlipH: true,
	}
	ApplyVisualTransform(shape, group)
	if shape.RotationDeg != 70 {
		t.Fatalf("rotation = %v, want 70", shape.RotationDeg)
	}
	if !shape.FlipH || shape.FlipV {
		t.Fatalf("flip = (%v,%v), want (true,false)", shape.FlipH, shape.FlipV)
	}
	if len(shape.ConnectorPath) != 2 {
		t.Fatalf("connector path len = %d, want 2", len(shape.ConnectorPath))
	}
	if shape.ConnectorPath[0] == (Point{X: 25, Y: 10}) {
		t.Fatal("connector path was not composed")
	}
}
