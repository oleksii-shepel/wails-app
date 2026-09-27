// Package msgraph parses embedded Microsoft Graph OLE chart workbooks into the
// shared canonical chart model used by both legacy Office and OOXML renderers.
package msgraph

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"office/pkg/ooxml/cfb"
	chartpkg "office/pkg/ooxml/chart"
	sheetir "office/pkg/sheet/ir"
	legacysheet "office/pkg/sheet/parsers/legacy"
)

// Chart is one semantic chart recovered from an MSGraph workbook.
type Chart struct {
	SheetName string
	Model     *chartpkg.Chart
}

// Document is the parsed semantic content of an embedded MSGraph OLE object.
type Document struct {
	Workbook *sheetir.Workbook
	Charts   []Chart
}

// IsProgID reports whether progID denotes a Microsoft Graph chart object.
func IsProgID(progID string) bool {
	p := strings.ToLower(strings.TrimSpace(progID))
	switch p {
	case "msgraph.chart", "msgraph.chart.8", "graph.chart", "graph.chart.8":
		return true
	default:
		return strings.HasPrefix(p, "msgraph.chart.") || strings.HasPrefix(p, "graph.chart.")
	}
}

// graphDatasheet is the compact cell table stored by Microsoft Graph in the
// embedded Workbook stream. Graph 8 commonly uses BIFF2 cell records even
// though the surrounding workbook advertises Unicode codepage 1200. Its LABEL
// records therefore carry an extra one-byte compressed/uncompressed string flag
// after cch. A generic XLS reader cannot safely infer this MSGraph-specific
// convention, so decode it while the object identity is still known here.
type graphDatasheet struct {
	cells  map[int]map[int]graphCell // zero-based row/column
	minRow int
	maxRow int
	minCol int
	maxCol int
}

type graphCell struct {
	text       string
	number     float64
	isText     bool
	isNum      bool
	numFmtID   uint16
	numFmtCode string
}

// graphTextSemantics captures authored Microsoft Graph chart text formatting
// that is stored in BIFF FONT/FONTINDEX/TEXT/OBJECTLINK records.  This layer
// belongs to MSGraph rather than the presentation parser because the same
// records are used by embedded Graph objects in Word, Excel, and PowerPoint.
type graphTextSemantics struct {
	titleStyle      *chartpkg.TextProperties
	xAxisTitleStyle *chartpkg.TextProperties
	yAxisTitleStyle *chartpkg.TextProperties
}

// graphLegendSemantics captures the authored MSGraph legend contract from the
// BIFF LEGEND container.  Keep this in the MSGraph layer so Word/PowerPoint
// embedding details never need to know how Graph positions or styles legends.
type graphLegendSemantics struct {
	seen      bool
	show      bool
	position  string
	textStyle *chartpkg.TextProperties
}

type graphFont struct {
	style chartpkg.TextProperties
}

type graphTextBlock struct {
	startDepth int
	fontIndex  int
	objectLink int
	color      string
}

func parseGraphFontRecord(payload []byte) (graphFont, bool) {
	// BIFF FONT: dyHeight(2), grbit(2), icv(2), bls(2), sss(2),
	// uls(1), family(1), charset(1), reserved(1), then the font name.
	if len(payload) < 14 {
		return graphFont{}, false
	}
	heightTwips := int(binary.LittleEndian.Uint16(payload[0:2]))
	options := binary.LittleEndian.Uint16(payload[2:4])
	weight := int(binary.LittleEndian.Uint16(payload[6:8]))
	return graphFont{style: chartpkg.TextProperties{
		FontSize: heightTwips * 5, // 20 twips/pt -> hundredths of a point
		Bold:     weight >= 700,
		Italic:   options&0x0002 != 0,
	}}, true
}

func scanGraphTextSemantics(workbook []byte) graphTextSemantics {
	const (
		biffFont            = 0x0031
		biffChartSeriesText = 0x100D
		biffChartText       = 0x1025
		biffChartFontIndex  = 0x1026
		biffChartObjectLink = 0x1027
		biffChartBegin      = 0x1033
		biffChartEnd        = 0x1034
	)

	// Excel reserves font index 4.  BIFF FONT records therefore map as
	// 0,1,2,3,5,6,... rather than being a dense zero-based slice.
	fonts := map[int]graphFont{}
	nextFontIndex := 0
	depth := 0
	var block *graphTextBlock
	var out graphTextSemantics

	applyBlock := func(b *graphTextBlock) {
		if b == nil || b.fontIndex < 0 {
			return
		}
		f, ok := fonts[b.fontIndex]
		if !ok {
			return
		}
		style := f.style
		if b.color != "" {
			style.Color = b.color
		}
		switch b.objectLink {
		case 1:
			out.titleStyle = &style
		case 2:
			out.yAxisTitleStyle = &style
		case 3:
			out.xAxisTitleStyle = &style
		}
	}

	for pos := 0; pos+4 <= len(workbook); {
		id := binary.LittleEndian.Uint16(workbook[pos : pos+2])
		sz := int(binary.LittleEndian.Uint16(workbook[pos+2 : pos+4]))
		pos += 4
		if sz < 0 || pos+sz > len(workbook) {
			break
		}
		payload := workbook[pos : pos+sz]
		pos += sz

		switch id {
		case biffFont:
			if f, ok := parseGraphFontRecord(payload); ok {
				idx := nextFontIndex
				if idx == 4 {
					idx = 5
				}
				fonts[idx] = f
				nextFontIndex = idx + 1
			}
		case biffChartText:
			// A TEXT record owns the immediately nested chart text block.  The
			// semantic target is supplied by OBJECTLINK inside that block. TEXT
			// also carries the authored foreground RGB for the block.
			block = &graphTextBlock{startDepth: depth, fontIndex: -1}
			if len(payload) >= 8 {
				raw := binary.LittleEndian.Uint32(payload[4:8])
				block.color = graphChartColor(raw, 0)
			}
		case biffChartBegin:
			depth++
		case biffChartFontIndex:
			if block != nil && len(payload) >= 2 {
				block.fontIndex = int(binary.LittleEndian.Uint16(payload[:2]))
			}
		case biffChartObjectLink:
			if block != nil && len(payload) >= 2 {
				block.objectLink = int(binary.LittleEndian.Uint16(payload[:2]))
			}
		case biffChartSeriesText:
			// Text content itself is already recovered by the canonical BIFF
			// chart parser; this scanner only owns MSGraph formatting semantics.
		case biffChartEnd:
			if depth > 0 {
				depth--
			}
			if block != nil && depth <= block.startDepth {
				applyBlock(block)
				block = nil
			}
		}
	}
	return out
}

