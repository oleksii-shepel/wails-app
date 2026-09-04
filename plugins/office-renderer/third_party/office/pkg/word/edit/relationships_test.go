package edit

import (
	"strings"
	"testing"
)

func TestDocumentRelationships(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetHyperlink("https://example.com")

	rels, err := d.Relationships()
	if err != nil {
		t.Fatalf("Relationships: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("expected some relationships")
	}

	// Find the hyperlink relationship
	found := false
	for _, rel := range rels {
		if strings.Contains(rel.Type, "hyperlink") && rel.Target == "https://example.com" {
			found = true
			if rel.TargetMode != "External" {
				t.Fatalf("targetMode = %q, want External", rel.TargetMode)
			}
		}
	}
	if !found {
		t.Fatal("expected hyperlink relationship")
	}
}

func TestDocumentRelationshipsByType(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetHyperlink("https://example.com")

	const hyperlinkRel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
	rels, err := d.RelationshipsByType(hyperlinkRel)
	if err != nil {
		t.Fatalf("RelationshipsByType: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("rels = %d, want 1", len(rels))
	}
}

func TestDocumentRelationshipsNil(t *testing.T) {
	var d *Document
	rels, err := d.Relationships()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if rels != nil {
		t.Fatal("expected nil")
	}
}

func TestDocumentAddRelationship(t *testing.T) {
	d := NewDocument()
	// Add a custom relationship.
	d.AddRelationship(Relationship{
		SourcePart: "word/document.xml",
		ID:         "rId99",
		Type:       "http://example.com/custom",
		Target:     "custom.xml",
	})

	rels, err := d.Relationships()
	if err != nil {
		t.Fatalf("Relationships: %v", err)
	}
	var found bool
	for _, r := range rels {
		if r.ID == "rId99" && r.Target == "custom.xml" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected custom relationship to appear")
	}

	// Adding a duplicate should be ignored.
	d.AddRelationship(Relationship{
		SourcePart: "word/document.xml",
		ID:         "rId99",
		Type:       "http://example.com/custom",
		Target:     "custom.xml",
	})
	rels, err = d.Relationships()
	if err != nil {
		t.Fatalf("Relationships: %v", err)
	}
	count := 0
	for _, r := range rels {
		if r.ID == "rId99" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("custom rel count = %d, want 1", count)
	}
}

func TestDocumentRemoveRelationship(t *testing.T) {
	d := NewDocument()
	d.AddRelationship(Relationship{
		SourcePart: "word/document.xml",
		ID:         "rId99",
		Type:       "http://example.com/custom",
		Target:     "custom.xml",
	})
	d.RemoveRelationship("word/document.xml", "rId99")

	rels, err := d.Relationships()
	if err != nil {
		t.Fatalf("Relationships: %v", err)
	}
	for _, r := range rels {
		if r.ID == "rId99" {
			t.Fatal("expected custom relationship to be removed")
		}
	}
}

func TestDocumentAddRelationshipNilDocument(t *testing.T) {
	var d *Document
	// Should not panic.
	d.AddRelationship(Relationship{ID: "rId1", Type: "test", Target: "t.xml"})
}