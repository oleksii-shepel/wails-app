package chart

import (
	"encoding/xml"
	"fmt"
	"strings"
)

const (
	cNS = "http://schemas.openxmlformats.org/drawingml/2006/chart"
	aNS = "http://schemas.openxmlformats.org/drawingml/2006/main"
)

// MarshalChart serializes a Chart to c:chartSpace XML.
func MarshalChart(c *Chart) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("nil chart")
	}
	cs := chartSpaceMarshal{}
	cs.XMLName.Space = cNS
	cs.XMLName.Local = "chartSpace"
	cs.Date1904 = &boolValMarshal{Val: false}
	cs.Lang = &strValMarshal{Val: "en-US"}
	cs.RoundedCorners = &boolValMarshal{Val: false}

	if c.ChartArea.FillColor != "" {
		cs.SpPr = &spPrMarshal{
			SolidFill: &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(c.ChartArea.FillColor)}},
		}
	}

	cs.Chart = chartMarshal{}
	if c.Title != "" {
		cs.Chart.Title = makeTitle(c.Title, c.TitleLayout)
	}
	cs.Chart.PlotArea = makePlotArea(c)
	if c.Legend.Show {
		cs.Chart.Legend = &legendMarshal{LegendPos: &strValMarshal{Val: c.Legend.Position}}
		for _, entry := range c.Legend.Entries {
			cs.Chart.Legend.LegendEntries = append(cs.Chart.Legend.LegendEntries, legendEntryMarshal{
				Idx:    intValMarshal{Val: entry.Index},
				Delete: &boolValMarshal{Val: entry.Delete},
			})
		}
	}
	if c.LegendLayout != nil {
		if cs.Chart.Legend == nil {
			cs.Chart.Legend = &legendMarshal{}
		}
		cs.Chart.Legend.Layout = makeLayout(c.LegendLayout)
	}
	cs.Chart.PlotVisOnly = &boolValMarshal{Val: true}
	if c.DisplayBlanksAs != "" {
		cs.Chart.DisplayBlanksAs = &strValMarshal{Val: c.DisplayBlanksAs}
	}

	data, err := xml.Marshal(cs)
	if err != nil {
		return nil, fmt.Errorf("marshal chart: %w", err)
	}
	return append([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`), data...), nil
}

// chartSpaceMarshal is the root c:chartSpace element.
type chartSpaceMarshal struct {
	XMLName        xml.Name        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart chartSpace"`
	Date1904       *boolValMarshal `xml:"date1904,omitempty"`
	Lang           *strValMarshal  `xml:"lang,omitempty"`
	RoundedCorners *boolValMarshal `xml:"roundedCorners,omitempty"`
	SpPr           *spPrMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spPr,omitempty"`
	Chart          chartMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart chart"`
}

type chartMarshal struct {
	Title           *titleMarshal   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart title,omitempty"`
	PlotArea        plotAreaMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart plotArea"`
	Legend          *legendMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart legend,omitempty"`
	PlotVisOnly     *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart plotVisOnly,omitempty"`
	DisplayBlanksAs *strValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart displayBlanksAs,omitempty"`
}

type boolValMarshal struct {
	Val bool `xml:"val,attr"`
}

type strValMarshal struct {
	Val string `xml:"val,attr"`
}

type intValMarshal struct {
	Val int `xml:"val,attr"`
}

type floatValMarshal struct {
	Val float64 `xml:"val,attr"`
}

type titleMarshal struct {
	Tx     txMarshal      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx"`
	Layout *layoutMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart layout,omitempty"`
}

type txMarshal struct {
	Rich richMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart rich"`
}

type richMarshal struct {
	BodyPr   struct{}         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bodyPr"`
	LstStyle struct{}         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lstStyle"`
	P        paragraphMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/main p"`
}

type paragraphMarshal struct {
	PPr struct{}   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pPr>defRPr"`
	R   runMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/main r"`
}

type runMarshal struct {
	RPr struct{} `xml:"http://schemas.openxmlformats.org/drawingml/2006/main rPr"`
	T   string   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main t"`
}

func makeTitle(title string, layout *ManualLayout) *titleMarshal {
	tm := &titleMarshal{
		Tx: txMarshal{
			Rich: richMarshal{
				P: paragraphMarshal{
					R: runMarshal{T: title},
				},
			},
		},
	}
	if layout != nil {
		tm.Layout = makeLayout(layout)
	}
	return tm
}

type plotAreaMarshal struct {
	Layout   *layoutMarshal        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart layout,omitempty"`
	Bar      *barChartMarshal      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart barChart,omitempty"`
	Line     *lineChartMarshal     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart lineChart,omitempty"`
	Pie      *pieChartMarshal      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pieChart,omitempty"`
	Doughnut *doughnutChartMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart doughnutChart,omitempty"`
	Area     *areaChartMarshal     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart areaChart,omitempty"`
	Scatter  *scatterChartMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart scatterChart,omitempty"`
	Bubble   *bubbleChartMarshal   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart bubbleChart,omitempty"`
	Stock    *stockChartMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart stockChart,omitempty"`
	Radar    *radarChartMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart radarChart,omitempty"`
	Surface  *surfaceChartMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart surfaceChart,omitempty"`
	CatAxes  []catAxisMarshal      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart catAx,omitempty"`
	ValAxes  []valAxisMarshal      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart valAx,omitempty"`
	DTable   *dTableMarshal        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dTable,omitempty"`
	SpPr     *spPrMarshal          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spPr,omitempty"`
}

func makePlotArea(c *Chart) plotAreaMarshal {
	pa := plotAreaMarshal{}
	if c.ManualLayout != nil {
		pa.Layout = makeLayout(c.ManualLayout)
	}
	if c.PlotArea.FillColor != "" || c.PlotArea.ShowBorder {
		pa.SpPr = &spPrMarshal{}
		if c.PlotArea.FillColor != "" {
			pa.SpPr.SolidFill = &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(c.PlotArea.FillColor)}}
		}
		if c.PlotArea.ShowBorder {
			pa.SpPr.Line = &lineMarshal{}
			if c.PlotArea.BorderColor != "" {
				pa.SpPr.Line.SolidFill = &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(c.PlotArea.BorderColor)}}
			}
		}
	}

	groups := marshalGroups(c)
	if len(groups) <= 1 {
		// Single-group chart: use the old switch-based logic for backward compat.
		return makePlotAreaSingleGroup(c, pa)
	}

	// Multi-group (combo) chart: emit one XML element per group.
	axisIDSet := make(map[int]bool)
	hasCatAxis := false
	for _, g := range groups {
		series := makeSeriesForGroup(c, g)
		axisIDs := groupAxisIDs(g)
		for _, id := range axisIDs {
			axisIDSet[id] = true
		}
		switch g.ChartType {
		case ChartBar, ChartColumn:
			dir := g.BarDir
			if dir == "" {
				dir = "col"
				if g.ChartType == ChartBar {
					dir = "bar"
				}
			}
			grouping := g.Grouping
			if grouping == "" {
				grouping = "clustered"
			}
			bm := &barChartMarshal{
				BarDir:   &strValMarshal{Val: dir},
				Grouping: &strValMarshal{Val: grouping},
				Series:   series,
				AxisID:   intVals(axisIDs),
			}
			if g.GapWidth != 0 {
				bm.GapWidth = &intValMarshal{Val: g.GapWidth}
			}
			if g.Overlap != 0 {
				bm.Overlap = &intValMarshal{Val: g.Overlap}
			}
			if g.VaryColors {
				bm.VaryColors = &intValMarshal{Val: 1}
			}
			pa.Bar = bm
			hasCatAxis = true
		case ChartLine, ChartLine3D:
			grouping := g.Grouping
			if grouping == "" {
				grouping = "standard"
			}
			pa.Line = &lineChartMarshal{
				Grouping: &strValMarshal{Val: grouping},
				Series:   series,
				AxisID:   intVals(axisIDs),
			}
			hasCatAxis = true
		case ChartArea, ChartArea3D:
			grouping := g.Grouping
			if grouping == "" {
				grouping = "standard"
			}
			pa.Area = &areaChartMarshal{
				Grouping: &strValMarshal{Val: grouping},
				Series:   series,
				AxisID:   intVals(axisIDs),
			}
			hasCatAxis = true
		case ChartScatter:
			style := groupScatterStyle(c, g)
			if style == "" {
				style = "lineMarker"
			}
			pa.Scatter = &scatterChartMarshal{
				ScatterStyle: &strValMarshal{Val: style},
				Series:       series,
				AxisID:       intVals(axisIDs),
			}
		case ChartBubble:
			pa.Bubble = &bubbleChartMarshal{
				Series: series,
				AxisID: intVals(axisIDs),
			}
		case ChartStock:
			scm := &stockChartMarshal{
				Series: series,
				AxisID: intVals(axisIDs),
			}
			if c.StockConfig != nil {
				if c.StockConfig.ShowDropLines {
					scm.DropLines = &linesMarshal{}
				}
				if c.StockConfig.ShowHiLowLines {
					scm.HiLowLines = &linesMarshal{}
				}
				var upBars, downBars *spPrMarshal
				if c.StockConfig.UpBarColor != "" {
					upBars = &spPrMarshal{SolidFill: &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(c.StockConfig.UpBarColor)}}}
				}
				if c.StockConfig.DownBarColor != "" {
					downBars = &spPrMarshal{SolidFill: &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(c.StockConfig.DownBarColor)}}}
				}
				if upBars != nil || downBars != nil {
					scm.UpDownBars = &upDownBarsMarshal{UpBars: upBars, DownBars: downBars}
				}
			}
			pa.Stock = scm
			hasCatAxis = true
		case ChartRadar:
			style := g.Grouping
			if style == "" {
				style = "standard"
			}
			pa.Radar = &radarChartMarshal{
				RadarStyle: &strValMarshal{Val: style},
				Series:     series,
				AxisID:     intVals(axisIDs),
			}
			hasCatAxis = true
		case ChartSurface:
			pa.Surface = &surfaceChartMarshal{
				Series: series,
				AxisID: intVals(axisIDs),
			}
			hasCatAxis = true
		case ChartPie:
			pm := &pieChartMarshal{Series: series}
			if g.VaryColors {
				pm.VaryColors = &intValMarshal{Val: 1}
			}
			pa.Pie = pm
		case ChartDoughnut:
			dm := &doughnutChartMarshal{Series: series}
			if g.HoleSize != 0 {
				dm.HoleSize = &intValMarshal{Val: g.HoleSize}
			}
			if g.VaryColors {
				dm.VaryColors = &intValMarshal{Val: 1}
			}
			pa.Doughnut = dm
		}
	}

	// Emit axes based on the unique axis IDs encountered.
	catAxisID, valAxisIDs := classifyAxisIDs(groups, hasCatAxis)
	if catAxisID != 0 {
		pa.CatAxes = append(pa.CatAxes, makeCatAxis(catAxisID, valAxisIDs[0], c.XAxis))
	}
	for i, vid := range valAxisIDs {
		cross := catAxisID
		if cross == 0 {
			cross = valAxisIDs[0]
			if i == 0 && len(valAxisIDs) > 1 {
				cross = valAxisIDs[1]
			}
		}
		axis := c.YAxis
		if i > 0 {
			axis = c.YAxis2
		}
		pa.ValAxes = append(pa.ValAxes, makeValAxis(vid, cross, axis))
	}

	if c.DataTable != nil {
		pa.DTable = &dTableMarshal{}
		if c.DataTable.ShowHorzBorder {
			pa.DTable.ShowHorzBorder = &boolValMarshal{Val: true}
		}
		if c.DataTable.ShowVertBorder {
			pa.DTable.ShowVertBorder = &boolValMarshal{Val: true}
		}
		if c.DataTable.ShowOutline {
			pa.DTable.ShowOutline = &boolValMarshal{Val: true}
		}
		if c.DataTable.ShowKeys {
			pa.DTable.ShowKeys = &boolValMarshal{Val: true}
		}
	}
	return pa
}

