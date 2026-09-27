package legacy

import "testing"

func TestStep1TextOwnershipSuppressesOnlySuccessfullyOwnedTablePaths(t *testing.T) {
	scene := legacyPptVisualScene{dispatch: legacyPptSlideVisualDispatch{containers: legacyPptShapeContainerScene{containers: []legacyPptShapeContainer{
		{ShapeID: 10, GroupPath: []int{1, 2}},
		{ShapeID: 11, GroupPath: []int{1, 2, 3}},
		{ShapeID: 20, GroupPath: []int{1, 4}},
	}}}}
	metas := []legacyPptTextShape{{shapeID: 10}, {shapeID: 11}, {shapeID: 20}}
	got := scene.filterTextMetasOwnedByGroupPaths(metas, [][]int{{1, 2}})
	if len(got) != 1 || got[0].shapeID != 20 {
		t.Fatalf("filtered text metas = %#v, want only shape 20 outside the materialized table path", got)
	}
}

func TestStep1TextOwnershipPreservesAllTextWhenNoTableMaterialized(t *testing.T) {
	scene := legacyPptVisualScene{dispatch: legacyPptSlideVisualDispatch{containers: legacyPptShapeContainerScene{containers: []legacyPptShapeContainer{
		{ShapeID: 10, GroupPath: []int{1, 2}},
		{ShapeID: 20, GroupPath: []int{1, 4}},
	}}}}
	metas := []legacyPptTextShape{{shapeID: 10}, {shapeID: 20}}
	got := scene.filterTextMetasOwnedByGroupPaths(metas, nil)
	if len(got) != 2 {
		t.Fatalf("filtered text metas = %#v, want all text when no table path materialized", got)
	}
}
