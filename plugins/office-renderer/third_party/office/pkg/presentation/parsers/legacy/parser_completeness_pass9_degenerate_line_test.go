package legacy

import (
	"encoding/binary"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
	presentationir "office/pkg/presentation/ir"
)

func pass9ClientAnchor(top, left, right, bottom int16) []byte {
	body := make([]byte, 8)
	binary.LittleEndian.PutUint16(body[0:2], uint16(top))
	binary.LittleEndian.PutUint16(body[2:4], uint16(left))
	binary.LittleEndian.PutUint16(body[4:6], uint16(right))
	binary.LittleEndian.PutUint16(body[6:8], uint16(bottom))
	return body
}

func TestPass9PlacementPreservesHorizontalConnectorClientAnchor(t *testing.T) {
	node := &officeartescher.ShapeNode{
		ShapeType:    20,
		ClientAnchor: pass9ClientAnchor(20, 10, 310, 20),
	}
	got := resolveLegacyPptShapePlacement(node, legacyPptGroupFrame{})
	if !got.valid() || got.Source != legacyPptPlacementClientAnchor {
		t.Fatalf("placement = %#v, want horizontal connector client anchor", got)
	}
	if got.Bounds.W <= 0 || got.Bounds.H != 0 {
		t.Fatalf("bounds = %#v, want positive width and zero height", got.Bounds)
	}
}

func TestPass9PlacementPreservesVerticalConnectorChildAnchorThroughGroup(t *testing.T) {
	node := &officeartescher.ShapeNode{
		ShapeType:   20,
		Flags:       legacyOfficeArtSpFlagChild,
		ChildAnchor: pass5ChildAnchor(250, 100, 250, 800),
	}
	frame := legacyPptGroupFrame{
		domain: presentationir.Rect{X: 0, Y: 0, W: 1000, H: 1000},
		target: presentationir.Rect{X: 100, Y: 200, W: 500, H: 400},
		valid:  true,
	}
	got := resolveLegacyPptShapePlacement(node, frame)
	if !got.valid() || got.Source != legacyPptPlacementMappedChildAnchor {
		t.Fatalf("placement = %#v, want mapped vertical connector child anchor", got)
	}
	if got.Bounds.W != 0 || got.Bounds.H <= 0 {
		t.Fatalf("bounds = %#v, want zero width and positive height", got.Bounds)
	}
}

func TestPass9PlacementPreservesDegenerateOrdinaryShape(t *testing.T) {
	node := &officeartescher.ShapeNode{
		ShapeType:    1,
		ClientAnchor: pass9ClientAnchor(20, 10, 310, 20),
	}
	got := resolveLegacyPptShapePlacement(node, legacyPptGroupFrame{})
	if !got.valid() || got.Bounds.W <= 0 || got.Bounds.H != 0 {
		t.Fatalf("placement = %#v, want authored zero-height ordinary shape", got)
	}
}

func TestPass9PlacementPreservesPointConnectorAnchor(t *testing.T) {
	node := &officeartescher.ShapeNode{
		ShapeType:    32,
		ClientAnchor: pass9ClientAnchor(20, 10, 10, 20),
	}
	got := resolveLegacyPptShapePlacement(node, legacyPptGroupFrame{})
	if !got.valid() || got.Bounds.W != 0 || got.Bounds.H != 0 {
		t.Fatalf("placement = %#v, point connector anchor must remain authored geometry", got)
	}
}

func TestPass9PlacementPreservesZeroWidthTextboxChildAnchor(t *testing.T) {
	node := &officeartescher.ShapeNode{
		ShapeType:        1,
		Flags:            legacyOfficeArtSpFlagChild,
		HasClientTextbox: true,
		ChildAnchor:      pass5ChildAnchor(500, 100, 500, 700),
	}
	frame := legacyPptGroupFrame{
		domain: presentationir.Rect{X: 0, Y: 0, W: 1000, H: 1000},
		target: presentationir.Rect{X: 100, Y: 200, W: 500, H: 400},
		valid:  true,
	}
	got := resolveLegacyPptShapePlacement(node, frame)
	if !got.valid() || got.Bounds.W != 0 || got.Bounds.H <= 0 {
		t.Fatalf("placement = %#v, want mapped zero-width textbox geometry", got)
	}
}

func TestPass9GenericProjectionOwnsZeroAxisStrokedNonPrimitive(t *testing.T) {
	node := &officeartescher.ShapeNode{
		ShapeID:      901,
		ShapeType:    0,
		ClientAnchor: pass9ClientAnchor(20, 10, 310, 20),
		Properties: []officeartescher.Property{
			{ID: officeartescher.PropLineColor, Value: 0x000000FF},
		},
	}
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{{ShapeID: node.ShapeID, Node: node}}
	projection := scene.completeProjection(legacySlideText{}, nil, nil, nil, nil, nil, nil, legacyPptDefaultCanvas())
	if len(projection.Shapes) != 1 {
		t.Fatalf("projected shapes = %d, want 1", len(projection.Shapes))
	}
	if projection.Shapes[0].Kind != presentationir.ShapeConnector {
		t.Fatalf("kind = %v, want connector fallback for zero-axis stroked shape", projection.Shapes[0].Kind)
	}
	if projection.Shapes[0].Bounds.H != 0 || projection.Shapes[0].Bounds.W <= 0 {
		t.Fatalf("bounds = %#v", projection.Shapes[0].Bounds)
	}
	if gaps := projection.unclaimed(); len(gaps) != 0 {
		t.Fatalf("ownership gaps = %+v", gaps)
	}
}

func TestPass9GenericProjectionPreservesNonPaintingDegenerateShape(t *testing.T) {
	node := &officeartescher.ShapeNode{
		ShapeID:      902,
		ShapeType:    1,
		ClientAnchor: pass9ClientAnchor(20, 10, 310, 20),
	}
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{{ShapeID: node.ShapeID, Node: node}}
	projection := scene.completeProjection(legacySlideText{}, nil, nil, nil, nil, nil, nil, legacyPptDefaultCanvas())
	if len(projection.Shapes) != 1 || projection.Shapes[0].OriginalShapeID != node.ShapeID {
		t.Fatalf("projection = %+v, want degenerate authored shape preserved", projection.Shapes)
	}
	if gaps := projection.unclaimed(); len(gaps) != 0 {
		t.Fatalf("ownership gaps = %+v", gaps)
	}
}
