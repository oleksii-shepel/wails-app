package chart

import (
	"math"
	"testing"
)

func TestView3DDefaults(t *testing.T) {
	// nil input → defaults
	d := view3DDefaults(nil)
	if d.RotX != 15 {
		t.Errorf("RotX default = %d, want 15", d.RotX)
	}
	if d.RotY != 20 {
		t.Errorf("RotY default = %d, want 20", d.RotY)
	}
	if d.Perspective != 30 {
		t.Errorf("Perspective default = %d, want 30", d.Perspective)
	}
	if d.DepthPercent != 100 {
		t.Errorf("DepthPercent default = %d, want 100", d.DepthPercent)
	}
	if d.RightAngleAxes {
		t.Error("RightAngleAxes default should be false")
	}

	// non-nil input passes through unchanged
	v := &View3D{RotX: 45, RotY: 60, Perspective: 100, DepthPercent: 50, RightAngleAxes: true}
	out := view3DDefaults(v)
	if out != v {
		t.Error("expected same pointer for non-nil input")
	}
}

func TestProject3DIdentity(t *testing.T) {
	// No rotation, no perspective: the center of the unit cube should map
	// to the center of the plot rectangle.
	view := &View3D{RotX: 0, RotY: 0, Perspective: 0, RightAngleAxes: true}
	plot := rect3D{x: 0, y: 0, w: 200, h: 100}
	p := newProjector(view, plot)

	sx, sy := p.project(0.5, 0.5, 0.5)
	wantX := float64(plot.x + plot.w/2)
	wantY := float64(plot.y + plot.h/2)
	if math.Abs(sx-wantX) > 1 {
		t.Errorf("center sx = %v, want %v", sx, wantX)
	}
	if math.Abs(sy-wantY) > 1 {
		t.Errorf("center sy = %v, want %v", sy, wantY)
	}
}

func TestProject3DRotY90(t *testing.T) {
	// 90° Y rotation swaps x and z (approximately).
	view := &View3D{RotX: 0, RotY: 90, Perspective: 0, RightAngleAxes: true}
	plot := rect3D{x: 0, y: 0, w: 200, h: 200}
	p := newProjector(view, plot)

	// After 90° Ry, the new x coordinate depends on the original z.
	// front-left (x=0, z=1) and front-right (x=1, z=1) both map to positive x.
	// back-left (x=0, z=0) and back-right (x=1, z=0) both map to negative x.
	flx, _ := p.project(0, 0.5, 1) // front-left
	blx, _ := p.project(0, 0.5, 0) // back-left
	if flx <= blx {
		t.Errorf("expected front-left x > back-left x after 90° Y rotation, got front=%v back=%v", flx, blx)
	}
}

func TestProject3DRightAngleAxes(t *testing.T) {
	// With RightAngleAxes=true and Perspective=0, a rotated cube should still
	// have parallel edges (no perspective convergence).
	view := &View3D{RotX: 15, RotY: 20, Perspective: 100, RightAngleAxes: true}
	plot := rect3D{x: 0, y: 0, w: 200, h: 200}
	p := newProjector(view, plot)

	// Project the four vertical edges of the cube.
	edges := [4][2]float64{
		{0, 0}, {1, 0}, {1, 1}, {0, 1},
	}
	var dx, dy float64
	for i, e := range edges {
		x1, y1 := p.project(e[0], 0, e[1])
		x2, y2 := p.project(e[0], 1, e[1])
		if i == 0 {
			dx = x2 - x1
			dy = y2 - y1
		} else {
			if math.Abs((x2-x1)-dx) > 0.01 || math.Abs((y2-y1)-dy) > 0.01 {
				t.Error("vertical edges are not parallel with RightAngleAxes=true")
			}
		}
	}
}

