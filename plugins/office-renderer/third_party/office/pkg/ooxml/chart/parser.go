package chart

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
)

// ParseChart parses a chart XML document into a Chart structure.
func ParseChart(data []byte) (*Chart, error) {
	var part ChartPart
	if err := xml.Unmarshal(data, &part); err != nil {
		return nil, fmt.Errorf("unmarshal chart: %w", err)
	}

	return part.ToChart()
}

// ToChart converts a ChartPart to a Chart.
func (cp ChartPart) ToChart() (*Chart, error) {
	cs := cp.ChartSpace
	chart := &Chart{}

	// Extract title
	if cs.Title != nil {
		chart.Title = cs.Title.PlainText()
		if chart.Title == "" {
			chart.Title = cs.Title.CachedText()
		}
		if cs.Title.Layout != nil && cs.Title.Layout.ManualLayout != nil {
			chart.TitleLayout = extractManualLayout(cs.Title.Layout.ManualLayout)
		}
	}

	// Extract legend
	if cs.Legend != nil {
		pos := ""
		if cs.Legend.LegendPos != nil {
			pos = cs.Legend.LegendPos.Val
		}
		chart.Legend = Legend{
			Position: pos,
			Show:     pos != "none",
		}
		if cs.Legend.Layout != nil && cs.Legend.Layout.ManualLayout != nil {
			chart.LegendLayout = extractManualLayout(cs.Legend.Layout.ManualLayout)
		}
		for _, entry := range cs.Legend.LegendEntries {
			e := LegendEntry{}
			if entry.Index != nil {
				e.Index = entry.Index.Val
			}
			if entry.Delete != nil {
				e.Delete = entry.Delete.Val
			}
			chart.Legend.Entries = append(chart.Legend.Entries, e)
		}
	}

	// Prefer chartSpace-level background and fall back to the nested chart node
	// so both parse paths are preserved.
	if fill, ref := extractSolidFillColor(cp.SpPr); fill != "" || ref != nil {
		chart.ChartArea.FillColor = fill
		chart.ChartArea.fillRef = ref
	} else if fill, ref := extractSolidFillColor(cs.ChartArea); fill != "" || ref != nil {
		chart.ChartArea.FillColor = fill
		chart.ChartArea.fillRef = ref
	}

	// Extract all series from all chart types in the plot area (combo chart support)
	chart.Series = chart.extractAllSeries(cs.PlotArea)

	// Determine primary chart type from the first present chart type
	chart.ChartType = chart.inferPrimaryChartType(cs.PlotArea)

	// Extract stock chart configuration
	if sc := cs.PlotArea.StockChart; sc != nil {
		chart.StockConfig = &StockConfig{}
		if sc.DropLines != nil {
			chart.StockConfig.ShowDropLines = true
		}
		if sc.HiLowLines != nil {
			chart.StockConfig.ShowHiLowLines = true
		}
		if sc.UpDownBars != nil {
			if sc.UpDownBars.UpBars != nil && sc.UpDownBars.UpBars.SolidFill != nil {
				ref := drawingxml.ResolveSolidFill(sc.UpDownBars.UpBars.SolidFill, nil)
				chart.StockConfig.UpBarColor = fallbackChartColor(ref)
			}
			if sc.UpDownBars.DownBars != nil && sc.UpDownBars.DownBars.SolidFill != nil {
				ref := drawingxml.ResolveSolidFill(sc.UpDownBars.DownBars.SolidFill, nil)
				chart.StockConfig.DownBarColor = fallbackChartColor(ref)
			}
		}
	}

	// Derive legacy chart-level options from the first matching group.
	for _, g := range chart.Groups {
		switch g.ChartType {
		case ChartBar, ChartColumn, ChartBar3D, ChartColumn3D:
			if g.GapWidth != 0 {
				chart.GapWidth = g.GapWidth
			}
			if g.Overlap != 0 {
				chart.Overlap = g.Overlap
			}
			if g.VaryColors {
				chart.VaryColors = true
			}
		case ChartPie, ChartPie3D:
			if g.VaryColors {
				chart.VaryColors = true
			}
		case ChartDoughnut:
			if g.HoleSize != 0 {
				chart.HoleSize = g.HoleSize
			}
			if g.VaryColors {
				chart.VaryColors = true
			}
		}
	}

	// Extract 3D view settings
	if cs.View3D != nil {
		chart.View3D = &View3D{}
		if cs.View3D.RotX != nil {
			chart.View3D.RotX = cs.View3D.RotX.Val
		}
		if cs.View3D.RotY != nil {
			chart.View3D.RotY = cs.View3D.RotY.Val
		}
		if cs.View3D.Perspective != nil {
			chart.View3D.Perspective = cs.View3D.Perspective.Val
		}
		if cs.View3D.DepthPct != nil {
			chart.View3D.DepthPercent = cs.View3D.DepthPct.Val
		}
		if cs.View3D.RightAngle != nil {
			chart.View3D.RightAngleAxes = cs.View3D.RightAngle.Val
		}
	}

	// Extract data table settings
	if cs.PlotArea.DTable != nil {
		chart.DataTable = &DataTable{}
		if cs.PlotArea.DTable.ShowHorzBorder != nil {
			chart.DataTable.ShowHorzBorder = cs.PlotArea.DTable.ShowHorzBorder.Val
		}
		if cs.PlotArea.DTable.ShowVertBorder != nil {
			chart.DataTable.ShowVertBorder = cs.PlotArea.DTable.ShowVertBorder.Val
		}
		if cs.PlotArea.DTable.ShowOutline != nil {
			chart.DataTable.ShowOutline = cs.PlotArea.DTable.ShowOutline.Val
		}
		if cs.PlotArea.DTable.ShowKeys != nil {
			chart.DataTable.ShowKeys = cs.PlotArea.DTable.ShowKeys.Val
		}
	}

	if cs.PlotArea.SpPr != nil {
		if fill, _ := extractSolidFillColor(cs.PlotArea.SpPr); fill != "" {
			chart.PlotArea.FillColor = fill
		}
		if line := cs.PlotArea.SpPr.Ln; line != nil {
			chart.PlotArea.ShowBorder = true
			if line.SolidFill != nil {
				if ref := drawingxml.ResolveSolidFill(line.SolidFill, nil); ref != nil {
					chart.PlotArea.BorderColor = fallbackChartColor(ref)
				}
			}
		}
	}

	// Extract plot area manual layout
	if cs.PlotArea.Layout != nil && cs.PlotArea.Layout.ManualLayout != nil {
		chart.ManualLayout = extractManualLayout(cs.PlotArea.Layout.ManualLayout)
	}

	// Extract displayBlanksAs
	if cs.DisplayBlanksAs != nil {
		chart.DisplayBlanksAs = cs.DisplayBlanksAs.Val
	}

	// Extract categories from first series
	if len(chart.Series) > 0 && len(chart.Series[0].Labels) > 0 {
		chart.Categories = chart.Series[0].Labels
	}

	// Extract axes from plotArea
	for _, ax := range cs.PlotArea.CatAxes {
		ax.Type = "catAx"
		chart.XAxis = extractAxis(ax)
	}
	for _, ax := range cs.PlotArea.DateAxes {
		ax.Type = "dateAx"
		chart.XAxis = extractAxis(ax)
	}
	for i, ax := range cs.PlotArea.ValAxes {
		ax.Type = "valAx"
		if i == 0 {
			chart.YAxis = extractAxis(ax)
		} else if i == 1 {
			chart.YAxis2 = extractAxis(ax)
		}
	}

	return chart, nil
}

