package layout

import (
	"strings"
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestOswaldUsesDisplayFaceAdvancesForPhysicalWrapping(t *testing.T) {
	// sample-3 embeds Oswald Regular. Its embedded TrueType hmtx measures
	// "WEST COAST" at 224.784pt and "ROAD TRIP" at 190.368pt at 48pt.
	// Keep the deterministic layout approximation on the same side of the
	// presentation's 237.8835pt content width so authored rows are not split.
	const contentWidthPt = 252.2835 - 7.2 - 7.2
	west := measurePlainTextWidthPt("WEST COAST", 48, "Oswald", false, false)
	road := measurePlainTextWidthPt("ROAD TRIP", 48, "Oswald", false, false)
	if west > contentWidthPt || road > contentWidthPt {
		t.Fatalf("Oswald authored rows do not fit: WEST COAST=%.3fpt ROAD TRIP=%.3fpt available=%.3fpt", west, road, contentWidthPt)
	}

	// This mirrors sample-3 slide 1: one paragraph, an explicit DrawingML <a:br>,
	// and noAutofit. The authored break must remain the only physical boundary.
	p := &ir.Paragraph{Runs: []*ir.Run{{Text: "WEST COAST\nROAD TRIP", FontFamily: "Oswald", FontSize: 4800}}}
	metrics := ParagraphMetrics{AvailableWidthPt: contentWidthPt, FirstLineAvailableWidthPt: contentWidthPt, LineHeightScale: 1}
	got := LayoutParagraphLines(p, metrics, 1, "", 0)
	if len(got.Lines) != 2 {
		t.Fatalf("physical lines = %d, want 2: %#v", len(got.Lines), got.Lines)
	}
	lineText := func(line PhysicalTextLine) string {
		var b strings.Builder
		for _, f := range line.Fragments {
			b.WriteString(f.Text)
		}
		return strings.TrimSpace(b.String())
	}
	if a, b := lineText(got.Lines[0]), lineText(got.Lines[1]); a != "WEST COAST" || b != "ROAD TRIP" {
		t.Fatalf("physical rows = %q / %q, want WEST COAST / ROAD TRIP", a, b)
	}
	if got.Lines[0].BreakAfter != "authored" {
		t.Fatalf("first break = %q, want authored", got.Lines[0].BreakAfter)
	}
	if got.Lines[1].BreakAfter == "wrap" {
		t.Fatalf("second row acquired calculated wrap: %#v", got.Lines[1])
	}
}
