package sax

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestParseParagraphPropertiesRichPreservesSnapToGridFalse(t *testing.T) {
	dec := xml.NewDecoder(strings.NewReader(`<w:pPr xmlns:w="urn:w"><w:snapToGrid w:val="0"/></w:pPr>`))
	tok, err := dec.Token()
	if err != nil {
		t.Fatal(err)
	}
	props, err := parseParagraphPropertiesRich(dec, tok.(xml.StartElement))
	if err != nil {
		t.Fatal(err)
	}
	if props.SnapToGrid == nil || *props.SnapToGrid {
		t.Fatalf("SnapToGrid = %#v, want explicit false", props.SnapToGrid)
	}
}

func TestParseSectionPropertiesPreservesDocGridWithoutType(t *testing.T) {
	dec := xml.NewDecoder(strings.NewReader(`<w:sectPr xmlns:w="urn:w"><w:docGrid w:linePitch="360" w:charSpace="12"/></w:sectPr>`))
	tok, err := dec.Token()
	if err != nil {
		t.Fatal(err)
	}
	props, err := parseSectionProperties(dec, tok.(xml.StartElement), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if props.DocGridLinePitch == nil || *props.DocGridLinePitch != 360 {
		t.Fatalf("linePitch = %#v", props.DocGridLinePitch)
	}
	if props.DocGridCharSpace == nil || *props.DocGridCharSpace != 12 {
		t.Fatalf("charSpace = %#v", props.DocGridCharSpace)
	}
}
