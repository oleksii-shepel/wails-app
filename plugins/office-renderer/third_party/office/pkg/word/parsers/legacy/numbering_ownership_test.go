package legacy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalWord97OwnsNumberingBinaryLayout(t *testing.T) {
	bridgeData, err := os.ReadFile("numbering_stream_bridge.go")
	if err != nil {
		t.Fatal(err)
	}
	bridge := string(bridgeData)
	if !strings.Contains(bridge, "func legacyListTablesFromCanonical(") {
		t.Fatal("legacy numbering compatibility must adapt the canonical catalog")
	}
	for _, forbidden := range []string{
		"FibRgFcLcb",
		"readFibFcLcbEntry(",
		"parseLegacyStructuredPlfLst",
		"parseLegacyStructuredPlfLfo",
		"binary.LittleEndian",
	} {
		if strings.Contains(bridge, forbidden) {
			t.Fatalf("numbering bridge regained Word97 binary ownership: found %q", forbidden)
		}
	}

	coreData, err := os.ReadFile("word97_core.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(coreData), "legacyListTablesFromCanonical(opened.Numbering)") {
		t.Fatal("legacy Word97 core must consume the canonical numbering catalog")
	}

	for _, path := range []string{
		"word97_core.go",
		"word97_fullbuffer_pipeline.go",
		"stream.go",
		"stream_cursor.go",
		"doc_pipeline.go",
		"word97_enrichment.go",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "loadLegacyListTables(") {
			t.Fatalf("production adapter %s must not invoke the old Word97 list decoder", path)
		}
	}

	canonicalPath := filepath.Join("..", "doc", "word97", "numbering.go")
	canonicalData, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatal(err)
	}
	canonical := string(canonicalData)
	for _, required := range []string{
		"LoadNumberingCatalogReaderAt(",
		"parsePlfLst(",
		"parsePlfLfo(",
		"parseLVL(",
	} {
		if !strings.Contains(canonical, required) {
			t.Fatalf("canonical Word97 numbering reader missing %q", required)
		}
	}
}