// marshalGroups returns the chart groups, synthesising a single group when
// none exist (backward compatibility for code that only sets ChartType).
func marshalGroups(c *Chart) []ChartGroup {
	if len(c.Groups) > 0 {
		return c.Groups
	}
	indices := make([]int, len(c.Series))
	for i := range c.Series {
		indices[i] = i
	}
	g := ChartGroup{
		ChartType:     c.ChartType,
		SeriesIndices: indices,
	}
	switch c.ChartType {
	case ChartBar, ChartColumn, ChartBar3D, ChartColumn3D:
		g.GapWidth = c.GapWidth
		g.Overlap = c.Overlap
		g.VaryColors = c.VaryColors
		if c.ChartType == ChartBar {
			g.BarDir = "bar"
		} else {
			g.BarDir = "col"
		}
		if c.PercentStacked {
			g.Grouping = "percentStacked"
		} else if c.Stacked {
			g.Grouping = "stacked"
		} else {
			g.Grouping = "clustered"
		}
	case ChartLine, ChartLine3D:
		if c.PercentStacked {
			g.Grouping = "percentStacked"
		} else if c.Stacked {
			g.Grouping = "stacked"
		} else {
			g.Grouping = "standard"
		}
	case ChartArea, ChartArea3D:
		if c.PercentStacked {
			g.Grouping = "percentStacked"
		} else if c.Stacked {
			g.Grouping = "stacked"
		} else {
			g.Grouping = "standard"
		}
	case ChartPie, ChartPie3D:
		g.VaryColors = c.VaryColors
	case ChartDoughnut:
		g.HoleSize = c.HoleSize
		g.VaryColors = c.VaryColors
	case ChartRadar:
		g.Grouping = c.RadarStyle
	}
	return []ChartGroup{g}
}

