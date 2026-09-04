package resolve

import (
	"strings"
	"testing"

	"office/pkg/ooxml/chart"
)

func TestHydrateWorkbookChartsFillsFormulaBackedSeries(t *testing.T) {
	wb := &Workbook{
		Sheets: []*Sheet{{
			Name: "Price History",
			Rows: []*Row{{
				Index: 27,
				Cells: []*Cell{{
					Row: 27, Col: 6, Ref: "F27", Kind: CellFormula, StrValue: "Close",
				}},
			}, {
				Index: 28,
				Cells: []*Cell{{
					Row: 28, Col: 2, Ref: "B28", Kind: CellNumber, NumValue: 44560.66667,
					Style: CellStyle{NumFmtCode: `mmm" "d", "yyyy`},
				}, {
					Row: 28, Col: 6, Ref: "F28", Kind: CellNumber, NumValue: 146,
				}},
			}, {
				Index: 29,
				Cells: []*Cell{{
					Row: 29, Col: 2, Ref: "B29", Kind: CellNumber, NumValue: 44561.66667,
					Style: CellStyle{NumFmtCode: `mmm" "d", "yyyy`},
				}, {
					Row: 29, Col: 6, Ref: "F29", Kind: CellNumber, NumValue: 144.68,
				}},
			}},
			Drawings: []*Drawing{{
				Type: DrawingTypeChart,
				Chart: &DrawingChart{Chart: &chart.Chart{
					ChartType: chart.ChartArea,
					Series: []chart.Series{{
						NameFormula:  "'Price History'!$F$27",
						ValueFormula: "'Price History'!$F$28:$F$29",
						LabelFormula: "'Price History'!$B$28:$B$29",
					}},
				}},
			}},
		}},
	}

	hydrateWorkbookCharts(wb)

	parsed := wb.Sheets[0].Drawings[0].Chart.Chart.(*chart.Chart)
	if parsed.Series[0].Name != "Close" {
		t.Fatalf("expected hydrated series name, got %q", parsed.Series[0].Name)
	}
	if len(parsed.Series[0].Values) != 2 || parsed.Series[0].Values[0] != 146 || parsed.Series[0].Values[1] != 144.68 {
		t.Fatalf("expected hydrated values, got %#v", parsed.Series[0].Values)
	}
	if len(parsed.Categories) != 2 || parsed.Categories[0] != "Dec 30, 2021" || parsed.Categories[1] != "Dec 31, 2021" {
		t.Fatalf("expected hydrated categories, got %#v", parsed.Categories)
	}
	if wb.Sheets[0].Drawings[0].Chart.SVG == "" || strings.Contains(wb.Sheets[0].Drawings[0].Chart.SVG, "area chart") {
		t.Fatalf("expected rendered SVG with real chart data, got %q", wb.Sheets[0].Drawings[0].Chart.SVG)
	}
}

func TestHydrateWorkbookChartsEvaluatesIndirectSpillSeries(t *testing.T) {
	wb := &Workbook{
		Sheets: []*Sheet{
			{
				Name: "Income",
				Rows: []*Row{{
					Index: 4,
					Cells: []*Cell{
						{Row: 4, Col: 4, Ref: "D4", Kind: CellNumber, NumValue: 120},
						{Row: 4, Col: 5, Ref: "E4", Kind: CellNumber, NumValue: 150},
						{Row: 4, Col: 6, Ref: "F4", Kind: CellNumber, NumValue: 180},
					},
				}},
			},
			{
				Name: "Summary",
				Rows: []*Row{
					{
						Index: 30,
						Cells: []*Cell{
							{Row: 30, Col: 4, Ref: "D30", Kind: CellString, StrValue: "Jan"},
							{Row: 30, Col: 5, Ref: "E30", Kind: CellString, StrValue: "Feb"},
							{Row: 30, Col: 6, Ref: "F30", Kind: CellString, StrValue: "Mar"},
						},
					},
					{
						Index: 31,
						Cells: []*Cell{
							{Row: 31, Col: 2, Ref: "B31", Kind: CellNumber, NumValue: 4},
							{
								Row: 31, Col: 4, Ref: "D31", Kind: CellFormula, NumValue: 0,
								Formula: `IF(NOT(ISBLANK(B31)),INDIRECT("Income!D"&B31&":F"&B31),"")`,
							},
						},
					},
				},
				Drawings: []*Drawing{{
					Type: DrawingTypeChart,
					Chart: &DrawingChart{Chart: &chart.Chart{
						ChartType: chart.ChartLine,
						Series: []chart.Series{{
							Name:         "Income",
							Values:       []float64{0, 0, 0},
							ValueFormula: "Summary!$D$31:$F$31",
							LabelFormula: "Summary!$D$30:$F$30",
						}},
					}},
				}},
			},
		},
	}

	hydrateWorkbookCharts(wb)

	parsed := wb.Sheets[1].Drawings[0].Chart.Chart.(*chart.Chart)
	if len(parsed.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(parsed.Series))
	}
	if got := parsed.Series[0].Values; len(got) != 3 || got[0] != 120 || got[1] != 150 || got[2] != 180 {
		t.Fatalf("expected hydrated spill values, got %#v", got)
	}
	if len(parsed.Categories) != 3 || parsed.Categories[0] != "Jan" || parsed.Categories[1] != "Feb" || parsed.Categories[2] != "Mar" {
		t.Fatalf("expected hydrated spill categories, got %#v", parsed.Categories)
	}
	if wb.Sheets[1].Drawings[0].Chart.SVG == "" || strings.Contains(wb.Sheets[1].Drawings[0].Chart.SVG, "line chart") {
		t.Fatalf("expected rendered SVG with hydrated spill data, got %q", wb.Sheets[1].Drawings[0].Chart.SVG)
	}
}

