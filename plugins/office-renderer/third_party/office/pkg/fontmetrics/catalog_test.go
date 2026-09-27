package fontmetrics

import "testing"

func TestCatalogMatchKeepsFamilyBoundaryAndChoosesStyle(t *testing.T) {
	regular := &Face{UnitsPerEm: 1000}
	bold := &Face{UnitsPerEm: 1000}
	other := &Face{UnitsPerEm: 1000}
	c := &Catalog{Faces: []NamedFace{
		{Family: "Demo Sans", Weight: 400, Face: regular},
		{Family: "Demo Sans", Weight: 700, Face: bold},
		{Family: "Other", Weight: 700, Face: other},
	}}
	if got := c.Match("demo   sans", true, false); got != bold {
		t.Fatalf("bold match = %p, want %p", got, bold)
	}
	if got := c.Match("Missing", false, false); got != nil {
		t.Fatalf("cross-family fallback = %p, want nil", got)
	}
}

func TestCatalogMeasureTextUsesPairAdjustment(t *testing.T) {
	face := &Face{
		UnitsPerEm:  1000,
		RuneToGlyph: map[rune]uint16{'A': 1, 'B': 2},
		Advances:    []uint16{0, 600, 500},
		gpos:        map[uint32]int16{uint32(1)<<16 | 2: -100},
	}
	c := &Catalog{Faces: []NamedFace{{Family: "Demo", Weight: 400, Face: face}}}
	got, ok := c.MeasureText("AB", "Demo", false, false, 20)
	if !ok {
		t.Fatal("MeasureText = not available")
	}
	// 12 + (-2) + 10 = 20.
	if got != 20 {
		t.Fatalf("MeasureText = %g, want 20", got)
	}
}
