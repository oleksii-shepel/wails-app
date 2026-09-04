package chart

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
)

func TestParseChartBarClustered(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:barChart>
        <c:barDir val="col"/>
        <c:grouping val="clustered"/>
        <c:ser>
          <c:idx val="0"/>
          <c:order val="0"/>
          <c:tx><c:strRef><c:f>Sheet1!$A$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>Sales</c:v></c:pt></c:strCache></c:strRef></c:tx>
          <c:cat><c:strRef><c:f>Sheet1!$B$1:$D$1</c:f><c:strCache><c:ptCount val="3"/><c:pt idx="0"><c:v>Q1</c:v></c:pt><c:pt idx="1"><c:v>Q2</c:v></c:pt><c:pt idx="2"><c:v>Q3</c:v></c:pt></c:strCache></c:strRef></c:cat>
          <c:val><c:numRef><c:f>Sheet1!$B$2:$D$2</c:f><c:numCache><c:ptCount val="3"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt><c:pt idx="2"><c:v>30</c:v></c:pt></c:numCache></c:numRef></c:val>
        </c:ser>
        <c:axId val="1"/>
        <c:axId val="2"/>
      </c:barChart>
      <c:catAx><c:axId val="1"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:crossAx val="2"/></c:catAx>
      <c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:crossAx val="1"/></c:valAx>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.ChartType != ChartColumn {
		t.Fatalf("type = %v, want ChartColumn", c.ChartType)
	}
	if c.Stacked {
		t.Fatal("unexpected stacked")
	}
	if len(c.Series) != 1 {
		t.Fatalf("series = %d, want 1", len(c.Series))
	}
	if c.Series[0].Name != "Sales" {
		t.Fatalf("name = %q, want Sales", c.Series[0].Name)
	}
	if len(c.Series[0].Values) != 3 || c.Series[0].Values[0] != 10 {
		t.Fatalf("values = %v", c.Series[0].Values)
	}
	if len(c.Categories) != 3 || c.Categories[0] != "Q1" {
		t.Fatalf("categories = %v", c.Categories)
	}
}

func TestParseChartBarStacked(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:grouping val="stacked"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !c.Stacked {
		t.Fatal("expected stacked")
	}
	if c.PercentStacked {
		t.Fatal("unexpected percent-stacked")
	}
}

func TestParseChartBarPercentStacked(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:grouping val="percentStacked"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !c.Stacked {
		t.Fatal("expected stacked")
	}
	if !c.PercentStacked {
		t.Fatal("expected percent-stacked")
	}
}

func TestParseChartCombo(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:areaChart><c:ser><c:idx val="0"/><c:tx><c:strRef><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>Area</c:v></c:pt></c:strCache></c:strRef></c:tx><c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>1</c:v></c:pt><c:pt idx="1"><c:v>2</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:areaChart>
    <c:lineChart><c:ser><c:idx val="1"/><c:tx><c:strRef><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>Line</c:v></c:pt></c:strCache></c:strRef></c:tx><c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>3</c:v></c:pt><c:pt idx="1"><c:v>4</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:lineChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Series) != 2 {
		t.Fatalf("series = %d, want 2", len(c.Series))
	}
	if c.Series[0].ChartType != ChartArea {
		t.Fatalf("series[0] type = %v, want ChartArea", c.Series[0].ChartType)
	}
	if c.Series[1].ChartType != ChartLine {
		t.Fatalf("series[1] type = %v, want ChartLine", c.Series[1].ChartType)
	}
	if c.Series[0].Name != "Area" {
		t.Fatalf("series[0] name = %q", c.Series[0].Name)
	}
	if c.Series[1].Name != "Line" {
		t.Fatalf("series[1] name = %q", c.Series[1].Name)
	}
}

func TestParseChartScatterWithXValues(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:scatterChart><c:ser>
      <c:idx val="0"/>
      <c:xVal><c:numRef><c:numCache><c:ptCount val="3"/><c:pt idx="0"><c:v>1</c:v></c:pt><c:pt idx="1"><c:v>2</c:v></c:pt><c:pt idx="2"><c:v>3</c:v></c:pt></c:numCache></c:numRef></c:xVal>
      <c:val><c:numRef><c:numCache><c:ptCount val="3"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt><c:pt idx="2"><c:v>30</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:scatterChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.ChartType != ChartScatter {
		t.Fatalf("type = %v, want ChartScatter", c.ChartType)
	}
	if len(c.Series[0].XValues) != 3 || c.Series[0].XValues[0] != 1 {
		t.Fatalf("xvalues = %v", c.Series[0].XValues)
	}
	if len(c.Series[0].Values) != 3 || c.Series[0].Values[0] != 10 {
		t.Fatalf("values = %v", c.Series[0].Values)
	}
}

func TestParseChartBubble(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:bubbleChart><c:ser>
      <c:idx val="0"/>
      <c:xVal><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>1</c:v></c:pt><c:pt idx="1"><c:v>2</c:v></c:pt></c:numCache></c:numRef></c:xVal>
      <c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val>
      <c:bubbleSize><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>5</c:v></c:pt><c:pt idx="1"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:bubbleSize>
    </c:ser></c:bubbleChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.ChartType != ChartBubble {
		t.Fatalf("type = %v, want ChartBubble", c.ChartType)
	}
	if len(c.Series[0].BubbleSizes) != 2 || c.Series[0].BubbleSizes[0] != 5 {
		t.Fatalf("bubbleSizes = %v", c.Series[0].BubbleSizes)
	}
}

func TestParseChartDoughnut(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:doughnutChart><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>30</c:v></c:pt><c:pt idx="1"><c:v>70</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:doughnutChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.ChartType != ChartDoughnut {
		t.Fatalf("type = %v, want ChartDoughnut", c.ChartType)
	}
}

func TestParseChartRadar(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:radarChart><c:radarStyle val="filled"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:radarChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.ChartType != ChartRadar {
		t.Fatalf("type = %v, want ChartRadar", c.ChartType)
	}
}

func TestParseChartStock(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:stockChart><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:stockChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.ChartType != ChartStock {
		t.Fatalf("type = %v, want ChartStock", c.ChartType)
	}
}

func TestParseChartSurface(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:surfaceChart><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:surfaceChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.ChartType != ChartSurface {
		t.Fatalf("type = %v, want ChartSurface", c.ChartType)
	}
}

func TestParseChartAxisTitle(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
    <c:catAx><c:axId val="1"/><c:title><c:tx><c:rich><a:p xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:r><a:t>Category Axis</a:t></a:r></a:p></c:rich></c:tx></c:title></c:catAx>
    <c:valAx><c:axId val="2"/><c:title><c:tx><c:rich><a:p xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:r><a:t>Value Axis</a:t></a:r></a:p></c:rich></c:tx></c:title></c:valAx>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.XAxis.Title != "Category Axis" {
		t.Fatalf("xaxis title = %q, want Category Axis", c.XAxis.Title)
	}
	if c.YAxis.Title != "Value Axis" {
		t.Fatalf("yaxis title = %q, want Value Axis", c.YAxis.Title)
	}
}

