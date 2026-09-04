package edit

import "testing"

func TestParagraphGettersUnset(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()

	if p.Style() != "" {
		t.Fatalf("Style = %q", p.Style())
	}
	if p.Alignment() != "" {
		t.Fatalf("Alignment = %q", p.Alignment())
	}
	if p.SpacingBefore() != 0 {
		t.Fatalf("SpacingBefore = %d", p.SpacingBefore())
	}
	if p.SpacingAfter() != 0 {
		t.Fatalf("SpacingAfter = %d", p.SpacingAfter())
	}
	if p.LineSpacing() != 0 {
		t.Fatalf("LineSpacing = %d", p.LineSpacing())
	}
	if p.LineRule() != "" {
		t.Fatalf("LineRule = %q", p.LineRule())
	}
	if p.IndentFirstLine() != 0 {
		t.Fatalf("IndentFirstLine = %d", p.IndentFirstLine())
	}
	if p.IndentLeft() != 0 {
		t.Fatalf("IndentLeft = %d", p.IndentLeft())
	}
	if p.IndentRight() != 0 {
		t.Fatalf("IndentRight = %d", p.IndentRight())
	}
	if p.Numbering() != nil {
		t.Fatal("Numbering should be nil")
	}
	if p.OutlineLevel() != -1 {
		t.Fatalf("OutlineLevel = %d", p.OutlineLevel())
	}
	if p.ContextualSpacing() {
		t.Fatal("ContextualSpacing should be false")
	}
	if p.KeepLines() {
		t.Fatal("KeepLines should be false")
	}
	if p.KeepNext() {
		t.Fatal("KeepNext should be false")
	}
	if p.PageBreakBefore() {
		t.Fatal("PageBreakBefore should be false")
	}
	if p.WidowControl() {
		t.Fatal("WidowControl should be false")
	}
	if p.Shading() != "" {
		t.Fatalf("Shading = %q", p.Shading())
	}
	if p.Bidi() {
		t.Fatal("Bidi should be false")
	}
	if p.NoLineNumber() {
		t.Fatal("NoLineNumber should be false")
	}
	if p.SnapToGrid() {
		t.Fatal("SnapToGrid should be false")
	}
	if p.Kinsoku() {
		t.Fatal("Kinsoku should be false")
	}
	if p.WordWrap() {
		t.Fatal("WordWrap should be false")
	}
	if p.OverflowPunct() {
		t.Fatal("OverflowPunct should be false")
	}
	if p.TopLinePunct() {
		t.Fatal("TopLinePunct should be false")
	}
	if p.AutoSpaceDE() {
		t.Fatal("AutoSpaceDE should be false")
	}
	if p.AutoSpaceDN() {
		t.Fatal("AutoSpaceDN should be false")
	}
	if p.FontAlign() != "" {
		t.Fatalf("FontAlign = %q", p.FontAlign())
	}
}

func TestParagraphGettersAfterSet(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()

	p.SetStyle("Heading1")
	if p.Style() != "Heading1" {
		t.Fatalf("Style = %q", p.Style())
	}

	p.SetAlignment("center")
	if p.Alignment() != "center" {
		t.Fatalf("Alignment = %q", p.Alignment())
	}

	p.SetSpacingBefore(120)
	if p.SpacingBefore() != 120 {
		t.Fatalf("SpacingBefore = %d", p.SpacingBefore())
	}

	p.SetSpacingAfter(240)
	if p.SpacingAfter() != 240 {
		t.Fatalf("SpacingAfter = %d", p.SpacingAfter())
	}

	p.SetLineSpacing(360, "auto")
	if p.LineSpacing() != 360 {
		t.Fatalf("LineSpacing = %d", p.LineSpacing())
	}
	if p.LineRule() != "auto" {
		t.Fatalf("LineRule = %q", p.LineRule())
	}

	p.SetIndentFirstLine(720)
	if p.IndentFirstLine() != 720 {
		t.Fatalf("IndentFirstLine = %d", p.IndentFirstLine())
	}

	p.SetIndentLeft(360)
	if p.IndentLeft() != 360 {
		t.Fatalf("IndentLeft = %d", p.IndentLeft())
	}

	p.SetIndentRight(180)
	if p.IndentRight() != 180 {
		t.Fatalf("IndentRight = %d", p.IndentRight())
	}

	p.SetNumbering(1, 2)
	if p.Numbering() == nil {
		t.Fatal("expected Numbering")
	}

	p.SetOutlineLevel(3)
	if p.OutlineLevel() != 3 {
		t.Fatalf("OutlineLevel = %d", p.OutlineLevel())
	}

	p.SetContextualSpacing(true)
	if !p.ContextualSpacing() {
		t.Fatal("expected ContextualSpacing")
	}

	p.SetKeepLines(true)
	if !p.KeepLines() {
		t.Fatal("expected KeepLines")
	}

	p.SetKeepNext(true)
	if !p.KeepNext() {
		t.Fatal("expected KeepNext")
	}

	p.SetPageBreakBefore(true)
	if !p.PageBreakBefore() {
		t.Fatal("expected PageBreakBefore")
	}

	p.SetWidowControl(true)
	if !p.WidowControl() {
		t.Fatal("expected WidowControl")
	}

	p.SetShading("FFFF00")
	if p.Shading() != "FFFF00" {
		t.Fatalf("Shading = %q", p.Shading())
	}

	p.SetBidi(true)
	if !p.Bidi() {
		t.Fatal("expected Bidi")
	}

	p.SetSuppressLineNumbers(true)
	if !p.NoLineNumber() {
		t.Fatal("expected NoLineNumber")
	}

	p.SetSnapToGrid(true)
	if !p.SnapToGrid() {
		t.Fatal("expected SnapToGrid")
	}

	p.SetKinsoku(true)
	if !p.Kinsoku() {
		t.Fatal("expected Kinsoku")
	}

	p.SetWordWrap(true)
	if !p.WordWrap() {
		t.Fatal("expected WordWrap")
	}

	p.SetOverflowPunct(true)
	if !p.OverflowPunct() {
		t.Fatal("expected OverflowPunct")
	}

	p.SetTopLinePunct(true)
	if !p.TopLinePunct() {
		t.Fatal("expected TopLinePunct")
	}

	p.SetAutoSpaceDE(true)
	if !p.AutoSpaceDE() {
		t.Fatal("expected AutoSpaceDE")
	}

	p.SetAutoSpaceDN(true)
	if !p.AutoSpaceDN() {
		t.Fatal("expected AutoSpaceDN")
	}

	p.SetFontAlign("center")
	if p.FontAlign() != "center" {
		t.Fatalf("FontAlign = %q", p.FontAlign())
	}
}

func TestParagraphNilGetters(t *testing.T) {
	var p *Paragraph
	if p.Style() != "" {
		t.Fatal("nil Style should be empty")
	}
	if p.Alignment() != "" {
		t.Fatal("nil Alignment should be empty")
	}
	if p.SpacingBefore() != 0 {
		t.Fatal("nil SpacingBefore should be 0")
	}
	if p.Numbering() != nil {
		t.Fatal("nil Numbering should be nil")
	}
	if p.OutlineLevel() != -1 {
		t.Fatal("nil OutlineLevel should be -1")
	}
	if p.ContextualSpacing() {
		t.Fatal("nil ContextualSpacing should be false")
	}
	if p.Shading() != "" {
		t.Fatal("nil Shading should be empty")
	}
}