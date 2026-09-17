package layout

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestPhysicalLayoutSuppressesEmptyPlaceholderPaint(t *testing.T) {
	empty := &ir.Shape{
		Kind:        ir.ShapeText,
		Placeholder: ir.PlaceholderSubtitle,
		Bounds:      ir.Rect{W: 1270000, H: 635000},
		FillColor:   "FAFAFA",
		Paragraphs:  []*ir.Paragraph{{Runs: []*ir.Run{{Text: ""}}}},
	}
	got := MaterializePhysicalVisualTree([]*ir.Shape{empty})
	if len(got) != 0 {
		t.Fatalf("empty placeholder materialized %d physical visuals, want 0", len(got))
	}
}

func TestPhysicalLayoutKeepsPopulatedPlaceholderPaint(t *testing.T) {
	populated := &ir.Shape{
		Kind:        ir.ShapeText,
		Placeholder: ir.PlaceholderTitle,
		Bounds:      ir.Rect{W: 1270000, H: 635000},
		Paragraphs:  []*ir.Paragraph{{Runs: []*ir.Run{{Text: "Visible title"}}}},
	}
	got := MaterializePhysicalVisualTree([]*ir.Shape{populated})
	if len(got) != 1 {
		t.Fatalf("populated placeholder materialized %d physical visuals, want 1", len(got))
	}
}

func TestPhysicalLayoutKeepsPlaceholderFieldsAndDenseZOrder(t *testing.T) {
	empty := &ir.Shape{Kind: ir.ShapeText, Placeholder: ir.PlaceholderSubtitle, Bounds: ir.Rect{W: 1, H: 1}}
	field := &ir.Shape{
		Kind:        ir.ShapeText,
		Placeholder: ir.PlaceholderSlideNumber,
		Bounds:      ir.Rect{W: 1, H: 1},
		Paragraphs:  []*ir.Paragraph{{Runs: []*ir.Run{{FieldKind: "slidenum"}}}},
	}
	ordinary := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{W: 1, H: 1}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "x"}}}}}
	got := MaterializePhysicalVisualTree([]*ir.Shape{empty, field, ordinary})
	if len(got) != 2 {
		t.Fatalf("materialized %d physical visuals, want 2", len(got))
	}
	if got[0].Drawing.Paint.ZIndex != 1 || got[1].Drawing.Paint.ZIndex != 2 {
		t.Fatalf("z-order = %d,%d, want dense 1,2", got[0].Drawing.Paint.ZIndex, got[1].Drawing.Paint.ZIndex)
	}
}
