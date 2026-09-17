package legacy

import (
	"encoding/binary"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

func pptTestSpContainerWithShapeID(id uint32) []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint16(b[2:4], legacyOfficeArtSp)
	binary.LittleEndian.PutUint32(b[4:8], 8)
	binary.LittleEndian.PutUint32(b[8:12], id)
	return b
}

func pptTestShapeStartEvent(t *testing.T, id uint32) OfficeArtEvent {
	t.Helper()
	payload := pptTestSpContainerWithShapeID(id)
	node, err := officeartescher.ParseShapeContainerPayload(payload)
	if err != nil {
		t.Fatalf("parse synthetic shape %d: %v", id, err)
	}
	return OfficeArtEvent{ShapeStart: true, Payload: payload, ShapeNode: node}
}

func TestShapeContainerSceneOwnsShapeIdentityAndGroupAncestry(t *testing.T) {
	events := []OfficeArtEvent{
		{GroupStart: true},
		pptTestShapeStartEvent(t, 41),
		{GroupStart: true},
		pptTestShapeStartEvent(t, 42),
		{GroupEnd: true},
		{GroupEnd: true},
	}
	scene := decodeLegacyPptShapeContainerScene(events)
	got := scene.shapeContainers()
	if len(got) != 2 {
		t.Fatalf("containers = %d, want 2", len(got))
	}
	if got[0].ShapeID != 41 || got[0].GroupDepth != 1 {
		t.Fatalf("first = %+v", got[0])
	}
	if got[1].ShapeID != 42 || got[1].GroupDepth != 2 {
		t.Fatalf("second = %+v", got[1])
	}
	if len(got[1].GroupPath) != 2 || got[1].GroupPath[0] == got[1].GroupPath[1] {
		t.Fatalf("nested group path = %v", got[1].GroupPath)
	}
}
