package layout

import (
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestFallbackListIndentIsMaterializedBeforeWrapping(t *testing.T) {
	p := &ir.Paragraph{Level: 1, Runs: []*ir.Run{{Text: "alpha beta gamma", FontSize: 1200, FontFamily: "Arial"}}}
	plan := ResolveParagraphVisualPlan(p, 120, 1, 1)
	if plan.FallbackIndentPt != 24 {
		t.Fatalf("fallback indent = %.4fpt, want 24pt", plan.FallbackIndentPt)
	}
	if !plan.Metrics.HasMarginLeft || plan.Metrics.MarginLeftPt != 24 {
		t.Fatalf("physical margin = %.4fpt (owned=%v), want 24pt", plan.Metrics.MarginLeftPt, plan.Metrics.HasMarginLeft)
	}
	if plan.Metrics.AvailableWidthPt != 96 || plan.Metrics.FirstLineAvailableWidthPt != 96 {
		t.Fatalf("physical widths = %.4f/%.4fpt, want 96/96pt", plan.Metrics.AvailableWidthPt, plan.Metrics.FirstLineAvailableWidthPt)
	}
	if !plan.LinesMeasured {
		t.Fatal("fallback-indented paragraph was not physically laid out")
	}
}
