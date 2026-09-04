package edit

import (
	"testing"
)

func TestAddSignatureLine(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	err := d.AddSignatureLine(p, SignatureLineOptions{
		SuggestedSigner:      "John Doe",
		SuggestedSigner2:     "CEO",
		SuggestedSignerEmail: "john@example.com",
		SigningInstructions:  "Please sign here",
		AllowComments:        true,
		ShowSignDate:         true,
	})
	if err != nil {
		t.Fatalf("AddSignatureLine: %v", err)
	}

	if !d.HasSignatureLines() {
		t.Fatal("expected HasSignatureLines")
	}

	sls := d.SignatureLines()
	if len(sls) != 1 {
		t.Fatalf("signature lines = %d, want 1", len(sls))
	}
	sl := sls[0]
	if sl.SuggestedSigner != "John Doe" {
		t.Fatalf("suggestedSigner = %q", sl.SuggestedSigner)
	}
	if sl.SuggestedSigner2 != "CEO" {
		t.Fatalf("suggestedSigner2 = %q", sl.SuggestedSigner2)
	}
	if sl.SuggestedSignerEmail != "john@example.com" {
		t.Fatalf("email = %q", sl.SuggestedSignerEmail)
	}
	if !sl.IsSignatureLine {
		t.Fatal("expected IsSignatureLine")
	}
	if !sl.AllowComments {
		t.Fatal("expected AllowComments")
	}
}

func TestAddSignatureLineNilSafety(t *testing.T) {
	var d *Document
	if err := d.AddSignatureLine(nil, SignatureLineOptions{}); err == nil {
		t.Fatal("expected error for nil document")
	}
}