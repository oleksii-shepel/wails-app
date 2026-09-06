package pre97

import (
	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"
)

// Run is the canonical sequential semantic unit consumed by the pre-Word97
// event state machine. Compatibility decoders may decorate runs before adapting
// them to this contract, but event orchestration no longer depends on legacy
// parser types.
type Run struct {
	Text                  string
	Kind                  string
	Format                wordir.RunFormat
	ParagraphFormat       wordir.ParagraphFormat
	TableDepth            int
	LegacyInTableHint     bool
	LegacyRowMarkHint     bool
	TableProps            *docformat.TableRowProps
	StartCP               int
	FieldKind             string
	FieldInstr            string
	FieldArg              string
	Target                string
	Anchor                string
	NoteID                int
	CommentID             int
	Image                  *wordir.Image
	Shape                  *wordir.Shape
	Spec                   bool
	Data                   bool
	Obj                    bool
	ChpxSprms              []byte
	PapxSprms              []byte
}

// Cursor is the bounded sequential source consumed by Walk.
type Cursor interface {
	Next() (Run, bool, error)
}

// Context carries document-level semantic state needed while emitting events
// and by the canonical CP-ordered decorators. It deliberately excludes
// parser-specific binary structures.
type Context struct {
	Sections                   []wordir.Section
	Headers                    map[string]wordir.Story
	Footers                    map[string]wordir.Story
	Footnotes                  map[int]wordir.Story
	Endnotes                   map[int]wordir.Story
	Comments                   map[int]wordir.Story
	FootnoteRefs               map[int]int
	EndnoteRefs                map[int]int
	CommentRefs                map[int]int
	BookmarkStarts             map[int][]string
	BookmarkEnds               map[int][]string
	OLEByCP                    map[int]*wordir.Run
	FootnoteSeparator          wordir.Story
	FootnoteContinuationSep    wordir.Story
	FootnoteContinuationNotice wordir.Story
	EndnoteSeparator           wordir.Story
	EndnoteContinuationSep     wordir.Story
	EndnoteContinuationNotice  wordir.Story
}
