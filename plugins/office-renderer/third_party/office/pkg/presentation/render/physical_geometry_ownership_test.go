package render

import (
	"os"
	"strings"
	"testing"
)

func TestRendererDoesNotRebuildCanonicalShapeOrConnectorGeometry(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, forbidden := range []string{"drawing.BuildShapePrimitives(", "drawing.BuildConnectorPrimitives(", "shape.GeometryData", "shape.GeometryAdjustments"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("renderer retained canonical geometry ownership %q", forbidden)
		}
	}
}
