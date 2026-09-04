package legacytoooxml

import (
	"bytes"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

func TestConvertNilWorkbook(t *testing.T) {
	_, err := Convert(nil)
	if err == nil {
		t.Fatal("expected error for nil workbook")
	}
}

func TestConvertEmptyWorkbook(t *testing.T) {
	wb := &sheetir.Workbook{}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg == nil {
		t.Fatal("expected non-nil package")
	}
	if pkg.Workbook == nil {
		t.Fatal("expected non-nil workbook part")
	}
}

func TestConvertWorkbookWithSheet(t *testing.T) {
	wb := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{
			{
				Name: "Sheet1",
				ID:   1,
				Rows: []*sheetir.Row{
					{
						Index: 1,
						Cells: []*sheetir.Cell{
							{Row: 1, Col: 1, Ref: "A1", Kind: sheetir.CellString, StrValue: "Hello"},
							{Row: 1, Col: 2, Ref: "B1", Kind: sheetir.CellNumber, NumValue: 42},
						},
					},
				},
			},
		},
	}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Workbook.Sheets) != 1 {
		t.Fatalf("expected 1 sheet, got %d", len(pkg.Workbook.Sheets))
	}
	if len(pkg.Worksheets) != 1 {
		t.Fatalf("expected 1 worksheet, got %d", len(pkg.Worksheets))
	}
	ws := pkg.Worksheets[0].Sheet
	if len(ws.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(ws.Rows))
	}
	if len(ws.Rows[0].Cells) != 2 {
		t.Fatalf("expected 2 cells, got %d", len(ws.Rows[0].Cells))
	}
	if ws.Rows[0].Cells[0].Type != "s" {
		t.Fatalf("expected shared string type, got %q", ws.Rows[0].Cells[0].Type)
	}
	if len(pkg.SharedStrings.Items) != 1 {
		t.Fatalf("expected 1 shared string, got %d", len(pkg.SharedStrings.Items))
	}
}

func TestConvertMergedCells(t *testing.T) {
	wb := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{
			{
				Name: "Sheet1",
				MergedCells: []*sheetir.MergedCell{
					{TopRow: 1, LeftCol: 1, BottomRow: 2, RightCol: 2},
				},
			},
		},
	}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ws := pkg.Worksheets[0].Sheet
	if len(ws.MergeCells) != 1 {
		t.Fatalf("expected 1 merged cell, got %d", len(ws.MergeCells))
	}
	if ws.MergeCells[0].Ref != "A1:B2" {
		t.Fatalf("expected A1:B2, got %q", ws.MergeCells[0].Ref)
	}
}

func TestConvertStyles(t *testing.T) {
	wb := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{
			{
				Name: "Sheet1",
				Rows: []*sheetir.Row{
					{
						Index: 1,
						Cells: []*sheetir.Cell{
							{
								Row: 1, Col: 1, Ref: "A1",
								Kind:     sheetir.CellString,
								StrValue: "Bold",
								Style: sheetir.CellStyle{
									Bold:       true,
									FontSizePt: 12,
									FontColor:  "FF0000",
									BgColor:    "FFFF00",
								},
							},
						},
					},
				},
			},
		},
	}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Styles.Fonts) < 2 {
		t.Fatalf("expected at least 2 fonts (default + custom), got %d", len(pkg.Styles.Fonts))
	}
	if len(pkg.Styles.Fills) < 2 {
		t.Fatalf("expected at least 2 fills, got %d", len(pkg.Styles.Fills))
	}
	if len(pkg.Styles.CellXfs) < 2 {
		t.Fatalf("expected at least 2 cell xfs, got %d", len(pkg.Styles.CellXfs))
	}
}

