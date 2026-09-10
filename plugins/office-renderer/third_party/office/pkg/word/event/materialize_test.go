package event

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestMaterializerBuildsSemanticDocument(t *testing.T) {
	m := NewMaterializer("sample.doc")
	story := wordir.Story{Text: "note"}
	img := &wordir.Image{MIMEType: "image/png"}
	events := []Event{
		{Type: StartDocument},
		{Type: StartParagraph},
		{Type: Text, Text: "hello"},
		{Type: Drawing, Image: img},
		{Type: FootnoteReference, NoteID: 3, Story: &story},
		{Type: EndParagraph},
		{Type: EndDocument},
	}
	for _, event := range events {
		if err := m.Handle(event); err != nil {
			t.Fatal(err)
		}
	}
	doc := m.Document()
	if doc == nil || doc.Filename != "sample.doc" {
		t.Fatalf("document = %#v", doc)
	}
	if len(doc.Paragraphs) != 1 || doc.Paragraphs[0].Text != "hello" {
		t.Fatalf("paragraphs = %#v", doc.Paragraphs)
	}
	if len(doc.Paragraphs[0].Runs) != 3 || doc.Paragraphs[0].Runs[1].Image != img {
		t.Fatalf("runs = %#v", doc.Paragraphs[0].Runs)
	}
	if doc.Footnotes[3].Text != "note" {
		t.Fatalf("footnote = %#v", doc.Footnotes)
	}
}

func TestMaterializerBuildsNestedTableState(t *testing.T) {
	m := NewMaterializer("table.doc")
	events := []Event{
		{Type: StartDocument},
		{Type: StartTable},
		{Type: StartTableRow},
		{Type: StartTableCell},
		{Type: StartParagraph},
		{Type: Text, Text: "cell"},
		{Type: EndParagraph},
		{Type: EndTableCell},
		{Type: EndTableRow},
		{Type: EndTable},
		{Type: EndDocument},
	}
	for _, event := range events {
		if err := m.Handle(event); err != nil {
			t.Fatal(err)
		}
	}
	doc := m.Document()
	if len(doc.Tables) != 1 || len(doc.Tables[0].Rows) != 1 || len(doc.Tables[0].Rows[0].Cells) != 1 {
		t.Fatalf("tables = %#v", doc.Tables)
	}
	if doc.Tables[0].Rows[0].Cells[0].Text != "cell" {
		t.Fatalf("cell = %#v", doc.Tables[0].Rows[0].Cells[0])
	}
}
