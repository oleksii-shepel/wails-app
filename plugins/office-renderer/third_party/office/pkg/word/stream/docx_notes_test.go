package stream

import (
	"archive/zip"
	"bytes"
	"testing"
)

func makeDOCXNotesFixture(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	write("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/footnotes.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml"/><Override PartName="/word/endnotes.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.endnotes+xml"/></Types>`)
	write("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)
	write("word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rFoot" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes" Target="footnotes.xml"/><Relationship Id="rEnd" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes" Target="endnotes.xml"/></Relationships>`)
	write("word/document.xml", `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Body</w:t></w:r><w:r><w:footnoteReference w:id="1"/></w:r><w:r><w:endnoteReference w:id="2"/></w:r></w:p></w:body></w:document>`)
	write("word/footnotes.xml", `<?xml version="1.0" encoding="UTF-8"?><w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:footnote w:id="-1" w:type="separator"><w:p><w:r><w:t>sep</w:t></w:r></w:p></w:footnote><w:footnote w:id="1"><w:p><w:pPr><w:spacing w:after="120"/></w:pPr><w:r><w:rPr><w:i/></w:rPr><w:t>Footnote text</w:t></w:r></w:p></w:footnote></w:footnotes>`)
	write("word/endnotes.xml", `<?xml version="1.0" encoding="UTF-8"?><w:endnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:endnote w:id="2"><w:p><w:r><w:t>Endnote text</w:t></w:r></w:p></w:endnote></w:endnotes>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDOCXLayoutLazilyLoadsReferencedNotes(t *testing.T) {
	data := makeDOCXNotesFixture(t)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	session := NewLayoutSession(1, 0)
	if err := source.Walk(session); err != nil {
		t.Fatalf("Walk: %v", err)
	}

	doc := session.Document()
	if doc == nil {
		t.Fatal("Document = nil")
	}
	footnote, ok := doc.Footnotes[1]
	if !ok || len(footnote.Paragraphs) != 1 || footnote.Paragraphs[0].Text != "Footnote text" {
		t.Fatalf("footnote[1] = %#v, want one parsed paragraph", footnote)
	}
	if len(footnote.Paragraphs[0].Runs) != 1 || footnote.Paragraphs[0].Runs[0].Format.Italic == nil || !*footnote.Paragraphs[0].Runs[0].Format.Italic {
		t.Fatalf("footnote run formatting = %#v, want italic", footnote.Paragraphs[0].Runs)
	}
	endnote, ok := doc.Endnotes[2]
	if !ok || len(endnote.Paragraphs) != 1 || endnote.Paragraphs[0].Text != "Endnote text" {
		t.Fatalf("endnote[2] = %#v, want one parsed paragraph", endnote)
	}

	if len(doc.Paragraphs) != 1 {
		t.Fatalf("body paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	var sawFootnoteRef, sawEndnoteRef bool
	for _, run := range doc.Paragraphs[0].Runs {
		if run.Kind == "footnoteRef" && run.NoteID == 1 {
			sawFootnoteRef = true
		}
		if run.Kind == "endnoteRef" && run.NoteID == 2 {
			sawEndnoteRef = true
		}
	}
	if !sawFootnoteRef || !sawEndnoteRef {
		t.Fatalf("body runs = %#v, want footnote/endnote references", doc.Paragraphs[0].Runs)
	}

	pages := session.Pages()
	if len(pages) == 0 {
		t.Fatal("Pages = empty")
	}
	if len(pages[0].Footnotes) == 0 {
		t.Fatalf("page 1 footnotes = %#v, want materialized footnote boxes", pages[0].Footnotes)
	}
}
