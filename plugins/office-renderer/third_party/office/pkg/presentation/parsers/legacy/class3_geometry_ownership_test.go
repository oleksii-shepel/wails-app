package legacy

import (
	"os"
	"strings"
	"testing"
)

// Class 3 owns coordinate-space semantics in canonical VisualTransform. Legacy
// parsing may decode Escher group frames, but it must not grow a second shape
// transform executor beside presentation/ir.ApplyVisualTransform.
func TestClass3LegacyGeometryUsesCanonicalVisualTransformExecutor(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if strings.Contains(src, "func applyLegacyPptGroupTransformToShape") {
		t.Fatal("legacy parser must not own a duplicate group-transform executor")
	}
	if !strings.Contains(src, "func legacyPptCanonicalGroupTransform") {
		t.Fatal("legacy parser must retain only the Escher-to-canonical VisualTransform adapter")
	}
}
