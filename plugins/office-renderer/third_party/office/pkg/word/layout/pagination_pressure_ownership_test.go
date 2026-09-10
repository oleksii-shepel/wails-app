package layout

import "testing"

func TestFlowRegionKeepsSamePageFloatOwnership(t *testing.T) {
	float := FloatBox{X: 10, Y: 20, Width: 30, Height: 40}
	page := Page{Floats: []FloatBox{float}}
	got := wrapFloatsForFlowRegion(page)
	if len(got) != 1 || got[0].X != 10 || got[0].Y != 20 {
		t.Fatalf("same-page flow region lost float ownership: %#v", got)
	}
}

func TestFreshPageHasNoInheritedWrapFloats(t *testing.T) {
	if got := wrapFloatsForFlowRegion(Page{}); len(got) != 0 {
		t.Fatalf("fresh page inherited %d floats", len(got))
	}
}

func TestTableContinuationHeaderOwnership(t *testing.T) {
	tests := []struct {
		start, headerEnd int
		want             bool
	}{
		{0, 2, false},
		{1, 2, false},
		{2, 2, true},
		{5, 2, true},
		{2, 0, false},
	}
	for _, tc := range tests {
		if got := tableFragmentRepeatsHeaders(tc.start, tc.headerEnd); got != tc.want {
			t.Fatalf("repeat headers(%d,%d) = %v, want %v", tc.start, tc.headerEnd, got, tc.want)
		}
	}
}

func TestTableContinuationUsesReservedBodyFloor(t *testing.T) {
	if got := tableFlowLimitHeight(40, 60); got != 1 {
		t.Fatalf("oversubscribed table region = %.2fpt, want 1pt progress floor", got)
	}
}
