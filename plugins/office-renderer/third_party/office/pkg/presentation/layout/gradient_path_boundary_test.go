package layout

import (
    "testing"
    "office/pkg/ooxml/drawing"
    "office/pkg/presentation/ir"
)

func TestResolveShapeVisualPlanCarriesPathGradientBoundary(t *testing.T) {
    s := &ir.Shape{Geometry: "ellipse", Bounds: ir.Rect{W: 914400, H: 457200}, Fill: drawing.GradientFill{GradientType: drawing.GradientTypePath, Stops: []drawing.GradientStop{{Position: 0, Color: "000000", Alpha: 1}, {Position: 1, Color: "FFFFFF", Alpha: 1}}}}
    p := ResolveShapeVisualPlan(s)
    if p.Fill.Gradient == nil { t.Fatal("missing gradient") }
    if got := p.Fill.Gradient.PathBoundary; got != "ellipse" { t.Fatalf("PathBoundary=%q want ellipse", got) }
}
