package msgraph

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	chartpkg "office/pkg/ooxml/chart"
)

func TestIsProgID(t *testing.T) {
	for _, s := range []string{"MSGraph.Chart.8", "MSGraph.Chart", "Graph.Chart.8", "msgraph.chart.12"} {
		if !IsProgID(s) {
			t.Fatalf("IsProgID(%q)=false", s)
		}
	}
	if IsProgID("Excel.Sheet.8") {
		t.Fatal("Excel object classified as MSGraph")
	}
}

func TestDecodeGraphBIFF2UnicodeCodepageCompressedLabel(t *testing.T) {
	// MSGraph 8 advertises CODEPAGE 1200 but stores BIFF2 LABEL text as
	// option-byte(0) + compressed characters. cch counts characters only.
	got, ok := decodeGraphBIFF2Label([]byte{0, 'P', 'e', 'r', 's', 'o', 'n', 'a', 'l'}, 8, 1200)
	if !ok {
		t.Fatal("decodeGraphBIFF2Label returned !ok")
	}
	if got != "Personal" {
		t.Fatalf("decoded label = %q, want Personal", got)
	}
}

func TestHydrateGraphDatasheetUsesCategoryColumnAndSeriesColumns(t *testing.T) {
	table := &graphDatasheet{cells: map[int]map[int]graphCell{}, minRow: -1, maxRow: -1, minCol: -1, maxCol: -1}
	table.put(0, 1, graphCell{text: "Personal", isText: true})
	table.put(0, 2, graphCell{text: "IPT", isText: true})
	table.put(0, 3, graphCell{text: "Bus", isText: true})
	for i, row := range []struct {
		year               float64
		personal, ipt, bus float64
	}{{1974, 174, 18, 5}, {1975, 182, 19, 6}, {1976, 217, 20, 6}} {
		r := i + 1
		table.put(r, 0, graphCell{number: row.year, isNum: true})
		table.put(r, 1, graphCell{number: row.personal, isNum: true})
		table.put(r, 2, graphCell{number: row.ipt, isNum: true})
		table.put(r, 3, graphCell{number: row.bus, isNum: true})
	}
	chart := &chartpkg.Chart{Series: []chartpkg.Series{{Name: "Personal"}, {Name: "IPT"}, {Name: "Bus"}}}
	hydrateGraphDatasheet(chart, table)
	if got := strings.Join(chart.Categories, ","); got != "1974,1975,1976" {
		t.Fatalf("categories = %q", got)
	}
	if got := chart.Series[0].Values; len(got) != 3 || got[0] != 174 || got[2] != 217 {
		t.Fatalf("Personal values = %v", got)
	}
	if got := chart.Series[1].Values; len(got) != 3 || got[0] != 18 || got[2] != 20 {
		t.Fatalf("IPT values = %v", got)
	}
	if got := chart.Series[2].Values; len(got) != 3 || got[0] != 5 || got[2] != 6 {
		t.Fatalf("Bus values = %v", got)
	}
}

func TestHydrateGraphDatasheetRepairsOnlyCorruptSeriesNames(t *testing.T) {
	table := &graphDatasheet{cells: map[int]map[int]graphCell{}, minRow: -1, maxRow: -1, minCol: -1, maxCol: -1}
	table.put(0, 1, graphCell{text: "Personal", isText: true})
	table.put(0, 2, graphCell{text: "IPT", isText: true})
	table.put(1, 0, graphCell{number: 1974, isNum: true})
	table.put(1, 1, graphCell{number: 174, isNum: true})
	table.put(1, 2, graphCell{number: 18, isNum: true})
	chart := &chartpkg.Chart{Series: []chartpkg.Series{{Name: "\ufffdPersonal"}, {Name: "Authored IPT"}}}
	hydrateGraphDatasheet(chart, table)
	if chart.Series[0].Name != "Personal" {
		t.Fatalf("repaired name = %q, want Personal", chart.Series[0].Name)
	}
	if chart.Series[1].Name != "Authored IPT" {
		t.Fatalf("authored name overwritten = %q", chart.Series[1].Name)
	}
}

func TestGraphBIFF2NumberFormatIndexUsesMiddleAttributeByte(t *testing.T) {
	if got := graphBIFF2NumberFormatIndex([]byte{0x00, 0x09, 0x00}); got != 9 {
		t.Fatalf("format index = %d, want 9", got)
	}
	if got := graphBuiltinNumberFormatCode(9); got != "0%" {
		t.Fatalf("format code = %q, want 0%%", got)
	}
}

