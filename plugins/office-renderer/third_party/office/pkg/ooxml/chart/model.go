// Package chart provides chart parsing and rendering for OOXML charts.
// This package handles c:chart elements from chart parts (chart1.xml, etc.).
package chart

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
)

// Chart represents a complete chart with all its data and configuration.
type Chart struct {
	// Title is the chart title, if any.
	Title string

	// ChartType identifies the kind of chart (bar, column, line, pie, etc.).
	ChartType ChartType

	// Series contains the data series displayed in the chart.
	Series []Series

	// Categories provides labels for the X-axis (category axis).
	// For pie charts, these are the slice labels.
	Categories []string

	// XAxis configuration for charts with X axes.
	XAxis Axis

	// YAxis configuration for charts with Y axes.
	YAxis Axis

	// YAxis2 is the secondary Y-axis for combo charts.
	YAxis2 Axis

	// Legend configuration.
	Legend Legend

	// Stacked indicates whether series values are stacked.
	Stacked bool

	// PercentStacked indicates whether series values are percent-stacked.
	PercentStacked bool

	// PlotArea contains the visual plot configuration.
	PlotArea PlotArea

	// ChartArea contains the chart area background configuration.
	ChartArea ChartAreaStyle

	// ThemeColors maps theme color names (e.g. "accent1") to hex RGB values.
	// Set by the caller after parsing if theme colors are available.
	ThemeColors map[string]string

	// GapWidth is the space between bar clusters as a percentage of bar width.
	// Used by bar and column charts. 0-500; default 150.
	GapWidth int

	// Overlap is the overlap between bars in the same cluster as a percentage.
	// Used by bar and column charts. -100 to 100; default 0.
	Overlap int

	// VaryColors indicates whether colors vary by point rather than by series.
	// Used by bar, column, pie, and doughnut charts.
	VaryColors bool

	// HoleSize is the doughnut hole size as a percentage of the chart radius.
	// 0-90; default 50.
	HoleSize int

	// View3D contains 3D view parameters when the chart is a 3D type.
	View3D *View3D

	// RadarStyle controls radar chart rendering: "standard", "marker", "filled".
	RadarStyle string

	// ManualLayout contains manual plot-area layout overrides.
	ManualLayout *ManualLayout

	// LegendLayout contains manual legend layout overrides.
	LegendLayout *ManualLayout

	// TitleLayout contains manual chart-title layout overrides.
	TitleLayout *ManualLayout

	// DisplayBlanksAs controls how blank cells are displayed: "gap", "span", or "zero".
	DisplayBlanksAs string

	// DataTable shows a grid of values below the plot area.
	DataTable *DataTable

	// StockConfig holds stock-chart-specific settings.
	StockConfig *StockConfig

	// Groups holds chart-type groups for combo (mixed) charts.
	// Each group references a subset of Series by index and carries its own
	// chart-type-specific options. When empty, the chart is treated as a
	// single-group chart using ChartType.
	Groups []ChartGroup
}

// ChartGroup represents a single chart-type group inside c:plotArea.
// Combo charts contain multiple groups (e.g. bar + line).
type ChartGroup struct {
	ChartType     ChartType
	SeriesIndices []int  // indices into Chart.Series
	AxisIDs       []int  // axis bindings for this group
	Grouping      string // "clustered", "stacked", "percentStacked", "standard"
	GapWidth      int
	Overlap       int
	VaryColors    bool
	HoleSize      int    // doughnut only
	BarDir        string // "bar" or "col"
}

// ChartType identifies the chart visualization type.
type ChartType int

const (
	ChartUnknown   ChartType = iota
	ChartBar                 // Horizontal bars
	ChartColumn              // Vertical bars
	ChartLine                // Line with markers
	ChartPie                 // Pie chart
	ChartArea                // Area chart
	ChartScatter             // X/Y scatter
	ChartDoughnut            // Doughnut variant of pie
	ChartBubble              // Bubble chart
	ChartStock               // Stock / OHLC chart
	ChartRadar               // Radar / spider chart
	ChartSurface             // Surface chart
	ChartBar3D               // 3D horizontal bars (rendered as 2D fallback)
	ChartColumn3D            // 3D vertical bars (rendered as 2D fallback)
	ChartLine3D              // 3D line (rendered as 2D fallback)
	ChartPie3D               // 3D pie (rendered as 2D fallback)
	ChartArea3D              // 3D area (rendered as 2D fallback)
	ChartSurface3D           // 3D surface (rendered as 2D fallback)
)

