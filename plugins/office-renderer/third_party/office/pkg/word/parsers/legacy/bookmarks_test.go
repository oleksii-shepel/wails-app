package legacy

import (
	"encoding/binary"
	"testing"
)

// ---------------------------------------------------------------------------
// parseLegacyBookmarkStartPLC
// ---------------------------------------------------------------------------

// TestParseLegacyBookmarkStartPLCDecodesSortedCPs verifies one-bookmark start.
// PlcfBkf format: (n+1) 4-byte CPs + n 4-byte descriptors = 12 bytes for n=1.
func TestParseLegacyBookmarkStartPLCDecodesSortedCPs(t *testing.T) {
	// 12 bytes: CP=7, terminator=99, descriptor=0
	plc := make([]byte, 12)
	binary.LittleEndian.PutUint32(plc[0:4], 7)
	binary.LittleEndian.PutUint32(plc[4:8], 99) // terminator CP
	// plc[8:12] = descriptor (zeroed)

	cps := parseLegacyBookmarkStartPLC(plc, 1000)
	if len(cps) != 1 || cps[0] != 7 {
		t.Fatalf("parseLegacyBookmarkStartPLC = %v, want [7]", cps)
	}
}

// TestParseLegacyBookmarkStartPLCDecodesMultipleCPs verifies two bookmarks.
func TestParseLegacyBookmarkStartPLCDecodesMultipleCPs(t *testing.T) {
	// n=2: 3 CPs + 2 descriptors = 20 bytes
	plc := make([]byte, 20)
	binary.LittleEndian.PutUint32(plc[0:4], 3)
	binary.LittleEndian.PutUint32(plc[4:8], 9)
	binary.LittleEndian.PutUint32(plc[8:12], 99) // terminator
	// plc[12:20] = two descriptors (zeroed)

	cps := parseLegacyBookmarkStartPLC(plc, 1000)
	if len(cps) != 2 || cps[0] != 3 || cps[1] != 9 {
		t.Fatalf("parseLegacyBookmarkStartPLC (2 bookmarks) = %v, want [3 9]", cps)
	}
}

// TestParseLegacyBookmarkStartPLCRejectsOutOfRangeCP confirms nil when a CP
// exceeds maxCP.
func TestParseLegacyBookmarkStartPLCRejectsOutOfRangeCP(t *testing.T) {
	plc := make([]byte, 12)
	binary.LittleEndian.PutUint32(plc[0:4], 5000) // > maxCP=100
	binary.LittleEndian.PutUint32(plc[4:8], 9999)

	if cps := parseLegacyBookmarkStartPLC(plc, 100); cps != nil {
		t.Fatalf("parseLegacyBookmarkStartPLC (out-of-range) = %v, want nil", cps)
	}
}

// TestParseLegacyBookmarkStartPLCRejectsUnsortedCPs confirms nil when CPs are
// not monotonically increasing.
func TestParseLegacyBookmarkStartPLCRejectsUnsortedCPs(t *testing.T) {
	plc := make([]byte, 20)
	binary.LittleEndian.PutUint32(plc[0:4], 9)
	binary.LittleEndian.PutUint32(plc[4:8], 3) // out of order
	binary.LittleEndian.PutUint32(plc[8:12], 99)

	if cps := parseLegacyBookmarkStartPLC(plc, 1000); cps != nil {
		t.Fatalf("parseLegacyBookmarkStartPLC (unsorted) = %v, want nil", cps)
	}
}

// ---------------------------------------------------------------------------
// parseLegacyBookmarkEndPLC
// ---------------------------------------------------------------------------

// TestParseLegacyBookmarkEndPLCDecodesEndCPs verifies end positions for two
// bookmarks.  PlcfBkl holds (n+1) 4-byte CPs; the last is a terminator and is
// not returned.
func TestParseLegacyBookmarkEndPLCDecodesEndCPs(t *testing.T) {
	// n=2: 3 CPs = 12 bytes; last is terminator
	plc := make([]byte, 12)
	binary.LittleEndian.PutUint32(plc[0:4], 11)
	binary.LittleEndian.PutUint32(plc[4:8], 20)
	binary.LittleEndian.PutUint32(plc[8:12], 999) // terminator; not returned

	ends := parseLegacyBookmarkEndPLC(plc, 2, 1000)
	if len(ends) != 2 || ends[0] != 11 || ends[1] != 20 {
		t.Fatalf("parseLegacyBookmarkEndPLC = %v, want [11 20]", ends)
	}
}

// TestParseLegacyBookmarkEndPLCReturnsNilWhenNIsZero confirms nil for n=0.
func TestParseLegacyBookmarkEndPLCReturnsNilWhenNIsZero(t *testing.T) {
	plc := make([]byte, 8)
	if ends := parseLegacyBookmarkEndPLC(plc, 0, 1000); ends != nil {
		t.Fatalf("parseLegacyBookmarkEndPLC (n=0) = %v, want nil", ends)
	}
}

// TestParseLegacyBookmarkEndPLCReturnsNilWhenTooShort confirms nil when the
// slice is shorter than (n+1)*4 bytes.
func TestParseLegacyBookmarkEndPLCReturnsNilWhenTooShort(t *testing.T) {
	plc := make([]byte, 4) // only 1 CP entry but n=2 requires 3
	if ends := parseLegacyBookmarkEndPLC(plc, 2, 1000); ends != nil {
		t.Fatalf("parseLegacyBookmarkEndPLC (too short) = %v, want nil", ends)
	}
}

// ---------------------------------------------------------------------------
// parseLegacyBookmarkNameTable
// ---------------------------------------------------------------------------

