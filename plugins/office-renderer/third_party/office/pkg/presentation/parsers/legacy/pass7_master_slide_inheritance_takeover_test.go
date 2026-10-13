package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestPass7EffectiveCompositionOwnsMasterArtworkOrdering(t *testing.T) {
	master := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 100}
	local := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 200}
	slides := []legacySlideText{{PersistID: 7, MasterID: 42, MasterPersistID: 142}}

	pres := buildPresentationWithEffectiveMasterComposition(
		slides,
		map[int][]*presentationir.Shape{7: {local}}, nil,
		nil, legacyPptHeaderFooterInfo{}, "test.ppt", nil, nil, nil, "", nil, nil,
		map[int][]*presentationir.Shape{142: {master}}, nil,
		legacyPptDefaultCanvas())
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 2 {
		t.Fatalf("effective slide shapes = %+v, want master + local", pres.Slides)
	}
	if got := pres.Slides[0].Shapes[0].OriginalShapeID; got != 100 {
		t.Fatalf("first shape id = %d, want inherited master 100", got)
	}
	if got := pres.Slides[0].Shapes[1].OriginalShapeID; got != 200 {
		t.Fatalf("second shape id = %d, want local 200", got)
	}
	if pres.Slides[0].Shapes[0] == master {
		t.Fatal("effective composition reused mutable master template instance")
	}
}

func TestPass7EffectiveCompositionHonorsMasterObjectSwitchIndependently(t *testing.T) {
	master := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 100}
	local := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 200}
	slides := []legacySlideText{{
		PersistID: 7, MasterID: 42, MasterPersistID: 142,
		HasSlideFlags: true, FollowMasterObjects: false, FollowMasterBackground: true,
	}}

	pres := buildPresentationWithEffectiveMasterComposition(
		slides,
		map[int][]*presentationir.Shape{7: {local}}, nil,
		nil, legacyPptHeaderFooterInfo{}, "test.ppt", nil, nil, nil, "AAAAAA",
		map[int]string{142: "112233"}, nil,
		map[int][]*presentationir.Shape{142: {master}}, nil,
		legacyPptDefaultCanvas())
	if len(pres.Slides) != 1 || len(pres.Slides[0].Shapes) != 1 || pres.Slides[0].Shapes[0].OriginalShapeID != 200 {
		t.Fatalf("fMasterObjects=false did not suppress only master artwork: %+v", pres.Slides)
	}
	if got := pres.Slides[0].BgColor; got != "112233" {
		t.Fatalf("master background inheritance coupled to object switch: got %q", got)
	}
}