func extractSolidFillColor(spPr *SpPrElement) (string, *drawing.SolidFill) {
	if spPr == nil || spPr.SolidFill == nil {
		return "", nil
	}
	ref := drawingxml.ResolveSolidFill(spPr.SolidFill, nil)
	if ref == nil {
		return "", nil
	}
	if color := fallbackChartColor(ref); color != "" {
		return color, ref
	}
	return "", ref
}

func fallbackChartColor(fill *drawing.SolidFill) string {
	if fill == nil {
		return ""
	}
	if color := strings.TrimSpace(fill.Color); color != "" {
		return color
	}
	if theme := strings.TrimSpace(fill.ThemeColor); theme != "" {
		return "scheme:" + theme
	}
	return ""
}

// dashPatternToInt maps OOXML preset dash names to a simple integer pattern
// identifier stored in Series.LinePattern.
func dashPatternToInt(name string) int {
	switch strings.ToLower(name) {
	case "solid":
		return 0
	case "sysdash", "dash":
		return 1
	case "sysdot", "dot":
		return 2
	case "dashdot":
		return 3
	case "lgdash":
		return 4
	case "lgdashdot":
		return 5
	case "lgdashdotdot":
		return 6
	}
	return 0
}

func hasExplicitAlpha(fill *SolidFill) bool {
	if fill == nil {
		return false
	}
	switch {
	case fill.SRGBClr != nil:
		return fill.SRGBClr.Alpha != nil || fill.SRGBClr.AlphaMod != nil || fill.SRGBClr.AlphaOff != nil
	case fill.SchemeClr != nil:
		return fill.SchemeClr.Alpha != nil || fill.SchemeClr.AlphaMod != nil || fill.SchemeClr.AlphaOff != nil
	}
	return false
}

func axisElementID(elem AxisElement) int {
	if elem.AxisRef != nil && elem.AxisRef.Val != 0 {
		return elem.AxisRef.Val
	}
	return elem.AxisID
}

type seriesExtractOptions struct {
	useSecondaryAxis bool
	scatterStyle     string // deprecated: will be copied to Series.ScatterStyle
}

