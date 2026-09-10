package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestPreWord97CompatibilityStateHasExplicitOwnership(t *testing.T) {
	data, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	for _, required := range []string{
		"type preWord97ParseData struct",
		"func extractPreWord97ParseDataByVersion(",
		"func extractDocRunsViaPreWord97FIB(",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("explicit pre-97 ownership missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"type legacyDocParseData struct",
		"extractPreWord97RunsByVersion(",
		"loadLegacyWord97ParseData(",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("retired mixed/generic compatibility surface remains %q", forbidden)
		}
	}
}
