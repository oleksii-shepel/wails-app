package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLPassesPhysicalVisualExplicitlyThroughLeafRendering(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, forbidden := range []string{
		"physicalVisuals  map[*ir.Shape]*playout.PhysicalVisual",
		"ctx.physicalVisuals[shape]",
		"physicalVisual   *playout.PhysicalVisual",
		"ctx.physicalVisual",
		"func physicalVisual(",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("renderer retained implicit physical-plan state %q", forbidden)
		}
	}
	for _, required := range []string{
		"renderShape(b, item, slideW, slideH, ctx)",
		"func renderShape(b *strings.Builder, item *playout.PhysicalVisual",
		"func renderShapeContent(b *strings.Builder, item *playout.PhysicalVisual",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("renderer must pass PhysicalVisual explicitly %q", required)
		}
	}
}
