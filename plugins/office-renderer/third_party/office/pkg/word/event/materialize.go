package event

import (
	"fmt"
	"sort"

	wordir "office/pkg/word/ir"
)

// Materializer is the format-neutral event -> Word IR builder shared by DOC,
// DOCX-facing composition, and compatibility entry points. Parser packages own
// decoding; this type owns only semantic document assembly.
type Materializer struct {
	doc             wordir.Document
	paragraph       *wordir.Paragraph
	tables          []*materializeTableFrame
	activeBookmarks map[string]struct{}
}

type materializeTableFrame struct {
	table wordir.Table
	row   *wordir.TableRow
	cell  *wordir.TableCell
}

func NewMaterializer(name string) *Materializer {
	return &Materializer{
		doc:             wordir.Document{Filename: name},
		activeBookmarks: map[string]struct{}{},
	}
}

func (m *Materializer) Document() *wordir.Document {
	if m == nil {
		return nil
	}
	return &m.doc
}

func runFromEvent(event Event, kind, text string) wordir.Run {
	r := wordir.Run{
		Kind: kind, Text: text,
		Revision: event.Revision, RevisionAuthor: event.RevisionAuthor, RevisionDate: event.RevisionDate,
		PropertyRevision:     cloneRevisionMark(event.RunPropertyRevision),
		DisplayFieldRevision: cloneRevisionMark(event.DisplayFieldRevision),
		FieldKind:            event.FieldKind, FieldInstr: event.FieldInstr, FieldArg: event.FieldArg,
		Target: event.Target, Anchor: event.Anchor, Source: event.RunSource,
	}
	if event.RunFormat != nil {
		r.Format = *event.RunFormat
	}
	return r
}

func cloneRevisionMark(mark *wordir.RevisionMark) *wordir.RevisionMark {
	if mark == nil {
		return nil
	}
	copyMark := *mark
	return &copyMark
}