// extractAllSeries collects series from all chart types in the plot area.
// It also populates c.Groups with one ChartGroup per chart-type element.
func (c *Chart) extractAllSeries(pa PlotAreaElement) []Series {
	var all []Series
	secondaryValAxisID := 0
	if len(pa.ValAxes) > 1 {
		secondaryValAxisID = axisElementID(pa.ValAxes[1])
	}
	add := func(ct ChartType, elems []SeriesElement, axisIDs []AxisID, opts seriesExtractOptions, makeGroup func(startIdx int) ChartGroup) {
		opts.useSecondaryAxis = groupUsesSecondaryValueAxis(axisIDs, secondaryValAxisID)
		series := extractSeries(elems, ct, opts)
		if len(series) == 0 && len(elems) > 0 {
			// XML had ser elements but no cached data; still create the group.
		}
		startIdx := len(all)
		all = append(all, series...)
		g := makeGroup(startIdx)
		g.ChartType = ct
		g.SeriesIndices = make([]int, len(series))
		for i := range series {
			g.SeriesIndices[i] = startIdx + i
		}
		for _, aid := range axisIDs {
			g.AxisIDs = append(g.AxisIDs, aid.Val)
		}
		c.Groups = append(c.Groups, g)
	}
	if pa.BarChart != nil {
		ct := c.extractBarChartType(pa.BarChart)
		add(ct, pa.BarChart.Series, pa.BarChart.AxisID, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			g := ChartGroup{BarDir: "bar"}
			if pa.BarChart.BarDir != nil {
				g.BarDir = pa.BarChart.BarDir.Val
			}
			if pa.BarChart.Grouping != nil {
				g.Grouping = pa.BarChart.Grouping.Val
			}
			if pa.BarChart.GapWidth != nil {
				g.GapWidth = pa.BarChart.GapWidth.Val
			}
			if pa.BarChart.Overlap != nil {
				g.Overlap = pa.BarChart.Overlap.Val
			}
			if pa.BarChart.VaryColors != nil {
				g.VaryColors = pa.BarChart.VaryColors.Val != 0
			}
			return g
		})
		if pa.BarChart.Grouping != nil && pa.BarChart.Grouping.Val == "stacked" {
			c.Stacked = true
		} else if pa.BarChart.Grouping != nil && pa.BarChart.Grouping.Val == "percentStacked" {
			c.Stacked = true
			c.PercentStacked = true
		}
	}
	if pa.AreaChart != nil {
		add(ChartArea, pa.AreaChart.Series, pa.AreaChart.AxisID, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			g := ChartGroup{}
			if pa.AreaChart.Grouping != nil {
				g.Grouping = pa.AreaChart.Grouping.Val
			}
			return g
		})
		if pa.AreaChart.Grouping != nil && pa.AreaChart.Grouping.Val == "stacked" {
			c.Stacked = true
		} else if pa.AreaChart.Grouping != nil && pa.AreaChart.Grouping.Val == "percentStacked" {
			c.Stacked = true
			c.PercentStacked = true
		}
	}
	if pa.LineChart != nil {
		add(ChartLine, pa.LineChart.Series, pa.LineChart.AxisID, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			g := ChartGroup{}
			if pa.LineChart.Grouping != nil {
				g.Grouping = pa.LineChart.Grouping.Val
			}
			return g
		})
		if pa.LineChart.Grouping != nil && pa.LineChart.Grouping.Val == "stacked" {
			c.Stacked = true
		} else if pa.LineChart.Grouping != nil && pa.LineChart.Grouping.Val == "percentStacked" {
			c.Stacked = true
			c.PercentStacked = true
		}
	}
	if pa.PieChart != nil {
		add(ChartPie, pa.PieChart.Series, nil, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			g := ChartGroup{}
			if pa.PieChart.VaryColors != nil {
				g.VaryColors = pa.PieChart.VaryColors.Val != 0
			}
			return g
		})
	}
	if pa.ScatterChart != nil {
		style := ""
		if pa.ScatterChart.ScatterStyle != nil {
			style = pa.ScatterChart.ScatterStyle.Val
		}
		add(ChartScatter, pa.ScatterChart.Series, pa.ScatterChart.AxisID, seriesExtractOptions{scatterStyle: style}, func(startIdx int) ChartGroup {
			return ChartGroup{}
		})
	}
	if pa.BubbleChart != nil {
		add(ChartBubble, pa.BubbleChart.Series, pa.BubbleChart.AxisID, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			return ChartGroup{}
		})
	}
	if pa.StockChart != nil {
		add(ChartStock, pa.StockChart.Series, pa.StockChart.AxisID, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			return ChartGroup{}
		})
	}
	if pa.RadarChart != nil {
		add(ChartRadar, pa.RadarChart.Series, pa.RadarChart.AxisID, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			g := ChartGroup{}
			if pa.RadarChart.RadarStyle != nil {
				g.Grouping = pa.RadarChart.RadarStyle.Val
			}
			return g
		})
	}
	if pa.SurfaceChart != nil {
		add(ChartSurface, pa.SurfaceChart.Series, pa.SurfaceChart.AxisID, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			return ChartGroup{}
		})
	}
	if pa.DoughnutChart != nil {
		add(ChartDoughnut, pa.DoughnutChart.Series, nil, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			g := ChartGroup{}
			if pa.DoughnutChart.HoleSize != nil {
				g.HoleSize = pa.DoughnutChart.HoleSize.Val
			}
			if pa.DoughnutChart.VaryColors != nil {
				g.VaryColors = pa.DoughnutChart.VaryColors.Val != 0
			}
			return g
		})
	}
	// 3D chart variants: extract series using 2D equivalent types for fallback rendering
	if pa.Bar3DChart != nil {
		ct := c.extractBarChartType(pa.Bar3DChart)
		add(ct, pa.Bar3DChart.Series, pa.Bar3DChart.AxisID, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			g := ChartGroup{BarDir: "col"}
			if pa.Bar3DChart.BarDir != nil {
				g.BarDir = pa.Bar3DChart.BarDir.Val
			}
			if pa.Bar3DChart.Grouping != nil {
				g.Grouping = pa.Bar3DChart.Grouping.Val
			}
			if pa.Bar3DChart.GapWidth != nil {
				g.GapWidth = pa.Bar3DChart.GapWidth.Val
			}
			if pa.Bar3DChart.VaryColors != nil {
				g.VaryColors = pa.Bar3DChart.VaryColors.Val != 0
			}
			return g
		})
		if pa.Bar3DChart.Grouping != nil && pa.Bar3DChart.Grouping.Val == "stacked" {
			c.Stacked = true
		} else if pa.Bar3DChart.Grouping != nil && pa.Bar3DChart.Grouping.Val == "percentStacked" {
			c.Stacked = true
			c.PercentStacked = true
		}
	}
	if pa.Line3DChart != nil {
		add(ChartLine3D, pa.Line3DChart.Series, pa.Line3DChart.AxisID, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			g := ChartGroup{}
			if pa.Line3DChart.Grouping != nil {
				g.Grouping = pa.Line3DChart.Grouping.Val
			}
			return g
		})
		if pa.Line3DChart.Grouping != nil && pa.Line3DChart.Grouping.Val == "stacked" {
			c.Stacked = true
		} else if pa.Line3DChart.Grouping != nil && pa.Line3DChart.Grouping.Val == "percentStacked" {
			c.Stacked = true
			c.PercentStacked = true
		}
	}
	if pa.Pie3DChart != nil {
		add(ChartPie3D, pa.Pie3DChart.Series, nil, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			g := ChartGroup{}
			if pa.Pie3DChart.VaryColors != nil {
				g.VaryColors = pa.Pie3DChart.VaryColors.Val != 0
			}
			return g
		})
	}
	if pa.Area3DChart != nil {
		add(ChartArea3D, pa.Area3DChart.Series, pa.Area3DChart.AxisID, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			g := ChartGroup{}
			if pa.Area3DChart.Grouping != nil {
				g.Grouping = pa.Area3DChart.Grouping.Val
			}
			return g
		})
		if pa.Area3DChart.Grouping != nil && pa.Area3DChart.Grouping.Val == "stacked" {
			c.Stacked = true
		} else if pa.Area3DChart.Grouping != nil && pa.Area3DChart.Grouping.Val == "percentStacked" {
			c.Stacked = true
			c.PercentStacked = true
		}
	}
	if pa.Surface3DChart != nil {
		add(ChartSurface3D, pa.Surface3DChart.Series, pa.Surface3DChart.AxisID, seriesExtractOptions{}, func(startIdx int) ChartGroup {
			return ChartGroup{}
		})
	}
	return all
}

