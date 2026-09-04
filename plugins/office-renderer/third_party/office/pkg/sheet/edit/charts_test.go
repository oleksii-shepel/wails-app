package edit_test

import (
	"bytes"
	"testing"

	chartmodel "office/pkg/ooxml/chart"
	"office/pkg/sheet/edit"
	sheetir "office/pkg/sheet/ir"
)

func TestSheetChartCreationRoundTrip(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	chart := edit.NewChart(chartmodel.ChartColumn, "Sales")
	chart.Categories = []string{"Q1", "Q2"}
	chart.Series = []chartmodel.Series{{
		Name:   "North",
		Values: []float64{10, 20},
	}}
	drawing, err := sheet.AddChartOneCell("Sales Chart", chart, 1, 4, 3600000, 2400000)
	if err != nil {
		t.Fatalf("AddChartOneCell: %v", err)
	}
	drawing.SetLegend(true, "b")
	drawing.SetTitle("Quarterly Sales")

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	drawings := roundTrip.Sheets[0].Drawings
	if len(drawings) != 1 {
		t.Fatalf("drawings = %d, want 1", len(drawings))
	}
	if drawings[0].Type != sheetir.DrawingTypeChart || drawings[0].Chart == nil {
		t.Fatalf("drawing = %#v, want chart", drawings[0])
	}
	got, ok := drawings[0].Chart.Chart.(*chartmodel.Chart)
	if !ok {
		t.Fatalf("chart payload = %T, want *chart.Chart", drawings[0].Chart.Chart)
	}
	if got.Title != "Quarterly Sales" || got.ChartType != chartmodel.ChartColumn {
		t.Errorf("chart = %#v", got)
	}
	if len(got.Series) != 1 || got.Series[0].Name != "North" || len(got.Series[0].Values) != 2 || got.Series[0].Values[1] != 20 {
		t.Errorf("series = %#v", got.Series)
	}
	if len(got.Categories) != 2 || got.Categories[0] != "Q1" {
		t.Errorf("categories = %#v", got.Categories)
	}
	if drawings[0].Position.Anchor != sheetir.AnchorOneCell || drawings[0].Position.FromCell.Col != 4 {
		t.Errorf("position = %#v", drawings[0].Position)
	}
	if drawings[0].Chart.SVG == "" {
		t.Errorf("expected hydrated chart SVG")
	}
}

func TestSheetChartMutationAndRemoval(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	chart := edit.NewChart(chartmodel.ChartLine, "Trend")
	drawing, err := sheet.AddChartTwoCell("Trend Chart", chart, 2, 2, 10, 8)
	if err != nil {
		t.Fatalf("AddChartTwoCell: %v", err)
	}
	drawing.SetCategories([]string{"Jan", "Feb"})
	drawing.AddSeries(chartmodel.Series{Name: "Actual", Values: []float64{1, 2}})
	drawing.ReplaceSeries([]chartmodel.Series{{Name: "Plan", Values: []float64{3, 4}}})
	if err := drawing.SetPosition(sheetir.DrawingPosition{
		Anchor: sheetir.AnchorAbsolute,
		X:      100,
		Y:      200,
		Width:  300,
		Height: 400,
	}); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	got, ok := sheet.Chart("Trend Chart")
	if !ok {
		t.Fatalf("Chart lookup failed")
	}
	if got.Chart().Series[0].Name != "Plan" || got.Chart().Series[0].Index != 0 {
		t.Fatalf("series = %#v", got.Chart().Series)
	}
	if !sheet.RemoveChart("Trend Chart") {
		t.Fatalf("RemoveChart returned false")
	}
	if len(sheet.Drawings()) != 0 {
		t.Fatalf("drawings = %d, want 0", len(sheet.Drawings()))
	}
}

