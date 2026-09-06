package stream

import (
	"errors"
	"strconv"

	wordir "office/pkg/word/ir"
	docparser "office/pkg/word/parsers/doc"
)

// DOCSource adapts the sequential binary DOC reader to the unified Word event IR.
// Data is used by in-memory callers; Path selects the file-backed CFB reader.
type DOCSource struct {
	Data []byte
	Path string
}

func NewDOCSource(data []byte) *DOCSource     { return &DOCSource{Data: data} }
func NewDOCFileSource(path string) *DOCSource { return &DOCSource{Path: path} }

func numberingFromReference(ref *wordir.NumberingReference) *wordir.Numbering {
	if ref == nil {
		return nil
	}
	return &wordir.Numbering{NumID: ref.NumID, Level: ref.Level}
}

// splitLegacyFixtureCaption promotes the title/caption convention used by
// legacy DOC fixture documents into separate semantic blocks. Word stores the
// two differently formatted runs in one paragraph, but treating the caption as
// inline text loses its authored display position in HTML.
func (s *DOCSource) Walk(handler Handler) error {
	if s == nil {
		return nil
	}
	layout, _ := handler.(*LayoutSession)
	consume := func(event docparser.StreamEvent) error {
		if layout != nil {
			switch event.Type {
			case docparser.StreamStartDocument:
				if len(event.Headers) > 0 {
					layout.doc.Headers = cloneDOCStoryMap(event.Headers)
				}
				if len(event.Footers) > 0 {
					layout.doc.Footers = cloneDOCStoryMap(event.Footers)
				}
				if event.FootnoteSeparator != nil {
					layout.doc.FootnoteSeparator = *event.FootnoteSeparator
				}
				if event.FootnoteContinuationSep != nil {
					layout.doc.FootnoteContinuationSep = *event.FootnoteContinuationSep
				}
				if event.FootnoteContinuationNotice != nil {
					layout.doc.FootnoteContinuationNotice = *event.FootnoteContinuationNotice
				}
				if event.EndnoteSeparator != nil {
					layout.doc.EndnoteSeparator = *event.EndnoteSeparator
				}
				if event.EndnoteContinuationSep != nil {
					layout.doc.EndnoteContinuationSep = *event.EndnoteContinuationSep
				}
				if event.EndnoteContinuationNotice != nil {
					layout.doc.EndnoteContinuationNotice = *event.EndnoteContinuationNotice
				}
				return handler.HandleEvent(Event{Type: EventStartDocument, Format: FormatDOC, Story: StoryDocument, Raw: event})
			case docparser.StreamSection:
				return handler.HandleEvent(Event{Type: EventSection, Format: FormatDOC, Story: StoryDocument, Section: event.Section, Raw: event})
			case docparser.StreamDrawing:
				layout.ensureParagraph()
				run := wordir.Run{Kind: "shape", Shape: event.Shape, Image: event.Image}
				if event.Image != nil {
					run.Kind = "image"
				}
				if event.RunFormat != nil {
					run.Format = *event.RunFormat
				}
				layout.paragraph.Runs = append(layout.paragraph.Runs, run)
				return nil
			case docparser.StreamEmbeddedObject:
				layout.ensureParagraph()
				run := wordir.Run{Kind: "olePlaceholder", Text: "Embedded Object", OLEMIME: "application/octet-stream", OLEType: "Embed"}
				if event.EmbeddedRun != nil {
					run = *event.EmbeddedRun
				}
				if event.RunFormat != nil {
					run.Format = *event.RunFormat
				}
				layout.paragraph.Runs = append(layout.paragraph.Runs, run)
				return nil
			case docparser.StreamBookmarkStart:
				layout.ensureParagraph()
				if event.BookmarkName != "" {
					layout.paragraph.Bookmarks = appendUniqueString(layout.paragraph.Bookmarks, event.BookmarkName)
				}
				return nil
			case docparser.StreamBookmarkEnd:
				return nil
			case docparser.StreamStartParagraph:
				err := handler.HandleEvent(Event{Type: EventStartParagraph, Format: FormatDOC, Story: StoryDocument, ParagraphFormat: event.ParagraphFormat, ParagraphNumbering: event.ParagraphNumbering, Raw: event})
				if err == nil && layout.paragraph != nil {
					layout.paragraph.Numbering = numberingFromReference(event.ParagraphNumbering)
				}
				return err
			case docparser.StreamText:
				layout.ensureParagraph()
				run := wordir.Run{Kind: "run", Text: event.Text, FieldKind: event.FieldKind, FieldInstr: event.FieldInstr, FieldArg: event.FieldArg, Target: event.Target, Anchor: event.Anchor}
				if event.RunFormat != nil {
					run.Format = *event.RunFormat
				}
				layout.paragraph.Text += event.Text
				layout.paragraph.Runs = append(layout.paragraph.Runs, run)
				return nil
			case docparser.StreamFootnoteReference, docparser.StreamEndnoteReference:
				layout.ensureParagraph()
				kind := "footnoteRef"
				if event.Type == docparser.StreamEndnoteReference {
					kind = "endnoteRef"
				}
				run := wordir.Run{Kind: kind, NoteID: event.NoteID}
				if event.RunFormat != nil {
					run.Format = *event.RunFormat
				}
				layout.paragraph.Runs = append(layout.paragraph.Runs, run)
				if event.Story != nil {
					if event.Type == docparser.StreamFootnoteReference {
						if layout.doc.Footnotes == nil {
							layout.doc.Footnotes = map[int]wordir.Story{}
						}
						layout.doc.Footnotes[event.NoteID] = *event.Story
					} else {
						if layout.doc.Endnotes == nil {
							layout.doc.Endnotes = map[int]wordir.Story{}
						}
						layout.doc.Endnotes[event.NoteID] = *event.Story
					}
				}
				return nil
			case docparser.StreamCommentReference:
				layout.ensureParagraph()
				run := wordir.Run{Kind: "commentRef", CommentID: event.CommentID}
				if event.RunFormat != nil {
					run.Format = *event.RunFormat
				}
				layout.paragraph.Runs = append(layout.paragraph.Runs, run)
				if event.Story != nil {
					if layout.doc.Comments == nil {
						layout.doc.Comments = map[int]wordir.Story{}
					}
					layout.doc.Comments[event.CommentID] = *event.Story
				}
				return nil
			}
		}
		ev := Event{
			Type: EventType(event.Type), Format: FormatDOC, Story: StoryDocument,
			Text: event.Text, BreakType: event.BreakType,
			ParagraphFormat: event.ParagraphFormat, ParagraphNumbering: event.ParagraphNumbering,
			RunFormat: event.RunFormat, FieldKind: event.FieldKind, FieldInstr: event.FieldInstr,
			FieldArg: event.FieldArg, Target: event.Target, Anchor: event.Anchor,
			BookmarkName: event.BookmarkName, Section: event.Section,
			Image: event.Image, Shape: event.Shape, EmbeddedRun: event.EmbeddedRun, Raw: event,
		}
		if event.NoteID > 0 {
			ev.NoteID = strconv.Itoa(event.NoteID)
		}
		if event.CommentID > 0 {
			ev.CommentID = strconv.Itoa(event.CommentID)
		}
		return handler.HandleEvent(ev)
	}

	var err error
	if s.Path != "" {
		err = docparser.WalkFile(s.Path, consume)
	} else {
		err = docparser.WalkBytes(s.Data, consume)
	}
	if err == nil {
		return nil
	}
	var unsupported docparser.StreamUnsupportedError
	if errors.As(err, &unsupported) {
		return StreamUnsupportedEventError{Type: EventType("legacy:" + unsupported.Feature)}
	}
	return err
}

func cloneDOCStoryMap(src map[string]wordir.Story) map[string]wordir.Story {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]wordir.Story, len(src))
	for key, story := range src {
		out[key] = story
	}
	return out
}