func groupUsesSecondaryValueAxis(axisIDs []AxisID, secondaryValAxisID int) bool {
	if secondaryValAxisID == 0 {
		return false
	}
	for _, axisID := range axisIDs {
		if axisID.Val == secondaryValAxisID {
			return true
		}
	}
	return false
}

// inferPrimaryChartType returns the first chart type found in the plot area.
func (c *Chart) inferPrimaryChartType(pa PlotAreaElement) ChartType {
	switch {
	case pa.BarChart != nil:
		return c.extractBarChartType(pa.BarChart)
	case pa.AreaChart != nil:
		return ChartArea
	case pa.LineChart != nil:
		return ChartLine
	case pa.PieChart != nil:
		return ChartPie
	case pa.ScatterChart != nil:
		return ChartScatter
	case pa.BubbleChart != nil:
		return ChartBubble
	case pa.StockChart != nil:
		return ChartStock
	case pa.RadarChart != nil:
		return ChartRadar
	case pa.SurfaceChart != nil:
		return ChartSurface
	case pa.DoughnutChart != nil:
		return ChartDoughnut
	case pa.Bar3DChart != nil:
		ct := c.extractBarChartType(pa.Bar3DChart)
		if ct == ChartColumn {
			return ChartColumn3D
		}
		return ChartBar3D
	case pa.Line3DChart != nil:
		return ChartLine3D
	case pa.Pie3DChart != nil:
		return ChartPie3D
	case pa.Area3DChart != nil:
		return ChartArea3D
	case pa.Surface3DChart != nil:
		return ChartSurface3D
	default:
		return ChartUnknown
	}
}

