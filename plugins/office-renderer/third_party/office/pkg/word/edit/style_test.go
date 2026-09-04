package edit

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestParagraphStyleGettersAndSetters(t *testing.T) {
	d := NewDocument()
	s := d.EnsureParagraphStyle("TestStyle", "Test Style")

	// Setters
	s.SetBasedOn("Normal")
	s.SetDefault(true)
	s.SetLinkedCharID("TestChar")
	s.SetAlignment("center")
	s.SetSpacingBefore(120)
	s.SetSpacingAfter(240)
	s.SetLineSpacing(360)
	s.SetLineRule("auto")
	s.SetIndentFirstLine(720)
	s.SetIndentLeft(360)
	s.SetIndentRight(360)
	s.SetNumbering(1, 2)
	s.SetKeepNext(true)
	s.SetKeepLines(true)
	s.SetWidowControl(true)
	s.SetPageBreakBefore(true)
	s.SetShading("FFFF00")
	s.SetContextualSpacing(true)
	s.SetRunBold(true)
	s.SetRunItalic(true)
	s.SetRunFontSize(24)
	s.SetRunColor("FF0000")
	s.SetRunFontFamily("Arial")
	s.SetNextStyleID("Normal")
	s.SetUIPriority(9)
	s.SetRSID("12345678")
	s.SetUnhideWhenUsed(true)
	s.SetQFormat(true)
	s.SetSemiHidden(true)
	s.SetAutoRedefine(true)

	// Getters
	if s.BasedOn() != "Normal" {
		t.Fatalf("BasedOn = %q", s.BasedOn())
	}
	if !s.Default() {
		t.Fatal("expected Default")
	}
	if s.LinkedCharID() != "TestChar" {
		t.Fatalf("LinkedCharID = %q", s.LinkedCharID())
	}
	if s.Alignment() != "center" {
		t.Fatalf("Alignment = %q", s.Alignment())
	}
	if s.SpacingBefore() != 120 {
		t.Fatalf("SpacingBefore = %d", s.SpacingBefore())
	}
	if s.SpacingAfter() != 240 {
		t.Fatalf("SpacingAfter = %d", s.SpacingAfter())
	}
	if s.LineSpacing() != 360 {
		t.Fatalf("LineSpacing = %d", s.LineSpacing())
	}
	if s.LineRule() != "auto" {
		t.Fatalf("LineRule = %q", s.LineRule())
	}
	if s.IndentFirstLine() != 720 {
		t.Fatalf("IndentFirstLine = %d", s.IndentFirstLine())
	}
	if s.IndentLeft() != 360 {
		t.Fatalf("IndentLeft = %d", s.IndentLeft())
	}
	if s.IndentRight() != 360 {
		t.Fatalf("IndentRight = %d", s.IndentRight())
	}
	if nr := s.Numbering(); nr == nil || nr.NumID != 1 || nr.Level != 2 {
		t.Fatalf("Numbering = %+v", nr)
	}
	if !s.KeepNext() {
		t.Fatal("expected KeepNext")
	}
	if !s.KeepLines() {
		t.Fatal("expected KeepLines")
	}
	if !s.WidowControl() {
		t.Fatal("expected WidowControl")
	}
	if !s.PageBreakBefore() {
		t.Fatal("expected PageBreakBefore")
	}
	if s.Shading() != "FFFF00" {
		t.Fatalf("Shading = %q", s.Shading())
	}
	if !s.ContextualSpacing() {
		t.Fatal("expected ContextualSpacing")
	}
	if !s.RunBold() {
		t.Fatal("expected RunBold")
	}
	if !s.RunItalic() {
		t.Fatal("expected RunItalic")
	}
	if s.RunFontSize() != 24 {
		t.Fatalf("RunFontSize = %d", s.RunFontSize())
	}
	if s.RunColor() != "FF0000" {
		t.Fatalf("RunColor = %q", s.RunColor())
	}
	if s.RunFontFamily() != "Arial" {
		t.Fatalf("RunFontFamily = %q", s.RunFontFamily())
	}
	if s.NextStyleID() != "Normal" {
		t.Fatalf("NextStyleID = %q", s.NextStyleID())
	}
	if s.UIPriority() != 9 {
		t.Fatalf("UIPriority = %d", s.UIPriority())
	}
	if s.RSID() != "12345678" {
		t.Fatalf("RSID = %q", s.RSID())
	}
	if !s.UnhideWhenUsed() {
		t.Fatal("expected UnhideWhenUsed")
	}
	if !s.QFormat() {
		t.Fatal("expected QFormat")
	}
	if !s.SemiHidden() {
		t.Fatal("expected SemiHidden")
	}
	if !s.AutoRedefine() {
		t.Fatal("expected AutoRedefine")
	}
}

