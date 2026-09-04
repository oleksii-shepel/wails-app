package edit_test

import (
	"bytes"
	"testing"

	"office/pkg/sheet/edit"
	"office/pkg/sheet/parsers/ooxml/formula"
)

func TestExternalLinkEditEvaluateAndRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	main, err := wb.CreateSheet("Main")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := main.CreateCell(1, 1); err != nil {
		t.Fatal(err)
	}

	if _, err := wb.SetExternalCachedCell("external.xlsx", "Prices", "A1", "n", "7"); err != nil {
		t.Fatalf("SetExternalCachedCell: %v", err)
	}
	sheetID := 0
	if _, err := wb.SetExternalDefinedName("external.xlsx", "ExtPrice", "Prices!$A$1", &sheetID); err != nil {
		t.Fatalf("SetExternalDefinedName: %v", err)
	}

	got, err := wb.EvaluateFormula("Main", "'[1]Prices'!A1*2", nil)
	if err != nil {
		t.Fatalf("EvaluateFormula: %v", err)
	}
	if got.Type != formula.TypeNumber || got.Num != 14 {
		t.Fatalf("evaluation = %#v, want 14", got)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	roundTrip := resolveWorkbook(t, buf.Bytes())
	if len(roundTrip.ExternalLinks) != 1 {
		t.Fatalf("external links = %d, want 1", len(roundTrip.ExternalLinks))
	}
	link := roundTrip.ExternalLinks[0]
	if link.Target != "external.xlsx" || len(link.DefinedNames) != 1 || link.DefinedNames[0].Name != "ExtPrice" {
		t.Fatalf("external link metadata = %#v", link)
	}
	if len(link.CachedSheets) != 1 || len(link.CachedSheets[0].Cells) != 1 || link.CachedSheets[0].Cells[0].Value != "7" {
		t.Fatalf("external cache = %#v", link.CachedSheets)
	}
}

func TestExternalLinkEditRemoveAndClearCache(t *testing.T) {
	wb := edit.NewWorkbook()
	if _, err := wb.CreateSheet("Main"); err != nil {
		t.Fatal(err)
	}
	if _, err := wb.SetExternalCachedCell("external.xlsx", "Prices", "A1", "n", "7"); err != nil {
		t.Fatal(err)
	}
	if !wb.ClearExternalCachedSheet("external.xlsx", "Prices") {
		t.Fatal("expected cache clear")
	}
	link, ok := wb.ExternalLink("external.xlsx")
	if !ok || len(link.CachedSheets) != 1 || len(link.CachedSheets[0].Cells) != 0 {
		t.Fatalf("link after cache clear = %#v", link)
	}
	if !wb.RemoveExternalLink("external.xlsx") || len(wb.ExternalLinks()) != 0 {
		t.Fatal("expected external link removal")
	}
}
