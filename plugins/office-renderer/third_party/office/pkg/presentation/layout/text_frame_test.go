package layout

import (
	"math"
	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
	"testing"
)

func TestResolveTextFrameMetricsOwnsNormAutofitInsetsAndAnchor(t *testing.T) {
	s := &ir.Shape{
		TextBodyProps: drawing.TextBodyProperties{
			Anchor:                    "ctr",
			InsetL:                    91440,
			InsetR:                    182880,
			InsetT:                    45720,
			InsetB:                    91440,
			AutofitType:               "normAutofit",
			AutofitFontScale:          92000,
			AutofitLineSpaceReduction: 18000,
		},
	}

	got := ResolveTextFrameMetrics(s)

	assertNear := func(name string, got, want float64) {
		t.Helper()

		const epsilon = 1e-9
		if math.Abs(got-want) > epsilon {
			t.Fatalf("%s = %.12f, want %.12f", name, got, want)
		}
	}

	assertNear("font scale", got.FontScale, 0.92)
	assertNear("line scale", got.LineScale, 0.82)

	assertNear("left inset", got.InsetLeftPt, 7.2)
	assertNear("right inset", got.InsetRightPt, 14.4)
	assertNear("top inset", got.InsetTopPt, 3.6)
	assertNear("bottom inset", got.InsetBottomPt, 7.2)

	if got.VerticalJustify != "center" {
		t.Fatalf(
			"vertical justify = %q, want %q",
			got.VerticalJustify,
			"center",
		)
	}
}

func TestResolveTextFrameMetricsOwnsAutogrowAndOverflow(t *testing.T) {
	grow := ResolveTextFrameMetrics(&ir.Shape{TextBodyProps: drawing.TextBodyProperties{AutofitType: "spAutoFit"}})
	if !grow.AutoGrow || grow.AllowOverflow {
		t.Fatalf("spAutoFit = %+v", grow)
	}
	overflow := ResolveTextFrameMetrics(&ir.Shape{TextBodyProps: drawing.TextBodyProperties{AutofitType: "noAutofit"}})
	if !overflow.AllowOverflow || overflow.AutoGrow {
		t.Fatalf("noAutofit = %+v", overflow)
	}
}

func TestTextFramePreservesExplicitZeroInsets(t *testing.T) {
	shape := &ir.Shape{HasTextInsets: true}
	got := ResolveTextFrameMetrics(shape)
	if got.InsetLeftPt != 0 || got.InsetRightPt != 0 || got.InsetTopPt != 0 || got.InsetBottomPt != 0 {
		t.Fatalf("explicit zero text insets = L%.2f R%.2f T%.2f B%.2f, want all zero", got.InsetLeftPt, got.InsetRightPt, got.InsetTopPt, got.InsetBottomPt)
	}
}

func TestEffectiveTextFrameMetricsOwnGeometryInsetsBeforeFlow(t *testing.T) {
	shape := &ir.Shape{Bounds: ir.Rect{W: 1270000, H: 635000}, HasTextInsets: true}
	visual := ShapeVisualPlan{}
	visual.Geometry.ContentInsetLeft = 0.10
	visual.Geometry.ContentInsetRight = 0.20
	visual.Geometry.ContentInsetTop = 0.10
	visual.Geometry.ContentInsetBottom = 0.20

	got := ResolveEffectiveTextFrameMetrics(shape, visual)
	if math.Abs(got.InsetLeftPt-10) > 0.001 || math.Abs(got.InsetRightPt-20) > 0.001 {
		t.Fatalf("horizontal effective insets = %.3f, %.3f; want 10, 20", got.InsetLeftPt, got.InsetRightPt)
	}
	if math.Abs(got.InsetTopPt-5) > 0.001 || math.Abs(got.InsetBottomPt-10) > 0.001 {
		t.Fatalf("vertical effective insets = %.3f, %.3f; want 5, 10", got.InsetTopPt, got.InsetBottomPt)
	}
	flow := ResolveTextFlowMetrics(shape, got)
	if math.Abs(flow.LineExtentPt-70) > 0.001 {
		t.Fatalf("line extent = %.3f, want geometry-inset content width 70", flow.LineExtentPt)
	}
}

func TestResolveTextFrameMetricsPreservesDistributedVerticalAnchors(t *testing.T) {
	for anchor, want := range map[string]string{"just": "space-between", "dist": "space-around"} {
		shape := &ir.Shape{TextBodyProps: drawing.TextBodyProperties{Anchor: anchor}}
		if got := ResolveTextFrameMetrics(shape).VerticalJustify; got != want {
			t.Fatalf("anchor %q justify = %q, want %q", anchor, got, want)
		}
	}
}
