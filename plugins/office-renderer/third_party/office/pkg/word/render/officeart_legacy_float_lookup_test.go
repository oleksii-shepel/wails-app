package render

import (
	"testing"

	resolve "office/pkg/word/ir"
	"office/pkg/word/layout"
)

func TestLegacyOfficeArtFloatLookupSurvivesShapeCopy(t *testing.T) {
	layoutShape := &resolve.Shape{LegacyShapeID: 1028, Floating: true}
	renderShape := &resolve.Shape{LegacyShapeID: 1028, Floating: true}
	fb := layout.FloatBox{Shape: layoutShape, X: 72, Y: 144, Width: 204, Height: 71}

	r := &htmlRenderer{
		floatByShape: map[*resolve.Shape]layout.FloatBox{
			layoutShape: fb,
		},
		floatByLegacyShapeID: map[uint32]layout.FloatBox{
			1028: fb,
		},
	}
	got, ok := r.shapeFloatBox(renderShape)
	if !ok {
		t.Fatal("expected copied legacy shape to resolve layout float by FSP.spid")
	}
	if got.X != 72 || got.Y != 144 || got.Width != 204 || got.Height != 71 {
		t.Fatalf("float = %+v", got)
	}
}
