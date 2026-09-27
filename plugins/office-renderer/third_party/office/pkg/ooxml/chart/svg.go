package chart

import (
	"fmt"
	"math"
	"strings"

	"office/pkg/ooxml/drawing"
)

func (c *Chart) defaultChartRect(width, height int) (x, y, w, h int) {
	left, right := 12, 12
	top, bottom := 12, 12

	if c.Title != "" {
		top = 42
	}
	if c.Legend.Show {
		switch c.Legend.Position {
		case "l", "left":
			left += minInt(80, maxInt(40, width/4))
		case "r", "right":
			right += minInt(80, maxInt(40, width/4))
		case "t", "top":
			top += minInt(32, maxInt(18, height/6))
		default:
			bottom += minInt(32, maxInt(18, height/6))
		}
	}

	// Keep enough room for the actual plot renderer even on compact dashboard
	// charts. These are fallback guards only; authored manual layout may replace
	// this rectangle below.
	if left+right > width-80 {
		left, right = 8, 8
	}
	if top+bottom > height-60 {
		top, bottom = 8, 8
		if c.Title != "" && height >= 90 {
			top = 30
		}
	}

	x, y = left, top
	w, h = width-left-right, height-top-bottom
	if w < 80 {
		w = 80
	}
	if h < 60 {
		h = 60
	}
	return
}

// chartPlotRect is the canonical drawable rectangle for Cartesian chart data.
// The outer chart frame may also contain titles, legends and axis labels. Keeping
// this rectangle in one place prevents individual renderers from applying their
// own fixed-pixel insets and therefore scaling the same chart differently.
type chartPlotRect struct {
	X, Y, W, H int
}

func (c *Chart) cartesianPlotRect(x, y, w, h int) chartPlotRect {
	// An authored inner manual layout already describes the plot rectangle.
	// Applying another renderer inset here would shrink it twice.
	if c.ManualLayout != nil && c.ManualLayout.LayoutTarget == "inner" {
		return chartPlotRect{X: x, Y: y, W: maxInt(0, w), H: maxInt(0, h)}
	}

	left := maxInt(34, minInt(60, int(math.Round(float64(w)*0.08))))
	right := maxInt(10, minInt(24, int(math.Round(float64(w)*0.02))))
	bottom := maxInt(28, minInt(44, int(math.Round(float64(h)*0.10))))
	top := 0
	if c.YAxis.Title != "" {
		left += 14
	}
	if c.XAxis.Title != "" {
		bottom += 12
	}
	if c.YAxis2.Title != "" {
		right += 18
	}
	if left+right >= w {
		left, right = maxInt(8, w/8), maxInt(6, w/16)
	}
	if top+bottom >= h {
		top, bottom = 0, maxInt(8, h/8)
	}
	return chartPlotRect{X: x + left, Y: y + top, W: maxInt(0, w-left-right), H: maxInt(0, h-top-bottom)}
}

func projectAxisValueY(plot chartPlotRect, value, minVal, maxVal float64) int {
	if maxVal <= minVal || plot.H <= 0 {
		return plot.Y + plot.H
	}
	return plot.Y + plot.H - int(math.Round(((value-minVal)/(maxVal-minVal))*float64(plot.H)))
}

func projectAxisValueX(plot chartPlotRect, value, minVal, maxVal float64) int {
	if maxVal <= minVal || plot.W <= 0 {
		return plot.X
	}
	return plot.X + int(math.Round(((value-minVal)/(maxVal-minVal))*float64(plot.W)))
}

func clampToPlotY(plot chartPlotRect, y int) int {
	if y < plot.Y {
		return plot.Y
	}
	if y > plot.Y+plot.H {
		return plot.Y + plot.H
	}
	return y
}

func clampToPlotX(plot chartPlotRect, x int) int {
	if x < plot.X {
		return plot.X
	}
	if x > plot.X+plot.W {
		return plot.X + plot.W
	}
	return x
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SVG renders the chart as an SVG string.
func (c *Chart) SVG(width, height int) string {
	if width == 0 {
		width = 600
	}
	if height == 0 {
		height = 400
	}

	var b strings.Builder

	// SVG header
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		width, height, width, height)

	// Background
	bg := "white"
	if color := c.chartAreaFillColor(); color != "" {
		bg = "#" + color
	}
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`, width, height, bg)

	// Allocate the outer chart frame by visible components instead of applying
	// large symmetric margins. Individual chart renderers already reserve space
	// for axes and labels; reserving it again here made short/wide dashboard
	// charts collapse to a very shallow plot area.
	chartX, chartY, chartW, chartH := c.defaultChartRect(width, height)

	defaultChartX := chartX
	defaultChartY := chartY
	defaultChartW := chartW
	defaultChartH := chartH

	// Title
	if c.Title != "" {
		titleX := width / 2
		titleY := 30
		if c.TitleLayout != nil {
			baseX, baseY, baseW, baseH := 0, 0, width, height
			if c.TitleLayout.LayoutTarget == "inner" {
				baseX = defaultChartX
				baseY = defaultChartY
				baseW = defaultChartW
				baseH = defaultChartH
			}
			tx, ty, tw, th := manualLayoutRect(c.TitleLayout, baseX, baseY, baseW, baseH)
			titleX = tx + tw/2
			if titleX < 0 {
				titleX = 0
			}
			if titleX > width {
				titleX = width
			}
			titleY = ty + th/2 + 5
			if titleY < 10 {
				titleY = 10
			}
			if titleY > height {
				titleY = height
			}
		}
		style := chartTextSVGStyle(c.TitleStyle, 16, true, "333333")
		fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle" %s>%s</text>`,
			titleX, titleY, style, escapeSVG(c.Title))
	}
	if c.ManualLayout != nil {
		baseX, baseY, baseW, baseH := 0, 0, width, height
		if c.ManualLayout.LayoutTarget == "inner" {
			baseX = defaultChartX
			baseY = defaultChartY
			baseW = defaultChartW
			baseH = defaultChartH
		}
		chartX, chartY, chartW, chartH = manualLayoutRect(c.ManualLayout, baseX, baseY, baseW, baseH)
	}

	// Reserve space for data table
	if c.DataTable != nil {
		rows := 1 + len(c.Series[0].Values)
		if len(c.Categories) > 0 && len(c.Categories) < len(c.Series[0].Values) {
			rows = 1 + len(c.Categories)
		}
		tableH := 18 * rows
		chartH -= tableH + 15
		if chartH < 50 {
			chartH = 50
		}
	}

	// Draw based on chart type
	switch c.ChartType {
	case ChartBar:
		c.renderBarChart(&b, chartX, chartY, chartW, chartH, true)
	case ChartColumn:
		c.renderBarChart(&b, chartX, chartY, chartW, chartH, false)
	case ChartLine:
		if c.canRenderCategoryComboChart() {
			c.renderCategoryComboChart(&b, chartX, chartY, chartW, chartH)
		} else {
			c.renderLineChart(&b, chartX, chartY, chartW, chartH)
		}
	case ChartPie:
		c.renderPieChart(&b, chartX, chartY, chartW, chartH)
	case ChartDoughnut:
		c.renderPieChart(&b, chartX, chartY, chartW, chartH)
	case ChartArea:
		if c.canRenderCategoryComboChart() {
			c.renderCategoryComboChart(&b, chartX, chartY, chartW, chartH)
		} else {
			c.renderAreaChart(&b, chartX, chartY, chartW, chartH)
		}
	case ChartScatter:
		c.renderScatterChart(&b, chartX, chartY, chartW, chartH)
	case ChartBubble:
		c.renderBubbleChart(&b, chartX, chartY, chartW, chartH)
	case ChartStock:
		c.renderStockChart(&b, chartX, chartY, chartW, chartH)
	case ChartRadar:
		c.renderRadarChart(&b, chartX, chartY, chartW, chartH)
	case ChartSurface:
		c.renderSurfaceChart(&b, chartX, chartY, chartW, chartH)
	case ChartBar3D:
		c.renderBar3DChart(&b, chartX, chartY, chartW, chartH, true)
	case ChartColumn3D:
		c.renderBar3DChart(&b, chartX, chartY, chartW, chartH, false)
	case ChartLine3D:
		if c.canRenderCategoryComboChart() {
			c.renderCategoryComboChart(&b, chartX, chartY, chartW, chartH)
		} else {
			c.renderLine3DChart(&b, chartX, chartY, chartW, chartH)
		}
	case ChartPie3D:
		c.renderPie3DChart(&b, chartX, chartY, chartW, chartH)
	case ChartArea3D:
		if c.canRenderCategoryComboChart() {
			c.renderCategoryComboChart(&b, chartX, chartY, chartW, chartH)
		} else {
			c.renderArea3DChart(&b, chartX, chartY, chartW, chartH)
		}
	case ChartSurface3D:
		c.renderSurface3DChart(&b, chartX, chartY, chartW, chartH)
	}

	// Data table
	if c.DataTable != nil {
		c.renderDataTable(&b, chartX, chartY, chartW, chartH)
	}

	// Legend
	if c.Legend.Show {
		c.renderLegend(&b, width, height, chartX, chartY, chartW, chartH)
	}

	// Close SVG
	b.WriteString(`</svg>`)

	return b.String()
}

// renderBarChart renders horizontal or vertical bar charts.
func manualLayoutRect(layout *ManualLayout, baseX, baseY, baseW, baseH int) (x, y, w, h int) {
	x = baseX
	y = baseY
	w = baseW
	h = baseH
	if layout == nil {
		return
	}
	if layout.XMode == "edge" || layout.XMode == "factor" {
		x = baseX + int(math.Round(layout.X*float64(baseW)))
	}
	if layout.YMode == "edge" || layout.YMode == "factor" {
		y = baseY + int(math.Round(layout.Y*float64(baseH)))
	}
	if layout.WMode == "edge" || layout.WMode == "factor" {
		if layout.W > 0 {
			w = int(math.Round(layout.W * float64(baseW)))
		}
	} else if layout.W > 0 {
		w = int(math.Round(layout.W * float64(baseW)))
	}
	if layout.HMode == "edge" || layout.HMode == "factor" {
		if layout.H > 0 {
			h = int(math.Round(layout.H * float64(baseH)))
		}
	} else if layout.H > 0 {
		h = int(math.Round(layout.H * float64(baseH)))
	}
	if x < baseX {
		x = baseX
	}
	if y < baseY {
		y = baseY
	}
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	if x+w > baseX+baseW {
		w = baseX + baseW - x
	}
	if y+h > baseY+baseH {
		h = baseY + baseH - y
	}
	return
}

func (c *Chart) renderBarChart(b *strings.Builder, x, y, w, h int, horizontal bool) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}
	if !horizontal && c.canRenderCategoryComboChart() {
		c.renderCategoryComboChart(b, x, y, w, h)
		return
	}

	// Draw axes (with override max=100 for percent-stacked).
	c.renderAxes(b, x, y, w, h, horizontal)

	// Calculate bar dimensions
	categories := len(c.Series[0].Values)
	seriesCount := len(c.Series)
	plot := c.cartesianPlotRect(x, y, w, h)
	plotX, plotY, plotW, plotH := plot.X, plot.Y, plot.W, plot.H

	var maxVal float64
	if c.PercentStacked {
		maxVal = 100
	} else {
		maxVal = c.axisMax()
	}
	if maxVal == 0 {
		maxVal = 1
	}

	// Compute group and bar sizes using gapWidth and overlap when available.
	var barSize, groupSize float64
	if horizontal {
		groupSize = float64(plotH) / float64(categories)
	} else {
		groupSize = float64(plotW) / float64(categories)
	}
	if c.GapWidth > 0 {
		gapRatio := float64(c.GapWidth) / 100.0
		overlapRatio := float64(c.Overlap) / 100.0
		barUnit := groupSize / (float64(seriesCount) + gapRatio)
		if c.Stacked || c.PercentStacked {
			barSize = barUnit * float64(seriesCount)
		} else {
			barSize = barUnit * (1 + overlapRatio)
		}
	} else {
		// Fallback to legacy heuristic when gapWidth is not set.
		barSize = (groupSize * 0.8) / float64(seriesCount)
	}

	// Per-category totals used for percent-stacked normalization.
	catTotals := make([]float64, categories)
	if c.PercentStacked {
		for catIdx := 0; catIdx < categories; catIdx++ {
			total := 0.0
			for _, series := range c.Series {
				if catIdx < len(series.Values) {
					total += series.Values[catIdx]
				}
			}
			catTotals[catIdx] = total
		}
	}

	// Pre-compute stacked cumulative values per category in *display* units
	// (raw for Stacked, percent-of-total for PercentStacked).
	stackedOffsets := make([][]float64, categories)
	if c.Stacked || c.PercentStacked {
		for catIdx := 0; catIdx < categories; catIdx++ {
			stackedOffsets[catIdx] = make([]float64, len(c.Series))
			cum := 0.0
			for seriesIdx, series := range c.Series {
				stackedOffsets[catIdx][seriesIdx] = cum
				if catIdx < len(series.Values) {
					v := series.Values[catIdx]
					if c.PercentStacked && catTotals[catIdx] > 0 {
						v = (v / catTotals[catIdx]) * 100
					}
					cum += v
				}
			}
		}
	}

	// Draw bars
	for catIdx := 0; catIdx < categories; catIdx++ {
		for seriesIdx, series := range c.Series {
			if catIdx >= len(series.Values) {
				continue
			}

			val := series.Values[catIdx]
			color := c.seriesFillColorFor(seriesIdx, catIdx)
			stroke := c.seriesStrokeColor(seriesIdx)

			var bx, by, bw, bh int
			displayVal := val
			if c.PercentStacked && catTotals[catIdx] > 0 {
				displayVal = (val / catTotals[catIdx]) * 100
				val = displayVal
			}

			if horizontal {
				if c.Stacked || c.PercentStacked {
					bw = int((val / maxVal) * float64(plotW))
					bh = int(groupSize * 0.8)
					by = plotY + int(float64(catIdx)*groupSize) + int(groupSize*0.1)
					bx = plotX + int((stackedOffsets[catIdx][seriesIdx]/maxVal)*float64(plotW))
				} else {
					bw = int((val / maxVal) * float64(plotW))
					bh = int(barSize * 0.9)
					by = plotY + int(float64(catIdx)*groupSize) + int(float64(seriesIdx)*barSize) + int(groupSize*0.1)
					bx = plotX
				}
			} else {
				if c.Stacked || c.PercentStacked {
					bw = int(groupSize * 0.8)
					bh = int((val / maxVal) * float64(plotH))
					bx = plotX + int(float64(catIdx)*groupSize) + int(groupSize*0.1)
					by = plotY + plotH - int((stackedOffsets[catIdx][seriesIdx]+val)/maxVal*float64(plotH))
				} else {
					bw = int(barSize * 0.9)
					bh = int((val / maxVal) * float64(plotH))
					bx = plotX + int(float64(catIdx)*groupSize) + int(float64(seriesIdx)*barSize) + int(groupSize*0.1)
					by = plotY + plotH - bh
				}
			}

			// Bar rect
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="#%s" stroke="#%s" stroke-width="1"/>`,
				bx, by, bw, bh, color, stroke)

			// Error bars
			if series.ErrorBars != nil {
				if horizontal {
					c.drawErrorBar(b, series.ErrorBars, bx+bw, by+bh/2, displayVal, maxVal, plotH, maxVal, plotW, series.Values, catIdx)
				} else {
					c.drawErrorBar(b, series.ErrorBars, bx+bw/2, by, displayVal, maxVal, plotH, maxVal, plotW, series.Values, catIdx)
				}
			}

			// Value label
			if series.ShowDataLabels || series.ShowPercent || series.ShowCatName || series.ShowSerName || series.ShowBubbleSize {
				labelX, labelY, anchor := dataLabelOffsetForBar(series, horizontal, bx, by, bw, bh)
				total := 0.0
				if c.PercentStacked || series.ShowPercent {
					total = maxVal
				}
				labelText := buildDataLabel(&series, catIdx, displayVal, total, c.Categories)
				style := dataLabelStyleAttrs(effectiveLabelStyle(&series, catIdx), 10, "#333")
				if series.ShowLeaderLines && (series.LabelPosition == 0 || series.LabelPosition == 1) {
					if horizontal {
						renderDataLabelLeaderLine(b, bx+bw, by+bh/2, labelX-2, labelY-2)
					} else {
						renderDataLabelLeaderLine(b, bx+bw/2, by, labelX, labelY+3)
					}
				}
				fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="%s" %s>%s</text>`,
					labelX, labelY, anchor, style, escapeSVG(labelText))
			}
		}

		// Category label
		if catIdx < len(c.Categories) {
			if horizontal {
				labelY := plotY + int(float64(catIdx)*groupSize) + int(groupSize/2)
				fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="end" font-size="10" fill="#666">%s</text>`,
					x+45, labelY, escapeSVG(c.Categories[catIdx]))
			} else {
				labelX := plotX + int(float64(catIdx)*groupSize) + int(groupSize/2)
				labelY := plotY + plotH + 20
				fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="10" fill="#666">%s</text>`,
					labelX, labelY, escapeSVG(c.Categories[catIdx]))
			}
		}
	}

	for _, series := range c.Series {
		c.renderTrendline(b, series, plotX, plotY, plotW, plotH, 0, float64(categories-1), maxVal, horizontal)
	}
}

