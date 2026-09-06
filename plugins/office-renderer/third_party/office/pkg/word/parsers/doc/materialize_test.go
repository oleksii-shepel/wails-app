package doc

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestMaterializerBuildsParagraphDrawingAndNote(t *testing.T){
	m:=newMaterializer("sample.doc")
	story:=wordir.Story{Text:"note"}
	img:=&wordir.Image{MIMEType:"image/png"}
	events:=[]StreamEvent{
		{Type:StreamStartDocument},
		{Type:StreamStartParagraph},
		{Type:StreamText,Text:"hello"},
		{Type:StreamDrawing,Image:img},
		{Type:StreamFootnoteReference,NoteID:3,Story:&story},
		{Type:StreamEndParagraph},
		{Type:StreamEndDocument},
	}
	for _,e:=range events{if err:=m.Handle(e);err!=nil{t.Fatal(err)}}
	doc:=m.Document();if doc==nil{t.Fatal("nil document")}
	if doc.Filename!="sample.doc"{t.Fatalf("filename=%q",doc.Filename)}
	if len(doc.Paragraphs)!=1||doc.Paragraphs[0].Text!="hello"{t.Fatalf("paragraphs=%#v",doc.Paragraphs)}
	if len(doc.Paragraphs[0].Runs)!=3{t.Fatalf("runs=%d",len(doc.Paragraphs[0].Runs))}
	if doc.Paragraphs[0].Runs[1].Kind!="image"||doc.Paragraphs[0].Runs[1].Image!=img{t.Fatalf("drawing=%#v",doc.Paragraphs[0].Runs[1])}
	if got:=doc.Footnotes[3].Text;got!="note"{t.Fatalf("footnote=%q",got)}
	if len(doc.Blocks)!=1||doc.Blocks[0].Kind!="paragraph"{t.Fatalf("blocks=%#v",doc.Blocks)}
}

func TestMaterializerBuildsTableInDocumentOrder(t *testing.T){
	m:=newMaterializer("table.doc")
	events:=[]StreamEvent{
		{Type:StreamStartDocument},
		{Type:StreamStartTable},{Type:StreamStartTableRow},{Type:StreamStartTableCell},
		{Type:StreamStartParagraph},{Type:StreamText,Text:"cell"},{Type:StreamEndParagraph},
		{Type:StreamEndTableCell},{Type:StreamEndTableRow},{Type:StreamEndTable},
		{Type:StreamEndDocument},
	}
	for _,e:=range events{if err:=m.Handle(e);err!=nil{t.Fatal(err)}}
	doc:=m.Document();if len(doc.Tables)!=1{t.Fatalf("tables=%d",len(doc.Tables))}
	if len(doc.Tables[0].Rows)!=1||len(doc.Tables[0].Rows[0].Cells)!=1{t.Fatalf("table=%#v",doc.Tables[0])}
	cell:=doc.Tables[0].Rows[0].Cells[0]
	if cell.Text!="cell"||len(cell.Paragraphs)!=1{t.Fatalf("cell=%#v",cell)}
	if len(doc.Blocks)!=1||doc.Blocks[0].Kind!="table"{t.Fatalf("blocks=%#v",doc.Blocks)}
}
