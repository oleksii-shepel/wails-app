package layout

import (
    "math"
    "testing"

    "office/pkg/presentation/ir"
)

func TestSparseRunVisualUsesSameInheritedFontSizeAsPhysicalMeasurement(t *testing.T) {
    p := &ir.Paragraph{Runs: []*ir.Run{
        {Text: "A", FontFamily: "Arial", FontSize: 2400},
        {Text: "B", FontFamily: "Arial"},
    }}
    got := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 500, LineHeightScale: 1}, 1, "", 0)
    if len(got.Lines) != 1 || len(got.Lines[0].Fragments) != 2 {
        t.Fatalf("physical fragments = %#v, want one line with two fragments", got.Lines)
    }
    inherited := got.Lines[0].Fragments[1]
    if math.Abs(inherited.Visual.FontSizePt-24) > .001 {
        t.Fatalf("sparse run visual font size = %.3fpt, want measured inherited 24pt", inherited.Visual.FontSizePt)
    }
    if inherited.AscentPt <= 0 || inherited.DescentPt <= 0 {
        t.Fatalf("sparse run physical metrics = ascent %.3f descent %.3f, want materialized metrics", inherited.AscentPt, inherited.DescentPt)
    }
}

func TestSparseRunKerningThresholdUsesEffectivePhysicalFontSize(t *testing.T) {
    p := &ir.Paragraph{Runs: []*ir.Run{
        {Text: "A", FontFamily: "Arial", FontSize: 2400},
        {Text: "V", FontFamily: "Arial", Kern: 1800},
    }}
    got := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 500, LineHeightScale: 1}, 1, "", 0)
    v := got.Lines[0].Fragments[1].Visual
    if v.FontSizePt != 24 || !v.KerningSpecified || !v.KerningEnabled {
        t.Fatalf("sparse run visual = size %.3f specified %v enabled %v, want 24/true/true", v.FontSizePt, v.KerningSpecified, v.KerningEnabled)
    }
}

func TestSparseRunEffectiveFontSizeHonorsFrameScale(t *testing.T) {
    p := &ir.Paragraph{Runs: []*ir.Run{
        {Text: "A", FontFamily: "Arial", FontSize: 2400},
        {Text: "B", FontFamily: "Arial"},
    }}
    got := LayoutParagraphLines(p, ParagraphMetrics{AvailableWidthPt: 500, LineHeightScale: 1}, .5, "", 0)
    if size := got.Lines[0].Fragments[1].Visual.FontSizePt; math.Abs(size-12) > .001 {
        t.Fatalf("scaled inherited visual font size = %.3fpt, want 12pt", size)
    }
}