// renderLineChart renders a line chart.
func (c *Chart) renderLineChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	minVal, maxVal := c.axisRange()
	if maxVal <= minVal {
		maxVal = minVal + 1
	}

	// Draw axes before the series. The series themselves are clipped to the
	// physical plot rectangle so out-of-range authored points cannot leak into
	// labels, legends, or neighbouring worksheet content.
	c.renderAxes(b, x, y, w, h, false)
	plot := c.cartesianPlotRect(x, y, w, h)
	plotX, plotY, plotW, plotH := plot.X, plot.Y, plot.W, plot.H
	if plotW < 0 {
		plotW = 0
	}
	if plotH < 0 {
		plotH = 0
	}
	clipID := fmt.Sprintf("chart-plot-%d-%d-%d-%d", plotX, plotY, plotW, plotH)
	fmt.Fprintf(b, `<defs><clipPath id="%s"><rect x="%d" y="%d" width="%d" height="%d"/></clipPath></defs>`, clipID, plotX, plotY, plotW, plotH)
	fmt.Fprintf(b, `<g clip-path="url(#%s)">`, clipID)

	points := len(c.Series[0].Values)
	xStep := 0.0
	if points > 1 {
		xStep = float64(plotW) / float64(points-1)
	}
	projectY := func(val float64) int { return projectAxisValueY(plot, val, minVal, maxVal) }

	for seriesIdx, series := range c.Series {
		color := c.seriesLineColor(seriesIdx)
		strokeWidth := 2
		if series.LineWidth > 0 {
			strokeWidth = series.LineWidth / 12700
			if strokeWidth < 1 {
				strokeWidth = 1
			}
		}

		var path strings.Builder
		coords := make([][2]int, len(series.Values))
		for i, val := range series.Values {
			px := plotX + int(float64(i)*xStep)
			py := projectY(val)
			coords[i] = [2]int{px, py}
			if i == 0 {
				path.WriteString("M")
			} else {
				path.WriteString(" L")
			}
			fmt.Fprintf(&path, "%d,%d", px, py)
		}

		if series.Smooth && len(coords) > 2 {
			fmt.Fprintf(b, `<path d="%s" fill="none" stroke="#%s" stroke-width="%d" stroke-linecap="round" stroke-linejoin="round"%s/>`, smoothPath(coords), color, strokeWidth, strokeDashAttr(series.LinePattern))
		} else {
			fmt.Fprintf(b, `<path d="%s" fill="none" stroke="#%s" stroke-width="%d" stroke-linecap="round" stroke-linejoin="round"%s/>`, path.String(), color, strokeWidth, strokeDashAttr(series.LinePattern))
		}

		markerColor := c.seriesMarkerColor(seriesIdx)
		markerSize := c.seriesMarkerSize(seriesIdx)
		for i := range series.Values {
			px, py := coords[i][0], coords[i][1]
			c.renderMarker(b, px, py, series.MarkerShape, markerColor, markerSize)
			if series.ShowDataLabels || series.ShowPercent || series.ShowCatName || series.ShowSerName || series.ShowBubbleSize {
				labelText := buildDataLabel(&series, i, series.Values[i], 0, c.Categories)
				style := dataLabelStyleAttrs(effectiveLabelStyle(&series, i), 9, "#333")
				if series.ShowLeaderLines {
					renderDataLabelLeaderLine(b, px, py, px, py-6)
				}
				fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" %s>%s</text>`, px, py-8, style, escapeSVG(labelText))
			}
		}
	}
	b.WriteString(`</g>`)

	// Keep existing error-bar/trendline behaviour for non-negative charts. Those
	// helpers currently use a zero-based value domain and need their own signed
	// range model before they can be projected safely on a mixed-sign axis.
	if minVal >= 0 {
		for _, series := range c.Series {
			if series.ErrorBars == nil {
				continue
			}
			for i, val := range series.Values {
				px := plotX + int(float64(i)*xStep)
				py := projectY(val)
				c.drawErrorBar(b, series.ErrorBars, px, py, val, maxVal, plotH, maxVal, plotW, series.Values, i)
			}
		}
		for _, series := range c.Series {
			c.renderTrendline(b, series, plotX, plotY, plotW, plotH, 0, float64(points-1), maxVal, false)
		}
	}

	c.renderCategoryLabels(b, x, y, w, h, points)
}

// renderPieChart renders a pie chart.
func (c *Chart) renderPieChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	series := c.Series[0]
	total := 0.0
	for _, v := range series.Values {
		total += v
	}
	if total == 0 {
		total = 1
	}

	// Center and radius
	cx := x + w/2
	cy := y + h/2
	r := min(w, h) / 3
	innerR := 0
	if c.ChartType == ChartDoughnut {
		holePct := c.HoleSize
		if holePct <= 0 {
			holePct = 50 // Word default
		}
		innerR = r * holePct / 100
	}

	// Draw pie slices
	startAngle := -90.0 // Start at top
	for i, val := range series.Values {
		angle := (val / total) * 360.0
		color := c.seriesFillColor(i)

		// Calculate path
		startRad := startAngle * math.Pi / 180
		endRad := (startAngle + angle) * math.Pi / 180

		// Explosion offset
		ex, ey := 0, 0
		explosion := c.seriesExplosionFor(0, i)
		if explosion > 0 {
			midRad := (startAngle + angle/2) * math.Pi / 180
			offset := explosion * r / 100
			ex = int(float64(offset) * math.Cos(midRad))
			ey = int(float64(offset) * math.Sin(midRad))
		}

		x1 := cx + ex + int(float64(r)*math.Cos(startRad))
		y1 := cy + ey + int(float64(r)*math.Sin(startRad))
		x2 := cx + ex + int(float64(r)*math.Cos(endRad))
		y2 := cy + ey + int(float64(r)*math.Sin(endRad))

		largeArc := 0
		if angle > 180 {
			largeArc = 1
		}

		if innerR > 0 {
			ix1 := cx + ex + int(float64(innerR)*math.Cos(startRad))
			iy1 := cy + ey + int(float64(innerR)*math.Sin(startRad))
			ix2 := cx + ex + int(float64(innerR)*math.Cos(endRad))
			iy2 := cy + ey + int(float64(innerR)*math.Sin(endRad))
			// Outer arc x1->x2, line in to inner ring, inner arc x2->x1 reversed, close.
			fmt.Fprintf(b, `<path d="M%d,%d A%d,%d 0 %d 1 %d,%d L%d,%d A%d,%d 0 %d 0 %d,%d Z" fill="#%s" stroke="white" stroke-width="2"/>`,
				x1, y1, r, r, largeArc, x2, y2, ix2, iy2, innerR, innerR, largeArc, ix1, iy1, color)
		} else {
			fmt.Fprintf(b, `<path d="M%d,%d L%d,%d A%d,%d 0 %d 1 %d,%d Z" fill="#%s" stroke="white" stroke-width="2"/>`,
				cx+ex, cy+ey, x1, y1, r, r, largeArc, x2, y2, color)
		}

		// Category label
		labelRad := (startAngle + angle/2) * math.Pi / 180
		labelR := r * 3 / 4
		if series.LabelPosition == 1 { // outEnd
			labelR = r + 25
		}
		lx := cx + ex + int(float64(labelR)*math.Cos(labelRad))
		ly := cy + ey + int(float64(labelR)*math.Sin(labelRad))

		label := ""
		if i < len(c.Categories) {
			label = c.Categories[i]
		}
		if label != "" {
			anchor := "middle"
			if series.LabelPosition == 1 {
				if math.Cos(labelRad) >= 0 {
					anchor = "start"
				} else {
					anchor = "end"
				}
			}
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="%s" font-size="10" fill="#333">%s</text>`,
				lx, ly, anchor, escapeSVG(label))
		}

		// Value / percent label
		if series.ShowDataLabels || series.ShowPercent || series.ShowCatName || series.ShowSerName || series.ShowBubbleSize {
			valR := r / 2
			if innerR > 0 {
				valR = (r + innerR) / 2
			}
			if series.LabelPosition == 1 { // outEnd
				valR = r + 25
			}
			vx := cx + ex + int(float64(valR)*math.Cos(labelRad))
			vy := cy + ey + int(float64(valR)*math.Sin(labelRad))
			labelText := buildDataLabel(&series, i, val, total, c.Categories)
			style := dataLabelStyleAttrs(effectiveLabelStyle(&series, i), 9, "#fff")
			anchor := "middle"
			if series.LabelPosition == 1 {
				if math.Cos(labelRad) >= 0 {
					anchor = "start"
				} else {
					anchor = "end"
				}
			}
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="%s" %s>%s</text>`,
				vx, vy, anchor, style, escapeSVG(labelText))
		}

		// Leader line / callout
		if series.ShowLeaderLines && (series.ShowDataLabels || series.ShowPercent || series.ShowCatName || series.ShowSerName || series.ShowBubbleSize) {
			if series.LabelPosition == 1 { // outEnd: L-shaped elbow callout
				stubR := r + 5
				fromX := cx + ex + int(float64(stubR)*math.Cos(labelRad))
				fromY := cy + ey + int(float64(stubR)*math.Sin(labelRad))
				toX := cx + ex + int(float64(r+25)*math.Cos(labelRad))
				toY := cy + ey + int(float64(r+25)*math.Sin(labelRad))
				// Horizontal first if slice is roughly left/right; vertical first if near top/bottom.
				horizontalFirst := math.Abs(math.Cos(labelRad)) >= math.Abs(math.Sin(labelRad))
				renderDataLabelElbowLine(b, fromX, fromY, toX, toY, horizontalFirst)
			} else {
				edgeR := r + 5
				x1 := cx + ex + int(float64(edgeR)*math.Cos(labelRad))
				y1 := cy + ey + int(float64(edgeR)*math.Sin(labelRad))
				x2 := cx + ex + int(float64(edgeR+15)*math.Cos(labelRad))
				y2 := cy + ey + int(float64(edgeR+15)*math.Sin(labelRad))
				fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#999" stroke-width="1"/>`,
					x1, y1, x2, y2)
			}
		}

		startAngle += angle
	}
}

// renderAreaChart renders an area chart.
func (c *Chart) renderAreaChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	minVal, maxVal := c.axisRange()
	if maxVal <= minVal {
		maxVal = minVal + 1
	}

	// Draw axes
	c.renderAxes(b, x, y, w, h, false)
	plot := c.cartesianPlotRect(x, y, w, h)

	// Draw areas
	points := len(c.Series[0].Values)
	xStep := 0.0
	if points > 1 {
		xStep = float64(plot.W) / float64(points-1)
	}

	for seriesIdx, series := range c.Series {
		color := c.seriesFillColor(seriesIdx)

		var path strings.Builder
		baseY := clampToPlotY(plot, projectAxisValueY(plot, 0, minVal, maxVal))

		coords := make([][2]int, len(series.Values))
		for i, val := range series.Values {
			px := plot.X + int(float64(i)*xStep)
			py := projectAxisValueY(plot, val, minVal, maxVal)
			coords[i] = [2]int{px, py}
		}

		// Start at bottom left
		fmt.Fprintf(&path, "M%d,%d", plot.X, baseY)

		// Top edge
		if series.Smooth && len(coords) > 2 {
			for i := 0; i < len(coords); i++ {
				if i == 0 {
					fmt.Fprintf(&path, " L%d,%d", coords[i][0], coords[i][1])
				} else {
					// Simple smooth curve using control points
					cp1x, cp1y, cp2x, cp2y := bezierControlPoints(coords, i)
					fmt.Fprintf(&path, " C%d,%d %d,%d %d,%d", cp1x, cp1y, cp2x, cp2y, coords[i][0], coords[i][1])
				}
			}
		} else {
			for i, coord := range coords {
				_ = i
				fmt.Fprintf(&path, " L%d,%d", coord[0], coord[1])
			}
		}

		// Back to bottom right and close
		lastX := plot.X + int(float64(points-1)*xStep)
		fmt.Fprintf(&path, " L%d,%d Z", lastX, baseY)

		// Draw area with transparency
		alpha := 0.3
		if seriesIdx < len(c.Series) && c.Series[seriesIdx].FillAlpha > 0 {
			alpha = c.Series[seriesIdx].FillAlpha
		}
		strokeColor := c.seriesLineColor(seriesIdx)
		strokeWidth := 2
		if series.LineWidth > 0 {
			strokeWidth = series.LineWidth / 12700
			if strokeWidth < 1 {
				strokeWidth = 1
			}
		}
		fmt.Fprintf(b, `<path d="%s" fill="#%s" fill-opacity="%g" stroke="#%s" stroke-width="%d"/>`,
			path.String(), color, alpha, strokeColor, strokeWidth)

		// Value labels
		for i, val := range series.Values {
			if series.ShowDataLabels || series.ShowPercent || series.ShowCatName || series.ShowSerName || series.ShowBubbleSize {
				labelText := buildDataLabel(&series, i, val, 0, c.Categories)
				style := dataLabelStyleAttrs(effectiveLabelStyle(&series, i), 9, "#333")
				if series.ShowLeaderLines {
					renderDataLabelLeaderLine(b, coords[i][0], coords[i][1], coords[i][0], coords[i][1]-6)
				}
				fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" %s>%s</text>`,
					coords[i][0], coords[i][1]-8, style, escapeSVG(labelText))
			}
		}
	}

	// Error bars
	for _, series := range c.Series {
		if series.ErrorBars == nil {
			continue
		}
		for i, val := range series.Values {
			px := plot.X + int(float64(i)*xStep)
			py := projectAxisValueY(plot, val, minVal, maxVal)
			c.drawErrorBar(b, series.ErrorBars, px, py, val, maxVal, plot.H, maxVal-minVal, plot.W, series.Values, i)
		}
	}

	// Trendlines
	for _, series := range c.Series {
		c.renderTrendline(b, series, plot.X, plot.Y, plot.W, plot.H, 0, float64(points-1), maxVal, false)
	}

	c.renderCategoryLabels(b, x, y, w, h, points)
}