func TestHydrateGraphDatasheetProjectsSharedValueFormatToAxis(t *testing.T) {
	table := &graphDatasheet{cells: map[int]map[int]graphCell{}, minRow: -1, maxRow: -1, minCol: -1, maxCol: -1}
	table.put(0, 1, graphCell{text: "Share", isText: true})
	for i, row := range []struct {
		year  float64
		share float64
	}{{1987, 0.35}, {1990, 0.52}, {1993, 0.64}, {1999, 0.75}} {
		r := i + 1
		table.put(r, 0, graphCell{number: row.year, isNum: true, numFmtID: 0, numFmtCode: "General"})
		table.put(r, 1, graphCell{number: row.share, isNum: true, numFmtID: 9, numFmtCode: "0%"})
	}
	chart := &chartpkg.Chart{Series: []chartpkg.Series{{Name: "Share"}}}
	hydrateGraphDatasheet(chart, table)
	if chart.YAxis.NumFmt != "0%" {
		t.Fatalf("YAxis.NumFmt = %q, want 0%%", chart.YAxis.NumFmt)
	}
	if got := chart.Series[0].Values; len(got) != 4 || got[0] != 0.35 || got[3] != 0.75 {
		t.Fatalf("values = %v", got)
	}
}

func appendGraphBIFFRecord(dst []byte, id uint16, payload []byte) []byte {
	var hdr [4]byte
	binary.LittleEndian.PutUint16(hdr[0:2], id)
	binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
	dst = append(dst, hdr[:]...)
	return append(dst, payload...)
}

func TestScanGraphAxisSemanticsReadsExplicitValueRangeAndFormat(t *testing.T) {
	var data []byte
	axis := make([]byte, 2)
	binary.LittleEndian.PutUint16(axis, 1) // value axis
	data = appendGraphBIFFRecord(data, 0x101D, axis)

	vr := make([]byte, 34)
	binary.LittleEndian.PutUint64(vr[0:8], math.Float64bits(0.35))
	binary.LittleEndian.PutUint64(vr[8:16], math.Float64bits(0.75))
	binary.LittleEndian.PutUint64(vr[16:24], math.Float64bits(0.05))
	binary.LittleEndian.PutUint16(vr[32:34], 0) // explicit min/max/major
	data = appendGraphBIFFRecord(data, 0x101F, vr)

	nf := make([]byte, 2)
	binary.LittleEndian.PutUint16(nf, 9) // 0%
	data = appendGraphBIFFRecord(data, 0x104E, nf)

	got := scanGraphAxisSemantics(data)
	if !got.primary.hasMin || got.primary.min != 0.35 {
		t.Fatalf("y min = %v explicit=%v, want 0.35 true", got.primary.min, got.primary.hasMin)
	}
	if !got.primary.hasMax || got.primary.max != 0.75 {
		t.Fatalf("y max = %v explicit=%v, want 0.75 true", got.primary.max, got.primary.hasMax)
	}
	if !got.primary.hasMajor || got.primary.major != 0.05 {
		t.Fatalf("y major = %v explicit=%v, want 0.05 true", got.primary.major, got.primary.hasMajor)
	}
	if got.primary.numFmt != "0%" {
		t.Fatalf("y num fmt = %q, want 0%%", got.primary.numFmt)
	}
}

func TestScanGraphAxisSemanticsHonorsAutomaticFlags(t *testing.T) {
	var data []byte
	axis := make([]byte, 2)
	binary.LittleEndian.PutUint16(axis, 1)
	data = appendGraphBIFFRecord(data, 0x101D, axis)
	vr := make([]byte, 34)
	binary.LittleEndian.PutUint64(vr[0:8], math.Float64bits(10))
	binary.LittleEndian.PutUint64(vr[8:16], math.Float64bits(90))
	binary.LittleEndian.PutUint64(vr[16:24], math.Float64bits(10))
	binary.LittleEndian.PutUint16(vr[32:34], 0x07) // all automatic
	data = appendGraphBIFFRecord(data, 0x101F, vr)

	got := scanGraphAxisSemantics(data)
	if got.primary.hasMin || got.primary.hasMax || got.primary.hasMajor {
		t.Fatalf("automatic value range projected as explicit: %+v", got)
	}
}

