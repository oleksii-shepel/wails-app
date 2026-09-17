package sax

import (
	"strings"
	"testing"

	prel "office/pkg/presentation/parsers/ooxml/relationship"
)

func TestDecodeRelationshipsCanonicalizesStrictAndResolvesTargets(t *testing.T) {
	xml := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
		<Relationship Id="rId1" Type="` + prel.StrictOfficeBase + `slide" Target="slides/slide1.xml"/>
		<Relationship Id="rId2" Type="` + prel.StrictOfficeBase + `hyperlink" Target="https://example.com" TargetMode="External"/>
	</Relationships>`
	rels, err := decodeRelationships(strings.NewReader(xml), "ppt/presentation.xml")
	if err != nil {
		t.Fatal(err)
	}
	if got := rels["rId1"]; got.Type != prel.Slide || got.Target != "ppt/slides/slide1.xml" || got.External {
		t.Fatalf("internal relationship = %+v", got)
	}
	if got := rels["rId2"]; got.Target != "https://example.com" || !got.External {
		t.Fatalf("external relationship = %+v", got)
	}
}

func TestSAXRelationshipSemanticsUseSharedCanonicalPackage(t *testing.T) {
	if got := canonicalRelationshipType(prel.StrictOfficeBase + "notesSlide"); got != prel.NotesSlide {
		t.Fatalf("canonical relationship = %q, want %q", got, prel.NotesSlide)
	}
}
