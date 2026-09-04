package chart

import (
	"strings"
	"testing"
)

func TestMarshalChartBar(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Title:      "Sales",
		Categories: []string{"Q1", "Q2", "Q3"},
		Series: []Series{
			{
				Index:  0,
				Name:   "Product A",
				Values: []float64{10, 20, 30},
			},
		},
		Legend: Legend{Show: true, Position: "b"},
	}

	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	xml := string(data)
	t.Logf("XML:\n%s", xml)
	if !strings.Contains(xml, `<chartSpace`) {
		t.Fatalf("missing chartSpace root")
	}
	if !strings.Contains(xml, `<barChart`) {
		t.Fatalf("missing barChart")
	}
	if !strings.Contains(xml, "barDir") || !strings.Contains(xml, `val="col"`) {
		t.Fatalf("missing barDir")
	}
	if !strings.Contains(xml, `<catAx`) {
		t.Fatalf("missing catAx")
	}
	if !strings.Contains(xml, `<valAx`) {
		t.Fatalf("missing valAx")
	}
	if !strings.Contains(xml, "Sales") {
		t.Fatalf("missing title")
	}
	if !strings.Contains(xml, "Product A") {
		t.Fatalf("missing series name")
	}

	// Verify it can be parsed back.
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if parsed.ChartType != ChartColumn {
		t.Fatalf("type = %v, want ChartColumn", parsed.ChartType)
	}
	if parsed.Title != "Sales" {
		t.Fatalf("title = %q, want Sales", parsed.Title)
	}
	if len(parsed.Series) != 1 {
		t.Fatalf("series = %d, want 1", len(parsed.Series))
	}
	if len(parsed.Categories) != 3 {
		t.Fatalf("categories = %d, want 3", len(parsed.Categories))
	}
}

func TestMarshalChartPie(t *testing.T) {
	c := &Chart{
		ChartType:  ChartPie,
		Title:      "Market Share",
		Categories: []string{"A", "B", "C"},
		Series: []Series{
			{
				Index:  0,
				Name:   "Share",
				Values: []float64{30, 50, 20},
			},
		},
	}

	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	xml := string(data)
	if !strings.Contains(xml, `<pieChart`) {
		t.Fatalf("missing pieChart")
	}
	if strings.Contains(xml, `<catAx`) {
		t.Fatalf("pie chart should not have catAx")
	}

	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if parsed.ChartType != ChartPie {
		t.Fatalf("type = %v, want ChartPie", parsed.ChartType)
	}
}

func TestMarshalChartLine(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Title:      "Trend",
		Categories: []string{"Jan", "Feb", "Mar"},
		Series: []Series{
			{
				Index:  0,
				Name:   "Revenue",
				Values: []float64{100, 150, 120},
			},
		},
	}

	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	xml := string(data)
	if !strings.Contains(xml, `<lineChart`) {
		t.Fatalf("missing lineChart")
	}

	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if parsed.ChartType != ChartLine {
		t.Fatalf("type = %v, want ChartLine", parsed.ChartType)
	}
}

func TestMarshalChartStackedBar(t *testing.T) {
	c := &Chart{
		ChartType: ChartBar,
		Stacked:   true,
		Series: []Series{
			{Index: 0, Name: "A", Values: []float64{10, 20}},
			{Index: 1, Name: "B", Values: []float64{15, 25}},
		},
	}

	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !strings.Contains(string(data), "grouping") || !strings.Contains(string(data), `val="stacked"`) {
		t.Fatalf("missing stacked grouping")
	}
}

func TestMarshalChartSeriesColors(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"Q1", "Q2"},
		Series: []Series{
			{
				Index:     0,
				Name:      "A",
				Values:    []float64{10, 20},
				FillColor: "FF0000",
				LineColor: "00FF00",
			},
		},
	}

	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	xml := string(data)
	if !strings.Contains(xml, `<spPr`) {
		t.Fatalf("missing spPr")
	}
	if !strings.Contains(xml, `val="FF0000"`) {
		t.Fatalf("missing fill color FF0000")
	}
	if !strings.Contains(xml, `val="00FF00"`) {
		t.Fatalf("missing line color 00FF00")
	}
}

