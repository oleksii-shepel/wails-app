package sheet

import (
	"os"
	"strings"
	"testing"
)

func TestHighLevelOpenDoesNotOwnOOXMLParserInternals(t *testing.T) {
	data, err := os.ReadFile("open.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	for _, forbidden := range []string{
		`office/pkg/ooxml/opc`,
		`office/pkg/sheet/parsers/ooxml/parts`,
		`office/pkg/sheet/parsers/ooxml/resolve`,
		`parts.LoadPackage(`,
		`resolve.ResolvePackage(`,
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("high-level sheet facade regained XLSX parser internals %q", forbidden)
		}
	}
	if !strings.Contains(source, `sheetooxml.ParseBytes(data)`) {
		t.Fatal("high-level sheet facade must delegate XLSX parsing to parsers/ooxml")
	}
}
