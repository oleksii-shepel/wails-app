package drawing

import (
    "math"
    "testing"
)

func TestCurvedArrowUsesAuthoredHeadLength(t *testing.T) {
    short, err := (&PresetGeometry{Name:"curvedRightArrow", Adjustments:[]float64{0.25, 0.60, 0.15}}).GetPath(200,100)
    if err != nil { t.Fatal(err) }
    long, err := (&PresetGeometry{Name:"curvedRightArrow", Adjustments:[]float64{0.25, 0.60, 0.40}}).GetPath(200,100)
    if err != nil { t.Fatal(err) }
    // Command 2 is the head-top point; a longer head starts farther left.
    sx := short.Commands[2].Points[0].X
    lx := long.Commands[2].Points[0].X
    if !(lx < sx) { t.Fatalf("longer head should move neck left: short=%g long=%g", sx, lx) }
}

func TestCurvedArrowUsesAuthoredShaftAndHeadWidth(t *testing.T) {
    p, err := (&PresetGeometry{Name:"curvedRightArrow", Adjustments:[]float64{0.40, 0.80, 0.25}}).GetPath(200,100)
    if err != nil { t.Fatal(err) }
    startY := p.Commands[0].Points[0].Y
    headTopY := p.Commands[2].Points[0].Y
    if math.Abs(startY-78) > 0.001 { t.Fatalf("shaft thickness not materialized: start y=%g want 78", startY) }
    if math.Abs(headTopY-18) > 0.001 { t.Fatalf("head width not materialized: top y=%g want 18", headTopY) }
}

func TestCurvedArrowDirectionsRotateSameAdjustedGeometry(t *testing.T) {
    for _, name := range []string{"curvedRightArrow","curvedLeftArrow","curvedUpArrow","curvedDownArrow"} {
        p, err := (&PresetGeometry{Name:name, Adjustments:[]float64{0.30,0.65,0.30}}).GetPath(200,100)
        if err != nil { t.Fatalf("%s: %v", name, err) }
        if len(p.Commands) != 8 { t.Fatalf("%s commands=%d want 8", name, len(p.Commands)) }
        for _, c := range p.Commands {
            for _, pt := range c.Points {
                if math.IsNaN(pt.X)||math.IsNaN(pt.Y)||math.IsInf(pt.X,0)||math.IsInf(pt.Y,0) {
                    t.Fatalf("%s produced invalid point %+v", name, pt)
                }
            }
        }
    }
}
