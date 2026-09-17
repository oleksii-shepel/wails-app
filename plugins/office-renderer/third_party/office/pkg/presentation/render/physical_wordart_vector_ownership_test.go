package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLWordArtVectorPathIsPhysicalLayoutOwned(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, forbidden := range []string{"textArchUpCurve", "textWave1", "textCircleCurve", "resolveWordArtPathData"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("renderer retained WordArt preset interpretation %q", forbidden)
		}
	}
	for _, required := range []string{"geometry.PathData", "<textPath", "geometry.StartOffset", "geometry.TextAnchor"} {
		if !strings.Contains(src, required) {
			t.Fatalf("renderer does not serialize physical WordArt vector contract %q", required)
		}
	}
}
