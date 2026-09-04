package legacy

import (
	"testing"

	"office/pkg/officeart/legacy/escher"
)

func TestIndexShapesIndexesNestedChildren(t *testing.T) {
	child := &escher.ShapeNode{ShapeID: 2}
	parent := &escher.ShapeNode{ShapeID: 1, Children: []*escher.ShapeNode{child}}
	idx := IndexShapes([]*escher.ShapeNode{parent})
	if _, ok := idx.Find(1); !ok {
		t.Fatal("missing parent")
	}
	if _, ok := idx.Find(2); !ok {
		t.Fatal("missing child")
	}
}
