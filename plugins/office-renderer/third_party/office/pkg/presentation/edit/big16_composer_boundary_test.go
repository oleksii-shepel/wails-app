package edit

import (
	"os"
	"strings"
	"testing"
)

func TestPPTXProductionLoaderDoesNotUseWholePackageVisualResolver(t *testing.T) {
	data, err := os.ReadFile("presentation.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, forbidden := range []string{"Compose" + "Package", "Resolve" + "Package", "InspectComposer" + "Dependencies"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("PPTX production loader reacquired superseded whole-package visual boundary %q", forbidden)
		}
	}
	for _, required := range []string{"pressax.ParseBytes"} {
		if !strings.Contains(src, required) {
			t.Fatalf("missing SAX ownership marker %q", required)
		}
	}
}
