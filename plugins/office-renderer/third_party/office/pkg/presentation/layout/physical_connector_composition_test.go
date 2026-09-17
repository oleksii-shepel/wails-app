package layout

import (
	"testing"

	"office/pkg/presentation/ir"
)

func TestMaterializedPhysicalConnectorCarriesPaintPlan(t *testing.T) {
	shape := &ir.Shape{
		Kind:          ir.ShapeConnector,
		ConnectorType: "straightConnector1",
		Bounds:        ir.Rect{X: 12700, Y: 25400, W: 127000, H: 63500},
	}
	items := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(items) != 1 || items[0].Connector == nil {
		t.Fatalf("expected one materialized bounded connector, got %#v", items)
	}
	if items[0].Connector.Composition.Paint.ZIndex != 1 {
		t.Fatalf("connector paint z-index = %d, want 1", items[0].Connector.Composition.Paint.ZIndex)
	}
}

func TestMaterializedPhysicalFreeConnectorCarriesPaintPlan(t *testing.T) {
	start := ir.Point{X: 10, Y: 20}
	end := ir.Point{X: 100, Y: 80}
	shape := &ir.Shape{Kind: ir.ShapeConnector, ConnectorStart: &start, ConnectorEnd: &end}
	items := MaterializePhysicalVisualTree([]*ir.Shape{shape})
	if len(items) != 1 || items[0].FreeConnector == nil {
		t.Fatalf("expected one materialized free connector, got %#v", items)
	}
	if items[0].FreeConnector.Composition.Paint.ZIndex != 1 {
		t.Fatalf("free connector paint z-index = %d, want 1", items[0].FreeConnector.Composition.Paint.ZIndex)
	}
}

func TestSmartArtConnectorRepresentationIsChosenByLayout(t *testing.T) {
	bounded := &ir.Shape{
		Kind:          ir.ShapeSmartArtConnector,
		ConnectorType: "straightConnector1",
		Bounds:        ir.Rect{W: 127000, H: 63500},
	}
	items := MaterializePhysicalVisualTree([]*ir.Shape{bounded})
	if len(items) != 1 || items[0].Connector == nil || items[0].FreeConnector != nil {
		t.Fatalf("bounded SmartArt connector must materialize as physical connector: %#v", items)
	}

	start := ir.Point{X: 10, Y: 20}
	end := ir.Point{X: 100, Y: 80}
	free := &ir.Shape{Kind: ir.ShapeSmartArtConnector, Bounds: ir.Rect{W: 127000, H: 63500}, ConnectorStart: &start, ConnectorEnd: &end}
	items = MaterializePhysicalVisualTree([]*ir.Shape{free})
	if len(items) != 1 || items[0].FreeConnector == nil || items[0].Connector != nil {
		t.Fatalf("endpoint SmartArt connector must materialize as physical free connector: %#v", items)
	}
}
