package render

import (
	"strings"
	"testing"

	"office/pkg/presentation/ir"
)

func TestHTMLSerializesLayoutOwnedAncestorEllipseClip(t *testing.T) {
	group := &ir.Shape{
		Kind:     ir.ShapeGroup,
		Geometry: "ellipse",
		GroupTransform: &ir.VisualTransform{
			Offset:      ir.Point{X: 127000, Y: 127000},
			Extent:      ir.VisualSize{W: 254000, H: 254000},
			ChildOffset: ir.Point{},
			ChildExtent: ir.VisualSize{W: 254000, H: 254000},
		},
		Children: []*ir.Shape{{
			Kind:       ir.ShapeText,
			Bounds:     ir.Rect{X: 0, Y: 0, W: 254000, H: 254000},
			Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "clipped"}}}},
		}},
	}

	var b strings.Builder
	ctx := &htmlRenderContext{}
	renderVisualShapes(&b, []*ir.Shape{group}, 100, 100, ctx)
	got := b.String()
	if !strings.Contains(got, `class="shape-ancestor-clip"`) {
		t.Fatalf("missing ancestor clip wrapper: %s", got)
	}
	if !strings.Contains(got, `clip-path:polygon(`) {
		t.Fatalf("missing layout-materialized ellipse polygon clip: %s", got)
	}
	if !strings.Contains(got, `>clipped<`) {
		t.Fatalf("clipped child was not rendered: %s", got)
	}
}

func TestHTMLSerializesNestedAncestorClipsAsIntersectingWrappers(t *testing.T) {
	leaf := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{W: 127000, H: 127000}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "nested"}}}}}
	inner := &ir.Shape{
		Kind: ir.ShapeGroup, Geometry: "ellipse",
		GroupTransform: &ir.VisualTransform{Extent: ir.VisualSize{W: 127000, H: 127000}, ChildExtent: ir.VisualSize{W: 127000, H: 127000}},
		Children:       []*ir.Shape{leaf},
	}
	outer := &ir.Shape{
		Kind: ir.ShapeGroup, Geometry: "ellipse",
		GroupTransform: &ir.VisualTransform{Extent: ir.VisualSize{W: 254000, H: 254000}, ChildExtent: ir.VisualSize{W: 254000, H: 254000}},
		Children:       []*ir.Shape{inner},
	}

	var b strings.Builder
	renderVisualShapes(&b, []*ir.Shape{outer}, 100, 100, &htmlRenderContext{})
	if got := strings.Count(b.String(), `class="shape-ancestor-clip"`); got != 2 {
		t.Fatalf("ancestor clip wrappers = %d, want 2: %s", got, b.String())
	}
}

func TestHTMLSerializesTransformedAncestorClipFromPhysicalPolygon(t *testing.T) {
	group := &ir.Shape{
		Kind: ir.ShapeGroup, Geometry: "ellipse",
		GroupTransform: &ir.VisualTransform{
			Offset:      ir.Point{X: 127000, Y: 127000},
			Extent:      ir.VisualSize{W: 254000, H: 127000},
			ChildExtent: ir.VisualSize{W: 254000, H: 127000},
			Rotation:    30,
		},
		Children: []*ir.Shape{{Kind: ir.ShapeText, Bounds: ir.Rect{W: 254000, H: 127000}, Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "rotated clip"}}}}}},
	}
	var b strings.Builder
	renderVisualShapes(&b, []*ir.Shape{group}, 100, 100, &htmlRenderContext{})
	got := b.String()
	if !strings.Contains(got, "clip-path:polygon(") {
		t.Fatalf("transformed ancestor clip did not serialize layout-owned polygon: %s", got)
	}
}