func (m *Materializer) Handle(event Event) error {
	if m == nil {
		return nil
	}
	switch event.Type {
	case StartDocument:
		if len(event.Headers) > 0 {
			m.doc.Headers = cloneStoryMap(event.Headers)
		}
		if len(event.Footers) > 0 {
			m.doc.Footers = cloneStoryMap(event.Footers)
		}
		if event.FootnoteSeparator != nil {
			m.doc.FootnoteSeparator = *event.FootnoteSeparator
		}
		if event.FootnoteContinuationSep != nil {
			m.doc.FootnoteContinuationSep = *event.FootnoteContinuationSep
		}
		if event.FootnoteContinuationNotice != nil {
			m.doc.FootnoteContinuationNotice = *event.FootnoteContinuationNotice
		}
		if event.EndnoteSeparator != nil {
			m.doc.EndnoteSeparator = *event.EndnoteSeparator
		}
		if event.EndnoteContinuationSep != nil {
			m.doc.EndnoteContinuationSep = *event.EndnoteContinuationSep
		}
		if event.EndnoteContinuationNotice != nil {
			m.doc.EndnoteContinuationNotice = *event.EndnoteContinuationNotice
		}
		return nil
	case StartParagraph:
		if m.paragraph != nil {
			if err := m.finishParagraph(); err != nil {
				return err
			}
		}
		p := &wordir.Paragraph{
			Source:           event.ParagraphSource,
			PropertyRevision: cloneRevisionMark(event.ParagraphPropertyRevision),
			Bookmarks:        m.activeBookmarkNames(),
		}
		if event.ParagraphFormat != nil {
			p.Format = *event.ParagraphFormat
		}
		if p.Format.BinaryFrame != nil {
			frame := *p.Format.BinaryFrame
			p.Frame = &frame
			p.Format.BinaryFrame = nil
		}
		if event.ParagraphNumbering != nil {
			p.Numbering = &wordir.Numbering{NumID: event.ParagraphNumbering.NumID, Level: event.ParagraphNumbering.Level}
		}
		m.paragraph = p
		return nil
	case Text:
		m.ensureParagraph()
		kind := "run"
		if event.FieldKind == "HYPERLINK" && event.Target != "" {
			kind = "hyperlink"
		}
		r := runFromEvent(event, kind, event.Text)
		m.paragraph.Text += event.Text
		m.paragraph.Runs = append(m.paragraph.Runs, r)
		return nil
	case Tab:
		m.ensureParagraph()
		r := runFromEvent(event, "tab", "\t")
		m.paragraph.Text += "\t"
		m.paragraph.Runs = append(m.paragraph.Runs, r)
		return nil
	case Break:
		m.ensureParagraph()
		r := runFromEvent(event, "break", "")
		r.BreakType = event.BreakType
		m.paragraph.Runs = append(m.paragraph.Runs, r)
		return nil
	case Drawing:
		m.ensureParagraph()
		kind := "shape"
		if event.Image != nil {
			kind = "image"
		}
		r := runFromEvent(event, kind, "")
		r.Image = event.Image
		r.Shape = event.Shape
		m.paragraph.Runs = append(m.paragraph.Runs, r)
		return nil
	case EmbeddedObject:
		m.ensureParagraph()
		r := runFromEvent(event, "olePlaceholder", "Embedded Object")
		r.OLEMIME = "application/octet-stream"
		r.OLEType = "Embed"
		if event.EmbeddedRun != nil {
			embedded := *event.EmbeddedRun
			embedded.Revision = event.Revision
			embedded.RevisionAuthor = event.RevisionAuthor
			embedded.RevisionDate = event.RevisionDate
			embedded.PropertyRevision = cloneRevisionMark(event.RunPropertyRevision)
			embedded.DisplayFieldRevision = cloneRevisionMark(event.DisplayFieldRevision)
			r = embedded
		}
		if event.RunSource != nil {
			r.Source = event.RunSource
		}
		if event.RunFormat != nil {
			r.Format = *event.RunFormat
		}
		m.paragraph.Runs = append(m.paragraph.Runs, r)
		return nil
	case BookmarkStart:
		m.ensureParagraph()
		if event.BookmarkName != "" {
			m.paragraph.Bookmarks = appendUnique(m.paragraph.Bookmarks, event.BookmarkName)
			m.activeBookmarks[event.BookmarkName] = struct{}{}
		}
		return nil
	case BookmarkEnd:
		if event.BookmarkName != "" {
			delete(m.activeBookmarks, event.BookmarkName)
		}
		return nil
	case FootnoteReference, EndnoteReference:
		m.ensureParagraph()
		kind := "footnoteRef"
		if event.Type == EndnoteReference {
			kind = "endnoteRef"
		}
		r := runFromEvent(event, kind, "")
		r.NoteID = event.NoteID
		m.paragraph.Runs = append(m.paragraph.Runs, r)
		if event.Story != nil {
			if event.Type == FootnoteReference {
				if m.doc.Footnotes == nil {
					m.doc.Footnotes = map[int]wordir.Story{}
				}
				m.doc.Footnotes[event.NoteID] = *event.Story
			} else {
				if m.doc.Endnotes == nil {
					m.doc.Endnotes = map[int]wordir.Story{}
				}
				m.doc.Endnotes[event.NoteID] = *event.Story
			}
		}
		return nil
	case CommentReference:
		m.ensureParagraph()
		r := runFromEvent(event, "commentRef", "")
		r.CommentID = event.CommentID
		m.paragraph.Runs = append(m.paragraph.Runs, r)
		if event.Story != nil {
			if m.doc.Comments == nil {
				m.doc.Comments = map[int]wordir.Story{}
			}
			m.doc.Comments[event.CommentID] = *event.Story
		}
		return nil
	case EndParagraph:
		return m.finishParagraph()
	case StartTable:
		if m.paragraph != nil {
			if err := m.finishParagraph(); err != nil {
				return err
			}
		}
		m.tables = append(m.tables, &materializeTableFrame{})
		return nil
	case StartTableRow:
		f, err := m.currentTable()
		if err != nil {
			return err
		}
		if f.row != nil {
			return fmt.Errorf("word event materialize: table row started before previous row ended")
		}
		f.row = &wordir.TableRow{}
		return nil
	case StartTableCell:
		f, err := m.currentTable()
		if err != nil {
			return err
		}
		if f.row == nil {
			return fmt.Errorf("word event materialize: cell started outside row")
		}
		if f.cell != nil {
			return fmt.Errorf("word event materialize: cell started before previous cell ended")
		}
		f.cell = &wordir.TableCell{}
		return nil
	case EndTableCell:
		return m.finishCell()
	case EndTableRow:
		return m.finishRow()
	case EndTable:
		return m.finishTable()
	case Section:
		if m.paragraph != nil {
			if err := m.finishParagraph(); err != nil {
				return err
			}
		}
		if len(m.tables) > 0 {
			return fmt.Errorf("word event materialize: section encountered inside table")
		}
		if event.Section != nil {
			sec := *event.Section
			m.doc.Sections = append(m.doc.Sections, sec)
			copySec := sec
			m.doc.Blocks = append(m.doc.Blocks, wordir.Block{Index: len(m.doc.Blocks), Kind: "section", Section: &copySec})
		}
		return nil
	case EndDocument:
		if m.paragraph != nil {
			if err := m.finishParagraph(); err != nil {
				return err
			}
		}
		if len(m.tables) != 0 {
			return fmt.Errorf("word event materialize: document ended with %d open table(s)", len(m.tables))
		}
		return nil
	default:
		return fmt.Errorf("word event materialize: unsupported event %q", event.Type)
	}
}