// groupAxisIDs returns the axis IDs for a group, defaulting to [1,2] when empty.
func groupAxisIDs(g ChartGroup) []int {
	if len(g.AxisIDs) > 0 {
		return g.AxisIDs
	}
	// Default axis assignment based on chart type.
	switch g.ChartType {
	case ChartScatter, ChartBubble:
		return []int{1, 2}
	default:
		return []int{1, 2}
	}
}

// classifyAxisIDs determines which axis ID is the category axis and which are
// value axes based on the groups' chart types and axis bindings.
func classifyAxisIDs(groups []ChartGroup, hasCatAxis bool) (catAxisID int, valAxisIDs []int) {
	if !hasCatAxis {
		// Scatter / bubble: all axes are value axes.
		seen := make(map[int]bool)
		for _, g := range groups {
			for _, id := range groupAxisIDs(g) {
				if !seen[id] {
					seen[id] = true
					valAxisIDs = append(valAxisIDs, id)
				}
			}
		}
		if len(valAxisIDs) == 0 {
			valAxisIDs = []int{1, 2}
		}
		return 0, valAxisIDs
	}
	// For charts with a category axis, the first axis ID of the first
	// non-scatter/non-bubble group is usually the cat axis.
	seen := make(map[int]bool)
	for _, g := range groups {
		ids := groupAxisIDs(g)
		if len(ids) == 0 {
			continue
		}
		if g.ChartType != ChartScatter && g.ChartType != ChartBubble {
			if catAxisID == 0 {
				catAxisID = ids[0]
			}
		}
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				valAxisIDs = append(valAxisIDs, id)
			}
		}
	}
	if catAxisID == 0 && len(valAxisIDs) > 0 {
		catAxisID = valAxisIDs[0]
	}
	// Reorder so cat axis is first, then val axes.
	if catAxisID != 0 {
		filtered := make([]int, 0, len(valAxisIDs))
		for _, id := range valAxisIDs {
			if id != catAxisID {
				filtered = append(filtered, id)
			}
		}
		valAxisIDs = filtered
	}
	if len(valAxisIDs) == 0 {
		valAxisIDs = []int{2}
	}
	return catAxisID, valAxisIDs
}

func intVals(ids []int) []intValMarshal {
	out := make([]intValMarshal, len(ids))
	for i, id := range ids {
		out[i] = intValMarshal{Val: id}
	}
	return out
}

func groupScatterStyle(c *Chart, g ChartGroup) string {
	for _, idx := range g.SeriesIndices {
		if idx >= 0 && idx < len(c.Series) && c.Series[idx].ScatterStyle != "" {
			return c.Series[idx].ScatterStyle
		}
	}
	return ""
}

