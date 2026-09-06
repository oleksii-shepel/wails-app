package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyStreamCoreDoesNotRegainCanonicalProjectionState(t *testing.T) {
	data, err := os.ReadFile("word97_core.go")
	if err != nil {
		t.Fatalf("read word97_core.go: %v", err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"TableStreamName string",
		"StyleCatalog    *word97.StyleCatalog",
		"ExpectedTextboxes           int",
		"ResolvedTextboxes           int",
		"buildLegacyWord97DocCore(",
		"applySequentialStyleCatalog(",
		"TableStream     []byte",
		"FibBase         int",
		"CbRgFcLcb       int",
		"StoryCounts     legacyStoryCounts",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("legacy stream core regained canonical-owned state %q", forbidden)
		}
	}
}
