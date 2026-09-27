package officeart

import (
	"encoding/binary"

	officeartir "office/pkg/officeart/ir"
	officeartlegacy "office/pkg/officeart/legacy"
	officeartescher "office/pkg/officeart/legacy/escher"
	"office/pkg/ooxml/units"
)

// Catalog is the canonical DOC-facing view of OfficeArt shapes keyed by FSP.spid.
// It deliberately keeps format-neutral OfficeArt semantics separate from Word
// anchor/story ownership, which is handled by doc/binary and the Word97/pre97
// stream layers.
type Catalog struct {
	Shapes map[uint32]*officeartir.Shape
}

// DecodeStreams composes all DOC OfficeArt shape/group trees through the shared
// OfficeArt frontend and indexes every root/child shape by source shape ID. DOC
// child anchors are normalized here because their coordinate interpretation is
// application-specific and must not leak into the shared OfficeArt frontend.
func DecodeStreams(streams ...[]byte) Catalog {
	out := Catalog{Shapes: make(map[uint32]*officeartir.Shape)}
	for _, stream := range streams {
		for _, shape := range decodeDOCStream(stream) {
			indexShape(shape, out.Shapes)
		}
	}
	if len(out.Shapes) == 0 {
		out.Shapes = nil
	}
	return out
}

// MergeMissing supplements a canonical catalog with shapes that are absent from
// it while preserving every existing authoritative entry. This is used for
// compatibility with DOC producers that place otherwise valid OfficeArt records
// outside FibDggInfo: recovery may fill holes, but must never replace DggInfo/Data
// semantics with a broad whole-WordDocument scan.
func (c Catalog) MergeMissing(fallback Catalog) Catalog {
	if len(fallback.Shapes) == 0 {
		return c
	}
	if len(c.Shapes) == 0 {
		out := Catalog{Shapes: make(map[uint32]*officeartir.Shape, len(fallback.Shapes))}
		for id, shape := range fallback.Shapes {
			out.Shapes[id] = shape
		}
		return out
	}
	out := Catalog{Shapes: make(map[uint32]*officeartir.Shape, len(c.Shapes)+len(fallback.Shapes))}
	for id, shape := range c.Shapes {
		out.Shapes[id] = shape
	}
	for id, shape := range fallback.Shapes {
		if _, exists := out.Shapes[id]; !exists {
			out.Shapes[id] = shape
		}
	}
	return out
}

func decodeDOCStream(stream []byte) []*officeartir.Shape {
	if len(stream) == 0 {
		return nil
	}
	// Word emits groups as an SpgrContainer whose first child SpContainer is
	// the group shape and whose remaining sibling records are its members. Scan
	// complete group containers first so their sibling relationship is not lost.
	records := officeartescher.Scan(stream, func(h officeartescher.Header) bool {
		return h.IsContainer() && (h.Type == officeartescher.RecordSpgrContainer || h.Type == officeartescher.RecordSpContainer)
	})
	out := make([]*officeartir.Shape, 0, len(records))
	for _, record := range records {
		node, err := docShapeNode(record)
		if err != nil || node == nil {
			continue
		}
		shape := officeartlegacy.ComposeShape(node, nil)
		if shape == nil {
			continue
		}
		applyDOCChildAnchors(node, shape)
		out = append(out, shape)
	}
	return out
}

// docShapeNode reconstructs the application-level tree used by Word. The
// shared Escher parser correctly preserves SpgrContainer records, but an
// SpgrContainer represents a group by sibling SpContainers rather than by
// nesting members inside the group's own SpContainer.
func docShapeNode(record officeartescher.Record) (*officeartescher.ShapeNode, error) {
	switch record.Header.Type {
	case officeartescher.RecordSpContainer:
		return officeartescher.ShapeNodeFromRecord(record)
	case officeartescher.RecordSpgrContainer:
		var root *officeartescher.ShapeNode
		for _, child := range record.Children {
			var node *officeartescher.ShapeNode
			var err error
			switch child.Header.Type {
			case officeartescher.RecordSpContainer, officeartescher.RecordSpgrContainer:
				node, err = docShapeNode(child)
			default:
				continue
			}
			if err != nil {
				return nil, err
			}
			if node == nil {
				continue
			}
			if root == nil {
				root = node
				continue
			}
			root.Children = append(root.Children, node)
		}
		return root, nil
	default:
		return nil, nil
	}
}

