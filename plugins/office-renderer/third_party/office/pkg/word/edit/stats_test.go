package edit

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestDocumentStatistics(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p1.SetText("Hello world")
	r1 := p1.AddRun()
	r1.SetText(" more text")

	p2 := d.AddParagraph()
	p2.SetText("Second paragraph.")

	d.AddTable()

	p3 := d.AddParagraph()
	p3.AddRun().SetText("")
	p3.AddRun().ir().Image = &wordir.Image{}

	s := d.Statistics()
	if s.Paragraphs != 3 {
		t.Fatalf("paragraphs = %d, want 3", s.Paragraphs)
	}
	if s.Tables != 1 {
		t.Fatalf("tables = %d, want 1", s.Tables)
	}
	if s.Runs != 5 {
		t.Fatalf("runs = %d, want 5", s.Runs)
	}
	if s.Words != 6 {
		t.Fatalf("words = %d, want 6", s.Words)
	}
	if s.Images != 1 {
		t.Fatalf("images = %d, want 1", s.Images)
	}
	if s.Chars == 0 {
		t.Fatal("expected non-zero chars")
	}
	if s.CharsNoSpaces >= s.Chars {
		t.Fatal("expected charsNoSpaces < chars")
	}
}

func TestDocumentStatisticsEmpty(t *testing.T) {
	d := NewDocument()
	s := d.Statistics()
	if s.Paragraphs != 0 {
		t.Fatalf("paragraphs = %d, want 0", s.Paragraphs)
	}
}

func TestDocumentSignatures(t *testing.T) {
	d := NewDocument()
	if d.HasSignatures() {
		t.Fatal("expected no signatures")
	}
	if len(d.Signatures()) != 0 {
		t.Fatal("expected nil signatures")
	}

	d.IR.HasSignatures = true
	d.IR.DigitalSignatures = []wordir.DigitalSignature{{}}
	if !d.HasSignatures() {
		t.Fatal("expected has signatures")
	}
	if len(d.Signatures()) != 1 {
		t.Fatalf("signatures = %d, want 1", len(d.Signatures()))
	}
}