package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyTextAndTableFamiliesUseOfficeArtSceneEvents(t *testing.T) {
	data, err := os.ReadFile("visual_scene.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	// BIG31 removed the old per-family OfficeArt entry points. Text and tables
	// must now be projections of the semantic scene's already-collected event
	// sequence rather than independent binary traversals.
	for _, required := range []string{
		"collectLegacyPptTextShapesFromEvents(s.dispatch.containers.officeArtEvents()",
		"collectLegacyPptTableShapesFromScene(s.dispatch.containers",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("semantic visual scene must contain %q", required)
		}
	}

	for _, forbidden := range []string{
		"collectLegacyPptTextShapesFromOfficeArt",
		"collectLegacyPptTableShapesFromOfficeArt",
		"CollectOfficeArtEvents(",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("semantic visual scene retained superseded family traversal %q", forbidden)
		}
	}

	start := strings.Index(src, "func collectLegacyPptVisualScene(")
	if start < 0 {
		t.Fatal("missing semantic OfficeArt scene collector")
	}
	body := src[start:]
	if next := strings.Index(body[5:], "\nfunc "); next >= 0 {
		body = body[:next+5]
	}
	if strings.Count(body, "collectLegacyPptSlideVisualDispatch(payload)") != 1 {
		t.Fatal("semantic visual scene must materialize exactly one OfficeArt dispatch")
	}
}
