package legacy

import (
	"encoding/binary"
	officeartescher "office/pkg/officeart/legacy/escher"
	presentationir "office/pkg/presentation/ir"
)

type legacyPptPlacementSource uint8

const (
	legacyPptPlacementNone legacyPptPlacementSource = iota
	legacyPptPlacementClientAnchor
	legacyPptPlacementMappedChildAnchor
	legacyPptPlacementStandaloneChildAnchor
)

type legacyPptAnchorEdges struct {
	Left, Top, Right, Bottom int64
	ClientUnits              bool
}

func (e legacyPptAnchorEdges) width() int64  { return e.Right - e.Left }
func (e legacyPptAnchorEdges) height() int64 { return e.Bottom - e.Top }

type legacyPptPlacement struct {
	Bounds presentationir.Rect
	Source legacyPptPlacementSource
	Reason string
	// Anchor preserves the authored edge representation until the family parser
	// has consumed placement. Bounds alone cannot distinguish Y1/Y2 semantics
	// from a prematurely interpreted (Y,H) pair, which is especially important
	// for legacy OLE/MathType objects whose cached preview is independently sized.
	Anchor legacyPptAnchorEdges
}

func (p legacyPptPlacement) valid() bool { return p.Source != legacyPptPlacementNone }

// resolveLegacyPptShapePlacement is the single placement policy for authored
// OfficeArt shapes. Placement preserves syntactically valid authored geometry,
// including zero-width, zero-height and point anchors. Degenerate geometry is a
// semantic property of the authored shape, not a malformed binary anchor; the
// consuming family decides whether it paints, contributes topology, or remains
// intentionally invisible. Child shapes prefer their group-local ChildAnchor
// mapped through the active group frame; ClientAnchor is the authoritative
// slide-space fallback. A raw ChildAnchor may be used directly only for
// non-child shapes.
func resolveLegacyPptShapePlacement(node *officeartescher.ShapeNode, frame legacyPptGroupFrame) legacyPptPlacement {
	if node == nil {
		return legacyPptPlacement{Reason: "missing decoded ShapeNode"}
	}
	if node.Flags&legacyOfficeArtSpFlagChild != 0 && len(node.ChildAnchor) != 0 {
		if local, edges, ok := parseLegacyPptChildPlacementAnchorWithEdges(node.ChildAnchor, true); ok {
			if mapped, ok := mapLegacyPptPlacementRectThroughGroupFrame(local, frame, true); ok {
				return legacyPptPlacement{Bounds: mapped, Source: legacyPptPlacementMappedChildAnchor, Reason: "mapped child anchor", Anchor: edges}
			}
		}
	}
	if len(node.ClientAnchor) != 0 {
		if bounds, edges, ok := parseLegacyPptClientPlacementAnchorWithEdges(node.ClientAnchor, true); ok {
			return legacyPptPlacement{Bounds: bounds, Source: legacyPptPlacementClientAnchor, Reason: "client anchor", Anchor: edges}
		}
	}
	if node.Flags&legacyOfficeArtSpFlagChild == 0 && len(node.ChildAnchor) != 0 {
		if bounds, edges, ok := parseLegacyPptChildPlacementAnchorWithEdges(node.ChildAnchor, true); ok {
			return legacyPptPlacement{Bounds: bounds, Source: legacyPptPlacementStandaloneChildAnchor, Reason: "standalone child anchor", Anchor: edges}
		}
	}
	if node.Flags&legacyOfficeArtSpFlagChild != 0 && len(node.ChildAnchor) != 0 && !frame.valid {
		return legacyPptPlacement{Reason: "child anchor has no resolved group frame"}
	}
	if len(node.ClientAnchor) == 0 && len(node.ChildAnchor) == 0 {
		return legacyPptPlacement{Reason: "no authored anchor"}
	}
	return legacyPptPlacement{Reason: "authored anchor is malformed or unusable"}
}

func (s legacyPptVisualScene) frameForContainer(container legacyPptShapeContainer) legacyPptGroupFrame {
	frame := legacyPptGroupFrame{}
	bestDepth := -1
	for _, group := range s.dispatch.containers.shapeGroups() {
		if legacyPptGroupPathHasPrefix(container.GroupPath, group.Path) && len(group.Path) > bestDepth {
			frame = group.Frame
			bestDepth = len(group.Path)
		}
	}
	return frame
}

// parseLegacyPptClientPlacementAnchor is placement-oriented rather than
// paint-oriented. When allowDegenerate is true, a zero axis (or even a point)
// is retained so the authored shape can still reach canonical IR.
func parseLegacyPptClientPlacementAnchor(body []byte, allowDegenerate bool) (presentationir.Rect, bool) {
	bounds, _, ok := parseLegacyPptClientPlacementAnchorWithEdges(body, allowDegenerate)
	return bounds, ok
}

