package drawing

import (
	"math"
	"reflect"
	"testing"
)

func TestEMUToPoints(t *testing.T) {
	// 914400 EMU = 1 inch = 72 points
	if got := EMUToPoints(914400); got != 72.0 {
		t.Errorf("EMUToPoints(914400) = %v, want 72.0", got)
	}
	// 12700 EMU = 1 point
	if got := EMUToPoints(12700); got != 1.0 {
		t.Errorf("EMUToPoints(12700) = %v, want 1.0", got)
	}
}

func TestPointsToEMU(t *testing.T) {
	if got := PointsToEMU(72.0); got != 914400 {
		t.Errorf("PointsToEMU(72.0) = %v, want 914400", got)
	}
	if got := PointsToEMU(1.0); got != 12700 {
		t.Errorf("PointsToEMU(1.0) = %v, want 12700", got)
	}
}

func TestTwipsToPoints(t *testing.T) {
	if got := TwipsToPoints(20); got != 1.0 {
		t.Errorf("TwipsToPoints(20) = %v, want 1.0", got)
	}
	if got := TwipsToPoints(0); got != 0.0 {
		t.Errorf("TwipsToPoints(0) = %v, want 0.0", got)
	}
}

func TestPointsToTwips(t *testing.T) {
	if got := PointsToTwips(1.0); got != 20 {
		t.Errorf("PointsToTwips(1.0) = %v, want 20", got)
	}
	if got := PointsToTwips(0.0); got != 0 {
		t.Errorf("PointsToTwips(0.0) = %v, want 0", got)
	}
}

func TestPresetGeometryBoundingBox(t *testing.T) {
	pg := PresetGeometry{Name: "rect"}
	rect := pg.BoundingBox(100, 50)
	if rect.X != 0 || rect.Y != 0 || rect.Width != 100 || rect.Height != 50 {
		t.Errorf("BoundingBox = %+v", rect)
	}
}

func TestApplyRect(t *testing.T) {
	// Translation
	tx := Translate(10, 20)
	rect := tx.ApplyRect(Rect{X: 0, Y: 0, Width: 100, Height: 50})
	if rect.X != 10 || rect.Y != 20 || rect.Width != 100 || rect.Height != 50 {
		t.Errorf("Translate.ApplyRect = %+v", rect)
	}

	// Scale
	sx := Scale(2, 3)
	rect2 := sx.ApplyRect(Rect{X: 0, Y: 0, Width: 100, Height: 50})
	if rect2.Width != 200 || rect2.Height != 150 {
		t.Errorf("Scale.ApplyRect = %+v", rect2)
	}

	// Rotation (45 degrees) - bounding box should expand
	rot := Rotate(45)
	rect3 := rot.ApplyRect(Rect{X: 0, Y: 0, Width: 100, Height: 0})
	// A 100-unit line at 45° has bounding box ~71x71
	if rect3.Width < 70 || rect3.Height < 70 {
		t.Errorf("Rotate(45).ApplyRect = %+v, expected larger bbox", rect3)
	}
}

func TestDefaultLineStyle(t *testing.T) {
	ls := DefaultLineStyle()
	if ls.Width != 1.0 || ls.Color != "000000" || ls.Alpha != 1.0 {
		t.Errorf("DefaultLineStyle = %+v", ls)
	}
	if ls.Cap != LineCapFlat || ls.Join != LineJoinMiter || ls.MiterLimit != 10.0 {
		t.Errorf("DefaultLineStyle cap/join = %+v", ls)
	}
	if ls.Dash != DashPatternSolid {
		t.Errorf("DefaultLineStyle dash = %q, want solid", ls.Dash)
	}
}

