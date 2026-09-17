package render

import (
	"os"
	"strings"
	"testing"
)

func TestConnectorRendererConsumesPhysicalCompositionPlans(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func renderSmartArtConnector(")
	end := strings.Index(source[start:], "func smartArtConnectorPointsString(")
	if start < 0 || end < 0 {
		t.Fatal("could not locate SmartArt connector renderer")
	}
	body := source[start : start+end]
	for _, forbidden := range []string{"shape.ConnectorType", "shape.Bounds.W", "shape.Bounds.H", "renderCanonicalConnector"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("SmartArt connector renderer retained canonical composition decision %q", forbidden)
		}
	}
	for _, required := range []string{"item.Connector", "item.FreeConnector", "renderPhysicalConnector", "renderPhysicalFreeConnector"} {
		if !strings.Contains(body, required) {
			t.Fatalf("SmartArt connector renderer must consume physical plan %q", required)
		}
	}
}

func TestConnectorPaintIsReadFromConnectorPlan(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, required := range []string{
		"physicalPaintSVGStyle(visual.Composition.Paint)",
		"func renderPhysicalConnector(",
		"func renderPhysicalFreeConnector(",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("connector renderer missing physical composition contract %q", required)
		}
	}
}
