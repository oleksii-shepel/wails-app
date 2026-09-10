package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97OLERecoveryIsCanonical(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	projection := string(data)
	for _, required := range []string{
		"word97.RecoverOLECompoundPayloads(f, doc)",
		"word97.RecoverOLEPreviews(f, doc)",
	} {
		if !strings.Contains(projection, required) {
			t.Fatalf("Word97 projection must use canonical %q", required)
		}
	}
	for _, forbidden := range []string{
		"applyLegacyOLECompoundRecovery(",
		"applyLegacyOLEMetafilePreviews(",
		"applyPreWord97OLECompoundRecovery(",
		"applyPreWord97OLEMetafilePreviews(",
	} {
		if strings.Contains(projection, forbidden) {
			t.Fatalf("Word97 projection retained legacy/pre-97 OLE recovery %q", forbidden)
		}
	}

	metaData, err := os.ReadFile("legacy_metadata.go")
	if err != nil {
		t.Fatal(err)
	}
	meta := string(metaData)
	for _, required := range []string{
		"func applyPreWord97OLECompoundRecovery(",
		"func applyPreWord97OLEMetafilePreviews(",
	} {
		if !strings.Contains(meta, required) {
			t.Fatalf("genuine pre-97 OLE compatibility must remain explicit: %q", required)
		}
	}
}
