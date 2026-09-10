package render

import (
	"strings"
	"testing"

	"office/pkg/sheet/parsers/ooxml/resolve"
)

func TestPrintHTMLUsesCanonicalPageSlicesAndRepeatsTitles(t *testing.T) {
	s := &resolve.Sheet{
		Name: "Report",
		Rows: []*resolve.Row{
			{Index: 1, Cells: []*resolve.Cell{{Col: 1, Kind: resolve.CellString, StrValue: "Title"}}},
			{Index: 2, Cells: []*resolve.Cell{{Col: 1, Kind: resolve.CellString, StrValue: "One"}}},
			{Index: 3, Cells: []*resolve.Cell{{Col: 1, Kind: resolve.CellString, StrValue: "Two"}}},
		},
		PrintArea:      "$A$1:$A$3",
		PrintTitleRows: "$1:$1",
		RowPageBreaks:  []int{2},
	}
	wb := &resolve.Workbook{Sheets: []*resolve.Sheet{s}}
	got, err := PrintHTML(wb)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(got, `class="print-page"`); n != 2 {
		t.Fatalf("print pages = %d, want 2", n)
	}
	if n := strings.Count(got, ">Title</td>"); n != 2 {
		t.Fatalf("repeated title render count = %d, want 2", n)
	}
	if !strings.Contains(got, `data-sheet-page="2"`) || !strings.Contains(got, `data-physical-page="2"`) {
		t.Fatal("second physical page metadata missing")
	}
}

func TestSheetForPrintPageCollapsesColumnsOutsideHorizontalSlice(t *testing.T) {
	s := &resolve.Sheet{
		Rows: []*resolve.Row{{Index: 1, Cells: []*resolve.Cell{
			{Col: 1, Kind: resolve.CellString, StrValue: "A"},
			{Col: 2, Kind: resolve.CellString, StrValue: "B"},
			{Col: 3, Kind: resolve.CellString, StrValue: "C"},
		}}},
	}
	page := sheetForPrintPage(s, 1, 1, 3, 3, 0, 0, 1, 1)
	if !page.HiddenCols[2] {
		t.Fatal("column B must collapse outside page slice")
	}
	if page.HiddenCols[1] || page.HiddenCols[3] {
		t.Fatal("title/body columns must remain visible")
	}
	if len(page.Rows) != 1 || len(page.Rows[0].Cells) != 2 {
		t.Fatalf("page cells = %#v", page.Rows)
	}
}
