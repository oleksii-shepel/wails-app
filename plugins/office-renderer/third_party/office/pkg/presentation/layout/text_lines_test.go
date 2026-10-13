package layout

import (
	"math"
	ir "office/pkg/presentation/ir"
	"testing"
)

func TestLayoutParagraphLinesWrapsAtPhysicalWidth(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "alpha beta gamma", FontSize: 1200}}}
	got := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 45, LineHeightScale: 1}, 1, "", 0)
	if len(got.Lines) < 2 {
		t.Fatalf("lines = %d, want wrapping", len(got.Lines))
	}
	if got.ContentHeightPt <= got.Lines[0].HeightPt {
		t.Fatalf("content height = %.2f, want multiple physical lines", got.ContentHeightPt)
	}
}
func TestLayoutParagraphLinesPreservesExplicitBreakAndRunOwnership(t *testing.T) {
	a := &ir.Run{Text: "one\ntwo", FontSize: 1200}
	b := &ir.Run{Text: " three", FontSize: 1800}
	got := LayoutParagraphLines(&ir.Paragraph{Runs: []*ir.Run{a, b}}, ParagraphMetrics{AvailableWidthPt: 300, LineHeightScale: 1}, 1, "", 0)
	if len(got.Lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(got.Lines))
	}
	if got.Lines[0].Fragments[0].Run != a || got.Lines[1].Fragments[len(got.Lines[1].Fragments)-1].Run != b {
		t.Fatal("physical lines lost canonical run ownership")
	}
	if got.Lines[1].HeightPt <= got.Lines[0].HeightPt {
		t.Fatal("mixed font line must own tallest run height")
	}
}
func TestLayoutParagraphLinesAccountsForMarkerWidth(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "alpha beta", FontSize: 1200}}}
	plain := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 60, LineHeightScale: 1}, 1, "", 0)
	marked := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 60, LineHeightScale: 1}, 1, "1.", 12)
	if len(marked.Lines) <= len(plain.Lines) {
		t.Fatalf("marker did not reduce first-line capacity: plain=%d marked=%d", len(plain.Lines), len(marked.Lines))
	}
}

func TestLayoutParagraphLinesSplitsOverwideWordAtCaretBoundary(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "Spread-spectrum", FontSize: 1600, FontFamily: "Arial"}}}
	got := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 40, LineHeightScale: 1}, 1, "", 0)
	if len(got.Lines) < 2 {
		t.Fatalf("overwide word lines = %d, want caret wrapping", len(got.Lines))
	}
	for i, line := range got.Lines {
		if line.WidthPt > 40.01 {
			t.Fatalf("line %d width = %.2f, want within 40pt: %#v", i, line.WidthPt, line)
		}
	}
}

func TestLayoutParagraphLinesDistinguishesAuthoredBreakFromWrap(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "one two\nthree", FontSize: 1200}}}
	got := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 25, LineHeightScale: 1}, 1, "", 0)
	seenWrap, seenAuthored := false, false
	for _, line := range got.Lines {
		seenWrap = seenWrap || line.BreakAfter == "wrap"
		seenAuthored = seenAuthored || line.BreakAfter == "authored"
	}
	if !seenWrap || !seenAuthored {
		t.Fatalf("break provenance lost: %#v", got.Lines)
	}
}

func TestLayoutParagraphLinesTextIndentOnlyConsumesFirstLine(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "alpha beta gamma delta", FontSize: 1200}}}
	got := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 60, HasTextIndent: true, TextIndentPt: 25, LineHeightScale: 1}, 1, "", 0)
	if len(got.Lines) < 2 {
		t.Fatalf("lines=%d, want wrap", len(got.Lines))
	}
	// The first-line capacity is 35pt while continuation lines recover the full
	// 60pt. Assert the physical contract directly rather than comparing content
	// widths, which legitimately vary with family-aware glyph metrics.
	if got.Lines[0].BreakAfter != "wrap" {
		t.Fatalf("first line break = %q, want wrap from first-line indent", got.Lines[0].BreakAfter)
	}
	withoutIndent := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 60, FirstLineAvailableWidthPt: 60, LineHeightScale: 1}, 1, "", 0)
	if len(withoutIndent.Lines) >= len(got.Lines) {
		t.Fatalf("first-line indent did not reduce only initial capacity: with=%d without=%d", len(got.Lines), len(withoutIndent.Lines))
	}
}

func TestCanonicalFontAdvancesFollowResolvedFamily(t *testing.T) {
	arial := measureRunTextWidthPt("Navigation Data", &ir.Run{FontFamily: "Arial"}, 16)
	mono := measureRunTextWidthPt("Navigation Data", &ir.Run{FontFamily: "Consolas"}, 16)
	if arial >= mono {
		t.Fatalf("family metrics ignored: Arial=%.2f mono=%.2f", arial, mono)
	}
}

func TestLayoutParagraphLinesMaterializesPhysicalTabAdvance(t *testing.T) {
	p := &ir.Paragraph{DefaultTabSizeEMU: 457200, Runs: []*ir.Run{{Text: "A\tB", FontSize: 1200}}}
	got := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 200, LineHeightScale: 1}, 1, "", 0)
	if len(got.Lines) != 1 {
		t.Fatalf("lines=%d", len(got.Lines))
	}
	found := false
	for _, f := range got.Lines[0].Fragments {
		if f.Text == "\t" {
			found = true
			if f.WidthPt <= 0 {
				t.Fatal("tab has no physical advance")
			}
		}
	}
	if !found {
		t.Fatal("tab was not materialized")
	}
}