func TestHydrateWorkbookChartsResolvesDefinedNameBackedFormula(t *testing.T) {
	wb := &Workbook{
		DefinedNames: []*DefinedName{{
			Name:  "StartingBalance",
			Value: `'Setup'!$C$13`,
		}},
		Sheets: []*Sheet{
			{
				Name: "Setup",
				Rows: []*Row{{
					Index: 13,
					Cells: []*Cell{{
						Row: 13, Col: 3, Ref: "C13", Kind: CellNumber, NumValue: 5000,
					}},
				}},
			},
			{
				Name: "Summary",
				Rows: []*Row{
					{
						Index: 30,
						Cells: []*Cell{{
							Row: 30, Col: 4, Ref: "D30", Kind: CellString, StrValue: "Jan",
						}},
					},
					{
						Index: 31,
						Cells: []*Cell{{
							Row: 31, Col: 4, Ref: "D31", Kind: CellNumber, NumValue: 0, Formula: "StartingBalance",
						}},
					},
				},
				Drawings: []*Drawing{{
					Type: DrawingTypeChart,
					Chart: &DrawingChart{Chart: &chart.Chart{
						ChartType: chart.ChartLine,
						Series: []chart.Series{{
							Name:         "Ending balance",
							Values:       []float64{0},
							ValueFormula: "Summary!$D$31",
							LabelFormula: "Summary!$D$30",
						}},
					}},
				}},
			},
		},
	}

	hydrateWorkbookCharts(wb)

	parsed := wb.Sheets[1].Drawings[0].Chart.Chart.(*chart.Chart)
	if got := parsed.Series[0].Values; len(got) != 1 || got[0] != 5000 {
		t.Fatalf("expected defined-name-backed value 5000, got %#v", got)
	}
	if len(parsed.Categories) != 1 || parsed.Categories[0] != "Jan" {
		t.Fatalf("expected hydrated category, got %#v", parsed.Categories)
	}
}

func TestHydrateWorkbookChartsResolvesCachedExternalWorkbookSeries(t *testing.T) {
	wb := &Workbook{
		ExternalLinks: []*WorkbookExternalLink{{
			Target: "file:///C:/workbooks/source.xlsx",
			CachedSheets: []*WorkbookExternalSheet{{
				ID:   0,
				Name: "Prices",
				Cells: []*WorkbookExternalCell{
					{Ref: "A1", Type: "str", Value: "Revenue"},
					{Ref: "B1", Type: "str", Value: "Jan"},
					{Ref: "B2", Type: "str", Value: "Feb"},
					{Ref: "B3", Type: "str", Value: "Mar"},
					{Ref: "C1", Type: "n", Value: "12"},
					{Ref: "C2", Type: "n", Value: "15"},
					{Ref: "C3", Type: "n", Value: "18"},
				},
			}},
		}},
		Sheets: []*Sheet{{
			Name: "Summary",
			Drawings: []*Drawing{{
				Type: DrawingTypeChart,
				Chart: &DrawingChart{Chart: &chart.Chart{
					ChartType: chart.ChartLine,
					Series: []chart.Series{{
						NameFormula:  `'[source.xlsx]Prices'!$A$1`,
						ValueFormula: `[1]Prices!$C$1:$C$3`,
						LabelFormula: `[1]Prices!$B$1:$B$3`,
					}},
				}},
			}},
		}},
	}

	hydrateWorkbookCharts(wb)

	parsed := wb.Sheets[0].Drawings[0].Chart.Chart.(*chart.Chart)
	if parsed.Series[0].Name != "Revenue" {
		t.Fatalf("expected hydrated external series name, got %q", parsed.Series[0].Name)
	}
	if got := parsed.Series[0].Values; len(got) != 3 || got[0] != 12 || got[1] != 15 || got[2] != 18 {
		t.Fatalf("expected hydrated external values, got %#v", got)
	}
	if len(parsed.Categories) != 3 || parsed.Categories[0] != "Jan" || parsed.Categories[1] != "Feb" || parsed.Categories[2] != "Mar" {
		t.Fatalf("expected hydrated external categories, got %#v", parsed.Categories)
	}
	if wb.Sheets[0].Drawings[0].Chart.SVG == "" || strings.Contains(wb.Sheets[0].Drawings[0].Chart.SVG, "line chart") {
		t.Fatalf("expected rendered SVG with hydrated external data, got %q", wb.Sheets[0].Drawings[0].Chart.SVG)
	}
}