func TestConvertTables(t *testing.T) {
	wb := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{
			{
				Name: "Sheet1",
				Tables: []*sheetir.Table{
					{
						ID:             1,
						Name:           "Table1",
						DisplayName:    "Table1",
						Ref:            "A1:C3",
						HeaderRowCount: 1,
						AutoFilterRef:  "A1:C3",
						TableStyle: &sheetir.TableStyleInfo{
							Name:           "TableStyleLight1",
							ShowRowStripes: true,
						},
						Columns: []*sheetir.TableColumn{
							{ID: 1, Name: "Name"},
							{ID: 2, Name: "Value"},
							{ID: 3, Name: "Total", TotalsRowFunction: "sum"},
						},
					},
				},
			},
		},
	}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Worksheets) != 1 {
		t.Fatalf("expected 1 worksheet, got %d", len(pkg.Worksheets))
	}
	if len(pkg.Worksheets[0].Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(pkg.Worksheets[0].Tables))
	}
	tbl := pkg.Worksheets[0].Tables[0]
	if tbl.Name != "Table1" || tbl.Ref != "A1:C3" {
		t.Errorf("table name/ref = %q/%q, want Table1/A1:C3", tbl.Name, tbl.Ref)
	}
	if len(tbl.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(tbl.Columns))
	}
	if tbl.Columns[2].TotalsRowFunction != "sum" {
		t.Errorf("column 3 totals function = %q, want sum", tbl.Columns[2].TotalsRowFunction)
	}

	// Round-trip through the parts writer/loader.
	var buf bytes.Buffer
	if err := sheetparts.WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}
	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}
	loaded, err := sheetparts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if len(loaded.Worksheets[0].Tables) != 1 {
		t.Fatalf("expected 1 table after round-trip, got %d", len(loaded.Worksheets[0].Tables))
	}
	if loaded.Worksheets[0].Sheet == nil || loaded.Worksheets[0].Sheet.TableParts == nil {
		t.Fatal("expected worksheet tableParts after round-trip")
	}
	if loaded.Worksheets[0].Sheet.TableParts.Count != 1 {
		t.Errorf("tableParts count = %d, want 1", loaded.Worksheets[0].Sheet.TableParts.Count)
	}
}

func TestConvertDrawingImageProducesMediaPart(t *testing.T) {
	wb := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{
			{
				Name: "Sheet1",
				Drawings: []*sheetir.Drawing{
					{
						Name: "logo",
						Type: sheetir.DrawingTypeImage,
						Position: sheetir.DrawingPosition{
							Anchor:   sheetir.AnchorOneCell,
							FromCell: sheetir.DrawingCellAnchor{Row: 1, Col: 1},
							Width:    914400,
							Height:   914400,
						},
						Image: &sheetir.DrawingImage{
							Data:        []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
							ContentType: "image/png",
						},
					},
				},
			},
		},
	}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Worksheets) != 1 {
		t.Fatalf("expected 1 worksheet, got %d", len(pkg.Worksheets))
	}
	entry := pkg.Worksheets[0]
	if entry.Drawing == nil {
		t.Fatal("expected drawing to be generated")
	}
	if len(entry.DrawingMedia) != 1 {
		t.Fatalf("expected 1 media part, got %d", len(entry.DrawingMedia))
	}
	for _, mp := range entry.DrawingMedia {
		if mp.Name != "xl/media/image1.png" {
			t.Errorf("media part name = %q, want xl/media/image1.png", mp.Name)
		}
		if mp.ContentType != "image/png" {
			t.Errorf("media part content type = %q, want image/png", mp.ContentType)
		}
	}
}