func TestParseChartAxisMinMax(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
    <c:valAx><c:axId val="2"/><c:scaling><c:min val="5"/><c:max val="50"/></c:scaling></c:valAx>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.YAxis.Min != 5 {
		t.Fatalf("ymin = %v, want 5", c.YAxis.Min)
	}
	if c.YAxis.Max != 50 {
		t.Fatalf("ymax = %v, want 50", c.YAxis.Max)
	}
}

func TestParseChartSeriesColors(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser>
      <c:idx val="0"/>
      <c:spPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill><a:ln><a:solidFill><a:srgbClr val="00FF00"/></a:solidFill></a:ln></c:spPr>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:barChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Series[0].FillColor != "FF0000" {
		t.Fatalf("fillColor = %q, want FF0000", c.Series[0].FillColor)
	}
	if c.Series[0].LineColor != "00FF00" {
		t.Fatalf("lineColor = %q, want 00FF00", c.Series[0].LineColor)
	}
}

func TestParseChartSchemeColors(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser>
      <c:idx val="0"/>
      <c:spPr><a:solidFill><a:schemeClr val="accent1"/></a:solidFill></c:spPr>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:barChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Series[0].FillColor != "scheme:accent1" {
		t.Fatalf("fillColor = %q, want scheme:accent1", c.Series[0].FillColor)
	}
	if c.Series[0].fillRef == nil || c.Series[0].fillRef.ThemeColor != "accent1" {
		t.Fatalf("fillRef = %#v, want theme accent1", c.Series[0].fillRef)
	}
}

func TestParseChartSchemeColorTransforms(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser>
      <c:idx val="0"/>
      <c:spPr><a:solidFill><a:schemeClr val="accent1"><a:lumMod val="50000"/><a:lumOff val="25000"/></a:schemeClr></a:solidFill></c:spPr>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:barChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Series[0].FillColor != "scheme:accent1" {
		t.Fatalf("fillColor = %q, want scheme:accent1", c.Series[0].FillColor)
	}
	if c.Series[0].fillRef == nil {
		t.Fatal("expected fillRef")
	}
	if c.Series[0].fillRef.ThemeColor != "accent1" || c.Series[0].fillRef.LumMod != 0.5 || c.Series[0].fillRef.LumOff != 0.25 {
		t.Fatalf("fillRef = %#v, want accent1 with lumMod/lumOff", c.Series[0].fillRef)
	}
}

func TestParseChartSmooth(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:lineChart><c:ser>
      <c:idx val="0"/>
      <c:smooth val="1"/>
      <c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:lineChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !c.Series[0].Smooth {
		t.Fatal("expected smooth")
	}
}

func TestParseChartMarkerSymbol(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:lineChart><c:ser>
      <c:idx val="0"/>
      <c:marker><c:symbol val="diamond"/></c:marker>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:lineChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Series[0].MarkerShape != "diamond" {
		t.Fatalf("markerShape = %q, want diamond", c.Series[0].MarkerShape)
	}
}

func TestParseChartDataLabels(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser>
      <c:idx val="0"/>
      <c:dLbls><c:showVal val="1"/><c:showCatName val="1"/><c:showSerName val="1"/><c:showPercent val="1"/></c:dLbls>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:barChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !c.Series[0].ShowDataLabels {
		t.Fatal("expected ShowDataLabels")
	}
	if !c.Series[0].ShowCatName {
		t.Fatal("expected ShowCatName")
	}
	if !c.Series[0].ShowSerName {
		t.Fatal("expected ShowSerName")
	}
	if !c.Series[0].ShowPercent {
		t.Fatal("expected ShowPercent")
	}
}

func TestParseChartPieExplosion(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:pieChart><c:ser>
      <c:idx val="0"/>
      <c:explosion val="15"/>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:pieChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Series[0].Explosion != 15 {
		t.Fatalf("explosion = %v, want 15", c.Series[0].Explosion)
	}
}

func TestParseChartLineWidth(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart><c:plotArea>
    <c:lineChart><c:ser>
      <c:idx val="0"/>
      <c:spPr><a:ln w="25400"><a:solidFill><a:srgbClr val="000000"/></a:solidFill></a:ln></c:spPr>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:lineChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Series[0].LineWidth != 25400 {
		t.Fatalf("lineWidth = %v, want 25400", c.Series[0].LineWidth)
	}
}

func TestParseChartLegend(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
  </c:plotArea><c:legend><c:legendPos val="r"/></c:legend></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !c.Legend.Show {
		t.Fatal("expected legend show")
	}
	if c.Legend.Position != "r" {
		t.Fatalf("legend pos = %q, want r", c.Legend.Position)
	}
}

func TestParseChartChartAreaBackground(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:spPr><a:solidFill><a:srgbClr val="F2F2F2"/></a:solidFill></c:spPr>
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.ChartArea.FillColor != "F2F2F2" {
		t.Fatalf("chartArea fill = %q, want F2F2F2", c.ChartArea.FillColor)
	}
}

func TestParseChartChartAreaBackgroundFromChartNode(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart>
    <c:spPr><a:solidFill><a:srgbClr val="EAEAEA"/></a:solidFill></c:spPr>
    <c:plotArea>
      <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.ChartArea.FillColor != "EAEAEA" {
		t.Fatalf("chartArea fill = %q, want EAEAEA", c.ChartArea.FillColor)
	}
}

func TestParseChartPlotAreaStyle(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart>
    <c:plotArea>
      <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
      <c:spPr>
        <a:solidFill><a:srgbClr val="FFF9E6"/></a:solidFill>
        <a:ln><a:solidFill><a:srgbClr val="333333"/></a:solidFill></a:ln>
      </c:spPr>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.PlotArea.FillColor != "FFF9E6" {
		t.Fatalf("plotArea fill = %q, want FFF9E6", c.PlotArea.FillColor)
	}
	if !c.PlotArea.ShowBorder {
		t.Fatal("expected plotArea border")
	}
	if c.PlotArea.BorderColor != "333333" {
		t.Fatalf("plotArea border = %q, want 333333", c.PlotArea.BorderColor)
	}
}

func TestParseChartSecondaryAxis(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
    <c:valAx><c:axId val="1"/><c:scaling><c:min val="0"/></c:scaling></c:valAx>
    <c:valAx><c:axId val="2"/><c:scaling><c:min val="100"/></c:scaling></c:valAx>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.YAxis.Min != 0 {
		t.Fatalf("primary ymin = %v, want 0", c.YAxis.Min)
	}
	if c.YAxis2.Min != 100 {
		t.Fatalf("secondary ymin = %v, want 100", c.YAxis2.Min)
	}
}

