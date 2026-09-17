package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestProductionPptPathDoesNotSeekPersistedOLEStorage(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	start := strings.Index(src, "func parsePptFile(")
	endRel := strings.Index(src[start:], "\nfunc parsePre97Ppt(")
	if start < 0 || endRel < 0 {
		t.Fatal("parsePptFile boundary not found")
	}
	body := src[start : start+endRel]
	if !strings.Contains(body, "parseLegacyPptExternalObjectsFromEvents(persistEvents)") {
		t.Fatal("production PPT path does not resolve external objects from persist events")
	}
	for _, forbidden := range []string{"parseLegacyPptExternalObjectsFromPayload(persistEvents.document.Payload, stream", "parseLegacyPptOleStorage(stream"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("production PPT path still seeks persisted OLE storage through %q", forbidden)
		}
	}
}

func TestEventDrivenConnectorsUseEffectiveSlideColorScheme(t *testing.T) {
	data, err := os.ReadFile("visual_scene.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if !strings.Contains(src, "collectLegacyPptConnectorShapesFromEvents(s.dispatch.containers.officeArtEvents(), legacyPptGroupFrame{}, colorScheme") {
		t.Fatal("unified visual scene does not pass its effective slide color scheme to connectors")
	}
}
