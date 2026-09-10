package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyLosslessStoryDoesNotRebuildWord97OpenCore(t *testing.T) {
	data, err := os.ReadFile("lossless_story.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, forbidden := range []string{
		"word97.OpenCore",
		"docbinary.ParseComplexFileTable",
		"canonicalLosslessStoryCore(",
		"materializeCanonicalLosslessStory(",
		"materializeCanonicalLosslessStoryRange(",
		"toSequentialCHPXEntries(",
		"toSequentialPAPXEntries(",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("legacy lossless story retained obsolete Word97 bridge %q", forbidden)
		}
	}
	if _, err := os.Stat("word97_run_convert.go"); !os.IsNotExist(err) {
		t.Fatalf("obsolete Word97 run conversion file still exists: %v", err)
	}
}
