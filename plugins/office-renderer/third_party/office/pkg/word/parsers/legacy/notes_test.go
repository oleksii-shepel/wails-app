package legacy

import (
	"encoding/binary"
	"testing"
)

// ---------------------------------------------------------------------------
// parseLegacyNoteRefCPs
// ---------------------------------------------------------------------------

// TestParseLegacyNoteRefCPsDecodesStride4Format verifies the second PLC branch:
// len%4==0, count = len/4-1.  One footnote reference at CP 5.
func TestParseLegacyNoteRefCPsDecodesStride4Format(t *testing.T) {
	// 8 bytes: [5 0 0 0] + terminator [99 0 0 0]
	plc := make([]byte, 8)
	binary.LittleEndian.PutUint32(plc[0:4], 5)
	binary.LittleEndian.PutUint32(plc[4:8], 99)

	cps := parseLegacyNoteRefCPs(plc)
	if len(cps) != 1 || cps[0] != 5 {
		t.Fatalf("parseLegacyNoteRefCPs (stride-4) = %v, want [5]", cps)
	}
}

// TestParseLegacyNoteRefCPsDecodesStride6Format verifies the first PLC branch:
// (len-4)%6==0, count = (len-4)/6.  One ref at CP 12 with a 2-byte descriptor.
func TestParseLegacyNoteRefCPsDecodesStride6Format(t *testing.T) {
	// 10 bytes: CP(4)=12, descriptor(2)=0, terminator(4)=99
	plc := make([]byte, 10)
	binary.LittleEndian.PutUint32(plc[0:4], 12)
	// plc[4:6] = descriptor (zeroed)
	binary.LittleEndian.PutUint32(plc[6:10], 99) // terminator CP

	cps := parseLegacyNoteRefCPs(plc)
	if len(cps) != 1 || cps[0] != 12 {
		t.Fatalf("parseLegacyNoteRefCPs (stride-6) = %v, want [12]", cps)
	}
}

// TestParseLegacyNoteRefCPsDecodesMultipleRefsStride4 tests two ref CPs.
func TestParseLegacyNoteRefCPsDecodesMultipleRefsStride4(t *testing.T) {
	// 12 bytes: CP=3, CP=7, terminator=20
	plc := make([]byte, 12)
	binary.LittleEndian.PutUint32(plc[0:4], 3)
	binary.LittleEndian.PutUint32(plc[4:8], 7)
	binary.LittleEndian.PutUint32(plc[8:12], 20)

	cps := parseLegacyNoteRefCPs(plc)
	if len(cps) != 2 || cps[0] != 3 || cps[1] != 7 {
		t.Fatalf("parseLegacyNoteRefCPs (stride-4, 2 refs) = %v, want [3 7]", cps)
	}
}

// TestParseLegacyNoteRefCPsRejectsTooShortInput verifies nil on short data.
func TestParseLegacyNoteRefCPsRejectsTooShortInput(t *testing.T) {
	for _, bad := range [][]byte{nil, {}, make([]byte, 5), make([]byte, 7)} {
		if cps := parseLegacyNoteRefCPs(bad); cps != nil {
			t.Errorf("parseLegacyNoteRefCPs(%d bytes) = %v, want nil", len(bad), cps)
		}
	}
}

// ---------------------------------------------------------------------------
// parseLegacyNoteTextCPs
// ---------------------------------------------------------------------------

// TestParseLegacyNoteTextCPsDecodesStartAndEnd verifies one-note text range.
func TestParseLegacyNoteTextCPsDecodesStartAndEnd(t *testing.T) {
	// 8 bytes → count=1, returns 2 CPs: [0, 10]
	plc := make([]byte, 8)
	binary.LittleEndian.PutUint32(plc[0:4], 0)
	binary.LittleEndian.PutUint32(plc[4:8], 10)

	cps := parseLegacyNoteTextCPs(plc)
	if len(cps) != 2 || cps[0] != 0 || cps[1] != 10 {
		t.Fatalf("parseLegacyNoteTextCPs = %v, want [0 10]", cps)
	}
}

