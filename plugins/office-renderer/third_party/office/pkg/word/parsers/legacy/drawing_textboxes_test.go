package legacy

import (
	"encoding/binary"
	"testing"

	docbinary "office/pkg/word/parsers/doc/binary"
)

func makeLegacyTextboxTestPlcfSpa(anchor legacyFloatingDrawingAnchor, endCP int) []byte {
	blob := make([]byte, 34)
	binary.LittleEndian.PutUint32(blob[0:4], uint32(anchor.CP))
	binary.LittleEndian.PutUint32(blob[4:8], uint32(endCP))
	off := 8
	binary.LittleEndian.PutUint32(blob[off:off+4], anchor.Shape)
	binary.LittleEndian.PutUint32(blob[off+4:off+8], uint32(anchor.Left))
	binary.LittleEndian.PutUint32(blob[off+8:off+12], uint32(anchor.Top))
	binary.LittleEndian.PutUint32(blob[off+12:off+16], uint32(anchor.Right))
	binary.LittleEndian.PutUint32(blob[off+16:off+20], uint32(anchor.Bottom))
	flags := uint16(anchor.BX&0x03)<<1 | uint16(anchor.BY&0x03)<<3
	binary.LittleEndian.PutUint16(blob[off+20:off+22], flags)
	return blob
}

func TestCanonicalTextboxStoryBaseSeparatesMainAndHeaderTextboxStories(t *testing.T) {
	counts := docbinary.StoryCounts{
		Text: 100,
		Ftn:  10,
		Hdr:  20,
		Mcr:  3,
		Atn:  4,
		Edn:  5,
		Txbx: 30,
	}
	if main := docbinary.TextboxStoryBase(counts, false); main != 142 {
		t.Fatalf("main base=%d want 142", main)
	}
	if header := docbinary.TextboxStoryBase(counts, true); header != 172 {
		t.Fatalf("header base=%d want 172", header)
	}
}

func TestLegacyTextboxAnchorByShapeIDHeaderOwnerWinsCollision(t *testing.T) {
	const (
		fibBase  = 16
		fibCount = 60
	)
	word := make([]byte, fibBase+fibCount*8)
	table := make([]byte, 256)
	hdrPLC := makeLegacyTextboxTestPlcfSpa(legacyFloatingDrawingAnchor{
		CP: 1, Shape: 0x777, Left: 70, Top: 80, Right: 170, Bottom: 180, BX: 2, BY: 2,
	}, 5)
	copy(table[80:], hdrPLC)
	off := fibBase + docbinary.FibPlcSpaHdr*8
	binary.LittleEndian.PutUint32(word[off:off+4], 80)
	binary.LittleEndian.PutUint32(word[off+4:off+8], uint32(len(hdrPLC)))

	main := map[int]legacyFloatingDrawingAnchor{
		2: {CP: 2, Shape: 0x777, Left: 1, Top: 2, Right: 3, Bottom: 4},
	}
	counts := legacyStoryCounts{Text: 50, Ftn: 5, Hdr: 5}
	got := legacyTextboxAnchorByShapeID(word, table, fibBase, fibCount, counts, main)
	if got[0x777].Left != 70 || got[0x777].CP != 56 {
		t.Fatalf("header ownership did not win: %+v", got[0x777])
	}
}
