package stream

import (
	"testing"

	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func TestDOCXSemanticWrapperPreservesRunAndBlockMetadata(t *testing.T) {
	session := NewLayoutSession(1, 0)
	base, ok := docxHandlerForLayout(session).(*docxLayoutHandler)
	if !ok {
		t.Fatal("layout handler not created")
	}
	h := wrapDOCXLayoutSemantics(base)
	events := []Event{
		{Type: EventStartContentControl, Story: StoryDocument, Raw: wordsax.Event{ControlID: "42", ControlAlias: "Customer", ControlTag: "customer", ControlType: "richText"}},
		{Type: EventStartCustomXML, Story: StoryDocument, Raw: wordsax.Event{MarkupURI: "urn:demo", MarkupElement: "customer"}},
		{Type: EventStartSmartTag, Story: StoryDocument, Raw: wordsax.Event{MarkupURI: "urn:smart", MarkupElement: "person"}},
		{Type: EventStartParagraph, Story: StoryDocument},
		{Type: EventCommentRangeStart, Story: StoryDocument, CommentID: "7", Raw: wordsax.Event{CommentID: "7"}},
		{Type: EventText, Story: StoryDocument, Text: "Alice"},
		{Type: EventCommentRangeEnd, Story: StoryDocument, CommentID: "7", Raw: wordsax.Event{CommentID: "7"}},
		{Type: EventEndParagraph, Story: StoryDocument},
		{Type: EventEndSmartTag, Story: StoryDocument},
		{Type: EventEndCustomXML, Story: StoryDocument},
		{Type: EventEndContentControl, Story: StoryDocument},
		{Type: EventEndDocument, Story: StoryDocument},
	}
	for _, event := range events {
		if err := h.HandleEvent(event); err != nil {
			t.Fatalf("%s: %v", event.Type, err)
		}
	}

	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) != 1 || len(doc.Blocks) == 0 {
		t.Fatalf("document = %#v", doc)
	}
	p := doc.Paragraphs[0]
	if len(p.Runs) != 1 {
		t.Fatalf("runs = %#v", p.Runs)
	}
	run := p.Runs[0]
	if run.ContentControl == nil || run.ContentControl.ID != 42 || run.ContentControl.Tag != "customer" || run.ContentControl.Title != "Customer" {
		t.Fatalf("content control = %#v", run.ContentControl)
	}
	if run.CustomXml == nil || run.CustomXml.URI != "urn:demo" || run.CustomXml.Element != "customer" {
		t.Fatalf("custom xml = %#v", run.CustomXml)
	}
	if run.SmartTag == nil || run.SmartTag.URI != "urn:smart" || run.SmartTag.Element != "person" {
		t.Fatalf("smart tag = %#v", run.SmartTag)
	}
	if len(p.CommentRangeStarts) != 1 || p.CommentRangeStarts[0] != 7 || len(p.CommentRangeEnds) != 1 || p.CommentRangeEnds[0] != 7 {
		t.Fatalf("comment ranges start=%v end=%v", p.CommentRangeStarts, p.CommentRangeEnds)
	}
	block := doc.Blocks[0]
	if block.ContentControl == nil || block.ContentControl.ID != 42 {
		t.Fatalf("block content control = %#v", block.ContentControl)
	}
	if block.CustomXml == nil || block.CustomXml.URI != "urn:demo" {
		t.Fatalf("block custom xml = %#v", block.CustomXml)
	}
}

func TestDOCXSemanticWrapperPreservesOLEPlaceholder(t *testing.T) {
	session := NewLayoutSession(1, 0)
	base := docxHandlerForLayout(session).(*docxLayoutHandler)
	h := wrapDOCXLayoutSemantics(base)
	payload := []byte{1, 2, 3, 4}
	events := []Event{
		{Type: EventStartParagraph, Story: StoryDocument},
		{Type: EventEmbeddedObject, Story: StoryDocument, Raw: wordsax.Event{
			ProgID:             "Excel.Sheet.12",
			RelationshipID:     "rIdOle1",
			PayloadContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			PayloadData:        payload,
		}},
		{Type: EventContentPart, Story: StoryDocument, Raw: wordsax.Event{RelationshipID: "rIdContent"}},
		{Type: EventEndParagraph, Story: StoryDocument},
		{Type: EventEndDocument, Story: StoryDocument},
	}
	for _, event := range events {
		if err := h.HandleEvent(event); err != nil {
			t.Fatalf("%s: %v", event.Type, err)
		}
	}
	p := session.Document().Paragraphs[0]
	if len(p.Runs) != 1 {
		t.Fatalf("runs = %#v", p.Runs)
	}
	run := p.Runs[0]
	if run.Kind != "olePlaceholder" || run.OLERelID != "rIdOle1" || run.OLEProgID != "Excel.Sheet.12" {
		t.Fatalf("OLE run = %#v", run)
	}
	if run.OLEMIME == "" || len(run.OLEData) != len(payload) {
		t.Fatalf("OLE payload mime=%q len=%d", run.OLEMIME, len(run.OLEData))
	}
}
