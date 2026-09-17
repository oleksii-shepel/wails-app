package layout

import "office/pkg/presentation/ir"

// MaterializeVisualTree is the layout boundary between canonical presentation IR
// and physical rendering. It clones the supplied canonical nodes, resolves every
// nested group coordinate system in parent-to-slide order, and returns a flat
// paint-order list. Renderers must consume this result instead of interpreting
// ShapeGroup or GroupTransform themselves.
type materializedAncestorPaint struct {
	Bounds      ir.Rect
	RotationDeg float64
}

type materializedAncestorClip struct {
	Bounds      ir.Rect
	Kind        string
	RotationDeg float64
	FlipH       bool
	FlipV       bool
}

type materializedVisualLeaf struct {
	// Source preserves semantic identity while Shape is the physical clone.
	Source        *ir.Shape
	Shape         *ir.Shape
	Ancestors     []materializedAncestorPaint
	AncestorClips []materializedAncestorClip
}

func MaterializeVisualTree(shapes []*ir.Shape) []*ir.Shape {
	leaves := materializeVisualTreeWithAncestry(shapes)
	out := make([]*ir.Shape, 0, len(leaves))
	for _, leaf := range leaves {
		out = append(out, leaf.Shape)
	}
	return out
}

// materializeVisualTreeWithAncestry preserves the physical envelopes of every
// group crossed while flattening the canonical hierarchy. The envelopes are
// not clips by themselves: PowerPoint groups may paint outside their nominal
// bounds. They are composition metadata used by physical layout for transformed
// child/effect intersection and future explicit group clipping.
func materializeVisualTreeWithAncestry(shapes []*ir.Shape) []materializedVisualLeaf {
	out := make([]materializedVisualLeaf, 0, len(shapes))
	for _, shape := range shapes {
		materializeVisualNodeWithAncestry(shape, nil, nil, nil, &out)
	}
	return out
}

func materializeVisualNodeWithAncestry(source *ir.Shape, ancestors []ir.VisualTransform, ancestorPaint []materializedAncestorPaint, ancestorClips []materializedAncestorClip, out *[]materializedVisualLeaf) {
	if source == nil {
		return
	}
	if source.Kind == ir.ShapeGroup {
		nextTransforms := ancestors
		nextPaint := ancestorPaint
		nextClips := ancestorClips
		if source.GroupTransform != nil {
			// Offset/Extent are the group's target rectangle in its immediate
			// parent coordinate system. Materialize that envelope through the
			// already-active ancestors, but not through the group's own child map.
			envelope := &ir.Shape{
				Bounds: ir.Rect{
					X: source.GroupTransform.Offset.X, Y: source.GroupTransform.Offset.Y,
					W: source.GroupTransform.Extent.W, H: source.GroupTransform.Extent.H,
				},
				RotationDeg: source.GroupTransform.Rotation,
				FlipH:       source.GroupTransform.FlipH,
				FlipV:       source.GroupTransform.FlipV,
			}
			for i := len(ancestors) - 1; i >= 0; i-- {
				ir.ApplyVisualTransform(envelope, ancestors[i])
			}
			nextPaint = append(append([]materializedAncestorPaint(nil), ancestorPaint...), materializedAncestorPaint{Bounds: envelope.Bounds, RotationDeg: envelope.RotationDeg})
			groupVisual := ResolveShapeVisualPlan(source)
			if groupVisual.ClipContent {
				nextClips = append(append([]materializedAncestorClip(nil), ancestorClips...), materializedAncestorClip{Bounds: envelope.Bounds, Kind: groupVisual.Geometry.ClipKind, RotationDeg: envelope.RotationDeg, FlipH: envelope.FlipH, FlipV: envelope.FlipV})
			}
			nextTransforms = append(append([]ir.VisualTransform(nil), ancestors...), *source.GroupTransform)
		}
		for _, child := range source.Children {
			materializeVisualNodeWithAncestry(child, nextTransforms, nextPaint, nextClips, out)
		}
		return
	}

	clone := cloneVisualShape(source)
	if clone == nil {
		return
	}
	for i := len(ancestors) - 1; i >= 0; i-- {
		ir.ApplyVisualTransform(clone, ancestors[i])
	}
	clone.GroupTransform = nil
	clone.Children = nil
	*out = append(*out, materializedVisualLeaf{
		Source:        source,
		Shape:         clone,
		Ancestors:     append([]materializedAncestorPaint(nil), ancestorPaint...),
		AncestorClips: append([]materializedAncestorClip(nil), ancestorClips...),
	})
}

