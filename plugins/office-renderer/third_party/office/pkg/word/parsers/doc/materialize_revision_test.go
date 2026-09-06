package doc

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestMaterializerPromotesRevisionEventSemantics(t *testing.T) {
	m := newMaterializer("revision.doc")
	paragraphRevision := &wordir.RevisionMark{Author:"Alice", Date:"2024-01-01"}
	runRevision := &wordir.RevisionMark{Author:"Bob", Date:"2024-02-02"}
	displayRevision := &wordir.RevisionMark{Author:"Carol", Date:"2024-03-03"}
	events := []StreamEvent{
		{Type:StreamStartDocument},
		{Type:StreamStartParagraph, ParagraphPropertyRevision:paragraphRevision},
		{Type:StreamText, Text:"changed", Revision:"inserted", RevisionAuthor:"Bob", RevisionDate:"2024-02-02", RunPropertyRevision:runRevision, DisplayFieldRevision:displayRevision},
		{Type:StreamEndParagraph},
		{Type:StreamEndDocument},
	}
	for _, event := range events { if err:=m.Handle(event);err!=nil{t.Fatal(err)} }
	doc:=m.Document()
	if len(doc.Paragraphs)!=1||doc.Paragraphs[0].PropertyRevision==nil||doc.Paragraphs[0].PropertyRevision.Author!="Alice"{t.Fatalf("paragraph revision = %#v",doc.Paragraphs)}
	if len(doc.Paragraphs[0].Runs)!=1{t.Fatalf("runs = %#v",doc.Paragraphs[0].Runs)}
	run:=doc.Paragraphs[0].Runs[0]
	if run.Revision!="inserted"||run.RevisionAuthor!="Bob"||run.RevisionDate!="2024-02-02"{t.Fatalf("revision = %#v",run)}
	if run.PropertyRevision==nil||run.PropertyRevision.Author!="Bob"{t.Fatalf("property revision = %#v",run.PropertyRevision)}
	if run.DisplayFieldRevision==nil||run.DisplayFieldRevision.Author!="Carol"{t.Fatalf("display revision = %#v",run.DisplayFieldRevision)}
}
