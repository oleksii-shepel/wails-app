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
	// Raw property XML captured by the SAX walker. Conditional table-style
	// formatting depends on a cell's row/column position, so the properties are
	// resolved when the table closes rather than as each cell arrives.
	rawTable string
	rawGrid  string
	rawRows  []string
	rawCells [][]string
	// pendingRow and pendingCell hold the property block for the row or cell
	// that is currently open, before it moves into the slices above.
	pendingRow  string
	pendingCell string
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
	end              int
	closed           bool
	lastBlocks       int
	emitted          int
}

// NewLayoutSession creates a 1-based inclusive page-window session. end <= 0
// means unbounded, matching layout.PaginateRange.
func NewLayoutSession(start, end int) *LayoutSession {
	return &LayoutSession{window: wordlayout.NewPageWindowSink(start, end), end: end}
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
		if event.ParagraphFrame != nil {
			frame := *event.ParagraphFrame
			p.Frame = &frame
		}
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
		markDirect := wordir.RunFormat{}
		if event.RunFormat != nil {
			markDirect = *event.RunFormat
		}
		if s.metadata != nil {
			mark := s.metadata.ResolveRunFormat(s.paragraphRunBase, event.RunStyleID, markDirect)
			p.DefaultRunFormat = &mark
		} else if event.RunFormat != nil {
			p.DefaultRunFormat = &markDirect
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
	case EventTableProperty:
		return s.recordTableProperty(event)
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

// tableStyles builds the style resolver for the document being streamed.
func (s *LayoutSession) tableStyles() streamTableStyles {
	return newStreamTableStyles(&s.doc, s.metadata)
}

// resolveStoryFormatting applies effective style resolution to a story parsed
// outside the body stream (footnotes, endnotes, comments, headers). Note styles
// carry their own line spacing and fonts, so a story that keeps only direct
// formatting inherits the document default instead: Word's EndnoteText, for
// example, is single-spaced while the body is not, which changes note line
// heights and therefore page fill.
func (s *LayoutSession) resolveStoryFormatting(story wordir.Story) wordir.Story {
	if s.metadata == nil {
		return story
	}
	var resolveBlocks func(blocks []wordir.Block)
	resolve := func(p *wordir.Paragraph) {
		if p == nil {
			return
		}
		format, runBase := s.metadata.ResolveParagraphFormat(p.StyleID, p.Format)
		p.Format = format
		for i := range p.Runs {
			run := &p.Runs[i]
			run.Format = s.metadata.ResolveRunFormat(runBase, run.StyleID, run.Format)
			if run.TextBox != nil {
				resolveBlocks(run.TextBox.Blocks)
			}
			if run.Shape != nil && run.Shape.TextBox != nil {
				resolveBlocks(run.Shape.TextBox.Blocks)
			}
		}
	}
	resolveBlocks = func(blocks []wordir.Block) {
		for i := range blocks {
			block := &blocks[i]
			if block.Paragraph != nil {
				resolve(block.Paragraph)
			}
			if block.Table != nil {
				for r := range block.Table.Rows {
					for c := range block.Table.Rows[r].Cells {
						cell := &block.Table.Rows[r].Cells[c]
						for pi := range cell.Paragraphs {
							resolve(&cell.Paragraphs[pi])
						}
						resolveBlocks(cell.Blocks)
					}
				}
			}
		}
	}
	for i := range story.Paragraphs {
		resolve(&story.Paragraphs[i])
	}
	resolveBlocks(story.Blocks)
	for i := range story.Tables {
		for r := range story.Tables[i].Rows {
			for c := range story.Tables[i].Rows[r].Cells {
				cell := &story.Tables[i].Rows[r].Cells[c]
				for pi := range cell.Paragraphs {
					resolve(&cell.Paragraphs[pi])
				}
				resolveBlocks(cell.Blocks)
			}
		}
	}
	return story
}

// recordTableProperty stores the raw w:tblPr / w:tblGrid / w:trPr / w:tcPr block
// the SAX walker captured for the innermost open table element.
func (s *LayoutSession) recordTableProperty(event Event) error {
	frame, err := s.currentTable()
	if err != nil {
		return err
	}
	raw := event.RawXML
	if raw == "" {
		return nil
	}
	switch event.TablePropertyScope {
	case "table":
		frame.rawTable = raw
	case "grid":
		frame.rawGrid = raw
	case "row":
		frame.pendingRow = raw
	case "cell":
		frame.pendingCell = raw
	}
	return nil
}

func (s *LayoutSession) ensureParagraph() {
	if s.paragraph == nil {
		s.paragraph = &wordir.Paragraph{}
		if s.metadata != nil {
			s.paragraph.Format, s.paragraphRunBase = s.metadata.ResolveParagraphFormat("", wordir.ParagraphFormat{})
			mark := s.metadata.ResolveRunFormat(s.paragraphRunBase, "", wordir.RunFormat{})
			s.paragraph.DefaultRunFormat = &mark
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
	// The semantic IR treats Paragraph.Index as the document-global paragraph
	// ordinal; the renderer keys per-paragraph state (list counters, note
	// numbers) on it. Assign it here so streamed documents match the event
	// materializer instead of leaving every paragraph at the zero value.
	p.Index = len(s.doc.Paragraphs)
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
	rowIndex := len(frame.table.Rows)
	for len(frame.rawCells) <= rowIndex {
		frame.rawCells = append(frame.rawCells, nil)
	}
	frame.rawCells[rowIndex] = append(frame.rawCells[rowIndex], frame.pendingCell)
	frame.pendingCell = ""
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
	frame.rawRows = append(frame.rawRows, frame.pendingRow)
	frame.pendingRow = ""
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
	s.tableStyles().apply(&table, frame.rawTable, frame.rawGrid, frame.rawRows, frame.rawCells)
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
	// An unbounded request needs no provisional pages: emit the definitive
	// pagination once the SAX walk reaches EOF. Re-paginating after each hard
	// break is quadratic for large documents with recorded Word page markers.
	if !final && s.end <= 0 {
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
