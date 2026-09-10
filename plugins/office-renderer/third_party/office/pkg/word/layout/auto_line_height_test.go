package layout

import (
	"testing"

	resolve "office/pkg/word/ir"
)

func TestAutoParagraphLineHeightUsesWordLineMultiple(t *testing.T) {
	line := 276 // 1.15 Word lines
	p := &resolve.Paragraph{Format: resolve.ParagraphFormat{
		LineRule:    "auto",
		LineSpacing: &line,
	}}
	got := autoParagraphLineHeightPt(p, 10)
	want := 10 * cssNormalLineRatio * 1.15
	if !almostEqual(got, want) {
		t.Fatalf("auto line height = %.3f, want %.3f", got, want)
	}
}

func TestAutoParagraphLineHeightPreservesSubNormalMultiples(t *testing.T) {
	single := 240
	oneFifteen := 276
	p1 := &resolve.Paragraph{Format: resolve.ParagraphFormat{LineRule: "auto", LineSpacing: &single}}
	p2 := &resolve.Paragraph{Format: resolve.ParagraphFormat{LineRule: "auto", LineSpacing: &oneFifteen}}

	h1 := autoParagraphLineHeightPt(p1, 10)
	h2 := autoParagraphLineHeightPt(p2, 10)
	if h2 <= h1 {
		t.Fatalf("1.15-line paragraph height = %.3f, single = %.3f; want larger authored line metric", h2, h1)
	}
	if !almostEqual(h2/h1, 1.15) {
		t.Fatalf("auto line-height ratio = %.3f, want 1.15", h2/h1)
	}
}
