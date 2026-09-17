package layout

import (
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
)

func TestImageVisualPlanOwnsAlphaOutsetFilterRegion(t *testing.T) {
	shape := &ir.Shape{
		Bounds:       ir.Rect{W: 12700 * 100, H: 12700 * 50},
		ImageEffects: []drawing.Effect{drawing.AlphaOutset{Radius: 20}},
	}
	plan := ResolveImageVisualPlan(shape)
	if math.Abs(plan.FilterXPct-(-20)) > 0.001 || math.Abs(plan.FilterWidthPct-140) > 0.001 {
		t.Fatalf("horizontal filter region = %.4f %.4f, want -20 140", plan.FilterXPct, plan.FilterWidthPct)
	}
	if math.Abs(plan.FilterYPct-(-40)) > 0.001 || math.Abs(plan.FilterHeightPct-180) > 0.001 {
		t.Fatalf("vertical filter region = %.4f %.4f, want -40 180", plan.FilterYPct, plan.FilterHeightPct)
	}
}

func TestImageVisualPlanFilterRegionUsesCroppedExpandedImageDimensions(t *testing.T) {
	shape := &ir.Shape{
		Bounds:        ir.Rect{W: 12700 * 100, H: 12700 * 100},
		ImageCropLeft: .25, ImageCropRight: .25,
		ImageFillLeft: .10, ImageFillRight: .10,
		ImageEffects: []drawing.Effect{drawing.AlphaOutset{Radius: 16}},
	}
	plan := ResolveImageVisualPlan(shape)
	// Final image width = 100pt * .8 frame * 2 crop expansion = 160pt.
	if math.Abs(plan.FilterXPct-(-10)) > 0.001 || math.Abs(plan.FilterWidthPct-120) > 0.001 {
		t.Fatalf("cropped filter region = %.4f %.4f, want -10 120", plan.FilterXPct, plan.FilterWidthPct)
	}
}
