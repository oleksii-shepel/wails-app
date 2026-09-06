package word97

import (
	"testing"

	wordir "office/pkg/word/ir"
)

type sliceCursor struct { runs []Run; index int }
func (c *sliceCursor) Next()(Run,bool,error){ if c==nil||c.index>=len(c.runs){return Run{},false,nil}; r:=c.runs[c.index]; c.index++; return r,true,nil }

func collectCursor(t *testing.T, c Cursor) []Run {
	t.Helper()
	var out []Run
	for { r,ok,err:=c.Next(); if err!=nil{t.Fatal(err)}; if !ok{break}; out=append(out,r) }
	return out
}

func TestReferenceCursorSplitsAtPhysicalCP(t *testing.T) {
	c:=NewReferenceCursor(&sliceCursor{runs:[]Run{{Text:"abc",StartCP:10}}},map[int]int{11:4},map[int]int{12:5},nil)
	got:=collectCursor(t,c)
	if len(got)!=5 || got[0].Text!="a" || got[1].Kind!="footnoteRef" || got[1].NoteID!=4 || got[2].Text!="b" || got[3].Kind!="endnoteRef" || got[3].NoteID!=5 || got[4].Text!="c" { t.Fatalf("reference stream = %#v",got) }
}

func TestBookmarkCursorInjectsBoundariesWithoutLosingText(t *testing.T) {
	c:=NewBookmarkCursor(&sliceCursor{runs:[]Run{{Text:"abcd",StartCP:20}}},map[int][]string{21:{"bm"}},map[int][]string{23:{"bm"}})
	got:=collectCursor(t,c)
	var text string; starts,ends:=0,0
	for _,r:=range got { text+=r.Text; if r.Kind=="bookmarkStart"&&r.Anchor=="bm"{starts++}; if r.Kind=="bookmarkEnd"&&r.Anchor=="bm"{ends++} }
	if text!="abcd" || starts!=1 || ends!=1 { t.Fatalf("bookmark stream text=%q starts=%d ends=%d runs=%#v",text,starts,ends,got) }
}

func TestSectionBoundaryCursorSplitsCrossingRun(t *testing.T) {
	sections:=[]wordir.Section{{Source:&wordir.SectionSource{EndCP:32}}}
	c:=NewSectionBoundaryCursor(&sliceCursor{runs:[]Run{{Text:"abcd",StartCP:30}}},sections)
	got:=collectCursor(t,c)
	if len(got)!=2 || got[0].Text!="ab" || got[0].StartCP!=30 || got[1].Text!="cd" || got[1].StartCP!=32 { t.Fatalf("section split = %#v",got) }
}
