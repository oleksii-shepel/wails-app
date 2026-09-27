package drawing

import (
	"math"
	"testing"
)

func TestConnectorWaypointsStraight(t *testing.T) {
	pts := ConnectorWaypoints("straightConnector1", 0, 0, 100, 50, nil, false, false)
	if len(pts) != 2 {
		t.Fatalf("straight points = %d, want 2", len(pts))
	}
	if pts[0].X != 0 || pts[0].Y != 0 {
		t.Errorf("start = (%v,%v), want (0,0)", pts[0].X, pts[0].Y)
	}
	if pts[1].X != 100 || pts[1].Y != 50 {
		t.Errorf("end = (%v,%v), want (100,50)", pts[1].X, pts[1].Y)
	}
}

func TestConnectorWaypointsBent2(t *testing.T) {
	pts := ConnectorWaypoints("bentConnector2", 0, 0, 100, 50, []int{50000}, false, false)
	if len(pts) < 2 {
		t.Fatalf("bent2 points = %d, want >= 2", len(pts))
	}
	if pts[0].X != 0 || pts[0].Y != 0 {
		t.Errorf("start = (%v,%v), want (0,0)", pts[0].X, pts[0].Y)
	}
	last := pts[len(pts)-1]
	if last.X != 100 || last.Y != 50 {
		t.Errorf("end = (%v,%v), want (100,50)", last.X, last.Y)
	}
}

func TestConnectorWaypointsBent3(t *testing.T) {
	pts := ConnectorWaypoints("bentConnector3", 0, 0, 100, 50, []int{50000}, false, false)
	if len(pts) < 2 {
		t.Fatalf("bent3 points = %d, want >= 2", len(pts))
	}
	last := pts[len(pts)-1]
	if last.X != 100 || last.Y != 50 {
		t.Errorf("end = (%v,%v), want (100,50)", last.X, last.Y)
	}
}

func TestConnectorWaypointsBent3DefaultsMissingGuideButPreservesAuthoredZero(t *testing.T) {
	defaulted := ConnectorWaypoints("bentConnector3", 0, 0, 100, 50, nil, false, false)
	if len(defaulted) != 4 || defaulted[1].X != 50 || defaulted[2].X != 50 {
		t.Fatalf("missing bentConnector3 guide = %#v, want midpoint elbow x=50", defaulted)
	}

	explicitZero := ConnectorWaypoints("bentConnector3", 0, 0, 100, 50, []int{0}, false, false)
	if len(explicitZero) != 3 {
		t.Fatalf("authored zero bentConnector3 guide = %#v, want zero-column elbow path", explicitZero)
	}
	if explicitZero[0] != (Point{X: 0, Y: 0}) || explicitZero[1] != (Point{X: 0, Y: 50}) || explicitZero[2] != (Point{X: 100, Y: 50}) {
		t.Fatalf("authored zero bentConnector3 guide = %#v, want zero-column elbow semantics", explicitZero)
	}
}

func TestConnectorWaypointsBent4(t *testing.T) {
	pts := ConnectorWaypoints("bentConnector4", 0, 0, 100, 50, []int{30000, 70000}, false, false)
	if len(pts) < 2 {
		t.Fatalf("bent4 points = %d, want >= 2", len(pts))
	}
	last := pts[len(pts)-1]
	if last.X != 100 || last.Y != 50 {
		t.Errorf("end = (%v,%v), want (100,50)", last.X, last.Y)
	}
}

func TestConnectorWaypointsBent5(t *testing.T) {
	pts := ConnectorWaypoints("bentConnector5", 0, 0, 100, 50, []int{25000, 75000}, false, false)
	if len(pts) < 2 {
		t.Fatalf("bent5 points = %d, want >= 2", len(pts))
	}
	last := pts[len(pts)-1]
	if last.X != 100 || last.Y != 50 {
		t.Errorf("end = (%v,%v), want (100,50)", last.X, last.Y)
	}
}

func TestConnectorWaypointsCurved2(t *testing.T) {
	pts := ConnectorWaypoints("curvedConnector2", 0, 0, 100, 50, []int{50000}, false, false)
	if len(pts) != 4 {
		t.Fatalf("curved2 points = %d, want 4", len(pts))
	}
	if pts[0].X != 0 || pts[0].Y != 0 {
		t.Errorf("start = (%v,%v), want (0,0)", pts[0].X, pts[0].Y)
	}
	if pts[3].X != 100 || pts[3].Y != 50 {
		t.Errorf("end = (%v,%v), want (100,50)", pts[3].X, pts[3].Y)
	}
}

