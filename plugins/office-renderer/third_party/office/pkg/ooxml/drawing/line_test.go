package drawing

import (
	"strings"
	"testing"
)

func TestLineStyleDashArray(t *testing.T) {
	tests := []struct {
		pattern DashPattern
		want    string
	}{
		{DashPatternSolid, "none"},
		{DashPatternDash, "3, 3"},
		{DashPatternDot, "1, 3"},
		{DashPatternDashDot, "3, 3, 1, 3"},
		{DashPatternLongDash, "5, 3"},
		{DashPatternLongDashDot, "5, 3, 1, 3"},
		{DashPatternLongDashDotDot, "5, 3, 1, 3, 1, 3"},
		{DashPatternSystemDash, "3, 3"},
		{DashPatternSystemDot, "1, 3"},
		{DashPatternSystemDashDot, "3, 3, 1, 3"},
		{DashPatternSystemDashDotDot, "3, 3, 1, 3, 1, 3"},
		{"unknown", "none"},
	}

	for _, tt := range tests {
		ls := LineStyle{Dash: tt.pattern}
		got := ls.DashArray()
		if got != tt.want {
			t.Errorf("DashArray() for %q = %q, want %q", tt.pattern, got, tt.want)
		}
	}
}

func TestArrowheadWidth(t *testing.T) {
	tests := []struct {
		size ArrowheadSize
		want float64
	}{
		{ArrowheadSizeSmall, 4.0},
		{ArrowheadSizeMedium, 6.0},
		{ArrowheadSizeLarge, 8.0},
		{"", 6.0},
	}
	for _, tt := range tests {
		a := Arrowhead{Size: tt.size}
		if got := a.Width(); got != tt.want {
			t.Errorf("Width() for %q = %v, want %v", tt.size, got, tt.want)
		}
	}
}

func TestArrowheadLengthPoints(t *testing.T) {
	tests := []struct {
		length ArrowheadSize
		want   float64
	}{
		{ArrowheadSizeSmall, 6.0},
		{ArrowheadSizeMedium, 9.0},
		{ArrowheadSizeLarge, 12.0},
		{"", 9.0},
	}
	for _, tt := range tests {
		a := Arrowhead{Length: tt.length}
		if got := a.LengthPoints(); got != tt.want {
			t.Errorf("LengthPoints() for %q = %v, want %v", tt.length, got, tt.want)
		}
	}
}

