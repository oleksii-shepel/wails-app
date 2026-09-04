package legacy

import (
	"encoding/binary"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

func xlsTestEscherRecord(version uint8, instance, typ uint16, payload []byte) []byte {
	out := make([]byte, officeartescher.HeaderSize+len(payload))
	binary.LittleEndian.PutUint16(out[0:2], (instance<<4)|uint16(version&0x0F))
	binary.LittleEndian.PutUint16(out[2:4], typ)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(payload)))
	copy(out[8:], payload)
	return out
}

func TestWalkLegacyOfficeArtRecordsPassesSpContainerPayload(t *testing.T) {
	fsp := make([]byte, 8)
	binary.LittleEndian.PutUint32(fsp[0:4], 42)
	binary.LittleEndian.PutUint32(fsp[4:8], 0)

	children := xlsTestEscherRecord(0x02, 75, officeartescher.RecordFSP, fsp)
	children = append(children, xlsTestEscherRecord(0x00, 0, officeartescher.RecordClientAnchor, make([]byte, 18))...)
	stream := xlsTestEscherRecord(officeartescher.ContainerVersion, 0, officeartescher.RecordSpContainer, children)

	var got []byte
	walkLegacyOfficeArtRecords(stream, func(recType, _, _ uint16, body []byte) {
		if recType == legacyOfficeArtSpContainer {
			got = append([]byte(nil), body...)
		}
	})
	if len(got) == 0 {
		t.Fatal("SpContainer payload was empty")
	}
	node, err := officeartescher.ParseShapeContainerPayload(got)
	if err != nil {
		t.Fatal(err)
	}
	if node.ShapeID != 42 || node.ShapeType != 75 {
		t.Fatalf("node=(id:%d type:%d), want (42,75)", node.ShapeID, node.ShapeType)
	}
	if len(node.ClientAnchor) == 0 {
		t.Fatal("client anchor disappeared from raw SpContainer payload")
	}
}
