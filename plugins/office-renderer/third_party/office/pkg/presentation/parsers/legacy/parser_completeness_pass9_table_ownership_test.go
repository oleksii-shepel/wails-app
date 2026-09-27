package legacy

import (
	"encoding/binary"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
	presentationir "office/pkg/presentation/ir"
)

func TestPass9ProjectedTableOwnsAuthoredGridLineContainers(t *testing.T) {
	prop := make([]byte, 6)
	binary.LittleEndian.PutUint16(prop[0:2], legacyOfficeArtPropTableProperties)
	binary.LittleEndian.PutUint32(prop[2:6], 1)

	groupRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xA01, 0x01)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, make([]byte, 16))...,
		),
		buildOfficeArtRecord(1, officeartescher.RecordTertiaryFOPT, 0x0003, prop)...,
	))
	groupPayload := append([]byte{}, groupRoot...)

	lineNode := &officeartescher.ShapeNode{ShapeID: 0xA02, ShapeType: 20}
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.groups = []legacyPptShapeGroup{{
		Event: OfficeArtEvent{Payload: groupPayload},
		Path:  []int{1},
	}}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{{
		ShapeID:   lineNode.ShapeID,
		GroupPath: []int{1},
		Node:      lineNode,
	}}

	audit := scene.ownershipAudit([]*presentationir.Shape{{Kind: presentationir.ShapeTable, OriginalShapeID: 0xA01}})
	if len(audit) != 1 || !audit[0].Claimed || audit[0].Reason != "table topology" {
		t.Fatalf("projected table grid-line ownership = %+v, want claimed table topology", audit)
	}
}

func TestPass9UnmaterializedExplicitTableClaimsGridTopologyButReportsOneGroupGap(t *testing.T) {
	prop := make([]byte, 6)
	binary.LittleEndian.PutUint16(prop[0:2], legacyOfficeArtPropTableProperties)
	binary.LittleEndian.PutUint32(prop[2:6], 1)

	groupRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xB01, 0x01)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, make([]byte, 16))...,
		),
		buildOfficeArtRecord(1, officeartescher.RecordTertiaryFOPT, 0x0003, prop)...,
	))
	lineNode := &officeartescher.ShapeNode{ShapeID: 0xB02, ShapeType: 20}
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.groups = []legacyPptShapeGroup{{
		Event: OfficeArtEvent{Payload: groupRoot},
		Path:  []int{1},
	}}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{{
		ShapeID:   lineNode.ShapeID,
		GroupPath: []int{1},
		Node:      lineNode,
	}}

	audit := scene.ownershipAudit(nil)
	if len(audit) != 2 {
		t.Fatalf("ownership entries = %d, want grid topology + one table-group gap: %+v", len(audit), audit)
	}
	if !audit[0].Claimed || audit[0].Reason != "table topology" {
		t.Fatalf("explicit table grid line = %+v, want structural table topology", audit[0])
	}
	if audit[1].Claimed || audit[1].Reason != "unclaimed: explicit table group did not materialize" {
		t.Fatalf("explicit table group = %+v, want one materialization gap", audit[1])
	}
}

func TestPass9RecoveredTableOwnershipUsesGroupPathNotRootShapeID(t *testing.T) {
	lineNode := &officeartescher.ShapeNode{ShapeID: 0xC02, ShapeType: 20}
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{{
		ShapeID:   lineNode.ShapeID,
		GroupPath: []int{7},
		Node:      lineNode,
	}}

	// Compatibility table recovery can successfully materialize a table even
	// when the authored group-root ShapeID cannot be recovered reliably. The
	// projector therefore reports the semantic group path directly.
	audit := scene.ownershipAuditWithTablePaths(nil, [][]int{{7}})
	if len(audit) != 1 || !audit[0].Claimed || audit[0].Reason != "table topology" {
		t.Fatalf("group-path table ownership = %+v, want claimed table topology", audit)
	}
}

func TestPass9AncestorTablePathDoesNotHideNestedExplicitTableMaterializationGap(t *testing.T) {
	prop := make([]byte, 6)
	binary.LittleEndian.PutUint16(prop[0:2], legacyOfficeArtPropTableProperties)
	binary.LittleEndian.PutUint32(prop[2:6], 1)
	nestedRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xD01, 0x01)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, make([]byte, 16))...,
		),
		buildOfficeArtRecord(1, officeartescher.RecordTertiaryFOPT, 0x0003, prop)...,
	))
	lineNode := &officeartescher.ShapeNode{ShapeID: 0xD02, ShapeType: 20}
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.groups = []legacyPptShapeGroup{{
		Event: OfficeArtEvent{Payload: nestedRoot},
		Path:  []int{1, 2},
	}}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{{
		ShapeID:   lineNode.ShapeID,
		GroupPath: []int{1, 2},
		Node:      lineNode,
	}}

	// Only the ancestor table materialized. The nested explicit table did not.
	// Its line is still structural table topology, but the nested table itself
	// must remain visible as one semantic materialization gap.
	audit := scene.ownershipAuditWithTablePaths(nil, [][]int{{1}})
	if len(audit) != 2 {
		t.Fatalf("nested table ownership entries = %d, want topology + group gap: %+v", len(audit), audit)
	}
	if !audit[0].Claimed || audit[0].Reason != "table topology" {
		t.Fatalf("nested table line = %+v, want structural topology", audit[0])
	}
	if audit[1].Claimed || audit[1].Reason != "unclaimed: explicit table group did not materialize" {
		t.Fatalf("nested explicit table gap = %+v", audit[1])
	}
}

func TestPass9OwnedAncestorDoesNotSuppressNestedExplicitTableProjection(t *testing.T) {
	prop := make([]byte, 6)
	binary.LittleEndian.PutUint16(prop[0:2], legacyOfficeArtPropTableProperties)
	binary.LittleEndian.PutUint32(prop[2:6], 1)
	explicitRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F, append(
		append(
			buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xE01, 0x01)),
			buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0x0000, make([]byte, 16))...,
		),
		buildOfficeArtRecord(1, officeartescher.RecordTertiaryFOPT, 0x0003, prop)...,
	))
	explicit := legacyPptShapeGroup{Event: OfficeArtEvent{Payload: explicitRoot}, Path: []int{1, 2}}
	if legacyPptSkipTableGroupProjection(explicit, [][]int{{1}}) {
		t.Fatal("owned ancestor suppressed nested explicit table")
	}

	ordinary := legacyPptShapeGroup{Path: []int{1, 3}}
	if !legacyPptSkipTableGroupProjection(ordinary, [][]int{{1}}) {
		t.Fatal("owned ancestor should suppress ordinary nested group table inference")
	}
}