func TestMarshalChartDoughnut(t *testing.T) {
	c := &Chart{
		ChartType:  ChartDoughnut,
		Title:      "Shares",
		Categories: []string{"A", "B", "C"},
		Series: []Series{
			{Index: 0, Name: "Share", Values: []float64{30, 50, 20}},
		},
	}

	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	xml := string(data)
	if !strings.Contains(xml, `<doughnutChart`) {
		t.Fatalf("missing doughnutChart")
	}
	if strings.Contains(xml, `<catAx`) {
		t.Fatalf("doughnut chart should not have catAx")
	}
}

func TestMarshalChartDataLabels(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"Q1", "Q2"},
		Series: []Series{
			{Index: 0, Name: "A", Values: []float64{10, 20}, ShowDataLabels: true},
		},
	}

	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	xml := string(data)
	if !strings.Contains(xml, `<dLbls`) {
		t.Fatalf("missing dLbls")
	}
	if !strings.Contains(xml, `showVal`) {
		t.Fatalf("missing showVal")
	}
}

func TestMarshalChartAxisCustomization(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"Q1", "Q2"},
		Series: []Series{
			{Index: 0, Name: "A", Values: []float64{10, 20}},
		},
		XAxis: Axis{Min: 1, Max: 5},
		YAxis: Axis{Min: 10, Max: 100, ShowGridlines: true},
	}

	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	xml := string(data)
	if !strings.Contains(xml, `<min val="1"`) {
		t.Fatalf("missing x-axis min scaling")
	}
	if !strings.Contains(xml, `<max val="5"`) {
		t.Fatalf("missing x-axis max scaling")
	}
	if !strings.Contains(xml, `<min val="10"`) {
		t.Fatalf("missing y-axis min scaling")
	}
	if !strings.Contains(xml, `<max val="100"`) {
		t.Fatalf("missing y-axis max scaling")
	}
	if !strings.Contains(xml, `<majorGridlines`) {
		t.Fatalf("missing majorGridlines")
	}

	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if !parsed.YAxis.ShowGridlines {
		t.Fatal("expected y-axis gridlines after round-trip")
	}
}

func TestMarshalChartErrorBars(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Name:   "S1",
			Values: []float64{10, 20},
			ErrorBars: &ErrorBars{
				Direction: "y",
				BarType:   "both",
				ValType:   "fixedVal",
				Value:     5,
				NoEndCap:  true,
			},
		}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `<errBars`) {
		t.Fatal("missing errBars")
	}
	if !strings.Contains(xml, `errDir`) {
		t.Fatal("missing errDir")
	}
	if !strings.Contains(xml, `errBarType`) {
		t.Fatal("missing errBarType")
	}
	if !strings.Contains(xml, `errValType`) {
		t.Fatal("missing errValType")
	}
	if !strings.Contains(xml, `noEndCap`) {
		t.Fatal("missing noEndCap")
	}
	if !strings.Contains(xml, `val="5"`) || !strings.Contains(xml, `</errBars>`) {
		t.Fatal("missing error bar val")
	}

	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	eb := parsed.Series[0].ErrorBars
	if eb == nil {
		t.Fatal("expected error bars after round-trip")
	}
	if eb.Value != 5 {
		t.Fatalf("value = %v, want 5", eb.Value)
	}
	if !eb.NoEndCap {
		t.Fatal("expected noEndCap after round-trip")
	}
}

func TestMarshalChartErrorBarsCustom(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Name:   "S1",
			Values: []float64{10, 20},
			ErrorBars: &ErrorBars{
				ValType: "cust",
				Plus:    []float64{1, 2},
				Minus:   []float64{3, 4},
			},
		}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `<plus`) {
		t.Fatal("missing plus")
	}
	if !strings.Contains(xml, `<minus`) {
		t.Fatal("missing minus")
	}

	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	eb := parsed.Series[0].ErrorBars
	if len(eb.Plus) != 2 || eb.Plus[0] != 1 {
		t.Fatalf("plus = %v", eb.Plus)
	}
	if len(eb.Minus) != 2 || eb.Minus[0] != 3 {
		t.Fatalf("minus = %v", eb.Minus)
	}
}

