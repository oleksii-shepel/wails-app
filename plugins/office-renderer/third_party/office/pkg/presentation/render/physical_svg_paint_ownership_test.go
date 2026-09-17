package render

import (
	"os"
	"strings"
	"testing"
)

func TestRendererConsumesLayoutOwnedSVGPaintOverflow(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "item.Drawing.Paint.SVG.OverflowVisible") {
		t.Fatal("renderer must consume layout-owned SVG paint overflow")
	}
	for _, forbidden := range []string{"resolvePhysicalSVGPaintPlan", "FilterXPercent =", "FilterWPercent ="} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("renderer re-derived SVG paint semantics via %q", forbidden)
		}
	}
}
