package layout

import (
	"math"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestDrawingExclusionSquareUsesRotatedBounds(t *testing.T) {
	image := &wordir.Image{Floating: true, Width: 1270000, Height: 635000, RotationDeg: 90, WrapType: "square"}
	fl := FloatBox{Image: image, X: 100, Y: 200, Width: 100, Height: 50}
	exclusion, ok := drawingExclusionForFloat(fl)
	if !ok {
		t.Fatal("expected drawing exclusion")
	}
	if !almostSame(exclusion.left, 125) || !almostSame(exclusion.right, 175) || !almostSame(exclusion.top, 175) || !almostSame(exclusion.bottom, 275) {
		t.Fatalf("rotated exclusion = %.2f,%.2f..%.2f,%.2f; want 125,175..175,275", exclusion.left, exclusion.top, exclusion.right, exclusion.bottom)
	}
}

func TestDrawingExclusionTightUsesResolvedContour(t *testing.T) {
	shape := &wordir.Shape{Floating: true, Width: 1270000, Height: 1270000, WrapType: "tight", WrapPolygon: [][2]float64{{0, 0}, {100, 0}, {50, 100}}}
	fl := FloatBox{Shape: shape, X: 10, Y: 20, Width: 200, Height: 100}
	exclusion, ok := drawingExclusionForFloat(fl)
	if !ok {
		t.Fatal("expected drawing exclusion")
	}
	if len(exclusion.polygon) != 3 {
		t.Fatalf("polygon vertices = %d, want 3", len(exclusion.polygon))
	}
	want := [][2]float64{{10, 20}, {210, 20}, {110, 120}}
	for i := range want {
		if math.Abs(exclusion.polygon[i][0]-want[i][0]) > 1e-6 || math.Abs(exclusion.polygon[i][1]-want[i][1]) > 1e-6 {
			t.Fatalf("polygon[%d] = %v, want %v", i, exclusion.polygon[i], want[i])
		}
	}
}

func TestPolygonIntervalsAcrossBandUsesVertexExtrema(t *testing.T) {
	polygon := [][2]float64{{0, 0}, {100, 50}, {0, 100}, {25, 50}}
	intervals := polygonIntervalsAcrossBand(polygon, 40, 60)
	if len(intervals) == 0 {
		t.Fatal("expected occupied intervals")
	}
	left := intervals[0][0]
	right := intervals[len(intervals)-1][1]
	if left > 25.000001 || right < 99.999999 {
		t.Fatalf("band extent = %.4f..%.4f; want to include vertex extrema 25..100", left, right)
	}
}

func TestDrawingOuterHeightIncludesRotation(t *testing.T) {
	box := &wordir.TextBox{Floating: true, Width: 1270000, Height: 635000, RotationDeg: 90, WrapType: "topAndBottom"}
	fl := FloatBox{TextBox: box, Width: 100, Height: 50}
	if got := drawingOuterHeightPt(fl); !almostSame(got, 100) {
		t.Fatalf("outer height = %.2f, want 100", got)
	}
}
