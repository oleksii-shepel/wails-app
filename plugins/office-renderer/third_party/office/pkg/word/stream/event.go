package stream

import wordir "office/pkg/word/ir"

type EventType string

const (
	EventStartDocument EventType = "startDocument"
	EventEndDocument EventType = "endDocument"
	EventStartStory EventType = "startStory"
	EventEndStory EventType = "endStory"
	EventStartParagraph EventType = "startParagraph"
	EventEndParagraph EventType = "endParagraph"
	EventStartTable EventType = "startTable"
	EventEndTable EventType = "endTable"
	// EventTableProperty carries raw w:tblPr / w:tblGrid / w:trPr / w:tcPr XML
	// for the innermost open table element, so table styles can be resolved once
	// the table closes.
	EventTableProperty EventType = "tableProperty"
	EventStartTableRow EventType = "startTableRow"
	EventEndTableRow EventType = "endTableRow"
	EventStartTableCell EventType = "startTableCell"
	EventEndTableCell EventType = "endTableCell"
	EventStartHyperlink EventType = "startHyperlink"
	EventEndHyperlink EventType = "endHyperlink"
	EventStartRevision EventType = "startRevision"
	EventEndRevision EventType = "endRevision"
	EventStartContentControl EventType = "startContentControl"
	EventEndContentControl EventType = "endContentControl"
	EventStartCustomXML EventType = "startCustomXML"
	EventEndCustomXML EventType = "endCustomXML"
	EventStartSmartTag EventType = "startSmartTag"
	EventEndSmartTag EventType = "endSmartTag"
	EventStartDirection EventType = "startDirection"
	EventEndDirection EventType = "endDirection"
	EventCommentRangeStart EventType = "commentRangeStart"
	EventCommentRangeEnd EventType = "commentRangeEnd"
	EventSection EventType = "section"
	EventText EventType = "text"
	EventTab EventType = "tab"
	EventBreak EventType = "break"
	EventSymbol EventType = "symbol"
	EventSoftHyphen EventType = "softHyphen"
	EventNoBreakHyphen EventType = "noBreakHyphen"
	EventAnnotationReference EventType = "annotationReference"
	EventAutoText EventType = "autoText"
	EventRuby EventType = "ruby"
	EventMath EventType = "math"
	EventDrawing EventType = "drawing"
	EventEmbeddedObject EventType = "embeddedObject"
	EventContentPart EventType = "contentPart"
	EventAltChunk EventType = "altChunk"
	EventFieldInstruction EventType = "fieldInstruction"
	EventFieldChar EventType = "fieldChar"
	EventBookmarkStart EventType = "bookmarkStart"
	EventBookmarkEnd EventType = "bookmarkEnd"
	EventFootnoteReference EventType = "footnoteReference"
	EventEndnoteReference EventType = "endnoteReference"
	EventCommentReference EventType = "commentReference"
)

type Format string
const (
	FormatDOCX Format = "docx"
	FormatDOC Format = "doc"
)

type StoryKind string
const (
	StoryDocument StoryKind = "document"
	StoryHeader StoryKind = "header"
	StoryFooter StoryKind = "footer"
	StoryFootnote StoryKind = "footnote"
	StoryEndnote StoryKind = "endnote"
	StoryComment StoryKind = "comment"
)

type Event struct {
	Type EventType
	Format Format
	Story StoryKind
	PartName string
	Text string
	StyleID string
	RunStyleID string
	BreakType string
	FieldInstr string
	FieldCharType string
	FieldKind string
	FieldArg string
	Target string
	Anchor string
	RelationshipID string
	BookmarkID string
	BookmarkName string
	NoteID string
	CommentID string
	ParagraphFormat *wordir.ParagraphFormat
	ParagraphFrame  *wordir.FrameProps
	ParagraphNumbering *wordir.NumberingReference
	// RunFormat and RunStyleID describe the paragraph mark (w:pPr/w:rPr) on
	// EventStartParagraph and the text run on run-level events.
	RunFormat *wordir.RunFormat
	Section *wordir.Section
	Image *wordir.Image
	Shape *wordir.Shape
	EmbeddedRun *wordir.Run
	// TablePropertyScope is "table", "grid", "row", or "cell" on
	// EventTableProperty, naming the element that owns RawXML.
	TablePropertyScope string
	// RawXML carries the OOXML payload for events whose meaning is defined by
	// raw markup rather than by promoted fields.
	RawXML string
	Raw    any
}

type Handler interface { HandleEvent(Event) error }
type HandlerFunc func(Event) error
func (f HandlerFunc) HandleEvent(event Event) error { return f(event) }
type Source interface { Walk(Handler) error }
