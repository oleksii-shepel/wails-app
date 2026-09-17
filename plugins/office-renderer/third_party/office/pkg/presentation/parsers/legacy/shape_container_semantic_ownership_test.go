package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestVisualSceneOwnsTableAndGroupStructuralProjection(t *testing.T) {
	data, err := os.ReadFile("visual_scene.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{
		"collectLegacyPptTableShapesFromScene(s.dispatch.containers",
		"buildLegacyPptGroupShapesFromScene(s.dispatch.containers",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("visual scene must consume decoded container scene via %q", required)
		}
	}
	for _, forbidden := range []string{
		"collectLegacyPptTableShapesFromEvents(s.dispatch.containers.officeArtEvents()",
		"buildLegacyPptGroupShapesFromEvents(s.dispatch.containers.officeArtEvents()",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("visual scene retained raw family event projection %q", forbidden)
		}
	}
}

func TestGroupProjectionConsumesDecodedShapeIdentity(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func buildLegacyPptGroupShapesFromScene")
	if start < 0 {
		t.Fatal("unified group scene projection missing")
	}
	end := strings.Index(src[start+5:], "\nfunc ")
	body := src[start:]
	if end >= 0 {
		body = src[start : start+5+end]
	}
	for _, required := range []string{"event.ShapeNode.GroupRect", "event.ShapeNode.ShapeID"} {
		if !strings.Contains(body, required) {
			t.Fatalf("group scene must consume decoded node field %q", required)
		}
	}
	if strings.Contains(body, "legacyPptSpContainerShapeID(event.Payload)") {
		t.Fatal("group projection must not reparse shape identity from raw SpContainer payload")
	}
}