func TestPresetGeometryBulkMoreShapes(t *testing.T) {
	shapes := []string{
		"rect", "roundRect", "ellipse", "triangle", "rightTriangle",
		"diamond", "parallelogram", "trapezoid", "pentagon", "hexagon",
		"octagon", "decagon", "dodecagon", "star4", "star6", "star7", "star8",
		"star10", "star12", "star16", "star24", "star32",
		"pie", "chord", "arc", "blockArc", "donut",
		"can", "cube", "cylinder", "bevel",
		"flowChartProcess", "flowChartDecision", "flowChartDocument",
		"flowChartInputOutput", "flowChartPredefinedProcess",
		"flowChartInternalStorage", "flowChartManualOperation",
		"flowChartOffPageConnector", "flowChartPunchedCard",
		"flowChartSort", "flowChartMerge", "flowChartExtract",
		"flowChartStoredData", "flowChartDelay", "flowChartMagneticTape",
		"flowChartMagneticDisk", "flowChartDisplay",
		"wedgeRectCallout", "wedgeRoundRectCallout", "wedgeEllipseCallout",
		"cloudCallout", "borderCallout1", "borderCallout2", "borderCallout3",
		"accentCallout1", "accentCallout2", "accentCallout3",
		"accentBorderCallout1", "accentBorderCallout2", "accentBorderCallout3",
		"leftRightArrow", "upDownArrow", "leftUpArrow", "bentUpArrow",
		"curvedRightArrow", "curvedLeftArrow", "curvedUpArrow", "curvedDownArrow",
		"stripedRightArrow", "notchedRightArrow", "homePlate", "chevron",
		" Pentagon", "hexagon", "heptagon", "octagon",
		"decagon", "dodecagon", "teardrop", "swooshArrow", "circularArrow",
		"leftCircularArrow", "leftRightCircularArrow",
		"curvedConnector2", "curvedConnector3", "curvedConnector4", "curvedConnector5",
		"straightConnector1", "bentConnector2", "bentConnector3", "bentConnector4", "bentConnector5",
		"plus", "minus", "multiply", "division", "equal",
		"corner", "diagStripe", "chord", "cube", "can",
		"lightningBolt", "heart", "sun", "moon", "cloud",
		"smileyFace", "donut", "noSmoking", "foldedCorner",
		"bevel", "frame", "plaque", "bracketPair", "bracePair",
		"leftBracket", "rightBracket", "leftBrace", "rightBrace",
		"irregularSeal1", "irregularSeal2", "cross",
		"mathPlus", "mathMinus", "mathMultiply", "mathDivide",
		"mathEqual", "mathNotEqual",
		"gear6", "gear9",
	}
	for _, name := range shapes {
		pg := PresetGeometry{Name: name}
		path, err := pg.GetPath(100, 60)
		if err != nil {
			t.Errorf("%s: GetPath error: %v", name, err)
			continue
		}
		if len(path.Commands) == 0 {
			t.Errorf("%s: empty path", name)
		}
	}
}

func TestRotatePoints(t *testing.T) {
	pts := [][2]float64{{10, 0}, {0, 10}}
	// Rotate 90° around center of 20x20 box
	rotated := rotatePoints(pts, 20, 20, 90)
	if len(rotated) != 2 {
		t.Fatalf("len = %d, want 2", len(rotated))
	}
	// (10,0) rotated 90° around (10,10) → approx (20, 10)
	if math.Abs(rotated[0][0]-20) > 0.001 || math.Abs(rotated[0][1]-10) > 0.001 {
		t.Errorf("rotated[0] = (%v,%v), want ~(20,10)", rotated[0][0], rotated[0][1])
	}
}

func TestRotatePointsZeroAngle(t *testing.T) {
	pts := [][2]float64{{5, 5}}
	rotated := rotatePoints(pts, 100, 100, 0)
	if rotated[0][0] != 5 || rotated[0][1] != 5 {
		t.Errorf("zero angle = (%v,%v), want (5,5)", rotated[0][0], rotated[0][1])
	}
}

