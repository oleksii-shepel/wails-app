package edit

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestRunAllGettersUnset(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()
	r.SetText("hello")

	// Boolean format getters — all unset should be false
	if r.Strike() {
		t.Fatal("Strike should be false")
	}
	if r.DoubleStrike() {
		t.Fatal("DoubleStrike should be false")
	}
	if r.Caps() {
		t.Fatal("Caps should be false")
	}
	if r.SmallCaps() {
		t.Fatal("SmallCaps should be false")
	}
	if r.Subscript() {
		t.Fatal("Subscript should be false")
	}
	if r.Superscript() {
		t.Fatal("Superscript should be false")
	}
	if r.Vanish() {
		t.Fatal("Vanish should be false")
	}
	if r.NoProof() {
		t.Fatal("NoProof should be false")
	}
	if r.WebHidden() {
		t.Fatal("WebHidden should be false")
	}
	if r.Shadow() {
		t.Fatal("Shadow should be false")
	}
	if r.Outline() {
		t.Fatal("Outline should be false")
	}
	if r.Emboss() {
		t.Fatal("Emboss should be false")
	}
	if r.Imprint() {
		t.Fatal("Imprint should be false")
	}
	if r.RTL() {
		t.Fatal("RTL should be false")
	}
	if r.ComplexScript() {
		t.Fatal("ComplexScript should be false")
	}
	if r.BoldCS() {
		t.Fatal("BoldCS should be false")
	}
	if r.ItalicCS() {
		t.Fatal("ItalicCS should be false")
	}

	// String getters — all unset should be empty
	if r.Highlight() != "" {
		t.Fatalf("Highlight = %q", r.Highlight())
	}
	if r.Background() != "" {
		t.Fatalf("Background = %q", r.Background())
	}
	if r.FontFamilyHAnsi() != "" {
		t.Fatalf("FontFamilyHAnsi = %q", r.FontFamilyHAnsi())
	}
	if r.FontFamilyEastAsia() != "" {
		t.Fatalf("FontFamilyEastAsia = %q", r.FontFamilyEastAsia())
	}
	if r.FontFamilyCS() != "" {
		t.Fatalf("FontFamilyCS = %q", r.FontFamilyCS())
	}
	if r.Lang() != "" {
		t.Fatalf("Lang = %q", r.Lang())
	}
	if r.UnderlineColor() != "" {
		t.Fatalf("UnderlineColor = %q", r.UnderlineColor())
	}
	if r.Ligatures() != "" {
		t.Fatalf("Ligatures = %q", r.Ligatures())
	}
	if r.NumberSpacing() != "" {
		t.Fatalf("NumberSpacing = %q", r.NumberSpacing())
	}

	// Int getters — all unset should be 0
	if r.FontSizeCS() != 0 {
		t.Fatalf("FontSizeCS = %d", r.FontSizeCS())
	}
	if r.CharacterSpacing() != 0 {
		t.Fatalf("CharacterSpacing = %d", r.CharacterSpacing())
	}
	if r.Kerning() != 0 {
		t.Fatalf("Kerning = %d", r.Kerning())
	}
	if r.CharScale() != 0 {
		t.Fatalf("CharScale = %d", r.CharScale())
	}

	if r.StylisticSets() != nil {
		t.Fatal("StylisticSets should be nil")
	}
}

