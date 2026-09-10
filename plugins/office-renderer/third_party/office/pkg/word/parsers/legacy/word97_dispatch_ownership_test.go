package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestParseDocDispatchesWord97ToDirectCanonicalDocumentProjection(t *testing.T) {
	data, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	for _, required := range []string{
		"doc, err := loadCanonicalWord97Document(filename, wordStream, f)",
		"return doc, nil",
		"extractPreWord97ParseDataByVersion(wordStream, f)",
		"func extractPreWord97ParseDataByVersion(",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("missing explicit Word97/pre-97 dispatch boundary %q", required)
		}
	}
	for _, forbidden := range []string{
		"loadLegacyWord97ParseData(",
		"func extractDocRunsViaFIB(",
		"func extractDocRunsByVersion(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("retired Word97 compatibility dispatch remains %q", forbidden)
		}
	}
	word97Branch := source[strings.Index(source, "if nFib, ok := legacyDocFibVersion(wordStream); ok && nFib >= legacyDocFibWord97 {"):]
	pre97At := strings.Index(word97Branch, "} else if data := extractPreWord97ParseDataByVersion")
	if pre97At < 0 {
		t.Fatal("cannot isolate Word97 dispatch branch")
	}
	word97Branch = word97Branch[:pre97At]
	for _, forbidden := range []string{"extractDocTextFromTable(", "fallbackScanDocText(", "buildLegacyDoc("} {
		if strings.Contains(word97Branch, forbidden) {
			t.Fatalf("recognized Word97 dispatch must not fall through to heuristic parser %q", forbidden)
		}
	}

}
