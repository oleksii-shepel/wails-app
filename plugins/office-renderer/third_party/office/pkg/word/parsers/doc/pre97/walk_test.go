package pre97

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestWalkEmitsTableAndReferenceEvents(t *testing.T) {
	cursor := &sliceRunCursor{runs: []Run{
		{Text: "cell\a", ParagraphFormat: wordir.ParagraphFormat{InTable: true, IsTableCellTrailingParagraph: true}, TableDepth: 1},
		{Kind: "footnoteRef", NoteID: 2, Text: "\x02"},
	}}
	context := &Context{Footnotes: map[int]wordir.Story{2: {Text: "note"}}}
	var seen []EventType
	var noteText string
	err := Walk(cursor, context, func(event Event) error {
		seen = append(seen, event.Type)
		if event.Type == EventFootnoteReference && event.Story != nil {
			noteText = event.Story.Text
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if noteText != "note" {
		t.Fatalf("footnote story = %q, want note", noteText)
	}
	want := map[EventType]bool{
		EventStartTable: true, EventStartTableRow: true, EventStartTableCell: true,
		EventEndTableCell: true, EventEndTableRow: true, EventEndTable: true,
		EventFootnoteReference: true,
	}
	for _, event := range seen {
		delete(want, event)
	}
	if len(want) != 0 {
		t.Fatalf("missing events: %v; seen %v", want, seen)
	}
}

func TestWalkEmitsSectionsByPhysicalBoundary(t *testing.T) {
	cursor := &sliceRunCursor{runs: []Run{{Text: "one\r", StartCP: 0}, {Text: "two\r", StartCP: 10}}}
	context := &Context{Sections: []wordir.Section{{Source: &wordir.SectionSource{StartCP: 0, EndCP: 10}}}}
	sectionBeforeSecond := false
	seenSecond := false
	err := Walk(cursor, context, func(event Event) error {
		if event.Type == EventSection && !seenSecond {
			sectionBeforeSecond = true
		}
		if event.Type == EventText && event.Text == "two" {
			seenSecond = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if !sectionBeforeSecond {
		t.Fatal("section boundary was not emitted before second run")
	}
}
