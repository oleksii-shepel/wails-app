package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLosslessMainRunAdapterDelegatesCanonical(t *testing.T) {
	data, err := os.ReadFile("lossless_runs.go")
	if err != nil {
		t.Fatalf("read lossless_runs.go: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "word97.MaterializeResolvedRunRangeFromCLX") {
		t.Fatalf("lossless main-run adapter must delegate resolved CLX range materialization to canonical Word97")
	}
	for _, forbidden := range []string{
		"word97.MaterializeRunRangeFromCLX(",
		"docbinary.ParseComplexFileTable(",
		"assembleDocRunsFromClxRange(",
		"assembleDocRunsFromPlcPcdRange(",
		"BTEPageNumbers(",
		"ParseCHPXFKP(",
		"ParsePAPXFKP(",
		"resolveCanonicalWord97RunRevisions(",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("lossless main-run adapter regained canonical assembly/semantic responsibility: found %q", forbidden)
		}
	}
}
