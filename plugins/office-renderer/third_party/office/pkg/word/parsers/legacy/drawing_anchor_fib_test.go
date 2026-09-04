package legacy

import (
	"encoding/binary"
	"testing"
)

func makeTestPlcfSpa(cp int, shape uint32, left, top, right, bottom int32) []byte {
	// One FSPA: (count+1)*4 CP bytes + 26-byte FSPA = 34 bytes.
	out := make([]byte, 34)
	binary.LittleEndian.PutUint32(out[0:4], uint32(cp))
	// Final CP is undefined by MS-DOC but must be greater than previous.
	binary.LittleEndian.PutUint32(out[4:8], uint32(cp+1))
	off := 8
	binary.LittleEndian.PutUint32(out[off:off+4], shape)
	binary.LittleEndian.PutUint32(out[off+4:off+8], uint32(left))
	binary.LittleEndian.PutUint32(out[off+8:off+12], uint32(top))
	binary.LittleEndian.PutUint32(out[off+12:off+16], uint32(right))
	binary.LittleEndian.PutUint32(out[off+16:off+20], uint32(bottom))
	return out
}

func TestLoadLegacyFloatingDrawingAnchorsPrefersFibPlcSpaMom(t *testing.T) {
	const (
		fibBase  = 16
		fibCount = 60
	)
	word := make([]byte, fibBase+fibCount*8+16)
	table := make([]byte, 128)

	authoritative := makeTestPlcfSpa(10, 1001, 100, 200, 500, 600)
	decoy := makeTestPlcfSpa(10, 9999, 9000, 9000, 9900, 9900)
	copy(table[8:], authoritative)
	copy(table[64:], decoy)

	putPair := func(index, fc, lcb int) {
		off := fibBase + index*8
		binary.LittleEndian.PutUint32(word[off:off+4], uint32(fc))
		binary.LittleEndian.PutUint32(word[off+4:off+8], uint32(lcb))
	}
	putPair(legacyFibPlcSpaMom, 8, len(authoritative))
	// A different FIB field that also happens to contain syntactically valid
	// PlcfSpa bytes and overlaps the observed anchor CP.
	putPair(12, 64, len(decoy))

	got := loadLegacyFloatingDrawingAnchors(word, table, fibBase, fibCount, 100, []int{10})
	anchor, ok := got[10]
	if !ok {
		t.Fatal("missing main-document anchor")
	}
	if anchor.Shape != 1001 {
		t.Fatalf("selected shape=%d, want authoritative shape 1001", anchor.Shape)
	}
	if anchor.Left != 100 || anchor.Top != 200 || anchor.Right != 500 || anchor.Bottom != 600 {
		t.Fatalf("selected decoy bounds: %+v", anchor)
	}
}

func TestLoadLegacyFloatingDrawingAnchorsFallsBackForMalformedFibPair(t *testing.T) {
	const (
		fibBase  = 16
		fibCount = 60
	)
	word := make([]byte, fibBase+fibCount*8+16)
	table := make([]byte, 128)

	decoy := makeTestPlcfSpa(10, 1234, 10, 20, 110, 120)
	copy(table[64:], decoy)

	putPair := func(index, fc, lcb int) {
		off := fibBase + index*8
		binary.LittleEndian.PutUint32(word[off:off+4], uint32(fc))
		binary.LittleEndian.PutUint32(word[off+4:off+8], uint32(lcb))
	}
	// Authoritative entry points outside the table stream.
	putPair(legacyFibPlcSpaMom, 1000, 34)
	putPair(12, 64, len(decoy))

	got := loadLegacyFloatingDrawingAnchors(word, table, fibBase, fibCount, 100, []int{10})
	if got[10].Shape != 1234 {
		t.Fatalf("compatibility fallback failed: %+v", got[10])
	}
}
