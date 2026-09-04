package edit

import ooxmlchart "office/pkg/ooxml/chart"

// Chart wraps a chart.Chart and provides convenience mutation methods.
type Chart struct {
	ir *ooxmlchart.Chart
}

// Chart returns the chart attached to this run, or nil if none.
func (r *Run) Chart() *Chart {
	if ir := r.ir(); ir != nil && ir.Chart != nil {
		return &Chart{ir: ir.Chart}
	}
	return nil
}

// SetChart attaches a chart to this run. Any previous chart, image, shape, or
// text box on the run is replaced.
func (r *Run) SetChart(c *ooxmlchart.Chart) {
	if ir := r.ir(); ir != nil {
		ir.Kind = "chart"
		ir.Text = ""
		ir.Image = nil
		ir.Shape = nil
		ir.TextBox = nil
		ir.Chart = c
	}
}

// AddChart creates a new empty chart, attaches it to the run, and returns a
// Chart editor. The chart kind can be set on the returned editor via SetKind.
func (r *Run) AddChart() *Chart {
	c := &ooxmlchart.Chart{}
	r.SetChart(c)
	return &Chart{ir: c}
}

// RemoveChart detaches any chart from this run.
func (r *Run) RemoveChart() {
	if ir := r.ir(); ir != nil {
		ir.Chart = nil
		if ir.Kind == "chart" {
			ir.Kind = "run"
		}
	}
}

// Charts returns all charts found in the document body (not headers/footers).
func (d *Document) Charts() []*Chart {
	if d == nil || d.IR == nil {
		return nil
	}
	var out []*Chart
	seen := make(map[*ooxmlchart.Chart]bool)
	for _, b := range d.IR.Blocks {
		if b.Paragraph == nil {
			continue
		}
		for i := range b.Paragraph.Runs {
			if c := b.Paragraph.Runs[i].Chart; c != nil && !seen[c] {
				seen[c] = true
				out = append(out, &Chart{ir: c})
			}
		}
	}
	return out
}

// Kind returns the chart type (e.g. "bar", "column", "line", "pie").
func (c *Chart) Kind() string {
	if c == nil || c.ir == nil {
		return ""
	}
	switch c.ir.ChartType {
	case ooxmlchart.ChartBar:
		return "bar"
	case ooxmlchart.ChartColumn:
		return "column"
	case ooxmlchart.ChartLine:
		return "line"
	case ooxmlchart.ChartPie:
		return "pie"
	case ooxmlchart.ChartArea:
		return "area"
	case ooxmlchart.ChartScatter:
		return "scatter"
	case ooxmlchart.ChartBubble:
		return "bubble"
	case ooxmlchart.ChartDoughnut:
		return "doughnut"
	case ooxmlchart.ChartRadar:
		return "radar"
	case ooxmlchart.ChartStock:
		return "stock"
	case ooxmlchart.ChartSurface:
		return "surface"
	default:
		return ""
	}
}

// SetKind sets the chart type.
func (c *Chart) SetKind(kind string) {
	if c == nil || c.ir == nil {
		return
	}
	switch kind {
	case "bar":
		c.ir.ChartType = ooxmlchart.ChartBar
	case "column":
		c.ir.ChartType = ooxmlchart.ChartColumn
	case "line":
		c.ir.ChartType = ooxmlchart.ChartLine
	case "pie":
		c.ir.ChartType = ooxmlchart.ChartPie
	case "area":
		c.ir.ChartType = ooxmlchart.ChartArea
	case "scatter":
		c.ir.ChartType = ooxmlchart.ChartScatter
	case "bubble":
		c.ir.ChartType = ooxmlchart.ChartBubble
	case "doughnut":
		c.ir.ChartType = ooxmlchart.ChartDoughnut
	case "radar":
		c.ir.ChartType = ooxmlchart.ChartRadar
	case "stock":
		c.ir.ChartType = ooxmlchart.ChartStock
	case "surface":
		c.ir.ChartType = ooxmlchart.ChartSurface
	default:
		c.ir.ChartType = ooxmlchart.ChartUnknown
	}
}

// Title returns the chart title.
func (c *Chart) Title() string {
	if c == nil || c.ir == nil {
		return ""
	}
	return c.ir.Title
}

// SetTitle sets the chart title.
func (c *Chart) SetTitle(v string) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.Title = v
}

// Categories returns the category labels.
func (c *Chart) Categories() []string {
	if c == nil || c.ir == nil {
		return nil
	}
	return append([]string(nil), c.ir.Categories...)
}

// SetCategories replaces the category labels.
func (c *Chart) SetCategories(v []string) {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.Categories = append([]string(nil), v...)
}

// SeriesCount returns the number of data series.
func (c *Chart) SeriesCount() int {
	if c == nil || c.ir == nil {
		return 0
	}
	return len(c.ir.Series)
}

// Series returns the series at the given index, or nil if out of range.
func (c *Chart) Series(index int) *ChartSeries {
	if c == nil || c.ir == nil || index < 0 || index >= len(c.ir.Series) {
		return nil
	}
	return &ChartSeries{chart: c.ir, index: index}
}

// AddSeries appends a new data series and returns it.
func (c *Chart) AddSeries(name string, values []float64) *ChartSeries {
	if c == nil || c.ir == nil {
		return nil
	}
	s := ooxmlchart.Series{
		Index:  len(c.ir.Series),
		Name:   name,
		Values: append([]float64(nil), values...),
	}
	c.ir.Series = append(c.ir.Series, s)
	return &ChartSeries{chart: c.ir, index: len(c.ir.Series) - 1}
}

// RemoveSeries deletes the series at the given index.
func (c *Chart) RemoveSeries(index int) {
	if c == nil || c.ir == nil || index < 0 || index >= len(c.ir.Series) {
		return
	}
	c.ir.Series = append(c.ir.Series[:index], c.ir.Series[index+1:]...)
	for i := range c.ir.Series {
		c.ir.Series[i].Index = i
	}
}

// ClearSeries removes all data series.
func (c *Chart) ClearSeries() {
	if c == nil || c.ir == nil {
		return
	}
	c.ir.Series = nil
}

// ChartSeries edits a single data series within a chart.
type ChartSeries struct {
	chart *ooxmlchart.Chart
	index int
}

func (cs *ChartSeries) s() *ooxmlchart.Series {
	if cs == nil || cs.chart == nil || cs.index < 0 || cs.index >= len(cs.chart.Series) {
		return nil
	}
	return &cs.chart.Series[cs.index]
}

// Name returns the series name.
func (cs *ChartSeries) Name() string {
	if s := cs.s(); s != nil {
		return s.Name
	}
	return ""
}

// SetName sets the series name.
func (cs *ChartSeries) SetName(v string) {
	if s := cs.s(); s != nil {
		s.Name = v
	}
}

// Values returns a copy of the series values.
func (cs *ChartSeries) Values() []float64 {
	if s := cs.s(); s != nil {
		return append([]float64(nil), s.Values...)
	}
	return nil
}

// SetValues replaces the series values.
func (cs *ChartSeries) SetValues(v []float64) {
	if s := cs.s(); s != nil {
		s.Values = append([]float64(nil), v...)
	}
}

// AppendValue appends a value to the series.
func (cs *ChartSeries) AppendValue(v float64) {
	if s := cs.s(); s != nil {
		s.Values = append(s.Values, v)
	}
}