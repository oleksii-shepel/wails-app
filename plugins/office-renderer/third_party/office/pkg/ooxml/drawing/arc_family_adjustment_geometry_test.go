package drawing

import (
    "math"
    "testing"
)

func TestPieUsesStartAndEndAngleAdjustments(t *testing.T) {
    p, err := (&PresetGeometry{Name:"pie", Adjustments:[]float64{0.25, 0.5}}).GetPath(200,100)
    if err != nil { t.Fatal(err) }
    start := p.Commands[1].Points[0]
    end := p.Commands[2].Points[0]
    if math.Abs(start.X-100)>1e-6 || math.Abs(start.Y-100)>1e-6 { t.Fatalf("start=%+v", start) }
    if math.Abs(end.X-0)>1e-6 || math.Abs(end.Y-50)>1e-6 { t.Fatalf("end=%+v", end) }
}

func TestChordWrapsEndAngleAcrossZero(t *testing.T) {
    p, err := (&PresetGeometry{Name:"chord", Adjustments:[]float64{0.75, 0.25}}).GetPath(200,100)
    if err != nil { t.Fatal(err) }
    arc := p.Commands[1]
    if len(arc.Args)<5 || arc.Args[3] != 0 || arc.Args[4] != 1 { t.Fatalf("arc args=%v", arc.Args) }
}