func TestParseChartMarksSeriesOnSecondaryAxis(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:barChart>
      <c:barDir val="col"/>
      <c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser>
      <c:axId val="1"/><c:axId val="2"/>
    </c:barChart>
    <c:lineChart>
      <c:ser><c:idx val="1"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>100</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser>
      <c:axId val="1"/><c:axId val="3"/>
    </c:lineChart>
    <c:catAx><c:axId val="1"/></c:catAx>
    <c:valAx><c:axId val="2"/></c:valAx>
    <c:valAx><c:axId val="3"/></c:valAx>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Series) != 2 {
		t.Fatalf("series = %d, want 2", len(c.Series))
	}
	if c.Series[0].secondaryValueAxis {
		t.Fatal("primary series unexpectedly marked as secondary")
	}
	if !c.Series[1].secondaryValueAxis {
		t.Fatal("expected line series to use secondary axis")
	}
}

func TestParseChartTitle(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart>
    <c:title><c:tx><c:rich><a:p><a:r><a:t>My Chart Title</a:t></a:r></a:p></c:rich></c:tx></c:title>
    <c:plotArea>
      <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Title != "My Chart Title" {
		t.Fatalf("title = %q, want My Chart Title", c.Title)
	}
}

func TestParseChartGridlines(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
    <c:valAx><c:axId val="2"/><c:majorGridlines><c:spPr><a:ln><a:solidFill><a:srgbClr val="CCCCCC"/></a:solidFill></a:ln></c:spPr></c:majorGridlines></c:valAx>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !c.YAxis.ShowGridlines {
		t.Fatal("expected ShowGridlines")
	}
	if c.YAxis.GridlineColor != "CCCCCC" {
		t.Fatalf("gridlineColor = %q, want CCCCCC", c.YAxis.GridlineColor)
	}
}

func TestParseChartFillAlpha(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser>
      <c:idx val="0"/>
      <c:spPr><a:solidFill><a:srgbClr val="FF0000"><a:alpha val="5000"/></a:srgbClr></a:solidFill></c:spPr>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:barChart>
  </c:plotArea></c:chart>
</c:chartSpace>`

	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Series[0].FillAlpha != 0.05 {
		t.Fatalf("fillAlpha = %v, want 0.05", c.Series[0].FillAlpha)
	}
}

func TestHexColor(t *testing.T) {
	tests := []struct {
		index int
		want  string
	}{
		{0, "4472C4"},
		{1, "ED7D31"},
		{9, "997300"},
		{10, "4472C4"}, // wraps around
	}
	for _, tt := range tests {
		got := HexColor(tt.index)
		if got != tt.want {
			t.Errorf("HexColor(%d) = %q, want %q", tt.index, got, tt.want)
		}
	}
}

func TestChartTypeString(t *testing.T) {
	tests := []struct {
		ct   ChartType
		want string
	}{
		{ChartBar, "bar"},
		{ChartColumn, "column"},
		{ChartLine, "line"},
		{ChartPie, "pie"},
		{ChartArea, "area"},
		{ChartScatter, "scatter"},
		{ChartDoughnut, "doughnut"},
		{ChartBubble, "bubble"},
		{ChartStock, "stock"},
		{ChartRadar, "radar"},
		{ChartSurface, "surface"},
		{ChartUnknown, "unknown"},
	}
	for _, tt := range tests {
		got := tt.ct.String()
		if got != tt.want {
			t.Errorf("%v.String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestParseChartType(t *testing.T) {
	tests := []struct {
		s    string
		want ChartType
	}{
		{"bar", ChartBar},
		{"column", ChartColumn},
		{"line", ChartLine},
		{"pie", ChartPie},
		{"area", ChartArea},
		{"scatter", ChartScatter},
		{"doughnut", ChartDoughnut},
		{"bubble", ChartBubble},
		{"stock", ChartStock},
		{"radar", ChartRadar},
		{"surface", ChartSurface},
		{"unknown", ChartUnknown},
	}
	for _, tt := range tests {
		got := ParseChartType(tt.s)
		if got != tt.want {
			t.Errorf("ParseChartType(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestResolveColor(t *testing.T) {
	c := &Chart{
		ThemeColors: map[string]string{"accent1": "AABBCC"},
	}
	tests := []struct {
		input string
		want  string
	}{
		{"FF0000", "FF0000"},
		{"scheme:accent1", "AABBCC"},
		{"scheme:accent2", "ED7D31"}, // fallback to default
	}
	for _, tt := range tests {
		got := c.resolveColor(tt.input)
		if got != tt.want {
			t.Errorf("resolveColor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestChartMaxMinValue(t *testing.T) {
	c := &Chart{
		Series: []Series{{
			Values: []float64{5, 15, 10},
		}},
	}
	if c.MaxValue() != 15 {
		t.Fatalf("MaxValue = %v, want 15", c.MaxValue())
	}
	if c.MinValue() != 5 {
		t.Fatalf("MinValue = %v, want 5", c.MinValue())
	}
}

func TestAxisTickStep(t *testing.T) {
	c := &Chart{}
	tests := []struct {
		maxVal float64
		want   float64
	}{
		{7, 2},
		{30, 10},
		{250, 50},
		{500, 100},
		{5000, 1000},
		{100, 20},
	}
	for _, tt := range tests {
		got := c.axisTickStep(tt.maxVal)
		if got != tt.want {
			t.Errorf("axisTickStep(%v) = %v, want %v", tt.maxVal, got, tt.want)
		}
	}
}

func TestSmoothPath(t *testing.T) {
	coords := [][2]int{{0, 0}, {50, 50}, {100, 0}}
	path := smoothPath(coords)
	if path == "" {
		t.Fatal("empty smooth path")
	}
	if !strings.Contains(path, "C") {
		t.Fatal("expected cubic bezier command")
	}
}

func TestRenderMarkerNone(t *testing.T) {
	var b strings.Builder
	c := &Chart{}
	c.renderMarker(&b, 10, 10, "none", "FF0000", 0)
	if b.String() != "" {
		t.Fatal("expected no output for none marker")
	}
}

func TestRenderMarkerShapes(t *testing.T) {
	shapes := []string{"circle", "diamond", "square", "triangle", "x", "star", "plus"}
	for _, shape := range shapes {
		var b strings.Builder
		c := &Chart{}
		c.renderMarker(&b, 10, 10, shape, "FF0000", 0)
		if b.String() == "" {
			t.Fatalf("shape %q: empty output", shape)
		}
	}
}

func TestParseChart3DColumn(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:bar3DChart>
        <c:barDir val="col"/>
        <c:ser><c:val><c:numRef><c:f>Sheet1!B2:B4</c:f></c:numRef></c:val></c:ser>
        <c:axId val="1"/><c:axId val="2"/>
      </c:bar3DChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`)
	chart, err := ParseChart(data)
	if err != nil {
		t.Fatalf("ParseChart: %v", err)
	}
	if chart.ChartType != ChartColumn3D {
		t.Errorf("ChartType = %v, want ChartColumn3D", chart.ChartType)
	}
	if len(chart.Series) != 1 {
		t.Errorf("len(Series) = %d, want 1", len(chart.Series))
	}
}

