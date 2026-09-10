package legacy

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestLegacyWord97DOPProjectionConsumesCanonicalSettings(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	for _, required := range []string{
		"word97.OpenSnapshotReaderAt(",
		"word97.ApplySnapshotDocumentState(doc, snapshot)",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("Word97 projection must consume canonical DOP/settings through %q", required)
		}
	}
	for _, forbidden := range []string{
		"applyCanonicalWord97Settings(",
		"loadLegacyDocSettings(",
		"loadLegacyDopRecord(",
		"DecodeDocumentSettings(",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Word97 projection retained legacy/piecemeal DOP ownership %q", forbidden)
		}
	}
	if _, err := os.Stat("dop.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dead legacy DOP carrier must stay removed: %v", err)
	}
}
