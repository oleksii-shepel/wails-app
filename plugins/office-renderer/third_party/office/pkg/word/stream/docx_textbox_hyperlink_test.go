package stream

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestDOCXTextBoxParagraphPreservesHyperlinkRuns(t *testing.T) {
	const raw = `<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
		<w:hyperlink r:id="rId9"><w:r><w:rPr><w:sz w:val="18"/></w:rPr><w:t>@oleksiishepel</w:t></w:r></w:hyperlink>
	</w:p>`
	dec := xml.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil { t.Fatal(err) }
	start := tok.(xml.StartElement)
	p, err := parseDOCXTextBoxParagraph(dec, start)
	if err != nil { t.Fatal(err) }
	if p.Text != "@oleksiishepel" {
		t.Fatalf("textbox hyperlink text = %q, want @oleksiishepel", p.Text)
	}
	if len(p.Runs) != 1 || p.Runs[0].Text != "@oleksiishepel" {
		t.Fatalf("textbox hyperlink runs = %#v", p.Runs)
	}
}
