package edit

import (
	"fmt"
	"strings"

	chartmodel "office/pkg/ooxml/chart"
	sheetir "office/pkg/sheet/ir"
)

// ChartDrawing wraps an editable worksheet chart drawing.
type ChartDrawing struct {
	drawing *sheetir.Drawing
	chart   *chartmodel.Chart
}

// NewChart creates a basic chart model for use with AddChart.
func NewChart(chartType chartmodel.ChartType, title string) *chartmodel.Chart {
	return &chartmodel.Chart{
		ChartType: chartType,
		Title:     title,
		Legend:    chartmodel.Legend{Show: true, Position: "r"},
	}
}

// AddChart adds a chart drawing with an explicit drawing position.
func (s *Sheet) AddChart(name string, chart *chartmodel.Chart, position sheetir.DrawingPosition) (*ChartDrawing, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	if chart == nil {
		return nil, fmt.Errorf("sheet edit: nil chart")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = s.nextDrawingName("Chart")
	}
	if err := validateDrawingPosition(position); err != nil {
		return nil, err
	}
	drawing := &sheetir.Drawing{
		Name:     name,
		Type:     sheetir.DrawingTypeChart,
		Position: position,
		Chart: &sheetir.DrawingChart{
			Chart: chart,
			SVG:   chart.SVG(600, 400),
		},
	}
	s.sheet.Drawings = append(s.sheet.Drawings, drawing)
	return &ChartDrawing{drawing: drawing, chart: chart}, nil
}

// AddChartOneCell adds a chart anchored to a single cell.
func (s *Sheet) AddChartOneCell(name string, chart *chartmodel.Chart, row, col int, widthEMU, heightEMU int64) (*ChartDrawing, error) {
	return s.AddChart(name, chart, sheetir.DrawingPosition{
		Anchor:   sheetir.AnchorOneCell,
		FromCell: sheetir.DrawingCellAnchor{Row: row, Col: col},
		Width:    widthEMU,
		Height:   heightEMU,
	})
}

// AddChartTwoCell adds a chart anchored between two cells.
func (s *Sheet) AddChartTwoCell(name string, chart *chartmodel.Chart, fromRow, fromCol, toRow, toCol int) (*ChartDrawing, error) {
	return s.AddChart(name, chart, sheetir.DrawingPosition{
		Anchor:   sheetir.AnchorTwoCell,
		FromCell: sheetir.DrawingCellAnchor{Row: fromRow, Col: fromCol},
		ToCell:   sheetir.DrawingCellAnchor{Row: toRow, Col: toCol},
	})
}

// AddChartAbsolute adds a chart using absolute EMU coordinates.
func (s *Sheet) AddChartAbsolute(name string, chart *chartmodel.Chart, xEMU, yEMU, widthEMU, heightEMU int64) (*ChartDrawing, error) {
	return s.AddChart(name, chart, sheetir.DrawingPosition{
		Anchor: sheetir.AnchorAbsolute,
		X:      xEMU,
		Y:      yEMU,
		Width:  widthEMU,
		Height: heightEMU,
	})
}

// Chart returns a chart drawing by name.
func (s *Sheet) Chart(name string) (*ChartDrawing, bool) {
	if s == nil || s.sheet == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	for _, drawing := range s.sheet.Drawings {
		if drawing == nil || drawing.Type != sheetir.DrawingTypeChart || !strings.EqualFold(drawing.Name, name) {
			continue
		}
		chart, ok := chartFromDrawing(drawing)
		if !ok {
			return nil, false
		}
		return &ChartDrawing{drawing: drawing, chart: chart}, true
	}
	return nil, false
}

// RemoveChart removes a chart drawing by name.
func (s *Sheet) RemoveChart(name string) bool {
	if s == nil || s.sheet == nil {
		return false
	}
	name = strings.TrimSpace(name)
	for i, drawing := range s.sheet.Drawings {
		if drawing != nil && drawing.Type == sheetir.DrawingTypeChart && strings.EqualFold(drawing.Name, name) {
			s.sheet.Drawings = append(s.sheet.Drawings[:i], s.sheet.Drawings[i+1:]...)
			return true
		}
	}
	return false
}

// Name returns the chart drawing name.
func (c *ChartDrawing) Name() string {
	if c == nil || c.drawing == nil {
		return ""
	}
	return c.drawing.Name
}

// Chart returns the underlying chart model.
func (c *ChartDrawing) Chart() *chartmodel.Chart {
	if c == nil {
		return nil
	}
	return c.chart
}

