package stream

import (
	"archive/zip"
	"bytes"
	"fmt"
	"testing"
)

func TestDOCXSourceMaterializesCommentReference(t *testing.T) {
	data := makeDOCXCommentFixture(t)
	session := NewLayoutSession(1, 0)
	if err := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{}).Walk(session); err != nil {
		t.Fatal(err)
	}
	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) == 0 {
		t.Fatal("missing streamed paragraph")
	}
	paragraph := doc.Paragraphs[0]
	foundRef := false
	for _, run := range paragraph.Runs {
		if run.Kind == "commentRef" && run.CommentID == 7 {
			foundRef = true
			break
		}
	}
	if !foundRef {
		t.Fatalf("runs = %#v, want commentRef id 7", paragraph.Runs)
	}
	story, ok := doc.Comments[7]
	if !ok {
		t.Fatal("comment 7 not materialized")
	}
	if story.Text != "review this" || story.Author != "Ada" {
		t.Fatalf("comment story = %#v", story)
	}
	if story.Date == "" {
		t.Fatal("comment date not preserved")
	}
}

func TestDOCXSourcePreservesDanglingCommentReference(t *testing.T) {
	data := makeDOCXCommentFixtureWithIDs(t, 7, 1)
	session := NewLayoutSession(1, 0)
	if err := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{}).Walk(session); err != nil {
		t.Fatal(err)
	}
	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) == 0 {
		t.Fatal("missing streamed paragraph")
	}
	foundRef := false
	for _, run := range doc.Paragraphs[0].Runs {
		if run.Kind == "commentRef" && run.CommentID == 7 {
			foundRef = true
			break
		}
	}
	if !foundRef {
		t.Fatalf("runs = %#v, want dangling commentRef id 7", doc.Paragraphs[0].Runs)
	}
	if _, ok := doc.Comments[7]; ok {
		t.Fatal("dangling comment reference unexpectedly materialized a story")
	}
}

func makeDOCXCommentFixture(t *testing.T) []byte {
	return makeDOCXCommentFixtureWithIDs(t, 7, 7)
}

func makeDOCXCommentFixtureWithIDs(t *testing.T, referenceID, storyID int) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/comments.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.comments+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdComments" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="comments.xml"/></Relationships>`,
		"word/document.xml": fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>body</w:t></w:r><w:r><w:commentReference w:id="%d"/></w:r></w:p><w:sectPr/></w:body></w:document>`, referenceID),
		"word/comments.xml": fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><w:comments xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:comment w:id="%d" w:author="Ada" w:date="2026-09-05T08:00:00Z"><w:p><w:r><w:t>review this</w:t></w:r></w:p></w:comment></w:comments>`, storyID),
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