func (c *Chart) renderScatterChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	c.renderAxes(b, x, y, w, h, false)

	// Determine if we have true X/Y values or index-based positioning
	hasXValues := false
	xMin, xMax := math.Inf(1), math.Inf(-1)
	for _, series := range c.Series {
		if len(series.XValues) > 0 {
			hasXValues = true
			for _, xv := range series.XValues {
				if xv < xMin {
					xMin = xv
				}
				if xv > xMax {
					xMax = xv
				}
			}
		}
	}
	if !hasXValues {
		xMin, xMax = 0.0, 0.0
	}

	// Apply axis bounds if configured
	if c.XAxis.Min != 0 || c.XAxis.Max != 0 {
		if c.XAxis.Min != 0 {
			xMin = c.XAxis.Min
		}
		if c.XAxis.Max != 0 {
			xMax = c.XAxis.Max
		}
		hasXValues = true
	}
	if xMax == xMin {
		xMax = xMin + 1
	}

	plot := c.cartesianPlotRect(x, y, w, h)
	plotX, plotY, plotW, plotH := plot.X, plot.Y, plot.W, plot.H
	yMin, yMax := c.axisRange()
	if yMax <= yMin {
		yMax = yMin + 1
	}

	for seriesIdx, series := range c.Series {
		color := c.seriesLineColor(seriesIdx)
		strokeWidth := 2
		if series.LineWidth > 0 {
			strokeWidth = series.LineWidth / 12700
			if strokeWidth < 1 {
				strokeWidth = 1
			}
		}
		drawLine, smooth, drawMarker := scatterRenderOptions(series)

		points := len(series.Values)
		if drawLine {
			var path strings.Builder
			path.WriteString("M")
			coords := make([][2]int, points)
			for i, val := range series.Values {
				var px, py int
				if hasXValues && i < len(series.XValues) {
					px = plotX + int(((series.XValues[i]-xMin)/(xMax-xMin))*float64(plotW))
				} else {
					groupSize := float64(plotW) / float64(points)
					px = plotX + int(float64(i)*groupSize) + int(groupSize/2)
				}
				py = projectAxisValueY(plot, val, yMin, yMax)
				coords[i] = [2]int{px, py}
				if i > 0 {
					path.WriteString(" L")
				}
				fmt.Fprintf(&path, "%d,%d", px, py)
			}
			if smooth && len(coords) > 2 {
				fmt.Fprintf(b, `<path d="%s" fill="none" stroke="#%s" stroke-width="%d" stroke-linecap="round" stroke-linejoin="round"%s/>`,
					smoothPath(coords), color, strokeWidth, strokeDashAttr(series.LinePattern))
			} else {
				fmt.Fprintf(b, `<path d="%s" fill="none" stroke="#%s" stroke-width="%d" stroke-linecap="round" stroke-linejoin="round"%s/>`,
					path.String(), color, strokeWidth, strokeDashAttr(series.LinePattern))
			}
		}

		for i, val := range series.Values {
			var px, py int
			if hasXValues && i < len(series.XValues) {
				px = plotX + int(((series.XValues[i]-xMin)/(xMax-xMin))*float64(plotW))
			} else {
				groupSize := float64(plotW) / float64(points)
				px = plotX + int(float64(i)*groupSize) + int(groupSize/2)
			}
			py = projectAxisValueY(plot, val, yMin, yMax)
			if drawMarker {
				markerColor := c.seriesMarkerColor(seriesIdx)
				markerSize := c.seriesMarkerSize(seriesIdx)
				c.renderMarker(b, px, py, series.MarkerShape, markerColor, markerSize)
			}

			// Value label
			if series.ShowDataLabels || series.ShowPercent || series.ShowCatName || series.ShowSerName || series.ShowBubbleSize {
				labelText := buildDataLabel(&series, i, val, 0, c.Categories)
				style := dataLabelStyleAttrs(effectiveLabelStyle(&series, i), 9, "#333")
				if series.ShowLeaderLines {
					renderDataLabelLeaderLine(b, px, py, px, py-6)
				}
				fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" %s>%s</text>`,
					px, py-8, style, escapeSVG(labelText))
			}
		}
	}

	// Error bars
	for _, series := range c.Series {
		if series.ErrorBars == nil {
			continue
		}
		points := len(series.Values)
		for i, val := range series.Values {
			var px, py int
			if hasXValues && i < len(series.XValues) {
				px = plotX + int(((series.XValues[i]-xMin)/(xMax-xMin))*float64(plotW))
			} else {
				groupSize := float64(plotW) / float64(points)
				px = plotX + int(float64(i)*groupSize) + int(groupSize/2)
			}
			py = projectAxisValueY(plot, val, yMin, yMax)
			xRange := xMax - xMin
			if !hasXValues {
				xRange = yMax - yMin
			}
			c.drawErrorBar(b, series.ErrorBars, px, py, val, yMax, plotH, xRange, plotW, series.Values, i)
		}
	}

	// Trendlines
	for _, series := range c.Series {
		c.renderTrendline(b, series, plotX, plotY, plotW, plotH, xMin, xMax, yMax, false)
	}

	if !hasXValues {
		c.renderCategoryLabels(b, x, y, w, h, len(c.Series[0].Values))
	}
}

// renderBubbleChart renders a bubble chart.
func (c *Chart) renderBubbleChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	c.renderAxes(b, x, y, w, h, false)

	plot := c.cartesianPlotRect(x, y, w, h)
	plotX, plotW, plotH := plot.X, plot.W, plot.H
	yMin, yMax := c.axisRange()
	if yMax <= yMin {
		yMax = yMin + 1
	}

	xMin, xMax := math.Inf(1), math.Inf(-1)
	hasXValues := false
	for _, series := range c.Series {
		if len(series.XValues) > 0 {
			hasXValues = true
			for _, xv := range series.XValues {
				if xv < xMin {
					xMin = xv
				}
				if xv > xMax {
					xMax = xv
				}
			}
		}
	}
	if !hasXValues {
		xMin, xMax = 0.0, 0.0
	}
	if c.XAxis.Min != 0 || c.XAxis.Max != 0 {
		if c.XAxis.Min != 0 {
			xMin = c.XAxis.Min
		}
		if c.XAxis.Max != 0 {
			xMax = c.XAxis.Max
		}
	}
	if xMax == xMin {
		xMax = xMin + 1
	}

	// Find max bubble size for scaling
	maxBubble := 0.0
	for _, series := range c.Series {
		for _, sz := range series.BubbleSizes {
			if sz > maxBubble {
				maxBubble = sz
			}
		}
	}
	if maxBubble == 0 {
		maxBubble = 1
	}

	for seriesIdx, series := range c.Series {
		color := c.seriesFillColor(seriesIdx)
		for i, val := range series.Values {
			var px, py int
			if i < len(series.XValues) {
				px = plotX + int(((series.XValues[i]-xMin)/(xMax-xMin))*float64(plotW))
			} else {
				px = plotX + int(float64(i)*float64(plotW)/float64(len(series.Values)))
			}
			py = projectAxisValueY(plot, val, yMin, yMax)

			// Bubble radius proportional to sqrt of size
			radius := 4
			if i < len(series.BubbleSizes) {
				radius = 4 + int(math.Sqrt(series.BubbleSizes[i]/maxBubble)*20)
			}

			fmt.Fprintf(b, `<circle cx="%d" cy="%d" r="%d" fill="#%s" fill-opacity="0.6" stroke="#%s" stroke-width="1"/>`,
				px, py, radius, color, color)

			if series.ShowDataLabels || series.ShowPercent || series.ShowCatName || series.ShowSerName || series.ShowBubbleSize {
				labelText := buildDataLabel(&series, i, val, 0, c.Categories)
				style := dataLabelStyleAttrs(effectiveLabelStyle(&series, i), 9, "#333")
				if series.ShowLeaderLines {
					renderDataLabelLeaderLine(b, px, py, px, py-radius-2)
				}
				fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" %s>%s</text>`,
					px, py-radius-4, style, escapeSVG(labelText))
			}
		}
	}

	// Error bars
	for _, series := range c.Series {
		if series.ErrorBars == nil {
			continue
		}
		for i, val := range series.Values {
			var px int
			if i < len(series.XValues) {
				px = plotX + int(((series.XValues[i]-xMin)/(xMax-xMin))*float64(plotW))
			} else {
				px = plotX + int(float64(i)*float64(plotW)/float64(len(series.Values)))
			}
			py := projectAxisValueY(plot, val, yMin, yMax)
			xRange := xMax - xMin
			if !hasXValues {
				xRange = yMax - yMin
			}
			c.drawErrorBar(b, series.ErrorBars, px, py, val, yMax, plotH, xRange, plotW, series.Values, i)
		}
	}
}

// renderStockChart renders a stock / OHLC chart.
func (c *Chart) renderStockChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	c.renderAxes(b, x, y, w, h, false)

	plot := c.cartesianPlotRect(x, y, w, h)
	plotX, plotY, plotW, plotH := plot.X, plot.Y, plot.W, plot.H

	points := len(c.Series[0].Values)
	if points == 0 {
		return
	}
	// Use the same x-step as renderCategoryLabels so candles line up with labels.
	xStep := 0.0
	if points > 1 {
		xStep = float64(plotW) / float64(points-1)
	}
	candleW := xStep
	if candleW == 0 {
		candleW = float64(plotW)
	}

	maxVal := c.axisMax()
	if maxVal == 0 {
		maxVal = 1
	}

	// Stock charts typically have 4 series: open, high, low, close
	// We render each as a candlestick using the series values
	for catIdx := 0; catIdx < points; catIdx++ {
		px := plotX + int(float64(catIdx)*xStep)

		// Gather values from up to 4 series at this category
		var vals []float64
		for _, series := range c.Series {
			if catIdx < len(series.Values) {
				vals = append(vals, series.Values[catIdx])
			}
		}
		if len(vals) == 0 {
			continue
		}

		// Determine high and low
		high, low := vals[0], vals[0]
		for _, v := range vals {
			if v > high {
				high = v
			}
			if v < low {
				low = v
			}
		}

		hy := plotY + plotH - int((high/maxVal)*float64(plotH))
		ly := plotY + plotH - int((low/maxVal)*float64(plotH))

		switch {
		case len(vals) >= 4:
			// Standard OHLC order: open, high, low, close.
			openVal := vals[0]
			high = vals[1]
			low = vals[2]
			closeVal := vals[3]
			hy = plotY + plotH - int((high/maxVal)*float64(plotH))
			ly = plotY + plotH - int((low/maxVal)*float64(plotH))
			oy := plotY + plotH - int((openVal/maxVal)*float64(plotH))
			cy := plotY + plotH - int((closeVal/maxVal)*float64(plotH))

			// Refresh the wick in case explicit high/low differ from min/max.
			fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#333" stroke-width="1"/>`,
				px, hy, px, ly)

			bw := int(candleW * 0.6)
			if bw < 2 {
				bw = 2
			}
			bodyColor := "70AD47" // green for up
			if closeVal < openVal {
				bodyColor = "DC3912" // red for down
			}
			if c.StockConfig != nil {
				if closeVal >= openVal && c.StockConfig.UpBarColor != "" {
					bodyColor = c.StockConfig.UpBarColor
				}
				if closeVal < openVal && c.StockConfig.DownBarColor != "" {
					bodyColor = c.StockConfig.DownBarColor
				}
			}
			by := oy
			bh := cy - oy
			if bh < 0 {
				by = cy
				bh = -bh
			}
			if bh < 1 {
				bh = 1
			}
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="#%s" stroke="#333" stroke-width="1"/>`,
				px-bw/2, by, bw, bh, bodyColor)
		case len(vals) == 3:
			// High-low-close charts have no open, so render a close tick only.
			closeVal := vals[2]
			cy := plotY + plotH - int((closeVal/maxVal)*float64(plotH))
			tick := int(candleW * 0.25)
			if tick < 3 {
				tick = 3
			}
			fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#333" stroke-width="1"/>`,
				px, hy, px, ly)
			fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#333" stroke-width="1"/>`,
				px, cy, px+tick, cy)
		case len(vals) == 2:
			fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#333" stroke-width="1"/>`,
				px, hy, px, ly)
		case len(vals) == 1:
			singleY := plotY + plotH - int((vals[0]/maxVal)*float64(plotH))
			c.renderMarker(b, px, singleY, "circle", "333333", 0)
		}
	}

	c.renderCategoryLabels(b, x, y, w, h, points)
}

