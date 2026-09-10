package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLosslessMainRunAdapterHasNoWord97Ownership(t *testing.T) {
	data, err := os.ReadFile("lossless_runs.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{
		"word97.Run",
		"legacyRunFromCanonicalWord97(",
		"adaptCanonicalWord97Run(",
		"MaterializeResolvedRunRangeFromCLX", "MaterializeRunRangeFromCLX",
		"ParseComplexFileTable", "BTEPageNumbers", "ParseCHPXFKP", "ParsePAPXFKP",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("generic lossless run code retained Word97 ownership %q", forbidden)
		}
	}
}