func TestHydrateWorkbookChartsEvaluatesGetPivotDataFormulaSeries(t *testing.T) {
	wb := &Workbook{
		Sheets: []*Sheet{
			{
				Name: "Data",
				Rows: []*Row{
					{Index: 1, Cells: []*Cell{
						{Row: 1, Col: 1, Ref: "A1", Kind: CellString, StrValue: "Region"},
						{Row: 1, Col: 2, Ref: "B1", Kind: CellString, StrValue: "Sales"},
					}},
					{Index: 2, Cells: []*Cell{
						{Row: 2, Col: 1, Ref: "A2", Kind: CellString, StrValue: "North"},
						{Row: 2, Col: 2, Ref: "B2", Kind: CellNumber, NumValue: 10},
					}},
					{Index: 3, Cells: []*Cell{
						{Row: 3, Col: 1, Ref: "A3", Kind: CellString, StrValue: "South"},
						{Row: 3, Col: 2, Ref: "B3", Kind: CellNumber, NumValue: 20},
					}},
					{Index: 4, Cells: []*Cell{
						{Row: 4, Col: 1, Ref: "A4", Kind: CellString, StrValue: "North"},
						{Row: 4, Col: 2, Ref: "B4", Kind: CellNumber, NumValue: 5},
					}},
				},
			},
			{
				Name: "Pivot",
				Rows: []*Row{{Index: 3, Cells: []*Cell{{Row: 3, Col: 1, Ref: "A3", Kind: CellString, StrValue: "Pivot"}}}},
				PivotTables: []*PivotTable{{
					Name:        "SalesPivot",
					LocationRef: "A3:C8",
					SourceSheet: "Data",
					SourceRef:   "A1:B4",
					RowFields:   []string{"Region"},
					DataFields: []*PivotDataField{{
						Name:            "Sum of Sales",
						SourceFieldName: "Sales",
						Subtotal:        "sum",
					}},
				}},
			},
			{
				Name: "Summary",
				Rows: []*Row{
					{Index: 1, Cells: []*Cell{
						{Row: 1, Col: 1, Ref: "A1", Kind: CellFormula, Formula: `GETPIVOTDATA("Sum of Sales",'Pivot'!A3,"Region","North")`},
						{Row: 1, Col: 2, Ref: "B1", Kind: CellString, StrValue: "North"},
					}},
				},
				Drawings: []*Drawing{{
					Type: DrawingTypeChart,
					Chart: &DrawingChart{Chart: &chart.Chart{
						ChartType: chart.ChartLine,
						Series: []chart.Series{{
							Name:         "North Sales",
							Values:       []float64{0},
							ValueFormula: "Summary!$A$1",
							LabelFormula: "Summary!$B$1",
						}},
					}},
				}},
			},
		},
	}

	hydrateWorkbookCharts(wb)

	parsed := wb.Sheets[2].Drawings[0].Chart.Chart.(*chart.Chart)
	if got := parsed.Series[0].Values; len(got) != 1 || got[0] != 15 {
		t.Fatalf("expected GETPIVOTDATA-hydrated value 15, got %#v", got)
	}
	if len(parsed.Categories) != 1 || parsed.Categories[0] != "North" {
		t.Fatalf("expected hydrated GETPIVOTDATA category, got %#v", parsed.Categories)
	}
}

