package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLConsumesLayoutPhysicalPlansWithoutReResolvingDrawingSemantics(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	forbidden := []string{
		"playout.ResolveShapeVisualPlan(",
		"playout.ResolveImageVisualPlan(",
		"playout.ResolveConnectorVisualPlan(",
		"playout.ResolveFreeConnectorVisualPlan(",
		"playout.ResolveStrokeVisualPlan(",
		"drawing.BuildShapePrimitives(",
		"drawing.BuildConnectorPrimitives(",
		"playout.MaterializeVisualTree(",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("HTML renderer retained layout-owned drawing operation %q", token)
		}
	}
	if !strings.Contains(src, "playout.MaterializePhysicalVisualTree(shapes)") {
		t.Fatal("HTML renderer must enter through the physical visual layout boundary")
	}
}
