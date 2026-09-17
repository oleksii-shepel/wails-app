package layout

import (
    "testing"
    ir "office/pkg/presentation/ir"
)

func TestRunVisualPlanOwnsKerningThresholdDecision(t *testing.T) {
    below := ResolveRunVisualPlan(&ir.Run{FontSize: 1199, Kern: 1200}, 1)
    if !below.KerningSpecified || below.KerningEnabled {
        t.Fatalf("11.99pt run at 12pt threshold = specified %v enabled %v, want true/false", below.KerningSpecified, below.KerningEnabled)
    }
    at := ResolveRunVisualPlan(&ir.Run{FontSize: 1200, Kern: 1200}, 1)
    if !at.KerningSpecified || !at.KerningEnabled {
        t.Fatalf("12pt run at 12pt threshold = specified %v enabled %v, want true/true", at.KerningSpecified, at.KerningEnabled)
    }
}

func TestRunVisualPlanKerningThresholdUsesEffectiveScaledFontSize(t *testing.T) {
    got := ResolveRunVisualPlan(&ir.Run{FontSize: 2400, Kern: 1800}, .5)
    if got.FontSizePt != 12 || got.KernPt != 18 || !got.KerningSpecified || got.KerningEnabled {
        t.Fatalf("scaled kerning plan = %#v, want 12pt below authored 18pt threshold", got)
    }
}

func TestRunVisualPlanDoesNotManufactureKerningForUnspecifiedThreshold(t *testing.T) {
    got := ResolveRunVisualPlan(&ir.Run{FontSize: 2400}, 1)
    if got.KerningSpecified || got.KerningEnabled {
        t.Fatalf("unspecified kerning = specified %v enabled %v", got.KerningSpecified, got.KerningEnabled)
    }
}
