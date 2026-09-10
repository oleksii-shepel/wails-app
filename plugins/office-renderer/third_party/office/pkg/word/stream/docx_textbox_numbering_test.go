package stream

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestDOCXTextBoxParagraphPreservesDirectNumberingReference(t *testing.T) {
	const raw = `<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
		<w:pPr><w:pStyle w:val="ListParagraph"/><w:numPr><w:ilvl w:val="2"/><w:numId w:val="7"/></w:numPr><w:ind w:left="720" w:hanging="360"/></w:pPr>
		<w:r><w:t>Item</w:t></w:r>
	</w:p>`
	dec := xml.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil { t.Fatal(err) }
	p, err := parseDOCXTextBoxParagraph(dec, tok.(xml.StartElement))
	if err != nil { t.Fatal(err) }
	if p.Numbering == nil || p.Numbering.NumID != 7 || p.Numbering.Level != 2 {
		t.Fatalf("numbering = %#v, want numId=7 level=2", p.Numbering)
	}
	if p.Format.IndentLeft == nil || *p.Format.IndentLeft != 720 || p.Format.IndentHanging == nil || *p.Format.IndentHanging != 360 {
		t.Fatalf("paragraph indentation was not preserved: %#v", p.Format)
	}
}