func TestRunAllGettersAfterSet(t *testing.T) {
	d := NewDocument()
	p := d.AddParagraph()
	r := p.AddRun()

	r.SetStrike(true)
	if !r.Strike() {
		t.Fatal("expected Strike")
	}

	r.SetDoubleStrike(true)
	if !r.DoubleStrike() {
		t.Fatal("expected DoubleStrike")
	}

	r.SetCaps(true)
	if !r.Caps() {
		t.Fatal("expected Caps")
	}

	r.SetSmallCaps(true)
	if !r.SmallCaps() {
		t.Fatal("expected SmallCaps")
	}

	r.SetSubscript(true)
	if !r.Subscript() {
		t.Fatal("expected Subscript")
	}
	if r.Superscript() {
		t.Fatal("expected not Superscript")
	}

	r.SetSuperscript(true)
	if !r.Superscript() {
		t.Fatal("expected Superscript")
	}
	if r.Subscript() {
		t.Fatal("expected not Subscript")
	}

	r.SetHighlight("yellow")
	if r.Highlight() != "yellow" {
		t.Fatalf("Highlight = %q", r.Highlight())
	}

	r.SetBackground("FFFF00")
	if r.Background() != "FFFF00" {
		t.Fatalf("Background = %q", r.Background())
	}

	r.SetFontSizeCS(28)
	if r.FontSizeCS() != 28 {
		t.Fatalf("FontSizeCS = %d", r.FontSizeCS())
	}

	r.SetBoldCS(true)
	if !r.BoldCS() {
		t.Fatal("expected BoldCS")
	}

	r.SetItalicCS(true)
	if !r.ItalicCS() {
		t.Fatal("expected ItalicCS")
	}

	r.SetFontFamilyHAnsi("Arial")
	if r.FontFamilyHAnsi() != "Arial" {
		t.Fatalf("FontFamilyHAnsi = %q", r.FontFamilyHAnsi())
	}

	r.SetFontEastAsia("MS Mincho")
	if r.FontFamilyEastAsia() != "MS Mincho" {
		t.Fatalf("FontFamilyEastAsia = %q", r.FontFamilyEastAsia())
	}

	r.SetFontCS("Simplified Arabic")
	if r.FontFamilyCS() != "Simplified Arabic" {
		t.Fatalf("FontFamilyCS = %q", r.FontFamilyCS())
	}

	r.SetVanish(true)
	if !r.Vanish() {
		t.Fatal("expected Vanish")
	}

	r.SetLang("en-US")
	if r.Lang() != "en-US" {
		t.Fatalf("Lang = %q", r.Lang())
	}

	r.SetCharacterSpacing(20)
	if r.CharacterSpacing() != 20 {
		t.Fatalf("CharacterSpacing = %d", r.CharacterSpacing())
	}

	r.SetKerning(24)
	if r.Kerning() != 24 {
		t.Fatalf("Kerning = %d", r.Kerning())
	}

	r.SetCharScale(150)
	if r.CharScale() != 150 {
		t.Fatalf("CharScale = %d", r.CharScale())
	}

	r.SetShadow(true)
	if !r.Shadow() {
		t.Fatal("expected Shadow")
	}

	r.SetOutline(true)
	if !r.Outline() {
		t.Fatal("expected Outline")
	}

	r.SetEmboss(true)
	if !r.Emboss() {
		t.Fatal("expected Emboss")
	}

	r.SetImprint(true)
	if !r.Imprint() {
		t.Fatal("expected Imprint")
	}

	r.SetRTL(true)
	if !r.RTL() {
		t.Fatal("expected RTL")
	}

	r.SetComplexScript(true)
	if !r.ComplexScript() {
		t.Fatal("expected ComplexScript")
	}

	r.SetUnderlineColor("0000FF")
	if r.UnderlineColor() != "0000FF" {
		t.Fatalf("UnderlineColor = %q", r.UnderlineColor())
	}

	r.SetLigatures("standard")
	if r.Ligatures() != "standard" {
		t.Fatalf("Ligatures = %q", r.Ligatures())
	}

	r.SetNumberSpacing("proportional")
	if r.NumberSpacing() != "proportional" {
		t.Fatalf("NumberSpacing = %q", r.NumberSpacing())
	}

	r.SetStylisticSets([]int{1, 3, 5})
	ss := r.StylisticSets()
	if len(ss) != 3 || ss[0] != 1 || ss[1] != 3 || ss[2] != 5 {
		t.Fatalf("StylisticSets = %v", ss)
	}
}

func TestRunNilAllGetters(t *testing.T) {
	var r *Run
	if r.Bold() {
		t.Fatal("nil Bold should be false")
	}
	if r.Strike() {
		t.Fatal("nil Strike should be false")
	}
	if r.FontSize() != 0 {
		t.Fatal("nil FontSize should be 0")
	}
	if r.Highlight() != "" {
		t.Fatal("nil Highlight should be empty")
	}
	if r.Lang() != "" {
		t.Fatal("nil Lang should be empty")
	}
	if r.StylisticSets() != nil {
		t.Fatal("nil StylisticSets should be nil")
	}
}

func TestRunDirectIRAccessGetters(t *testing.T) {
	// Verify that getters read from the IR correctly even when the wrapper
	// is constructed directly without going through the document API.
	ir := wordir.Run{
		Kind: "run",
		Text: "direct",
		Format: wordir.RunFormat{
			Bold:      ptr(true),
			Italic:    ptr(true),
			Strike:    ptr(true),
			Caps:      ptr(true),
			SmallCaps: ptr(true),
			FontSize:  ptr(24),
			Color:     "FF0000",
			FontASCII: "Calibri",
			Highlight: "green",
			Lang:      "en-GB",
		},
	}
	r := &Run{para: &Paragraph{ir: &wordir.Paragraph{Runs: []wordir.Run{ir}}}, index: 0}
	if !r.Bold() {
		t.Fatal("expected Bold")
	}
	if !r.Italic() {
		t.Fatal("expected Italic")
	}
	if !r.Strike() {
		t.Fatal("expected Strike")
	}
	if !r.Caps() {
		t.Fatal("expected Caps")
	}
	if !r.SmallCaps() {
		t.Fatal("expected SmallCaps")
	}
	if r.FontSize() != 24 {
		t.Fatalf("FontSize = %d", r.FontSize())
	}
	if r.Color() != "FF0000" {
		t.Fatalf("Color = %q", r.Color())
	}
	if r.FontFamily() != "Calibri" {
		t.Fatalf("FontFamily = %q", r.FontFamily())
	}
	if r.Highlight() != "green" {
		t.Fatalf("Highlight = %q", r.Highlight())
	}
	if r.Lang() != "en-GB" {
		t.Fatalf("Lang = %q", r.Lang())
	}
}

func ptr[T any](v T) *T { return &v }