package render

import (
	"os"
	"strings"
	"testing"
)

func TestRichContentRendererConsumesPhysicalTableAndChartPlans(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	tableStart := strings.Index(s, "func renderTableShape(")
	tableEnd := strings.Index(s[tableStart:], "\nfunc renderTextShape(")
	if tableStart < 0 || tableEnd < 0 {
		t.Fatal("renderTableShape not found")
	}
	table := s[tableStart : tableStart+tableEnd]
	for _, forbidden := range []string{"shape.Table.ColumnWidthsEMU", "shape.Table.Rows", "effectStylePropsForList(cell.Effects)"} {
		if strings.Contains(table, forbidden) {
			t.Fatalf("table renderer retained canonical interpretation %q", forbidden)
		}
	}
	if !strings.Contains(table, "item.Table") {
		t.Fatal("table renderer must consume physical table plan")
	}
	chartStart := strings.Index(s, "func renderChartShape(")
	chartEnd := strings.Index(s[chartStart:], "\nfunc renderConnector(")
	if chartStart < 0 || chartEnd < 0 {
		t.Fatal("renderChartShape not found")
	}
	chart := s[chartStart : chartStart+chartEnd]
	for _, forbidden := range []string{"shape.ImageData", "shape.ImageMIME", "shape.Name"} {
		if strings.Contains(chart, forbidden) {
			t.Fatalf("chart renderer retained canonical artifact interpretation %q", forbidden)
		}
	}
	if !strings.Contains(chart, "item.Chart") {
		t.Fatal("chart renderer must consume physical chart plan")
	}
}

func TestChartRendererConsumesPhysicalFrameTransformAndArtifactFit(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func renderChartShape(")
	end := strings.Index(src[start:], "\nfunc renderConnector(")
	if start < 0 || end < 0 {
		t.Fatal("renderChartShape bounds not found")
	}
	chart := src[start : start+end]
	for _, required := range []string{"chart.Transform.RotationDeg", "chart.Transform.FlipH", "chart.Transform.FlipV", "chart.ObjectFit", "presentationEffectPlanStyles(chart.Effects)", "ppt-chart__artifact"} {
		if !strings.Contains(chart, required) {
			t.Fatalf("chart renderer must consume physical %q", required)
		}
	}
	if strings.Contains(chart, "object-fit:contain") {
		t.Fatal("chart renderer retained browser contain heuristic")
	}
}

func TestChartRendererDoesNotReinterpretCanonicalFrameOrPaintComposition(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func renderChartShape(")
	end := strings.Index(src[start:], "\nfunc renderConnector(")
	if start < 0 || end < 0 {
		t.Fatal("renderChartShape bounds not found")
	}
	chart := src[start : start+end]
	for _, forbidden := range []string{"boundsPercent(shape.Bounds", "shape.RotationDeg", "shape.FlipH", "shape.FlipV", "appendPhysicalPaintStyles(styles, ctx, shape)"} {
		if strings.Contains(chart, forbidden) {
			t.Fatalf("chart renderer retained canonical composition interpretation %q", forbidden)
		}
	}
	for _, required := range []string{"physicalBoundsPercent(chart.FrameBounds", "chart.Paint.ZIndex", "chart.Paint.OverflowVisible"} {
		if !strings.Contains(chart, required) {
			t.Fatalf("chart renderer must serialize layout-owned composition %q", required)
		}
	}
}

func TestTableRendererConsumesPhysicalFrameTransformEffectsAndPaint(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func renderTableShape(")
	end := strings.Index(src[start:], "\nfunc renderTextShape(")
	if start < 0 || end < 0 {
		t.Fatal("renderTableShape bounds not found")
	}
	table := src[start : start+end]
	for _, forbidden := range []string{"boundsPercent(shape.Bounds", "shape.RotationDeg", "shape.FlipH", "shape.FlipV", "effectStyleProps(ctx, shape)", "appendPhysicalPaintStyles(styles, ctx, shape)"} {
		if strings.Contains(table, forbidden) {
			t.Fatalf("table renderer retained canonical composition interpretation %q", forbidden)
		}
	}
	for _, required := range []string{"physicalBoundsPercent(table.FrameBounds", "table.Transform.RotationDeg", "table.Transform.FlipH", "table.Transform.FlipV", "presentationEffectPlanStyles(table.Effects)", "table.Paint.ZIndex", "table.Paint.OverflowVisible"} {
		if !strings.Contains(table, required) {
			t.Fatalf("table renderer must serialize layout-owned composition %q", required)
		}
	}
}