func makePlotAreaSingleGroup(c *Chart, pa plotAreaMarshal) plotAreaMarshal {
	switch c.ChartType {
	case ChartBar, ChartColumn:
		dir := "col"
		if c.ChartType == ChartBar {
			dir = "bar"
		}
		grouping := "clustered"
		if c.PercentStacked {
			grouping = "percentStacked"
		} else if c.Stacked {
			grouping = "stacked"
		}
		bm := &barChartMarshal{
			BarDir:   &strValMarshal{Val: dir},
			Grouping: &strValMarshal{Val: grouping},
			Series:   makeSeries(c),
			AxisID:   []intValMarshal{{Val: 1}, {Val: 2}},
		}
		if c.GapWidth != 0 {
			bm.GapWidth = &intValMarshal{Val: c.GapWidth}
		}
		if c.Overlap != 0 {
			bm.Overlap = &intValMarshal{Val: c.Overlap}
		}
		if c.VaryColors {
			bm.VaryColors = &intValMarshal{Val: 1}
		}
		pa.Bar = bm
		pa.CatAxes = append(pa.CatAxes, makeCatAxis(1, 2, c.XAxis))
		pa.ValAxes = append(pa.ValAxes, makeValAxis(2, 1, c.YAxis))
	case ChartLine:
		grouping := "standard"
		if c.PercentStacked {
			grouping = "percentStacked"
		} else if c.Stacked {
			grouping = "stacked"
		}
		pa.Line = &lineChartMarshal{
			Grouping: &strValMarshal{Val: grouping},
			Series:   makeSeries(c),
			AxisID:   []intValMarshal{{Val: 1}, {Val: 2}},
		}
		pa.CatAxes = append(pa.CatAxes, makeCatAxis(1, 2, c.XAxis))
		pa.ValAxes = append(pa.ValAxes, makeValAxis(2, 1, c.YAxis))
	case ChartPie:
		pm := &pieChartMarshal{Series: makeSeries(c)}
		if c.VaryColors {
			pm.VaryColors = &intValMarshal{Val: 1}
		}
		pa.Pie = pm
	case ChartDoughnut:
		dm := &doughnutChartMarshal{Series: makeSeries(c)}
		if c.HoleSize != 0 {
			dm.HoleSize = &intValMarshal{Val: c.HoleSize}
		}
		if c.VaryColors {
			dm.VaryColors = &intValMarshal{Val: 1}
		}
		pa.Doughnut = dm
	case ChartArea:
		grouping := "standard"
		if c.PercentStacked {
			grouping = "percentStacked"
		} else if c.Stacked {
			grouping = "stacked"
		}
		pa.Area = &areaChartMarshal{
			Grouping: &strValMarshal{Val: grouping},
			Series:   makeSeries(c),
			AxisID:   []intValMarshal{{Val: 1}, {Val: 2}},
		}
		pa.CatAxes = append(pa.CatAxes, makeCatAxis(1, 2, c.XAxis))
		pa.ValAxes = append(pa.ValAxes, makeValAxis(2, 1, c.YAxis))
	case ChartScatter:
		style := "lineMarker"
		if len(c.Series) > 0 && c.Series[0].ScatterStyle != "" {
			style = c.Series[0].ScatterStyle
		}
		pa.Scatter = &scatterChartMarshal{
			ScatterStyle: &strValMarshal{Val: style},
			Series:       makeSeries(c),
			AxisID:       []intValMarshal{{Val: 1}, {Val: 2}},
		}
		pa.ValAxes = append(pa.ValAxes, makeValAxis(1, 2, c.XAxis), makeValAxis(2, 1, c.YAxis))
	case ChartBubble:
		pa.Bubble = &bubbleChartMarshal{
			Series: makeSeries(c),
			AxisID: []intValMarshal{{Val: 1}, {Val: 2}},
		}
		pa.ValAxes = append(pa.ValAxes, makeValAxis(1, 2, c.XAxis), makeValAxis(2, 1, c.YAxis))
	case ChartStock:
		scm := &stockChartMarshal{
			Series: makeSeries(c),
			AxisID: []intValMarshal{{Val: 1}, {Val: 2}},
		}
		if c.StockConfig != nil {
			if c.StockConfig.ShowDropLines {
				scm.DropLines = &linesMarshal{}
			}
			if c.StockConfig.ShowHiLowLines {
				scm.HiLowLines = &linesMarshal{}
			}
			var upBars, downBars *spPrMarshal
			if c.StockConfig.UpBarColor != "" {
				upBars = &spPrMarshal{SolidFill: &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(c.StockConfig.UpBarColor)}}}
			}
			if c.StockConfig.DownBarColor != "" {
				downBars = &spPrMarshal{SolidFill: &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(c.StockConfig.DownBarColor)}}}
			}
			if upBars != nil || downBars != nil {
				scm.UpDownBars = &upDownBarsMarshal{UpBars: upBars, DownBars: downBars}
			}
		}
		pa.Stock = scm
		pa.CatAxes = append(pa.CatAxes, makeCatAxis(1, 2, c.XAxis))
		pa.ValAxes = append(pa.ValAxes, makeValAxis(2, 1, c.YAxis))
	case ChartRadar:
		style := c.RadarStyle
		if style == "" {
			style = "standard"
		}
		pa.Radar = &radarChartMarshal{
			RadarStyle: &strValMarshal{Val: style},
			Series:     makeSeries(c),
			AxisID:     []intValMarshal{{Val: 1}, {Val: 2}},
		}
		pa.CatAxes = append(pa.CatAxes, makeCatAxis(1, 2, c.XAxis))
		pa.ValAxes = append(pa.ValAxes, makeValAxis(2, 1, c.YAxis))
	case ChartSurface:
		pa.Surface = &surfaceChartMarshal{
			Series: makeSeries(c),
			AxisID: []intValMarshal{{Val: 1}, {Val: 2}},
		}
		pa.CatAxes = append(pa.CatAxes, makeCatAxis(1, 2, c.XAxis))
		pa.ValAxes = append(pa.ValAxes, makeValAxis(2, 1, c.YAxis))
	}
	if c.DataTable != nil {
		pa.DTable = &dTableMarshal{}
		if c.DataTable.ShowHorzBorder {
			pa.DTable.ShowHorzBorder = &boolValMarshal{Val: true}
		}
		if c.DataTable.ShowVertBorder {
			pa.DTable.ShowVertBorder = &boolValMarshal{Val: true}
		}
		if c.DataTable.ShowOutline {
			pa.DTable.ShowOutline = &boolValMarshal{Val: true}
		}
		if c.DataTable.ShowKeys {
			pa.DTable.ShowKeys = &boolValMarshal{Val: true}
		}
	}
	return pa
}

func makeSeriesForGroup(c *Chart, g ChartGroup) []seriesMarshal {
	out := make([]seriesMarshal, 0, len(g.SeriesIndices))
	for _, idx := range g.SeriesIndices {
		if idx < 0 || idx >= len(c.Series) {
			continue
		}
		s := c.Series[idx]
		cats := c.Categories
		if len(s.Labels) > 0 {
			cats = s.Labels
		}
		out = append(out, makeSeriesMarshal(idx, &s, cats))
	}
	return out
}

type barChartMarshal struct {
	BarDir     *strValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart barDir,omitempty"`
	Grouping   *strValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart grouping,omitempty"`
	GapWidth   *intValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart gapWidth,omitempty"`
	Overlap    *intValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart overlap,omitempty"`
	VaryColors *intValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Series     []seriesMarshal `xml:"ser"`
	AxisID     []intValMarshal `xml:"axId"`
}

type lineChartMarshal struct {
	Grouping *strValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart grouping,omitempty"`
	Series   []seriesMarshal `xml:"ser"`
	AxisID   []intValMarshal `xml:"axId"`
}

type pieChartMarshal struct {
	VaryColors *intValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Series     []seriesMarshal `xml:"ser"`
}

type doughnutChartMarshal struct {
	HoleSize   *intValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart holeSize,omitempty"`
	VaryColors *intValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Series     []seriesMarshal `xml:"ser"`
}

type areaChartMarshal struct {
	Grouping *strValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart grouping,omitempty"`
	Series   []seriesMarshal `xml:"ser"`
	AxisID   []intValMarshal `xml:"axId"`
}

type scatterChartMarshal struct {
	ScatterStyle *strValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart scatterStyle,omitempty"`
	Series       []seriesMarshal `xml:"ser"`
	AxisID       []intValMarshal `xml:"axId"`
}

type bubbleChartMarshal struct {
	Series []seriesMarshal `xml:"ser"`
	AxisID []intValMarshal `xml:"axId"`
}

type stockChartMarshal struct {
	Series     []seriesMarshal    `xml:"ser"`
	AxisID     []intValMarshal    `xml:"axId"`
	DropLines  *linesMarshal      `xml:"dropLines,omitempty"`
	HiLowLines *linesMarshal      `xml:"hiLowLines,omitempty"`
	UpDownBars *upDownBarsMarshal `xml:"upDownBars,omitempty"`
}

type linesMarshal struct {
	SpPr *spPrMarshal `xml:"spPr,omitempty"`
}

type upDownBarsMarshal struct {
	UpBars   *spPrMarshal `xml:"upBars,omitempty"`
	DownBars *spPrMarshal `xml:"downBars,omitempty"`
}

type radarChartMarshal struct {
	RadarStyle *strValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart radarStyle,omitempty"`
	Series     []seriesMarshal `xml:"ser"`
	AxisID     []intValMarshal `xml:"axId"`
}

