package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestWalkLegacyCursorEmitsEmbeddedObject(t *testing.T) {
	core := &legacyDocStreamCore{OLEByCP: map[int]*wordir.Run{9: {Kind: "olePlaceholder", Text: "Embedded Object", OLEData: []byte{4,5}, OLEType: "Embed"}}}
	seen := false
	if err := walkLegacyCursorWithCore(newLegacyRunSliceCursor([]docRun{{Kind: "olePlaceholder", StartCP: 9}}), func(event StreamEvent) error {
		if event.Type == StreamEmbeddedObject {
			seen = true
			if event.EmbeddedRun == nil || len(event.EmbeddedRun.OLEData) != 2 { t.Fatalf("event = %#v", event) }
		}
		return nil
	}, core); err != nil { t.Fatalf("walk: %v", err) }
	if !seen { t.Fatal("embedded object event not emitted") }
}
