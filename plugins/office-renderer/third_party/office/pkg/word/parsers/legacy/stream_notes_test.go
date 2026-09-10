package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestWalkLegacyCursorEmitsReferenceStoryPayload(t *testing.T) {
	core := &legacyDocStreamCore{Footnotes: map[int]wordir.Story{1: {Paragraphs: []wordir.Paragraph{{Text: "note"}}}}}
	cursor := newLegacyRunSliceCursor([]binaryWordRun{{Kind: "footnoteRef", NoteID: 1}})
	seen := false
	if err := walkLegacyCursorWithCore(cursor, func(event StreamEvent) error {
		if event.Type == StreamFootnoteReference {
			seen = true
			if event.NoteID != 1 || event.Story == nil || len(event.Story.Paragraphs) != 1 {
				t.Fatalf("bad footnote event: %#v", event)
			}
		}
		return nil
	}, core); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if !seen {
		t.Fatal("footnote reference event not emitted")
	}
}