func TestConnectorWaypointsCurved3(t *testing.T) {
	pts := ConnectorWaypoints("curvedConnector3", 0, 0, 100, 50, []int{50000}, false, false)
	if len(pts) < 2 {
		t.Fatalf("curved3 points = %d, want >= 2", len(pts))
	}
	last := pts[len(pts)-1]
	if last.X != 100 || last.Y != 50 {
		t.Errorf("end = (%v,%v), want (100,50)", last.X, last.Y)
	}
}

func TestConnectorWaypointsCurved4(t *testing.T) {
	pts := ConnectorWaypoints("curvedConnector4", 0, 0, 100, 50, []int{30000, 70000}, false, false)
	if len(pts) < 2 {
		t.Fatalf("curved4 points = %d, want >= 2", len(pts))
	}
	last := pts[len(pts)-1]
	if last.X != 100 || last.Y != 50 {
		t.Errorf("end = (%v,%v), want (100,50)", last.X, last.Y)
	}
}

func TestConnectorWaypointsCurved5(t *testing.T) {
	pts := ConnectorWaypoints("curvedConnector5", 0, 0, 100, 50, []int{25000, 75000}, false, false)
	if len(pts) < 2 {
		t.Fatalf("curved5 points = %d, want >= 2", len(pts))
	}
	last := pts[len(pts)-1]
	if last.X != 100 || last.Y != 50 {
		t.Errorf("end = (%v,%v), want (100,50)", last.X, last.Y)
	}
}

func TestConnectorWaypointsUnknown(t *testing.T) {
	pts := ConnectorWaypoints("unknownConnector", 0, 0, 100, 50, nil, false, false)
	if len(pts) != 2 {
		t.Fatalf("unknown fallback points = %d, want 2", len(pts))
	}
	if pts[1].X != 100 || pts[1].Y != 50 {
		t.Errorf("end = (%v,%v), want (100,50)", pts[1].X, pts[1].Y)
	}
}

func TestConnectorWaypointsFlipH(t *testing.T) {
	for _, connType := range []string{"straightConnector1", "bentConnector2", "bentConnector3", "bentConnector4", "bentConnector5", "curvedConnector2", "curvedConnector3", "curvedConnector4", "curvedConnector5"} {
		pts := ConnectorWaypoints(connType, 0, 0, 100, 50, nil, true, false)
		if len(pts) < 2 {
			t.Errorf("%s flipH: points = %d, want >= 2", connType, len(pts))
			continue
		}
		if pts[0].X != 100 {
			t.Errorf("%s flipH start.X = %v, want 100", connType, pts[0].X)
		}
	}
}

func TestConnectorWaypointsFlipV(t *testing.T) {
	for _, connType := range []string{"straightConnector1", "bentConnector2", "bentConnector3", "bentConnector4", "bentConnector5", "curvedConnector2", "curvedConnector3", "curvedConnector4", "curvedConnector5"} {
		pts := ConnectorWaypoints(connType, 0, 0, 100, 50, nil, false, true)
		if len(pts) < 2 {
			t.Errorf("%s flipV: points = %d, want >= 2", connType, len(pts))
			continue
		}
		if pts[0].Y != 50 {
			t.Errorf("%s flipV start.Y = %v, want 50", connType, pts[0].Y)
		}
	}
}

func TestShortenWaypointsForArrows(t *testing.T) {
	pts := []Point{{X: 0, Y: 0}, {X: 100, Y: 0}}

	shortened := ShortenWaypointsForArrows(pts, 10, 10)
	if len(shortened) != 2 {
		t.Fatalf("len = %d, want 2", len(shortened))
	}
	if shortened[0].X != 10 || shortened[0].Y != 0 {
		t.Errorf("shortened start = (%v,%v), want (10,0)", shortened[0].X, shortened[0].Y)
	}
	if shortened[1].X != 90 || shortened[1].Y != 0 {
		t.Errorf("shortened end = (%v,%v), want (90,0)", shortened[1].X, shortened[1].Y)
	}
}

