package legacy

import (
	"os"
	"strings"
	"testing"
)

// Pass 10 freezes the parser takeover: slide/master production must enter the
// semantic OfficeArt scene once and leave through completeProjection/effective
// composition. Compatibility-only façades and dead raw scanners are forbidden.
func TestPass10SingleAuthoritativeVisualPipeline(t *testing.T) {
	pptBytes, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	visualBytes, err := os.ReadFile("visual_scene.go")
	if err != nil {
		t.Fatal(err)
	}
	ppt, visual := string(pptBytes), string(visualBytes)

	start := strings.Index(ppt, "func parseLegacyPptSlideVisualsEventDriven(")
	if start < 0 {
		t.Fatal("missing production slide visual boundary")
	}
	body := ppt[start:]
	if next := strings.Index(body[5:], "\nfunc "); next >= 0 {
		body = body[:next+5]
	}
	if strings.Count(body, "collectLegacyPptVisualScene(payload)") != 1 {
		t.Fatal("slide visual production must materialize exactly one semantic scene")
	}
	if !strings.Contains(body, "scene.completeProjection(") {
		t.Fatal("slide visual production does not consume completeProjection")
	}
	for _, forbidden := range []string{
		"scene.project(",
		"collectLegacyPptTextShapesFromEvents(",
		"collectLegacyPptTableShapesFromSceneForSlideWithOwnership(",
		"collectLegacyPptImageShapesFromEvents(",
		"collectLegacyPptConnectorShapesFromEvents(",
		"buildLegacyPptGroupShapesFromSceneWithOwnership(",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("production slide pass bypasses completeProjection via %q", forbidden)
		}
	}

	for _, forbidden := range []string{
		"func (s legacyPptVisualScene) project(",
		"func (s legacyPptVisualScene) pictures(",
		"func (s legacyPptVisualScene) tables(",
		"func (s legacyPptVisualScene) connectors(",
		"func (s legacyPptVisualScene) groups(",
		"func (s legacyPptVisualScene) textShapes(",
	} {
		if strings.Contains(visual, forbidden) {
			t.Fatalf("semantic scene retained compatibility facade %q", forbidden)
		}
	}
	for _, required := range []string{
		"func (s legacyPptVisualScene) completeProjection(",
		"collectLegacyPptTableShapesFromSceneForSlideWithOwnership(",
		"collectLegacyPptImageShapesFromEvents(",
		"collectLegacyPptConnectorShapesFromEvents(",
		"buildLegacyPptGroupShapesFromSceneWithOwnership(",
		"return legacyPptVisualProjection{Shapes: shapes, Ownership:",
	} {
		if !strings.Contains(visual, required) {
			t.Fatalf("authoritative semantic projection lost %q", required)
		}
	}
}

func TestPass10SupersededCompatibilityEntrypointsStayDeleted(t *testing.T) {
	pptBytes, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	ppt := string(pptBytes)
	for _, dead := range []string{
		"func buildLegacyPptGroupShapesFromScene(",
		"func collectLegacyPptTableShapesFromScene(",
		"func collectLegacyPptTableShapesFromSceneForSlide(",
		"func extractSlideTexts(",
		"func extractSlideTextsFromCurrentUser(",
		"func mergeLegacyShapeMaps(",
		"func parseLegacyPptBSEPicture(",
		"func extractLegacyPptPictureBlob(",
		"func extractLegacyPptOfficeArtBlipRaster(",
		"func extractLegacyPptOfficeArtBlipVector(",
		"func legacyPptExtractVectorBlob(",
		"func legacyPptVectorImageStart(",
		"func legacyPptLooksLikeWMF(",
		"func legacyPptInflate(",
		"func legacyPptLocalChildBoundsFromSpContainer(",
		"func parseLegacyPptGroupRect(",
		"func parseLegacyPptPictureBlipRef(",
		"func parseLegacyPptConnectorAdjustments(",
		"func legacyPptFopt32ToFraction(",
		"func legacyPptImageContentType(",
		"func legacyPptImageEnd(",
		"func legacyPptScanPNGEnd(",
		"func legacyPptScanJPEGEnd(",
		"func legacyPptScanGIFEnd(",
		"func sortedLegacyPptTablePositions(",
	} {
		if strings.Contains(ppt, dead) {
			t.Fatalf("Pass 10 retained superseded compatibility/dead entrypoint %q", dead)
		}
	}
}
