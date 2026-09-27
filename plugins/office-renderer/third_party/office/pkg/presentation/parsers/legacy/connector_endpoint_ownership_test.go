package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestCanonicalizeLegacyPptConnectorEndpointOwnershipDropsRedundantPresetEndpoints(t *testing.T) {
	start := &presentationir.Point{X: 914400, Y: 1828800}
	end := &presentationir.Point{X: 2743200, Y: 2743200}
	shape := &presentationir.Shape{
		Kind:           presentationir.ShapeConnector,
		Bounds:         presentationir.Rect{X: 914400, Y: 1828800, W: 1828800, H: 914400},
		ConnectorType:  "straightConnector1",
		ConnectorStart: start, ConnectorEnd: end,
	}
	canonicalizeLegacyPptConnectorEndpointOwnership(shape)
	if shape.ConnectorStart != nil || shape.ConnectorEnd != nil {
		t.Fatalf("redundant solver endpoints survived: start=%+v end=%+v", shape.ConnectorStart, shape.ConnectorEnd)
	}
}

func TestCanonicalizeLegacyPptConnectorEndpointOwnershipPreservesSolverRoute(t *testing.T) {
	start := &presentationir.Point{X: 914400, Y: 1828800}
	end := &presentationir.Point{X: 2743200, Y: 2743200}
	shape := &presentationir.Shape{
		Kind:          presentationir.ShapeConnector,
		Bounds:        presentationir.Rect{X: 914400, Y: 1828800, W: 1828800, H: 914400},
		ConnectorType: "bentConnector3", ConnectorAdjustments: []int{50000},
		// Move both solver endpoints materially away from the bounded preset tips.
		ConnectorStart: &presentationir.Point{X: start.X + 127000, Y: start.Y},
		ConnectorEnd:   &presentationir.Point{X: end.X - 127000, Y: end.Y},
	}
	canonicalizeLegacyPptConnectorEndpointOwnership(shape)
	if shape.ConnectorStart == nil || shape.ConnectorEnd == nil {
		t.Fatal("non-redundant solver endpoints were discarded")
	}
}

func TestCanonicalizeLegacyPptConnectorEndpointOwnershipDropsRotatedFrameDiagonalCache(t *testing.T) {
	shape := &presentationir.Shape{
		Kind:          presentationir.ShapeConnector,
		Bounds:        presentationir.Rect{X: 1270000, Y: 1270000, W: 1270000, H: 635000}, // 100x50 pt at 100,100
		ConnectorType: "bentConnector3",
		RotationDeg:   -90,
		// PPT solver cache for the opposite local-frame diagonal: local TR -> BL,
		// rotated into slide coordinates. The PPTX conversion keeps only the
		// bounded preset, so these coordinates are representational noise.
		ConnectorStart: &presentationir.Point{X: 1587500, Y: 952500},
		ConnectorEnd:   &presentationir.Point{X: 2222500, Y: 2222500},
	}
	canonicalizeLegacyPptConnectorEndpointOwnership(shape)
	if shape.ConnectorStart != nil || shape.ConnectorEnd != nil {
		t.Fatalf("rotated frame-diagonal solver cache survived: start=%+v end=%+v", shape.ConnectorStart, shape.ConnectorEnd)
	}
}

func TestCanonicalizeLegacyPptConnectorEndpointOwnershipPreservesRotatedInsetAttachment(t *testing.T) {
	shape := &presentationir.Shape{
		Kind:          presentationir.ShapeConnector,
		Bounds:        presentationir.Rect{X: 1270000, Y: 1270000, W: 1270000, H: 635000},
		ConnectorType: "bentConnector3",
		RotationDeg:   -90,
		// One endpoint is inset by 1 pt, so it carries real solver attachment
		// information and must not be collapsed into the bounded preset.
		ConnectorStart: &presentationir.Point{X: 1587500 + 12700, Y: 952500},
		ConnectorEnd:   &presentationir.Point{X: 2222500, Y: 2222500},
	}
	canonicalizeLegacyPptConnectorEndpointOwnership(shape)
	if shape.ConnectorStart == nil || shape.ConnectorEnd == nil {
		t.Fatal("rotated non-corner solver attachment was discarded")
	}
}
