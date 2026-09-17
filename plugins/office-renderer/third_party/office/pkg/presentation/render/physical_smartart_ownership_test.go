package render

import (
	"os"
	"strings"
	"testing"

	playout "office/pkg/presentation/layout"
)

func TestSmartArtRendererConsumesPhysicalNodePlan(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func renderSmartArtShape(")
	end := strings.Index(src[start:], "// renderSmartArtVectorShape")
	if start < 0 || end < 0 {
		t.Fatal("renderSmartArtShape not found")
	}
	body := src[start : start+end]
	for _, forbidden := range []string{
		"shape.Fill", "shape.FillColor", "shape.Paragraphs",
		"boundsPercent(shape.Bounds", "effectStyleProps(ctx, shape)",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("SmartArt renderer retained canonical interpretation %q", forbidden)
		}
	}
	if !strings.Contains(body, "item.SmartArt") {
		t.Fatal("SmartArt renderer must consume physical SmartArt plan")
	}
	for _, required := range []string{"smartArt.FrameBounds", "smartArt.Transform", "smartArt.Effects", "smartArt.Paint"} {
		if !strings.Contains(body, required) {
			t.Fatalf("SmartArt renderer must consume physical composition field %q", required)
		}
	}
}

func TestSmartArtAndOrdinaryTextSharePhysicalPlacementSerialization(t *testing.T) {
	frame := playout.TextFrameMetrics{InsetTopPt: 3.6, InsetRightPt: 7.2, InsetBottomPt: 3.6, InsetLeftPt: 7.2, VerticalJustify: "center"}
	measured := playout.TextFrameLayout{Measured: true, VerticalOffsetPt: 18}
	box := playout.TextBoxVisualPlan{InsetTopPt: 3.6, InsetRightPt: 7.2, InsetBottomPt: 3.6, InsetLeftPt: 7.2, ContentTopPt: 18}
	got := appendTextBoxPlacementStyles(nil, frame, measured, box)
	joined := strings.Join(got, ";")
	if !strings.Contains(joined, "justify-content:flex-start") {
		t.Fatalf("measured placement must materialize anchor once, got %q", joined)
	}
	if !strings.Contains(joined, "padding:21.60pt 7.20pt 3.60pt 7.20pt") {
		t.Fatalf("physical vertical offset must be serialized with effective insets, got %q", joined)
	}
}


func TestClass12SerializerPreservesNegativePhysicalOverflowAnchor(t *testing.T) {
	frame := playout.TextFrameMetrics{InsetTopPt: 3.6, InsetRightPt: 7.2, InsetBottomPt: 3.6, InsetLeftPt: 7.2, VerticalJustify: "center"}
	measured := playout.TextFrameLayout{Measured: true, OverflowPt: 20, VerticalOffsetPt: -10}
	box := playout.TextBoxVisualPlan{
		InsetTopPt: 3.6, InsetRightPt: 7.2, InsetBottomPt: 3.6, InsetLeftPt: 7.2,
		ContentTopPt: -10, ClipContent: true,
	}
	got := appendTextBoxPlacementStyles(nil, frame, measured, box)
	joined := strings.Join(got, ";")
	if !strings.Contains(joined, "position:relative;top:-10.00pt") {
		t.Fatalf("signed physical overflow offset lost: %q", joined)
	}
	if strings.Contains(joined, "padding:-") {
		t.Fatalf("negative physical anchor must not be encoded as invalid padding: %q", joined)
	}
}
