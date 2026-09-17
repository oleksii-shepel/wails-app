package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLHasNoPhysicalVisualAccessorLayer(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	forbidden := []string{
		"physicalVisual   *playout.PhysicalVisual",
		"ctx.physicalVisual",
		"func physicalVisual(",
		"func physicalShapeVisual(",
		"func physicalTextVisual(",
		"func physicalImageVisual(",
		"func physicalConnectorVisual(",
		"func physicalFreeConnectorVisual(",
		"func physicalSmartArtVisual(",
		"func physicalTableVisual(",
		"func physicalChartVisual(",
		"func physicalMediaVisual(",
		"func physicalOLEVisual(",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("renderer retained transitional physical accessor %q", token)
		}
	}
	required := []string{
		"renderShape(b, item, slideW, slideH, ctx)",
		"table := item.Table",
		"imageVisual := *item.Image",
		"smartArt := item.SmartArt",
		"chart := item.Chart",
		"media := item.Media",
		"ole := item.OLE",
	}
	for _, token := range required {
		if !strings.Contains(src, token) {
			t.Fatalf("renderer does not consume explicit PhysicalVisual contract %q", token)
		}
	}
}
