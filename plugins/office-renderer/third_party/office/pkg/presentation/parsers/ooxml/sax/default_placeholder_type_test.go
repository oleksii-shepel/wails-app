package sax

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestPlaceholderKindDefaultsMissingTypeByIndex(t *testing.T) {
	if got := placeholderKind(""); got != ir.PlaceholderTitle {
		t.Fatalf("placeholderKind(empty) = %v, want title placeholder", got)
	}
	if got := placeholderKind("", 2); got != ir.PlaceholderBody {
		t.Fatalf("placeholderKind(empty, idx=2) = %v, want body placeholder", got)
	}
}

func TestMissingPlaceholderTypeStillInheritsLayoutTransformByIndex(t *testing.T) {
	target := &ir.Shape{
		Placeholder:    placeholderKind("", 2),
		PlaceholderIdx: 2,
	}
	layout := []*ir.Shape{{
		Placeholder:    placeholderKind("", 2),
		PlaceholderIdx: 2,
		HasTransform:   true,
		Bounds:         ir.Rect{X: 4648200, Y: 1481328, W: 4038600, H: 4525963},
	}}

	inheritShapeTree(target, layout, nil)

	want := ir.Rect{X: 4648200, Y: 1481328, W: 4038600, H: 4525963}
	if !target.HasTransform || target.Bounds != want {
		t.Fatalf("generic placeholder transform = has=%v bounds=%#v, want has=true bounds=%#v", target.HasTransform, target.Bounds, want)
	}
}

func TestPlaceholderIndexCollisionDoesNotCrossSemanticRoles(t *testing.T) {
	target := &ir.Shape{Placeholder: ir.PlaceholderContent, PlaceholderIdx: 2}
	master := []*ir.Shape{
		{Placeholder: ir.PlaceholderDateTime, PlaceholderIdx: 2, HasTransform: true, Bounds: ir.Rect{X: 1, Y: 2, W: 3, H: 4}, TextBodyProps: drawing.TextBodyProperties{Anchor: "b"}},
		{Placeholder: ir.PlaceholderBody, PlaceholderIdx: 1, HasTransform: true, Bounds: ir.Rect{X: 10, Y: 20, W: 30, H: 40}, TextBodyProps: drawing.TextBodyProperties{Anchor: "t"}},
	}
	inheritShapeTree(target, nil, master)
	if target.TextBodyProps.Anchor == "b" || target.Bounds == (ir.Rect{X: 1, Y: 2, W: 3, H: 4}) {
		t.Fatalf("content placeholder inherited date/time semantics through idx collision: %#v", target)
	}
}
