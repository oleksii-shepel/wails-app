package layout

import (
	"math"
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestMixedFontFragmentsOwnPhysicalTopOffsets(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{
		{Text: "A", FontSize: 2400, FontFamily: "Arial"},
		{Text: "g", FontSize: 1800, FontFamily: "Times New Roman"},
	}}
	metrics := ResolveParagraphMetrics(p, 300, 24, 1)
	got := LayoutParagraphLines(p, metrics, 1, "", 0)
	if len(got.Lines) != 1 || len(got.Lines[0].Fragments) != 2 {
		t.Fatalf("lines/fragments = %d/%d", len(got.Lines), len(got.Lines[0].Fragments))
	}
	line := got.Lines[0]
	for i, f := range line.Fragments {
		want := math.Max(0, line.AscentPt-f.AscentPt)
		if math.Abs(f.TopOffsetPt-want) > .001 {
			t.Fatalf("fragment %d top = %.3f, want %.3f", i, f.TopOffsetPt, want)
		}
		if f.AscentPt <= 0 || f.DescentPt <= 0 {
			t.Fatalf("fragment %d missing baseline extents: %.3f/%.3f", i, f.AscentPt, f.DescentPt)
		}
	}
	if !(line.Fragments[1].TopOffsetPt > line.Fragments[0].TopOffsetPt) {
		t.Fatalf("smaller run should sit lower in shared baseline box: %+v", line.Fragments)
	}
}