type surfaceChartMarshal struct {
	Series []seriesMarshal `xml:"ser"`
	AxisID []intValMarshal `xml:"axId"`
}

func makeSeries(c *Chart) []seriesMarshal {
	out := make([]seriesMarshal, len(c.Series))
	for i, s := range c.Series {
		cats := c.Categories
		if len(s.Labels) > 0 {
			cats = s.Labels
		}
		out[i] = makeSeriesMarshal(i, &s, cats)
	}
	return out
}

func makeSeriesMarshal(index int, s *Series, categories []string) seriesMarshal {
	m := seriesMarshal{
		Idx:   intValMarshal{Val: index},
		Order: intValMarshal{Val: index},
	}
	if tx := makeTxValue(s.Name, s.NameFormula); tx != nil {
		m.Tx = tx
	}
	if val := makeNumData(s.Values, s.ValueFormula); val != nil {
		m.Val = val
	}
	if xVal := makeNumData(s.XValues, s.XValueFormula); xVal != nil {
		m.XVal = xVal
	}
	if bubbleSize := makeNumData(s.BubbleSizes, ""); bubbleSize != nil {
		m.BubbleSize = bubbleSize
	}
	if cat := makeStrData(categories, s.LabelFormula); cat != nil {
		m.Cat = cat
	}
	m.SpPr = makeSeriesSpPr(s)
	if s.MarkerShape != "" && s.MarkerShape != "circle" || s.MarkerSize > 0 {
		m.Marker = &markerMarshal{}
		if s.MarkerShape != "" && s.MarkerShape != "circle" {
			m.Marker.Symbol = &strValMarshal{Val: s.MarkerShape}
		}
		if s.MarkerSize > 0 {
			m.Marker.Size = &intValMarshal{Val: s.MarkerSize}
		}
	}
	if s.Smooth {
		m.Smooth = &boolValMarshal{Val: true}
	}
	if s.Explosion > 0 {
		m.Explosion = &intValMarshal{Val: s.Explosion}
	}
	for _, dp := range s.DataPoints {
		dpm := dPtMarshal{Idx: intValMarshal{Val: dp.Index}}
		if dp.FillColor != "" || dp.LineColor != "" {
			var fill *solidFillMarshal
			var line *lineMarshal
			if dp.FillColor != "" {
				fill = &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(dp.FillColor)}}
			}
			if dp.LineColor != "" {
				line = &lineMarshal{SolidFill: &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(dp.LineColor)}}}
			}
			dpm.SpPr = &spPrMarshal{SolidFill: fill, Line: line}
		}
		if dp.Explosion > 0 {
			dpm.Explosion = &intValMarshal{Val: dp.Explosion}
		}
		m.DPt = append(m.DPt, dpm)
	}
	m.DLbls = makeDLblsMarshal(s)
	if s.ErrorBars != nil {
		m.ErrBars = makeErrBarsMarshal(s.ErrorBars)
	}
	if s.Trendline != nil {
		m.Trendline = makeTrendlineMarshal(s.Trendline)
	}
	return m
}

func makeDLblsMarshal(s *Series) *dLblsMarshal {
	if !s.ShowDataLabels && !s.ShowCatName && !s.ShowSerName && !s.ShowPercent && !s.ShowLeaderLines && !s.ShowBubbleSize && s.Separator == "" && s.LabelPosition == 0 && s.LabelStyle == nil && len(s.DataPoints) == 0 {
		return nil
	}
	dm := &dLblsMarshal{}
	if s.ShowDataLabels {
		dm.ShowVal = &boolValMarshal{Val: true}
	}
	if s.ShowCatName {
		dm.ShowCatName = &boolValMarshal{Val: true}
	}
	if s.ShowSerName {
		dm.ShowSerName = &boolValMarshal{Val: true}
	}
	if s.ShowPercent {
		dm.ShowPercent = &boolValMarshal{Val: true}
	}
	if s.ShowLeaderLines {
		dm.ShowLeaderLines = &boolValMarshal{Val: true}
	}
	if s.ShowBubbleSize {
		dm.ShowBubbleSize = &boolValMarshal{Val: true}
	}
	if s.Separator != "" {
		dm.Separator = &strValMarshal{Val: s.Separator}
	}
	if s.LabelPosition != 0 {
		dm.LblPos = &strValMarshal{Val: intToDLblPos(s.LabelPosition)}
	}
	if s.NumFmt != "" {
		dm.NumFmt = &numFmtMarshal{FormatCode: s.NumFmt}
	}
	if s.LabelStyle != nil {
		dm.TxPr = makeTxPrMarshal(s.LabelStyle)
	}
	for _, dp := range s.DataPoints {
		if dp.Label != "" || dp.NumFmt != "" || dp.LabelStyle != nil {
			dlm := dLblMarshal{Idx: intValMarshal{Val: dp.Index}}
			if dp.Label != "" {
				dlm.Tx = &txValueMarshal{V: dp.Label}
			}
			if dp.NumFmt != "" {
				dlm.NumFmt = &numFmtMarshal{FormatCode: dp.NumFmt}
			}
			if dp.LabelStyle != nil {
				dlm.TxPr = makeTxPrMarshal(dp.LabelStyle)
			}
			dm.DLbl = append(dm.DLbl, dlm)
		}
	}
	if dm.ShowVal == nil && dm.ShowCatName == nil && dm.ShowSerName == nil && dm.ShowPercent == nil && dm.ShowLeaderLines == nil && dm.ShowBubbleSize == nil && dm.Separator == nil && dm.LblPos == nil && dm.TxPr == nil && len(dm.DLbl) == 0 {
		return nil
	}
	return dm
}

func makeTxPrMarshal(tp *TextProperties) *txPrMarshal {
	if tp == nil {
		return nil
	}
	m := &txPrMarshal{
		DefRPr: &defRPrMarshal{
			Sz: tp.FontSize,
			B:  tp.Bold,
			I:  tp.Italic,
		},
	}
	if tp.Color != "" {
		m.DefRPr.SolidFill = &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(tp.Color)}}
	}
	return m
}

func intToDLblPos(i int) string {
	switch i {
	case 1:
		return "outEnd"
	case 2:
		return "inEnd"
	case 3:
		return "ctr"
	case 4:
		return "inBase"
	}
	return ""
}

func makeSeriesSpPr(s *Series) *spPrMarshal {
	var fill *solidFillMarshal
	var line *lineMarshal

	// Determine effective fill color.
	fillColor := s.FillColor
	if fillColor == "" {
		fillColor = s.Color
	}
	if fillColor != "" {
		fill = &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(fillColor)}}
	}

	if s.LineColor != "" {
		line = &lineMarshal{
			Width:     s.LineWidth,
			SolidFill: &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: normalizeHexColor(s.LineColor)}},
		}
	}

	if fill == nil && line == nil {
		return nil
	}
	return &spPrMarshal{SolidFill: fill, Line: line}
}

