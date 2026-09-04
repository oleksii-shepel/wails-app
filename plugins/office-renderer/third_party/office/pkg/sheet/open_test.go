package sheet_test

import (
	"bytes"
	"strings"
	"testing"

	"office/pkg/sheet"
)

func TestHighLevelFacadeOpenEditAndDetectXLSX(t *testing.T) {
	editable := sheet.New()
	ws, err := editable.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet: %v", err)
	}
	cell, err := ws.CreateCell(1, 1)
	if err != nil {
		t.Fatalf("CreateCell: %v", err)
	}
	cell.SetString("hello")

	var buf bytes.Buffer
	if err := editable.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	if got := sheet.DetectFormat(buf.Bytes()); got != sheet.FormatXLSX {
		t.Fatalf("DetectFormat = %q, want %q", got, sheet.FormatXLSX)
	}
	wb, err := sheet.OpenBytes(buf.Bytes(), "book.xlsx")
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	if len(wb.Sheets) != 1 || wb.Sheets[0].Name != "Data" {
		t.Fatalf("resolved sheets = %#v", wb.Sheets)
	}
	html, err := sheet.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, "hello") {
		t.Fatalf("HTML output does not contain cell text: %q", html)
	}

	reopened, err := sheet.Edit(wb)
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	gotSheet, ok := reopened.SheetByName("Data")
	if !ok {
		t.Fatal("expected editable Data sheet")
	}
	gotCell, ok := gotSheet.Cell(1, 1)
	if !ok || gotCell.Ref() != "A1" {
		t.Fatalf("A1 = %#v, ok=%v", gotCell, ok)
	}
}

func TestSpreadsheetExtensionPolicy(t *testing.T) {
	for _, name := range []string{"a.xlsx", "a.xlsm", "a.xltx", "a.xltm", "a.xls"} {
		if !sheet.IsSpreadsheetExtension(name) {
			t.Errorf("IsSpreadsheetExtension(%q) = false", name)
		}
	}
	if sheet.IsSpreadsheetExtension("a.csv") {
		t.Fatal("CSV should not be classified as a supported Office spreadsheet package")
	}
}
