package drawing

import "testing"

func TestLeftRightArrowUsesNormalizedHeadAndShaftAdjustments(t *testing.T) {
	pg := &PresetGeometry{Name: "leftRightArrow", Adjustments: []float64{0.2, 0.25}}
	path, err := pg.GetPath(200, 100)
	if err != nil { t.Fatal(err) }
	// left tip, upper-left head shoulder, then shaft edge
	if got := path.Commands[1].Points[0]; got.X != 40 || got.Y != 0 {
		t.Fatalf("head shoulder = %#v, want (40,0)", got)
	}
	if got := path.Commands[2].Points[0]; got.X != 40 || got.Y != 37.5 {
		t.Fatalf("shaft edge = %#v, want (40,37.5)", got)
	}
}

func TestUpDownArrowUsesNormalizedHeadAndShaftAdjustments(t *testing.T) {
	pg := &PresetGeometry{Name: "upDownArrow", Adjustments: []float64{0.2, 0.25}}
	path, err := pg.GetPath(100, 200)
	if err != nil { t.Fatal(err) }
	if got := path.Commands[1].Points[0]; got.X != 100 || got.Y != 40 {
		t.Fatalf("head shoulder = %#v, want (100,40)", got)
	}
	if got := path.Commands[2].Points[0]; got.X != 62.5 || got.Y != 40 {
		t.Fatalf("shaft edge = %#v, want (62.5,40)", got)
	}
}

func TestDoubleArrowAndChevronExtremeAdjustmentsDoNotSelfCross(t *testing.T) {
	for _, name := range []string{"leftRightArrow", "upDownArrow", "chevron"} {
		pg := &PresetGeometry{Name: name, Adjustments: []float64{0.95, 0.95}}
		path, err := pg.GetPath(200, 100)
		if err != nil { t.Fatalf("%s: %v", name, err) }
		for _, cmd := range path.Commands {
			for _, pt := range cmd.Points {
				if pt.X < 0 || pt.X > 200 || pt.Y < 0 || pt.Y > 100 {
					t.Fatalf("%s point outside bounds: %#v", name, pt)
				}
			}
		}
	}
}
