package layout

import (
	"os"
	"strings"
	"testing"
)

func TestPhysicalPlanLifecycleHasNoPaintSpecificCompositionResolvers(t *testing.T) {
	for _, name := range []string{"physical_visual.go", "shape_visual.go", "text_visual.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		if strings.Contains(src, "WithPaint") || strings.Contains(src, "AndPaint") {
			t.Fatalf("%s retained transitional paint-specific physical resolver", name)
		}
	}
}
