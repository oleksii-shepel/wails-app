package drawing

import (
	"math"
	"testing"
)

func TestBuildShapePrimitivesZeroSize(t *testing.T) {
	prims, err := BuildShapePrimitives(ShapeGeometrySpec{Width: 0, Height: 10})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if prims != nil {
		t.Errorf("prims = %v, want nil", prims)
	}

	prims2, err2 := BuildShapePrimitives(ShapeGeometrySpec{Width: 10, Height: 0})
	if err2 != nil {
		t.Fatalf("err = %v, want nil", err2)
	}
	if prims2 != nil {
		t.Errorf("prims = %v, want nil", prims2)
	}
}

func TestBuildShapePrimitivesNilPresetAndGeometry(t *testing.T) {
	prims, err := BuildShapePrimitives(ShapeGeometrySpec{Width: 100, Height: 50})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if prims != nil {
		t.Errorf("prims = %v, want nil", prims)
	}
}

func TestBuildShapePrimitivesCustomGeometryEmptyPaths(t *testing.T) {
	prims, err := BuildShapePrimitives(ShapeGeometrySpec{
		Width:    100,
		Height:   50,
		Geometry: &Geometry{Type: GeometryTypeCustom, Paths: []Path{}},
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if prims != nil {
		t.Errorf("prims = %v, want nil", prims)
	}
}

func TestBuildShapePrimitivesPresetGeometry(t *testing.T) {
	prims, err := BuildShapePrimitives(ShapeGeometrySpec{
		Width:     100,
		Height:    50,
		Geometry:  &Geometry{Type: GeometryTypePreset, Name: "rect"},
		FillColor: "FF0000",
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(prims) != 1 {
		t.Fatalf("prims = %d, want 1", len(prims))
	}
	if prims[0].Kind != PrimitivePath {
		t.Errorf("kind = %v, want PrimitivePath", prims[0].Kind)
	}
	if prims[0].Style.FillColor != "FF0000" {
		t.Errorf("fillColor = %q, want FF0000", prims[0].Style.FillColor)
	}
}

func TestBuildShapePrimitivesPresetFromGeometry(t *testing.T) {
	prims, err := BuildShapePrimitives(ShapeGeometrySpec{
		Width:    100,
		Height:   50,
		Geometry: &Geometry{Type: GeometryTypePreset, Name: "rect"},
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(prims) != 1 {
		t.Fatalf("prims = %d, want 1", len(prims))
	}
}

func TestBuildShapePrimitivesStrokeFallback(t *testing.T) {
	stroke := &LineStyle{Color: "0000FF", Width: 2}
	prims, err := BuildShapePrimitives(ShapeGeometrySpec{
		Width:  100,
		Height: 50,
		Preset: &PresetGeometry{Name: "rect"},
		Stroke: stroke,
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(prims) != 1 {
		t.Fatalf("prims = %d, want 1", len(prims))
	}
	if prims[0].Style.StrokeColor != "0000FF" {
		t.Errorf("strokeColor = %q, want 0000FF", prims[0].Style.StrokeColor)
	}
	if prims[0].Style.StrokeWidth != 2 {
		t.Errorf("strokeWidth = %v, want 2", prims[0].Style.StrokeWidth)
	}
}

func TestBuildShapePrimitivesCompoundLine(t *testing.T) {
	stroke := &LineStyle{Color: "000000", Width: 4, Compound: true, CompoundType: "dbl"}
	prims, err := BuildShapePrimitives(ShapeGeometrySpec{
		Width:     100,
		Height:    50,
		Preset:    &PresetGeometry{Name: "rect"},
		FillColor: "FFFFFF",
		Stroke:    stroke,
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(prims) < 2 {
		t.Fatalf("prims = %d, want >= 2 for compound line", len(prims))
	}
}

func TestBuildShapePrimitivesCompoundLineNoFillColor(t *testing.T) {
	stroke := &LineStyle{Color: "000000", Width: 4, Compound: true, CompoundType: "dbl"}
	prims, err := BuildShapePrimitives(ShapeGeometrySpec{
		Width:  100,
		Height: 50,
		Preset: &PresetGeometry{Name: "rect"},
		Stroke: stroke,
	})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if len(prims) < 2 {
		t.Fatalf("prims = %d, want >= 2 for compound line", len(prims))
	}
}

func TestBuildConnectorPrimitivesStraight(t *testing.T) {
	prims := BuildConnectorPrimitives(ConnectorSpec{
		Type:    "straightConnector1",
		BoundsX: 0, BoundsY: 0,
		BoundsW: 100, BoundsH: 50,
		LineColor:   "000000",
		LineWidthPt: 1,
	})
	if len(prims) != 1 {
		t.Fatalf("prims = %d, want 1", len(prims))
	}
	if prims[0].Kind != PrimitivePolyline {
		t.Errorf("kind = %v, want PrimitivePolyline", prims[0].Kind)
	}
}

func TestBuildConnectorPrimitivesCurved(t *testing.T) {
	prims := BuildConnectorPrimitives(ConnectorSpec{
		Type:    "curvedConnector2",
		BoundsX: 0, BoundsY: 0,
		BoundsW: 100, BoundsH: 50,
		Adjustments: []int{50000},
		LineColor:   "000000",
		LineWidthPt: 1,
	})
	if len(prims) < 1 {
		t.Fatalf("prims = %d, want >= 1", len(prims))
	}
	if prims[0].Kind != PrimitivePath {
		t.Errorf("kind = %v, want PrimitivePath", prims[0].Kind)
	}
}

func TestBuildConnectorPrimitivesWithArrows(t *testing.T) {
	prims := BuildConnectorPrimitives(ConnectorSpec{
		Type:    "straightConnector1",
		BoundsX: 0, BoundsY: 0,
		BoundsW: 100, BoundsH: 50,
		LineColor:       "000000",
		LineWidthPt:     1,
		ArrowHead:       "triangle",
		ArrowHeadWidth:  "med",
		ArrowHeadLength: "med",
		ArrowTail:       "arrow",
		ArrowTailWidth:  "med",
		ArrowTailLength: "med",
	})
	if len(prims) < 3 {
		t.Fatalf("prims = %d, want >= 3 (line + 2 arrows)", len(prims))
	}
}

func TestBuildConnectorPrimitivesLineStyleFallback(t *testing.T) {
	prims := BuildConnectorPrimitives(ConnectorSpec{
		Type:    "straightConnector1",
		BoundsX: 0, BoundsY: 0,
		BoundsW: 100, BoundsH: 50,
		LineStyle:   &LineStyle{Color: "FF0000", Width: 2},
		LineWidthPt: 0,
	})
	if len(prims) != 1 {
		t.Fatalf("prims = %d, want 1", len(prims))
	}
	if prims[0].Style.StrokeColor != "FF0000" {
		t.Errorf("strokeColor = %q, want FF0000", prims[0].Style.StrokeColor)
	}
	if prims[0].Style.StrokeWidth != 2 {
		t.Errorf("strokeWidth = %v, want 2", prims[0].Style.StrokeWidth)
	}
}

func TestBuildConnectorPrimitivesDefaultColor(t *testing.T) {
	prims := BuildConnectorPrimitives(ConnectorSpec{
		Type:    "straightConnector1",
		BoundsX: 0, BoundsY: 0,
		BoundsW: 100, BoundsH: 50,
	})
	if len(prims) != 1 {
		t.Fatalf("prims = %d, want 1", len(prims))
	}
	if prims[0].Style.StrokeColor != "000000" {
		t.Errorf("default color = %q, want 000000", prims[0].Style.StrokeColor)
	}
}

func TestBuildConnectorPrimitivesMinWidth(t *testing.T) {
	prims := BuildConnectorPrimitives(ConnectorSpec{
		Type:    "straightConnector1",
		BoundsX: 0, BoundsY: 0,
		BoundsW: 100, BoundsH: 50,
		LineWidthPt: 0.1,
	})
	if prims[0].Style.StrokeWidth != 0.5 {
		t.Errorf("min width = %v, want 0.5", prims[0].Style.StrokeWidth)
	}
}

func TestBuildConnectorPrimitivesCompoundLine(t *testing.T) {
	prims := BuildConnectorPrimitives(ConnectorSpec{
		Type:    "straightConnector1",
		BoundsX: 0, BoundsY: 0,
		BoundsW: 100, BoundsH: 50,
		LineStyle:   &LineStyle{Color: "000000", Width: 4, Compound: true, CompoundType: "dbl"},
		LineWidthPt: 4,
	})
	if len(prims) < 2 {
		t.Fatalf("prims = %d, want >= 2 for compound line", len(prims))
	}
}

func TestBuildConnectorPrimitivesTooFewPoints(t *testing.T) {
	// With zero bounds, straight connector still returns 2 identical points,
	// and ShortenWaypointsForArrows won't reduce them below 2, so we get a polyline.
	prims := BuildConnectorPrimitives(ConnectorSpec{
		Type:    "unknown",
		BoundsX: 0, BoundsY: 0,
		BoundsW: 0, BoundsH: 0,
	})
	// The function returns at least the line primitive (even if both points are identical)
	if len(prims) == 0 {
		t.Errorf("prims = %v, want at least one primitive", prims)
	}
}

func TestArrowPrimitiveTypes(t *testing.T) {
	tip := Point{X: 100, Y: 0}
	neighbor := Point{X: 0, Y: 0}

	for _, typ := range []string{"triangle", "stealth", "diamond", "arrow", "oval"} {
		prim, ok := arrowPrimitive(typ, "med", "med", "000000", tip, neighbor)
		if !ok {
			t.Errorf("%s: expected ok", typ)
			continue
		}
		switch typ {
		case "triangle", "stealth", "diamond":
			if prim.Kind != PrimitivePolyline {
				t.Errorf("%s: kind = %v, want PrimitivePolyline", typ, prim.Kind)
			}
			if !prim.Closed {
				t.Errorf("%s: closed = false, want true", typ)
			}
		case "arrow":
			if prim.Kind != PrimitivePolyline {
				t.Errorf("%s: kind = %v, want PrimitivePolyline", typ, prim.Kind)
			}
		case "oval":
			if prim.Kind != PrimitiveEllipse {
				t.Errorf("%s: kind = %v, want PrimitiveEllipse", typ, prim.Kind)
			}
		}
	}
}

func TestArrowPrimitiveNil(t *testing.T) {
	_, ok := arrowPrimitive("none", "med", "med", "000000", Point{X: 100, Y: 0}, Point{X: 0, Y: 0})
	if ok {
		t.Error("expected false for none arrow")
	}
}

func TestCompoundLineGapPrimitives(t *testing.T) {
	prims := []VectorPrimitive{{
		Kind:   PrimitivePolyline,
		Points: []Point{{X: 0, Y: 0}, {X: 100, Y: 0}},
	}}

	// dbl
	gaps := compoundLineGapPrimitives(prims, "dbl", 4, "FFFFFF")
	if len(gaps) != 1 {
		t.Fatalf("dbl gaps = %d, want 1", len(gaps))
	}

	// thickThin
	gaps2 := compoundLineGapPrimitives(prims, "thickThin", 4, "FFFFFF")
	if len(gaps2) != 1 {
		t.Fatalf("thickThin gaps = %d, want 1", len(gaps2))
	}

	// thinThick
	gaps3 := compoundLineGapPrimitives(prims, "thinThick", 4, "FFFFFF")
	if len(gaps3) != 1 {
		t.Fatalf("thinThick gaps = %d, want 1", len(gaps3))
	}

	// tri
	gaps4 := compoundLineGapPrimitives(prims, "tri", 4, "FFFFFF")
	if len(gaps4) != 2 {
		t.Fatalf("tri gaps = %d, want 2", len(gaps4))
	}
}

func TestCompoundLineGapPrimitivesEmpty(t *testing.T) {
	prims := []VectorPrimitive{{Kind: PrimitivePolyline, Points: []Point{{X: 0, Y: 0}, {X: 100, Y: 0}}}}

	if got := compoundLineGapPrimitives(prims, "sng", 4, "FFFFFF"); got != nil {
		t.Errorf("sng = %v, want nil", got)
	}
	if got := compoundLineGapPrimitives(prims, "", 4, "FFFFFF"); got != nil {
		t.Errorf("empty = %v, want nil", got)
	}
	if got := compoundLineGapPrimitives(prims, "dbl", 4, ""); got != nil {
		t.Errorf("empty gap color = %v, want nil", got)
	}
	if got := compoundLineGapPrimitives(prims, "dbl", 0, "FFFFFF"); got != nil {
		t.Errorf("zero width = %v, want nil", got)
	}
	if got := compoundLineGapPrimitives(prims, "unknown", 4, "FFFFFF"); got != nil {
		t.Errorf("unknown type = %v, want nil", got)
	}
}

func TestCreateCubicBezierPathFewPoints(t *testing.T) {
	// 2 points: fallback to line
	path := createCubicBezierPath([]Point{{X: 0, Y: 0}, {X: 100, Y: 0}})
	if len(path.Commands) != 2 {
		t.Fatalf("commands = %d, want 2", len(path.Commands))
	}
	if path.Commands[0].Type != "M" || path.Commands[1].Type != "L" {
		t.Errorf("types = %s %s, want M L", path.Commands[0].Type, path.Commands[1].Type)
	}

	// 3 points: fallback to line
	path2 := createCubicBezierPath([]Point{{X: 0, Y: 0}, {X: 50, Y: 0}, {X: 100, Y: 0}})
	if len(path2.Commands) != 3 {
		t.Fatalf("commands = %d, want 3", len(path2.Commands))
	}
}

func TestCreateCubicBezierPathFourPoints(t *testing.T) {
	path := createCubicBezierPath([]Point{
		{X: 0, Y: 0}, {X: 30, Y: 0}, {X: 70, Y: 50}, {X: 100, Y: 50},
	})
	if len(path.Commands) != 2 {
		t.Fatalf("commands = %d, want 2", len(path.Commands))
	}
	if path.Commands[0].Type != "M" || path.Commands[1].Type != "C" {
		t.Errorf("types = %s %s, want M C", path.Commands[0].Type, path.Commands[1].Type)
	}
	if len(path.Commands[1].Points) != 3 {
		t.Errorf("C points = %d, want 3", len(path.Commands[1].Points))
	}
}

func TestCreateCubicBezierPathManyPoints(t *testing.T) {
	path := createCubicBezierPath([]Point{
		{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 40, Y: 20},
		{X: 60, Y: 20}, {X: 80, Y: 40}, {X: 100, Y: 50},
	})
	if len(path.Commands) < 2 {
		t.Fatalf("commands = %d, want >= 2", len(path.Commands))
	}
	if path.Commands[0].Type != "M" {
		t.Errorf("first type = %s, want M", path.Commands[0].Type)
	}
	if path.Commands[1].Type != "C" {
		t.Errorf("second type = %s, want C", path.Commands[1].Type)
	}
}

func TestCreateCubicBezierPathSevenPoints(t *testing.T) {
	// 7 points triggers the inner loop: i=3, len(points)-3=4, i+3=6 < 7
	path := createCubicBezierPath([]Point{
		{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 20, Y: 10},
		{X: 30, Y: 10}, {X: 40, Y: 20}, {X: 50, Y: 20}, {X: 60, Y: 30},
	})
	if len(path.Commands) < 3 {
		t.Fatalf("commands = %d, want >= 3", len(path.Commands))
	}
	if path.Commands[2].Type != "C" {
		t.Errorf("third type = %s, want C", path.Commands[2].Type)
	}
}

func TestCreateCubicBezierPathSinglePoint(t *testing.T) {
	path := createCubicBezierPath([]Point{{X: 0, Y: 0}})
	if len(path.Commands) != 0 {
		t.Errorf("commands = %d, want 0", len(path.Commands))
	}
}

func TestArrowPrimitiveDefault(t *testing.T) {
	// Use an arrow type that ComputeArrowGeometry returns with an unknown type string
	// We can test via arrowPrimitive directly by forcing a type it doesn't know
	_, ok := arrowPrimitive("unknownArrow", "med", "med", "000000", Point{X: 100, Y: 0}, Point{X: 0, Y: 0})
	if ok {
		t.Error("expected false for unknown arrow type in arrowPrimitive")
	}
}

func TestBuildShapePrimitivesMaterializesFreeformLineEndArrow(t *testing.T) {
	geom := &Geometry{Type: GeometryTypeCustom, Paths: []Path{{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 0}}},
		{Type: "L", Points: []Point{{X: 50, Y: 25}}},
		{Type: "L", Points: []Point{{X: 100, Y: 50}}},
	}}}}
	prims, err := BuildShapePrimitives(ShapeGeometrySpec{
		Geometry: geom, Width: 100, Height: 50,
		StrokeColor: "FFFF00", StrokeWidth: 3,
		ArrowTail: "triangle", ArrowTailWidth: "med", ArrowTailLength: "lg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prims) != 2 {
		t.Fatalf("primitives = %d, want path + endpoint marker", len(prims))
	}
	marker := prims[1]
	if marker.Kind != PrimitivePolyline || !marker.Closed || len(marker.Points) != 3 {
		t.Fatalf("marker = %+v, want closed triangle", marker)
	}
	if marker.Style.FillColor != "FFFF00" {
		t.Fatalf("marker color = %q, want FFFF00", marker.Style.FillColor)
	}
	pathEnd := prims[0].Path.Commands[len(prims[0].Path.Commands)-1].Points[0]
	if marker.Points[0] != pathEnd {
		t.Fatalf("marker tip = %+v, want normalized path end %+v", marker.Points[0], pathEnd)
	}
}

func TestBuildShapePrimitivesUsesCurveTangentForFreeformArrow(t *testing.T) {
	geom := &Geometry{Type: GeometryTypeCustom, Paths: []Path{{Commands: []PathCommand{
		{Type: "M", Points: []Point{{X: 0, Y: 50}}},
		{Type: "C", Points: []Point{{X: 20, Y: 0}, {X: 80, Y: 0}, {X: 100, Y: 50}}},
	}}}}
	prims, err := BuildShapePrimitives(ShapeGeometrySpec{
		Geometry: geom, Width: 100, Height: 50,
		StrokeColor: "000000", StrokeWidth: 2,
		ArrowTail: "triangle", ArrowTailWidth: "med", ArrowTailLength: "med",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prims) != 2 || len(prims[1].Points) != 3 {
		t.Fatalf("curve marker primitives = %+v", prims)
	}
	// The last cubic control point is horizontally left of the endpoint, so the
	// triangle must point to the right rather than using the path's start point.
	tip := prims[1].Points[0]
	left, right := prims[1].Points[1], prims[1].Points[2]
	pathEnd := prims[0].Path.Commands[len(prims[0].Path.Commands)-1].Points[2]
	if tip != pathEnd || left.X >= tip.X || right.X >= tip.X {
		t.Fatalf("curve endpoint marker does not follow terminal tangent: tip=%+v pathEnd=%+v left=%+v right=%+v", tip, pathEnd, left, right)
	}
}

func TestBuildBentConnectorPrimitivesTrimsLargeArrowAcrossTerminalDogleg(t *testing.T) {
	primitives := BuildConnectorPrimitives(ConnectorSpec{
		Type: "bentConnector3", BoundsW: 100, BoundsH: 40,
		Adjustments: []int{5000},
		LineWidthPt: 2,
		ArrowHead:   "triangle", ArrowHeadLength: "lg", ArrowHeadWidth: "lg",
	})
	if len(primitives) < 2 || primitives[0].Kind != PrimitivePolyline {
		t.Fatalf("connector primitives = %#v", primitives)
	}
	points := primitives[0].Points
	// The first dogleg is only 5pt wide while the 2pt/lg triangle is 12pt
	// deep. The shaft therefore consumes the 5pt segment and continues 7pt
	// down the next segment instead of clamping at the elbow.
	if len(points) < 2 || math.Abs(points[0].X-5) > 1e-9 || math.Abs(points[0].Y-7) > 1e-9 {
		t.Fatalf("trimmed bent connector starts at %#v, want (5,7)", points)
	}
}
