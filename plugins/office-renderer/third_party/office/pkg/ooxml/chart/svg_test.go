package chart

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
)

func TestSVGBarChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 30},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<rect`) {
		t.Fatal("expected bars")
	}
	if !strings.Contains(svg, ">A<") {
		t.Fatal("expected category A")
	}
}

func TestSVGPieChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartPie,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{30, 50, 20},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<path d="M`) {
		t.Fatal("expected pie slices")
	}
}

func TestSVGDoughnutChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartDoughnut,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{30, 50, 20},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `A`) {
		t.Fatal("expected doughnut arcs")
	}
}

func TestSVGDoughnutChartWithHoleSize(t *testing.T) {
	c := &Chart{
		ChartType:  ChartDoughnut,
		Categories: []string{"A", "B", "C"},
		HoleSize:   75,
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{30, 50, 20},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `A`) {
		t.Fatal("expected doughnut arcs")
	}
}

func TestSVGBarChartWithGapWidthAndOverlap(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B", "C"},
		GapWidth:   200,
		Overlap:    50,
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 30},
		}, {
			Index:  1,
			Name:   "S2",
			Values: []float64{15, 25, 35},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<rect`) {
		t.Fatal("expected bars")
	}
}

func TestSVGLineChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 30},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `fill="none"`) {
		t.Fatal("expected line path")
	}
}

func TestSVGAreaChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartArea,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 30},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `fill-opacity="0.3"`) {
		t.Fatal("expected area fill")
	}
}

func TestSVGScatterChart(t *testing.T) {
	c := &Chart{
		ChartType: ChartScatter,
		Series: []Series{{
			Index:   0,
			Name:    "S1",
			Values:  []float64{10, 20, 30},
			XValues: []float64{1, 2, 3},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<circle`) {
		t.Fatal("expected scatter markers")
	}
}

func TestSVGScatterMarkerStyleDoesNotDrawConnectingLine(t *testing.T) {
	c := &Chart{
		ChartType: ChartScatter,
		Series: []Series{{
			Index:        0,
			Name:         "S1",
			Values:       []float64{10, 20, 30},
			XValues:      []float64{1, 2, 3},
			MarkerShape:  "circle",
			ScatterStyle: "marker",
		}},
	}
	svg := c.SVG(400, 300)
	if strings.Contains(svg, `fill="none" stroke="#`) {
		t.Fatalf("expected marker-only scatter without connecting line, got:\n%s", svg)
	}
	if !strings.Contains(svg, `<circle`) {
		t.Fatalf("expected scatter markers, got:\n%s", svg)
	}
}

func TestSVGBubbleChart(t *testing.T) {
	c := &Chart{
		ChartType: ChartBubble,
		Series: []Series{{
			Index:       0,
			Name:        "S1",
			Values:      []float64{10, 20, 30},
			XValues:     []float64{1, 2, 3},
			BubbleSizes: []float64{10, 20, 30},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `fill-opacity="0.6"`) {
		t.Fatal("expected bubble circles")
	}
}

func TestSVGStockChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartStock,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "Open",
			Values: []float64{10, 15, 20},
		}, {
			Index:  1,
			Name:   "High",
			Values: []float64{20, 25, 30},
		}, {
			Index:  2,
			Name:   "Low",
			Values: []float64{5, 10, 15},
		}, {
			Index:  3,
			Name:   "Close",
			Values: []float64{15, 20, 25},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<rect`) {
		t.Fatal("expected candlestick bodies")
	}
}

func TestSVGStockChartThreeSeriesDoesNotInventCandles(t *testing.T) {
	c := &Chart{
		ChartType:  ChartStock,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:  0,
			Name:   "High",
			Values: []float64{20, 25},
		}, {
			Index:  1,
			Name:   "Low",
			Values: []float64{5, 10},
		}, {
			Index:  2,
			Name:   "Close",
			Values: []float64{15, 20},
		}},
	}
	svg := c.SVG(400, 300)
	if strings.Contains(svg, `fill="#70AD47"`) || strings.Contains(svg, `fill="#DC3912"`) {
		t.Fatalf("expected HLC rendering without fabricated candle bodies, got:\n%s", svg)
	}
}

func TestSVGRadarChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartRadar,
		Categories: []string{"A", "B", "C", "D"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 30, 25},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<path`) {
		t.Fatal("expected radar path")
	}
}

func TestSVGSurfaceChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartSurface,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 30},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<rect`) {
		t.Fatal("expected surface heatmap cells")
	}
}

func TestSVGStackedBarChart(t *testing.T) {
	c := &Chart{
		ChartType: ChartColumn,
		Stacked:   true,
		Series: []Series{{
			Index:       0,
			Name:        "A",
			Values:      []float64{10, 20},
			ShowPercent: true,
		}, {
			Index:       1,
			Name:        "B",
			Values:      []float64{15, 25},
			ShowPercent: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<rect`) {
		t.Fatal("expected stacked bars")
	}
}