func TestApplyGraphAxisSemanticsOverridesGenericAxisWithAuthoredMetadata(t *testing.T) {
	chart := &chartpkg.Chart{}
	chart.YAxis.Min = 0
	chart.YAxis.Max = 1
	chart.YAxis.MajorUnit = 0.2
	chart.YAxis.NumFmt = "General"
	applyGraphAxisSemantics(chart, graphAxisSemantics{primary: graphValueAxisSemantics{
		min: 0.35, max: 0.75, major: 0.05,
		hasMin: true, hasMax: true, hasMajor: true,
		numFmt: "0%",
	}})
	if chart.YAxis.Min != 0.35 || chart.YAxis.Max != 0.75 || chart.YAxis.MajorUnit != 0.05 {
		t.Fatalf("axis range = min=%v max=%v major=%v", chart.YAxis.Min, chart.YAxis.Max, chart.YAxis.MajorUnit)
	}
	if chart.YAxis.NumFmt != "0%" {
		t.Fatalf("axis format = %q, want 0%%", chart.YAxis.NumFmt)
	}
}

func makeGraphFontRecord(heightTwips uint16, italic bool, weight uint16) []byte {
	p := make([]byte, 16)
	binary.LittleEndian.PutUint16(p[0:2], heightTwips)
	if italic {
		binary.LittleEndian.PutUint16(p[2:4], 0x0002)
	}
	binary.LittleEndian.PutUint16(p[6:8], weight)
	p[14] = 0 // empty name is sufficient for style parsing
	p[15] = 0
	return p
}

func TestParseGraphFontRecordProjectsSizeWeightAndItalic(t *testing.T) {
	f, ok := parseGraphFontRecord(makeGraphFontRecord(400, true, 700))
	if !ok {
		t.Fatal("parseGraphFontRecord returned !ok")
	}
	if f.style.FontSize != 2000 {
		t.Fatalf("font size = %d, want 2000", f.style.FontSize)
	}
	if !f.style.Bold || !f.style.Italic {
		t.Fatalf("font style = bold:%v italic:%v, want true/true", f.style.Bold, f.style.Italic)
	}
}

func TestScanGraphTextSemanticsUsesReservedBIFFFontIndexAndObjectLink(t *testing.T) {
	var data []byte
	// FONT indexes are 0,1,2,3,5,6.  Index 4 is reserved by BIFF.
	for i := 0; i < 6; i++ {
		height := uint16(160 + i*20)
		weight := uint16(400)
		if i == 5 { // logical index 6
			height = 320 // 16pt
			weight = 700
		}
		data = appendGraphBIFFRecord(data, 0x0031, makeGraphFontRecord(height, false, weight))
	}

	data = appendGraphBIFFRecord(data, 0x1025, make([]byte, 32)) // TEXT
	data = appendGraphBIFFRecord(data, 0x1033, nil)              // BEGIN
	fontIdx := make([]byte, 2)
	binary.LittleEndian.PutUint16(fontIdx, 6)
	data = appendGraphBIFFRecord(data, 0x1026, fontIdx)
	link := make([]byte, 6)
	binary.LittleEndian.PutUint16(link, 2) // Y-axis title
	data = appendGraphBIFFRecord(data, 0x1027, link)
	data = appendGraphBIFFRecord(data, 0x1034, nil) // END

	got := scanGraphTextSemantics(data)
	if got.yAxisTitleStyle == nil {
		t.Fatal("Y-axis title style = nil")
	}
	if got.yAxisTitleStyle.FontSize != 1600 {
		t.Fatalf("Y-axis title font size = %d, want 1600", got.yAxisTitleStyle.FontSize)
	}
	if !got.yAxisTitleStyle.Bold {
		t.Fatal("Y-axis title bold = false, want true")
	}
}

func TestApplyGraphTextSemanticsProjectsCanonicalChartStyles(t *testing.T) {
	chart := &chartpkg.Chart{}
	title := &chartpkg.TextProperties{FontSize: 2000, Bold: true}
	x := &chartpkg.TextProperties{FontSize: 1200}
	y := &chartpkg.TextProperties{FontSize: 1600, Italic: true}
	applyGraphTextSemantics(chart, graphTextSemantics{
		titleStyle: title, xAxisTitleStyle: x, yAxisTitleStyle: y,
	})
	if chart.TitleStyle == nil || chart.TitleStyle.FontSize != 2000 || !chart.TitleStyle.Bold {
		t.Fatalf("title style = %+v", chart.TitleStyle)
	}
	if chart.XAxis.TitleStyle == nil || chart.XAxis.TitleStyle.FontSize != 1200 {
		t.Fatalf("X-axis title style = %+v", chart.XAxis.TitleStyle)
	}
	if chart.YAxis.TitleStyle == nil || chart.YAxis.TitleStyle.FontSize != 1600 || !chart.YAxis.TitleStyle.Italic {
		t.Fatalf("Y-axis title style = %+v", chart.YAxis.TitleStyle)
	}
}

