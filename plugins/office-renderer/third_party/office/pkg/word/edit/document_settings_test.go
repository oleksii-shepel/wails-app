package edit

import (
	"testing"
)

func TestDocumentSettingsSetters(t *testing.T) {
	d := NewDocument()

	// View mode
	d.SetViewMode("web")
	if d.IR.ViewMode != "web" {
		t.Fatalf("viewMode = %q", d.IR.ViewMode)
	}

	// Default table style
	d.SetDefaultTableStyle("TableGrid")
	if d.IR.DefaultTableStyle != "TableGrid" {
		t.Fatalf("defaultTableStyle = %q", d.IR.DefaultTableStyle)
	}

	// Background color
	d.SetBackgroundColor("FF0000")
	if d.IR.BackgroundColor != "FF0000" {
		t.Fatalf("backgroundColor = %q", d.IR.BackgroundColor)
	}

	// Display background shape
	d.SetDisplayBackgroundShape(true)
	if d.IR.DisplayBackgroundShape == nil || !*d.IR.DisplayBackgroundShape {
		t.Fatal("expected DisplayBackgroundShape = true")
	}

	// Do not display page boundaries
	d.SetDoNotDisplayPageBoundaries(true)
	if !d.IR.DoNotDisplayPageBoundaries {
		t.Fatal("expected DoNotDisplayPageBoundaries = true")
	}

	// Do not track formatting
	d.SetDoNotTrackFormatting(true)
	if !d.IR.DoNotTrackFormatting {
		t.Fatal("expected DoNotTrackFormatting = true")
	}

	// Do not track moves
	d.SetDoNotTrackMoves(true)
	if !d.IR.DoNotTrackMoves {
		t.Fatal("expected DoNotTrackMoves = true")
	}

	// Consecutive hyphen limit
	d.SetConsecutiveHyphenLimit(3)
	if d.IR.ConsecutiveHyphenLimit != 3 {
		t.Fatalf("consecutiveHyphenLimit = %d", d.IR.ConsecutiveHyphenLimit)
	}

	// Hyphenation zone
	d.SetHyphenationZone(360)
	if d.IR.HyphenationZone != 360 {
		t.Fatalf("hyphenationZone = %d", d.IR.HyphenationZone)
	}

	// Do not hyphenate caps
	d.SetDoNotHyphenateCaps(true)
	if !d.IR.DoNotHyphenateCaps {
		t.Fatal("expected DoNotHyphenateCaps = true")
	}

	// Document type
	d.SetDocumentType("letter")
	if d.IR.DocumentType != "letter" {
		t.Fatalf("documentType = %q", d.IR.DocumentType)
	}
}

func TestDefaultCharacterStyleID(t *testing.T) {
	d := NewDocument()
	if d.DefaultCharacterStyleID() != "" {
		t.Fatal("expected empty default character style")
	}
	d.SetDefaultCharacterStyle("DefaultParagraphFont")
	if d.DefaultCharacterStyleID() != "DefaultParagraphFont" {
		t.Fatalf("defaultCharStyle = %q", d.DefaultCharacterStyleID())
	}
}

func TestMiscSettingsGettersAndSetters(t *testing.T) {
	d := NewDocument()

	// int fields (drawing grid)
	d.SetDisplayHorizontalDrawingGrid(100)
	if d.DisplayHorizontalDrawingGrid() != 100 {
		t.Fatalf("DisplayHorizontalDrawingGrid = %d", d.DisplayHorizontalDrawingGrid())
	}

	d.SetBookFoldPrintingSheets(4)
	if d.BookFoldPrintingSheets() != 4 {
		t.Fatalf("BookFoldPrintingSheets = %d", d.BookFoldPrintingSheets())
	}

	d.SetLvlRestart(1)
	if d.LvlRestart() != 1 {
		t.Fatalf("LvlRestart = %d", d.LvlRestart())
	}

	// bool fields
	d.SetBookFoldPrinting(true)
	if !d.BookFoldPrinting() {
		t.Fatal("expected BookFoldPrinting = true")
	}

	d.SetEmbedTrueTypeFonts(true)
	if !d.EmbedTrueTypeFonts() {
		t.Fatal("expected EmbedTrueTypeFonts = true")
	}

	d.SetHideSpellingErrors(true)
	if !d.HideSpellingErrors() {
		t.Fatal("expected HideSpellingErrors = true")
	}

	d.SetDoNotValidateAgainstSchema(true)
	if !d.DoNotValidateAgainstSchema() {
		t.Fatal("expected DoNotValidateAgainstSchema = true")
	}

	d.SetSuppressTopSpacing(true)
	if !d.SuppressTopSpacing() {
		t.Fatal("expected SuppressTopSpacing = true")
	}

	// string fields
	d.SetDecimalSymbol(",")
	if d.DecimalSymbol() != "," {
		t.Fatalf("DecimalSymbol = %q", d.DecimalSymbol())
	}

	d.SetStylePaneSortMethod("alpha")
	if d.StylePaneSortMethod() != "alpha" {
		t.Fatalf("StylePaneSortMethod = %q", d.StylePaneSortMethod())
	}

	// Verify MiscSettings was created.
	if d.IR.MiscSettings == nil {
		t.Fatal("expected MiscSettings to be created")
	}
}