func TestPresetGeometryAdjustments(t *testing.T) {
	// can with adjustment
	pg := PresetGeometry{Name: "can", Adjustments: []float64{0.2}}
	path, err := pg.GetPath(100, 50)
	if err != nil {
		t.Fatalf("can adj: %v", err)
	}
	if len(path.Commands) == 0 {
		t.Error("can adj: empty path")
	}

	// donut with adjustment
	pg2 := PresetGeometry{Name: "donut", Adjustments: []float64{0.3}}
	path2, err2 := pg2.GetPath(100, 100)
	if err2 != nil {
		t.Fatalf("donut adj: %v", err2)
	}
	if len(path2.Commands) == 0 {
		t.Error("donut adj: empty path")
	}

	// frame with adjustment
	pg3 := PresetGeometry{Name: "frame", Adjustments: []float64{0.1}}
	path3, err3 := pg3.GetPath(100, 50)
	if err3 != nil {
		t.Fatalf("frame adj: %v", err3)
	}
	if len(path3.Commands) == 0 {
		t.Error("frame adj: empty path")
	}

	// snip1Rect with adjustment
	pg4 := PresetGeometry{Name: "snip1Rect", Adjustments: []float64{0.2}}
	path4, err4 := pg4.GetPath(100, 50)
	if err4 != nil {
		t.Fatalf("snip1Rect adj: %v", err4)
	}
	if len(path4.Commands) == 0 {
		t.Error("snip1Rect adj: empty path")
	}

	// roundRect with adjustment
	pg5 := PresetGeometry{Name: "roundRect", Adjustments: []float64{0.2}}
	path5, err5 := pg5.GetPath(100, 50)
	if err5 != nil {
		t.Fatalf("roundRect adj: %v", err5)
	}
	if len(path5.Commands) == 0 {
		t.Error("roundRect adj: empty path")
	}

	// parallelogram with adjustment
	pg6 := PresetGeometry{Name: "parallelogram", Adjustments: []float64{0.2}}
	path6, err6 := pg6.GetPath(100, 50)
	if err6 != nil {
		t.Fatalf("parallelogram adj: %v", err6)
	}
	if len(path6.Commands) == 0 {
		t.Error("parallelogram adj: empty path")
	}

	// trapezoid with adjustment
	pg7 := PresetGeometry{Name: "trapezoid", Adjustments: []float64{0.2}}
	path7, err7 := pg7.GetPath(100, 50)
	if err7 != nil {
		t.Fatalf("trapezoid adj: %v", err7)
	}
	if len(path7.Commands) == 0 {
		t.Error("trapezoid adj: empty path")
	}

	// chevron with adjustment
	pg8 := PresetGeometry{Name: "chevron", Adjustments: []float64{0.2}}
	path8, err8 := pg8.GetPath(100, 50)
	if err8 != nil {
		t.Fatalf("chevron adj: %v", err8)
	}
	if len(path8.Commands) == 0 {
		t.Error("chevron adj: empty path")
	}

	// pie with adjustment
	pg9 := PresetGeometry{Name: "pie", Adjustments: []float64{0.2, 0.8}}
	path9, err9 := pg9.GetPath(100, 100)
	if err9 != nil {
		t.Fatalf("pie adj: %v", err9)
	}
	if len(path9.Commands) == 0 {
		t.Error("pie adj: empty path")
	}

	// chord with adjustment
	pg10 := PresetGeometry{Name: "chord", Adjustments: []float64{0.2, 0.8}}
	path10, err10 := pg10.GetPath(100, 100)
	if err10 != nil {
		t.Fatalf("chord adj: %v", err10)
	}
	if len(path10.Commands) == 0 {
		t.Error("chord adj: empty path")
	}

	// blockArc with adjustment
	pg11 := PresetGeometry{Name: "blockArc", Adjustments: []float64{0.2, 0.8, 0.5}}
	path11, err11 := pg11.GetPath(100, 100)
	if err11 != nil {
		t.Fatalf("blockArc adj: %v", err11)
	}
	if len(path11.Commands) == 0 {
		t.Error("blockArc adj: empty path")
	}
}