func TestMarshalChartDataTable(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		Series:     []Series{{Name: "S1", Values: []float64{10, 20}}},
		DataTable: &DataTable{
			ShowHorzBorder: true,
			ShowVertBorder: true,
			ShowOutline:    true,
			ShowKeys:       true,
		},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `<dTable`) {
		t.Fatal("missing dTable")
	}
	if !strings.Contains(xml, `showHorzBorder`) {
		t.Fatal("missing showHorzBorder")
	}
	if !strings.Contains(xml, `showVertBorder`) {
		t.Fatal("missing showVertBorder")
	}
	if !strings.Contains(xml, `showOutline`) {
		t.Fatal("missing showOutline")
	}
	if !strings.Contains(xml, `showKeys`) {
		t.Fatal("missing showKeys")
	}

	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if parsed.DataTable == nil {
		t.Fatal("expected data table after round-trip")
	}
	if !parsed.DataTable.ShowHorzBorder {
		t.Fatal("expected showHorzBorder after round-trip")
	}
}

func TestMarshalChartTrendline(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 30},
			Trendline: &Trendline{
				Type:         "linear",
				Order:        2,
				Period:       3,
				Forward:      1,
				Backward:     0.5,
				ShowEquation: true,
				ShowRSquared: true,
			},
		}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `<trendline`) {
		t.Fatalf("missing trendline element, got:\n%s", xml)
	}
	if !strings.Contains(xml, `trendlineType`) {
		t.Fatal("missing trendlineType")
	}
	if !strings.Contains(xml, `dispEq`) {
		t.Fatal("missing dispEq")
	}
	if !strings.Contains(xml, `dispRSqr`) {
		t.Fatal("missing dispRSqr")
	}

	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if len(parsed.Series) != 1 {
		t.Fatalf("series = %d, want 1", len(parsed.Series))
	}
	trend := parsed.Series[0].Trendline
	if trend == nil {
		t.Fatal("expected trendline after round-trip")
	}
	if trend.Type != "linear" {
		t.Fatalf("type = %q, want linear", trend.Type)
	}
	if trend.Order != 2 {
		t.Fatalf("order = %v, want 2", trend.Order)
	}
	if trend.Period != 3 {
		t.Fatalf("period = %v, want 3", trend.Period)
	}
	if trend.Forward != 1.0 {
		t.Fatalf("forward = %v, want 1.0", trend.Forward)
	}
	if trend.Backward != 0.5 {
		t.Fatalf("backward = %v, want 0.5", trend.Backward)
	}
	if !trend.ShowEquation {
		t.Fatal("expected ShowEquation after round-trip")
	}
	if !trend.ShowRSquared {
		t.Fatal("expected ShowRSquared after round-trip")
	}
}

func TestMarshalChartMarkerSize(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:       0,
			Values:      []float64{10, 20},
			MarkerSize:  7,
			MarkerShape: "diamond",
		}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `<marker`) {
		t.Fatal("missing marker element")
	}
	if !strings.Contains(xml, `<size`) {
		t.Fatal("missing size element")
	}
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if parsed.Series[0].MarkerSize != 7 {
		t.Fatalf("markerSize = %v, want 7", parsed.Series[0].MarkerSize)
	}
	if parsed.Series[0].MarkerShape != "diamond" {
		t.Fatalf("markerShape = %q, want diamond", parsed.Series[0].MarkerShape)
	}
}

func TestMarshalChartDisplayBlanksAs(t *testing.T) {
	c := &Chart{
		ChartType:       ChartColumn,
		Categories:      []string{"A"},
		Series:          []Series{{Values: []float64{10}}},
		DisplayBlanksAs: "zero",
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `displayBlanksAs`) {
		t.Fatalf("missing displayBlanksAs, got:\n%s", xml)
	}
	if !strings.Contains(xml, `val="zero"`) {
		t.Fatalf("missing zero value, got:\n%s", xml)
	}
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if parsed.DisplayBlanksAs != "zero" {
		t.Fatalf("displayBlanksAs = %q, want zero", parsed.DisplayBlanksAs)
	}
}

