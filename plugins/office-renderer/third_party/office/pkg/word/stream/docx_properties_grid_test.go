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

func TestSectionFromSAXKeepsLinePitchOnlyDocGridInactive(t *testing.T) {
	// w:docGrid/@w:type defaults to "default", which means no grid: a bare
	// w:linePitch does not make Word snap lines to the pitch. Treating it as an
	// active line grid makes pages denser than the source document.
	pitch := 360
	got := sectionFromSAX(wordsax.SectionProperties{DocGridLinePitch: &pitch})
	if got.DocGrid == nil {
		t.Fatal("DocGrid nil")
	}
	if got.DocGrid.LinePitch != 360 || got.DocGrid.Type != "" {
		t.Fatalf("DocGrid = %+v, want linePitch=360 with no active grid type", got.DocGrid)
	}

	got = sectionFromSAX(wordsax.SectionProperties{DocGridType: "lines", DocGridLinePitch: &pitch})
	if got.DocGrid == nil || got.DocGrid.Type != "lines" {
		t.Fatalf("explicit w:type=lines must stay active: %+v", got.DocGrid)
	}
}
