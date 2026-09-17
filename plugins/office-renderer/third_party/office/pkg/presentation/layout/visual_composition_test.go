package layout

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestMaterializeGroupChildrenOwnsCanonicalGroupFlattening(t *testing.T) {
	transform := ir.VisualTransform{
		Offset:      ir.Point{X: 1000, Y: 2000},
		Extent:      ir.VisualSize{W: 4000, H: 6000},
		ChildOffset: ir.Point{},
		ChildExtent: ir.VisualSize{W: 2000, H: 3000},
	}
	child := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{X: 500, Y: 500, W: 1000, H: 1000}}
	group := &ir.Shape{Kind: ir.ShapeGroup, GroupTransform: &transform, Children: []*ir.Shape{child}}

	got := MaterializeGroupChildren(group)
	if len(got) != 1 {
		t.Fatalf("materialized children = %d, want 1", len(got))
	}
	if got[0].Bounds != (ir.Rect{X: 2000, Y: 3000, W: 2000, H: 2000}) {
		t.Fatalf("materialized bounds = %+v", got[0].Bounds)
	}
	if child.Bounds != (ir.Rect{X: 500, Y: 500, W: 1000, H: 1000}) {
		t.Fatalf("canonical IR was mutated: %+v", child.Bounds)
	}
}

func TestMaterializeGroupChildrenComposesNestedCoordinateSystemsInOrder(t *testing.T) {
	outerXfrm := ir.VisualTransform{
		Offset: ir.Point{X: 1000, Y: 1000}, Extent: ir.VisualSize{W: 4000, H: 4000},
		ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 2000, H: 2000},
	}
	innerXfrm := ir.VisualTransform{
		Offset: ir.Point{X: 200, Y: 300}, Extent: ir.VisualSize{W: 1000, H: 1000},
		ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 1000, H: 1000},
	}
	leaf := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{X: 100, Y: 100, W: 100, H: 100}}
	inner := &ir.Shape{Kind: ir.ShapeGroup, GroupTransform: &innerXfrm, Children: []*ir.Shape{leaf}}
	outer := &ir.Shape{Kind: ir.ShapeGroup, GroupTransform: &outerXfrm, Children: []*ir.Shape{inner}}

	got := MaterializeGroupChildren(outer)
	if len(got) != 1 || len(got[0].Children) != 1 {
		t.Fatalf("nested composition structure was lost")
	}
	materializedLeaf := got[0].Children[0]
	// inner: (100,100) -> (300,400); outer scale 2 + offset -> (1600,1800)
	if materializedLeaf.Bounds != (ir.Rect{X: 1600, Y: 1800, W: 200, H: 200}) {
		t.Fatalf("nested materialized bounds = %+v", materializedLeaf.Bounds)
	}
	if inner.GroupTransform == nil || leaf.Bounds.X != 100 {
		t.Fatalf("canonical nested IR was mutated")
	}
}

func TestMaterializeVisualTreeFlattensNestedGroupsInPaintOrder(t *testing.T) {
	outerT := ir.VisualTransform{Offset: ir.Point{X: 1000, Y: 2000}, Extent: ir.VisualSize{W: 2000, H: 1000}, ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 1000, H: 1000}}
	innerT := ir.VisualTransform{Offset: ir.Point{X: 100, Y: 200}, Extent: ir.VisualSize{W: 500, H: 500}, ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 500, H: 500}}
	first := &ir.Shape{ID: 1, Kind: ir.ShapeText, Bounds: ir.Rect{X: 0, Y: 0, W: 100, H: 100}}
	second := &ir.Shape{ID: 2, Kind: ir.ShapeImage, Bounds: ir.Rect{X: 100, Y: 100, W: 100, H: 100}}
	inner := &ir.Shape{ID: 20, Kind: ir.ShapeGroup, GroupTransform: &innerT, Children: []*ir.Shape{second}}
	outer := &ir.Shape{ID: 10, Kind: ir.ShapeGroup, GroupTransform: &outerT, Children: []*ir.Shape{first, inner}}

	got := MaterializeVisualTree([]*ir.Shape{outer})
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("materialized paint order = %#v, want leaf ids [1 2]", []int{got[0].ID, got[1].ID})
	}
	if got[0].Kind == ir.ShapeGroup || got[1].Kind == ir.ShapeGroup {
		t.Fatal("layout leaked canonical group node into physical visual tree")
	}
	if got[1].GroupTransform != nil || len(got[1].Children) != 0 {
		t.Fatal("materialized leaf retained canonical composition state")
	}
}

func TestMaterializeVisualTreeDoesNotMutateCanonicalIR(t *testing.T) {
	transform := ir.VisualTransform{Offset: ir.Point{X: 1000, Y: 1000}, Extent: ir.VisualSize{W: 1000, H: 1000}, ChildExtent: ir.VisualSize{W: 1000, H: 1000}, Rotation: 90, FlipH: true}
	child := &ir.Shape{ID: 7, Kind: ir.ShapeConnector, Bounds: ir.Rect{X: 10, Y: 20, W: 100, H: 50}, ConnectorStart: &ir.Point{X: 10, Y: 20}, ConnectorEnd: &ir.Point{X: 110, Y: 70}}
	group := &ir.Shape{Kind: ir.ShapeGroup, GroupTransform: &transform, Children: []*ir.Shape{child}}
	beforeBounds := child.Bounds
	beforeStart := *child.ConnectorStart

	got := MaterializeVisualTree([]*ir.Shape{group})
	if len(got) != 1 {
		t.Fatalf("materialized leaves = %d, want 1", len(got))
	}
	if child.Bounds != beforeBounds || *child.ConnectorStart != beforeStart {
		t.Fatal("layout mutated canonical child geometry")
	}
	if group.GroupTransform == nil {
		t.Fatal("layout consumed canonical group transform in-place")
	}
}
