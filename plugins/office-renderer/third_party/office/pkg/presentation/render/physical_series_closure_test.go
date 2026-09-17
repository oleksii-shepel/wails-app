package render

import (
	"os"
	"strings"
	"testing"
)

// TestPhysicalSeriesClosureLeavesNumberingStateInLayout ensures HTML only
// serializes the marker text materialized by layout and cannot recreate list state.
func TestPhysicalSeriesClosureLeavesNumberingStateInLayout(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, stale := range []string{"func formatAutoNum(", "func romanUpper(", "func romanLower(", "autoCounters"} {
		if strings.Contains(src, stale) {
			t.Fatalf("renderer retained migration-era numbering helper/state %q", stale)
		}
	}
	start := strings.Index(src, "func renderParagraphWithPlan(")
	if start < 0 {
		t.Fatal("renderParagraphWithPlan not found")
	}
	end := strings.Index(src[start:], "\nfunc ")
	if end < 0 {
		t.Fatal("renderParagraphWithPlan end not found")
	}
	body := src[start : start+end]
	if !strings.Contains(body, "markerText = marker.Text") {
		t.Fatal("renderer does not serialize layout-owned auto-number marker text")
	}
}
