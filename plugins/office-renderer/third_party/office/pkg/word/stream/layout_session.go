package stream

import (
	"errors"
	"fmt"

	wordir "office/pkg/word/ir"
	wordlayout "office/pkg/word/layout"
	wordresolve "office/pkg/word/parsers/ooxml/resolve"
)

const streamPaginationBlockChunk = 128

// ErrStreamWindowComplete is returned when the requested page window has been
// completed. Sources should propagate it so SAX/binary walking can stop early.
var ErrStreamWindowComplete = errors.New("word layout stream window complete")

// StreamUnsupportedEventError reports a normalized event that cannot yet be
// represented safely by the bounded semantic bridge.
type StreamUnsupportedEventError struct {
	Type EventType
}

func (e StreamUnsupportedEventError) Error() string {
	return fmt.Sprintf("word stream event %q is not supported by bounded semantic layout", e.Type)
}

type tableFrame struct {
	table wordir.Table
	row   *wordir.TableRow
	cell  *wordir.TableCell
}

// LayoutSession consumes normalized Word events and incrementally builds the
// semantic prefix needed by the existing paginator. Body paragraphs, nested
// tables, promoted formatting and section markers are represented directly in
// word/ir; richer drawing/field/note constructs still fail explicitly.
type LayoutSession struct {
	window           *wordlayout.PageWindowSink
	doc              wordir.Document
	paragraph        *wordir.Paragraph
	paragraphRunBase wordir.RunFormat
	metadata         *wordresolve.RenderMetadata
	tables           []*tableFrame
	closed           bool
	lastBlocks       int
	emitted          int
}

// NewLayoutSession creates a 1-based inclusive page-window session. end <= 0
// means unbounded, matching layout.PaginateRange.
func NewLayoutSession(start, end int) *LayoutSession {
	return &LayoutSession{window: wordlayout.NewPageWindowSink(start, end)}
}

// SetRenderMetadata seeds the body-independent document metadata used by layout
// and enables effective style resolution for subsequently streamed paragraphs
// and runs. It must be called before body events are consumed.
func (s *LayoutSession) SetRenderMetadata(meta *wordresolve.RenderMetadata) {
	if s == nil || meta == nil {
		return
	}
	s.metadata = meta
	s.doc = meta.Document
	if s.doc.Headers == nil {
		s.doc.Headers = make(map[string]wordir.Story)
	}
	if s.doc.Footers == nil {
		s.doc.Footers = make(map[string]wordir.Story)
	}
	if s.doc.Footnotes == nil {
		s.doc.Footnotes = make(map[int]wordir.Story)
	}
	if s.doc.Endnotes == nil {
		s.doc.Endnotes = make(map[int]wordir.Story)
	}
	if s.doc.Comments == nil {
		s.doc.Comments = make(map[int]wordir.Story)
	}
}

func (s *LayoutSession) Pages() []wordlayout.Page {
	if s == nil || s.window == nil {
		return nil
	}
	return s.window.Pages()
}

func (s *LayoutSession) Document() *wordir.Document {
	if s == nil {
		return nil
	}
	return &s.doc
}

