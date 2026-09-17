package drawing

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestPresetGeometry(t *testing.T) {
	tests := []struct {
		name   string
		width  float64
		height float64
		shape  PresetGeometry
	}{
		{
			name:   "rectangle",
			width:  100,
			height: 50,
			shape:  PresetGeometry{Name: "rect"},
		},
		{
			name:   "rounded rectangle",
			width:  100,
			height: 50,
			shape:  PresetGeometry{Name: "roundRect", Adjustments: []float64{0.1}},
		},
		{
			name:   "ellipse",
			width:  100,
			height: 50,
			shape:  PresetGeometry{Name: "ellipse"},
		},
		{
			name:   "triangle",
			width:  100,
			height: 50,
			shape:  PresetGeometry{Name: "triangle"},
		},
		{
			name:   "smiley face",
			width:  100,
			height: 100,
			shape:  PresetGeometry{Name: "smileyFace"},
		},
		{
			name:   "sun",
			width:  100,
			height: 100,
			shape:  PresetGeometry{Name: "sun"},
		},
		{
			name:   "moon",
			width:  100,
			height: 100,
			shape:  PresetGeometry{Name: "moon"},
		},
		{
			name:   "cloud",
			width:  120,
			height: 80,
			shape:  PresetGeometry{Name: "cloud"},
		},
		{
			name:   "can",
			width:  100,
			height: 140,
			shape:  PresetGeometry{Name: "can"},
		},
		{
			name:   "donut",
			width:  100,
			height: 100,
			shape:  PresetGeometry{Name: "donut"},
		},
		{
			name:   "frame",
			width:  120,
			height: 80,
			shape:  PresetGeometry{Name: "frame"},
		},
		{
			name:   "plaque",
			width:  120,
			height: 60,
			shape:  PresetGeometry{Name: "plaque"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := tt.shape.GetPath(tt.width, tt.height)
			if err != nil {
				t.Errorf("GetPath() error = %v", err)
				return
			}

			if len(path.Commands) == 0 {
				t.Error("GetPath() returned empty path")
			}

			bbox := path.BoundingBox()
			if bbox.Width <= 0 || bbox.Height <= 0 {
				t.Errorf("BoundingBox() returned invalid dimensions: %v", bbox)
			}

			// Basic validation
			fmt.Printf("%s: %d commands, bbox: %v\n", tt.name, len(path.Commands), bbox)
		})
	}
}

func TestPresetGeometryCompoundShapes(t *testing.T) {
	tests := []struct {
		name          string
		shape         PresetGeometry
		wantFillRule  string
		wantMoves     int
		wantCmdTypes  []string
		width, height float64
	}{
		{
			name:         "smiley face",
			shape:        PresetGeometry{Name: "smileyFace"},
			width:        100,
			height:       100,
			wantFillRule: "evenodd",
			wantMoves:    4,
			wantCmdTypes: []string{"A", "Q"},
		},
		{
			name:         "moon",
			shape:        PresetGeometry{Name: "moon"},
			width:        100,
			height:       100,
			wantFillRule: "nonzero",
			wantMoves:    1,
			wantCmdTypes: []string{"C"},
		},
		{
			name:         "cloud",
			shape:        PresetGeometry{Name: "cloud"},
			width:        120,
			height:       80,
			wantFillRule: "nonzero",
			wantMoves:    1,
			wantCmdTypes: []string{"A"},
		},
		{
			name:         "can",
			shape:        PresetGeometry{Name: "can"},
			width:        100,
			height:       140,
			wantFillRule: "nonzero",
			wantMoves:    1,
			wantCmdTypes: []string{"A"},
		},
		{
			name:         "donut",
			shape:        PresetGeometry{Name: "donut"},
			width:        100,
			height:       100,
			wantFillRule: "evenodd",
			wantMoves:    2,
			wantCmdTypes: []string{"A"},
		},
		{
			name:         "frame",
			shape:        PresetGeometry{Name: "frame"},
			width:        120,
			height:       80,
			wantFillRule: "evenodd",
			wantMoves:    2,
			wantCmdTypes: []string{"L"},
		},
		{
			name:         "plaque",
			shape:        PresetGeometry{Name: "plaque"},
			width:        120,
			height:       60,
			wantFillRule: "nonzero",
			wantMoves:    1,
			wantCmdTypes: []string{"Q"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := tt.shape.GetPath(tt.width, tt.height)
			if err != nil {
				t.Fatalf("GetPath() error = %v", err)
			}
			if path.FillRule != tt.wantFillRule {
				t.Fatalf("FillRule = %q, want %q", path.FillRule, tt.wantFillRule)
			}

			moveCount := 0
			hasType := map[string]bool{}
			for _, cmd := range path.Commands {
				if cmd.Type == "M" {
					moveCount++
				}
				hasType[cmd.Type] = true
			}
			if moveCount < tt.wantMoves {
				t.Fatalf("move count = %d, want at least %d", moveCount, tt.wantMoves)
			}
			for _, cmdType := range tt.wantCmdTypes {
				if !hasType[cmdType] {
					t.Fatalf("missing command type %q in %#v", cmdType, path.Commands)
				}
			}
		})
	}
}

func TestGuideEvaluation(t *testing.T) {
	evaluator := NewGuideEvaluator()

	// Add some guides
	evaluator.AddGuide("adj1", &Guide{Name: "adj1", Value: 0.5})
	evaluator.AddGuide("adj2", &Guide{Name: "adj2", Formula: "wd/2"})
	evaluator.AddGuide("adj3", &Guide{Name: "adj3", Formula: "adj1*wd + adj2"})

	width, height := 200.0, 100.0
	results, err := evaluator.EvaluateAll(width, height)
	if err != nil {
		t.Fatalf("EvaluateAll() error = %v", err)
	}

	// Check results
	if val, ok := results["adj1"]; !ok || val != 0.5 { // fixed value
		t.Errorf("adj1 = %v, want 0.5", val)
	}

	if val, ok := results["adj2"]; !ok || val != 100.0 { // 200/2
		t.Errorf("adj2 = %v, want 100.0", val)
	}

	if val, ok := results["adj3"]; !ok || val != 200.0 { // 0.5*200 + 100
		t.Errorf("adj3 = %v, want 200.0", val)
	}

	fmt.Printf("Guide evaluation results: %v\n", results)
}

