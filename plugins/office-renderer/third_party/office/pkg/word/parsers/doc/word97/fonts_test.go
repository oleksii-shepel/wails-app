package word97

import (
	"bytes"
	"encoding/binary"
	"testing"

	docbinary "office/pkg/word/parsers/doc/binary"
)

func TestParseCanonicalFFN(t *testing.T) {
	rec := make([]byte, 52)
	rec[0] = 51
	rec[1] = 0x22
	rec[4] = 0xEE
	rec[6] = 2
	copy(rec[40:], []byte{'A', 0, 'r', 0, 'i', 0, 'a', 0, 'l', 0, 0, 0})
	entries := parseFontTable(rec)
	if len(entries) != 1 {
		t.Fatalf("entries=%#v", entries)
	}
	if entries[0].Name != "Arial" {
		t.Fatalf("name=%q", entries[0].Name)
	}
	if entries[0].Info.Family != "swiss" || entries[0].Info.Pitch != "variable" || entries[0].Info.Charset != "EE" {
		t.Fatalf("info=%+v", entries[0].Info)
	}
}

func TestLoadFontNamesReaderAtPreservesFFNOrder(t *testing.T) {
	fontTable := append(testFontRecord("Aptos"), testFontRecord("MS Mincho")...)
	fib := make([]byte, 16*8)
	putFcLcb(fib, fibFontTable, 0, len(fontTable))
	layout := docbinary.Word97Layout{FibBase: 0, CbRgFcLcb: 16}

	names := LoadFontNamesReaderAt(bytes.NewReader(fontTable), int64(len(fontTable)), fib, layout)
	if len(names) != 2 || names[0] != "Aptos" || names[1] != "MS Mincho" {
		t.Fatalf("names=%v, want [Aptos MS Mincho]", names)
	}
}

func TestLoadEmbeddedFontsReaderAt(t *testing.T) {
	const (
		font1Offset = 1024
		font2Offset = 1088
	)
	fontTable := testFontRecord("Oswald")
	descriptors := make([]byte, 10+2*12)
	binary.LittleEndian.PutUint16(descriptors[2:4], 2)
	binary.LittleEndian.PutUint16(descriptors[8:10], 10)
	putEmbeddedDescriptor(descriptors[10:22], font1Offset, 0, 0, 0xFFFFFFFF)
	putEmbeddedDescriptor(descriptors[22:34], font2Offset, 0, 0x0003, 1)

	table := append(append([]byte(nil), fontTable...), descriptors...)
	fib := make([]byte, 62*8)
	putFcLcb(fib, fibFontTable, 0, len(fontTable))
	putFcLcb(fib, fibEmbeddedFonts, len(fontTable), len(descriptors))
	layout := docbinary.Word97Layout{FibBase: 0, CbRgFcLcb: 62}

	word := make([]byte, 2048)
	copy(word[font1Offset:], testEOTPayload(40, 0x11))
	copy(word[font2Offset:], testEOTPayload(44, 0x22))

	fonts := LoadEmbeddedFontsReaderAt(
		bytes.NewReader(word), int64(len(word)),
		bytes.NewReader(table), int64(len(table)),
		fib, layout,
	)
	if len(fonts) != 1 {
		t.Fatalf("fonts=%#v, want one family", fonts)
	}
	if fonts[0].Family != "Oswald" || len(fonts[0].Faces) != 2 {
		t.Fatalf("font=%#v, want Oswald with two faces", fonts[0])
	}
	if fonts[0].Faces[0].ContentType != "application/vnd.ms-fontobject" {
		t.Fatalf("face[0] content type=%q, want EOT", fonts[0].Faces[0].ContentType)
	}
	if fonts[0].Faces[1].Weight != 700 || fonts[0].Faces[1].Style != "italic" {
		t.Fatalf("face[1]=%#v, want bold italic", fonts[0].Faces[1])
	}
}

func testFontRecord(name string) []byte {
	nameBytes := make([]byte, 0, len(name)*2+2)
	for _, r := range name {
		nameBytes = append(nameBytes, byte(r), 0)
	}
	nameBytes = append(nameBytes, 0, 0)
	record := make([]byte, 40+len(nameBytes))
	record[0] = byte(len(record) - 1)
	copy(record[40:], nameBytes)
	return record
}

func putEmbeddedDescriptor(dst []byte, offset, fontIndex int, flags uint16, subsetOrder uint32) {
	binary.LittleEndian.PutUint32(dst[0:4], uint32(offset))
	binary.LittleEndian.PutUint16(dst[4:6], uint16(fontIndex))
	binary.LittleEndian.PutUint16(dst[6:8], flags)
	binary.LittleEndian.PutUint32(dst[8:12], subsetOrder)
}

func putFcLcb(fib []byte, index, fc, lcb int) {
	off := index * 8
	binary.LittleEndian.PutUint32(fib[off:off+4], uint32(fc))
	binary.LittleEndian.PutUint32(fib[off+4:off+8], uint32(lcb))
}

func testEOTPayload(size int, fill byte) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = fill
	}
	binary.LittleEndian.PutUint32(data[0:4], uint32(size))
	binary.LittleEndian.PutUint32(data[8:12], 0x00020001)
	binary.LittleEndian.PutUint16(data[34:36], 0x504C)
	return data
}
