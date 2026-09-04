package legacy

import (
	"encoding/binary"
	"testing"
)

func legacySharedEscherTestRecord(ver uint16, inst uint16, typ uint16, body []byte) []byte {
	out := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint16(out[0:2], (inst<<4)|(ver&0xF))
	binary.LittleEndian.PutUint16(out[2:4], typ)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(body)))
	copy(out[8:], body)
	return out
}

func TestLegacyCollectOfficeArtShapePropsCompatibilityBridge(t *testing.T) {
	fspBody := make([]byte, 8)
	binary.LittleEndian.PutUint32(fspBody[0:4], 1234)
	fsp := legacySharedEscherTestRecord(2, 75, legacyOfficeArtFSPRecord, fspBody)
	sp := legacySharedEscherTestRecord(0xF, 0, legacyOfficeArtSpContainer, fsp)

	got := map[uint32]legacyOfficeArtShapeProps{}
	legacyCollectOfficeArtShapeProps(sp, got)
	props, ok := got[1234]
	if !ok {
		t.Fatalf("shape 1234 missing: %#v", got)
	}
	if props.ShapeType != 75 {
		t.Fatalf("shape type = %d, want 75", props.ShapeType)
	}
}