func graphLegendPosition(t byte) string {
	switch t {
	case 0:
		return "bottom"
	case 1:
		return "corner"
	case 2:
		return "top"
	case 3:
		return "right"
	case 4:
		return "left"
	case 7: // undocked legend; right is the canonical fallback used by Graph.
		return "right"
	default:
		return "right"
	}
}

func scanGraphLegendSemantics(workbook []byte) graphLegendSemantics {
	const (
		biffFont           = 0x0031
		biffChartLegend    = 0x1015
		biffChartFontIndex = 0x1026
		biffChartBegin     = 0x1033
		biffChartEnd       = 0x1034
	)

	fonts := map[int]graphFont{}
	nextFontIndex := 0
	depth := 0
	legendDepth := -1
	legendFontIndex := -1
	var out graphLegendSemantics

	applyLegendFont := func() {
		if legendFontIndex < 0 {
			return
		}
		if f, ok := fonts[legendFontIndex]; ok {
			style := f.style
			out.textStyle = &style
		}
	}

	for pos := 0; pos+4 <= len(workbook); {
		id := binary.LittleEndian.Uint16(workbook[pos : pos+2])
		sz := int(binary.LittleEndian.Uint16(workbook[pos+2 : pos+4]))
		pos += 4
		if sz < 0 || pos+sz > len(workbook) {
			break
		}
		payload := workbook[pos : pos+sz]
		pos += sz

		switch id {
		case biffFont:
			if f, ok := parseGraphFontRecord(payload); ok {
				idx := nextFontIndex
				if idx == 4 {
					idx = 5
				}
				fonts[idx] = f
				nextFontIndex = idx + 1
			}
		case biffChartLegend:
			out.seen = true
			out.show = true
			out.position = "right"
			if len(payload) >= 17 {
				out.position = graphLegendPosition(payload[16])
			}
			legendDepth = depth
			legendFontIndex = -1
		case biffChartBegin:
			depth++
		case biffChartFontIndex:
			if legendDepth >= 0 && depth > legendDepth && len(payload) >= 2 {
				legendFontIndex = int(binary.LittleEndian.Uint16(payload[:2]))
			}
		case biffChartEnd:
			if depth > 0 {
				depth--
			}
			if legendDepth >= 0 && depth <= legendDepth {
				applyLegendFont()
				legendDepth = -1
				legendFontIndex = -1
			}
		}
	}
	if legendDepth >= 0 {
		applyLegendFont()
	}
	return out
}

func applyGraphLegendSemantics(chart *chartpkg.Chart, legend graphLegendSemantics) {
	if chart == nil || !legend.seen {
		return
	}
	chart.Legend.Show = legend.show
	if legend.position != "" {
		chart.Legend.Position = legend.position
	}
	if legend.textStyle != nil {
		style := *legend.textStyle
		chart.Legend.TextStyle = &style
	}
}

func applyGraphTextSemantics(chart *chartpkg.Chart, text graphTextSemantics) {
	if chart == nil {
		return
	}
	if text.titleStyle != nil {
		style := *text.titleStyle
		chart.TitleStyle = &style
	}
	if text.xAxisTitleStyle != nil {
		style := *text.xAxisTitleStyle
		chart.XAxis.TitleStyle = &style
	}
	if text.yAxisTitleStyle != nil {
		style := *text.yAxisTitleStyle
		chart.YAxis.TitleStyle = &style
	}
}

// graphAxisSemantics captures authored MSGraph chart-axis metadata directly
// from the BIFF chart substream. The generic XLS reader owns worksheet/chart
// parsing, but Microsoft Graph embeds chart records in a compact workbook where
// axis NUMBERFORMATINDEX and VALUERANGE records are authoritative and may not
// be projected by an ordinary worksheet reader.
type graphValueAxisSemantics struct {
	min, max, major          float64
	hasMin, hasMax, hasMajor bool
	numFmt                   string
}

// graphAxisSemantics captures primary and secondary value-axis metadata.
// MSGraph stores both under AXISPARENT containers (iax=0 primary, iax=1
// secondary); keeping that ownership here prevents combo charts from silently
// collapsing both scales onto one canonical Y axis.
type graphAxisSemantics struct {
	primary   graphValueAxisSemantics
	secondary graphValueAxisSemantics
}

func openGraphWorkbookStream(ole []byte) ([]byte, error) {
	file, err := cfb.OpenBytes(ole)
	if err != nil {
		return nil, fmt.Errorf("msgraph: open compound document: %w", err)
	}
	workbook, ok := file.Stream("Workbook")
	if !ok {
		workbook, ok = file.Stream("Book")
	}
	if !ok || len(workbook) < 4 {
		return nil, errors.New("msgraph: workbook stream missing")
	}
	return workbook, nil
}

