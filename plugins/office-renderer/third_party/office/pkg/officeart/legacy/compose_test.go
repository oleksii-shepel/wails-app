package legacy

import (
	"encoding/binary"
	"testing"

	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

func TestComposeShapeResolvesPictureRecursively(t *testing.T) {
	root := &escher.ShapeNode{
		ShapeID: 1,
		Children: []*escher.ShapeNode{
			{
				ShapeID: 2,
				Properties: []escher.Property{
					{ID: 0x0104, Value: 3},
				},
			},
		},
	}
	store := BStore{
		3: BSEntry{Media: Media{MIME: "image/png", Data: []byte{1, 2, 3}}},
	}

	got := ComposeShape(root, store)
	if got == nil || len(got.Children) != 1 {
		t.Fatalf("shape = %#v", got)
	}
	child := got.Children[0]
	if child.Image == nil || child.Image.MIMEType != "image/png" || len(child.Image.Data) != 3 {
		t.Fatalf("image = %#v", child.Image)
	}
	if child.Kind != ir.ShapePicture {
		t.Fatalf("kind = %q, want picture", child.Kind)
	}
}

func TestComposeStreamPreservesRootOrder(t *testing.T) {
	makeShape := func(id uint32) []byte {
		fsp := make([]byte, 8)
		binary.LittleEndian.PutUint32(fsp[0:4], id)
		fspRecord := testEscherRecord(2, 0, escher.RecordFSP, fsp)
		return testEscherRecord(0xF, 0, escher.RecordSpContainer, fspRecord)
	}
	stream := append(makeShape(11), makeShape(22)...)

	got := ComposeStream(stream, nil)
	if len(got) != 2 || got[0].ID != 11 || got[1].ID != 22 {
		t.Fatalf("got %#v", got)
	}
}
