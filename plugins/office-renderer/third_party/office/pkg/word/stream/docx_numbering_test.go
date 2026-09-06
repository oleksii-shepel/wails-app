package stream

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestDOCXStreamResolvesDirectAndStyleNumbering(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil { t.Fatal(err) }
		if _, err := w.Write([]byte(content)); err != nil { t.Fatal(err) }
	}
	write("[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`)
	write("_rels/.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)
	write("word/_rels/document.xml.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/><Relationship Id="rNum" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/></Relationships>`)
	write("word/styles.xml", `<?xml version="1.0"?><w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:style w:type="paragraph" w:styleId="ListStyle"><w:name w:val="List Style"/><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr></w:style></w:styles>`)
	write("word/numbering.xml", `<?xml version="1.0"?><w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:abstractNum w:abstractNumId="10"><w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/></w:lvl></w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="10"/></w:num><w:num w:numId="2"><w:abstractNumId w:val="10"/></w:num></w:numbering>`)
	write("word/document.xml", `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:pPr><w:pStyle w:val="ListStyle"/></w:pPr><w:r><w:t>style item</w:t></w:r></w:p><w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="2"/></w:numPr></w:pPr><w:r><w:t>direct item</w:t></w:r></w:p></w:body></w:document>`)
	if err := zw.Close(); err != nil { t.Fatal(err) }

	data := buf.Bytes()
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	session := NewLayoutSession(1, 0)
	if err := source.Walk(session); err != nil { t.Fatalf("Walk: %v", err) }
	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) != 2 { t.Fatalf("paragraphs = %#v", doc) }
	for i, wantID := range []int{1, 2} {
		p := doc.Paragraphs[i]
		if p.Numbering == nil || p.Numbering.NumID != wantID || p.Numbering.Level != 0 {
			t.Fatalf("paragraph %d numbering = %#v, want numID=%d level=0", i, p.Numbering, wantID)
		}
		if p.List == nil || p.List.Format != "bullet" {
			t.Fatalf("paragraph %d list = %#v, want bullet marker", i, p.List)
		}
	}
}
