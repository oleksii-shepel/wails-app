package legacy

import (
	"encoding/binary"
	"testing"
)

func makeTextboxPLC(ranges [][3]uint32) []byte {
	n := len(ranges)
	blob := make([]byte, 4*(n+1)+legacyFTXBXSSize*n)
	for i, rg := range ranges {
		binary.LittleEndian.PutUint32(blob[i*4:i*4+4], rg[0])
		if i == n-1 {
			binary.LittleEndian.PutUint32(blob[(i+1)*4:(i+1)*4+4], rg[1])
		}
		recOff := 4*(n+1) + i*legacyFTXBXSSize
		binary.LittleEndian.PutUint32(
			blob[recOff+legacyFTXBXSLidOffset:recOff+legacyFTXBXSLidOffset+4],
			rg[2],
		)
	}
	return blob
}

func makeTextboxTestPlcfSpa(anchors []legacyFloatingDrawingAnchor, endCP int) []byte {
	count := len(anchors)
	blob := make([]byte, 4*(count+1)+26*count)
	cpBytes := 4 * (count + 1)
	for i, anchor := range anchors {
		binary.LittleEndian.PutUint32(blob[i*4:i*4+4], uint32(anchor.CP))
		off := cpBytes + i*26
		binary.LittleEndian.PutUint32(blob[off:off+4], anchor.Shape)
		binary.LittleEndian.PutUint32(blob[off+4:off+8], uint32(anchor.Left))
		binary.LittleEndian.PutUint32(blob[off+8:off+12], uint32(anchor.Top))
		binary.LittleEndian.PutUint32(blob[off+12:off+16], uint32(anchor.Right))
		binary.LittleEndian.PutUint32(blob[off+16:off+20], uint32(anchor.Bottom))
		flags := uint16(anchor.BX&0x03)<<1 | uint16(anchor.BY&0x03)<<3
		binary.LittleEndian.PutUint16(blob[off+20:off+22], flags)
	}
	binary.LittleEndian.PutUint32(blob[count*4:count*4+4], uint32(endCP))
	return blob
}

func TestParseLegacyTextboxPLCReturnsOwnedRanges(t *testing.T) {
	blob := makeTextboxPLC([][3]uint32{
		{0, 5, 101},
		{5, 11, 202},
	})
	got := parseLegacyTextboxPLC(blob, false)
	if len(got) != 2 {
		t.Fatalf("ranges=%d want 2", len(got))
	}
	if got[0].StartCP != 0 || got[0].EndCP != 5 || got[0].ShapeID != 101 {
		t.Fatalf("range[0]=%+v", got[0])
	}
	if got[1].StartCP != 5 || got[1].EndCP != 11 || got[1].ShapeID != 202 {
		t.Fatalf("range[1]=%+v", got[1])
	}
}

func TestLoadLegacyTextboxOwnedRangesReadsMainTextboxPLC(t *testing.T) {
	const (
		fibBase = 16
		count   = 60
	)
	word := make([]byte, fibBase+count*8)
	table := make([]byte, 128)
	plc := makeTextboxPLC([][3]uint32{{0, 5, 321}})
	copy(table[24:], plc)

	off := fibBase + legacyFibPlcftxbxTxt*8
	binary.LittleEndian.PutUint32(word[off:off+4], 24)
	binary.LittleEndian.PutUint32(word[off+4:off+8], uint32(len(plc)))

	got := loadLegacyTextboxOwnedRanges(word, table, fibBase, count)
	if len(got) != 1 {
		t.Fatalf("ranges=%d want 1", len(got))
	}
	if got[0].StartCP != 0 || got[0].EndCP != 5 || got[0].ShapeID != 321 || got[0].Header {
		t.Fatalf("range=%+v", got[0])
	}
}

