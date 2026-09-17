package cascade

import (
	"testing"

	"office/pkg/ooxml/drawingxml"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestApplyShapeGeometryOwnsPresetProjectionWithoutChangingLegacyAdjustmentSemantics(t *testing.T) {
	sp := &parts.ShapeProperties{PrstGeom: &parts.PresetGeometry{
		Prst:  "roundRect",
		AvLst: []drawingxml.GeometryGuide{{Name: "adj", Fmla: "val 25000"}},
	}}
	shape := &ir.Shape{}
	ApplyShapeGeometry(sp, shape)
	if shape.Geometry != "roundRect" {
		t.Fatalf("geometry = %q, want roundRect", shape.Geometry)
	}
	if len(shape.GeometryAdjustments) != 0 {
		t.Fatalf("adjustments = %v, want unchanged legacy semantics", shape.GeometryAdjustments)
	}
}

func TestConnectorGeometryOwnsPresetAdjustmentProjection(t *testing.T) {
	geom := &parts.PresetGeometry{Prst: "bentConnector3", AvLst: []drawingxml.GeometryGuide{{Name: "adj1", Fmla: "val 100000"}}}
	name, adj := ConnectorGeometry(geom)
	if name != "bentConnector3" || len(adj) != 1 || adj[0] != 100000 {
		t.Fatalf("connector geometry = %q %v", name, adj)
	}
}

func TestConnectionPointUsesCanonicalBounds(t *testing.T) {
	b := ir.Rect{X: 100, Y: 200, W: 400, H: 200}
	if got := ConnectionPoint(b, 1); got != (ir.Point{X: 500, Y: 300}) {
		t.Fatalf("right connection = %+v", got)
	}
	if got := ConnectionPoint(b, 3); got != (ir.Point{X: 100, Y: 300}) {
		t.Fatalf("left connection = %+v", got)
	}
}
