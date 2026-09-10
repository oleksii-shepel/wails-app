package layout

import "testing"

func TestWidowControlAllowsAlreadyStartedFinalSingleLine(t *testing.T) {
	m := paragraphFlowMetrics{
		lineHeights: []float64{12, 12, 12},
	}
	gotLines, gotHeight := m.fragmentFit(2, 12, true)
	if gotLines != 1 || gotHeight != 12 {
		t.Fatalf("final continuation = (%d, %.2fpt), want (1, 12pt)", gotLines, gotHeight)
	}
}

func TestWidowControlStillRejectsSingleLineAtFirstSplitBoundary(t *testing.T) {
	m := paragraphFlowMetrics{
		lineHeights: []float64{12, 12, 12},
	}
	gotLines, _ := m.fragmentFit(0, 12, true)
	if gotLines != 0 {
		t.Fatalf("first fragment lines = %d, want 0 under widow control", gotLines)
	}
}

func TestWidowControlLeavesTwoLinesForContinuation(t *testing.T) {
	m := paragraphFlowMetrics{
		lineHeights: []float64{12, 12, 12, 12},
	}
	gotLines, gotHeight := m.fragmentFit(0, 36, true)
	if gotLines != 2 || gotHeight != 24 {
		t.Fatalf("first fragment = (%d, %.2fpt), want (2, 24pt)", gotLines, gotHeight)
	}
}