func TestScanGraphSeriesSemanticsReadsSeriesLineFillAndMarkerFormatting(t *testing.T) {
	var data []byte
	data = appendGraphBIFFRecord(data, 0x1003, make([]byte, 12)) // SERIES
	data = appendGraphBIFFRecord(data, 0x1033, nil)              // BEGIN

	line := make([]byte, 12)
	binary.LittleEndian.PutUint32(line[0:4], 0x00332211) // RGB 11 22 33
	binary.LittleEndian.PutUint16(line[4:6], 2)          // dash pattern
	binary.LittleEndian.PutUint16(line[6:8], 1)          // medium
	data = appendGraphBIFFRecord(data, 0x1007, line)

	area := make([]byte, 16)
	binary.LittleEndian.PutUint32(area[0:4], 0x00665544) // RGB 44 55 66
	data = appendGraphBIFFRecord(data, 0x100A, area)

	marker := make([]byte, 20)
	binary.LittleEndian.PutUint32(marker[0:4], 0x00998877) // RGB 77 88 99
	binary.LittleEndian.PutUint16(marker[8:10], 2)         // diamond
	data = appendGraphBIFFRecord(data, 0x1009, marker)
	data = appendGraphBIFFRecord(data, 0x1034, nil) // END

	got := scanGraphSeriesSemantics(data)
	if len(got.series) != 1 {
		t.Fatalf("series semantics = %d, want 1", len(got.series))
	}
	s := got.series[0]
	if s.lineColor != "112233" || s.linePattern != 2 || s.lineWidth != 25400 {
		t.Fatalf("line style = color:%q pattern:%d width:%d", s.lineColor, s.linePattern, s.lineWidth)
	}
	if s.fillColor != "445566" {
		t.Fatalf("fill color = %q, want 445566", s.fillColor)
	}
	if s.markerColor != "778899" || s.markerShape != "diamond" {
		t.Fatalf("marker = color:%q shape:%q", s.markerColor, s.markerShape)
	}
}

func TestScanGraphSeriesSemanticsHonorsDataFormatPointOwnership(t *testing.T) {
	var data []byte
	data = appendGraphBIFFRecord(data, 0x1003, make([]byte, 12))
	data = appendGraphBIFFRecord(data, 0x1033, nil)

	format := make([]byte, 8)
	binary.LittleEndian.PutUint16(format[0:2], 2) // point 2
	binary.LittleEndian.PutUint16(format[2:4], 0) // series 0
	data = appendGraphBIFFRecord(data, 0x1006, format)
	data = appendGraphBIFFRecord(data, 0x1033, nil)

	area := make([]byte, 16)
	binary.LittleEndian.PutUint32(area[0:4], 0x00030201) // RGB 01 02 03
	data = appendGraphBIFFRecord(data, 0x100A, area)
	line := make([]byte, 12)
	binary.LittleEndian.PutUint32(line[0:4], 0x00060504) // RGB 04 05 06
	data = appendGraphBIFFRecord(data, 0x1007, line)
	data = appendGraphBIFFRecord(data, 0x1034, nil)
	data = appendGraphBIFFRecord(data, 0x1034, nil)

	got := scanGraphSeriesSemantics(data)
	pt, ok := got.series[0].points[2]
	if !ok {
		t.Fatal("point 2 style missing")
	}
	if pt.fillColor != "010203" || pt.lineColor != "040506" {
		t.Fatalf("point style = fill:%q line:%q", pt.fillColor, pt.lineColor)
	}
}