// Series represents a single data series in a chart.
type Series struct {
	// Index is the series order.
	Index int

	// ChartType overrides the chart-level type for combo charts.
	ChartType ChartType

	// Name is the series name (appears in legend).
	Name string

	// NameFormula is the optional worksheet formula backing the series name.
	NameFormula string

	// Values are the numeric data points.
	Values []float64

	// ValueFormula is the optional worksheet range formula backing the series values.
	ValueFormula string

	// XValues are the numeric X-axis data points for scatter charts.
	XValues []float64

	// XValueFormula is the optional worksheet range formula backing the X values.
	XValueFormula string

	// BubbleSizes are the bubble size values for bubble charts.
	BubbleSizes []float64

	// Color is the series color (hex, optional).
	Color string

	// LineColor is the series line color (hex, optional).
	LineColor string

	// LineWidth is the line width in EMUs (12700 EMUs = 1pt).
	LineWidth int

	// LinePattern is the BIFF/OOXML line pattern identifier when available.
	LinePattern int

	// FillColor is the series fill/area color (hex, optional).
	FillColor string

	// MarkerColor is the point marker color for line/scatter series.
	MarkerColor string

	// MarkerSize is the point marker size (default ~5 when unset).
	MarkerSize int

	// MarkerShape is the point marker shape, e.g. "circle" or "diamond".
	MarkerShape string

	// FillAlpha is the fill opacity in the range [0,1] (optional).
	FillAlpha float64

	fillRef *drawing.SolidFill
	lineRef *drawing.SolidFill

	// Labels are per-value labels (optional).
	Labels []string

	// LabelFormula is the optional worksheet range formula backing the series labels.
	LabelFormula string

	// ShowDataLabels enables value labels on each data point.
	ShowDataLabels bool

	// ShowCatName enables category name in data labels.
	ShowCatName bool

	// ShowSerName enables series name in data labels.
	ShowSerName bool

	// ShowPercent enables percentage in data labels (for pie/doughnut/stacked).
	ShowPercent bool

	// ShowLeaderLines enables leader lines for data labels.
	ShowLeaderLines bool

	// ShowBubbleSize enables bubble size in data labels (bubble charts).
	ShowBubbleSize bool

	// Separator is the string used to join multiple label parts (e.g. ", ").
	Separator string

	// LabelPosition controls data-label placement: 0=default, 1=outEnd, 2=inEnd, 3=ctr, 4=inBase.
	LabelPosition int

	// NumFmt is the number format code for data labels (e.g. "0.00", "0%").
	NumFmt string

	// Smooth enables smooth (Bezier) lines for line/area/scatter series.
	Smooth bool

	// Explosion offsets a pie slice from the center (in percent).
	Explosion int

	// ErrorBars configures per-series error bars.
	ErrorBars *ErrorBars

	// Trendline configures a trendline for this series.
	Trendline *Trendline

	// DataPoints holds per-point formatting overrides.
	DataPoints []DataPoint

	// secondaryValueAxis reports whether this series is bound to the secondary
	// Y axis in a combo chart.
	secondaryValueAxis bool

	// ScatterStyle preserves the chart group's scatterStyle for SVG rendering.
	ScatterStyle string

	// LabelStyle holds data-label text properties (font size, bold, italic, color).
	LabelStyle *TextProperties
}

