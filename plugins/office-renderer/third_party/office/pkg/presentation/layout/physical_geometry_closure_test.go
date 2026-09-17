package layout

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"testing"
)

func TestCustomGeometryMaterializesToPhysicalPrimitivesWithoutCanonicalSpec(t *testing.T) {
	shape := &ir.Shape{
		Bounds: ir.Rect{W: 914400, H: 914400},
		GeometryData: &drawing.Geometry{Type: drawing.GeometryTypeCustom, Paths: []drawing.Path{{Commands: []drawing.PathCommand{
			{Type: "M", Points: []drawing.Point{{X: 0, Y: 0}}},
			{Type: "L", Points: []drawing.Point{{X: 1, Y: 0}, {X: .5, Y: 1}}},
			{Type: "Z"},
		}}}},
		FillColor: "336699",
	}
	plan := ResolveShapeVisualPlan(shape)
	if !plan.Geometry.Custom || !plan.UseVector || len(plan.Primitives) == 0 {
		t.Fatalf("custom geometry did not cross physical boundary: %+v", plan)
	}
	shape.GeometryData.Paths = nil
	if len(plan.Primitives) == 0 {
		t.Fatal("physical geometry aliases canonical custom paths")
	}
}

func TestConnectorPlanOwnsAuthoredGeometryWithoutDrawingSpec(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 1828800, H: 914400}, ConnectorType: "bentConnector3", ConnectorAdjustments: []int{25000, 50000}, ConnectorFlipH: true, ArrowHead: "triangle", ArrowTail: "diamond"}
	plan := ResolveConnectorVisualPlan(shape)
	shape.ConnectorAdjustments[0] = 90000
	if plan.Type != "bentConnector3" || !plan.FlipH || plan.Adjustments[0] != 25000 || plan.ArrowHead != "triangle" || plan.ArrowTail != "diamond" {
		t.Fatalf("physical connector metadata = %+v", plan)
	}
}
