package word97

import (
	"bytes"
	"io"

	docbinary "office/pkg/word/parsers/doc/binary"
	docformat "office/pkg/word/parsers/doc/format"
)

// FormatEntries is the canonical Word97 character/paragraph formatting set
// collected from the FKP page tables referenced by the FIB.
type FormatEntries struct {
	CHPX []docformat.CHPXEntry
	PAPX []docformat.PAPXEntry
}

// LoadFormatEntriesFromStreams is the compatibility-facing Word97 formatting
// entrypoint for callers that already materialized WordDocument and its table
// stream. FIB navigation, ordered font resolution, BTE traversal, and FKP page
// decoding remain entirely in the canonical parser.
func LoadFormatEntriesFromStreams(wordStream, tableStream []byte) FormatEntries {
	layout, ok := docbinary.ParseWord97Layout(wordStream)
	if !ok {
		return FormatEntries{}
	}
	return LoadFormatEntriesBytes(wordStream, tableStream, wordStream, layout, nil)
}

// LoadFormatEntriesBytes reuses the canonical ReaderAt FKP loaders for callers
// that already hold whole WordDocument and table streams in memory. Font names
// are resolved from the canonical SttbfFfn reader when callers do not already
// have an ordered font-name table.
func LoadFormatEntriesBytes(wordStream, tableStream []byte, fib []byte, layout docbinary.Word97Layout, fontNames []string) FormatEntries {
	if len(wordStream) == 0 || len(tableStream) == 0 || len(fib) == 0 {
		return FormatEntries{}
	}
	word := bytes.NewReader(wordStream)
	table := bytes.NewReader(tableStream)
	if len(fontNames) == 0 {
		fontNames = LoadFontNamesReaderAt(table, int64(len(tableStream)), fib, layout)
	}
	return FormatEntries{
		CHPX: loadCHPXReaderAt(word, int64(len(wordStream)), table, int64(len(tableStream)), fib, layout, fontNames),
		PAPX: loadPAPXReaderAt(word, int64(len(wordStream)), table, int64(len(tableStream)), fib, layout),
	}
}

// LoadFontNamesReaderAt returns the SttbfFfn entries in font-index order for
// consumers that need to resolve CHPX font references without owning FFN
// parsing. The reader retains Word97's malformed-slot compatibility scanning.
func LoadFontNamesReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) []string {
	entries := loadOrderedFontEntriesReaderAt(table, tableSize, fib, layout)
	if len(entries) == 0 {
		return nil
	}
	names := make([]string, len(entries))
	for i := range entries {
		names[i] = entries[i].Name
	}
	return names
}
