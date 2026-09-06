package legacy

import (
	officeartir "office/pkg/officeart/ir"
	docofficeart "office/pkg/word/parsers/doc/officeart"
)

// Writer tests intentionally validate serialized OfficeArt through the same
// canonical DOC decoder used by production parsing. These helpers preserve the
// old test vocabulary without restoring the removed production decoder.
func extractLegacyDrawingTextFlows(stream []byte) map[uint32]string {
	return docofficeart.DecodeStreams(stream).TextFlows()
}

type legacyOfficeArtShapeNode struct {
	ShapeID  uint32
	Bounds   *legacyOfficeArtRect
	Children []legacyOfficeArtShapeNode
}

func extractLegacyDrawingShapeTrees(stream []byte) map[uint32]legacyOfficeArtShapeNode {
	catalog := docofficeart.DecodeStreams(stream)
	if len(catalog.Shapes) == 0 {
		return nil
	}
	out := make(map[uint32]legacyOfficeArtShapeNode)
	for id, shape := range catalog.Shapes {
		if shape == nil || shape.ID != id {
			continue
		}
		out[id] = legacyWriterShapeNode(shape)
	}
	return out
}

func legacyWriterShapeNode(shape *officeartir.Shape) legacyOfficeArtShapeNode {
	if shape == nil {
		return legacyOfficeArtShapeNode{}
	}
	node := legacyOfficeArtShapeNode{ShapeID: shape.ID}
	r := shape.Transform.Rect
	if r.W != 0 || r.H != 0 || r.X != 0 || r.Y != 0 {
		const emuPerTwip = int64(635)
		node.Bounds = &legacyOfficeArtRect{
			Left:   int32(r.X / emuPerTwip),
			Top:    int32(r.Y / emuPerTwip),
			Right:  int32((r.X + r.W) / emuPerTwip),
			Bottom: int32((r.Y + r.H) / emuPerTwip),
		}
	}
	for _, child := range shape.Children {
		if child != nil {
			node.Children = append(node.Children, legacyWriterShapeNode(child))
		}
	}
	return node
}