func materializeVisualNode(source *ir.Shape, ancestors []ir.VisualTransform, out *[]*ir.Shape) {
	if source == nil {
		return
	}
	if source.Kind == ir.ShapeGroup {
		next := ancestors
		if source.GroupTransform != nil {
			next = append(append([]ir.VisualTransform(nil), ancestors...), *source.GroupTransform)
		}
		for _, child := range source.Children {
			materializeVisualNode(child, next, out)
		}
		return
	}

	clone := cloneVisualShape(source)
	if clone == nil {
		return
	}
	// A leaf cannot own a group coordinate system after layout. Apply transforms
	// from its immediate parent outward so each mapping sees the coordinate space
	// authored by the previous level.
	for i := len(ancestors) - 1; i >= 0; i-- {
		ir.ApplyVisualTransform(clone, ancestors[i])
	}
	clone.GroupTransform = nil
	clone.Children = nil
	*out = append(*out, clone)
}

// MaterializeGroupChildren resolves a canonical group's local visual composition
// into slide-space shapes. Parsers preserve the hierarchy and transform in IR;
// layout is the first layer allowed to flatten that coordinate relationship.
// Returned shapes are clones, so rendering never mutates canonical IR.
func MaterializeGroupChildren(group *ir.Shape) []*ir.Shape {
	if group == nil || group.Kind != ir.ShapeGroup || len(group.Children) == 0 {
		return nil
	}
	out := make([]*ir.Shape, 0, len(group.Children))
	for _, child := range group.Children {
		clone := cloneVisualShape(child)
		if clone == nil {
			continue
		}
		materializeNestedGroup(clone)
		if group.GroupTransform != nil {
			ir.ApplyVisualTransform(clone, *group.GroupTransform)
		}
		out = append(out, clone)
	}
	return out
}

func cloneVisualShape(shape *ir.Shape) *ir.Shape {
	if shape == nil {
		return nil
	}
	clone := *shape
	if shape.ConnectorStart != nil {
		p := *shape.ConnectorStart
		clone.ConnectorStart = &p
	}
	if shape.ConnectorEnd != nil {
		p := *shape.ConnectorEnd
		clone.ConnectorEnd = &p
	}
	clone.ConnectorPath = append([]ir.Point(nil), shape.ConnectorPath...)
	if shape.GroupTransform != nil {
		t := *shape.GroupTransform
		clone.GroupTransform = &t
	}
	if len(shape.Children) > 0 {
		clone.Children = make([]*ir.Shape, 0, len(shape.Children))
		for _, child := range shape.Children {
			clone.Children = append(clone.Children, cloneVisualShape(child))
		}
	}
	return &clone
}

// materializeNestedGroup resolves a nested group's own local coordinate system
// before an ancestor transform is applied. This preserves transform order:
// leaf local -> nested group -> parent group -> slide.
func materializeNestedGroup(shape *ir.Shape) {
	if shape == nil || shape.Kind != ir.ShapeGroup || shape.GroupTransform == nil {
		return
	}
	transform := *shape.GroupTransform
	for _, child := range shape.Children {
		materializeNestedGroup(child)
		ir.ApplyVisualTransform(child, transform)
	}
	shape.GroupTransform = nil
}