func (m *Materializer) activeBookmarkNames() []string {
	if m == nil || len(m.activeBookmarks) == 0 {
		return nil
	}
	out := make([]string, 0, len(m.activeBookmarks))
	for name := range m.activeBookmarks {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (m *Materializer) ensureParagraph() {
	if m.paragraph == nil {
		m.paragraph = &wordir.Paragraph{}
	}
}

func (m *Materializer) topTable() *materializeTableFrame {
	if len(m.tables) == 0 {
		return nil
	}
	return m.tables[len(m.tables)-1]
}

func (m *Materializer) currentTable() (*materializeTableFrame, error) {
	f := m.topTable()
	if f == nil {
		return nil, fmt.Errorf("word event materialize: table boundary outside table")
	}
	return f, nil
}

func (m *Materializer) finishParagraph() error {
	if m.paragraph == nil {
		return nil
	}
	p := m.paragraph
	m.paragraph = nil
	p.Index = len(m.doc.Paragraphs)
	m.doc.Paragraphs = append(m.doc.Paragraphs, *p)
	block := wordir.Block{Kind: "paragraph", Paragraph: p}
	if f := m.topTable(); f != nil {
		if f.cell == nil {
			return fmt.Errorf("word event materialize: paragraph inside table outside cell")
		}
		block.Index = len(f.cell.Blocks)
		f.cell.Paragraphs = append(f.cell.Paragraphs, *p)
		f.cell.Blocks = append(f.cell.Blocks, block)
		f.cell.Text += p.Text
		return nil
	}
	block.Index = len(m.doc.Blocks)
	m.doc.Blocks = append(m.doc.Blocks, block)
	return nil
}

func (m *Materializer) finishCell() error {
	if m.paragraph != nil {
		if err := m.finishParagraph(); err != nil {
			return err
		}
	}
	f, err := m.currentTable()
	if err != nil {
		return err
	}
	if f.row == nil || f.cell == nil {
		return fmt.Errorf("word event materialize: cell ended without open cell")
	}
	f.table.Text += f.cell.Text
	f.row.Cells = append(f.row.Cells, *f.cell)
	f.cell = nil
	return nil
}

func (m *Materializer) finishRow() error {
	f, err := m.currentTable()
	if err != nil {
		return err
	}
	if f.cell != nil {
		if err := m.finishCell(); err != nil {
			return err
		}
	}
	if f.row == nil {
		return fmt.Errorf("word event materialize: row ended without open row")
	}
	f.table.Rows = append(f.table.Rows, *f.row)
	f.row = nil
	return nil
}

func (m *Materializer) finishTable() error {
	if m.paragraph != nil {
		if err := m.finishParagraph(); err != nil {
			return err
		}
	}
	f, err := m.currentTable()
	if err != nil {
		return err
	}
	if f.cell != nil {
		if err := m.finishCell(); err != nil {
			return err
		}
	}
	if f.row != nil {
		if err := m.finishRow(); err != nil {
			return err
		}
	}
	table := f.table
	table.Index = len(m.doc.Tables)
	m.tables = m.tables[:len(m.tables)-1]
	block := wordir.Block{Kind: "table", Table: &table}
	if parent := m.topTable(); parent != nil {
		if parent.cell == nil {
			return fmt.Errorf("word event materialize: nested table ended outside parent cell")
		}
		block.Index = len(parent.cell.Blocks)
		parent.cell.Tables = append(parent.cell.Tables, table)
		parent.cell.Blocks = append(parent.cell.Blocks, block)
		return nil
	}
	block.Index = len(m.doc.Blocks)
	m.doc.Tables = append(m.doc.Tables, table)
	m.doc.Blocks = append(m.doc.Blocks, block)
	return nil
}

func cloneStoryMap(src map[string]wordir.Story) map[string]wordir.Story {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]wordir.Story, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func appendUnique(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	return append(values, value)
}
