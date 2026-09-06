package pre97

import (
	"errors"
	"fmt"

	"office/pkg/ooxml/cfb"
	wordevent "office/pkg/word/event"
)

// ErrNotApplicable indicates that a decoder does not recognize the opened
// compound file as a pre-Word97 document. Composition roots may continue with
// another decoder when this sentinel is returned.
var ErrNotApplicable = errors.New("doc pre97: not applicable")

// UnsupportedError reports a recognized old-DOC feature that the sequential
// contract cannot currently project without losing semantics.
type UnsupportedError struct{ Feature string }

func (e UnsupportedError) Error() string {
	return fmt.Sprintf("doc pre97: unsupported %s", e.Feature)
}

// EventType, Event and EventHandler remain exported aliases while the semantic
// transport schema is owned by pkg/word/event.
type EventType = wordevent.Type

const (
	EventStartDocument     = wordevent.StartDocument
	EventEndDocument       = wordevent.EndDocument
	EventStartParagraph    = wordevent.StartParagraph
	EventEndParagraph      = wordevent.EndParagraph
	EventStartTable        = wordevent.StartTable
	EventEndTable          = wordevent.EndTable
	EventStartTableRow     = wordevent.StartTableRow
	EventEndTableRow       = wordevent.EndTableRow
	EventStartTableCell    = wordevent.StartTableCell
	EventEndTableCell      = wordevent.EndTableCell
	EventText              = wordevent.Text
	EventTab               = wordevent.Tab
	EventBreak             = wordevent.Break
	EventDrawing           = wordevent.Drawing
	EventEmbeddedObject    = wordevent.EmbeddedObject
	EventBookmarkStart     = wordevent.BookmarkStart
	EventBookmarkEnd       = wordevent.BookmarkEnd
	EventFootnoteReference = wordevent.FootnoteReference
	EventEndnoteReference  = wordevent.EndnoteReference
	EventCommentReference  = wordevent.CommentReference
	EventSection           = wordevent.Section
)

type Event = wordevent.Event
type EventHandler = wordevent.Handler

// CompatibilityServices supplies only the still-legacy semantic decorators.
// Piece decoding, fields, references, bookmarks, section splitting and event
// orchestration are owned by the canonical pre97 package.
type CompatibilityServices interface {
	OpenOpenedCFB(*cfb.File) (Cursor, *Context, error)
}

func WalkOpenedCFB(f *cfb.File, services CompatibilityServices, handler EventHandler) error {
	if f == nil {
		return fmt.Errorf("doc pre97: nil cfb")
	}
	if services == nil {
		return fmt.Errorf("doc pre97: compatibility services are required")
	}
	if handler == nil {
		return fmt.Errorf("doc pre97: handler cannot be nil")
	}
	cursor, context, err := services.OpenOpenedCFB(f)
	if err != nil { return err }
	if context != nil {
		cursor = NewReferenceCursor(cursor, context.FootnoteRefs, context.EndnoteRefs, context.CommentRefs)
		cursor = NewBookmarkCursor(cursor, context.BookmarkStarts, context.BookmarkEnds)
		cursor = NewSectionBoundaryCursor(cursor, context.Sections)
	}
	return Walk(cursor, context, handler)
}