func TestParseChart3DPie(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:pie3DChart>
        <c:ser><c:val><c:numRef><c:f>Sheet1!B2:B4</c:f></c:numRef></c:val></c:ser>
      </c:pie3DChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`)
	chart, err := ParseChart(data)
	if err != nil {
		t.Fatalf("ParseChart: %v", err)
	}
	if chart.ChartType != ChartPie3D {
		t.Errorf("ChartType = %v, want ChartPie3D", chart.ChartType)
	}
}

func TestParseChartView3D(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:view3D>
      <c:rotX val="15"/>
      <c:rotY val="20"/>
      <c:perspective val="30"/>
      <c:depthPercent val="100"/>
      <c:rAngAx val="1"/>
    </c:view3D>
    <c:plotArea>
      <c:bar3DChart>
        <c:barDir val="col"/>
        <c:ser><c:val><c:numRef><c:f>Sheet1!B2:B4</c:f></c:numRef></c:val></c:ser>
        <c:axId val="1"/><c:axId val="2"/>
      </c:bar3DChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`)
	chart, err := ParseChart(data)
	if err != nil {
		t.Fatalf("ParseChart: %v", err)
	}
	if chart.View3D == nil {
		t.Fatal("expected View3D, got nil")
	}
	if chart.View3D.RotX != 15 {
		t.Errorf("View3D.RotX = %d, want 15", chart.View3D.RotX)
	}
	if chart.View3D.RotY != 20 {
		t.Errorf("View3D.RotY = %d, want 20", chart.View3D.RotY)
	}
	if chart.View3D.Perspective != 30 {
		t.Errorf("View3D.Perspective = %d, want 30", chart.View3D.Perspective)
	}
	if chart.View3D.DepthPercent != 100 {
		t.Errorf("View3D.DepthPercent = %d, want 100", chart.View3D.DepthPercent)
	}
	if !chart.View3D.RightAngleAxes {
		t.Errorf("View3D.RightAngleAxes = %v, want true", chart.View3D.RightAngleAxes)
	}
}

