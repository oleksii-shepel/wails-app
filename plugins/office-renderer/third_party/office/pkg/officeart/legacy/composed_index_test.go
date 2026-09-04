package legacy

import (
	"testing"

	"office/pkg/officeart/legacy/escher"
)

func TestComposeShapeIndexIncludesNestedShapes(t *testing.T) {
	nodes := []*escher.ShapeNode{
		{
			ShapeID: 10,
			Children: []*escher.ShapeNode{
				{ShapeID: 20},
				{ShapeID: 30},
			},
		},
	}
	index := ComposeShapeIndex(nodes, nil)
	for _, id := range []uint32{10, 20, 30} {
		if _, ok := index.Find(id); !ok {
			t.Fatalf("shape %d missing from composed index", id)
		}
	}
}
