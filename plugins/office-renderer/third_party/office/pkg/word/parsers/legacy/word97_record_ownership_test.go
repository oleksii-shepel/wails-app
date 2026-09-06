package legacy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyWord97CoreConsumesCanonicalRecords(t *testing.T) {
	data, err := os.ReadFile("word97_core.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, forbidden := range []string{
		"loadLegacySttbfRMark(",
		"loadLegacyDopRecord(",
		"word97.LoadRevisionAuthorsReaderAt(",
		"word97.LoadDOPReaderAt(",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("legacy Word97 core retains or re-reads canonical record ownership: found %q", forbidden)
		}
	}
	for _, required := range []string{
		"opened.RevisionAuthors",
		"opened.RawDOP",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("legacy Word97 core must consume cached canonical record state: missing %q", required)
		}
	}

	openPath := filepath.Join("..", "doc", "word97", "open_readerat.go")
	openData, err := os.ReadFile(openPath)
	if err != nil {
		t.Fatal(err)
	}
	openText := string(openData)
	for _, required := range []string{
		"RevisionAuthors: LoadRevisionAuthorsReaderAt(",
		"RawDOP:          LoadDOPReaderAt(",
	} {
		if !strings.Contains(openText, required) {
			t.Fatalf("canonical OpenCore must own record decoding: missing %q", required)
		}
	}
}
