package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestCanonicalWord97OwnsRevisionResolution(t *testing.T) {
	core, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	coreSource := string(core)
	if !strings.Contains(coreSource, "word97.MaterializeRunRange(") {
		t.Fatal("legacy Word97 core must materialize through the canonical resolved range API")
	}
	for _, forbidden := range []string{"RevisionAuthors []string", "fullBuffer.RevisionAuthors", "resolveCanonicalWord97RunRevisions("} {
		if strings.Contains(coreSource, forbidden) {
			t.Fatalf("legacy Word97 core retained revision ownership %q", forbidden)
		}
	}
	lossless, err := os.ReadFile("word97_run_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(lossless)
	for _, want := range []string{"r.RevisionAuthor", "r.RevisionDate", "r.RunPropertyRevision", "r.DisplayFieldRevision"} {
		if !strings.Contains(source, want) {
			t.Fatalf("legacy run adapter must preserve %q", want)
		}
	}
}
