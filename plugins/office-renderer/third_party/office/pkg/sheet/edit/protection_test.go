package edit_test

import (
	"bytes"
	"testing"

	"office/pkg/sheet/edit"
	sheetir "office/pkg/sheet/ir"
)

func TestWorkbookProtectionRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}

	workbookProtection := sheetir.WorkbookProtection{
		WorkbookPassword: "ABCD",
		LockStructure:    true,
		LockWindows:      true,
		AlgorithmName:    "SHA-512",
		HashValue:        "deadbeef",
		SaltValue:        "c2FsdA==",
		SpinCount:        100000,
	}
	gotWorkbookProtection := wb.SetWorkbookProtection(workbookProtection)
	if gotWorkbookProtection == nil || !gotWorkbookProtection.LockStructure || gotWorkbookProtection.HashValue != "deadbeef" {
		t.Fatalf("workbook protection = %#v", gotWorkbookProtection)
	}
	workbookProtection.HashValue = "mutated"
	if wb.WorkbookProtection().HashValue != "deadbeef" {
		t.Fatal("SetWorkbookProtection should copy input")
	}

	sheetProtection := sheetir.SheetProtection{
		Password:            "C0DE",
		Sheet:               true,
		Objects:             true,
		Scenarios:           true,
		FormatColumns:       true,
		InsertRows:          true,
		Sort:                true,
		AutoFilter:          true,
		SelectLockedCells:   true,
		SelectUnlockedCells: true,
		AlgorithmName:       "SHA-512",
		HashValue:           "beadfeed",
		SaltValue:           "c2hlZXQ=",
		SpinCount:           25000,
	}
	gotSheetProtection := sheet.SetSheetProtection(sheetProtection)
	if gotSheetProtection == nil || !gotSheetProtection.Sheet || gotSheetProtection.HashValue != "beadfeed" {
		t.Fatalf("sheet protection = %#v", gotSheetProtection)
	}
	sheetProtection.HashValue = "mutated"
	if sheet.SheetProtection().HashValue != "beadfeed" {
		t.Fatal("SetSheetProtection should copy input")
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	if roundTrip.Protection == nil {
		t.Fatal("expected workbook protection after round-trip")
	}
	if !roundTrip.Protection.LockStructure || !roundTrip.Protection.LockWindows || roundTrip.Protection.AlgorithmName != "SHA-512" || roundTrip.Protection.SpinCount != 100000 {
		t.Fatalf("unexpected workbook protection: %+v", roundTrip.Protection)
	}
	if roundTrip.Protection.WorkbookPassword != "ABCD" || roundTrip.Protection.HashValue != "deadbeef" || roundTrip.Protection.SaltValue != "c2FsdA==" {
		t.Fatalf("unexpected workbook protection hashing fields: %+v", roundTrip.Protection)
	}
	if len(roundTrip.Sheets) != 1 || roundTrip.Sheets[0].Protection == nil {
		t.Fatalf("expected worksheet protection after round-trip, got %+v", roundTrip.Sheets)
	}
	if sp := roundTrip.Sheets[0].Protection; sp.Password != "C0DE" || !sp.Sheet || !sp.Objects || !sp.AutoFilter || sp.HashValue != "beadfeed" || sp.SpinCount != 25000 {
		t.Fatalf("unexpected worksheet protection: %+v", sp)
	}
}

func TestWorkbookAndSheetProtectionMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}

	if wb.WorkbookProtection() != nil {
		t.Fatal("expected nil workbook protection initially")
	}
	wp := wb.EnsureWorkbookProtection()
	if wp == nil {
		t.Fatal("expected EnsureWorkbookProtection to allocate metadata")
	}
	wp.LockRevision = true
	if wb.WorkbookProtection() == nil || !wb.WorkbookProtection().LockRevision {
		t.Fatalf("workbook protection = %#v", wb.WorkbookProtection())
	}
	wb.ClearWorkbookProtection()
	if wb.WorkbookProtection() != nil {
		t.Fatal("expected workbook protection to be cleared")
	}

	if sheet.SheetProtection() != nil {
		t.Fatal("expected nil sheet protection initially")
	}
	sp := sheet.EnsureSheetProtection()
	if sp == nil {
		t.Fatal("expected EnsureSheetProtection to allocate metadata")
	}
	sp.DeleteRows = true
	sp.PivotTables = true
	if sheet.SheetProtection() == nil || !sheet.SheetProtection().DeleteRows || !sheet.SheetProtection().PivotTables {
		t.Fatalf("sheet protection = %#v", sheet.SheetProtection())
	}
	sheet.ClearSheetProtection()
	if sheet.SheetProtection() != nil {
		t.Fatal("expected sheet protection to be cleared")
	}
}