func TestConvertFormControlsRoundTrip(t *testing.T) {
	wb := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{{
			Name: "Sheet1",
			FormControls: []*sheetir.FormControl{
				{
					Type:     "Checkbox",
					AltText:  "Opt In",
					Anchor:   "0, 0, 0, 0, 1, 0, 1, 0",
					Mixed:    true,
					FmlaLink: "$B$1",
				},
				{
					Type:      "Drop",
					AltText:   "Status",
					Anchor:    "4, 0, 0, 0, 5, 0, 2, 0",
					FmlaLink:  "$C$1",
					FmlaRange: "$D$1:$D$3",
					Val:       2,
					DropLines: 7,
				},
				{
					Type:          "List",
					AltText:       "Teams",
					Anchor:        "6, 0, 0, 0, 7, 0, 3, 0",
					FmlaRange:     "$E$1:$E$4",
					SelectionType: "multi",
					Selected:      []int{1, 3},
					Val:           2,
				},
			},
		}},
	}

	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Worksheets) != 1 {
		t.Fatalf("expected 1 worksheet, got %d", len(pkg.Worksheets))
	}
	entry := pkg.Worksheets[0]
	if len(entry.FormControls) != 3 {
		t.Fatalf("expected 3 form controls, got %d", len(entry.FormControls))
	}
	if len(entry.LegacyDrawingVML) == 0 {
		t.Fatal("expected synthesized legacy drawing VML")
	}

	var buf bytes.Buffer
	if err := sheetparts.WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}
	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}
	loaded, err := sheetparts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if len(loaded.Worksheets) != 1 {
		t.Fatalf("expected 1 loaded worksheet, got %d", len(loaded.Worksheets))
	}
	loadedEntry := loaded.Worksheets[0]
	if len(loadedEntry.LegacyDrawingVML) == 0 {
		t.Fatal("expected legacy drawing VML after round-trip")
	}
	if loadedEntry.Sheet == nil || loadedEntry.Sheet.LegacyDrawing == nil {
		t.Fatal("expected worksheet legacyDrawing relationship after round-trip")
	}
	if len(loadedEntry.FormControls) != 3 {
		t.Fatalf("expected 3 loaded form controls, got %d", len(loadedEntry.FormControls))
	}
	if got := loadedEntry.FormControls[0]; got.Type != "Checkbox" || got.AltText != "Opt In" || got.Checked || !got.Mixed || got.FmlaLink != "$B$1" {
		t.Fatalf("checkbox round-trip = %#v", got)
	}
	if got := loadedEntry.FormControls[1]; got.Type != "Drop" || got.AltText != "Status" || got.FmlaLink != "$C$1" || got.FmlaRange != "$D$1:$D$3" || got.Val != 2 || got.DropLines != 7 {
		t.Fatalf("drop round-trip = %#v", got)
	}
	if got := loadedEntry.FormControls[2]; got.Type != "List" || got.AltText != "Teams" || got.FmlaRange != "$E$1:$E$4" || got.SelectionType != "multi" || got.Val != 2 {
		t.Fatalf("list round-trip = %#v", got)
	}
	if got := loadedEntry.FormControls[2].Selected; len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("list selected round-trip = %v, want [1 3]", got)
	}
}

func TestConvertRadioGroupControlsRoundTrip(t *testing.T) {
	wb := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{{
			Name: "Sheet1",
			Rows: []*sheetir.Row{{
				Index: 1,
				Cells: []*sheetir.Cell{{
					Row:      1,
					Col:      8,
					Ref:      "H1",
					Kind:     sheetir.CellNumber,
					NumValue: 99,
				}},
			}},
			FormControls: []*sheetir.FormControl{
				{
					Type:      "GBox",
					AltText:   "Region",
					Anchor:    "0, 0, 4, 0, 3, 0, 8, 0",
					FmlaGroup: "$H$1",
				},
				{
					Type:        "Radio",
					AltText:     "North",
					Anchor:      "1, 0, 5, 0, 2, 0, 6, 0",
					Checked:     true,
					FirstButton: true,
					FmlaLink:    "$H$1",
				},
				{
					Type:     "Radio",
					AltText:  "South",
					Anchor:   "1, 0, 6, 0, 2, 0, 7, 0",
					FmlaLink: "$H$1",
				},
			},
		}},
	}

	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry := pkg.Worksheets[0]
	if len(entry.FormControls) != 3 {
		t.Fatalf("expected 3 form controls, got %d", len(entry.FormControls))
	}

	var buf bytes.Buffer
	if err := sheetparts.WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}
	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}
	loaded, err := sheetparts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	loadedEntry := loaded.Worksheets[0]
	if len(loadedEntry.FormControls) != 3 {
		t.Fatalf("expected 3 loaded form controls, got %d", len(loadedEntry.FormControls))
	}
	if got := loadedEntry.FormControls[0]; got.Type != "GBox" || got.FmlaGroup != "$H$1" {
		t.Fatalf("group box round-trip = %#v", got)
	}
	if got := loadedEntry.FormControls[1]; got.Type != "Radio" || !got.Checked || !got.FirstButton || got.FmlaLink != "$H$1" {
		t.Fatalf("first radio round-trip = %#v", got)
	}
	if got := loadedEntry.FormControls[2]; got.Type != "Radio" || got.Checked || got.FirstButton || got.FmlaLink != "$H$1" {
		t.Fatalf("second radio round-trip = %#v", got)
	}
	if loadedEntry.Sheet == nil || len(loadedEntry.Sheet.Rows) == 0 || len(loadedEntry.Sheet.Rows[0].Cells) == 0 {
		t.Fatalf("expected worksheet rows after round-trip, got %#v", loadedEntry.Sheet)
	}
	gotCell := loadedEntry.Sheet.Rows[0].Cells[0]
	if gotCell.Ref != "H1" || gotCell.Value != "1" {
		t.Fatalf("linked cell round-trip = %#v, want H1=1", gotCell)
	}
}

