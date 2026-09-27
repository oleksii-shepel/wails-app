package layout

import (
	"math"
	"testing"

	"office/pkg/presentation/ir"
)

func TestResolveFreeConnectorVisualPlanOwnsPathTrimCurveAndArrowGeometry(t *testing.T) {
	s := &ir.Shape{
		ConnectorType:  "curved",
		ConnectorStart: &ir.Point{X: 0, Y: 0},
		ConnectorEnd:   &ir.Point{X: 1270000, Y: 0}, // 100pt
		LineColor:      "112233", LineWidthPt: 2,
		ArrowHead: "triangle", ArrowHeadLength: "med", ArrowHeadWidth: "med",
		ArrowTail: "diamond", ArrowTailLength: "sm", ArrowTailWidth: "sm",
	}
	got := ResolveFreeConnectorVisualPlan(s)
	if len(got.Points) != 2 || !got.Curved {
		t.Fatalf("plan = %+v", got)
	}
	if got.EndArrow == nil || got.StartArrow == nil {
		t.Fatalf("arrow geometry not materialized: %+v", got)
	}
	if got.Points[0].X <= 0 || got.Points[1].X >= 100 {
		t.Fatalf("arrow trim not materialized: %+v", got.Points)
	}
	if math.Abs(got.Control1.Y) > 1e-9 || math.Abs(got.Control2.Y) > 1e-9 {
		t.Fatalf("unexpected curve controls: %+v %+v", got.Control1, got.Control2)
	}
	if got.Stroke.Color != "112233" || got.Stroke.WidthPt != 2 {
		t.Fatalf("stroke = %+v", got.Stroke)
	}
}

func TestResolveFreeConnectorVisualPlanDoesNotMutateCanonicalPath(t *testing.T) {
	s := &ir.Shape{
		ConnectorStart: &ir.Point{X: 0, Y: 0},
		ConnectorEnd:   &ir.Point{X: 1270000, Y: 0},
		ConnectorPath:  []ir.Point{{X: 0, Y: 0}, {X: 635000, Y: 127000}, {X: 1270000, Y: 0}},
		ArrowHead:      "triangle",
	}
	before := append([]ir.Point(nil), s.ConnectorPath...)
	_ = ResolveFreeConnectorVisualPlan(s)
	for i := range before {
		if before[i] != s.ConnectorPath[i] {
			t.Fatalf("canonical connector path mutated at %d", i)
		}
	}
}

func TestResolveFreeConnectorVisualPlanPreservesBentPresetRouteForSolverEndpoints(t *testing.T) {
	start := ir.Point{X: 12700 * 10, Y: 12700 * 20}
	end := ir.Point{X: 12700 * 110, Y: 12700 * 70}
	s := &ir.Shape{
		Kind: ir.ShapeConnector, ConnectorType: "bentConnector3",
		ConnectorStart: &start, ConnectorEnd: &end,
		ConnectorAdjustments: []int{50000}, LineWidthPt: 1,
		ArrowHead: "triangle",
	}
	got := ResolveFreeConnectorVisualPlan(s)
	if len(got.Points) < 3 {
		t.Fatalf("solver-connected bent connector collapsed to %d points: %#v", len(got.Points), got.Points)
	}
	// The head arrow owns the first/start endpoint, so its tangent must come
	// from the first routed segment rather than the diagonal start/end chord.
	if got.StartArrow == nil {
		t.Fatal("missing start arrow")
	}
}

func TestResolveFreeConnectorVisualPlanRoutesReverseBentEndpointsInSlideSpace(t *testing.T) {
	start := ir.Point{X: 12700 * 110, Y: 12700 * 70}
	end := ir.Point{X: 12700 * 10, Y: 12700 * 20}
	s := &ir.Shape{Kind: ir.ShapeConnector, ConnectorType: "bentConnector3", ConnectorStart: &start, ConnectorEnd: &end, ConnectorAdjustments: []int{50000}}
	got := ResolveFreeConnectorVisualPlan(s)
	if len(got.Points) < 3 {
		t.Fatalf("reverse bent connector collapsed to %d points", len(got.Points))
	}
	if got.Points[0].X != 110 || got.Points[0].Y != 70 {
		t.Fatalf("start = %#v, want solver start (110,70)", got.Points[0])
	}
	last := got.Points[len(got.Points)-1]
	if last.X != 10 || last.Y != 20 {
		t.Fatalf("end = %#v, want solver end (10,20)", last)
	}
}