func TestPresetGeometryMissingShapes(t *testing.T) {
	for _, name := range []string{"rtTriangle", "leftRightArrowCallout"} {
		pg := PresetGeometry{Name: name}
		path, err := pg.GetPath(100, 60)
		if err != nil {
			t.Errorf("%s: GetPath error: %v", name, err)
			continue
		}
		if len(path.Commands) == 0 {
			t.Errorf("%s: empty path", name)
		}
	}
}

func TestPresetGeometryMoreShapes(t *testing.T) {
	for _, name := range []string{"leftArrow", "upArrow", "downArrow", "flowChartConnector", "flowChartPunchedTape"} {
		pg := PresetGeometry{Name: name}
		path, err := pg.GetPath(100, 60)
		if err != nil {
			t.Errorf("%s: GetPath error: %v", name, err)
			continue
		}
		if len(path.Commands) == 0 {
			t.Errorf("%s: empty path", name)
		}
	}
}

func TestPresetGeometryEdgeAdjustments(t *testing.T) {
	tests := []struct {
		name string
		adj  []float64
	}{
		{"can", []float64{0.01}},           // clamp lower to 0.05
		{"donut", []float64{0.01}},         // clamp lower to 0.1
		{"frame", []float64{0.01}},         // clamp lower
		{"snip1Rect", []float64{0.6}},      // large snip
		{"snip2SameRect", []float64{0.6}},  // large snip
		{"snip2DiagRect", []float64{0.6}},  // large snip
		{"snipRoundRect", []float64{0.6}},  // large snip
		{"round1Rect", []float64{0.6}},     // large radius
		{"round2SameRect", []float64{0.6}}, // large radius
		{"round2DiagRect", []float64{0.6}}, // large radius
		{"notch", []float64{0.6}},          // large notch
		{"homePlate", []float64{0.6}},      // large adjustment
		{"foldedCorner", []float64{0.6}},   // large fold
		{"bevel", []float64{0.6}},          // large bevel
		{"roundRect", []float64{0.01}},     // small radius
	}
	for _, tt := range tests {
		pg := PresetGeometry{Name: tt.name, Adjustments: tt.adj}
		path, err := pg.GetPath(100, 50)
		if err != nil {
			t.Errorf("%s: GetPath error: %v", tt.name, err)
			continue
		}
		if len(path.Commands) == 0 {
			t.Errorf("%s: empty path", tt.name)
		}
	}
}

func TestPresetGeometryUpperClampAdjustments(t *testing.T) {
	tests := []struct {
		name string
		adj  []float64
	}{
		{"can", []float64{0.6}},       // clamp upper to 0.5
		{"donut", []float64{0.95}},    // clamp upper to 0.9
		{"frame", []float64{0.6}},     // clamp upper
		{"roundRect", []float64{0.8}}, // clamp upper
	}
	for _, tt := range tests {
		pg := PresetGeometry{Name: tt.name, Adjustments: tt.adj}
		path, err := pg.GetPath(100, 50)
		if err != nil {
			t.Errorf("%s: GetPath error: %v", tt.name, err)
			continue
		}
		if len(path.Commands) == 0 {
			t.Errorf("%s: empty path", tt.name)
		}
	}
}

func TestPresetGeometryCanNoAdjustments(t *testing.T) {
	pg := PresetGeometry{Name: "can"}
	path, err := pg.GetPath(100, 50)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(path.Commands) == 0 {
		t.Error("empty path")
	}
}

func TestPresetGeometryDonutNoAdjustments(t *testing.T) {
	pg := PresetGeometry{Name: "donut"}
	path, err := pg.GetPath(100, 50)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(path.Commands) == 0 {
		t.Error("empty path")
	}
}

func TestPresetGeometryFrameNoAdjustments(t *testing.T) {
	pg := PresetGeometry{Name: "frame"}
	path, err := pg.GetPath(100, 50)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(path.Commands) == 0 {
		t.Error("empty path")
	}
}

func TestPresetGeometrySnipRoundRectNoAdjustments(t *testing.T) {
	pg := PresetGeometry{Name: "snipRoundRect"}
	path, err := pg.GetPath(100, 50)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(path.Commands) == 0 {
		t.Error("empty path")
	}
}