func TestMultipleSheetChartsUseDistinctParts(t *testing.T) {
	wb := edit.NewWorkbook()
	for _, name := range []string{"One", "Two"} {
		sheet, err := wb.CreateSheet(name)
		if err != nil {
			t.Fatalf("CreateSheet %s: %v", name, err)
		}
		chart := edit.NewChart(chartmodel.ChartPie, name+" Chart")
		chart.Categories = []string{"A", "B"}
		chart.Series = []chartmodel.Series{{Name: "Share", Values: []float64{40, 60}}}
		if _, err := sheet.AddChartOneCell(name+" Chart", chart, 1, 1, 2000000, 1200000); err != nil {
			t.Fatalf("AddChartOneCell %s: %v", name, err)
		}
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	roundTrip := resolveWorkbook(t, buf.Bytes())
	if len(roundTrip.Sheets) != 2 {
		t.Fatalf("sheet count = %d, want 2", len(roundTrip.Sheets))
	}
	for i, sheet := range roundTrip.Sheets {
		if len(sheet.Drawings) != 1 || sheet.Drawings[0].Chart == nil {
			t.Fatalf("sheet %d drawings = %#v", i, sheet.Drawings)
		}
	}
}

func TestOpenXLSXExistingChartMutationRoundTrip(t *testing.T) {
	wb := openFixtureWorkbook(t, "sample-4.xlsx")
	sheet, chart := findSheetChart(t, wb, "Line chart showing stock changes over time.")

	chart.SetTitle("Updated Existing Chart")
	chart.SetLegend(true, "t")
	chart.SetCategories([]string{"Jan", "Feb", "Mar"})
	chart.ReplaceSeries([]chartmodel.Series{{
		Name:   "Adjusted",
		Values: []float64{3, 5, 8},
	}})
	if err := chart.SetPosition(sheetir.DrawingPosition{
		Anchor:   sheetir.AnchorOneCell,
		FromCell: sheetir.DrawingCellAnchor{Row: 20, Col: 3},
		Width:    7200000,
		Height:   2400000,
	}); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	var got *sheetir.Drawing
	for _, rtSheet := range roundTrip.Sheets {
		for _, drawing := range rtSheet.Drawings {
			if drawing != nil && drawing.Type == sheetir.DrawingTypeChart && drawing.Name == "Line chart showing stock changes over time." {
				got = drawing
				break
			}
		}
		if got != nil {
			break
		}
	}
	if got == nil {
		t.Fatal("existing chart not found after round-trip")
	}
	rtChart, ok := got.Chart.Chart.(*chartmodel.Chart)
	if !ok {
		t.Fatalf("round-trip chart payload = %T, want *chart.Chart", got.Chart.Chart)
	}
	if rtChart.Title != "Updated Existing Chart" {
		t.Fatalf("chart title = %q, want Updated Existing Chart", rtChart.Title)
	}
	if rtChart.Legend.Position != "t" || !rtChart.Legend.Show {
		t.Fatalf("legend = %#v", rtChart.Legend)
	}
	if len(rtChart.Categories) != 3 || rtChart.Categories[2] != "Mar" {
		t.Fatalf("categories = %#v", rtChart.Categories)
	}
	if len(rtChart.Series) != 1 || rtChart.Series[0].Name != "Adjusted" || len(rtChart.Series[0].Values) != 3 || rtChart.Series[0].Values[2] != 8 {
		t.Fatalf("series = %#v", rtChart.Series)
	}
	if got.Position.Anchor != sheetir.AnchorOneCell || got.Position.FromCell.Row != 20 || got.Position.FromCell.Col != 3 {
		t.Fatalf("position = %#v", got.Position)
	}
	if got.Chart.SVG == "" {
		t.Fatal("expected hydrated chart SVG after mutation")
	}

	// Ensure the chart still belongs to the same worksheet after mutation.
	if sheet.Name() == "" {
		t.Fatal("expected source sheet name")
	}
}

func TestOpenXLSXExistingChartRemovalRoundTrip(t *testing.T) {
	wb := openFixtureWorkbook(t, "sample-2.xlsx")
	sheet, beforeCount := findSheetWithChartCount(t, wb, "Chart")
	if beforeCount != 2 {
		t.Fatalf("existing chart count = %d, want 2", beforeCount)
	}
	if !sheet.RemoveChart("Chart") {
		t.Fatal("RemoveChart returned false")
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	for _, rtSheet := range roundTrip.Sheets {
		count := 0
		for _, drawing := range rtSheet.Drawings {
			if drawing != nil && drawing.Type == sheetir.DrawingTypeChart && drawing.Name == "Chart" {
				count++
			}
		}
		if count > 0 {
			if count != 1 {
				t.Fatalf("chart count after removal = %d, want 1", count)
			}
			return
		}
	}
	t.Fatal("expected one remaining existing chart after removal")
}

func TestSheetChartAdvancedMutationRoundTrip(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	chart := edit.NewChart(chartmodel.ChartColumn, "Advanced")
	chart.Categories = []string{"Jan", "Feb"}
	chart.Series = []chartmodel.Series{
		{
			Index:  0,
			Name:   "Bars",
			Values: []float64{10, 20},
		},
		{
			Index:     1,
			ChartType: chartmodel.ChartLine,
			Name:      "Line",
			Values:    []float64{15, 25},
		},
	}
	drawing, err := sheet.AddChartOneCell("Advanced Chart", chart, 1, 1, 3600000, 2400000)
	if err != nil {
		t.Fatalf("AddChartOneCell: %v", err)
	}
	drawing.SetAxes(
		chartmodel.Axis{Title: "Months", LabelPosition: 1},
		chartmodel.Axis{Title: "Revenue", Min: 0, Max: 30, ShowGridlines: true, MajorUnit: 10},
	)
	drawing.SetSecondaryYAxis(chartmodel.Axis{Title: "Target", Min: 10, Max: 50})
	drawing.SetLegendEntries([]chartmodel.LegendEntry{{Index: 1, Delete: true}})
	drawing.SetChartAreaFill("F3F0E8")
	drawing.SetPlotArea(chartmodel.PlotArea{
		ShowBorder:  true,
		BorderColor: "333333",
		FillColor:   "FFF9E6",
	})
	drawing.SetManualLayout(&chartmodel.ManualLayout{
		LayoutTarget: "inner",
		XMode:        "edge",
		YMode:        "edge",
		WMode:        "factor",
		HMode:        "factor",
		X:            0.1,
		Y:            0.15,
		W:            0.65,
		H:            0.5,
	})
	drawing.SetLegendLayout(&chartmodel.ManualLayout{
		LayoutTarget: "outer",
		XMode:        "factor",
		YMode:        "factor",
		WMode:        "factor",
		HMode:        "factor",
		X:            0.68,
		Y:            0.2,
		W:            0.2,
		H:            0.2,
	})
	drawing.SetTitleLayout(&chartmodel.ManualLayout{
		LayoutTarget: "outer",
		XMode:        "factor",
		YMode:        "factor",
		WMode:        "factor",
		HMode:        "factor",
		X:            0.1,
		Y:            0.02,
		W:            0.7,
		H:            0.08,
	})
	drawing.SetDataTable(&chartmodel.DataTable{
		ShowHorzBorder: true,
		ShowVertBorder: true,
		ShowOutline:    true,
		ShowKeys:       true,
	})
	drawing.ReplaceGroups([]chartmodel.ChartGroup{
		{
			ChartType:     chartmodel.ChartColumn,
			SeriesIndices: []int{0},
			AxisIDs:       []int{1, 2},
			Grouping:      "clustered",
			GapWidth:      200,
		},
		{
			ChartType:     chartmodel.ChartLine,
			SeriesIndices: []int{1},
			AxisIDs:       []int{1, 3},
			Grouping:      "standard",
		},
	})
	if err := drawing.SetSeriesFormulas(0, "Sheet1!$B$1", "Sheet1!$B$2:$B$3", "", "Sheet1!$A$2:$A$3"); err != nil {
		t.Fatalf("SetSeriesFormulas: %v", err)
	}
	if err := drawing.SetSeriesDataLabels(0, true, true, false, false, true, false, 1, " / ", "0.0"); err != nil {
		t.Fatalf("SetSeriesDataLabels: %v", err)
	}
	if err := drawing.SetSeriesTrendline(1, &chartmodel.Trendline{
		Type:         "linear",
		Forward:      1,
		ShowEquation: true,
		ShowRSquared: true,
	}); err != nil {
		t.Fatalf("SetSeriesTrendline: %v", err)
	}
	if err := drawing.SetSeriesErrorBars(1, &chartmodel.ErrorBars{
		Direction: "y",
		BarType:   "both",
		ValType:   "fixedVal",
		Value:     2,
	}); err != nil {
		t.Fatalf("SetSeriesErrorBars: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	if len(roundTrip.Sheets[0].Drawings) != 1 || roundTrip.Sheets[0].Drawings[0].Chart == nil {
		t.Fatalf("drawings = %#v", roundTrip.Sheets[0].Drawings)
	}
	rtChart, ok := roundTrip.Sheets[0].Drawings[0].Chart.Chart.(*chartmodel.Chart)
	if !ok {
		t.Fatalf("chart payload = %T, want *chart.Chart", roundTrip.Sheets[0].Drawings[0].Chart.Chart)
	}
	if rtChart.XAxis.Title != "Months" || rtChart.XAxis.LabelPosition != 1 {
		t.Fatalf("x axis = %#v", rtChart.XAxis)
	}
	if rtChart.YAxis.Title != "Revenue" || rtChart.YAxis.Max != 30 || !rtChart.YAxis.ShowGridlines {
		t.Fatalf("y axis = %#v", rtChart.YAxis)
	}
	if rtChart.YAxis2.Title != "Target" || rtChart.YAxis2.Min != 10 || rtChart.YAxis2.Max != 50 {
		t.Fatalf("y axis 2 = %#v", rtChart.YAxis2)
	}
	if rtChart.ChartArea.FillColor != "F3F0E8" {
		t.Fatalf("chart area = %#v", rtChart.ChartArea)
	}
	if !rtChart.PlotArea.ShowBorder || rtChart.PlotArea.BorderColor != "333333" || rtChart.PlotArea.FillColor != "FFF9E6" {
		t.Fatalf("plot area = %#v", rtChart.PlotArea)
	}
	if rtChart.ManualLayout == nil || rtChart.ManualLayout.LayoutTarget != "inner" || rtChart.ManualLayout.X != 0.1 || rtChart.ManualLayout.H != 0.5 {
		t.Fatalf("manual layout = %#v", rtChart.ManualLayout)
	}
	if rtChart.LegendLayout == nil || rtChart.LegendLayout.X != 0.68 || rtChart.LegendLayout.W != 0.2 {
		t.Fatalf("legend layout = %#v", rtChart.LegendLayout)
	}
	if rtChart.TitleLayout == nil || rtChart.TitleLayout.Y != 0.02 || rtChart.TitleLayout.H != 0.08 {
		t.Fatalf("title layout = %#v", rtChart.TitleLayout)
	}
	if len(rtChart.Legend.Entries) != 1 || rtChart.Legend.Entries[0].Index != 1 || !rtChart.Legend.Entries[0].Delete {
		t.Fatalf("legend entries = %#v", rtChart.Legend.Entries)
	}
	if rtChart.DataTable == nil || !rtChart.DataTable.ShowHorzBorder || !rtChart.DataTable.ShowVertBorder || !rtChart.DataTable.ShowOutline || !rtChart.DataTable.ShowKeys {
		t.Fatalf("data table = %#v", rtChart.DataTable)
	}
	if len(rtChart.Groups) != 2 || rtChart.Groups[0].GapWidth != 200 || len(rtChart.Groups[1].AxisIDs) != 2 || rtChart.Groups[1].AxisIDs[1] != 3 {
		t.Fatalf("groups = %#v", rtChart.Groups)
	}
	if rtChart.Series[0].NameFormula != "Sheet1!$B$1" || rtChart.Series[0].ValueFormula != "Sheet1!$B$2:$B$3" || rtChart.Series[0].LabelFormula != "Sheet1!$A$2:$A$3" {
		t.Fatalf("series formulas = %#v", rtChart.Series[0])
	}
	if !rtChart.Series[0].ShowDataLabels || !rtChart.Series[0].ShowCatName || !rtChart.Series[0].ShowLeaderLines || rtChart.Series[0].LabelPosition != 1 || rtChart.Series[0].Separator != " / " {
		t.Fatalf("series label config = %#v", rtChart.Series[0])
	}
	if rtChart.Series[1].Trendline == nil || rtChart.Series[1].Trendline.Type != "linear" || !rtChart.Series[1].Trendline.ShowEquation || !rtChart.Series[1].Trendline.ShowRSquared {
		t.Fatalf("series trendline = %#v", rtChart.Series[1].Trendline)
	}
	if rtChart.Series[1].ErrorBars == nil || rtChart.Series[1].ErrorBars.Value != 2 || rtChart.Series[1].ErrorBars.ValType != "fixedVal" {
		t.Fatalf("series error bars = %#v", rtChart.Series[1].ErrorBars)
	}
	if roundTrip.Sheets[0].Drawings[0].Chart.SVG == "" {
		t.Fatal("expected hydrated chart SVG after advanced mutation")
	}
}

func TestSheetChartAdvancedMutationValidatesSeriesIndex(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	chart := edit.NewChart(chartmodel.ChartLine, "Single")
	chart.Series = []chartmodel.Series{{Name: "Only", Values: []float64{1, 2}}}
	drawing, err := sheet.AddChartOneCell("Single Chart", chart, 1, 1, 1000, 1000)
	if err != nil {
		t.Fatalf("AddChartOneCell: %v", err)
	}
	if err := drawing.SetSeriesTrendline(2, &chartmodel.Trendline{Type: "linear"}); err == nil {
		t.Fatal("expected out-of-range series index error")
	}
}

func findSheetChart(t *testing.T, wb *edit.Workbook, chartName string) (*edit.Sheet, *edit.ChartDrawing) {
	t.Helper()
	for _, sheet := range wb.Sheets() {
		chart, ok := sheet.Chart(chartName)
		if ok {
			return sheet, chart
		}
	}
	t.Fatalf("chart %q not found", chartName)
	return nil, nil
}

func findSheetWithChartCount(t *testing.T, wb *edit.Workbook, chartName string) (*edit.Sheet, int) {
	t.Helper()
	for _, sheet := range wb.Sheets() {
		count := 0
		for _, drawing := range sheet.Drawings() {
			if drawing != nil && drawing.Type == sheetir.DrawingTypeChart && drawing.Name == chartName {
				count++
			}
		}
		if count > 0 {
			return sheet, count
		}
	}
	t.Fatalf("chart %q not found", chartName)
	return nil, 0
}
