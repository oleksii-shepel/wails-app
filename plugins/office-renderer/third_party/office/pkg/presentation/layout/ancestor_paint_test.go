package layout

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestPhysicalPaintPreservesNestedGroupEnvelopes(t *testing.T) {
	leaf := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{X: 10, Y: 10, W: 20, H: 20}}
	nested := &ir.Shape{
		Kind: ir.ShapeGroup,
		GroupTransform: &ir.VisualTransform{
			Offset: ir.Point{X: 50, Y: 50}, Extent: ir.VisualSize{W: 200, H: 200},
			ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 200, H: 200},
		},
		Children: []*ir.Shape{leaf},
	}
	root := &ir.Shape{
		Kind: ir.ShapeGroup,
		GroupTransform: &ir.VisualTransform{
			Offset: ir.Point{X: 100, Y: 100}, Extent: ir.VisualSize{W: 400, H: 400},
			ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 400, H: 400},
		},
		Children: []*ir.Shape{nested},
	}

	got := MaterializePhysicalVisualTree([]*ir.Shape{root})
	if len(got) != 1 {
		t.Fatalf("physical leaves = %d, want 1", len(got))
	}
	ancestors := got[0].Drawing.Paint.Ancestors
	if len(ancestors) != 2 {
		t.Fatalf("ancestor paint envelopes = %d, want 2", len(ancestors))
	}
	for i, ancestor := range ancestors {
		if !ancestor.Intersects {
			t.Fatalf("ancestor %d does not intersect child paint: %+v", i, ancestor)
		}
		if !ancestor.ContainsBase {
			t.Fatalf("ancestor %d does not contain child base bounds: %+v", i, ancestor)
		}
	}
	if ancestors[0].Bounds.LeftPt >= ancestors[1].Bounds.LeftPt {
		t.Fatalf("nested envelope was not materialized through parent: outer=%+v nested=%+v", ancestors[0].Bounds, ancestors[1].Bounds)
	}
}

func TestPhysicalPaintAncestorEnvelopesDoNotClipOverflow(t *testing.T) {
	leaf := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{X: 90, Y: 10, W: 20, H: 20}}
	group := &ir.Shape{
		Kind: ir.ShapeGroup,
		GroupTransform: &ir.VisualTransform{
			Offset: ir.Point{}, Extent: ir.VisualSize{W: 100, H: 100},
			ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 100, H: 100},
		},
		Children: []*ir.Shape{leaf},
	}
	got := MaterializePhysicalVisualTree([]*ir.Shape{group})
	if len(got) != 1 || len(got[0].Drawing.Paint.Ancestors) != 1 {
		t.Fatalf("unexpected physical composition: %+v", got)
	}
	ancestor := got[0].Drawing.Paint.Ancestors[0]
	if ancestor.ContainsBase {
		t.Fatalf("group envelope unexpectedly treated as containing overflowing child: %+v", ancestor)
	}
	if !ancestor.Intersects {
		t.Fatalf("overflowing child should still intersect group envelope: %+v", ancestor)
	}
	if got[0].Drawing.Paint.Clip.ClipContent {
		t.Fatal("group envelope must not become an implicit child clip")
	}
}
