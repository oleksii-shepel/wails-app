package layout

import (
	"testing"

	resolve "office/pkg/word/ir"
)

func TestKeepNextStopsBeforePageBreakBefore(t *testing.T) {
	blocks := []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Format: resolve.ParagraphFormat{KeepNext: true}}},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Format: resolve.ParagraphFormat{PageBreakBefore: true}}},
	}
	if got := keepNextClusterEnd(blocks, 0); got != 0 {
		t.Fatalf("keep cluster crossed PageBreakBefore: end = %d, want 0", got)
	}
}

func TestKeepNextStopsAfterManualPageBreak(t *testing.T) {
	blocks := []resolve.Block{
		{
			Kind: "paragraph",
			Paragraph: &resolve.Paragraph{
				Format: resolve.ParagraphFormat{KeepNext: true},
				Runs: []resolve.Run{{Kind: "break", BreakType: "page"}},
			},
		},
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{}},
	}
	if got := keepNextClusterEnd(blocks, 0); got != 0 {
		t.Fatalf("keep cluster crossed manual page break: end = %d, want 0", got)
	}
}

func TestKeepNextCanStillTerminateInTableWithoutBoundary(t *testing.T) {
	blocks := []resolve.Block{
		{Kind: "paragraph", Paragraph: &resolve.Paragraph{Format: resolve.ParagraphFormat{KeepNext: true}}},
		{Kind: "table", Table: &resolve.Table{}},
	}
	if got := keepNextClusterEnd(blocks, 0); got != 1 {
		t.Fatalf("paragraph->table keep cluster end = %d, want 1", got)
	}
}

func TestOversizedKeepClusterDoesNotAdvanceForever(t *testing.T) {
	if keepClusterShouldAdvance(120, 40, 100, true) {
		t.Fatal("oversized keep group must fall back to normal fragmentation")
	}
	if !keepClusterShouldAdvance(70, 40, 100, true) {
		t.Fatal("fitting keep group should advance when remaining region is too small")
	}
	if keepClusterShouldAdvance(70, 0, 100, false) {
		t.Fatal("empty region must accept the keep group")
	}
}

func TestExplicitPageBoundaryClassification(t *testing.T) {
	before := resolve.Block{
		Kind: "paragraph",
		Paragraph: &resolve.Paragraph{Format: resolve.ParagraphFormat{PageBreakBefore: true}},
	}
	if got := blockExplicitFlowBoundary(&before); got != explicitFlowBoundaryPageBefore {
		t.Fatalf("page-before boundary = %v", got)
	}

	after := resolve.Block{
		Kind: "paragraph",
		Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{Kind: "break", BreakType: "PAGE"}}},
	}
	if got := blockExplicitFlowBoundary(&after); got != explicitFlowBoundaryPageAfter {
		t.Fatalf("page-after boundary = %v", got)
	}
}
