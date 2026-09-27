package legacy

import (
	"os"
	"strings"
	"testing"
)

// BIG39 keeps all OfficeArt-authored visual semantics attached when a shape is
// promoted into a richer PPT family. Pictures/OLE/media, connectors and grouped
// children must not lose the shared visual model merely because their semantic
// payload is specialized.
func TestBIG39RicherVisualFamiliesRetainOfficeArtComposition(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	required := []string{
		"shape.officeArt = officeartlegacy.ShapeFromEscher(node)",
		"OfficeArt: connector.officeArt",
		"applyLegacyPptOfficeArtSurface(connectorIR, connector.officeArt, connector.transform)",
		"pictureIR.OfficeArt = pictureShape.officeArt",
		"applyLegacyPptOfficeArtSurface(pictureIR, pictureShape.officeArt, pictureShape.transform)",
		"child.OfficeArt = officeartlegacy.ShapeFromEscherWithColorResolver(node",
		"applyLegacyPptOfficeArtSurface(child, child.OfficeArt, transform)",
	}
	for _, want := range required {
		if !strings.Contains(src, want) {
			t.Fatalf("BIG39 richer visual family lost shared OfficeArt composition boundary %q", want)
		}
	}
}
