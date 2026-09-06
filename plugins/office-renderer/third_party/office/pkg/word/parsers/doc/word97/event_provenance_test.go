package word97

import (
	"bytes"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestWalkCarriesParagraphAndRunProvenance(t *testing.T) {
	pStyle:=7
	cStyle:=9
	run:=Run{
		Text:"hello\r",StartCP:12,
		ParagraphFormat:wordir.ParagraphFormat{BinaryStyleIndex:&pStyle},
		Format:wordir.RunFormat{BinaryStyleIndex:&cStyle},
		PapxSprms:[]byte{1,2,3},ChpxSprms:[]byte{4,5,6},
		Revision:"inserted",Spec:true,
	}
	events:=collectEvents(t,&sliceCursor{runs:[]Run{run}},nil)
	var start *Event
	var text *Event
	for i:=range events{
		if events[i].Type==EventStartParagraph{start=&events[i]}
		if events[i].Type==EventText{text=&events[i]}
	}
	if start==nil||start.ParagraphSource==nil{t.Fatalf("paragraph source missing: %#v",events)}
	if start.ParagraphSource.StartCP!=12||start.ParagraphSource.StyleIndex==nil||*start.ParagraphSource.StyleIndex!=7||!bytes.Equal(start.ParagraphSource.PapxSprms,[]byte{1,2,3}){t.Fatalf("paragraph source=%+v",start.ParagraphSource)}
	if text==nil||text.RunSource==nil{t.Fatalf("run source missing: %#v",events)}
	if text.RunSource.StyleIndex==nil||*text.RunSource.StyleIndex!=9||text.RunSource.Revision!="inserted"||!text.RunSource.Spec||!bytes.Equal(text.RunSource.ChpxSprms,[]byte{4,5,6}){t.Fatalf("run source=%+v",text.RunSource)}
}