func TestSVGPercentStackedBarChart(t *testing.T) {
	c := &Chart{
		ChartType:      ChartColumn,
		Stacked:        true,
		PercentStacked: true,
		Series: []Series{{
			Index:       0,
			Name:        "A",
			Values:      []float64{10, 20},
			ShowPercent: true,
		}, {
			Index:       1,
			Name:        "B",
			Values:      []float64{15, 25},
			ShowPercent: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `%`) {
		t.Fatal("expected percent labels")
	}
}

func TestSVGComboChart(t *testing.T) {
	c := &Chart{
		ChartType: ChartColumn,
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
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<rect`) {
		t.Fatal("expected bars")
	}
	if !strings.Contains(svg, `fill="none"`) {
		t.Fatal("expected line overlay")
	}
}

func TestSVGAreaLineComboChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartArea,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:     0,
			ChartType: ChartArea,
			Name:      "Area",
			Values:    []float64{10, 20, 15},
		}, {
			Index:     1,
			ChartType: ChartLine,
			Name:      "Line",
			Values:    []float64{30, 25, 35},
		}},
	}
	svg := c.SVG(400, 300)
	if strings.Count(svg, `fill-opacity="0.3"`) != 1 {
		t.Fatalf("expected exactly one area fill, got:\n%s", svg)
	}
	if !strings.Contains(svg, `fill="none"`) {
		t.Fatalf("expected line series to stay a line, got:\n%s", svg)
	}
}

func TestSVGAreaLineComboChartStartsAreaVerticallyAtFirstCategory(t *testing.T) {
	c := &Chart{
		ChartType:  ChartArea,
		Categories: []string{"Jan", "Feb", "Mar"},
		Series: []Series{{
			Index:     0,
			ChartType: ChartArea,
			Name:      "Area",
			Values:    []float64{50, 50, 50},
		}, {
			Index:     1,
			ChartType: ChartLine,
			Name:      "Line",
			Values:    []float64{0, 0, 0},
		}},
	}
	svg := c.SVG(1253, 302)
	if !strings.Contains(svg, `M288,202 L288,60`) {
		t.Fatalf("expected combo area to rise vertically from the first category, got:\n%s", svg)
	}
	if strings.Contains(svg, `M110,202 L288,60`) {
		t.Fatalf("expected combo area not to start diagonally from the plot edge, got:\n%s", svg)
	}
}

func TestSVGTitle(t *testing.T) {
	c := &Chart{
		ChartType: ChartColumn,
		Title:     "My Title",
		Series:    []Series{{Values: []float64{10}}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, ">My Title<") {
		t.Fatal("expected title")
	}
}

func TestSVGLegendPositions(t *testing.T) {
	positions := []string{"bottom", "top", "left", "right"}
	for _, pos := range positions {
		c := &Chart{
			ChartType: ChartColumn,
			Series:    []Series{{Name: "S1", Values: []float64{10}}},
			Legend:    Legend{Show: true, Position: pos},
		}
		svg := c.SVG(400, 300)
		if !strings.Contains(svg, ">S1<") {
			t.Fatalf("position %q: expected legend", pos)
		}
	}
}

func TestSVGManualLayoutInnerPlotArea(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		PlotArea:   PlotArea{ShowBorder: true, BorderColor: "FF0000"},
		ManualLayout: &ManualLayout{
			LayoutTarget: "inner",
			XMode:        "edge",
			YMode:        "edge",
			WMode:        "factor",
			HMode:        "factor",
			X:            0.1,
			Y:            0.1,
			W:            0.6,
			H:            0.5,
		},
		Series: []Series{{Name: "S1", Values: []float64{10, 20}}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `x="138" y="78" width="108" height="50"`) {
		t.Fatalf("expected manual inner plot area rect in SVG, got:\n%s", svg)
	}
}

func TestSVGLegendLayoutManual(t *testing.T) {
	c := &Chart{
		ChartType: ChartColumn,
		Series:    []Series{{Name: "S1", Values: []float64{10}}, {Name: "S2", Values: []float64{20}}},
		Legend:    Legend{Show: true, Position: "bottom"},
		LegendLayout: &ManualLayout{
			LayoutTarget: "outer",
			XMode:        "factor",
			YMode:        "factor",
			WMode:        "factor",
			HMode:        "factor",
			X:            0.5,
			Y:            0.6,
			W:            0.2,
			H:            0.2,
		},
	}
	svg := c.SVG(400, 300)
	// Legend box starts at (200,180) with 5px padding, so first item is at (205,185)
	if !strings.Contains(svg, `x="205" y="185" width="12" height="12" fill="#4472C4"`) {
		t.Fatalf("expected manual legend position in SVG, got:\n%s", svg)
	}
	if !strings.Contains(svg, `x="205" y="205" width="12" height="12" fill="#ED7D31"`) {
		t.Fatalf("expected second manual legend item in SVG, got:\n%s", svg)
	}
}

func TestSVGManualLayoutOuterPlotArea(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		PlotArea:   PlotArea{ShowBorder: true, BorderColor: "FF0000"},
		ManualLayout: &ManualLayout{
			LayoutTarget: "outer",
			XMode:        "factor",
			YMode:        "factor",
			WMode:        "factor",
			HMode:        "factor",
			X:            0.2,
			Y:            0.2,
			W:            0.5,
			H:            0.4,
		},
		Series: []Series{{Name: "S1", Values: []float64{10, 20}}},
	}
	svg := c.SVG(400, 300)
	// Outer layout on full canvas: x=80, y=60, w=200, h=120
	// After renderAxes offset: plotX=130, plotY=60, plotW=140, plotH=80
	if !strings.Contains(svg, `x="130" y="60" width="140" height="80"`) {
		t.Fatalf("expected manual outer plot area rect in SVG, got:\n%s", svg)
	}
}

func TestSVGLegendLayoutInner(t *testing.T) {
	c := &Chart{
		ChartType: ChartColumn,
		Series:    []Series{{Name: "S1", Values: []float64{10}}, {Name: "S2", Values: []float64{20}}},
		Legend:    Legend{Show: true, Position: "right"},
		LegendLayout: &ManualLayout{
			LayoutTarget: "inner",
			XMode:        "edge",
			YMode:        "edge",
			WMode:        "factor",
			HMode:        "factor",
			X:            0.1,
			Y:            0.1,
			W:            0.3,
			H:            0.5,
		},
	}
	svg := c.SVG(400, 300)
	// Inner legend relative to default plot area (60,60,220,180) with right legend space.
	// Base: chartX=60, chartY=60, chartW=200, chartH=180 (80px reserved for right legend)
	// legendX = 60 + round(0.1*200) = 80, legendY = 60 + round(0.1*180) = 78
	// With 5px padding, first item at (85, 83)
	if !strings.Contains(svg, `x="85" y="83" width="12" height="12" fill="#4472C4"`) {
		t.Fatalf("expected inner legend layout in SVG, got:\n%s", svg)
	}
}

func TestSVGManualLayoutBarChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartBar,
		Categories: []string{"A", "B"},
		PlotArea:   PlotArea{ShowBorder: true, BorderColor: "0000FF"},
		ManualLayout: &ManualLayout{
			LayoutTarget: "inner",
			XMode:        "edge",
			YMode:        "edge",
			WMode:        "factor",
			HMode:        "factor",
			X:            0.05,
			Y:            0.05,
			W:            0.7,
			H:            0.6,
		},
		Series: []Series{{Name: "S1", Values: []float64{10, 20}}},
	}
	svg := c.SVG(400, 300)
	// Should render bars inside manually positioned plot area
	if !strings.Contains(svg, `stroke="#0000FF"`) {
		t.Fatalf("expected manual layout bar chart border in SVG, got:\n%s", svg)
	}
	if !strings.Contains(svg, ">A<") || !strings.Contains(svg, ">B<") {
		t.Fatalf("expected category labels in SVG, got:\n%s", svg)
	}
}

func TestSVGManualLayoutPieChart(t *testing.T) {
	c := &Chart{
		ChartType:  ChartPie,
		Categories: []string{"A", "B"},
		ManualLayout: &ManualLayout{
			LayoutTarget: "outer",
			XMode:        "factor",
			YMode:        "factor",
			WMode:        "factor",
			HMode:        "factor",
			X:            0.1,
			Y:            0.1,
			W:            0.6,
			H:            0.6,
		},
		Series: []Series{{Name: "S1", Values: []float64{30, 70}}},
	}
	svg := c.SVG(400, 300)
	// Pie should be drawn in the manually positioned area
	if !strings.Contains(svg, `<path d="M`) {
		t.Fatalf("expected pie slices in SVG, got:\n%s", svg)
	}
}

func TestSVGCombinedManualAndLegendLayout(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		PlotArea:   PlotArea{ShowBorder: true, BorderColor: "008000"},
		ManualLayout: &ManualLayout{
			LayoutTarget: "inner",
			XMode:        "edge",
			YMode:        "edge",
			WMode:        "factor",
			HMode:        "factor",
			X:            0.1,
			Y:            0.1,
			W:            0.5,
			H:            0.5,
		},
		Legend: Legend{Show: true, Position: "bottom"},
		LegendLayout: &ManualLayout{
			LayoutTarget: "outer",
			XMode:        "factor",
			YMode:        "factor",
			WMode:        "factor",
			HMode:        "factor",
			X:            0.1,
			Y:            0.75,
			W:            0.3,
			H:            0.15,
		},
		Series: []Series{{Name: "S1", Values: []float64{10, 20}}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `stroke="#008000"`) {
		t.Fatalf("expected plot area border in SVG, got:\n%s", svg)
	}
	if !strings.Contains(svg, ">S1<") {
		t.Fatalf("expected legend in SVG, got:\n%s", svg)
	}
}

func TestSVGTitleLayoutManual(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A"},
		Title:      "Revenue",
		TitleLayout: &ManualLayout{
			LayoutTarget: "outer",
			XMode:        "factor",
			YMode:        "factor",
			WMode:        "factor",
			HMode:        "factor",
			X:            0.1,
			Y:            0.05,
			W:            0.8,
			H:            0.1,
		},
		Series: []Series{{Values: []float64{10}}},
	}
	svg := c.SVG(400, 300)
	// Title centered in box: x=40+160=200, y=15+15+5=35
	if !strings.Contains(svg, `x="200" y="35" text-anchor="middle"`) {
		t.Fatalf("expected manual title position in SVG, got:\n%s", svg)
	}
	if !strings.Contains(svg, ">Revenue<") {
		t.Fatalf("expected title text in SVG, got:\n%s", svg)
	}
}

func TestSVGTitleLayoutInner(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A"},
		Title:      "Sales",
		TitleLayout: &ManualLayout{
			LayoutTarget: "inner",
			XMode:        "edge",
			YMode:        "edge",
			WMode:        "factor",
			HMode:        "factor",
			X:            0.2,
			Y:            0.1,
			W:            0.6,
			H:            0.2,
		},
		Series: []Series{{Values: []float64{10}}},
	}
	svg := c.SVG(400, 300)
	// Inner base: chartX=60, chartY=75 (clamped), chartW=280, chartH=150
	// Title box: x=60+56=116, y=75+15=90, w=168, h=30
	// Center: x=116+84=200, y=90+15+5=110
	if !strings.Contains(svg, `x="200" y="110" text-anchor="middle"`) {
		t.Fatalf("expected inner title position in SVG, got:\n%s", svg)
	}
	if !strings.Contains(svg, ">Sales<") {
		t.Fatalf("expected title text in SVG, got:\n%s", svg)
	}
}