func TestMarshalChartLeaderLines(t *testing.T) {
	c := &Chart{
		ChartType:  ChartPie,
		Categories: []string{"A"},
		Series: []Series{{
			Values:          []float64{10},
			ShowDataLabels:  true,
			ShowLeaderLines: true,
		}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `showLeaderLines`) {
		t.Fatalf("missing showLeaderLines, got:\n%s", xml)
	}
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if !parsed.Series[0].ShowLeaderLines {
		t.Fatal("expected ShowLeaderLines after round-trip")
	}
}

func TestMarshalScatterStyle(t *testing.T) {
	c := &Chart{
		ChartType: ChartScatter,
		Series: []Series{{
			Values:       []float64{10, 20},
			ScatterStyle: "marker",
		}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `scatterStyle`) {
		t.Fatalf("missing scatterStyle, got:\n%s", xml)
	}
	if !strings.Contains(xml, `"marker"`) {
		t.Fatalf("expected marker style, got:\n%s", xml)
	}
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if parsed.Series[0].ScatterStyle != "marker" {
		t.Errorf("scatterStyle = %q, want marker", parsed.Series[0].ScatterStyle)
	}
}

func TestMarshalRadarStyle(t *testing.T) {
	c := &Chart{
		ChartType:  ChartRadar,
		RadarStyle: "filled",
		Series:     []Series{{Values: []float64{10, 20}}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `radarStyle`) {
		t.Fatalf("missing radarStyle, got:\n%s", xml)
	}
	if !strings.Contains(xml, `"filled"`) {
		t.Fatalf("expected filled style, got:\n%s", xml)
	}
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if len(parsed.Groups) != 1 || parsed.Groups[0].Grouping != "filled" {
		t.Errorf("radar style = %q, want filled", parsed.Groups[0].Grouping)
	}
}

func TestMarshalDataLabelNumFmt(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A"},
		Series: []Series{{
			Values:         []float64{10},
			ShowDataLabels: true,
			NumFmt:         "0.00",
			DataPoints: []DataPoint{{
				Index:  0,
				NumFmt: "0.000",
			}},
		}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `numFmt`) {
		t.Fatalf("missing numFmt, got:\n%s", xml)
	}
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	s := parsed.Series[0]
	if s.NumFmt != "0.00" {
		t.Errorf("series numFmt = %q, want 0.00", s.NumFmt)
	}
	if len(s.DataPoints) == 0 || s.DataPoints[0].NumFmt != "0.000" {
		t.Errorf("dataPoint numFmt = %q, want 0.000", s.DataPoints[0].NumFmt)
	}
}

func TestMarshalDataLabelSurface(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A"},
		Series: []Series{{
			Values:         []float64{10},
			ShowDataLabels: true,
			ShowCatName:    true,
			ShowBubbleSize: true,
			Separator:      " | ",
			LabelPosition:  1,
			DataPoints: []DataPoint{{
				Index: 0,
				Label: "Custom",
			}},
		}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `separator`) {
		t.Fatalf("missing separator, got:\n%s", xml)
	}
	if !strings.Contains(xml, `dLblPos`) {
		t.Fatalf("missing dLblPos, got:\n%s", xml)
	}
	if !strings.Contains(xml, `showBubbleSize`) {
		t.Fatalf("missing showBubbleSize, got:\n%s", xml)
	}
	if !strings.Contains(xml, `dLbl`) {
		t.Fatalf("missing dLbl, got:\n%s", xml)
	}
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	s := parsed.Series[0]
	if s.Separator != " | " {
		t.Errorf("separator = %q, want %q", s.Separator, " | ")
	}
	if s.LabelPosition != 1 {
		t.Errorf("labelPosition = %d, want 1", s.LabelPosition)
	}
	if !s.ShowBubbleSize {
		t.Error("expected ShowBubbleSize after round-trip")
	}
	if len(s.DataPoints) == 0 || s.DataPoints[0].Label != "Custom" {
		t.Errorf("custom label = %q, want %q", s.DataPoints[0].Label, "Custom")
	}
}

func TestMarshalComboChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:     0,
			ChartType: ChartColumn,
			Name:      "Bars",
			Values:    []float64{10, 20},
		}, {
			Index:     1,
			ChartType: ChartLine,
			Name:      "Line",
			Values:    []float64{15, 25},
		}},
		Groups: []ChartGroup{{
			ChartType:     ChartColumn,
			SeriesIndices: []int{0},
			AxisIDs:       []int{1, 2},
			Grouping:      "clustered",
			GapWidth:      200,
		}, {
			ChartType:     ChartLine,
			SeriesIndices: []int{1},
			AxisIDs:       []int{1, 2},
			Grouping:      "standard",
		}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `barChart`) {
		t.Fatalf("missing barChart, got:\n%s", xml)
	}
	if !strings.Contains(xml, `lineChart`) {
		t.Fatalf("missing lineChart, got:\n%s", xml)
	}
	if !strings.Contains(xml, `gapWidth`) {
		t.Fatalf("missing gapWidth, got:\n%s", xml)
	}
	// Verify bar group has only the bar series.
	// Simple heuristic: count ser elements.
	if strings.Count(xml, `<ser>`) != 2 {
		t.Fatalf("expected 2 ser elements total, got %d:\n%s", strings.Count(xml, `<ser>`), xml)
	}
	// Round-trip parse back.
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if len(parsed.Groups) != 2 {
		t.Fatalf("expected 2 groups after round-trip, got %d", len(parsed.Groups))
	}
}

