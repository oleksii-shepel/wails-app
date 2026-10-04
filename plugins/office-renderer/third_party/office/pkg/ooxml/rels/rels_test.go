package rels_test

import (
	"testing"

	"office/pkg/ooxml/rels"
)

func TestParseRelationshipsAndLookup(t *testing.T) {
	doc, err := rels.Parse([]byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="type/styles" Target="styles.xml"/>
  <Relationship Id="rId2" Type="type/hyperlink" Target="https://example.com" TargetMode="External"/>
</Relationships>`))
	if err != nil {
		t.Fatalf("parse relationships: %v", err)
	}

	if got := len(doc.Relationships); got != 2 {
		t.Fatalf("relationship count = %d, want 2", got)
	}
	rel, ok := doc.ByID("rId2")
	if !ok {
		t.Fatal("expected rId2 lookup to succeed")
	}
	if !rel.IsExternal() {
		t.Fatal("expected hyperlink relationship to be external")
	}
}

func TestParseCanonicalizesStrictRelationshipTypes(t *testing.T) {
	doc, err := rels.Parse([]byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://purl.oclc.org/ooxml/officeDocument/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rId2" Type="http://purl.oclc.org/ooxml/officeDocument/relationships/image" Target="media/image1.png"/>
</Relationships>`))
	if err != nil {
		t.Fatalf("parse relationships: %v", err)
	}
	if got, want := doc.Relationships[0].Type, "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"; got != want {
		t.Fatalf("officeDocument relationship type = %q, want %q", got, want)
	}
	if got, want := doc.Relationships[1].Type, "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"; got != want {
		t.Fatalf("image relationship type = %q, want %q", got, want)
	}
}

func TestRelationshipsPartName(t *testing.T) {
	cases := map[string]string{
		"":                      "_rels/.rels",
		"word/document.xml":     "word/_rels/document.xml.rels",
		"ppt/slides/slide1.xml": "ppt/slides/_rels/slide1.xml.rels",
		"doc.xml":               "_rels/doc.xml.rels",
	}
	for sourcePart, want := range cases {
		if got := rels.RelationshipsPartName(sourcePart); got != want {
			t.Fatalf("RelationshipsPartName(%q) = %q, want %q", sourcePart, got, want)
		}
	}
}

func TestResolveTarget(t *testing.T) {
	cases := []struct {
		sourcePart string
		target     string
		want       string
	}{
		{"word/document.xml", "styles.xml", "word/styles.xml"},
		{"word/document.xml", "../custom/item1.xml", "custom/item1.xml"},
		{"word/document.xml", "/word/theme/theme1.xml", "word/theme/theme1.xml"},
		{"", "word/document.xml", "word/document.xml"},
	}
	for _, tc := range cases {
		if got := rels.ResolveTarget(tc.sourcePart, tc.target); got != tc.want {
			t.Fatalf("ResolveTarget(%q, %q) = %q, want %q", tc.sourcePart, tc.target, got, tc.want)
		}
	}
}
