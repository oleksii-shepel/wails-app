package edit

import (
	"strings"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestTextRangeText(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("hello ")
	r2 := p.AddRun()
	r2.SetText("world")

	tr := &TextRange{
		paragraph: p,
		startRun:  0,
		startPos:  2,
		endRun:    1,
		endPos:    3,
	}
	if got := tr.Text(); got != "llo wor" {
		t.Fatalf("text = %q, want llo wor", got)
	}
}

func TestTextRangeDelete(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("hello ")
	r2 := p.AddRun()
	r2.SetText("world")

	tr := p.FindTextRange("llo wor")
	if tr == nil {
		t.Fatal("expected range")
	}
	tr.Delete()
	if p.Text() != "held" {
		t.Fatalf("text = %q, want held", p.Text())
	}
}

func TestTextRangeReplaceText(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("hello ")
	r2 := p.AddRun()
	r2.SetText("world")

	tr := p.FindTextRange("llo wor")
	if tr == nil {
		t.Fatal("expected range")
	}
	tr.ReplaceText("XYZ")
	if p.Text() != "heXYZld" {
		t.Fatalf("text = %q, want heXYZld", p.Text())
	}
}

func TestTextRangeReplaceTextSingleRun(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("abcdef")

	tr := p.FindTextRange("bcd")
	if tr == nil {
		t.Fatal("expected range")
	}
	tr.ReplaceText("123")
	if p.Text() != "a123ef" {
		t.Fatalf("text = %q, want a123ef", p.Text())
	}
}

func TestTextRangeDeleteAll(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("abc")
	r2 := p.AddRun()
	r2.SetText("def")

	tr := &TextRange{
		paragraph: p,
		startRun:  0,
		startPos:  0,
		endRun:    1,
		endPos:    3,
	}
	tr.Delete()
	if p.Text() != "" {
		t.Fatalf("text = %q, want empty", p.Text())
	}
}

func TestParagraphFindTextRangeAll(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("abc abc abc")

	ranges := p.FindTextRangeAll("abc")
	if len(ranges) != 3 {
		t.Fatalf("len(ranges) = %d, want 3", len(ranges))
	}
	for _, tr := range ranges {
		if tr.Text() != "abc" {
			t.Fatalf("range text = %q, want abc", tr.Text())
		}
	}
}

func TestParagraphFindTextRangeNotFound(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.AddRun().SetText("hello")
	if p.FindTextRange("xyz") != nil {
		t.Fatal("expected nil")
	}
}

func TestParagraphFindTextRangeEmpty(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	p.AddRun().SetText("hello")
	if p.FindTextRange("") != nil {
		t.Fatal("expected nil for empty search")
	}
}

func TestRunIndexForCharPos(t *testing.T) {
	runs := []wordir.Run{
		{Text: "abc"},
		{Text: "de"},
		{Text: "f"},
	}
	tests := []struct {
		offset  int
		wantRun int
		wantPos int
	}{
		{0, 0, 0},
		{1, 0, 1},
		{3, 1, 0},
		{4, 1, 1},
		{5, 2, 0},
		{6, 2, 1}, // end position
		{7, -1, -1},
		{-1, -1, -1},
	}
	for _, tt := range tests {
		gotRun, gotPos := runIndexForCharPos(runs, tt.offset)
		if gotRun != tt.wantRun || gotPos != tt.wantPos {
			t.Fatalf("offset %d: got (%d,%d), want (%d,%d)", tt.offset, gotRun, gotPos, tt.wantRun, tt.wantPos)
		}
	}
}

func TestTextRangeNilSafety(t *testing.T) {
	var tr *TextRange
	if tr.Text() != "" {
		t.Fatal("expected empty text")
	}
	tr.Delete() // should not panic
	tr.ReplaceText("x")
}

func TestTextRangePreservesFormatting(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r1 := p.AddRun()
	r1.SetText("hello ")
	r1.SetBold(true)
	r2 := p.AddRun()
	r2.SetText("world")
	r2.SetItalic(true)

	tr := p.FindTextRange("llo wor")
	if tr == nil {
		t.Fatal("expected range")
	}
	tr.ReplaceText("XYZ")

	// The replacement run should inherit bold from the start run
	found := false
	for _, r := range p.Runs() {
		if strings.Contains(r.Text(), "XYZ") {
			found = true
			if !r.Bold() {
				t.Fatal("replacement run should inherit bold from start run")
			}
		}
	}
	if !found {
		t.Fatal("replacement run not found")
	}
}