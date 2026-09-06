package event

import wordir "office/pkg/word/ir"

// Type identifies one semantic event emitted by a sequential Word parser.
// It is shared by DOC and DOCX-facing stream composition so format decoders do
// not need to duplicate the transport contract.
type Type string

const (
	StartDocument     Type = "startDocument"
	EndDocument       Type = "endDocument"
	StartParagraph    Type = "startParagraph"
	EndParagraph      Type = "endParagraph"
	StartTable        Type = "startTable"
	EndTable          Type = "endTable"
	StartTableRow     Type = "startTableRow"
	EndTableRow       Type = "endTableRow"
	StartTableCell    Type = "startTableCell"
	EndTableCell      Type = "endTableCell"
	Text              Type = "text"
	Tab               Type = "tab"
	Break             Type = "break"
	Drawing           Type = "drawing"
	EmbeddedObject    Type = "embeddedObject"
	BookmarkStart     Type = "bookmarkStart"
	BookmarkEnd       Type = "bookmarkEnd"
	FootnoteReference Type = "footnoteReference"
	EndnoteReference  Type = "endnoteReference"
	CommentReference  Type = "commentReference"
	Section           Type = "section"
)

// Event is the format-neutral semantic transport between Word parsers and
// materializers/layout. Fields may be nil when a format does not expose the
// corresponding provenance or document-wide semantic object.
type Event struct {
	Type Type
	Text string
	BreakType string
	ParagraphFormat *wordir.ParagraphFormat
	ParagraphNumbering *wordir.NumberingReference
	ParagraphSource *wordir.ParagraphSource
	ParagraphPropertyRevision *wordir.RevisionMark
	RunFormat *wordir.RunFormat
	RunSource *wordir.RunSource
	Revision string
	RevisionAuthor string
	RevisionDate string
	RunPropertyRevision *wordir.RevisionMark
	DisplayFieldRevision *wordir.RevisionMark
	FieldKind string
	FieldInstr string
	FieldArg string
	Target string
	Anchor string
	BookmarkName string
	NoteID int
	CommentID int
	Story *wordir.Story
	Section *wordir.Section
	Headers map[string]wordir.Story
	Footers map[string]wordir.Story
	FootnoteSeparator *wordir.Story
	FootnoteContinuationSep *wordir.Story
	FootnoteContinuationNotice *wordir.Story
	EndnoteSeparator *wordir.Story
	EndnoteContinuationSep *wordir.Story
	EndnoteContinuationNotice *wordir.Story
	Image *wordir.Image
	Shape *wordir.Shape
	EmbeddedRun *wordir.Run
}

type Handler func(Event) error
