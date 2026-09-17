package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestBIG39SemanticSceneRestoresAuthoredShapeOrderAcrossFamilies(t *testing.T) {
	scene := legacyPptVisualScene{dispatch: legacyPptSlideVisualDispatch{containers: legacyPptShapeContainerScene{
		containers: []legacyPptShapeContainer{{ShapeID: 30}, {ShapeID: 10}, {ShapeID: 20}},
	}}}
	synthesized := &presentationir.Shape{Kind: presentationir.ShapeText, Name: "synthesized"}
	shapes := []*presentationir.Shape{
		{Kind: presentationir.ShapeConnector, OriginalShapeID: 20},
		synthesized,
		{Kind: presentationir.ShapeText, OriginalShapeID: 10},
		{Kind: presentationir.ShapeImage, OriginalShapeID: 30},
	}
	got := scene.sourceOrdered(shapes)
	if len(got) != 4 {
		t.Fatalf("shape count = %d, want 4", len(got))
	}
	want := []uint32{30, 10, 20, 0}
	for i, id := range want {
		if got[i].OriginalShapeID != id {
			t.Fatalf("shape %d id = %d, want %d", i, got[i].OriginalShapeID, id)
		}
	}
	if got[3] != synthesized {
		t.Fatal("synthesized shape did not retain stable trailing ownership")
	}
}

func TestBIG39SemanticSceneOrdersGroupByItsAuthoredRootShape(t *testing.T) {
	scene := legacyPptVisualScene{dispatch: legacyPptSlideVisualDispatch{containers: legacyPptShapeContainerScene{
		containers: []legacyPptShapeContainer{{ShapeID: 7, IsGroupRoot: true}, {ShapeID: 8, GroupPath: []int{1}}, {ShapeID: 9}},
	}}}
	group := &presentationir.Shape{Kind: presentationir.ShapeGroup, OriginalShapeID: 7}
	flat := &presentationir.Shape{Kind: presentationir.ShapeText, OriginalShapeID: 9}
	got := scene.sourceOrdered([]*presentationir.Shape{flat, group})
	if got[0] != group || got[1] != flat {
		t.Fatalf("authored group order not preserved: %#v", got)
	}
}
