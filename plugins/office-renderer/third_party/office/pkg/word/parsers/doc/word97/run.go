package word97

import (
	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"
)

// Run is the sequential Word97 run model shared by piece decoding and the
// semantic decorator pipeline. Binary formatting/provenance stays canonical,
// while synthetic semantic runs (fields, references, bookmarks, drawings)
// travel through the same bounded stream without falling back to legacy types.
type Run struct {
	Text                      string
	Kind                      string
	Format                    wordir.RunFormat
	ParagraphFormat           wordir.ParagraphFormat
	TableDepth                int
	LegacyInTableHint         bool
	LegacyRowMarkHint         bool
	TableProps                *docformat.TableRowProps
	StartCP                   int
	Revision                  string
	RevisionAuthor            string
	RevisionDate              string
	ParagraphPropertyRevision *wordir.RevisionMark
	RunPropertyRevision       *wordir.RevisionMark
	DisplayFieldRevision      *wordir.RevisionMark

	FieldKind  string
	FieldInstr string
	FieldArg   string
	Target     string
	Anchor     string
	NoteID     int
	CommentID  int

	Image       *wordir.Image
	Shape       *wordir.Shape
	EmbeddedRun *wordir.Run
	FormField   *wordir.FormField
	Fields      []*docformat.MaterializedField[Run]

	IbstRMark    uint16
	HasIbstRMark bool
	DttmRMark    uint32
	HasDttmRMark bool

	PropRMarkIbst    uint16
	HasPropRMarkIbst bool
	PropRMarkDttm    uint32
	HasPropRMarkDttm bool
	HasPropRMark     bool

	RunPropRMarkIbst    uint16
	HasRunPropRMarkIbst bool
	RunPropRMarkDttm    uint32
	HasRunPropRMarkDttm bool
	HasRunPropRMark     bool

	DispFldRMarkIbst    uint16
	HasDispFldRMarkIbst bool
	DispFldRMarkDttm    uint32
	HasDispFldRMarkDttm bool
	HasDispFldRMark     bool

	Spec bool
	Data bool
	Obj  bool

	// PicLocation is the sprmCPicLocation Data-stream byte offset. The separate
	// presence bit is required because offset 0 is a valid location for the
	// first PICF in a Word97 Data stream.
	PicLocation    int
	HasPicLocation bool

	ChpxSprms []byte
	PapxSprms []byte
}

// Cursor is the bounded sequential Word97 run source used by semantic
// decorators. PieceReaderAtCursor and PieceCursor both satisfy this contract.
type Cursor interface {
	Next() (Run, bool, error)
}
