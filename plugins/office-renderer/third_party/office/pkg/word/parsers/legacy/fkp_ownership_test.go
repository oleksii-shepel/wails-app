package legacy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyBinaryFKPAdapterIsRemoved(t *testing.T) {
	if _, err := os.Stat("fkp_binary_adapter.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy binary FKP adapter must stay removed: stat error = %v", err)
	}
}

func TestCanonicalWord97OwnsFKPTraversal(t *testing.T) {
	path := filepath.Join("..", "doc", "word97", "open_readerat.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{"docbinary.BTEPageNumbers(", "docformat.ParseCHPXFKP(", "docformat.ParsePAPXFKP("} {
		if !strings.Contains(text, required) {
			t.Fatalf("canonical Word97 open path must own FKP traversal/decoding: missing %q", required)
		}
	}
}

func TestLegacyProductionDoesNotCallWord97FKPLoaders(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") || base == "chpx.go" || base == "papx.go" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, forbidden := range []string{"loadChpxEntries(", "loadPapxEntries("} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s must not call legacy Word97 FKP loader %q", base, forbidden)
			}
		}
	}
}