// renderRadarChart renders a radar / spider chart.
func (c *Chart) renderRadarChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	cx := x + w/2
	cy := y + h/2
	r := min(w, h) / 3
	if r < 20 {
		r = 20
	}

	points := len(c.Series[0].Values)
	if points < 3 {
		points = 3
	}
	maxVal := c.axisMax()
	if maxVal == 0 {
		maxVal = 1
	}

	// Draw web grid
	for ring := 1; ring <= 4; ring++ {
		rr := r * ring / 4
		var web strings.Builder
		for i := 0; i < points; i++ {
			angle := float64(i)*2*math.Pi/float64(points) - math.Pi/2
			px := cx + int(float64(rr)*math.Cos(angle))
			py := cy + int(float64(rr)*math.Sin(angle))
			if i == 0 {
				fmt.Fprintf(&web, "M%d,%d", px, py)
			} else {
				fmt.Fprintf(&web, " L%d,%d", px, py)
			}
		}
		web.WriteString(" Z")
		fmt.Fprintf(b, `<path d="%s" fill="none" stroke="#ddd" stroke-width="1"/>`, web.String())
	}

	// Draw spokes
	for i := 0; i < points; i++ {
		angle := float64(i)*2*math.Pi/float64(points) - math.Pi/2
		px := cx + int(float64(r)*math.Cos(angle))
		py := cy + int(float64(r)*math.Sin(angle))
		fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#ddd" stroke-width="1"/>`,
			cx, cy, px, py)
		// Category label
		if i < len(c.Categories) {
			lx := cx + int(float64(r+15)*math.Cos(angle))
			ly := cy + int(float64(r+15)*math.Sin(angle))
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="9" fill="#666">%s</text>`,
				lx, ly+4, escapeSVG(c.Categories[i]))
		}
	}

	// Determine radar style (chart-level property).
	radarStyle := strings.ToLower(c.RadarStyle)
	if radarStyle == "" && len(c.Groups) > 0 {
		radarStyle = strings.ToLower(c.Groups[0].Grouping)
	}

	// Draw series
	for seriesIdx, series := range c.Series {
		color := c.seriesLineColor(seriesIdx)
		fill := c.seriesFillColor(seriesIdx)
		var pts []string
		var markers []struct{ x, y int }
		for i, val := range series.Values {
			angle := float64(i)*2*math.Pi/float64(points) - math.Pi/2
			rr := int((val / maxVal) * float64(r))
			px := cx + int(float64(rr)*math.Cos(angle))
			py := cy + int(float64(rr)*math.Sin(angle))
			pts = append(pts, fmt.Sprintf("%d,%d", px, py))
			markers = append(markers, struct{ x, y int }{px, py})
		}
		switch radarStyle {
		case "filled":
			if len(series.Values) > 2 {
				fmt.Fprintf(b, `<polygon points="%s" fill="#%s" fill-opacity="0.5" stroke="none"/>`,
					strings.Join(pts, " "), fill)
			} else {
				fmt.Fprintf(b, `<polyline points="%s" fill="none" stroke="#%s" stroke-width="2"/>`,
					strings.Join(pts, " "), color)
			}
		case "marker":
			markerColor := c.seriesMarkerColor(seriesIdx)
			markerSize := c.seriesMarkerSize(seriesIdx)
			for _, m := range markers {
				c.renderMarker(b, m.x, m.y, series.MarkerShape, markerColor, markerSize)
			}
		default: // "standard" and fallback
			if len(series.Values) > 2 {
				fmt.Fprintf(b, `<polygon points="%s" fill="#%s" fill-opacity="0.25" stroke="#%s" stroke-width="2"/>`,
					strings.Join(pts, " "), fill, color)
			} else {
				fmt.Fprintf(b, `<polyline points="%s" fill="none" stroke="#%s" stroke-width="2"/>`,
					strings.Join(pts, " "), color)
			}
			markerColor := c.seriesMarkerColor(seriesIdx)
			markerSize := c.seriesMarkerSize(seriesIdx)
			for _, m := range markers {
				c.renderMarker(b, m.x, m.y, series.MarkerShape, markerColor, markerSize)
			}
		}
	}
}

// renderSurfaceChart renders a surface chart as a heatmap grid.
func (c *Chart) renderSurfaceChart(b *strings.Builder, x, y, w, h int) {
	if len(c.Series) == 0 || len(c.Series[0].Values) == 0 {
		c.renderEmptyFrame(b, x, y, w, h)
		return
	}

	plotX := x + 50
	plotY := y
	plotW := w - 60
	plotH := h - 40

	seriesCount := len(c.Series)
	points := len(c.Series[0].Values)
	if points == 0 {
		return
	}

	cellW := float64(plotW) / float64(points)
	cellH := float64(plotH) / float64(seriesCount)
	if cellW < 1 {
		cellW = 1
	}
	if cellH < 1 {
		cellH = 1
	}

	maxVal := c.axisMax()
	if maxVal == 0 {
		maxVal = 1
	}

	for seriesIdx, series := range c.Series {
		for i, val := range series.Values {
			cx := plotX + int(float64(i)*cellW)
			cy := plotY + int(float64(seriesIdx)*cellH)
			cw := plotX + int(float64(i+1)*cellW) - cx
			ch := plotY + int(float64(seriesIdx+1)*cellH) - cy
			intensity := val / maxVal
			// Heatmap color from blue (cold) to red (hot)
			r := int(255 * intensity)
			bb := int(255 * (1 - intensity))
			color := fmt.Sprintf("%02X00%02X", r, bb)
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="#%s" stroke="white" stroke-width="0.5"/>`,
				cx, cy, cw, ch, color)
		}
	}

	c.renderCategoryLabels(b, x, y, w, h, points)
}

func (c *Chart) renderCategoryLabels(b *strings.Builder, x, y, w, h, points int) {
	if points == 0 || len(c.Categories) == 0 {
		return
	}
	maxLabels := 10
	step := 1
	if len(c.Categories) > maxLabels {
		step = int(math.Ceil(float64(len(c.Categories)) / float64(maxLabels)))
	}
	plot := c.cartesianPlotRect(x, y, w, h)
	xStep := 0.0
	if points > 1 {
		xStep = float64(plot.W) / float64(points-1)
	}

	// Rotate labels when there are many categories to prevent overlap.
	rotateLabels := len(c.Categories) > 15
	labelY := plot.Y + plot.H + 18
	if rotateLabels {
		labelY = plot.Y + plot.H + 30
	}

	lastDrawn := -1
	for i := 0; i < points && i < len(c.Categories); i += step {
		px := plot.X
		if points > 1 {
			px += int(float64(i) * xStep)
		}
		if rotateLabels {
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="end" font-size="9" fill="#666" transform="rotate(-45 %d %d)">%s</text>`,
				px, labelY, px, labelY, escapeSVG(c.Categories[i]))
		} else {
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="10" fill="#666">%s</text>`,
				px, labelY, escapeSVG(c.Categories[i]))
		}
		lastDrawn = i
	}
	last := points - 1
	if last >= len(c.Categories) {
		last = len(c.Categories) - 1
	}
	if last >= 0 && last != lastDrawn {
		px := plot.X
		if points > 1 {
			px += int(float64(last) * xStep)
		}
		if rotateLabels {
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="end" font-size="9" fill="#666" transform="rotate(-45 %d %d)">%s</text>`,
				px, labelY, px, labelY, escapeSVG(c.Categories[last]))
		} else {
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="10" fill="#666">%s</text>`,
				px, labelY, escapeSVG(c.Categories[last]))
		}
	}
}

