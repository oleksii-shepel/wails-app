package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyWord97AdapterConsumesCanonicalSnapshot(t *testing.T) {
	data, err := os.ReadFile("word97_core.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "word97.OpenSnapshotReaderAt(") {
		t.Fatal("legacy Word97 adapter must consume canonical snapshot")
	}
	for _, forbidden := range []string{
		"word97.OpenCoreReaderAt(",
		"word97.LoadDocumentContextReaderAt(",
		"word97.LoadDocumentSettingsReaderAt(",
		"word97.LoadFontFamiliesReaderAt(",
		"word97.LoadEmbeddedFontsReaderAt(",
		"word97.LoadRevisionAuthorsReaderAt(",
		"word97.LoadDOPReaderAt(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("legacy Word97 adapter must not orchestrate canonical loaders: found %q", forbidden)
		}
	}
	for _, want := range []string{
		"opened.RawCLX",
		"opened.CHPX",
		"opened.PAPX",
		"opened.Numbering",
		"opened.Settings",
		"opened.FontFamilies",
		"opened.EmbeddedFonts",
		"opened.RevisionAuthors",
		"opened.RawDOP",
		"snapshot.Context",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("legacy Word97 adapter must project %q", want)
		}
	}
}
