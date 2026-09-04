package edit_test

import (
	"bytes"
	"fmt"
	"testing"

	"office/pkg/ooxml/opc"
	"office/pkg/sheet/edit"
	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

func TestWorkbookDefinedNamesRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	if _, err := wb.CreateSheet("Data"); err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	localIndex := 0
	if _, err := wb.SetDefinedName("Total", "Data!$B$2", nil); err != nil {
		t.Fatalf("SetDefinedName workbook: %v", err)
	}
	if _, err := wb.SetDefinedNameWithOptions("LocalName", "Data!$A$1", &localIndex, true, "local"); err != nil {
		t.Fatalf("SetDefinedName local: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	if len(roundTrip.DefinedNames) != 2 {
		t.Fatalf("defined names = %d, want 2", len(roundTrip.DefinedNames))
	}
	if roundTrip.DefinedNames[0].Name != "Total" || roundTrip.DefinedNames[0].Value != "Data!$B$2" {
		t.Errorf("workbook name = %#v", roundTrip.DefinedNames[0])
	}
	gotLocal := roundTrip.DefinedNames[1]
	if gotLocal.Name != "LocalName" || gotLocal.LocalSheetID == nil || *gotLocal.LocalSheetID != 0 || !gotLocal.Hidden {
		t.Errorf("local name = %#v", gotLocal)
	}
}

func TestSheetTablesAndAutoFilterRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	for col, value := range []string{"Name", "Qty"} {
		cell, err := sheet.CreateCell(1, col+1)
		if err != nil {
			t.Fatalf("CreateCell header: %v", err)
		}
		cell.SetString(value)
	}
	if _, err := sheet.AddTable("Sales", "A1:B10", []string{"Name", "Qty"}); err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	if err := sheet.SetTableStyle("Sales", "TableStyleLight9", false, true, true, false); err != nil {
		t.Fatalf("SetTableStyle: %v", err)
	}
	if err := sheet.SetTableTotalsRow("Sales", true); err != nil {
		t.Fatalf("SetTableTotalsRow: %v", err)
	}
	if err := sheet.SetTableColumnCalculatedFormula("Sales", "Qty", "SUM([Qty])"); err != nil {
		t.Fatalf("SetTableColumnCalculatedFormula: %v", err)
	}
	if err := sheet.SetTableColumnTotalsFormula("Sales", "Qty", "SUBTOTAL(109,[Qty])"); err != nil {
		t.Fatalf("SetTableColumnTotalsFormula: %v", err)
	}
	if err := sheet.SetTableColumnTotalsLabel("Sales", "Name", "Total"); err != nil {
		t.Fatalf("SetTableColumnTotalsLabel: %v", err)
	}
	sheet.SetAutoFilter("A1:B10")
	if err := sheet.SetAutoFilterValues(0, []string{"A", "B"}, true); err != nil {
		t.Fatalf("SetAutoFilterValues: %v", err)
	}
	if err := sheet.SetAutoFilterTop10(1, true, false, 5); err != nil {
		t.Fatalf("SetAutoFilterTop10: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	got := roundTrip.Sheets[0]
	if len(got.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(got.Tables))
	}
	table := got.Tables[0]
	if table.Name != "Sales" || table.Ref != "A1:B10" || table.TableStyle == nil || table.TableStyle.Name != "TableStyleLight9" {
		t.Errorf("table = %#v", table)
	}
	if !table.TotalsRowShown || table.TotalsRowCount != 1 {
		t.Errorf("table totals = shown %v count %d", table.TotalsRowShown, table.TotalsRowCount)
	}
	if table.Columns[0].TotalsRowLabel != "Total" {
		t.Errorf("totals label = %q", table.Columns[0].TotalsRowLabel)
	}
	if table.Columns[1].CalculatedColumnFormula != "SUM([Qty])" || table.Columns[1].TotalsRowFormula != "SUBTOTAL(109,[Qty])" {
		t.Errorf("table formulas = %#v", table.Columns[1])
	}
	if got.AutoFilterRef != "A1:B10" {
		t.Fatalf("auto-filter ref = %q, want A1:B10", got.AutoFilterRef)
	}
	if len(got.FilterColumns) != 2 {
		t.Fatalf("filter columns = %d, want 2", len(got.FilterColumns))
	}
	first := got.FilterColumns[0]
	if first.ColIndex != 0 || !first.ShowBlanks || len(first.Filters) != 2 || first.Filters[0] != "A" {
		t.Errorf("first filter = %#v", first)
	}
	second := got.FilterColumns[1]
	if second.Top10 == nil || !second.Top10.Top || second.Top10.Val != 5 {
		t.Errorf("second filter = %#v", second)
	}
}

func TestSheetTableMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	table, err := sheet.AddTable("", "A1:C5", []string{"A", "B", "C"})
	if err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	if table.Name != "Table1" {
		t.Fatalf("table name = %q, want Table1", table.Name)
	}
	if err := sheet.ResizeTable("Table1", "A1:C6"); err != nil {
		t.Fatalf("ResizeTable: %v", err)
	}
	if got, ok := sheet.Table("Table1"); !ok || got.Ref != "A1:C6" {
		t.Fatalf("resized table = %#v, ok %v", got, ok)
	}
	if !sheet.RemoveTable("Table1") {
		t.Fatalf("RemoveTable returned false")
	}
	if len(sheet.Tables()) != 0 {
		t.Fatalf("tables after remove = %d, want 0", len(sheet.Tables()))
	}
}

func TestSheetCommentsMutationAndRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Notes")
	if err != nil {
		t.Fatalf("CreateSheet Notes: %v", err)
	}
	if err := sheet.SetComment(" a1 ", "header"); err != nil {
		t.Fatalf("SetComment A1: %v", err)
	}
	if err := sheet.SetComment("B2", "second"); err != nil {
		t.Fatalf("SetComment B2: %v", err)
	}
	if got, ok := sheet.Comment("A1"); !ok || got != "header" {
		t.Fatalf("Comment A1 = %q, %v", got, ok)
	}
	comments := sheet.Comments()
	if len(comments) != 2 || comments["A1"] != "header" || comments["B2"] != "second" {
		t.Fatalf("Comments = %#v", comments)
	}
	comments["A1"] = "changed"
	if got, _ := sheet.Comment("A1"); got != "header" {
		t.Fatalf("stored comment mutated via copy: %q", got)
	}
	if err := sheet.SetComment("B2", "updated"); err != nil {
		t.Fatalf("SetComment B2 update: %v", err)
	}
	if got, ok := sheet.Comment("B2"); !ok || got != "updated" {
		t.Fatalf("Comment B2 = %q, %v", got, ok)
	}
	if !sheet.RemoveComment("B2") {
		t.Fatalf("RemoveComment B2 returned false")
	}
	if _, ok := sheet.Comment("B2"); ok {
		t.Fatalf("Comment B2 still present")
	}
	if err := sheet.SetComment("C3", "keep"); err != nil {
		t.Fatalf("SetComment C3: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	roundTrip := resolveWorkbook(t, buf.Bytes())
	got := roundTrip.Sheets[0]
	if len(got.Comments) != 2 || got.Comments["A1"] != "header" || got.Comments["C3"] != "keep" {
		t.Fatalf("round-trip comments = %#v", got.Comments)
	}
	loaded := loadPartsPackageFromBytes(t, buf.Bytes())
	if len(loaded.Worksheets) != 1 || len(loaded.Worksheets[0].LegacyDrawingVML) == 0 {
		t.Fatalf("expected worksheet legacyDrawing.vml for comments, got %+v", loaded.Worksheets)
	}
	if loaded.Worksheets[0].Sheet == nil || loaded.Worksheets[0].Sheet.LegacyDrawing == nil {
		t.Fatalf("expected worksheet legacyDrawing relationship for comments, got %+v", loaded.Worksheets[0].Sheet)
	}
	if loaded.Worksheets[0].LegacyNotes["A1"].Ref != "A1" || loaded.Worksheets[0].LegacyNotes["C3"].Ref != "C3" {
		t.Fatalf("legacy notes after comment save = %#v", loaded.Worksheets[0].LegacyNotes)
	}

	if removed := sheet.ClearComments(); removed != 2 {
		t.Fatalf("ClearComments removed %d, want 2", removed)
	}
	if got := sheet.Comments(); len(got) != 0 {
		t.Fatalf("Comments after clear = %#v", got)
	}
	buf.Reset()
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX after clear: %v", err)
	}
	cleared := loadPartsPackageFromBytes(t, buf.Bytes())
	if len(cleared.Worksheets) != 1 {
		t.Fatalf("worksheets after clear = %d, want 1", len(cleared.Worksheets))
	}
	if len(cleared.Worksheets[0].LegacyDrawingVML) != 0 {
		t.Fatalf("expected comment VML removed after clear, got %d bytes", len(cleared.Worksheets[0].LegacyDrawingVML))
	}
	if cleared.Worksheets[0].Sheet != nil && cleared.Worksheets[0].Sheet.LegacyDrawing != nil {
		t.Fatalf("expected legacyDrawing removed after clear, got %+v", cleared.Worksheets[0].Sheet.LegacyDrawing)
	}
}

