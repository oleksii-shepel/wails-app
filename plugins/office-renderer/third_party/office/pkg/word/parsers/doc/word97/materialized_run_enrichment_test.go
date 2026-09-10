package word97

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestEnrichMaterializedRunsOwnsReferencesAndDrawings(t *testing.T) {
	image := &wordir.Image{}
	runs := []Run{
		{Text: "a\x08b", StartCP: 10, Spec: true},
		{Text: "x\x02y", StartCP: 20},
	}
	context := &Context{
		Drawings: DrawingProjection{
			Drawings: map[int]Drawing{11: {Image: image}},
		},
		FootnoteRefs: map[int]int{21: 7},
	}
	got, err := EnrichMaterializedRuns(runs, Metadata{}, context, nil)
	if err != nil {
		t.Fatal(err)
	}
	var imageRun, noteRun bool
	for _, run := range got {
		if run.Kind == "image" && run.Image == image && run.StartCP == 11 {
			imageRun = true
		}
		if run.Kind == "footnoteRef" && run.NoteID == 7 && run.StartCP == 21 {
			noteRun = true
		}
	}
	if !imageRun || !noteRun {
		t.Fatalf("enriched runs = %#v", got)
	}
}
