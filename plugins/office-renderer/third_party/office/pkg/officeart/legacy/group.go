package legacy

import "office/pkg/officeart/ir"

// GroupFrame maps a child coordinate domain into its rendered target bounds.
type GroupFrame struct {
	Domain ir.Rect
	Target ir.Rect
	Valid  bool
}

// NormalizeRect canonicalizes potentially reversed OfficeArt coordinates.
func NormalizeRect(left, top, right, bottom int64) (ir.Rect, bool) {
	if right < left {
		left, right = right, left
	}
	if bottom < top {
		top, bottom = bottom, top
	}
	if right == left || bottom == top {
		return ir.Rect{}, false
	}
	return ir.Rect{X: left, Y: top, W: right - left, H: bottom - top}, true
}

// MapRectThroughGroup maps a child-space rectangle through an OfficeArt group
// frame. It preserves negative scale by normalizing the resulting rectangle.
func MapRectThroughGroup(local ir.Rect, frame GroupFrame) (ir.Rect, bool) {
	if !frame.Valid || frame.Domain.W == 0 || frame.Domain.H == 0 {
		return ir.Rect{}, false
	}
	x := frame.Target.X + (local.X-frame.Domain.X)*frame.Target.W/frame.Domain.W
	y := frame.Target.Y + (local.Y-frame.Domain.Y)*frame.Target.H/frame.Domain.H
	w := local.W * frame.Target.W / frame.Domain.W
	h := local.H * frame.Target.H / frame.Domain.H
	if w == 0 || h == 0 {
		return ir.Rect{}, false
	}
	if w < 0 {
		x += w
		w = -w
	}
	if h < 0 {
		y += h
		h = -h
	}
	return ir.Rect{X: x, Y: y, W: w, H: h}, true
}

// ComposeGroupChildren maps child rectangles from an OfficeArt group coordinate
// domain into the group's rendered target bounds. When the frame is invalid the
// authored child rectangles are preserved unchanged.
func ComposeGroupChildren(children []*ir.Shape, frame GroupFrame) {
	if len(children) == 0 || !frame.Valid {
		return
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		if mapped, ok := MapRectThroughGroup(child.Transform.Rect, frame); ok {
			child.Transform.Rect = mapped
		}
		if len(child.Children) > 0 {
			// Nested groups are already represented in the parent's rendered
			// coordinate space after this mapping. Descendants keep their own
			// local authored coordinates until their own frame is applied.
			continue
		}
	}
}
