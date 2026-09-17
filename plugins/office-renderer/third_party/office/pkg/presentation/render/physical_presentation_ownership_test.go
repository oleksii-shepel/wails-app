package render

import (
	"os"
	"strings"
	"testing"
)

func TestHTMLConsumesPhysicalPresentationAndPlaceholderPlans(t *testing.T) {
	data, err := os.ReadFile("html.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, forbidden := range []string{"pres.Theme.ColorScheme", "placeholderClass(shape.Placeholder)"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("renderer retained presentation/layout interpretation %q", forbidden)
		}
	}
	for _, required := range []string{"ResolvePresentationVisualPlan(pres)", "item.PlaceholderClass"} {
		if !strings.Contains(source, required) {
			t.Fatalf("renderer must consume physical presentation contract %q", required)
		}
	}
}
