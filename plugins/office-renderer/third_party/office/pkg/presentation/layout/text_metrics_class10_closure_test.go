package layout

import (
	"math"
	"testing"

	"office/pkg/presentation/ir"
)

// Class 10 closure: one physical line must compose all resolved run metrics
// without asking the renderer to reinterpret font size, script metrics,
// character spacing, kerning thresholds, or baseline shifts.
func TestClass10MixedRunMetricsConvergeInPhysicalLayout(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{
		{Text: "AV", FontFamily: "Arial", FontSize: 2400, CharSpacing: 100, Kern: 1800},
		{Text: "日本", FontFamily: "MS Gothic", FontSize: 2400},
		{Text: "\ue000", FontFamily: "Wingdings", FontSize: 2400, BaselineShiftPct: 30, Superscript: true},
	}}
	got := LayoutParagraphLines(p, ParagraphMetrics{
		AvailableWidthPt:          500,
		FirstLineAvailableWidthPt: 500,
		LineHeightScale:           1,
	}, 1, "", 0)

	if len(got.Lines) != 1 || len(got.Lines[0].Fragments) != 3 {
		t.Fatalf("physical layout = %#v, want one line with three metric-owned fragments", got.Lines)
	}
	line := got.Lines[0]
	latin, cjk, symbol := line.Fragments[0], line.Fragments[1], line.Fragments[2]

	if latin.Visual.FontSizePt != 24 || latin.Visual.CharSpacingPt != 1 ||
		!latin.Visual.KerningSpecified || !latin.Visual.KerningEnabled {
		t.Fatalf("latin visual = %#v, want resolved 24pt/1pt spacing/enabled kerning", latin.Visual)
	}
	if ResolveFontAdvanceMetrics(cjk.Visual.FontFamily).Profile != "cjk" {
		t.Fatalf("CJK font %q did not resolve to CJK physical metrics", cjk.Visual.FontFamily)
	}
	if ResolveFontAdvanceMetrics(symbol.Visual.FontFamily).Profile != "symbol" {
		t.Fatalf("symbol font %q did not resolve to symbol physical metrics", symbol.Visual.FontFamily)
	}
	if symbol.Visual.FontSizePt != 24 || symbol.Visual.BaselineShiftPct != 30 {
		t.Fatalf("symbol visual size/shift = %.3f/%.3f, want authored 24pt/+30%%", symbol.Visual.FontSizePt, symbol.Visual.BaselineShiftPct)
	}
	if symbol.AscentPt <= cjk.AscentPt {
		t.Fatalf("shifted symbol ascent %.3f must contribute beyond unshifted CJK ascent %.3f", symbol.AscentPt, cjk.AscentPt)
	}
	if math.Abs(line.AscentPt-symbol.AscentPt) > .001 {
		t.Fatalf("line ascent %.3f, want max shifted ascent %.3f", line.AscentPt, symbol.AscentPt)
	}
	if math.Abs(line.HeightPt-(line.AscentPt+line.DescentPt)) > .001 {
		t.Fatalf("line box %.3f != ascent+descent %.3f", line.HeightPt, line.AscentPt+line.DescentPt)
	}
}

func TestClass10FrameScaleConvergesFontSizeKerningAndBaselineShift(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{
		{Text: "A", FontFamily: "Arial", FontSize: 2400},
		{Text: "V", FontFamily: "Arial", Kern: 1800, BaselineShiftPct: -20, Subscript: true},
	}}
	got := LayoutParagraphLines(p, ParagraphMetrics{
		AvailableWidthPt:          500,
		FirstLineAvailableWidthPt: 500,
		LineHeightScale:           1,
	}, .5, "", 0)
	if len(got.Lines) != 1 || len(got.Lines[0].Fragments) != 2 {
		t.Fatalf("physical layout = %#v", got.Lines)
	}
	v := got.Lines[0].Fragments[1]
	if math.Abs(v.Visual.FontSizePt-12) > .001 {
		t.Fatalf("scaled inherited font size = %.3fpt, want 12pt", v.Visual.FontSizePt)
	}
	if !v.Visual.KerningSpecified || v.Visual.KerningEnabled {
		t.Fatalf("18pt kern threshold at effective 12pt = specified %v enabled %v, want true/false",
			v.Visual.KerningSpecified, v.Visual.KerningEnabled)
	}
	if v.Visual.BaselineShiftPct != -20 || v.DescentPt <= 0 {
		t.Fatalf("scaled subscript shift/descent = %.3f/%.3f, want -20 and physical descent", v.Visual.BaselineShiftPct, v.DescentPt)
	}
}
