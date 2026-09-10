package word97

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestStyleCatalogAppliesNameToFlatAndBlockParagraphViews(t *testing.T) {
	index := 1
	style := wordir.Style{ID: "Heading1", Name: "Heading 1", StylePath: []string{"Heading1", "Normal"}}
	c := &StyleCatalog{
		ParagraphStyles:  map[string]wordir.Style{"Heading1": style},
		ParagraphByIndex: map[int]wordir.Style{index: style},
	}
	flat := wordir.Paragraph{Format: wordir.ParagraphFormat{BinaryStyleIndex: &index}}
	blockPara := wordir.Paragraph{Format: wordir.ParagraphFormat{BinaryStyleIndex: &index}}
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{flat},
		Blocks:     []wordir.Block{{Kind: "paragraph", Paragraph: &blockPara}},
	}
	c.ApplyToDocument(doc)
	for label, p := range map[string]*wordir.Paragraph{"flat": &doc.Paragraphs[0], "block": doc.Blocks[0].Paragraph} {
		if p.StyleID != "Heading1" || p.StyleName != "Heading 1" {
			t.Fatalf("%s style = %q/%q", label, p.StyleID, p.StyleName)
		}
	}
}
