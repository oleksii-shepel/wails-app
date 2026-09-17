package drawing

import "testing"

func TestTrianglePresetUsesNormalizedLegacyApexAdjustment(t *testing.T) {
	pg := &PresetGeometry{Name: "triangle", Adjustments: []float64{0.25}}
	path, err := pg.GetPath(200, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(path.Commands) < 2 || len(path.Commands[1].Points) != 1 {
		t.Fatalf("unexpected path: %#v", path)
	}
	got := path.Commands[1].Points[0]
	if got.X != 50 || got.Y != 0 {
		t.Fatalf("apex = %#v, want (50,0)", got)
	}
}

func TestTrianglePresetDefaultsApexToCenter(t *testing.T) {
	pg := &PresetGeometry{Name: "triangle"}
	path, err := pg.GetPath(200, 100)
	if err != nil {
		t.Fatal(err)
	}
	got := path.Commands[1].Points[0]
	if got.X != 100 || got.Y != 0 {
		t.Fatalf("apex = %#v, want (100,0)", got)
	}
}

func TestAdjustedQuadrilateralPresetsDoNotSelfInvert(t *testing.T) {
	for _, name := range []string{"parallelogram", "trapezoid"} {
		pg := &PresetGeometry{Name: name, Adjustments: []float64{0.9}}
		path, err := pg.GetPath(200, 100)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, cmd := range path.Commands {
			for _, pt := range cmd.Points {
				if pt.X < 0 || pt.X > 200 || pt.Y < 0 || pt.Y > 100 {
					t.Fatalf("%s point outside bounds: %#v", name, pt)
				}
			}
		}
	}
}