func TestResolveFreeConnectorVisualPlanMaterializesSolverCurvedPresetAsBezier(t *testing.T) {
	start := ir.Point{X: 12700 * 10, Y: 12700 * 20}
	end := ir.Point{X: 12700 * 110, Y: 12700 * 70}
	s := &ir.Shape{Kind: ir.ShapeConnector, ConnectorType: "curvedConnector2", ConnectorStart: &start, ConnectorEnd: &end, ArrowHead: "triangle"}
	got := ResolveFreeConnectorVisualPlan(s)
	if !got.Curved || len(got.CurveSegments) != 1 {
		t.Fatalf("curved preset not materialized as cubic: %+v", got)
	}
	seg := got.CurveSegments[0]
	if seg.Start != got.Points[0] || seg.End != got.Points[len(got.Points)-1] {
		t.Fatalf("cubic endpoints = %+v..%+v, want trimmed connector endpoints %+v..%+v", seg.Start, seg.End, got.Points[0], got.Points[len(got.Points)-1])
	}
	if got.StartArrow == nil {
		t.Fatal("missing arrow geometry")
	}
}

func TestResolveFreeConnectorVisualPlanHigherCurvedPresetProducesContinuousCubics(t *testing.T) {
	start := ir.Point{X: 12700 * 110, Y: 12700 * 70}
	end := ir.Point{X: 12700 * 10, Y: 12700 * 20}
	s := &ir.Shape{Kind: ir.ShapeConnector, ConnectorType: "curvedConnector5", ConnectorStart: &start, ConnectorEnd: &end, ConnectorAdjustments: []int{25000, 75000}}
	got := ResolveFreeConnectorVisualPlan(s)
	if !got.Curved || len(got.CurveSegments) < 2 {
		t.Fatalf("curvedConnector5 = %+v, want multi-segment cubic path", got)
	}
	for i := 1; i < len(got.CurveSegments); i++ {
		if got.CurveSegments[i-1].End != got.CurveSegments[i].Start {
			t.Fatalf("curve discontinuity between segments %d and %d", i-1, i)
		}
	}
}

func TestResolveFreeConnectorVisualPlanOpenArrowKeepsShaftAtTip(t *testing.T) {
	shape := &ir.Shape{
		ConnectorStart: &ir.Point{X: 0, Y: 0},
		ConnectorEnd:   &ir.Point{X: 1270000, Y: 0}, // 100pt
		ArrowHead:      "arrow",
		ArrowHeadWidth: "med", ArrowHeadLength: "med",
	}
	plan := ResolveFreeConnectorVisualPlan(shape)
	if len(plan.Points) != 2 || plan.Points[0].X != 0 {
		t.Fatalf("open-arrow shaft start = %#v, want authored 0pt tip", plan.Points)
	}
	if plan.StartArrow == nil || plan.StartArrow.Type != "arrow" {
		t.Fatalf("open arrow geometry = %#v", plan.StartArrow)
	}
}

func TestResolveFreeConnectorVisualPlanFilledDiamondTrimsToBackVertex(t *testing.T) {
	shape := &ir.Shape{
		ConnectorStart: &ir.Point{X: 0, Y: 0},
		ConnectorEnd:   &ir.Point{X: 1270000, Y: 0}, // 100pt
		ArrowHead:      "diamond",
		ArrowHeadWidth: "med", ArrowHeadLength: "med",
	}
	plan := ResolveFreeConnectorVisualPlan(shape)
	if len(plan.Points) != 2 || plan.Points[0].X != 4 {
		t.Fatalf("diamond shaft start = %#v, want 4pt back vertex", plan.Points)
	}
	if plan.StartArrow == nil || len(plan.StartArrow.PolyPts) != 4 || plan.StartArrow.PolyPts[2].X != 4 {
		t.Fatalf("diamond geometry = %#v, want back vertex at 4pt", plan.StartArrow)
	}
}

