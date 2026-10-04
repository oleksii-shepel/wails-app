package fontmetrics

import (
	"testing"

	shared "office/pkg/fontmetrics"
)

func TestResolveFamilyUsesBundledSubstitute(t *testing.T) {
	catalog := shared.BundledCatalog()
	if catalog == nil || catalog.Empty() {
		t.Skip("no bundled metrics available")
	}
	tests := []struct {
		family string
		want   string
	}{
		{"Arial", "Arial"},                // bundled family wins over any substitute
		{"Helvetica Neue", "Arial"},       // the browser's substitute on this stack
		{"Aller Light", "Segoe UI"},       // first fallback the catalog can measure
		{"Totally Unknown Face", "Arial"}, // generic sans default
	}
	for _, tt := range tests {
		if got := ResolveFamily(catalog, tt.family, false, false); got != tt.want {
			t.Errorf("ResolveFamily(%q) = %q, want %q", tt.family, got, tt.want)
		}
	}
	if got := ResolveFamily(nil, "Arial", false, false); got != "" {
		t.Errorf("ResolveFamily(nil catalog) = %q, want empty", got)
	}
}

func TestFamilyFallbacksCoverCommonStacks(t *testing.T) {
	for family, want := range map[string]string{
		"Helvetica Neue": "Arial",
		"Helvetica":      "Arial",
		"Cambria":        "Times New Roman",
		"Consolas":       "Courier New",
	} {
		fallbacks := FamilyFallbacks(family)
		if len(fallbacks) == 0 || fallbacks[0] != want {
			t.Errorf("FamilyFallbacks(%q) = %v, want first %q", family, fallbacks, want)
		}
	}
}
