package layout

import (
	"os"
	"strings"
	"testing"
)

func TestPhysicalCompositionMaterializesOncePerVisual(t *testing.T) {
	data, err := os.ReadFile("physical_visual.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, stale := range []string{
		"resolvePhysicalCompositionPlanWithPaint",
		"resolvePhysicalMediaPlanWithPaint",
		"resolvePhysicalOLEPlanWithPaint",
		"resolvePhysicalTablePlanWithPaint",
		"resolvePhysicalChartPlanWithPaint",
		"resolvePhysicalSmartArtPlanWithPaint",
		"resolveImageVisualPlanWithPaint",
		"resolveConnectorVisualPlanWithPaint",
		"resolveFreeConnectorVisualPlanWithPaint",
	} {
		if strings.Contains(src, stale) {
			t.Fatalf("physical composition retained transitional paint resolver %q", stale)
		}
	}
	for _, required := range []string{
		"composition := materializePhysicalCompositionPlan(shape, paint, transform, visual.Effects)",
		"resolvePhysicalMediaPlanWithComposition(shape, composition)",
		"resolvePhysicalOLEPlanWithComposition(shape, composition)",
		"resolvePhysicalTablePlanWithComposition(shape, composition)",
		"resolvePhysicalChartPlanWithComposition(shape, composition)",
		"resolvePhysicalSmartArtPlanWithComposition(shape, composition)",
		"resolveImageVisualPlanWithComposition(shape, composition)",
		"resolveConnectorVisualPlanWithComposition(shape, composition)",
		"resolveFreeConnectorVisualPlanWithComposition(shape, composition)",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("physical tree does not reuse canonical composition through %q", required)
		}
	}
}