func TestSVGDataLabels(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A"},
		Series: []Series{{
			Index:          0,
			Values:         []float64{42},
			ShowDataLabels: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, ">42<") {
		t.Fatal("expected data label 42")
	}
}

func TestSVGSmoothLines(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B", "C", "D"},
		Series: []Series{{
			Index:  0,
			Values: []float64{10, 30, 20, 40},
			Smooth: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `C`) {
		t.Fatal("expected cubic Bezier curve command")
	}
}

func TestSVGMarkerShapes(t *testing.T) {
	shapes := []string{"circle", "diamond", "square", "triangle", "x", "star", "plus", "none"}
	for _, shape := range shapes {
		c := &Chart{
			ChartType:  ChartLine,
			Categories: []string{"A", "B"},
			Series: []Series{{
				Index:       0,
				Values:      []float64{10, 20},
				MarkerShape: shape,
			}},
		}
		svg := c.SVG(400, 300)
		// Just ensure it renders without panic.
		if svg == "" {
			t.Fatalf("shape %q: empty svg", shape)
		}
	}
}

func TestSVGPieExplosion(t *testing.T) {
	c := &Chart{
		ChartType:  ChartPie,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:     0,
			Values:    []float64{50, 50},
			Explosion: 10,
		}},
	}
	svg := c.SVG(400, 300)
	// With explosion, the slice center is offset from the pie center.
	// Just verify it renders without panic.
	if svg == "" {
		t.Fatal("empty svg")
	}
}

