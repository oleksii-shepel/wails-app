package legacy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWord97CompatibilityAdapterIsRemoved(t *testing.T) {
	if _, err := os.Stat("word97_open_compat.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy Word97 compatibility adapter must stay removed: stat error = %v", err)
	}
}

func TestCanonicalWord97OwnsSEPXProjection(t *testing.T) {
	contextPath := filepath.Join("..", "doc", "word97", "context_readerat.go")
	contextData, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatal(err)
	}
	contextText := string(contextData)
	if !strings.Contains(contextText, "docformat.DecodeSEPX(") {
		t.Fatal("canonical Word97 context must project sections through doc/format DecodeSEPX")
	}

	formatPath := filepath.Join("..", "doc", "format", "sepx_full.go")
	formatData, err := os.ReadFile(formatPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(formatData), "func DecodeSEPX(") {
		t.Fatal("canonical DOC format package must own DecodeSEPX")
	}
}
