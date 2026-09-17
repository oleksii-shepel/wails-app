package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyProductionVisualPassCollectsOfficeArtOnce(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func parseLegacyPptSlideVisualsEventDriven(")
	if start < 0 {
		t.Fatal("missing production visual pass")
	}
	body := src[start:]
	if next := strings.Index(body[5:], "\nfunc "); next >= 0 {
		body = body[:next+5]
	}
	if strings.Count(body, "collectLegacyPptVisualScene(payload)") != 1 {
		t.Fatal("production visual pass must collect one OfficeArt dispatch per slide")
	}
	for _, forbidden := range []string{
		"parseLegacyPptTextShapesFromSlide(payload",
		"parseLegacyPptTableShapesFromSlide(payload",
		"parseLegacyPptImageShapesFromSlide(payload",
		"parseLegacyPptConnectorShapesFromSlideWithTargets(payload",
		"parseLegacyPptGroupShapesFromSlide(payload",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("production visual pass retained independent traversal %q", forbidden)
		}
	}
	if !strings.Contains(body, "scene.project(") {
		t.Fatal("production groups must be built from the shared OfficeArt event sequence")
	}
}
