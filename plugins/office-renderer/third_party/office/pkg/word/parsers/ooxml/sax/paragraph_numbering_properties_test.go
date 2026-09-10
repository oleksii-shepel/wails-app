package sax

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestParagraphPropertiesRichCarriesNumbering(t *testing.T) {
	const raw = `<w:pPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:numPr><w:ilvl w:val="1"/><w:numId w:val="4"/></w:numPr></w:pPr>`
	dec := xml.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil { t.Fatal(err) }
	props, err := parseParagraphPropertiesRich(dec, tok.(xml.StartElement))
	if err != nil { t.Fatal(err) }
	if props.NumberingID == nil || *props.NumberingID != 4 || props.NumberingLevel == nil || *props.NumberingLevel != 1 {
		t.Fatalf("numbering = id:%v level:%v", props.NumberingID, props.NumberingLevel)
	}
}
