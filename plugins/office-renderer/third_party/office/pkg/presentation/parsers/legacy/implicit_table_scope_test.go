package legacy

import (
	officeartescher "office/pkg/officeart/legacy/escher"
	presentationir "office/pkg/presentation/ir"
	"testing"
)

func TestImplicitTableRecoveryDoesNotFlattenNestedOrdinaryGroups(t *testing.T) {
	scene := legacyPptShapeContainerScene{
		groups: []legacyPptShapeGroup{
			{Path: []int{1}},
			{Path: []int{1, 2}, ParentPath: []int{1}},
			{Path: []int{1, 3}, ParentPath: []int{1}},
		},
		containers: []legacyPptShapeContainer{
			{GroupPath: []int{1}, Node: &officeartescher.ShapeNode{ShapeType: 20}},
			{GroupPath: []int{1, 2}, Node: &officeartescher.ShapeNode{HasClientTextbox: true}},
			{GroupPath: []int{1, 2}, Node: &officeartescher.ShapeNode{HasClientTextbox: true}},
			{GroupPath: []int{1, 3}, Node: &officeartescher.ShapeNode{HasClientTextbox: true}},
			{GroupPath: []int{1, 3}, Node: &officeartescher.ShapeNode{HasClientTextbox: true}},
		},
	}
	if legacyPptSceneGroupMayNeedTableRecovery(scene, scene.groups[0]) {
		t.Fatal("ancestor diagram group became an implicit table recovery candidate")
	}
	if legacyPptSceneGroupMayNeedTableRecovery(scene, scene.groups[1]) {
		t.Fatal("one-axis textbox stack became an implicit table recovery candidate without table semantics")
	}
}

func TestImplicitTableRecoveryStillAcceptsExplicitTableSemantics(t *testing.T) {
	cells := []legacyPptTableCellShape{{bounds: presentationir.Rect{X: 0, Y: 0, W: 100, H: 100}}}
	if !legacyPptSceneGroupOwnsTable(true, cells, nil) {
		t.Fatal("explicit table semantics must remain authoritative")
	}
}

func TestImplicitTableOwnershipRejectsStructuralRootGroup(t *testing.T) {
	root := legacyPptShapeGroup{Path: []int{1}}
	if legacyPptGroupMayOwnImplicitTable(root) {
		t.Fatal("structural root drawing group must never own an implicit table")
	}
	nested := legacyPptShapeGroup{Path: []int{1, 2}}
	if legacyPptGroupMayOwnImplicitTable(nested) {
		t.Fatal("nested geometry without authored table semantics must remain a group")
	}
}
