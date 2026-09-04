package legacy

import (
	"encoding/binary"
	"testing"
)

func TestLegacyOfficeArtFibContractUsesAuthoritativeIndices(t *testing.T) {
	if legacyFibPlcSpaMom != 40 {
		t.Fatalf("PlcSpaMom index = %d, want 40", legacyFibPlcSpaMom)
	}
	if legacyFibPlcftxbxTxt != 56 {
		t.Fatalf("PlcftxbxTxt index = %d, want 56", legacyFibPlcftxbxTxt)
	}
	if legacyFibPlcfHdrtxbxTxt != 58 {
		t.Fatalf("PlcfHdrtxbxTxt index = %d, want 58", legacyFibPlcfHdrtxbxTxt)
	}
	if legacyFTXBXSSize != 22 || legacyFTXBXSLidOffset != 14 {
		t.Fatalf("FTXBXS contract = size %d lid offset %d, want 22/14",
			legacyFTXBXSSize, legacyFTXBXSLidOffset)
	}
}

func TestParseLegacyTextboxPLCReadsAuthoritativeLid(t *testing.T) {
	// One PLC entry: two CPs followed by one 22-byte FTXBXS.
	blob := make([]byte, 4*2+legacyFTXBXSSize)
	binary.LittleEndian.PutUint32(blob[0:4], 10)
	binary.LittleEndian.PutUint32(blob[4:8], 20)
	binary.LittleEndian.PutUint32(
		blob[8+legacyFTXBXSLidOffset:8+legacyFTXBXSLidOffset+4],
		0x11223344,
	)

	got := parseLegacyTextboxPLC(blob, false)
	if len(got) != 1 {
		t.Fatalf("ranges = %d, want 1", len(got))
	}
	if got[0].StartCP != 10 || got[0].EndCP != 20 || got[0].ShapeID != 0x11223344 {
		t.Fatalf("range = %+v", got[0])
	}
}

func TestLegacyFibRangeReadsFcLcbPair(t *testing.T) {
	const (
		fibBase = 12
		count   = 60
		index   = legacyFibPlcftxbxTxt
	)
	word := make([]byte, fibBase+count*8)
	off := fibBase + index*8
	binary.LittleEndian.PutUint32(word[off:off+4], 123)
	binary.LittleEndian.PutUint32(word[off+4:off+8], 456)

	fc, lcb, ok := legacyFibRange(word, fibBase, count, index)
	if !ok || fc != 123 || lcb != 456 {
		t.Fatalf("legacyFibRange = (%d,%d,%v), want (123,456,true)", fc, lcb, ok)
	}
}
