package layout

import (
	"math"
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestResolveParagraphMetricsOwnsPhysicalSpacingAndAvailableWidth(t *testing.T) {
	p := &ir.Paragraph{
		MarginLeftEMU:  91440,
		MarginRightEMU: 182880,
		TextIndentEMU:  -45720,
		HasMarginLeft:  true,
		HasMarginRight: true,
		HasTextIndent:  true,
		SpaceBefore:    600,
		SpaceAfter:     300,
		HasSpaceBefore: true,
		HasSpaceAfter:  true,
		LineSpacingPct: 120000,
		HasLineSpacing: true,
		Runs:           []*ir.Run{{Text: "text"}},
	}
	got := ResolveParagraphMetrics(p, 300, 18, .82)
	near(t, "left", got.MarginLeftPt, 7.2)
	near(t, "right", got.MarginRightPt, 14.4)
	near(t, "indent", got.TextIndentPt, -3.6)
	near(t, "before", got.SpaceBeforePt, 6)
	near(t, "after", got.SpaceAfterPt, 3)
	near(t, "width", got.AvailableWidthPt, 278.4)
	near(t, "line scale", got.LineHeightScale, .984)
}

func TestResolveParagraphMetricsConvertsPercentageSpacingUsingTextLineHeight(t *testing.T) {
	p := &ir.Paragraph{SpaceBeforePct: 50000, SpaceAfterPct: 125000, Runs: []*ir.Run{{Text: "text"}}}
	got := ResolveParagraphMetrics(p, 100, 20, 1)
	near(t, "before", got.SpaceBeforePt, 12)
	near(t, "after", got.SpaceAfterPt, 30)
}

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s = %.12f, want %.12f", name, got, want)
	}
}
