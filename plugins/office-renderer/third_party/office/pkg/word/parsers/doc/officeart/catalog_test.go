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

func TestDecodeStreamsNormalizesDOCGroupCoordinateDomain(t *testing.T) {
	fsp := func(shapeID uint32, flags uint32) []byte {
		payload := make([]byte, 8)
		binary.LittleEndian.PutUint32(payload[0:4], shapeID)
		binary.LittleEndian.PutUint32(payload[4:8], flags)
		return officeArtTestRecord(0x0002, 0xF00A, payload)
	}
	childAnchor := func(left, top, right, bottom int32) []byte {
		payload := make([]byte, 16)
		binary.LittleEndian.PutUint32(payload[0:4], uint32(left))
		binary.LittleEndian.PutUint32(payload[4:8], uint32(top))
		binary.LittleEndian.PutUint32(payload[8:12], uint32(right))
		binary.LittleEndian.PutUint32(payload[12:16], uint32(bottom))
		return payload
	}
	sp := func(shapeID uint32, flags uint32, extra ...[]byte) []byte {
		payload := append([]byte(nil), fsp(shapeID, flags)...)
		for _, record := range extra {
			payload = append(payload, record...)
		}
		return officeArtTestRecord(0x000F, 0xF004, payload)
	}

	groupRect := make([]byte, 16)
	groupTop := int32(-1694)
	binary.LittleEndian.PutUint32(groupRect[0:4], 0)
	binary.LittleEndian.PutUint32(groupRect[4:8], uint32(groupTop))
	binary.LittleEndian.PutUint32(groupRect[8:12], 2381)
	binary.LittleEndian.PutUint32(groupRect[12:16], 1587)
	root := sp(1026, 0x0205, officeArtTestRecord(0x0000, 0xF009, groupRect))
	first := sp(1027, 0x0202, officeArtTestRecord(0x0000, 0xF00F, childAnchor(0, -1694, 2380, -108)))
	second := sp(1028, 0x0202, officeArtTestRecord(0x0000, 0xF00F, childAnchor(0, 0, 2380, 1586)))
	group := officeArtTestRecord(0x000F, 0xF003, append(append(root, first...), second...))

	catalog := DecodeStreams(group)
	got, ok := catalog.Find(1026)
	if !ok || got == nil || len(got.Children) != 2 {
		t.Fatalf("group=%+v", got)
	}
	if got.Children[0].Transform.Rect.Y != 0 {
		t.Fatalf("first child y=%d, want normalized 0", got.Children[0].Transform.Rect.Y)
	}
	wantSecondY := int64(1694 * 635)
	if got.Children[1].Transform.Rect.Y != wantSecondY {
		t.Fatalf("second child y=%d, want %d", got.Children[1].Transform.Rect.Y, wantSecondY)
	}
	if got.Transform.Rect.W != int64(2381*635) || got.Transform.Rect.H != int64(3281*635) {
		t.Fatalf("group rect=%+v", got.Transform.Rect)
	}
}