func TestMarshalChartStockConfig(t *testing.T) {
	c := &Chart{
		ChartType: ChartStock,
		Series:    []Series{{Values: []float64{10, 20, 15, 25}}},
		StockConfig: &StockConfig{
			UpBarColor:     "00FF00",
			DownBarColor:   "FF0000",
			ShowDropLines:  true,
			ShowHiLowLines: true,
		},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `<stockChart`) {
		t.Fatalf("missing stockChart, got:\n%s", xml)
	}
	if !strings.Contains(xml, `<dropLines`) {
		t.Fatal("missing dropLines")
	}
	if !strings.Contains(xml, `<hiLowLines`) {
		t.Fatal("missing hiLowLines")
	}
	if !strings.Contains(xml, `<upDownBars`) {
		t.Fatal("missing upDownBars")
	}
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if parsed.StockConfig == nil {
		t.Fatal("expected stockConfig after round-trip")
	}
	if parsed.StockConfig.UpBarColor != "00FF00" {
		t.Fatalf("upBarColor = %q, want 00FF00", parsed.StockConfig.UpBarColor)
	}
}

func TestMarshalChartLegendEntries(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A"},
		Series:     []Series{{Name: "S1", Values: []float64{10}}},
		Legend:     Legend{Show: true, Position: "b", Entries: []LegendEntry{{Index: 0, Delete: true}}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `<legendEntry`) {
		t.Fatalf("missing legendEntry, got:\n%s", xml)
	}
	if !strings.Contains(xml, `delete`) {
		t.Fatal("missing delete")
	}
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if len(parsed.Legend.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(parsed.Legend.Entries))
	}
	if !parsed.Legend.Entries[0].Delete {
		t.Fatal("expected delete after round-trip")
	}
}

func TestMarshalChartManualLayout(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A"},
		Series:     []Series{{Values: []float64{10}}},
		ManualLayout: &ManualLayout{
			LayoutTarget: "inner",
			XMode:        "edge",
			YMode:        "edge",
			X:            0.1,
			Y:            0.2,
			W:            0.8,
			H:            0.7,
		},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `<manualLayout`) {
		t.Fatalf("missing manualLayout, got:\n%s", xml)
	}
	if !strings.Contains(xml, `layoutTarget`) {
		t.Fatal("missing layoutTarget")
	}
	if !strings.Contains(xml, `xMode`) {
		t.Fatal("missing xMode")
	}
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if parsed.ManualLayout == nil {
		t.Fatal("expected manualLayout after round-trip")
	}
	if parsed.ManualLayout.LayoutTarget != "inner" {
		t.Fatalf("layoutTarget = %q, want inner", parsed.ManualLayout.LayoutTarget)
	}
	if parsed.ManualLayout.X != 0.1 {
		t.Fatalf("x = %v, want 0.1", parsed.ManualLayout.X)
	}
	if parsed.ManualLayout.H != 0.7 {
		t.Fatalf("h = %v, want 0.7", parsed.ManualLayout.H)
	}
}