func TestConnectorArrowOrientationContractAcrossStraightBentAndCurvedRoutes(t *testing.T) {
	cases := []struct {
		name, typ  string
		start, end ir.Point
	}{
		{"straight-forward", "straightConnector1", ir.Point{X: 0, Y: 0}, ir.Point{X: 12700 * 100, Y: 0}},
		{"straight-reverse", "straightConnector1", ir.Point{X: 12700 * 100, Y: 0}, ir.Point{X: 0, Y: 0}},
		{"bent-forward", "bentConnector3", ir.Point{X: 12700 * 10, Y: 12700 * 20}, ir.Point{X: 12700 * 110, Y: 12700 * 70}},
		{"bent-reverse", "bentConnector3", ir.Point{X: 12700 * 110, Y: 12700 * 70}, ir.Point{X: 12700 * 10, Y: 12700 * 20}},
		{"curved-forward", "curvedConnector2", ir.Point{X: 12700 * 10, Y: 12700 * 20}, ir.Point{X: 12700 * 110, Y: 12700 * 70}},
		{"curved-reverse", "curvedConnector2", ir.Point{X: 12700 * 110, Y: 12700 * 70}, ir.Point{X: 12700 * 10, Y: 12700 * 20}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &ir.Shape{Kind: ir.ShapeConnector, ConnectorType: tc.typ, ConnectorStart: &tc.start, ConnectorEnd: &tc.end, LineWidthPt: 2,
				ArrowHead: "triangle", ArrowHeadWidth: "med", ArrowHeadLength: "med",
				ArrowTail: "triangle", ArrowTailWidth: "med", ArrowTailLength: "med"}
			p := ResolveFreeConnectorVisualPlan(s)
			if p.StartArrow == nil || p.EndArrow == nil {
				t.Fatalf("missing endpoint markers: %+v", p)
			}
			if p.StartArrow.Tip.X != float64(tc.start.X)/12700 || p.StartArrow.Tip.Y != float64(tc.start.Y)/12700 {
				t.Fatalf("start marker tip moved: %+v", p.StartArrow.Tip)
			}
			if p.EndArrow.Tip.X != float64(tc.end.X)/12700 || p.EndArrow.Tip.Y != float64(tc.end.Y)/12700 {
				t.Fatalf("end marker tip moved: %+v", p.EndArrow.Tip)
			}
			// Filled triangles must extend inward from each authored endpoint; this
			// catches swapped head/tail references and reversed tangent ownership.
			if len(p.StartArrow.PolyPts) != 3 || len(p.EndArrow.PolyPts) != 3 {
				t.Fatalf("triangle geometry missing")
			}
			startBase := (p.StartArrow.PolyPts[1].X-p.StartArrow.Tip.X)*(p.Points[0].X-p.StartArrow.Tip.X) + (p.StartArrow.PolyPts[1].Y-p.StartArrow.Tip.Y)*(p.Points[0].Y-p.StartArrow.Tip.Y)
			endBase := (p.EndArrow.PolyPts[1].X-p.EndArrow.Tip.X)*(p.Points[len(p.Points)-1].X-p.EndArrow.Tip.X) + (p.EndArrow.PolyPts[1].Y-p.EndArrow.Tip.Y)*(p.Points[len(p.Points)-1].Y-p.EndArrow.Tip.Y)
			if startBase <= 0 || endBase <= 0 {
				t.Fatalf("marker points away from trimmed shaft: startDot=%v endDot=%v", startBase, endBase)
			}
		})
	}
}

func TestClass8LargeTailArrowConsumesShortPolylineSegments(t *testing.T) {
	shape := &ir.Shape{
		ConnectorStart: &ir.Point{X: 0, Y: 0},
		ConnectorEnd:   &ir.Point{X: 12700 * 100, Y: 0},
		ConnectorPath: []ir.Point{
			{X: 0, Y: 0}, {X: 12700 * 90, Y: 0}, {X: 12700 * 100, Y: 0},
		},
		LineWidthPt: 3,
		ArrowTail:   "triangle", ArrowTailLength: "lg", ArrowTailWidth: "lg",
	}
	plan := ResolveFreeConnectorVisualPlan(shape)
	if len(plan.Points) != 2 {
		t.Fatalf("trimmed polyline points = %#v, want consumed short final segment", plan.Points)
	}
	// lg triangle depth is 6 * 3 = 18pt, so a 100pt endpoint trims to 82pt.
	if math.Abs(plan.Points[len(plan.Points)-1].X-82) > 1e-9 {
		t.Fatalf("shaft end = %#v, want 82pt after cross-segment trim", plan.Points[len(plan.Points)-1])
	}
	if plan.EndArrow == nil || math.Abs(plan.EndArrow.Tip.X-100) > 1e-9 {
		t.Fatalf("arrow tip moved while trimming shaft: %#v", plan.EndArrow)
	}
}

