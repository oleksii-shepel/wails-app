package legacy

import (
	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

// ComposeShape builds the complete shared OfficeArt shape tree from one parsed
// Escher node. ShapeFromEscher owns visual/text/geometry semantics; this layer
// adds media ownership through the shared BStore.
func ComposeShape(node *escher.ShapeNode, store BStore) *ir.Shape {
	if node == nil {
		return nil
	}
	shape := ShapeFromEscher(node)
	if shape == nil {
		return nil
	}
	composeShapeMedia(node, shape, store)
	return shape
}

func composeShapeMedia(node *escher.ShapeNode, shape *ir.Shape, store BStore) {
	if node == nil || shape == nil {
		return
	}

	if media, ok := ResolveShapeMedia(node, store); ok {
		if shape.Image == nil {
			shape.Image = &ir.Image{}
		}
		shape.Image.MIMEType = media.MIME
		shape.Image.Data = append([]byte(nil), media.Data...)
		if len(shape.Children) == 0 && shape.Kind != ir.ShapeTextBox {
			shape.Kind = ir.ShapePicture
		}
	}

	// ShapeFromEscher preserves child order, so node/IR children can be walked
	// in parallel without another shape-ID lookup.
	limit := len(node.Children)
	if len(shape.Children) < limit {
		limit = len(shape.Children)
	}
	for i := 0; i < limit; i++ {
		composeShapeMedia(node.Children[i], shape.Children[i], store)
	}
}

// ComposeShapes preserves OfficeArt source order for a root list.
func ComposeShapes(nodes []*escher.ShapeNode, store BStore) []*ir.Shape {
	out := make([]*ir.Shape, 0, len(nodes))
	for _, node := range nodes {
		if shape := ComposeShape(node, store); shape != nil {
			out = append(out, shape)
		}
	}
	return out
}

// ComposeStream scans all top-level SpContainer records in an OfficeArt stream,
// parses them through the shared Escher tree, and composes the resulting IR.
func ComposeStream(data []byte, store BStore) []*ir.Shape {
	if len(data) == 0 {
		return nil
	}
	records := escher.Scan(data, func(h escher.Header) bool {
		return h.Type == escher.RecordSpContainer && h.IsContainer()
	})
	nodes := make([]*escher.ShapeNode, 0, len(records))
	for _, record := range records {
		node, err := escher.ShapeNodeFromRecord(record)
		if err == nil && node != nil {
			nodes = append(nodes, node)
		}
	}
	return ComposeShapes(nodes, store)
}
