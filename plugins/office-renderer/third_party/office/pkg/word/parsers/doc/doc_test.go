package doc

import (
	"errors"
	"testing"

	legacy "office/pkg/word/parsers/legacy"
)

func TestStreamContractOwnsUnsupportedErrors(t *testing.T) {
	err := translateStreamError(legacy.StreamUnsupportedError{Feature: "drawing anchor"})
	var unsupported StreamUnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %T %v, want StreamUnsupportedError", err, err)
	}
	if unsupported.Feature != "drawing anchor" {
		t.Fatalf("feature = %q, want drawing anchor", unsupported.Feature)
	}
}

func TestStreamEventTypesMatchUnifiedWordEvents(t *testing.T) {
	cases := map[StreamEventType]string{
		StreamStartDocument:     "startDocument",
		StreamEndDocument:       "endDocument",
		StreamStartParagraph:    "startParagraph",
		StreamEndParagraph:      "endParagraph",
		StreamStartTable:        "startTable",
		StreamEndTable:          "endTable",
		StreamStartTableRow:     "startTableRow",
		StreamEndTableRow:       "endTableRow",
		StreamStartTableCell:    "startTableCell",
		StreamEndTableCell:      "endTableCell",
		StreamText:              "text",
		StreamTab:               "tab",
		StreamBreak:             "break",
		StreamDrawing:           "drawing",
		StreamEmbeddedObject:    "embeddedObject",
		StreamBookmarkStart:     "bookmarkStart",
		StreamBookmarkEnd:       "bookmarkEnd",
		StreamFootnoteReference: "footnoteReference",
		StreamEndnoteReference:  "endnoteReference",
		StreamCommentReference:  "commentReference",
		StreamSection:           "section",
	}
	for eventType, want := range cases {
		if got := string(eventType); got != want {
			t.Fatalf("event type = %q, want %q", got, want)
		}
	}
}