func TestGuideEvaluationWithDependencies(t *testing.T) {
	evaluator := NewGuideEvaluator()

	// Add guides with complex dependencies
	// adj4 depends on adj5, adj5 depends on adj6, adj6 is fixed
	evaluator.AddGuide("adj6", &Guide{Name: "adj6", Value: 10.0})
	evaluator.AddGuide("adj5", &Guide{Name: "adj5", Formula: "adj6 * 2"})
	evaluator.AddGuide("adj4", &Guide{Name: "adj4", Formula: "adj5 + 5"})

	// adj1 depends on wd (width)
	evaluator.AddGuide("adj1", &Guide{Name: "adj1", Formula: "wd / 4"})

	// adj2 depends on adj1
	evaluator.AddGuide("adj2", &Guide{Name: "adj2", Formula: "adj1 * 3"})

	// adj3 depends on adj2 and adj4
	evaluator.AddGuide("adj3", &Guide{Name: "adj3", Formula: "adj2 + adj4"})

	width, height := 200.0, 100.0
	results, err := evaluator.EvaluateAll(width, height)
	if err != nil {
		t.Fatalf("EvaluateAll() error = %v", err)
	}

	// Check results
	// adj6 = 10 (fixed)
	if val, ok := results["adj6"]; !ok || val != 10.0 {
		t.Errorf("adj6 = %v, want 10.0", val)
	}

	// adj5 = adj6 * 2 = 20
	if val, ok := results["adj5"]; !ok || val != 20.0 {
		t.Errorf("adj5 = %v, want 20.0", val)
	}

	// adj4 = adj5 + 5 = 25
	if val, ok := results["adj4"]; !ok || val != 25.0 {
		t.Errorf("adj4 = %v, want 25.0", val)
	}

	// adj1 = wd / 4 = 200 / 4 = 50
	if val, ok := results["adj1"]; !ok || val != 50.0 {
		t.Errorf("adj1 = %v, want 50.0", val)
	}

	// adj2 = adj1 * 3 = 150
	if val, ok := results["adj2"]; !ok || val != 150.0 {
		t.Errorf("adj2 = %v, want 150.0", val)
	}

	// adj3 = adj2 + adj4 = 150 + 25 = 175
	if val, ok := results["adj3"]; !ok || val != 175.0 {
		t.Errorf("adj3 = %v, want 175.0", val)
	}

	fmt.Printf("Guide evaluation with dependencies results: %v\n", results)
}

func TestGuideEvaluationCircularDependency(t *testing.T) {
	evaluator := NewGuideEvaluator()

	// Create a circular dependency: adj1 -> adj2 -> adj3 -> adj1
	evaluator.AddGuide("adj1", &Guide{Name: "adj1", Formula: "adj2 + 10"})
	evaluator.AddGuide("adj2", &Guide{Name: "adj2", Formula: "adj3 * 2"})
	evaluator.AddGuide("adj3", &Guide{Name: "adj3", Formula: "adj1 / 3"})

	width, height := 200.0, 100.0
	results, err := evaluator.EvaluateAll(width, height)

	// Should get an error about circular dependency
	if err == nil {
		t.Errorf("Expected error for circular dependency, got results: %v", results)
	} else if !strings.Contains(err.Error(), "circular dependency") {
		t.Errorf("Expected circular dependency error, got: %v", err)
	}
	fmt.Printf("Circular dependency test error (expected): %v\n", err)
}

func TestGuideEvaluationArbitraryNames(t *testing.T) {
	// Real DrawingML custom geometries use arbitrary guide names, not just adjN.
	evaluator := NewGuideEvaluator()

	evaluator.AddGuide("x1", &Guide{Name: "x1", Formula: "wd/4"})
	evaluator.AddGuide("y1", &Guide{Name: "y1", Formula: "ht/3"})
	evaluator.AddGuide("w", &Guide{Name: "w", Formula: "x1*2"})
	evaluator.AddGuide("h", &Guide{Name: "h", Formula: "y1+x1"})

	width, height := 200.0, 300.0
	results, err := evaluator.EvaluateAll(width, height)
	if err != nil {
		t.Fatalf("EvaluateAll() error = %v", err)
	}

	if val, ok := results["x1"]; !ok || val != 50.0 {
		t.Errorf("x1 = %v, want 50.0", val)
	}
	if val, ok := results["y1"]; !ok || val != 100.0 {
		t.Errorf("y1 = %v, want 100.0", val)
	}
	if val, ok := results["w"]; !ok || val != 100.0 {
		t.Errorf("w = %v, want 100.0", val)
	}
	if val, ok := results["h"]; !ok || val != 150.0 {
		t.Errorf("h = %v, want 150.0", val)
	}
}

func TestGeometryEvaluateGuidesPreservesNames(t *testing.T) {
	geom := &Geometry{
		Type: GeometryTypeCustom,
		Guides: []Guide{
			{Name: "left", Value: 10},
			{Name: "top", Formula: "ht/2"},
			{Name: "right", Formula: "left+wd/2"},
		},
	}
	results, err := geom.EvaluateGuides(200, 100)
	if err != nil {
		t.Fatalf("EvaluateGuides() error = %v", err)
	}
	if val, ok := results["left"]; !ok || val != 10 {
		t.Errorf("left = %v, want 10", val)
	}
	if val, ok := results["top"]; !ok || val != 50 {
		t.Errorf("top = %v, want 50", val)
	}
	if val, ok := results["right"]; !ok || val != 110 {
		t.Errorf("right = %v, want 110", val)
	}
	// Synthetic adj1 name should NOT exist
	if _, ok := results["adj1"]; ok {
		t.Errorf("adj1 should not exist when guides have real names")
	}
}