func TestShortenWaypointsForArrowsNoTrim(t *testing.T) {
	pts := []Point{{X: 0, Y: 0}, {X: 100, Y: 0}}
	shortened := ShortenWaypointsForArrows(pts, 0, 0)
	if len(shortened) != 2 {
		t.Fatalf("len = %d, want 2", len(shortened))
	}
	if shortened[0].X != 0 || shortened[1].X != 100 {
		t.Errorf("expected unchanged, got (%v,%v) -> (%v,%v)", shortened[0].X, shortened[0].Y, shortened[1].X, shortened[1].Y)
	}
}

func TestShortenWaypointsForArrowsShortLine(t *testing.T) {
	pts := []Point{{X: 0, Y: 0}, {X: 5, Y: 0}}
	shortened := ShortenWaypointsForArrows(pts, 10, 10)
	if len(shortened) != 2 {
		t.Fatalf("len = %d, want 2", len(shortened))
	}
	if shortened[0].X != shortened[1].X {
		t.Errorf("clamped = (%v,%v) -> (%v,%v)", shortened[0].X, shortened[0].Y, shortened[1].X, shortened[1].Y)
	}
}

func TestArrowTrimDistance(t *testing.T) {
	if got := ArrowTrimDistance("triangle", "med"); got != 4.0 {
		t.Errorf("triangle med = %v, want 4.0", got)
	}
	if got := ArrowTrimDistance("diamond", "med"); got != 4.0 {
		t.Errorf("diamond med = %v, want 4.0", got)
	}
	if got := ArrowTrimDistance("oval", "med"); got != 4.0 {
		t.Errorf("oval med = %v, want 4.0", got)
	}
	if got := ArrowTrimDistance("arrow", "med"); got != 0 {
		t.Errorf("arrow med = %v, want 0", got)
	}
}

func TestArrowDimensions(t *testing.T) {
	lenPt, widthPt := ArrowDimensions("triangle", "sm", "lg")
	if lenPt != 6.0 {
		t.Errorf("triangle lg length = %v, want 6.0", lenPt)
	}
	if widthPt != 2.25 {
		t.Errorf("triangle sm width = %v, want 2.25", widthPt)
	}

	lenPt2, widthPt2 := ArrowDimensions("diamond", "lg", "lg")
	if lenPt2 != 3.0 {
		t.Errorf("diamond lg length = %v, want 3.0", lenPt2)
	}
	if widthPt2 != 4.0 {
		t.Errorf("diamond lg width = %v, want 4.0", widthPt2)
	}
}

func TestMapArrowLengthPt(t *testing.T) {
	if got := mapArrowLengthPt("sm"); got != 3.0 {
		t.Errorf("small length = %v, want 3.0", got)
	}
	if got := mapArrowLengthPt("med"); got != 4.0 {
		t.Errorf("medium length = %v, want 4.0", got)
	}
	if got := mapArrowLengthPt("lg"); got != 6.0 {
		t.Errorf("large length = %v, want 6.0", got)
	}
	if got := mapArrowLengthPt("unknown"); got != 4.0 {
		t.Errorf("unknown length = %v, want 4.0", got)
	}
}

func TestMapArrowWidthPt(t *testing.T) {
	if got := mapArrowWidthPt("sm"); got != 2.25 {
		t.Errorf("small width = %v, want 2.25", got)
	}
	if got := mapArrowWidthPt("med"); got != 2.75 {
		t.Errorf("medium width = %v, want 2.75", got)
	}
	if got := mapArrowWidthPt("lg"); got != 4.0 {
		t.Errorf("large width = %v, want 4.0", got)
	}
	if got := mapArrowWidthPt("unknown"); got != 2.75 {
		t.Errorf("unknown width = %v, want 2.75", got)
	}
}

func TestComputeArrowGeometryTypes(t *testing.T) {
	tip := Point{X: 100, Y: 0}
	neighbor := Point{X: 0, Y: 0}

	for _, typ := range []string{"triangle", "stealth", "arrow", "diamond", "oval"} {
		ag := ComputeArrowGeometry(typ, "med", "med", tip, neighbor)
		if ag == nil {
			t.Errorf("%s: expected non-nil geometry", typ)
			continue
		}
		if ag.Type != typ {
			t.Errorf("%s: Type = %q, want %q", typ, ag.Type, typ)
		}
	}
}

