package legacy

import (
	"testing"

	"office/pkg/officeart/ir"
)

func TestMapRectThroughGroup(t *testing.T) {
	frame := GroupFrame{
		Domain: ir.Rect{X: 0, Y: 0, W: 100, H: 200},
		Target: ir.Rect{X: 10, Y: 20, W: 200, H: 400},
		Valid:  true,
	}
	got, ok := MapRectThroughGroup(ir.Rect{X: 25, Y: 50, W: 50, H: 100}, frame)
	if !ok {
		t.Fatal("map failed")
	}
	want := ir.Rect{X: 60, Y: 120, W: 100, H: 200}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestComposeGroupChildrenMapsChildRectIntoTargetBounds(t *testing.T) {
	child := &ir.Shape{Transform: ir.Transform{
		Rect: ir.Rect{X: 25, Y: 50, W: 50, H: 100},
	}}
	ComposeGroupChildren([]*ir.Shape{child}, GroupFrame{
		Domain: ir.Rect{X: 0, Y: 0, W: 100, H: 200},
		Target: ir.Rect{X: 10, Y: 20, W: 200, H: 400},
		Valid:  true,
	})
	want := ir.Rect{X: 60, Y: 120, W: 100, H: 200}
	if child.Transform.Rect != want {
		t.Fatalf("child rect = %+v, want %+v", child.Transform.Rect, want)
	}
}

func TestComposeGroupChildrenLeavesInvalidFrameUntouched(t *testing.T) {
	orig := ir.Rect{X: 5, Y: 6, W: 7, H: 8}
	child := &ir.Shape{Transform: ir.Transform{Rect: orig}}
	ComposeGroupChildren([]*ir.Shape{child}, GroupFrame{})
	if child.Transform.Rect != orig {
		t.Fatalf("child rect changed: %+v", child.Transform.Rect)
	}
}
