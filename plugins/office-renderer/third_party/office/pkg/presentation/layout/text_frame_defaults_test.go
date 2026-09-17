package layout

import (
	ir "office/pkg/presentation/ir"
	"testing"
)

func TestResolveTextFrameMetricsOwnsDefaultInsets(t *testing.T) {
	m := ResolveTextFrameMetrics(&ir.Shape{})
	if !nearDefaultInset(m.InsetTopPt, 3.6) || !nearDefaultInset(m.InsetBottomPt, 3.6) ||
		!nearDefaultInset(m.InsetLeftPt, 7.2) || !nearDefaultInset(m.InsetRightPt, 7.2) {
		t.Fatalf("default text-frame insets = %+v, want 3.6/7.2 DrawingML defaults", m)
	}
}

func nearDefaultInset(a, b float64) bool {
	if a > b {
		return a-b < 1e-9
	}
	return b-a < 1e-9
}