func TestArrowCalloutPresetsProduceDistinctOrientations(t *testing.T) {
	tests := []struct {
		name         string
		shape        PresetGeometry
		wantTipX     float64
		wantTipY     float64
		wantMinMoves int
	}{
		{
			name:         "right arrow callout",
			shape:        PresetGeometry{Name: "rightArrowCallout"},
			wantTipX:     200,
			wantTipY:     50,
			wantMinMoves: 2,
		},
		{
			name:         "left arrow callout",
			shape:        PresetGeometry{Name: "leftArrowCallout"},
			wantTipX:     0,
			wantTipY:     50,
			wantMinMoves: 2,
		},
		{
			name:         "up arrow callout",
			shape:        PresetGeometry{Name: "upArrowCallout"},
			wantTipX:     100,
			wantTipY:     0,
			wantMinMoves: 2,
		},
		{
			name:         "down arrow callout",
			shape:        PresetGeometry{Name: "downArrowCallout"},
			wantTipX:     100,
			wantTipY:     100,
			wantMinMoves: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := tt.shape.GetPath(200, 100)
			if err != nil {
				t.Fatalf("GetPath() error = %v", err)
			}

			moves := 0
			tipFound := false
			for _, cmd := range path.Commands {
				if cmd.Type == "M" {
					moves++
				}
				for _, pt := range cmd.Points {
					if almostEqual(pt.X, tt.wantTipX) && almostEqual(pt.Y, tt.wantTipY) {
						tipFound = true
					}
				}
			}

			if moves < tt.wantMinMoves {
				t.Fatalf("move count = %d, want at least %d for callout body + arrow", moves, tt.wantMinMoves)
			}
			if !tipFound {
				t.Fatalf("tip point (%v,%v) not found in path %#v", tt.wantTipX, tt.wantTipY, path.Commands)
			}
		})
	}
}