func TestProject3DPerspectiveDistortion(t *testing.T) {
	// With perspective enabled, points closer to camera should appear larger.
	view := &View3D{RotX: 0, RotY: 0, Perspective: 120, RightAngleAxes: false}
	plot := rect3D{x: 0, y: 0, w: 200, h: 200}
	p := newProjector(view, plot)

	// Width of front face (z=1) vs back face (z=0).
	xFront1, _ := p.project(0, 0.5, 1)
	xFront2, _ := p.project(1, 0.5, 1)
	frontW := xFront2 - xFront1

	xBack1, _ := p.project(0, 0.5, 0)
	xBack2, _ := p.project(1, 0.5, 0)
	backW := xBack2 - xBack1

	// Front face (z=1) is closer to camera than back face (z=0).
	if frontW <= backW {
		t.Errorf("expected front face width > back face width, got front=%v back=%v", frontW, backW)
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		input string
		wantR int
		wantG int
		wantB int
	}{
		{"FF0000", 255, 0, 0},
		{"#00FF00", 0, 255, 0},
		{"#00F", 0, 0, 255},
		{"ABC", 170, 187, 204},
		{"zzzzzz", 128, 128, 128},
	}
	for _, tt := range tests {
		r, g, b := parseHexColor(tt.input)
		if r != tt.wantR || g != tt.wantG || b != tt.wantB {
			t.Errorf("parseHexColor(%q) = (%d,%d,%d), want (%d,%d,%d)",
				tt.input, r, g, b, tt.wantR, tt.wantG, tt.wantB)
		}
	}
}

func TestLightenColor(t *testing.T) {
	if got := lightenColor("000000", 100); got != "FFFFFF" {
		t.Errorf("lighten black 100%% = %s, want FFFFFF", got)
	}
	if got := lightenColor("808080", 0); got != "808080" {
		t.Errorf("lighten 50%% 0%% = %s, want 808080", got)
	}
	got := lightenColor("4466AA", 50)
	if got == "4466AA" {
		t.Error("lighten should change the color")
	}
}

func TestDarkenColor(t *testing.T) {
	if got := darkenColor("FFFFFF", 100); got != "000000" {
		t.Errorf("darken white 100%% = %s, want 000000", got)
	}
	if got := darkenColor("808080", 0); got != "808080" {
		t.Errorf("darken 50%% 0%% = %s, want 808080", got)
	}
	got := darkenColor("4466AA", 50)
	if got == "4466AA" {
		t.Error("darken should change the color")
	}
}

func TestNewBoxFacesCulling(t *testing.T) {
	// A box viewed from the front-right-top should show front, right, and top faces.
	view := &View3D{RotX: 15, RotY: 20, Perspective: 30, RightAngleAxes: false}
	plot := rect3D{x: 0, y: 0, w: 200, h: 200}
	p := newProjector(view, plot)

	faces := newBoxFaces(0.3, 0.3, 0.3, 0.4, 0.4, 0.4, "4472C4", p)
	if len(faces) == 0 {
		t.Fatal("expected some visible faces, got none")
	}
	if len(faces) > 3 {
		t.Errorf("expected at most 3 visible faces from a corner view, got %d", len(faces))
	}

	// Ensure at least one face uses a shaded variant.
	var hasLight, hasDark bool
	for _, f := range faces {
		if f.color != "4472C4" {
			if f.color == lightenColor("4472C4", 15) {
				hasLight = true
			}
			if f.color == darkenColor("4472C4", 15) {
				hasDark = true
			}
		}
	}
	if !hasLight && !hasDark {
		t.Error("expected at least one shaded face (top or side)")
	}
}

func TestSortFacesByDepth(t *testing.T) {
	faces := []face3D{
		{depth: 5},
		{depth: 1},
		{depth: 3},
	}
	sortFacesByDepth(faces)
	if faces[0].depth != 1 || faces[1].depth != 3 || faces[2].depth != 5 {
		t.Errorf("sortFacesByDepth order = %v, want [1,3,5]", []float64{faces[0].depth, faces[1].depth, faces[2].depth})
	}
}
