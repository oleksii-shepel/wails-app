package word97

import (
	"errors"
	"fmt"
	"io"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
	docformat "office/pkg/word/parsers/doc/format"
)

// OpenCore is the bounded, document-wide state required to construct the
// canonical Word97 run stream. It owns FIB-addressed document metadata and
// formatting discovery so compatibility adapters only project canonical state.
type OpenCore struct {
	FIB             []byte
	Layout          docbinary.Word97Layout
	Table           io.ReaderAt
	TableSize       int64
	Source          *wordir.DocumentSource
	RawCLX          []byte
	Complex         *docbinary.ComplexFileTable
	CHPX            []docformat.CHPXEntry
	PAPX            []docformat.PAPXEntry
	Metadata        Metadata
	Settings        DocumentSettings
	RawDOP          []byte
	RevisionAuthors []string
	FontFamilies    map[string]wordir.FontFamilyInfo
	EmbeddedFonts   []*wordir.EmbeddedFont
	SideStories     SideStoryRanges
	HeaderFooters   *HeaderFooterRanges
	Sections        []SectionRecord
	Styles          *StyleCatalog
	Numbering       *NumberingCatalog
	Drawings        DrawingMetadata
	Cursor          Cursor
}

// OpenCoreReaderAt owns normal Word97+ core construction: bounded FIB/table
// access, CLX/PieceTable decoding, FKP formatting discovery and the canonical
// piece cursor. It does not materialize the document body or table stream.
func OpenCoreReaderAt(word io.ReaderAt, wordSize int64, f *cfb.File) (*OpenCore, error) {
	if word == nil || wordSize <= 0 {
		return nil, fmt.Errorf("doc word97 open: invalid WordDocument reader")
	}
	if f == nil {
		return nil, fmt.Errorf("doc word97 open: nil compound file")
	}

	fib, ok := docbinary.ReadWord97FIBReaderAt(word, wordSize)
	if !ok {
		return nil, fmt.Errorf("doc word97 open: invalid FIB")
	}
	layout, ok := docbinary.ParseWord97Layout(fib)
	if !ok {
		return nil, fmt.Errorf("doc word97 open: unsupported FIB layout")
	}
	table, tableSize, ok, err := f.StreamReaderAt(layout.TableStreamName)
	if err != nil {
		return nil, fmt.Errorf("doc word97 open: open %s: %w", layout.TableStreamName, err)
	}
	if !ok || table == nil || tableSize <= 0 {
		return nil, fmt.Errorf("doc word97 open: missing %s stream", layout.TableStreamName)
	}

	clx, ok := readTableRange(table, tableSize, fib, layout, 33, 1)
	if !ok {
		return nil, fmt.Errorf("doc word97 open: missing CLX")
	}
	complex := docbinary.ParseComplexFileTable(clx)
	if complex == nil || len(complex.PlcPcd) == 0 {
		return nil, fmt.Errorf("doc word97 open: invalid PieceTable")
	}

	fontNames := LoadFontNamesReaderAt(table, tableSize, fib, layout)
	chpx := loadCHPXReaderAt(word, wordSize, table, tableSize, fib, layout, fontNames)
	papx := loadPAPXReaderAt(word, wordSize, table, tableSize, fib, layout)
	piece, err := NewPieceReaderAtCursor(word, wordSize, complex.PlcPcd, chpx, papx, nil)
	if err != nil {
		return nil, err
	}

	metadata := LoadMetadataReaderAt(table, tableSize, fib, layout)
	sections := LoadSectionRecords(word, wordSize, table, tableSize, fib, layout)
	return &OpenCore{
		FIB:             fib,
		Layout:          layout,
		Table:           table,
		TableSize:       tableSize,
		Source:          LoadDocumentSourceReaderAt(table, tableSize, fib, layout),
		RawCLX:          append([]byte(nil), clx...),
		Complex:         complex,
		CHPX:            chpx,
		PAPX:            papx,
		Metadata:        metadata,
		Settings:        LoadDocumentSettingsReaderAt(table, tableSize, fib, layout),
		RawDOP:          LoadDOPReaderAt(table, tableSize, fib, layout),
		RevisionAuthors: LoadRevisionAuthorsReaderAt(table, tableSize, fib, layout),
		FontFamilies:    LoadFontFamiliesReaderAt(table, tableSize, fib, layout),
		EmbeddedFonts:   LoadEmbeddedFontsReaderAt(word, wordSize, table, tableSize, fib, layout),
		SideStories:     LoadSideStoryRanges(table, tableSize, fib, layout, metadata.StoryCounts),
		HeaderFooters:   LoadHeaderFooterRanges(table, tableSize, fib, layout, metadata.StoryCounts, len(sections)),
		Sections:        sections,
		Styles:          LoadStyleCatalogReaderAt(table, tableSize, fib, layout),
		Numbering:       LoadNumberingCatalogReaderAt(table, tableSize, fib, layout),
		Drawings:        LoadDrawingMetadataReaderAt(table, tableSize, fib, layout, metadata.StoryCounts),
		Cursor:          piece,
	}, nil
}

func loadCHPXReaderAt(word io.ReaderAt, wordSize int64, table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout, fontNames []string) []docformat.CHPXEntry {
	plc, ok := readTableRange(table, tableSize, fib, layout, 12, 12)
	if !ok {
		return nil
	}
	pages, err := docbinary.BTEPageNumbers(plc)
	if err != nil {
		return nil
	}
	var out []docformat.CHPXEntry
	for _, pn := range pages {
		page := readFKPPage(word, wordSize, int64(pn))
		if page != nil {
			out = append(out, docformat.ParseCHPXFKP(page, fontNames, nil)...)
		}
	}
	return out
}

func loadPAPXReaderAt(word io.ReaderAt, wordSize int64, table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) []docformat.PAPXEntry {
	plc, ok := readTableRange(table, tableSize, fib, layout, 13, 12)
	if !ok {
		return nil
	}
	pages, err := docbinary.BTEPageNumbers(plc)
	if err != nil {
		return nil
	}
	var out []docformat.PAPXEntry
	for _, pn := range pages {
		page := readFKPPage(word, wordSize, int64(pn))
		if page != nil {
			out = append(out, docformat.ParsePAPXFKP(page, nil)...)
		}
	}
	return out
}

func readFKPPage(word io.ReaderAt, wordSize, pageNumber int64) []byte {
	const pageSize = int64(512)
	off := pageNumber * pageSize
	if word == nil || pageNumber < 0 || off < 0 || off+pageSize > wordSize {
		return nil
	}
	buf := make([]byte, pageSize)
	n, err := word.ReadAt(buf, off)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil
	}
	if n != len(buf) {
		return nil
	}
	return buf
}