func parseLegacyPptClientPlacementAnchorWithEdges(body []byte, allowDegenerate bool) (presentationir.Rect, legacyPptAnchorEdges, bool) {
	edges, ok := parseLegacyPptClientAnchorEdges(body)
	if !ok {
		return presentationir.Rect{}, legacyPptAnchorEdges{}, false
	}
	bounds, ok := legacyPptPlacementRect(edges.Left, edges.Top, edges.Right, edges.Bottom, true, allowDegenerate)
	return bounds, edges, ok
}

func parseLegacyPptClientAnchorEdges(body []byte) (legacyPptAnchorEdges, bool) {
	var top, left, right, bottom int64
	switch {
	case len(body) >= 16:
		top = int64(int32(binary.LittleEndian.Uint32(body[0:4])))
		left = int64(int32(binary.LittleEndian.Uint32(body[4:8])))
		right = int64(int32(binary.LittleEndian.Uint32(body[8:12])))
		bottom = int64(int32(binary.LittleEndian.Uint32(body[12:16])))
	case len(body) >= 8:
		top = int64(int16(binary.LittleEndian.Uint16(body[0:2])))
		left = int64(int16(binary.LittleEndian.Uint16(body[2:4])))
		right = int64(int16(binary.LittleEndian.Uint16(body[4:6])))
		bottom = int64(int16(binary.LittleEndian.Uint16(body[6:8])))
	default:
		return legacyPptAnchorEdges{}, false
	}
	return legacyPptAnchorEdges{Left: left, Top: top, Right: right, Bottom: bottom, ClientUnits: true}, true
}

func parseLegacyPptChildPlacementAnchor(body []byte, allowDegenerate bool) (presentationir.Rect, bool) {
	bounds, _, ok := parseLegacyPptChildPlacementAnchorWithEdges(body, allowDegenerate)
	return bounds, ok
}

func parseLegacyPptChildPlacementAnchorWithEdges(body []byte, allowDegenerate bool) (presentationir.Rect, legacyPptAnchorEdges, bool) {
	if len(body) < 16 {
		return presentationir.Rect{}, legacyPptAnchorEdges{}, false
	}
	edges := legacyPptAnchorEdges{
		Left:   int64(int32(binary.LittleEndian.Uint32(body[0:4]))),
		Top:    int64(int32(binary.LittleEndian.Uint32(body[4:8]))),
		Right:  int64(int32(binary.LittleEndian.Uint32(body[8:12]))),
		Bottom: int64(int32(binary.LittleEndian.Uint32(body[12:16]))),
	}
	bounds, ok := legacyPptPlacementRect(edges.Left, edges.Top, edges.Right, edges.Bottom, false, allowDegenerate)
	return bounds, edges, ok
}

func legacyPptPlacementRect(left, top, right, bottom int64, clientUnits, allowDegenerate bool) (presentationir.Rect, bool) {
	if right < left {
		left, right = right, left
	}
	if bottom < top {
		top, bottom = bottom, top
	}
	w, h := right-left, bottom-top
	if !allowDegenerate && (w == 0 || h == 0) {
		return presentationir.Rect{}, false
	}
	if clientUnits {
		return presentationir.Rect{
			X: left * defaultSlideWidthEMU / legacyPptMasterWidth,
			Y: top * defaultSlideHeightEMU / legacyPptMasterHeight,
			W: w * defaultSlideWidthEMU / legacyPptMasterWidth,
			H: h * defaultSlideHeightEMU / legacyPptMasterHeight,
		}, true
	}
	return presentationir.Rect{X: left, Y: top, W: w, H: h}, true
}

func mapLegacyPptPlacementRectThroughGroupFrame(local presentationir.Rect, frame legacyPptGroupFrame, allowDegenerate bool) (presentationir.Rect, bool) {
	if !allowDegenerate && (local.W == 0 || local.H == 0) {
		return presentationir.Rect{}, false
	}
	if local.W != 0 && local.H != 0 {
		return mapLegacyPptRectThroughGroupFrame(local, frame)
	}
	if !frame.valid || frame.domain.W == 0 || frame.domain.H == 0 {
		return presentationir.Rect{}, false
	}
	x1 := frame.target.X + (local.X-frame.domain.X)*frame.target.W/frame.domain.W
	y1 := frame.target.Y + (local.Y-frame.domain.Y)*frame.target.H/frame.domain.H
	x2 := frame.target.X + (local.X+local.W-frame.domain.X)*frame.target.W/frame.domain.W
	y2 := frame.target.Y + (local.Y+local.H-frame.domain.Y)*frame.target.H/frame.domain.H
	if x2 < x1 {
		x1, x2 = x2, x1
	}
	if y2 < y1 {
		y1, y2 = y2, y1
	}
	return presentationir.Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}, true
}
