package edit

import "testing"

func TestTextRangeInsertAfterAtEnd(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("hello")

	tr := p.FindTextRange("hello")
	if tr == nil {
		t.Fatal("expected range")
	}
	tr.InsertAfter(" world")
	if p.Text() != "hello world" {
		t.Fatalf("text = %q, want hello world", p.Text())
	}
}

func TestTextRangeInsertAfterMidRun(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("abcde")

	tr := p.FindTextRange("bcd")
	if tr == nil {
		t.Fatal("expected range")
	}
	tr.InsertAfter("XYZ")
	if p.Text() != "abcdXYZe" {
		t.Fatalf("text = %q, want abcdXYZe", p.Text())
	}
}

func TestTextRangeInsertAfterCrossRun(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("ab")
	r2 := p.AddRun()
	r2.SetText("cd")

	tr := p.FindTextRange("bc")
	if tr == nil {
		t.Fatal("expected range")
	}
	tr.InsertAfter("XYZ")
	if p.Text() != "abcXYZd" {
		t.Fatalf("text = %q, want abcXYZd", p.Text())
	}
}

func TestTextRangeInsertBeforeAtStart(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("world")

	tr := p.FindTextRange("world")
	if tr == nil {
		t.Fatal("expected range")
	}
	tr.InsertBefore("hello ")
	if p.Text() != "hello world" {
		t.Fatalf("text = %q, want hello world", p.Text())
	}
}

func TestTextRangeInsertBeforeMidRun(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("abcde")

	tr := p.FindTextRange("bcd")
	if tr == nil {
		t.Fatal("expected range")
	}
	tr.InsertBefore("XYZ")
	if p.Text() != "aXYZbcde" {
		t.Fatalf("text = %q, want aXYZbcde", p.Text())
	}
}

func TestTextRangeInsertBeforeCrossRun(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("ab")
	r2 := p.AddRun()
	r2.SetText("cd")

	tr := p.FindTextRange("bc")
	if tr == nil {
		t.Fatal("expected range")
	}
	tr.InsertBefore("XYZ")
	if p.Text() != "aXYZbcd" {
		t.Fatalf("text = %q, want aXYZbcd", p.Text())
	}
}

func TestTextRangeInsertNilSafety(t *testing.T) {
	var tr *TextRange
	tr.InsertAfter("x") // should not panic
	tr.InsertBefore("x")
}