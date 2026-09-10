package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97DrawingImageRecoveryDoesNotUseLegacyMetafileAdapter(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	projection := string(data)

	if !strings.Contains(projection, "word97.RecoverDetachedDrawingImages(doc, wordStream)") {
		t.Fatal("Word97 detached image recovery must be fully canonical")
	}
	for _, forbidden := range []string{
		"extractLegacyDrawingImages(",
		"extractPreWord97DrawingImages(",
		"legacyOfficeArtMetafileImage(",
		"legacyExtractOfficeArtMetafile(",
		"legacyEMFToSVG(",
	} {
		if strings.Contains(projection, forbidden) {
			t.Fatalf("Word97 projection retained legacy image/metafile dependency %q", forbidden)
		}
	}

	drawingData, err := os.ReadFile("drawings.go")
	if err != nil {
		t.Fatal(err)
	}
	drawing := string(drawingData)
	if strings.Contains(drawing, "func extractLegacyDrawingImages(") {
		t.Fatal("mixed-era drawing image extraction name must stay removed")
	}
	if !strings.Contains(drawing, "func extractPreWord97DrawingImages(") {
		t.Fatal("genuine pre-97 drawing compatibility extraction must remain explicit")
	}
}
