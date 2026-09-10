package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestDocRunsToIRRunsPreservesEmbeddedObjectPayload(t *testing.T) {
	embedded := &wordir.Run{
		Kind:    "olePlaceholder",
		Text:    "Embedded Object",
		OLEMIME: "application/octet-stream",
		OLEType: "Embed",
	}
	got := binaryWordRunsToIRRuns([]binaryWordRun{{
		Kind:        "olePlaceholder",
		StartCP:     7,
		EmbeddedRun: embedded,
	}}, wordir.RunFormat{})
	if len(got) != 1 {
		t.Fatalf("runs = %#v, want one embedded-object run", got)
	}
	if got[0].Kind != "olePlaceholder" || got[0].Text != "Embedded Object" ||
		got[0].OLEMIME != "application/octet-stream" || got[0].OLEType != "Embed" {
		t.Fatalf("embedded object = %#v", got[0])
	}
}
