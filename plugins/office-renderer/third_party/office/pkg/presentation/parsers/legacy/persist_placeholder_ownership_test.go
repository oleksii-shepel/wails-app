package legacy

import (
	"os"
	"strings"
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestMasterPlaceholderTemplateInheritsMissingVisualProperties(t *testing.T) {
	template := legacyPptTextShape{
		placeholder: presentationir.PlaceholderTitle,
		bounds:      presentationir.Rect{X: 10, Y: 20, W: 300, H: 40},
		fillColor:   "112233",
		transform: legacyPptShapeTransform{
			HasRotation: true, Rotation: 5,
			LineColor: "445566", LineWidthEMU: 12700,
		},
	}
	local := []legacyPptTextShape{{placeholder: presentationir.PlaceholderTitle}}
	got := inheritLegacyPptMasterPlaceholderTemplates(local, map[presentationir.PlaceholderKind]legacyPptTextShape{
		presentationir.PlaceholderTitle: template,
	})
	if len(got) != 1 || got[0].bounds != template.bounds {
		t.Fatalf("master placeholder bounds not inherited: %#v", got)
	}
	if got[0].fillColor != template.fillColor || got[0].transform.LineColor != template.transform.LineColor || !got[0].transform.HasRotation {
		t.Fatalf("master placeholder visual properties not inherited: %#v", got[0])
	}
}

func TestMasterPlaceholderTemplateDoesNotOverrideSlideAnchorOrPaint(t *testing.T) {
	localBounds := presentationir.Rect{X: 1, Y: 2, W: 100, H: 50}
	local := []legacyPptTextShape{{placeholder: presentationir.PlaceholderBody, bounds: localBounds, fillColor: "ABCDEF", transform: legacyPptShapeTransform{LineColor: "010203"}}}
	template := legacyPptTextShape{placeholder: presentationir.PlaceholderBody, bounds: presentationir.Rect{X: 9, Y: 9, W: 900, H: 900}, fillColor: "FFFFFF", transform: legacyPptShapeTransform{LineColor: "999999"}}
	got := inheritLegacyPptMasterPlaceholderTemplates(local, map[presentationir.PlaceholderKind]legacyPptTextShape{presentationir.PlaceholderBody: template})
	if got[0].bounds != localBounds || got[0].fillColor != "ABCDEF" || got[0].transform.LineColor != "010203" {
		t.Fatalf("slide-authored placeholder properties were overwritten: %#v", got[0])
	}
}

func TestPptProductionPlaceholderInheritanceUsesPersistEvents(t *testing.T) {
	data, err := os.ReadFile("ppt.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{
		"composeLegacyPptMasterSemanticScenes(",
		"masterComposition.PlaceholderTemplates",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("production PPT path must contain %q", required)
		}
	}
}