func normalizeHexColor(c string) string {
	c = strings.TrimPrefix(c, "#")
	if len(c) == 6 {
		return c
	}
	if len(c) == 3 {
		return string([]byte{c[0], c[0], c[1], c[1], c[2], c[2]})
	}
	return c
}

type seriesMarshal struct {
	Idx        intValMarshal     `xml:"idx"`
	Order      intValMarshal     `xml:"order"`
	Tx         *txValueMarshal   `xml:"tx,omitempty"`
	Cat        *catMarshal       `xml:"cat,omitempty"`
	Val        *valMarshal       `xml:"val,omitempty"`
	XVal       *valMarshal       `xml:"xVal,omitempty"`
	BubbleSize *valMarshal       `xml:"bubbleSize,omitempty"`
	SpPr       *spPrMarshal      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spPr,omitempty"`
	Marker     *markerMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart marker,omitempty"`
	DLbls      *dLblsMarshal     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	Smooth     *boolValMarshal   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart smooth,omitempty"`
	Explosion  *intValMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart explosion,omitempty"`
	ErrBars    *errBarsMarshal   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errBars,omitempty"`
	Trendline  *trendlineMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart trendline,omitempty"`
	DPt        []dPtMarshal      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dPt,omitempty"`
}

type dPtMarshal struct {
	Idx       intValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx"`
	SpPr      *spPrMarshal   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spPr,omitempty"`
	Explosion *intValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart explosion,omitempty"`
}

type markerMarshal struct {
	Symbol *strValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart symbol,omitempty"`
	Size   *intValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart size,omitempty"`
}

type trendlineMarshal struct {
	TrendlineType *strValMarshal   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart trendlineType,omitempty"`
	Order         *intValMarshal   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart order,omitempty"`
	Period        *intValMarshal   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart period,omitempty"`
	Forward       *floatValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart forward,omitempty"`
	Backward      *floatValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart backward,omitempty"`
	Intercept     *floatValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart intercept,omitempty"`
	DispRSqr      *boolValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dispRSqr,omitempty"`
	DispEq        *boolValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dispEq,omitempty"`
	SpPr          *spPrMarshal     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spPr,omitempty"`
}

type txValueMarshal struct {
	StrRef *strRefMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart strRef,omitempty"`
	V      string         `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart v,omitempty"`
}

type catMarshal struct {
	StrRef *strRefMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart strRef,omitempty"`
}

type valMarshal struct {
	NumRef *numRefMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numRef,omitempty"`
}

type strRefMarshal struct {
	Formula  string          `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart f"`
	StrCache strCacheMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart strCache"`
}

type numRefMarshal struct {
	Formula  string          `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart f"`
	NumCache numCacheMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numCache"`
}

type strCacheMarshal struct {
	PtCount intValMarshal  `xml:"ptCount"`
	Pts     []strPtMarshal `xml:"pt"`
}

type numCacheMarshal struct {
	PtCount intValMarshal  `xml:"ptCount"`
	Pts     []numPtMarshal `xml:"pt"`
}

type strPtMarshal struct {
	Idx intValMarshal `xml:"idx"`
	V   string        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart v"`
}

type numPtMarshal struct {
	Idx intValMarshal `xml:"idx"`
	V   float64       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart v"`
}

func makeTxValue(value, formula string) *txValueMarshal {
	if formula == "" && value == "" {
		return nil
	}
	if formula == "" {
		return &txValueMarshal{V: value}
	}
	return &txValueMarshal{
		StrRef: &strRefMarshal{
			Formula: formula,
			StrCache: strCacheMarshal{
				PtCount: intValMarshal{Val: 1},
				Pts:     []strPtMarshal{{Idx: intValMarshal{Val: 0}, V: value}},
			},
		},
	}
}

func makeStrLit(values []string) *catMarshal {
	return makeStrData(values, "")
}

func makeStrData(values []string, formula string) *catMarshal {
	if len(values) == 0 && formula == "" {
		return nil
	}
	pts := make([]strPtMarshal, len(values))
	for i, v := range values {
		pts[i] = strPtMarshal{Idx: intValMarshal{Val: i}, V: v}
	}
	return &catMarshal{
		StrRef: &strRefMarshal{
			Formula: formula,
			StrCache: strCacheMarshal{
				PtCount: intValMarshal{Val: len(values)},
				Pts:     pts,
			},
		},
	}
}

func makeNumLit(values []float64) *valMarshal {
	return makeNumData(values, "")
}

func makeNumData(values []float64, formula string) *valMarshal {
	if len(values) == 0 && formula == "" {
		return nil
	}
	pts := make([]numPtMarshal, len(values))
	for i, v := range values {
		pts[i] = numPtMarshal{Idx: intValMarshal{Val: i}, V: v}
	}
	return &valMarshal{
		NumRef: &numRefMarshal{
			Formula: formula,
			NumCache: numCacheMarshal{
				PtCount: intValMarshal{Val: len(values)},
				Pts:     pts,
			},
		},
	}
}

type spPrMarshal struct {
	SolidFill *solidFillMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	Line      *lineMarshal      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ln,omitempty"`
}

type solidFillMarshal struct {
	SRgbClr *srgbClrMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr"`
}

type srgbClrMarshal struct {
	Val string `xml:"val,attr"`
}

type lineMarshal struct {
	Width     int               `xml:"w,attr,omitempty"`
	SolidFill *solidFillMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
}

type dLblsMarshal struct {
	ShowVal         *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showVal,omitempty"`
	ShowCatName     *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showCatName,omitempty"`
	ShowSerName     *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showSerName,omitempty"`
	ShowPercent     *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showPercent,omitempty"`
	ShowBubbleSize  *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showBubbleSize,omitempty"`
	ShowLeaderLines *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showLeaderLines,omitempty"`
	Separator       *strValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart separator,omitempty"`
	LblPos          *strValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLblPos,omitempty"`
	NumFmt          *numFmtMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numFmt,omitempty"`
	DLbl            []dLblMarshal   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbl,omitempty"`
	TxPr            *txPrMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
}

