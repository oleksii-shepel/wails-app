package word97

import (
	"bytes"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestAttachOLEPreviewsPrefersVisibleCenteredObject(t *testing.T) {
	preview := &wordir.Image{MIMEType: "image/svg+xml", Data: []byte("<svg/>")}
	doc := &wordir.Document{Blocks: []wordir.Block{
		{Kind: "paragraph", Paragraph: &wordir.Paragraph{Runs: []wordir.Run{{Kind: "olePlaceholder", Text: "Embedded Object"}}}},
		{Kind: "paragraph", Paragraph: &wordir.Paragraph{Runs: []wordir.Run{{Text: "before"}, {Kind: "olePlaceholder", Text: "Embedded Object"}, {Text: "after"}}}},
		{Kind: "paragraph", Paragraph: &wordir.Paragraph{
			Format: wordir.ParagraphFormat{Alignment: "center"},
			Runs:   []wordir.Run{{Kind: "olePlaceholder", Text: "Embedded Object"}},
		}},
	}}
	AttachOLEPreviews(doc, []*wordir.Image{preview})
	if doc.Blocks[0].Paragraph.Runs[0].OLEPreview != nil {
		t.Fatal("structural anchor received preview")
	}
	if doc.Blocks[1].Paragraph.Runs[1].OLEPreview != nil {
		t.Fatal("inline structural anchor received preview")
	}
	got := doc.Blocks[2].Paragraph.Runs[0].OLEPreview
	if got == nil || got.MIMEType != "image/svg+xml" || !bytes.Equal(got.Data, preview.Data) {
		t.Fatalf("visible preview=%#v", got)
	}
	if &got.Data[0] == &preview.Data[0] {
		t.Fatal("preview payload was not cloned")
	}
}

func TestAttachOLEPreviewsUsesOLEMetadataAsSecondarySignal(t *testing.T) {
	doc := &wordir.Document{Paragraphs: []wordir.Paragraph{
		{Runs: []wordir.Run{{Kind: "olePlaceholder"}}},
		{Runs: []wordir.Run{{Kind: "olePlaceholder", OLEProgID: "Excel.Sheet"}}},
	}}
	AttachOLEPreviews(doc, []*wordir.Image{{Data: []byte{1}}})
	if doc.Paragraphs[0].Runs[0].OLEPreview != nil {
		t.Fatal("lower-score anchor received preview")
	}
	if doc.Paragraphs[1].Runs[0].OLEPreview == nil {
		t.Fatal("metadata-bearing anchor did not receive preview")
	}
}
