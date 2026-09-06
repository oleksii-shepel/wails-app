package word97

import (
	"bytes"
	"encoding/binary"
	"testing"

	docbinary "office/pkg/word/parsers/doc/binary"
)

func TestLoadRevisionAuthorsReaderAt(t *testing.T) {
	blob := make([]byte, 6)
	binary.LittleEndian.PutUint16(blob[0:2], 0xFFFF)
	binary.LittleEndian.PutUint16(blob[2:4], 2)
	binary.LittleEndian.PutUint16(blob[4:6], 0)
	blob = appendUnicodeSTTBString(blob, "Alice")
	blob = appendUnicodeSTTBString(blob, "Богдан")

	fib := make([]byte, 52*8)
	putFcLcb(fib, fibRevisionAuthors, 0, len(blob))
	layout := docbinary.Word97Layout{FibBase: 0, CbRgFcLcb: 52}

	got := LoadRevisionAuthorsReaderAt(bytes.NewReader(blob), int64(len(blob)), fib, layout)
	if len(got) != 2 || got[0] != "Alice" || got[1] != "Богдан" {
		t.Fatalf("authors=%q, want [Alice Богдан]", got)
	}
}

func TestLoadDOPReaderAt(t *testing.T) {
	dop := []byte{1, 2, 3, 4, 5}
	fib := make([]byte, (fibDOP+1)*8)
	putFcLcb(fib, fibDOP, 0, len(dop))
	layout := docbinary.Word97Layout{FibBase: 0, CbRgFcLcb: fibDOP + 1}

	got := LoadDOPReaderAt(bytes.NewReader(dop), int64(len(dop)), fib, layout)
	if !bytes.Equal(got, dop) {
		t.Fatalf("dop=%v, want %v", got, dop)
	}
}

func appendUnicodeSTTBString(dst []byte, value string) []byte {
	runes := []rune(value)
	binaryLen := make([]byte, 2)
	binary.LittleEndian.PutUint16(binaryLen, uint16(len(runes)))
	dst = append(dst, binaryLen...)
	for _, r := range runes {
		if r <= 0xFFFF {
			buf := []byte{byte(r), byte(r >> 8)}
			dst = append(dst, buf...)
			continue
		}
		for _, u := range []uint16{uint16(0xD800 + ((r-0x10000)>>10)), uint16(0xDC00 + ((r-0x10000)&0x3FF))} {
			dst = append(dst, byte(u), byte(u>>8))
		}
	}
	return dst
}