func TestComputeArrowGeometryNilCases(t *testing.T) {
	if got := ComputeArrowGeometry("none", "med", "med", Point{X: 100, Y: 0}, Point{X: 0, Y: 0}); got != nil {
		t.Errorf("none = %+v", got)
	}
	if got := ComputeArrowGeometry("", "med", "med", Point{X: 100, Y: 0}, Point{X: 0, Y: 0}); got != nil {
		t.Errorf("empty = %+v", got)
	}
	if got := ComputeArrowGeometry("triangle", "med", "med", Point{X: 100, Y: 0}, Point{X: 100, Y: 0}); got != nil {
		t.Errorf("zero mag = %+v", got)
	}
}

func TestCompactWaypoints(t *testing.T) {
	pts := []Point{{X: 0, Y: 0}, {X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 0}}
	out := compactWaypoints(pts)
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	if out[0].X != 0 || out[1].X != 10 {
		t.Errorf("compact = %v", out)
	}
}

func TestCompactWaypointsEmpty(t *testing.T) {
	out := compactWaypoints(nil)
	if out != nil {
		t.Errorf("nil = %v, want nil", out)
	}
}

func TestShortenWaypointsForArrowsSinglePoint(t *testing.T) {
	pts := []Point{{X: 0, Y: 0}}
	out := ShortenWaypointsForArrows(pts, 10, 10)
	if len(out) != 1 {
		t.Errorf("len = %d, want 1", len(out))
	}
}

func TestShortenWaypointsTailOnly(t *testing.T) {
	pts := []Point{{X: 0, Y: 0}, {X: 100, Y: 0}}
	shortened := ShortenWaypointsForArrows(pts, 0, 20)
	if shortened[0].X != 0 || shortened[1].X != 80 {
		t.Errorf("tail-only = (%v,%v) -> (%v,%v)", shortened[0].X, shortened[0].Y, shortened[1].X, shortened[1].Y)
	}
}

func TestShortenWaypointsHeadOnly(t *testing.T) {
	pts := []Point{{X: 0, Y: 0}, {X: 100, Y: 0}}
	shortened := ShortenWaypointsForArrows(pts, 20, 0)
	if shortened[0].X != 20 || shortened[1].X != 100 {
		t.Errorf("head-only = (%v,%v) -> (%v,%v)", shortened[0].X, shortened[0].Y, shortened[1].X, shortened[1].Y)
	}
}

func TestShortenWaypointsDiagonal(t *testing.T) {
	pts := []Point{{X: 0, Y: 0}, {X: 100, Y: 100}}
	shortened := ShortenWaypointsForArrows(pts, 10, 10)
	// Should trim along the diagonal
	if len(shortened) != 2 {
		t.Fatalf("len = %d, want 2", len(shortened))
	}
	// Head moved 10 units toward start
	mag := math.Hypot(100, 100)
	expectedHeadX := 100 - 10*100/mag
	expectedHeadY := 100 - 10*100/mag
	if math.Abs(shortened[1].X-expectedHeadX) > 0.01 || math.Abs(shortened[1].Y-expectedHeadY) > 0.01 {
		t.Errorf("head = (%v,%v), want ~(%.2f,%.2f)", shortened[1].X, shortened[1].Y, expectedHeadX, expectedHeadY)
	}
}

func TestComputeArrowGeometryLargeAngle(t *testing.T) {
	// 45 degree angle
	tip := Point{X: 0, Y: 0}
	neighbor := Point{X: 10, Y: 10}
	ag := ComputeArrowGeometry("stealth", "lg", "lg", tip, neighbor)
	if ag == nil {
		t.Fatal("expected non-nil geometry")
	}
	if ag.Type != "stealth" {
		t.Errorf("type = %q, want stealth", ag.Type)
	}
}

func TestArrowTrimDistanceUnknown(t *testing.T) {
	if got := ArrowTrimDistance("unknown", "med"); got != 0 {
		t.Errorf("unknown arrow type = %v, want 0 because no marker geometry can be materialized", got)
	}
}

func TestUnsupportedArrowMarkerDoesNotTrimOrMaterialize(t *testing.T) {
	for _, typ := range []string{"", "none", "unknown", "extensionArrow"} {
		if got := ArrowTrimDistanceForStroke(typ, "lg", 4); got != 0 {
			t.Fatalf("%q trim = %v, want 0", typ, got)
		}
		if got := ComputeArrowGeometryForStroke(typ, "lg", "lg", 4, Point{X: 100}, Point{}); got != nil {
			t.Fatalf("%q geometry = %#v, want nil", typ, got)
		}
	}
}

