package word97

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestCompatibilityDrawingCursorResolvesInlineImageWithoutPicLocation(t *testing.T) {
	image := &wordir.Image{}
	cursor := NewCompatibilityDrawingCursorWithPictureData(
		&sliceCursor{runs: []Run{{Text: "before\x01after", StartCP: 4, Spec: true}}},
		nil,
		[]*wordir.Image{image},
		nil,
		nil,
		nil,
	)
	got := collectCursor(t, cursor)
	if len(got) != 3 || got[0].Text != "before" || got[1].Kind != "image" || got[1].Image != image || got[2].Text != "after" {
		t.Fatalf("compatibility picture stream = %#v", got)
	}
}

func TestCompatibilityDrawingCursorResolvesDataAnchorWithoutPicLocation(t *testing.T) {
	image := &wordir.Image{}
	cursor := NewCompatibilityDrawingCursorWithPictureData(
		&sliceCursor{runs: []Run{{Text: "before\x01after", StartCP: 4, Data: true}}},
		nil,
		[]*wordir.Image{image},
		nil,
		nil,
		nil,
	)
	got := collectCursor(t, cursor)
	if len(got) != 3 || got[1].Kind != "image" || got[1].Image != image {
		t.Fatalf("compatibility data-picture stream = %#v", got)
	}
}

func TestStrictDrawingCursorStillDoesNotTreatFDataAsPicture(t *testing.T) {
	image := &wordir.Image{}
	cursor := NewDrawingCursorWithPictureData(
		&sliceCursor{runs: []Run{{Text: "\x01", StartCP: 4, Data: true}}},
		nil,
		[]*wordir.Image{image},
		nil,
		nil,
		nil,
	)
	got := collectCursor(t, cursor)
	if len(got) != 1 || got[0].Text != "\x01" || got[0].Kind != "" {
		t.Fatalf("strict drawing stream = %#v", got)
	}
}
