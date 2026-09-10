package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestLegacySectionBoundaryRunCursorSplitsTextAtSectionEnd(t *testing.T) {
	sections := []wordir.Section{
		{Source: &wordir.SectionSource{StartCP: 0, EndCP: 3}},
		{Source: &wordir.SectionSource{StartCP: 3, EndCP: 6}},
	}
	cursor := newLegacySectionBoundaryRunCursor(
		newLegacyRunSliceCursor([]binaryWordRun{{Text: "abcdef", StartCP: 0}}),
		sections,
	)
	first, ok, err := cursor.Next()
	if err != nil || !ok {
		t.Fatalf("first Next = %#v, %v, %v", first, ok, err)
	}
	second, ok, err := cursor.Next()
	if err != nil || !ok {
		t.Fatalf("second Next = %#v, %v, %v", second, ok, err)
	}
	if first.Text != "abc" || first.StartCP != 0 {
		t.Fatalf("first = %#v", first)
	}
	if second.Text != "def" || second.StartCP != 3 {
		t.Fatalf("second = %#v", second)
	}
	if _, ok, err := cursor.Next(); err != nil || ok {
		t.Fatalf("final Next ok=%v err=%v", ok, err)
	}
}
