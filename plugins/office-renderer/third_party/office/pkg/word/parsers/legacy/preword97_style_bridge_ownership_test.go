package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestPreWord97StyleBridgeExposesOnlyNeutralAssemblyCatalog(t *testing.T) {
	data, err := os.ReadFile("preword97_style_bridge.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	for _, required := range []string{
		"func binaryWordStyleCatalogFromPre97Catalog(",
		"*binaryWordStyleCatalog",
		"binaryWordStyleCatalogFromResolved(",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("pre-97 style bridge missing neutral assembly boundary %q", required)
		}
	}
	for _, forbidden := range []string{
		"legacyStyleSheetFromPre97Catalog(",
		"return &legacyStyleSheet{",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("pre-97 style bridge retained obsolete legacy projection %q", forbidden)
		}
	}
}
