package word97

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestApplyDocumentPostProjectionPreservesGridAndCrossReferenceSemantics(t *testing.T) {
	doc := &wordir.Document{
		Sections: []wordir.Section{{DocGrid: &wordir.DocGrid{Type: "lines"}}},
		Paragraphs: []wordir.Paragraph{
			{
				StyleName: "Heading 1",
				Runs:      []wordir.Run{{Text: "Canonical heading"}},
			},
			{
				Runs: []wordir.Run{
					{FieldKind: "REF", Anchor: "missing", Text: "missing"},
					{FieldKind: "REF", Anchor: "kept", Text: "kept"},
					{FieldKind: "STYLEREF", FieldArg: "Heading 1", Text: "stale"},
				},
			},
		},
	}
	ApplyDocumentPostProjection(doc, map[int][]string{4: {"kept"}})

	for i := range doc.Paragraphs {
		if !doc.Paragraphs[i].Format.SnapToGrid {
			t.Fatalf("paragraph %d SnapToGrid=false, want true", i)
		}
	}
	runs := doc.Paragraphs[1].Runs
	if runs[0].Anchor != "" {
		t.Fatalf("missing REF anchor=%q, want cleared", runs[0].Anchor)
	}
	if runs[1].Anchor != "kept" {
		t.Fatalf("known REF anchor=%q, want kept", runs[1].Anchor)
	}
	if runs[2].Text != "Canonical heading" {
		t.Fatalf("STYLEREF text=%q", runs[2].Text)
	}
}

func TestApplyDocumentPostProjectionIgnoresDefaultGrid(t *testing.T) {
	doc := &wordir.Document{
		Sections:   []wordir.Section{{DocGrid: &wordir.DocGrid{Type: "default"}}},
		Paragraphs: []wordir.Paragraph{{}},
	}
	ApplyDocumentPostProjection(doc, nil)
	if doc.Paragraphs[0].Format.SnapToGrid {
		t.Fatal("default grid must not force SnapToGrid")
	}
}
