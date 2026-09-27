package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestBig36ProductionTablesUseSceneOwnedTopology(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func collectLegacyPptTableShapesFromSceneForSlideWithOwnership")
	end := strings.Index(src[start:], "func legacyPptGroupPathKey")
	if start < 0 || end < 0 {
		t.Fatal("scene table boundary not found")
	}
	body := src[start : start+end]
	if strings.Contains(body, "recoverLegacyPptTableCellsFromGroup") {
		t.Fatal("scene table path must not recursively walk raw group payloads")
	}
	if !strings.Contains(body, "scene.containersUnder(group.Path)") {
		t.Fatal("scene table path must consume scene-owned descendants")
	}
}

func TestBig36ProductionGroupsDoNotUseRecursiveTableClassifier(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func buildLegacyPptGroupShapesFromSceneWithOwnership")
	end := strings.Index(src[start:], "func parseLegacyPptSlideVisualsEventDriven")
	if start < 0 || end < 0 {
		t.Fatal("scene group boundary not found")
	}
	body := src[start : start+end]
	if strings.Contains(body, "recoverLegacyPptGroupIsTableLike") || strings.Contains(body, "recoverLegacyPptTableShapeFromGroupContainer") {
		t.Fatal("production group projection must use scene-owned table routing")
	}
	if !strings.Contains(body, "parseLegacyPptTableShapeFromSceneGroup") {
		t.Fatal("production group projection does not consume scene table semantics")
	}
}
