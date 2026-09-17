package layout

import (
    "math"
    "testing"

    "office/pkg/ooxml/drawing"
    ir "office/pkg/presentation/ir"
)

func TestClass9NormAutofitCannotEnlargeAuthoredText(t *testing.T) {
    shape := &ir.Shape{TextBodyProps: drawing.TextBodyProperties{AutofitType: "normAutofit", AutofitFontScale: 125000}}
    got := ResolveTextFrameMetrics(shape)
    if math.Abs(got.FontScale-1) > 0.000001 {
        t.Fatalf("normAutofit font scale = %.4f, want 1.0 clamp", got.FontScale)
    }
}

func TestClass9NormAutofitMaterializesPersistedShrink(t *testing.T) {
    shape := &ir.Shape{TextBodyProps: drawing.TextBodyProperties{AutofitType: "normAutofit", AutofitFontScale: 72500, AutofitLineSpaceReduction: 18000}}
    got := ResolveTextFrameMetrics(shape)
    if math.Abs(got.FontScale-.725) > 0.000001 || math.Abs(got.LineScale-.82) > 0.000001 {
        t.Fatalf("normAutofit metrics = %+v, want font=.725 line=.82", got)
    }
}

func TestClass9NormAutofitRejectsCollapsedLineScale(t *testing.T) {
    shape := &ir.Shape{TextBodyProps: drawing.TextBodyProperties{AutofitType: "normAutofit", AutofitLineSpaceReduction: 100000}}
    got := ResolveTextFrameMetrics(shape)
    if math.Abs(got.LineScale-1) > 0.000001 {
        t.Fatalf("malformed normAutofit line scale = %.4f, want safe 1.0", got.LineScale)
    }
}
