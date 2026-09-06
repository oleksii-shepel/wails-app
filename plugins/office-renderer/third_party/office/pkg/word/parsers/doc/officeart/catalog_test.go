package officeart

import (
	"encoding/binary"
	"testing"
)

func TestDecodeStreamsIndexesShape(t *testing.T) {
	// Minimal SpContainer with FSP record: version/instance=0x0002, type=0xF00A,
	// payload is spid + flags. The shared Escher frontend owns interpretation.
	fsp := make([]byte, 16)
	binary.LittleEndian.PutUint16(fsp[0:2], 0x0002)
	binary.LittleEndian.PutUint16(fsp[2:4], 0xF00A)
	binary.LittleEndian.PutUint32(fsp[4:8], 8)
	binary.LittleEndian.PutUint32(fsp[8:12], 42)

	container := make([]byte, 8+len(fsp))
	binary.LittleEndian.PutUint16(container[0:2], 0x000F)
	binary.LittleEndian.PutUint16(container[2:4], 0xF004)
	binary.LittleEndian.PutUint32(container[4:8], uint32(len(fsp)))
	copy(container[8:], fsp)

	catalog := DecodeStreams(container)
	shape, ok := catalog.Find(42)
	if !ok || shape == nil { t.Fatal("shape 42 not indexed") }
	if shape.ID != 42 { t.Fatalf("shape ID=%d", shape.ID) }
}

func TestDecodeStreamsReconstructsSpgrSiblingGroup(t *testing.T) {
	fsp := func(shapeID uint32, flags uint32) []byte {
		payload := make([]byte, 8)
		binary.LittleEndian.PutUint32(payload[0:4], shapeID)
		binary.LittleEndian.PutUint32(payload[4:8], flags)
		return officeArtTestRecord(0x0002, 0xF00A, payload)
	}
	sp := func(shapeID uint32, flags uint32, childAnchor []byte) []byte {
		payload := append([]byte(nil), fsp(shapeID, flags)...)
		if len(childAnchor) > 0 {
			payload = append(payload, officeArtTestRecord(0x0000, 0xF00F, childAnchor)...)
		}
		return officeArtTestRecord(0x000F, 0xF004, payload)
	}
	anchor := make([]byte, 16)
	binary.LittleEndian.PutUint32(anchor[0:4], 20)
	binary.LittleEndian.PutUint32(anchor[4:8], 30)
	binary.LittleEndian.PutUint32(anchor[8:12], 160)
	binary.LittleEndian.PutUint32(anchor[12:16], 150)

	root := sp(100, 0x0205, nil) // have-anchor + group + patriarch
	child := sp(101, 0x0202, anchor) // have-anchor + child
	group := officeArtTestRecord(0x000F, 0xF003, append(root, child...))

	catalog := DecodeStreams(group)
	rootShape, ok := catalog.Find(100)
	if !ok || rootShape == nil {
		t.Fatal("group root not indexed")
	}
	if len(rootShape.Children) != 1 {
		t.Fatalf("group children=%d, want 1", len(rootShape.Children))
	}
	if rootShape.Children[0].ID != 101 {
		t.Fatalf("child ID=%d, want 101", rootShape.Children[0].ID)
	}
	if rootShape.Children[0].Transform.Rect.W <= 0 || rootShape.Children[0].Transform.Rect.H <= 0 {
		t.Fatalf("child anchor not projected: %+v", rootShape.Children[0].Transform.Rect)
	}
}

func officeArtTestRecord(verInst, recType uint16, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint16(out[0:2], verInst)
	binary.LittleEndian.PutUint16(out[2:4], recType)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(payload)))
	copy(out[8:], payload)
	return out
}

func TestEmptyCatalog(t *testing.T) {
	catalog := DecodeStreams(nil, []byte{})
	if catalog.Shapes != nil { t.Fatalf("shapes=%v", catalog.Shapes) }
	if flows := catalog.TextFlows(); flows != nil { t.Fatalf("flows=%v", flows) }
}
