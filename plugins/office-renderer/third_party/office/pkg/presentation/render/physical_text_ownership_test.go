package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLTextCompositionConsumesPhysicalPlan(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, forbidden := range []string{
		"drawingcss.TextBodyProps(shape.TextBodyProps)",
		"playout.ResolveTextFrameMetrics(shape)",
		"playout.ResolveTextFlowMetrics(shape",
		"playout.LayoutTextFrame(shape",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("renderer retained canonical text composition path %q", forbidden)
		}
	}
	if !strings.Contains(src, "item.Text") {
		t.Fatal("renderer does not consume physical text plan")
	}
}