func TestParseChart3DDefaultsToBarWhenNoDir(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:bar3DChart>
        <c:ser><c:val><c:numRef><c:f>Sheet1!B2:B4</c:f></c:numRef></c:val></c:ser>
        <c:axId val="1"/><c:axId val="2"/>
      </c:bar3DChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`)
	chart, err := ParseChart(data)
	if err != nil {
		t.Fatalf("ParseChart: %v", err)
	}
	if chart.ChartType != ChartBar3D {
		t.Errorf("ChartType = %v, want ChartBar3D", chart.ChartType)
	}
}

func TestChart3DRenderNative(t *testing.T) {
	// Ensure 3D column chart renders native 3D SVG (polygons, not 2D rects)
	chart := &Chart{
		ChartType: ChartColumn3D,
		Series: []Series{{
			Values: []float64{10, 20, 30},
		}},
		Categories: []string{"A", "B", "C"},
	}
	svg := chart.SVG(400, 300)
	if !strings.HasPrefix(svg, "<svg") {
		t.Errorf("expected SVG output, got: %s", svg[:min(len(svg), 100)])
	}
	if !strings.Contains(svg, "<polygon") {
		t.Errorf("expected 3D polygon faces in native SVG")
	}
}

func TestChartTypeString3D(t *testing.T) {
	tests := []struct {
		ct   ChartType
		want string
	}{
		{ChartBar3D, "bar3d"},
		{ChartColumn3D, "column3d"},
		{ChartLine3D, "line3d"},
		{ChartPie3D, "pie3d"},
		{ChartArea3D, "area3d"},
		{ChartSurface3D, "surface3d"},
	}
	for _, tt := range tests {
		got := tt.ct.String()
		if got != tt.want {
			t.Errorf("%v.String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestParseChartErrorBars(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:lineChart>
      <c:ser>
        <c:idx val="0"/>
        <c:order val="0"/>
        <c:val><c:numRef><c:numCache><c:ptCount val="3"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt><c:pt idx="2"><c:v>30</c:v></c:pt></c:numCache></c:numRef></c:val>
        <c:errBars>
          <c:errDir val="y"/>
          <c:errBarType val="both"/>
          <c:errValType val="fixedVal"/>
          <c:val val="5"/>
          <c:noEndCap val="1"/>
        </c:errBars>
      </c:ser>
    </c:lineChart>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Series) != 1 {
		t.Fatalf("series = %d, want 1", len(c.Series))
	}
	eb := c.Series[0].ErrorBars
	if eb == nil {
		t.Fatal("expected error bars")
	}
	if eb.Direction != "y" {
		t.Fatalf("direction = %q, want y", eb.Direction)
	}
	if eb.BarType != "both" {
		t.Fatalf("barType = %q, want both", eb.BarType)
	}
	if eb.ValType != "fixedVal" {
		t.Fatalf("valType = %q, want fixedVal", eb.ValType)
	}
	if eb.Value != 5 {
		t.Fatalf("value = %v, want 5", eb.Value)
	}
	if !eb.NoEndCap {
		t.Fatal("expected noEndCap")
	}
}

func TestParseChartErrorBarsCustom(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:lineChart>
      <c:ser>
        <c:idx val="0"/>
        <c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val>
        <c:errBars>
          <c:errValType val="cust"/>
          <c:plus><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>1</c:v></c:pt><c:pt idx="1"><c:v>2</c:v></c:pt></c:numCache></c:numRef></c:plus>
          <c:minus><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>3</c:v></c:pt><c:pt idx="1"><c:v>4</c:v></c:pt></c:numCache></c:numRef></c:minus>
        </c:errBars>
      </c:ser>
    </c:lineChart>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	eb := c.Series[0].ErrorBars
	if eb == nil {
		t.Fatal("expected error bars")
	}
	if eb.ValType != "cust" {
		t.Fatalf("valType = %q, want cust", eb.ValType)
	}
	if len(eb.Plus) != 2 || eb.Plus[0] != 1 || eb.Plus[1] != 2 {
		t.Fatalf("plus = %v", eb.Plus)
	}
	if len(eb.Minus) != 2 || eb.Minus[0] != 3 || eb.Minus[1] != 4 {
		t.Fatalf("minus = %v", eb.Minus)
	}
}

func TestParseChartDataTable(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/></c:ser></c:barChart>
    <c:dTable>
      <c:showHorzBorder val="1"/>
      <c:showVertBorder val="1"/>
      <c:showOutline val="1"/>
      <c:showKeys val="1"/>
    </c:dTable>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.DataTable == nil {
		t.Fatal("expected data table")
	}
	if !c.DataTable.ShowHorzBorder {
		t.Fatal("expected showHorzBorder")
	}
	if !c.DataTable.ShowVertBorder {
		t.Fatal("expected showVertBorder")
	}
	if !c.DataTable.ShowOutline {
		t.Fatal("expected showOutline")
	}
	if !c.DataTable.ShowKeys {
		t.Fatal("expected showKeys")
	}
}

func TestThemeAccentColorResolvesFromThemeColors(t *testing.T) {
	c := &Chart{
		ThemeColors: map[string]string{
			"accent1": "FF0000",
			"accent2": "00FF00",
		},
	}
	if got := c.themeAccentColor(0); got != "FF0000" {
		t.Errorf("themeAccentColor(0) = %q, want FF0000", got)
	}
	if got := c.themeAccentColor(1); got != "00FF00" {
		t.Errorf("themeAccentColor(1) = %q, want 00FF00", got)
	}
	if got := c.themeAccentColor(6); got != "FF0000" {
		t.Errorf("themeAccentColor(6) = %q, want FF0000 (wrap)", got)
	}
}

func TestThemeAccentColorFallsBackToDefault(t *testing.T) {
	c := &Chart{}
	if got := c.themeAccentColor(0); got != defaultThemeColor("accent1") {
		t.Errorf("themeAccentColor(0) = %q, want default accent1", got)
	}
}

func TestSeriesFillColorUsesThemeColors(t *testing.T) {
	c := &Chart{
		ThemeColors: map[string]string{"accent1": "FF0000"},
		Series:      []Series{{Values: []float64{1}}},
	}
	if got := c.seriesFillColor(0); got != "FF0000" {
		t.Errorf("seriesFillColor(0) = %q, want FF0000", got)
	}
}

func TestSeriesFillColorPrefersExplicitColor(t *testing.T) {
	c := &Chart{
		ThemeColors: map[string]string{"accent1": "FF0000"},
		Series:      []Series{{Values: []float64{1}, FillColor: "123456"}},
	}
	if got := c.seriesFillColor(0); got != "123456" {
		t.Errorf("seriesFillColor(0) = %q, want 123456", got)
	}
}

func TestSeriesLineColorUsesThemeColors(t *testing.T) {
	c := &Chart{
		ThemeColors: map[string]string{"accent1": "FF0000"},
		Series:      []Series{{Values: []float64{1}}},
	}
	if got := c.seriesLineColor(0); got != "FF0000" {
		t.Errorf("seriesLineColor(0) = %q, want FF0000", got)
	}
}

func TestSeriesFillColorForRespectsVaryColors(t *testing.T) {
	c := &Chart{
		ThemeColors: map[string]string{
			"accent1": "FF0000",
			"accent2": "00FF00",
		},
		VaryColors: true,
		Series:     []Series{{Values: []float64{1, 2}}},
	}
	if got := c.seriesFillColorFor(0, 0); got != "FF0000" {
		t.Errorf("seriesFillColorFor(0,0) = %q, want FF0000", got)
	}
	if got := c.seriesFillColorFor(0, 1); got != "00FF00" {
		t.Errorf("seriesFillColorFor(0,1) = %q, want 00FF00", got)
	}
}

func TestSeriesFillColorForIgnoresVaryColorsWithMultipleSeries(t *testing.T) {
	c := &Chart{
		ThemeColors: map[string]string{
			"accent1": "FF0000",
			"accent2": "00FF00",
		},
		VaryColors: true,
		Series:     []Series{{Values: []float64{1}}, {Values: []float64{2}}},
	}
	if got := c.seriesFillColorFor(1, 0); got != "00FF00" {
		t.Errorf("seriesFillColorFor(1,0) = %q, want 00FF00", got)
	}
}

func TestDefaultThemeColorCoversAliases(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"tx1", "44546A"},
		{"text1", "44546A"},
		{"bg1", "FFFFFF"},
		{"background1", "FFFFFF"},
		{"tx2", "44546A"},
		{"text2", "44546A"},
		{"bg2", "E7E6E6"},
		{"background2", "E7E6E6"},
	}
	for _, tt := range tests {
		if got := defaultThemeColor(tt.name); got != tt.want {
			t.Errorf("defaultThemeColor(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestResolveColorHandlesSchemePrefix(t *testing.T) {
	c := &Chart{
		ThemeColors: map[string]string{"accent1": "FF0000"},
	}
	if got := c.resolveColor("scheme:accent1"); got != "FF0000" {
		t.Errorf("resolveColor(scheme:accent1) = %q, want FF0000", got)
	}
	if got := c.resolveColor("scheme:missing"); got != "999999" {
		t.Errorf("resolveColor(scheme:missing) = %q, want 999999", got)
	}
	if got := c.resolveColor("123456"); got != "123456" {
		t.Errorf("resolveColor(123456) = %q, want 123456", got)
	}
}

func TestResolveSolidFillAppliesThemeTransforms(t *testing.T) {
	c := &Chart{
		ThemeColors: map[string]string{"accent1": "FF0000"},
	}
	got := c.resolveSolidFill(&drawing.SolidFill{ThemeColor: "accent1", LumMod: 0.5, LumOff: 0.25})
	if got != "BF4040" {
		t.Fatalf("resolveSolidFill(theme transform) = %q, want BF4040", got)
	}
}

func TestParseChartMarkerSize(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:lineChart><c:ser>
      <c:idx val="0"/>
      <c:marker><c:symbol val="diamond"/><c:size val="8"/></c:marker>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:lineChart>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Series[0].MarkerSize != 8 {
		t.Fatalf("markerSize = %v, want 8", c.Series[0].MarkerSize)
	}
}

func TestParseChartMarkerColor(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart><c:plotArea>
    <c:lineChart><c:ser>
      <c:idx val="0"/>
      <c:marker>
        <c:spPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></c:spPr>
      </c:marker>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:lineChart>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Series[0].MarkerColor != "FF0000" {
		t.Fatalf("markerColor = %q, want FF0000", c.Series[0].MarkerColor)
	}
}

func TestParseChartLinePattern(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart><c:plotArea>
    <c:lineChart><c:ser>
      <c:idx val="0"/>
      <c:spPr><a:ln w="12700"><a:prstDash val="dashDot"/><a:solidFill><a:srgbClr val="000000"/></a:solidFill></a:ln></c:spPr>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:lineChart>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Series[0].LinePattern != 3 {
		t.Fatalf("linePattern = %v, want 3", c.Series[0].LinePattern)
	}
}

func TestParseChartTrendlineWithEquation(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:lineChart><c:ser>
      <c:idx val="0"/>
      <c:trendline>
        <c:trendlineType val="linear"/>
        <c:dispEq val="1"/>
        <c:dispRSqr val="1"/>
        <c:order val="2"/>
        <c:forward val="1"/>
        <c:backward val="0.5"/>
      </c:trendline>
      <c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:lineChart>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Series[0].Trendline == nil {
		t.Fatal("expected trendline")
	}
	if c.Series[0].Trendline.Type != "linear" {
		t.Fatalf("type = %q, want linear", c.Series[0].Trendline.Type)
	}
	if !c.Series[0].Trendline.ShowEquation {
		t.Fatal("expected ShowEquation")
	}
	if !c.Series[0].Trendline.ShowRSquared {
		t.Fatal("expected ShowRSquared")
	}
	if c.Series[0].Trendline.Order != 2 {
		t.Fatalf("order = %v, want 2", c.Series[0].Trendline.Order)
	}
	if c.Series[0].Trendline.Forward != 1.0 {
		t.Fatalf("forward = %v, want 1.0", c.Series[0].Trendline.Forward)
	}
	if c.Series[0].Trendline.Backward != 0.5 {
		t.Fatalf("backward = %v, want 0.5", c.Series[0].Trendline.Backward)
	}
}

func TestParseChartDisplayBlanksAs(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
    </c:plotArea>
    <c:displayBlanksAs val="span"/>
  </c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.DisplayBlanksAs != "span" {
		t.Fatalf("displayBlanksAs = %q, want span", c.DisplayBlanksAs)
	}
}

func TestParseChartManualLayout(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:layout>
        <c:manualLayout>
          <c:layoutTarget val="inner"/>
          <c:xMode val="edge"/>
          <c:yMode val="edge"/>
          <c:wMode val="factor"/>
          <c:hMode val="factor"/>
          <c:x val="0.1"/>
          <c:y val="0.2"/>
          <c:w val="0.8"/>
          <c:h val="0.7"/>
        </c:manualLayout>
      </c:layout>
      <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.ManualLayout == nil {
		t.Fatal("expected manualLayout")
	}
	if c.ManualLayout.LayoutTarget != "inner" {
		t.Fatalf("layoutTarget = %q, want inner", c.ManualLayout.LayoutTarget)
	}
	if c.ManualLayout.XMode != "edge" {
		t.Fatalf("xMode = %q, want edge", c.ManualLayout.XMode)
	}
	if c.ManualLayout.YMode != "edge" {
		t.Fatalf("yMode = %q, want edge", c.ManualLayout.YMode)
	}
	if c.ManualLayout.WMode != "factor" {
		t.Fatalf("wMode = %q, want factor", c.ManualLayout.WMode)
	}
	if c.ManualLayout.HMode != "factor" {
		t.Fatalf("hMode = %q, want factor", c.ManualLayout.HMode)
	}
	if c.ManualLayout.X != 0.1 {
		t.Fatalf("x = %v, want 0.1", c.ManualLayout.X)
	}
	if c.ManualLayout.Y != 0.2 {
		t.Fatalf("y = %v, want 0.2", c.ManualLayout.Y)
	}
	if c.ManualLayout.W != 0.8 {
		t.Fatalf("w = %v, want 0.8", c.ManualLayout.W)
	}
	if c.ManualLayout.H != 0.7 {
		t.Fatalf("h = %v, want 0.7", c.ManualLayout.H)
	}
}

func TestParseChartDataPoints(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart><c:plotArea>
    <c:pieChart><c:ser>
      <c:idx val="0"/>
      <c:dPt><c:idx val="0"/><c:spPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></c:spPr></c:dPt>
      <c:dPt><c:idx val="1"/><c:explosion val="15"/></c:dPt>
      <c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>30</c:v></c:pt><c:pt idx="1"><c:v>70</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:pieChart>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Series[0].DataPoints) != 2 {
		t.Fatalf("dataPoints = %d, want 2", len(c.Series[0].DataPoints))
	}
	if c.Series[0].DataPoints[0].FillColor != "FF0000" {
		t.Fatalf("point 0 fill = %q, want FF0000", c.Series[0].DataPoints[0].FillColor)
	}
	if c.Series[0].DataPoints[1].Explosion != 15 {
		t.Fatalf("point 1 explosion = %v, want 15", c.Series[0].DataPoints[1].Explosion)
	}
}

func TestParseScatterStyle(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<chartSpace xmlns="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <chart>
    <plotArea>
      <scatterChart>
        <scatterStyle val="smoothMarker"/>
        <ser><idx val="0"/><order val="0"/><tx><v>Ser1</v></tx><val><numRef><f/></numRef></val></ser>
        <axId val="1"/><axId val="2"/>
      </scatterChart>
      <valAx><axId val="1"/></valAx>
      <valAx><axId val="2"/></valAx>
    </plotArea>
  </chart>
</chartSpace>`)
	c, err := ParseChart(xml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(c.Series))
	}
	if c.Series[0].ScatterStyle != "smoothMarker" {
		t.Errorf("scatterStyle = %q, want smoothMarker", c.Series[0].ScatterStyle)
	}
}

