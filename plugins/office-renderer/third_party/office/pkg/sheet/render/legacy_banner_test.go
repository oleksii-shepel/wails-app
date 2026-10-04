package render

import (
	"testing"

	"office/pkg/sheet/parsers/ooxml/resolve"
)

func TestClampBannerWidthKeepsLegacyHeaderInsideInnerGutter(t *testing.T) {
	drawing := &resolve.Drawing{
		Type:  resolve.DrawingTypeShape,
		Shape: &resolve.DrawingShape{Text: "Conditionally Formatting Data"},
	}
	layout := drawingLayout{dataRightPx: 899, defaultColPxV: 60}
	if got := clampBannerWidthPx(drawing, 84, 851, layout); got != 801 {
		t.Fatalf("clamped legacy header width = %dpx, want 801px", got)
	}
}
