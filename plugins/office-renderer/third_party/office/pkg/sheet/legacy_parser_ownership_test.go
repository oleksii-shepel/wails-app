package sheet

import (
	"os"
	"strings"
	"testing"
)

func TestHighLevelXLSReadsUseCanonicalLegacyParserBoundary(t *testing.T) {
	data, err := os.ReadFile("open.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "sheetlegacy.ParseXlsBytes(data, name)") {
		t.Fatal("high-level XLS reads must delegate to the canonical legacy parser")
	}
	for _, forbidden := range []string{
		"cfb.Open",
		"parseWorkbook(",
		"ParseBIFFStream(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("high-level sheet API must not acquire BIFF parsing ownership %q", forbidden)
		}
	}
}