func TestConvertScalarFormControlLinkedCellsRoundTrip(t *testing.T) {
	wb := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{{
			Name: "Sheet1",
			Rows: []*sheetir.Row{{
				Index: 1,
				Cells: []*sheetir.Cell{
					{Row: 1, Col: 3, Ref: "C1", Kind: sheetir.CellNumber, NumValue: 99},
					{Row: 1, Col: 6, Ref: "F1", Kind: sheetir.CellNumber, NumValue: 99},
					{Row: 1, Col: 9, Ref: "I1", Kind: sheetir.CellNumber, NumValue: 99},
				},
			}},
			FormControls: []*sheetir.FormControl{
				{
					Type:      "Drop",
					AltText:   "Status",
					Anchor:    "0, 0, 0, 0, 1, 0, 1, 0",
					FmlaLink:  "$C$1",
					FmlaRange: "$D$1:$D$3",
					Val:       2,
				},
				{
					Type:          "List",
					AltText:       "Priority",
					Anchor:        "2, 0, 0, 0, 3, 0, 2, 0",
					FmlaLink:      "$F$1",
					FmlaRange:     "$G$1:$G$3",
					SelectionType: "single",
					Val:           3,
				},
				{
					Type:     "Spin",
					AltText:  "Counter",
					Anchor:   "4, 0, 0, 0, 5, 0, 1, 0",
					FmlaLink: "$I$1",
					Min:      0,
					Max:      9,
					Val:      4,
				},
			},
		}},
	}

	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if err := sheetparts.WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}
	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}
	loaded, err := sheetparts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	loadedEntry := loaded.Worksheets[0]
	if got := findWrittenCellValue(loadedEntry.Sheet, "C1"); got != "2" {
		t.Fatalf("drop linked cell = %q, want 2", got)
	}
	if got := findWrittenCellValue(loadedEntry.Sheet, "F1"); got != "3" {
		t.Fatalf("list linked cell = %q, want 3", got)
	}
	if got := findWrittenCellValue(loadedEntry.Sheet, "I1"); got != "4" {
		t.Fatalf("spin linked cell = %q, want 4", got)
	}
}

func findWrittenCellValue(ws *sheetparts.Worksheet, ref string) string {
	if ws == nil {
		return ""
	}
	for _, row := range ws.Rows {
		for _, cell := range row.Cells {
			if strings.EqualFold(cell.Ref, ref) {
				return cell.Value
			}
		}
	}
	return ""
}

func TestConvertConnections(t *testing.T) {
	wb := &sheetir.Workbook{
		Connections: []*sheetir.WorkbookConnection{
			{
				ID:            1,
				Name:          "SalesDB",
				Kind:          "1",
				Description:   "Sales database",
				RefreshOnLoad: true,
				SaveData:      true,
				Connection:    "Provider=SQLOLEDB",
				Command:       "SELECT * FROM Sales",
				CommandType:   "2",
			},
		},
	}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Connections == nil {
		t.Fatal("expected connections")
	}
	if len(pkg.Connections.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(pkg.Connections.Connections))
	}
	conn := pkg.Connections.Connections[0]
	if conn.Name != "SalesDB" {
		t.Errorf("connection name = %q, want SalesDB", conn.Name)
	}
	if conn.DBPr == nil || conn.DBPr.Command != "SELECT * FROM Sales" {
		t.Errorf("connection dbPr.command = %q, want SELECT * FROM Sales", conn.DBPr.Command)
	}
}