// Axis represents a chart axis configuration.
type Axis struct {
	// Title is the axis title.
	Title string

	// Min and Max define the axis range (0 means auto).
	Min float64
	Max float64

	// MajorUnit is the spacing between major tick marks.
	MajorUnit float64

	// MinorUnit is the spacing between minor tick marks.
	MinorUnit float64

	// ShowGridlines enables grid line display.
	ShowGridlines bool

	// GridlineColor is the hex color for major grid lines (optional).
	GridlineColor string

	gridlineRef *drawing.SolidFill

	// MajorTickType indicates major tick mark style (0=none, 1=outside, 2=inside, 3=cross).
	MajorTickType int

	// MinorTickType indicates minor tick mark style (0=none, 1=outside, 2=inside, 3=cross).
	MinorTickType int

	// LabelPosition indicates axis label placement (0=none, 1=low, 2=high, 3=next to axis).
	LabelPosition int

	// Orientation indicates axis direction: "minMax" or "maxMin".
	Orientation string

	// CrossingPoint is the axis crossing point (category index for category axes).
	CrossingPoint int

	// CrossingAxisID is the axis ID that this axis crosses.
	CrossingAxisID int

	// NumFmt is the number format code for axis labels.
	NumFmt string

	// LabelFrequency is the interval between visible axis labels.
	LabelFrequency int

	// TickMarkFrequency is the interval between tick marks.
	TickMarkFrequency int

	// Reversed indicates whether categories are displayed in reverse order.
	Reversed bool
}

// Legend represents chart legend configuration.
type Legend struct {
	// Position: "bottom", "top", "left", "right", "none".
	Position string

	// Show determines if legend is displayed.
	Show bool

	// Entries holds per-entry visibility overrides.
	Entries []LegendEntry
}

// LegendEntry represents a single legend entry override.
type LegendEntry struct {
	Index  int
	Delete bool
}

// PlotArea represents the visual plot configuration.
type PlotArea struct {
	// Width and Height in EMUs.
	Width, Height int64

	// X and Y position in EMUs (relative to chart area).
	X, Y int64

	// ShowBorder draws a border around the plot area.
	ShowBorder bool

	// BorderColor is the plot area border color (hex, optional).
	BorderColor string

	// FillColor is the plot area fill color (hex, optional).
	FillColor string
}

// ChartAreaStyle represents the chart area background configuration.
type ChartAreaStyle struct {
	// FillColor is the chart area background color (hex, optional).
	FillColor string

	fillRef *drawing.SolidFill
}

// View3D represents 3D view parameters for a chart.
type View3D struct {
	RotX           int  // Rotation around X axis (-90 to 90, default 0)
	RotY           int  // Rotation around Y axis (default 0)
	Perspective    int  // Perspective angle (0-240, default 30)
	DepthPercent   int  // Depth as percentage (default 100)
	RightAngleAxes bool // true = right-angle axes (no perspective)
}

// ErrorBars represents per-series error bars.
type ErrorBars struct {
	Direction string // "x" or "y" (default "y")
	BarType   string // "both", "minus", "plus"
	ValType   string // "cust", "fixedVal", "percentage", "stdDev", "stdErr"
	NoEndCap  bool
	Value     float64   // scalar amount for fixedVal/percentage/stdDev/stdErr
	Plus      []float64 // custom positive errors
	Minus     []float64 // custom negative errors
}

// Trendline represents a series trendline.
type Trendline struct {
	Type         string   // "exp", "linear", "log", "movingAvg", "poly", "power"
	Order        int      // polynomial order
	Period       int      // moving average period
	Forward      float64  // forward forecast
	Backward     float64  // backward forecast
	Intercept    *float64 // optional intercept
	ShowRSquared bool
	ShowEquation bool
	Color        string // hex color
	LineWidth    int    // in EMUs

	colorRef *drawing.SolidFill
}

// DataTable represents a data table shown below the plot area.
type DataTable struct {
	ShowHorzBorder bool
	ShowVertBorder bool
	ShowOutline    bool
	ShowKeys       bool
}

// ManualLayout represents manual layout positioning for chart elements.
type ManualLayout struct {
	LayoutTarget string  // "inner" or "outer"
	XMode        string  // "edge" or "factor"
	YMode        string  // "edge" or "factor"
	WMode        string  // "edge" or "factor"
	HMode        string  // "edge" or "factor"
	X            float64 // Position (0-1)
	Y            float64 // Position (0-1)
	W            float64 // Width (0-1)
	H            float64 // Height (0-1)
}