func TestApplyGraphSeriesSemanticsProjectsCanonicalSeriesAndPoints(t *testing.T) {
	chart := &chartpkg.Chart{Series: []chartpkg.Series{{Index: 0}}}
	applyGraphSeriesSemantics(chart, graphSeriesSemantics{series: []graphSeriesStyle{{
		lineColor: "112233", linePattern: 2, lineWidth: 25400,
		fillColor: "445566", markerColor: "778899", markerShape: "diamond",
		points: map[int]graphPointStyle{1: {fillColor: "ABCDEF", lineColor: "123456"}},
	}}})
	s := chart.Series[0]
	if s.LineColor != "112233" || s.LinePattern != 2 || s.LineWidth != 25400 || s.FillColor != "445566" {
		t.Fatalf("canonical series style = %+v", s)
	}
	if s.MarkerColor != "778899" || s.MarkerShape != "diamond" {
		t.Fatalf("canonical marker = color:%q shape:%q", s.MarkerColor, s.MarkerShape)
	}
	if len(s.DataPoints) != 1 || s.DataPoints[0].Index != 1 || s.DataPoints[0].FillColor != "ABCDEF" || s.DataPoints[0].LineColor != "123456" {
		t.Fatalf("canonical point style = %+v", s.DataPoints)
	}
}

func TestGraphChartColorUsesPaletteForAutomaticRGB(t *testing.T) {
	if got := graphChartColor(0xFF000000, 0x0A); got != "FF0000" {
		t.Fatalf("palette color = %q, want FF0000", got)
	}
}

func TestScanGraphLegendSemanticsReadsPositionAndFont(t *testing.T) {
	var data []byte
	for i := 0; i < 6; i++ {
		height := uint16(180)
		weight := uint16(400)
		if i == 5 { // logical BIFF font index 6
			height = 220
			weight = 700
		}
		data = appendGraphBIFFRecord(data, 0x0031, makeGraphFontRecord(height, false, weight))
	}
	legend := make([]byte, 17)
	legend[16] = 2 // top
	data = appendGraphBIFFRecord(data, 0x1015, legend)
	data = appendGraphBIFFRecord(data, 0x1033, nil)
	fontIdx := make([]byte, 2)
	binary.LittleEndian.PutUint16(fontIdx, 6)
	data = appendGraphBIFFRecord(data, 0x1026, fontIdx)
	data = appendGraphBIFFRecord(data, 0x1034, nil)

	got := scanGraphLegendSemantics(data)
	if !got.seen || !got.show || got.position != "top" {
		t.Fatalf("legend = seen:%v show:%v pos:%q", got.seen, got.show, got.position)
	}
	if got.textStyle == nil || got.textStyle.FontSize != 1100 || !got.textStyle.Bold {
		t.Fatalf("legend text style = %+v", got.textStyle)
	}
}

func TestApplyGraphLegendSemanticsProjectsCanonicalLegend(t *testing.T) {
	chart := &chartpkg.Chart{}
	style := &chartpkg.TextProperties{FontSize: 900, Italic: true}
	applyGraphLegendSemantics(chart, graphLegendSemantics{
		seen: true, show: true, position: "left", textStyle: style,
	})
	if !chart.Legend.Show || chart.Legend.Position != "left" {
		t.Fatalf("legend = %+v", chart.Legend)
	}
	if chart.Legend.TextStyle == nil || chart.Legend.TextStyle.FontSize != 900 || !chart.Legend.TextStyle.Italic {
		t.Fatalf("legend style = %+v", chart.Legend.TextStyle)
	}
}

func TestScanGraphTextSemanticsPreservesTextColor(t *testing.T) {
	var data []byte
	data = appendGraphBIFFRecord(data, 0x0031, makeGraphFontRecord(400, false, 700))
	text := make([]byte, 32)
	binary.LittleEndian.PutUint32(text[4:8], 0x00332211) // RGB 11 22 33
	data = appendGraphBIFFRecord(data, 0x1025, text)
	data = appendGraphBIFFRecord(data, 0x1033, nil)
	fontIdx := make([]byte, 2)
	binary.LittleEndian.PutUint16(fontIdx, 0)
	data = appendGraphBIFFRecord(data, 0x1026, fontIdx)
	link := make([]byte, 6)
	binary.LittleEndian.PutUint16(link, 1) // chart title
	data = appendGraphBIFFRecord(data, 0x1027, link)
	data = appendGraphBIFFRecord(data, 0x1034, nil)

	got := scanGraphTextSemantics(data)
	if got.titleStyle == nil {
		t.Fatal("title style = nil")
	}
	if got.titleStyle.Color != "112233" {
		t.Fatalf("title color = %q, want 112233", got.titleStyle.Color)
	}
}