// SetTitle updates the chart title.
func (c *ChartDrawing) SetTitle(title string) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.Title = title
	c.refreshSVG()
}

// SetCategories updates category labels.
func (c *ChartDrawing) SetCategories(categories []string) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.Categories = append([]string(nil), categories...)
	c.refreshSVG()
}

// SetAxes replaces the primary X and Y axis configuration.
func (c *ChartDrawing) SetAxes(xAxis, yAxis chartmodel.Axis) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.XAxis = cloneChartAxis(xAxis)
	c.chart.YAxis = cloneChartAxis(yAxis)
	c.refreshSVG()
}

// SetSecondaryYAxis replaces the secondary Y axis configuration.
func (c *ChartDrawing) SetSecondaryYAxis(axis chartmodel.Axis) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.YAxis2 = cloneChartAxis(axis)
	c.refreshSVG()
}

// AddSeries appends a chart data series.
func (c *ChartDrawing) AddSeries(series chartmodel.Series) {
	if c == nil || c.chart == nil {
		return
	}
	series.Index = len(c.chart.Series)
	c.chart.Series = append(c.chart.Series, series)
	c.refreshSVG()
}

// ReplaceSeries replaces all chart data series and normalizes their indexes.
func (c *ChartDrawing) ReplaceSeries(series []chartmodel.Series) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.Series = append([]chartmodel.Series(nil), series...)
	for i := range c.chart.Series {
		c.chart.Series[i].Index = i
	}
	c.refreshSVG()
}

// ReplaceGroups replaces combo-chart group metadata.
func (c *ChartDrawing) ReplaceGroups(groups []chartmodel.ChartGroup) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.Groups = cloneChartGroups(groups)
	c.refreshSVG()
}

// SetLegend updates legend visibility and position.
func (c *ChartDrawing) SetLegend(show bool, position string) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.Legend.Show = show
	c.chart.Legend.Position = position
	c.refreshSVG()
}

// SetLegendEntries replaces per-entry legend visibility overrides.
func (c *ChartDrawing) SetLegendEntries(entries []chartmodel.LegendEntry) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.Legend.Entries = cloneLegendEntries(entries)
	c.refreshSVG()
}

// SetPlotArea replaces plot-area styling metadata.
func (c *ChartDrawing) SetPlotArea(plotArea chartmodel.PlotArea) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.PlotArea = plotArea
	c.refreshSVG()
}

// SetChartAreaFill updates the chart-area background fill.
func (c *ChartDrawing) SetChartAreaFill(fillColor string) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.ChartArea.FillColor = strings.TrimSpace(fillColor)
	c.refreshSVG()
}

// SetManualLayout replaces plot-area manual layout overrides.
func (c *ChartDrawing) SetManualLayout(layout *chartmodel.ManualLayout) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.ManualLayout = cloneManualLayout(layout)
	c.refreshSVG()
}

// SetLegendLayout replaces legend manual layout overrides.
func (c *ChartDrawing) SetLegendLayout(layout *chartmodel.ManualLayout) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.LegendLayout = cloneManualLayout(layout)
	c.refreshSVG()
}

// SetTitleLayout replaces title manual layout overrides.
func (c *ChartDrawing) SetTitleLayout(layout *chartmodel.ManualLayout) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.TitleLayout = cloneManualLayout(layout)
	c.refreshSVG()
}

// SetDataTable replaces the chart data-table configuration.
func (c *ChartDrawing) SetDataTable(table *chartmodel.DataTable) {
	if c == nil || c.chart == nil {
		return
	}
	c.chart.DataTable = cloneDataTable(table)
	c.refreshSVG()
}

// SetSeriesFormulas updates worksheet-backed formula bindings for a series.
func (c *ChartDrawing) SetSeriesFormulas(index int, nameFormula, valueFormula, xValueFormula, labelFormula string) error {
	series, err := c.seriesAt(index)
	if err != nil {
		return err
	}
	series.NameFormula = strings.TrimSpace(nameFormula)
	series.ValueFormula = strings.TrimSpace(valueFormula)
	series.XValueFormula = strings.TrimSpace(xValueFormula)
	series.LabelFormula = strings.TrimSpace(labelFormula)
	c.refreshSVG()
	return nil
}

// SetSeriesTrendline updates the trendline for a series.
func (c *ChartDrawing) SetSeriesTrendline(index int, trendline *chartmodel.Trendline) error {
	series, err := c.seriesAt(index)
	if err != nil {
		return err
	}
	series.Trendline = cloneTrendline(trendline)
	c.refreshSVG()
	return nil
}

