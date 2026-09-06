package stream

import (
	"testing"

	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func TestDOCXSemanticWrapperPreservesDirectionAnnotationAndAltChunk(t *testing.T) {
	session := NewLayoutSession(1, 0)
	base := docxHandlerForLayout(session).(*docxLayoutHandler)
	h := wrapDOCXLayoutSemantics(base)
	payload := []byte("<html><body>chunk</body></html>")

	events := []Event{
		{Type: EventStartParagraph, Story: StoryDocument},
		{Type: EventStartDirection, Story: StoryDocument, Raw: wordsax.Event{Direction: "rtl"}},
		{Type: EventText, Story: StoryDocument, Text: "مرحبا"},
		{Type: EventAnnotationReference, Story: StoryDocument},
		{Type: EventAutoText, Story: StoryDocument, Raw: wordsax.Event{AutoTextKind: "dayShort"}},
		{Type: EventEndDirection, Story: StoryDocument, Raw: wordsax.Event{Direction: "rtl"}},
		{Type: EventEndParagraph, Story: StoryDocument},
		{Type: EventAltChunk, Story: StoryDocument, Raw: wordsax.Event{
			RelationshipID:     "rAlt1",
			PayloadContentType: "text/html",
			PayloadData:        payload,
		}},
		{Type: EventEndDocument, Story: StoryDocument},
	}
	for _, event := range events {
		if err := h.HandleEvent(event); err != nil {
			t.Fatalf("%s: %v", event.Type, err)
		}
	}

	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) != 1 {
		t.Fatalf("document = %#v", doc)
	}
	p := doc.Paragraphs[0]
	if len(p.Runs) != 2 {
		t.Fatalf("runs = %#v, want text + annotationRef", p.Runs)
	}
	if p.Runs[0].Text != "مرحبا" || p.Runs[0].Format.Dir != "rtl" {
		t.Fatalf("directed run = %#v", p.Runs[0])
	}
	if p.Runs[1].Kind != "annotationRef" || p.Runs[1].Format.Dir != "rtl" {
		t.Fatalf("annotation run = %#v", p.Runs[1])
	}
	if len(doc.Blocks) != 2 || doc.Blocks[1].Kind != "altChunk" || doc.Blocks[1].AltChunk == nil {
		t.Fatalf("blocks = %#v", doc.Blocks)
	}
	if doc.Blocks[1].AltChunk.ContentType != "text/html" || string(doc.Blocks[1].AltChunk.Data) != string(payload) {
		t.Fatalf("altChunk = %#v", doc.Blocks[1].AltChunk)
	}
}
