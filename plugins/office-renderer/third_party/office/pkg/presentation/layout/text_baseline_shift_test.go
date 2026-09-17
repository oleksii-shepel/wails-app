package layout

import (
    "math"
    "testing"
    ir "office/pkg/presentation/ir"
)

func TestRunVisualPlanPreservesAuthoredBaselineShiftWithoutChangingFontMetrics(t *testing.T) {
    r := &ir.Run{Text: "2", FontSize: 2400, FontFamily: "Arial", Superscript: true, BaselineShiftPct: 30}
    got := ResolveRunVisualPlan(r, 1)
    if math.Abs(got.FontSizePt-24) > .001 { t.Fatalf("font size = %.3f, want 24pt", got.FontSizePt) }
    if math.Abs(got.BaselineShiftPct-30) > .001 { t.Fatalf("baseline = %.3f, want 30%%", got.BaselineShiftPct) }
    p := &ir.Paragraph{Runs: []*ir.Run{{Text:"H", FontSize:2400, FontFamily:"Arial"}, r}}
    m := ResolveParagraphMetrics(p, 200, 24, 1)
    lines := LayoutParagraphLines(p, m, 1, "", 0)
    // The authored font size remains 24pt, but the physical line box must grow
    // by the 30% baseline displacement so the raised glyph is not clipped.
    // The ordinary run still owns the descent on the opposite side.
    base := 24 * ResolveFontAdvanceMetrics("Arial").LineBoxScale
    want := base + 24*0.30
    if len(lines.Lines)!=1 || math.Abs(lines.Lines[0].HeightPt-want)>.001 { t.Fatalf("line height = %#v, want %.3f", lines.Lines, want) }
}
