package escher

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func testEscherRecord(version uint8, instance, typ uint16, payload []byte) []byte {
	out := make([]byte, HeaderSize+len(payload))
	verInst := (instance << 4) | uint16(version&0x0F)
	binary.LittleEndian.PutUint16(out[0:2], verInst)
	binary.LittleEndian.PutUint16(out[2:4], typ)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(payload)))
	copy(out[8:], payload)
	return out
}

func TestParseRecordsPreservesContainerPayloadAndChildren(t *testing.T) {
	fspPayload := make([]byte, 8)
	binary.LittleEndian.PutUint32(fspPayload[0:4], 1234)
	child := testEscherRecord(0x02, 1, RecordFSP, fspPayload)
	container := testEscherRecord(ContainerVersion, 0, RecordSpContainer, child)

	records, err := ParseRecords(container)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records=%d want 1", len(records))
	}
	rec := records[0]
	if !bytes.Equal(rec.Data, child) {
		t.Fatalf("container raw payload lost: got %x want %x", rec.Data, child)
	}
	if len(rec.Children) != 1 || rec.Children[0].Header.Type != RecordFSP {
		t.Fatalf("container children=%#v", rec.Children)
	}

	node, err := ParseShapeContainerPayload(rec.Data)
	if err != nil {
		t.Fatalf("raw container payload no longer consumable: %v", err)
	}
	if node.ShapeID != 1234 {
		t.Fatalf("shape id=%d want 1234", node.ShapeID)
	}
}

func TestWalkExposesRawContainerPayload(t *testing.T) {
	child := testEscherRecord(0x02, 0, RecordClientAnchor, make([]byte, 18))
	container := testEscherRecord(ContainerVersion, 0, RecordSpContainer, child)

	var saw bool
	err := WalkStream(container, func(rec Record) error {
		if rec.Header.Type == RecordSpContainer {
			saw = true
			if len(rec.Data) == 0 {
				t.Fatal("Walk exposed empty container Data")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !saw {
		t.Fatal("SpContainer not visited")
	}
}

func TestScanPreservesContainerPayload(t *testing.T) {
	child := testEscherRecord(0x02, 0, RecordClientData, []byte{1, 2, 3})
	container := testEscherRecord(ContainerVersion, 0, RecordSpContainer, child)
	stream := append([]byte{9, 8, 7}, container...)

	records := Scan(stream, func(h Header) bool { return h.Type == RecordSpContainer })
	if len(records) != 1 {
		t.Fatalf("records=%d want 1", len(records))
	}
	if !bytes.Equal(records[0].Data, child) {
		t.Fatalf("scan container payload=%x want %x", records[0].Data, child)
	}
}