// renderEmptyFrame draws an axes skeleton with a label for charts with no cached data.
func (c *Chart) renderEmptyFrame(b *strings.Builder, x, y, w, h int) {
	axisColor := "#000"
	plot := c.cartesianPlotRect(x, y, w, h)
	fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`,
		plot.X, plot.Y, plot.X, plot.Y+plot.H, axisColor)
	fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`,
		plot.X, plot.Y+plot.H, plot.X+plot.W, plot.Y+plot.H, axisColor)
	label := c.ChartType.String() + " chart"
	if len(c.Series) > 0 && c.Series[0].Name != "" {
		label = c.Series[0].Name
	}
	fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="12" fill="#999" font-style="italic">%s</text>`,
		plot.X+plot.W/2, plot.Y+plot.H/2, escapeSVG(label))
}

// solveLinearSystem solves A·x = b using Gaussian elimination with partial pivoting.
func solveLinearSystem(a [][]float64, b []float64) []float64 {
	n := len(b)
	for i := 0; i < n; i++ {
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(a[k][i]) > math.Abs(a[maxRow][i]) {
				maxRow = k
			}
		}
		a[i], a[maxRow] = a[maxRow], a[i]
		b[i], b[maxRow] = b[maxRow], b[i]
		if math.Abs(a[i][i]) < 1e-12 {
			return nil
		}
		for k := i + 1; k < n; k++ {
			factor := a[k][i] / a[i][i]
			for j := i; j < n; j++ {
				a[k][j] -= factor * a[i][j]
			}
			b[k] -= factor * b[i]
		}
	}
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		sum := b[i]
		for j := i + 1; j < n; j++ {
			sum -= a[i][j] * x[j]
		}
		x[i] = sum / a[i][i]
	}
	return x
}

type trendPoint struct{ x, y float64 }

// linearRegression returns a function y = slope*x + intercept fitted by least squares.
// If forcedIntercept is non-nil, the intercept is fixed to that value.
func linearRegression(points []trendPoint, forcedIntercept *float64) func(float64) float64 {
	if len(points) < 2 {
		return nil
	}
	if forcedIntercept != nil {
		intercept := *forcedIntercept
		var sumX2, sumXY float64
		for _, p := range points {
			sumX2 += p.x * p.x
			sumXY += p.x * (p.y - intercept)
		}
		if sumX2 == 0 {
			return nil
		}
		slope := sumXY / sumX2
		return func(x float64) float64 {
			return slope*x + intercept
		}
	}
	n := float64(len(points))
	var sumX, sumY, sumXY, sumX2 float64
	for _, p := range points {
		sumX += p.x
		sumY += p.y
		sumXY += p.x * p.y
		sumX2 += p.x * p.x
	}
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return nil
	}
	slope := (n*sumXY - sumX*sumY) / denominator
	intercept := (sumY - slope*sumX) / n
	return func(x float64) float64 {
		return slope*x + intercept
	}
}

// trendlineFunc returns a function that computes the trendline y-value for a given x.
func (c *Chart) trendlineFunc(series Series) func(float64) float64 {
	t := series.Trendline
	if t == nil || len(series.Values) < 2 {
		return nil
	}

	points := make([]trendPoint, 0, len(series.Values))
	for i, v := range series.Values {
		xv := float64(i)
		if len(series.XValues) > i {
			xv = series.XValues[i]
		}
		points = append(points, trendPoint{xv, v})
	}

	switch t.Type {
	case "linear":
		return linearRegression(points, t.Intercept)
	case "poly":
		order := t.Order
		if order < 2 {
			order = 2
		}
		if order > 6 {
			order = 6
		}
		if len(points) < order+2 {
			return nil
		}
		n := order + 1
		mat := make([][]float64, n)
		vec := make([]float64, n)
		for i := range mat {
			mat[i] = make([]float64, n)
		}
		for _, p := range points {
			for i := 0; i < n; i++ {
				vec[i] += p.y * math.Pow(p.x, float64(i))
				for j := 0; j < n; j++ {
					mat[i][j] += math.Pow(p.x, float64(i+j))
				}
			}
		}
		coeffs := solveLinearSystem(mat, vec)
		if coeffs == nil {
			return nil
		}
		return func(x float64) float64 {
			sum := 0.0
			for i := 0; i < n; i++ {
				sum += coeffs[i] * math.Pow(x, float64(i))
			}
			return sum
		}
	case "exp":
		expPoints := make([]trendPoint, 0, len(points))
		for _, p := range points {
			if p.y > 0 {
				expPoints = append(expPoints, trendPoint{p.x, math.Log(p.y)})
			}
		}
		if len(expPoints) < 2 {
			return nil
		}
		f := linearRegression(expPoints, nil)
		if f == nil {
			return nil
		}
		intercept := f(0)
		slope := f(1) - intercept
		a := math.Exp(intercept)
		return func(x float64) float64 {
			return a * math.Exp(slope*x)
		}
	case "log":
		logPoints := make([]trendPoint, 0, len(points))
		for _, p := range points {
			if p.x > 0 {
				logPoints = append(logPoints, trendPoint{math.Log(p.x), p.y})
			}
		}
		if len(logPoints) < 2 {
			return nil
		}
		f := linearRegression(logPoints, nil)
		if f == nil {
			return nil
		}
		intercept := f(0)
		slope := f(1) - intercept
		return func(x float64) float64 {
			return slope*math.Log(x) + intercept
		}
	case "power":
		powPoints := make([]trendPoint, 0, len(points))
		for _, p := range points {
			if p.x > 0 && p.y > 0 {
				powPoints = append(powPoints, trendPoint{math.Log(p.x), math.Log(p.y)})
			}
		}
		if len(powPoints) < 2 {
			return nil
		}
		f := linearRegression(powPoints, nil)
		if f == nil {
			return nil
		}
		intercept := f(0)
		slope := f(1) - intercept
		a := math.Exp(intercept)
		return func(x float64) float64 {
			return a * math.Pow(x, slope)
		}
	case "movingAvg":
		period := t.Period
		if period < 2 {
			period = 2
		}
		return func(x float64) float64 {
			i := int(math.Round(x))
			if i < 0 {
				i = 0
			}
			if i >= len(series.Values) {
				i = len(series.Values) - 1
			}
			start := i - period + 1
			if start < 0 {
				start = 0
			}
			sum := 0.0
			count := 0
			for j := start; j <= i && j < len(series.Values); j++ {
				sum += series.Values[j]
				count++
			}
			if count == 0 {
				return 0
			}
			return sum / float64(count)
		}
	default:
		return nil
	}
}

// trendlineStats computes the equation string and R² label for a trendline.
func (c *Chart) trendlineStats(series Series) (equation, rSquared string) {
	t := series.Trendline
	if t == nil || len(series.Values) < 2 {
		return "", ""
	}

	points := make([]trendPoint, 0, len(series.Values))
	for i, v := range series.Values {
		xv := float64(i)
		if len(series.XValues) > i {
			xv = series.XValues[i]
		}
		points = append(points, trendPoint{xv, v})
	}

	var f func(float64) float64
	var eq string

	switch t.Type {
	case "linear":
		lr := linearRegression(points, t.Intercept)
		if lr == nil {
			return "", ""
		}
		f = lr
		intercept := lr(0)
		slope := lr(1) - intercept
		if math.Abs(slope) < 1e-9 {
			eq = fmt.Sprintf("y = %.4g", intercept)
		} else if math.Abs(intercept) < 1e-9 {
			eq = fmt.Sprintf("y = %.4gx", slope)
		} else if intercept < 0 {
			eq = fmt.Sprintf("y = %.4gx - %.4g", slope, math.Abs(intercept))
		} else {
			eq = fmt.Sprintf("y = %.4gx + %.4g", slope, intercept)
		}
	case "poly":
		order := t.Order
		if order < 2 {
			order = 2
		}
		if order > 6 {
			order = 6
		}
		if len(points) < order+2 {
			return "", ""
		}
		n := order + 1
		mat := make([][]float64, n)
		vec := make([]float64, n)
		for i := range mat {
			mat[i] = make([]float64, n)
		}
		for _, p := range points {
			for i := 0; i < n; i++ {
				vec[i] += p.y * math.Pow(p.x, float64(i))
				for j := 0; j < n; j++ {
					mat[i][j] += math.Pow(p.x, float64(i+j))
				}
			}
		}
		coeffs := solveLinearSystem(mat, vec)
		if coeffs == nil {
			return "", ""
		}
		f = func(x float64) float64 {
			sum := 0.0
			for i := 0; i < n; i++ {
				sum += coeffs[i] * math.Pow(x, float64(i))
			}
			return sum
		}
		var terms []string
		for i := 0; i < n; i++ {
			if math.Abs(coeffs[i]) < 1e-9 {
				continue
			}
			switch i {
			case 0:
				terms = append(terms, fmt.Sprintf("%.4g", coeffs[i]))
			case 1:
				terms = append(terms, fmt.Sprintf("%.4gx", coeffs[i]))
			default:
				terms = append(terms, fmt.Sprintf("%.4gx^%d", coeffs[i], i))
			}
		}
		if len(terms) == 0 {
			eq = "y = 0"
		} else {
			eq = "y = " + strings.Join(terms, " + ")
		}
	case "exp":
		expPoints := make([]trendPoint, 0, len(points))
		for _, p := range points {
			if p.y > 0 {
				expPoints = append(expPoints, trendPoint{p.x, math.Log(p.y)})
			}
		}
		if len(expPoints) < 2 {
			return "", ""
		}
		lr := linearRegression(expPoints, nil)
		if lr == nil {
			return "", ""
		}
		intercept := lr(0)
		slope := lr(1) - intercept
		a := math.Exp(intercept)
		f = func(x float64) float64 { return a * math.Exp(slope*x) }
		eq = fmt.Sprintf("y = %.4g e^(%.4gx)", a, slope)
	case "log":
		logPoints := make([]trendPoint, 0, len(points))
		for _, p := range points {
			if p.x > 0 {
				logPoints = append(logPoints, trendPoint{math.Log(p.x), p.y})
			}
		}
		if len(logPoints) < 2 {
			return "", ""
		}
		lr := linearRegression(logPoints, nil)
		if lr == nil {
			return "", ""
		}
		intercept := lr(0)
		slope := lr(1) - intercept
		f = func(x float64) float64 { return slope*math.Log(x) + intercept }
		eq = fmt.Sprintf("y = %.4g ln(x) + %.4g", slope, intercept)
	case "power":
		powPoints := make([]trendPoint, 0, len(points))
		for _, p := range points {
			if p.x > 0 && p.y > 0 {
				powPoints = append(powPoints, trendPoint{math.Log(p.x), math.Log(p.y)})
			}
		}
		if len(powPoints) < 2 {
			return "", ""
		}
		lr := linearRegression(powPoints, nil)
		if lr == nil {
			return "", ""
		}
		intercept := lr(0)
		slope := lr(1) - intercept
		a := math.Exp(intercept)
		f = func(x float64) float64 { return a * math.Pow(x, slope) }
		eq = fmt.Sprintf("y = %.4g x^%.4g", a, slope)
	case "movingAvg":
		return "", ""
	default:
		return "", ""
	}

	// Compute R²
	var yMean, tss, rss float64
	validCount := 0
	for _, p := range points {
		yMean += p.y
		validCount++
	}
	if validCount > 0 {
		yMean /= float64(validCount)
	}
	for _, p := range points {
		if f != nil {
			pred := f(p.x)
			if !math.IsNaN(pred) && !math.IsInf(pred, 0) {
				rss += (p.y - pred) * (p.y - pred)
			}
		}
		tss += (p.y - yMean) * (p.y - yMean)
	}
	if tss > 1e-12 {
		r2 := 1.0 - rss/tss
		if r2 < 0 {
			r2 = 0
		}
		if r2 > 1 {
			r2 = 1
		}
		return eq, fmt.Sprintf("R² = %.4g", r2)
	}
	return eq, ""
}

// renderTrendline draws a trendline for the given series across the plot area.
func (c *Chart) renderTrendline(b *strings.Builder, series Series, plotX, plotY, plotW, plotH int, xMin, xMax, yMax float64, horizontal bool) {
	if series.Trendline == nil || len(series.Values) < 2 {
		return
	}
	t := series.Trendline
	if yMax == 0 {
		yMax = 1
	}

	f := c.trendlineFunc(series)
	if f == nil {
		return
	}

	xStart := xMin - t.Backward
	xEnd := xMax + t.Forward
	if xStart == xEnd {
		xEnd = xStart + 1
	}
	xRange := xEnd - xStart
	if xRange == 0 {
		xRange = 1
	}

	samples := 200
	if t.Type == "movingAvg" {
		samples = len(series.Values)
	}

	var path strings.Builder
	path.WriteString("M")
	first := true
	for i := 0; i <= samples; i++ {
		var xv float64
		if t.Type == "movingAvg" {
			xv = float64(i)
			if i >= len(series.Values) {
				break
			}
		} else {
			xv = xStart + xRange*float64(i)/float64(samples)
		}
		yv := f(xv)
		if math.IsNaN(yv) || math.IsInf(yv, 0) {
			continue
		}
		var px, py int
		if horizontal {
			px = plotX + int((yv/yMax)*float64(plotW))
			py = plotY + plotH - int(((xv-xStart)/xRange)*float64(plotH))
		} else {
			px = plotX + int(((xv-xStart)/xRange)*float64(plotW))
			py = plotY + plotH - int((yv/yMax)*float64(plotH))
		}
		if !first {
			path.WriteString(" L")
		}
		first = false
		fmt.Fprintf(&path, "%d,%d", px, py)
	}
	if first {
		return
	}

	color := "333"
	if resolved := c.resolveSolidFill(t.colorRef); resolved != "" {
		color = resolved
	} else if t.Color != "" {
		color = c.resolveColor(t.Color)
	}
	width := 2
	if t.LineWidth > 0 {
		width = t.LineWidth / 12700
		if width < 1 {
			width = 1
		}
	}

	fmt.Fprintf(b, `<path d="%s" fill="none" stroke="#%s" stroke-width="%d" stroke-dasharray="5,3"/>`,
		path.String(), color, width)

	// Render equation and/or R² label if requested
	if t.ShowEquation || t.ShowRSquared {
		eq, r2 := c.trendlineStats(series)
		labelY := plotY + 12
		labelX := plotX + plotW - 10
		anchor := "end"
		if t.ShowEquation && eq != "" {
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="%s" font-size="10" fill="#%s">%s</text>`,
				labelX, labelY, anchor, color, escapeSVG(eq))
			labelY += 14
		}
		if t.ShowRSquared && r2 != "" {
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="%s" font-size="10" fill="#%s">%s</text>`,
				labelX, labelY, anchor, color, escapeSVG(r2))
		}
	}
}

// renderAxes renders X and Y axes.
func (c *Chart) renderAxes(b *strings.Builder, x, y, w, h int, horizontal bool) {
	axisColor := "#000"
	plot := c.cartesianPlotRect(x, y, w, h)
	plotX, plotY, plotW, plotH := plot.X, plot.Y, plot.W, plot.H
	fill := "white"
	if c.PlotArea.FillColor != "" {
		fill = "#" + c.resolveColor(c.PlotArea.FillColor)
	}
	border := "none"
	if c.PlotArea.ShowBorder {
		border = "#808080"
		if c.PlotArea.BorderColor != "" {
			border = "#" + c.resolveColor(c.PlotArea.BorderColor)
		}
	}
	fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s" stroke="%s" stroke-width="1"/>`,
		plotX, plotY, plotW, plotH, fill, border)

	// Y axis
	fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`,
		plotX, plotY, plotX, plotY+plotH, axisColor)

	// X axis. Cross at zero for a mixed-sign value domain; otherwise use the
	// nearest plot edge.
	axisMin, axisMax := c.axisRange()
	xAxisY := plotY + plotH
	if axisMin < 0 && axisMax > 0 {
		xAxisY = plotY + plotH - int(((0-axisMin)/(axisMax-axisMin))*float64(plotH))
	} else if axisMax <= 0 {
		xAxisY = plotY
	}
	fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`,
		plotX, xAxisY, plotX+plotW, xAxisY, axisColor)

	// Grid lines and Y labels
	minVal, maxVal := c.axisRange()
	tick := axisTickStep(maxVal-minVal, c.YAxis.MajorUnit)
	if maxVal > minVal && tick > 0 {
		startTick := math.Ceil(minVal/tick) * tick
		for val := startTick; val <= maxVal+0.001; val += tick {
			yPos := plotY + plotH - int(((val-minVal)/(maxVal-minVal))*float64(plotH))
			if c.YAxis.ShowGridlines && math.Abs(val) > 1e-9 {
				gridColor := "#ccc"
				if resolved := c.resolveSolidFill(c.YAxis.gridlineRef); resolved != "" {
					gridColor = "#" + resolved
				} else if c.YAxis.GridlineColor != "" {
					gridColor = "#" + c.resolveColor(c.YAxis.GridlineColor)
				}
				fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`,
					plotX, yPos, plotX+plotW, yPos, gridColor)
			}
			fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`,
				plotX-4, yPos, plotX, yPos, axisColor)
			label := formatAxisValue(val, c.YAxis.NumFmt, c.PercentStacked)
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="end" %s>%s</text>`,
				plotX-10, yPos+3, chartTextSVGStyle(c.YAxis.LabelStyle, 9, false, "000000"), escapeSVG(label))
		}
	}

	// Axis titles
	if c.YAxis.Title != "" {
		fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" %s transform="rotate(-90 %d %d)">%s</text>`,
			x+14, plotY+plotH/2, chartTextSVGStyle(c.YAxis.TitleStyle, 11, false, "333333"), x+14, plotY+plotH/2, escapeSVG(c.YAxis.Title))
	}
	if c.XAxis.Title != "" {
		fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" %s>%s</text>`,
			plotX+plotW/2, y+h-5, chartTextSVGStyle(c.XAxis.TitleStyle, 11, false, "333333"), escapeSVG(c.XAxis.Title))
	}
}

func (c *Chart) renderCategoryComboChart(b *strings.Builder, x, y, w, h int) {
	primaryIndices := make([]int, 0, len(c.Series))
	for i := range c.Series {
		if !c.Series[i].UsesSecondaryValueAxis() {
			primaryIndices = append(primaryIndices, i)
		}
	}
	primaryMin, primaryMax := c.axisRangeForSeriesIndices(primaryIndices, c.YAxis)
	if primaryMax <= primaryMin {
		primaryMax = primaryMin + 1
	}
	c.renderAxes(b, x, y, w, h, false)

	secondarySeries := c.secondaryAxisSeriesIndices()
	secondaryMin, secondaryMax := 0.0, 1.0
	if len(secondarySeries) > 0 {
		secondaryMin, secondaryMax = c.axisRangeForSeriesIndices(secondarySeries, c.YAxis2)
		if secondaryMax <= secondaryMin {
			secondaryMax = secondaryMin + 1
		}
		c.renderSecondaryYAxis(b, x, y, w, h, secondaryMin, secondaryMax)
	}

	categories := len(c.Series[0].Values)
	columnSeries := make([]int, 0, len(c.Series))
	areaSeries := make([]int, 0, len(c.Series))
	lineSeries := make([]int, 0, len(c.Series))
	for i := range c.Series {
		switch c.seriesChartType(i) {
		case ChartColumn:
			columnSeries = append(columnSeries, i)
		case ChartArea:
			areaSeries = append(areaSeries, i)
		case ChartLine:
			lineSeries = append(lineSeries, i)
		}
	}

	plot := c.cartesianPlotRect(x, y, w, h)
	plotX, plotY, plotW, plotH := plot.X, plot.Y, plot.W, plot.H
	groupSize := float64(plotW) / float64(categories)
	seriesRange := func(seriesIdx int) (float64, float64) {
		if seriesIdx >= 0 && seriesIdx < len(c.Series) && c.Series[seriesIdx].UsesSecondaryValueAxis() {
			return secondaryMin, secondaryMax
		}
		return primaryMin, primaryMax
	}
	barSize := groupSize * 0.7
	if len(columnSeries) > 0 {
		barSize = (groupSize * 0.7) / float64(len(columnSeries))
	}
	if barSize < 1 {
		barSize = 1
	}

	for _, seriesIdx := range areaSeries {
		series := c.Series[seriesIdx]
		minVal, maxVal := seriesRange(seriesIdx)
		var path strings.Builder
		baseY := plotY + plotH
		coords := make([][2]int, len(series.Values))
		for i, val := range series.Values {
			px := plotX + int(float64(i)*groupSize) + int(groupSize/2)
			py := projectAxisValueY(plot, val, minVal, maxVal)
			coords[i] = [2]int{px, py}
		}
		startX := plotX
		if len(coords) > 0 {
			startX = coords[0][0]
		}
		fmt.Fprintf(&path, "M%d,%d", startX, baseY)
		if series.Smooth && len(coords) > 2 {
			for i := 0; i < len(coords); i++ {
				if i == 0 {
					fmt.Fprintf(&path, " L%d,%d", coords[i][0], coords[i][1])
				} else {
					cp1x, cp1y, cp2x, cp2y := bezierControlPoints(coords, i)
					fmt.Fprintf(&path, " C%d,%d %d,%d %d,%d", cp1x, cp1y, cp2x, cp2y, coords[i][0], coords[i][1])
				}
			}
		} else {
			for _, coord := range coords {
				fmt.Fprintf(&path, " L%d,%d", coord[0], coord[1])
			}
		}
		lastX := plotX + int(float64(categories-1)*groupSize) + int(groupSize/2)
		fmt.Fprintf(&path, " L%d,%d Z", lastX, baseY)
		alpha := 0.3
		if series.FillAlpha > 0 {
			alpha = series.FillAlpha
		}
		strokeWidth := 2
		if series.LineWidth > 0 {
			strokeWidth = series.LineWidth / 12700
			if strokeWidth < 1 {
				strokeWidth = 1
			}
		}
		fmt.Fprintf(b, `<path d="%s" fill="#%s" fill-opacity="%g" stroke="#%s" stroke-width="%d"/>`,
			path.String(), c.seriesFillColor(seriesIdx), alpha, c.seriesLineColor(seriesIdx), strokeWidth)
	}

	for catIdx := 0; catIdx < categories; catIdx++ {
		for pos, seriesIdx := range columnSeries {
			series := c.Series[seriesIdx]
			if catIdx >= len(series.Values) {
				continue
			}
			minVal, maxVal := seriesRange(seriesIdx)
			val := series.Values[catIdx]
			bw := int(barSize * 0.9)
			baseY := clampToPlotY(plot, projectAxisValueY(plot, 0, minVal, maxVal))
			valueY := clampToPlotY(plot, projectAxisValueY(plot, val, minVal, maxVal))
			bh := absInt(baseY - valueY)
			bx := plotX + int(float64(catIdx)*groupSize) + int(groupSize*0.15) + int(float64(pos)*barSize)
			by := minInt(baseY, valueY)
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="#%s" stroke="#%s" stroke-width="1"/>`,
				bx, by, bw, bh, c.seriesFillColorFor(seriesIdx, catIdx), c.seriesStrokeColor(seriesIdx))
		}
	}

	for _, seriesIdx := range lineSeries {
		series := c.Series[seriesIdx]
		minVal, maxVal := seriesRange(seriesIdx)
		color := c.seriesLineColor(seriesIdx)
		strokeWidth := 2
		if series.LineWidth > 0 {
			strokeWidth = series.LineWidth / 12700
			if strokeWidth < 1 {
				strokeWidth = 1
			}
		}
		var path strings.Builder
		coords := make([][2]int, len(series.Values))
		for i, val := range series.Values {
			px := plotX + int(float64(i)*groupSize) + int(groupSize/2)
			py := projectAxisValueY(plot, val, minVal, maxVal)
			coords[i] = [2]int{px, py}
			if i == 0 {
				path.WriteString("M")
			} else {
				path.WriteString(" L")
			}
			fmt.Fprintf(&path, "%d,%d", px, py)
		}
		if series.Smooth && len(coords) > 2 {
			fmt.Fprintf(b, `<path d="%s" fill="none" stroke="#%s" stroke-width="%d" stroke-linecap="round" stroke-linejoin="round"%s/>`,
				smoothPath(coords), color, strokeWidth, strokeDashAttr(series.LinePattern))
		} else {
			fmt.Fprintf(b, `<path d="%s" fill="none" stroke="#%s" stroke-width="%d" stroke-linecap="round" stroke-linejoin="round"%s/>`,
				path.String(), color, strokeWidth, strokeDashAttr(series.LinePattern))
		}
		markerColor := c.seriesMarkerColor(seriesIdx)
		markerSize := c.seriesMarkerSize(seriesIdx)
		for _, coord := range coords {
			c.renderMarker(b, coord[0], coord[1], c.seriesMarkerShape(seriesIdx), markerColor, markerSize)
		}
	}

	for i := 0; i < categories && i < len(c.Categories); i++ {
		px := plotX + int(float64(i)*groupSize) + int(groupSize/2)
		fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="10" fill="#666">%s</text>`,
			px, plotY+plotH+20, escapeSVG(c.Categories[i]))
	}
}

