package layout

import (
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestPhysicalTextFragmentsCarryResolvedRunPaint(t *testing.T) {
	run := &ir.Run{Text: "Hello", FontSize: 1800, FontFamily: "Arial", Bold: true, Color: "112233"}
	p := &ir.Paragraph{Runs: []*ir.Run{run}}
	metrics := ParagraphMetrics{AvailableWidthPt: 200, FirstLineAvailableWidthPt: 200, LineHeightScale: 1}
	got := LayoutParagraphLines(p, metrics, 0.8, "", 0)
	if len(got.Lines) != 1 || len(got.Lines[0].Fragments) != 1 {
		t.Fatalf("physical fragments = %#v, want one composed fragment", got.Lines)
	}
	visual := got.Lines[0].Fragments[0].Visual
	if visual.FontFamily != "Arial" || visual.FontSizePt != 14.4 || !visual.Bold || visual.Color != "112233" {
		t.Fatalf("physical run visual = %#v", visual)
	}
}

func TestAuthoredPhysicalTextFragmentsCarryResolvedRunPaint(t *testing.T) {
	run := &ir.Run{Text: "A\nB", FontSize: 1200, FontFamily: "Courier New", Italic: true}
	p := &ir.Paragraph{Runs: []*ir.Run{run}}
	metrics := ParagraphMetrics{AvailableWidthPt: 100, FirstLineAvailableWidthPt: 100, LineHeightScale: 1}
	got := LayoutParagraphAuthoredLines(p, metrics, 1, "", 0)
	if len(got.Lines) != 2 {
		t.Fatalf("authored lines = %d, want 2", len(got.Lines))
	}
	for i, line := range got.Lines {
		if len(line.Fragments) != 1 || line.Fragments[0].Visual.FontFamily != "Courier New" || !line.Fragments[0].Visual.Italic {
			t.Fatalf("line %d physical run visual = %#v", i, line.Fragments)
		}
	}
}
