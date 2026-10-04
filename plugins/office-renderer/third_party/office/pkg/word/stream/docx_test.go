package stream

import (
	"archive/zip"
	"bytes"
	"testing"
)

func makeDOCXStreamFixture(t *testing.T, text string) []byte {
	return makeDOCXBodyFixture(t, `<w:p><w:r><w:t>`+text+`</w:t></w:r></w:p>`)
}

func makeDOCXBodyFixture(t *testing.T, body string) []byte {
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
	write("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`)
	write("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)
	write("word/document.xml", `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`+body+`</w:body></w:document>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDOCXSourceNormalizesSAXEvents(t *testing.T) {
	data := makeDOCXStreamFixture(t, "hello stream")
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	var events []Event
	if err := source.Walk(HandlerFunc(func(event Event) error {
		events = append(events, event)
		return nil
	})); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	var sawText bool
	for _, event := range events {
		if event.Type == EventText && event.Format == FormatDOCX && event.Text == "hello stream" {
			sawText = true
		}
	}
	if !sawText {
		t.Fatalf("events = %#v; expected normalized DOCX text event", events)
	}
}

func TestDOCXSourceStreamsTableIntoLayoutSession(t *testing.T) {
	data := makeDOCXBodyFixture(t, `<w:tbl><w:tr><w:tc><w:p><w:r><w:t>left</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>right</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	session := NewLayoutSession(1, 0)
	if err := source.Walk(session); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	doc := session.Document()
	if doc == nil || len(doc.Blocks) != 1 || doc.Blocks[0].Table == nil {
		t.Fatalf("document blocks = %#v, want one table", doc)
	}
	table := doc.Blocks[0].Table
	if len(table.Rows) != 1 || len(table.Rows[0].Cells) != 2 {
		t.Fatalf("table = %#v, want one row/two cells", table)
	}
	if table.Rows[0].Cells[0].Text != "left" || table.Rows[0].Cells[1].Text != "right" {
		t.Fatalf("cell texts = %q / %q, want left / right", table.Rows[0].Cells[0].Text, table.Rows[0].Cells[1].Text)
	}
}

func TestDOCXSourcePromotesFormattingAndSectionsIntoLayoutSession(t *testing.T) {
	body := `<w:p>` +
		`<w:pPr><w:pStyle w:val="Heading1"/><w:keepNext/><w:keepLines/><w:jc w:val="center"/><w:ind w:left="720"/><w:spacing w:before="120" w:after="240"/>` +
		`<w:sectPr><w:type w:val="nextPage"/><w:pgSz w:w="10000" w:h="14000"/><w:pgMar w:top="720" w:right="800" w:bottom="900" w:left="1000"/></w:sectPr></w:pPr>` +
		`<w:r><w:rPr><w:rStyle w:val="Emphasis"/><w:b/><w:i/><w:color w:val="336699"/><w:rFonts w:ascii="Aptos"/><w:sz w:val="28"/></w:rPr><w:t>styled</w:t></w:r>` +
		`</w:p>` +
		`<w:p><w:r><w:t>second section</w:t></w:r></w:p>` +
		`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720"/><w:cols w:num="2" w:space="360"/><w:titlePg/></w:sectPr>`
	data := makeDOCXBodyFixture(t, body)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	session := NewLayoutSession(1, 0)
	if err := source.Walk(session); err != nil {
		t.Fatalf("Walk: %v", err)
	}

	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %#v, want two", doc)
	}
	p := doc.Paragraphs[0]
	if p.StyleID != "Heading1" || !p.Format.KeepNext || !p.Format.KeepLines || p.Format.Alignment != "center" {
		t.Fatalf("paragraph = %#v, formatting was not promoted", p)
	}
	if p.Format.IndentLeft == nil || *p.Format.IndentLeft != 720 || p.Format.SpacingAfter == nil || *p.Format.SpacingAfter != 240 {
		t.Fatalf("paragraph format = %#v, want indent/spacing", p.Format)
	}
	if len(p.Runs) != 1 {
		t.Fatalf("runs = %#v, want one", p.Runs)
	}
	run := p.Runs[0]
	if run.StyleID != "Emphasis" || run.Format.Bold == nil || !*run.Format.Bold || run.Format.Italic == nil || !*run.Format.Italic {
		t.Fatalf("run = %#v, style/bold/italic were not promoted", run)
	}
	if run.Format.Color != "336699" || run.Format.FontASCII != "Aptos" || run.Format.FontSize == nil || *run.Format.FontSize != 28 {
		t.Fatalf("run format = %#v, want color/font/size", run.Format)
	}

	if len(doc.Sections) != 2 {
		t.Fatalf("sections = %#v, want inline and final sections", doc.Sections)
	}
	first := doc.Sections[0]
	if first.Type != "nextPage" || first.PageSize == nil || first.PageSize.Width != 10000 || first.PageMargin == nil || first.PageMargin.Left != 1000 {
		t.Fatalf("first section = %#v", first)
	}
	last := doc.Sections[1]
	if last.PageSize == nil || last.PageSize.Width != 12240 || last.Columns == nil || last.Columns.Num != 2 || last.Columns.Space != 360 || !last.TitlePage {
		t.Fatalf("final section = %#v", last)
	}
}

func TestDOCXSourcePromotesParagraphMarkRunProperties(t *testing.T) {
	body := `<w:p><w:pPr><w:rPr><w:rFonts w:ascii="Cambria"/><w:sz w:val="32"/></w:rPr></w:pPr></w:p>` +
		`<w:sectPr><w:pgSz w:w="12240" w:h="15840"/></w:sectPr>`
	data := makeDOCXBodyFixture(t, body)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	session := NewLayoutSession(1, 0)
	if err := source.Walk(session); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %#v, want one", doc)
	}
	mark := doc.Paragraphs[0].DefaultRunFormat
	if mark == nil || mark.FontASCII != "Cambria" || mark.FontSize == nil || *mark.FontSize != 32 {
		t.Fatalf("paragraph mark = %#v, want 16pt Cambria from w:pPr/w:rPr", mark)
	}
}

func TestDOCXPaginateStopsBeforeLaterUnsupportedContent(t *testing.T) {
	body := `<w:p><w:r><w:t>page one</w:t><w:br w:type="page"/></w:r></w:p>` +
		`<w:p><w:r><w:fldChar w:fldCharType="begin"/><w:t>must not be read</w:t></w:r></w:p>`
	data := makeDOCXBodyFixture(t, body)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	pages, err := Paginate(source, 1, 1)
	if err != nil {
		t.Fatalf("Paginate page 1: %v", err)
	}
	if len(pages) != 1 || pages[0].Number != 1 {
		t.Fatalf("pages = %#v, want physical page 1 only", pages)
	}
}
