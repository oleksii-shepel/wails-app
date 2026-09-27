package layout

import (
	"testing"

	sharedfontmetrics "office/pkg/fontmetrics"
	resolve "office/pkg/word/ir"
)

func TestParagraphFlowUsesEmbeddedFaceMetricsForWrapDecisions(t *testing.T) {
	face := &sharedfontmetrics.Face{
		UnitsPerEm:  1000,
		RuneToGlyph: map[rune]uint16{'A': 1},
		GlyphToRune: map[uint16]rune{1: 'A'},
		Advances:    []uint16{0, 300},
	}
	cat := &sharedfontmetrics.Catalog{Faces: []sharedfontmetrics.NamedFace{{Family: "Demo", Weight: 400, Face: face}}}
	size := 20 // half-points = 10pt
	p := &resolve.Paragraph{Runs: []resolve.Run{{Text: "AAAA AAAA", Format: resolve.RunFormat{FontASCII: "Demo", FontHAnsi: "Demo", FontSize: &size}}}}
	plan := paragraphSlotPlan{PhysicalLineCount: 1, Slots: []lineSlot{{BandIndex: 0, SlotIndex: 0, Span: lineSpan{Left: 0, Right: 30}}}}

	exact := breakParagraphAcrossSlotsWithContext(p, plan, 10, paragraphFlowContext{FontMetrics: cat})
	fallback := breakParagraphAcrossSlots(p, plan, 10)

	if got := len(exact.Lines); got != 1 {
		t.Fatalf("embedded metrics line count = %d, want 1", got)
	}
	if got := len(fallback.Lines); got <= 1 {
		t.Fatalf("heuristic line count = %d, want >1 to prove the metric path changes wrapping", got)
	}
}

func TestRunWidthUsesScriptSpecificEmbeddedFaces(t *testing.T) {
	latin := &sharedfontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{'A': 1}, Advances: []uint16{0, 500}}
	east := &sharedfontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{'漢': 1}, Advances: []uint16{0, 1000}}
	complex := &sharedfontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{'ا': 1}, Advances: []uint16{0, 700}}
	cat := &sharedfontmetrics.Catalog{Faces: []sharedfontmetrics.NamedFace{
		{Family: "Latin", Weight: 400, Face: latin},
		{Family: "East", Weight: 400, Face: east},
		{Family: "Complex", Weight: 400, Face: complex},
	}}
	size := 20 // 10pt
	format := resolve.RunFormat{FontASCII: "Latin", FontHAnsi: "Latin", FontEastAsia: "East", FontCS: "Complex", FontSize: &size, FontSizeCS: &size}
	got, _ := runWordWidthPtWithCatalog(cat, format, "A漢ا", 10)
	// 5 + 10 + 7 = 22pt.
	if got != 22 {
		t.Fatalf("mixed-script width = %g, want 22", got)
	}
}

func TestRunWidthSynthesizesSmallCapsAtReducedSize(t *testing.T) {
	face := &sharedfontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{'A': 1}, Advances: []uint16{0, 500}}
	cat := &sharedfontmetrics.Catalog{Faces: []sharedfontmetrics.NamedFace{{Family: "Demo", Weight: 400, Face: face}}}
	size := 20 // 10pt
	on := true
	format := resolve.RunFormat{FontASCII: "Demo", FontHAnsi: "Demo", FontSize: &size, SmallCaps: &on}
	got, _ := runWordWidthPtWithCatalog(cat, format, "aA", 10)
	// lowercase a -> uppercase A at 80% size: 4pt; authored A remains 5pt.
	if got != 9 {
		t.Fatalf("small-caps width = %g, want 9", got)
	}
}

func TestRunWidthAppliesCharScaleBeforeFitTextFinalWidth(t *testing.T) {
	face := &sharedfontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{'A': 1}, Advances: []uint16{0, 500}}
	cat := &sharedfontmetrics.Catalog{Faces: []sharedfontmetrics.NamedFace{{Family: "Demo", Weight: 400, Face: face}}}
	size, scale, fit := 20, 150, 200 // 10pt, 150%, final 10pt fit width
	format := resolve.RunFormat{FontASCII: "Demo", FontHAnsi: "Demo", FontSize: &size, CharScale: &scale, FitTextWidth: &fit}
	got, _ := runWordWidthPtWithCatalog(cat, format, "AA", 10)
	if got != 10 {
		t.Fatalf("fitText final width = %g, want 10", got)
	}
	if !runNeedsWholeToken(resolve.Run{Text: "AA AA", Format: format}) {
		t.Fatal("fitText run must remain one token so target width is not applied per word")
	}
}

func TestInterWordSpaceUsesEmbeddedSpaceAdvance(t *testing.T) {
	face := &sharedfontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{' ': 1}, Advances: []uint16{0, 200}}
	cat := &sharedfontmetrics.Catalog{Faces: []sharedfontmetrics.NamedFace{{Family: "Demo", Weight: 400, Face: face}}}
	size := 20 // 10pt
	format := resolve.RunFormat{FontASCII: "Demo", FontHAnsi: "Demo", FontSize: &size}
	got := runInterWordSpaceWidthPt(cat, format, 10, 10)
	if got != 2 {
		t.Fatalf("embedded space width = %g, want 2", got)
	}
	fallback := runInterWordSpaceWidthPt(nil, format, 10, 10)
	if fallback == got {
		t.Fatalf("fallback space width = %g, want legacy estimate distinct from embedded 2", fallback)
	}
}
