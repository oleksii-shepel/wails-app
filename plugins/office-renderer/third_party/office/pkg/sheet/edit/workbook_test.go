package edit_test

import (
	"bytes"
	"testing"

	"office/pkg/ooxml/opc"
	"office/pkg/sheet/edit"
	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
	sheetresolve "office/pkg/sheet/parsers/ooxml/resolve"
)

func TestWorkbookEditSaveAsXLSXRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet: %v", err)
	}
	if err := sheet.FreezePanes(1, 1, "B2", "bottomRight"); err != nil {
		t.Fatalf("FreezePanes: %v", err)
	}
	if err := sheet.SetColumnWidth(2, 18.5); err != nil {
		t.Fatalf("SetColumnWidth: %v", err)
	}
	if err := sheet.SetVisibility("veryHidden"); err != nil {
		t.Fatalf("SetVisibility: %v", err)
	}
	if err := sheet.SetDefaultColumnWidthChars(11.25); err != nil {
		t.Fatalf("SetDefaultColumnWidthChars: %v", err)
	}
	if err := sheet.SetDefaultRowHeightPt(17.5); err != nil {
		t.Fatalf("SetDefaultRowHeightPt: %v", err)
	}
	if err := sheet.SetZoomScale(125); err != nil {
		t.Fatalf("SetZoomScale: %v", err)
	}
	sheet.SetGridLinesVisible(false)
	cell, err := sheet.CreateCell(1, 1)
	if err != nil {
		t.Fatalf("CreateCell A1: %v", err)
	}
	cell.SetString("Name")
	cell.SetComment("header")
	cell, err = sheet.CreateCell(2, 2)
	if err != nil {
		t.Fatalf("CreateCell B2: %v", err)
	}
	cell.SetNumber(42)
	cell.SetHyperlink("https://example.com")
	cell.SetStyle(sheetir.CellStyle{Bold: true, BgColor: "FFFF00"})
	if err := sheet.AddMergedRegion(4, 1, 4, 3); err != nil {
		t.Fatalf("AddMergedRegion: %v", err)
	}
	sheet.SetAutoFilter("A1:B2")
	sheet.SetPrintArea("$A$1:$B$20,$D$1:$D$5")
	sheet.SetPrintTitles("$1:$2", "$A:$B")

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	if len(roundTrip.Sheets) != 1 {
		t.Fatalf("sheet count = %d, want 1", len(roundTrip.Sheets))
	}
	got := roundTrip.Sheets[0]
	if got.Name != "Data" {
		t.Errorf("sheet name = %q, want Data", got.Name)
	}
	if !got.Hidden {
		t.Errorf("sheet hidden = %v, want true", got.Hidden)
	}
	if got.Visibility != "veryHidden" {
		t.Errorf("sheet visibility = %q, want veryHidden", got.Visibility)
	}
	cellA1 := findCell(t, got, 1, 1)
	if cellA1.StrValue != "Name" {
		t.Errorf("A1 value = %q, want Name", cellA1.StrValue)
	}
	cellB2 := findCell(t, got, 2, 2)
	if cellB2.NumValue != 42 {
		t.Errorf("B2 value = %v, want 42", cellB2.NumValue)
	}
	if cellB2.HyperlinkURL != "https://example.com" {
		t.Errorf("B2 hyperlink = %q", cellB2.HyperlinkURL)
	}
	if len(got.MergedCells) != 1 || got.MergedCells[0].RightCol != 3 {
		t.Fatalf("merged cells = %#v, want A4:C4", got.MergedCells)
	}
	if got.AutoFilterRef != "A1:B2" {
		t.Errorf("auto filter = %q, want A1:B2", got.AutoFilterRef)
	}
	if got.PrintArea != "$A$1:$B$20,$D$1:$D$5" {
		t.Errorf("print area = %q, want $A$1:$B$20,$D$1:$D$5", got.PrintArea)
	}
	if got.PrintTitleRows != "$1:$2" || got.PrintTitleCols != "$A:$B" {
		t.Errorf("print titles = %q / %q, want $1:$2 / $A:$B", got.PrintTitleRows, got.PrintTitleCols)
	}
	if got.DefaultColWidthChars != 11.25 {
		t.Errorf("default col width = %v, want 11.25", got.DefaultColWidthChars)
	}
	if got.DefaultRowHeightPt != 17.5 {
		t.Errorf("default row height = %v, want 17.5", got.DefaultRowHeightPt)
	}
	if got.ZoomScale != 125 {
		t.Errorf("zoom scale = %d, want 125", got.ZoomScale)
	}
	if got.ShowGridLines == nil || *got.ShowGridLines {
		t.Errorf("show gridlines = %v, want explicit false", got.ShowGridLines)
	}
	if got.FrozenRows != 1 || got.FrozenCols != 1 || got.TopLeftCell != "B2" {
		t.Errorf("freeze panes = rows %d cols %d top-left %q", got.FrozenRows, got.FrozenCols, got.TopLeftCell)
	}
	if got.Comments["A1"] != "header" {
		t.Errorf("A1 comment = %q, want header", got.Comments["A1"])
	}
}

func TestWorkbookSheetMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	first, err := wb.CreateSheet("First")
	if err != nil {
		t.Fatalf("CreateSheet First: %v", err)
	}
	cell, err := first.CreateCell(3, 2)
	if err != nil {
		t.Fatalf("CreateCell: %v", err)
	}
	cell.SetString("source")

	clone, err := wb.CloneSheet(0, "Copy")
	if err != nil {
		t.Fatalf("CloneSheet: %v", err)
	}
	if err := clone.SetName("Second"); err != nil {
		t.Fatalf("SetName: %v", err)
	}
	if err := wb.MoveSheet(1, 0); err != nil {
		t.Fatalf("MoveSheet: %v", err)
	}
	if wb.ActiveSheetIndex() != 1 {
		t.Fatalf("active sheet = %d, want 1 after moving clone before active source", wb.ActiveSheetIndex())
	}
	if err := wb.SetActiveSheet(0); err != nil {
		t.Fatalf("SetActiveSheet: %v", err)
	}
	if err := wb.SelectSheet(1, true); err != nil {
		t.Fatalf("SelectSheet: %v", err)
	}
	selected := wb.SelectedSheetIndexes()
	if len(selected) != 2 || selected[0] != 0 || selected[1] != 1 {
		t.Fatalf("selected = %v, want [0 1]", selected)
	}
	if err := wb.RemoveSheet(1); err != nil {
		t.Fatalf("RemoveSheet: %v", err)
	}
	if wb.SheetCount() != 1 {
		t.Fatalf("sheet count = %d, want 1", wb.SheetCount())
	}
	got, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if got.Name() != "Second" {
		t.Errorf("remaining sheet = %q, want Second", got.Name())
	}
	if _, _, _, lastCol := got.Dimensions(); lastCol != 2 {
		t.Errorf("last col = %d, want 2", lastCol)
	}
}

func TestSheetPrintTitleMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data Sheet")
	if err != nil {
		t.Fatalf("CreateSheet Data Sheet: %v", err)
	}
	rows, cols := sheet.PrintTitles()
	if rows != "" || cols != "" {
		t.Fatalf("initial print titles = %q / %q, want empty", rows, cols)
	}
	sheet.SetPrintTitles("$1:$3", "$A:$C")
	rows, cols = sheet.PrintTitles()
	if rows != "$1:$3" || cols != "$A:$C" {
		t.Fatalf("print titles = %q / %q, want $1:$3 / $A:$C", rows, cols)
	}
	sheet.ClearPrintTitles()
	rows, cols = sheet.PrintTitles()
	if rows != "" || cols != "" {
		t.Fatalf("cleared print titles = %q / %q, want empty", rows, cols)
	}
}

func TestSheetPrintAreaMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data Sheet")
	if err != nil {
		t.Fatalf("CreateSheet Data Sheet: %v", err)
	}
	if area := sheet.PrintArea(); area != "" {
		t.Fatalf("initial print area = %q, want empty", area)
	}
	sheet.SetPrintArea("$A$1:$D$20,$F$1:$F$10")
	if area := sheet.PrintArea(); area != "$A$1:$D$20,$F$1:$F$10" {
		t.Fatalf("print area = %q, want $A$1:$D$20,$F$1:$F$10", area)
	}
	sheet.ClearPrintArea()
	if area := sheet.PrintArea(); area != "" {
		t.Fatalf("cleared print area = %q, want empty", area)
	}
}

func TestSheetPageSetupMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data Sheet")
	if err != nil {
		t.Fatalf("CreateSheet Data Sheet: %v", err)
	}
	if got := sheet.PageSetup(); got != nil {
		t.Fatalf("initial page setup = %+v, want nil", got)
	}
	setup := &sheetir.PageSetup{
		PaperSize:          9,
		Orientation:        "landscape",
		Scale:              85,
		FitToPage:          true,
		FitToWidth:         1,
		FitToHeight:        2,
		MarginLeft:         0.5,
		MarginRight:        0.75,
		MarginTop:          1,
		MarginBottom:       1.25,
		MarginHeader:       0.3,
		MarginFooter:       0.4,
		CenterHorizontally: true,
		CenterVertically:   true,
		PrintHeadings:      true,
		PrintGridlines:     true,
	}
	sheet.SetPageSetup(setup)
	got := sheet.PageSetup()
	if got == nil || got.PaperSize != 9 || got.Orientation != "landscape" || !got.FitToPage || !got.CenterHorizontally || !got.PrintGridlines {
		t.Fatalf("page setup = %+v, want populated copy", got)
	}
	got.Scale = 60
	if sheet.PageSetup().Scale != 85 {
		t.Fatalf("page setup getter returned live pointer")
	}
	sheet.ClearPageSetup()
	if got := sheet.PageSetup(); got != nil {
		t.Fatalf("cleared page setup = %+v, want nil", got)
	}
}

func TestSheetHeaderFooterMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data Sheet")
	if err != nil {
		t.Fatalf("CreateSheet Data Sheet: %v", err)
	}
	if got := sheet.HeaderFooter(); got != nil {
		t.Fatalf("initial header/footer = %+v, want nil", got)
	}
	headerFooter := &sheetir.HeaderFooter{
		OddHeader:        "&LLeft&CQuarterly Report&RPage &P",
		OddFooter:        "&RConfidential",
		EvenHeader:       "Even Header",
		EvenFooter:       "Even Footer",
		FirstHeader:      "First Header",
		FirstFooter:      "First Footer",
		DifferentOddEven: true,
		DifferentFirst:   true,
		Images: []sheetir.HFImage{
			{ContentType: "image/png", Data: []byte{0x01, 0x02, 0x03}},
		},
	}
	sheet.SetHeaderFooter(headerFooter)
	got := sheet.HeaderFooter()
	if got == nil || got.OddHeader != headerFooter.OddHeader || !got.DifferentOddEven || len(got.Images) != 1 {
		t.Fatalf("header/footer = %+v, want populated copy", got)
	}
	got.Images[0].Data[0] = 0xFF
	if sheet.HeaderFooter().Images[0].Data[0] != 0x01 {
		t.Fatalf("header/footer getter returned live image bytes")
	}
	sheet.ClearHeaderFooter()
	if got := sheet.HeaderFooter(); got != nil {
		t.Fatalf("cleared header/footer = %+v, want nil", got)
	}
}

func TestSheetPageBreakMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data Sheet")
	if err != nil {
		t.Fatalf("CreateSheet Data Sheet: %v", err)
	}
	if err := sheet.AddRowPageBreak(20); err != nil {
		t.Fatalf("AddRowPageBreak(20): %v", err)
	}
	if err := sheet.AddRowPageBreak(5); err != nil {
		t.Fatalf("AddRowPageBreak(5): %v", err)
	}
	if err := sheet.AddRowPageBreak(20); err != nil {
		t.Fatalf("AddRowPageBreak(20 dup): %v", err)
	}
	if err := sheet.AddColumnPageBreak(8); err != nil {
		t.Fatalf("AddColumnPageBreak(8): %v", err)
	}
	if err := sheet.AddColumnPageBreak(3); err != nil {
		t.Fatalf("AddColumnPageBreak(3): %v", err)
	}
	if got := sheet.RowPageBreaks(); len(got) != 2 || got[0] != 5 || got[1] != 20 {
		t.Fatalf("row page breaks = %v, want [5 20]", got)
	}
	if got := sheet.ColumnPageBreaks(); len(got) != 2 || got[0] != 3 || got[1] != 8 {
		t.Fatalf("column page breaks = %v, want [3 8]", got)
	}
	if !sheet.RemoveRowPageBreak(5) {
		t.Fatal("RemoveRowPageBreak(5) = false, want true")
	}
	if sheet.RemoveRowPageBreak(5) {
		t.Fatal("RemoveRowPageBreak(5) second call = true, want false")
	}
	if !sheet.RemoveColumnPageBreak(8) {
		t.Fatal("RemoveColumnPageBreak(8) = false, want true")
	}
	if sheet.RemoveColumnPageBreak(8) {
		t.Fatal("RemoveColumnPageBreak(8) second call = true, want false")
	}
	if got := sheet.RowPageBreaks(); len(got) != 1 || got[0] != 20 {
		t.Fatalf("row page breaks after remove = %v, want [20]", got)
	}
	if got := sheet.ColumnPageBreaks(); len(got) != 1 || got[0] != 3 {
		t.Fatalf("column page breaks after remove = %v, want [3]", got)
	}
	sheet.ClearRowPageBreaks()
	sheet.ClearColumnPageBreaks()
	if got := sheet.RowPageBreaks(); got != nil {
		t.Fatalf("cleared row page breaks = %v, want nil", got)
	}
	if got := sheet.ColumnPageBreaks(); got != nil {
		t.Fatalf("cleared column page breaks = %v, want nil", got)
	}
}

func TestWorkbookEditPageMetadataRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Print Sheet")
	if err != nil {
		t.Fatalf("CreateSheet Print Sheet: %v", err)
	}
	sheet.SetPageSetup(&sheetir.PageSetup{
		PaperSize:          9,
		Orientation:        "landscape",
		FitToPage:          true,
		FitToWidth:         1,
		FitToHeight:        0,
		Scale:              90,
		MarginLeft:         0.5,
		MarginRight:        0.75,
		MarginTop:          1,
		MarginBottom:       1.25,
		MarginHeader:       0.3,
		MarginFooter:       0.4,
		CenterHorizontally: true,
		PrintHeadings:      true,
		PrintGridlines:     true,
	})
	sheet.SetHeaderFooter(&sheetir.HeaderFooter{
		OddHeader:        "&LLeft&CReport&RPage &P",
		OddFooter:        "&RConfidential",
		FirstHeader:      "First Page",
		FirstFooter:      "First Footer",
		DifferentFirst:   true,
		DifferentOddEven: false,
	})
	if err := sheet.AddRowPageBreak(15); err != nil {
		t.Fatalf("AddRowPageBreak: %v", err)
	}
	if err := sheet.AddColumnPageBreak(4); err != nil {
		t.Fatalf("AddColumnPageBreak: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	roundTrip := resolveWorkbook(t, buf.Bytes())
	got := roundTrip.Sheets[0]
	if got.PageSetup == nil || got.PageSetup.PaperSize != 9 || got.PageSetup.Orientation != "landscape" || !got.PageSetup.FitToPage || !got.PageSetup.PrintGridlines {
		t.Fatalf("page setup round-trip = %+v", got.PageSetup)
	}
	if got.HeaderFooter == nil || got.HeaderFooter.OddHeader != "&LLeft&CReport&RPage &P" || !got.HeaderFooter.DifferentFirst {
		t.Fatalf("header/footer round-trip = %+v", got.HeaderFooter)
	}
	if len(got.RowPageBreaks) != 1 || got.RowPageBreaks[0] != 15 {
		t.Fatalf("row page breaks round-trip = %v, want [15]", got.RowPageBreaks)
	}
	if len(got.ColPageBreaks) != 1 || got.ColPageBreaks[0] != 4 {
		t.Fatalf("column page breaks round-trip = %v, want [4]", got.ColPageBreaks)
	}
}

func TestCloneSheetClonesPageMetadata(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Source")
	if err != nil {
		t.Fatalf("CreateSheet Source: %v", err)
	}
	sheet.SetPageSetup(&sheetir.PageSetup{Scale: 90, Orientation: "landscape"})
	sheet.SetHeaderFooter(&sheetir.HeaderFooter{OddHeader: "Original"})

	clone, err := wb.CloneSheet(0, "Copy")
	if err != nil {
		t.Fatalf("CloneSheet: %v", err)
	}
	ps := clone.PageSetup()
	ps.Scale = 50
	hf := clone.HeaderFooter()
	hf.OddHeader = "Clone"
	clone.SetPageSetup(ps)
	clone.SetHeaderFooter(hf)

	orig, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if orig.PageSetup() == nil || orig.PageSetup().Scale != 90 {
		t.Fatalf("original page setup = %+v, want scale 90", orig.PageSetup())
	}
	if orig.HeaderFooter() == nil || orig.HeaderFooter().OddHeader != "Original" {
		t.Fatalf("original header/footer = %+v, want Original", orig.HeaderFooter())
	}
}

func TestSheetDefaultSizingMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data Sheet")
	if err != nil {
		t.Fatalf("CreateSheet Data Sheet: %v", err)
	}
	if got := sheet.DefaultColumnWidthChars(); got != 0 {
		t.Fatalf("initial default col width = %v, want 0", got)
	}
	if got := sheet.DefaultRowHeightPt(); got != 0 {
		t.Fatalf("initial default row height = %v, want 0", got)
	}
	if err := sheet.SetDefaultColumnWidthChars(12.5); err != nil {
		t.Fatalf("SetDefaultColumnWidthChars: %v", err)
	}
	if err := sheet.SetDefaultRowHeightPt(18); err != nil {
		t.Fatalf("SetDefaultRowHeightPt: %v", err)
	}
	if got := sheet.DefaultColumnWidthChars(); got != 12.5 {
		t.Fatalf("default col width = %v, want 12.5", got)
	}
	if got := sheet.DefaultRowHeightPt(); got != 18 {
		t.Fatalf("default row height = %v, want 18", got)
	}
	if err := sheet.SetDefaultColumnWidthChars(-1); err == nil {
		t.Fatal("expected negative default column width to fail")
	}
	if err := sheet.SetDefaultRowHeightPt(-1); err == nil {
		t.Fatal("expected negative default row height to fail")
	}
}

func TestSheetHiddenMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data Sheet")
	if err != nil {
		t.Fatalf("CreateSheet Data Sheet: %v", err)
	}
	if sheet.Hidden() {
		t.Fatal("initial hidden = true, want false")
	}
	sheet.SetHidden(true)
	if !sheet.Hidden() {
		t.Fatal("hidden = false, want true")
	}
	if got := sheet.Visibility(); got != "hidden" {
		t.Fatalf("visibility = %q, want hidden", got)
	}
	sheet.SetHidden(false)
	if sheet.Hidden() {
		t.Fatal("hidden = true, want false after clear")
	}
	if got := sheet.Visibility(); got != "" {
		t.Fatalf("visibility = %q, want empty after clear", got)
	}
}

func TestSheetVisibilityMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data Sheet")
	if err != nil {
		t.Fatalf("CreateSheet Data Sheet: %v", err)
	}
	if got := sheet.Visibility(); got != "" {
		t.Fatalf("initial visibility = %q, want empty", got)
	}
	if err := sheet.SetVisibility("veryHidden"); err != nil {
		t.Fatalf("SetVisibility veryHidden: %v", err)
	}
	if !sheet.Hidden() {
		t.Fatal("hidden = false, want true for veryHidden")
	}
	if got := sheet.Visibility(); got != "veryHidden" {
		t.Fatalf("visibility = %q, want veryHidden", got)
	}
	if err := sheet.SetVisibility("visible"); err != nil {
		t.Fatalf("SetVisibility visible: %v", err)
	}
	if sheet.Hidden() {
		t.Fatal("hidden = true, want false after visible")
	}
	if got := sheet.Visibility(); got != "" {
		t.Fatalf("visibility = %q, want empty after visible", got)
	}
	if err := sheet.SetVisibility("sideways"); err == nil {
		t.Fatal("expected invalid visibility to fail")
	}
}

func TestSheetZoomScaleMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data Sheet")
	if err != nil {
		t.Fatalf("CreateSheet Data Sheet: %v", err)
	}
	if got := sheet.ZoomScale(); got != 0 {
		t.Fatalf("initial zoom scale = %d, want 0", got)
	}
	if err := sheet.SetZoomScale(140); err != nil {
		t.Fatalf("SetZoomScale: %v", err)
	}
	if got := sheet.ZoomScale(); got != 140 {
		t.Fatalf("zoom scale = %d, want 140", got)
	}
	if err := sheet.SetZoomScale(0); err != nil {
		t.Fatalf("clear zoom scale: %v", err)
	}
	if got := sheet.ZoomScale(); got != 0 {
		t.Fatalf("cleared zoom scale = %d, want 0", got)
	}
	if err := sheet.SetZoomScale(9); err == nil {
		t.Fatal("expected zoom scale below 10 to fail")
	}
	if err := sheet.SetZoomScale(401); err == nil {
		t.Fatal("expected zoom scale above 400 to fail")
	}
}

func TestSheetGridLinesVisibilityMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data Sheet")
	if err != nil {
		t.Fatalf("CreateSheet Data Sheet: %v", err)
	}
	if _, ok := sheet.GridLinesVisible(); ok {
		t.Fatal("expected initial gridline visibility to be unset")
	}
	sheet.SetGridLinesVisible(false)
	visible, ok := sheet.GridLinesVisible()
	if !ok || visible {
		t.Fatalf("gridline visibility = (%v, %v), want (false, true)", visible, ok)
	}
	sheet.SetGridLinesVisible(true)
	visible, ok = sheet.GridLinesVisible()
	if !ok || !visible {
		t.Fatalf("gridline visibility = (%v, %v), want (true, true)", visible, ok)
	}
	sheet.ClearGridLinesVisible()
	if _, ok := sheet.GridLinesVisible(); ok {
		t.Fatal("expected cleared gridline visibility to be unset")
	}
}

func resolveWorkbook(t *testing.T, data []byte) *sheetresolve.Workbook {
	t.Helper()
	opcPkg, err := opc.OpenReaderAt(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	xlPkg, err := sheetparts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}
	wb, err := sheetresolve.ResolvePackage(xlPkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	return wb
}

func findCell(t *testing.T, sheet *sheetir.Sheet, row, col int) *sheetir.Cell {
	t.Helper()
	for _, r := range sheet.Rows {
		if r.Index != row {
			continue
		}
		for _, c := range r.Cells {
			if c.Col == col {
				return c
			}
		}
	}
	t.Fatalf("cell %d,%d not found", row, col)
	return nil
}
