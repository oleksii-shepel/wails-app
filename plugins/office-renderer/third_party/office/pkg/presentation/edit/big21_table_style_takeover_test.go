package edit

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
	"testing"
)

func TestTableStyleProjectionKeepsDirectCellPaintMoreSpecific(t *testing.T) {
	dst := &ir.Table{Rows: []*ir.TableRow{{Cells: []*ir.TableCell{{}}}}}
	src := &parts.PresentationTable{
		Props: &parts.PresentationTableProps{TableStyleID: "style", FirstRow: true},
		Grid:  parts.PresentationTableGrid{Columns: []parts.PresentationTableColumn{{W: 100}}},
		Rows:  []parts.PresentationTableRow{{Cells: []parts.PresentationTableCell{{Props: &parts.PresentationTableCellProps{SolidFill: &parts.ColorRef{Val: "112233"}}}}}},
	}
	styles := &parts.TableStyleList{Styles: []parts.DrawingTableStyle{{StyleID: "style", WholeTbl: &parts.TableStyleRegion{CellStyle: &parts.DrawingTableCellStyle{FillColor: &parts.ColorRef{Val: "AABBCC"}}}}}}
	applyTableStyleFrame(dst, src, styles, nil, nil, nil)
	fill, ok := dst.Rows[0].Cells[0].Fill.(drawing.SolidFill)
	if !ok || fill.Color != "112233" {
		t.Fatalf("direct cell fill lost precedence: %#v", dst.Rows[0].Cells[0].Fill)
	}
}
