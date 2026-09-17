package layout

import (
    "math"
    "testing"

    ir "office/pkg/presentation/ir"
)

func TestPhysicalMarkerOwnsHangingIndentOrigin(t *testing.T) {
    p := &ir.Paragraph{
        HasMarginLeft: true, MarginLeftEMU: 457200, // 36pt text margin
        HasTextIndent: true, TextIndentEMU: -381000, // marker origin 30pt left
        BulletChar: "•", BulletFontFamily: "Arial", BulletSize: 1200,
        Runs: []*ir.Run{{Text: "Item", FontFamily: "Arial", FontSize: 1200}},
    }
    plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
    if math.Abs(plan.Marker.OffsetPt-(-30)) > 0.001 {
        t.Fatalf("marker offset = %.3fpt, want -30pt", plan.Marker.OffsetPt)
    }
    markerWidth := measurePlainTextWidthPt("•", 12, "Arial", false, false)
    if math.Abs((plan.Marker.OffsetPt+markerWidth+plan.Marker.GapPt)-0) > 0.001 {
        t.Fatalf("marker+gap ends at %.3fpt, want paragraph text origin 0pt", plan.Marker.OffsetPt+markerWidth+plan.Marker.GapPt)
    }
}

func TestPhysicalMarkerOwnsPositiveFirstLineIndentOrigin(t *testing.T) {
    p := &ir.Paragraph{
        HasTextIndent: true, TextIndentEMU: 127000, // +10pt
        BulletChar: "•", BulletFontFamily: "Arial", BulletSize: 1200,
        Runs: []*ir.Run{{Text: "Item", FontFamily: "Arial", FontSize: 1200}},
    }
    plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
    if math.Abs(plan.Marker.OffsetPt-10) > 0.001 {
        t.Fatalf("marker offset = %.3fpt, want 10pt", plan.Marker.OffsetPt)
    }
}
