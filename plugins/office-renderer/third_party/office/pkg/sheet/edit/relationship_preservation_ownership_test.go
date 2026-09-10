package edit

import (
	"os"
	"strings"
	"testing"
)

func TestOpenXLSXReusesCanonicalParsePackageForRawPreservation(t *testing.T) {
	data, err := os.ReadFile("workbook.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "sheetooxml.ParseReaderAtWithPackage(r, size)") {
		t.Fatal("edit OpenXLSX must reuse the exact package graph from canonical OOXML parsing")
	}
	for _, forbidden := range []string{
		"opc.OpenReaderAt(r, size)",
		"sheetparts.LoadMetadataPackage(",
		"sheetparts.LoadRelationshipObjects(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("edit OpenXLSX retained duplicate OOXML parsing surface %q", forbidden)
		}
	}
}