func TestParseRadarStyle(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<chartSpace xmlns="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <chart>
    <plotArea>
      <radarChart>
        <radarStyle val="filled"/>
        <ser><idx val="0"/><order val="0"/><tx><v>Ser1</v></tx><val><numRef><f/></numRef></val></ser>
        <axId val="1"/><axId val="2"/>
      </radarChart>
      <catAx><axId val="1"/></catAx>
      <valAx><axId val="2"/></valAx>
    </plotArea>
  </chart>
</chartSpace>`)
	c, err := ParseChart(xml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Groups) != 1 || c.Groups[0].Grouping != "filled" {
		t.Errorf("radar group style = %q, want filled", c.Groups[0].Grouping)
	}
}

func TestParseDataLabelNumFmt(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<chartSpace xmlns="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <chart>
    <plotArea>
      <barChart>
        <barDir val="col"/>
        <ser>
          <idx val="0"/><order val="0"/>
          <tx><v>Ser1</v></tx>
          <val><numRef><f/></numRef></val>
          <dLbls>
            <showVal val="1"/>
            <numFmt formatCode="0.00" sourceLinked="0"/>
            <dLbl>
              <idx val="1"/>
              <numFmt formatCode="0.000" sourceLinked="0"/>
            </dLbl>
          </dLbls>
        </ser>
        <axId val="1"/><axId val="2"/>
      </barChart>
      <catAx><axId val="1"/></catAx>
      <valAx><axId val="2"/></valAx>
    </plotArea>
  </chart>
</chartSpace>`)
	c, err := ParseChart(xml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(c.Series))
	}
	s := c.Series[0]
	if s.NumFmt != "0.00" {
		t.Errorf("series numFmt = %q, want 0.00", s.NumFmt)
	}
	if len(s.DataPoints) < 2 || s.DataPoints[1].NumFmt != "0.000" {
		t.Errorf("dataPoint[1] numFmt = %q, want 0.000", s.DataPoints[1].NumFmt)
	}
}