func (c *Chart) seriesChartType(index int) ChartType {
	if index < 0 || index >= len(c.Series) {
		return c.ChartType
	}
	if c.Series[index].ChartType != ChartUnknown {
		return c.Series[index].ChartType
	}
	return c.ChartType
}

func (c *Chart) hasMixedSeriesTypes() bool {
	if len(c.Series) == 0 {
		return false
	}
	base := c.seriesChartType(0)
	for i := 1; i < len(c.Series); i++ {
		if c.seriesChartType(i) != base {
			return true
		}
	}
	return false
}

// chartGroups returns the chart groups. If none were parsed (e.g. older code
// that only sets ChartType), it synthesises groups from the flat Series slice.
func (c *Chart) chartGroups() []ChartGroup {
	if len(c.Groups) > 0 {
		return c.Groups
	}
	if len(c.Series) == 0 {
		return nil
	}
	// Group series by their effective chart type.
	type bucket struct {
		indices   []int
		secondary bool
	}
	buckets := make(map[ChartType]*bucket)
	for i, s := range c.Series {
		ct := c.seriesChartType(i)
		b, ok := buckets[ct]
		if !ok {
			b = &bucket{}
			buckets[ct] = b
		}
		b.indices = append(b.indices, i)
		if s.secondaryValueAxis {
			b.secondary = true
		}
	}
	// Build a ChartGroup per bucket.
	groups := make([]ChartGroup, 0, len(buckets))
	for ct, b := range buckets {
		g := ChartGroup{
			ChartType:     ct,
			SeriesIndices: b.indices,
		}
		// Default axis IDs: primary [1,2], secondary [1,3].
		if b.secondary {
			g.AxisIDs = []int{1, 3}
		} else {
			g.AxisIDs = []int{1, 2}
		}
		// Copy legacy chart-level options into each synthetic group.
		switch ct {
		case ChartBar, ChartColumn, ChartBar3D, ChartColumn3D:
			g.GapWidth = c.GapWidth
			g.Overlap = c.Overlap
			g.VaryColors = c.VaryColors
			if ct == ChartBar {
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
		groups = append(groups, g)
	}
	return groups
}

func (c *Chart) canRenderCategoryComboChart() bool {
	groups := c.chartGroups()
	if len(groups) <= 1 {
		return false
	}
	base := groups[0].ChartType
	hasMixed := false
	for i := 1; i < len(groups); i++ {
		if groups[i].ChartType != base {
			hasMixed = true
		}
	}
	if !hasMixed {
		return false
	}
	for _, g := range groups {
		switch g.ChartType {
		case ChartColumn, ChartLine, ChartArea:
		default:
			return false
		}
	}
	return true
}

func (c *Chart) secondaryAxisSeriesIndices() []int {
	indices := make([]int, 0, len(c.Series))
	for i := range c.Series {
		if c.Series[i].secondaryValueAxis {
			indices = append(indices, i)
		}
	}
	return indices
}

func (c *Chart) seriesAxisMax(seriesIdx int, primaryMax, secondaryMax float64) float64 {
	if seriesIdx >= 0 && seriesIdx < len(c.Series) && c.Series[seriesIdx].secondaryValueAxis && secondaryMax > 0 {
		return secondaryMax
	}
	return primaryMax
}

func (c *Chart) resolveColor(color string) string {
	if strings.HasPrefix(color, "scheme:") {
		name := strings.TrimPrefix(color, "scheme:")
		if c.ThemeColors != nil {
			if hex, ok := c.ThemeColors[name]; ok {
				return hex
			}
		}
		return defaultThemeColor(name)
	}
	return color
}

func defaultThemeColor(name string) string {
	switch strings.ToLower(name) {
	case "accent1":
		return "4472C4"
	case "accent2":
		return "ED7D31"
	case "accent3":
		return "A5A5A5"
	case "accent4":
		return "FFC000"
	case "accent5":
		return "5B9BD5"
	case "accent6":
		return "70AD47"
	case "dk1", "tx1", "text1":
		return "44546A"
	case "lt1", "bg1", "background1":
		return "FFFFFF"
	case "dk2", "tx2", "text2":
		return "44546A"
	case "lt2", "bg2", "background2":
		return "E7E6E6"
	case "hlink":
		return "0563C1"
	case "folhlink":
		return "954F72"
	}
	return "999999"
}

// themeAccentColor returns a theme accent color for the given index,
// cycling through accent1–accent6. It prefers the resolved ThemeColors
// map, then falls back to defaultThemeColor, then HexColor.
func (c *Chart) themeAccentColor(index int) string {
	accents := []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6"}
	name := accents[index%len(accents)]
	if c.ThemeColors != nil {
		if hex, ok := c.ThemeColors[name]; ok {
			return hex
		}
	}
	return defaultThemeColor(name)
}

func (c *Chart) seriesFillColor(index int) string {
	if index >= 0 && index < len(c.Series) {
		if color := c.resolveSolidFill(c.Series[index].fillRef); color != "" {
			return color
		}
		if c.Series[index].FillColor != "" {
			return c.resolveColor(c.Series[index].FillColor)
		}
		if c.Series[index].Color != "" {
			return c.resolveColor(c.Series[index].Color)
		}
	}
	return c.themeAccentColor(index)
}

func (c *Chart) seriesStrokeColor(index int) string {
	if index >= 0 && index < len(c.Series) {
		if color := c.resolveSolidFill(c.Series[index].lineRef); color != "" {
			return color
		}
		if c.Series[index].LineColor != "" {
			return c.resolveColor(c.Series[index].LineColor)
		}
		if c.Series[index].FillColor != "" {
			return "000000"
		}
	}
	return "FFFFFF"
}

func (c *Chart) seriesLineColor(index int) string {
	if index >= 0 && index < len(c.Series) {
		if c.Series[index].MarkerColor != "" {
			return c.resolveColor(c.Series[index].MarkerColor)
		}
		if color := c.resolveSolidFill(c.Series[index].lineRef); color != "" {
			return color
		}
		if c.Series[index].LineColor != "" {
			return c.resolveColor(c.Series[index].LineColor)
		}
		if c.Series[index].Color != "" {
			return c.resolveColor(c.Series[index].Color)
		}
		if c.seriesChartType(index) != c.ChartType {
			return "FF0000"
		}
	}
	return c.themeAccentColor(index)
}

func (c *Chart) chartAreaFillColor() string {
	if color := c.resolveSolidFill(c.ChartArea.fillRef); color != "" {
		return color
	}
	if c.ChartArea.FillColor != "" {
		return c.resolveColor(c.ChartArea.FillColor)
	}
	return ""
}

func (c *Chart) resolveSolidFill(fill *drawing.SolidFill) string {
	if fill == nil {
		return ""
	}
	base := strings.TrimSpace(fill.Color)
	if base == "" && fill.ThemeColor != "" {
		if c.ThemeColors != nil {
			base = strings.TrimSpace(c.ThemeColors[fill.ThemeColor])
		}
		if base == "" {
			base = defaultThemeColor(fill.ThemeColor)
		}
	}
	if base == "" {
		return ""
	}
	return fill.ApplyColorTransforms(base)
}

// seriesFillColorFor returns the fill color for a specific series/point,
// respecting per-point overrides and VaryColors for single-series charts.
func (c *Chart) seriesFillColorFor(seriesIdx, pointIdx int) string {
	if seriesIdx >= 0 && seriesIdx < len(c.Series) {
		for _, dp := range c.Series[seriesIdx].DataPoints {
			if dp.Index == pointIdx && dp.FillColor != "" {
				return c.resolveColor(dp.FillColor)
			}
		}
	}
	if c.VaryColors && len(c.Series) == 1 {
		return c.seriesFillColor(pointIdx)
	}
	return c.seriesFillColor(seriesIdx)
}

// seriesLineColorFor returns the line color for a specific series/point,
// respecting per-point overrides and VaryColors for single-series charts.
func (c *Chart) seriesLineColorFor(seriesIdx, pointIdx int) string {
	if seriesIdx >= 0 && seriesIdx < len(c.Series) {
		for _, dp := range c.Series[seriesIdx].DataPoints {
			if dp.Index == pointIdx && dp.LineColor != "" {
				return c.resolveColor(dp.LineColor)
			}
		}
	}
	if c.VaryColors && len(c.Series) == 1 {
		return c.seriesLineColor(pointIdx)
	}
	return c.seriesLineColor(seriesIdx)
}

// seriesExplosionFor returns the explosion value for a specific pie point.
func (c *Chart) seriesExplosionFor(seriesIdx, pointIdx int) int {
	if seriesIdx >= 0 && seriesIdx < len(c.Series) {
		for _, dp := range c.Series[seriesIdx].DataPoints {
			if dp.Index == pointIdx {
				return dp.Explosion
			}
		}
	}
	return c.Series[seriesIdx].Explosion
}

func strokeDashAttr(pattern int) string {
	if pattern == 0 {
		return ""
	}
	var dash string
	switch pattern {
	case 1: // dash
		dash = ` stroke-dasharray="6,3"`
	case 2: // dot
		dash = ` stroke-dasharray="2,3"`
	case 3: // dash-dot
		dash = ` stroke-dasharray="6,3,2,3"`
	case 4: // long dash
		dash = ` stroke-dasharray="10,3"`
	case 5: // long dash-dot
		dash = ` stroke-dasharray="10,3,2,3"`
	case 6: // long dash-dot-dot
		dash = ` stroke-dasharray="10,3,2,3,2,3"`
	}
	return dash
}

func (c *Chart) seriesMarkerShape(index int) string {
	if index >= 0 && index < len(c.Series) && c.Series[index].MarkerShape != "" {
		return c.Series[index].MarkerShape
	}
	return "circle"
}

func (c *Chart) seriesMarkerColor(index int) string {
	if index >= 0 && index < len(c.Series) && c.Series[index].MarkerColor != "" {
		return c.resolveColor(c.Series[index].MarkerColor)
	}
	return c.seriesLineColor(index)
}

func (c *Chart) seriesMarkerSize(index int) int {
	if index >= 0 && index < len(c.Series) && c.Series[index].MarkerSize > 0 {
		return c.Series[index].MarkerSize
	}
	return 0
}

func (c *Chart) renderMarker(b *strings.Builder, x, y int, shape, color string, size int) {
	if size <= 0 {
		size = 4 // default radius / half-size
	}
	switch shape {
	case "none":
		// No marker
	case "diamond":
		fmt.Fprintf(b, `<path d="M%d,%d L%d,%d L%d,%d L%d,%d Z" fill="#%s" stroke="#%s" stroke-width="1"/>`,
			x, y-size, x+size, y, x, y+size, x-size, y, color, color)
	case "square":
		s2 := size + size/2
		fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="#%s" stroke="white" stroke-width="1"/>`,
			x-size, y-size, s2, s2, color)
	case "triangle":
		fmt.Fprintf(b, `<path d="M%d,%d L%d,%d L%d,%d Z" fill="#%s" stroke="white" stroke-width="1"/>`,
			x, y-size, x+size, y+size-1, x-size, y+size-1, color)
	case "x":
		fmt.Fprintf(b, `<path d="M%d,%d L%d,%d M%d,%d L%d,%d" stroke="#%s" stroke-width="2"/>`,
			x-size, y-size, x+size, y+size, x+size, y-size, x-size, y+size, color)
	case "star":
		renderStarMarker(b, x, y, color, size)
	case "plus":
		fmt.Fprintf(b, `<path d="M%d,%d L%d,%d M%d,%d L%d,%d" stroke="#%s" stroke-width="2"/>`,
			x, y-size, x, y+size, x-size, y, x+size, y, color)
	default:
		fmt.Fprintf(b, `<circle cx="%d" cy="%d" r="%d" fill="#%s" stroke="white" stroke-width="1"/>`,
			x, y, size, color)
	}
}

func renderStarMarker(b *strings.Builder, cx, cy int, color string, size int) {
	if size <= 0 {
		size = 5
	}
	var pts []string
	for i := 0; i < 10; i++ {
		angle := float64(i)*math.Pi/5 - math.Pi/2
		r := size
		if i%2 == 1 {
			r = size / 2
			if r < 1 {
				r = 1
			}
		}
		px := cx + int(float64(r)*math.Cos(angle))
		py := cy + int(float64(r)*math.Sin(angle))
		pts = append(pts, fmt.Sprintf("%d,%d", px, py))
	}
	fmt.Fprintf(b, `<polygon points="%s" fill="#%s" stroke="white" stroke-width="1"/>`,
		strings.Join(pts, " "), color)
}

func scatterRenderOptions(series Series) (drawLine, smooth, drawMarker bool) {
	style := strings.ToLower(series.ScatterStyle)
	switch style {
	case "line":
		return true, false, false
	case "linemarker", "line_marker", "line-marker", "linemarkerstyle", "lineMarker":
		return true, false, series.MarkerShape != "none"
	case "marker":
		return false, false, series.MarkerShape != "none"
	case "none":
		return false, false, false
	case "smooth":
		return true, true, false
	case "smoothmarker", "smooth_marker", "smooth-marker", "smoothMarker":
		return true, true, series.MarkerShape != "none"
	default:
		return series.LinePattern != 5 || series.Smooth, series.Smooth, series.MarkerShape != "none"
	}
}

func (c *Chart) chartValueRange() (float64, float64, bool) {
	minVal, maxVal := 0.0, 0.0
	haveValues := false
	for _, series := range c.Series {
		for _, value := range series.Values {
			if !haveValues {
				minVal, maxVal = value, value
				haveValues = true
				continue
			}
			if value < minVal {
				minVal = value
			}
			if value > maxVal {
				maxVal = value
			}
		}
	}
	return minVal, maxVal, haveValues
}

func (c *Chart) axisRange() (float64, float64) {
	minVal, maxVal, haveValues := c.chartValueRange()
	if !haveValues {
		// Charts with authored series metadata but no cached/resolved points are
		// valid during OOXML resolution. Keep a stable automatic domain instead
		// of calling MinValue(), which assumes Series[0].Values[0] exists.
		minVal, maxVal = 0, 0
	}
	// Axis.Min/Max use zero as the legacy "auto" sentinel in the canonical
	// model, so only non-zero authored bounds override the data-derived range.
	if c.YAxis.Min != 0 {
		minVal = c.YAxis.Min
	}
	if c.YAxis.Max != 0 {
		maxVal = c.YAxis.Max
	}
	if c.YAxis.Min == 0 && minVal > 0 {
		minVal = 0
	}
	if c.YAxis.Max == 0 && maxVal < 0 {
		maxVal = 0
	}
	span := maxVal - minVal
	if span <= 0 {
		return minVal, maxVal
	}
	step := axisTickStep(span, c.YAxis.MajorUnit)
	if step > 0 {
		if c.YAxis.Min == 0 && minVal < 0 {
			minVal = math.Floor(minVal/step) * step
		}
		if c.YAxis.Max == 0 && maxVal > 0 {
			maxVal = math.Ceil(maxVal/step) * step
		}
	}
	return minVal, maxVal
}

func (c *Chart) axisRangeForSeriesIndices(indices []int, axis Axis) (float64, float64) {
	minVal, maxVal := 0.0, 0.0
	have := false
	for _, idx := range indices {
		if idx < 0 || idx >= len(c.Series) {
			continue
		}
		for _, value := range c.Series[idx].Values {
			if !have {
				minVal, maxVal, have = value, value, true
				continue
			}
			if value < minVal {
				minVal = value
			}
			if value > maxVal {
				maxVal = value
			}
		}
	}
	if !have {
		minVal, maxVal = 0, 0
	}
	if axis.Min != 0 {
		minVal = axis.Min
	} else if minVal > 0 {
		minVal = 0
	}
	if axis.Max != 0 {
		maxVal = axis.Max
	} else if maxVal < 0 {
		maxVal = 0
	}
	span := maxVal - minVal
	if span > 0 {
		step := axisTickStep(span, axis.MajorUnit)
		if step > 0 {
			if axis.Min == 0 && minVal < 0 {
				minVal = math.Floor(minVal/step) * step
			}
			if axis.Max == 0 && maxVal > 0 {
				maxVal = math.Ceil(maxVal/step) * step
			}
		}
	}
	return minVal, maxVal
}

func (c *Chart) axisMax() float64 {
	return c.axisMaxForValueRange(c.MaxValue(), c.YAxis)
}

func (c *Chart) axisMaxForSeriesIndices(indices []int, axis Axis) float64 {
	if len(indices) == 0 {
		return c.axisMaxForValueRange(0, axis)
	}
	maxVal := 0.0
	for _, idx := range indices {
		if idx < 0 || idx >= len(c.Series) {
			continue
		}
		for _, v := range c.Series[idx].Values {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	return c.axisMaxForValueRange(maxVal, axis)
}

func (c *Chart) axisMaxForValueRange(maxVal float64, axis Axis) float64 {
	if axis.Max > 0 {
		return axis.Max
	}
	if maxVal <= 0 {
		return 0
	}
	step := axisTickStep(maxVal, axis.MajorUnit)
	if step <= 0 {
		return maxVal
	}
	return math.Ceil(maxVal/step) * step
}

func (c *Chart) axisTickStep(maxVal float64) float64 {
	return axisTickStep(maxVal, c.YAxis.MajorUnit)
}

func axisTickStep(maxVal, majorUnit float64) float64 {
	if majorUnit > 0 {
		return majorUnit
	}
	if maxVal <= 0 {
		return 1
	}
	// Target approximately 5-8 ticks.
	rough := maxVal / 5
	power := math.Pow(10, math.Floor(math.Log10(rough)))
	normalized := rough / power
	if normalized <= 1 {
		return power
	} else if normalized <= 2 {
		return 2 * power
	} else if normalized <= 5 {
		return 5 * power
	}
	return 10 * power
}

func (c *Chart) renderSecondaryYAxis(b *strings.Builder, x, y, w, h int, minVal, maxVal float64) {
	if maxVal <= minVal {
		return
	}
	plot := c.cartesianPlotRect(x, y, w, h)
	axisColor := "#000"
	rightX := plot.X + plot.W
	fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`, rightX, plot.Y, rightX, plot.Y+plot.H, axisColor)
	tick := axisTickStep(maxVal-minVal, c.YAxis2.MajorUnit)
	if tick <= 0 {
		return
	}
	startTick := math.Ceil(minVal/tick) * tick
	for val := startTick; val <= maxVal+0.001; val += tick {
		yPos := projectAxisValueY(plot, val, minVal, maxVal)
		fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`, rightX, yPos, rightX+4, yPos, axisColor)
		label := formatAxisValue(val, c.YAxis2.NumFmt, false)
		fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="start" %s>%s</text>`, rightX+8, yPos+3, chartTextSVGStyle(c.YAxis2.LabelStyle, 9, false, "000000"), escapeSVG(label))
	}
	if c.YAxis2.Title != "" {
		titleX := rightX + 24
		titleY := plot.Y + plot.H/2
		fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" %s transform="rotate(90 %d %d)">%s</text>`, titleX, titleY, chartTextSVGStyle(c.YAxis2.TitleStyle, 11, false, "333333"), titleX, titleY, escapeSVG(c.YAxis2.Title))
	}
}

// smoothPath generates a smooth SVG path through the given coordinates using cubic Bezier curves.
func smoothPath(coords [][2]int) string {
	if len(coords) == 0 {
		return ""
	}
	if len(coords) == 1 {
		return fmt.Sprintf("M%d,%d", coords[0][0], coords[0][1])
	}
	if len(coords) == 2 {
		return fmt.Sprintf("M%d,%d L%d,%d", coords[0][0], coords[0][1], coords[1][0], coords[1][1])
	}

	var b strings.Builder
	fmt.Fprintf(&b, "M%d,%d", coords[0][0], coords[0][1])
	for i := 1; i < len(coords); i++ {
		cp1x, cp1y, cp2x, cp2y := bezierControlPoints(coords, i)
		fmt.Fprintf(&b, " C%d,%d %d,%d %d,%d", cp1x, cp1y, cp2x, cp2y, coords[i][0], coords[i][1])
	}
	return b.String()
}

// bezierControlPoints computes control points for a smooth curve through coords at index i.
func bezierControlPoints(coords [][2]int, i int) (cp1x, cp1y, cp2x, cp2y int) {
	var p0, p1, p2 [2]int
	if i == 1 {
		p0 = coords[0]
		p1 = coords[1]
		p2 = coords[2]
	} else if i == len(coords)-1 {
		p0 = coords[i-2]
		p1 = coords[i-1]
		p2 = coords[i]
	} else {
		p0 = coords[i-2]
		p1 = coords[i-1]
		p2 = coords[i]
	}

	tension := 0.3
	cp1x = p1[0] + int(tension*float64(p2[0]-p0[0])/2)
	cp1y = p1[1] + int(tension*float64(p2[1]-p0[1])/2)
	cp2x = p2[0] - int(tension*float64(coords[min(i+1, len(coords)-1)][0]-p1[0])/2)
	cp2y = p2[1] - int(tension*float64(coords[min(i+1, len(coords)-1)][1]-p1[1])/2)

	// If at endpoints, adjust
	if i == 1 {
		cp1x = p1[0]
		cp1y = p1[1]
	}
	if i == len(coords)-1 {
		cp2x = p2[0]
		cp2y = p2[1]
	}
	return
}

// renderLegend renders the chart legend.
func (c *Chart) legendEntryDeleted(idx int) bool {
	for _, e := range c.Legend.Entries {
		if e.Index == idx && e.Delete {
			return true
		}
	}
	return false
}

func (c *Chart) visibleSeriesIndices() []int {
	var vis []int
	for i := range c.Series {
		if !c.legendEntryDeleted(i) {
			vis = append(vis, i)
		}
	}
	return vis
}

func (c *Chart) renderLegend(b *strings.Builder, width, height, chartX, chartY, chartW, chartH int) {
	if len(c.Series) == 0 {
		return
	}

	vis := c.visibleSeriesIndices()
	if len(vis) == 0 {
		return
	}

	if c.LegendLayout != nil {
		legendX, legendY, legendW, legendH := c.legendLayoutBox(width, height, chartX, chartY, chartW, chartH)
		c.renderLegendBox(b, vis, legendX, legendY, legendW, legendH)
		return
	}

	switch c.Legend.Position {
	case "r", "right":
		itemHeight := 20
		legendX := chartX + chartW + 10
		if legendX+70 > width {
			legendX = width - 70
		}
		startY := chartY + chartH/2 - (len(vis)*itemHeight)/2
		if startY < chartY {
			startY = chartY
		}
		for pos, i := range vis {
			y := startY + pos*itemHeight
			color := c.seriesFillColor(i)
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="12" height="12" fill="#%s"/>`,
				legendX, y, color)
			fmt.Fprintf(b, `<text x="%d" y="%d" %s>%s</text>`,
				legendX+16, y+10, chartTextSVGStyle(c.Legend.TextStyle, 11, false, "333333"), escapeSVG(c.Series[i].Name))
		}
	case "l", "left":
		itemHeight := 20
		legendX := 10
		if legendX < 0 {
			legendX = 0
		}
		startY := chartY + chartH/2 - (len(vis)*itemHeight)/2
		if startY < chartY {
			startY = chartY
		}
		for pos, i := range vis {
			y := startY + pos*itemHeight
			color := c.seriesFillColor(i)
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="12" height="12" fill="#%s"/>`,
				legendX, y, color)
			fmt.Fprintf(b, `<text x="%d" y="%d" %s>%s</text>`,
				legendX+16, y+10, chartTextSVGStyle(c.Legend.TextStyle, 11, false, "333333"), escapeSVG(c.Series[i].Name))
		}
	case "t", "top":
		itemWidth := 100
		startX := (width - (len(vis) * itemWidth)) / 2
		if startX < 10 {
			startX = 10
		}
		ly := chartY - 30
		if ly < 5 {
			ly = 5
		}
		for pos, i := range vis {
			lx := startX + pos*itemWidth
			color := c.seriesFillColor(i)
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="12" height="12" fill="#%s"/>`,
				lx, ly, color)
			fmt.Fprintf(b, `<text x="%d" y="%d" %s>%s</text>`,
				lx+16, ly+10, chartTextSVGStyle(c.Legend.TextStyle, 11, false, "333333"), escapeSVG(c.Series[i].Name))
		}
	default:
		// bottom
		itemWidth := 100
		startX := (width - (len(vis) * itemWidth)) / 2
		if startX < 10 {
			startX = 10
		}
		y := height - 25
		for pos, i := range vis {
			x := startX + pos*itemWidth
			color := c.seriesFillColor(i)
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="12" height="12" fill="#%s"/>`,
				x, y, color)
			fmt.Fprintf(b, `<text x="%d" y="%d" %s>%s</text>`,
				x+16, y+10, chartTextSVGStyle(c.Legend.TextStyle, 11, false, "333333"), escapeSVG(c.Series[i].Name))
		}
	}
}