func (s *LayoutSession) HandleEvent(event Event) error {
	if s == nil {
		return nil
	}
	if s.closed {
		return ErrStreamWindowComplete
	}
	if event.Story != "" && event.Story != StoryDocument {
		return nil
	}

	switch event.Type {
	case EventStartDocument, EventStartStory, EventEndStory:
		return nil
	case EventStartParagraph:
		if s.paragraph != nil {
			if err := s.finishParagraph(); err != nil {
				return err
			}
		}
		p := &wordir.Paragraph{StyleID: event.StyleID}
		direct := wordir.ParagraphFormat{}
		if event.ParagraphFormat != nil {
			direct = *event.ParagraphFormat
		}
		if s.metadata != nil {
			p.Format, s.paragraphRunBase = s.metadata.ResolveParagraphFormat(event.StyleID, direct)
		} else {
			p.Format = direct
			s.paragraphRunBase = wordir.RunFormat{}
		}
		s.paragraph = p
		return nil
	case EventText:
		s.ensureParagraph()
		run := wordir.Run{Kind: "run", Text: event.Text, StyleID: event.RunStyleID}
		direct := wordir.RunFormat{}
		if event.RunFormat != nil {
			direct = *event.RunFormat
		}
		if s.metadata != nil {
			run.Format = s.metadata.ResolveRunFormat(s.paragraphRunBase, event.RunStyleID, direct)
		} else {
			run.Format = direct
		}
		s.paragraph.Text += event.Text
		s.paragraph.Runs = append(s.paragraph.Runs, run)
		return nil
	case EventTab:
		s.ensureParagraph()
		run := wordir.Run{Kind: "tab", StyleID: event.RunStyleID}
		direct := wordir.RunFormat{}
		if event.RunFormat != nil {
			direct = *event.RunFormat
		}
		if s.metadata != nil {
			run.Format = s.metadata.ResolveRunFormat(s.paragraphRunBase, event.RunStyleID, direct)
		} else {
			run.Format = direct
		}
		s.paragraph.Text += "\t"
		s.paragraph.Runs = append(s.paragraph.Runs, run)
		return nil
	case EventBreak:
		s.ensureParagraph()
		run := wordir.Run{Kind: "break", BreakType: event.BreakType, StyleID: event.RunStyleID}
		direct := wordir.RunFormat{}
		if event.RunFormat != nil {
			direct = *event.RunFormat
		}
		if s.metadata != nil {
			run.Format = s.metadata.ResolveRunFormat(s.paragraphRunBase, event.RunStyleID, direct)
		} else {
			run.Format = direct
		}
		s.paragraph.Runs = append(s.paragraph.Runs, run)
		return nil
	case EventEndParagraph:
		return s.finishParagraph()
	case EventStartTable:
		if s.paragraph != nil {
			if err := s.finishParagraph(); err != nil {
				return err
			}
		}
		s.tables = append(s.tables, &tableFrame{})
		return nil
	case EventStartTableRow:
		frame, err := s.currentTable()
		if err != nil {
			return err
		}
		if frame.row != nil {
			return fmt.Errorf("word stream: table row started before previous row ended")
		}
		frame.row = &wordir.TableRow{}
		return nil
	case EventStartTableCell:
		frame, err := s.currentTable()
		if err != nil {
			return err
		}
		if frame.row == nil {
			return fmt.Errorf("word stream: table cell started outside a row")
		}
		if frame.cell != nil {
			return fmt.Errorf("word stream: table cell started before previous cell ended")
		}
		frame.cell = &wordir.TableCell{}
		return nil
	case EventEndTableCell:
		return s.finishTableCell()
	case EventEndTableRow:
		return s.finishTableRow()
	case EventEndTable:
		return s.finishTable()
	case EventSection:
		if len(s.tables) > 0 {
			return fmt.Errorf("word stream: section marker encountered inside table")
		}
		if s.paragraph != nil {
			if err := s.finishParagraph(); err != nil {
				return err
			}
		}
		if event.Section == nil {
			return StreamUnsupportedEventError{Type: event.Type}
		}
		section := *event.Section
		s.doc.Sections = append(s.doc.Sections, section)
		s.doc.Blocks = append(s.doc.Blocks, wordir.Block{Kind: "section", Section: &section})
		return s.flush(false)
	case EventEndDocument:
		if s.paragraph != nil {
			if err := s.finishParagraph(); err != nil && !errors.Is(err, ErrStreamWindowComplete) {
				return err
			}
		}
		if len(s.tables) != 0 {
			return fmt.Errorf("word stream: document ended with %d open table(s)", len(s.tables))
		}
		return s.flush(true)
	case EventDrawing, EventFieldInstruction,
		EventFieldChar, EventBookmarkStart,
		EventBookmarkEnd, EventFootnoteReference,
		EventEndnoteReference, EventCommentReference:
		return StreamUnsupportedEventError{Type: event.Type}
	default:
		return StreamUnsupportedEventError{Type: event.Type}
	}
}

func (s *LayoutSession) ensureParagraph() {
	if s.paragraph == nil {
		s.paragraph = &wordir.Paragraph{}
		if s.metadata != nil {
			s.paragraph.Format, s.paragraphRunBase = s.metadata.ResolveParagraphFormat("", wordir.ParagraphFormat{})
		}
	}
}