func TestMarshalChartTitleLayout(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A"},
		Series:     []Series{{Values: []float64{10}}},
		Title:      "Revenue",
		TitleLayout: &ManualLayout{
			LayoutTarget: "outer",
			XMode:        "factor",
			YMode:        "factor",
			X:            0.1,
			Y:            0.05,
			W:            0.8,
			H:            0.1,
		},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `<title`) {
		t.Fatalf("missing title element, got:\n%s", xml)
	}
	if !strings.Contains(xml, `<layout`) {
		t.Fatalf("missing title layout, got:\n%s", xml)
	}
	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if parsed.Title != "Revenue" {
		t.Fatalf("title = %q, want Revenue", parsed.Title)
	}
	if parsed.TitleLayout == nil {
		t.Fatal("expected titleLayout after round-trip")
	}
	if parsed.TitleLayout.X != 0.1 {
		t.Fatalf("title x = %v, want 0.1", parsed.TitleLayout.X)
	}
	if parsed.TitleLayout.W != 0.8 {
		t.Fatalf("title w = %v, want 0.8", parsed.TitleLayout.W)
	}
}

func TestMarshalChartDataLabelTextProperties(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:          0,
			Values:         []float64{10, 20},
			ShowDataLabels: true,
			LabelStyle: &TextProperties{
				FontSize: 1200,
				Bold:     true,
				Italic:   true,
				Color:    "FF0000",
			},
			DataPoints: []DataPoint{
				{Index: 1, LabelStyle: &TextProperties{FontSize: 900, Bold: false, Italic: false, Color: "00FF00"}},
			},
		}},
	}
	data, err := MarshalChart(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	xml := string(data)
	if !strings.Contains(xml, `<txPr`) {
		t.Fatalf("missing txPr, got:\n%s", xml)
	}
	if !strings.Contains(xml, `sz="1200"`) {
		t.Fatalf("missing sz=1200, got:\n%s", xml)
	}
	if !strings.Contains(xml, `b="true"`) {
		t.Fatalf("missing b=true, got:\n%s", xml)
	}
	if !strings.Contains(xml, `i="true"`) {
		t.Fatalf("missing i=true, got:\n%s", xml)
	}
	if !strings.Contains(xml, `val="FF0000"`) {
		t.Fatalf("missing red color, got:\n%s", xml)
	}
	if !strings.Contains(xml, `val="00FF00"`) {
		t.Fatalf("missing green color, got:\n%s", xml)
	}

	parsed, err := ParseChart(data)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	s := parsed.Series[0]
	if s.LabelStyle == nil {
		t.Fatal("expected series LabelStyle after round-trip")
	}
	if s.LabelStyle.FontSize != 1200 {
		t.Fatalf("series fontSize = %d, want 1200", s.LabelStyle.FontSize)
	}
	if !s.LabelStyle.Bold || !s.LabelStyle.Italic {
		t.Fatal("expected bold and italic after round-trip")
	}
	if s.LabelStyle.Color != "FF0000" {
		t.Fatalf("series color = %q, want FF0000", s.LabelStyle.Color)
	}
	if len(s.DataPoints) < 2 || s.DataPoints[1].LabelStyle == nil {
		t.Fatal("expected per-point LabelStyle after round-trip")
	}
	dp := s.DataPoints[1]
	if dp.LabelStyle.FontSize != 900 {
		t.Fatalf("point fontSize = %d, want 900", dp.LabelStyle.FontSize)
	}
	if dp.LabelStyle.Color != "00FF00" {
		t.Fatalf("point color = %q, want 00FF00", dp.LabelStyle.Color)
	}
}
