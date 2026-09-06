package word97

import (
	"testing"

	docformat "office/pkg/word/parsers/doc/format"
)

type revisionTestCursor struct {
	runs []Run
	pos  int
}

func (c *revisionTestCursor) Next() (Run, bool, error) {
	if c.pos >= len(c.runs) {
		return Run{}, false, nil
	}
	run := c.runs[c.pos]
	c.pos++
	return run, true, nil
}

func testRevisionDTTM(year, month, day int) uint32 {
	return uint32(year-1900)<<20 | uint32(month)<<16 | uint32(day)<<11
}

func TestRevisionCursorResolvesRunParagraphAndTableChanges(t *testing.T) {
	dttm := testRevisionDTTM(2024, 3, 4)
	inner := &revisionTestCursor{runs: []Run{{
		Text:          "changed",
		Revision:      "inserted",
		IbstRMark:     1,
		HasIbstRMark:  true,
		DttmRMark:     dttm,
		HasDttmRMark:  true,
		HasPropRMark:  true,
		PropRMarkIbst: 0,
		HasPropRMarkIbst: true,
		PropRMarkDttm: dttm,
		HasPropRMarkDttm: true,
		HasRunPropRMark: true,
		RunPropRMarkIbst: 1,
		HasRunPropRMarkIbst: true,
		RunPropRMarkDttm: dttm,
		HasRunPropRMarkDttm: true,
		HasDispFldRMark: true,
		DispFldRMarkIbst: 1,
		HasDispFldRMarkIbst: true,
		DispFldRMarkDttm: dttm,
		HasDispFldRMarkDttm: true,
		TableProps: &docformat.TableRowProps{PropertyRevision: &docformat.PendingRevisionMark{Present:true,Ibst:0,HasIbst:true,Dttm:dttm,HasDttm:true}},
	}}}
	cursor := NewRevisionCursor(inner, []string{"Alice", "Bob"})
	run, ok, err := cursor.Next()
	if err != nil || !ok { t.Fatalf("Next = ok %v err %v", ok, err) }
	if run.RevisionAuthor != "Bob" || run.RevisionDate != "2024-03-04" {
		t.Fatalf("revision = %q %q", run.RevisionAuthor, run.RevisionDate)
	}
	if run.ParagraphPropertyRevision == nil || run.ParagraphPropertyRevision.Author != "Alice" || run.ParagraphPropertyRevision.Date != "2024-03-04" {
		t.Fatalf("paragraph revision = %#v", run.ParagraphPropertyRevision)
	}
	if run.RunPropertyRevision == nil || run.RunPropertyRevision.Author != "Bob" {
		t.Fatalf("run property revision = %#v", run.RunPropertyRevision)
	}
	if run.DisplayFieldRevision == nil || run.DisplayFieldRevision.Author != "Bob" {
		t.Fatalf("display field revision = %#v", run.DisplayFieldRevision)
	}
	if run.TableProps == nil || run.TableProps.ResolvedPropertyRevision == nil || run.TableProps.ResolvedPropertyRevision.Author != "Alice" {
		t.Fatalf("table revision = %#v", run.TableProps)
	}
}

func TestWalkEmitsResolvedRevisionSemantics(t *testing.T) {
	dttm := testRevisionDTTM(2025, 1, 2)
	cursor := NewRevisionCursor(&revisionTestCursor{runs: []Run{{
		Text:"hello\r", Revision:"deleted", IbstRMark:0, HasIbstRMark:true, DttmRMark:dttm, HasDttmRMark:true,
		HasRunPropRMark:true, RunPropRMarkIbst:0, HasRunPropRMarkIbst:true,
	}}}, []string{"Carol"})
	var text Event
	if err := Walk(cursor, nil, func(event Event) error { if event.Type == EventText { text = event }; return nil }); err != nil { t.Fatal(err) }
	if text.Revision != "deleted" || text.RevisionAuthor != "Carol" || text.RevisionDate != "2025-01-02" {
		t.Fatalf("text revision event = %#v", text)
	}
	if text.RunPropertyRevision == nil || text.RunPropertyRevision.Author != "Carol" {
		t.Fatalf("property revision event = %#v", text.RunPropertyRevision)
	}
}