func TestCharacterStyleGettersAndSetters(t *testing.T) {
	d := NewDocument()
	s := d.EnsureCharacterStyle("CharStyle", "Char Style")

	s.SetBasedOn("DefaultParagraphFont")
	s.SetBold(true)
	s.SetItalic(true)
	s.SetUnderline("single")
	s.SetFontSize(22)
	s.SetColor("0000FF")
	s.SetFontFamily("Calibri")
	s.SetCaps(true)
	s.SetSmallCaps(true)
	s.SetStrike(true)
	s.SetHighlight("yellow")
	s.SetVertAlign("superscript")
	s.SetNextStyleID("NextChar")
	s.SetUIPriority(20)
	s.SetRSID("87654321")
	s.SetUnhideWhenUsed(true)
	s.SetQFormat(true)
	s.SetSemiHidden(true)
	s.SetAutoRedefine(true)

	if s.BasedOn() != "DefaultParagraphFont" {
		t.Fatalf("BasedOn = %q", s.BasedOn())
	}
	if !s.Bold() {
		t.Fatal("expected Bold")
	}
	if !s.Italic() {
		t.Fatal("expected Italic")
	}
	if s.Underline() != "single" {
		t.Fatalf("Underline = %q", s.Underline())
	}
	if s.FontSize() != 22 {
		t.Fatalf("FontSize = %d", s.FontSize())
	}
	if s.Color() != "0000FF" {
		t.Fatalf("Color = %q", s.Color())
	}
	if s.FontFamily() != "Calibri" {
		t.Fatalf("FontFamily = %q", s.FontFamily())
	}
	if !s.Caps() {
		t.Fatal("expected Caps")
	}
	if !s.SmallCaps() {
		t.Fatal("expected SmallCaps")
	}
	if !s.Strike() {
		t.Fatal("expected Strike")
	}
	if s.Highlight() != "yellow" {
		t.Fatalf("Highlight = %q", s.Highlight())
	}
	if s.VertAlign() != "superscript" {
		t.Fatalf("VertAlign = %q", s.VertAlign())
	}
	if s.NextStyleID() != "NextChar" {
		t.Fatalf("NextStyleID = %q", s.NextStyleID())
	}
	if s.UIPriority() != 20 {
		t.Fatalf("UIPriority = %d", s.UIPriority())
	}
	if s.RSID() != "87654321" {
		t.Fatalf("RSID = %q", s.RSID())
	}
	if !s.UnhideWhenUsed() {
		t.Fatal("expected UnhideWhenUsed")
	}
	if !s.QFormat() {
		t.Fatal("expected QFormat")
	}
	if !s.SemiHidden() {
		t.Fatal("expected SemiHidden")
	}
	if !s.AutoRedefine() {
		t.Fatal("expected AutoRedefine")
	}
}

func TestTableStyleGettersAndSetters(t *testing.T) {
	d := NewDocument()
	s := d.EnsureTableStyle("TblStyle", "Table Style")

	s.SetBasedOn("TableNormal")
	s.SetDefault(true)
	fmt := &wordir.TableFormat{Layout: "fixed"}
	s.SetTableFormat(fmt)
	s.SetNextStyleID("TableNormal")
	s.SetUIPriority(99)
	s.SetRSID("ABCD1234")
	s.SetUnhideWhenUsed(true)
	s.SetQFormat(true)
	s.SetSemiHidden(true)
	s.SetAutoRedefine(true)

	if s.BasedOn() != "TableNormal" {
		t.Fatalf("BasedOn = %q", s.BasedOn())
	}
	if !s.Default() {
		t.Fatal("expected Default")
	}
	if tf := s.TableFormat(); tf == nil || tf.Layout != "fixed" {
		t.Fatalf("TableFormat = %+v", tf)
	}
	if s.NextStyleID() != "TableNormal" {
		t.Fatalf("NextStyleID = %q", s.NextStyleID())
	}
	if s.UIPriority() != 99 {
		t.Fatalf("UIPriority = %d", s.UIPriority())
	}
	if s.RSID() != "ABCD1234" {
		t.Fatalf("RSID = %q", s.RSID())
	}
	if !s.UnhideWhenUsed() {
		t.Fatal("expected UnhideWhenUsed")
	}
	if !s.QFormat() {
		t.Fatal("expected QFormat")
	}
	if !s.SemiHidden() {
		t.Fatal("expected SemiHidden")
	}
	if !s.AutoRedefine() {
		t.Fatal("expected AutoRedefine")
	}
}

