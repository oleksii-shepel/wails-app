package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestOwnedLegacyTextBoxIsNeverEmittedDetached(t *testing.T) {
	doc := &wordir.Document{}
	owned := &wordir.TextBox{
		LegacyShapeID: 101,
		Blocks:        []wordir.Block{{Kind: "paragraph"}},
	}
	appendLegacyDetachedDrawingTextBoxes(doc, []*wordir.TextBox{owned})

	if len(doc.Paragraphs) != 0 || len(doc.Blocks) != 0 {
		t.Fatalf("owned textbox escaped into document flow: paragraphs=%d blocks=%d", len(doc.Paragraphs), len(doc.Blocks))
	}
}

func TestUnownedLegacyTextBoxKeepsCompatibilityFallback(t *testing.T) {
	doc := &wordir.Document{}
	unowned := &wordir.TextBox{
		Blocks: []wordir.Block{{Kind: "paragraph"}},
	}
	appendLegacyDetachedDrawingTextBoxes(doc, []*wordir.TextBox{unowned})

	if len(doc.Paragraphs) != 1 || len(doc.Blocks) != 1 {
		t.Fatalf("compatibility fallback missing: paragraphs=%d blocks=%d", len(doc.Paragraphs), len(doc.Blocks))
	}
	if got := doc.Paragraphs[0].Runs[0].TextBox; got != unowned {
		t.Fatal("wrong detached compatibility textbox")
	}
}
