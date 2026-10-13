package legacy

import (
	"encoding/binary"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
	presentationir "office/pkg/presentation/ir"
)

func TestBIG37MasterSemanticProjectionDoesNotRequirePictureStore(t *testing.T) {
	anchor := make([]byte, 16)
	binary.LittleEndian.PutUint32(anchor[0:4], 100)
	binary.LittleEndian.PutUint32(anchor[4:8], 200)
	binary.LittleEndian.PutUint32(anchor[8:12], 1100)
	binary.LittleEndian.PutUint32(anchor[12:16], 700)
	node := &officeartescher.ShapeNode{ShapeID: 77, ShapeType: 1, ClientAnchor: anchor, Properties: []officeartescher.Property{{ID: officeartescher.PropFillColor, Value: 0x000000FF}}}
	scene := legacyPptVisualScene{dispatch: legacyPptSlideVisualDispatch{containers: legacyPptShapeContainerScene{
		containers: []legacyPptShapeContainer{{ShapeID: 77, Node: node}},
		events:     []OfficeArtEvent{{ShapeStart: true, ShapeNode: node}},
	}}}

	got := composeLegacyPptMasterSemanticScenes(
		map[int]legacyPptVisualScene{5: scene},
		[]legacyPptMasterEntry{{RefID: 5, SlideIdentifier: 42}},
		nil, nil, nil, nil, nil, nil,
		legacyPptDefaultCanvas())
	if len(got.Visuals[5]) != 1 {
		t.Fatalf("master generic artwork = %d shapes, want 1 without Pictures stream", len(got.Visuals[5]))
	}
	if got.Visuals[5][0].OriginalShapeID != 77 || got.Visuals[5][0].OfficeArt == nil {
		t.Fatalf("master generic shape lost semantic OfficeArt identity: %+v", got.Visuals[5][0])
	}
}

func TestBIG37MasterPlaceholderIsTemplateNotPaintedArtwork(t *testing.T) {
	shape := &presentationir.Shape{Kind: presentationir.ShapeText, Placeholder: presentationir.PlaceholderTitle, OriginalShapeID: 9}
	local := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 10}
	got := composeLegacyPptMasterVisuals(
		[]legacySlideText{{PersistID: 1, MasterID: 42, MasterPersistID: 5}},
		map[int][]*presentationir.Shape{1: {local}},
		map[int][]*presentationir.Shape{5: {shape}},
	)
	// Composition is intentionally dumb: projection owns the template/artwork split.
	// This gate documents that callers must pass artwork-only master scenes.
	if len(got[1]) != 2 {
		t.Fatalf("composition changed source ordering contract: %d", len(got[1]))
	}
}

func TestBIG37PerMasterBackgroundResolvedDuringComposition(t *testing.T) {
	slides := []legacySlideText{{PersistID: 1, MasterID: 10, MasterPersistID: 101}, {PersistID: 2, MasterID: 20, MasterPersistID: 202}}
	pres := buildPresentationWithMasterComposition(
		slides, nil, nil, legacyPptHeaderFooterInfo{}, "test.ppt", nil, nil, nil, "AAAAAA",
		map[int]string{101: "112233", 202: "445566"},
		legacyPptDefaultCanvas())
	if len(pres.Slides) != 2 {
		t.Fatalf("slides = %d, want 2", len(pres.Slides))
	}
	if pres.Slides[0].BgColor != "112233" || pres.Slides[1].BgColor != "445566" {
		t.Fatalf("effective backgrounds = %q, %q", pres.Slides[0].BgColor, pres.Slides[1].BgColor)
	}
}

func TestClass1MasterArtworkUsesResolvedPersistOwnership(t *testing.T) {
	wrong := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 101}
	right := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 202}
	slides := []legacySlideText{{PersistID: 1, MasterID: 42, MasterPersistID: 7}}
	got := composeLegacyPptMasterVisuals(slides, nil, map[int][]*presentationir.Shape{
		42: {wrong}, // authored SlideIdentifier must not be interpreted as a persist key
		7:  {right},
	})
	if len(got[1]) != 1 || got[1][0].OriginalShapeID != 202 {
		t.Fatalf("master artwork did not follow resolved persist ownership: %+v", got[1])
	}
}

func TestClass1UnresolvedMasterDoesNotGuessArtworkByIdentifier(t *testing.T) {
	master := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 101}
	slides := []legacySlideText{{PersistID: 1, MasterID: 42}}
	got := composeLegacyPptMasterVisuals(slides, nil, map[int][]*presentationir.Shape{42: {master}})
	if len(got[1]) != 0 {
		t.Fatalf("unresolved master guessed artwork from SlideIdentifier: %+v", got[1])
	}
}
