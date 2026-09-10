package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

func TestCanonicalWord97RunProjectionPreservesEmbeddedRun(t *testing.T) {
	embedded := &wordir.Run{
		Kind:    "olePlaceholder",
		Text:    "Embedded Object",
		OLEMIME: "application/octet-stream",
		OLEType: "Embed",
	}
	got := adaptCanonicalWord97Run(word97.Run{
		Kind:        "olePlaceholder",
		StartCP:     7,
		EmbeddedRun: embedded,
	})
	if got.Kind != "olePlaceholder" || got.StartCP != 7 || got.EmbeddedRun != embedded {
		t.Fatalf("projected run = %#v, want embedded semantic run preserved", got)
	}
}
