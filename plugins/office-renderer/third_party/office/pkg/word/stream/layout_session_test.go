package stream

import (
	"errors"
	"testing"

	wordir "office/pkg/word/ir"
)

type testStreamSource struct {
	events []Event
	seen   int
}

func (s *testStreamSource) Walk(handler Handler) error {
	for _, event := range s.events {
		s.seen++
		if err := handler.HandleEvent(event); err != nil {
			return err
		}
	}
	return nil
}

func TestLayoutSessionBuildsParagraphSubset(t *testing.T) {
	s := NewLayoutSession(1, 0)
	events := []Event{
		{Type: EventStartDocument, Story: StoryDocument},
		{Type: EventStartParagraph, Story: StoryDocument, StyleID: "Body"},
		{Type: EventText, Story: StoryDocument, Text: "Hello"},
		{Type: EventTab, Story: StoryDocument},
		{Type: EventText, Story: StoryDocument, Text: "world"},
		{Type: EventBreak, Story: StoryDocument, BreakType: "line"},
		{Type: EventEndParagraph, Story: StoryDocument},
		{Type: EventEndDocument, Story: StoryDocument},
	}
	for _, event := range events {
		if err := s.HandleEvent(event); err != nil {
			t.Fatalf("HandleEvent(%s): %v", event.Type, err)
		}
	}

	doc := s.Document()
	if doc == nil || len(doc.Blocks) != 1 || doc.Blocks[0].Paragraph == nil {
		t.Fatalf("document blocks = %#v, want one paragraph", doc)
	}
	p := doc.Blocks[0].Paragraph
	if p.StyleID != "Body" {
		t.Fatalf("style = %q, want Body", p.StyleID)
	}
	if p.Text != "Hello\tworld" {
		t.Fatalf("text = %q, want Hello\\tworld", p.Text)
	}
	if len(p.Runs) != 4 {
		t.Fatalf("run count = %d, want 4", len(p.Runs))
	}
	if p.Runs[1].Kind != "tab" {
		t.Fatalf("second run kind = %q, want tab", p.Runs[1].Kind)
	}
	if p.Runs[3].Kind != "break" || p.Runs[3].BreakType != "line" {
		t.Fatalf("last run = %#v, want line break", p.Runs[3])
	}
}

func TestLayoutSessionBuildsTableCellsWithoutLeakingBlocks(t *testing.T) {
	s := NewLayoutSession(1, 0)
	events := []Event{
		{Type: EventStartDocument, Story: StoryDocument},
		{Type: EventStartTable, Story: StoryDocument},
		{Type: EventStartTableRow, Story: StoryDocument},
		{Type: EventStartTableCell, Story: StoryDocument},
		{Type: EventStartParagraph, Story: StoryDocument},
		{Type: EventText, Story: StoryDocument, Text: "A"},
		{Type: EventEndParagraph, Story: StoryDocument},
		{Type: EventEndTableCell, Story: StoryDocument},
		{Type: EventStartTableCell, Story: StoryDocument},
		{Type: EventStartParagraph, Story: StoryDocument},
		{Type: EventText, Story: StoryDocument, Text: "B"},
		{Type: EventEndParagraph, Story: StoryDocument},
		{Type: EventEndTableCell, Story: StoryDocument},
		{Type: EventEndTableRow, Story: StoryDocument},
		{Type: EventEndTable, Story: StoryDocument},
		{Type: EventEndDocument, Story: StoryDocument},
	}
	for _, event := range events {
		if err := s.HandleEvent(event); err != nil {
			t.Fatalf("HandleEvent(%s): %v", event.Type, err)
		}
	}
	doc := s.Document()
	if len(doc.Blocks) != 1 || doc.Blocks[0].Table == nil {
		t.Fatalf("document blocks = %#v, want one table block", doc.Blocks)
	}
	if len(doc.Tables) != 1 || len(doc.Tables[0].Rows) != 1 || len(doc.Tables[0].Rows[0].Cells) != 2 {
		t.Fatalf("table = %#v, want one row with two cells", doc.Tables)
	}
	if got := doc.Tables[0].Rows[0].Cells[0].Text; got != "A" {
		t.Fatalf("first cell text = %q, want A", got)
	}
	if got := doc.Tables[0].Rows[0].Cells[1].Text; got != "B" {
		t.Fatalf("second cell text = %q, want B", got)
	}
}