// ChartPart represents the raw chart XML structure for parsing.
// This mirrors the c:chartSpace OOXML structure.
type ChartPart struct {
	// ChartSpace background spPr (direct child of chartSpace)
	SpPr *SpPrElement `xml:"spPr"`
	// ChartSpace properties (the c:chart element)
	ChartSpace ChartSpace `xml:"chart"`
}

// ChartSpace is the root chart element.
type ChartSpace struct {
	// Title element
	Title *TextElement `xml:"title"`

	// Plot area contains the chart type and data
	PlotArea PlotAreaElement `xml:"plotArea"`

	// Legend element
	Legend *LegendElement `xml:"legend"`

	// ChartArea background
	ChartArea *SpPrElement `xml:"spPr"`

	// Axis elements
	Axes []AxisElement `xml:">axId"` // References to axes

	// View3D contains 3D view settings.
	View3D *View3DElement `xml:"view3D"`

	// DisplayBlanksAs controls how blank cells are displayed.
	DisplayBlanksAs *StrValElement `xml:"displayBlanksAs"`
}

// TextElement represents text content in chart.
type TextElement struct {
	Text         string         `xml:"rich>p>r>t"`
	NestedText   string         `xml:"tx>rich>p>r>t"`
	StrRef       *StrRefData    `xml:"strRef"`
	NestedStrRef *StrRefData    `xml:"tx>strRef"`
	V            string         `xml:"v"`
	Layout       *LayoutElement `xml:"layout"`
}

// PlainText returns rich-text content when present.
func (t TextElement) PlainText() string {
	if t.Text != "" {
		return t.Text
	}
	if t.NestedText != "" {
		return t.NestedText
	}
	return t.V
}

// FormulaRef returns the worksheet formula referenced by the text element.
func (t TextElement) FormulaRef() string {
	if t.StrRef != nil && t.StrRef.Formula != "" {
		return t.StrRef.Formula
	}
	if t.NestedStrRef != nil {
		return t.NestedStrRef.Formula
	}
	return ""
}

// CachedText returns the first cached text point when available.
func (t TextElement) CachedText() string {
	if t.StrRef != nil && len(t.StrRef.StrCache.Points) > 0 {
		return t.StrRef.StrCache.Points[0].Value
	}
	if t.NestedStrRef != nil && len(t.NestedStrRef.StrCache.Points) > 0 {
		return t.NestedStrRef.StrCache.Points[0].Value
	}
	return ""
}

// PlotAreaElement contains the chart visualization.
type PlotAreaElement struct {
	// Chart type specific elements.
	// BarChart covers both horizontal bar (barDir="bar") and column (barDir="col") charts.
	BarChart      *BarChartElement     `xml:"barChart"`
	LineChart     *LineChartElement    `xml:"lineChart"`
	PieChart      *PieChartElement     `xml:"pieChart"`
	AreaChart     *AreaChartElement    `xml:"areaChart"`
	ScatterChart  *ScatterChartElement `xml:"scatterChart"`
	BubbleChart   *BubbleChartElement  `xml:"bubbleChart"`
	StockChart    *StockChartElement   `xml:"stockChart"`
	RadarChart    *RadarChartElement   `xml:"radarChart"`
	SurfaceChart  *SurfaceChartElement `xml:"surfaceChart"`
	DoughnutChart *PieChartElement     `xml:"doughnutChart"`

	// 3D chart variants (parsed for compatibility; rendered as 2D fallback)
	Bar3DChart     *BarChartElement     `xml:"bar3DChart"`
	Line3DChart    *LineChartElement    `xml:"line3DChart"`
	Pie3DChart     *PieChartElement     `xml:"pie3DChart"`
	Area3DChart    *AreaChartElement    `xml:"area3DChart"`
	Surface3DChart *SurfaceChartElement `xml:"surface3DChart"`

	// Axes
	CatAxes  []AxisElement `xml:"catAx"`
	ValAxes  []AxisElement `xml:"valAx"`
	DateAxes []AxisElement `xml:"dateAx"`

	// Axes references
	AxisIDs []AxisID `xml:"axId"`

	// Data table
	DTable *DTableElement `xml:"dTable,omitempty"`

	// SpPr contains plot-area fill and line styling.
	SpPr *SpPrElement `xml:"spPr"`

	// Layout contains manual plot-area layout.
	Layout *LayoutElement `xml:"layout"`
}