func TestSVGPieThemeColors(t *testing.T) {
	c := &Chart{
		ChartType:  ChartPie,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:     0,
			Values:    []float64{50, 50},
			FillColor: "FF0000",
		}},
		ThemeColors: map[string]string{"accent1": "4472C4"},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `#FF0000`) {
		t.Fatal("expected explicit series fill color")
	}
}

func TestSVGChartAreaThemeTransform(t *testing.T) {
	c := &Chart{
		ChartType: ChartColumn,
		Series:    []Series{{Values: []float64{10}}},
		ThemeColors: map[string]string{
			"accent1": "FF0000",
		},
		ChartArea: ChartAreaStyle{
			fillRef: &drawing.SolidFill{ThemeColor: "accent1", LumMod: 0.5, LumOff: 0.25},
		},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `fill="#BF4040"`) {
		t.Fatalf("expected transformed chart area background, got %s", svg)
	}
}

func TestSVGChartAreaBackground(t *testing.T) {
	c := &Chart{
		ChartType: ChartColumn,
		Series:    []Series{{Values: []float64{10}}},
		ChartArea: ChartAreaStyle{FillColor: "EEEEEE"},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `fill="#EEEEEE"`) {
		t.Fatal("expected chart area background")
	}
}

func TestSVGPlotAreaBorder(t *testing.T) {
	c := &Chart{
		ChartType: ChartColumn,
		Series:    []Series{{Values: []float64{10}}},
		PlotArea:  PlotArea{ShowBorder: true, BorderColor: "333333"},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `stroke="#333333"`) {
		t.Fatal("expected plot area border")
	}
}

func TestSVGAxisTitles(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A"},
		Series:     []Series{{Values: []float64{10}}},
		XAxis:      Axis{Title: "X Title"},
		YAxis:      Axis{Title: "Y Title"},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, ">X Title<") {
		t.Fatal("expected X axis title")
	}
	if !strings.Contains(svg, ">Y Title<") {
		t.Fatal("expected Y axis title")
	}
}

func TestSVGScatterAxisBounds(t *testing.T) {
	c := &Chart{
		ChartType: ChartScatter,
		XAxis:     Axis{Min: 0, Max: 10},
		Series: []Series{{
			Index:   0,
			Values:  []float64{10, 20},
			XValues: []float64{5, 15}, // 15 exceeds XAxis.Max=10
		}},
	}
	svg := c.SVG(400, 300)
	// With XAxis.Max=10, the x=15 point should be clamped to the right edge.
	if !strings.Contains(svg, `<circle`) {
		t.Fatal("expected scatter markers")
	}
}

func TestSVGPercentLabelsOnPie(t *testing.T) {
	c := &Chart{
		ChartType:  ChartPie,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:       0,
			Values:      []float64{25, 75},
			ShowPercent: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `%`) {
		t.Fatal("expected percent labels")
	}
}

