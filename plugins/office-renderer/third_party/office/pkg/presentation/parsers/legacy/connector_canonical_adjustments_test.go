package legacy

import (
	"testing"

	officeartir "office/pkg/officeart/ir"
	presentationir "office/pkg/presentation/ir"
)

func TestCanonicalLegacyBentConnector3UsesMidpointForMissingFirstAdjustment(t *testing.T) {
	shared := &officeartir.Shape{Geometry: officeartir.Geometry{
		Preset:            "bentConnector3",
		Adjustments:       []float64{0, 75600, -124200},
		AdjustmentPresent: []bool{false, true, true},
	}}
	got := canonicalLegacyPptConnectorAdjustments(0x22, shared)
	if len(got) != 1 || got[0] != 50000 {
		t.Fatalf("adjustments = %v, want [50000]", got)
	}
}

func TestCanonicalLegacyBentConnector3ScalesAuthoredGuideWithoutClamping(t *testing.T) {
	for _, tc := range []struct {
		raw  float64
		want int
	}{{149, 690}, {4669, 21616}, {-35, -162}, {21712, 100519}, {21562, 99824}, {21673, 100338}} {
		shared := &officeartir.Shape{Geometry: officeartir.Geometry{
			Preset:            "bentConnector3",
			Adjustments:       []float64{tc.raw},
			AdjustmentPresent: []bool{true},
		}}
		got := canonicalLegacyPptConnectorAdjustments(0x22, shared)
		if len(got) != 1 || got[0] != tc.want {
			t.Fatalf("raw %.0f => %v, want [%d]", tc.raw, got, tc.want)
		}
	}
}

func TestCanonicalizeLegacyPptConnectorEndpointOwnershipAccountsForRotation(t *testing.T) {
	shape := &presentationir.Shape{
		Kind:          presentationir.ShapeConnector,
		Bounds:        presentationir.Rect{X: 0, Y: 0, W: 100 * 12700, H: 50 * 12700},
		ConnectorType: "straightConnector1",
		RotationDeg:   90,
		ConnectorStart: &presentationir.Point{
			X: 75 * 12700, Y: -25 * 12700,
		},
		ConnectorEnd: &presentationir.Point{
			X: 25 * 12700, Y: 75 * 12700,
		},
	}
	canonicalizeLegacyPptConnectorEndpointOwnership(shape)
	if shape.ConnectorStart != nil || shape.ConnectorEnd != nil {
		t.Fatalf("rotated preset endpoints were retained: start=%#v end=%#v", shape.ConnectorStart, shape.ConnectorEnd)
	}
}

func TestCanonicalizeLegacyPptConnectorEndpointOwnershipKeepsSolverAttachment(t *testing.T) {
	shape := &presentationir.Shape{
		Kind:          presentationir.ShapeConnector,
		Bounds:        presentationir.Rect{X: 0, Y: 0, W: 100 * 12700, H: 50 * 12700},
		ConnectorType: "straightConnector1",
		RotationDeg:   90,
		ConnectorStart: &presentationir.Point{
			X: 70 * 12700, Y: -25 * 12700,
		},
		ConnectorEnd: &presentationir.Point{
			X: 25 * 12700, Y: 75 * 12700,
		},
	}
	canonicalizeLegacyPptConnectorEndpointOwnership(shape)
	if shape.ConnectorStart == nil || shape.ConnectorEnd == nil {
		t.Fatalf("non-redundant solver endpoint was discarded")
	}
}