func scanGraphAxisSemantics(workbook []byte) graphAxisSemantics {
	const (
		biffChartAxis              = 0x101D
		biffChartValueRange        = 0x101F
		biffChartAxisParent        = 0x1041
		biffChartNumberFormatIndex = 0x104E
		biffChartBegin             = 0x1033
		biffChartEnd               = 0x1034
	)
	axisType := -1
	axisParent := 0
	axisParentDepth := -1
	depth := 0
	var out graphAxisSemantics
	target := func() *graphValueAxisSemantics {
		if axisParent == 1 {
			return &out.secondary
		}
		return &out.primary
	}
	for pos := 0; pos+4 <= len(workbook); {
		id := binary.LittleEndian.Uint16(workbook[pos : pos+2])
		sz := int(binary.LittleEndian.Uint16(workbook[pos+2 : pos+4]))
		pos += 4
		if sz < 0 || pos+sz > len(workbook) {
			break
		}
		payload := workbook[pos : pos+sz]
		pos += sz
		switch id {
		case biffChartAxisParent:
			if len(payload) >= 2 {
				axisParent = int(binary.LittleEndian.Uint16(payload[:2]))
				axisParentDepth = depth + 1
			}
		case biffChartAxis:
			if len(payload) >= 2 {
				axisType = int(binary.LittleEndian.Uint16(payload[:2]))
			}
		case biffChartValueRange:
			if axisType != 1 || len(payload) < 34 {
				continue
			}
			t := target()
			min := math.Float64frombits(binary.LittleEndian.Uint64(payload[0:8]))
			max := math.Float64frombits(binary.LittleEndian.Uint64(payload[8:16]))
			major := math.Float64frombits(binary.LittleEndian.Uint64(payload[16:24]))
			options := binary.LittleEndian.Uint16(payload[32:34])
			if options&0x01 == 0 {
				t.min, t.hasMin = min, true
			}
			if options&0x02 == 0 {
				t.max, t.hasMax = max, true
			}
			if options&0x04 == 0 {
				t.major, t.hasMajor = major, true
			}
		case biffChartNumberFormatIndex:
			if axisType == 1 && len(payload) >= 2 {
				target().numFmt = graphBuiltinNumberFormatCode(binary.LittleEndian.Uint16(payload[:2]))
			}
		case biffChartBegin:
			depth++
		case biffChartEnd:
			if depth > 0 {
				depth--
			}
			if axisParentDepth >= 0 && depth < axisParentDepth {
				axisParent = 0
				axisParentDepth = -1
				axisType = -1
			}
		}
	}
	return out
}

func applyGraphAxisSemantics(chart *chartpkg.Chart, axis graphAxisSemantics) {
	if chart == nil {
		return
	}
	apply := func(dst *chartpkg.Axis, src graphValueAxisSemantics) {
		if src.hasMin {
			dst.Min = src.min
		}
		if src.hasMax {
			dst.Max = src.max
		}
		if src.hasMajor {
			dst.MajorUnit = src.major
		}
		if src.numFmt != "" {
			dst.NumFmt = src.numFmt
		}
	}
	apply(&chart.YAxis, axis.primary)
	apply(&chart.YAxis2, axis.secondary)
}

func parseGraphDatasheet(ole []byte) (*graphDatasheet, error) {
	workbook, err := openGraphWorkbookStream(ole)
	if err != nil {
		return nil, err
	}

	const (
		biffCodepage    = 0x0042
		biffLabelBIFF2  = 0x0004
		biffNumberBIFF2 = 0x0003
	)
	codepage := 0
	t := &graphDatasheet{cells: map[int]map[int]graphCell{}, minRow: -1, maxRow: -1, minCol: -1, maxCol: -1}
	for pos := 0; pos+4 <= len(workbook); {
		id := binary.LittleEndian.Uint16(workbook[pos : pos+2])
		sz := int(binary.LittleEndian.Uint16(workbook[pos+2 : pos+4]))
		pos += 4
		if sz < 0 || pos+sz > len(workbook) {
			break
		}
		payload := workbook[pos : pos+sz]
		pos += sz
		switch id {
		case biffCodepage:
			if len(payload) >= 2 {
				codepage = int(binary.LittleEndian.Uint16(payload[:2]))
			}
		case biffLabelBIFF2:
			if len(payload) < 8 {
				continue
			}
			row := int(binary.LittleEndian.Uint16(payload[0:2]))
			col := int(binary.LittleEndian.Uint16(payload[2:4]))
			cch := int(payload[7])
			text, ok := decodeGraphBIFF2Label(payload[8:], cch, codepage)
			if ok {
				t.put(row, col, graphCell{text: strings.TrimSpace(text), isText: true})
			}
		case biffNumberBIFF2:
			if len(payload) < 15 {
				continue
			}
			row := int(binary.LittleEndian.Uint16(payload[0:2]))
			col := int(binary.LittleEndian.Uint16(payload[2:4]))
			fmtID := graphBIFF2NumberFormatIndex(payload[4:7])
			t.put(row, col, graphCell{
				number:     mathFloat64(payload[7:15]),
				isNum:      true,
				numFmtID:   fmtID,
				numFmtCode: graphBuiltinNumberFormatCode(fmtID),
			})
		}
	}
	if len(t.cells) == 0 {
		return nil, errors.New("msgraph: no datasheet cells found")
	}
	return t, nil
}

func graphBIFF2NumberFormatIndex(attrs []byte) uint16 {
	// BIFF2 stores the built-in format index in the middle cell-attribute byte.
	// Microsoft Graph 8 uses this directly in its datasheet NUMBER records (for
	// example 0x09 for percentage values). The upper two bits are unrelated cell
	// attribute flags, so only the lower six bits belong to the format index.
	if len(attrs) < 2 {
		return 0
	}
	return uint16(attrs[1] & 0x3f)
}

func graphBuiltinNumberFormatCode(index uint16) string {
	// Excel/MSGraph built-in number formats. Keep this table local to the Graph
	// datasheet decoder because BIFF2 cells carry the format directly instead of
	// referencing an XF record.
	switch index {
	case 0:
		return "General"
	case 1:
		return "0"
	case 2:
		return "0.00"
	case 3:
		return "#,##0"
	case 4:
		return "#,##0.00"
	case 9:
		return "0%"
	case 10:
		return "0.00%"
	case 11:
		return "0.00E+00"
	case 12:
		return "# ?/?"
	case 13:
		return "# ??/??"
	case 14:
		return "m/d/yy"
	case 15:
		return "d-mmm-yy"
	case 16:
		return "d-mmm"
	case 17:
		return "mmm-yy"
	case 18:
		return "h:mm AM/PM"
	case 19:
		return "h:mm:ss AM/PM"
	case 20:
		return "h:mm"
	case 21:
		return "h:mm:ss"
	case 22:
		return "m/d/yy h:mm"
	case 37:
		return "#,##0 ;(#,##0)"
	case 38:
		return "#,##0 ;[Red](#,##0)"
	case 39:
		return "#,##0.00;(#,##0.00)"
	case 40:
		return "#,##0.00;[Red](#,##0.00)"
	case 45:
		return "mm:ss"
	case 46:
		return "[h]:mm:ss"
	case 47:
		return "mmss.0"
	case 48:
		return "##0.0E+0"
	case 49:
		return "@"
	default:
		return ""
	}
}

