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

func TestParseParagraphPropertiesUniversalMeasures(t *testing.T) {
	// The strict vocabulary writes measurement attributes as ST_UniversalMeasure
	// values, so a suffixed length must resolve to twips instead of being
	// dropped. Half-point sizes share the attribute reader and must not be
	// rescaled as if they were twips.
	dec, start := decodeTestElement(t, `<w:pPr xmlns:w="http://purl.oclc.org/ooxml/wordprocessingml/main"><w:ind w:start="54pt" w:end="0.5in" w:firstLine="36pt"/><w:spacing w:before="10pt" w:after="30pt" w:line="18pt"/></w:pPr>`)
	got, err := parseParagraphPropertiesRich(dec, start)
	if err != nil {
		t.Fatal(err)
	}
	if got.IndentLeft == nil || *got.IndentLeft != 1080 {
		t.Fatalf("indent left = %v, want 1080 twips", got.IndentLeft)
	}
	if got.IndentRight == nil || *got.IndentRight != 720 {
		t.Fatalf("indent right = %v, want 720 twips", got.IndentRight)
	}
	if got.IndentFirstLine == nil || *got.IndentFirstLine != 720 {
		t.Fatalf("first line indent = %v, want 720 twips", got.IndentFirstLine)
	}
	if got.SpacingBefore == nil || *got.SpacingBefore != 200 || got.SpacingAfter == nil || *got.SpacingAfter != 600 {
		t.Fatalf("spacing = %v/%v, want 200/600 twips", got.SpacingBefore, got.SpacingAfter)
	}
	if got.LineSpacing == nil || *got.LineSpacing != 360 {
		t.Fatalf("line spacing = %v, want 360 twips", got.LineSpacing)
	}

	dec, start = decodeTestElement(t, `<w:rPr xmlns:w="http://purl.oclc.org/ooxml/wordprocessingml/main"><w:sz w:val="12pt"/></w:rPr>`)
	run, err := parseRunProperties(dec, start)
	if err != nil {
		t.Fatal(err)
	}
	if run.FontSizeHalfPoints != nil {
		t.Fatalf("font size = %v, half-point measures must not be rescaled as twips", *run.FontSizeHalfPoints)
	}
}

func TestParseSectionPropertiesUniversalMeasures(t *testing.T) {
	dec, start := decodeTestElement(t, `<w:sectPr xmlns:w="http://purl.oclc.org/ooxml/wordprocessingml/main"><w:pgSz w:w="612pt" w:h="792pt"/><w:pgMar w:top="72pt" w:right="90pt" w:bottom="72pt" w:left="90pt"/><w:cols w:num="1" w:space="36pt"/></w:sectPr>`)
	got, err := parseSectionProperties(dec, start, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.PageWidth == nil || *got.PageWidth != 12240 || got.PageHeight == nil || *got.PageHeight != 15840 {
		t.Fatalf("page size = %v x %v, want 12240 x 15840 twips", got.PageWidth, got.PageHeight)
	}
	if got.MarginLeft == nil || *got.MarginLeft != 1800 || got.MarginRight == nil || *got.MarginRight != 1800 {
		t.Fatalf("page margins = %v/%v, want 1800 twips", got.MarginLeft, got.MarginRight)
	}
	if got.ColumnSpace == nil || *got.ColumnSpace != 720 {
		t.Fatalf("column space = %v, want 720 twips", got.ColumnSpace)
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
