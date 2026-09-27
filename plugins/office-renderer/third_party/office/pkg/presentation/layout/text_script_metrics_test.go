package layout

import (
	"math"
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestEastAsianOfficeFaceUsesFullEmIdeographicAdvance(t *testing.T) {
	m := ResolveFontAdvanceMetrics("MS Gothic")
	if m.Profile != "cjk" {
		t.Fatalf("profile = %q, want cjk", m.Profile)
	}
	got := measurePlainTextWidthPt("日本", 20, "MS Gothic", false, false)
	if math.Abs(got-40) > .001 {
		t.Fatalf("CJK width = %.3fpt, want 40pt", got)
	}
}

func TestEastAsianOfficeFaceKeepsLatinApproximatelyHalfWidth(t *testing.T) {
	got := measurePlainTextWidthPt("AB", 20, "MS Gothic", false, false)
	if math.Abs(got-20) > .001 {
		t.Fatalf("Latin width in CJK face = %.3fpt, want 20pt", got)
	}
}

func TestSymbolFacePrivateUseGlyphUsesEmCellAdvance(t *testing.T) {
	m := ResolveFontAdvanceMetrics("Wingdings")
	if m.Profile != "symbol" {
		t.Fatalf("profile = %q, want symbol", m.Profile)
	}
	got := measurePlainTextWidthPt("\uF0A7", 18, "Wingdings", false, false)
	if math.Abs(got-18) > .001 {
		t.Fatalf("symbol width = %.3fpt, want 18pt", got)
	}
}

func TestScriptFacesOwnDistinctBaselineMetrics(t *testing.T) {
	latin := ResolveFontAdvanceMetrics("Arial")
	cjk := ResolveFontAdvanceMetrics("MS Gothic")
	if cjk.AscentShare <= latin.AscentShare {
		t.Fatalf("CJK ascent share = %.3f, want > Arial %.3f", cjk.AscentShare, latin.AscentShare)
	}
}

func TestExplicitScriptRunUsesReducedSharedPhysicalSize(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{
		{Text: "The most evil waveform for M monitors, J", FontFamily: "Times New Roman", FontSize: 2400},
		{Text: "M", FontFamily: "Times New Roman", FontSize: 2400, Subscript: true, BaselineShiftPct: -25},
		{Text: " , lies in the null space of", FontFamily: "Times New Roman", FontSize: 2400},
	}}
	got := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 663.35, FirstLineAvailableWidthPt: 663.35, LineHeightScale: 1}, 1, "", 0)
	if len(got.Lines) != 1 {
		t.Fatalf("script-aware wrapping produced %d lines, want one: %#v", len(got.Lines), got.Lines)
	}
	script := got.Lines[0].Fragments[1]
	if math.Abs(script.Visual.FontSizePt-24*officeScriptFontScale) > .001 {
		t.Fatalf("script size = %.3fpt, want %.3fpt", script.Visual.FontSizePt, 24*officeScriptFontScale)
	}
}

func TestDecodedSymbolAsciiUsesTextualAdvances(t *testing.T) {
	got := measurePlainTextWidthPt("= 733 ", 20, "Symbol", false, false)
	if got >= 100 {
		t.Fatalf("decoded Symbol text width = %.3fpt, want textual rather than full-em advances", got)
	}
}

func TestDecodedSymbolTextDistinguishesPrivateUseGlyphs(t *testing.T) {
	if !decodedSymbolText("= 733 ") {
		t.Fatal("decoded textual Symbol run should use textual font metrics")
	}
	if decodedSymbolText("\uF0A7") {
		t.Fatal("private-use Symbol glyph must retain Symbol metrics")
	}
}
