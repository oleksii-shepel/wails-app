package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestParserCompletenessTextProjectionCarriesFullOfficeArt(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, required := range []string{
		"officeArt      *officeartir.Shape",
		"ShapeFromEscherWithColorResolver(node",
		"OfficeArt:       meta.officeArt",
		"applyLegacyPptOfficeArtSurface(shape, meta.officeArt, meta.transform)",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("sparse text projection still drops OfficeArt semantics: missing %q", required)
		}
	}
}

func TestParserCompletenessEmptyTextboxDoesNotEraseAuthoredVisual(t *testing.T) {
	data, err := os.ReadFile("visual_scene.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if strings.Contains(body, "node.ShapeType == legacyOfficeArtShapePicture || node.HasClientTextbox") {
		t.Fatal("generic projection still drops every ClientTextbox-bearing OfficeArt shape")
	}
	if !strings.Contains(body, "if _, textOwned := ownedIDs[container.ShapeID]; textOwned") {
		t.Fatal("ClientTextbox ownership is not conditioned on an actual text projection")
	}
}
