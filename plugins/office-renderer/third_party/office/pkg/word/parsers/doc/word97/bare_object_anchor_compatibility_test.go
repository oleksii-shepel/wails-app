package word97

import "testing"

func TestCompatibilityDrawingCursorTurnsBareObjectAnchorIntoPlaceholder(t *testing.T) {
	cursor := NewCompatibilityDrawingCursorWithPictureData(
		&sliceCursor{runs: []Run{{Text: "Before \x01 After", StartCP: 0}}},
		nil, nil, nil, nil, nil,
	)
	got := collectCursor(t, cursor)
	if len(got) != 3 {
		t.Fatalf("compatibility bare-object stream = %#v, want text + placeholder + text", got)
	}
	if got[0].Text != "Before " || got[1].Kind != "olePlaceholder" ||
		got[1].StartCP != 7 || got[2].Text != " After" {
		t.Fatalf("compatibility bare-object stream = %#v", got)
	}
}

func TestStrictDrawingCursorLeavesBareObjectAnchorUntouched(t *testing.T) {
	cursor := NewDrawingCursorWithPictureData(
		&sliceCursor{runs: []Run{{Text: "Before \x01 After", StartCP: 0}}},
		nil, nil, nil, nil, nil,
	)
	got := collectCursor(t, cursor)
	if len(got) != 1 || got[0].Text != "Before \x01 After" || got[0].Kind != "" {
		t.Fatalf("strict bare-object stream = %#v", got)
	}
}
