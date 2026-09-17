package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLConsumesLayoutOwnedPhysicalPaintPlan(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	// A31 passes the active PhysicalVisual explicitly. Shared paint order and
	// SVG overflow are consumed from item.Drawing.Paint, while family-specific outer
	// overflow is consumed from the typed physical plans (textPlan.Paint,
	// image.Paint, table.Paint, and so on). Do not require the retired
	// item.Drawing.Paint.OverflowVisible spelling: outer overflow is no longer owned by
	// the generic renderer path.
	for _, required := range []string{
		"item.Drawing.Paint.ZIndex",
		"item.Drawing.Paint.SVG.OverflowVisible",
		".Paint.OverflowVisible",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("renderer does not consume physical paint contract %q", required)
		}
	}

	for _, forbidden := range []string{
		"resolvePhysicalOverflowPlan",
		"appendPhysicalPaintStyles(styles, ctx, shape)",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("renderer must not reconstruct physical paint through %q", forbidden)
		}
	}
}