func TestClass8LargeHeadArrowConsumesShortPolylineSegments(t *testing.T) {
	shape := &ir.Shape{
		ConnectorStart: &ir.Point{X: 0, Y: 0},
		ConnectorEnd:   &ir.Point{X: 12700 * 100, Y: 0},
		ConnectorPath: []ir.Point{
			{X: 0, Y: 0}, {X: 12700 * 10, Y: 0}, {X: 12700 * 100, Y: 0},
		},
		LineWidthPt: 3,
		ArrowHead:   "triangle", ArrowHeadLength: "lg", ArrowHeadWidth: "lg",
	}
	plan := ResolveFreeConnectorVisualPlan(shape)
	if len(plan.Points) != 2 || math.Abs(plan.Points[0].X-18) > 1e-9 {
		t.Fatalf("shaft start = %#v, want 18pt after cross-segment trim", plan.Points)
	}
	if plan.StartArrow == nil || math.Abs(plan.StartArrow.Tip.X) > 1e-9 {
		t.Fatalf("start arrow tip moved while trimming shaft: %#v", plan.StartArrow)
	}
}

func TestClass8CurvedArrowTrimsByBezierArcLengthWithoutMovingAuthoredTangent(t *testing.T) {
	shape := &ir.Shape{
		ConnectorType:  "curved",
		ConnectorStart: &ir.Point{X: 0, Y: 0},
		ConnectorEnd:   &ir.Point{X: 12700 * 100, Y: 12700 * 60},
		LineWidthPt:    3,
		ArrowHead:      "triangle", ArrowHeadLength: "lg", ArrowHeadWidth: "lg",
	}
	plan := ResolveFreeConnectorVisualPlan(shape)
	if !plan.Curved || len(plan.CurveSegments) != 1 || plan.StartArrow == nil {
		t.Fatalf("curved arrow plan = %+v", plan)
	}
	seg := plan.CurveSegments[0]
	// A 3pt large triangle consumes 18pt along the curve. The visible shaft
	// must therefore start strictly inside the authored curve, but the marker
	// tip and orientation still belong to the untrimmed endpoint/tangent.
	if seg.Start.X <= 0 || seg.Start.Y < 0 {
		t.Fatalf("trimmed curve start = %+v, want interior point", seg.Start)
	}
	if math.Abs(plan.StartArrow.Tip.X) > 1e-9 || math.Abs(plan.StartArrow.Tip.Y) > 1e-9 {
		t.Fatalf("authored arrow tip moved: %+v", plan.StartArrow.Tip)
	}
	if plan.StartArrow.PolyPts == nil || len(plan.StartArrow.PolyPts) != 3 {
		t.Fatalf("triangle marker geometry missing: %+v", plan.StartArrow)
	}
	// De Casteljau trimming preserves the original cubic locus. Rebuilding a
	// curve from the shortened endpoint would keep Y=0 at control1 for this
	// case; the split curve control must move with the trim instead.
	if seg.Control1.X <= seg.Start.X {
		t.Fatalf("trimmed cubic control did not advance with arc-length split: %+v", seg)
	}
}

func TestClass8CurvedBidirectionalMarkersCanConsumeWholeShaftWithoutLosingPaint(t *testing.T) {
	shape := &ir.Shape{
		ConnectorType:  "curved",
		ConnectorStart: &ir.Point{X: 0, Y: 0},
		ConnectorEnd:   &ir.Point{X: 12700 * 20, Y: 0},
		LineWidthPt:    3,
		ArrowHead:      "triangle", ArrowHeadLength: "lg", ArrowHeadWidth: "lg",
		ArrowTail: "triangle", ArrowTailLength: "lg", ArrowTailWidth: "lg",
	}
	plan := ResolveFreeConnectorVisualPlan(shape)
	if plan.StartArrow == nil || plan.EndArrow == nil {
		t.Fatalf("endpoint paint lost when markers consume shaft: %+v", plan)
	}
	if len(plan.CurveSegments) != 0 {
		t.Fatalf("short curved shaft survived two 36pt trims: %+v", plan.CurveSegments)
	}
}
