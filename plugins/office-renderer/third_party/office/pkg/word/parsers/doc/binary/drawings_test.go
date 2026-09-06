package docbinary

import (
	"encoding/binary"
	"testing"
)

func makeTextboxPLC(ranges [][3]uint32) []byte {
	n := len(ranges)
	blob := make([]byte, 4*(n+1)+22*n)
	for i, rg := range ranges {
		binary.LittleEndian.PutUint32(blob[i*4:i*4+4], rg[0])
		if i == n-1 {
			binary.LittleEndian.PutUint32(blob[(i+1)*4:(i+1)*4+4], rg[1])
		}
		recOff := 4*(n+1) + i*22
		binary.LittleEndian.PutUint32(blob[recOff+14:recOff+18], rg[2])
	}
	return blob
}

func TestParsePlcfSpaDecodesAnchorFlags(t *testing.T) {
	blob := make([]byte, 34)
	binary.LittleEndian.PutUint32(blob[0:], 5)
	binary.LittleEndian.PutUint32(blob[4:], 6)
	off := 8
	binary.LittleEndian.PutUint32(blob[off:], 1024)
	binary.LittleEndian.PutUint32(blob[off+4:], 100)
	binary.LittleEndian.PutUint32(blob[off+8:], 200)
	binary.LittleEndian.PutUint32(blob[off+12:], 500)
	binary.LittleEndian.PutUint32(blob[off+16:], 700)
	flags := uint16(0x0001 | (2 << 1) | (1 << 3) | (3 << 5) | (2 << 9) | 0x2000 | 0x4000 | 0x8000)
	binary.LittleEndian.PutUint16(blob[off+20:], flags)
	got, ok := ParsePlcfSpa(blob, 20)
	if !ok || len(got) != 1 {
		t.Fatalf("anchors=%+v ok=%v", got, ok)
	}
	a := got[0]
	if a.CP != 5 || a.Shape != 1024 || a.Left != 100 || a.Top != 200 || a.Right != 500 || a.Bottom != 700 {
		t.Fatalf("anchor=%+v", a)
	}
	if !a.Header || a.BX != 2 || a.BY != 1 || a.Wrap != 3 || a.WrapSide != 2 || !a.RcaSimple || !a.BelowText || !a.AnchorLock {
		t.Fatalf("flags=%+v", a)
	}
}

func TestParseTextboxPLCReadsFTXBXSLid(t *testing.T) {
	blob := make([]byte, 30)
	binary.LittleEndian.PutUint32(blob[0:], 2)
	binary.LittleEndian.PutUint32(blob[4:], 9)
	binary.LittleEndian.PutUint32(blob[8+14:], 0x10002)
	got := ParseTextboxPLC(blob, false)
	if len(got) != 1 || got[0].StartCP != 2 || got[0].EndCP != 9 || got[0].ShapeID != 0x10002 || got[0].Header {
		t.Fatalf("ranges=%+v", got)
	}
}

func TestFibRangeReadsFcLcbPair(t *testing.T) {
	const (
		fibBase = 12
		count   = 60
		index   = FibPlcftxbxTxt
	)
	word := make([]byte, fibBase+count*8)
	off := fibBase + index*8
	binary.LittleEndian.PutUint32(word[off:off+4], 123)
	binary.LittleEndian.PutUint32(word[off+4:off+8], 456)
	fc, lcb, ok := FibRange(word, fibBase, count, index)
	if !ok || fc != 123 || lcb != 456 {
		t.Fatalf("FibRange=(%d,%d,%v)", fc, lcb, ok)
	}
}

func TestTextboxRangesKeepsMainAndHeaderOwnershipSeparate(t *testing.T) {
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
	put(FibPlcftxbxTxt, 16, len(mainPLC))
	put(FibPlcfHdrtxbxTxt, 128, len(hdrPLC))
	got := TextboxRanges(word, table, fibBase, count)
	if len(got) != 2 || got[0].Header || !got[1].Header || got[0].ShapeID != 11 || got[1].ShapeID != 22 {
		t.Fatalf("ranges=%+v", got)
	}
}

func TestFloatingAnchorsAtAppliesStoryBaseAndHeader(t *testing.T) {
	const (
		fibBase = 16
		count   = 60
	)
	word := make([]byte, fibBase+count*8)
	table := make([]byte, 128)
	blob := make([]byte, 34)
	binary.LittleEndian.PutUint32(blob[0:4], 3)
	binary.LittleEndian.PutUint32(blob[4:8], 5)
	off := 8
	binary.LittleEndian.PutUint32(blob[off:off+4], 0x202)
	binary.LittleEndian.PutUint32(blob[off+4:off+8], 30)
	binary.LittleEndian.PutUint32(blob[off+8:off+12], 40)
	binary.LittleEndian.PutUint32(blob[off+12:off+16], 130)
	binary.LittleEndian.PutUint32(blob[off+16:off+20], 140)
	copy(table[32:], blob)
	pair := fibBase + FibPlcSpaHdr*8
	binary.LittleEndian.PutUint32(word[pair:pair+4], 32)
	binary.LittleEndian.PutUint32(word[pair+4:pair+8], uint32(len(blob)))
	got := FloatingAnchorsAt(word, table, fibBase, count, FibPlcSpaHdr, 110, 10, true)
	if len(got) != 1 || got[0].CP != 113 || !got[0].Header || got[0].Shape != 0x202 {
		t.Fatalf("anchors=%+v", got)
	}
}

func TestAnchorMapByShapeIDLastStoryWins(t *testing.T) {
	got := AnchorMapByShapeID([]FloatingAnchor{
		{CP: 2, Shape: 0x777, Left: 1},
		{CP: 56, Shape: 0x777, Left: 70, Header: true},
	})
	if got[0x777].Left != 70 || got[0x777].CP != 56 || !got[0x777].Header {
		t.Fatalf("owner=%+v", got[0x777])
	}
}
