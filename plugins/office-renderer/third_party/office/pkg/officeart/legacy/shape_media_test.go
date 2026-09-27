package legacy

import (
	"testing"

	"office/pkg/officeart/legacy/escher"
)

func TestResolveShapeMedia(t *testing.T) {
	node := &escher.ShapeNode{
		Properties: []escher.Property{{ID: 0x0104, Value: 2}},
	}
	store := BStore{
		2: BSEntry{Media: Media{MIME: "image/png", Data: []byte{1}}},
	}
	media, ok := ResolveShapeMedia(node, store)
	if !ok || media.MIME != "image/png" || len(media.Data) != 1 {
		t.Fatalf("media=%#v ok=%v", media, ok)
	}
}

func TestPictureRefPreservesPictureOwnershipWithoutBStore(t *testing.T) {
	node := &escher.ShapeNode{
		ShapeType: 75, // mso_sptPictureFrame
		Properties: []escher.Property{
			{ID: 0x0081, Value: 0}, // default text inset can appear on Word picture frames
			{ID: 0x0104, Value: 1, IsBlip: true},
		},
	}
	shape := ComposeShape(node, nil)
	if shape == nil {
		t.Fatal("ComposeShape returned nil")
	}
	if shape.Kind != "picture" {
		t.Fatalf("shape kind = %q, want picture", shape.Kind)
	}
	if shape.Image != nil {
		t.Fatalf("shape image = %#v, want unresolved payload", shape.Image)
	}
}
