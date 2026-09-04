package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestAttachLegacyShapeTextBoxesDoesNotRequirePlaceholder(t *testing.T) {
	shape := &wordir.Shape{
		LegacyShapeID: 77,
		Kind:          "rect",
	}
	story := &wordir.TextBox{
		LegacyShapeID: 77,
		Blocks:        []wordir.Block{{Kind: "paragraph"}},
	}

	remaining := attachLegacyShapeTextBoxes(shape, []*wordir.TextBox{story})
	if len(remaining) != 0 {
		t.Fatalf("remaining=%d, want 0", len(remaining))
	}
	if shape.TextBox != story {
		t.Fatal("authoritative textbox story was not attached directly to shape")
	}
}

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

func TestShapeRotationOwnsAttachedText(t *testing.T) {
	shape := &wordir.Shape{
		LegacyShapeID: 55,
		Kind:          "rect",
		RotationDeg:   45,
	}
	story := &wordir.TextBox{
		LegacyShapeID: 55,
		Blocks:        []wordir.Block{{Kind: "paragraph"}},
	}
	remaining := attachLegacyShapeTextBoxes(shape, []*wordir.TextBox{story})
	if len(remaining) != 0 || shape.TextBox != story {
		t.Fatal("story not attached to rotated owning shape")
	}
	if shape.RotationDeg != 45 {
		t.Fatalf("shape rotation changed to %v", shape.RotationDeg)
	}
}