// AxisID references an axis by ID.
type AxisID struct {
	Val int `xml:"val,attr"`
}

// BarChartElement represents bar/column chart data.
// When BarDir is "bar" it is a horizontal bar chart; when "col" it is a column chart.
type BarChartElement struct {
	BarDir     *StrValElement  `xml:"barDir"`     // "bar" or "col"
	Grouping   *StrValElement  `xml:"grouping"`   // "clustered", "stacked", "percentStacked"
	GapWidth   *IntValElement  `xml:"gapWidth"`   // 0-500 percentage
	Overlap    *IntValElement  `xml:"overlap"`    // -100 to 100 percentage
	VaryColors *IntValElement  `xml:"varyColors"` // 0 or 1
	Series     []SeriesElement `xml:"ser"`
	AxisID     []AxisID        `xml:"axId"`
}

// LineChartElement represents line chart data.
type LineChartElement struct {
	Grouping *StrValElement  `xml:"grouping"`
	Series   []SeriesElement `xml:"ser"`
	AxisID   []AxisID        `xml:"axId"`
}

// PieChartElement represents pie chart data.
type PieChartElement struct {
	VaryColors *IntValElement  `xml:"varyColors"` // 0 or 1
	HoleSize   *IntValElement  `xml:"holeSize"`   // 0-90 percentage (doughnut only)
	Series     []SeriesElement `xml:"ser"`
}

// AreaChartElement represents area chart data.
type AreaChartElement struct {
	Grouping *StrValElement  `xml:"grouping"`
	Series   []SeriesElement `xml:"ser"`
	AxisID   []AxisID        `xml:"axId"`
}

// ScatterChartElement represents scatter chart data.
type ScatterChartElement struct {
	ScatterStyle *StrValElement  `xml:"scatterStyle"`
	Series       []SeriesElement `xml:"ser"`
	AxisID       []AxisID        `xml:"axId"`
}

// BubbleChartElement represents bubble chart data.
type BubbleChartElement struct {
	Series []SeriesElement `xml:"ser"`
	AxisID []AxisID        `xml:"axId"`
}

// StockChartElement represents stock / OHLC chart data.
type StockChartElement struct {
	Series     []SeriesElement    `xml:"ser"`
	AxisID     []AxisID           `xml:"axId"`
	DropLines  *LinesElement      `xml:"dropLines"`
	HiLowLines *LinesElement      `xml:"hiLowLines"`
	UpDownBars *UpDownBarsElement `xml:"upDownBars"`
}

// LinesElement represents line-formatting elements like dropLines/hiLowLines.
type LinesElement struct {
	SpPr *SpPrElement `xml:"spPr"`
}

// UpDownBarsElement represents up/down bar formatting for stock charts.
type UpDownBarsElement struct {
	UpBars   *SpPrElement `xml:"upBars"`
	DownBars *SpPrElement `xml:"downBars"`
}

// StockConfig holds stock-chart-specific configuration.
type StockConfig struct {
	UpBarColor     string
	DownBarColor   string
	ShowDropLines  bool
	ShowHiLowLines bool
}

// RadarChartElement represents radar / spider chart data.
type RadarChartElement struct {
	RadarStyle *StrValElement  `xml:"radarStyle"`
	Series     []SeriesElement `xml:"ser"`
	AxisID     []AxisID        `xml:"axId"`
}

// SurfaceChartElement represents surface chart data.
type SurfaceChartElement struct {
	Series []SeriesElement `xml:"ser"`
	AxisID []AxisID        `xml:"axId"`
}

