package drawing

import (
	"math"
	"testing"
)

func TestCurvedArrowUsesAuthoredHeadLength(t *testing.T) {
	short, err := (&PresetGeometry{Name: "curvedRightArrow", Adjustments: []float64{0.25, 0.60, 0.15}}).GetPath(200, 100)
	if err != nil {
		t.Fatal(err)
	}
	long, err := (&PresetGeometry{Name: "curvedRightArrow", Adjustments: []float64{0.25, 0.60, 0.40}}).GetPath(200, 100)
	if err != nil {
		t.Fatal(err)
	}
	// The first line command is the arrow head's neck; a longer head starts
	// farther left.
	sx := firstLineX(t, short)
	lx := firstLineX(t, long)
	if !(lx < sx) {
		t.Fatalf("longer head should move neck left: short=%g long=%g", sx, lx)
	}
}

// firstLineX returns the x coordinate of the first lineTo command, which the
// curved-arrow preset emits at the arrow head's neck.
func firstLineX(t *testing.T, p Path) float64 {
	t.Helper()
	for _, c := range p.Commands {
		if c.Type == "L" && len(c.Points) > 0 {
			return c.Points[0].X
		}
	}
	t.Fatal("curved arrow produced no line command")
	return 0
}

func TestCurvedArrowMaterializesAuthoredBandAndHead(t *testing.T) {
	// 200x100 box, adj1=0.40 band thickness, adj2=0.80 head height, adj3=0.25
	// head length. maxAdj2 clamps the head height to 50000*h/ss = 0.5, so
	// th = 40 and aw = 50, giving the band's outer radius
	// hR = h/2 - (th+aw)/4 = 27.5, with the neck at x = w - ss*adj3 = 175.
	p, err := (&PresetGeometry{Name: "curvedRightArrow", Adjustments: []float64{0.40, 0.80, 0.25}}).GetPath(200, 100)
	if err != nil {
		t.Fatal(err)
	}
	startY := p.Commands[0].Points[0].Y
	if math.Abs(startY-27.5) > 0.001 {
		t.Fatalf("band radius not materialized: start y=%g want 27.5", startY)
	}
	neckX := firstLineX(t, p)
	if math.Abs(neckX-175) > 0.01 {
		t.Fatalf("head length not materialized: neck x=%g want 175", neckX)
	}
	// The band closes on a straight tail cut: the subpath's Z follows the
	// band's inner arc, which ends one shaft thickness below the outer start.
	firstClose := -1
	for i, c := range p.Commands {
		if c.Type == "Z" {
			firstClose = i
			break
		}
	}
	if firstClose < 0 {
		t.Fatal("curved arrow must close its band")
	}
	inner := p.Commands[firstClose-1].Points[2]
	if math.Abs(inner.X) > 0.01 || math.Abs(inner.Y-(startY+40)) > 0.01 {
		t.Fatalf("band inner edge ends at %+v, want the tail cut at (0, %g)", inner, startY+40)
	}
	// The preset's second subpath carries the band's tail; without it the
	// arrow loses the upper half of its band.
	if len(p.Commands) <= firstClose+1 || p.Commands[firstClose+1].Type != "M" {
		t.Fatalf("curved arrow must draw the tail subpath after the band: %+v", p.Commands[firstClose:])
	}
}

func TestCurvedArrowDirectionsRotateSameAdjustedGeometry(t *testing.T) {
	for _, name := range []string{"curvedRightArrow", "curvedLeftArrow", "curvedUpArrow", "curvedDownArrow"} {
		p, err := (&PresetGeometry{Name: name, Adjustments: []float64{0.30, 0.65, 0.30}}).GetPath(200, 100)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		// Band subpath (move, arc, four edges, arc, close) plus the tail
		// subpath (move, two arcs, close); arcs split into cubics.
		if len(p.Commands) < 12 {
			t.Fatalf("%s commands=%d want at least 12", name, len(p.Commands))
		}
		if p.Commands[len(p.Commands)-1].Type != "Z" {
			t.Fatalf("%s must close its tail subpath", name)
		}
		for _, c := range p.Commands {
			for _, pt := range c.Points {
				if math.IsNaN(pt.X) || math.IsNaN(pt.Y) || math.IsInf(pt.X, 0) || math.IsInf(pt.Y, 0) {
					t.Fatalf("%s produced invalid point %+v", name, pt)
				}
			}
		}
	}
}