func TestConvertProtectionMetadata(t *testing.T) {
	wb := &sheetir.Workbook{
		Protection: &sheetir.WorkbookProtection{
			WorkbookPassword: "ABCD",
			LockStructure:    true,
			LockWindows:      true,
			AlgorithmName:    "SHA-512",
			HashValue:        "deadbeef",
			SaltValue:        "c2FsdA==",
			SpinCount:        100000,
		},
		Sheets: []*sheetir.Sheet{{
			Name: "Protected",
			Protection: &sheetir.SheetProtection{
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
			},
		}},
	}

	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Workbook == nil || pkg.Workbook.Protection == nil {
		t.Fatal("expected workbook protection in OOXML package")
	}
	if !pkg.Workbook.Protection.LockStructure || pkg.Workbook.Protection.HashValue != "deadbeef" {
		t.Fatalf("unexpected workbook protection: %+v", pkg.Workbook.Protection)
	}
	if len(pkg.Worksheets) != 1 || pkg.Worksheets[0].Sheet == nil || pkg.Worksheets[0].Sheet.SheetProtection == nil {
		t.Fatalf("expected worksheet protection in OOXML package, got %+v", pkg.Worksheets)
	}
	if sp := pkg.Worksheets[0].Sheet.SheetProtection; !sp.Sheet || sp.Password != "C0DE" || sp.HashValue != "beadfeed" {
		t.Fatalf("unexpected worksheet protection: %+v", sp)
	}
}

func TestConvertExternalLinks(t *testing.T) {
	sheetID := 1
	wb := &sheetir.Workbook{
		ExternalLinks: []*sheetir.WorkbookExternalLink{
			{
				Target:     "external.xlsx",
				SheetNames: []string{"Data"},
				DefinedNames: []*sheetir.WorkbookExternalDefinedName{
					{Name: "ExtName", RefersTo: "'Data'!$A$1", SheetID: &sheetID},
				},
				CachedSheets: []*sheetir.WorkbookExternalSheet{
					{
						ID:   1,
						Name: "Data",
						Cells: []*sheetir.WorkbookExternalCell{
							{Ref: "A1", Type: "n", Value: "123"},
							{Ref: "A2", Type: "s", Value: "hello"},
						},
					},
				},
			},
		},
	}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.ExternalLinks) != 1 {
		t.Fatalf("expected 1 external link, got %d", len(pkg.ExternalLinks))
	}
	entry := pkg.ExternalLinks[0]
	if entry.Target != "external.xlsx" {
		t.Errorf("target = %q, want external.xlsx", entry.Target)
	}
	if entry.Link == nil || entry.Link.ExternalBook == nil {
		t.Fatal("expected external book")
	}
	if len(entry.Link.ExternalBook.SheetNames) != 1 || entry.Link.ExternalBook.SheetNames[0].Val != "Data" {
		t.Errorf("sheet names = %v, want [Data]", entry.Link.ExternalBook.SheetNames)
	}
	if len(entry.Link.ExternalBook.DefinedNames) != 1 {
		t.Fatalf("expected 1 defined name, got %d", len(entry.Link.ExternalBook.DefinedNames))
	}
	if entry.Link.ExternalBook.SheetDataSet == nil || len(entry.Link.ExternalBook.SheetDataSet.Sheets) != 1 {
		t.Fatal("expected cached sheet data")
	}

	// Round-trip.
	var buf bytes.Buffer
	if err := sheetparts.WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}
	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}
	loaded, err := sheetparts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if len(loaded.ExternalLinks) != 1 {
		t.Fatalf("expected 1 external link after round-trip, got %d", len(loaded.ExternalLinks))
	}
	if loaded.ExternalLinks[0].Target != "external.xlsx" {
		t.Errorf("round-trip target = %q, want external.xlsx", loaded.ExternalLinks[0].Target)
	}
}

