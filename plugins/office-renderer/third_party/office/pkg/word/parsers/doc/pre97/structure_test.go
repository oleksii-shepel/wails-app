package pre97

import (
	"encoding/binary"
	"testing"
)

func TestInspectBodySpanOwnsWord95Structure(t *testing.T) {
	word := make([]byte, 0x300)
	binary.LittleEndian.PutUint16(word[0:2], 0xA5DC)
	binary.LittleEndian.PutUint16(word[2:4], 0x0068)
	word[0x0b] = 0x04
	binary.LittleEndian.PutUint32(word[24:28], 0x180)
	binary.LittleEndian.PutUint32(word[28:32], 0x1f0)
	binary.LittleEndian.PutUint32(word[52:56], 0x40)
	binary.LittleEndian.PutUint32(word[0x38:0x3c], 7)
	binary.LittleEndian.PutUint32(word[0xb8:0xbc], 0x100)
	binary.LittleEndian.PutUint32(word[0xbc:0xc0], 10)
	binary.LittleEndian.PutUint32(word[0xc0:0xc4], 0x120)
	binary.LittleEndian.PutUint32(word[0xc4:0xc8], 10)
	binary.LittleEndian.PutUint32(word[0x160:0x164], 0x220)

	span, ok := InspectBodySpan(word)
	if !ok {
		t.Fatal("InspectBodySpan = !ok")
	}
	if span.Start != 0x180 || span.BodyEnd != 0x1c0 {
		t.Fatalf("body = %#x..%#x, want %#x..%#x", span.Start, span.BodyEnd, 0x180, 0x1c0)
	}
	if !span.FComplex || span.CcpFtn != 7 {
		t.Fatalf("structural flags/counts = complex:%v ccpFtn:%d", span.FComplex, span.CcpFtn)
	}
	if span.ChpTableOffset != 0x100 || span.PapTableOffset != 0x120 || span.ComplexTableOffset != 0x220 {
		t.Fatalf("table offsets = chp:%#x pap:%#x clx:%#x", span.ChpTableOffset, span.PapTableOffset, span.ComplexTableOffset)
	}
}

func TestBuildSinglePiecePLC(t *testing.T) {
	plc := BuildSinglePiecePLC(Span{Start: 0x100, BodyEnd: 0x130})
	if len(plc) != 16 {
		t.Fatalf("len = %d, want 16", len(plc))
	}
	if got := binary.LittleEndian.Uint32(plc[4:8]); got != 0x30 {
		t.Fatalf("cpMac = %#x, want %#x", got, 0x30)
	}
	if got := binary.LittleEndian.Uint32(plc[10:14]); got != 0x40000200 {
		t.Fatalf("fcCompressed = %#x, want %#x", got, uint32(0x40000200))
	}
}

func TestFKPPageNumbers(t *testing.T) {
	word := make([]byte, 2048)
	off := 0x100
	plc := word[off : off+10]
	binary.LittleEndian.PutUint32(plc[0:4], 0)
	binary.LittleEndian.PutUint32(plc[4:8], 10)
	binary.LittleEndian.PutUint16(plc[8:10], 2)

	pages := FKPPageNumbers(word, off, len(plc))
	if len(pages) != 1 || pages[0] != 2 {
		t.Fatalf("pages = %v, want [2]", pages)
	}
}