func TestDocumentStyleLookups(t *testing.T) {
	d := NewDocument()
	d.EnsureParagraphStyle("P1", "Paragraph 1")
	d.EnsureCharacterStyle("C1", "Character 1")
	d.EnsureTableStyle("T1", "Table 1")

	if d.ParagraphStyle("P1") == nil {
		t.Fatal("expected to find paragraph style P1")
	}
	if d.ParagraphStyle("Missing") != nil {
		t.Fatal("expected nil for missing paragraph style")
	}
	if d.CharacterStyle("C1") == nil {
		t.Fatal("expected to find character style C1")
	}
	if d.CharacterStyle("Missing") != nil {
		t.Fatal("expected nil for missing character style")
	}
	if d.TableStyle("T1") == nil {
		t.Fatal("expected to find table style T1")
	}
	if d.TableStyle("Missing") != nil {
		t.Fatal("expected nil for missing table style")
	}
	// Ensure paragraph style lookup does not return table styles.
	if d.ParagraphStyle("T1") != nil {
		t.Fatal("expected paragraph lookup to exclude table styles")
	}
}

func TestDocumentStyleRemovals(t *testing.T) {
	d := NewDocument()
	d.EnsureParagraphStyle("P1", "Paragraph 1")
	d.EnsureCharacterStyle("C1", "Character 1")
	d.EnsureTableStyle("T1", "Table 1")

	d.RemoveParagraphStyle("P1")
	if d.ParagraphStyle("P1") != nil {
		t.Fatal("expected P1 to be removed")
	}

	d.RemoveCharacterStyle("C1")
	if d.CharacterStyle("C1") != nil {
		t.Fatal("expected C1 to be removed")
	}

	d.RemoveTableStyle("T1")
	if d.TableStyle("T1") != nil {
		t.Fatal("expected T1 to be removed")
	}

	// Removing non-existent styles should not panic.
	d.RemoveParagraphStyle("Missing")
	d.RemoveCharacterStyle("Missing")
	d.RemoveTableStyle("Missing")
}

func TestStyleNilSafety(t *testing.T) {
	var ps *ParagraphStyle
	if ps.ID() != "" {
		t.Fatal("nil ParagraphStyle ID should be empty")
	}
	if ps.Name() != "" {
		t.Fatal("nil ParagraphStyle Name should be empty")
	}
	if ps.BasedOn() != "" {
		t.Fatal("nil ParagraphStyle BasedOn should be empty")
	}
	if ps.Default() {
		t.Fatal("nil ParagraphStyle Default should be false")
	}
	if ps.NextStyleID() != "" {
		t.Fatal("nil ParagraphStyle NextStyleID should be empty")
	}
	if ps.UIPriority() != 0 {
		t.Fatal("nil ParagraphStyle UIPriority should be 0")
	}
	if ps.RSID() != "" {
		t.Fatal("nil ParagraphStyle RSID should be empty")
	}
	if ps.UnhideWhenUsed() {
		t.Fatal("nil ParagraphStyle UnhideWhenUsed should be false")
	}
	if ps.QFormat() {
		t.Fatal("nil ParagraphStyle QFormat should be false")
	}
	if ps.SemiHidden() {
		t.Fatal("nil ParagraphStyle SemiHidden should be false")
	}
	if ps.AutoRedefine() {
		t.Fatal("nil ParagraphStyle AutoRedefine should be false")
	}

	var cs *CharacterStyle
	if cs.ID() != "" {
		t.Fatal("nil CharacterStyle ID should be empty")
	}
	if cs.Bold() {
		t.Fatal("nil CharacterStyle Bold should be false")
	}
	if cs.NextStyleID() != "" {
		t.Fatal("nil CharacterStyle NextStyleID should be empty")
	}
	if cs.UIPriority() != 0 {
		t.Fatal("nil CharacterStyle UIPriority should be 0")
	}
	if cs.RSID() != "" {
		t.Fatal("nil CharacterStyle RSID should be empty")
	}
	if cs.UnhideWhenUsed() {
		t.Fatal("nil CharacterStyle UnhideWhenUsed should be false")
	}
	if cs.QFormat() {
		t.Fatal("nil CharacterStyle QFormat should be false")
	}
	if cs.SemiHidden() {
		t.Fatal("nil CharacterStyle SemiHidden should be false")
	}
	if cs.AutoRedefine() {
		t.Fatal("nil CharacterStyle AutoRedefine should be false")
	}

	var ts *TableStyle
	if ts.ID() != "" {
		t.Fatal("nil TableStyle ID should be empty")
	}
	if ts.Default() {
		t.Fatal("nil TableStyle Default should be false")
	}
	if ts.NextStyleID() != "" {
		t.Fatal("nil TableStyle NextStyleID should be empty")
	}
	if ts.UIPriority() != 0 {
		t.Fatal("nil TableStyle UIPriority should be 0")
	}
	if ts.RSID() != "" {
		t.Fatal("nil TableStyle RSID should be empty")
	}
	if ts.UnhideWhenUsed() {
		t.Fatal("nil TableStyle UnhideWhenUsed should be false")
	}
	if ts.QFormat() {
		t.Fatal("nil TableStyle QFormat should be false")
	}
	if ts.SemiHidden() {
		t.Fatal("nil TableStyle SemiHidden should be false")
	}
	if ts.AutoRedefine() {
		t.Fatal("nil TableStyle AutoRedefine should be false")
	}
}