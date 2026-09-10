package layout

import (
	"testing"

	resolve "office/pkg/word/ir"
)

func TestResolvedTableCellInsetsMatchesRendererPrecedence(t *testing.T) {
	tf := resolve.TableFormat{
		CellPadTop: 100, CellPadBottom: 110,
		CellPadLeft: 120, CellPadRight: 130,
		CellSpacing: 200,
	}
	cf := resolve.TableCellFormat{
		CellPadTop: 210,
		CellPadLeft: 220,
	}
	override := &resolve.TableRowOverride{
		CellPadTop: 310,
		CellPadRight: 330,
	}

	top, bottom, left, right := resolvedTableCellInsets(tf, override, 400, cf)
	if top != 310 || bottom != 110 || left != 220 || right != 330 {
		t.Fatalf("resolved insets = (%d,%d,%d,%d), want (310,110,220,330)", top, bottom, left, right)
	}
}

func TestResolvedTableCellInsetsUsesRowSpacingHalfGap(t *testing.T) {
	top, bottom, left, right := resolvedTableCellInsets(
		resolve.TableFormat{CellSpacing: 200},
		nil,
		300,
		resolve.TableCellFormat{},
	)
	if top != 0 || bottom != 0 || left != 150 || right != 150 {
		t.Fatalf("resolved insets = (%d,%d,%d,%d), want (0,0,150,150)", top, bottom, left, right)
	}
}

func TestTableRowHeightIncludesCellSpecificHorizontalInsets(t *testing.T) {
	text := "one two three four five six seven eight nine ten eleven twelve"
	doc := &resolve.Document{}
	base := resolve.TableRow{
		Cells: []resolve.TableCell{{
			Text: text,
			Blocks: []resolve.Block{{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: text,
					Runs: []resolve.Run{{Kind: "run", Text: text}},
				},
			}},
		}},
	}
	padded := base
	padded.Cells = append([]resolve.TableCell(nil), base.Cells...)
	padded.Cells[0].Format.CellPadLeft = 300
	padded.Cells[0].Format.CellPadRight = 300

	tf := resolve.TableFormat{}
	baseHeight := tableRowHeight(doc, base, tf, 90, true, true)
	paddedHeight := tableRowHeight(doc, padded, tf, 90, true, true)
	if paddedHeight <= baseHeight {
		t.Fatalf("padded row height = %.2fpt, base = %.2fpt; narrower canonical content box must not measure shorter", paddedHeight, baseHeight)
	}
}

func TestTableRowOverrideInsetsAffectPaginationMeasurement(t *testing.T) {
	text := "one two three four five six seven eight nine ten eleven twelve"
	row := resolve.TableRow{
		Override: &resolve.TableRowOverride{CellPadLeft: 400, CellPadRight: 400},
		Cells: []resolve.TableCell{{
			Text: text,
			Blocks: []resolve.Block{{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Text: text,
					Runs: []resolve.Run{{Kind: "run", Text: text}},
				},
			}},
		}},
	}
	without := row
	without.Override = nil

	doc := &resolve.Document{}
	withHeight := tableRowHeight(doc, row, resolve.TableFormat{}, 100, true, true)
	withoutHeight := tableRowHeight(doc, without, resolve.TableFormat{}, 100, true, true)
	if withHeight <= withoutHeight {
		t.Fatalf("row override height = %.2fpt, base = %.2fpt; tblPrEx geometry was ignored", withHeight, withoutHeight)
	}
}