func TestScanGraphLabelSemanticsReadsSeriesLabelFlags(t *testing.T) {
	var data []byte
	data = appendGraphBIFFRecord(data, 0x1003, make([]byte, 12)) // SERIES
	data = appendGraphBIFFRecord(data, 0x1033, nil)              // BEGIN
	labels := make([]byte, 2)
	// show value + percent + smooth + category + bubble size
	binary.LittleEndian.PutUint16(labels, 0x0001|0x0002|0x0008|0x0010|0x0020)
	data = appendGraphBIFFRecord(data, 0x100C, labels)
	data = appendGraphBIFFRecord(data, 0x1034, nil)

	got := scanGraphLabelSemantics(data)
	if len(got.series) != 1 {
		t.Fatalf("series label semantics = %d, want 1", len(got.series))
	}
	s := got.series[0]
	if !s.seen || !s.showValue || !s.showPercent || !s.showCategory || !s.showBubbleSize || !s.smooth {
		t.Fatalf("series label flags = %+v", s)
	}
}

func TestScanGraphLabelSemanticsReadsAxisTickLabelColor(t *testing.T) {
	var data []byte
	axis := make([]byte, 2)
	binary.LittleEndian.PutUint16(axis, 0) // category axis
	data = appendGraphBIFFRecord(data, 0x101D, axis)
	tick := make([]byte, 30)
	tick[0] = 1 // outside major tick
	tick[1] = 2 // inside minor tick
	tick[2] = 3 // labels next to axis
	binary.LittleEndian.PutUint32(tick[4:8], 0x00332211)
	data = appendGraphBIFFRecord(data, 0x101E, tick)

	got := scanGraphLabelSemantics(data)
	if !got.xAxis.seen {
		t.Fatal("category-axis label semantics missing")
	}
	if got.xAxis.majorTickType != 1 || got.xAxis.minorTickType != 2 || got.xAxis.labelPosition != 3 {
		t.Fatalf("axis tick semantics = %+v", got.xAxis)
	}
	if got.xAxis.color != "112233" {
		t.Fatalf("axis label color = %q, want 112233", got.xAxis.color)
	}
}

func TestApplyGraphLabelSemanticsProjectsCanonicalLabels(t *testing.T) {
	chart := &chartpkg.Chart{Series: []chartpkg.Series{{Index: 0}}}
	applyGraphLabelSemantics(chart, graphLabelSemantics{
		series: []graphSeriesLabelStyle{{
			seen: true, showValue: true, showPercent: true, showCategory: true, showBubbleSize: true, smooth: true,
		}},
		xAxis: graphAxisLabelStyle{seen: true, majorTickType: 1, minorTickType: 2, labelPosition: 3, color: "AABBCC"},
	})
	s := chart.Series[0]
	if !s.ShowDataLabels || !s.ShowPercent || !s.ShowCatName || !s.ShowBubbleSize || !s.Smooth {
		t.Fatalf("canonical series label semantics = %+v", s)
	}
	if chart.XAxis.MajorTickType != 1 || chart.XAxis.MinorTickType != 2 || chart.XAxis.LabelPosition != 3 {
		t.Fatalf("canonical X axis tick semantics = %+v", chart.XAxis)
	}
	if chart.XAxis.LabelStyle == nil || chart.XAxis.LabelStyle.Color != "AABBCC" {
		t.Fatalf("canonical X axis label style = %+v", chart.XAxis.LabelStyle)
	}
}

func TestScanGraphAxisSemanticsSeparatesSecondaryAxisParent(t *testing.T) {
	var data []byte
	axisParent := make([]byte, 18)
	binary.LittleEndian.PutUint16(axisParent[:2], 1)
	data = appendGraphBIFFRecord(data, 0x1041, axisParent)
	data = appendGraphBIFFRecord(data, 0x1033, nil)
	axis := make([]byte, 2)
	binary.LittleEndian.PutUint16(axis, 1)
	data = appendGraphBIFFRecord(data, 0x101D, axis)
	vr := make([]byte, 34)
	binary.LittleEndian.PutUint64(vr[0:8], math.Float64bits(10))
	binary.LittleEndian.PutUint64(vr[8:16], math.Float64bits(50))
	binary.LittleEndian.PutUint64(vr[16:24], math.Float64bits(5))
	data = appendGraphBIFFRecord(data, 0x101F, vr)
	data = appendGraphBIFFRecord(data, 0x1034, nil)

	got := scanGraphAxisSemantics(data)
	if got.primary.hasMin || !got.secondary.hasMin || got.secondary.min != 10 || got.secondary.max != 50 || got.secondary.major != 5 {
		t.Fatalf("axis semantics = %+v", got)
	}
}

