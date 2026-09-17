package layout

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestDegenerateAreaShapeDoesNotMaterializeEffectRectangle(t *testing.T) {
	shape := &ir.Shape{
		Kind:    ir.ShapeText,
		Bounds:  ir.Rect{W: 9144000, H: 0},
		Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Shadow{Color: "000000", Blur: 4, Distance: 3}}},
	}
	if got := MaterializePhysicalVisualTree([]*ir.Shape{shape}); len(got) != 0 {
		t.Fatalf("zero-height text shape materialized %d visuals, want none", len(got))
	}
}
