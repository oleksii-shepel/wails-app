package legacy

import (
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

// A picture-frame SpContainer is still authored OfficeArt even when its BLIP or
// ExObj relationship cannot be resolved. The generic OfficeArt projection must
// retain its geometry/visual instead of reserving the ID for a family projector
// that produced nothing.
func TestParserCompletenessUnresolvedPictureFrameFallsBackToCanonicalOfficeArt(t *testing.T) {
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{{
		ShapeID: 91,
		Node: &officeartescher.ShapeNode{
			ShapeID:      91,
			ShapeType:    legacyOfficeArtShapePicture,
			ClientAnchor: legacyParserCompletenessClientAnchor(10, 20, 210, 120),
		},
	}}

	projection := scene.completeProjection(legacySlideText{}, nil, nil, nil, nil, nil, nil, legacyPptDefaultCanvas())
	if len(projection.Shapes) != 1 {
		t.Fatalf("shapes = %d, want unresolved picture frame retained as OfficeArt shape", len(projection.Shapes))
	}
	if projection.Shapes[0].OriginalShapeID != 91 {
		t.Fatalf("shape id = %d, want 91", projection.Shapes[0].OriginalShapeID)
	}
	if gaps := projection.unclaimed(); len(gaps) != 0 {
		t.Fatalf("unresolved picture frame must be claimed: %+v", gaps)
	}
}

func legacyParserCompletenessClientAnchor(left, top, right, bottom int16) []byte {
	// PowerPoint ClientAnchor SmallRectStruct: top,left,right,bottom.
	b := make([]byte, 8)
	put := func(off int, v int16) {
		b[off] = byte(v)
		b[off+1] = byte(uint16(v) >> 8)
	}
	put(0, top)
	put(2, left)
	put(4, right)
	put(6, bottom)
	return b
}
