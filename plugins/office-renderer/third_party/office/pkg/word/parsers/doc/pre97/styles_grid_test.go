package pre97

import (
	"testing"

	wordir "office/pkg/word/ir"
)

type styleGridSliceCursor struct { runs []Run; i int }
func (c *styleGridSliceCursor) Next() (Run,bool,error) { if c==nil||c.i>=len(c.runs){return Run{},false,nil}; r:=c.runs[c.i]; c.i++; return r,true,nil }

func TestStyleCursorPreservesDirectFormatting(t *testing.T) {
	left, directLeft, level, num := 720, 1440, 2, 7
	catalog := &StyleCatalog{ParagraphByIndex: map[int]wordir.Style{4:{Format:wordir.ParagraphFormat{Alignment:"center",IndentLeft:&left,InTable:true,BinaryListOverride:&num,BinaryListLevel:&level}}}}
	istd := 4
	in := Run{Text:"cell\a", ParagraphFormat:wordir.ParagraphFormat{BinaryStyleIndex:&istd,IndentLeft:&directLeft}}
	cursor := NewStyleCursor(&styleGridSliceCursor{runs:[]Run{in}}, catalog)
	got,ok,err := cursor.Next(); if err!=nil||!ok{t.Fatalf("Next = %v %v",ok,err)}
	if got.ParagraphFormat.Alignment!="center" { t.Fatalf("alignment = %q",got.ParagraphFormat.Alignment) }
	if got.ParagraphFormat.IndentLeft==nil||*got.ParagraphFormat.IndentLeft!=directLeft { t.Fatalf("direct indent lost: %v",got.ParagraphFormat.IndentLeft) }
	if !got.ParagraphFormat.InTable || !got.LegacyInTableHint || !got.ParagraphFormat.IsTableCellTrailingParagraph { t.Fatalf("table inheritance not projected: %+v",got.ParagraphFormat) }
	if got.ParagraphFormat.BinaryListOverride==nil||*got.ParagraphFormat.BinaryListOverride!=num { t.Fatalf("num id = %v",got.ParagraphFormat.BinaryListOverride) }
}

func TestGridCursorReconstructsDenseRows(t *testing.T) {
	// Three physical rows, four grid marks each. Each row contains two visible
	// logical cells and two empty/merged grid cells.
	text := "A\aB\a\a\aC\aD\a\a\aE\aF\a\a\a"
	got := NormalizeGridRun(Run{Text:text,StartCP:10,ParagraphFormat:wordir.ParagraphFormat{InTable:true},LegacyInTableHint:true})
	if len(got)!=4 { t.Fatalf("runs = %d, want 4 (%#v)",len(got),got) }
	if got[0].Text!="A\tB\r" || got[1].Text!="C\tD\r" || got[2].Text!="E\tF\r" { t.Fatalf("rows = %q | %q | %q",got[0].Text,got[1].Text,got[2].Text) }
	if got[3].Kind!=GridBoundaryKind { t.Fatalf("boundary kind = %q",got[3].Kind) }
	for i:=0;i<3;i++ { if got[i].LegacyInTableHint || got[i].ParagraphFormat.InTable { t.Fatalf("row %d retained native table hint",i) } }
}

func TestGridCursorLeavesOrdinaryTableRunAlone(t *testing.T) {
	in := Run{Text:"cell\a",LegacyInTableHint:true}
	got := NormalizeGridRun(in)
	if len(got)!=1 || got[0].Text!=in.Text || !got[0].LegacyInTableHint { t.Fatalf("ordinary run changed: %#v",got) }
}
