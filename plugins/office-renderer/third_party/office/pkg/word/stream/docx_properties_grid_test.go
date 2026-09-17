package stream

import (
	wordsax "office/pkg/word/parsers/ooxml/sax"
	"testing"
)

func TestParagraphFormatFromSAXUsesWordSnapToGridDefaultAndOptOut(t *testing.T) {
	got := paragraphFormatFromSAX(wordsax.ParagraphProperties{})
	if !got.SnapToGrid {
		t.Fatal("unset w:snapToGrid should use Word default true")
	}
	f := false
	got = paragraphFormatFromSAX(wordsax.ParagraphProperties{SnapToGrid: &f})
	if got.SnapToGrid {
		t.Fatal("explicit w:snapToGrid=0 should disable snapping")
	}
}

func TestSectionFromSAXNormalizesLinePitchOnlyDocGrid(t *testing.T) {
	pitch := 360
	got := sectionFromSAX(wordsax.SectionProperties{DocGridLinePitch: &pitch})
	if got.DocGrid == nil {
		t.Fatal("DocGrid nil")
	}
	if got.DocGrid.LinePitch != 360 || got.DocGrid.Type != "lines" {
		t.Fatalf("DocGrid = %+v, want linePitch=360 type=lines", got.DocGrid)
	}
}
