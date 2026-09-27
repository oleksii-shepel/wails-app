package legacy

import (
	"encoding/binary"
	"strings"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
	presentationir "office/pkg/presentation/ir"
)

func pass5ChildAnchor(l, t, r, b int32) []byte {
	body := make([]byte, 16)
	binary.LittleEndian.PutUint32(body[0:4], uint32(l))
	binary.LittleEndian.PutUint32(body[4:8], uint32(t))
	binary.LittleEndian.PutUint32(body[8:12], uint32(r))
	binary.LittleEndian.PutUint32(body[12:16], uint32(b))
	return body
}

func pass5ClientAnchor(x, y, w, h uint16) []byte {
	body := make([]byte, 8)
	binary.LittleEndian.PutUint16(body[0:2], x)
	binary.LittleEndian.PutUint16(body[2:4], y)
	binary.LittleEndian.PutUint16(body[4:6], x+w)
	binary.LittleEndian.PutUint16(body[6:8], y+h)
	return body
}

func TestParserCompletenessPlacementMapsChildAnchorThroughGroup(t *testing.T) {
	node := &officeartescher.ShapeNode{
		Flags:       legacyOfficeArtSpFlagChild,
		ChildAnchor: pass5ChildAnchor(100, 200, 700, 600),
	}
	frame := legacyPptGroupFrame{
		domain: presentationir.Rect{X: 0, Y: 0, W: 1000, H: 1000},
		target: presentationir.Rect{X: 100, Y: 200, W: 500, H: 400},
		valid:  true,
	}
	got := resolveLegacyPptShapePlacement(node, frame)
	if !got.valid() || got.Source != legacyPptPlacementMappedChildAnchor {
		t.Fatalf("placement = %#v, want mapped child anchor", got)
	}
	want, ok := mapLegacyPptRectThroughGroupFrame(presentationir.Rect{X: 100, Y: 200, W: 600, H: 400}, frame)
	if !ok || got.Bounds != want {
		t.Fatalf("bounds = %#v, want %#v", got.Bounds, want)
	}
}

func TestParserCompletenessPlacementFallsBackToClientAnchorWhenChildFrameMissing(t *testing.T) {
	node := &officeartescher.ShapeNode{
		Flags:        legacyOfficeArtSpFlagChild,
		ChildAnchor:  pass5ChildAnchor(100, 200, 700, 600),
		ClientAnchor: pass5ClientAnchor(10, 20, 300, 200),
	}
	got := resolveLegacyPptShapePlacement(node, legacyPptGroupFrame{})
	if !got.valid() || got.Source != legacyPptPlacementClientAnchor {
		t.Fatalf("placement = %#v, want client-anchor fallback", got)
	}
}

func TestParserCompletenessOwnershipExplainsMissingPlacement(t *testing.T) {
	node := &officeartescher.ShapeNode{ShapeID: 501}
	scene := legacyPptVisualScene{dispatch: legacyPptSlideVisualDispatch{containers: legacyPptShapeContainerScene{
		containers: []legacyPptShapeContainer{{ShapeID: 501, Node: node}},
	}}}
	audit := scene.ownershipAudit(nil)
	if len(audit) != 1 || audit[0].Claimed || !strings.Contains(audit[0].Reason, "placement: no authored anchor") {
		t.Fatalf("ownership = %#v, want explicit missing-placement gap", audit)
	}
}