func mathFloat64(b []byte) float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(b))
}

func decodeGraphBIFF2Label(data []byte, cch, codepage int) (string, bool) {
	if cch < 0 {
		return "", false
	}
	// MSGraph 8 emits BIFF2 LABEL records with the XLUnicodeString option byte
	// when CODEPAGE is 1200. A zero option means compressed 8-bit characters;
	// one means UTF-16LE. The option byte is not part of cch.
	if codepage == 1200 && len(data) >= 1 {
		switch data[0] & 0x01 {
		case 0:
			if 1+cch <= len(data) {
				return string(data[1 : 1+cch]), true
			}
		case 1:
			if 1+2*cch <= len(data) {
				runes := make([]rune, 0, cch)
				for i := 0; i < cch; i++ {
					runes = append(runes, rune(binary.LittleEndian.Uint16(data[1+i*2:1+i*2+2])))
				}
				return string(runes), true
			}
		}
	}
	if cch <= len(data) {
		return strings.TrimRight(string(data[:cch]), "\x00"), true
	}
	return "", false
}

func (t *graphDatasheet) put(row, col int, cell graphCell) {
	if t.cells[row] == nil {
		t.cells[row] = map[int]graphCell{}
	}
	t.cells[row][col] = cell
	if t.minRow < 0 || row < t.minRow {
		t.minRow = row
	}
	if row > t.maxRow {
		t.maxRow = row
	}
	if t.minCol < 0 || col < t.minCol {
		t.minCol = col
	}
	if col > t.maxCol {
		t.maxCol = col
	}
}

func (t *graphDatasheet) cell(row, col int) (graphCell, bool) {
	if t == nil {
		return graphCell{}, false
	}
	rowCells := t.cells[row]
	if rowCells == nil {
		return graphCell{}, false
	}
	c, ok := rowCells[col]
	return c, ok
}

func hydrateGraphDatasheet(chart *chartpkg.Chart, table *graphDatasheet) {
	if chart == nil || table == nil || len(chart.Series) == 0 || table.minRow < 0 || table.maxRow <= table.minRow {
		return
	}
	headerRow := table.minRow
	dataStart := headerRow + 1
	categoryCol := table.minCol

	// Graph's native datasheet convention is one category column followed by
	// one column per chart series. Preserve SeriesText names parsed from the BIFF
	// chart substream; table headers are only a fallback for missing/corrupt names.
	categories := make([]string, 0, table.maxRow-dataStart+1)
	for r := dataStart; r <= table.maxRow; r++ {
		if c, ok := table.cell(r, categoryCol); ok {
			switch {
			case c.isText && c.text != "":
				categories = append(categories, c.text)
			case c.isNum:
				categories = append(categories, strconv.FormatFloat(c.number, 'f', -1, 64))
			}
		}
	}
	if len(categories) > 0 {
		chart.Categories = categories
	}

	var sharedValueNumFmt string
	sharedValueNumFmtSet := false
	sharedValueNumFmtConflict := false
	for i := range chart.Series {
		series := &chart.Series[i]
		col := categoryCol + 1 + i
		if nameCell, ok := table.cell(headerRow, col); ok && nameCell.isText {
			name := strings.TrimSpace(nameCell.text)
			if graphTextNeedsRepair(series.Name) && name != "" {
				series.Name = name
			}
		}
		values := make([]float64, 0, table.maxRow-dataStart+1)
		seriesNumFmt := ""
		for r := dataStart; r <= table.maxRow; r++ {
			if c, ok := table.cell(r, col); ok && c.isNum {
				values = append(values, c.number)
				if seriesNumFmt == "" && c.numFmtCode != "" && c.numFmtCode != "General" {
					seriesNumFmt = c.numFmtCode
				}
			}
		}
		if len(values) > 0 {
			series.Values = values
		}
		if seriesNumFmt != "" {
			if !sharedValueNumFmtSet {
				sharedValueNumFmt = seriesNumFmt
				sharedValueNumFmtSet = true
			} else if sharedValueNumFmt != seriesNumFmt {
				sharedValueNumFmtConflict = true
			}
		}
	}
	// MSGraph datasheets carry number formatting on BIFF2 cell attributes rather
	// than necessarily emitting a separate chart NUMBERFORMATINDEX record. When
	// all value-series columns agree, project that authored format onto the shared
	// value axis. This turns stored fractions such as 0.35..0.75 into the authored
	// 35%..75% axis without any presentation-specific inference.
	if chart.YAxis.NumFmt == "" && sharedValueNumFmtSet && !sharedValueNumFmtConflict {
		chart.YAxis.NumFmt = sharedValueNumFmt
	}
}

func graphTextNeedsRepair(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || strings.ContainsRune(s, '\uFFFD') || strings.ContainsRune(s, '\x00') || strings.HasPrefix(s, "?")
}

// graphLabelSemantics captures the label contract authored by Microsoft Graph.
// SERIESLABELS belongs to the Graph chart model (not to the host document), and
// TICK carries the visible axis-label color/placement for each axis.
type graphLabelSemantics struct {
	series []graphSeriesLabelStyle
	xAxis  graphAxisLabelStyle
	yAxis  graphAxisLabelStyle
}

type graphSeriesLabelStyle struct {
	seen           bool
	showValue      bool
	showPercent    bool
	showCategory   bool
	showBubbleSize bool
	smooth         bool
}

type graphAxisLabelStyle struct {
	seen          bool
	majorTickType int
	minorTickType int
	labelPosition int
	color         string
}