func TestArrowDimensionsUnknownWidth(t *testing.T) {
	lenPt, widthPt := ArrowDimensions("triangle", "unknown", "med")
	if widthPt != 2.75 {
		t.Errorf("unknown width = %v, want 2.75", widthPt)
	}
	_ = lenPt
}

func TestArrowDimensionsUnknownLength(t *testing.T) {
	lenPt, widthPt := ArrowDimensions("triangle", "med", "unknown")
	if lenPt != 4.0 {
		t.Errorf("unknown length = %v, want 4.0", lenPt)
	}
	_ = widthPt
}

func TestArrowDimensionsStealth(t *testing.T) {
	lenPt, widthPt := ArrowDimensions("stealth", "sm", "lg")
	if lenPt != 6.0 {
		t.Errorf("stealth lg length = %v, want 6.0", lenPt)
	}
	if widthPt != 2.25 {
		t.Errorf("stealth sm width = %v, want 2.25", widthPt)
	}
}

func TestArrowDimensionsOalWithDefaults(t *testing.T) {
	// "oal" is not a known type, falls through to default
	lenPt, widthPt := ArrowDimensions("oal", "", "")
	if lenPt != 4.0 {
		t.Errorf("default length = %v, want 4.0", lenPt)
	}
	if widthPt != 2.75 {
		t.Errorf("default width = %v, want 2.75", widthPt)
	}
}

func TestArrowTrimDistanceMatchesMaterializedMarkerExtent(t *testing.T) {
	if got := ArrowTrimDistance("arrow", "med"); got != 0 {
		t.Fatalf("open arrow trim = %v, want 0 so the shaft reaches the wing vertex", got)
	}
	if got, want := ArrowTrimDistance("triangle", "med"), 4.0; got != want {
		t.Fatalf("triangle trim = %v, want %v", got, want)
	}
	if got, want := ArrowTrimDistance("oval", "med"), 4.0; got != want {
		t.Fatalf("oval trim = %v, want %v", got, want)
	}
	// ArrowDimensions intentionally represents a diamond by its tip-to-center
	// half-length. The materialized marker extends twice that far to its back vertex.
	if got, want := ArrowTrimDistance("diamond", "med"), 4.0; got != want {
		t.Fatalf("diamond trim = %v, want %v", got, want)
	}
}

func TestArrowDimensionsForStrokeClampThinMarkersToMinimumEffectiveStroke(t *testing.T) {
	tinyL, tinyW := ArrowDimensionsForStroke("triangle", "med", "med", 0.5)
	thinL, thinW := ArrowDimensionsForStroke("triangle", "med", "med", 1.5)
	thickL, thickW := ArrowDimensionsForStroke("triangle", "med", "med", 3)
	if tinyL != 6 || tinyW != 4.125 {
		t.Fatalf("0.5pt medium arrow = %.3fx%.3f, want 6x4.125", tinyL, tinyW)
	}
	if tinyL != thinL || tinyW != thinW {
		t.Fatalf("thin markers should clamp to a 1.5pt effective stroke: tiny %.3fx%.3f thin %.3fx%.3f", tinyL, tinyW, thinL, thinW)
	}
	if thickL != 12 || thickW != 8.25 {
		t.Fatalf("3pt medium arrow = %.2fx%.2f, want 12x8.25", thickL, thickW)
	}
}

func TestOpenArrowStrokeTracksConnectorStroke(t *testing.T) {
	ag := ComputeArrowGeometryForStroke("arrow", "med", "med", 4, Point{X: 100}, Point{})
	if ag == nil || ag.StrokeWidth != 4 {
		t.Fatalf("open arrow stroke = %#v, want the same 4pt pen as the connector", ag)
	}
}

