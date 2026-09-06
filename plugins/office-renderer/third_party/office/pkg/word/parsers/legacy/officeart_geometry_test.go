package legacy

import (
	"testing"

	officeartlegacy "office/pkg/officeart/legacy"
)

func TestLegacyOfficeArtShapeTypeToGeometryDelegatesToSharedMSOSPTMap(t *testing.T) {
	for _, shapeType := range []uint16{
		1, 5, 20, 22, 23,
		32, 33, 34, 40,
		41, 52, 56, 57, 58, 60,
		66, 69, 70, 75, 76,
		84, 89, 96, 99, 106,
		109, 120, 129, 135,
		0, 14, 24, 136, 999,
	} {
		got := legacyOfficeArtShapeTypeToGeometry(shapeType)
		want := officeartlegacy.ShapeTypeToPreset(shapeType)
		if got != want {
			t.Fatalf("legacyOfficeArtShapeTypeToGeometry(%d) = %q, shared = %q", shapeType, got, want)
		}
	}
}
