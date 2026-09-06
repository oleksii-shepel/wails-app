package stream

import (
	"archive/zip"
	"bytes"
	"testing"
)

func makeDOCXStyleFixture(t *testing.T) []byte {
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
	write("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`)
	write("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)
	write("word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`)
	write("word/styles.xml", `<?xml version="1.0" encoding="UTF-8"?><w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docDefaults><w:rPrDefault><w:rPr><w:sz w:val="20"/></w:rPr></w:rPrDefault></w:docDefaults><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:pPr><w:jc w:val="center"/></w:pPr><w:rPr><w:b/></w:rPr></w:style><w:style w:type="character" w:styleId="Emphasis"><w:name w:val="Emphasis"/><w:rPr><w:i/></w:rPr></w:style></w:styles>`)
	write("word/document.xml", `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:pPr><w:pStyle w:val="Normal"/></w:pPr><w:r><w:rPr><w:rStyle w:val="Emphasis"/><w:color w:val="FF0000"/></w:rPr><w:t>Styled</w:t></w:r></w:p></w:body></w:document>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDOCXSourceResolvesCompactStyleMetadata(t *testing.T) {
	data := makeDOCXStyleFixture(t)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	session := NewLayoutSession(1, 0)
	if err := source.Walk(session); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) != 1 {
		t.Fatalf("document = %#v", doc)
	}
	if doc.DefaultParagraphStyleID != "Normal" || len(doc.Styles) == 0 || len(doc.CharacterStyles) == 0 {
		t.Fatalf("metadata default=%q styles=%d charStyles=%d", doc.DefaultParagraphStyleID, len(doc.Styles), len(doc.CharacterStyles))
	}
	p := doc.Paragraphs[0]
	if p.Format.Alignment != "center" {
		t.Fatalf("paragraph alignment = %q", p.Format.Alignment)
	}
	if len(p.Runs) != 1 {
		t.Fatalf("runs = %#v", p.Runs)
	}
	run := p.Runs[0]
	if run.Format.Bold == nil || !*run.Format.Bold {
		t.Fatalf("run bold = %v", run.Format.Bold)
	}
	if run.Format.Italic == nil || !*run.Format.Italic {
		t.Fatalf("run italic = %v", run.Format.Italic)
	}
	if run.Format.Color != "FF0000" {
		t.Fatalf("run color = %q", run.Format.Color)
	}
	if run.Format.FontSize == nil || *run.Format.FontSize != 20 {
		t.Fatalf("run font size = %v", run.Format.FontSize)
	}
}