func TestSVGLineWidth(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:     0,
			Values:    []float64{10, 20},
			LineWidth: 25400, // 2pt in EMUs
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `stroke-width="2"`) {
		t.Fatal("expected 2pt stroke width")
	}
}

func TestSVGSecondaryYAxis(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		YAxis:      Axis{Title: "Primary"},
		YAxis2:     Axis{Title: "Secondary"},
		Series: []Series{{
			ChartType: ChartColumn,
			Values:    []float64{10, 20},
		}, {
			ChartType:          ChartLine,
			Values:             []float64{100, 200},
			secondaryValueAxis: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, ">Secondary<") {
		t.Fatalf("expected rendered secondary axis title, got:\n%s", svg)
	}
}

func TestSVGEmptySeries(t *testing.T) {
	c := &Chart{
		ChartType: ChartColumn,
		Series:    []Series{},
	}
	svg := c.SVG(400, 300)
	if svg == "" {
		t.Fatal("empty svg for empty series")
	}
}

func TestSVGNullDimensions(t *testing.T) {
	c := &Chart{
		ChartType: ChartColumn,
		Series:    []Series{{Values: []float64{10}}},
	}
	svg := c.SVG(0, 0)
	if !strings.Contains(svg, `width="600"`) {
		t.Fatal("expected default width 600")
	}
	if !strings.Contains(svg, `height="400"`) {
		t.Fatal("expected default height 400")
	}
}

func TestSVGErrorBarsVertical(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Name:   "S1",
			Values: []float64{10, 20, 30},
			ErrorBars: &ErrorBars{
				Direction: "y",
				BarType:   "both",
				ValType:   "fixedVal",
				Value:     5,
			},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<line`) {
		t.Fatal("expected error bar lines")
	}
	// Should have 3 data points, each with a vertical line and 2 end caps = 9 lines
	count := strings.Count(svg, `stroke="#666"`)
	if count < 3 {
		t.Fatalf("expected at least 3 error bar strokes, got %d", count)
	}
}

func TestSVGErrorBarsNoEndCap(t *testing.T) {
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
	svg := c.SVG(400, 300)
	// With noEndCap, each point has only 1 line instead of 3
	count := strings.Count(svg, `stroke="#666"`)
	if count != 2 {
		t.Fatalf("expected 2 error bar strokes (no caps), got %d", count)
	}
}

func TestSVGErrorBarsHorizontal(t *testing.T) {
	c := &Chart{
		ChartType:  ChartBar,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Name:   "S1",
			Values: []float64{10, 20},
			ErrorBars: &ErrorBars{
				Direction: "x",
				BarType:   "both",
				ValType:   "percentage",
				Value:     10,
			},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `stroke="#666"`) {
		t.Fatal("expected error bar lines")
	}
}

func TestSVGDataTable(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		Series: []Series{
			{Name: "S1", Values: []float64{10, 20}},
			{Name: "S2", Values: []float64{15, 25}},
		},
		DataTable: &DataTable{
			ShowHorzBorder: true,
			ShowVertBorder: true,
			ShowOutline:    true,
			ShowKeys:       true,
		},
	}
	svg := c.SVG(400, 400)
	if !strings.Contains(svg, "S1") || !strings.Contains(svg, "S2") {
		t.Fatal("expected series names in data table")
	}
	if !strings.Contains(svg, "A") || !strings.Contains(svg, "B") {
		t.Fatal("expected category labels in data table")
	}
	if !strings.Contains(svg, "10") || !strings.Contains(svg, "20") {
		t.Fatal("expected data values in data table")
	}
}

func TestSVGTrendlineLinear(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B", "C", "D"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 15, 30},
			Trendline: &Trendline{
				Type: "linear",
			},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `stroke-dasharray="5,3"`) {
		t.Fatal("expected dashed trendline")
	}
}

func TestSVGTrendlinePolynomial(t *testing.T) {
	c := &Chart{
		ChartType: ChartScatter,
		Series: []Series{{
			Index:   0,
			Name:    "S1",
			Values:  []float64{1, 4, 9, 16},
			XValues: []float64{1, 2, 3, 4},
			Trendline: &Trendline{
				Type:  "poly",
				Order: 2,
			},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `stroke-dasharray="5,3"`) {
		t.Fatal("expected dashed trendline for polynomial")
	}
}

func TestSVGTrendlineMovingAvg(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B", "C", "D", "E"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 15, 25, 30},
			Trendline: &Trendline{
				Type:   "movingAvg",
				Period: 2,
			},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `stroke-dasharray="5,3"`) {
		t.Fatal("expected dashed trendline for moving average")
	}
}

func TestSVGTrendlineExponential(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B", "C", "D"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{2, 4, 8, 16},
			Trendline: &Trendline{
				Type: "exp",
			},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `stroke-dasharray="5,3"`) {
		t.Fatal("expected dashed trendline for exponential")
	}
}

func TestSVGTrendlineUnsupportedType(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 30},
			Trendline: &Trendline{
				Type: "unknown",
			},
		}},
	}
	svg := c.SVG(400, 300)
	// Should still render chart without trendline
	if !strings.Contains(svg, `<path d="M`) {
		t.Fatal("expected line chart path")
	}
	// Count occurrences of stroke-dasharray to ensure no trendline was drawn
	if strings.Count(svg, `stroke-dasharray="5,3"`) != 0 {
		t.Fatal("expected no trendline for unsupported type")
	}
}

