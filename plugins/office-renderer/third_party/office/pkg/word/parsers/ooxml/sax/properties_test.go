package sax

import (
	"encoding/xml"
	"strings"
	"testing"
)

func decodeTestElement(t *testing.T, body string) (*xml.Decoder, xml.StartElement) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(body))
	for {
		tok, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			return dec, start
		}
	}
}

func TestParseParagraphPropertiesRich(t *testing.T) {
	dec, start := decodeTestElement(t, `<w:pPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:pStyle w:val="Body"/><w:jc w:val="center"/><w:ind w:left="720" w:right="360" w:firstLine="240"/><w:spacing w:before="120" w:after="240" w:line="360" w:lineRule="auto"/><w:keepNext/><w:keepLines/><w:pageBreakBefore/><w:widowControl w:val="0"/></w:pPr>`)
	got, err := parseParagraphPropertiesRich(dec, start)
	if err != nil {
		t.Fatal(err)
	}
	if got.StyleID != "Body" || got.Alignment != "center" || !got.KeepNext || !got.KeepLines || !got.PageBreakBefore {
		t.Fatalf("paragraph properties = %#v", got)
	}
	if got.IndentLeft == nil || *got.IndentLeft != 720 || got.SpacingAfter == nil || *got.SpacingAfter != 240 {
		t.Fatalf("paragraph dimensions = %#v", got)
	}
	if got.WidowControl == nil || *got.WidowControl {
		t.Fatalf("widow control = %#v, want explicit false", got.WidowControl)
	}
}

func TestParseRunProperties(t *testing.T) {
	dec, start := decodeTestElement(t, `<w:rPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:rStyle w:val="Emphasis"/><w:rFonts w:ascii="Aptos"/><w:b/><w:i w:val="0"/><w:u w:val="double"/><w:color w:val="112233"/><w:sz w:val="24"/></w:rPr>`)
	got, err := parseRunProperties(dec, start)
	if err != nil {
		t.Fatal(err)
	}
	if got.StyleID != "Emphasis" || got.FontFamily != "Aptos" || got.Bold == nil || !*got.Bold || got.Italic == nil || *got.Italic {
		t.Fatalf("run properties = %#v", got)
	}
	if got.Underline != "double" || got.Color != "112233" || got.FontSizeHalfPoints == nil || *got.FontSizeHalfPoints != 24 {
		t.Fatalf("run presentation = %#v", got)
	}
}

func TestParseSectionProperties(t *testing.T) {
	dec, start := decodeTestElement(t, `<w:sectPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:headerReference r:id="rId1"/><w:footerReference r:id="rId2"/><w:type w:val="nextPage"/><w:pgSz w:w="12240" w:h="15840"/><w:pgMar w:top="1440" w:right="1080" w:bottom="1440" w:left="1080" w:header="720" w:footer="720"/><w:cols w:num="2" w:space="720"/><w:titlePg/></w:sectPr>`)
	var headerID, footerID string
	got, err := parseSectionProperties(dec, start, func(id string) { headerID = id }, func(id string) { footerID = id })
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "nextPage" || got.PageWidth == nil || *got.PageWidth != 12240 || got.Columns == nil || *got.Columns != 2 || !got.TitlePage {
		t.Fatalf("section properties = %#v", got)
	}
	if headerID != "rId1" || footerID != "rId2" {
		t.Fatalf("section refs = %q/%q", headerID, footerID)
	}
}