func TestPathForArrowhead(t *testing.T) {
	tests := []struct {
		typ        ArrowheadType
		wantPrefix string
	}{
		{ArrowheadNone, ""},
		{ArrowheadTriangle, "M 0,0 L"},
		{ArrowheadArrow, "M 0,0 L"},
		{ArrowheadStealth, "M 0,0 L"},
		{ArrowheadDiamond, "M 0,0 L"},
		{ArrowheadOval, "M 0,0 A"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		a := Arrowhead{Type: tt.typ, Size: ArrowheadSizeMedium, Length: ArrowheadSizeMedium}
		got := a.PathForArrowhead(1.0)
		if !strings.HasPrefix(got, tt.wantPrefix) {
			t.Errorf("PathForArrowhead() for %q = %q, want prefix %q", tt.typ, got, tt.wantPrefix)
		}
	}
}

func TestCalculateConnectionSites(t *testing.T) {
	sites := CalculateConnectionSites(100, 50)
	if len(sites) != 8 {
		t.Fatalf("sites = %d, want 8", len(sites))
	}
	for i, site := range sites {
		if site.Position < 0 || site.Position > 1 {
			t.Errorf("site %d position = %v, want [0,1]", i, site.Position)
		}
	}
}

func TestPointForConnectionSite(t *testing.T) {
	rect := Rect{X: 10, Y: 20, Width: 100, Height: 50}

	// PointForConnectionSite traverses the perimeter starting at top-left,
	// going top edge → right edge → bottom edge → left edge.

	// Position 0: top-left corner
	p := PointForConnectionSite(ConnectionSite{Position: 0, Angle: 0}, rect)
	if p.X != 10 || p.Y != 20 {
		t.Errorf("position 0 = (%v,%v), want (10,20)", p.X, p.Y)
	}

	// Position 0.25: 75 units along top edge
	p = PointForConnectionSite(ConnectionSite{Position: 0.25, Angle: 90}, rect)
	if p.X != 85 || p.Y != 20 {
		t.Errorf("position 0.25 = (%v,%v), want (85,20)", p.X, p.Y)
	}

	// Position 0.5: end of right edge (bottom-right corner)
	p = PointForConnectionSite(ConnectionSite{Position: 0.5, Angle: 180}, rect)
	if p.X != 110 || p.Y != 70 {
		t.Errorf("position 0.5 = (%v,%v), want (110,70)", p.X, p.Y)
	}

	// Position 0.75: 225 units = top(100)+right(50)+bottom(75) → on bottom edge
	p = PointForConnectionSite(ConnectionSite{Position: 0.75, Angle: 270}, rect)
	if p.X != 35 || p.Y != 70 {
		t.Errorf("position 0.75 = (%v,%v), want (35,70)", p.X, p.Y)
	}

	// Verify position 1.0 returns to top-left
	p = PointForConnectionSite(ConnectionSite{Position: 1.0, Angle: 0}, rect)
	if p.X != 10 || p.Y != 20 {
		t.Errorf("position 1.0 = (%v,%v), want (10,20)", p.X, p.Y)
	}
}

func TestCalculateElbowPath(t *testing.T) {
	start := Point{X: 0, Y: 0}
	end := Point{X: 100, Y: 100}
	path := CalculateElbowPath(start, end, 0, 0)
	if len(path.Commands) != 4 {
		t.Fatalf("commands = %d, want 4", len(path.Commands))
	}
	if path.Commands[0].Type != "M" {
		t.Errorf("first command = %q, want M", path.Commands[0].Type)
	}
	if path.Commands[3].Type != "L" {
		t.Errorf("last command = %q, want L", path.Commands[3].Type)
	}
}

func TestCalculateCurvedPath(t *testing.T) {
	start := Point{X: 0, Y: 0}
	end := Point{X: 100, Y: 100}
	path := CalculateCurvedPath(start, end, 0, 180)
	if len(path.Commands) != 2 {
		t.Fatalf("commands = %d, want 2", len(path.Commands))
	}
	if path.Commands[0].Type != "M" {
		t.Errorf("first command = %q, want M", path.Commands[0].Type)
	}
	if path.Commands[1].Type != "Q" {
		t.Errorf("second command = %q, want Q", path.Commands[1].Type)
	}
	if len(path.Commands[1].Points) != 2 {
		t.Errorf("quadratic points = %d, want 2", len(path.Commands[1].Points))
	}
}

func TestCalculateCurvedPathSmallAngleDiff(t *testing.T) {
	// angleDiff <= 180 hits the else branch
	start := Point{X: 0, Y: 0}
	end := Point{X: 100, Y: 100}
	path := CalculateCurvedPath(start, end, 0, 90)
	if len(path.Commands) != 2 {
		t.Fatalf("commands = %d, want 2", len(path.Commands))
	}
	ctrl := path.Commands[1].Points[0]
	// Control point should be different from the large-angle-diff case
	if ctrl.X == 0 && ctrl.Y == 0 {
		t.Errorf("control point = (%v,%v), want non-zero", ctrl.X, ctrl.Y)
	}
}

func TestPointForConnectionSiteAllEdges(t *testing.T) {
	rect := Rect{X: 10, Y: 20, Width: 100, Height: 50}
	perimeter := 2 * (rect.Width + rect.Height)

	// Right edge
	p1 := PointForConnectionSite(ConnectionSite{Position: 0.1, Angle: 0}, rect)
	if p1.Y != rect.Y {
		t.Errorf("right edge Y = %v, want %v", p1.Y, rect.Y)
	}

	// Bottom edge: pos between width and width+height
	p2 := PointForConnectionSite(ConnectionSite{Position: float64(rect.Width+10) / perimeter, Angle: 0}, rect)
	if p2.X != rect.X+rect.Width {
		t.Errorf("bottom edge X = %v, want %v", p2.X, rect.X+rect.Width)
	}

	// Left edge
	p3 := PointForConnectionSite(ConnectionSite{Position: float64(rect.Width+rect.Height+10) / perimeter, Angle: 0}, rect)
	if p3.Y != rect.Y+rect.Height {
		t.Errorf("left edge Y = %v, want %v", p3.Y, rect.Y+rect.Height)
	}

	// Top edge
	p4 := PointForConnectionSite(ConnectionSite{Position: 0.95, Angle: 0}, rect)
	if p4.X != rect.X {
		t.Errorf("top edge X = %v, want %v", p4.X, rect.X)
	}
}

func TestCalculateCurvedPathLargeAngleDiff(t *testing.T) {
	// angleDiff > 180 hits the if branch
	start := Point{X: 0, Y: 0}
	end := Point{X: 100, Y: 100}
	path := CalculateCurvedPath(start, end, 0, 270)
	if len(path.Commands) != 2 {
		t.Fatalf("commands = %d, want 2", len(path.Commands))
	}
	ctrl := path.Commands[1].Points[0]
	// Control point should differ from small-angle-diff case
	if ctrl.X == 50 || ctrl.Y == 50 {
		t.Errorf("control point = (%v,%v), want different from midpoint", ctrl.X, ctrl.Y)
	}
}
