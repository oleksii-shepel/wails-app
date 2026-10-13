package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestLegacyBuilderPublishesCanonicalMasterLayoutGraph(t *testing.T) {
	masterShape := &presentationir.Shape{ID: 7, Name: "master-art", Kind: presentationir.ShapeText}
	slides := []legacySlideText{{PersistID: 11, MasterID: 3, MasterPersistID: 41, LayoutType: "titleBody"}}
	pres := buildPresentationWithEffectiveMasterComposition(
		slides,
		map[int][]*presentationir.Shape{11: {{ID: 9, Name: "local", Kind: presentationir.ShapeText}}},
		nil, nil, legacyPptHeaderFooterInfo{}, "fixture.ppt", nil, nil, legacyPptMasterTextStyles{}, "", nil, nil,
		map[int][]*presentationir.Shape{41: {masterShape}}, nil,
		legacyPptDefaultCanvas())
	if len(pres.Slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(pres.Slides))
	}
	slide := pres.Slides[0]
	if slide.Master == nil || slide.Layout == nil {
		t.Fatalf("canonical template graph missing: master=%v layout=%v", slide.Master, slide.Layout)
	}
	if slide.Layout.Type != "titleBody" || slide.Layout.Master != slide.Master {
		t.Fatalf("layout = %#v, want titleBody owned by slide master", slide.Layout)
	}
	if len(pres.SlideMasters) != 1 || pres.SlideMasters[0] != slide.Master {
		t.Fatalf("presentation master catalog is not the slide's canonical master")
	}
	if len(slide.Master.Shapes) != 1 || slide.Master.Shapes[0].Name != "master-art" {
		t.Fatalf("master template shapes = %#v", slide.Master.Shapes)
	}
	if slide.Master.Shapes[0] == masterShape {
		t.Fatal("canonical master retained mutable parser-owned shape")
	}
	if len(slide.Shapes) < 2 || slide.Shapes[0] == slide.Master.Shapes[0] {
		t.Fatal("effective slide paint must be independent from master template graph")
	}
}

func TestLegacyBuilderReusesCanonicalMasterAndLogicalLayout(t *testing.T) {
	slides := []legacySlideText{
		{PersistID: 11, MasterPersistID: 41, LayoutType: "titleBody"},
		{PersistID: 12, MasterPersistID: 41, LayoutType: "titleBody"},
		{PersistID: 13, MasterPersistID: 41, LayoutType: "blank"},
	}
	extra := map[int][]*presentationir.Shape{
		11: {{ID: 1, Kind: presentationir.ShapeText}},
		12: {{ID: 2, Kind: presentationir.ShapeText}},
		13: {{ID: 3, Kind: presentationir.ShapeText}},
	}
	pres := buildPresentationWithEffectiveMasterComposition(slides, extra, nil, nil, legacyPptHeaderFooterInfo{}, "fixture.ppt", nil, nil, legacyPptMasterTextStyles{}, "", nil, nil, nil, nil, legacyPptDefaultCanvas())
	if len(pres.Slides) != 3 || len(pres.SlideMasters) != 1 {
		t.Fatalf("slides/masters = %d/%d, want 3/1", len(pres.Slides), len(pres.SlideMasters))
	}
	if pres.Slides[0].Master != pres.Slides[1].Master || pres.Slides[1].Master != pres.Slides[2].Master {
		t.Fatal("slides referencing one legacy master must share one canonical master template")
	}
	if pres.Slides[0].Layout != pres.Slides[1].Layout {
		t.Fatal("same legacy layout atom must share one canonical logical layout")
	}
	if pres.Slides[0].Layout == pres.Slides[2].Layout || len(pres.SlideMasters[0].Layouts) != 2 {
		t.Fatal("distinct legacy layout atoms must remain distinct canonical layouts")
	}
}
