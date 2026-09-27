package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func step4GroupChildContainer(shapeID uint32, shapeType uint16, flags uint32, anchor []byte) []byte {
	return append(
		buildOfficeArtRecord(shapeType, legacyOfficeArtSp, 0x0002, officeArtSpPayload(shapeID, flags)),
		buildOfficeArtRecord(0, legacyOfficeArtChildAnchor, 0x0000, anchor)...,
	)
}

func TestStep4GroupCanonicalizationPreservesHorizontalConnectorChild(t *testing.T) {
	child := &presentationir.Shape{Kind: presentationir.ShapeConnector}
	data := step4GroupChildContainer(
		117771,
		20,
		legacyOfficeArtSpFlagChild,
		pass5ChildAnchor(100, 250, 700, 250),
	)

	if !canonicalizeLegacyPptGroupChild(child, data, nil) {
		t.Fatal("canonicalizeLegacyPptGroupChild rejected authored zero-height connector")
	}
	if child.OriginalShapeID != 117771 {
		t.Fatalf("OriginalShapeID = %d, want 117771", child.OriginalShapeID)
	}
	if child.Bounds != (presentationir.Rect{X: 100, Y: 250, W: 600, H: 0}) {
		t.Fatalf("Bounds = %#v, want authored group-local horizontal line", child.Bounds)
	}
	if child.ConnectorStart == nil || child.ConnectorEnd == nil {
		t.Fatalf("connector endpoints = (%v,%v), want authored endpoints", child.ConnectorStart, child.ConnectorEnd)
	}
	if *child.ConnectorStart != (presentationir.Point{X: 100, Y: 250}) ||
		*child.ConnectorEnd != (presentationir.Point{X: 700, Y: 250}) {
		t.Fatalf("connector endpoints = (%#v,%#v), want group-local horizontal endpoints", *child.ConnectorStart, *child.ConnectorEnd)
	}
}

func TestStep4GroupCanonicalizationPreservesVerticalConnectorChild(t *testing.T) {
	child := &presentationir.Shape{Kind: presentationir.ShapeConnector}
	data := step4GroupChildContainer(
		117772,
		20,
		legacyOfficeArtSpFlagChild,
		pass5ChildAnchor(350, 100, 350, 800),
	)

	if !canonicalizeLegacyPptGroupChild(child, data, nil) {
		t.Fatal("canonicalizeLegacyPptGroupChild rejected authored zero-width connector")
	}
	if child.Bounds != (presentationir.Rect{X: 350, Y: 100, W: 0, H: 700}) {
		t.Fatalf("Bounds = %#v, want authored group-local vertical line", child.Bounds)
	}
	if child.ConnectorStart == nil || child.ConnectorEnd == nil {
		t.Fatalf("connector endpoints = (%v,%v), want authored endpoints", child.ConnectorStart, child.ConnectorEnd)
	}
}
