package resolve

import (
	"testing"

	"office/pkg/ooxml/drawing"
)

func TestSyncSheetOfficeArtDrawing(t *testing.T) {
	d := &Drawing{
		Name:     "shape",
		Type:     DrawingTypeConnector,
		Position: DrawingPosition{X: 1, Y: 2, Width: 300, Height: 100},
		Shape: &DrawingShape{
			Geometry:      "bentConnector3",
			Adjustments:   []float64{0.25},
			ConnectorType: "bentConnector3",
			Fill:          drawing.NoFill{},
		},
	}
	got := syncSheetOfficeArtDrawing(d)
	if got == nil || got.Connector == nil || got.Connector.Type != "bentConnector3" || got.Transform.Rect.W != 300 {
		t.Fatalf("officeart=%#v", got)
	}
}
