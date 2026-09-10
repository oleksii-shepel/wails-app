package event

import "testing"

func TestMaterializerCarriesBookmarkSpanAcrossParagraphs(t *testing.T) {
	m := NewMaterializer("bookmark.doc")
	events := []Event{
		{Type: StartDocument},
		{Type: StartParagraph},
		{Type: BookmarkStart, BookmarkName: "SpanBookmark"},
		{Type: Text, Text: "first"},
		{Type: EndParagraph},
		{Type: StartParagraph},
		{Type: Text, Text: "second"},
		{Type: BookmarkEnd, BookmarkName: "SpanBookmark"},
		{Type: EndParagraph},
		{Type: EndDocument},
	}
	for _, event := range events {
		if err := m.Handle(event); err != nil {
			t.Fatal(err)
		}
	}
	doc := m.Document()
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d", len(doc.Paragraphs))
	}
	for i := range doc.Paragraphs {
		if len(doc.Paragraphs[i].Bookmarks) != 1 || doc.Paragraphs[i].Bookmarks[0] != "SpanBookmark" {
			t.Fatalf("paragraph[%d].Bookmarks = %#v", i, doc.Paragraphs[i].Bookmarks)
		}
	}
}

func TestMaterializerProjectsHyperlinkFieldAsHyperlinkRun(t *testing.T) {
	m := NewMaterializer("field.doc")
	for _, event := range []Event{
		{Type: StartDocument},
		{Type: StartParagraph},
		{Type: Text, Text: "click here", FieldKind: "HYPERLINK", FieldInstr: `HYPERLINK "http://example.com"`, Target: "http://example.com"},
		{Type: EndParagraph},
		{Type: EndDocument},
	} {
		if err := m.Handle(event); err != nil {
			t.Fatal(err)
		}
	}
	run := m.Document().Paragraphs[0].Runs[0]
	if run.Kind != "hyperlink" || run.Target != "http://example.com" || run.FieldKind != "HYPERLINK" {
		t.Fatalf("run = %#v", run)
	}
}