func TestPresetGeometryRoundRectNoAdjustments(t *testing.T) {
	pg := PresetGeometry{Name: "roundRect"}
	path, err := pg.GetPath(100, 50)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(path.Commands) == 0 {
		t.Error("empty path")
	}
}

func TestPresetGeometryRoundRectClamped(t *testing.T) {
	// Large adjustment should clamp to maxRadius = min(width,height)/2
	pg := PresetGeometry{Name: "roundRect", Adjustments: []float64{0.8}}
	path, err := pg.GetPath(100, 50)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(path.Commands) == 0 {
		t.Error("empty path")
	}
}

func TestPresetGeometrySnipRoundRectTwoAdjustments(t *testing.T) {
	pg := PresetGeometry{Name: "snipRoundRect", Adjustments: []float64{0.2, 0.1}}
	path, err := pg.GetPath(100, 50)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(path.Commands) == 0 {
		t.Error("empty path")
	}
}

func TestApplyRectRotated(t *testing.T) {
	// Rotation should transform corners and produce a bounding box larger than the original
	tx := Rotate(45)
	rect := Rect{X: 0, Y: 0, Width: 100, Height: 50}
	result := tx.ApplyRect(rect)
	if result.Width <= rect.Width || result.Height <= rect.Height {
		t.Errorf("rotated rect = %+v, want larger than original", result)
	}
}

func TestApplyRectRotated135(t *testing.T) {
	// 135-degree rotation should make some corners have negative Y, hitting the minY branch
	tx := Rotate(135)
	rect := Rect{X: 0, Y: 0, Width: 100, Height: 50}
	result := tx.ApplyRect(rect)
	if result.Y >= 0 {
		t.Errorf("rotated135 Y = %v, want negative", result.Y)
	}
}

func TestBoundingBoxEmpty(t *testing.T) {
	p := Path{}
	bb := p.BoundingBox()
	if bb != (Rect{}) {
		t.Errorf("empty path bbox = %+v, want zero", bb)
	}
}

func TestBoundingBoxNoPoints(t *testing.T) {
	p := Path{Commands: []PathCommand{{Type: "Z"}}}
	bb := p.BoundingBox()
	if bb != (Rect{}) {
		t.Errorf("no-points path bbox = %+v, want zero", bb)
	}
}

func TestApplyRectScale(t *testing.T) {
	s := Scale(2, 3)
	rect := s.ApplyRect(Rect{X: 0, Y: 0, Width: 100, Height: 50})
	if rect.Width != 200 || rect.Height != 150 {
		t.Errorf("scale = %+v", rect)
	}
}

func TestBoundingBox(t *testing.T) {
	pg := PresetGeometry{Name: "rect"}
	rect := pg.BoundingBox(100, 50)
	if rect.X != 0 || rect.Y != 0 || rect.Width != 100 || rect.Height != 50 {
		t.Errorf("BoundingBox = %+v", rect)
	}
}

