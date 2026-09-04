package legacy

import (
	"encoding/binary"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

func tolerantWalkTestRecord(version uint8, instance, typ uint16, payload []byte) []byte {
	out := make([]byte, officeartescher.HeaderSize+len(payload))
	binary.LittleEndian.PutUint16(out[0:2], (instance<<4)|uint16(version&0x0F))
	binary.LittleEndian.PutUint16(out[2:4], typ)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(payload)))
	copy(out[8:], payload)
	return out
}

func TestWalkLegacyOfficeArtRecordsRecoversContainerFromFragmentedAggregate(t *testing.T) {
	fspPayload := make([]byte, 8)
	binary.LittleEndian.PutUint32(fspPayload[0:4], 77)
	fsp := tolerantWalkTestRecord(0x02, 75, officeartescher.RecordFSP, fspPayload)
	anchor := tolerantWalkTestRecord(0x00, 0, officeartescher.RecordClientAnchor, make([]byte, 18))
	container := tolerantWalkTestRecord(officeartescher.ContainerVersion, 0, officeartescher.RecordSpContainer, append(fsp, anchor...))

	// Bytes before the valid Escher record force strict ParseRecords to fail.
	stream := append([]byte{0xAA, 0xBB, 0xCC}, container...)
	stream = append(stream, 0xDD)

	var payload []byte
	walkLegacyOfficeArtRecords(stream, func(recType, _, _ uint16, body []byte) {
		if recType == legacyOfficeArtSpContainer {
			payload = append([]byte(nil), body...)
		}
	})
	if len(payload) == 0 {
		t.Fatal("fragmented BIFF aggregate dropped the shape container")
	}
	node, err := officeartescher.ParseShapeContainerPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if node.ShapeID != 77 || node.ShapeType != 75 {
		t.Fatalf("shape=(id:%d type:%d), want (77,75)", node.ShapeID, node.ShapeType)
	}
}
