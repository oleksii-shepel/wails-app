package edit_test

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"office/pkg/sheet/edit"
	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

func TestCellStyleRegistryRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	format, err := wb.RegisterDataFormat("$#,##0.00")
	if err != nil {
		t.Fatalf("RegisterDataFormat: %v", err)
	}
	style, err := wb.CreateCellStyle("Money")
	if err != nil {
		t.Fatalf("CreateCellStyle: %v", err)
	}
	style.SetFont("Aptos", 14, "FF0000", true, true, false, false).
		SetFillColor("FFFF00").
		SetBorders(
			sheetir.BorderStyle{Style: "thin", Color: "000000"},
			sheetir.BorderStyle{Style: "thin", Color: "000000"},
			sheetir.BorderStyle{Style: "double", Color: "000000"},
			sheetir.BorderStyle{Style: "thin", Color: "000000"},
		).
		SetAlignment("center", "middle", true, 45).
		SetNumberFormat(format).
		SetProtection(true, true)

	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet: %v", err)
	}
	cell, err := sheet.CreateCell(2, 2)
	if err != nil {
		t.Fatalf("CreateCell: %v", err)
	}
	cell.SetNumber(12.5)
	if err := cell.ApplyStyle(style); err != nil {
		t.Fatalf("ApplyStyle: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	got := findCell(t, roundTrip.Sheets[0], 2, 2)
	if got.Style.NumFmtCode != "$#,##0.00" {
		t.Errorf("NumFmtCode = %q", got.Style.NumFmtCode)
	}
	if !got.Style.Bold || !got.Style.Italic || got.Style.FontFamily != "Aptos" || got.Style.FontSizePt != 14 {
		t.Errorf("font style = %#v", got.Style)
	}
	if got.Style.FontColor != "FF0000" || got.Style.BgColor != "FFFF00" {
		t.Errorf("colors = font %q bg %q", got.Style.FontColor, got.Style.BgColor)
	}
	if got.Style.HAlign != "center" || got.Style.VAlign != "middle" || !got.Style.WrapText || got.Style.TextRotation != 45 {
		t.Errorf("alignment = %#v", got.Style)
	}
	if got.Style.BorderBottom.Style != "double" {
		t.Errorf("bottom border = %#v", got.Style.BorderBottom)
	}
	if !got.Style.Locked || !got.Style.FormulaHidden {
		t.Errorf("protection = locked %v hidden %v, want true/true", got.Style.Locked, got.Style.FormulaHidden)
	}
}

func TestCellStyleRegistryCloneAndReuse(t *testing.T) {
	wb := edit.NewWorkbook()
	style, err := wb.CreateOrReuseCellStyle("Base", sheetir.CellStyle{Bold: true})
	if err != nil {
		t.Fatalf("CreateOrReuseCellStyle: %v", err)
	}
	same, err := wb.CreateOrReuseCellStyle("Duplicate", sheetir.CellStyle{Bold: true})
	if err != nil {
		t.Fatalf("CreateOrReuseCellStyle duplicate: %v", err)
	}
	if same != style {
		t.Fatalf("expected equal style to be reused")
	}
	clone, err := wb.CloneCellStyle("Base", "Base Copy")
	if err != nil {
		t.Fatalf("CloneCellStyle: %v", err)
	}
	if clone.Name() != "Base" {
		t.Fatalf("equal clone should reuse Base, got %q", clone.Name())
	}
	if len(wb.CellStyles()) != 1 {
		t.Fatalf("style count = %d, want 1", len(wb.CellStyles()))
	}
	if !wb.RemoveCellStyle("Base") {
		t.Fatalf("RemoveCellStyle returned false")
	}
}

func TestOpenXLSXSeedsExistingStyleRegistries(t *testing.T) {
	wb := openFixtureWorkbook(t, "tableStyle.xlsx")
	if wb == nil {
		t.Fatal("failed to open fixture workbook")
	}
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if _, ok := sheet.Cell(2, 2); !ok {
		t.Fatal("expected existing styled cell at B2")
	}
	cell := findCell(t, wb.IR().Sheets[0], 2, 2)
	if _, ok := wb.CellStyleByValue(cell.Style); !ok {
		t.Fatalf("expected existing cell style to be seeded into registry: %#v", cell.Style)
	}
	if len(wb.CellStyles()) == 0 {
		t.Fatal("expected seeded cell styles for existing workbook")
	}
	if cell.Style.NumFmtCode != "" {
		if _, ok := wb.DataFormat(cell.Style.NumFmtCode); !ok {
			t.Fatalf("expected existing number format %q to be seeded", cell.Style.NumFmtCode)
		}
	}
}

func TestOpenXLSXExistingWorkbookTableAndFormulaRoundTrip(t *testing.T) {
	wb := openFixtureWorkbook(t, "ExcelTables.xlsx")
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if _, ok := sheet.Table("TableName"); !ok {
		t.Fatal("expected existing structured table TableName")
	}
	if err := sheet.SetTableStyle("TableName", "TableStyleMedium2", false, false, true, false); err != nil {
		t.Fatalf("SetTableStyle: %v", err)
	}
	formulaCell, ok := sheet.Cell(2, 3)
	if !ok {
		t.Fatal("expected existing formula cell C2")
	}
	formulaCell.SetFormula("INT(B2/5)", "")

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	gotSheet := roundTrip.Sheets[0]
	if len(gotSheet.Tables) == 0 {
		t.Fatal("expected structured table after round-trip")
	}
	gotTable := gotSheet.Tables[0]
	if gotTable.Name != "TableName" || gotTable.TableStyle == nil || gotTable.TableStyle.Name != "TableStyleMedium2" {
		t.Fatalf("table round-trip = %#v", gotTable)
	}
	gotFormula := findCell(t, gotSheet, 2, 3)
	if gotFormula.Formula != "INT(B2/5)" {
		t.Fatalf("formula round-trip = %q, want INT(B2/5)", gotFormula.Formula)
	}
}

func TestCellStyleNamedStyleTableRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	style, err := wb.CreateCellStyle("Money")
	if err != nil {
		t.Fatalf("CreateCellStyle: %v", err)
	}
	if _, err := style.SetNumberFormatCode("$#,##0.00"); err != nil {
		t.Fatalf("SetNumberFormatCode: %v", err)
	}
	style.SetFont("Aptos", 12, "008000", true, false, false, false)

	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet: %v", err)
	}
	cell, err := sheet.CreateCell(2, 2)
	if err != nil {
		t.Fatalf("CreateCell: %v", err)
	}
	cell.SetNumber(42)
	if err := cell.ApplyStyle(style); err != nil {
		t.Fatalf("ApplyStyle: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	styles := readStyleSheetFromXLSX(t, buf.Bytes())
	if len(styles.CellStyleXfs) < 2 {
		t.Fatalf("cellStyleXfs = %d, want at least 2", len(styles.CellStyleXfs))
	}
	if len(styles.CellStyles) < 2 {
		t.Fatalf("cellStyles = %d, want at least 2", len(styles.CellStyles))
	}
	var money *sheetparts.CellStyle
	for i := range styles.CellStyles {
		if styles.CellStyles[i].Name == "Money" {
			money = &styles.CellStyles[i]
			break
		}
	}
	if money == nil {
		t.Fatalf("named styles = %#v, want Money", styles.CellStyles)
	}
	if money.XfID <= 0 || money.XfID >= len(styles.CellStyleXfs) {
		t.Fatalf("Money xfId = %d, want style xf index in [1,%d)", money.XfID, len(styles.CellStyleXfs))
	}
	if len(styles.CellXfs) < 2 || styles.CellXfs[1].XfID != money.XfID {
		t.Fatalf("cell xf linkage = %#v, want xfId %d", styles.CellXfs, money.XfID)
	}

	reopened, err := edit.OpenXLSX(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("OpenXLSX: %v", err)
	}
	if _, ok := reopened.CellStyle("Money"); !ok {
		t.Fatalf("expected Money style in reopened registry: %#v", reopened.CellStyles())
	}
	if _, ok := reopened.CellStyle("ImportedStyle1"); ok {
		t.Fatalf("expected reopened named styles to avoid ImportedStyle fallback: %#v", reopened.CellStyles())
	}
}

func openFixtureWorkbook(t *testing.T, name string) *edit.Workbook {
	t.Helper()
	path := repoPath(t, "testdata", "xlsx", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	wb, err := edit.OpenXLSX(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenXLSX(%s): %v", name, err)
	}
	return wb
}

func repoPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}

func readStyleSheetFromXLSX(t *testing.T, data []byte) *sheetparts.StyleSheet {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	for _, file := range zr.File {
		if file.Name != "xl/styles.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("styles.xml open: %v", err)
		}
		defer rc.Close()
		xmlData, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("styles.xml read: %v", err)
		}
		var styles sheetparts.StyleSheet
		if err := xml.Unmarshal(xmlData, &styles); err != nil {
			t.Fatalf("styles.xml unmarshal: %v", err)
		}
		return &styles
	}
	t.Fatal("xl/styles.xml not found")
	return nil
}
