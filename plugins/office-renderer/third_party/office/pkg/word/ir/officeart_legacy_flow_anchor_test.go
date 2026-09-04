package ir

import "testing"

func TestShapePreservesLegacyFlowAnchorContract(t *testing.T) {
	shape := Shape{LegacyFlowAnchor: true}
	if !shape.LegacyFlowAnchor {
		t.Fatal("LegacyFlowAnchor was not preserved")
	}
}
