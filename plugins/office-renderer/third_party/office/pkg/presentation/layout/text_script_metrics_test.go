package layout

import (
    "math"
    "testing"
)

func TestEastAsianOfficeFaceUsesFullEmIdeographicAdvance(t *testing.T) {
    m := ResolveFontAdvanceMetrics("MS Gothic")
    if m.Profile != "cjk" { t.Fatalf("profile = %q, want cjk", m.Profile) }
    got := measurePlainTextWidthPt("日本", 20, "MS Gothic", false, false)
    if math.Abs(got-40) > .001 { t.Fatalf("CJK width = %.3fpt, want 40pt", got) }
}

func TestEastAsianOfficeFaceKeepsLatinApproximatelyHalfWidth(t *testing.T) {
    got := measurePlainTextWidthPt("AB", 20, "MS Gothic", false, false)
    if math.Abs(got-20) > .001 { t.Fatalf("Latin width in CJK face = %.3fpt, want 20pt", got) }
}

func TestSymbolFacePrivateUseGlyphUsesEmCellAdvance(t *testing.T) {
    m := ResolveFontAdvanceMetrics("Wingdings")
    if m.Profile != "symbol" { t.Fatalf("profile = %q, want symbol", m.Profile) }
    got := measurePlainTextWidthPt("\uF0A7", 18, "Wingdings", false, false)
    if math.Abs(got-18) > .001 { t.Fatalf("symbol width = %.3fpt, want 18pt", got) }
}

func TestScriptFacesOwnDistinctBaselineMetrics(t *testing.T) {
    latin := ResolveFontAdvanceMetrics("Arial")
    cjk := ResolveFontAdvanceMetrics("MS Gothic")
    if cjk.AscentShare <= latin.AscentShare { t.Fatalf("CJK ascent share = %.3f, want > Arial %.3f", cjk.AscentShare, latin.AscentShare) }
}