func TestParseDataLabelSurface(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<chartSpace xmlns="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <chart>
    <plotArea>
      <barChart>
        <barDir val="col"/>
        <ser>
          <idx val="0"/><order val="0"/>
          <tx><v>Ser1</v></tx>
          <val><numRef><f/></numRef></val>
          <dLbls>
            <showVal val="1"/>
            <showCatName val="1"/>
            <separator val=" | "/>
            <dLblPos val="outEnd"/>
            <showBubbleSize val="1"/>
            <dLbl>
              <idx val="0"/>
              <tx><rich><p><r><t>Custom A</t></r></p></rich></tx>
            </dLbl>
          </dLbls>
        </ser>
        <axId val="1"/><axId val="2"/>
      </barChart>
      <catAx><axId val="1"/></catAx>
      <valAx><axId val="2"/></valAx>
    </plotArea>
  </chart>
</chartSpace>`)
	c, err := ParseChart(xml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(c.Series))
	}
	s := c.Series[0]
	if !s.ShowDataLabels {
		t.Error("expected ShowDataLabels")
	}
	if !s.ShowCatName {
		t.Error("expected ShowCatName")
	}
	if !s.ShowBubbleSize {
		t.Error("expected ShowBubbleSize")
	}
	if s.Separator != " | " {
		t.Errorf("separator = %q, want %q", s.Separator, " | ")
	}
	if s.LabelPosition != 1 {
		t.Errorf("labelPosition = %d, want 1 (outEnd)", s.LabelPosition)
	}
	if len(s.DataPoints) == 0 || s.DataPoints[0].Label != "Custom A" {
		t.Errorf("custom label = %q, want %q", s.DataPoints[0].Label, "Custom A")
	}
}

func TestParseComboChartGroups(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<chartSpace xmlns="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <chart>
    <plotArea>
      <barChart>
        <barDir val="col"/>
        <grouping val="clustered"/>
        <gapWidth val="200"/>
        <ser><idx val="0"/><order val="0"/><tx><v>BarSer</v></tx><val><numRef><f/></numRef></val></ser>
        <axId val="1"/><axId val="2"/>
      </barChart>
      <lineChart>
        <grouping val="standard"/>
        <ser><idx val="1"/><order val="1"/><tx><v>LineSer</v></tx><val><numRef><f/></numRef></val></ser>
        <axId val="1"/><axId val="3"/>
      </lineChart>
      <catAx><axId val="1"/></catAx>
      <valAx><axId val="2"/></valAx>
      <valAx><axId val="3"/></valAx>
    </plotArea>
  </chart>
</chartSpace>`)
	c, err := ParseChart(xml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(c.Groups))
	}
	if c.Groups[0].ChartType != ChartColumn {
		t.Errorf("group0 type: got %v, want ChartColumn", c.Groups[0].ChartType)
	}
	if c.Groups[1].ChartType != ChartLine {
		t.Errorf("group1 type: got %v, want ChartLine", c.Groups[1].ChartType)
	}
	if len(c.Groups[0].SeriesIndices) != 1 || c.Groups[0].SeriesIndices[0] != 0 {
		t.Errorf("group0 series indices: %v", c.Groups[0].SeriesIndices)
	}
	if len(c.Groups[1].SeriesIndices) != 1 || c.Groups[1].SeriesIndices[0] != 1 {
		t.Errorf("group1 series indices: %v", c.Groups[1].SeriesIndices)
	}
	if c.Groups[0].GapWidth != 200 {
		t.Errorf("group0 gapWidth: got %d, want 200", c.Groups[0].GapWidth)
	}
	if len(c.Groups[0].AxisIDs) != 2 || c.Groups[0].AxisIDs[0] != 1 || c.Groups[0].AxisIDs[1] != 2 {
		t.Errorf("group0 axisIDs: %v", c.Groups[0].AxisIDs)
	}
	if len(c.Groups[1].AxisIDs) != 2 || c.Groups[1].AxisIDs[0] != 1 || c.Groups[1].AxisIDs[1] != 3 {
		t.Errorf("group1 axisIDs: %v", c.Groups[1].AxisIDs)
	}
	// Backward compat: series should still have per-series ChartType.
	if len(c.Series) != 2 {
		t.Fatalf("expected 2 series, got %d", len(c.Series))
	}
	if c.Series[0].ChartType != ChartColumn {
		t.Errorf("series0 type: got %v, want ChartColumn", c.Series[0].ChartType)
	}
	if c.Series[1].ChartType != ChartLine {
		t.Errorf("series1 type: got %v, want ChartLine", c.Series[1].ChartType)
	}
}

func TestParseChartLeaderLines(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:pieChart><c:ser>
      <c:idx val="0"/>
      <c:dLbls><c:showVal val="1"/><c:showLeaderLines val="1"/></c:dLbls>
      <c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val>
    </c:ser></c:pieChart>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !c.Series[0].ShowLeaderLines {
		t.Fatal("expected ShowLeaderLines")
	}
}

func TestParseChartStockConfig(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart><c:plotArea>
    <c:stockChart>
      <c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser>
      <c:dropLines/><c:hiLowLines/>
      <c:upDownBars>
        <c:upBars><a:solidFill><a:srgbClr val="00FF00"/></a:solidFill></c:upBars>
        <c:downBars><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></c:downBars>
      </c:upDownBars>
      <c:axId val="1"/><c:axId val="2"/>
    </c:stockChart>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.StockConfig == nil {
		t.Fatal("expected stockConfig")
	}
	if !c.StockConfig.ShowDropLines {
		t.Fatal("expected ShowDropLines")
	}
	if !c.StockConfig.ShowHiLowLines {
		t.Fatal("expected ShowHiLowLines")
	}
	if c.StockConfig.UpBarColor != "00FF00" {
		t.Fatalf("upBarColor = %q, want 00FF00", c.StockConfig.UpBarColor)
	}
	if c.StockConfig.DownBarColor != "FF0000" {
		t.Fatalf("downBarColor = %q, want FF0000", c.StockConfig.DownBarColor)
	}
}

func TestParseChartLegendEntries(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
  </c:plotArea>
  <c:legend>
    <c:legendPos val="b"/>
    <c:legendEntry><c:idx val="0"/><c:delete val="1"/></c:legendEntry>
  </c:legend>
  </c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(c.Legend.Entries) != 1 {
		t.Fatalf("legend entries = %d, want 1", len(c.Legend.Entries))
	}
	if c.Legend.Entries[0].Index != 0 {
		t.Fatalf("entry index = %v, want 0", c.Legend.Entries[0].Index)
	}
	if !c.Legend.Entries[0].Delete {
		t.Fatal("expected entry deleted")
	}
}

