package stream

import (
	"strings"
	"testing"

	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func TestDOCXLayoutBridgePreservesInlineSemantics(t *testing.T) {
	session := NewLayoutSession(1, 0)
	h := docxHandlerForLayout(session)
	events := []Event{
		{Type: EventStartParagraph, Story: StoryDocument},
		{Type: EventStartHyperlink, Story: StoryDocument, Raw: wordsax.Event{HyperlinkTarget: "https://example.com", RelationshipID: "rId5"}},
		{Type: EventText, Story: StoryDocument, Text: "link"},
		{Type: EventEndHyperlink, Story: StoryDocument},
		{Type: EventStartRevision, Story: StoryDocument, Raw: wordsax.Event{RevisionKind: "ins", RevisionID: "12", Author: "Ada", Date: "2026-09-05T08:00:00Z"}},
		{Type: EventText, Story: StoryDocument, Text: "new"},
		{Type: EventEndRevision, Story: StoryDocument},
		{Type: EventSymbol, Story: StoryDocument, Raw: wordsax.Event{Text: "★"}},
		{Type: EventRuby, Story: StoryDocument, Raw: wordsax.Event{RubyBaseText: "東京", RubyText: "とうきょう"}},
		{Type: EventMath, Story: StoryDocument, Raw: wordsax.Event{MathText: "x+y", RawXML: `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"><m:r><m:t>x+y</m:t></m:r></m:oMath>`}},
		{Type: EventEndParagraph, Story: StoryDocument},
		{Type: EventEndDocument, Story: StoryDocument},
	}
	for _, event := range events {
		if err := h.HandleEvent(event); err != nil {
			t.Fatalf("%s: %v", event.Type, err)
		}
	}

	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %#v", doc)
	}
	runs := doc.Paragraphs[0].Runs
	if len(runs) != 5 {
		t.Fatalf("runs = %#v, want 5", runs)
	}
	if runs[0].Target != "https://example.com" || runs[0].RelID != "rId5" {
		t.Fatalf("hyperlink run = %#v", runs[0])
	}
	if runs[1].Revision != "inserted" || runs[1].RevisionAuthor != "Ada" || runs[1].MoveID != 0 {
		t.Fatalf("revision run = %#v", runs[1])
	}
	if runs[2].Text != "★" {
		t.Fatalf("symbol run = %#v", runs[2])
	}
	if runs[3].Kind != "ruby" || runs[3].Ruby == nil || runs[3].Ruby.Base != "東京" || runs[3].Ruby.Text != "とうきょう" {
		t.Fatalf("ruby run = %#v", runs[3])
	}
	if runs[4].Kind != "math" || !strings.Contains(runs[4].MathML, "<math") || !strings.Contains(runs[4].MathML, "x+y") {
		t.Fatalf("math run = %#v", runs[4])
	}
	if got := doc.Paragraphs[0].Text; got != "linknew★東京x+y" {
		t.Fatalf("paragraph text = %q", got)
	}
}
