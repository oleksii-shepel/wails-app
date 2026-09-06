package docbinary

import (
	"encoding/binary"
	"testing"
)

func TestParseWord97LayoutLocatesTableAndFcLcbRegion(t *testing.T) {
	fib := make([]byte, 32+2+2*2+2+11*4+2+40*8)
	binary.LittleEndian.PutUint16(fib[0:2], fibSignature)
	binary.LittleEndian.PutUint16(fib[2:4], fibWord97)
	binary.LittleEndian.PutUint16(fib[10:12], 1<<9)

	pos := 32
	binary.LittleEndian.PutUint16(fib[pos:pos+2], 2)
	pos += 2 + 4
	binary.LittleEndian.PutUint16(fib[pos:pos+2], 11)
	storyBase := pos + 2
	pos = storyBase + 44
	binary.LittleEndian.PutUint16(fib[pos:pos+2], 40)
	fibBase := pos + 2

	const clxIndex = 33
	binary.LittleEndian.PutUint32(fib[fibBase+clxIndex*8:], 1234)
	binary.LittleEndian.PutUint32(fib[fibBase+clxIndex*8+4:], 567)

	layout, ok := ParseWord97Layout(fib)
	if !ok {
		t.Fatal("ParseWord97Layout returned !ok")
	}
	if layout.TableStreamName != "1Table" || layout.StoryCountsBase != storyBase || layout.FibBase != fibBase || layout.CbRgFcLcb != 40 {
		t.Fatalf("layout = %+v", layout)
	}
	fc, lcb, ok := layout.FcLcb(fib, clxIndex)
	if !ok || fc != 1234 || lcb != 567 {
		t.Fatalf("FcLcb = %d,%d,%v", fc, lcb, ok)
	}
}

func TestParseWord97LayoutRejectsTruncatedFcLcbRegion(t *testing.T) {
	fib := make([]byte, 64)
	binary.LittleEndian.PutUint16(fib[0:2], fibSignature)
	binary.LittleEndian.PutUint16(fib[2:4], fibWord97)
	binary.LittleEndian.PutUint16(fib[32:34], 0)
	binary.LittleEndian.PutUint16(fib[34:36], 0)
	binary.LittleEndian.PutUint16(fib[36:38], 10)
	if _, ok := ParseWord97Layout(fib); ok {
		t.Fatal("ParseWord97Layout accepted truncated FibRgFcLcb97")
	}
}