func TestSheetFormControlAccessors(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	wb.IR().Sheets[0].FormControls = []*sheetir.FormControl{{
		Type:     "Checkbox",
		AltText:  "Opt In",
		Anchor:   "0, 0, 0, 0, 1, 0, 1, 0",
		Checked:  true,
		FmlaLink: "$B$1",
	}}

	controls := sheet.FormControls()
	if len(controls) != 1 || controls[0].AltText != "Opt In" || !controls[0].Checked {
		t.Fatalf("form controls = %#v", controls)
	}
	controls[0].AltText = "Mutated"
	got, ok := sheet.FormControl(" opt in ")
	if !ok {
		t.Fatal("FormControl lookup failed")
	}
	if got.AltText != "Opt In" || got.FmlaLink != "$B$1" {
		t.Fatalf("FormControl lookup = %#v", got)
	}
}

func TestSheetFormControlMutationAndRoundTrip(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	control, err := sheet.AddFormControl("Checkbox", "Opt In", "0, 0, 0, 0, 1, 0, 1, 0")
	if err != nil {
		t.Fatalf("AddFormControl: %v", err)
	}
	control.Checked = true
	control.FmlaLink = "$B$1"
	if err := sheet.SetFormControl(control); err != nil {
		t.Fatalf("SetFormControl checkbox: %v", err)
	}
	if _, err := sheet.AddFormControl("Spin", "Counter", "2, 0, 0, 0, 3, 0, 1, 0"); err != nil {
		t.Fatalf("AddFormControl spin: %v", err)
	}
	if err := sheet.SetFormControl(&sheetir.FormControl{
		Type:    "Spin",
		AltText: "Counter",
		Anchor:  "2, 0, 0, 0, 3, 0, 1, 0",
		Min:     1,
		Max:     9,
		Val:     3,
	}); err != nil {
		t.Fatalf("SetFormControl spin: %v", err)
	}
	if _, err := sheet.AddFormControl("Drop", "Status", "4, 0, 0, 0, 5, 0, 2, 0"); err != nil {
		t.Fatalf("AddFormControl drop: %v", err)
	}
	if err := sheet.SetFormControl(&sheetir.FormControl{
		Type:      "Drop",
		AltText:   "Status",
		Anchor:    "4, 0, 0, 0, 5, 0, 2, 0",
		FmlaLink:  "$C$1",
		FmlaRange: "$D$1:$D$3",
		Val:       2,
	}); err != nil {
		t.Fatalf("SetFormControl drop: %v", err)
	}
	if _, err := sheet.AddFormControl("List", "Teams", "6, 0, 0, 0, 7, 0, 3, 0"); err != nil {
		t.Fatalf("AddFormControl list: %v", err)
	}
	if err := sheet.SetFormControl(&sheetir.FormControl{
		Type:          "List",
		AltText:       "Teams",
		Anchor:        "6, 0, 0, 0, 7, 0, 3, 0",
		FmlaRange:     "$E$1:$E$4",
		SelectionType: "extended",
		Selected:      []int{2, 4},
		Val:           2,
	}); err != nil {
		t.Fatalf("SetFormControl list: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	roundTrip := resolveWorkbook(t, buf.Bytes())
	gotSheet := roundTrip.Sheets[0]
	if len(gotSheet.FormControls) != 4 {
		t.Fatalf("form controls = %d, want 4", len(gotSheet.FormControls))
	}
	if gotSheet.FormControls[0].AltText != "Opt In" || !gotSheet.FormControls[0].Checked || gotSheet.FormControls[0].FmlaLink != "$B$1" {
		t.Fatalf("checkbox round-trip = %#v", gotSheet.FormControls[0])
	}
	if gotSheet.FormControls[1].Type != "Spin" || gotSheet.FormControls[1].Min != 1 || gotSheet.FormControls[1].Max != 9 || gotSheet.FormControls[1].Val != 3 {
		t.Fatalf("spin round-trip = %#v", gotSheet.FormControls[1])
	}
	if gotSheet.FormControls[2].Type != "Drop" || gotSheet.FormControls[2].FmlaLink != "$C$1" || gotSheet.FormControls[2].FmlaRange != "$D$1:$D$3" || gotSheet.FormControls[2].Val != 2 {
		t.Fatalf("drop round-trip = %#v", gotSheet.FormControls[2])
	}
	if got := gotSheet.FormControls[3]; got.Type != "List" || got.FmlaRange != "$E$1:$E$4" || got.SelectionType != "extended" || got.Val != 2 {
		t.Fatalf("list round-trip = %#v", got)
	}
	if got := gotSheet.FormControls[3].Selected; len(got) != 2 || got[0] != 2 || got[1] != 4 {
		t.Fatalf("list selected round-trip = %v, want [2 4]", got)
	}
	loaded := loadPartsPackageFromBytes(t, buf.Bytes())
	if len(loaded.Worksheets) != 1 || len(loaded.Worksheets[0].LegacyDrawingVML) == 0 {
		t.Fatalf("expected worksheet legacyDrawing.vml write-back, got %+v", loaded.Worksheets)
	}
}

func TestSheetRadioGroupFormControlRoundTrip(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	linkedCell, err := sheet.CreateCell(1, 8)
	if err != nil {
		t.Fatalf("CreateCell H1: %v", err)
	}
	linkedCell.SetNumber(99)
	if _, err := sheet.AddFormControl("GBox", "Region", "0, 0, 4, 0, 3, 0, 8, 0"); err != nil {
		t.Fatalf("AddFormControl group box: %v", err)
	}
	if err := sheet.SetFormControl(&sheetir.FormControl{
		Type:      "GBox",
		AltText:   "Region",
		Anchor:    "0, 0, 4, 0, 3, 0, 8, 0",
		FmlaGroup: "$H$1",
	}); err != nil {
		t.Fatalf("SetFormControl group box: %v", err)
	}
	if _, err := sheet.AddFormControl("Radio", "North", "1, 0, 5, 0, 2, 0, 6, 0"); err != nil {
		t.Fatalf("AddFormControl first radio: %v", err)
	}
	if err := sheet.SetFormControl(&sheetir.FormControl{
		Type:        "Radio",
		AltText:     "North",
		Anchor:      "1, 0, 5, 0, 2, 0, 6, 0",
		Checked:     true,
		FirstButton: true,
		FmlaLink:    "$H$1",
	}); err != nil {
		t.Fatalf("SetFormControl first radio: %v", err)
	}
	if _, err := sheet.AddFormControl("Radio", "South", "1, 0, 6, 0, 2, 0, 7, 0"); err != nil {
		t.Fatalf("AddFormControl second radio: %v", err)
	}
	if err := sheet.SetFormControl(&sheetir.FormControl{
		Type:     "Radio",
		AltText:  "South",
		Anchor:   "1, 0, 6, 0, 2, 0, 7, 0",
		FmlaLink: "$H$1",
	}); err != nil {
		t.Fatalf("SetFormControl second radio: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	roundTrip := resolveWorkbook(t, buf.Bytes())
	gotSheet := roundTrip.Sheets[0]
	if len(gotSheet.FormControls) != 3 {
		t.Fatalf("form controls = %d, want 3", len(gotSheet.FormControls))
	}
	if got := gotSheet.FormControls[0]; got.Type != "GBox" || got.FmlaGroup != "$H$1" {
		t.Fatalf("group box round-trip = %#v", got)
	}
	if got := gotSheet.FormControls[1]; got.Type != "Radio" || !got.Checked || !got.FirstButton || got.FmlaLink != "$H$1" {
		t.Fatalf("first radio round-trip = %#v", got)
	}
	if got := gotSheet.FormControls[2]; got.Type != "Radio" || got.Checked || got.FirstButton || got.FmlaLink != "$H$1" {
		t.Fatalf("second radio round-trip = %#v", got)
	}
	if got := findCell(t, gotSheet, 1, 8); got.Kind != sheetir.CellNumber || got.NumValue != 1 {
		t.Fatalf("linked cell round-trip = %#v, want numeric 1", got)
	}
}

func TestSheetScalarFormControlLinkedCellsRoundTrip(t *testing.T) {
	wb := newWorkbookWithSheet(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	for _, col := range []int{3, 6, 9} {
		cell, err := sheet.CreateCell(1, col)
		if err != nil {
			t.Fatalf("CreateCell row 1 col %d: %v", col, err)
		}
		cell.SetNumber(99)
	}
	if _, err := sheet.AddFormControl("Drop", "Status", "0, 0, 0, 0, 1, 0, 1, 0"); err != nil {
		t.Fatalf("AddFormControl drop: %v", err)
	}
	if err := sheet.SetFormControl(&sheetir.FormControl{
		Type:      "Drop",
		AltText:   "Status",
		Anchor:    "0, 0, 0, 0, 1, 0, 1, 0",
		FmlaLink:  "$C$1",
		FmlaRange: "$D$1:$D$3",
		Val:       2,
	}); err != nil {
		t.Fatalf("SetFormControl drop: %v", err)
	}
	if _, err := sheet.AddFormControl("List", "Priority", "2, 0, 0, 0, 3, 0, 2, 0"); err != nil {
		t.Fatalf("AddFormControl list: %v", err)
	}
	if err := sheet.SetFormControl(&sheetir.FormControl{
		Type:          "List",
		AltText:       "Priority",
		Anchor:        "2, 0, 0, 0, 3, 0, 2, 0",
		FmlaLink:      "$F$1",
		FmlaRange:     "$G$1:$G$3",
		SelectionType: "single",
		Val:           3,
	}); err != nil {
		t.Fatalf("SetFormControl list: %v", err)
	}
	if _, err := sheet.AddFormControl("Spin", "Counter", "4, 0, 0, 0, 5, 0, 1, 0"); err != nil {
		t.Fatalf("AddFormControl spin: %v", err)
	}
	if err := sheet.SetFormControl(&sheetir.FormControl{
		Type:     "Spin",
		AltText:  "Counter",
		Anchor:   "4, 0, 0, 0, 5, 0, 1, 0",
		FmlaLink: "$I$1",
		Min:      0,
		Max:      9,
		Val:      4,
	}); err != nil {
		t.Fatalf("SetFormControl spin: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	gotSheet := resolveWorkbook(t, buf.Bytes()).Sheets[0]
	if got := findCell(t, gotSheet, 1, 3); got.Kind != sheetir.CellNumber || got.NumValue != 2 {
		t.Fatalf("drop linked cell round-trip = %#v, want numeric 2", got)
	}
	if got := findCell(t, gotSheet, 1, 6); got.Kind != sheetir.CellNumber || got.NumValue != 3 {
		t.Fatalf("list linked cell round-trip = %#v, want numeric 3", got)
	}
	if got := findCell(t, gotSheet, 1, 9); got.Kind != sheetir.CellNumber || got.NumValue != 4 {
		t.Fatalf("spin linked cell round-trip = %#v, want numeric 4", got)
	}
}

func TestOpenXLSXFormControlEditPreservesLegacyNotes(t *testing.T) {
	vml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<xml xmlns:v="urn:schemas-microsoft-com:vml" xmlns:x="urn:schemas-microsoft-com:office:excel">
  <v:shape id="_x0000_s1025" type="#_x0000_t202">
    <x:ClientData ObjectType="Note">
      <x:Anchor>3, 15, 25, 10, 7, 15, 27, 4</x:Anchor>
      <x:Row>24</x:Row>
      <x:Column>2</x:Column>
    </x:ClientData>
  </v:shape>
  <v:shape id="_x0000_s2049" type="#_x0000_t201" alt="Opt In">
    <x:ClientData ObjectType="Checkbox">
      <x:Anchor>0, 0, 0, 0, 1, 0, 1, 0</x:Anchor>
      <x:FmlaLink>$B$1</x:FmlaLink>
    </x:ClientData>
  </v:shape>
</xml>`)
	source := &sheetparts.Package{
		Workbook: &sheetparts.Workbook{
			Sheets: []sheetparts.SheetRef{{SheetID: 7, Name: "Sheet1", RelID: "rId1"}},
		},
		Worksheets: []*sheetparts.WorksheetEntry{{
			SheetID:  7,
			RelID:    "rId1",
			Name:     "Sheet1",
			PartName: "xl/worksheets/sheet1.xml",
			Sheet: &sheetparts.Worksheet{
				LegacyDrawing: &sheetparts.DrawingRef{RelID: "rIdVML1"},
				Rows: []sheetparts.Row{{
					Index: 1,
					Cells: []sheetparts.Cell{{Ref: "A1", Type: "str", Value: "ok"}},
				}},
			},
			Comments: &sheetparts.Comments{
				Authors: []string{""},
				CommentList: []sheetparts.CommentEntry{{
					Ref:      "C25",
					AuthorID: 0,
					Text:     sheetparts.RichText{PlainText: "Legacy note"},
				}},
			},
			LegacyDrawingVML: vml,
		}},
		Styles: &sheetparts.StyleSheet{
			Fonts:   []sheetparts.Font{{}},
			Fills:   []sheetparts.Fill{{PatternFill: &sheetparts.PatternFill{PatternType: "none"}}},
			Borders: []sheetparts.Border{{}},
			CellXfs: []sheetparts.Xf{{}},
		},
	}
	var sourceBuf bytes.Buffer
	if err := sheetparts.WritePackage(source, &sourceBuf); err != nil {
		t.Fatalf("WritePackage source: %v", err)
	}
	wb, err := edit.OpenXLSX(bytes.NewReader(sourceBuf.Bytes()), int64(sourceBuf.Len()))
	if err != nil {
		t.Fatalf("OpenXLSX: %v", err)
	}
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if err := sheet.SetFormControl(&sheetir.FormControl{
		Type:     "Checkbox",
		AltText:  "Opt In",
		Anchor:   "0, 0, 0, 0, 1, 0, 1, 0",
		Checked:  true,
		FmlaLink: "$B$1",
	}); err != nil {
		t.Fatalf("SetFormControl: %v", err)
	}

	var out bytes.Buffer
	if err := wb.SaveAsXLSX(&out); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	loaded := loadPartsPackageFromBytes(t, out.Bytes())
	if len(loaded.Worksheets) != 1 {
		t.Fatalf("worksheets = %d, want 1", len(loaded.Worksheets))
	}
	entry := loaded.Worksheets[0]
	if len(entry.FormControls) != 1 || !entry.FormControls[0].Checked || entry.FormControls[0].AltText != "Opt In" {
		t.Fatalf("form controls after save = %#v", entry.FormControls)
	}
	if entry.LegacyNotes["C25"].Ref != "C25" {
		t.Fatalf("legacy notes after save = %#v", entry.LegacyNotes)
	}
}

func loadPartsPackageFromBytes(t *testing.T, data []byte) *sheetparts.Package {
	t.Helper()
	opcPkg, err := opc.OpenReaderAt(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReaderAt: %v", err)
	}
	xlPkg, err := sheetparts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("LoadPackage: %v", err)
	}
	return xlPkg
}

func buildExistingPivotWorkbookBytes(t *testing.T, useNamedSource bool) []byte {
	t.Helper()
	tableRaw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<pivotTableDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" name="SalesPivot" cacheId="9" dataCaption="Values">
  <location ref="A3:C8" firstHeaderRow="1" firstDataRow="2" firstDataCol="1"/>
  <pivotFields count="2">
    <pivotField name="Name" axis="axisRow" showAll="1"/>
    <pivotField name="Value" dataField="1" showAll="1"/>
  </pivotFields>
  <rowFields count="1">
    <field x="0"/>
  </rowFields>
  <dataFields count="1">
    <dataField name="Sum of Value" fld="1" subtotal="sum"/>
  </dataFields>
</pivotTableDefinition>`)
	cacheRaw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<pivotCacheDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" refreshedBy="Excel" refreshedDate="2026-01-01T00:00:00Z" createdVersion="3" refreshedVersion="3" minRefreshableVersion="3" recordCount="2">
  <cacheSource type="worksheet">
    <worksheetSource sheet="Data" ref="A1:B3"/>
  </cacheSource>
  <cacheFields count="2">
    <cacheField name="Name">
      <sharedItems count="2">
        <s v="A"/>
        <s v="B"/>
      </sharedItems>
    </cacheField>
    <cacheField name="Value"/>
  </cacheFields>
</pivotCacheDefinition>`)
	worksheetSource := &sheetparts.WorksheetSource{Sheet: "Data", Ref: "A1:B3"}
	var definedNames []sheetparts.DefinedName
	if useNamedSource {
		cacheRaw = []byte(`<?xml version="1.0" encoding="UTF-8"?>
<pivotCacheDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" refreshedBy="Excel" refreshedDate="2026-01-01T00:00:00Z" createdVersion="3" refreshedVersion="3" minRefreshableVersion="3" recordCount="2">
  <cacheSource type="worksheet">
    <worksheetSource name="PivotData"/>
  </cacheSource>
  <cacheFields count="2">
    <cacheField name="Name">
      <sharedItems count="2">
        <s v="A"/>
        <s v="B"/>
      </sharedItems>
    </cacheField>
    <cacheField name="Value"/>
  </cacheFields>
</pivotCacheDefinition>`)
		worksheetSource = &sheetparts.WorksheetSource{Name: "PivotData"}
		definedNames = []sheetparts.DefinedName{{
			Name:  "PivotData",
			Value: "Data!$A$1:$B$3",
		}}
	}
	recordsRaw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<pivotCacheRecords xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="2">
  <r><c t="s" v="0"/><c t="n" v="10"/></r>
  <r><c t="s" v="1"/><c t="n" v="20"/></r>
</pivotCacheRecords>`)
	source := &sheetparts.Package{
		Workbook: &sheetparts.Workbook{
			Sheets: []sheetparts.SheetRef{
				{SheetID: 1, Name: "Data", RelID: "rId1"},
				{SheetID: 2, Name: "Pivot", RelID: "rId2"},
			},
			DefinedNames: definedNames,
		},
		Worksheets: []*sheetparts.WorksheetEntry{
			{
				SheetID:  1,
				RelID:    "rId1",
				Name:     "Data",
				PartName: "xl/worksheets/sheet1.xml",
				Sheet: &sheetparts.Worksheet{
					Rows: []sheetparts.Row{
						{Index: 1, Cells: []sheetparts.Cell{{Ref: "A1", Type: "str", Value: "Name"}, {Ref: "B1", Type: "str", Value: "Value"}}},
						{Index: 2, Cells: []sheetparts.Cell{{Ref: "A2", Type: "str", Value: "A"}, {Ref: "B2", Value: "10"}}},
						{Index: 3, Cells: []sheetparts.Cell{{Ref: "A3", Type: "str", Value: "B"}, {Ref: "B3", Value: "20"}}},
					},
				},
			},
			{
				SheetID:  2,
				RelID:    "rId2",
				Name:     "Pivot",
				PartName: "xl/worksheets/sheet2.xml",
				Sheet: &sheetparts.Worksheet{
					Rows: []sheetparts.Row{{
						Index: 1,
						Cells: []sheetparts.Cell{{Ref: "A1", Type: "str", Value: "Pivot"}},
					}},
				},
				PivotTables: []*sheetparts.PivotTableWithCache{{
					Table: &sheetparts.PivotTableDefinition{
						Name:        "SalesPivot",
						CacheID:     9,
						DataCaption: "Values",
						Location: sheetparts.PivotLocation{
							Ref:            "A3:C8",
							FirstHeaderRow: 1,
							FirstDataRow:   2,
							FirstDataCol:   1,
						},
						PivotFields: []sheetparts.PivotField{
							{Name: "Name", Axis: "axisRow", ShowAll: true},
							{Name: "Value", DataField: true, ShowAll: true},
						},
						RowFields: []sheetparts.PivotFieldIndex{{Index: 0}},
						DataFields: []sheetparts.DataField{{
							Name:     "Sum of Value",
							Field:    1,
							Subtotal: "sum",
						}},
						Raw: tableRaw,
					},
					Cache: &sheetparts.PivotCacheDefinition{
						RefreshedBy:           "Excel",
						RefreshedDate:         "2026-01-01T00:00:00Z",
						CreatedVersion:        3,
						RefreshedVersion:      3,
						MinRefreshableVersion: 3,
						RecordCount:           2,
						WorksheetSource:       worksheetSource,
						Raw:                   cacheRaw,
					},
					Records: &sheetparts.PivotCacheRecords{
						Count: 2,
						Raw:   recordsRaw,
					},
				}},
			},
		},
		Styles: &sheetparts.StyleSheet{
			Fonts:   []sheetparts.Font{{}},
			Fills:   []sheetparts.Fill{{PatternFill: &sheetparts.PatternFill{PatternType: "none"}}},
			Borders: []sheetparts.Border{{}},
			CellXfs: []sheetparts.Xf{{}},
		},
	}
	var buf bytes.Buffer
	if err := sheetparts.WritePackage(source, &buf); err != nil {
		t.Fatalf("WritePackage source: %v", err)
	}
	return buf.Bytes()
}

func TestSheetAutoFilterCustomCriteria(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	sheet.SetAutoFilter("A1:A10")
	err = sheet.SetAutoFilterCustom(0, []*sheetir.CustomFilter{
		{Operator: "greaterThan", Val: "10"},
		{Operator: "lessThan", Val: "20"},
	}, true)
	if err != nil {
		t.Fatalf("SetAutoFilterCustom: %v", err)
	}
	if !sheet.ClearAutoFilterColumn(0) {
		t.Fatalf("ClearAutoFilterColumn returned false")
	}
	if !sheet.ClearAutoFilterColumn(0) {
		return
	}
	t.Fatalf("ClearAutoFilterColumn returned true for absent column")
}

func TestSheetAutoFilterDynamicAndSortStateRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	sheet.SetAutoFilter("A1:B10")
	if err := sheet.SetAutoFilterDynamic(0, sheetir.DynamicFilter{Type: "lastMonth"}); err != nil {
		t.Fatalf("SetAutoFilterDynamic: %v", err)
	}
	updatedSort, err := sheet.SetSortState(sheetir.SortState{
		Ref:           "A1:B10",
		CaseSensitive: true,
		Conditions: []*sheetir.SortCondition{
			{Ref: "B2:B10", Descending: true},
			{Ref: "A2:A10"},
		},
	})
	if err != nil {
		t.Fatalf("SetSortState: %v", err)
	}
	if updatedSort == nil || len(updatedSort.Conditions) != 2 {
		t.Fatalf("updated sort state = %#v", updatedSort)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	got := roundTrip.Sheets[0]
	if len(got.FilterColumns) != 1 {
		t.Fatalf("filter columns = %d, want 1", len(got.FilterColumns))
	}
	if got.FilterColumns[0].Dynamic == nil || got.FilterColumns[0].Dynamic.Type != "lastMonth" {
		t.Fatalf("dynamic filter = %#v", got.FilterColumns[0].Dynamic)
	}
	if got.SortState == nil {
		t.Fatal("sort state is nil")
	}
	if got.SortState.Ref != "A1:B10" || !got.SortState.CaseSensitive {
		t.Fatalf("sort state = %#v", got.SortState)
	}
	if len(got.SortState.Conditions) != 2 {
		t.Fatalf("sort conditions = %d, want 2", len(got.SortState.Conditions))
	}
	if got.SortState.Conditions[0].Ref != "B2:B10" || !got.SortState.Conditions[0].Descending {
		t.Fatalf("sort condition 0 = %#v", got.SortState.Conditions[0])
	}
	if got.SortState.Conditions[1].Ref != "A2:A10" {
		t.Fatalf("sort condition 1 = %#v", got.SortState.Conditions[1])
	}
}

func TestSheetDataValidationRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	list, err := sheet.AddListValidation("A2:A5", []string{"Open", "Closed"}, true)
	if err != nil {
		t.Fatalf("AddListValidation: %v", err)
	}
	list.ShowInputMessage = true
	list.PromptTitle = "Status"
	list.Prompt = "Choose a status"
	list.ShowErrorMessage = true
	list.ErrorTitle = "Invalid status"
	list.Error = "Use the list"
	if _, err := sheet.AddComparisonValidation("B2:B5", "whole", "between", "1", "10", false); err != nil {
		t.Fatalf("AddComparisonValidation: %v", err)
	}
	if _, err := sheet.SetDataValidation(sheetir.DataValidation{
		SqRef:            "C2:C5",
		Type:             "custom",
		ShowErrorMessage: true,
		Error:            "At least four characters",
		Formula1:         "LEN(C2)>3",
	}); err != nil {
		t.Fatalf("SetDataValidation: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	got := roundTrip.Sheets[0]
	if len(got.DataValidations) != 3 {
		t.Fatalf("data validations = %d, want 3", len(got.DataValidations))
	}
	gotList := got.DataValidations[0]
	if gotList.SqRef != "A2:A5" || gotList.Type != "list" || !gotList.AllowBlank {
		t.Errorf("list validation = %#v", gotList)
	}
	if gotList.Formula1 != `"Open,Closed"` {
		t.Errorf("list formula = %q, want quoted inline list", gotList.Formula1)
	}
	if len(gotList.ListValues) != 2 || gotList.ListValues[0] != "Open" || gotList.ListValues[1] != "Closed" {
		t.Errorf("list values = %#v", gotList.ListValues)
	}
	if !gotList.ShowInputMessage || gotList.PromptTitle != "Status" || gotList.ErrorTitle != "Invalid status" {
		t.Errorf("list prompt/error = %#v", gotList)
	}
	gotWhole := got.DataValidations[1]
	if gotWhole.Type != "whole" || gotWhole.Operator != "between" || gotWhole.Formula1 != "1" || gotWhole.Formula2 != "10" {
		t.Errorf("whole validation = %#v", gotWhole)
	}
	gotCustom := got.DataValidations[2]
	if gotCustom.Type != "custom" || gotCustom.Formula1 != "LEN(C2)>3" || gotCustom.Error != "At least four characters" {
		t.Errorf("custom validation = %#v", gotCustom)
	}
}

func TestSheetDataValidationMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	if _, err := sheet.AddFormulaListValidation("A1:A3", "Lists!$A$1:$A$3", false); err != nil {
		t.Fatalf("AddFormulaListValidation: %v", err)
	}
	replaced, err := sheet.SetDataValidation(sheetir.DataValidation{
		SqRef:    "A1:A3",
		Type:     "list",
		Formula1: "Lists!$B$1:$B$3",
	})
	if err != nil {
		t.Fatalf("SetDataValidation: %v", err)
	}
	if replaced.Formula1 != "Lists!$B$1:$B$3" {
		t.Fatalf("replaced formula = %q", replaced.Formula1)
	}
	if len(sheet.DataValidations()) != 1 {
		t.Fatalf("data validations = %d, want 1", len(sheet.DataValidations()))
	}
	if got, ok := sheet.DataValidation("A1:A3", "list"); !ok || got.Formula1 != "Lists!$B$1:$B$3" {
		t.Fatalf("lookup = %#v, ok %v", got, ok)
	}
	if removed := sheet.RemoveDataValidations("A1:A3"); removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if len(sheet.DataValidations()) != 0 {
		t.Fatalf("data validations after remove = %d, want 0", len(sheet.DataValidations()))
	}
}

func TestSheetConditionalFormattingRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	bold := true
	style := sheetir.DxfStyle{Bold: &bold, BgColor: "FFF2CC"}
	if _, err := sheet.AddCellIsConditionalFormat("A1:A5", "greaterThan", "10", "", style); err != nil {
		t.Fatalf("AddCellIsConditionalFormat: %v", err)
	}
	if _, err := sheet.AddColorScaleConditionalFormat("B1:B5", nil, []string{"F8696B", "63BE7B"}); err != nil {
		t.Fatalf("AddColorScaleConditionalFormat: %v", err)
	}
	if _, err := sheet.AddDataBarConditionalFormat("C1:C5", "638EC6", nil, true); err != nil {
		t.Fatalf("AddDataBarConditionalFormat: %v", err)
	}
	if _, err := sheet.AddIconSetConditionalFormat("D1:D5", "3TrafficLights1", nil, true, false); err != nil {
		t.Fatalf("AddIconSetConditionalFormat: %v", err)
	}
	if _, err := sheet.AddFormulaConditionalFormat("E1:E5", "MOD(ROW(),2)=0", style); err != nil {
		t.Fatalf("AddFormulaConditionalFormat: %v", err)
	}
	if _, err := sheet.AddTextConditionalFormat("F1:F5", "containsText", "late", style); err != nil {
		t.Fatalf("AddTextConditionalFormat: %v", err)
	}
	if _, err := sheet.AddTimePeriodConditionalFormat("G1:G5", "last7Days", style); err != nil {
		t.Fatalf("AddTimePeriodConditionalFormat: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	got := roundTrip.Sheets[0]
	if len(got.CFRules) != 7 {
		t.Fatalf("conditional formatting rules = %d, want 7", len(got.CFRules))
	}
	cellIs := got.CFRules[0]
	if cellIs.Type != "cellIs" || cellIs.Operator != "greaterThan" || cellIs.Priority != 1 || len(cellIs.Formula) != 1 || cellIs.Formula[0] != "10" {
		t.Errorf("cellIs rule = %#v", cellIs)
	}
	if cellIs.DxfStyle.Bold == nil || !*cellIs.DxfStyle.Bold || cellIs.DxfStyle.BgColor != "FFF2CC" {
		t.Errorf("cellIs style = %#v", cellIs.DxfStyle)
	}
	colorScale := got.CFRules[1]
	if colorScale.Type != "colorScale" || colorScale.ColorScale == nil || len(colorScale.ColorScale.Colors) != 2 || colorScale.ColorScale.Colors[1] != "63BE7B" {
		t.Errorf("color scale rule = %#v", colorScale)
	}
	dataBar := got.CFRules[2]
	if dataBar.Type != "dataBar" || dataBar.DataBar == nil || dataBar.DataBar.Color != "638EC6" || !dataBar.DataBar.ShowValue {
		t.Errorf("data bar rule = %#v", dataBar)
	}
	iconSet := got.CFRules[3]
	if iconSet.Type != "iconSet" || iconSet.IconSet == nil || iconSet.IconSet.Set != "3TrafficLights1" || !iconSet.IconSet.ShowValue {
		t.Errorf("icon set rule = %#v", iconSet)
	}
	expression := got.CFRules[4]
	if expression.Type != "expression" || len(expression.Formula) != 1 || expression.Formula[0] != "MOD(ROW(),2)=0" {
		t.Errorf("expression rule = %#v", expression)
	}
	text := got.CFRules[5]
	if text.Type != "containsText" || text.Text != "late" || len(text.Formula) != 1 {
		t.Errorf("text rule = %#v", text)
	}
	timePeriod := got.CFRules[6]
	if timePeriod.Type != "timePeriod" || timePeriod.TimePeriod != "last7Days" {
		t.Errorf("time-period rule = %#v", timePeriod)
	}
}

func TestSheetConditionalFormattingMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	if _, err := sheet.AddDuplicateValuesConditionalFormat("A1:A5", sheetir.DxfStyle{BgColor: "FFC7CE"}); err != nil {
		t.Fatalf("AddDuplicateValuesConditionalFormat: %v", err)
	}
	replaced, err := sheet.SetConditionalFormattingRule(sheetir.CFRule{
		SqRef:    "A1:A5",
		Type:     "duplicateValues",
		Priority: 1,
		DxfStyle: sheetir.DxfStyle{BgColor: "C6EFCE"},
	})
	if err != nil {
		t.Fatalf("SetConditionalFormattingRule: %v", err)
	}
	if replaced.DxfStyle.BgColor != "C6EFCE" {
		t.Fatalf("replaced style = %#v", replaced.DxfStyle)
	}
	if len(sheet.ConditionalFormattingRules()) != 1 {
		t.Fatalf("conditional formatting rules = %d, want 1", len(sheet.ConditionalFormattingRules()))
	}
	if got, ok := sheet.ConditionalFormattingRule("A1:A5", "duplicateValues", 1); !ok || got.DxfStyle.BgColor != "C6EFCE" {
		t.Fatalf("lookup = %#v, ok %v", got, ok)
	}
	if removed := sheet.RemoveConditionalFormattingRules("A1:A5"); removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if len(sheet.ConditionalFormattingRules()) != 0 {
		t.Fatalf("conditional formatting rules after remove = %d, want 0", len(sheet.ConditionalFormattingRules()))
	}
}

func TestSheetPivotTableRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	data, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	for col, value := range []string{"Region", "Product", "Channel", "Sales"} {
		cell, err := data.CreateCell(1, col+1)
		if err != nil {
			t.Fatalf("CreateCell header: %v", err)
		}
		cell.SetString(value)
	}
	rows := []struct {
		region  string
		product string
		channel string
		sales   float64
	}{
		{"East", "A", "Online", 10},
		{"East", "B", "Retail", 15},
		{"West", "A", "Online", 20},
	}
	for i, row := range rows {
		r := i + 2
		region, err := data.CreateCell(r, 1)
		if err != nil {
			t.Fatalf("CreateCell region: %v", err)
		}
		region.SetString(row.region)
		product, err := data.CreateCell(r, 2)
		if err != nil {
			t.Fatalf("CreateCell product: %v", err)
		}
		product.SetString(row.product)
		channel, err := data.CreateCell(r, 3)
		if err != nil {
			t.Fatalf("CreateCell channel: %v", err)
		}
		channel.SetString(row.channel)
		sales, err := data.CreateCell(r, 4)
		if err != nil {
			t.Fatalf("CreateCell sales: %v", err)
		}
		sales.SetNumber(row.sales)
	}
	pivotSheet, err := wb.CreateSheet("Pivot")
	if err != nil {
		t.Fatalf("CreateSheet Pivot: %v", err)
	}
	if _, err := pivotSheet.AddPivotTable("SalesPivot", "Data", "A1:D4", "A3:D8"); err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}
	if err := pivotSheet.AddPivotRowField("SalesPivot", "Region"); err != nil {
		t.Fatalf("AddPivotRowField: %v", err)
	}
	if err := pivotSheet.AddPivotColumnField("SalesPivot", "Product"); err != nil {
		t.Fatalf("AddPivotColumnField: %v", err)
	}
	if err := pivotSheet.AddPivotPageField("SalesPivot", "Channel"); err != nil {
		t.Fatalf("AddPivotPageField: %v", err)
	}
	if err := pivotSheet.SetPivotDataCaption("SalesPivot", "Measures"); err != nil {
		t.Fatalf("SetPivotDataCaption: %v", err)
	}
	if err := pivotSheet.SetPivotLayout("SalesPivot", false, true, false); err != nil {
		t.Fatalf("SetPivotLayout: %v", err)
	}
	if _, err := pivotSheet.SetPivotDataField("SalesPivot", sheetir.PivotDataField{
		Name:            "Avg of Sales",
		SourceFieldName: "Sales",
		Subtotal:        "average",
		NumFmtID:        4,
	}); err != nil {
		t.Fatalf("SetPivotDataField: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	got := roundTrip.Sheets[1]
	if len(got.PivotTables) != 1 {
		t.Fatalf("pivot tables = %d, want 1", len(got.PivotTables))
	}
	pivot := got.PivotTables[0]
	if pivot.Name != "SalesPivot" || pivot.SourceSheet != "Data" || pivot.SourceRef != "A1:D4" || pivot.LocationRef != "A3:D8" {
		t.Errorf("pivot = %#v", pivot)
	}
	if pivot.DataCaption != "Measures" || pivot.ShowHeaders || !pivot.RowGrandTotals || pivot.ColGrandTotals {
		t.Errorf("pivot display metadata = %#v", pivot)
	}
	if len(pivot.RowFields) != 1 || pivot.RowFields[0] != "Region" {
		t.Errorf("row fields = %#v", pivot.RowFields)
	}
	if len(pivot.ColFields) != 1 || pivot.ColFields[0] != "Product" {
		t.Errorf("col fields = %#v", pivot.ColFields)
	}
	if len(pivot.PageFields) != 1 || pivot.PageFields[0] != "Channel" {
		t.Errorf("page fields = %#v", pivot.PageFields)
	}
	if len(pivot.DataFields) != 1 || pivot.DataFields[0].SourceFieldName != "Sales" || pivot.DataFields[0].Subtotal != "average" || pivot.DataFields[0].NumFmtID != 4 {
		t.Errorf("data fields = %#v", pivot.DataFields)
	}
}

func TestPivotItemFilterAndPageSelectionRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	data, _ := wb.CreateSheet("Data")
	for col, text := range []string{"Region", "Sales"} {
		cell, _ := data.CreateCell(1, col+1)
		cell.SetString(text)
	}
	for i, region := range []string{"East", "West", "East"} {
		cell, _ := data.CreateCell(i+2, 1)
		cell.SetString(region)
		value, _ := data.CreateCell(i+2, 2)
		value.SetNumber(float64((i + 1) * 10))
	}
	pivotSheet, _ := wb.CreateSheet("Pivot")
	pivot, err := pivotSheet.AddPivotTable("SalesPivot", "Data", "A1:B4", "D3:G10")
	if err != nil {
		t.Fatal(err)
	}
	pivot.PageFields = []string{"Region"}
	pivot.Fields = []*sheetir.PivotField{{Name: "Region", Axis: "axisPage", ShowAll: true, Items: []*sheetir.PivotItem{{CacheIndex: 0, Type: "data", Name: "East"}, {CacheIndex: 1, Type: "data", Name: "West"}}}}
	if err := pivotSheet.SetPivotItemHidden("SalesPivot", "Region", "West", true); err != nil {
		t.Fatal(err)
	}
	if err := pivotSheet.SetPivotPageSelection("SalesPivot", "Region", 0); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatal(err)
	}
	roundTrip := resolveWorkbook(t, buf.Bytes()).Sheets[1].PivotTables[0]
	if len(roundTrip.Fields) == 0 || len(roundTrip.Fields[0].Items) != 2 || !roundTrip.Fields[0].Items[1].Hidden {
		t.Fatalf("round-trip items = %#v", roundTrip.Fields)
	}
	if len(roundTrip.PageFieldSelections) != 1 || roundTrip.PageFieldSelections[0].ItemIndex != 0 {
		t.Fatalf("page selections = %#v", roundTrip.PageFieldSelections)
	}
}

func TestSheetPivotTableMutation(t *testing.T) {
	wb := edit.NewWorkbook()
	if _, err := wb.CreateSheet("Data"); err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	sheet, err := wb.CreateSheet("Pivot")
	if err != nil {
		t.Fatalf("CreateSheet Pivot: %v", err)
	}
	pivot, err := sheet.AddPivotTable("", "Data", "A1:B10", "D3:F8")
	if err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}
	if pivot.Name != "PivotTable1" {
		t.Fatalf("pivot name = %q, want PivotTable1", pivot.Name)
	}
	if _, err := wb.SetDefinedName("PivotData", "Data!$A$1:$B$10", nil); err != nil {
		t.Fatalf("SetDefinedName: %v", err)
	}
	if err := sheet.SetPivotSourceName("PivotTable1", "PivotData"); err != nil {
		t.Fatalf("SetPivotSourceName: %v", err)
	}
	if err := sheet.SetPivotSource("PivotTable1", "Data", "A1:C20"); err != nil {
		t.Fatalf("SetPivotSource: %v", err)
	}
	if err := sheet.SetPivotLocation("PivotTable1", "E4:G9"); err != nil {
		t.Fatalf("SetPivotLocation: %v", err)
	}
	if err := sheet.SetPivotDataCaption("PivotTable1", "Metrics"); err != nil {
		t.Fatalf("SetPivotDataCaption: %v", err)
	}
	if err := sheet.SetPivotLayout("PivotTable1", false, false, true); err != nil {
		t.Fatalf("SetPivotLayout: %v", err)
	}
	if err := sheet.AddPivotRowField("PivotTable1", "Region"); err != nil {
		t.Fatalf("AddPivotRowField: %v", err)
	}
	if err := sheet.AddPivotColumnField("PivotTable1", "Product"); err != nil {
		t.Fatalf("AddPivotColumnField: %v", err)
	}
	if err := sheet.AddPivotPageField("PivotTable1", "Channel"); err != nil {
		t.Fatalf("AddPivotPageField: %v", err)
	}
	dataField, err := sheet.SetPivotDataField("PivotTable1", sheetir.PivotDataField{
		Name:            "Revenue",
		SourceFieldName: "Sales",
		Subtotal:        "sum",
		NumFmtID:        7,
	})
	if err != nil {
		t.Fatalf("SetPivotDataField: %v", err)
	}
	if dataField.Name != "Revenue" || dataField.NumFmtID != 7 {
		t.Fatalf("pivot data field = %#v", dataField)
	}
	if got, ok := sheet.PivotTable("PivotTable1"); !ok || got.SourceRef != "A1:C20" || got.SourceName != "" || got.LocationRef != "E4:G9" {
		t.Fatalf("lookup = %#v, ok %v", got, ok)
	}
	if got, ok := sheet.PivotTable("PivotTable1"); !ok || got.DataCaption != "Metrics" || got.ShowHeaders || got.RowGrandTotals || !got.ColGrandTotals || len(got.DataFields) != 1 {
		t.Fatalf("layout/data fields = %#v, ok %v", got, ok)
	}
	if !sheet.RemovePivotRowField("PivotTable1", "Region") {
		t.Fatalf("RemovePivotRowField returned false")
	}
	if !sheet.RemovePivotColumnField("PivotTable1", "Product") {
		t.Fatalf("RemovePivotColumnField returned false")
	}
	if !sheet.RemovePivotPageField("PivotTable1", "Channel") {
		t.Fatalf("RemovePivotPageField returned false")
	}
	if removed := sheet.RemovePivotDataFields("PivotTable1", "Sales"); removed != 1 {
		t.Fatalf("RemovePivotDataFields removed %d, want 1", removed)
	}
	if removed := sheet.ClearPivotRowFields("PivotTable1"); removed != 0 {
		t.Fatalf("ClearPivotRowFields removed %d, want 0", removed)
	}
	if removed := sheet.ClearPivotColumnFields("PivotTable1"); removed != 0 {
		t.Fatalf("ClearPivotColumnFields removed %d, want 0", removed)
	}
	if removed := sheet.ClearPivotPageFields("PivotTable1"); removed != 0 {
		t.Fatalf("ClearPivotPageFields removed %d, want 0", removed)
	}
	if removed := sheet.ClearPivotDataFields("PivotTable1"); removed != 0 {
		t.Fatalf("ClearPivotDataFields removed %d, want 0", removed)
	}
	if !sheet.RemovePivotTable("PivotTable1") {
		t.Fatalf("RemovePivotTable returned false")
	}
	if len(sheet.PivotTables()) != 0 {
		t.Fatalf("pivot tables after remove = %d, want 0", len(sheet.PivotTables()))
	}
}

func TestPivotTableNamedSourceRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	data, err := wb.CreateSheet("Data")
	if err != nil {
		t.Fatalf("CreateSheet Data: %v", err)
	}
	headers := []string{"Region", "Sales"}
	for col, header := range headers {
		cell, err := data.CreateCell(1, col+1)
		if err != nil {
			t.Fatalf("CreateCell header %d: %v", col+1, err)
		}
		cell.SetString(header)
	}
	rows := []struct {
		region string
		sales  float64
	}{
		{region: "North", sales: 10},
		{region: "South", sales: 20},
	}
	for i, row := range rows {
		r := i + 2
		region, err := data.CreateCell(r, 1)
		if err != nil {
			t.Fatalf("CreateCell region: %v", err)
		}
		region.SetString(row.region)
		sales, err := data.CreateCell(r, 2)
		if err != nil {
			t.Fatalf("CreateCell sales: %v", err)
		}
		sales.SetNumber(row.sales)
	}
	if _, err := wb.SetDefinedName("PivotData", "Data!$A$1:$B$3", nil); err != nil {
		t.Fatalf("SetDefinedName: %v", err)
	}
	pivotSheet, err := wb.CreateSheet("Pivot")
	if err != nil {
		t.Fatalf("CreateSheet Pivot: %v", err)
	}
	if _, err := pivotSheet.AddPivotTable("SalesPivot", "Data", "A1:B3", "A3:C8"); err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}
	if err := pivotSheet.SetPivotSourceName("SalesPivot", "PivotData"); err != nil {
		t.Fatalf("SetPivotSourceName: %v", err)
	}
	if err := pivotSheet.AddPivotRowField("SalesPivot", "Region"); err != nil {
		t.Fatalf("AddPivotRowField: %v", err)
	}
	if _, err := pivotSheet.SetPivotDataField("SalesPivot", sheetir.PivotDataField{
		SourceFieldName: "Sales",
		Name:            "Sales Total",
		Subtotal:        "sum",
		NumFmtID:        4,
	}); err != nil {
		t.Fatalf("SetPivotDataField: %v", err)
	}
	if err := pivotSheet.SetPivotLayout("SalesPivot", true, false, false); err != nil {
		t.Fatalf("SetPivotLayout: %v", err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}

	roundTrip := resolveWorkbook(t, buf.Bytes())
	got := roundTrip.Sheets[1]
	if len(got.PivotTables) != 1 {
		t.Fatalf("pivot tables = %d, want 1", len(got.PivotTables))
	}
	pivot := got.PivotTables[0]
	if pivot.Name != "SalesPivot" || pivot.SourceName != "PivotData" || pivot.SourceSheet != "" || pivot.SourceRef != "" {
		t.Errorf("pivot = %#v", pivot)
	}
	if !pivot.ShowHeaders || pivot.RowGrandTotals || pivot.ColGrandTotals {
		t.Errorf("pivot layout = %#v", pivot)
	}
	if len(pivot.DataFields) != 1 || pivot.DataFields[0].Name != "Sales Total" || pivot.DataFields[0].NumFmtID != 4 {
		t.Errorf("pivot data fields = %#v", pivot.DataFields)
	}
}

func TestOpenXLSXPivotRoundTripPreservesRawCacheParts(t *testing.T) {
	sourceData := buildExistingPivotWorkbookBytes(t, false)
	wb, err := edit.OpenXLSX(bytes.NewReader(sourceData), int64(len(sourceData)))
	if err != nil {
		t.Fatalf("OpenXLSX: %v", err)
	}

	var out bytes.Buffer
	if err := wb.SaveAsXLSX(&out); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	loaded := loadPartsPackageFromBytes(t, out.Bytes())
	if len(loaded.Worksheets) != 2 || len(loaded.Worksheets[1].PivotTables) != 1 {
		t.Fatalf("unexpected pivot tables after round-trip: %#v", loaded.Worksheets)
	}
	ptc := loaded.Worksheets[1].PivotTables[0]
	if ptc.Cache == nil || ptc.Cache.RefreshedBy != "Excel" || ptc.Cache.RefreshedDate != "2026-01-01T00:00:00Z" {
		t.Fatalf("pivot cache metadata = %#v", ptc.Cache)
	}
	if ptc.Cache.CreatedVersion != 3 || ptc.Cache.RefreshedVersion != 3 || ptc.Cache.MinRefreshableVersion != 3 {
		t.Fatalf("pivot cache versions = %#v", ptc.Cache)
	}
	if ptc.Records == nil || len(ptc.Records.Rows) != 2 || len(ptc.Records.Rows[0].Cells) != 2 || ptc.Records.Rows[0].Cells[1].Value != "10" {
		t.Fatalf("pivot cache records = %#v", ptc.Records)
	}
}

func TestOpenXLSXRefreshPivotTableRebuildsCacheFromSourceData(t *testing.T) {
	sourceData := buildExistingPivotWorkbookBytes(t, false)
	wb, err := edit.OpenXLSX(bytes.NewReader(sourceData), int64(len(sourceData)))
	if err != nil {
		t.Fatalf("OpenXLSX: %v", err)
	}
	dataSheet, ok := wb.SheetByName("Data")
	if !ok {
		t.Fatal("Data sheet not found")
	}
	valueCell, ok := dataSheet.Cell(2, 2)
	if !ok {
		t.Fatal("expected B2 cell")
	}
	valueCell.SetNumber(25)
	pivotSheet, ok := wb.SheetByName("Pivot")
	if !ok {
		t.Fatal("Pivot sheet not found")
	}
	if err := pivotSheet.RefreshPivotTable("SalesPivot"); err != nil {
		t.Fatalf("RefreshPivotTable: %v", err)
	}

	var out bytes.Buffer
	if err := wb.SaveAsXLSX(&out); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	loaded := loadPartsPackageFromBytes(t, out.Bytes())
	ptc := loaded.Worksheets[1].PivotTables[0]
	if ptc.Cache == nil || ptc.Cache.RefreshedBy != "office" {
		t.Fatalf("expected regenerated pivot cache metadata, got %#v", ptc.Cache)
	}
	if ptc.Records == nil || len(ptc.Records.Rows) != 2 || len(ptc.Records.Rows[0].Cells) != 2 || ptc.Records.Rows[0].Cells[1].Value != "25" {
		t.Fatalf("expected rebuilt pivot cache records, got %#v", ptc.Records)
	}
}

func TestOpenXLSXNamedPivotSourceDefinedNameEditRebuildsCache(t *testing.T) {
	sourceData := buildExistingPivotWorkbookBytes(t, true)
	wb, err := edit.OpenXLSX(bytes.NewReader(sourceData), int64(len(sourceData)))
	if err != nil {
		t.Fatalf("OpenXLSX: %v", err)
	}
	if _, err := wb.SetDefinedName("PivotData", "Data!$A$1:$B$2", nil); err != nil {
		t.Fatalf("SetDefinedName: %v", err)
	}

	var out bytes.Buffer
	if err := wb.SaveAsXLSX(&out); err != nil {
		t.Fatalf("SaveAsXLSX: %v", err)
	}
	loaded := loadPartsPackageFromBytes(t, out.Bytes())
	ptc := loaded.Worksheets[1].PivotTables[0]
	if ptc.Cache == nil || ptc.Cache.WorksheetSource == nil || ptc.Cache.WorksheetSource.Name != "PivotData" {
		t.Fatalf("expected named worksheet source, got %#v", ptc.Cache)
	}
	if ptc.Cache.RefreshedBy != "office" || ptc.Cache.RecordCount != 1 {
		t.Fatalf("expected regenerated named-source cache metadata, got %#v", ptc.Cache)
	}
	if ptc.Records == nil || len(ptc.Records.Rows) != 1 {
		t.Fatalf("expected one rebuilt cache record, got %#v", ptc.Records)
	}
}

func TestFormControlSemanticSettersAndScrollMetadataRoundTrip(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Controls")
	if err != nil {
		t.Fatal(err)
	}
	scroll, err := sheet.AddFormControl("Scroll", "Scroller", "0,0,0,0,2,0,1,0")
	if err != nil {
		t.Fatal(err)
	}
	scroll.Min = 1
	scroll.Max = 20
	scroll.Val = 7
	scroll.Increment = 2
	scroll.PageStep = 5
	scroll.Horizontal = true
	if err := sheet.SetFormControl(scroll); err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetFormControlValue("Scroller", 9); err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetFormControlValue("Scroller", 21); err == nil {
		t.Fatal("expected range validation")
	}

	list, err := sheet.AddFormControl("List", "Choices", "0,0,2,0,2,0,4,0")
	if err != nil {
		t.Fatal(err)
	}
	list.SelectionType = "extended"
	list.FmlaRange = "$A$1:$A$4"
	if err := sheet.SetFormControl(list); err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetFormControlSelection("Choices", 2, 4); err != nil {
		t.Fatal(err)
	}

	check, err := sheet.AddFormControl("Checkbox", "Enabled", "3,0,0,0,4,0,1,0")
	if err != nil {
		t.Fatal(err)
	}
	check.Mixed = true
	if err := sheet.SetFormControl(check); err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetFormControlChecked("Enabled", true); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := wb.SaveAsXLSX(&buf); err != nil {
		t.Fatal(err)
	}
	roundTrip := resolveWorkbook(t, buf.Bytes())
	got := roundTrip.Sheets[0]
	if len(got.FormControls) != 3 {
		t.Fatalf("controls=%d", len(got.FormControls))
	}
	gscroll := got.FormControls[0]
	if gscroll.Val != 9 || gscroll.Increment != 2 || gscroll.PageStep != 5 || !gscroll.Horizontal {
		t.Fatalf("scroll=%#v", gscroll)
	}
	if sel := got.FormControls[1].Selected; len(sel) != 2 || sel[0] != 2 || sel[1] != 4 {
		t.Fatalf("selected=%v", sel)
	}
	if !got.FormControls[2].Checked || got.FormControls[2].Mixed {
		t.Fatalf("checkbox=%#v", got.FormControls[2])
	}
}

func TestSelectRadioButtonClearsLinkedGroupPeers(t *testing.T) {
	wb := edit.NewWorkbook()
	sheet, err := wb.CreateSheet("Controls")
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"North", "South", "East"} {
		control, err := sheet.AddFormControl("Radio", name, fmt.Sprintf("0,0,%d,0,2,0,%d,0", i, i+1))
		if err != nil {
			t.Fatal(err)
		}
		control.FmlaLink = "$B$1"
		control.FirstButton = i == 0
		control.Checked = i == 0
		if err := sheet.SetFormControl(control); err != nil {
			t.Fatal(err)
		}
	}
	if err := sheet.SelectRadioButton("East"); err != nil {
		t.Fatal(err)
	}
	controls := sheet.FormControls()
	if controls[0].Checked || controls[1].Checked || !controls[2].Checked {
		t.Fatalf("radio states = %v, %v, %v", controls[0].Checked, controls[1].Checked, controls[2].Checked)
	}
}