func TestParseChartAxisTickMarksAndNumFmt(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
    <c:catAx><c:axId val="1"/><c:scaling><c:orientation val="maxMin"/></c:scaling><c:majorTickMark val="cross"/><c:minorTickMark val="in"/><c:tickLblPos val="low"/><c:crossAx val="2"/><c:numFmt formatCode="0.00" sourceLinked="0"/></c:catAx>
    <c:valAx><c:axId val="2"/><c:scaling><c:min val="5"/><c:max val="50"/></c:scaling><c:majorUnit val="10"/><c:minorUnit val="2"/><c:crossAx val="1"/></c:valAx>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.XAxis.Orientation != "maxMin" {
		t.Fatalf("x orientation = %q, want maxMin", c.XAxis.Orientation)
	}
	if c.XAxis.MajorTickType != 3 {
		t.Fatalf("x majorTick = %v, want 3 (cross)", c.XAxis.MajorTickType)
	}
	if c.XAxis.MinorTickType != 2 {
		t.Fatalf("x minorTick = %v, want 2 (in)", c.XAxis.MinorTickType)
	}
	if c.XAxis.LabelPosition != 1 {
		t.Fatalf("x labelPos = %v, want 1 (low)", c.XAxis.LabelPosition)
	}
	if c.XAxis.NumFmt != "0.00" {
		t.Fatalf("x numFmt = %q, want 0.00", c.XAxis.NumFmt)
	}
	if c.YAxis.Min != 5 {
		t.Fatalf("y min = %v, want 5", c.YAxis.Min)
	}
	if c.YAxis.Max != 50 {
		t.Fatalf("y max = %v, want 50", c.YAxis.Max)
	}
	if c.YAxis.MajorUnit != 10 {
		t.Fatalf("y majorUnit = %v, want 10", c.YAxis.MajorUnit)
	}
	if c.YAxis.MinorUnit != 2 {
		t.Fatalf("y minorUnit = %v, want 2", c.YAxis.MinorUnit)
	}
}

func TestParseChartLegendManualLayout(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:plotArea>
      <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
    </c:plotArea>
    <c:legend>
      <c:legendPos val="r"/>
      <c:layout>
        <c:manualLayout>
          <c:x val="0.5"/>
          <c:y val="0.6"/>
        </c:manualLayout>
      </c:layout>
    </c:legend>
  </c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.LegendLayout == nil {
		t.Fatal("expected legendLayout")
	}
	if c.LegendLayout.X != 0.5 {
		t.Fatalf("legend x = %v, want 0.5", c.LegendLayout.X)
	}
	if c.LegendLayout.Y != 0.6 {
		t.Fatalf("legend y = %v, want 0.6", c.LegendLayout.Y)
	}
}

func TestParseChartTitleManualLayout(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">
  <c:chart>
    <c:title>
      <c:tx><c:rich><a:p xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:r><a:t>My Title</a:t></a:r></a:p></c:rich></c:tx>
      <c:layout>
        <c:manualLayout>
          <c:layoutTarget val="outer"/>
          <c:xMode val="factor"/>
          <c:yMode val="factor"/>
          <c:x val="0.1"/>
          <c:y val="0.05"/>
          <c:w val="0.8"/>
          <c:h val="0.1"/>
        </c:manualLayout>
      </c:layout>
    </c:title>
    <c:plotArea>
      <c:barChart><c:barDir val="col"/><c:ser><c:idx val="0"/><c:val><c:numRef><c:numCache><c:ptCount val="1"/><c:pt idx="0"><c:v>10</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser></c:barChart>
    </c:plotArea>
  </c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Title != "My Title" {
		t.Fatalf("title = %q, want My Title", c.Title)
	}
	if c.TitleLayout == nil {
		t.Fatal("expected titleLayout")
	}
	if c.TitleLayout.LayoutTarget != "outer" {
		t.Fatalf("layoutTarget = %q, want outer", c.TitleLayout.LayoutTarget)
	}
	if c.TitleLayout.X != 0.1 {
		t.Fatalf("x = %v, want 0.1", c.TitleLayout.X)
	}
	if c.TitleLayout.W != 0.8 {
		t.Fatalf("w = %v, want 0.8", c.TitleLayout.W)
	}
}

func TestParseChartDataLabelTextProperties(t *testing.T) {
	xml := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <c:chart><c:plotArea>
    <c:barChart><c:barDir val="col"/><c:ser>
      <c:idx val="0"/>
      <c:val><c:numRef><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val>
      <c:dLbls>
        <c:showVal val="1"/>
        <c:txPr><a:defRPr sz="1200" b="1" i="1"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:defRPr></c:txPr>
        <c:dLbl>
          <c:idx val="1"/>
          <c:tx><c:rich><a:p><a:r><a:t>Custom</a:t></a:r></a:p></c:rich></c:tx>
          <c:txPr><a:defRPr sz="900" b="0" i="0"><a:solidFill><a:srgbClr val="00FF00"/></a:solidFill></a:defRPr></c:txPr>
        </c:dLbl>
      </c:dLbls>
    </c:ser></c:barChart>
  </c:plotArea></c:chart>
</c:chartSpace>`
	c, err := ParseChart([]byte(xml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	s := c.Series[0]
	if s.LabelStyle == nil {
		t.Fatal("expected series LabelStyle")
	}
	if s.LabelStyle.FontSize != 1200 {
		t.Fatalf("series fontSize = %d, want 1200", s.LabelStyle.FontSize)
	}
	if !s.LabelStyle.Bold {
		t.Fatal("expected series bold")
	}
	if !s.LabelStyle.Italic {
		t.Fatal("expected series italic")
	}
	if s.LabelStyle.Color != "FF0000" {
		t.Fatalf("series color = %q, want FF0000", s.LabelStyle.Color)
	}
	if len(s.DataPoints) != 2 {
		t.Fatalf("dataPoints = %d, want 2", len(s.DataPoints))
	}
	dp := s.DataPoints[1]
	if dp.LabelStyle == nil {
		t.Fatal("expected per-point LabelStyle")
	}
	if dp.LabelStyle.FontSize != 900 {
		t.Fatalf("point fontSize = %d, want 900", dp.LabelStyle.FontSize)
	}
	if dp.LabelStyle.Bold {
		t.Fatal("expected point not bold")
	}
	if dp.LabelStyle.Italic {
		t.Fatal("expected point not italic")
	}
	if dp.LabelStyle.Color != "00FF00" {
		t.Fatalf("point color = %q, want 00FF00", dp.LabelStyle.Color)
	}
}
