package legacy

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestLegacyStreamCoreDoesNotRegainCanonicalProjectionState(t *testing.T) {
	if _, err := os.Stat("word97_core.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retired Word97 staging core must stay removed: stat error = %v", err)
	}

	data, err := os.ReadFile("stream_core.go")
	if err != nil {
		t.Fatalf("read stream_core.go: %v", err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"CanonicalRuns",
		"word97.OpenSnapshotReaderAt(",
		"word97.MaterializeRunRange(",
		"word97.OpenCoreReaderAt(",
		"RawCLX",
		"RevisionAuthors",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("legacy/pre-97 stream core regained normal Word97 projection state %q", forbidden)
		}
	}
}
