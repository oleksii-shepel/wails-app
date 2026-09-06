package pre97

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func collectRuns(t *testing.T, cursor Cursor) []Run {
	t.Helper()
	var out []Run
	for {
		r, ok, err := cursor.Next()
		if err != nil { t.Fatalf("Next: %v", err) }
		if !ok { break }
		out = append(out, r)
	}
	return out
}

func TestFieldCursorWithholdsInstructionAndAnnotatesResult(t *testing.T) {
	in := &sliceRunCursor{runs: []Run{{Text:"\x13HYPERLINK \"https://example.test\"\x14click\x15", StartCP:10}}}
	got := collectRuns(t, NewFieldCursor(in))
	if len(got) != 1 { t.Fatalf("runs = %d, want 1: %#v", len(got), got) }
	if got[0].Text != "click" || got[0].FieldKind != "HYPERLINK" || got[0].Target != "https://example.test" {
		t.Fatalf("field result = %#v", got[0])
	}
}

func TestReferenceCursorInjectsReferenceAtCP(t *testing.T) {
	in := &sliceRunCursor{runs: []Run{{Text:"A\x02B", StartCP:5}}}
	got := collectRuns(t, NewReferenceCursor(in, map[int]int{6:3}, nil, nil))
	if len(got) != 3 { t.Fatalf("runs = %#v", got) }
	if got[1].Kind != "footnoteRef" || got[1].NoteID != 3 || got[1].StartCP != 6 {
		t.Fatalf("reference = %#v", got[1])
	}
}

func TestBookmarkCursorSplitsAndInjectsBoundaries(t *testing.T) {
	in := &sliceRunCursor{runs: []Run{{Text:"abcd", StartCP:0}}}
	got := collectRuns(t, NewBookmarkCursor(in, map[int][]string{1:{"mark"}}, map[int][]string{3:{"mark"}}))
	var start, end bool
	for _, r := range got {
		if r.Kind == "bookmarkStart" && r.Anchor == "mark" && r.StartCP == 1 { start = true }
		if r.Kind == "bookmarkEnd" && r.Anchor == "mark" && r.StartCP == 3 { end = true }
	}
	if !start || !end { t.Fatalf("bookmark runs = %#v", got) }
}

func TestSectionBoundaryCursorSplitsVisibleRun(t *testing.T) {
	in := &sliceRunCursor{runs: []Run{{Text:"abcdef", StartCP:0}}}
	sections := []wordir.Section{{Source:&wordir.SectionSource{StartCP:0, EndCP:3}}}
	got := collectRuns(t, NewSectionBoundaryCursor(in, sections))
	if len(got) != 2 || got[0].Text != "abc" || got[1].Text != "def" || got[1].StartCP != 3 {
		t.Fatalf("split runs = %#v", got)
	}
}
