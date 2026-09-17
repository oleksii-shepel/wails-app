package edit

import (
	pasir "office/pkg/presentation/ir"
	"testing"
)

func TestTransferSAXShapeOwnershipUsesSAXAuthoredFactsAndKeepsResolvedPayload(t *testing.T) {
	sax := &pasir.Shape{ID: 7, Name: "sax", Kind: pasir.ShapeImage, Bounds: pasir.Rect{X: 10, Y: 20, W: 30, H: 40}, ImageRelID: "rId7", ImageCropLeft: .1, FillColor: "112233", HyperlinkURL: "https://sax.example.test"}
	old := &pasir.Shape{ID: 7, Name: "compose", Kind: pasir.ShapeImage, Bounds: pasir.Rect{X: 1, Y: 2, W: 3, H: 4}, ImageRelID: "old", ImageData: []byte{1, 2, 3}, ImageMIME: "image/png", HyperlinkURL: "https://example.test"}
	resolved := &pasir.Presentation{Slides: []*pasir.Slide{{Index: 0, Shapes: []*pasir.Shape{old}}}}
	if err := transferSAXShapeOwnership([]*pasir.Slide{{Index: 0, Shapes: []*pasir.Shape{sax}}}, resolved); err != nil {
		t.Fatal(err)
	}
	got := resolved.Slides[0].Shapes[0]
	if got != sax {
		t.Fatal("production tree retained composer core node; want SAX node identity")
	}
	if got.Name != "sax" || got.Bounds.X != 10 || got.ImageRelID != "rId7" || got.ImageCropLeft != .1 {
		t.Fatalf("SAX authored facts not transferred: %+v", got)
	}
	if string(got.ImageData) != string([]byte{1, 2, 3}) || got.ImageMIME != "image/png" || got.HyperlinkURL != "https://sax.example.test" {
		t.Fatalf("resolved payload was lost: %+v", got)
	}
}

func TestTransferSAXShapeOwnershipRecursesThroughGroups(t *testing.T) {
	saxChild := &pasir.Shape{ID: 12, Kind: pasir.ShapeConnector, Geometry: "bentConnector3", LineColor: "AABBCC"}
	saxGroup := &pasir.Shape{ID: 10, Kind: pasir.ShapeGroup, Children: []*pasir.Shape{saxChild}}
	oldChild := &pasir.Shape{ID: 12, Kind: pasir.ShapeConnector, Geometry: "line", LineColor: "000000", ArrowHead: "triangle"}
	oldGroup := &pasir.Shape{ID: 10, Kind: pasir.ShapeGroup, Children: []*pasir.Shape{oldChild}}
	resolved := &pasir.Presentation{Slides: []*pasir.Slide{{Shapes: []*pasir.Shape{oldGroup}}}}
	if err := transferSAXShapeOwnership([]*pasir.Slide{{Shapes: []*pasir.Shape{saxGroup}}}, resolved); err != nil {
		t.Fatal(err)
	}
	got := resolved.Slides[0].Shapes[0].Children[0]
	if got.Geometry != "bentConnector3" || got.ConnectorType != "bentConnector3" || got.LineColor != "AABBCC" {
		t.Fatalf("connector did not move to SAX ownership: %+v", got)
	}
	if got.ArrowHead != "triangle" {
		t.Fatal("unmigrated connector decoration payload was lost")
	}
}

func TestTransferSAXShapeOwnershipLeavesPromotedFeatureFamiliesToComposer(t *testing.T) {
	sax := &pasir.Shape{ID: 99, Kind: pasir.ShapeImage}
	resolved := &pasir.Presentation{Slides: []*pasir.Slide{{Shapes: []*pasir.Shape{{ID: 99, Kind: pasir.ShapeMedia, MediaKind: "video"}}}}}
	if err := transferSAXShapeOwnership([]*pasir.Slide{{Shapes: []*pasir.Shape{sax}}}, resolved); err != nil {
		t.Fatal(err)
	}
	if resolved.Slides[0].Shapes[0].Kind != pasir.ShapeMedia {
		t.Fatal("composer-owned promoted media shape changed")
	}
}