// TrendlineElement represents c:trendline in chart XML.
type TrendlineElement struct {
	SpPr          *SpPrElement     `xml:"spPr"`
	TrendlineType *StrValElement   `xml:"trendlineType,omitempty"`
	Order         *IntValElement   `xml:"order,omitempty"`
	Period        *IntValElement   `xml:"period,omitempty"`
	Forward       *FloatValElement `xml:"forward,omitempty"`
	Backward      *FloatValElement `xml:"backward,omitempty"`
	Intercept     *FloatValElement `xml:"intercept,omitempty"`
	DispRSqr      *BoolValElement  `xml:"dispRSqr,omitempty"`
	DispEq        *BoolValElement  `xml:"dispEq,omitempty"`
}

// FloatValElement represents a floating-point value element.
type FloatValElement struct {
	Val float64 `xml:"val,attr"`
}

// SeriesElement represents a series in chart XML.
type SeriesElement struct {
	Index      int                `xml:"idx,attr"`
	Order      int                `xml:"order,attr"`
	Name       TextElement        `xml:"tx"`
	Values     NumberRef          `xml:"val"`
	XVal       *NumberRef         `xml:"xVal,omitempty"`
	BubbleSize *NumberRef         `xml:"bubbleSize,omitempty"`
	Cats       *CategoryRef       `xml:"cat,omitempty"`
	Explosion  *IntValElement     `xml:"explosion"` // For pie slices
	SpPr       *SpPrElement       `xml:"spPr"`
	Marker     *MarkerElement     `xml:"marker"`
	DLbls      *DLblsElement      `xml:"dLbls,omitempty"`
	Smooth     *BoolValElement    `xml:"smooth,omitempty"`
	ErrBars    *ErrBarsElement    `xml:"errBars,omitempty"`
	Trendline  *TrendlineElement  `xml:"trendline,omitempty"`
	DataPoints []DataPointElement `xml:"dPt"`
}

// MarkerElement represents marker properties for line/scatter series.
type MarkerElement struct {
	Symbol *StrValElement `xml:"symbol,omitempty"`
	Size   *IntValElement `xml:"size,omitempty"`
	SpPr   *SpPrElement   `xml:"spPr,omitempty"`
}

// DataPoint represents per-point formatting for a chart data point.
type DataPoint struct {
	Index     int
	FillColor string
	LineColor string
	Explosion int
	Label     string // custom text override from c:dLbl/c:tx
	NumFmt    string // per-point number format override

	// LabelStyle holds per-point data-label text properties.
	LabelStyle *TextProperties
}

// DataPointElement represents c:dPt in chart XML.
type DataPointElement struct {
	Index     *IntValElement `xml:"idx"`
	SpPr      *SpPrElement   `xml:"spPr"`
	Explosion *IntValElement `xml:"explosion"`
}

// StrValElement represents a string value element.
type StrValElement struct {
	Val string `xml:"val,attr"`
}

// IntValElement represents an integer value element.
type IntValElement struct {
	Val int `xml:"val,attr"`
}

// TextProperties holds basic text formatting for chart labels.
type TextProperties struct {
	FontSize int // in hundredths of a point (e.g. 1000 = 10pt)
	Bold     bool
	Italic   bool
	Color    string // hex RGB, e.g. "FF0000"
}

// DLblsElement represents data labels configuration for a series.
type DLblsElement struct {
	ShowVal         *BoolValElement `xml:"showVal,omitempty"`
	ShowCatName     *BoolValElement `xml:"showCatName,omitempty"`
	ShowSerName     *BoolValElement `xml:"showSerName,omitempty"`
	ShowPercent     *BoolValElement `xml:"showPercent,omitempty"`
	ShowBubbleSize  *BoolValElement `xml:"showBubbleSize,omitempty"`
	ShowLeaderLines *BoolValElement `xml:"showLeaderLines,omitempty"`
	Separator       *StrValElement  `xml:"separator,omitempty"`
	LblPos          *StrValElement  `xml:"dLblPos,omitempty"`
	NumFmt          *NumFmtElement  `xml:"numFmt,omitempty"`
	DLbl            []DLblElement   `xml:"dLbl,omitempty"`
	TxPr            *TxPrElement    `xml:"txPr,omitempty"`
}

