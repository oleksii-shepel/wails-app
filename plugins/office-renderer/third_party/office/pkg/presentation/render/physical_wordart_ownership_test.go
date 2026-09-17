package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLWordArtConsumesPhysicalTextGeometryPlan(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, forbidden := range []string{
		"shape.OfficeArt.GeometryText",
		"shape.OfficeArt.Text",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("renderer retained OfficeArt geometry-text interpretation %q", forbidden)
		}
	}
	for _, required := range []string{"textPlan.Geometry", "data-text-geometry", "data-text-warp"} {
		if !strings.Contains(src, required) {
			t.Fatalf("renderer does not serialize physical WordArt contract %q", required)
		}
	}
}