func TestLoadLegacyTextboxOwnedRangesKeepsHeaderOwnershipSeparate(t *testing.T) {
	const (
		fibBase = 16
		count   = 60
	)
	word := make([]byte, fibBase+count*8)
	table := make([]byte, 256)
	mainPLC := makeTextboxPLC([][3]uint32{{0, 4, 11}})
	hdrPLC := makeTextboxPLC([][3]uint32{{0, 6, 22}})
	copy(table[16:], mainPLC)
	copy(table[128:], hdrPLC)

	put := func(index, fc, lcb int) {
		off := fibBase + index*8
		binary.LittleEndian.PutUint32(word[off:off+4], uint32(fc))
		binary.LittleEndian.PutUint32(word[off+4:off+8], uint32(lcb))
	}
	put(legacyFibPlcftxbxTxt, 16, len(mainPLC))
	put(legacyFibPlcfHdrtxbxTxt, 128, len(hdrPLC))

	got := loadLegacyTextboxOwnedRanges(word, table, fibBase, count)
	if len(got) != 2 {
		t.Fatalf("ranges=%d want 2", len(got))
	}
	if got[0].Header || !got[1].Header {
		t.Fatalf("header flags=%v/%v", got[0].Header, got[1].Header)
	}
	if got[0].ShapeID != 11 || got[1].ShapeID != 22 {
		t.Fatalf("shape ids=%d/%d", got[0].ShapeID, got[1].ShapeID)
	}
}

func TestLegacyTextboxStoryBaseSeparatesMainAndHeaderTextboxStories(t *testing.T) {
	counts := legacyStoryCounts{
		Text: 100,
		Ftn:  10,
		Hdr:  20,
		Mcr:  3,
		Atn:  4,
		Edn:  5,
		Txbx: 30,
	}
	main := legacyTextboxStoryBase(counts, false)
	header := legacyTextboxStoryBase(counts, true)
	if main != 142 {
		t.Fatalf("main base=%d want 142", main)
	}
	if header != 172 {
		t.Fatalf("header base=%d want 172", header)
	}
}

func TestLegacyTextboxAnchorByShapeIDIncludesMainAndHeaderStories(t *testing.T) {
	const (
		fibBase  = 32
		fibCount = 42
	)
	word := make([]byte, fibBase+fibCount*8)
	table := make([]byte, 512)

	mainPLC := makeTextboxTestPlcfSpa([]legacyFloatingDrawingAnchor{
		{CP: 4, Shape: 0x101, Left: 10, Top: 20, Right: 110, Bottom: 120, BX: 1, BY: 1},
	}, 20)
	hdrPLC := makeTextboxTestPlcfSpa([]legacyFloatingDrawingAnchor{
		{CP: 3, Shape: 0x202, Left: 30, Top: 40, Right: 130, Bottom: 140, BX: 2, BY: 2},
	}, 10)

	putPair := func(index, fc, lcb int) {
		off := fibBase + index*8
		binary.LittleEndian.PutUint32(word[off:off+4], uint32(fc))
		binary.LittleEndian.PutUint32(word[off+4:off+8], uint32(lcb))
	}
	copy(table[16:], mainPLC)
	copy(table[160:], hdrPLC)
	putPair(legacyFibPlcSpaMom, 16, len(mainPLC))
	putPair(legacyFibPlcSpaHdr, 160, len(hdrPLC))

	counts := legacyStoryCounts{Text: 100, Ftn: 10, Hdr: 10}
	main := loadLegacyFloatingDrawingAnchorsAtIndex(
		word, table, fibBase, fibCount,
		legacyFibPlcSpaMom, 0, counts.Text, nil, false,
	)

	got := legacyTextboxAnchorByShapeID(word, table, fibBase, fibCount, counts, main)
	if len(got) != 2 {
		t.Fatalf("shape anchor count = %d, want 2: %#v", len(got), got)
	}
	if got[0x101].CP != 4 {
		t.Fatalf("main anchor = %+v", got[0x101])
	}
	if got[0x202].CP != 113 {
		t.Fatalf("header anchor CP = %d, want global CP 113", got[0x202].CP)
	}
}

func TestLegacyTextboxAnchorByShapeIDHeaderShapeOverridesImpossibleMainCollision(t *testing.T) {
	const (
		fibBase  = 16
		fibCount = 42
	)
	word := make([]byte, fibBase+fibCount*8)
	table := make([]byte, 256)

	hdrPLC := makeTextboxTestPlcfSpa([]legacyFloatingDrawingAnchor{
		{CP: 1, Shape: 0x777, Left: 70, Top: 80, Right: 170, Bottom: 180, BX: 2, BY: 2},
	}, 5)
	copy(table[80:], hdrPLC)
	off := fibBase + legacyFibPlcSpaHdr*8
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
