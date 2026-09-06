package stream

import (
	"bytes"
	"testing"
)

func TestDOCXLayoutPreservesBookmarkAndComplexFieldResult(t *testing.T) {
	data := makeDOCXBodyFixture(t, `<w:p><w:bookmarkStart w:id="1" w:name="target"/><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>7</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r><w:bookmarkEnd w:id="1"/></w:p>`)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{IncludeFieldInstructions: true})
	session := NewLayoutSession(1, 0)
	if err := source.Walk(session); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %#v, want one paragraph", doc)
	}
	p := doc.Paragraphs[0]
	if len(p.Bookmarks) != 1 || p.Bookmarks[0] != "target" {
		t.Fatalf("bookmarks = %#v, want target", p.Bookmarks)
	}
	if len(p.Runs) != 1 {
		t.Fatalf("runs = %#v, want one visible field-result run", p.Runs)
	}
	if p.Runs[0].Text != "7" || p.Runs[0].FieldKind != "PAGE" || p.Runs[0].FieldInstr != "PAGE" {
		t.Fatalf("field result = %#v, want PAGE result 7", p.Runs[0])
	}
}

func TestDOCXRawConsumerStillReceivesFieldAndBookmarkEvents(t *testing.T) {
	data := makeDOCXBodyFixture(t, `<w:p><w:bookmarkStart w:id="1" w:name="target"/><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>7</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r><w:bookmarkEnd w:id="1"/></w:p>`)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{IncludeFieldInstructions: true})
	var bookmarks, instructions, fieldChars int
	if err := source.Walk(HandlerFunc(func(event Event) error {
		switch event.Type {
		case EventBookmarkStart:
			bookmarks++
		case EventFieldInstruction:
			instructions++
		case EventFieldChar:
			fieldChars++
		}
		return nil
	})); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if bookmarks != 1 || instructions != 1 || fieldChars != 3 {
		t.Fatalf("events bookmark/instr/fieldChars = %d/%d/%d, want 1/1/3", bookmarks, instructions, fieldChars)
	}
}
