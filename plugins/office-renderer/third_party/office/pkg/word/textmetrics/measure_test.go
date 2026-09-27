package textmetrics

import (
	"math"
	"testing"

	sharedfontmetrics "office/pkg/fontmetrics"
	resolve "office/pkg/word/ir"
)

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

func testFace(adv map[rune]uint16) *sharedfontmetrics.Face {
	face := &sharedfontmetrics.Face{UnitsPerEm: 1000, RuneToGlyph: map[rune]uint16{}, Advances: []uint16{0}}
	for r, a := range adv {
		face.RuneToGlyph[r] = uint16(len(face.Advances))
		face.Advances = append(face.Advances, a)
	}
	return face
}

func TestRunWidthUsesScriptSpecificFaces(t *testing.T) {
	cat := &sharedfontmetrics.Catalog{Faces: []sharedfontmetrics.NamedFace{
		{Family: "Latin", Weight: 400, Face: testFace(map[rune]uint16{'A': 500})},
		{Family: "CJK", Weight: 400, Face: testFace(map[rune]uint16{'漢': 900})},
		{Family: "CS", Weight: 400, Face: testFace(map[rune]uint16{'ا': 700})},
	}}
	size := 20 // 10pt half-points
	got, _ := RunWidthPt(cat, resolve.RunFormat{FontASCII: "Latin", FontHAnsi: "Latin", FontEastAsia: "CJK", FontCS: "CS", FontSize: &size, FontSizeCS: &size}, "A漢ا", 10)
	if math.Abs(got-21) > .001 {
		t.Fatalf("width = %.3f, want 21", got)
	}
}

func TestRunWidthSmallCapsCharScaleAndFitTextOrder(t *testing.T) {
	cat := &sharedfontmetrics.Catalog{Faces: []sharedfontmetrics.NamedFace{{Family: "Demo", Weight: 400, Face: testFace(map[rune]uint16{'A': 600})}}}
	size, scale, fit := 20, 150, 400 // 10pt, 150%, final 20pt
	format := resolve.RunFormat{FontASCII: "Demo", FontSize: &size, SmallCaps: boolPtr(true), CharScale: &scale, FitTextWidth: &fit}
	got, _ := RunWidthPt(cat, format, "aa", 10)
	if math.Abs(got-20) > .001 {
		t.Fatalf("fitText final width = %.3f, want 20", got)
	}
	natural := NaturalWidthPt(cat, format, "aa", 10)
	if math.Abs(natural-9.6) > .001 {
		t.Fatalf("small-caps natural width = %.3f, want 9.6", natural)
	}
}

func TestInterWordSpaceUsesRealSpaceAdvance(t *testing.T) {
	cat := &sharedfontmetrics.Catalog{Faces: []sharedfontmetrics.NamedFace{{Family: "Demo", Weight: 400, Face: testFace(map[rune]uint16{' ': 500})}}}
	size := 20
	got := InterWordSpaceWidthPt(cat, resolve.RunFormat{FontASCII: "Demo", FontSize: &size}, 10, 10)
	if math.Abs(got-5) > .001 {
		t.Fatalf("space width = %.3f, want 5", got)
	}
}

func TestRunWidthReservesFallbackBoldWidth(t *testing.T) {
	size := 20
	regular, _ := RunWidthPt(nil, resolve.RunFormat{FontSize: &size}, "legacy", 10)
	bold, _ := RunWidthPt(nil, resolve.RunFormat{FontSize: &size, Bold: boolPtr(true)}, "legacy", 10)
	if math.Abs(bold-regular*1.08) > .001 {
		t.Fatalf("fallback bold width = %.3f, want %.3f", bold, regular*1.08)
	}
}