func TestHydrateWorkbookChartsEvaluatesStructuredTableFormulaSeries(t *testing.T) {
	wb := &Workbook{
		Sheets: []*Sheet{{
			Name: "Sheet1",
			Tables: []*Table{{
				ID:             1,
				Name:           "SalesTable",
				DisplayName:    "SalesTable",
				Ref:            "A1:B4",
				HeaderRowCount: 1,
				Columns: []*TableColumn{
					{ID: 1, Name: "Region"},
					{ID: 2, Name: "Sales"},
				},
			}},
			Rows: []*Row{
				{Index: 1, Cells: []*Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: CellString, StrValue: "Region"},
					{Row: 1, Col: 2, Ref: "B1", Kind: CellString, StrValue: "Sales"},
					{Row: 1, Col: 4, Ref: "D1", Kind: CellString, StrValue: "Total"},
				}},
				{Index: 2, Cells: []*Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: CellString, StrValue: "North"},
					{Row: 2, Col: 2, Ref: "B2", Kind: CellNumber, NumValue: 10},
					{Row: 2, Col: 4, Ref: "D2", Kind: CellFormula, Formula: `SUM(SalesTable[Sales])`},
				}},
				{Index: 3, Cells: []*Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: CellString, StrValue: "South"},
					{Row: 3, Col: 2, Ref: "B3", Kind: CellNumber, NumValue: 20},
				}},
				{Index: 4, Cells: []*Cell{
					{Row: 4, Col: 1, Ref: "A4", Kind: CellString, StrValue: "North"},
					{Row: 4, Col: 2, Ref: "B4", Kind: CellNumber, NumValue: 5},
				}},
			},
			Drawings: []*Drawing{{
				Type: DrawingTypeChart,
				Chart: &DrawingChart{Chart: &chart.Chart{
					ChartType: chart.ChartLine,
					Series: []chart.Series{{
						Name:         "Sales Total",
						Values:       []float64{0},
						ValueFormula: "Sheet1!$D$2",
						LabelFormula: "Sheet1!$D$1",
					}},
				}},
			}},
		}},
	}

	hydrateWorkbookCharts(wb)

	parsed := wb.Sheets[0].Drawings[0].Chart.Chart.(*chart.Chart)
	if got := parsed.Series[0].Values; len(got) != 1 || got[0] != 35 {
		t.Fatalf("expected structured-table-hydrated value 35, got %#v", got)
	}
	if len(parsed.Categories) != 1 || parsed.Categories[0] != "Total" {
		t.Fatalf("expected structured-table category, got %#v", parsed.Categories)
	}
}

func TestHydrateWorkbookChartsEvaluatesThreeDReferenceSeries(t *testing.T) {
	wb := &Workbook{
		Sheets: []*Sheet{
			{
				Name: "Jan",
				Rows: []*Row{{
					Index: 2,
					Cells: []*Cell{{
						Row: 2, Col: 2, Ref: "B2", Kind: CellNumber, NumValue: 10,
					}},
				}},
			},
			{
				Name: "Feb",
				Rows: []*Row{{
					Index: 2,
					Cells: []*Cell{{
						Row: 2, Col: 2, Ref: "B2", Kind: CellNumber, NumValue: 15,
					}},
				}},
			},
			{
				Name: "Mar",
				Rows: []*Row{{
					Index: 2,
					Cells: []*Cell{{
						Row: 2, Col: 2, Ref: "B2", Kind: CellNumber, NumValue: 20,
					}},
				}},
			},
			{
				Name: "Summary",
				Rows: []*Row{
					{Index: 1, Cells: []*Cell{
						{Row: 1, Col: 1, Ref: "A1", Kind: CellFormula, Formula: `SUM(Jan:Mar!B2)`},
						{Row: 1, Col: 2, Ref: "B1", Kind: CellString, StrValue: "Q1"},
					}},
				},
				Drawings: []*Drawing{{
					Type: DrawingTypeChart,
					Chart: &DrawingChart{Chart: &chart.Chart{
						ChartType: chart.ChartLine,
						Series: []chart.Series{{
							Name:         "Quarter total",
							Values:       []float64{0},
							ValueFormula: "Summary!$A$1",
							LabelFormula: "Summary!$B$1",
						}},
					}},
				}},
			},
		},
	}

	hydrateWorkbookCharts(wb)

	parsed := wb.Sheets[3].Drawings[0].Chart.Chart.(*chart.Chart)
	if got := parsed.Series[0].Values; len(got) != 1 || got[0] != 45 {
		t.Fatalf("expected 3D-reference-hydrated value 45, got %#v", got)
	}
	if len(parsed.Categories) != 1 || parsed.Categories[0] != "Q1" {
		t.Fatalf("expected 3D-reference category, got %#v", parsed.Categories)
	}
}
