package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestLegacySectionAdapterDoesNotOwnWord97Structure(t *testing.T) {
	data, err := os.ReadFile("sepx.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "word97.OpenSnapshotReaderAt(") {
		t.Fatal("legacy section adapter must consume canonical Word97 snapshot")
	}
	for _, forbidden := range []string{
		"func loadDocSections(",
		"legacySepxGrpprlAt(",
		"loadLegacySttbfRMark(",
		"sedFibIndex",
		"tableStreamName := \"0Table\"",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("legacy section adapter must not own Word97 structural decoding: found %q", forbidden)
		}
	}
}

func TestLegacyHeaderAdapterDoesNotOwnWord97Structure(t *testing.T) {
	data, err := os.ReadFile("hdrftr.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "word97.OpenSnapshotReaderAt(") {
		t.Fatal("legacy header/footer adapter must consume canonical Word97 snapshot")
	}
	for _, forbidden := range []string{
		"loadDocHeadersFootersViaPlcfHdd(",
		"extractPlcPcdFromClx(",
		"extractDocTextInCPRange(",
		"buildHdrFtrStoryFromRange(",
		"loadChpxEntries(",
		"loadPapxEntries(",
		"hddIndex",
		"fcClx",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("legacy header/footer adapter must not own Word97 structural decoding: found %q", forbidden)
		}
	}
}
