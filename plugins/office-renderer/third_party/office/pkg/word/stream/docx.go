package stream

import (
	"io"

	wordsax "office/pkg/word/parsers/ooxml/sax"
)

// DOCXOptions controls optional OOXML stories emitted by DOCXSource.
type DOCXOptions struct {
	IncludeFieldInstructions bool
	IncludeHeadersAndFooters bool
	IncludeFootnotes         bool
	IncludeEndnotes          bool
	IncludeComments          bool
}

// DOCXSource adapts the existing WordprocessingML SAX walker to the unified
// Word event IR without materializing the semantic Document first.
type DOCXSource struct {
	Reader  io.ReaderAt
	Size    int64
	Options DOCXOptions
}

// NewDOCXSource creates a streaming OOXML source over a seekable ZIP payload.
func NewDOCXSource(r io.ReaderAt, size int64, opts DOCXOptions) *DOCXSource {
	return &DOCXSource{Reader: r, Size: size, Options: opts}
}

// Walk streams the DOCX in document order through the normalized event IR.
func (s *DOCXSource) Walk(handler Handler) error {
	if s == nil {
		return nil
	}
	baseTarget := docxHandlerForLayout(handler)
	var layoutTarget *docxLayoutHandler
	if candidate, ok := baseTarget.(*docxLayoutHandler); ok {
		layoutTarget = candidate
		loader, _ := newDOCXNoteLoader(s.Reader, s.Size)
		layoutTarget.notes = loader
		media, _ := newDOCXMediaLoader(s.Reader, s.Size)
		layoutTarget.media = media
		headers, _ := newDOCXHeaderFooterLoader(s.Reader, s.Size)
		layoutTarget.headers = headers
		if headers != nil {
			defer headers.Close()
		}
		comments, _ := newDOCXCommentLoader(s.Reader, s.Size)
		layoutTarget.comments = comments
		if metadata, err := loadDOCXRenderMetadata(s.Reader, s.Size); err == nil && metadata != nil {
			layoutTarget.session.SetRenderMetadata(metadata)
		}
	}
	target := wrapDOCXDrawingSemantics(wrapDOCXLayoutSemantics(baseTarget), layoutTarget)

	// Promote pPr/rPr/sectPr through a cursor that walks the main XML lazily in
	// parallel with the structural SAX reader. It stays at most one paragraph
	// ahead, so a bounded page request can still stop without scanning the rest
	// of a plain ZIP-backed DOCX. Encrypted/non-ZIP packages fall back to the
	// structural SAX stream until these payloads move directly into sax.Event.
	cursor, _ := newDOCXPropertyCursor(s.Reader, s.Size)
	if cursor != nil {
		defer cursor.Close()
	}
	numberingCursor, _ := newDOCXNumberingCursor(s.Reader, s.Size)
	if numberingCursor != nil {
		defer numberingCursor.Close()
	}
	var currentParagraph *docxParagraphPlan

	return wordsax.WalkReaderAt(s.Reader, s.Size, wordsax.HandlerFunc(func(event wordsax.Event) error {
		converted := fromSAXEvent(event)

		if cursor != nil && event.Story == wordsax.StoryDocument {
			switch event.Type {
			case wordsax.EventStartParagraph:
				paragraph, err := cursor.NextParagraph()
				if err != nil {
					return err
				}
				currentParagraph = paragraph
				if paragraph != nil {
					props := paragraph.Properties
					if props.StyleID != "" {
						converted.StyleID = props.StyleID
					}
					format := paragraphFormatFromSAX(props)
					converted.ParagraphFormat = &format
				}
				if numberingCursor != nil {
					ref, err := numberingCursor.NextParagraph()
					if err != nil {
						return err
					}
					converted.ParagraphNumbering = ref
				}
				if err := target.HandleEvent(converted); err != nil {
					return err
				}
				if layoutTarget != nil && layoutTarget.session != nil && layoutTarget.session.paragraph != nil && layoutTarget.session.metadata != nil {
					numbering, list := layoutTarget.session.metadata.ResolveParagraphNumbering(converted.StyleID, converted.ParagraphNumbering)
					layoutTarget.session.paragraph.Numbering = numbering
					layoutTarget.session.paragraph.List = list
				}
				return nil
			case wordsax.EventText:
				if props := cursor.NextTextRun(); props != nil {
					converted.RunStyleID = props.StyleID
					format := runFormatFromSAX(*props)
					converted.RunFormat = &format
				}
			case wordsax.EventEndParagraph:
				if err := target.HandleEvent(converted); err != nil {
					return err
				}
				if currentParagraph != nil && currentParagraph.Section != nil {
					section := sectionFromSAX(*currentParagraph.Section)
					if err := target.HandleEvent(Event{
						Type:     EventSection,
						Format:   FormatDOCX,
						Story:    StoryDocument,
						PartName: event.PartName,
						Section:  &section,
						Raw:      *currentParagraph.Section,
					}); err != nil {
						return err
					}
				}
				currentParagraph = nil
				return nil
			case wordsax.EventEndDocument:
				sectionProps, err := cursor.FinalSection()
				if err != nil {
					return err
				}
				if sectionProps != nil {
					section := sectionFromSAX(*sectionProps)
					if err := target.HandleEvent(Event{
						Type:     EventSection,
						Format:   FormatDOCX,
						Story:    StoryDocument,
						PartName: event.PartName,
						Section:  &section,
						Raw:      *sectionProps,
					}); err != nil {
						return err
					}
				}
			}
		}

		return target.HandleEvent(converted)
	}), wordsax.Options{
		IncludeFieldInstructions: s.Options.IncludeFieldInstructions,
		IncludeHeadersAndFooters: s.Options.IncludeHeadersAndFooters,
		IncludeFootnotes:         s.Options.IncludeFootnotes,
		IncludeEndnotes:          s.Options.IncludeEndnotes,
		IncludeComments:          s.Options.IncludeComments,
	})
}

func fromSAXEvent(event wordsax.Event) Event {
	return Event{
		Type:               EventType(event.Type),
		Format:             FormatDOCX,
		Story:              StoryKind(event.Story),
		PartName:           event.PartName,
		Text:               event.Text,
		StyleID:            event.StyleID,
		BreakType:          event.BreakType,
		FieldInstr:         event.FieldInstr,
		FieldCharType:      event.FieldCharType,
		RelationshipID:     event.RelationshipID,
		BookmarkID:         event.BookmarkID,
		BookmarkName:       event.BookmarkName,
		NoteID:             event.NoteID,
		CommentID:          event.CommentID,
		Raw:                event,
	}
}