func TestConvertPivotTables(t *testing.T) {
	wb := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{
			{
				Name: "Data",
				Rows: []*sheetir.Row{
					{
						Index: 1,
						Cells: []*sheetir.Cell{
							{Row: 1, Col: 1, Ref: "A1", Kind: sheetir.CellString, StrValue: "Name"},
							{Row: 1, Col: 2, Ref: "B1", Kind: sheetir.CellString, StrValue: "Value"},
						},
					},
					{
						Index: 2,
						Cells: []*sheetir.Cell{
							{Row: 2, Col: 1, Ref: "A2", Kind: sheetir.CellString, StrValue: "A"},
							{Row: 2, Col: 2, Ref: "B2", Kind: sheetir.CellNumber, NumValue: 10},
						},
					},
					{
						Index: 3,
						Cells: []*sheetir.Cell{
							{Row: 3, Col: 1, Ref: "A3", Kind: sheetir.CellString, StrValue: "B"},
							{Row: 3, Col: 2, Ref: "B3", Kind: sheetir.CellNumber, NumValue: 20},
						},
					},
				},
			},
			{
				Name: "Pivot",
				PivotTables: []*sheetir.PivotTable{
					{
						Name:           "Pivot1",
						LocationRef:    "A1:C3",
						DataCaption:    "Measures",
						SourceSheet:    "Data",
						SourceRef:      "A1:B3",
						RowGrandTotals: false,
						ColGrandTotals: true,
						ShowHeaders:    false,
						RowFields:      []string{"Name"},
						DataFields: []*sheetir.PivotDataField{
							{Name: "Sum of Value", SourceFieldName: "Value", Subtotal: "sum", NumFmtID: 4},
						},
					},
				},
			},
		},
	}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Worksheets) != 2 {
		t.Fatalf("expected 2 worksheets, got %d", len(pkg.Worksheets))
	}
	if len(pkg.Worksheets[1].PivotTables) != 1 {
		t.Fatalf("expected 1 pivot table, got %d", len(pkg.Worksheets[1].PivotTables))
	}
	ptc := pkg.Worksheets[1].PivotTables[0]
	if ptc.Table == nil || ptc.Table.Name != "Pivot1" {
		t.Errorf("table name = %q, want Pivot1", ptc.Table.Name)
	}
	if ptc.Table == nil || ptc.Table.DataCaption != "Measures" || ptc.Table.RowGrandTotals == nil || *ptc.Table.RowGrandTotals || ptc.Table.ColGrandTotals == nil || !*ptc.Table.ColGrandTotals || ptc.Table.ShowHeaders == nil || *ptc.Table.ShowHeaders {
		t.Fatalf("pivot table display metadata = %#v", ptc.Table)
	}
	if ptc.Cache == nil || ptc.Cache.CacheFields == nil {
		t.Fatal("expected pivot cache fields")
	}
	if len(ptc.Table.DataFields) != 1 || ptc.Table.DataFields[0].NumFmtID != 4 {
		t.Fatalf("pivot table data fields = %#v", ptc.Table.DataFields)
	}
	if len(ptc.Cache.CacheFields.Fields) != 2 {
		t.Errorf("cache fields = %d, want 2", len(ptc.Cache.CacheFields.Fields))
	}
	if ptc.Records == nil || len(ptc.Records.Rows) != 2 {
		t.Errorf("expected 2 cache records, got %d", len(ptc.Records.Rows))
	}

	// Round-trip.
	var buf bytes.Buffer
	if err := sheetparts.WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}
	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}
	loaded, err := sheetparts.LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if len(loaded.Worksheets[1].PivotTables) != 1 {
		t.Fatalf("expected 1 pivot table after round-trip, got %d", len(loaded.Worksheets[1].PivotTables))
	}
	loadedPivot := loaded.Worksheets[1].PivotTables[0]
	if loadedPivot.Table == nil || loadedPivot.Table.DataCaption != "Measures" || loadedPivot.Table.RowGrandTotals == nil || *loadedPivot.Table.RowGrandTotals || loadedPivot.Table.ColGrandTotals == nil || !*loadedPivot.Table.ColGrandTotals || loadedPivot.Table.ShowHeaders == nil || *loadedPivot.Table.ShowHeaders {
		t.Fatalf("expected pivot display metadata after round-trip, got %#v", loadedPivot.Table)
	}
	if len(loadedPivot.Table.DataFields) != 1 || loadedPivot.Table.DataFields[0].NumFmtID != 4 {
		t.Fatalf("expected parsed pivot data field numFmtId after round-trip, got %#v", loadedPivot.Table.DataFields)
	}
	if loadedPivot.Cache == nil || loadedPivot.Cache.CacheFields == nil || len(loadedPivot.Cache.CacheFields.Fields) != 2 {
		t.Fatalf("expected parsed pivot cache fields after round-trip, got %#v", loadedPivot.Cache)
	}
	if loadedPivot.Records == nil || len(loadedPivot.Records.Rows) != 2 || len(loadedPivot.Records.Rows[0].Cells) != 2 {
		t.Fatalf("expected parsed pivot cache records after round-trip, got %#v", loadedPivot.Records)
	}
	if loaded.Workbook.PivotCaches == nil || len(loaded.Workbook.PivotCaches) != 1 {
		t.Fatalf("expected 1 workbook pivot cache, got %d", len(loaded.Workbook.PivotCaches))
	}
}

