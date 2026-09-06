package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestCanonicalWord97OwnsRevisionResolution(t *testing.T) {
	pipeline, err := os.ReadFile("word97_fullbuffer_pipeline.go")
	if err != nil { t.Fatal(err) }
	pipelineSource := string(pipeline)
	if strings.Contains(pipelineSource, "resolveCanonicalWord97RunRevisions(") {
		t.Fatal("legacy full-buffer pipeline must not resolve Word97 revisions")
	}
	if !strings.Contains(pipelineSource, "core.RevisionAuthors") {
		t.Fatal("legacy adapter must pass revision authors into canonical range materialization")
	}

	lossless, err := os.ReadFile("lossless_runs.go")
	if err != nil { t.Fatal(err) }
	losslessSource := string(lossless)
	if !strings.Contains(losslessSource, "word97.MaterializeResolvedRunRangeFromCLX(") {
		t.Fatal("legacy range adapter must consume canonically resolved Word97 runs")
	}
	for _, want := range []string{"r.RevisionAuthor", "r.RevisionDate", "r.RunPropertyRevision", "r.DisplayFieldRevision"} {
		if !strings.Contains(losslessSource, want) {
			t.Fatalf("legacy run adapter must preserve %q", want)
		}
	}
}
