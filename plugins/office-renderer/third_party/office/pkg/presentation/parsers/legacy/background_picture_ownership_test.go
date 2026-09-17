package legacy

import (
	"bytes"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
	"office/pkg/ooxml/drawing"
)

func TestLegacyPictureBackgroundFillOwnsBackgroundBlip(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xd9}
	node := &officeartescher.ShapeNode{
		Flags:      0x00000400,
		Properties: []officeartescher.Property{{ID: 0x0186, Value: 2, IsBlip: true}},
	}
	scene := legacyPptVisualScene{dispatch: legacyPptSlideVisualDispatch{}}
	scene.dispatch.containers.events = []OfficeArtEvent{{ShapeStart: true, ShapeNode: node}}
	fill, ok := legacyPptPictureBackgroundFill(scene, map[uint32]*legacyPptPicture{2: {Data: jpeg, MIME: "image/jpeg"}}).(drawing.PictureFill)
	if !ok {
		t.Fatalf("fill = %T, want drawing.PictureFill", fill)
	}
	if !fill.Stretch || fill.ContentType != "image/jpeg" || !bytes.Equal(fill.ImageData, jpeg) {
		t.Fatalf("picture background = %+v", fill)
	}
}
