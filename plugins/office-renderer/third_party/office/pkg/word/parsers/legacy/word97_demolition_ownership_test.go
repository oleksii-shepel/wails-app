package legacy

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestOrphanedWord97StoryRecoveryStaysRemoved(t *testing.T) {
	if _, err := os.Stat("word97_story_recovery.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphaned Word97 story recovery must stay removed: stat error = %v", err)
	}
}

func TestLegacyRevisionHelpersDoNotDecodeSttbfRMark(t *testing.T) {
	data, err := os.ReadFile("revisions.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{
		"loadLegacySttbfRMark(",
		"sttbfRMarkFibIndex",
		"utf16.Decode(",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("legacy revision helpers must not decode Word97 revision-author records: found %q", forbidden)
		}
	}
}
