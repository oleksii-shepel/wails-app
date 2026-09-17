package layout

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestPhysicalVisualOwnsMaterializedGroupAncestry(t *testing.T) {
	group := &ir.Shape{
		Kind: ir.ShapeGroup,
		GroupTransform: &ir.VisualTransform{
			Offset:      ir.Point{X: 12700, Y: 25400},
			Extent:      ir.VisualSize{W: 127000, H: 127000},
			ChildOffset: ir.Point{},
			ChildExtent: ir.VisualSize{W: 127000, H: 127000},
		},
		Children: []*ir.Shape{{
			Kind:   ir.ShapeText,
			Bounds: ir.Rect{X: 12700, Y: 12700, W: 25400, H: 25400},
		}},
	}

	got := MaterializePhysicalVisualTree([]*ir.Shape{group})
	if len(got) != 1 {
		t.Fatalf("physical leaves = %d, want 1", len(got))
	}
	if got[0].Shape.Kind == ir.ShapeGroup || got[0].Shape.GroupTransform != nil {
		t.Fatal("physical leaf retained canonical group ownership")
	}
	if len(got[0].Drawing.Paint.Ancestors) != 1 {
		t.Fatalf("physical ancestry envelopes = %#v, want one", got[0].Drawing.Paint.Ancestors)
	}
	if got[0].Drawing == nil || got[0].Drawing.FrameBounds.Empty() {
		t.Fatal("group child did not materialize a physical slide-space frame")
	}
}
