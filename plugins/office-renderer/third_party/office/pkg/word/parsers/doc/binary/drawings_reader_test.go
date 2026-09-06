package docbinary

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func makeReaderTestPlcfSpa(cp int, shape uint32, left, top, right, bottom int32) []byte {
	out := make([]byte, 34)
	binary.LittleEndian.PutUint32(out[0:4], uint32(cp))
	binary.LittleEndian.PutUint32(out[4:8], uint32(cp+1))
	off := 8
	binary.LittleEndian.PutUint32(out[off:off+4], shape)
	binary.LittleEndian.PutUint32(out[off+4:off+8], uint32(left))
	binary.LittleEndian.PutUint32(out[off+8:off+12], uint32(top))
	binary.LittleEndian.PutUint32(out[off+12:off+16], uint32(right))
	binary.LittleEndian.PutUint32(out[off+16:off+20], uint32(bottom))
	return out
}

func TestFloatingAnchorsReaderAtPrefersCanonicalSlot(t *testing.T) {
	const (
		fibBase  = 16
		fibCount = 60
	)
	fib := make([]byte, fibBase+fibCount*8+16)
	table := make([]byte, 128)
	authoritative := makeReaderTestPlcfSpa(10, 1001, 100, 200, 500, 600)
	decoy := makeReaderTestPlcfSpa(10, 9999, 9000, 9000, 9900, 9900)
	copy(table[8:], authoritative)
	copy(table[64:], decoy)
	putPair := func(index, fc, lcb int) {
		off := fibBase + index*8
		binary.LittleEndian.PutUint32(fib[off:off+4], uint32(fc))
		binary.LittleEndian.PutUint32(fib[off+4:off+8], uint32(lcb))
	}
	putPair(FibPlcSpaMom, 8, len(authoritative))
	putPair(12, 64, len(decoy))

	anchors := FloatingAnchorsReaderAt(bytes.NewReader(table), int64(len(table)), fib, fibBase, fibCount, 100)
	if len(anchors) != 1 || anchors[0].Shape != 1001 {
		t.Fatalf("anchors=%+v, want canonical shape 1001", anchors)
	}
}

func TestFloatingAnchorsReaderAtRecoversNonCanonicalSlot(t *testing.T) {
	const (
		fibBase  = 16
		fibCount = 60
	)
	fib := make([]byte, fibBase+fibCount*8+16)
	table := make([]byte, 128)
	fallback := makeReaderTestPlcfSpa(10, 1234, 10, 20, 110, 120)
	copy(table[64:], fallback)
	putPair := func(index, fc, lcb int) {
		off := fibBase + index*8
		binary.LittleEndian.PutUint32(fib[off:off+4], uint32(fc))
		binary.LittleEndian.PutUint32(fib[off+4:off+8], uint32(lcb))
	}
	putPair(FibPlcSpaMom, 1000, 34)
	putPair(12, 64, len(fallback))

	anchors := FloatingAnchorsReaderAt(bytes.NewReader(table), int64(len(table)), fib, fibBase, fibCount, 100)
	if len(anchors) != 1 || anchors[0].CP != 10 || anchors[0].Shape != 1234 {
		t.Fatalf("anchors=%+v, want recovered CP=10 shape=1234", anchors)
	}
}
