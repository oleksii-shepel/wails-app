package legacy

import "office/pkg/officeart/legacy/escher"

// ShapeIndex indexes a parsed OfficeArt tree by FSP shape ID. This is the
// common ownership bridge used by Word textbox lid->spid mapping and other
// application records that refer to OfficeArt shapes externally.
type ShapeIndex map[uint32]*escher.ShapeNode

func IndexShapes(nodes []*escher.ShapeNode) ShapeIndex {
	out := ShapeIndex{}
	var visit func(*escher.ShapeNode)
	visit = func(node *escher.ShapeNode) {
		if node == nil {
			return
		}
		if node.ShapeID != 0 {
			out[node.ShapeID] = node
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	for _, node := range nodes {
		visit(node)
	}
	return out
}

func (idx ShapeIndex) Find(shapeID uint32) (*escher.ShapeNode, bool) {
	node, ok := idx[shapeID]
	return node, ok
}
