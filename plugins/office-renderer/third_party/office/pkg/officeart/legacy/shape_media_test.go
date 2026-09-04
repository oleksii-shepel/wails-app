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