// TestParseLegacyNoteTextCPsDecodesMultipleRanges tests two notes.
func TestParseLegacyNoteTextCPsDecodesMultipleRanges(t *testing.T) {
	// 12 bytes → count=2, returns 3 CPs: [0, 5, 12]
	plc := make([]byte, 12)
	binary.LittleEndian.PutUint32(plc[0:4], 0)
	binary.LittleEndian.PutUint32(plc[4:8], 5)
	binary.LittleEndian.PutUint32(plc[8:12], 12)

	cps := parseLegacyNoteTextCPs(plc)
	if len(cps) != 3 || cps[0] != 0 || cps[1] != 5 || cps[2] != 12 {
		t.Fatalf("parseLegacyNoteTextCPs = %v, want [0 5 12]", cps)
	}
}

// TestParseLegacyNoteTextCPsRejectsBadInput verifies nil on malformed data.
func TestParseLegacyNoteTextCPsRejectsBadInput(t *testing.T) {
	for _, bad := range [][]byte{nil, {}, make([]byte, 5)} {
		if cps := parseLegacyNoteTextCPs(bad); cps != nil {
			t.Errorf("parseLegacyNoteTextCPs(%d bytes) = %v, want nil", len(bad), cps)
		}
	}
}

// ---------------------------------------------------------------------------
// loadDocFootnotes (unit-level with synthetic FIB + table stream)
// ---------------------------------------------------------------------------

// TestLoadDocFootnotesBuildsRefByCP constructs a minimal FIB and tableStream
// with one footnote reference at CP 5 and verifies that loadDocFootnotes returns
// a non-nil *legacyNoteData with the correct RefByCP mapping.
func TestLoadDocFootnotesBuildsRefByCP(t *testing.T) {
	const (
		fibBase   = 0
		cbRgFcLcb = 4 // need indices 0-3
		fibSize   = fibBase + cbRgFcLcb*8
		ccpText   = 100
	)

	// Ref PLC at offset 200: [refCP=5, terminator=99] — stride-4, 1 ref.
	refPLC := make([]byte, 8)
	binary.LittleEndian.PutUint32(refPLC[0:4], 5)
	binary.LittleEndian.PutUint32(refPLC[4:8], 99)

	// Text PLC at offset 208: [start=0, end=10] — 1 range.
	textPLC := make([]byte, 8)
	binary.LittleEndian.PutUint32(textPLC[0:4], 0)
	binary.LittleEndian.PutUint32(textPLC[4:8], 10)

	tableStream := make([]byte, 216)
	copy(tableStream[200:], refPLC)
	copy(tableStream[208:], textPLC)

	wordStream := make([]byte, fibSize)
	// FIB index 2 = ref; FIB index 3 = text
	putFibFcLcb(wordStream, fibBase, 2, 200, len(refPLC))
	putFibFcLcb(wordStream, fibBase, 3, 208, len(textPLC))

	notes := loadDocFootnotes(wordStream, tableStream, nil, fibBase, cbRgFcLcb, ccpText, nil, nil)
	if notes == nil {
		t.Fatal("loadDocFootnotes = nil, want legacyNoteData with one ref")
	}
	if id, ok := notes.RefByCP[5]; !ok || id != 1 {
		t.Fatalf("RefByCP[5] = %d %v, want 1 true", id, ok)
	}
}

// TestLoadDocFootnotesReturnsNilWhenNoRefTable confirms nil when the ref table
// LCB is zero (no footnotes present).
func TestLoadDocFootnotesReturnsNilWhenNoRefTable(t *testing.T) {
	const (
		fibBase   = 0
		cbRgFcLcb = 4
		fibSize   = cbRgFcLcb * 8
	)
	wordStream := make([]byte, fibSize)
	// All fc/lcb entries remain zero → refLCB=0, early return.
	notes := loadDocFootnotes(wordStream, nil, nil, fibBase, cbRgFcLcb, 100, nil, nil)
	if notes != nil {
		t.Fatalf("loadDocFootnotes (no ref table) = %v, want nil", notes)
	}
}