// extractBarChartType determines if it's bar or column based on barDir.
func (c *Chart) extractBarChartType(bc *BarChartElement) ChartType {
	if bc == nil {
		return ChartUnknown
	}
	if bc.BarDir != nil && bc.BarDir.Val == "col" {
		return ChartColumn
	}
	return ChartBar
}

// extractSeries converts XML series elements to Series structs.
func extractSeries(elements []SeriesElement, chartType ChartType, opts seriesExtractOptions) []Series {
	var series []Series
	for i, elem := range elements {
		name := elem.Name.PlainText()
		if name == "" {
			name = elem.Name.CachedText()
		}
		s := Series{
			Index:              i,
			ChartType:          chartType,
			Name:               name,
			NameFormula:        elem.Name.FormulaRef(),
			secondaryValueAxis: opts.useSecondaryAxis,
			ScatterStyle:       opts.scatterStyle,
		}

		// Extract values from numRef > numCache
		if elem.Values.NumRef != nil {
			s.ValueFormula = elem.Values.NumRef.Formula
			for _, pt := range elem.Values.NumRef.NumCache.Points {
				s.Values = append(s.Values, pt.Value)
			}
		}

		// Extract X values from xVal > numRef > numCache (scatter charts)
		if elem.XVal != nil && elem.XVal.NumRef != nil {
			s.XValueFormula = elem.XVal.NumRef.Formula
			for _, pt := range elem.XVal.NumRef.NumCache.Points {
				s.XValues = append(s.XValues, pt.Value)
			}
		}

		// Extract bubble sizes from bubbleSize > numRef > numCache (bubble charts)
		if elem.BubbleSize != nil && elem.BubbleSize.NumRef != nil {
			for _, pt := range elem.BubbleSize.NumRef.NumCache.Points {
				s.BubbleSizes = append(s.BubbleSizes, pt.Value)
			}
		}

		// Extract categories/labels from strRef > strCache
		if elem.Cats != nil && elem.Cats.StrRef != nil {
			s.LabelFormula = elem.Cats.StrRef.Formula
			for _, pt := range elem.Cats.StrRef.StrCache.Points {
				s.Labels = append(s.Labels, pt.Value)
			}
		}

		// Extract series colors from shape properties
		if elem.SpPr != nil {
			if elem.SpPr.SolidFill != nil {
				s.fillRef = drawingxml.ResolveSolidFill(elem.SpPr.SolidFill, nil)
				s.FillColor = fallbackChartColor(s.fillRef)
				s.Color = s.FillColor
				if s.fillRef != nil && hasExplicitAlpha(elem.SpPr.SolidFill) {
					s.FillAlpha = s.fillRef.ResolvedAlpha()
				}
			}
			if elem.SpPr.Ln != nil {
				if elem.SpPr.Ln.Width > 0 {
					s.LineWidth = elem.SpPr.Ln.Width
				}
				if elem.SpPr.Ln.SolidFill != nil {
					s.lineRef = drawingxml.ResolveSolidFill(elem.SpPr.Ln.SolidFill, nil)
					s.LineColor = fallbackChartColor(s.lineRef)
				}
				if elem.SpPr.Ln.PrstDash != nil {
					s.LinePattern = dashPatternToInt(elem.SpPr.Ln.PrstDash.Val)
				}
			}
		}

		// Extract marker properties
		if elem.Marker != nil {
			if elem.Marker.Symbol != nil {
				s.MarkerShape = elem.Marker.Symbol.Val
			}
			if elem.Marker.Size != nil {
				s.MarkerSize = elem.Marker.Size.Val
			}
			if elem.Marker.SpPr != nil {
				if elem.Marker.SpPr.SolidFill != nil {
					if ref := drawingxml.ResolveSolidFill(elem.Marker.SpPr.SolidFill, nil); ref != nil {
						s.MarkerColor = fallbackChartColor(ref)
					}
				}
			}
		}

		// Extract smooth line setting
		if elem.Smooth != nil {
			s.Smooth = elem.Smooth.Val
		}

		// Extract pie explosion
		if elem.Explosion != nil {
			s.Explosion = elem.Explosion.Val
		}

		// Extract data label settings
		if elem.DLbls != nil {
			if elem.DLbls.ShowVal != nil {
				s.ShowDataLabels = elem.DLbls.ShowVal.Val
			}
			if elem.DLbls.ShowCatName != nil {
				s.ShowCatName = elem.DLbls.ShowCatName.Val
			}
			if elem.DLbls.ShowSerName != nil {
				s.ShowSerName = elem.DLbls.ShowSerName.Val
			}
			if elem.DLbls.ShowPercent != nil {
				s.ShowPercent = elem.DLbls.ShowPercent.Val
			}
			if elem.DLbls.ShowLeaderLines != nil {
				s.ShowLeaderLines = elem.DLbls.ShowLeaderLines.Val
			}
			if elem.DLbls.ShowBubbleSize != nil {
				s.ShowBubbleSize = elem.DLbls.ShowBubbleSize.Val
			}
			if elem.DLbls.Separator != nil {
				s.Separator = elem.DLbls.Separator.Val
			}
			if elem.DLbls.LblPos != nil {
				s.LabelPosition = dLblPosToInt(elem.DLbls.LblPos.Val)
			}
			if elem.DLbls.NumFmt != nil {
				s.NumFmt = elem.DLbls.NumFmt.FormatCode
			}
			if elem.DLbls.TxPr != nil && elem.DLbls.TxPr.DefRPr != nil {
				s.LabelStyle = extractTextProperties(elem.DLbls.TxPr.DefRPr)
			}
			// Per-point custom labels from c:dLbl inside c:dLbls.
			for _, dLbl := range elem.DLbls.DLbl {
				if dLbl.Idx == nil {
					continue
				}
				idx := dLbl.Idx.Val
				label := ""
				if dLbl.Tx != nil {
					label = dLbl.Tx.PlainText()
					if label == "" {
						label = dLbl.Tx.CachedText()
					}
				}
				// Ensure DataPoints slice is long enough.
				for len(s.DataPoints) <= idx {
					s.DataPoints = append(s.DataPoints, DataPoint{Index: len(s.DataPoints)})
				}
				s.DataPoints[idx].Index = idx
				s.DataPoints[idx].Label = label
				if dLbl.NumFmt != nil {
					s.DataPoints[idx].NumFmt = dLbl.NumFmt.FormatCode
				}
				if dLbl.TxPr != nil && dLbl.TxPr.DefRPr != nil {
					s.DataPoints[idx].LabelStyle = extractTextProperties(dLbl.TxPr.DefRPr)
				}
			}
		}

		// Extract error bars
		if elem.ErrBars != nil {
			s.ErrorBars = &ErrorBars{}
			if elem.ErrBars.ErrDir != nil {
				s.ErrorBars.Direction = elem.ErrBars.ErrDir.Val
			}
			if elem.ErrBars.ErrBarType != nil {
				s.ErrorBars.BarType = elem.ErrBars.ErrBarType.Val
			}
			if elem.ErrBars.ErrValType != nil {
				s.ErrorBars.ValType = elem.ErrBars.ErrValType.Val
			}
			if elem.ErrBars.NoEndCap != nil {
				s.ErrorBars.NoEndCap = elem.ErrBars.NoEndCap.Val
			}
			if elem.ErrBars.Val != nil {
				s.ErrorBars.Value = elem.ErrBars.Val.Val
			}
			if elem.ErrBars.Plus != nil && elem.ErrBars.Plus.NumRef != nil {
				for _, pt := range elem.ErrBars.Plus.NumRef.NumCache.Points {
					s.ErrorBars.Plus = append(s.ErrorBars.Plus, pt.Value)
				}
			}
			if elem.ErrBars.Minus != nil && elem.ErrBars.Minus.NumRef != nil {
				for _, pt := range elem.ErrBars.Minus.NumRef.NumCache.Points {
					s.ErrorBars.Minus = append(s.ErrorBars.Minus, pt.Value)
				}
			}
		}

		// Extract trendline
		if elem.Trendline != nil {
			t := &Trendline{}
			if elem.Trendline.TrendlineType != nil {
				t.Type = elem.Trendline.TrendlineType.Val
			}
			if elem.Trendline.Order != nil {
				t.Order = elem.Trendline.Order.Val
			}
			if elem.Trendline.Period != nil {
				t.Period = elem.Trendline.Period.Val
			}
			if elem.Trendline.Forward != nil {
				t.Forward = elem.Trendline.Forward.Val
			}
			if elem.Trendline.Backward != nil {
				t.Backward = elem.Trendline.Backward.Val
			}
			if elem.Trendline.Intercept != nil {
				v := elem.Trendline.Intercept.Val
				t.Intercept = &v
			}
			if elem.Trendline.DispRSqr != nil {
				t.ShowRSquared = elem.Trendline.DispRSqr.Val
			}
			if elem.Trendline.DispEq != nil {
				t.ShowEquation = elem.Trendline.DispEq.Val
			}
			if elem.Trendline.SpPr != nil && elem.Trendline.SpPr.Ln != nil {
				t.LineWidth = elem.Trendline.SpPr.Ln.Width
				if elem.Trendline.SpPr.Ln.SolidFill != nil {
					t.colorRef = drawingxml.ResolveSolidFill(elem.Trendline.SpPr.Ln.SolidFill, nil)
					t.Color = fallbackChartColor(t.colorRef)
				}
			}
			s.Trendline = t
		}

		// Extract per-point formatting
		for _, dpElem := range elem.DataPoints {
			dp := DataPoint{}
			if dpElem.Index != nil {
				dp.Index = dpElem.Index.Val
			}
			if dpElem.Explosion != nil {
				dp.Explosion = dpElem.Explosion.Val
			}
			if dpElem.SpPr != nil {
				if dpElem.SpPr.SolidFill != nil {
					ref := drawingxml.ResolveSolidFill(dpElem.SpPr.SolidFill, nil)
					dp.FillColor = fallbackChartColor(ref)
				}
				if dpElem.SpPr.Ln != nil && dpElem.SpPr.Ln.SolidFill != nil {
					ref := drawingxml.ResolveSolidFill(dpElem.SpPr.Ln.SolidFill, nil)
					dp.LineColor = fallbackChartColor(ref)
				}
			}
			s.DataPoints = append(s.DataPoints, dp)
		}

		series = append(series, s)
	}
	return series
}

