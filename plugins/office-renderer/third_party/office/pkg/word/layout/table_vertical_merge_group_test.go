package layout

import (
	"testing"

	resolve "office/pkg/word/ir"
)

func TestVerticalMergeIntrinsicHeightBelongsToWholeRowGroup(t *testing.T) {
	long := "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen eighteen"
	table := &resolve.Table{
		Format: resolve.TableFormat{ColumnWidths: []int{700, 700}},
		Rows: []resolve.TableRow{
			{Cells: []resolve.TableCell{
				{Text: long, Format: resolve.TableCellFormat{VMerge: "restart"}, Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: long, Runs: []resolve.Run{{Kind: "run", Text: long}}}}}},
				{Text: "a"},
			}},
			{Cells: []resolve.TableCell{
				{Format: resolve.TableCellFormat{VMerge: "continue"}},
				{Text: "b"},
			}},
			{Cells: []resolve.TableCell{
				{Format: resolve.TableCellFormat{VMerge: "continue"}},
				{Text: "c"},
			}},
		},
	}
	heights := TableRowHeights(&resolve.Document{}, table, 140)
	if len(heights) != 3 {
		t.Fatalf("row heights = %d, want 3", len(heights))
	}
	required := tableCellHeight(&resolve.Document{}, table.Rows[0].Cells[0], table.Format, 70, true, true)
	if got := sumFloat64s(heights); got+0.01 < required {
		t.Fatalf("merge group height = %.2fpt, want >= intrinsic %.2fpt", got, required)
	}
	if heights[0] >= required-0.01 {
		t.Fatalf("restart row absorbed whole merged-cell height %.2fpt; row-group ownership was lost", heights[0])
	}
}

func TestVerticalMergeGroupTracksGridColumnAcrossSpans(t *testing.T) {
	table := &resolve.Table{Rows: []resolve.TableRow{
		{Format: resolve.TableRowFormat{GridBefore: 1}, Cells: []resolve.TableCell{
			{Format: resolve.TableCellFormat{GridSpan: 2, VMerge: "restart"}},
		}},
		{Cells: []resolve.TableCell{
			{}, {Format: resolve.TableCellFormat{GridSpan: 2, VMerge: "continue"}},
		}},
		{Cells: []resolve.TableCell{
			{}, {Format: resolve.TableCellFormat{GridSpan: 2, VMerge: "continue"}},
		}},
	}}
	if got := verticalMergeGroupEnd(table, 0, 1); got != 3 {
		t.Fatalf("vertical merge group end = %d, want 3", got)
	}
}

func TestVerticalMergeBoundaryRemainsAtomic(t *testing.T) {
	table := &resolve.Table{Rows: []resolve.TableRow{
		{Cells: []resolve.TableCell{{Format: resolve.TableCellFormat{VMerge: "restart"}}}},
		{Cells: []resolve.TableCell{{Format: resolve.TableCellFormat{VMerge: "continue"}}}},
		{Cells: []resolve.TableCell{{}}},
	}}
	if tableSplitBoundarySafe(table, 1) {
		t.Fatal("split after restart row must not cut a vertical merge")
	}
	if !tableSplitBoundarySafe(table, 2) {
		t.Fatal("split after completed vertical merge should be legal")
	}
}