// SetSeriesErrorBars updates error bars for a series.
func (c *ChartDrawing) SetSeriesErrorBars(index int, bars *chartmodel.ErrorBars) error {
	series, err := c.seriesAt(index)
	if err != nil {
		return err
	}
	series.ErrorBars = cloneErrorBars(bars)
	c.refreshSVG()
	return nil
}

// SetSeriesDataLabels updates data-label visibility and formatting for a series.
func (c *ChartDrawing) SetSeriesDataLabels(index int, showValue, showCatName, showSeriesName, showPercent, showLeaderLines, showBubbleSize bool, labelPosition int, separator, numFmt string) error {
	series, err := c.seriesAt(index)
	if err != nil {
		return err
	}
	series.ShowDataLabels = showValue
	series.ShowCatName = showCatName
	series.ShowSerName = showSeriesName
	series.ShowPercent = showPercent
	series.ShowLeaderLines = showLeaderLines
	series.ShowBubbleSize = showBubbleSize
	series.LabelPosition = labelPosition
	series.Separator = separator
	series.NumFmt = numFmt
	c.refreshSVG()
	return nil
}

// SetPosition updates the chart drawing position.
func (c *ChartDrawing) SetPosition(position sheetir.DrawingPosition) error {
	if c == nil || c.drawing == nil {
		return fmt.Errorf("sheet edit: nil chart drawing")
	}
	if err := validateDrawingPosition(position); err != nil {
		return err
	}
	c.drawing.Position = position
	return nil
}

func (c *ChartDrawing) refreshSVG() {
	if c.drawing != nil && c.drawing.Chart != nil && c.chart != nil {
		c.drawing.Chart.SVG = c.chart.SVG(600, 400)
	}
}

func chartFromDrawing(drawing *sheetir.Drawing) (*chartmodel.Chart, bool) {
	if drawing == nil || drawing.Chart == nil || drawing.Chart.Chart == nil {
		return nil, false
	}
	switch c := drawing.Chart.Chart.(type) {
	case *chartmodel.Chart:
		return c, true
	case chartmodel.Chart:
		chart := c
		drawing.Chart.Chart = &chart
		return &chart, true
	default:
		return nil, false
	}
}

func (c *ChartDrawing) seriesAt(index int) (*chartmodel.Series, error) {
	if c == nil || c.chart == nil {
		return nil, fmt.Errorf("sheet edit: nil chart drawing")
	}
	if index < 0 || index >= len(c.chart.Series) {
		return nil, fmt.Errorf("sheet edit: chart series index %d out of range", index)
	}
	return &c.chart.Series[index], nil
}

func cloneChartAxis(axis chartmodel.Axis) chartmodel.Axis {
	return axis
}

func cloneChartGroups(groups []chartmodel.ChartGroup) []chartmodel.ChartGroup {
	if len(groups) == 0 {
		return nil
	}
	out := make([]chartmodel.ChartGroup, len(groups))
	for i, group := range groups {
		out[i] = group
		out[i].SeriesIndices = append([]int(nil), group.SeriesIndices...)
		out[i].AxisIDs = append([]int(nil), group.AxisIDs...)
	}
	return out
}

func cloneLegendEntries(entries []chartmodel.LegendEntry) []chartmodel.LegendEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]chartmodel.LegendEntry, len(entries))
	copy(out, entries)
	return out
}

func cloneManualLayout(layout *chartmodel.ManualLayout) *chartmodel.ManualLayout {
	if layout == nil {
		return nil
	}
	clone := *layout
	return &clone
}

func cloneDataTable(table *chartmodel.DataTable) *chartmodel.DataTable {
	if table == nil {
		return nil
	}
	clone := *table
	return &clone
}

func cloneErrorBars(bars *chartmodel.ErrorBars) *chartmodel.ErrorBars {
	if bars == nil {
		return nil
	}
	clone := *bars
	clone.Plus = append([]float64(nil), bars.Plus...)
	clone.Minus = append([]float64(nil), bars.Minus...)
	return &clone
}

func cloneTrendline(trendline *chartmodel.Trendline) *chartmodel.Trendline {
	if trendline == nil {
		return nil
	}
	clone := *trendline
	if trendline.Intercept != nil {
		intercept := *trendline.Intercept
		clone.Intercept = &intercept
	}
	return &clone
}
