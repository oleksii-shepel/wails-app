package sax

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestParagraphPropertiesPreserveGridAndContextualSpacing(t *testing.T) {
	const raw = `<w:pPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:snapToGrid w:val="0"/><w:contextualSpacing/></w:pPr>`
	dec := xml.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil { t.Fatal(err) }
	got, err := parseParagraphPropertiesRich(dec, tok.(xml.StartElement))
	if err != nil { t.Fatal(err) }
	if got.SnapToGrid == nil || *got.SnapToGrid {
		t.Fatalf("SnapToGrid = %v, want explicit false", got.SnapToGrid)
	}
	if !got.ContextualSpacing {
		t.Fatal("ContextualSpacing = false, want true")
	}
}

func TestRunPropertiesPreserveWordCharacterMetrics(t *testing.T) {
	const raw = `<w:rPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:b/><w:bCs/><w:spacing w:val="50"/><w:kern w:val="24"/><w:sz w:val="28"/><w:szCs w:val="30"/></w:rPr>`
	dec := xml.NewDecoder(strings.NewReader(raw))
	tok, err := dec.Token()
	if err != nil { t.Fatal(err) }
	got, err := parseRunProperties(dec, tok.(xml.StartElement))
	if err != nil { t.Fatal(err) }
	if got.Bold == nil || !*got.Bold || got.BoldCS == nil || !*got.BoldCS {
		t.Fatalf("bold metrics lost: %#v", got)
	}
	if got.CharacterSpacing == nil || *got.CharacterSpacing != 50 {
		t.Fatalf("CharacterSpacing = %v, want 50", got.CharacterSpacing)
	}
	if got.KerningHalfPoints == nil || *got.KerningHalfPoints != 24 {
		t.Fatalf("Kerning = %v, want 24", got.KerningHalfPoints)
	}
	if got.FontSizeCSHalfPoints == nil || *got.FontSizeCSHalfPoints != 30 {
		t.Fatalf("FontSizeCS = %v, want 30", got.FontSizeCSHalfPoints)
	}
}