func scanGraphLabelSemantics(workbook []byte) graphLabelSemantics {
	const (
		biffChartSeries       = 0x1003
		biffChartDataFormat   = 0x1006
		biffChartSeriesLabels = 0x100C
		biffChartAxis         = 0x101D
		biffChartTick         = 0x101E
		biffChartBegin        = 0x1033
		biffChartEnd          = 0x1034
	)

	var out graphLabelSemantics
	depth := 0
	currentSeries := -1
	seriesDepth := -1
	formatSeries := -1
	formatDepth := -1
	axisType := -1

	ensureSeries := func(index int) *graphSeriesLabelStyle {
		for len(out.series) <= index {
			out.series = append(out.series, graphSeriesLabelStyle{})
		}
		return &out.series[index]
	}

	for pos := 0; pos+4 <= len(workbook); {
		id := binary.LittleEndian.Uint16(workbook[pos : pos+2])
		sz := int(binary.LittleEndian.Uint16(workbook[pos+2 : pos+4]))
		pos += 4
		if sz < 0 || pos+sz > len(workbook) {
			break
		}
		payload := workbook[pos : pos+sz]
		pos += sz

		switch id {
		case biffChartSeries:
			currentSeries++
			ensureSeries(currentSeries)
			seriesDepth = depth + 1
		case biffChartDataFormat:
			if len(payload) >= 4 {
				s := int(int16(binary.LittleEndian.Uint16(payload[2:4])))
				if s >= 0 {
					formatSeries, formatDepth = s, depth+1
					ensureSeries(s)
				}
			}
		case biffChartAxis:
			if len(payload) >= 2 {
				axisType = int(binary.LittleEndian.Uint16(payload[:2]))
			}
		case biffChartTick:
			if len(payload) < 30 {
				continue
			}
			style := graphAxisLabelStyle{
				seen:          true,
				majorTickType: int(payload[0]),
				minorTickType: int(payload[1]),
				labelPosition: int(payload[2]),
				color:         graphChartColor(binary.LittleEndian.Uint32(payload[4:8]), binary.LittleEndian.Uint16(payload[26:28])),
			}
			switch axisType {
			case 0: // category axis
				out.xAxis = style
			case 1: // value axis
				out.yAxis = style
			}
		case biffChartSeriesLabels:
			if len(payload) < 2 {
				continue
			}
			target := formatSeries
			if target < 0 {
				target = currentSeries
			}
			if target < 0 {
				continue
			}
			flags := binary.LittleEndian.Uint16(payload[:2])
			st := ensureSeries(target)
			st.seen = true
			st.showValue = flags&0x0001 != 0
			st.showPercent = flags&0x0002 != 0
			st.smooth = flags&0x0008 != 0
			st.showCategory = flags&0x0010 != 0
			st.showBubbleSize = flags&0x0020 != 0
		case biffChartBegin:
			depth++
		case biffChartEnd:
			if depth > 0 {
				depth--
			}
			if formatDepth > 0 && depth < formatDepth {
				formatSeries, formatDepth = -1, -1
			}
			if seriesDepth > 0 && depth < seriesDepth {
				currentSeries, seriesDepth = -1, -1
			}
		}
	}
	return out
}

func applyGraphLabelSemantics(chart *chartpkg.Chart, semantics graphLabelSemantics) {
	if chart == nil {
		return
	}
	for i := range chart.Series {
		if i >= len(semantics.series) || !semantics.series[i].seen {
			continue
		}
		st := semantics.series[i]
		s := &chart.Series[i]
		s.ShowDataLabels = st.showValue
		s.ShowPercent = st.showPercent
		s.ShowCatName = st.showCategory
		s.ShowBubbleSize = st.showBubbleSize
		s.Smooth = st.smooth
	}
	applyAxis := func(axis *chartpkg.Axis, style graphAxisLabelStyle) {
		if axis == nil || !style.seen {
			return
		}
		axis.MajorTickType = style.majorTickType
		axis.MinorTickType = style.minorTickType
		axis.LabelPosition = style.labelPosition
		if style.color != "" {
			if axis.LabelStyle == nil {
				axis.LabelStyle = &chartpkg.TextProperties{}
			}
			axis.LabelStyle.Color = style.color
		}
	}
	applyAxis(&chart.XAxis, semantics.xAxis)
	applyAxis(&chart.YAxis, semantics.yAxis)
}

// graphSeriesSemantics captures Microsoft Graph BIFF series/data-point visual
// formatting directly from SERIES/DATAFORMAT/LINEFORMAT/MARKERFORMAT/AREAFORMAT
// records. The generic XLS chart parser already understands these records, but
// MSGraph owns the embedded chart contract and therefore re-projects the
// authoritative formatting while the object identity is still known.
type graphSeriesSemantics struct {
	series []graphSeriesStyle
}

type graphSeriesStyle struct {
	lineColor   string
	linePattern int
	lineWidth   int
	fillColor   string
	markerColor string
	markerShape string
	points      map[int]graphPointStyle
}

type graphPointStyle struct {
	lineColor string
	fillColor string
}

var graphDefaultBIFFPalette = [56]string{
	"000000", "FFFFFF", "FF0000", "00FF00", "0000FF", "FFFF00", "FF00FF", "00FFFF",
	"800000", "008000", "000080", "808000", "800080", "008080", "C0C0C0", "808080",
	"9999FF", "993366", "FFFFCC", "CCFFFF", "660066", "FF8080", "0066CC", "CCCCFF",
	"000080", "FF00FF", "FFFF00", "00FFFF", "800080", "800000", "008080", "0000FF",
	"00CCFF", "CCFFFF", "CCFFCC", "FFFF99", "99CCFF", "FF99CC", "CC99FF", "FFCC99",
	"3366FF", "33CCCC", "99CC00", "FFCC00", "FF9900", "FF6600", "666699", "969696",
	"003366", "339966", "003300", "333300", "993300", "993366", "333399", "333333",
}

func graphChartColor(raw uint32, paletteIndex uint16) string {
	// 0xFF000000 and zero are automatic. Prefer the explicit RGB payload when
	// present, otherwise honor the BIFF palette index carried by the record.
	if raw != 0 && raw != 0xFF000000 {
		r := byte(raw)
		g := byte(raw >> 8)
		b := byte(raw >> 16)
		return fmt.Sprintf("%02X%02X%02X", r, g, b)
	}
	if paletteIndex >= 0x08 && paletteIndex <= 0x3F {
		return graphDefaultBIFFPalette[paletteIndex-0x08]
	}
	return ""
}

