package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestShapeContainerSceneMaterializesGroupTopology(t *testing.T) {
	scene := decodeLegacyPptShapeContainerScene([]OfficeArtEvent{
		{GroupStart: true}, pptTestShapeStartEvent(t, 1),
		{GroupStart: true}, pptTestShapeStartEvent(t, 2), {GroupEnd: true},
		{GroupEnd: true},
	})
	groups := scene.shapeGroups()
	if len(groups) != 2 {
		t.Fatalf("groups=%d, want 2", len(groups))
	}
	if groups[0].Depth != 1 || len(groups[0].ParentPath) != 0 {
		t.Fatalf("root=%+v", groups[0])
	}
	if groups[1].Depth != 2 || len(groups[1].ParentPath) != 1 || groups[1].ParentPath[0] != groups[0].Path[0] {
		t.Fatalf("nested=%+v root=%+v", groups[1], groups[0])
	}
}

func TestTableProjectionConsumesMaterializedGroupTopology(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func collectLegacyPptTableShapesFromScene")
	end := strings.Index(src[start:], "\nfunc legacyPptOfficeArtContainsExternalObject")
	if start < 0 || end < 0 {
		t.Fatal("table scene projection boundary not found")
	}
	body := src[start : start+end]
	if !strings.Contains(body, "scene.shapeGroups()") {
		t.Fatal("table projection must consume materialized group topology")
	}
	if strings.Contains(body, "walkCollectedOfficeArtEvents") || strings.Contains(body, "event.GroupStart") {
		t.Fatal("table projection must not replay raw group events")
	}
}

func TestShapeSceneGroupRootUsesDecodedNode(t *testing.T) {
	data, err := os.ReadFile("shape_container_scene.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "event.ShapeNode != nil && event.ShapeNode.GroupRect != nil") {
		t.Fatal("group-root identity must come from decoded ShapeNode")
	}
	if strings.Contains(src, "IsGroupRoot: legacyPptSpContainerHasGroupRect") {
		t.Fatal("shape scene must not reparse payload for group-root identity")
	}
}
