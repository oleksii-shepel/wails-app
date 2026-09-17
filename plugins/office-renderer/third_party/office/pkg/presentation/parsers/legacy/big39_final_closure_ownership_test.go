package legacy

import (
	"os"
	"strings"
	"testing"
)

// BIG39 closes the planned legacy PPT architecture/fidelity program. Future
// corpus fixes must extend the semantic scene or effective composition model,
// not restore family-specific raw OfficeArt compatibility entrypoints.
func TestBIG39FinalClosureKeepsSemanticSceneAndEffectiveCompositionAuthoritative(t *testing.T) {
	visual, err := os.ReadFile("visual_scene.go")
	if err != nil {
		t.Fatal(err)
	}
	ppt, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	visualSource, pptSource := string(visual), string(ppt)
	for _, want := range []string{"return s.sourceOrdered(shapes)", "s.genericShapes(shapes, colorScheme)", "buildLegacyPptGroupShapesFromScene"} {
		if !strings.Contains(visualSource, want) {
			t.Fatalf("BIG39 semantic scene lost closure boundary %q", want)
		}
	}
	for _, want := range []string{"composeLegacyPptMasterVisuals", "cloneLegacyShapeRecursive", "cloneLegacyOfficeArtShape", "buildPresentationWithMasterComposition"} {
		if !strings.Contains(pptSource, want) {
			t.Fatalf("BIG39 effective composition lost closure boundary %q", want)
		}
	}
	if strings.Contains(pptSource, "func parseLegacyPptPictureShapeContainer(") {
		t.Fatal("BIG39 restored unused raw picture-container compatibility parser")
	}
}
