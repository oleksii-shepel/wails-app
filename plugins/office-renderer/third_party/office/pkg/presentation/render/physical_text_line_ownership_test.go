package render

import (
	"os"
	"strings"
	"testing"
)

func TestRendererConsumesPhysicalTextLinesWithoutRelayout(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, forbidden := range []string{"playout.LayoutParagraphLines(", "playout.CanLayoutParagraphLines("} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("renderer retained text line layout ownership %q", forbidden)
		}
	}
	if !strings.Contains(s, "plan.LinesMeasured") {
		t.Fatal("renderer does not consume physical paragraph lines")
	}
	if !strings.Contains(s, "fragment.WidthPt") {
		t.Fatal("renderer does not consume physical tab advance")
	}
}
