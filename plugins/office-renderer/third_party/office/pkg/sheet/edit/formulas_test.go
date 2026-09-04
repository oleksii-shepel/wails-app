package edit_test

import (
	"bytes"
	"testing"

	"office/pkg/sheet/edit"
)

func TestSheetArrayFormulaRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet: %v", err)
	}
	group, err := sheet.SetArrayFormula("A1:B1", "ROW(A1:B1)")
	if err != nil {
		t.Fatalf("SetArrayFormula: %v", err)
	}
	if group.AnchorRef != "A1" || group.RangeRef != "A1:B1" {
		t.Fatalf("array group = %#v", group)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	gotSheet := roundTrip.Sheets[0]
	if len(gotSheet.ArrayFormulas) != 1 {
		t.Fatalf("array formulas = %d, want 1", len(gotSheet.ArrayFormulas))
	}
	got := gotSheet.ArrayFormulas[0]
	if got.AnchorRef != "A1" || got.RangeRef != "A1:B1" || got.Formula != "ROW(A1:B1)" {
		t.Errorf("array formula = %#v", got)
	}
	member := findCell(t, gotSheet, 1, 2)
	if member.ArrayFormulaAnchor != "A1" || member.ArrayFormulaRange != "A1:B1" || member.Formula != "ROW(A1:B1)" {
		t.Errorf("array member = %#v", member)
	}
}

func TestSheetSharedFormulaRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet: %v", err)
	}
	group, err := sheet.SetSharedFormula("A1:A3", "B1+1")
	if err != nil {
		t.Fatalf("SetSharedFormula: %v", err)
	}
	if group.AnchorRef != "A1" || group.RangeRef != "A1:A3" || group.Index != 0 {
		t.Fatalf("shared group = %#v", group)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	gotSheet := roundTrip.Sheets[0]
	if len(gotSheet.SharedFormulas) != 1 {
		t.Fatalf("shared formulas = %d, want 1", len(gotSheet.SharedFormulas))
	}
	if gotSheet.SharedFormulas[0].RangeRef != "A1:A3" || gotSheet.SharedFormulas[0].Formula != "B1+1" {
		t.Errorf("shared formula group = %#v", gotSheet.SharedFormulas[0])
	}
	if findCell(t, gotSheet, 1, 1).Formula != "B1+1" {
		t.Errorf("A1 formula = %q", findCell(t, gotSheet, 1, 1).Formula)
	}
	if findCell(t, gotSheet, 2, 1).Formula != "B2+1" {
		t.Errorf("A2 formula = %q", findCell(t, gotSheet, 2, 1).Formula)
	}
	if findCell(t, gotSheet, 3, 1).Formula != "B3+1" {
		t.Errorf("A3 formula = %q", findCell(t, gotSheet, 3, 1).Formula)
	}
}

func TestSheetFormulaGroupClearing(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet: %v", err)
	}
	if _, err := sheet.SetArrayFormula("A1:A2", "ROW(A1:A2)"); err != nil {
		t.Fatalf("SetArrayFormula: %v", err)
	}
	if !sheet.ClearArrayFormula("A1") {
		t.Fatalf("ClearArrayFormula returned false")
	}
	if len(wb.IR().Sheets[0].ArrayFormulas) != 0 {
		t.Fatalf("array groups = %d, want 0", len(wb.IR().Sheets[0].ArrayFormulas))
	}
	if _, err := sheet.SetSharedFormula("B1:B2", "C1+1"); err != nil {
		t.Fatalf("SetSharedFormula: %v", err)
	}
	if !sheet.ClearSharedFormula("B1") {
		t.Fatalf("ClearSharedFormula returned false")
	}
	if len(wb.IR().Sheets[0].SharedFormulas) != 0 {
		t.Fatalf("shared groups = %d, want 0", len(wb.IR().Sheets[0].SharedFormulas))
	}
}

func TestOpenXLSXExistingWorkbookMixedFormulaRoundTrip(t *testing.T) {
	wb := openFixtureWorkbook(t, "sample3.xlsx")
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if len(wb.IR().Sheets[0].SharedFormulas) == 0 {
		t.Fatal("expected existing shared formula group in sample3.xlsx")
	}

	regular, ok := sheet.Cell(2, 1)
	if !ok {
		t.Fatal("expected existing regular formula cell A2")
	}
	regular.SetFormula(`IF(B2="","",42)`, "")

	group, err := sheet.SetSharedFormula("D3:D13", `UPPER(B3)`)
	if err != nil {
		t.Fatalf("SetSharedFormula: %v", err)
	}
	if group.AnchorRef != "D3" || group.RangeRef != "D3:D13" {
		t.Fatalf("shared group = %#v", group)
	}

	arrayGroup, err := sheet.SetArrayFormula("J2:K2", "COLUMN(J2:K2)")
	if err != nil {
		t.Fatalf("SetArrayFormula: %v", err)
	}
	if arrayGroup.AnchorRef != "J2" || arrayGroup.RangeRef != "J2:K2" {
		t.Fatalf("array group = %#v", arrayGroup)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	gotSheet := roundTrip.Sheets[0]

	gotRegular := findCell(t, gotSheet, 2, 1)
	if gotRegular.Formula != `IF(B2="","",42)` {
		t.Fatalf("A2 formula = %q, want IF(B2=\"\",\"\",42)", gotRegular.Formula)
	}

	if len(gotSheet.SharedFormulas) != 1 {
		t.Fatalf("shared formulas = %d, want 1", len(gotSheet.SharedFormulas))
	}
	gotShared := gotSheet.SharedFormulas[0]
	if gotShared.AnchorRef != "D3" || gotShared.RangeRef != "D3:D13" || gotShared.Formula != "UPPER(B3)" {
		t.Fatalf("shared formula group = %#v", gotShared)
	}
	if findCell(t, gotSheet, 3, 4).Formula != "UPPER(B3)" {
		t.Fatalf("D3 formula = %q, want UPPER(B3)", findCell(t, gotSheet, 3, 4).Formula)
	}
	if findCell(t, gotSheet, 4, 4).Formula != "UPPER(B4)" {
		t.Fatalf("D4 formula = %q, want UPPER(B4)", findCell(t, gotSheet, 4, 4).Formula)
	}

	if len(gotSheet.ArrayFormulas) != 1 {
		t.Fatalf("array formulas = %d, want 1", len(gotSheet.ArrayFormulas))
	}
	gotArray := gotSheet.ArrayFormulas[0]
	if gotArray.AnchorRef != "J2" || gotArray.RangeRef != "J2:K2" || gotArray.Formula != "COLUMN(J2:K2)" {
		t.Fatalf("array formula group = %#v", gotArray)
	}
	if findCell(t, gotSheet, 2, 11).ArrayFormulaAnchor != "J2" || findCell(t, gotSheet, 2, 11).Formula != "COLUMN(J2:K2)" {
		t.Fatalf("K2 array member = %#v", findCell(t, gotSheet, 2, 11))
	}
}
