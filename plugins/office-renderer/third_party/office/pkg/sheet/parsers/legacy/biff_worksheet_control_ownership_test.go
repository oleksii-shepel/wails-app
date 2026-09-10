package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestCanonicalBIFFWorksheetParserHasNoInlineControlSwitch(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func parseBIFFWorksheet")
	if start < 0 {
		t.Fatal("canonical BIFF worksheet parser missing")
	}
	tail := source[start:]
	if end := strings.Index(tail[1:], "\nfunc "); end >= 0 {
		tail = tail[:end+1]
	}
	if !strings.Contains(tail, "consumeWorksheetControlRecord(") {
		t.Fatal("BIFF worksheet parser must delegate substream/control records")
	}
	for _, id := range []string{"biffEOF", "biffBOF", "biffPrintSize", "biffPLS", "biffBitmap", "biffIndex", "biffDBCell"} {
		if strings.Contains(tail, "case "+id) {
			t.Fatalf("physical control record leaked into canonical dispatcher: %s", id)
		}
	}
}

func TestWorksheetControlFamilyOwnsEmbeddedChartSubstreams(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	start := strings.Index(source, "func consumeWorksheetControlRecord")
	end := strings.Index(source[start:], "func parseBIFFWorksheet")
	if start < 0 || end < 0 {
		t.Fatal("worksheet control boundary missing")
	}
	block := source[start : start+end]
	for _, required := range []string{"biffEOF", "biffBOF", "skipChartSubstreamRecordAggregate", "biffPrintSize", "biffDBCell"} {
		if !strings.Contains(block, required) {
			t.Fatalf("control family missing %q", required)
		}
	}
}

func TestLegacyWorksheetParserNameIsRetired(t *testing.T) {
	data, err := os.ReadFile("xls.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "parseWorksheetWithDrawingStore") {
		t.Fatal("legacy implementation-detail worksheet parser name must be retired")
	}
}