func (c *Chart) legendLayoutBox(width, height, chartX, chartY, chartW, chartH int) (int, int, int, int) {
	baseX, baseY, baseW, baseH := 0, 0, width, height
	if c.LegendLayout != nil && c.LegendLayout.LayoutTarget == "inner" {
		baseX = chartX
		baseY = chartY
		baseW = chartW
		baseH = chartH
	}
	x, y, w, h := manualLayoutRect(c.LegendLayout, baseX, baseY, baseW, baseH)
	if w <= 0 || h <= 0 {
		return baseX, baseY, baseW, baseH
	}
	return x, y, w, h
}

func (c *Chart) renderLegendBox(b *strings.Builder, vis []int, x, y, w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	padding := 5
	innerX := x + padding
	innerY := y + padding
	innerW := w - 2*padding
	if innerW < 0 {
		innerW = 0
	}
	itemHeight := 20
	if innerH := h - 2*padding; innerH >= len(vis)*itemHeight {
		for pos, i := range vis {
			y := innerY + pos*itemHeight
			color := c.seriesFillColor(i)
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="12" height="12" fill="#%s"/>`,
				innerX, y, color)
			fmt.Fprintf(b, `<text x="%d" y="%d" %s>%s</text>`,
				innerX+16, y+10, chartTextSVGStyle(c.Legend.TextStyle, 11, false, "333333"), escapeSVG(c.Series[i].Name))
		}
		return
	}
	itemWidth := innerW / len(vis)
	if itemWidth < 70 {
		itemWidth = 70
	}
	currentX := innerX
	for _, i := range vis {
		color := c.seriesFillColor(i)
		fmt.Fprintf(b, `<rect x="%d" y="%d" width="12" height="12" fill="#%s"/>`,
			currentX, innerY, color)
		fmt.Fprintf(b, `<text x="%d" y="%d" %s>%s</text>`,
			currentX+16, innerY+10, chartTextSVGStyle(c.Legend.TextStyle, 11, false, "333333"), escapeSVG(c.Series[i].Name))
		currentX += itemWidth
	}
}

func chartTextSVGStyle(style *TextProperties, defaultSize int, defaultBold bool, defaultColor string) string {
	size := float64(defaultSize)
	bold := defaultBold
	italic := false
	color := defaultColor
	if style != nil {
		if style.FontSize > 0 {
			size = float64(style.FontSize) / 100.0
		}
		bold = style.Bold
		italic = style.Italic
		if style.Color != "" {
			color = style.Color
		}
	}
	weight := "normal"
	if bold {
		weight = "bold"
	}
	fontStyle := "normal"
	if italic {
		fontStyle = "italic"
	}
	return fmt.Sprintf(`font-size="%.2f" font-weight="%s" font-style="%s" fill="#%s"`, size, weight, fontStyle, color)
}

func formatAxisValue(val float64, numFmt string, percentStacked bool) string {
	if percentStacked {
		if numFmt == "" || numFmt == "General" {
			return fmt.Sprintf("%.0f%%", val)
		}
		return formatDataLabelValue(val, numFmt, true)
	}
	return formatDataLabelValue(val, numFmt, false)
}

// escapeSVG escapes special characters for SVG text content using XML entities.
func escapeSVG(s string) string {
	var result strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			result.WriteString("&amp;")
		case '<':
			result.WriteString("&lt;")
		case '>':
			result.WriteString("&gt;")
		case '"':
			result.WriteString("&quot;")
		case '\'':
			result.WriteString("&apos;")
		default:
			result.WriteRune(r)
		}
	}
	return result.String()
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// stdDev computes the population standard deviation of values.
func stdDev(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(n)
	var sqSum float64
	for _, v := range values {
		d := v - mean
		sqSum += d * d
	}
	return math.Sqrt(sqSum / float64(n))
}

// errorBarMagnitude returns the plus and minus error magnitudes for a data point.
func (c *Chart) errorBarMagnitude(eb *ErrorBars, val float64, values []float64, idx int) (plus, minus float64) {
	if eb == nil {
		return 0, 0
	}
	var mag float64
	switch eb.ValType {
	case "fixedVal":
		mag = eb.Value
	case "percentage":
		mag = math.Abs(val) * eb.Value / 100.0
	case "stdDev":
		mag = stdDev(values)
	case "stdErr":
		n := len(values)
		if n > 0 {
			mag = stdDev(values) / math.Sqrt(float64(n))
		}
	case "cust":
		if idx < len(eb.Plus) {
			mag = eb.Plus[idx]
		}
	}
	barType := eb.BarType
	if barType == "" {
		barType = "both"
	}
	switch barType {
	case "plus":
		return mag, 0
	case "minus":
		return 0, mag
	default:
		if eb.ValType == "cust" {
			p, m := mag, mag
			if idx < len(eb.Plus) {
				p = eb.Plus[idx]
			}
			if idx < len(eb.Minus) {
				m = eb.Minus[idx]
			}
			return p, m
		}
		return mag, mag
	}
}

// drawErrorBar draws an error bar at the given data point.
func (c *Chart) drawErrorBar(b *strings.Builder, eb *ErrorBars, px, py int, val float64, maxVal float64, plotH int, xRange float64, plotW int, values []float64, idx int) {
	if eb == nil || maxVal == 0 {
		return
	}
	capW := 4
	horizontal := eb.Direction == "x"
	if horizontal {
		if xRange == 0 {
			xRange = maxVal
		}
		plus, minus := c.errorBarMagnitude(eb, val, values, idx)
		pp := int((plus / xRange) * float64(plotW))
		pm := int((minus / xRange) * float64(plotW))
		x1 := px - pm
		x2 := px + pp
		fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#666" stroke-width="1"/>`, x1, py, x2, py)
		if !eb.NoEndCap {
			fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#666" stroke-width="1"/>`, x1, py-capW, x1, py+capW)
			fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#666" stroke-width="1"/>`, x2, py-capW, x2, py+capW)
		}
	} else {
		plus, minus := c.errorBarMagnitude(eb, val, values, idx)
		pp := int((plus / maxVal) * float64(plotH))
		pm := int((minus / maxVal) * float64(plotH))
		y1 := py + pm // down
		y2 := py - pp // up
		fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#666" stroke-width="1"/>`, px, y1, px, y2)
		if !eb.NoEndCap {
			fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#666" stroke-width="1"/>`, px-capW, y1, px+capW, y1)
			fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#666" stroke-width="1"/>`, px-capW, y2, px+capW, y2)
		}
	}
}