func TestScanGraphGroupSemanticsBindsSeriesToGroupsAndSecondaryAxis(t *testing.T) {
	var data []byte
	// Primary axis parent + first column group.
	primary := make([]byte, 18)
	binary.LittleEndian.PutUint16(primary[:2], 0)
	data = appendGraphBIFFRecord(data, 0x1041, primary)
	data = appendGraphBIFFRecord(data, 0x1033, nil)
	data = appendGraphBIFFRecord(data, 0x1014, make([]byte, 20))
	data = appendGraphBIFFRecord(data, 0x1033, nil)
	bar := make([]byte, 6)
	data = appendGraphBIFFRecord(data, 0x1017, bar)
	data = appendGraphBIFFRecord(data, 0x1034, nil)
	data = appendGraphBIFFRecord(data, 0x1034, nil)

	// Secondary axis parent + second line group.
	secondary := make([]byte, 18)
	binary.LittleEndian.PutUint16(secondary[:2], 1)
	data = appendGraphBIFFRecord(data, 0x1041, secondary)
	data = appendGraphBIFFRecord(data, 0x1033, nil)
	data = appendGraphBIFFRecord(data, 0x1014, make([]byte, 20))
	data = appendGraphBIFFRecord(data, 0x1033, nil)
	data = appendGraphBIFFRecord(data, 0x1018, nil)
	data = appendGraphBIFFRecord(data, 0x1034, nil)
	data = appendGraphBIFFRecord(data, 0x1034, nil)

	// Series 0 -> group 0, series 1 -> group 1.
	for _, group := range []uint16{0, 1} {
		data = appendGraphBIFFRecord(data, 0x1003, make([]byte, 12))
		data = appendGraphBIFFRecord(data, 0x1033, nil)
		payload := make([]byte, 2)
		binary.LittleEndian.PutUint16(payload, group)
		data = appendGraphBIFFRecord(data, 0x1045, payload)
		data = appendGraphBIFFRecord(data, 0x1034, nil)
	}

	got := scanGraphGroupSemantics(data)
	if len(got.groups) != 2 || got.groups[0].chartType != chartpkg.ChartColumn || got.groups[1].chartType != chartpkg.ChartLine {
		t.Fatalf("groups = %+v", got.groups)
	}
	if got.groups[0].axisParent != 0 || got.groups[1].axisParent != 1 {
		t.Fatalf("axis parents = %d, %d", got.groups[0].axisParent, got.groups[1].axisParent)
	}
	if got.seriesToGroup[0] != 0 || got.seriesToGroup[1] != 1 {
		t.Fatalf("series map = %+v", got.seriesToGroup)
	}
}

func TestApplyGraphGroupSemanticsCreatesCanonicalComboGroups(t *testing.T) {
	chart := &chartpkg.Chart{Series: []chartpkg.Series{{Index: 0}, {Index: 1}}}
	applyGraphGroupSemantics(chart, graphGroupSemantics{
		groups: []graphGroup{
			{chartType: chartpkg.ChartColumn, axisParent: 0, grouping: "clustered", barDir: "col"},
			{chartType: chartpkg.ChartLine, axisParent: 1},
		},
		seriesToGroup: map[int]int{0: 0, 1: 1},
	})
	if len(chart.Groups) != 2 || chart.ChartType != chartpkg.ChartColumn {
		t.Fatalf("canonical groups = %+v, type=%v", chart.Groups, chart.ChartType)
	}
	if chart.Series[0].ChartType != chartpkg.ChartColumn || chart.Series[1].ChartType != chartpkg.ChartLine {
		t.Fatalf("series types = %v, %v", chart.Series[0].ChartType, chart.Series[1].ChartType)
	}
	if chart.Series[0].UsesSecondaryValueAxis() || !chart.Series[1].UsesSecondaryValueAxis() {
		t.Fatalf("secondary-axis binding = %v, %v", chart.Series[0].UsesSecondaryValueAxis(), chart.Series[1].UsesSecondaryValueAxis())
	}
}