// extractAxis converts an AxisElement to an Axis.
func dLblPosToInt(s string) int {
	switch strings.ToLower(s) {
	case "outend", "out end":
		return 1
	case "inend", "in end":
		return 2
	case "ctr", "center":
		return 3
	case "inbase", "in base":
		return 4
	}
	return 0
}

func tickMarkTypeToInt(s string) int {
	switch strings.ToLower(s) {
	case "out", "outside":
		return 1
	case "in", "inside":
		return 2
	case "cross":
		return 3
	}
	return 0
}

func labelPositionToInt(s string) int {
	switch strings.ToLower(s) {
	case "low":
		return 1
	case "high":
		return 2
	case "nextto":
		return 3
	}
	return 0
}

func extractAxis(elem AxisElement) Axis {
	axis := Axis{}

	if elem.Title != nil {
		axis.Title = elem.Title.PlainText()
		if axis.Title == "" {
			axis.Title = elem.Title.CachedText()
		}
	}

	if elem.Min != nil {
		axis.Min = elem.Min.Val
	}
	if elem.Max != nil {
		axis.Max = elem.Max.Val
	}
	if elem.MajorUnit != nil {
		axis.MajorUnit = elem.MajorUnit.Val
	}
	if elem.MinorUnit != nil {
		axis.MinorUnit = elem.MinorUnit.Val
	}
	if elem.Orientation != nil {
		axis.Orientation = elem.Orientation.Val
	}
	if elem.MajorTickMark != nil {
		axis.MajorTickType = tickMarkTypeToInt(elem.MajorTickMark.Val)
	}
	if elem.MinorTickMark != nil {
		axis.MinorTickType = tickMarkTypeToInt(elem.MinorTickMark.Val)
	}
	if elem.TickLblPos != nil {
		axis.LabelPosition = labelPositionToInt(elem.TickLblPos.Val)
	}
	if elem.Crosses != nil {
		if elem.Crosses.Val == "min" {
			axis.CrossingPoint = -1
		} else if elem.Crosses.Val == "max" {
			axis.CrossingPoint = -2
		}
	}
	if elem.CrossesAt != nil {
		axis.CrossingPoint = int(elem.CrossesAt.Val)
	}
	if elem.NumFmt != nil {
		axis.NumFmt = elem.NumFmt.FormatCode
	}

	if elem.MajorGridlines != nil {
		axis.ShowGridlines = true
		if elem.MajorGridlines.SpPr != nil {
			spPr := elem.MajorGridlines.SpPr
			if spPr.Ln != nil && spPr.Ln.SolidFill != nil {
				axis.gridlineRef = drawingxml.ResolveSolidFill(spPr.Ln.SolidFill, nil)
				axis.GridlineColor = fallbackChartColor(axis.gridlineRef)
			}
		}
	}

	return axis
}

