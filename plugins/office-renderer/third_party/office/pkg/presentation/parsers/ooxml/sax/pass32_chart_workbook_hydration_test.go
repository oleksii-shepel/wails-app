package sax

import (
	"testing"

	chartpkg "office/pkg/ooxml/chart"
	sheetir "office/pkg/sheet/ir"
)

func TestPass32HydratesFormulaBackedOOXMLChartFromEmbeddedWorkbookModel(t *testing.T) {
	c := &chartpkg.Chart{
		ChartType: chartpkg.ChartLine,
		Series: []chartpkg.Series{{
			NameFormula:  "'Data Sheet'!$A$1",
			LabelFormula: "'Data Sheet'!$B$1:$D$1",
			ValueFormula: "'Data Sheet'!$B$2:$D$2",
		}},
	}
	wb := &sheetir.Workbook{Sheets: []*sheetir.Sheet{{
		Name: "Data Sheet",
		Rows: []*sheetir.Row{
			{Index: 1, Cells: []*sheetir.Cell{
				{Row: 1, Col: 1, Ref: "A1", Kind: sheetir.CellString, StrValue: "Revenue"},
				{Row: 1, Col: 2, Ref: "B1", Kind: sheetir.CellString, StrValue: "Jan"},
				{Row: 1, Col: 3, Ref: "C1", Kind: sheetir.CellString, StrValue: "Feb"},
				{Row: 1, Col: 4, Ref: "D1", Kind: sheetir.CellString, StrValue: "Mar"},
			}},
			{Index: 2, Cells: []*sheetir.Cell{
				{Row: 2, Col: 2, Ref: "B2", Kind: sheetir.CellNumber, NumValue: 10},
				{Row: 2, Col: 3, Ref: "C2", Kind: sheetir.CellNumber, NumValue: 20},
				{Row: 2, Col: 4, Ref: "D2", Kind: sheetir.CellNumber, NumValue: 30},
			}},
		},
	}}}

	if !hydrateChartFromWorkbook(c, wb) {
		t.Fatal("expected embedded workbook to hydrate chart")
	}
	if c.Series[0].Name != "Revenue" {
		t.Fatalf("series name = %q, want Revenue", c.Series[0].Name)
	}
	if got := c.Series[0].Values; len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 30 {
		t.Fatalf("values = %#v", got)
	}
	if got := c.Categories; len(got) != 3 || got[0] != "Jan" || got[2] != "Mar" {
		t.Fatalf("categories = %#v", got)
	}
}

func TestPass32ParsesQuotedChartFormulaRanges(t *testing.T) {
	sheet, sr, sc, er, ec, ok := parseChartA1Range("='Sales Data'!$B$2:$D$4")
	if !ok || sheet != "Sales Data" || sr != 2 || sc != 2 || er != 4 || ec != 4 {
		t.Fatalf("range = %q %d,%d:%d,%d ok=%v", sheet, sr, sc, er, ec, ok)
	}
}