// DLblElement represents a per-point data label override (c:dLbl).
type DLblElement struct {
	Idx         *IntValElement  `xml:"idx"`
	Tx          *TextElement    `xml:"tx"`
	NumFmt      *NumFmtElement  `xml:"numFmt,omitempty"`
	SpPr        *SpPrElement    `xml:"spPr"`
	DLblPos     *StrValElement  `xml:"dLblPos"`
	ShowVal     *BoolValElement `xml:"showVal"`
	ShowCatName *BoolValElement `xml:"showCatName"`
	ShowSerName *BoolValElement `xml:"showSerName"`
	ShowPercent *BoolValElement `xml:"showPercent"`
	TxPr        *TxPrElement    `xml:"txPr,omitempty"`
}

// BoolValElement represents a boolean value element.
type BoolValElement struct {
	Val bool `xml:"val,attr"`
}

// NumberRef references numeric values via a numRef wrapper.
type NumberRef struct {
	NumRef *NumRefData `xml:"numRef"`
}

// NumRefData holds the formula and cached numeric values.
type NumRefData struct {
	Formula  string   `xml:"f"`
	NumCache NumCache `xml:"numCache"`
}

// CategoryRef references category labels via a strRef wrapper.
type CategoryRef struct {
	StrRef *StrRefData `xml:"strRef"`
}

// StrRefData holds the formula and cached string values.
type StrRefData struct {
	Formula  string   `xml:"f"`
	StrCache StrCache `xml:"strCache"`
}

// NumCache holds cached numeric values.
type NumCache struct {
	FormatCode string     `xml:"formatCode,attr"`
	Points     []NumPoint `xml:"pt"`
}

// StrCache holds cached string values.
type StrCache struct {
	Points []StrPoint `xml:"pt"`
}

// NumPoint is a numeric data point.
type NumPoint struct {
	Idx   int     `xml:"idx,attr"`
	Value float64 `xml:"v"`
}

// StrPoint is a string data point.
type StrPoint struct {
	Idx   int    `xml:"idx,attr"`
	Value string `xml:"v"`
}

// LegendElement represents legend configuration.
type LegendElement struct {
	LegendPos     *LegendPos           `xml:"legendPos"`
	Layout        *LayoutElement       `xml:"layout"`
	LegendEntries []LegendEntryElement `xml:"legendEntry"`
}

// LegendEntryElement represents c:legendEntry in chart XML.
type LegendEntryElement struct {
	Index  *IntValElement  `xml:"idx"`
	Delete *BoolValElement `xml:"delete"`
}

// LegendPos holds the legend position value.
type LegendPos struct {
	Val string `xml:"val,attr"`
}

// View3DElement represents the c:view3D XML element.
type View3DElement struct {
	RotX        *IntValElement  `xml:"rotX"`
	HPercent    *IntValElement  `xml:"hPercent"`
	RotY        *IntValElement  `xml:"rotY"`
	DepthPct    *IntValElement  `xml:"depthPercent"`
	RightAngle  *BoolValElement `xml:"rAngAx"`
	Perspective *IntValElement  `xml:"perspective"`
}

// ErrBarsElement represents the c:errBars XML element.
type ErrBarsElement struct {
	ErrDir     *StrValElement  `xml:"errDir"`
	ErrBarType *StrValElement  `xml:"errBarType"`
	ErrValType *StrValElement  `xml:"errValType"`
	NoEndCap   *BoolValElement `xml:"noEndCap"`
	Plus       *NumberRef      `xml:"plus,omitempty"`
	Minus      *NumberRef      `xml:"minus,omitempty"`
	Val        *NumericValue   `xml:"val"`
}

// DTableElement represents the c:dTable XML element.
type DTableElement struct {
	ShowHorzBorder *BoolValElement `xml:"showHorzBorder"`
	ShowVertBorder *BoolValElement `xml:"showVertBorder"`
	ShowOutline    *BoolValElement `xml:"showOutline"`
	ShowKeys       *BoolValElement `xml:"showKeys"`
}

