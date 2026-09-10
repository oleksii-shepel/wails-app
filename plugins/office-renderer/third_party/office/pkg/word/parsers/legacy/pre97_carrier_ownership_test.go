package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyDocParseDataIsPreWord97Only(t *testing.T) {
	docData, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(docData)

	start := strings.Index(doc, "type preWord97ParseData struct {")
	if start < 0 {
		t.Fatal("preWord97ParseData missing")
	}
	end := strings.Index(doc[start:], "\n}")
	if end < 0 {
		t.Fatal("preWord97ParseData declaration is incomplete")
	}
	carrier := doc[start : start+end]

	if strings.Contains(doc, "type legacyDocParseData struct") {
		t.Fatal("generic legacyDocParseData carrier must remain retired")
	}

	for _, dead := range []string{
		"DocSettings",
		"FontFamilies",
		"EmbeddedFonts",
		"RawDOP",
		"ClxData",
		"Compatibility",
		"TextBoxes",
		"Diagnostics",
	} {
		if strings.Contains(carrier, dead) {
			t.Fatalf("pre-97 compatibility carrier retains unproduced field %q", dead)
		}
	}

	if strings.Contains(carrier, "Structure") {
		t.Fatal("pre-97 compatibility carrier must not retain canonical Word97 structure state")
	}
	if strings.Contains(doc, "data.Structure") {
		t.Fatal("pre-97 parse path must not branch on canonical Word97 structure")
	}
	if !strings.Contains(doc, "extractPreWord97ParseDataByVersion(wordStream, f)") {
		t.Fatal("preWord97ParseData must remain behind the explicit pre-97 dispatcher")
	}

	projectionData, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	projection := string(projectionData)
	for _, forbidden := range []string{
		"*preWord97ParseData",
		"preWord97ParseData{",
		"loadLegacyWord97ParseData(",
		"legacyDocSettingsFromSequential(",
	} {
		if strings.Contains(projection, forbidden) {
			t.Fatalf("direct Word97 projection must not use pre-97 carrier plumbing %q", forbidden)
		}
	}
}