func TestArrowMarkerTypeAndSizeMatrixMaterializesFiniteGeometry(t *testing.T) {
	for _, typ := range []string{"triangle", "stealth", "arrow", "diamond", "oval"} {
		for _, w := range []string{"sm", "med", "lg"} {
			for _, l := range []string{"sm", "med", "lg"} {
				name := typ + "/" + w + "/" + l
				t.Run(name, func(t *testing.T) {
					ag := ComputeArrowGeometryForStroke(typ, w, l, 2.5, Point{X: 100, Y: 40}, Point{X: 75, Y: 25})
					if ag == nil || ag.LengthPt <= 0 || ag.WidthPt <= 0 || math.IsNaN(ag.LengthPt) || math.IsNaN(ag.WidthPt) {
						t.Fatalf("invalid marker geometry: %#v", ag)
					}
					trim := ArrowTrimDistanceForStroke(typ, l, 2.5)
					if trim < 0 || math.IsNaN(trim) || math.IsInf(trim, 0) {
						t.Fatalf("invalid trim %v", trim)
					}
					if typ == "arrow" && trim != 0 {
						t.Fatalf("open arrow trim=%v, want 0", trim)
					}
					if typ != "arrow" && trim <= 0 {
						t.Fatalf("filled marker trim=%v, want >0", trim)
					}
				})
			}
		}
	}
}

func TestShortenPolylineWaypointsForArrowsConsumesShortBentDogleg(t *testing.T) {
	pts := []Point{{X: 0, Y: 0}, {X: 5, Y: 0}, {X: 5, Y: 40}, {X: 60, Y: 40}}
	got := ShortenPolylineWaypointsForArrows(pts, 12, 0)
	want := []Point{{X: 5, Y: 7}, {X: 5, Y: 40}, {X: 60, Y: 40}}
	if len(got) != len(want) {
		t.Fatalf("trimmed points = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("trimmed points[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestShortenPolylineWaypointsForArrowsConsumesShortTailDogleg(t *testing.T) {
	pts := []Point{{X: 0, Y: 0}, {X: 55, Y: 0}, {X: 55, Y: 5}, {X: 60, Y: 5}}
	got := ShortenPolylineWaypointsForArrows(pts, 0, 12)
	want := []Point{{X: 0, Y: 0}, {X: 53, Y: 0}}
	if len(got) != len(want) {
		t.Fatalf("trimmed points = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("trimmed points[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestStealthArrowTrimStopsAtConcaveNotch(t *testing.T) {
	length, _ := ArrowDimensionsForStroke("stealth", "med", "lg", 2)
	got := ArrowTrimDistanceForStroke("stealth", "lg", 2)
	want := length * 0.55
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("stealth trim = %v, want notch depth %v", got, want)
	}
	geometry := ComputeArrowGeometryForStroke("stealth", "med", "lg", 2, Point{X: 0, Y: 0}, Point{X: 100, Y: 0})
	if geometry == nil || len(geometry.PolyPts) != 4 {
		t.Fatalf("stealth geometry = %#v", geometry)
	}
	notch := geometry.PolyPts[2]
	if math.Abs(math.Hypot(notch.X, notch.Y)-got) > 1e-9 {
		t.Fatalf("stealth notch distance = %v, trim = %v", math.Hypot(notch.X, notch.Y), got)
	}
}

func TestPowerPointReferenceArrowScaleMatchesEffectiveMarkers(t *testing.T) {
	// 23884 slide 1 provides two useful PowerPoint-rendered references:
	// the 3pt large yellow triangle is about 18pt deep, while the 2.75pt
	// medium stealth arrows under the monitor stations are about 11pt deep.
	length, width := ArrowDimensionsForStroke("triangle", "lg", "lg", 3)
	if length != 18 || width != 12 {
		t.Fatalf("3pt large triangle = %.4fx%.4fpt, want 18x12pt", length, width)
	}
	length, width = ArrowDimensionsForStroke("stealth", "med", "med", 2.75)
	if length != 11 || math.Abs(width-7.5625) > 1e-9 {
		t.Fatalf("2.75pt medium stealth = %.4fx%.4fpt, want 11x7.5625pt", length, width)
	}
}

func TestPowerPointThinStealthMarkersRespectMinimumEffectiveStroke(t *testing.T) {
	// 23884 slide 3 uses 0.75pt white stealth tails that PowerPoint still
	// renders with a visibly medium marker instead of a 3pt-deep sliver.
	length, width := ArrowDimensionsForStroke("stealth", "med", "med", 0.75)
	if length != 6 || math.Abs(width-4.125) > 1e-9 {
		t.Fatalf("0.75pt medium stealth = %.4fx%.4fpt, want 6x4.125pt", length, width)
	}
}
