package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestProductionPptPathConsumesPersistEventsForMasterAndHeaderFooterSemantics(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func parsePptFile(")
	end := strings.Index(src[start:], "\nfunc parsePre97Ppt(")
	if start < 0 || end < 0 {
		t.Fatal("parsePptFile boundary not found")
	}
	body := src[start : start+end]
	required := []string{
		"masterTextFallbackFromPersistEvents",
		"parseMasterStylesFromPersistEvents",
		"parsePersistHeaderFooterInfos",
		"composeLegacyPptMasterSemanticScenes",
		"masterComposition.Visuals",
		"extractSlideTextsFromPersistEvents",
		"presentationColorSchemeFromPersistEvents",
		"masterColorSchemesFromPersistEvents",
		"slideColorSchemesFromPersistEvents",
		"masterBackgroundsFromPersistEvents",
		"parseLegacyPptExHyperlinkURLsFromPayload",
		"parseLegacyPptThemeFromPayload",
		"parseLegacyPptExternalObjectsFromEvents",
		"parseLegacyPptPictureStoreFromPayload",
	}
	for _, token := range required {
		if !strings.Contains(body, token) {
			t.Fatalf("production PPT path does not consume event semantic %q", token)
		}
	}
	forbidden := []string{
		"parseLegacyPptMasterStylesByID(stream",
		"parseLegacyPptSlideHeaderFooterInfo(stream",
		"parseLegacyPptPersistHeaderFooterInfos(stream",
		"parseLegacyPptMasterVisualShapes(stream",
		"extractSlideTextsFromDocument(stream",
		"parseLegacyPptMasterBgFill(stream",
		"parseLegacyPptExHyperlinkURLs(stream",
		"parseLegacyPptTheme(stream",
		"parseLegacyPptExternalObjects(stream",
		"parseLegacyPptPictureStore(stream",
		"parseLegacyPptExternalObjectsFromPayload(persistEvents.document.Payload, stream",
		"parseLegacyPptOleStorage(stream",
	}
	for _, token := range forbidden {
		if strings.Contains(body, token) {
			t.Fatalf("production PPT path reopened persist semantics through %q", token)
		}
	}
}