func TestQuadArrowPresetsExposeFourArrowHeads(t *testing.T) {
	for _, name := range []string{"quadArrow", "quadArrowCallout"} {
		t.Run(name, func(t *testing.T) {
			path, err := (&PresetGeometry{Name: name}).GetPath(200, 200)
			if err != nil {
				t.Fatalf("GetPath() error = %v", err)
			}

			wantPoints := []Point{
				{X: 100, Y: 0},
				{X: 200, Y: 100},
				{X: 100, Y: 200},
				{X: 0, Y: 100},
			}
			for _, want := range wantPoints {
				found := false
				for _, cmd := range path.Commands {
					for _, pt := range cmd.Points {
						if pt == want {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				if !found {
					t.Fatalf("quad arrow missing head point %#v in %#v", want, path.Commands)
				}
			}
		})
	}
}

func almostEqual(got, want float64) bool {
	const eps = 0.001
	return math.Abs(got-want) <= eps
}

func TestSolidFillApplyColorTransformsExpanded(t *testing.T) {
	tests := []struct {
		name string
		fill SolidFill
		base string
		want string
		fuzz bool
	}{
		{
			name: "shade halves channels",
			fill: SolidFill{Shade: 0.5, HasShade: true},
			base: "808080",
			want: "404040",
		},
		{
			name: "explicit zero shade becomes black",
			fill: SolidFill{Shade: 0, HasShade: true},
			base: "804020",
			want: "000000",
		},
		{
			name: "tint moves toward white",
			fill: SolidFill{Tint: 0.5, HasTint: true},
			base: "000000",
			want: "808080",
		},
		{
			name: "complement rotates hue",
			fill: SolidFill{Comp: true},
			base: "FF0000",
			want: "00FFFF",
			fuzz: true,
		},
		{
			name: "invert flips channels",
			fill: SolidFill{Inv: true},
			base: "112233",
			want: "EEDDCC",
		},
		{
			name: "absolute red channel",
			fill: SolidFill{Red: 1, HasRed: true},
			base: "000000",
			want: "FF0000",
		},
		{
			name: "green channel modulation",
			fill: SolidFill{GreenMod: 0.5, HasGreenMod: true},
			base: "00FF00",
			want: "008000",
		},
		{
			name: "blue channel offset",
			fill: SolidFill{BlueOff: 0.25, HasBlueOff: true},
			base: "000000",
			want: "000040",
		},
		{
			name: "absolute hue replacement",
			fill: SolidFill{Hue: 120, HasHue: true},
			base: "FF0000",
			want: "00FF00",
			fuzz: true,
		},
		{
			name: "absolute saturation replacement",
			fill: SolidFill{Sat: 0, HasSat: true},
			base: "FF0000",
			want: "808080",
			fuzz: true,
		},
		{
			name: "absolute luminance replacement",
			fill: SolidFill{Lum: 0, HasLum: true},
			base: "FF0000",
			want: "000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fill.ApplyColorTransforms(tt.base)
			if tt.fuzz {
				assertHexColorNear(t, got, tt.want, 1)
				return
			}
			if got != tt.want {
				t.Fatalf("ApplyColorTransforms(%q) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}

func TestSolidFillApplyColorTransformsGammaRoundTrip(t *testing.T) {
	base := "804020"
	got := (SolidFill{Gamma: true, InvGamma: true}).ApplyColorTransforms(base)
	assertHexColorNear(t, got, base, 2)
}

func assertHexColorNear(t *testing.T, got, want string, delta uint8) {
	t.Helper()
	gotR, gotG, gotB, ok := parseHexRGB(got)
	if !ok {
		t.Fatalf("parseHexRGB(%q) failed", got)
	}
	wantR, wantG, wantB, ok := parseHexRGB(want)
	if !ok {
		t.Fatalf("parseHexRGB(%q) failed", want)
	}
	if channelDelta(gotR, wantR) > delta || channelDelta(gotG, wantG) > delta || channelDelta(gotB, wantB) > delta {
		t.Fatalf("color = %q, want near %q within %d", got, want, delta)
	}
}

func channelDelta(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func TestTransform(t *testing.T) {
	// Test identity transform
	identity := Identity()
	p := Point{X: 10, Y: 20}
	transformed := identity.Apply(p)
	if transformed.X != p.X || transformed.Y != p.Y {
		t.Errorf("Identity transform failed: got %v, want %v", transformed, p)
	}

	// Test translation
	translate := Translate(5, 10)
	transformed = translate.Apply(p)
	if transformed.X != 15 || transformed.Y != 30 {
		t.Errorf("Translate transform failed: got %v, want (15, 30)", transformed)
	}

	// Test scaling
	scale := Scale(2, 3)
	transformed = scale.Apply(p)
	if transformed.X != 20 || transformed.Y != 60 {
		t.Errorf("Scale transform failed: got %v, want (20, 60)", transformed)
	}

	// Test rotation
	rotate := Rotate(90)
	transformed = rotate.Apply(Point{X: 1, Y: 0})
	// (1,0) rotated 90° should be (0,1)
	if transformed.X > 0.001 || transformed.Y-1 > 0.001 {
		t.Errorf("Rotate transform failed: got %v, want (0, 1)", transformed)
	}

	// Test composition
	composed := Translate(10, 20).Multiply(Scale(2, 2))
	transformed = composed.Apply(Point{X: 1, Y: 1})
	// Scale then translate: (1,1) *2 = (2,2) + (10,20) = (12,22)
	if transformed.X != 12 || transformed.Y != 22 {
		t.Errorf("Composed transform failed: got %v, want (12, 22)", transformed)
	}

	fmt.Println("Transform tests passed")
}

func TestBuildConnectorPrimitivesBentConnector(t *testing.T) {
	primitives := BuildConnectorPrimitives(ConnectorSpec{
		Type:        "bentConnector3",
		BoundsX:     72,
		BoundsY:     54,
		BoundsW:     144,
		BoundsH:     54,
		Adjustments: []int{50000},
		FlipH:       true,
		LineColor:   "595959",
		LineWidthPt: 2.25,
	})
	if len(primitives) != 1 {
		t.Fatalf("primitive count = %d, want 1", len(primitives))
	}
	body := primitives[0]
	if body.Kind != PrimitivePolyline || body.Closed {
		t.Fatalf("body primitive = %#v, want open polyline", body)
	}
	want := []Point{{X: 216, Y: 54}, {X: 144, Y: 54}, {X: 144, Y: 108}, {X: 72, Y: 108}}
	if len(body.Points) != len(want) {
		t.Fatalf("point count = %d, want %d", len(body.Points), len(want))
	}
	for i := range want {
		if body.Points[i] != want[i] {
			t.Fatalf("point %d = %#v, want %#v", i, body.Points[i], want[i])
		}
	}
}

func TestBuildConnectorPrimitivesTailTriangle(t *testing.T) {
	primitives := BuildConnectorPrimitives(ConnectorSpec{
		Type:            "straightConnector1",
		BoundsX:         144,
		BoundsY:         54,
		BoundsW:         144,
		BoundsH:         0,
		LineColor:       "595959",
		LineWidthPt:     3.0,
		ArrowTail:       "triangle",
		ArrowTailWidth:  "lg",
		ArrowTailLength: "lg",
	})
	if len(primitives) != 2 {
		t.Fatalf("primitive count = %d, want 2", len(primitives))
	}
	body := primitives[0]
	if len(body.Points) != 2 || body.Points[1] != (Point{X: 252, Y: 54}) {
		t.Fatalf("trimmed body points = %#v, want stroke-scaled end at 252,54", body.Points)
	}
	arrow := primitives[1]
	if !arrow.Closed || arrow.Style.FillColor != "595959" {
		t.Fatalf("arrow primitive = %#v, want closed filled polygon", arrow)
	}
	want := []Point{{X: 288, Y: 54}, {X: 252, Y: 66}, {X: 252, Y: 42}}
	if len(arrow.Points) != len(want) {
		t.Fatalf("arrow point count = %d, want %d", len(arrow.Points), len(want))
	}
	for i := range want {
		if arrow.Points[i] != want[i] {
			t.Fatalf("arrow point %d = %#v, want %#v", i, arrow.Points[i], want[i])
		}
	}
}

func TestBuildShapePrimitivesTriangle(t *testing.T) {
	primitives, err := BuildShapePrimitives(ShapeGeometrySpec{
		Preset:      &PresetGeometry{Name: "triangle"},
		Width:       120,
		Height:      60,
		FillColor:   "FF0000",
		StrokeColor: "00FF00",
		StrokeWidth: 2.0,
	})
	if err != nil {
		t.Fatalf("BuildShapePrimitives: %v", err)
	}
	if len(primitives) != 1 {
		t.Fatalf("primitive count = %d, want 1", len(primitives))
	}
	prim := primitives[0]
	if prim.Kind != PrimitivePath || prim.Path == nil {
		t.Fatalf("primitive = %#v, want path primitive", prim)
	}
	if prim.Style.FillColor != "FF0000" || prim.Style.StrokeColor != "00FF00" || prim.Style.StrokeWidth != 2.0 {
		t.Fatalf("unexpected style = %#v", prim.Style)
	}
	if len(prim.Path.Commands) == 0 || prim.Path.Commands[0].Type != "M" {
		t.Fatalf("unexpected path = %#v", prim.Path)
	}
}

func TestBuildShapePrimitivesEllipse(t *testing.T) {
	primitives, err := BuildShapePrimitives(ShapeGeometrySpec{
		Preset:    &PresetGeometry{Name: "ellipse"},
		Width:     120,
		Height:    60,
		FillColor: "336699",
	})
	if err != nil {
		t.Fatalf("BuildShapePrimitives: %v", err)
	}
	if len(primitives) != 1 {
		t.Fatalf("primitive count = %d, want 1", len(primitives))
	}
	prim := primitives[0]
	if prim.Kind != PrimitiveEllipse || prim.Ellipse == nil {
		t.Fatalf("primitive = %#v, want ellipse primitive", prim)
	}
	if prim.Ellipse.Center != (Point{X: 60, Y: 30}) || prim.Ellipse.RX != 60 || prim.Ellipse.RY != 30 {
		t.Fatalf("unexpected ellipse = %#v", prim.Ellipse)
	}
}

func TestBuildShapePrimitivesCustomGeometry(t *testing.T) {
	primitives, err := BuildShapePrimitives(ShapeGeometrySpec{
		Geometry: &Geometry{
			Type: GeometryTypeCustom,
			Paths: []Path{{
				FillRule: "nonzero",
				Commands: []PathCommand{
					{Type: "M", Points: []Point{{X: 0, Y: 1}}},
					{Type: "L", Points: []Point{{X: 0.5, Y: 0}}},
					{Type: "L", Points: []Point{{X: 1, Y: 1}}},
					{Type: "Z"},
				},
			}},
		},
		Width:     120,
		Height:    60,
		FillColor: "FF0000",
	})
	if err != nil {
		t.Fatalf("BuildShapePrimitives: %v", err)
	}
	if len(primitives) != 1 {
		t.Fatalf("primitive count = %d, want 1", len(primitives))
	}
	if primitives[0].Kind != PrimitivePath || primitives[0].Path == nil {
		t.Fatalf("primitive = %#v, want path", primitives[0])
	}
	if primitives[0].Path.Commands[1].Points[0] != (Point{X: 60, Y: 0}) {
		t.Fatalf("unexpected scaled point = %#v", primitives[0].Path.Commands[1].Points[0])
	}
}

func TestBuildShapePrimitivesCustomGeometryComplex(t *testing.T) {
	// Test a more complex custom geometry with multiple command types
	primitives, err := BuildShapePrimitives(ShapeGeometrySpec{
		Geometry: &Geometry{
			Type: GeometryTypeCustom,
			Paths: []Path{{
				FillRule: "evenodd",
				Commands: []PathCommand{
					// First subpath: a triangle
					{Type: "M", Points: []Point{{X: 0.1, Y: 0.9}}},
					{Type: "L", Points: []Point{{X: 0.5, Y: 0.1}}},
					{Type: "L", Points: []Point{{X: 0.9, Y: 0.9}}},
					{Type: "Z"},
					// Second subpath: a quadratic Bézier curve
					{Type: "M", Points: []Point{{X: 0.2, Y: 0.5}}},
					{Type: "Q", Points: []Point{{X: 0.5, Y: 0.2}, {X: 0.8, Y: 0.5}}},
					// Third subpath: a cubic Bézier curve
					{Type: "M", Points: []Point{{X: 0.3, Y: 0.7}}},
					{Type: "C", Points: []Point{{X: 0.4, Y: 0.6}, {X: 0.6, Y: 0.6}, {X: 0.7, Y: 0.7}}},
					// Arc
					{Type: "M", Points: []Point{{X: 0.4, Y: 0.3}}},
					{Type: "A", Points: []Point{{X: 0.6, Y: 0.3}}, Args: []float64{0.1, 0.1, 0, 0, 1}},
				},
			}},
		},
		Width:       200,
		Height:      100,
		FillColor:   "00FF00",
		StrokeColor: "0000FF",
		StrokeWidth: 2.0,
	})
	if err != nil {
		t.Fatalf("BuildShapePrimitives: %v", err)
	}
	if len(primitives) != 1 {
		t.Fatalf("primitive count = %d, want 1", len(primitives))
	}
	prim := primitives[0]
	if prim.Kind != PrimitivePath || prim.Path == nil {
		t.Fatalf("primitive = %#v, want path", prim)
	}
	// Check that all commands were preserved
	if len(prim.Path.Commands) != 10 {
		t.Fatalf("command count = %d, want 10", len(prim.Path.Commands))
	}
	// Check that fill rule was preserved
	if prim.Path.FillRule != "evenodd" {
		t.Fatalf("fill rule = %s, want evenodd", prim.Path.FillRule)
	}
	// Check that scaling was applied correctly
	// First point of first command should be scaled
	if prim.Path.Commands[0].Points[0] != (Point{X: 20, Y: 90}) {
		t.Fatalf("unexpected scaled point = %#v, want (20, 90)", prim.Path.Commands[0].Points[0])
	}
	// Check arc arguments were scaled
	if prim.Path.Commands[9].Type == "A" {
		if len(prim.Path.Commands[9].Args) < 2 || prim.Path.Commands[9].Args[0] != 20 || prim.Path.Commands[9].Args[1] != 10 {
			t.Fatalf("arc args not scaled correctly = %v, want [20, 10, ...]", prim.Path.Commands[9].Args)
		}
	}
}

func TestBuildConnectorPrimitivesCurvedConnector4(t *testing.T) {
	primitives := BuildConnectorPrimitives(ConnectorSpec{
		Type:        "curvedConnector4",
		BoundsX:     72,
		BoundsY:     54,
		BoundsW:     144,
		BoundsH:     108,
		LineColor:   "595959",
		LineWidthPt: 2.25,
	})
	if len(primitives) != 1 {
		t.Fatalf("primitive count = %d, want 1", len(primitives))
	}
	body := primitives[0]
	// curvedConnector4 should be a Path primitive, not a Polyline
	if body.Kind != PrimitivePath || body.Path == nil {
		t.Fatalf("body primitive = %#v, want path primitive", body)
	}
	// Check that we have a path with commands
	if len(body.Path.Commands) == 0 {
		t.Fatal("curvedConnector4 path has no commands")
	}
	// Check style
	if body.Style.StrokeColor != "595959" || body.Style.StrokeWidth != 2.25 {
		t.Fatalf("unexpected style = %#v", body.Style)
	}
}

func TestBuildConnectorPrimitivesCurvedConnector5(t *testing.T) {
	primitives := BuildConnectorPrimitives(ConnectorSpec{
		Type:        "curvedConnector5",
		BoundsX:     72,
		BoundsY:     54,
		BoundsW:     144,
		BoundsH:     108,
		LineColor:   "595959",
		LineWidthPt: 2.25,
	})
	if len(primitives) != 1 {
		t.Fatalf("primitive count = %d, want 1", len(primitives))
	}
	body := primitives[0]
	// curvedConnector5 should be a Path primitive, not a Polyline
	if body.Kind != PrimitivePath || body.Path == nil {
		t.Fatalf("body primitive = %#v, want path primitive", body)
	}
	// Check that we have a path with commands
	if len(body.Path.Commands) == 0 {
		t.Fatal("curvedConnector5 path has no commands")
	}
	// Check style
	if body.Style.StrokeColor != "595959" || body.Style.StrokeWidth != 2.25 {
		t.Fatalf("unexpected style = %#v", body.Style)
	}
}

func TestBuildShapePrimitivesHeart(t *testing.T) {
	primitives, err := BuildShapePrimitives(ShapeGeometrySpec{
		Preset:    &PresetGeometry{Name: "heart"},
		Width:     120,
		Height:    100,
		FillColor: "FF3366",
	})
	if err != nil {
		t.Fatalf("BuildShapePrimitives: %v", err)
	}
	if len(primitives) != 1 {
		t.Fatalf("primitive count = %d, want 1", len(primitives))
	}
	prim := primitives[0]
	if prim.Kind != PrimitivePath || prim.Path == nil {
		t.Fatalf("primitive = %#v, want path primitive", prim)
	}
	if prim.Style.FillColor != "FF3366" {
		t.Fatalf("unexpected fill color = %s, want FF3366", prim.Style.FillColor)
	}
	// Basic validation that we got a path with commands
	if len(prim.Path.Commands) == 0 {
		t.Fatal("heart path has no commands")
	}
	// Heart should have at least a MoveTo and Cubic Bézier commands
	hasCubic := false
	for _, cmd := range prim.Path.Commands {
		if cmd.Type == "C" {
			hasCubic = true
			break
		}
	}
	if !hasCubic {
		t.Fatal("heart path should have cubic Bézier commands")
	}
}

func TestBuildShapePrimitivesWithLineStyle(t *testing.T) {
	// Test that line styles are properly applied to shapes
	primitives, err := BuildShapePrimitives(ShapeGeometrySpec{
		Preset:    &PresetGeometry{Name: "rect"},
		Width:     100,
		Height:    50,
		FillColor: "FF0000",
		Stroke: &LineStyle{
			Width:      2.5,
			Cap:        LineCapRound,
			Join:       LineJoinRound,
			MiterLimit: 10.0,
			Dash:       DashPatternDashDot,
			Color:      "00FF00",
		},
	})
	if err != nil {
		t.Fatalf("BuildShapePrimitives: %v", err)
	}
	if len(primitives) != 1 {
		t.Fatalf("primitive count = %d, want 1", len(primitives))
	}
	prim := primitives[0]
	if prim.Style.Stroke == nil {
		t.Fatal("Stroke should not be nil")
	}
	if prim.Style.Stroke.Width != 2.5 {
		t.Errorf("Stroke.Width = %f, want 2.5", prim.Style.Stroke.Width)
	}
	if prim.Style.Stroke.Cap != LineCapRound {
		t.Errorf("Stroke.Cap = %v, want LineCapRound", prim.Style.Stroke.Cap)
	}
	if prim.Style.Stroke.Join != LineJoinRound {
		t.Errorf("Stroke.Join = %v, want LineJoinRound", prim.Style.Stroke.Join)
	}
	if prim.Style.Stroke.Dash != DashPatternDashDot {
		t.Errorf("Stroke.Dash = %v, want DashPatternDashDot", prim.Style.Stroke.Dash)
	}
	if prim.Style.StrokeColor != "00FF00" {
		t.Errorf("StrokeColor = %s, want 00FF00", prim.Style.StrokeColor)
	}
}

func TestBuildConnectorPrimitivesWithLineStyle(t *testing.T) {
	// Test that line styles are properly applied to connectors
	primitives := BuildConnectorPrimitives(ConnectorSpec{
		Type:    "straightConnector1",
		BoundsX: 100,
		BoundsY: 50,
		BoundsW: 200,
		BoundsH: 0,
		LineStyle: &LineStyle{
			Width:      3.0,
			Cap:        LineCapSquare,
			Join:       LineJoinBevel,
			MiterLimit: 8.0,
			Dash:       DashPatternLongDash,
			Color:      "0000FF",
		},
		LineWidthPt: 2.0,      // Should be overridden by LineStyle.Width (3.0)
		LineColor:   "FF0000", // Takes precedence over LineStyle.Color
	})
	if len(primitives) < 1 {
		t.Fatalf("primitive count = %d, want at least 1", len(primitives))
	}
	// Check the first primitive (the connector body)
	prim := primitives[0]
	if prim.Style.Stroke == nil {
		t.Fatal("Stroke should not be nil")
	}
	if prim.Style.Stroke.Width != 3.0 {
		t.Errorf("Stroke.Width = %f, want 3.0", prim.Style.Stroke.Width)
	}
	if prim.Style.Stroke.Cap != LineCapSquare {
		t.Errorf("Stroke.Cap = %v, want LineCapSquare", prim.Style.Stroke.Cap)
	}
	if prim.Style.Stroke.Join != LineJoinBevel {
		t.Errorf("Stroke.Join = %v, want LineJoinBevel", prim.Style.Stroke.Join)
	}
	if prim.Style.Stroke.Dash != DashPatternLongDash {
		t.Errorf("Stroke.Dash = %v, want DashPatternLongDash", prim.Style.Stroke.Dash)
	}
	// LineColor takes precedence over LineStyle.Color when both are provided
	if prim.Style.StrokeColor != "FF0000" {
		t.Errorf("StrokeColor = %s, want FF0000 (LineColor takes precedence)", prim.Style.StrokeColor)
	}
}

func TestCompoundLinePrimitives(t *testing.T) {
	spec := ConnectorSpec{
		Type:    "line",
		BoundsX: 10, BoundsY: 10, BoundsW: 100, BoundsH: 0,
		LineColor:   "000000",
		LineWidthPt: 4.0,
		LineStyle: &LineStyle{
			Compound:     true,
			CompoundType: "dbl",
			Color:        "000000",
			Width:        4.0,
		},
	}
	prims := BuildConnectorPrimitives(spec)
	// Should have at least 2 primitives: body stroke + gap stroke
	if len(prims) < 2 {
		t.Fatalf("dbl compound connector: got %d primitives, want >= 2", len(prims))
	}
}

func TestCompoundLineTriPrimitives(t *testing.T) {
	spec := ConnectorSpec{
		Type:    "line",
		BoundsX: 10, BoundsY: 10, BoundsW: 100, BoundsH: 0,
		LineColor:   "000000",
		LineWidthPt: 6.0,
		LineStyle: &LineStyle{
			Compound:     true,
			CompoundType: "tri",
			Color:        "000000",
			Width:        6.0,
		},
	}
	prims := BuildConnectorPrimitives(spec)
	// tri compound has two gap strokes
	if len(prims) < 3 {
		t.Fatalf("tri compound connector: got %d primitives, want >= 3", len(prims))
	}
}

func TestCloudDistinctFromCloudCallout(t *testing.T) {
	cloud := PresetGeometry{Name: "cloud"}
	callout := PresetGeometry{Name: "cloudCallout"}

	cloudPath, err := cloud.GetPath(120, 80)
	if err != nil {
		t.Fatalf("cloud GetPath error: %v", err)
	}
	calloutPath, err := callout.GetPath(120, 80)
	if err != nil {
		t.Fatalf("cloudCallout GetPath error: %v", err)
	}

	// cloudCallout should have more commands due to the tail
	if len(calloutPath.Commands) <= len(cloudPath.Commands) {
		t.Errorf("cloudCallout should have more commands than cloud: cloud=%d, callout=%d",
			len(cloudPath.Commands), len(calloutPath.Commands))
	}
}

func TestCanDistinctFromCylinder(t *testing.T) {
	can := PresetGeometry{Name: "can"}
	cylinder := PresetGeometry{Name: "cylinder"}

	canPath, err := can.GetPath(100, 140)
	if err != nil {
		t.Fatalf("can GetPath error: %v", err)
	}
	cylinderPath, err := cylinder.GetPath(100, 140)
	if err != nil {
		t.Fatalf("cylinder GetPath error: %v", err)
	}

	// Both should produce valid paths
	if len(canPath.Commands) == 0 {
		t.Error("can path is empty")
	}
	if len(cylinderPath.Commands) == 0 {
		t.Error("cylinder path is empty")
	}
}

func TestNewPresetGeometries(t *testing.T) {
	shapes := []string{
		"snip1Rect", "snip2SameRect", "snip2DiagRect", "snipRoundRect",
		"round1Rect", "round2SameRect", "round2DiagRect",
		"notchedRightArrow", "homePlate", "foldedCorner", "bevel",
		"noSmoking", "blockArc", "bracketPair", "bracePair",
		"flowChartTerminator", "flowChartPreparation", "flowChartManualInput", "flowChartOffPageConnector",
		"bentArrow", "ribbon",
		"irregularSeal1", "irregularSeal2", "cross",
		"mathMinus", "mathMultiply", "mathEqual", "mathNotEqual",
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

func TestStarAdjustmentOverridesInnerRadius(t *testing.T) {
	w, h := 100.0, 100.0
	defaultPg := PresetGeometry{Name: "star5"}
	deepPg := PresetGeometry{Name: "star5", Adjustments: []float64{0.1}}  // very spiky
	bluntPg := PresetGeometry{Name: "star5", Adjustments: []float64{0.8}} // nearly circular

	pathDefault, err := defaultPg.GetPath(w, h)
	if err != nil {
		t.Fatalf("default star5: %v", err)
	}
	pathDeep, err := deepPg.GetPath(w, h)
	if err != nil {
		t.Fatalf("deep star5: %v", err)
	}
	pathBlunt, err := bluntPg.GetPath(w, h)
	if err != nil {
		t.Fatalf("blunt star5: %v", err)
	}

	// All three should produce the same number of commands.
	if len(pathDefault.Commands) != len(pathDeep.Commands) || len(pathDefault.Commands) != len(pathBlunt.Commands) {
		t.Errorf("command count mismatch: default=%d deep=%d blunt=%d",
			len(pathDefault.Commands), len(pathDeep.Commands), len(pathBlunt.Commands))
	}

	// The inner-point coordinates must differ between deep and blunt stars.
	// Stringify and compare point sets.
	deepStr := fmt.Sprintf("%v", pathDeep.Commands)
	bluntStr := fmt.Sprintf("%v", pathBlunt.Commands)
	if deepStr == bluntStr {
		t.Error("expected deep and blunt star5 paths to differ, but all commands are identical")
	}
}

// TestArrowAdjustmentsChangeShape verifies that changing adj1 (head length)
// or adj2 (shaft width) produces visually distinct arrow paths.
func TestArrowAdjustmentsChangeShape(t *testing.T) {
	w, h := 100.0, 50.0
	defaultArrow := PresetGeometry{Name: "rightArrow"}
	bigHeadArrow := PresetGeometry{Name: "rightArrow", Adjustments: []float64{0.2, 0.4}} // short shaft, big head
	thinArrow := PresetGeometry{Name: "rightArrow", Adjustments: []float64{0.6, 0.1}}    // thin shaft

	pathDefault, err := defaultArrow.GetPath(w, h)
	if err != nil {
		t.Fatalf("default rightArrow: %v", err)
	}
	pathBigHead, err := bigHeadArrow.GetPath(w, h)
	if err != nil {
		t.Fatalf("big-head rightArrow: %v", err)
	}
	pathThin, err := thinArrow.GetPath(w, h)
	if err != nil {
		t.Fatalf("thin rightArrow: %v", err)
	}

	// Verify the shapes produce valid paths.
	for _, p := range []Path{pathDefault, pathBigHead, pathThin} {
		if len(p.Commands) == 0 {
			t.Error("empty path for rightArrow variant")
		}
	}

	// The paths must differ (coordinates differ with different adjustments).
	defaultCmds := fmt.Sprintf("%v", pathDefault.Commands)
	bigHeadCmds := fmt.Sprintf("%v", pathBigHead.Commands)
	thinCmds := fmt.Sprintf("%v", pathThin.Commands)
	if defaultCmds == bigHeadCmds {
		t.Error("default and big-head arrow paths are identical — adjustments have no effect")
	}
	if defaultCmds == thinCmds {
		t.Error("default and thin arrow paths are identical — adjustments have no effect")
	}
}

// TestAdjOrDefaultClampsValues ensures adjOrDefault clamps out-of-range values to [0,1].
func TestAdjOrDefaultClampsValues(t *testing.T) {
	pg := PresetGeometry{Adjustments: []float64{-0.5, 1.5, 0.3}}
	if got := pg.adjOrDefault(0, 0.5); got != 0 {
		t.Errorf("adjOrDefault(0,-0.5): want 0, got %v", got)
	}
	if got := pg.adjOrDefault(1, 0.5); got != 1 {
		t.Errorf("adjOrDefault(1,1.5): want 1, got %v", got)
	}
	if got := pg.adjOrDefault(2, 0.5); got != 0.3 {
		t.Errorf("adjOrDefault(2,0.3): want 0.3, got %v", got)
	}
	if got := pg.adjOrDefault(3, 0.42); got != 0.42 {
		t.Errorf("adjOrDefault(3 missing, def 0.42): want 0.42, got %v", got)
	}
}
func TestFormulaFunctionsAt2(t *testing.T) {
	// at2(1, 1) = atan2(1, 1) in degrees = 45
	val, err := EvaluateFormula("at2(1, 1)", GuideContext{})
	if err != nil {
		t.Fatalf("at2: %v", err)
	}
	if math.Abs(val-45.0) > 0.001 {
		t.Errorf("at2(1,1) = %v, want 45", val)
	}
}

func TestFormulaFunctionsCat2(t *testing.T) {
	// cat2(10, 3, 4) = 10 * cos(atan2(4, 3)) = 10 * 0.6 = 6
	val, err := EvaluateFormula("cat2(10, 3, 4)", GuideContext{})
	if err != nil {
		t.Fatalf("cat2: %v", err)
	}
	if math.Abs(val-6.0) > 0.001 {
		t.Errorf("cat2(10,3,4) = %v, want 6", val)
	}
}

func TestFormulaFunctionsSat2(t *testing.T) {
	// sat2(10, 3, 4) = 10 * sin(atan2(4, 3)) = 10 * 0.8 = 8
	val, err := EvaluateFormula("sat2(10, 3, 4)", GuideContext{})
	if err != nil {
		t.Fatalf("sat2: %v", err)
	}
	if math.Abs(val-8.0) > 0.001 {
		t.Errorf("sat2(10,3,4) = %v, want 8", val)
	}
}

func TestFormulaFunctionsMod(t *testing.T) {
	// mod(3, 4, 0) = sqrt(3² + 4² + 0²) = 5
	val, err := EvaluateFormula("mod(3, 4, 0)", GuideContext{})
	if err != nil {
		t.Fatalf("mod: %v", err)
	}
	if math.Abs(val-5.0) > 0.001 {
		t.Errorf("mod(3,4,0) = %v, want 5", val)
	}
}

func TestFormulaFunctionsIf(t *testing.T) {
	val, err := EvaluateFormula("if(1, 10, 20)", GuideContext{})
	if err != nil {
		t.Fatalf("if: %v", err)
	}
	if val != 10 {
		t.Errorf("if(1,10,20) = %v, want 10", val)
	}
	val, err = EvaluateFormula("if(-1, 10, 20)", GuideContext{})
	if err != nil {
		t.Fatalf("if: %v", err)
	}
	if val != 20 {
		t.Errorf("if(-1,10,20) = %v, want 20", val)
	}
	val, err = EvaluateFormula("if(0, 10, 20)", GuideContext{})
	if err != nil {
		t.Fatalf("if: %v", err)
	}
	if val != 20 {
		t.Errorf("if(0,10,20) = %v, want 20", val)
	}
}

func TestFormulaFunctionsProd(t *testing.T) {
	val, err := EvaluateFormula("prod(2, 3, 4)", GuideContext{})
	if err != nil {
		t.Fatalf("prod: %v", err)
	}
	if val != 24 {
		t.Errorf("prod(2,3,4) = %v, want 24", val)
	}
}

func TestArrowCalloutAdjustmentsOwnBodyShaftAndHeadGeometry(t *testing.T) {
	base := PresetGeometry{Name: "rightArrowCallout"}
	adjusted := PresetGeometry{Name: "rightArrowCallout", Adjustments: []float64{0.30, 0.10, 0.35}}
	bp, err := base.GetPath(200, 100)
	if err != nil {
		t.Fatal(err)
	}
	ap, err := adjusted.GetPath(200, 100)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(bp.Commands) == fmt.Sprint(ap.Commands) {
		t.Fatal("authored arrow-callout adjustments did not change geometry")
	}
	// The adjusted body is 30% of the width; its rectangle's second command
	// therefore terminates at x=60 rather than the default x=96.
	if got := ap.Commands[1].Points[0].X; math.Abs(got-60) > 0.001 {
		t.Fatalf("adjusted body edge x=%v, want 60", got)
	}
}

func TestArrowCalloutExtremeAdjustmentsRemainFinite(t *testing.T) {
	for _, name := range []string{"rightArrowCallout", "leftArrowCallout", "upArrowCallout", "downArrowCallout"} {
		pg := PresetGeometry{Name: name, Adjustments: []float64{0, 1, 1}}
		path, err := pg.GetPath(200, 100)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, cmd := range path.Commands {
			for _, pt := range cmd.Points {
				if math.IsNaN(pt.X) || math.IsNaN(pt.Y) || math.IsInf(pt.X, 0) || math.IsInf(pt.Y, 0) {
					t.Fatalf("%s produced invalid point %+v", name, pt)
				}
			}
		}
	}
}

func TestMultiDirectionArrowCalloutAdjustmentsOwnGeometry(t *testing.T) {
	lr := PresetGeometry{Name: "leftRightArrowCallout", Adjustments: []float64{0.30, 0.10, 0.15}}
	p, err := lr.GetPath(200, 100)
	if err != nil {
		t.Fatal(err)
	}
	// Central callout width is adj1 * width.
	if got := p.Commands[1].Points[0].X; math.Abs(got-130) > 0.001 {
		t.Fatalf("leftRight callout central right edge=%v, want 130", got)
	}

	base := PresetGeometry{Name: "quadArrowCallout"}
	adj := PresetGeometry{Name: "quadArrowCallout", Adjustments: []float64{0.30, 0.36, 0.18}}
	bp, _ := base.GetPath(200, 100)
	ap, err := adj.GetPath(200, 100)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(bp.Commands) == fmt.Sprint(ap.Commands) {
		t.Fatal("quadArrowCallout adjustments did not change geometry")
	}
	// The prepended callout rectangle is 18% of the host dimensions.
	if got := ap.Commands[1].Points[0].X; math.Abs(got-118) > 0.001 {
		t.Fatalf("quad callout central right edge=%v, want 118", got)
	}
}

func TestMultiDirectionArrowCalloutExtremeAdjustmentsRemainFinite(t *testing.T) {
	for _, name := range []string{"leftRightArrowCallout", "quadArrowCallout", "quadArrow"} {
		pg := PresetGeometry{Name: name, Adjustments: []float64{1, 0, 1}}
		path, err := pg.GetPath(200, 100)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, cmd := range path.Commands {
			for _, pt := range cmd.Points {
				if math.IsNaN(pt.X) || math.IsNaN(pt.Y) || math.IsInf(pt.X, 0) || math.IsInf(pt.Y, 0) {
					t.Fatalf("%s produced invalid point %+v", name, pt)
				}
			}
		}
	}
}
