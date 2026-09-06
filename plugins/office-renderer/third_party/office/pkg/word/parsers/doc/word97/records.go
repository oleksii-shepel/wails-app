package word97

import (
	"encoding/binary"
	"io"
	"unicode/utf16"

	docbinary "office/pkg/word/parsers/doc/binary"
)

const fibRevisionAuthors = 51

// LoadRevisionAuthorsReaderAt reads SttbfRMark in stable author-index order.
// Binary ownership stays in the canonical Word97 reader so compatibility
// adapters only resolve semantic author references.
func LoadRevisionAuthorsReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) []string {
	blob, ok := readTableRange(table, tableSize, fib, layout, fibRevisionAuthors, 6)
	if !ok {
		return nil
	}
	return ParseRevisionAuthors(blob)
}

// ParseRevisionAuthors decodes one raw SttbfRMark payload in stable author-index
// order. It is exported so tests and compatibility boundaries can reuse the
// canonical record grammar without reimplementing the Unicode STTB decoder.
func ParseRevisionAuthors(blob []byte) []string {
	if len(blob) < 6 || binary.LittleEndian.Uint16(blob[0:2]) != 0xFFFF {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(blob[2:4]))
	extra := int(binary.LittleEndian.Uint16(blob[4:6]))
	pos := 6
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		if pos+2 > len(blob) {
			break
		}
		cch := int(int16(binary.LittleEndian.Uint16(blob[pos : pos+2])))
		pos += 2
		if cch < 0 {
			out = append(out, "")
			continue
		}
		if pos+cch*2 > len(blob) {
			break
		}
		chars := make([]uint16, cch)
		for j := range chars {
			chars[j] = binary.LittleEndian.Uint16(blob[pos+j*2 : pos+j*2+2])
		}
		out = append(out, string(utf16.Decode(chars)))
		pos += cch * 2
		if pos+extra > len(blob) {
			break
		}
		pos += extra
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// LoadDOPReaderAt returns the raw FIB-addressed DOP record for lossless source
// preservation. Semantic DOP decoding is provided by LoadDocumentSettingsReaderAt.
func LoadDOPReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) []byte {
	blob, ok := readTableRange(table, tableSize, fib, layout, fibDOP, 1)
	if !ok {
		return nil
	}
	return blob
}
