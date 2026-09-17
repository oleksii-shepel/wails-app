package render

import (
	"strings"
	"testing"

	"office/pkg/presentation/ir"
)

func TestLegacyEndpointConnectorSerializesPhysicalFreeConnectorInsteadOfEmptySVG(t *testing.T) {
	start := ir.Point{X: 914400, Y: 914400}
	end := ir.Point{X: 2743200, Y: 1828800}
	connector := &ir.Shape{
		Kind:           ir.ShapeConnector,
		Bounds:         ir.Rect{X: start.X, Y: start.Y, W: end.X - start.X, H: end.Y - start.Y},
		ConnectorType:  "straightConnector1",
		ConnectorStart: &start,
		ConnectorEnd:   &end,
		LineColor:      "FFFF00",
		LineWidthPt:    1.5,
	}
	var out strings.Builder
	renderVisualShapes(&out, []*ir.Shape{connector}, 720, 540, &htmlRenderContext{})
	html := out.String()
	if !strings.Contains(html, "<line ") {
		t.Fatalf("endpoint connector did not serialize its physical line: %s", html)
	}
}