// ManualLayoutElement represents the c:manualLayout XML element.
type ManualLayoutElement struct {
	LayoutTarget *StrValElement `xml:"layoutTarget"`
	XMode        *StrValElement `xml:"xMode"`
	YMode        *StrValElement `xml:"yMode"`
	WMode        *StrValElement `xml:"wMode"`
	HMode        *StrValElement `xml:"hMode"`
	X            *NumericValue  `xml:"x"`
	Y            *NumericValue  `xml:"y"`
	W            *NumericValue  `xml:"w"`
	H            *NumericValue  `xml:"h"`
}

// LayoutElement represents the c:layout XML element.
type LayoutElement struct {
	ManualLayout *ManualLayoutElement `xml:"manualLayout"`
}

// AxisElement represents an axis definition.
type AxisElement struct {
	AxisID         int             `xml:"axId,attr"`
	AxisRef        *AxisID         `xml:"axId"`
	Type           string          // Inferred from element name: catAx, valAx, dateAx
	Title          *TextElement    `xml:"title"`
	Min            *NumericValue   `xml:"scaling>min"`
	Max            *NumericValue   `xml:"scaling>max"`
	Orientation    *StrValElement  `xml:"scaling>orientation"`
	MajorGridlines *MajorGridlines `xml:"majorGridlines"`
	MajorTickMark  *StrValElement  `xml:"majorTickMark"`
	MinorTickMark  *StrValElement  `xml:"minorTickMark"`
	TickLblPos     *StrValElement  `xml:"tickLblPos"`
	Crosses        *StrValElement  `xml:"crosses"`
	CrossesAt      *NumericValue   `xml:"crossesAt"`
	MajorUnit      *NumericValue   `xml:"majorUnit"`
	MinorUnit      *NumericValue   `xml:"minorUnit"`
	NumFmt         *NumFmtElement  `xml:"numFmt"`
}

// MajorGridlines holds gridline formatting.
type MajorGridlines struct {
	SpPr *SpPrElement `xml:"spPr"`
}

// NumFmtElement represents the c:numFmt XML element.
type NumFmtElement struct {
	FormatCode   string `xml:"formatCode,attr"`
	SourceLinked bool   `xml:"sourceLinked,attr"`
}

// SpPrElement holds shape properties (fill, line, etc.).
type SpPrElement struct {
	Ln        *LineProperties `xml:"ln"`
	SolidFill *SolidFill      `xml:"solidFill"`
}

// LineProperties holds line formatting.
type LineProperties struct {
	Width     int            `xml:"w,attr"`
	Cmpd      string         `xml:"cmpd,attr"`
	SolidFill *SolidFill     `xml:"solidFill"`
	PrstDash  *StrValElement `xml:"prstDash,omitempty"`
}

// SolidFill reuses the shared DrawingML solid fill model so theme transforms
// such as lumMod/lumOff survive chart parsing.
type SolidFill = drawingxml.SolidFill

// SRGBClr reuses the shared DrawingML sRGB colour model.
type SRGBClr = drawingxml.SRGBClr

// SchemeClr reuses the shared DrawingML scheme colour model.
type SchemeClr = drawingxml.SchemeClr

// Alpha reuses the shared DrawingML colour transform model.
type Alpha = drawingxml.ColorTransform

// TxPrElement represents c:txPr — text properties for chart labels.
type TxPrElement struct {
	DefRPr *DefRPrElement `xml:"defRPr,omitempty"`
}

// DefRPrElement represents a:defRPr — default run properties.
type DefRPrElement struct {
	Sz        int        `xml:"sz,attr"`
	B         bool       `xml:"b,attr"`
	I         bool       `xml:"i,attr"`
	SolidFill *SolidFill `xml:"solidFill,omitempty"`
}

// NumericValue holds a numeric value.
type NumericValue struct {
	Val float64 `xml:"val,attr"`
}
