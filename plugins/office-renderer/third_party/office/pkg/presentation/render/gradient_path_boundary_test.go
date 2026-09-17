package render

import (
    "strings"
    "testing"
    "office/pkg/ooxml/drawing"
    playout "office/pkg/presentation/layout"
)

func TestSVGPhysicalPathGradientUsesEllipseBoundaryRadius(t *testing.T) {
    g := &playout.GradientVisualPlan{Kind: drawing.GradientTypePath, PathBoundary: "ellipse", CenterX: .5, CenterY: .5, HasCenter: true, Stops: []playout.GradientStopVisualPlan{{Position: 0, Color: "000000", Alpha: 1}, {Position: 1, Color: "FFFFFF", Alpha: 1}}}
    got := svgPhysicalGradientDefinition(g, "g")
    if !strings.Contains(got, `r="50.0000%"`) { t.Fatalf("ellipse ShadeShape radius not boundary-correct: %s", got) }
}
