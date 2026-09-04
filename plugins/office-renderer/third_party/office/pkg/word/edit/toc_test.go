package edit

import (
	"strings"
	"testing"
)

func TestDocumentInsertTOC(t *testing.T) {
	d := NewDocument()
	toc := d.InsertTOC(3)
	if toc == nil {
		t.Fatal("expected toc")
	}
	if toc.StoryField() == nil {
		t.Fatal("expected story field")
	}
	if toc.StoryField().Kind() != "TOC" {
		t.Fatalf("kind = %q, want TOC", toc.StoryField().Kind())
	}
	if !strings.Contains(toc.StoryField().Instr(), `TOC \o "1-3"`) {
		t.Fatalf("instr = %q", toc.StoryField().Instr())
	}
}

func TestTOCUpdateFromHeadings(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p1.SetText("Introduction")
	p1.SetOutlineLevel(0)

	p2 := d.AddParagraph()
	p2.SetText("Background")
	p2.SetOutlineLevel(1)

	p3 := d.AddParagraph()
	p3.SetText("Not a heading")

	p4 := d.AddParagraph()
	p4.SetText("Conclusion")
	p4.SetOutlineLevel(0)

	toc := d.InsertTOC(3)
	toc.UpdateFromHeadings()

	count := toc.StoryField().ResultBlockCount()
	if count != 3 {
		t.Fatalf("entries = %d, want 3", count)
	}
}

func TestTOCUpdateFromHeadingStyles(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p1.SetText("Chapter 1")
	p1.ir.StyleID = "Heading1"

	p2 := d.AddParagraph()
	p2.SetText("Section 1.1")
	p2.ir.StyleID = "Heading2"

	p3 := d.AddParagraph()
	p3.SetText("Body text")
	p3.ir.StyleID = "Normal"

	toc := d.InsertTOC(2)
	toc.UpdateFromHeadings()

	count := toc.StoryField().ResultBlockCount()
	if count != 2 {
		t.Fatalf("entries = %d, want 2", count)
	}
}

func TestTOCUpdateClearsPrevious(t *testing.T) {
	d := NewDocument()
	p1 := d.AddParagraph()
	p1.SetText("Heading")
	p1.SetOutlineLevel(0)

	toc := d.InsertTOC(3)
	toc.UpdateFromHeadings()
	if toc.StoryField().ResultBlockCount() != 1 {
		t.Fatalf("first count = %d, want 1", toc.StoryField().ResultBlockCount())
	}

	// Remove heading and regenerate
	p1.SetOutlineLevel(9)
	toc.UpdateFromHeadings()
	if toc.StoryField().ResultBlockCount() != 0 {
		t.Fatalf("second count = %d, want 0", toc.StoryField().ResultBlockCount())
	}
}

func TestTOCInsertAtBeginning(t *testing.T) {
	d := NewDocument()
	d.AddParagraph().SetText("first body")
	toc := d.InsertTOC(3)
	_ = toc

	// The TOC title should be the first block
	if d.IR.Blocks[0].Kind != "paragraph" {
		t.Fatalf("first block kind = %q", d.IR.Blocks[0].Kind)
	}
	if d.IR.Blocks[0].Paragraph == nil {
		t.Fatal("expected paragraph")
	}
	titlePara := d.IR.Blocks[0].Paragraph
	var titleText string
	for _, r := range titlePara.Runs {
		titleText += r.Text
	}
	if titleText != "Table of Contents" {
		t.Fatalf("text = %q", titleText)
	}
}