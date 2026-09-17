package layout

import (
	"math"
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestMixedFamilyLineComposesAscentAndDescentIndependently(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{
		{Text: "Arial", FontFamily: "Arial", FontSize: 2200},
		{Text: "Times", FontFamily: "Times New Roman", FontSize: 2000},
	}}
	got := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 500, FirstLineAvailableWidthPt: 500, LineHeightScale: 1}, 1, "", 0)
	if len(got.Lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(got.Lines))
	}
	line := got.Lines[0]
	aa, ad := paragraphPhysicalBaselineExtentsPt(22, "Arial", ParagraphMetrics{LineHeightScale: 1})
	ta, td := paragraphPhysicalBaselineExtentsPt(20, "Times New Roman", ParagraphMetrics{LineHeightScale: 1})
	wantA, wantD := math.Max(aa, ta), math.Max(ad, td)
	if math.Abs(line.AscentPt-wantA) > .001 || math.Abs(line.DescentPt-wantD) > .001 {
		t.Fatalf("extents = %.3f/%.3f, want %.3f/%.3f", line.AscentPt, line.DescentPt, wantA, wantD)
	}
	if math.Abs(line.HeightPt-(wantA+wantD)) > .001 {
		t.Fatalf("height = %.3f, want %.3f", line.HeightPt, wantA+wantD)
	}
	tallest := math.Max(paragraphPhysicalLineHeightPt(22, "Arial", ParagraphMetrics{LineHeightScale: 1}), paragraphPhysicalLineHeightPt(20, "Times New Roman", ParagraphMetrics{LineHeightScale: 1}))
	if line.HeightPt <= tallest {
		t.Fatalf("mixed baseline box %.3f should exceed tallest undifferentiated run %.3f", line.HeightPt, tallest)
	}
}

func TestAbsoluteLineSpacingRemainsAuthoredLineBox(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "A", FontFamily: "Arial", FontSize: 3000}, {Text: "g", FontFamily: "Times New Roman", FontSize: 2800}}}
	m := ParagraphMetrics{AvailableWidthPt: 500, FirstLineAvailableWidthPt: 500, LineHeightScale: 1, LineHeightPt: 18}
	got := LayoutParagraphLines(p, m, 1, "", 0)
	if len(got.Lines) != 1 {
		t.Fatalf("lines=%d", len(got.Lines))
	}
	if math.Abs(got.Lines[0].HeightPt-18) > .001 {
		t.Fatalf("height=%.3f, want authored 18pt", got.Lines[0].HeightPt)
	}
}
