package layout

import (
	"testing"

	resolve "office/pkg/word/ir"
)

func TestFirstParagraphSpacingBeforeContributesInsideTableCell(t *testing.T) {
	before := 240 // 12pt
	p := &resolve.Paragraph{
		Text: "cell",
		Format: resolve.ParagraphFormat{SpacingBefore: &before},
		Runs: []resolve.Run{{Kind: "run", Text: "cell"}},
	}
	cell := resolve.TableCell{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: p}}}
	doc := &resolve.Document{}

	got := tableCellHeight(doc, cell, resolve.TableFormat{}, 120, true, true)

	plain := *p
	plain.Format.SpacingBefore = nil
	cell.Blocks[0].Paragraph = &plain
	base := tableCellHeight(doc, cell, resolve.TableFormat{}, 120, true, true)

	if got-base < 11.99 {
		t.Fatalf("first cell paragraph spacing-before delta = %.2fpt, want 12pt", got-base)
	}
}

func TestCellParagraphContextualSpacingStillSuppressesAdjacentSameStyle(t *testing.T) {
	before := 200
	p1 := &resolve.Paragraph{Text: "a", StyleID: "Body", Runs: []resolve.Run{{Kind: "run", Text: "a"}}}
	p2 := &resolve.Paragraph{
		Text: "b",
		StyleID: "Body",
		Format: resolve.ParagraphFormat{
			SpacingBefore: &before,
			ContextualSpacing: true,
		},
		Runs: []resolve.Run{{Kind: "run", Text: "b"}},
	}
	if got := tableCellParagraphSpacingBefore(&resolve.Document{}, p2, p1); got != 0 {
		t.Fatalf("contextual spacing-before = %.2fpt, want 0", got)
	}
}

func TestCellParagraphSpacingBeforeDoesNotUsePageFlowStartSuppression(t *testing.T) {
	before := 160
	p := &resolve.Paragraph{
		Format: resolve.ParagraphFormat{SpacingBefore: &before},
	}
	if got, want := tableCellParagraphSpacingBefore(&resolve.Document{}, p, nil), 8.0; got != want {
		t.Fatalf("cell first paragraph spacing-before = %.2fpt, want %.2fpt", got, want)
	}
	if got := ParagraphSpacingBefore(&resolve.Document{}, p, nil, PageGeometry{}, true); got != 0 {
		t.Fatalf("page-flow start spacing-before = %.2fpt, want 0", got)
	}
}
