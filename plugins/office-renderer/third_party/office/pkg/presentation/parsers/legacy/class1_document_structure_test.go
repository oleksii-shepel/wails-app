package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestClass1MasterBackgroundUsesResolvedPersistOwnership(t *testing.T) {
	theme := &presentationir.Theme{ColorScheme: map[string]string{"lt1": "EEEEEE"}}
	slides := []legacySlideText{{PersistID: 1, MasterID: 42, MasterPersistID: 7}}
	pres := buildPresentationWithMasterComposition(
		slides, nil, nil, legacyPptHeaderFooterInfo{}, "test.ppt", theme, nil, nil, "",
		map[int]string{42: "BADBAD", 7: "123456"},
	)
	if len(pres.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(pres.Slides))
	}
	if got := pres.Slides[0].BgColor; got != "123456" {
		t.Fatalf("BgColor = %q, want persist-owned 123456", got)
	}
}

func TestClass1UnresolvedMasterDoesNotBorrowBackgroundByIdentifier(t *testing.T) {
	theme := &presentationir.Theme{ColorScheme: map[string]string{"lt1": "EEEEEE"}}
	slides := []legacySlideText{{PersistID: 1, MasterID: 42}}
	pres := buildPresentationWithMasterComposition(
		slides, nil, nil, legacyPptHeaderFooterInfo{}, "test.ppt", theme, nil, nil, "",
		map[int]string{42: "BADBAD"},
	)
	if got := pres.Slides[0].BgColor; got != "EEEEEE" {
		t.Fatalf("BgColor = %q, want document/theme fallback EEEEEE", got)
	}
}

func TestProjectLegacySlidesInAuthoredOrderDropsUnownedParserProjections(t *testing.T) {
	slides := []legacySlideText{{PersistID: 30}, {PersistID: 10}, {PersistID: 20}}
	got := projectLegacySlidesInAuthoredOrder([]int{20, 10}, slides)
	if len(got) != 2 || got[0].PersistID != 20 || got[1].PersistID != 10 {
		t.Fatalf("projected slide order = %#v, want persist IDs [20 10] only", got)
	}
}

func TestProjectLegacySlidesInAuthoredOrderMaterializesEmptyAuthoredSlides(t *testing.T) {
	got := projectLegacySlidesInAuthoredOrder([]int{10, 20}, []legacySlideText{{PersistID: 10}})
	if len(got) != 2 || got[0].PersistID != 10 || got[1].PersistID != 20 {
		t.Fatalf("projected slide order = %#v, want persist IDs [10 20]", got)
	}
}

func TestProjectLegacySlidesInAuthoredOrderDeduplicatesMalformedMembership(t *testing.T) {
	got := projectLegacySlidesInAuthoredOrder([]int{10, 10, 20}, []legacySlideText{{PersistID: 10}, {PersistID: 20}})
	if len(got) != 2 || got[0].PersistID != 10 || got[1].PersistID != 20 {
		t.Fatalf("projected slide order = %#v, want persist IDs [10 20]", got)
	}
}

func TestClass1MasterVisualCompositionHonorsFollowMasterObjects(t *testing.T) {
	master := &presentationir.Shape{Kind: presentationir.ShapeOther, OriginalShapeID: 99}
	local := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 7}
	slides := []legacySlideText{{PersistID: 10, MasterPersistID: 20, HasSlideFlags: true, FollowMasterObjects: false}}
	got := composeLegacyPptMasterVisuals(slides, map[int][]*presentationir.Shape{10: {local}}, map[int][]*presentationir.Shape{20: {master}})
	if len(got[10]) != 1 || got[10][0].OriginalShapeID != 7 {
		t.Fatalf("shapes = %#v, want only slide-local shape when fMasterObjects is clear", got[10])
	}
}
