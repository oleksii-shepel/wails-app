package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestOfficeArtEventsDecodeShapeContainersOnce(t *testing.T) {
	data, err := os.ReadFile("officeart_events.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "e.ShapeNode, _ = officeartescher.ParseShapeContainerPayload(r.Payload)") {
		t.Fatal("OfficeArt event boundary must own SpContainer decoding")
	}
}

func TestLeafVisualFamiliesConsumeEventShapeNodes(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{
		"parseLegacyPptTextShapeNode(event.ShapeNode",
		"parseLegacyPptConnectorShapeNode(event.ShapeNode",
		"parseLegacyPptPictureShapeNode(event.ShapeNode",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("missing shared shape-node consumption %q", required)
		}
	}
}

func TestShapeContainerSceneCarriesDecodedNodeIdentity(t *testing.T) {
	data, err := os.ReadFile("shape_container_scene.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{"Node        *officeartescher.ShapeNode", "shapeID = event.ShapeNode.ShapeID", "Node: event.ShapeNode"} {
		if !strings.Contains(src, required) {
			t.Fatalf("shape-container scene missing %q", required)
		}
	}
}