func TestLayoutSessionBuildsNestedTableInParentCell(t *testing.T) {
	s := NewLayoutSession(1, 0)
	events := []Event{
		{Type: EventStartTable, Story: StoryDocument},
		{Type: EventStartTableRow, Story: StoryDocument},
		{Type: EventStartTableCell, Story: StoryDocument},
		{Type: EventStartTable, Story: StoryDocument},
		{Type: EventStartTableRow, Story: StoryDocument},
		{Type: EventStartTableCell, Story: StoryDocument},
		{Type: EventStartParagraph, Story: StoryDocument},
		{Type: EventText, Story: StoryDocument, Text: "nested"},
		{Type: EventEndParagraph, Story: StoryDocument},
		{Type: EventEndTableCell, Story: StoryDocument},
		{Type: EventEndTableRow, Story: StoryDocument},
		{Type: EventEndTable, Story: StoryDocument},
		{Type: EventEndTableCell, Story: StoryDocument},
		{Type: EventEndTableRow, Story: StoryDocument},
		{Type: EventEndTable, Story: StoryDocument},
		{Type: EventEndDocument, Story: StoryDocument},
	}
	for _, event := range events {
		if err := s.HandleEvent(event); err != nil {
			t.Fatalf("HandleEvent(%s): %v", event.Type, err)
		}
	}
	cell := s.Document().Tables[0].Rows[0].Cells[0]
	if len(cell.Tables) != 1 || len(cell.Blocks) != 1 || cell.Blocks[0].Table == nil {
		t.Fatalf("parent cell = %#v, want one nested table block", cell)
	}
}

func TestLayoutSessionPromotesFormattingAndSection(t *testing.T) {
	bold := true
	pf := wordir.ParagraphFormat{KeepNext: true}
	rf := wordir.RunFormat{Bold: &bold}
	section := wordir.Section{Type: "nextPage"}
	s := NewLayoutSession(1, 0)
	events := []Event{
		{Type: EventStartParagraph, Story: StoryDocument, StyleID: "Heading1", ParagraphFormat: &pf},
		{Type: EventText, Story: StoryDocument, Text: "Title", RunStyleID: "Strong", RunFormat: &rf},
		{Type: EventEndParagraph, Story: StoryDocument},
		{Type: EventSection, Story: StoryDocument, Section: &section},
		{Type: EventEndDocument, Story: StoryDocument},
	}
	for _, event := range events {
		if err := s.HandleEvent(event); err != nil {
			t.Fatalf("HandleEvent(%s): %v", event.Type, err)
		}
	}
	doc := s.Document()
	p := doc.Blocks[0].Paragraph
	if p == nil || !p.Format.KeepNext || p.StyleID != "Heading1" {
		t.Fatalf("paragraph = %#v, want promoted paragraph format/style", p)
	}
	if len(p.Runs) != 1 || p.Runs[0].StyleID != "Strong" || p.Runs[0].Format.Bold == nil || !*p.Runs[0].Format.Bold {
		t.Fatalf("run = %#v, want promoted run style/bold", p.Runs)
	}
	if len(doc.Sections) != 1 || len(doc.Blocks) != 2 || doc.Blocks[1].Section == nil || doc.Blocks[1].Section.Type != "nextPage" {
		t.Fatalf("sections/blocks = %#v / %#v, want nextPage section marker", doc.Sections, doc.Blocks)
	}
}

func TestPaginateStopsSourceWhenWindowCompletes(t *testing.T) {
	events := []Event{{Type: EventStartDocument, Story: StoryDocument}}
	for i := 0; i < 5; i++ {
		events = append(events,
			Event{Type: EventStartParagraph, Story: StoryDocument},
			Event{Type: EventText, Story: StoryDocument, Text: "page"},
			Event{Type: EventBreak, Story: StoryDocument, BreakType: "page"},
			Event{Type: EventEndParagraph, Story: StoryDocument},
		)
	}
	events = append(events,
		Event{Type: EventDrawing, Story: StoryDocument},
		Event{Type: EventEndDocument, Story: StoryDocument},
	)

	source := &testStreamSource{events: events}
	pages, err := Paginate(source, 2, 2)
	if err != nil {
		t.Fatalf("Paginate: %v", err)
	}
	if len(pages) != 1 || pages[0].Number != 2 {
		t.Fatalf("pages = %#v, want physical page 2 only", pages)
	}
	if source.seen >= len(events) {
		t.Fatalf("source consumed all %d events; want early stop", source.seen)
	}
}

func TestLayoutSessionRejectsUnsupportedRichEvent(t *testing.T) {
	s := NewLayoutSession(1, 1)
	err := s.HandleEvent(Event{Type: EventDrawing, Story: StoryDocument})
	var unsupported StreamUnsupportedEventError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %v, want StreamUnsupportedEventError", err)
	}
	if unsupported.Type != EventDrawing {
		t.Fatalf("unsupported type = %q, want drawing", unsupported.Type)
	}
}
