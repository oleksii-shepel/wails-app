package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

func TestApplyCanonicalWord97EnrichmentUsesCanonicalContext(t *testing.T) {
	shape := &wordir.Shape{Kind: "rect"}
	textBox := &wordir.TextBox{LegacyShapeID: 7}
	core := &legacyDocStreamCore{
		FootnoteRefs: map[int]int{1: 11},
		EndnoteRefs:  map[int]int{2: 22},
		CommentRefs:  map[int]int{3: 33},
		Footnotes:    map[int]wordir.Story{11: {Text: "footnote"}},
		Endnotes:     map[int]wordir.Story{22: {Text: "endnote"}},
		Comments:     map[int]wordir.Story{33: {Text: "comment"}},
		BookmarkStarts: map[int][]string{0: {"mark"}},
		BookmarkEnds:   map[int][]string{4: {"mark"}},
		Drawings:       map[int]word97.Drawing{3: {Shape: shape}},
		TextBoxes:      []*wordir.TextBox{textBox},
	}

	runs, enrichment := applyCanonicalWord97Enrichment([]docRun{{Text: "abc\x08", StartCP: 0, Spec: true}}, core)
	if len(runs) < 4 {
		t.Fatalf("expected injected references and drawing, got %#v", runs)
	}
	var footnote, endnote, comment, drawing bool
	for _, run := range runs {
		switch run.Kind {
		case "footnoteRef":
			footnote = run.NoteID == 11
		case "endnoteRef":
			endnote = run.NoteID == 22
		case "commentRef":
			comment = run.CommentID == 33
		}
		if run.Shape == shape {
			drawing = true
		}
	}
	if !footnote || !endnote || !comment || !drawing {
		t.Fatalf("missing canonical enrichment: footnote=%v endnote=%v comment=%v drawing=%v runs=%#v", footnote, endnote, comment, drawing, runs)
	}
	if enrichment.Footnotes[11].Text != "footnote" || enrichment.Endnotes[22].Text != "endnote" || enrichment.Comments[33].Text != "comment" {
		t.Fatalf("story projection mismatch: %#v", enrichment)
	}
	if enrichment.BookmarkStarts[0][0] != "mark" || enrichment.BookmarkEnds[4][0] != "mark" {
		t.Fatalf("bookmark projection mismatch: %#v", enrichment)
	}
	if len(enrichment.TextBoxes) != 1 || enrichment.TextBoxes[0] != textBox {
		t.Fatalf("textbox projection mismatch: %#v", enrichment.TextBoxes)
	}
}