func graphLineWidthEMU(weight uint16) int {
	switch weight {
	case 0xFFFF:
		return 6350 // hairline, 0.5pt
	case 1:
		return 25400 // medium, 2pt
	case 2:
		return 38100 // wide, 3pt
	default:
		return 12700 // narrow/default, 1pt
	}
}

func graphMarkerShape(marker uint16) string {
	switch marker {
	case 0:
		return "none"
	case 1:
		return "square"
	case 2:
		return "diamond"
	case 3:
		return "triangle"
	case 4:
		return "x"
	case 5:
		return "star"
	case 6:
		return "shortDash"
	case 7:
		return "longDash"
	case 8:
		return "circle"
	case 9:
		return "plus"
	default:
		return "circle"
	}
}

func scanGraphSeriesSemantics(workbook []byte) graphSeriesSemantics {
	const (
		biffChartSeries       = 0x1003
		biffChartDataFormat   = 0x1006
		biffChartLineFormat   = 0x1007
		biffChartMarkerFormat = 0x1009
		biffChartAreaFormat   = 0x100A
		biffChartBegin        = 0x1033
		biffChartEnd          = 0x1034
	)

	var out graphSeriesSemantics
	depth := 0
	currentSeries := -1
	seriesDepth := -1
	formatSeries := -1
	formatPoint := -1
	formatDepth := -1

	ensureSeries := func(index int) *graphSeriesStyle {
		for len(out.series) <= index {
			out.series = append(out.series, graphSeriesStyle{})
		}
		return &out.series[index]
	}
	styleTarget := func() (series int, point int, ok bool) {
		series, point = formatSeries, formatPoint
		if series < 0 {
			series = currentSeries
			point = -1
		}
		return series, point, series >= 0
	}

	for pos := 0; pos+4 <= len(workbook); {
		id := binary.LittleEndian.Uint16(workbook[pos : pos+2])
		sz := int(binary.LittleEndian.Uint16(workbook[pos+2 : pos+4]))
		pos += 4
		if sz < 0 || pos+sz > len(workbook) {
			break
		}
		payload := workbook[pos : pos+sz]
		pos += sz

		switch id {
		case biffChartSeries:
			currentSeries++
			ensureSeries(currentSeries)
			seriesDepth = depth + 1
		case biffChartBegin:
			depth++
		case biffChartEnd:
			if depth > 0 {
				depth--
			}
			if formatDepth > 0 && depth < formatDepth {
				formatSeries, formatPoint, formatDepth = -1, -1, -1
			}
			if seriesDepth > 0 && depth < seriesDepth {
				currentSeries, seriesDepth = -1, -1
			}
		case biffChartDataFormat:
			if len(payload) >= 4 {
				p := int(binary.LittleEndian.Uint16(payload[0:2]))
				if p == 0xFFFF {
					p = -1
				}
				s := int(int16(binary.LittleEndian.Uint16(payload[2:4])))
				if s >= 0 {
					formatSeries, formatPoint, formatDepth = s, p, depth+1
					ensureSeries(s)
				}
			}
		case biffChartLineFormat:
			series, point, ok := styleTarget()
			if !ok || len(payload) < 8 {
				continue
			}
			raw := binary.LittleEndian.Uint32(payload[0:4])
			pattern := int(binary.LittleEndian.Uint16(payload[4:6]))
			weight := binary.LittleEndian.Uint16(payload[6:8])
			palette := uint16(0)
			if len(payload) >= 12 {
				palette = binary.LittleEndian.Uint16(payload[10:12])
			}
			color := graphChartColor(raw, palette)
			st := ensureSeries(series)
			if point >= 0 {
				if st.points == nil {
					st.points = map[int]graphPointStyle{}
				}
				pt := st.points[point]
				pt.lineColor = color
				st.points[point] = pt
			} else {
				st.lineColor = color
				st.linePattern = pattern
				st.lineWidth = graphLineWidthEMU(weight)
			}
		case biffChartAreaFormat:
			series, point, ok := styleTarget()
			if !ok || len(payload) < 10 {
				continue
			}
			raw := binary.LittleEndian.Uint32(payload[0:4])
			palette := uint16(0)
			if len(payload) >= 14 {
				palette = binary.LittleEndian.Uint16(payload[12:14])
			}
			color := graphChartColor(raw, palette)
			st := ensureSeries(series)
			if point >= 0 {
				if st.points == nil {
					st.points = map[int]graphPointStyle{}
				}
				pt := st.points[point]
				pt.fillColor = color
				st.points[point] = pt
			} else {
				st.fillColor = color
			}
		case biffChartMarkerFormat:
			series, point, ok := styleTarget()
			if !ok || point >= 0 || len(payload) < 10 {
				continue
			}
			forePalette := uint16(0)
			if len(payload) >= 16 {
				forePalette = binary.LittleEndian.Uint16(payload[12:14])
			}
			color := graphChartColor(binary.LittleEndian.Uint32(payload[0:4]), forePalette)
			st := ensureSeries(series)
			st.markerColor = color
			st.markerShape = graphMarkerShape(binary.LittleEndian.Uint16(payload[8:10]))
		}
	}
	return out
}

func ensureGraphDataPoint(series *chartpkg.Series, index int) *chartpkg.DataPoint {
	for i := range series.DataPoints {
		if series.DataPoints[i].Index == index {
			return &series.DataPoints[i]
		}
	}
	series.DataPoints = append(series.DataPoints, chartpkg.DataPoint{Index: index})
	return &series.DataPoints[len(series.DataPoints)-1]
}

func applyGraphSeriesSemantics(chart *chartpkg.Chart, semantics graphSeriesSemantics) {
	if chart == nil {
		return
	}
	for i := range chart.Series {
		if i >= len(semantics.series) {
			break
		}
		style := semantics.series[i]
		series := &chart.Series[i]
		if style.lineColor != "" {
			series.LineColor = style.lineColor
		}
		if style.lineWidth > 0 {
			series.LineWidth = style.lineWidth
		}
		series.LinePattern = style.linePattern
		if style.fillColor != "" {
			series.FillColor = style.fillColor
		}
		if style.markerColor != "" {
			series.MarkerColor = style.markerColor
		}
		if style.markerShape != "" {
			series.MarkerShape = style.markerShape
		}
		for index, pointStyle := range style.points {
			point := ensureGraphDataPoint(series, index)
			if pointStyle.lineColor != "" {
				point.LineColor = pointStyle.lineColor
			}
			if pointStyle.fillColor != "" {
				point.FillColor = pointStyle.fillColor
			}
		}
	}
}

