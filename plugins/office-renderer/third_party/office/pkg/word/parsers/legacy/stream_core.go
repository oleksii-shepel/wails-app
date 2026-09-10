package legacy

import (
	"office/pkg/diag"
	wordir "office/pkg/word/ir"
)

// legacyDocStreamCore is the semantic compatibility context used by the
// remaining legacy/pre-Word97 streaming adapters. Normal Word97 parsing no
// longer stages its canonical snapshot through this type.
type legacyDocStreamCore struct {
	ListTables                 *binaryWordListCatalog
	StyleSheet                 *binaryWordStyleCatalog
	Compatibility              *wordir.DocCompatibility
	FontFamilies               map[string]wordir.FontFamilyInfo
	EmbeddedFonts              []*wordir.EmbeddedFont
	BookmarkStarts             map[int][]string
	BookmarkEnds               map[int][]string
	FootnoteRefs               map[int]int
	EndnoteRefs                map[int]int
	CommentRefs                map[int]int
	Footnotes                  map[int]wordir.Story
	Endnotes                   map[int]wordir.Story
	Comments                   map[int]wordir.Story
	Sections                   []wordir.Section
	Headers                    map[string]wordir.Story
	Footers                    map[string]wordir.Story
	FootnoteSeparator          wordir.Story
	FootnoteContinuationSep    wordir.Story
	FootnoteContinuationNotice wordir.Story
	EndnoteSeparator           wordir.Story
	EndnoteContinuationSep     wordir.Story
	EndnoteContinuationNotice  wordir.Story
	InlineImages               []*wordir.Image
	TextBoxes                  []*wordir.TextBox
	OLEByCP                    map[int]*wordir.Run
	RawDOP                     []byte
	Diagnostics                diag.List
	Version                    uint16
}
