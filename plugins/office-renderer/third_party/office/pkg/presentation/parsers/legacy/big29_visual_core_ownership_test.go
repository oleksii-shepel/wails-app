package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyVisualCoreHasNoPersistReopenFamilyEntrypoints(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, dead := range []string{
		"func parseLegacyPptSlideTextShapes(",
		"func parseLegacyPptSlideTableShapes(",
		"func parseLegacyPptSlideImageShapes(",
		"func parseLegacyPptSlideConnectorShapes(",
		"func parseLegacyPptSlideGroupShapes(",
		"func parseLegacyPptMasterVisualShapes(",
	} {
		if strings.Contains(src, dead) {
			t.Fatalf("obsolete visual persist-reopen entrypoint retained: %s", dead)
		}
	}
	if !strings.Contains(src, "collectLegacyPptMasterVisualScenes(persistEvents, ctx.masterEntries)") {
		t.Fatal("production path must materialize one shared OfficeArt scene per master")
	}
	if !strings.Contains(src, "composeLegacyPptMasterSemanticScenes(") {
		t.Fatal("master placeholder and artwork projections must share one master semantic composition boundary")
	}
	for _, split := range []string{
		"parseLegacyPptMasterPlaceholderTemplatesFromScenes(masterVisualScenes",
		"parseLegacyPptMasterVisualShapesFromScenes(masterVisualScenes",
	} {
		if strings.Contains(src, split) {
			t.Fatalf("production path must not restore split master semantic projection %q", split)
		}
	}
}

func TestLegacyPersistNotesConsumeSharedOfficeArtEvents(t *testing.T) {
	data, err := os.ReadFile("persist_events.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func hydrateLegacyPptNotesFromPersistEvents(")
	if start < 0 {
		t.Fatal("missing persist-event notes hydration")
	}
	body := src[start:]
	if next := strings.Index(body[5:], "\nfunc "); next >= 0 {
		body = body[:next+5]
	}
	if strings.Contains(body, "parseLegacyPptTextShapesFromSlide(") {
		t.Fatal("valid persist notes path must not reopen the OfficeArt family parser")
	}
	if !strings.Contains(body, "collectLegacyPptVisualScene(e.Payload)") ||
		!strings.Contains(body, "scene.textMetas(") {
		t.Fatal("notes text must consume the shared OfficeArt event scene")
	}
}
