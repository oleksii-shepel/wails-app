package layout

import (
	"testing"

	resolve "office/pkg/word/ir"
)

func TestAutoTableIndentConsumesContainingBlockWidth(t *testing.T) {
	table := &resolve.Table{
		Format: resolve.TableFormat{Indent: 400}, // 20pt
		Rows:   []resolve.TableRow{{Cells: []resolve.TableCell{{Text: "x"}}}},
	}
	if got, want := TableWidthPtForDocument(&resolve.Document{}, table, 100), 80.0; got != want {
		t.Fatalf("auto indented table width = %.2fpt, want %.2fpt", got, want)
	}
}

func TestExplicitTableWidthRemainsAuthoritativeWithIndent(t *testing.T) {
	width := 1800 // 90pt
	table := &resolve.Table{
		Format: resolve.TableFormat{Width: &width, WidthType: "dxa", Indent: 400},
	}
	if got, want := TableWidthPtForDocument(&resolve.Document{}, table, 100), 90.0; got != want {
		t.Fatalf("explicit indented table width = %.2fpt, want %.2fpt", got, want)
	}
}

func TestNestedAutoTableIndentIncreasesParentCellHeight(t *testing.T) {
	text := "one two three four five six seven eight nine ten eleven twelve thirteen fourteen"
	nested := func(indent int) *resolve.Table {
		return &resolve.Table{
			Format: resolve.TableFormat{Indent: indent},
			Rows: []resolve.TableRow{{Cells: []resolve.TableCell{{
				Text: text,
				Blocks: []resolve.Block{{
					Kind: "paragraph",
					Paragraph: &resolve.Paragraph{
						Text: text,
						Runs: []resolve.Run{{Kind: "run", Text: text}},
					},
				}},
			}}}},
		}
	}
	parent := func(child *resolve.Table) resolve.TableCell {
		return resolve.TableCell{Blocks: []resolve.Block{{Kind: "table", Table: child}}}
	}

	doc := &resolve.Document{}
	base := tableCellHeight(doc, parent(nested(0)), resolve.TableFormat{}, 100, true, true)
	indented := tableCellHeight(doc, parent(nested(600)), resolve.TableFormat{}, 100, true, true)
	if indented <= base {
		t.Fatalf("indented nested table height = %.2fpt, base = %.2fpt; containing-block width was not propagated recursively", indented, base)
	}
}

func TestNestedTableUsesParentCellContentBox(t *testing.T) {
	text := "one two three four five six seven eight nine ten eleven twelve"
	child := &resolve.Table{
		Rows: []resolve.TableRow{{Cells: []resolve.TableCell{{
			Text: text,
			Blocks: []resolve.Block{{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: text,
					Runs: []resolve.Run{{Kind: "run", Text: text}},
				},
			}},
		}}}},
	}
	parentCell := resolve.TableCell{
		Format: resolve.TableCellFormat{CellPadLeft: 300, CellPadRight: 300},
		Blocks: []resolve.Block{{Kind: "table", Table: child}},
	}
	doc := &resolve.Document{}
	withInsets := tableCellHeight(doc, parentCell, resolve.TableFormat{}, 100, true, true)

	parentCell.Format.CellPadLeft = 0
	parentCell.Format.CellPadRight = 0
	withoutInsets := tableCellHeight(doc, parentCell, resolve.TableFormat{}, 100, true, true)
	if withInsets <= withoutInsets {
		t.Fatalf("nested table in padded cell = %.2fpt, unpadded = %.2fpt; child did not inherit the parent content box", withInsets, withoutInsets)
	}
}
