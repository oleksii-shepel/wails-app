package layout

import (
	"office/pkg/presentation/ir"
	"testing"
)

func TestMaterializePhysicalVisualTreeClosesHierarchyAndPreservesPaintOrder(t *testing.T) {
	group := &ir.Shape{Kind: ir.ShapeGroup, GroupTransform: &ir.VisualTransform{
		Offset: ir.Point{X: 100, Y: 200}, Extent: ir.VisualSize{W: 1000, H: 1000},
		ChildOffset: ir.Point{}, ChildExtent: ir.VisualSize{W: 100, H: 100},
	}, Children: []*ir.Shape{
		{Kind: ir.ShapeText, Bounds: ir.Rect{X: 10, Y: 10, W: 20, H: 20}},
		{Kind: ir.ShapeImage, Bounds: ir.Rect{X: 40, Y: 40, W: 20, H: 20}},
	}}
	got := MaterializePhysicalVisualTree([]*ir.Shape{group})
	if len(got) != 2 || got[0].Drawing.Paint.ZIndex != 1 || got[1].Drawing.Paint.ZIndex != 2 {
		t.Fatalf("physical paint order = %#v", got)
	}
	if got[0].Shape.Kind == ir.ShapeGroup || got[1].Shape.Kind == ir.ShapeGroup {
		t.Fatal("physical tree leaked canonical group node")
	}
	if got[1].Image == nil {
		t.Fatal("image leaf missing layout-owned image plan")
	}
	if group.Children[0].Bounds.X != 10 {
		t.Fatal("physical materialization mutated canonical child")
	}
}

func TestMaterializePhysicalVisualTreeOwnsConnectorAndEffectOverflow(t *testing.T) {
	connector := &ir.Shape{Kind: ir.ShapeConnector, ConnectorStart: &ir.Point{X: 0, Y: 0}, ConnectorEnd: &ir.Point{X: 914400, Y: 0}}
	got := MaterializePhysicalVisualTree([]*ir.Shape{connector})
	if len(got) != 1 || got[0].FreeConnector == nil {
		t.Fatalf("free connector plan = %#v", got)
	}
}

func TestMaterializePhysicalVisualTreePrecomputesVectorPrimitives(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeText, Bounds: ir.Rect{W: 12700 * 100, H: 12700 * 50}, Geometry: "ellipse", LineColor: "112233", LineWidthPt: 2}
	got := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(got) != 1 {
		t.Fatalf("physical visuals = %d, want 1", len(got))
	}
	if len(got[0].Drawing.Visual.Primitives) == 0 {
		t.Fatal("shape vector primitives were not precomputed by layout")
	}
}

func TestMaterializePhysicalVisualTreePrecomputesConnectorPrimitives(t *testing.T) {
	shape := &ir.Shape{Kind: ir.ShapeConnector, Bounds: ir.Rect{W: 12700 * 100, H: 12700 * 50}, ConnectorType: "bentConnector3", LineColor: "112233", LineWidthPt: 2}
	got := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(got) != 1 || got[0].Connector == nil {
		t.Fatal("connector physical plan missing")
	}
	if len(got[0].Connector.Primitives) == 0 {
		t.Fatal("connector vector primitives were not precomputed by layout")
	}
}
