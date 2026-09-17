package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestBIG38ProductionHasNoSupersededMasterCompositionEntrypoints(t *testing.T) {
	ppt, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	persist, err := os.ReadFile("persist_events.go")
	if err != nil {
		t.Fatal(err)
	}
	visualScene, err := os.ReadFile("visual_scene.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(ppt) + "\n" + string(persist) + "\n" + string(visualScene)
	for _, dead := range []string{
		"func buildPresentationWithExtraShapes(",
		"func parseLegacyPptMasterPlaceholderTemplatesFromScenes(",
		"func parseLegacyPptMasterVisualShapesFromScenes(",
		"func applyLegacyPptPerMasterBackgrounds(",
		// BIG38 pass 2: valid persist semantics must not be reopened through
		// stream/document-offset compatibility wrappers. Payload/event owners are
		// now the only semantic entrypoints; damaged-file recovery stays explicit.
		"func parseLegacyPptTheme(",
		"func parseLegacyPptExHyperlinkURLs(",
		"func parseLegacyPptExternalObjects(",
		"func parseLegacyPptMasterStylesByID(",
		"func parseLegacyPptDocumentHeaderFooterInfo(",
		"func parseLegacyPptDocumentNotesHeaderFooterInfo(",
		"func parseLegacyPptDocumentHeaderFooterInfoByInstance(",
		"func parseLegacyPptSlideHeaderFooterInfo(",
		"func parseLegacyPptPersistHeaderFooterInfos(",
		"func parseLegacyPptPictureStore(",
		"func parseLegacyPptMasterBgFill(",
		// BIG38 pass 3: group hierarchy is materialized from the decoded
		// OfficeArt event/shape-container scene, never by recursively reopening
		// SpgrContainer payloads.
		"func parseLegacyPptGroupShapeFromContainer(",
		"func collectLegacyPptGroupChildren(",
		// BIG38 closure: production text/table ownership is scene-based. The old
		// event text walker and raw table classifier must not return as normal
		// semantic entrypoints. Raw compatibility inference is recovery-prefixed.
		"func parseLegacyPptTableShapeFromGroupContainer(",
		"func legacyPptGroupIsTableLike(",
		"func collectLegacyPptTableCellsFromGroup(",
	} {
		if strings.Contains(src, dead) {
			t.Fatalf("BIG38 retained superseded composition entrypoint %q", dead)
		}
	}
	for _, required := range []string{
		"composeLegacyPptMasterSemanticScenes(",
		"buildPresentationWithMasterComposition(",
		"masterComposition.PlaceholderTemplates",
		"masterComposition.Visuals",
	} {
		if !strings.Contains(string(ppt), required) {
			t.Fatalf("BIG38 production lost authoritative composition boundary %q", required)
		}
	}

	// Text is deliberately event-owned rather than ShapeNode-only: damaged and
	// encrypted PPTs can retain ClientTextbox semantics when structural nodes are
	// incomplete. Assert the ownership contract across the semantic-scene file
	// instead of coupling this gate to a call's exact formatting in ppt.go.
	visualSrc := string(visualScene)
	for _, required := range []string{
		"func (s legacyPptVisualScene) textMetas(",
		"collectLegacyPptTextShapesFromEvents(",
		"s.dispatch.containers.officeArtEvents()",
		"collectLegacyPptTableShapesFromScene(",
	} {
		if !strings.Contains(visualSrc, required) {
			t.Fatalf("BIG38 semantic scene lost authoritative visual ownership %q", required)
		}
	}
	for _, forbidden := range []string{
		"CollectOfficeArtEvents(",
		"collectLegacyPptTextShapesFromOfficeArt",
		"collectLegacyPptTableShapesFromOfficeArt",
	} {
		if strings.Contains(visualSrc, forbidden) {
			t.Fatalf("BIG38 semantic scene reopened a superseded OfficeArt traversal %q", forbidden)
		}
	}
}
