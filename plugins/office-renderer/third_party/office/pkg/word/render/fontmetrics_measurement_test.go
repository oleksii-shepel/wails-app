package render

import (
	"math"
	"testing"

	"office/pkg/fontmetrics"
	resolve "office/pkg/word/ir"
)

func TestEstimateTextWidthUsesSharedFontMetricsWhenAvailable(t *testing.T) {
	face := &fontmetrics.Face{
		UnitsPerEm:  1000,
		RuneToGlyph: map[rune]uint16{'A': 1, 'B': 2},
		Advances:    []uint16{0, 600, 500},
	}
	r := &htmlRenderer{
		doc:         &resolve.Document{},
		fontMetrics: &fontmetrics.Catalog{Faces: []fontmetrics.NamedFace{{Family: "Demo", Weight: 400, Face: face}}},
	}
	sz := 40 // 20pt
	got := r.estimateTextWidthForRun("AB", resolve.RunFormat{FontASCII: "Demo", FontSize: &sz})
	if math.Abs(got-22) > .001 {
		t.Fatalf("width = %.3f, want 22", got)
	}
}

func TestEstimateTextWidthHonorsWordKerningThreshold(t *testing.T) {
	face := &fontmetrics.Face{
		UnitsPerEm:  1000,
		RuneToGlyph: map[rune]uint16{'A': 1, 'B': 2},
		Advances:    []uint16{0, 600, 500},
	}
	// Pair positioning is deliberately omitted from the public constructor in
	// this test; the threshold behavior is covered through MeasureTextWithPairs
	// in pkg/fontmetrics. This assertion protects the Word-side fallback path.
	r := &htmlRenderer{
		doc:         &resolve.Document{},
		fontMetrics: &fontmetrics.Catalog{Faces: []fontmetrics.NamedFace{{Family: "Demo", Weight: 400, Face: face}}},
	}
	sz := 40
	threshold := 48 // 24pt, so 20pt text must not enable pair positioning.
	got := r.estimateTextWidthForRun("AB", resolve.RunFormat{FontASCII: "Demo", FontSize: &sz, Kerning: &threshold})
	if math.Abs(got-22) > .001 {
		t.Fatalf("width below kerning threshold = %.3f, want 22", got)
	}
}

func TestEstimateTextWidthSharesScriptAwareContractWithLayout(t *testing.T) {
	latin := &fontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{'A': 1}, Advances: []uint16{0, 500}}
	cjk := &fontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{'漢': 1}, Advances: []uint16{0, 900}}
	cs := &fontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{'ا': 1}, Advances: []uint16{0, 700}}
	r := &htmlRenderer{doc: &resolve.Document{}, fontMetrics: &fontmetrics.Catalog{Faces: []fontmetrics.NamedFace{
		{Family: "Latin", Weight: 400, Face: latin},
		{Family: "CJK", Weight: 400, Face: cjk},
		{Family: "CS", Weight: 400, Face: cs},
	}}}
	sz := 20 // 10pt
	got := r.estimateTextWidthForRun("A漢ا", resolve.RunFormat{FontASCII: "Latin", FontHAnsi: "Latin", FontEastAsia: "CJK", FontCS: "CS", FontSize: &sz, FontSizeCS: &sz})
	if math.Abs(got-21) > .001 {
		t.Fatalf("mixed-script width = %.3f, want 21", got)
	}
}

func TestEstimateTextWidthHonorsSmallCapsCharScaleAndFitText(t *testing.T) {
	face := &fontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{'A': 1}, Advances: []uint16{0, 600}}
	r := &htmlRenderer{doc: &resolve.Document{}, fontMetrics: &fontmetrics.Catalog{Faces: []fontmetrics.NamedFace{{Family: "Demo", Weight: 400, Face: face}}}}
	sz, scale, fit := 20, 150, 400
	smallCaps := true
	got := r.estimateTextWidthForRun("aa", resolve.RunFormat{FontASCII: "Demo", FontSize: &sz, SmallCaps: &smallCaps, CharScale: &scale, FitTextWidth: &fit})
	if math.Abs(got-20) > .001 {
		t.Fatalf("fitText width = %.3f, want 20", got)
	}
}

func TestPeekAheadTextWidthForTabsUsesSharedRunContract(t *testing.T) {
	face := &fontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{'A': 1}, Advances: []uint16{0, 600}}
	r := &htmlRenderer{doc: &resolve.Document{}, fontMetrics: &fontmetrics.Catalog{Faces: []fontmetrics.NamedFace{{Family: "Demo", Weight: 400, Face: face}}}}
	sz, scale, fit := 20, 150, 360 // final authored width = 18pt
	smallCaps := true
	runs := []resolve.Run{
		{Kind: "tab"},
		{Kind: "text", Text: "aa", Format: resolve.RunFormat{FontASCII: "Demo", FontSize: &sz, SmallCaps: &smallCaps, CharScale: &scale, FitTextWidth: &fit}},
	}
	got := r.peekAheadTextWidth(runs, 1)
	if math.Abs(got-18) > .001 {
		t.Fatalf("tab look-ahead width = %.3f, want 18", got)
	}
}
