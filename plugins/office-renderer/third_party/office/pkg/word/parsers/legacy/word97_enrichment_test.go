package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

func TestCanonicalWord97EnrichmentUsesCanonicalContext(t *testing.T) {
	shape := &wordir.Shape{Kind: "rect"}
	metadata := word97.Metadata{
		FootnoteRefs: map[int]int{1: 11},
		EndnoteRefs:  map[int]int{2: 22},
		CommentRefs:  map[int]int{3: 33},
	}
	ctx := &word97.Context{Drawings: word97.DrawingProjection{
		Drawings: map[int]word97.Drawing{3: {Shape: shape}},
	}}
	canonical, err := word97.EnrichMaterializedRuns(
		[]word97.Run{{Text: "abc\x08", StartCP: 0, Spec: true}}, metadata, ctx, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	runs := adaptCanonicalWord97Runs(canonical)
	var footnote, endnote, comment, drawing bool
	for _, run := range runs {
		switch run.Kind {
		case "footnoteRef":
			footnote = run.NoteID == 11
		case "endnoteRef":
			endnote = run.NoteID == 22
		case "commentRef":
			comment = run.CommentID == 33
		case "shape":
			drawing = run.Shape == shape
		}
	}
	if !footnote || !endnote || !comment || !drawing {
		t.Fatalf("missing canonical enrichment: footnote=%v endnote=%v comment=%v drawing=%v runs=%#v", footnote, endnote, comment, drawing, runs)
	}
}

func TestCanonicalWord97EnrichmentPrefersResolvedContextRefs(t *testing.T) {
	metadata := word97.Metadata{FootnoteRefs: map[int]int{1: 11}}
	ctx := &word97.Context{FootnoteRefs: map[int]int{1: 22}}
	canonical, err := word97.EnrichMaterializedRuns(
		[]word97.Run{{Text: "abc", StartCP: 0}}, metadata, ctx, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range adaptCanonicalWord97Runs(canonical) {
		if run.Kind == "footnoteRef" {
			if run.NoteID != 22 {
				t.Fatalf("footnote id = %d, want resolved context id 22", run.NoteID)
			}
			return
		}
	}
	t.Fatal("missing footnote reference")
}
