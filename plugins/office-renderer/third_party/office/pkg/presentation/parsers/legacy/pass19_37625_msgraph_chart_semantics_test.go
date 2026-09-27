package legacy

import (
	"os"
	"path/filepath"
	"testing"

	chartpkg "office/pkg/ooxml/chart"
	presentationir "office/pkg/presentation/ir"
)

func TestPass19NonMSGraphOLEIsNotPromotedToSemanticChart(t *testing.T) {
	if got := extractLegacyPptMSGraphChartData(legacyPptExternalObject{
		Kind:   presentationir.ShapeOleObject,
		ProgID: "Package",
		Data:   []byte("not-an-msgraph-storage"),
	}); len(got) != 0 {
		t.Fatalf("non-MSGraph OLE produced %d bytes of chart data", len(got))
	}
}

func TestPass19_37625MSGraphObjectsOwnSemanticChartData(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "ppt", "37625.ppt")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("37625 fixture is not installed: %v", err)
	}
	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt: %v", err)
	}
	if len(pres.Slides) < 4 {
		t.Fatalf("slides = %d, want at least 4", len(pres.Slides))
	}
	var charts []*presentationir.Shape
	var walk func([]*presentationir.Shape)
	walk = func(shapes []*presentationir.Shape) {
		for _, shape := range shapes {
			if shape == nil {
				continue
			}
			if shape.Kind == presentationir.ShapeChart && shape.OLEObjectProgID == "MSGraph.Chart.8" {
				charts = append(charts, shape)
			}
			walk(shape.Children)
		}
	}
	walk(pres.Slides[3].Shapes)
	if len(charts) != 2 {
		t.Fatalf("slide 4 semantic MSGraph charts = %d, want 2", len(charts))
	}
	for i, shape := range charts {
		if len(shape.ChartData) == 0 {
			t.Fatalf("chart %d has no canonical ChartData", i+1)
		}
		// A cached preview is optional. Semantic chart ownership must survive even
		// when no preview BLIP is available. When present, it remains fallback data.
		if len(shape.ImageData) > 0 && shape.ImageMIME == "" {
			t.Fatalf("chart %d has preview bytes without MIME", i+1)
		}
		chart, err := chartpkg.ParseChart(shape.ChartData)
		if err != nil || chart == nil {
			t.Fatalf("chart %d canonical data parse: chart=%v err=%v", i+1, chart, err)
		}
		if len(chart.Series) == 0 {
			t.Fatalf("chart %d has no semantic series", i+1)
		}
		if len(chart.Categories) == 0 {
			t.Fatalf("chart %d has no semantic categories", i+1)
		}
		for seriesIndex, series := range chart.Series {
			if len(series.Values) == 0 {
				t.Fatalf("chart %d series %d has no semantic values", i+1, seriesIndex+1)
			}
		}
	}
}