// TestParseLegacyBookmarkNameTableDecodesAsciiName verifies ASCII bookmark
// names via the fExtend=0x0001 (non-Unicode) path.
func TestParseLegacyBookmarkNameTableDecodesAsciiName(t *testing.T) {
	// SttbfBkmk: fExtend=1 (not 0xFFFF), cData=1, cbExtra=0, then cch=3 + "Ref"
	data := buildBookmarkNameTable(false, []string{"Ref"})
	names := parseLegacyBookmarkNameTable(data)
	if len(names) != 1 || names[0] != "Ref" {
		t.Fatalf("parseLegacyBookmarkNameTable (ascii) = %v, want [Ref]", names)
	}
}

// TestParseLegacyBookmarkNameTableDecodesUnicodeName verifies Unicode bookmark
// names via the fExtend=0xFFFF path.
func TestParseLegacyBookmarkNameTableDecodesUnicodeName(t *testing.T) {
	data := buildBookmarkNameTable(true, []string{"Link1"})
	names := parseLegacyBookmarkNameTable(data)
	if len(names) != 1 || names[0] != "Link1" {
		t.Fatalf("parseLegacyBookmarkNameTable (unicode) = %v, want [Link1]", names)
	}
}

// TestParseLegacyBookmarkNameTableDecodesMultipleNames verifies two bookmark
// names in a single SttbfBkmk.
func TestParseLegacyBookmarkNameTableDecodesMultipleNames(t *testing.T) {
	data := buildBookmarkNameTable(false, []string{"Alpha", "Beta"})
	names := parseLegacyBookmarkNameTable(data)
	if len(names) != 2 || names[0] != "Alpha" || names[1] != "Beta" {
		t.Fatalf("parseLegacyBookmarkNameTable (2 names) = %v, want [Alpha Beta]", names)
	}
}

// TestParseLegacyBookmarkNameTableRejectsMalformedHeader confirms nil on header
// shorter than 6 bytes.
func TestParseLegacyBookmarkNameTableRejectsMalformedHeader(t *testing.T) {
	for _, bad := range [][]byte{nil, {}, make([]byte, 5)} {
		if names := parseLegacyBookmarkNameTable(bad); names != nil {
			t.Errorf("parseLegacyBookmarkNameTable (%d bytes) = %v, want nil", len(bad), names)
		}
	}
}

// ---------------------------------------------------------------------------
// loadDocBookmarks (unit-level with synthetic FIB + table stream)
// ---------------------------------------------------------------------------

// TestLoadDocBookmarksBuildsStartMap verifies that loadDocBookmarks returns a
// non-nil startMap with the bookmark name attached to the correct CP.
func TestLoadDocBookmarksBuildsStartMap(t *testing.T) {
	const (
		fibBase   = 0
		cbRgFcLcb = 22 // need index 20 (PlcfBkl)
		fibSize   = fibBase + cbRgFcLcb*8
		ccpText   = 100
	)

	// SttbfBkmk with one ASCII name "BkA" at FIB index 0.
	namePLC := buildBookmarkNameTable(false, []string{"BkA"})
	// PlcfBkf with start CP=5, at FIB index 1 (adjacent → bestScore=1).
	startPLC := make([]byte, 12)
	binary.LittleEndian.PutUint32(startPLC[0:4], 5)
	binary.LittleEndian.PutUint32(startPLC[4:8], 99)
	// descriptor at [8:12] zero
	// PlcfBkl with end CP=12 at FIB index 20.
	endPLC := make([]byte, 8)
	binary.LittleEndian.PutUint32(endPLC[0:4], 12)
	binary.LittleEndian.PutUint32(endPLC[4:8], 999) // terminator

	// Pack the table stream.
	tableStream := make([]byte, 400)
	const nameOff = 100
	const startOff = 120
	const endOff = 200
	copy(tableStream[nameOff:], namePLC)
	copy(tableStream[startOff:], startPLC)
	copy(tableStream[endOff:], endPLC)

	wordStream := make([]byte, fibSize)
	putFibFcLcb(wordStream, fibBase, 0, nameOff, len(namePLC))
	putFibFcLcb(wordStream, fibBase, 1, startOff, len(startPLC))
	putFibFcLcb(wordStream, fibBase, 20, endOff, len(endPLC))

	startMap, endMap := loadDocBookmarks(wordStream, tableStream, fibBase, cbRgFcLcb, ccpText)
	if startMap == nil {
		t.Fatal("loadDocBookmarks startMap = nil")
	}
	names, ok := startMap[5]
	if !ok || len(names) == 0 || names[0] != "BkA" {
		t.Fatalf("startMap[5] = %v %v, want [BkA]", names, ok)
	}
	if endMap == nil {
		t.Fatal("loadDocBookmarks endMap = nil")
	}
	if end, ok := endMap["BkA"]; !ok || end != 12 {
		t.Fatalf("endMap[BkA] = %d %v, want 12 true", end, ok)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// buildBookmarkNameTable constructs a minimal SttbfBkmk binary with the given
// names.  When unicode=true uses fExtend=0xFFFF (UTF-16LE entries); otherwise
// uses fExtend=0x0001 (single-byte entries).
func buildBookmarkNameTable(unicode bool, names []string) []byte {
	var out []byte
	out = append(out, 0, 0) // fExtend placeholder
	out = append(out, byte(len(names)), byte(len(names)>>8))
	out = append(out, 0, 0) // cbExtra = 0
	if unicode {
		out[0] = 0xFF
		out[1] = 0xFF
	} else {
		out[0] = 0x01
		out[1] = 0x00
	}
	for _, name := range names {
		if unicode {
			out = append(out, byte(len(name)), byte(len(name)>>8))
			for _, ch := range []byte(name) {
				out = append(out, ch, 0)
			}
		} else {
			out = append(out, byte(len(name)), byte(len(name)>>8))
			out = append(out, []byte(name)...)
		}
	}
	return out
}