// graphGroupSemantics captures MSGraph chart-group ownership.  BIFF stores
// chart groups in CHARTFORMAT containers, series-to-group links in SerToCrt,
// and primary/secondary-axis ownership in AXISPARENT.
type graphGroupSemantics struct {
	groups        []graphGroup
	seriesToGroup map[int]int
}

type graphGroup struct {
	chartType  chartpkg.ChartType
	axisParent int
	grouping   string
	barDir     string
	holeSize   int
}

func scanGraphGroupSemantics(workbook []byte) graphGroupSemantics {
	const (
		biffChartSeries     = 0x1003
		biffChartFormat     = 0x1014
		biffChartBar        = 0x1017
		biffChartLine       = 0x1018
		biffChartPie        = 0x1019
		biffChartArea       = 0x101A
		biffChartScatter    = 0x101B
		biffChartRadar      = 0x103E
		biffChartSurface    = 0x103F
		biffChartRadarArea  = 0x1040
		biffChartAxisParent = 0x1041
		biffChartSerToCrt   = 0x1045
		biffChartBegin      = 0x1033
		biffChartEnd        = 0x1034
	)
	out := graphGroupSemantics{seriesToGroup: map[int]int{}}
	depth := 0
	axisParent := 0
	axisParentDepth := -1
	currentGroup := -1
	groupDepth := -1
	currentSeries := -1
	seriesDepth := -1

	ensureGroup := func(i int) *graphGroup {
		for len(out.groups) <= i {
			out.groups = append(out.groups, graphGroup{axisParent: axisParent})
		}
		return &out.groups[i]
	}

	for pos := 0; pos+4 <= len(workbook); {
		id := binary.LittleEndian.Uint16(workbook[pos : pos+2])
		sz := int(binary.LittleEndian.Uint16(workbook[pos+2 : pos+4]))
		pos += 4
		if sz < 0 || pos+sz > len(workbook) {
			break
		}
		payload := workbook[pos : pos+sz]
		pos += sz

		switch id {
		case biffChartAxisParent:
			if len(payload) >= 2 {
				axisParent = int(binary.LittleEndian.Uint16(payload[:2]))
				axisParentDepth = depth + 1
			}
		case biffChartFormat:
			currentGroup++
			g := ensureGroup(currentGroup)
			g.axisParent = axisParent
			groupDepth = depth + 1
		case biffChartBar:
			if currentGroup >= 0 {
				g := ensureGroup(currentGroup)
				g.chartType = chartpkg.ChartColumn
				g.barDir = "col"
				if len(payload) >= 6 {
					flags := binary.LittleEndian.Uint16(payload[4:6])
					if flags&0x0001 != 0 {
						g.chartType, g.barDir = chartpkg.ChartBar, "bar"
					}
					switch {
					case flags&0x0004 != 0:
						g.grouping = "percentStacked"
					case flags&0x0002 != 0:
						g.grouping = "stacked"
					default:
						g.grouping = "clustered"
					}
				}
			}
		case biffChartLine:
			if currentGroup >= 0 {
				ensureGroup(currentGroup).chartType = chartpkg.ChartLine
			}
		case biffChartPie:
			if currentGroup >= 0 {
				g := ensureGroup(currentGroup)
				g.chartType = chartpkg.ChartPie
				if len(payload) >= 4 {
					g.holeSize = int(binary.LittleEndian.Uint16(payload[2:4]))
					if g.holeSize >= 10 && g.holeSize <= 90 {
						g.chartType = chartpkg.ChartDoughnut
					}
				}
			}
		case biffChartArea:
			if currentGroup >= 0 {
				ensureGroup(currentGroup).chartType = chartpkg.ChartArea
			}
		case biffChartScatter:
			if currentGroup >= 0 {
				ensureGroup(currentGroup).chartType = chartpkg.ChartScatter
			}
		case biffChartRadar, biffChartRadarArea:
			if currentGroup >= 0 {
				ensureGroup(currentGroup).chartType = chartpkg.ChartRadar
			}
		case biffChartSurface:
			if currentGroup >= 0 {
				ensureGroup(currentGroup).chartType = chartpkg.ChartSurface
			}
		case biffChartSeries:
			currentSeries++
			seriesDepth = depth + 1
		case biffChartSerToCrt:
			if currentSeries >= 0 && len(payload) >= 2 {
				out.seriesToGroup[currentSeries] = int(binary.LittleEndian.Uint16(payload[:2]))
			}
		case biffChartBegin:
			depth++
		case biffChartEnd:
			if depth > 0 {
				depth--
			}
			if seriesDepth > 0 && depth < seriesDepth {
				currentSeries, seriesDepth = -1, -1
			}
			if groupDepth > 0 && depth < groupDepth {
				groupDepth = -1
			}
			if axisParentDepth > 0 && depth < axisParentDepth {
				axisParent, axisParentDepth = 0, -1
			}
		}
	}
	return out
}

func applyGraphGroupSemantics(chart *chartpkg.Chart, semantics graphGroupSemantics) {
	if chart == nil || len(semantics.groups) == 0 {
		return
	}
	groups := make([]chartpkg.ChartGroup, len(semantics.groups))
	for i, src := range semantics.groups {
		groups[i] = chartpkg.ChartGroup{ChartType: src.chartType, Grouping: src.grouping, HoleSize: src.holeSize, BarDir: src.barDir}
	}
	for i := range chart.Series {
		g := 0
		if explicit, ok := semantics.seriesToGroup[i]; ok {
			g = explicit
		}
		if g < 0 || g >= len(groups) {
			continue
		}
		groups[g].SeriesIndices = append(groups[g].SeriesIndices, i)
		if groups[g].ChartType != chartpkg.ChartUnknown {
			chart.Series[i].ChartType = groups[g].ChartType
		}
		chart.Series[i].SetSecondaryValueAxis(semantics.groups[g].axisParent == 1)
	}
	chart.Groups = groups
	if groups[0].ChartType != chartpkg.ChartUnknown {
		chart.ChartType = groups[0].ChartType
	}
	chart.Stacked = groups[0].Grouping == "stacked" || groups[0].Grouping == "percentStacked"
	chart.PercentStacked = groups[0].Grouping == "percentStacked"
	if groups[0].HoleSize > 0 {
		chart.HoleSize = groups[0].HoleSize
	}
}

