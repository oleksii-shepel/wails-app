package escher

import (
	"encoding/binary"
	"testing"
)

func makeRecord(ver uint8, inst uint16, typ uint16, body []byte) []byte {
	out := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint16(out[0:2], (inst<<4)|uint16(ver))
	binary.LittleEndian.PutUint16(out[2:4], typ)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(body)))
	copy(out[8:], body)
	return out
}

func TestParseRecordsContainer(t *testing.T) {
	child := makeRecord(2, 7, RecordFSP, make([]byte, 8))
	container := makeRecord(ContainerVersion, 0, RecordSpContainer, child)

	records, err := ParseRecords(container)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || len(records[0].Children) != 1 {
		t.Fatalf("records = %#v", records)
	}
	if records[0].Children[0].Header.Type != RecordFSP {
		t.Fatalf("child type = 0x%04X", records[0].Children[0].Header.Type)
	}
}