func TestSVGMarkerSize(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:       0,
			Values:      []float64{10, 20},
			MarkerShape: "circle",
			MarkerSize:  10,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `r="10"`) {
		t.Fatalf("expected marker radius 10, got:\n%s", svg)
	}
}

func TestSVGMarkerColor(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:       0,
			Values:      []float64{10, 20},
			MarkerShape: "circle",
			MarkerColor: "FF0000",
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `fill="#FF0000"`) {
		t.Fatalf("expected marker fill #FF0000, got:\n%s", svg)
	}
}

func TestSVGLinePatternDash(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:       0,
			Values:      []float64{10, 20},
			LinePattern: 1,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `stroke-dasharray="6,3"`) {
		t.Fatalf("expected dash pattern, got:\n%s", svg)
	}
}

func TestSVGLinePatternDot(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:       0,
			Values:      []float64{10, 20},
			LinePattern: 2,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `stroke-dasharray="2,3"`) {
		t.Fatalf("expected dot pattern, got:\n%s", svg)
	}
}

func TestSVGTrendlineWithEquation(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B", "C", "D"},
		Series: []Series{{
			Index:  0,
			Values: []float64{10, 20, 15, 30},
			Trendline: &Trendline{
				Type:         "linear",
				ShowEquation: true,
				ShowRSquared: true,
			},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `y =`) {
		t.Fatalf("expected equation text, got:\n%s", svg)
	}
	if !strings.Contains(svg, `R² =`) {
		t.Fatalf("expected R² text, got:\n%s", svg)
	}
}

func TestSVGTrendlineHorizontalBar(t *testing.T) {
	c := &Chart{
		ChartType:  ChartBar,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Values: []float64{10, 20, 15},
			Trendline: &Trendline{
				Type: "linear",
			},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `stroke-dasharray="5,3"`) {
		t.Fatalf("expected trendline on horizontal bar chart, got:\n%s", svg)
	}
}

func TestSVGDataLabelSeparator(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Name:           "Sales",
			Values:         []float64{10, 20},
			ShowDataLabels: true,
			ShowCatName:    true,
			ShowSerName:    true,
			Separator:      " | ",
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, "Sales | A | 10") {
		t.Fatalf("expected separator join, got:\n%s", svg)
	}
}

func TestSVGDataLabelCustomText(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Values:         []float64{10, 20},
			ShowDataLabels: true,
			DataPoints: []DataPoint{{
				Index: 1,
				Label: "Custom",
			}},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, ">Custom<") {
		t.Fatalf("expected custom label, got:\n%s", svg)
	}
	// First point should still show auto label
	if !strings.Contains(svg, ">10<") {
		t.Fatalf("expected auto label for first point, got:\n%s", svg)
	}
}