func applyDOCChildAnchors(node *officeartescher.ShapeNode, shape *officeartir.Shape) {
	if node == nil || shape == nil {
		return
	}
	if len(node.ChildAnchor) >= 16 {
		left := int32(binary.LittleEndian.Uint32(node.ChildAnchor[0:4]))
		top := int32(binary.LittleEndian.Uint32(node.ChildAnchor[4:8]))
		right := int32(binary.LittleEndian.Uint32(node.ChildAnchor[8:12]))
		bottom := int32(binary.LittleEndian.Uint32(node.ChildAnchor[12:16]))
		if right > left && bottom > top {
			shape.Transform.Rect = officeartir.Rect{
				X: units.TwipsToEMU(int64(left)),
				Y: units.TwipsToEMU(int64(top)),
				W: units.TwipsToEMU(int64(right - left)),
				H: units.TwipsToEMU(int64(bottom - top)),
			}
		}
	}
	limit := len(node.Children)
	if len(shape.Children) < limit {
		limit = len(shape.Children)
	}
	for i := 0; i < limit; i++ {
		applyDOCChildAnchors(node.Children[i], shape.Children[i])
	}

	// Word stores group-member ChildAnchor rectangles in the FSPGR coordinate
	// domain, not in the group's eventual FSPA page rectangle. The domain may
	// have a negative origin while FSPA contains only the enclosing group box.
	// Normalize members into that local box before layout/rendering.
	if node.GroupRect != nil && len(shape.Children) > 0 {
		domain, ok := officeartlegacy.NormalizeRect(
			units.TwipsToEMU(int64(node.GroupRect.Left)),
			units.TwipsToEMU(int64(node.GroupRect.Top)),
			units.TwipsToEMU(int64(node.GroupRect.Right)),
			units.TwipsToEMU(int64(node.GroupRect.Bottom)),
		)
		if ok {
			target := shape.Transform.Rect
			if target.W <= 0 || target.H <= 0 {
				target = officeartir.Rect{W: domain.W, H: domain.H}
			}
			officeartlegacy.ComposeGroupChildren(shape.Children, officeartlegacy.GroupFrame{
				Domain: domain,
				Target: target,
				Valid:  true,
			})
			if shape.Transform.Rect.W <= 0 || shape.Transform.Rect.H <= 0 {
				shape.Transform.Rect = target
			}
		}
	}
}

func indexShape(shape *officeartir.Shape, index map[uint32]*officeartir.Shape) {
	if shape == nil {
		return
	}
	if shape.ID != 0 {
		// Later streams win. DOC commonly spreads OfficeArt records between the
		// DggInfo range and Data stream; the more specific later record should
		// replace an earlier partial projection.
		index[shape.ID] = shape
	}
	for _, child := range shape.Children {
		indexShape(child, index)
	}
}

func (c Catalog) Find(shapeID uint32) (*officeartir.Shape, bool) {
	if c.Shapes == nil {
		return nil, false
	}
	shape, ok := c.Shapes[shapeID]
	return shape, ok
}

// TextFlows projects only the textbox vertical-flow vocabulary needed by Word
// story ownership. Ordinary horizontal text is intentionally omitted.
func (c Catalog) TextFlows() map[uint32]string {
	var out map[uint32]string
	for id, shape := range c.Shapes {
		if shape == nil || shape.Text == nil || shape.Text.Vert == "" {
			continue
		}
		if out == nil {
			out = make(map[uint32]string)
		}
		out[id] = string(shape.Text.Vert)
	}
	return out
}

// OwnsTextBox reports structural/text-property evidence that an OfficeArt shape
// owns a textbox. Story contents still come from PlcftxbxTxt/PlcfHdrtxbxTxt.
func (c Catalog) OwnsTextBox(shapeID uint32) bool {
	shape, ok := c.Find(shapeID)
	return ok && shape != nil && shape.Text != nil
}