type dLblMarshal struct {
	Idx         intValMarshal   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx"`
	Tx          *txValueMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	NumFmt      *numFmtMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numFmt,omitempty"`
	SpPr        *spPrMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spPr,omitempty"`
	DLblPos     *strValMarshal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLblPos,omitempty"`
	ShowVal     *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showVal,omitempty"`
	ShowCatName *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showCatName,omitempty"`
	ShowSerName *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showSerName,omitempty"`
	ShowPercent *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showPercent,omitempty"`
	TxPr        *txPrMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
}

type legendMarshal struct {
	LegendPos     *strValMarshal       `xml:"legendPos"`
	Layout        *layoutMarshal       `xml:"layout,omitempty"`
	LegendEntries []legendEntryMarshal `xml:"legendEntry,omitempty"`
}

type legendEntryMarshal struct {
	Idx    intValMarshal   `xml:"idx"`
	Delete *boolValMarshal `xml:"delete,omitempty"`
}

type layoutMarshal struct {
	ManualLayout *manualLayoutMarshal `xml:"manualLayout,omitempty"`
}

type manualLayoutMarshal struct {
	LayoutTarget *strValMarshal   `xml:"layoutTarget,omitempty"`
	XMode        *strValMarshal   `xml:"xMode,omitempty"`
	YMode        *strValMarshal   `xml:"yMode,omitempty"`
	WMode        *strValMarshal   `xml:"wMode,omitempty"`
	HMode        *strValMarshal   `xml:"hMode,omitempty"`
	X            *floatValMarshal `xml:"x,omitempty"`
	Y            *floatValMarshal `xml:"y,omitempty"`
	W            *floatValMarshal `xml:"w,omitempty"`
	H            *floatValMarshal `xml:"h,omitempty"`
}

func makeLayout(m *ManualLayout) *layoutMarshal {
	if m == nil {
		return nil
	}
	lm := &manualLayoutMarshal{}
	if m.LayoutTarget != "" {
		lm.LayoutTarget = &strValMarshal{Val: m.LayoutTarget}
	}
	if m.XMode != "" {
		lm.XMode = &strValMarshal{Val: m.XMode}
	}
	if m.YMode != "" {
		lm.YMode = &strValMarshal{Val: m.YMode}
	}
	if m.WMode != "" {
		lm.WMode = &strValMarshal{Val: m.WMode}
	}
	if m.HMode != "" {
		lm.HMode = &strValMarshal{Val: m.HMode}
	}
	if m.X != 0 {
		lm.X = &floatValMarshal{Val: m.X}
	}
	if m.Y != 0 {
		lm.Y = &floatValMarshal{Val: m.Y}
	}
	if m.W != 0 {
		lm.W = &floatValMarshal{Val: m.W}
	}
	if m.H != 0 {
		lm.H = &floatValMarshal{Val: m.H}
	}
	return &layoutMarshal{ManualLayout: lm}
}

type catAxisMarshal struct {
	AxID          intValMarshal   `xml:"axId"`
	Scaling       scalingMarshal  `xml:"scaling"`
	Delete        *boolValMarshal `xml:"delete"`
	Title         *titleMarshal   `xml:"title,omitempty"`
	AxPos         *strValMarshal  `xml:"axPos"`
	MajorTickMark *strValMarshal  `xml:"majorTickMark,omitempty"`
	MinorTickMark *strValMarshal  `xml:"minorTickMark,omitempty"`
	TickLblPos    *strValMarshal  `xml:"tickLblPos"`
	CrossAx       intValMarshal   `xml:"crossAx"`
	Crosses       *strValMarshal  `xml:"crosses"`
	Auto          *boolValMarshal `xml:"auto"`
	LblAlgn       *strValMarshal  `xml:"lblAlgn"`
	LblOffset     *intValMarshal  `xml:"lblOffset"`
	NumFmt        *numFmtMarshal  `xml:"numFmt,omitempty"`
}

type valAxisMarshal struct {
	AxID           intValMarshal     `xml:"axId"`
	Scaling        scalingMarshal    `xml:"scaling"`
	Delete         *boolValMarshal   `xml:"delete"`
	Title          *titleMarshal     `xml:"title,omitempty"`
	AxPos          *strValMarshal    `xml:"axPos"`
	MajorGridlines *struct{}         `xml:"majorGridlines,omitempty"`
	MajorTickMark  *strValMarshal    `xml:"majorTickMark,omitempty"`
	MinorTickMark  *strValMarshal    `xml:"minorTickMark,omitempty"`
	TickLblPos     *strValMarshal    `xml:"tickLblPos"`
	CrossAx        intValMarshal     `xml:"crossAx"`
	Crosses        *strValMarshal    `xml:"crosses"`
	CrossBetween   *strValMarshal    `xml:"crossBetween"`
	MajorUnit      *doubleValMarshal `xml:"majorUnit,omitempty"`
	MinorUnit      *doubleValMarshal `xml:"minorUnit,omitempty"`
	NumFmt         *numFmtMarshal    `xml:"numFmt,omitempty"`
}

type scalingMarshal struct {
	Orientation *strValMarshal    `xml:"orientation"`
	Min         *doubleValMarshal `xml:"min,omitempty"`
	Max         *doubleValMarshal `xml:"max,omitempty"`
}

type numFmtMarshal struct {
	FormatCode   string `xml:"formatCode,attr"`
	SourceLinked bool   `xml:"sourceLinked,attr"`
}

type txPrMarshal struct {
	DefRPr *defRPrMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/main defRPr,omitempty"`
}

type defRPrMarshal struct {
	Sz        int               `xml:"sz,attr,omitempty"`
	B         bool              `xml:"b,attr,omitempty"`
	I         bool              `xml:"i,attr,omitempty"`
	SolidFill *solidFillMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
}

type doubleValMarshal struct {
	Val float64 `xml:"val,attr"`
}

func intToTickMark(i int) string {
	switch i {
	case 1:
		return "out"
	case 2:
		return "in"
	case 3:
		return "cross"
	}
	return "none"
}

func intToLabelPosition(i int) string {
	switch i {
	case 1:
		return "low"
	case 2:
		return "high"
	case 3:
		return "nextTo"
	}
	return "none"
}

