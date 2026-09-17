package resolve

import "testing"

func TestRelativeRectPctPreservesSignedDrawingMLPercentages(t *testing.T) {
	for _, tc := range []struct {
		in   int
		want float64
	}{
		{-25000, -.25},
		{0, 0},
		{25000, .25},
		{100000, 1},
		{125000, 1.25},
	} {
		if got := relativeRectPct(tc.in); got != tc.want {
			t.Fatalf("relativeRectPct(%d) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
