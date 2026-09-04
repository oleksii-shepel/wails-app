package edit

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestSectionCreateHeader(t *testing.T) {
	d := NewDocument()
	d.IR.Sections = append(d.IR.Sections, wordir.Section{})
	s := d.Sections()[0]

	se := s.CreateHeader("default")
	if se == nil {
		t.Fatal("expected story editor")
	}
	se.AddParagraph().SetText("header text")

	refs := s.HeaderRefs()
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want 1", len(refs))
	}
	if refs[0].Type != "default" {
		t.Fatalf("type = %q, want default", refs[0].Type)
	}

	// HeaderStory should return the same story
	got := s.HeaderStory("default")
	if got == nil {
		t.Fatal("expected header story")
	}
	if got.Paragraphs()[0].Text() != "header text" {
		t.Fatalf("text = %q", got.Paragraphs()[0].Text())
	}
}

func TestSectionCreateFooter(t *testing.T) {
	d := NewDocument()
	d.IR.Sections = append(d.IR.Sections, wordir.Section{})
	s := d.Sections()[0]

	se := s.CreateFooter("default")
	if se == nil {
		t.Fatal("expected story editor")
	}
	se.AddParagraph().SetText("footer text")

	refs := s.FooterRefs()
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want 1", len(refs))
	}
}

func TestSectionHeaderStoryMissing(t *testing.T) {
	d := NewDocument()
	d.IR.Sections = append(d.IR.Sections, wordir.Section{})
	s := d.Sections()[0]
	if s.HeaderStory("default") != nil {
		t.Fatal("expected nil")
	}
	if s.FooterStory("default") != nil {
		t.Fatal("expected nil")
	}
}

func TestSectionClearHeaders(t *testing.T) {
	d := NewDocument()
	d.IR.Sections = append(d.IR.Sections, wordir.Section{})
	s := d.Sections()[0]
	s.CreateHeader("default")
	s.CreateHeader("first")
	s.ClearHeaders()
	if len(s.HeaderRefs()) != 0 {
		t.Fatalf("refs = %d, want 0", len(s.HeaderRefs()))
	}
}

func TestSectionClearFooters(t *testing.T) {
	d := NewDocument()
	d.IR.Sections = append(d.IR.Sections, wordir.Section{})
	s := d.Sections()[0]
	s.CreateFooter("default")
	s.ClearFooters()
	if len(s.FooterRefs()) != 0 {
		t.Fatalf("refs = %d, want 0", len(s.FooterRefs()))
	}
}

func TestSectionCreateHeaderReplacesExisting(t *testing.T) {
	d := NewDocument()
	d.IR.Sections = append(d.IR.Sections, wordir.Section{})
	s := d.Sections()[0]
	se1 := s.CreateHeader("default")
	se1.AddParagraph().SetText("first")

	se2 := s.CreateHeader("default")
	se2.AddParagraph().SetText("second")

	refs := s.HeaderRefs()
	if len(refs) != 1 {
		t.Fatalf("refs = %d, want 1", len(refs))
	}
	// The document should now have the second story
	if s.HeaderStory("default").Paragraphs()[0].Text() != "second" {
		t.Fatal("expected second story")
	}
}