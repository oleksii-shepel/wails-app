package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLConsumesPhysicalVisualDirectlyForGenericShape(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "visual := item.Drawing.Visual") {
		t.Fatal("renderer must consume generic drawing visual directly from PhysicalVisual")
	}
	for _, forbidden := range []string{"func physicalShapeVisual(", "func physicalShapePlan(", "physicalShapeVisual(ctx, shape)", "physicalShapePlan(ctx, shape)"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("renderer retained generic physical accessor %q", forbidden)
		}
	}
}
