package layout

import (
	"math"
	"testing"

	"office/pkg/fontmetrics"
	ir "office/pkg/presentation/ir"
)

func TestParagraphLayoutUsesSharedFontMetricsCatalogWhenFaceExists(t *testing.T) {
	face := &fontmetrics.Face{
		UnitsPerEm:  1000,
		RuneToGlyph: map[rune]uint16{'A': 1, 'B': 2},
		Advances:    []uint16{0, 600, 500},
	}
	cat := &fontmetrics.Catalog{Faces: []fontmetrics.NamedFace{{Family: "Embedded Demo", Weight: 400, Face: face}}}
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "AB", FontFamily: "Embedded Demo", FontSize: 2000}}}
	got := layoutParagraphLinesWithMarkerAndCatalog(p, ParagraphMetrics{AvailableWidthPt: 100, FirstLineAvailableWidthPt: 100, LineHeightScale: 1}, 1, "", 0, "", 0, cat)
	if len(got.Lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(got.Lines))
	}
	// 600/1000*20 + 500/1000*20 = 22pt.
	if math.Abs(got.Lines[0].WidthPt-22) > .001 {
		t.Fatalf("catalog line width = %.3f, want 22", got.Lines[0].WidthPt)
	}
}

func TestParagraphLayoutRetainsHeuristicFallbackWhenFamilyUnavailable(t *testing.T) {
	cat := &fontmetrics.Catalog{Faces: []fontmetrics.NamedFace{{Family: "Other", Weight: 400, Face: &fontmetrics.Face{UnitsPerEm: 1000}}}}
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "AB", FontFamily: "Arial", FontSize: 2000}}}
	withCatalog := layoutParagraphLinesWithMarkerAndCatalog(p, ParagraphMetrics{AvailableWidthPt: 100, FirstLineAvailableWidthPt: 100, LineHeightScale: 1}, 1, "", 0, "", 0, cat)
	fallback := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 100, FirstLineAvailableWidthPt: 100, LineHeightScale: 1}, 1, "", 0)
	if len(withCatalog.Lines) != 1 || len(fallback.Lines) != 1 || math.Abs(withCatalog.Lines[0].WidthPt-fallback.Lines[0].WidthPt) > .001 {
		t.Fatalf("missing-family fallback drift: catalog=%#v fallback=%#v", withCatalog.Lines, fallback.Lines)
	}
}