func TestSVGDataLabelNumFmt(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A"},
		Series: []Series{{
			Values:         []float64{12.345},
			ShowDataLabels: true,
			NumFmt:         "0.00",
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, ">12.35<") && !strings.Contains(svg, ">12.34<") {
		t.Fatalf("expected formatted value 12.34/12.35, got:\n%s", svg)
	}
}

func TestSVGDataLabelPosition(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A"},
		Series: []Series{{
			Values:         []float64{50},
			ShowDataLabels: true,
			LabelPosition:  3, // ctr
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, ">50<") {
		t.Fatalf("expected label at center, got:\n%s", svg)
	}
}

func TestSVGRadarFilled(t *testing.T) {
	c := &Chart{
		ChartType:  ChartRadar,
		Categories: []string{"A", "B", "C", "D", "E"},
		RadarStyle: "filled",
		Series: []Series{{
			Values: []float64{30, 50, 40, 60, 20},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `fill-opacity="0.5"`) {
		t.Fatalf("expected filled radar polygon, got:\n%s", svg)
	}
	if strings.Contains(svg, `stroke-width="2"`) {
		t.Fatalf("filled radar should not have stroke, got:\n%s", svg)
	}
}

func TestSVGRadarMarkerOnly(t *testing.T) {
	c := &Chart{
		ChartType:  ChartRadar,
		Categories: []string{"A", "B", "C", "D", "E"},
		RadarStyle: "marker",
		Series: []Series{{
			Values: []float64{30, 50, 40, 60, 20},
		}},
	}
	svg := c.SVG(400, 300)
	if strings.Contains(svg, `polygon`) {
		t.Fatalf("marker-only radar should not draw polygon, got:\n%s", svg)
	}
}

func TestSVGPieLeaderLines(t *testing.T) {
	c := &Chart{
		ChartType:  ChartPie,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:           0,
			Values:          []float64{50, 50},
			ShowDataLabels:  true,
			ShowLeaderLines: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<line`) {
		t.Fatalf("expected leader lines, got:\n%s", svg)
	}
}

func TestSVGLegendEntryDeleted(t *testing.T) {
	c := &Chart{
		ChartType: ChartColumn,
		Series: []Series{
			{Name: "S1", Values: []float64{10}},
			{Name: "S2", Values: []float64{20}},
			{Name: "S3", Values: []float64{30}},
		},
		Legend: Legend{Show: true, Position: "bottom", Entries: []LegendEntry{
			{Index: 1, Delete: true},
		}},
	}
	svg := c.SVG(400, 300)
	if strings.Contains(svg, ">S2<") {
		t.Fatalf("expected S2 to be hidden from legend, got:\n%s", svg)
	}
	if !strings.Contains(svg, ">S1<") || !strings.Contains(svg, ">S3<") {
		t.Fatalf("expected S1 and S3 in legend, got:\n%s", svg)
	}
}

func TestSVGDataLabelStyle(t *testing.T) {
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
				{Index: 0, LabelStyle: &TextProperties{FontSize: 800, Color: "00FF00"}},
			},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `font-size="12"`) {
		t.Fatalf("expected font-size=12, got:\n%s", svg)
	}
	if !strings.Contains(svg, `font-weight="bold"`) {
		t.Fatalf("expected bold, got:\n%s", svg)
	}
	if !strings.Contains(svg, `font-style="italic"`) {
		t.Fatalf("expected italic, got:\n%s", svg)
	}
	if !strings.Contains(svg, `fill="#FF0000"`) {
		t.Fatalf("expected red fill, got:\n%s", svg)
	}
	if !strings.Contains(svg, `fill="#00FF00"`) {
		t.Fatalf("expected green per-point fill, got:\n%s", svg)
	}
}

func TestSVGDataLabelStylePie(t *testing.T) {
	c := &Chart{
		ChartType:  ChartPie,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:          0,
			Values:         []float64{50, 50},
			ShowDataLabels: true,
			LabelStyle: &TextProperties{
				FontSize: 1400,
				Color:    "FFFFFF",
			},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `font-size="14"`) {
		t.Fatalf("expected font-size=14, got:\n%s", svg)
	}
	if !strings.Contains(svg, `fill="#FFFFFF"`) {
		t.Fatalf("expected white fill, got:\n%s", svg)
	}
}

func TestSVGLeaderLinesBar(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:           0,
			Values:          []float64{10, 20},
			ShowDataLabels:  true,
			ShowLeaderLines: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<line`) {
		t.Fatalf("expected leader lines in column chart, got:\n%s", svg)
	}
}

func TestSVGLeaderLinesHorizontalBar(t *testing.T) {
	c := &Chart{
		ChartType:  ChartBar,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:           0,
			Values:          []float64{10, 20},
			ShowDataLabels:  true,
			ShowLeaderLines: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<line`) {
		t.Fatalf("expected leader lines in horizontal bar chart, got:\n%s", svg)
	}
}

func TestSVGLeaderLinesLine(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:           0,
			Values:          []float64{10, 20},
			ShowDataLabels:  true,
			ShowLeaderLines: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<line`) {
		t.Fatalf("expected leader lines in line chart, got:\n%s", svg)
	}
}

func TestSVGLeaderLinesArea(t *testing.T) {
	c := &Chart{
		ChartType:  ChartArea,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:           0,
			Values:          []float64{10, 20},
			ShowDataLabels:  true,
			ShowLeaderLines: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<line`) {
		t.Fatalf("expected leader lines in area chart, got:\n%s", svg)
	}
}

func TestSVGLeaderLinesScatter(t *testing.T) {
	c := &Chart{
		ChartType:  ChartScatter,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:           0,
			Values:          []float64{10, 20},
			ShowDataLabels:  true,
			ShowLeaderLines: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<line`) {
		t.Fatalf("expected leader lines in scatter chart, got:\n%s", svg)
	}
}

func TestSVGLeaderLinesBubble(t *testing.T) {
	c := &Chart{
		ChartType:  ChartBubble,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:           0,
			Values:          []float64{10, 20},
			BubbleSizes:     []float64{5, 10},
			ShowDataLabels:  true,
			ShowLeaderLines: true,
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<line`) {
		t.Fatalf("expected leader lines in bubble chart, got:\n%s", svg)
	}
}

func TestSVGNoLeaderLinesWhenDisabled(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn,
		Categories: []string{"A", "B"},
		Series: []Series{{
			Index:           0,
			Values:          []float64{10, 20},
			ShowDataLabels:  true,
			ShowLeaderLines: false,
		}},
	}
	svg := c.SVG(400, 300)
	// Axis grid lines use <line>, so we need a more specific check.
	// The leader line uses stroke="#999"; axis lines use stroke="#ccc" or "#000".
	if strings.Contains(svg, `stroke="#999"`) {
		t.Fatalf("unexpected leader lines when disabled, got:\n%s", svg)
	}
}

func TestSVGColumn3D(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn3D,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 30},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, "<polygon") {
		t.Fatal("expected 3D column polygons")
	}
	if !strings.Contains(svg, ">A<") {
		t.Fatal("expected category A")
	}
}

func TestSVGColumn3DRotated(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn3D,
		Categories: []string{"A", "B"},
		View3D: &View3D{
			RotX:        30,
			RotY:        45,
			Perspective: 60,
		},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{15, 25},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, "<polygon") {
		t.Fatal("expected 3D column polygons with custom rotation")
	}
}

func TestSVGBar3D(t *testing.T) {
	c := &Chart{
		ChartType:  ChartBar3D,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 30},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, "<polygon") {
		t.Fatal("expected 3D bar polygons")
	}
}

func TestSVGColumn3DMultiSeries(t *testing.T) {
	c := &Chart{
		ChartType:  ChartColumn3D,
		Categories: []string{"A", "B"},
		Series: []Series{
			{Index: 0, Name: "S1", Values: []float64{10, 20}},
			{Index: 1, Name: "S2", Values: []float64{15, 25}},
		},
	}
	svg := c.SVG(400, 300)
	// With two series we should see more polygons than a single series.
	count := strings.Count(svg, "<polygon")
	if count < 6 {
		t.Fatalf("expected at least 6 polygons for 2-series 3D column, got %d", count)
	}
}

func TestSVGPie3D(t *testing.T) {
	c := &Chart{
		ChartType:  ChartPie3D,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{30, 50, 20},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<path d="M`) {
		t.Fatal("expected pie slices")
	}
	// Should have side walls (extra paths beyond the top faces).
	pathCount := strings.Count(svg, `<path d="M`)
	if pathCount < 6 {
		t.Fatalf("expected at least 6 paths (top + wall per slice), got %d", pathCount)
	}
}

func TestSVGPie3DExtruded(t *testing.T) {
	c := &Chart{
		ChartType: ChartPie3D,
		View3D: &View3D{
			DepthPercent: 150,
		},
		Series: []Series{{
			Values: []float64{25, 75},
		}},
	}
	svg := c.SVG(400, 300)
	// Each slice should have a side wall with darker fill.
	if !strings.Contains(svg, `fill="#`) {
		t.Fatal("expected filled paths")
	}
}

func TestSVGLine3D(t *testing.T) {
	c := &Chart{
		ChartType:  ChartLine3D,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 15},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `<path d="M`) {
		t.Fatal("expected line path")
	}
	// Axes should still be present.
	if !strings.Contains(svg, `<line`) {
		t.Fatal("expected axis lines")
	}
}

func TestSVGArea3D(t *testing.T) {
	c := &Chart{
		ChartType:  ChartArea3D,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:  0,
			Name:   "S1",
			Values: []float64{10, 20, 15},
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, `fill-opacity="0.6"`) {
		t.Fatal("expected area fill with reduced opacity")
	}
	if !strings.Contains(svg, `<path d="M`) {
		t.Fatal("expected area path")
	}
}

func TestSVGSurface3D(t *testing.T) {
	c := &Chart{
		ChartType: ChartSurface3D,
		Series: []Series{
			{Index: 0, Values: []float64{10, 20, 30}},
			{Index: 1, Values: []float64{15, 25, 35}},
		},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, "<polygon") {
		t.Fatal("expected 3D surface polygons")
	}
}

func TestSVGPieElbowCallout(t *testing.T) {
	c := &Chart{
		ChartType:  ChartPie,
		Categories: []string{"A", "B", "C"},
		Series: []Series{{
			Index:           0,
			Name:            "S1",
			Values:          []float64{30, 50, 20},
			ShowDataLabels:  true,
			ShowLeaderLines: true,
			LabelPosition:   1, // outEnd
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, "<polyline") {
		t.Fatal("expected L-shaped elbow callout polyline")
	}
	// Labels should be outside the pie.
	if !strings.Contains(svg, `text-anchor="start"`) && !strings.Contains(svg, `text-anchor="end"`) {
		t.Fatal("expected outside-end labels with start/end anchor")
	}
}

func TestSVGPie3DElbowCallout(t *testing.T) {
	c := &Chart{
		ChartType: ChartPie3D,
		Series: []Series{{
			Values:          []float64{25, 75},
			ShowDataLabels:  true,
			ShowLeaderLines: true,
			LabelPosition:   1, // outEnd
		}},
	}
	svg := c.SVG(400, 300)
	if !strings.Contains(svg, "<polyline") {
		t.Fatal("expected 3D pie L-shaped elbow callout polyline")
	}
}

func TestRenderDataLabelElbowLineHorizontal(t *testing.T) {
	var b strings.Builder
	renderDataLabelElbowLine(&b, 100, 100, 150, 50, true)
	s := b.String()
	if !strings.Contains(s, "<polyline") {
		t.Fatal("expected polyline for horizontal-first elbow")
	}
	if !strings.Contains(s, "100,100 150,100 150,50") {
		t.Fatalf("unexpected elbow points: %s", s)
	}
}

func TestRenderDataLabelElbowLineVertical(t *testing.T) {
	var b strings.Builder
	renderDataLabelElbowLine(&b, 100, 100, 150, 50, false)
	s := b.String()
	if !strings.Contains(s, "<polyline") {
		t.Fatal("expected polyline for vertical-first elbow")
	}
	if !strings.Contains(s, "100,100 100,50 150,50") {
		t.Fatalf("unexpected elbow points: %s", s)
	}
}

func TestRenderDataLabelElbowLineDegenerate(t *testing.T) {
	// When the bend coincides with an endpoint, draw a straight line.
	var b strings.Builder
	renderDataLabelElbowLine(&b, 100, 100, 100, 50, true)
	s := b.String()
	if !strings.Contains(s, "<line") {
		t.Fatal("expected straight line for degenerate elbow")
	}
}
