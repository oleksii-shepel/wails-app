package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyWord97DOPProjectionConsumesCanonicalSettings(t *testing.T) {
	coreData, err := os.ReadFile("word97_core.go")
	if err != nil {
		t.Fatal(err)
	}
	core := string(coreData)
	if !strings.Contains(core, "core.Compatibility = opened.Settings.Compatibility") {
		t.Fatal("legacy Word97 core must carry canonical DOP compatibility")
	}

	pipelineData, err := os.ReadFile("word97_fullbuffer_pipeline.go")
	if err != nil {
		t.Fatal(err)
	}
	pipeline := string(pipelineData)
	if !strings.Contains(pipeline, "Compatibility:  core.Compatibility") {
		t.Fatal("legacy Word97 full-buffer adapter must consume canonical compatibility")
	}
	if strings.Contains(pipeline, "loadLegacyDocCompatibility(") {
		t.Fatal("legacy Word97 full-buffer adapter must not re-decode RawDOP")
	}

	dopData, err := os.ReadFile("dop.go")
	if err != nil {
		t.Fatal(err)
	}
	dop := string(dopData)
	if !strings.Contains(dop, "word97.DecodeDocumentSettings(") {
		t.Fatal("legacy DOP compatibility shim must delegate semantic decoding to canonical Word97")
	}
	for _, forbidden := range []string{"0x1FC:0x21C", "SuppressSpacingAtTopOfPage", "FootnoteNumberFormat =", "EndnoteNumberFormat ="} {
		if strings.Contains(dop, forbidden) {
			t.Fatalf("legacy dop.go must not own DOP semantic bit layout: found %q", forbidden)
		}
	}
}