func extractManualLayout(elem *ManualLayoutElement) *ManualLayout {
	if elem == nil {
		return nil
	}
	m := &ManualLayout{}
	if elem.LayoutTarget != nil {
		m.LayoutTarget = elem.LayoutTarget.Val
	}
	if elem.XMode != nil {
		m.XMode = elem.XMode.Val
	}
	if elem.YMode != nil {
		m.YMode = elem.YMode.Val
	}
	if elem.WMode != nil {
		m.WMode = elem.WMode.Val
	}
	if elem.HMode != nil {
		m.HMode = elem.HMode.Val
	}
	if elem.X != nil {
		m.X = elem.X.Val
	}
	if elem.Y != nil {
		m.Y = elem.Y.Val
	}
	if elem.W != nil {
		m.W = elem.W.Val
	}
	if elem.H != nil {
		m.H = elem.H.Val
	}
	return m
}

// extractTextProperties converts a DefRPrElement to TextProperties.
func extractTextProperties(defRPr *DefRPrElement) *TextProperties {
	if defRPr == nil {
		return nil
	}
	tp := &TextProperties{
		FontSize: defRPr.Sz,
		Bold:     defRPr.B,
		Italic:   defRPr.I,
	}
	if defRPr.SolidFill != nil {
		ref := drawingxml.ResolveSolidFill(defRPr.SolidFill, nil)
		if color := fallbackChartColor(ref); color != "" {
			tp.Color = color
		}
	}
	return tp
}