// Parse parses an embedded MSGraph compound document. Microsoft Graph stores
// its semantic chart model in a BIFF workbook stream; reuse the authoritative
// legacy workbook/chart parser and return chart models rather than preview art.
func Parse(data []byte) (*Document, error) {
	if len(data) == 0 {
		return nil, errors.New("msgraph: empty OLE payload")
	}
	wb, err := legacysheet.ParseXlsBytes(data, "embedded-msgraph.xls")
	if err != nil {
		return nil, fmt.Errorf("msgraph: parse embedded workbook: %w", err)
	}
	if wb == nil {
		return nil, errors.New("msgraph: embedded workbook is nil")
	}
	doc := &Document{Workbook: wb}
	graphTable, _ := parseGraphDatasheet(data)
	workbookStream, _ := openGraphWorkbookStream(data)
	axisSemantics := scanGraphAxisSemantics(workbookStream)
	textSemantics := scanGraphTextSemantics(workbookStream)
	legendSemantics := scanGraphLegendSemantics(workbookStream)
	seriesSemantics := scanGraphSeriesSemantics(workbookStream)
	labelSemantics := scanGraphLabelSemantics(workbookStream)
	groupSemantics := scanGraphGroupSemantics(workbookStream)
	for _, sheet := range wb.Sheets {
		if sheet == nil {
			continue
		}
		for _, drawing := range sheet.Drawings {
			if drawing == nil || drawing.Chart == nil || drawing.Chart.Chart == nil {
				continue
			}
			model, ok := drawing.Chart.Chart.(*chartpkg.Chart)
			if !ok || model == nil {
				continue
			}
			if graphTable != nil {
				hydrateGraphDatasheet(model, graphTable)
			} else {
				hydrateGraphTableFallback(model, sheet)
			}
			applyGraphAxisSemantics(model, axisSemantics)
			applyGraphTextSemantics(model, textSemantics)
			applyGraphLegendSemantics(model, legendSemantics)
			applyGraphSeriesSemantics(model, seriesSemantics)
			applyGraphLabelSemantics(model, labelSemantics)
			applyGraphGroupSemantics(model, groupSemantics)
			doc.Charts = append(doc.Charts, Chart{SheetName: sheet.Name, Model: model})
		}
	}
	if len(doc.Charts) == 0 {
		return doc, errors.New("msgraph: no semantic chart records found")
	}
	return doc, nil
}

// MarshalPrimaryChart returns the first recovered chart in canonical OOXML
// chart-part form so existing shared chart layout/rendering can consume it.
func MarshalPrimaryChart(data []byte) ([]byte, error) {
	doc, err := Parse(data)
	if err != nil {
		return nil, err
	}
	for _, c := range doc.Charts {
		if c.Model == nil {
			continue
		}
		out, err := chartpkg.MarshalChart(c.Model)
		if err == nil && len(out) > 0 {
			return out, nil
		}
	}
	return nil, errors.New("msgraph: no chart could be marshaled")
}

// hydrateGraphTableFallback handles the native Microsoft Graph datasheet
// convention when old BIFF AI formulas are absent or intentionally dummy.
// This belongs to MSGraph parsing rather than presentation parsing: column 1 is
// the category column, later columns are series, and row 1 is the optional
// header row. Existing AI-resolved values always win.
func hydrateGraphTableFallback(chart *chartpkg.Chart, sheet *sheetir.Sheet) {
	if chart == nil || sheet == nil || len(chart.Series) == 0 || len(sheet.Rows) == 0 {
		return
	}
	rows := make(map[int]map[int]*sheetir.Cell, len(sheet.Rows))
	minRow, maxRow := 0, 0
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		if minRow == 0 || row.Index < minRow {
			minRow = row.Index
		}
		if row.Index > maxRow {
			maxRow = row.Index
		}
		cols := map[int]*sheetir.Cell{}
		for _, cell := range row.Cells {
			if cell != nil {
				cols[cell.Col] = cell
			}
		}
		rows[row.Index] = cols
	}
	if minRow == 0 || maxRow <= minRow {
		return
	}
	dataStart := minRow + 1

	if len(chart.Categories) == 0 {
		categories := make([]string, 0, maxRow-dataStart+1)
		for r := dataStart; r <= maxRow; r++ {
			cell := rows[r][1]
			if cell == nil {
				continue
			}
			switch cell.Kind {
			case sheetir.CellString, sheetir.CellFormula, sheetir.CellDate:
				if s := strings.TrimSpace(cell.StrValue); s != "" {
					categories = append(categories, s)
				}
			case sheetir.CellNumber, sheetir.CellBool:
				categories = append(categories, strconv.FormatFloat(cell.NumValue, 'f', -1, 64))
			}
		}
		if len(categories) > 0 {
			chart.Categories = categories
		}
	}

	for i := range chart.Series {
		series := &chart.Series[i]
		column := i + 2
		if series.Name == "" {
			if header := rows[minRow][column]; header != nil {
				if s := strings.TrimSpace(header.StrValue); s != "" {
					series.Name = s
				}
			}
		}
		if len(series.Values) != 0 {
			continue
		}
		values := make([]float64, 0, maxRow-dataStart+1)
		for r := dataStart; r <= maxRow; r++ {
			cell := rows[r][column]
			if cell != nil && (cell.Kind == sheetir.CellNumber || cell.Kind == sheetir.CellBool) {
				values = append(values, cell.NumValue)
			}
		}
		if len(values) > 0 {
			series.Values = values
		}
	}
}
