package legacy

import (
	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

type ComposedShapeIndex map[uint32]*ir.Shape

func ComposeShapeIndex(nodes []*escher.ShapeNode, store BStore) ComposedShapeIndex {
	index := make(ComposedShapeIndex)
	for _, shape := range ComposeShapes(nodes, store) {
		indexComposedShape(shape, index)
	}
	if len(index) == 0 {
		return nil
	}
	return index
}

func indexComposedShape(shape *ir.Shape, index ComposedShapeIndex) {
	if shape == nil {
		return
	}
	if shape.ID != 0 {
		index[shape.ID] = shape
	}
	for _, child := range shape.Children {
		indexComposedShape(child, index)
	}
}

func (i ComposedShapeIndex) Find(shapeID uint32) (*ir.Shape, bool) {
	shape, ok := i[shapeID]
	return shape, ok
}