// renderDataTable draws a data table below the plot area.
func (c *Chart) renderDataTable(b *strings.Builder, x, y, w, h int) {
	if c.DataTable == nil {
		return
	}
	dt := c.DataTable
	rowHeight := 18
	colCount := 1 + len(c.Series)
	rows := 1 + len(c.Series[0].Values)
	if len(c.Categories) > 0 && len(c.Categories) < len(c.Series[0].Values) {
		rows = 1 + len(c.Categories)
	}
	tableH := rowHeight * rows
	tableW := w
	tableX := x
	tableY := y + h + 10

	// Background
	fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="white" stroke="none"/>`, tableX, tableY, tableW, tableH)

	colWidth := tableW / colCount
	if colWidth < 60 {
		colWidth = 60
	}

	// Outline border
	if dt.ShowOutline {
		fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" fill="none" stroke="#333" stroke-width="1"/>`, tableX, tableY, tableW, tableH)
	}

	// Header row
	headerY := tableY + 14
	fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="10" font-weight="bold" fill="#333">%s</text>`, tableX+colWidth/2, headerY, "")
	for si, series := range c.Series {
		cx := tableX + (si+1)*colWidth + colWidth/2
		if dt.ShowKeys {
			color := c.seriesFillColor(si)
			fmt.Fprintf(b, `<rect x="%d" y="%d" width="8" height="8" fill="#%s"/>`, cx-20, headerY-8, color)
		}
		fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="10" font-weight="bold" fill="#333">%s</text>`, cx, headerY, escapeSVG(series.Name))
	}

	// Horizontal borders (header separator)
	if dt.ShowHorzBorder {
		fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#333" stroke-width="1"/>`, tableX, tableY+rowHeight, tableX+tableW, tableY+rowHeight)
	}

	// Data rows
	for ri := 0; ri < rows-1; ri++ {
		ry := tableY + (ri+1)*rowHeight + 14
		catLabel := ""
		if ri < len(c.Categories) {
			catLabel = c.Categories[ri]
		}
		fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="9" fill="#333">%s</text>`, tableX+colWidth/2, ry, escapeSVG(catLabel))
		for si, series := range c.Series {
			cx := tableX + (si+1)*colWidth + colWidth/2
			valText := ""
			if ri < len(series.Values) {
				valText = fmt.Sprintf("%.0f", series.Values[ri])
			}
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="9" fill="#333">%s</text>`, cx, ry, valText)
		}
		if dt.ShowHorzBorder {
			fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#999" stroke-width="0.5"/>`, tableX, tableY+(ri+2)*rowHeight, tableX+tableW, tableY+(ri+2)*rowHeight)
		}
	}

	// Vertical borders
	if dt.ShowVertBorder {
		for ci := 1; ci < colCount; ci++ {
			vx := tableX + ci*colWidth
			fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#999" stroke-width="0.5"/>`, vx, tableY, vx, tableY+tableH)
		}
	}
}

// effectiveLabelStyle returns the data-label text properties for a given point,
// preferring per-point overrides over series-level settings.
func effectiveLabelStyle(series *Series, catIdx int) *TextProperties {
	for _, dp := range series.DataPoints {
		if dp.Index == catIdx && dp.LabelStyle != nil {
			return dp.LabelStyle
		}
	}
	return series.LabelStyle
}

// dataLabelStyleAttrs returns SVG text style attributes. If props is nil, it
// returns the fallback size and fill. Size is in pt (FontSize is hundredths).
func dataLabelStyleAttrs(props *TextProperties, defaultSize int, defaultFill string) string {
	if props == nil {
		return fmt.Sprintf(`font-size="%d" fill="%s"`, defaultSize, defaultFill)
	}
	size := defaultSize
	if props.FontSize > 0 {
		size = props.FontSize / 100
	}
	fill := defaultFill
	if props.Color != "" {
		fill = "#" + props.Color
	}
	var attrs []string
	attrs = append(attrs, fmt.Sprintf(`font-size="%d"`, size))
	if props.Bold {
		attrs = append(attrs, `font-weight="bold"`)
	}
	if props.Italic {
		attrs = append(attrs, `font-style="italic"`)
	}
	attrs = append(attrs, fmt.Sprintf(`fill="%s"`, fill))
	return strings.Join(attrs, " ")
}

// buildDataLabel constructs the text for a data label based on series flags.
// If the data point has a custom Label, that is returned directly.
// Parts are joined with series.Separator (default ", ").
func buildDataLabel(series *Series, catIdx int, val float64, total float64, categories []string) string {
	// Check for per-point custom label.
	for _, dp := range series.DataPoints {
		if dp.Index == catIdx && dp.Label != "" {
			return dp.Label
		}
	}
	sep := series.Separator
	if sep == "" {
		sep = ", "
	}
	numFmt := series.NumFmt
	for _, dp := range series.DataPoints {
		if dp.Index == catIdx && dp.NumFmt != "" {
			numFmt = dp.NumFmt
			break
		}
	}
	var parts []string
	if series.ShowSerName && series.Name != "" {
		parts = append(parts, series.Name)
	}
	if series.ShowCatName && catIdx < len(categories) {
		parts = append(parts, categories[catIdx])
	}
	if series.ShowPercent && total > 0 {
		pct := (val / total) * 100
		parts = append(parts, formatDataLabelValue(pct, numFmt, true))
	} else if series.ShowDataLabels {
		parts = append(parts, formatDataLabelValue(val, numFmt, false))
	}
	if series.ShowBubbleSize && catIdx < len(series.BubbleSizes) {
		parts = append(parts, formatDataLabelValue(series.BubbleSizes[catIdx], "", false))
	}
	return strings.Join(parts, sep)
}

// formatDataLabelValue formats a numeric value using an Excel-style number format.
// isPercent indicates the value is already a percentage number (e.g. 25 for 25%).
func formatDataLabelValue(val float64, numFmt string, isPercent bool) string {
	if numFmt == "" || numFmt == "General" {
		if isPercent {
			return fmt.Sprintf("%.0f%%", val)
		}
		return fmt.Sprintf("%.0f", val)
	}

	// Percentage formats: 0%, 0.0%, 0.00%. For ordinary numeric values the
	// stored value is a fraction (0.51 == 51%). Callers rendering an already
	// computed percentage, such as pie data-label percentages, pass isPercent=true
	// and therefore must not be scaled a second time.
	if strings.HasSuffix(numFmt, "%") {
		base := strings.TrimSuffix(numFmt, "%")
		dec := countDecimalPlaces(base)
		if !isPercent {
			val *= 100
		}
		return fmt.Sprintf("%.*f%%", dec, val)
	}

	// Thousands separator: #,##0, #,##0.00
	if strings.Contains(numFmt, ",") {
		dec := countDecimalPlaces(numFmt)
		return formatWithThousands(val, dec)
	}

	// Decimal places: 0, 0.0, 0.00
	dec := countDecimalPlaces(numFmt)
	return fmt.Sprintf("%.*f", dec, val)
}

// countDecimalPlaces returns the number of '0' digits after the first '.' in a
// simple format string. Returns 0 if there is no '.'.
func countDecimalPlaces(numFmt string) int {
	if i := strings.Index(numFmt, "."); i >= 0 {
		return len(numFmt) - i - 1
	}
	return 0
}

// formatWithThousands formats a value with thousands separators and the given
// number of decimal places.
func formatWithThousands(val float64, decimals int) string {
	var numStr string
	if decimals <= 0 {
		numStr = fmt.Sprintf("%.0f", val)
	} else {
		numStr = fmt.Sprintf("%.*f", decimals, val)
	}
	parts := strings.Split(numStr, ".")
	intPart := addThousandsSeparators(parts[0])
	if len(parts) > 1 {
		return intPart + "." + parts[1]
	}
	return intPart
}

func addThousandsSeparators(s string) string {
	start := 0
	if len(s) > 0 && (s[0] == '-' || s[0] == '+') {
		start = 1
	}
	var result strings.Builder
	if start > 0 {
		result.WriteByte(s[0])
	}
	n := len(s) - start
	for i := start; i < len(s); i++ {
		if i > start && (n-(i-start))%3 == 0 {
			result.WriteByte(',')
		}
		result.WriteByte(s[i])
	}
	return result.String()
}

// renderDataLabelLeaderLine draws a thin straight leader line from a data point/bar
// to its label. The line stops slightly before the label so it does not overlap text.
func renderDataLabelLeaderLine(b *strings.Builder, fromX, fromY, toX, toY int) {
	fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#999" stroke-width="1"/>`,
		fromX, fromY, toX, toY)
}

// renderDataLabelElbowLine draws an L-shaped callout with a right-angle bend.
// When horizontalFirst is true the bend is at (toX, fromY); otherwise at (fromX, toY).
func renderDataLabelElbowLine(b *strings.Builder, fromX, fromY, toX, toY int, horizontalFirst bool) {
	bendX, bendY := toX, fromY
	if !horizontalFirst {
		bendX, bendY = fromX, toY
	}
	if (bendX == fromX && bendY == fromY) || (bendX == toX && bendY == toY) {
		// Degenerate to straight line.
		fmt.Fprintf(b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#999" stroke-width="1"/>`,
			fromX, fromY, toX, toY)
		return
	}
	fmt.Fprintf(b, `<polyline points="%d,%d %d,%d %d,%d" fill="none" stroke="#999" stroke-width="1"/>`,
		fromX, fromY, bendX, bendY, toX, toY)
}

// dataLabelOffsetForBar returns label x, y offsets and text-anchor for a bar/column
// data label based on the series LabelPosition and horizontal orientation.
func dataLabelOffsetForBar(series Series, horizontal bool, bx, by, bw, bh int) (labelX, labelY int, anchor string) {
	labelX = bx + bw/2
	labelY = by - 5
	anchor = "middle"
	if horizontal {
		labelY = by + bh/2 + 4
		labelX = bx + bw + 5
		anchor = "start"
		switch series.LabelPosition {
		case 2: // inEnd
			labelX = bx + bw - 5
			anchor = "end"
		case 3: // ctr
			labelX = bx + bw/2
			anchor = "middle"
		case 4: // inBase
			labelX = bx + 5
			anchor = "start"
		}
	} else {
		switch series.LabelPosition {
		case 2: // inEnd
			labelY = by + 12
			anchor = "middle"
		case 3: // ctr
			labelY = by + bh/2 + 4
			anchor = "middle"
		case 4: // inBase
			labelY = by + bh - 2
			anchor = "middle"
		}
	}
	return labelX, labelY, anchor
}
