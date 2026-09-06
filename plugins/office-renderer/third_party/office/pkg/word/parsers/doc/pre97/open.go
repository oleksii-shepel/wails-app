package pre97

import (
	docbinary "office/pkg/word/parsers/doc/binary"

	"golang.org/x/text/encoding/charmap"
)

// Core is the canonical structural/formatting state for one Word 6/95
// WordDocument stream. Higher-level story and event composition builds on this
// without reopening FIB, font, FKP, CLX, style, or side-story metadata.
type Core struct {
	Span             Span
	Fonts            []FontEntry
	FontNames        []string
	PreferredCharset *charmap.Charmap
	CHPX             []CHPXEntry
	PAPX             []PAPXEntry
	Complex          *docbinary.ComplexFileTable
	PiecePLC         []byte
	Styles           *StyleCatalog

	Bookmarks    BookmarkData
	Stories      StoryRanges
	HeaderFooter HeaderFooterRanges
	Sections     []SectionRecord
}

// OpenCore opens the canonical Word 6/95 binary core from an in-memory
// WordDocument stream. A structurally valid FIB returns a Core even when a
// malformed complex CLX cannot produce a piece PLC; callers retain the same
// compatibility/fallback decision they had before this migration.
func OpenCore(word []byte) (*Core, bool) {
	span, ok := InspectBodySpan(word)
	if !ok { return nil, false }
	fonts := LoadFontTable(word, span.FontTableOffset, span.FontTableSize)
	fontNames := FontNames(fonts)
	bookmarks, stories, headerFooter, sections := LoadMetadata(word, span)
	core := &Core{
		Span:             span,
		Fonts:            fonts,
		FontNames:        fontNames,
		PreferredCharset: GuessCharset(fonts),
		CHPX:             LoadCHPXEntries(word, span),
		PAPX:             LoadPAPXEntries(word, span),
		Styles:           LoadStyleCatalog(word, span, fontNames),
		Bookmarks:        bookmarks,
		Stories:          stories,
		HeaderFooter:     headerFooter,
		Sections:         sections,
	}
	if span.FComplex {
		if span.ComplexTableOffset > 0 && span.ComplexTableOffset < len(word) {
			core.Complex = docbinary.ParseComplexFileTable(word[span.ComplexTableOffset:])
			if core.Complex != nil { core.PiecePLC = core.Complex.PlcPcd }
		}
	} else {
		core.PiecePLC = BuildSinglePiecePLC(span)
	}
	return core, true
}
