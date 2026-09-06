package legacy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalWord97OwnsEmbeddedFontDecoding(t *testing.T) {
	legacyData, err := os.ReadFile("fonts.go")
	if err != nil {
		t.Fatal(err)
	}
	legacyText := string(legacyData)
	for _, forbidden := range []string{
		"loadLegacyEmbeddedFontDescriptors",
		"legacyEmbeddedFontDataSlice",
		"legacyLooksLikeEOT",
		"loadLegacyDocFontFamilies",
	} {
		if strings.Contains(legacyText, forbidden) {
			t.Fatalf("legacy font adapter retains canonical Word97 ownership: found %q", forbidden)
		}
	}

	canonicalPath := filepath.Join("..", "doc", "word97", "fonts.go")
	canonicalData, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	canonicalText := string(canonicalData)
	for _, required := range []string{
		"LoadFontFamiliesReaderAt(",
		"LoadEmbeddedFontsReaderAt(",
		"parseEmbeddedFontDescriptors(",
		"readEmbeddedFontPayload(",
		"looksLikeEOT(",
	} {
		if !strings.Contains(canonicalText, required) {
			t.Fatalf("canonical Word97 font reader must own font metadata decoding: missing %q", required)
		}
	}

	openPath := filepath.Join("..", "doc", "word97", "open_readerat.go")
	openData, err := os.ReadFile(openPath)
	if err != nil {
		t.Fatal(err)
	}
	openText := string(openData)
	for _, required := range []string{
		"FontFamilies:    LoadFontFamiliesReaderAt(",
		"EmbeddedFonts:   LoadEmbeddedFontsReaderAt(",
	} {
		if !strings.Contains(openText, required) {
			t.Fatalf("canonical OpenCore must cache font metadata: missing %q", required)
		}
	}

	coreData, err := os.ReadFile("word97_core.go")
	if err != nil {
		t.Fatal(err)
	}
	coreText := string(coreData)
	for _, required := range []string{
		"opened.FontFamilies",
		"opened.EmbeddedFonts",
	} {
		if !strings.Contains(coreText, required) {
			t.Fatalf("legacy Word97 adapter must consume cached canonical font metadata: missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"word97.LoadFontFamiliesReaderAt(",
		"word97.LoadEmbeddedFontsReaderAt(",
	} {
		if strings.Contains(coreText, forbidden) {
			t.Fatalf("legacy Word97 adapter must not re-read canonical font metadata: found %q", forbidden)
		}
	}
}
