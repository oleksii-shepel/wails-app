package pre97

import (
	"encoding/binary"
	"testing"
)

func TestLoadStoryRangesUsesExplicitOldPLCs(t *testing.T) {
	word := make([]byte, 256)
	span := Span{CcpText: 20, CcpFtn: 10, FtnRefTableOffset: 40, FtnRefTableSize: 8, FtnTxtTableOffset: 60, FtnTxtTableSize: 8}
	// one reference at body CP 7 + terminator
	binary.LittleEndian.PutUint32(word[40:44], 7)
	binary.LittleEndian.PutUint32(word[44:48], 20)
	// one note range [0,10]
	binary.LittleEndian.PutUint32(word[60:64], 0)
	binary.LittleEndian.PutUint32(word[64:68], 10)
	got := LoadStoryRanges(word, span).Footnotes
	if len(got) != 1 {
		t.Fatalf("footnote ranges = %d, want 1", len(got))
	}
	if got[0].ReferenceCP != 7 || got[0].Range.StartCP != 20 || got[0].Range.EndCP != 30 {
		t.Fatalf("footnote range = %+v", got[0])
	}
}

func TestLoadHeaderFooterRangesProjectsSixSlots(t *testing.T) {
	word := make([]byte, 256)
	span := Span{CcpText: 30, CcpFtn: 5, CcpHdd: 12, HddTableOffset: 80, HddTableSize: 32}
	// 8 CPs => 6 logical slots.
	for i, cp := range []uint32{0, 2, 4, 6, 8, 10, 12, 12} {
		binary.LittleEndian.PutUint32(word[80+i*4:84+i*4], cp)
	}
	got := LoadHeaderFooterRanges(word, span)
	if got.Slots[0].StartCP != 35 || got.Slots[0].EndCP != 37 {
		t.Fatalf("slot0 = %+v", got.Slots[0])
	}
	if got.Slots[5].StartCP != 45 || got.Slots[5].EndCP != 47 {
		t.Fatalf("slot5 = %+v", got.Slots[5])
	}
}

func TestLoadSectionRecordsPreservesBoundsAndSepx(t *testing.T) {
	word := make([]byte, 256)
	span := Span{SedTableOffset: 100, SedTableSize: 20}
	plc := word[100:120]
	binary.LittleEndian.PutUint32(plc[0:4], 0)
	binary.LittleEndian.PutUint32(plc[4:8], 25)
	// one 12-byte SED starts at offset 8; fcSepx is at +2.
	binary.LittleEndian.PutUint32(plc[10:14], 160)
	binary.LittleEndian.PutUint16(word[160:162], 3)
	copy(word[162:165], []byte{1, 2, 3})
	got := LoadSectionRecords(word, span)
	if len(got) != 1 {
		t.Fatalf("sections = %d, want 1", len(got))
	}
	if got[0].StartCP != 0 || got[0].EndCP != 25 {
		t.Fatalf("section bounds = %d..%d", got[0].StartCP, got[0].EndCP)
	}
	if len(got[0].Sepx) != 3 || got[0].Sepx[2] != 3 {
		t.Fatalf("sepx = %v", got[0].Sepx)
	}
}