func (s *LayoutSession) finishParagraph() error {
	if s.paragraph == nil {
		return nil
	}
	p := s.paragraph
	s.paragraph = nil
	s.paragraphRunBase = wordir.RunFormat{}
	s.doc.Paragraphs = append(s.doc.Paragraphs, *p)
	block := wordir.Block{Kind: "paragraph", Paragraph: p}
	if frame := s.topTable(); frame != nil {
		if frame.cell == nil {
			return fmt.Errorf("word stream: paragraph ended inside table but outside a cell")
		}
		frame.cell.Paragraphs = append(frame.cell.Paragraphs, *p)
		frame.cell.Blocks = append(frame.cell.Blocks, block)
		frame.cell.Text += p.Text
		return nil
	}
	s.doc.Blocks = append(s.doc.Blocks, block)
	return s.flush(false)
}

func (s *LayoutSession) currentTable() (*tableFrame, error) {
	frame := s.topTable()
	if frame == nil {
		return nil, fmt.Errorf("word stream: table boundary encountered outside table")
	}
	return frame, nil
}

func (s *LayoutSession) topTable() *tableFrame {
	if len(s.tables) == 0 {
		return nil
	}
	return s.tables[len(s.tables)-1]
}

func (s *LayoutSession) finishTableCell() error {
	if s.paragraph != nil {
		if err := s.finishParagraph(); err != nil {
			return err
		}
	}
	frame, err := s.currentTable()
	if err != nil {
		return err
	}
	if frame.row == nil || frame.cell == nil {
		return fmt.Errorf("word stream: table cell ended without open cell")
	}
	frame.row.Cells = append(frame.row.Cells, *frame.cell)
	frame.cell = nil
	return nil
}

func (s *LayoutSession) finishTableRow() error {
	frame, err := s.currentTable()
	if err != nil {
		return err
	}
	if frame.cell != nil {
		if err := s.finishTableCell(); err != nil {
			return err
		}
	}
	if frame.row == nil {
		return fmt.Errorf("word stream: table row ended without open row")
	}
	frame.table.Rows = append(frame.table.Rows, *frame.row)
	frame.row = nil
	return nil
}

func (s *LayoutSession) finishTable() error {
	if s.paragraph != nil {
		if err := s.finishParagraph(); err != nil {
			return err
		}
	}
	frame, err := s.currentTable()
	if err != nil {
		return err
	}
	if frame.cell != nil {
		if err := s.finishTableCell(); err != nil {
			return err
		}
	}
	if frame.row != nil {
		if err := s.finishTableRow(); err != nil {
			return err
		}
	}
	table := frame.table
	s.tables = s.tables[:len(s.tables)-1]
	block := wordir.Block{Kind: "table", Table: &table}
	if parent := s.topTable(); parent != nil {
		if parent.cell == nil {
			return fmt.Errorf("word stream: nested table ended outside parent cell")
		}
		parent.cell.Tables = append(parent.cell.Tables, table)
		parent.cell.Blocks = append(parent.cell.Blocks, block)
		return nil
	}
	s.doc.Tables = append(s.doc.Tables, table)
	s.doc.Blocks = append(s.doc.Blocks, block)
	return s.flush(false)
}

func (s *LayoutSession) flush(final bool) error {
	if s.window == nil || len(s.doc.Blocks) == 0 {
		return nil
	}
	if !final && len(s.doc.Blocks)-s.lastBlocks < streamPaginationBlockChunk && !lastBlockForcesPageBreak(s.doc.Blocks) {
		return nil
	}
	s.lastBlocks = len(s.doc.Blocks)
	pages := wordlayout.Paginate(&s.doc)
	stable := len(pages)
	if !final && stable > 0 {
		stable--
	}
	for s.emitted < stable {
		page := pages[s.emitted]
		s.emitted++
		if !s.window.Emit(page) {
			s.closed = true
			return ErrStreamWindowComplete
		}
	}
	return nil
}

func lastBlockForcesPageBreak(blocks []wordir.Block) bool {
	if len(blocks) == 0 {
		return false
	}
	block := &blocks[len(blocks)-1]
	if block.Paragraph == nil {
		return false
	}
	for _, run := range block.Paragraph.Runs {
		if run.Kind == "break" && run.BreakType == "page" {
			return true
		}
	}
	return false
}

// Paginate consumes a normalized Word source until EOF or until the requested
// page window is complete.
func Paginate(source Source, start, end int) ([]wordlayout.Page, error) {
	if source == nil {
		return nil, nil
	}
	session := NewLayoutSession(start, end)
	err := source.Walk(session)
	if err != nil && !errors.Is(err, ErrStreamWindowComplete) {
		return nil, err
	}
	return session.Pages(), nil
}
