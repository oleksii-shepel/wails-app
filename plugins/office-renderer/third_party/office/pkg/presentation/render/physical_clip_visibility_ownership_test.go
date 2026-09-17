package render

import (
	"os"
	"strings"
	"testing"
)

// The renderer consumes the physical ancestor-clip plans materialized by layout.
// Visibility is not represented by a default-false Paint.Visible gate: a leaf
// that survives layout is rendered, while its ancestor clip geometry controls
// the visible painted region.
func TestHTMLConsumesLayoutOwnedAncestorClipPlans(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatalf("read html.go: %v", err)
	}
	src := string(data)
	if !strings.Contains(src, "item.Drawing.Paint.AncestorClips") {
		t.Fatal("renderer does not consume layout-owned ancestor clip plans")
	}
	if strings.Contains(src, "!item.Drawing.Paint.Visible") {
		t.Fatal("renderer must not use a default-false Paint.Visible gate")
	}
}