func TestHomePlateDefaultTipUsesHalfShortSide(t *testing.T) {
	pg := PresetGeometry{Name: "homePlate"}
	path, err := pg.GetPath(600, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(path.Commands) < 4 {
		t.Fatalf("commands = %d", len(path.Commands))
	}
	// Default DrawingML adj=50000 => dx = 50% of ss (100), so the
	// shoulder is x=550. A width-relative implementation would incorrectly
	// put it near x=420 and produce a shallow SmartArt banner wedge.
	if got := path.Commands[1].Points[0].X; math.Abs(got-550) > 0.001 {
		t.Fatalf("homePlate shoulder x = %v, want 550", got)
	}
	if got := path.Commands[2].Points[0].X; math.Abs(got-600) > 0.001 {
		t.Fatalf("homePlate tip x = %v, want 600", got)
	}
}

func TestBlockArcPreservesAuthoredStartEndAndThickness(t *testing.T) {
	pg := PresetGeometry{Name: "blockArc", Adjustments: []float64{0.25, 0.75, 0.20}}
	path, err := pg.GetPath(200, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(path.Commands) != 5 {
		t.Fatalf("commands = %d, want 5", len(path.Commands))
	}
	start := path.Commands[0].Points[0]
	// 0.25 turn is the bottom centre in the renderer's clockwise coordinate system.
	if math.Abs(start.X-100) > 1e-6 || math.Abs(start.Y-100) > 1e-6 {
		t.Fatalf("outer start = (%v,%v), want (100,100)", start.X, start.Y)
	}
	end := path.Commands[1].Points[0]
	if math.Abs(end.X-100) > 1e-6 || math.Abs(end.Y-0) > 1e-6 {
		t.Fatalf("outer end = (%v,%v), want (100,0)", end.X, end.Y)
	}
	innerEnd := path.Commands[2].Points[0]
	// 20%% radial thickness leaves an 80%% inner ellipse.
	if math.Abs(innerEnd.X-100) > 1e-6 || math.Abs(innerEnd.Y-10) > 1e-6 {
		t.Fatalf("inner end = (%v,%v), want (100,10)", innerEnd.X, innerEnd.Y)
	}
}

func TestBlockArcWrapsAngularHandlesWithoutClamping(t *testing.T) {
	pg := PresetGeometry{Name: "blockArc", Adjustments: []float64{0.75, 0.25, 0.25}}
	path, err := pg.GetPath(100, 100)
	if err != nil {
		t.Fatal(err)
	}
	arc := path.Commands[1]
	if len(arc.Args) < 5 {
		t.Fatalf("outer arc args = %#v", arc.Args)
	}
	if arc.Args[3] != 0 {
		t.Fatalf("large-arc = %v, want 0 for half turn", arc.Args[3])
	}
	start := path.Commands[0].Points[0]
	end := arc.Points[0]
	if math.Abs(start.X-50) > 1e-6 || math.Abs(start.Y-0) > 1e-6 {
		t.Fatalf("start = (%v,%v), want (50,0)", start.X, start.Y)
	}
	if math.Abs(end.X-50) > 1e-6 || math.Abs(end.Y-100) > 1e-6 {
		t.Fatalf("end = (%v,%v), want (50,100)", end.X, end.Y)
	}
}

func TestClass13NumberedCalloutsOwnLeaderGeometry(t *testing.T) {
	for _, name := range []string{"callout1", "callout2", "callout3", "borderCallout1", "accentCallout1", "accentBorderCallout3"} {
		path, err := (&PresetGeometry{Name: name}).GetPath(216, 108)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(path.Commands) < 7 {
			t.Fatalf("%s: commands=%d, want rectangular body plus leader", name, len(path.Commands))
		}
		first := path.Commands[0]
		if first.Type != "M" || len(first.Points) != 1 || first.Points[0] != (Point{X: 0, Y: 0}) {
			t.Fatalf("%s: first command=%+v, numbered callout must retain rectangular body", name, first)
		}
		if path.Commands[4].Type != "Z" {
			t.Fatalf("%s: body is not closed before leader: %+v", name, path.Commands)
		}
	}
}

func TestClass13NumberedCalloutLeaderDepthIsDistinct(t *testing.T) {
	counts := map[string]int{}
	for _, name := range []string{"callout1", "callout2", "callout3"} {
		path, err := (&PresetGeometry{Name: name}).GetPath(160, 90)
		if err != nil {
			t.Fatal(err)
		}
		counts[name] = len(path.Commands)
	}
	if !(counts["callout1"] < counts["callout2"] && counts["callout2"] < counts["callout3"]) {
		t.Fatalf("leader topology collapsed: %#v", counts)
	}
}

func TestClass13AccentCalloutAddsAccentBar(t *testing.T) {
	plain, _ := (&PresetGeometry{Name: "callout2"}).GetPath(180, 100)
	accent, _ := (&PresetGeometry{Name: "accentCallout2"}).GetPath(180, 100)
	if len(accent.Commands) != len(plain.Commands)+2 {
		t.Fatalf("accent commands=%d plain=%d, want dedicated accent subpath", len(accent.Commands), len(plain.Commands))
	}
}

func TestClass13NumberedCalloutsRespectAdjustmentsAndNonSquareBounds(t *testing.T) {
	base, _ := (&PresetGeometry{Name: "callout3"}).GetPath(240, 80)
	adjusted, _ := (&PresetGeometry{Name: "callout3", Adjustments: []float64{0.8, 0.2, 0.7, 0.3, 0.6, 0.4}}).GetPath(240, 80)
	if reflect.DeepEqual(base.Commands, adjusted.Commands) {
		t.Fatal("callout3 adjustments did not change leader geometry")
	}
	portrait, _ := (&PresetGeometry{Name: "accentBorderCallout3"}).GetPath(80, 240)
	if len(portrait.Commands) == 0 {
		t.Fatal("portrait callout geometry is empty")
	}
	body := portrait.Commands[:5]
	if body[1].Points[0].X != 80 || body[2].Points[0].Y != 240 {
		t.Fatalf("portrait body did not normalize to host bounds: %+v", body)
	}
}

func TestClass13WedgeCalloutHostsAreDistinct(t *testing.T) {
	names := []string{"wedgeRectCallout", "wedgeRoundRectCallout", "wedgeEllipseCallout"}
	paths := make(map[string]Path, len(names))
	for _, name := range names {
		p, err := (&PresetGeometry{Name: name}).GetPath(200, 100)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		paths[name] = p
	}
	if reflect.DeepEqual(paths[names[0]].Commands, paths[names[1]].Commands) ||
		reflect.DeepEqual(paths[names[1]].Commands, paths[names[2]].Commands) ||
		reflect.DeepEqual(paths[names[0]].Commands, paths[names[2]].Commands) {
		t.Fatal("wedge callout host geometries collapsed to a shared approximation")
	}
	if paths["wedgeEllipseCallout"].Commands[0].Type == "M" && len(paths["wedgeEllipseCallout"].Commands) <= 6 {
		t.Fatal("wedgeEllipseCallout appears to have fallen back to rectangle geometry")
	}
}

func TestClass13RemainingDistinctPresetsDoNotFallBackToRectangle(t *testing.T) {
	names := []string{"teardrop", "swooshArrow", "leftCircularArrow", "leftRightCircularArrow", "corner", "diagStripe"}
	rect, _ := (&PresetGeometry{Name: "rect"}).GetPath(21600, 10800)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got, err := (&PresetGeometry{Name: name}).GetPath(21600, 10800)
			if err != nil {
				t.Fatal(err)
			}
			if reflect.DeepEqual(got, rect) {
				t.Fatalf("%s still falls back to rectangle", name)
			}
			if len(got.Commands) < 4 {
				t.Fatalf("%s geometry too small: %d commands", name, len(got.Commands))
			}
		})
	}
}

func TestClass13RemainingDistinctPresetsSurvivePortraitBounds(t *testing.T) {
	for _, name := range []string{"teardrop", "swooshArrow", "leftCircularArrow", "leftRightCircularArrow", "corner", "diagStripe"} {
		p, err := (&PresetGeometry{Name: name}).GetPath(10800, 21600)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(p.Commands) == 0 {
			t.Fatalf("%s returned empty portrait geometry", name)
		}
	}
}

func TestClass13RemainingPresetAdjustmentsAffectGeometry(t *testing.T) {
	for _, name := range []string{"teardrop", "swooshArrow", "leftCircularArrow", "corner", "diagStripe"} {
		a, _ := (&PresetGeometry{Name: name, Adjustments: []float64{.12}}).GetPath(21600, 10800)
		b, _ := (&PresetGeometry{Name: name, Adjustments: []float64{.38}}).GetPath(21600, 10800)
		if reflect.DeepEqual(a, b) {
			t.Fatalf("%s ignores its primary adjustment", name)
		}
	}
}