// ParseChartType converts a string to ChartType.
func ParseChartType(s string) ChartType {
	switch s {
	case "bar":
		return ChartBar
	case "col", "column":
		return ChartColumn
	case "line":
		return ChartLine
	case "pie":
		return ChartPie
	case "area":
		return ChartArea
	case "scatter":
		return ChartScatter
	case "doughnut":
		return ChartDoughnut
	case "bubble":
		return ChartBubble
	case "stock":
		return ChartStock
	case "radar":
		return ChartRadar
	case "surface":
		return ChartSurface
	default:
		return ChartUnknown
	}
}

// String returns the string representation of ChartType.
func (ct ChartType) String() string {
	switch ct {
	case ChartBar:
		return "bar"
	case ChartColumn:
		return "column"
	case ChartLine:
		return "line"
	case ChartPie:
		return "pie"
	case ChartArea:
		return "area"
	case ChartScatter:
		return "scatter"
	case ChartDoughnut:
		return "doughnut"
	case ChartBubble:
		return "bubble"
	case ChartStock:
		return "stock"
	case ChartRadar:
		return "radar"
	case ChartSurface:
		return "surface"
	case ChartBar3D:
		return "bar3d"
	case ChartColumn3D:
		return "column3d"
	case ChartLine3D:
		return "line3d"
	case ChartPie3D:
		return "pie3d"
	case ChartArea3D:
		return "area3d"
	case ChartSurface3D:
		return "surface3d"
	default:
		return "unknown"
	}
}

// MaxValue returns the maximum value across all series.
// For stacked charts the per-category sum is used so axes can hold the whole stack.
// PercentStacked always returns 100 since each category sums to 100%.
func (c *Chart) MaxValue() float64 {
	if c.PercentStacked {
		return 100
	}
	if c.Stacked && !c.PercentStacked {
		categories := 0
		for _, s := range c.Series {
			if len(s.Values) > categories {
				categories = len(s.Values)
			}
		}
		max := 0.0
		for i := 0; i < categories; i++ {
			sum := 0.0
			for _, s := range c.Series {
				if i < len(s.Values) && s.Values[i] > 0 {
					sum += s.Values[i]
				}
			}
			if sum > max {
				max = sum
			}
		}
		return max
	}
	max := 0.0
	for _, s := range c.Series {
		for _, v := range s.Values {
			if v > max {
				max = v
			}
		}
	}
	return max
}

// MinValue returns the minimum value across all series.
func (c *Chart) MinValue() float64 {
	if len(c.Series) == 0 {
		return 0
	}
	min := c.Series[0].Values[0]
	for _, s := range c.Series {
		for _, v := range s.Values {
			if v < min {
				min = v
			}
		}
	}
	return min
}

// CategoryCount returns the number of categories.
func (c *Chart) CategoryCount() int {
	if len(c.Series) > 0 {
		return len(c.Series[0].Values)
	}
	return len(c.Categories)
}

// SeriesCount returns the number of series.
func (c *Chart) SeriesCount() int {
	return len(c.Series)
}

// IsCartesian returns true if the chart has X and Y axes.
func (c *Chart) IsCartesian() bool {
	switch c.ChartType {
	case ChartBar, ChartColumn, ChartLine, ChartArea, ChartScatter, ChartBubble, ChartStock:
		return true
	default:
		return false
	}
}

// HexColor returns a hex color string for a series index.
func HexColor(index int) string {
	colors := []string{
		"4472C4", // Blue
		"ED7D31", // Orange
		"A5A5A5", // Gray
		"FFC000", // Yellow
		"5B9BD5", // Light Blue
		"70AD47", // Green
		"264478", // Dark Blue
		"9E480E", // Brown
		"636363", // Dark Gray
		"997300", // Dark Yellow
	}
	if index < len(colors) {
		return colors[index]
	}
	return colors[index%len(colors)]
}

// ParseFloat safely parses a float from string.
func ParseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