func makeCatAxis(id, crossAx int, cfg Axis) catAxisMarshal {
	sc := scalingMarshal{Orientation: &strValMarshal{Val: "minMax"}}
	if cfg.Orientation != "" {
		sc.Orientation = &strValMarshal{Val: cfg.Orientation}
	}
	if cfg.Min != 0 {
		sc.Min = &doubleValMarshal{Val: cfg.Min}
	}
	if cfg.Max != 0 {
		sc.Max = &doubleValMarshal{Val: cfg.Max}
	}
	ax := catAxisMarshal{
		AxID:       intValMarshal{Val: id},
		Scaling:    sc,
		Delete:     &boolValMarshal{Val: false},
		AxPos:      &strValMarshal{Val: "b"},
		TickLblPos: &strValMarshal{Val: "nextTo"},
		CrossAx:    intValMarshal{Val: crossAx},
		Crosses:    &strValMarshal{Val: "autoZero"},
		Auto:       &boolValMarshal{Val: true},
		LblAlgn:    &strValMarshal{Val: "ctr"},
		LblOffset:  &intValMarshal{Val: 100},
	}
	if cfg.MajorTickType != 0 {
		ax.MajorTickMark = &strValMarshal{Val: intToTickMark(cfg.MajorTickType)}
	}
	if cfg.MinorTickType != 0 {
		ax.MinorTickMark = &strValMarshal{Val: intToTickMark(cfg.MinorTickType)}
	}
	if cfg.LabelPosition != 0 {
		ax.TickLblPos = &strValMarshal{Val: intToLabelPosition(cfg.LabelPosition)}
	}
	if cfg.NumFmt != "" {
		ax.NumFmt = &numFmtMarshal{FormatCode: cfg.NumFmt}
	}
	if cfg.Title != "" {
		ax.Title = makeTitle(cfg.Title, nil)
	}
	return ax
}

func makeValAxis(id, crossAx int, cfg Axis) valAxisMarshal {
	sc := scalingMarshal{Orientation: &strValMarshal{Val: "minMax"}}
	if cfg.Orientation != "" {
		sc.Orientation = &strValMarshal{Val: cfg.Orientation}
	}
	if cfg.Min != 0 {
		sc.Min = &doubleValMarshal{Val: cfg.Min}
	}
	if cfg.Max != 0 {
		sc.Max = &doubleValMarshal{Val: cfg.Max}
	}
	ax := valAxisMarshal{
		AxID:         intValMarshal{Val: id},
		Scaling:      sc,
		Delete:       &boolValMarshal{Val: false},
		AxPos:        &strValMarshal{Val: "l"},
		TickLblPos:   &strValMarshal{Val: "nextTo"},
		CrossAx:      intValMarshal{Val: crossAx},
		Crosses:      &strValMarshal{Val: "autoZero"},
		CrossBetween: &strValMarshal{Val: "between"},
	}
	if cfg.ShowGridlines {
		ax.MajorGridlines = &struct{}{}
	}
	if cfg.MajorTickType != 0 {
		ax.MajorTickMark = &strValMarshal{Val: intToTickMark(cfg.MajorTickType)}
	}
	if cfg.MinorTickType != 0 {
		ax.MinorTickMark = &strValMarshal{Val: intToTickMark(cfg.MinorTickType)}
	}
	if cfg.LabelPosition != 0 {
		ax.TickLblPos = &strValMarshal{Val: intToLabelPosition(cfg.LabelPosition)}
	}
	if cfg.MajorUnit != 0 {
		ax.MajorUnit = &doubleValMarshal{Val: cfg.MajorUnit}
	}
	if cfg.MinorUnit != 0 {
		ax.MinorUnit = &doubleValMarshal{Val: cfg.MinorUnit}
	}
	if cfg.NumFmt != "" {
		ax.NumFmt = &numFmtMarshal{FormatCode: cfg.NumFmt}
	}
	if cfg.Title != "" {
		ax.Title = makeTitle(cfg.Title, nil)
	}
	return ax
}

type errBarsMarshal struct {
	ErrDir     *strValMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errDir,omitempty"`
	ErrBarType *strValMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errBarType"`
	ErrValType *strValMarshal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errValType"`
	NoEndCap   *boolValMarshal   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart noEndCap,omitempty"`
	Plus       *valMarshal       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart plus,omitempty"`
	Minus      *valMarshal       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minus,omitempty"`
	Val        *doubleValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart val,omitempty"`
}

type dTableMarshal struct {
	ShowHorzBorder *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showHorzBorder,omitempty"`
	ShowVertBorder *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showVertBorder,omitempty"`
	ShowOutline    *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showOutline,omitempty"`
	ShowKeys       *boolValMarshal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showKeys,omitempty"`
}

func makeErrBarsMarshal(eb *ErrorBars) *errBarsMarshal {
	m := &errBarsMarshal{}
	if eb.Direction != "" {
		m.ErrDir = &strValMarshal{Val: eb.Direction}
	}
	if eb.BarType != "" {
		m.ErrBarType = &strValMarshal{Val: eb.BarType}
	}
	if eb.ValType != "" {
		m.ErrValType = &strValMarshal{Val: eb.ValType}
	}
	if eb.NoEndCap {
		m.NoEndCap = &boolValMarshal{Val: true}
	}
	if eb.Value != 0 {
		m.Val = &doubleValMarshal{Val: eb.Value}
	}
	if len(eb.Plus) > 0 {
		m.Plus = makeNumLit(eb.Plus)
	}
	if len(eb.Minus) > 0 {
		m.Minus = makeNumLit(eb.Minus)
	}
	return m
}

func makeTrendlineMarshal(tl *Trendline) *trendlineMarshal {
	m := &trendlineMarshal{}
	if tl.Type != "" {
		m.TrendlineType = &strValMarshal{Val: tl.Type}
	}
	if tl.Order > 0 {
		m.Order = &intValMarshal{Val: tl.Order}
	}
	if tl.Period > 0 {
		m.Period = &intValMarshal{Val: tl.Period}
	}
	if tl.Forward != 0 {
		m.Forward = &floatValMarshal{Val: tl.Forward}
	}
	if tl.Backward != 0 {
		m.Backward = &floatValMarshal{Val: tl.Backward}
	}
	if tl.Intercept != nil {
		m.Intercept = &floatValMarshal{Val: *tl.Intercept}
	}
	if tl.ShowRSquared {
		m.DispRSqr = &boolValMarshal{Val: true}
	}
	if tl.ShowEquation {
		m.DispEq = &boolValMarshal{Val: true}
	}
	if tl.Color != "" || tl.LineWidth > 0 {
		m.SpPr = &spPrMarshal{}
		if tl.Color != "" {
			m.SpPr.Line = &lineMarshal{SolidFill: &solidFillMarshal{SRgbClr: &srgbClrMarshal{Val: tl.Color}}}
		}
		if tl.LineWidth > 0 {
			if m.SpPr.Line == nil {
				m.SpPr.Line = &lineMarshal{}
			}
			m.SpPr.Line.Width = tl.LineWidth
		}
	}
	return m
}
