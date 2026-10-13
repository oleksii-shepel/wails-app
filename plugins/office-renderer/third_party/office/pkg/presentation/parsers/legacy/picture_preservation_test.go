package legacy

import (
	"encoding/binary"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
	presentationir "office/pkg/presentation/ir"
)

func TestLegacyPictureProjectionPreservesShapeWhenBlipPayloadIsMissing(t *testing.T) {
	shape := legacyPptPictureShape{shapeID: 42, bounds: presentationir.Rect{X: 1, Y: 2, W: 30, H: 40}, blipRef: 7}
	got := buildLegacyPptPictureOrExternalObjectShape(1, shape, nil, legacyPptExternalObject{})
	if got == nil {
		t.Fatal("unresolved BLIP dropped the authored picture shape")
	}
	if got.Kind != presentationir.ShapeImage {
		t.Fatalf("kind = %v, want image", got.Kind)
	}
	if got.OriginalShapeID != 42 {
		t.Fatalf("shape id = %d, want 42", got.OriginalShapeID)
	}
	if got.Bounds != shape.bounds {
		t.Fatalf("bounds = %+v, want %+v", got.Bounds, shape.bounds)
	}
	if len(got.ImageData) != 0 {
		t.Fatalf("unresolved picture unexpectedly has %d payload bytes", len(got.ImageData))
	}
}

func TestLegacyScenePreservesPictureWhenEntirePictureStoreIsUnavailable(t *testing.T) {
	anchor := make([]byte, 16)
	binary.LittleEndian.PutUint32(anchor[0:4], 100)
	binary.LittleEndian.PutUint32(anchor[4:8], 200)
	binary.LittleEndian.PutUint32(anchor[8:12], 1100)
	binary.LittleEndian.PutUint32(anchor[12:16], 700)
	node := &officeartescher.ShapeNode{
		ShapeID: 91, ShapeType: legacyOfficeArtShapePicture, ClientAnchor: anchor,
		Properties: []officeartescher.Property{{ID: legacyOfficeArtPropPib, Value: 3, IsBlip: true}},
	}
	scene := legacyPptVisualScene{dispatch: legacyPptSlideVisualDispatch{containers: legacyPptShapeContainerScene{
		containers: []legacyPptShapeContainer{{ShapeID: 91, Node: node}},
		events:     []OfficeArtEvent{{ShapeStart: true, ShapeNode: node}},
	}}}

	got := scene.completeProjection(legacySlideText{}, nil, nil, nil, nil, nil, nil, legacyPptDefaultCanvas()).Shapes
	if len(got) != 1 {
		t.Fatalf("projected shapes = %d, want one preserved picture", len(got))
	}
	if got[0].Kind != presentationir.ShapeImage || got[0].OriginalShapeID != 91 {
		t.Fatalf("unresolved scene picture = %+v", got[0])
	}
	if len(got[0].ImageData) != 0 {
		t.Fatalf("unresolved picture unexpectedly has %d payload bytes", len(got[0].ImageData))
	}
}
