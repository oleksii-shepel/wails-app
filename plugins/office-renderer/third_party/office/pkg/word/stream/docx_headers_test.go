package stream

import (
	"archive/zip"
	"bytes"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestDOCXSourceLoadsAndInheritsSectionHeadersFooters(t *testing.T) {
	data := makeDOCXHeaderFooterFixture(t)
	session := NewLayoutSession(1, 0)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	if err := source.Walk(session); err != nil {
		t.Fatal(err)
	}

	doc := session.Document()
	if doc == nil {
		t.Fatal("nil document")
	}
	if len(doc.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(doc.Sections))
	}
	if !doc.EvenAndOddHeaders {
		t.Fatal("EvenAndOddHeaders = false, want true")
	}
	if got := doc.Headers["word/header1.xml"].Text; got != "default header" {
		t.Fatalf("default header text = %q", got)
	}
	if got := doc.Headers["word/header2.xml"].Text; got != "even header" {
		t.Fatalf("even header text = %q", got)
	}
	if got := doc.Headers["word/header3.xml"].Text; got != "first header" {
		t.Fatalf("first header text = %q", got)
	}
	if got := doc.Footers["word/footer1.xml"].Text; got != "default footer" {
		t.Fatalf("footer text = %q", got)
	}

	firstHeaders := refsByType(doc.Sections[0].Headers)
	if firstHeaders["default"] != "word/header1.xml" || firstHeaders["even"] != "word/header2.xml" {
		t.Fatalf("section 1 headers = %#v", firstHeaders)
	}
	secondHeaders := refsByType(doc.Sections[1].Headers)
	if secondHeaders["default"] != "word/header1.xml" || secondHeaders["even"] != "word/header2.xml" || secondHeaders["first"] != "word/header3.xml" {
		t.Fatalf("section 2 inherited headers = %#v", secondHeaders)
	}
	secondFooters := refsByType(doc.Sections[1].Footers)
	if secondFooters["default"] != "word/footer1.xml" {
		t.Fatalf("section 2 inherited footers = %#v", secondFooters)
	}
	if !doc.Sections[1].TitlePage {
		t.Fatal("section 2 TitlePage = false, want true")
	}
}

func TestDOCXSourcePreservesHeaderLineSpacing(t *testing.T) {
	data := makeDOCXHeaderFooterFixture(t)
	session := NewLayoutSession(1, 0)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	if err := source.Walk(session); err != nil {
		t.Fatal(err)
	}

	header, ok := session.Document().Headers["word/header1.xml"]
	if !ok || len(header.Blocks) != 1 || header.Blocks[0].Paragraph == nil {
		t.Fatalf("header blocks = %#v", header)
	}
	format := header.Blocks[0].Paragraph.Format
	if format.LineSpacing == nil || *format.LineSpacing != 480 || format.LineRule != "auto" {
		t.Fatalf("header line spacing = %#v, want 480/auto", format)
	}
	styledHeader, ok := session.Document().Headers["word/header2.xml"]
	if !ok || len(styledHeader.Blocks) != 1 || styledHeader.Blocks[0].Paragraph == nil {
		t.Fatalf("styled header blocks = %#v", styledHeader)
	}
	styledFormat := styledHeader.Blocks[0].Paragraph.Format
	if styledFormat.LineSpacing == nil || *styledFormat.LineSpacing != 360 || styledFormat.LineRule != "exact" {
		t.Fatalf("styled header line spacing = %#v, want 360/exact", styledFormat)
	}
}

func refsByType(refs []wordir.Reference) map[string]string {
	out := make(map[string]string, len(refs))
	for _, ref := range refs {
		out[ref.Type] = ref.PartName
	}
	return out
}

func makeDOCXHeaderFooterFixture(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>
  <Override PartName="/word/header2.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>
  <Override PartName="/word/header3.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>
  <Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>
  <Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>
	  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rIdH1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>
  <Relationship Id="rIdH2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header2.xml"/>
  <Relationship Id="rIdH3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header3.xml"/>
  <Relationship Id="rIdF1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>
  <Relationship Id="rIdSettings" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>
	  <Relationship Id="rIdStyles" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:pPr><w:sectPr>
      <w:headerReference w:type="default" r:id="rIdH1"/>
      <w:headerReference w:type="even" r:id="rIdH2"/>
      <w:footerReference w:type="default" r:id="rIdF1"/>
      <w:type w:val="nextPage"/>
    </w:sectPr></w:pPr><w:r><w:t>section one</w:t></w:r></w:p>
    <w:p><w:r><w:t>section two</w:t></w:r></w:p>
    <w:sectPr>
      <w:headerReference w:type="first" r:id="rIdH3"/>
      <w:titlePg/>
    </w:sectPr>
  </w:body>
</w:document>`,
		"word/settings.xml": `<?xml version="1.0" encoding="UTF-8"?><w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:evenAndOddHeaders/></w:settings>`,
		"word/styles.xml":   `<?xml version="1.0" encoding="UTF-8"?><w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:style w:type="paragraph" w:styleId="HeaderLine"><w:name w:val="Header line"/><w:pPr><w:spacing w:line="360" w:lineRule="exact"/></w:pPr></w:style></w:styles>`,
		"word/header1.xml":  `<?xml version="1.0" encoding="UTF-8"?><w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:pPr><w:spacing w:line="480" w:lineRule="auto"/></w:pPr><w:r><w:t>default header</w:t></w:r></w:p></w:hdr>`,
		"word/header2.xml":  `<?xml version="1.0" encoding="UTF-8"?><w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:pPr><w:pStyle w:val="HeaderLine"/></w:pPr><w:r><w:t>even header</w:t></w:r></w:p></w:hdr>`,
		"word/header3.xml":  storyXML("hdr", "first header"),
		"word/footer1.xml":  storyXML("ftr", "default footer"),
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

func storyXML(root, text string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><w:` + root + ` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:` + root + `>`
}