func TestConvertPivotTablesWithNamedSource(t *testing.T) {
	wb := &sheetir.Workbook{
		DefinedNames: []*sheetir.DefinedName{
			{Name: "PivotData", Value: "Data!$A$1:$B$3"},
		},
		Sheets: []*sheetir.Sheet{
			{
				Name: "Data",
				Rows: []*sheetir.Row{
					{
						Index: 1,
						Cells: []*sheetir.Cell{
							{Row: 1, Col: 1, Ref: "A1", Kind: sheetir.CellString, StrValue: "Name"},
							{Row: 1, Col: 2, Ref: "B1", Kind: sheetir.CellString, StrValue: "Value"},
						},
					},
					{
						Index: 2,
						Cells: []*sheetir.Cell{
							{Row: 2, Col: 1, Ref: "A2", Kind: sheetir.CellString, StrValue: "A"},
							{Row: 2, Col: 2, Ref: "B2", Kind: sheetir.CellNumber, NumValue: 10},
						},
					},
					{
						Index: 3,
						Cells: []*sheetir.Cell{
							{Row: 3, Col: 1, Ref: "A3", Kind: sheetir.CellString, StrValue: "B"},
							{Row: 3, Col: 2, Ref: "B3", Kind: sheetir.CellNumber, NumValue: 20},
						},
					},
				},
			},
			{
				Name: "Pivot",
				PivotTables: []*sheetir.PivotTable{
					{
						Name:        "Pivot1",
						LocationRef: "A1:C3",
						SourceName:  "PivotData",
						RowFields:   []string{"Name"},
						DataFields: []*sheetir.PivotDataField{
							{Name: "Sum of Value", SourceFieldName: "Value", Subtotal: "sum"},
						},
					},
				},
			},
		},
	}

	pkg, err := Convert(wb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Worksheets) != 2 || len(pkg.Worksheets[1].PivotTables) != 1 {
		t.Fatalf("unexpected pivot table count: %#v", pkg.Worksheets)
	}
	ptc := pkg.Worksheets[1].PivotTables[0]
	if ptc.Cache == nil || ptc.Cache.WorksheetSource == nil {
		t.Fatal("expected pivot cache worksheet source")
	}
	if ptc.Cache.WorksheetSource.Name != "PivotData" {
		t.Fatalf("worksheet source name = %q, want PivotData", ptc.Cache.WorksheetSource.Name)
	}
	if ptc.Cache.WorksheetSource.Sheet != "" || ptc.Cache.WorksheetSource.Ref != "" {
		t.Fatalf("worksheet source = %#v, want name-only source", ptc.Cache.WorksheetSource)
	}
	if ptc.Records == nil || len(ptc.Records.Rows) != 2 {
		t.Fatalf("expected 2 cache records, got %#v", ptc.Records)
	}
}
