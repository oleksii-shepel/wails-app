package word97

import (
	"io"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

const (
	fibStyleSheet    = 1
	fibFontTable     = 15
	fibDOP           = 31
	fibCLX           = 33
	fibEmbeddedFonts = 61
)

// LoadDocumentSourceReaderAt loads document-wide lossless provenance directly
// from FIB-addressed table ranges. It does not parse or materialize body text.
func LoadDocumentSourceReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) *wordir.DocumentSource {
	if table == nil {
		return &wordir.DocumentSource{Version: layout.Version}
	}
	source := &wordir.DocumentSource{Version: layout.Version}
	if blob, ok := readTableRange(table, tableSize, fib, layout, fibDOP, 1); ok {
		source.DOP = append([]byte(nil), blob...)
	}
	if blob, ok := readTableRange(table, tableSize, fib, layout, fibCLX, 1); ok {
		source.ComplexFileTable = append([]byte(nil), blob...)
	}
	if blob, ok := readTableRange(table, tableSize, fib, layout, fibFontTable, 1); ok {
		source.FontTable = append([]byte(nil), blob...)
	}
	if blob, ok := readTableRange(table, tableSize, fib, layout, fibStyleSheet, 1); ok {
		source.StyleSheet = append([]byte(nil), blob...)
	}
	return source
}
