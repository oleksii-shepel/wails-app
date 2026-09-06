package word97

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestStyleCursorPromotesListAndTableHints(t *testing.T) {
	istd:=3
	ilfo:=8
	level:=2
	inner:=&sliceCursor{runs:[]Run{{Text:"cell\a",ParagraphFormat:wordir.ParagraphFormat{BinaryStyleIndex:&istd}}}}
	resolve:=func(index int)(wordir.ParagraphFormat,bool){
		if index!=istd{return wordir.ParagraphFormat{},false}
		return wordir.ParagraphFormat{InTable:true,BinaryListOverride:&ilfo,BinaryListLevel:&level},true
	}
	c:=NewStyleCursor(inner,resolve,func(run *Run,style wordir.ParagraphFormat){ if run.ParagraphFormat.Alignment==""{run.ParagraphFormat.Alignment="center"} })
	got:=collectCursor(t,c)
	if len(got)!=1 { t.Fatalf("runs=%#v",got) }
	r:=got[0]
	if r.ParagraphFormat.Alignment!="center" || !r.ParagraphFormat.InTable || !r.LegacyInTableHint || !r.ParagraphFormat.IsTableCellTrailingParagraph { t.Fatalf("style projection=%#v",r) }
	if r.ParagraphFormat.BinaryListOverride==nil || *r.ParagraphFormat.BinaryListOverride!=ilfo || r.ParagraphFormat.BinaryListLevel==nil || *r.ParagraphFormat.BinaryListLevel!=level { t.Fatalf("list projection=%#v",r.ParagraphFormat) }
}
