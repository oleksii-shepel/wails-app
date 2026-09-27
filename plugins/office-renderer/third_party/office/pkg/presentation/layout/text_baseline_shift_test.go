package layout

import (
	"math"
	ir "office/pkg/presentation/ir"
	"testing"
)

func TestRunVisualPlanPreservesAuthoredBaselineShiftWhileLayoutScalesScriptMetrics(t *testing.T) {
	r := &ir.Run{Text: "2", FontSize: 2400, FontFamily: "Arial", Superscript: true, BaselineShiftPct: 30}
	got := ResolveRunVisualPlan(r, 1)
	if math.Abs(got.FontSizePt-24) > .001 {
		t.Fatalf("font size = %.3f, want 24pt", got.FontSizePt)
	}
	if math.Abs(got.BaselineShiftPct-30) > .001 {
		t.Fatalf("baseline = %.3f, want 30%%", got.BaselineShiftPct)
	}
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "H", FontSize: 2400, FontFamily: "Arial"}, r}}
	m := ResolveParagraphMetrics(p, 200, 24, 1)
	lines := LayoutParagraphLines(p, m, 1, "", 0)
	// The canonical run remains 24pt, while its physical script glyph uses the
	// shared PowerPoint script scale for wrapping, painting and line extents.
	base := 24 * ResolveFontAdvanceMetrics("Arial").LineBoxScale
	want := base
	if len(lines.Lines) != 1 || math.Abs(lines.Lines[0].HeightPt-want) > .001 {
		t.Fatalf("line height = %#v, want %.3f", lines.Lines, want)
	}
	if got := lines.Lines[0].Fragments[1].Visual.FontSizePt; math.Abs(got-24*officeScriptFontScale) > .001 {
		t.Fatalf("script size = %.3fpt", got)
	}
}
