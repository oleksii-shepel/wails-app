package escher

import (
	"encoding/binary"
	"testing"
)

func TestShapeTreeRecognizesClientTextbox(t *testing.T) {
	fspBody := make([]byte, 8)
	binary.LittleEndian.PutUint32(fspBody[0:4], 0x11223344)
	fsp := makeRecord(2, 1, RecordFSP, fspBody)
	txbx := makeRecord(0, 0, RecordClientTextbox, nil)
	sp := makeRecord(ContainerVersion, 0, RecordSpContainer, append(fsp, txbx...))

	records, err := ParseRecords(sp)
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := ShapeTree(records)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes = %d", len(nodes))
	}
	if nodes[0].ShapeID != 0x11223344 || !nodes[0].HasClientTextbox {
		t.Fatalf("node = %#v", nodes[0])
	}
}

func TestParseShapeContainerPayloadKeepsApplicationPayloads(t *testing.T) {
	fspBody := make([]byte, 8)
	binary.LittleEndian.PutUint32(fspBody[0:4], 99)
	fsp := makeRecord(2, 5, RecordFSP, fspBody)
	clientAnchor := makeRecord(0, 0, RecordClientAnchor, []byte{1, 2, 3})
	childAnchor := makeRecord(0, 0, RecordChildAnchor, []byte{4, 5, 6})
	clientData := makeRecord(0, 0, RecordClientData, []byte{7})
	textbox := makeRecord(0, 0, RecordClientTextbox, []byte{8, 9})
	payload := append(append(append(append(fsp, clientAnchor...), childAnchor...), clientData...), textbox...)

	node, err := ParseShapeContainerPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if node.ShapeID != 99 || node.ShapeType != 5 || !node.HasClientTextbox {
		t.Fatalf("node = %#v", node)
	}
	if len(node.ClientAnchor) != 3 || len(node.ChildAnchor) != 3 || len(node.ClientData) != 1 || len(node.ClientTextbox) != 2 {
		t.Fatalf("application payloads lost: %#v", node)
	}
}
