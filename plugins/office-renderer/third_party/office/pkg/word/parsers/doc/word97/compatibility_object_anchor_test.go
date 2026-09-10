package word97

import "testing"

func TestCompatibilityDrawingCursorFallsBackToEmbeddedObjectPlaceholder(t *testing.T) {
	cursor := NewCompatibilityDrawingCursorWithPictureData(
		&sliceCursor{runs: []Run{{Text: "Before \x01 After", StartCP: 0, Spec: true}}},
		nil, nil, nil, nil, nil,
	)
	got := collectCursor(t, cursor)
	if len(got) != 3 {
		t.Fatalf("compatibility object stream = %#v, want text + placeholder + text", got)
	}
	if got[0].Text != "Before " || got[1].Kind != "olePlaceholder" || got[1].StartCP != 7 || got[1].EmbeddedRun == nil || got[1].EmbeddedRun.Kind != "olePlaceholder" || got[2].Text != " After" {
		t.Fatalf("compatibility object stream = %#v", got)
	}
}

func TestStrictDrawingCursorReportsUnresolvedRecognizedPicture(t *testing.T) {
	cursor := NewDrawingCursorWithPictureData(
		&sliceCursor{runs: []Run{{Text: "\x01", StartCP: 7, Spec: true, HasPicLocation: true, PicLocation: 123}}},
		nil, nil, nil, nil, nil,
	)
	_, ok, err := cursor.Next()
	if err == nil || ok {
		t.Fatalf("strict unresolved picture = ok %v err %v, want unsupported error", ok, err)
	}
}
