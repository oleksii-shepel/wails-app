package legacy

import (
	"testing"

	officeartir "office/pkg/officeart/ir"
	presentationir "office/pkg/presentation/ir"
)

func TestLegacyPptZeroAxisCustomConnectorKeepsCanonicalLineGeometry(t *testing.T) {
	shape := &presentationir.Shape{
		Kind:   presentationir.ShapeConnector,
		Bounds: presentationir.Rect{X: 10, Y: 20, W: 0, H: 100},
	}
	shared := &officeartir.Shape{Geometry: officeartir.Geometry{
		Kind: officeartir.GeometryCustom,
		Paths: []officeartir.Path{{Commands: []officeartir.PathCommand{
			{Op: "M", Points: []officeartir.PointF{{X: 0, Y: 1}}},
			{Op: "L", Points: []officeartir.PointF{{X: 0, Y: 0}}},
			{Op: "L", Points: []officeartir.PointF{{X: 0, Y: 0}}},
		}}},
	}}

	applyLegacyPptOfficeArtGeometry(shape, shared)
	if shape.Geometry != "line" || shape.GeometryData != nil {
		t.Fatalf("canonical connector geometry = %q data=%#v, want line with no custom data", shape.Geometry, shape.GeometryData)
	}
	if shape.ConnectorType != "straightConnector1" {
		t.Fatalf("connector type = %q, want straightConnector1", shape.ConnectorType)
	}
}

func TestLegacyPptNonDegenerateCustomShapePreservesAuthoredGeometry(t *testing.T) {
	shape := &presentationir.Shape{
		Kind:   presentationir.ShapeText,
		Bounds: presentationir.Rect{W: 100, H: 100},
	}
	shared := &officeartir.Shape{Geometry: officeartir.Geometry{
		Kind: officeartir.GeometryCustom,
		Paths: []officeartir.Path{{Commands: []officeartir.PathCommand{
			{Op: "M", Points: []officeartir.PointF{{X: 0, Y: 0}}},
			{Op: "L", Points: []officeartir.PointF{{X: 1, Y: 1}}},
		}}},
	}}

	applyLegacyPptOfficeArtGeometry(shape, shared)
	if shape.GeometryData == nil || shape.Geometry != "" {
		t.Fatalf("custom geometry = %q data=%#v, want preserved custom data", shape.Geometry, shape.GeometryData)
	}
}