func TestArialPhysicalAdvancesRespectNarrowAndWideGlyphs(t *testing.T) {
	narrow := measurePlainTextWidthPt("ill", 16, "Arial", false, false)
	wide := measurePlainTextWidthPt("mwm", 16, "Arial", false, false)
	if !(narrow < wide*.5) {
		t.Fatalf("Arial narrow glyph width = %.3f, wide = %.3f; want family glyph classes to affect physical advances", narrow, wide)
	}
}

func TestLegacyStyleLabelFitsWithCanonicalGlyphAdvances(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "   - Clock Data", FontSize: 1600}}}
	metrics := ParagraphMetrics{AvailableWidthPt: 102, FirstLineAvailableWidthPt: 102, LineHeightScale: 1}
	got := LayoutParagraphLines(p, metrics, 1, "", 0)
	if len(got.Lines) != 1 {
		t.Fatalf("legacy label physical lines = %d, want 1; average-glyph metrics must not over-wrap narrow letters", len(got.Lines))
	}
}

func TestArialCanonicalAdvancesDistinguishRealGlyphWidths(t *testing.T) {
	narrow := measurePlainTextWidthPt("Illi", 16, "Arial", false, false)
	wide := measurePlainTextWidthPt("MWmw", 16, "Arial", false, false)
	if !(wide > narrow*2.5) {
		t.Fatalf("Arial exact advances lost: narrow=%.3f wide=%.3f", narrow, wide)
	}
}

func TestTimesCanonicalAdvancesKeepProseMeasurementFamilySpecific(t *testing.T) {
	arial := measurePlainTextWidthPt("Navigation Data", 16, "Arial", false, false)
	times := measurePlainTextWidthPt("Navigation Data", 16, "Times New Roman", false, false)
	if arial == times {
		t.Fatalf("family-specific standard advances collapsed: Arial=%.3f Times=%.3f", arial, times)
	}
}

func TestPlayfairDisplayUsesSerifMetricsNotNarrowSans(t *testing.T) {
	playfair := ResolveFontAdvanceMetrics("Playfair Display")
	if playfair.Profile != "times" {
		t.Fatalf("Playfair profile = %q, want serif/times metrics", playfair.Profile)
	}
	if playfair.LineBoxScale != 1.20 {
		t.Fatalf("Playfair line box = %.3f, want Office-style 1.20 leading", playfair.LineBoxScale)
	}
}

func TestLucidaSansUnicodeUsesDedicatedMetricsForPowerPointWrapping(t *testing.T) {
	lucida := measurePlainTextWidthPt("Euclidean Geometry", 27, "Lucida Sans Unicode", false, false)
	generic := measurePlainTextWidthPt("Euclidean Geometry", 27, "", false, false)
	if lucida == generic {
		t.Fatalf("Lucida width = %.3f, generic = %.3f; want dedicated Lucida metrics", lucida, generic)
	}
}

func TestPhysicalMarkerUsesResolvedFamilyGapAndOwnsFirstLineHeight(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "alpha beta", FontSize: 1200, FontFamily: "Arial"}}}
	metrics := ParagraphMetrics{AvailableWidthPt: 70, FirstLineAvailableWidthPt: 70, LineHeightScale: 1}
	gap := measurePlainTextWidthPt(" ", 24, "Times New Roman", false, false)
	got := layoutParagraphLinesWithMarker(p, metrics, 1, "1.", 24, "Times New Roman", gap)
	if len(got.Lines) == 0 {
		t.Fatal("marker paragraph produced no physical line")
	}
	wantMinHeight := paragraphPhysicalLineHeightPt(24, "Arial", metrics)
	if got.Lines[0].HeightPt < wantMinHeight {
		t.Fatalf("first line height = %.4f, want marker-owned minimum %.4f", got.Lines[0].HeightPt, wantMinHeight)
	}
}

func TestUnknownFontMeasuresWithHTMLArialFallbackMetrics(t *testing.T) {
	unknown := ResolveFontAdvanceMetrics("Some Missing Office Font")
	arial := ResolveFontAdvanceMetrics("Arial")
	if unknown.Profile != arial.Profile || unknown.LineBoxScale != arial.LineBoxScale {
		t.Fatalf("unknown metrics = %#v, want Arial fallback %#v", unknown, arial)
	}
	got := measurePlainTextWidthPt("Property ID: 12345-6", 20, "Some Missing Office Font", false, false)
	want := measurePlainTextWidthPt("Property ID: 12345-6", 20, "Arial", false, false)
	// Arial itself is answered by the bundled metric tables while an unknown
	// family keeps the Arial advance model. The two agree within a percent; the
	// assertion pins the model, not the glyph table behind it.
	if math.Abs(got-want)/want > .01 {
		t.Fatalf("fallback width = %.4f, want Arial %.4f within one percent", got, want)
	}
}

func TestImplicitLineHeightUsesResolvedFaceMetrics(t *testing.T) {
	m := ParagraphMetrics{LineHeightScale: 1}
	got := paragraphPhysicalLineHeightPt(20, "Arial", m)
	// PowerPoint's single spacing for Latin faces is 1.2 em.
	want := 20.0 * 1.2
	if math.Abs(got-want) > .001 {
		t.Fatalf("Arial line height = %.4f, want %.4f", got, want)
	}
}
