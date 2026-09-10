package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97DetachedDrawingImageProjectionIsCanonical(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	projection := string(data)
	if !strings.Contains(projection, "word97.RecoverDetachedDrawingImages(doc, wordStream)") {
		t.Fatal("Word97 detached drawing-image recovery must be canonical")
	}
	for _, forbidden := range []string{
		"appendLegacyDetachedDrawingImages(",
		"appendPreWord97DetachedDrawingImages(",
		"extractLegacyDrawingImages(",
		"extractPreWord97DrawingImages(",
	} {
		if strings.Contains(projection, forbidden) {
			t.Fatalf("Word97 projection must not use legacy/pre-97 detached-image recovery %q", forbidden)
		}
	}

	drawingData, err := os.ReadFile("drawings.go")
	if err != nil {
		t.Fatal(err)
	}
	drawing := string(drawingData)
	if strings.Contains(drawing, "func appendLegacyDetachedDrawingImages(") {
		t.Fatal("mixed-era detached drawing-image projector must stay removed")
	}
	if !strings.Contains(drawing, "func appendPreWord97DetachedDrawingImages(") {
		t.Fatal("genuine pre-97 detached drawing-image compatibility path must remain explicit")
	}
}
