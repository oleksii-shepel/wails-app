package legacy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	chartpkg "office/pkg/ooxml/chart"
	drawing "office/pkg/ooxml/drawing"
	sheetir "office/pkg/sheet/ir"
	sheetrender "office/pkg/sheet/render"
)

func TestParseWorksheetHandlesBlankCell(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffBlank, blankCellPayload(0, 1, 1))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Blank"}, nil, []legacyXF{{}, {NumFmtCode: "@"}})
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(sheet.Rows))
	}
	if len(sheet.Rows[0].Cells) != 1 {
		t.Fatalf("cells = %d, want 1", len(sheet.Rows[0].Cells))
	}
	cell := sheet.Rows[0].Cells[0]
	if cell.Kind != sheetir.CellEmpty {
		t.Fatalf("cell kind = %v, want empty", cell.Kind)
	}
	if cell.Ref != "B1" {
		t.Fatalf("cell ref = %q, want B1", cell.Ref)
	}
	if cell.Style.NumFmtCode != "@" {
		t.Fatalf("cell style = %q, want @", cell.Style.NumFmtCode)
	}
}

func TestParseWorksheetHandlesMulBlankCells(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffMulBlank, mulBlankPayload(2, 0, []uint16{1, 2, 1}))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "MulBlank"}, nil, []legacyXF{{}, {NumFmtCode: "0.00"}, {NumFmtCode: "m/d/yy"}})
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(sheet.Rows))
	}
	row := sheet.Rows[0]
	if row.Index != 3 {
		t.Fatalf("row index = %d, want 3", row.Index)
	}
	if len(row.Cells) != 3 {
		t.Fatalf("cells = %d, want 3", len(row.Cells))
	}
	for index, wantRef := range []string{"A3", "B3", "C3"} {
		cell := row.Cells[index]
		if cell.Kind != sheetir.CellEmpty {
			t.Fatalf("cell %d kind = %v, want empty", index, cell.Kind)
		}
		if cell.Ref != wantRef {
			t.Fatalf("cell %d ref = %q, want %s", index, cell.Ref, wantRef)
		}
	}
	if row.Cells[1].Style.NumFmtCode != "m/d/yy" {
		t.Fatalf("middle cell style = %q, want m/d/yy", row.Cells[1].Style.NumFmtCode)
	}
}

func TestApplyColInfoCapturesOutlineAndCollapsedFlags(t *testing.T) {
	sheet := &sheetir.Sheet{
		ColumnWidths:  map[int]float64{},
		HiddenCols:    map[int]bool{},
		OutlineCols:   map[int]int{},
		CollapsedCols: map[int]bool{},
	}
	payload := make([]byte, 10)
	binary.LittleEndian.PutUint16(payload[0:2], 1)
	binary.LittleEndian.PutUint16(payload[2:4], 2)
	binary.LittleEndian.PutUint16(payload[4:6], 512)
	binary.LittleEndian.PutUint16(payload[8:10], 0x1201)

	applyColInfo(sheet, payload)

	for _, col := range []int{2, 3} {
		if sheet.ColumnWidths[col] != 2 {
			t.Fatalf("col %d width = %v, want 2", col, sheet.ColumnWidths[col])
		}
		if !sheet.HiddenCols[col] {
			t.Fatalf("col %d hidden = false, want true", col)
		}
		if sheet.OutlineCols[col] != 2 {
			t.Fatalf("col %d outline = %d, want 2", col, sheet.OutlineCols[col])
		}
		if !sheet.CollapsedCols[col] {
			t.Fatalf("col %d collapsed = false, want true", col)
		}
	}
}

func TestParseWorksheetPromotesRowOutlineMetadata(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffRow, rowPayload(4, 0x0012))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Rows"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.OutlineRows[5] != 2 {
		t.Fatalf("row 5 outline = %d, want 2", sheet.OutlineRows[5])
	}
	if !sheet.CollapsedRows[5] {
		t.Fatalf("row 5 collapsed = false, want true")
	}
}

func TestParseWorksheetCapturesGutsRecord(t *testing.T) {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint16(payload[0:2], 18) // left row gutter
	binary.LittleEndian.PutUint16(payload[2:4], 12) // top col gutter
	binary.LittleEndian.PutUint16(payload[4:6], 4)  // row level max
	binary.LittleEndian.PutUint16(payload[6:8], 3)  // col level max

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffGuts, payload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Guts"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.RowGutter != 18 || sheet.ColGutter != 12 {
		t.Fatalf("gutters = row:%d col:%d, want row:18 col:12", sheet.RowGutter, sheet.ColGutter)
	}
	if sheet.RowOutlineLevelMax != 4 || sheet.ColOutlineLevelMax != 3 {
		t.Fatalf("outline max = row:%d col:%d, want row:4 col:3", sheet.RowOutlineLevelMax, sheet.ColOutlineLevelMax)
	}
}

func TestParseWorksheetCapturesDefaultRowHeight(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint16(payload[2:4], 360)
	data = appendBiffRecord(data, biffDefRowHeight, payload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Defaults"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.DefaultRowHeightPt != 18 {
		t.Fatalf("default row height = %v, want 18", sheet.DefaultRowHeightPt)
	}
}

func TestParseWorksheetCapturesDefaultColWidth(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	payload := make([]byte, 2)
	binary.LittleEndian.PutUint16(payload[0:2], 12)
	data = appendBiffRecord(data, biffDefColWidth, payload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Defaults"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.DefaultColWidthChars != 12 {
		t.Fatalf("default col width = %v, want 12", sheet.DefaultColWidthChars)
	}
}

func TestParseWorksheetBuildsTableColumnsFromHeaderRowCellText(t *testing.T) {
	labelSst := func(row, col, xf, sstIndex int) []byte {
		payload := make([]byte, 10)
		binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
		binary.LittleEndian.PutUint16(payload[2:4], uint16(col))
		binary.LittleEndian.PutUint16(payload[4:6], uint16(xf))
		binary.LittleEndian.PutUint32(payload[6:10], uint32(sstIndex))
		return payload
	}
	featPayload := func(firstRow, lastRow, firstCol, lastCol, headerRows, totalsRows int) []byte {
		payload := make([]byte, 27+8+16)
		binary.LittleEndian.PutUint16(payload[12:14], 0x05) // SHAREDFEATURES_ISFLIST
		binary.LittleEndian.PutUint16(payload[19:21], 1)    // cref
		binary.LittleEndian.PutUint32(payload[21:25], 16)   // cbFeatData
		binary.LittleEndian.PutUint16(payload[27:29], uint16(firstRow))
		binary.LittleEndian.PutUint16(payload[29:31], uint16(lastRow))
		binary.LittleEndian.PutUint16(payload[31:33], uint16(firstCol))
		binary.LittleEndian.PutUint16(payload[33:35], uint16(lastCol))
		binary.LittleEndian.PutUint32(payload[39:43], uint32(headerRows))
		binary.LittleEndian.PutUint32(payload[43:47], uint32(totalsRows))
		return payload
	}

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffLabelSst, labelSst(0, 0, 0, 0))
	data = appendBiffRecord(data, biffLabelSst, labelSst(0, 1, 0, 1))
	data = appendBiffRecord(data, biffFeat, featPayload(0, 2, 0, 1, 1, 1))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Tables"}, []string{" Name ", "Score"}, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(sheet.Tables))
	}
	table := sheet.Tables[0]
	if table.HeaderRowCount != 1 || table.TotalsRowCount != 1 || !table.TotalsRowShown {
		t.Fatalf("table header/totals = (%d,%d,%v), want (1,1,true)", table.HeaderRowCount, table.TotalsRowCount, table.TotalsRowShown)
	}
	if len(table.Columns) != 2 {
		t.Fatalf("columns = %d, want 2", len(table.Columns))
	}
	if table.Columns[0].ID != 1 || table.Columns[0].Name != "Name" {
		t.Fatalf("column 0 = %#v, want ID=1 Name=Name", table.Columns[0])
	}
	if table.Columns[1].ID != 2 || table.Columns[1].Name != "Score" {
		t.Fatalf("column 1 = %#v, want ID=2 Name=Score", table.Columns[1])
	}
}

func TestParseWorksheetBuildsFeature11TableStyleAndDisplayName(t *testing.T) {
	labelSst := func(row, col, xf, sstIndex int) []byte {
		payload := make([]byte, 10)
		binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
		binary.LittleEndian.PutUint16(payload[2:4], uint16(col))
		binary.LittleEndian.PutUint16(payload[4:6], uint16(xf))
		binary.LittleEndian.PutUint32(payload[6:10], uint32(sstIndex))
		return payload
	}

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffLabelSst, labelSst(0, 0, 0, 0))
	data = appendBiffRecord(data, biffLabelSst, labelSst(0, 1, 0, 1))
	data = appendBiffRecord(data, biffFeature11, legacyFeature11Payload(0, 2, 0, 1, 9, "LegacyTable", 1, 1))
	data = appendBiffRecord(data, biffList12, legacyList12StylePayload(9, 0x0005, "TableStyleMedium9"))
	data = appendBiffRecord(data, biffList12, legacyList12DisplayNamePayload(9, "Sales_Table", "ignored"))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Tables"}, []string{" Name ", "Score"}, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(sheet.Tables))
	}
	table := sheet.Tables[0]
	if table.ID != 9 {
		t.Fatalf("table ID = %d, want 9", table.ID)
	}
	if table.Name != "LegacyTable" {
		t.Fatalf("table name = %q, want LegacyTable", table.Name)
	}
	if table.DisplayName != "Sales_Table" {
		t.Fatalf("table display name = %q, want Sales_Table", table.DisplayName)
	}
	if table.Ref != "A1:B3" {
		t.Fatalf("table ref = %q, want A1:B3", table.Ref)
	}
	if table.HeaderRowCount != 1 || table.TotalsRowCount != 1 || !table.TotalsRowShown {
		t.Fatalf("table header/totals = (%d,%d,%v), want (1,1,true)", table.HeaderRowCount, table.TotalsRowCount, table.TotalsRowShown)
	}
	if table.TableStyle == nil {
		t.Fatal("expected table style")
	}
	if table.TableStyle.Name != "TableStyleMedium9" {
		t.Fatalf("table style name = %q, want TableStyleMedium9", table.TableStyle.Name)
	}
	if !table.TableStyle.ShowFirstColumn || table.TableStyle.ShowLastColumn || !table.TableStyle.ShowRowStripes || table.TableStyle.ShowColumnStripes {
		t.Fatalf("table style flags = %#v, want first-column + row-stripes only", table.TableStyle)
	}
	if len(table.Columns) != 2 {
		t.Fatalf("columns = %d, want 2", len(table.Columns))
	}
	if table.Columns[0].Name != "Name" || table.Columns[1].Name != "Score" {
		t.Fatalf("columns = (%q,%q), want (Name,Score)", table.Columns[0].Name, table.Columns[1].Name)
	}
}

func TestApplyDefaultColWidthIgnoresZeroAndShortPayload(t *testing.T) {
	sheet := &sheetir.Sheet{}
	applyDefaultColWidth(sheet, []byte{0x00, 0x00})
	if sheet.DefaultColWidthChars != 0 {
		t.Fatalf("default col width = %v, want 0 for zero payload", sheet.DefaultColWidthChars)
	}
	applyDefaultColWidth(sheet, []byte{0x08})
	if sheet.DefaultColWidthChars != 0 {
		t.Fatalf("default col width = %v, want 0 for short payload", sheet.DefaultColWidthChars)
	}
}

func legacyFeature11Payload(firstRow, lastRow, firstCol, lastCol int, idList uint32, name string, headerRows, totalsRows int) []byte {
	nameBytes := xlUnicodeString(name)
	payload := make([]byte, 27+64+len(nameBytes)+2)
	binary.LittleEndian.PutUint16(payload[0:2], biffFeature11)
	binary.LittleEndian.PutUint16(payload[4:6], uint16(firstRow))
	binary.LittleEndian.PutUint16(payload[6:8], uint16(lastRow))
	binary.LittleEndian.PutUint16(payload[8:10], uint16(firstCol))
	binary.LittleEndian.PutUint16(payload[10:12], uint16(lastCol))
	binary.LittleEndian.PutUint16(payload[12:14], 0x0005)
	binary.LittleEndian.PutUint32(payload[21:25], uint32(64+len(nameBytes)+2))
	feat := payload[27:]
	binary.LittleEndian.PutUint32(feat[4:8], idList)
	binary.LittleEndian.PutUint32(feat[8:12], uint32(headerRows))
	binary.LittleEndian.PutUint32(feat[12:16], uint32(totalsRows))
	binary.LittleEndian.PutUint32(feat[20:24], 64)
	copy(feat[64:], nameBytes)
	return payload
}

func legacyList12StylePayload(idList uint32, flags uint16, styleName string) []byte {
	styleBytes := xlUnicodeString(styleName)
	payload := make([]byte, 18+2+len(styleBytes))
	binary.LittleEndian.PutUint16(payload[0:2], biffList12)
	binary.LittleEndian.PutUint16(payload[12:14], 0x0001)
	binary.LittleEndian.PutUint32(payload[14:18], idList)
	binary.LittleEndian.PutUint16(payload[18:20], flags)
	copy(payload[20:], styleBytes)
	return payload
}

func legacyList12DisplayNamePayload(idList uint32, displayName, comment string) []byte {
	nameBytes := xlUnicodeString(displayName)
	commentBytes := xlUnicodeString(comment)
	payload := make([]byte, 18+len(nameBytes)+len(commentBytes))
	binary.LittleEndian.PutUint16(payload[0:2], biffList12)
	binary.LittleEndian.PutUint16(payload[12:14], 0x0002)
	binary.LittleEndian.PutUint32(payload[14:18], idList)
	copy(payload[18:], nameBytes)
	copy(payload[18+len(nameBytes):], commentBytes)
	return payload
}

func xlUnicodeString(text string) []byte {
	payload := make([]byte, 3+len(text))
	binary.LittleEndian.PutUint16(payload[0:2], uint16(len(text)))
	payload[2] = 0x00
	copy(payload[3:], []byte(text))
	return payload
}

func TestLegacyWorksheetColumnPxUsesDefaultColWidthFallback(t *testing.T) {
	sheet := &sheetir.Sheet{
		ColumnWidths:         map[int]float64{},
		DefaultColWidthChars: 10,
	}
	got := legacyWorksheetColumnPx(sheet, 4)
	want := 70 // 10 * legacyCharUnitToPx
	if got != want {
		t.Fatalf("legacyWorksheetColumnPx() = %d, want %d", got, want)
	}
}

func TestParseWorksheetCombinesCommentTextAcrossContinueRecords(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffTxO, nil)
	data = appendBiffRecord(data, biffContinue, append([]byte{0x00}, []byte("Hello ")...))
	data = appendBiffRecord(data, biffContinue, append([]byte{0x00}, []byte("World")...))
	note := make([]byte, 4)
	data = appendBiffRecord(data, biffNote, note)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Comments"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.Comments["A1"] != "Hello World" {
		t.Fatalf("comment = %q, want Hello World", sheet.Comments["A1"])
	}
}

func TestParseWorksheetCommentTextStopsAtTxOCharCount(t *testing.T) {
	txo := make([]byte, 8)
	binary.LittleEndian.PutUint16(txo[4:6], 5) // cchText

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffTxO, txo)
	// "Hello" + trailing run bytes that should not be interpreted as comment text.
	data = appendBiffRecord(data, biffContinue, append([]byte{0x00}, append([]byte("Hello"), []byte{0x00, 0x01, 0x02, 0x03}...)...))
	note := make([]byte, 4)
	data = appendBiffRecord(data, biffNote, note)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Comments"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.Comments["A1"] != "Hello" {
		t.Fatalf("comment = %q, want Hello", sheet.Comments["A1"])
	}
}

func TestParseWorksheetAssociatesCommentTextByObjectID(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffNote, notePayload(1, 1, 7, "Bob"))
	data = appendBiffRecord(data, biffNote, notePayload(0, 0, 5, "Alice"))
	data = appendBiffRecord(data, biffObj, objPayload(legacyObjTypeNote, 5))
	data = appendBiffRecord(data, biffTxO, nil)
	data = appendBiffRecord(data, biffContinue, append([]byte{0x00}, []byte("First comment")...))
	data = appendBiffRecord(data, biffObj, objPayload(legacyObjTypeNote, 7))
	data = appendBiffRecord(data, biffTxO, nil)
	data = appendBiffRecord(data, biffContinue, append([]byte{0x00}, []byte("Second comment")...))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "CommentsByObj"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.Comments["A1"] != "First comment" {
		t.Fatalf("A1 comment = %q, want First comment", sheet.Comments["A1"])
	}
	if sheet.Comments["B2"] != "Second comment" {
		t.Fatalf("B2 comment = %q, want Second comment", sheet.Comments["B2"])
	}
}

func TestParseWorksheetExtractsLegacyFormControls(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})

	data = appendBiffRecord(data, biffObj, objPayload(legacyObjTypeButton, 1))
	data = appendBiffRecord(data, biffTxO, txoPayloadWithLink(6, ptgRefFormula(0, 1)))
	data = appendBiffRecord(data, biffContinue, append([]byte{0x00}, append([]byte("Run me"), []byte{0x00, 0x01}...)...))

	data = appendBiffRecord(data, biffObj, objPayloadWithLinkFormulaAndLbsData(
		legacyObjTypeDrop,
		2,
		objLinkFormulaPayload(ptgRefFormula(0, 2)),
		lbsDataPayloadWithRangeAndSelection(ptgAreaFormula(0, 2, 3, 3), 3, 2, true),
	))

	data = appendBiffRecord(data, biffMsoDrawing, legacyControlDrawing(
		legacyControlShape(1, 1, 3, 3),
		legacyControlShape(5, 5, 7, 6),
	))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Controls"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.FormControls) != 2 {
		t.Fatalf("len(sheet.FormControls) = %d, want 2", len(sheet.FormControls))
	}
	if got := sheet.FormControls[0]; got.Type != "Button" || got.AltText != "Run me" || got.FmlaLink != "$B$1" || got.Anchor != "0, 0, 0, 0, 2, 0, 2, 0" {
		t.Fatalf("control[0] = %#v", got)
	}
	if got := sheet.FormControls[1]; got.Type != "Drop" || got.FmlaLink != "$C$1" || got.FmlaRange != "$D$1:$D$3" || got.Val != 2 || got.DropLines != 8 || got.Anchor != "4, 0, 4, 0, 5, 0, 6, 0" {
		t.Fatalf("control[1] = %#v", got)
	}
}

func TestParseWorksheetExtractsLegacyListMultiSelection(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffObj, objPayloadWithLbsData(
		legacyObjTypeList,
		1,
		lbsDataPayloadWithSelectionModeAndEntries(ptgAreaFormula(0, 3, 4, 4), 4, 2, "multi", []int{1, 3}, false),
	))
	data = appendBiffRecord(data, biffMsoDrawing, legacyControlDrawing(
		legacyControlShape(1, 1, 3, 4),
	))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "ListMulti"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.FormControls) != 1 {
		t.Fatalf("len(sheet.FormControls) = %d, want 1", len(sheet.FormControls))
	}
	got := sheet.FormControls[0]
	if got.Type != "List" || got.FmlaRange != "$E$1:$E$4" || got.SelectionType != "multi" || got.Val != 2 {
		t.Fatalf("list control = %#v", got)
	}
	if len(got.Selected) != 2 || got.Selected[0] != 1 || got.Selected[1] != 3 {
		t.Fatalf("list selected = %v, want [1 3]", got.Selected)
	}
}

func TestParseWorksheetExtractsLegacyRadioGroupStarts(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})

	firstRadio := objPayloadWithCheckboxData(legacyObjTypeRadio, 1, checkboxDataPayload(0x0001))
	firstRadio = appendObjSubRecord(firstRadio, 0x0014, objLinkFormulaPayload(ptgRefFormula(0, 1)))
	secondRadio := objPayloadWithCheckboxData(legacyObjTypeRadio, 2, checkboxDataPayload(0x0000))
	secondRadio = appendObjSubRecord(secondRadio, 0x0014, objLinkFormulaPayload(ptgRefFormula(0, 1)))
	thirdRadio := objPayloadWithCheckboxData(legacyObjTypeRadio, 3, checkboxDataPayload(0x0001))
	thirdRadio = appendObjSubRecord(thirdRadio, 0x0014, objLinkFormulaPayload(ptgRefFormula(0, 2)))
	data = appendBiffRecord(data, biffObj, firstRadio)
	data = appendBiffRecord(data, biffObj, secondRadio)
	data = appendBiffRecord(data, biffObj, objPayload(legacyObjTypeButton, 4))
	data = appendBiffRecord(data, biffObj, thirdRadio)
	data = appendBiffRecord(data, biffMsoDrawing, legacyControlDrawing(
		legacyControlShape(1, 1, 2, 2),
		legacyControlShape(3, 3, 4, 4),
		legacyControlShape(5, 5, 6, 6),
		legacyControlShape(7, 7, 8, 8),
	))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "RadioGroups"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.FormControls) != 4 {
		t.Fatalf("len(sheet.FormControls) = %d, want 4", len(sheet.FormControls))
	}
	if got := sheet.FormControls[0]; got.Type != "Radio" || !got.FirstButton || !got.Checked || got.FmlaLink != "$B$1" {
		t.Fatalf("radio[0] = %#v", got)
	}
	if got := sheet.FormControls[1]; got.Type != "Radio" || got.FirstButton || got.Checked || got.FmlaLink != "$B$1" {
		t.Fatalf("radio[1] = %#v", got)
	}
	if got := sheet.FormControls[3]; got.Type != "Radio" || !got.FirstButton || !got.Checked || got.FmlaLink != "$C$1" {
		t.Fatalf("radio[2] = %#v", got)
	}
}

func TestParseWorksheetExtractsLegacyGroupBoxLink(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	groupBox := objPayload(legacyObjTypeGroupBox, 1)
	groupBox = appendObjSubRecord(groupBox, 0x0014, objLinkFormulaPayload(ptgRefFormula(0, 7)))
	data = appendBiffRecord(data, biffObj, groupBox)
	data = appendBiffRecord(data, biffTxO, txoPayload(6))
	data = appendBiffRecord(data, biffContinue, append([]byte{0x00}, []byte("Region")...))
	data = appendBiffRecord(data, biffMsoDrawing, legacyControlDrawing(
		legacyControlShape(1, 1, 4, 6),
	))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "GroupBox"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.FormControls) != 1 {
		t.Fatalf("len(sheet.FormControls) = %d, want 1", len(sheet.FormControls))
	}
	if got := sheet.FormControls[0]; got.Type != "GBox" || got.AltText != "Region" || got.FmlaGroup != "$H$1" || got.FmlaLink != "" {
		t.Fatalf("group box = %#v", got)
	}
}

func TestParseWorksheetLegacyFormControlAnchorsRespectObjectOrder(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})

	data = appendBiffRecord(data, biffObj, objPayload(legacyObjTypeButton, 1))
	data = appendBiffRecord(data, biffTxO, txoPayload(3))
	data = appendBiffRecord(data, biffContinue, append([]byte{0x00}, []byte("One")...))

	data = appendBiffRecord(data, biffObj, objPayload(5, 2))

	data = appendBiffRecord(data, biffObj, objPayload(legacyObjTypeCheckbox, 3))
	data = appendBiffRecord(data, biffTxO, txoPayload(3))
	data = appendBiffRecord(data, biffContinue, append([]byte{0x00}, []byte("Two")...))

	data = appendBiffRecord(data, biffMsoDrawing, legacyControlDrawing(
		legacyControlShape(1, 1, 2, 2),
		legacyControlShape(3, 3, 4, 4),
		legacyControlShape(5, 5, 6, 6),
	))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Controls"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.FormControls) != 2 {
		t.Fatalf("len(sheet.FormControls) = %d, want 2", len(sheet.FormControls))
	}
	if got := sheet.FormControls[0].Anchor; got != "0, 0, 0, 0, 1, 0, 1, 0" {
		t.Fatalf("control[0].Anchor = %q, want first non-picture anchor", got)
	}
	if got := sheet.FormControls[1].Anchor; got != "4, 0, 4, 0, 5, 0, 5, 0" {
		t.Fatalf("control[1].Anchor = %q, want third non-picture anchor after chart object consumes second", got)
	}
}

func TestParseWorksheetExtractsLegacyFormControlState(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})

	data = appendBiffRecord(data, biffObj, objPayloadWithCheckboxData(legacyObjTypeCheckbox, 1, checkboxDataPayload(0x0001)))
	data = appendBiffRecord(data, biffObj, objPayloadWithSbsData(legacyObjTypeSpin, 2, sbsDataPayload(3, 1, 9)))
	data = appendBiffRecord(data, biffObj, objPayloadWithCheckboxData(legacyObjTypeCheckbox, 3, checkboxDataPayload(0x0002)))

	data = appendBiffRecord(data, biffMsoDrawing, legacyControlDrawing(
		legacyControlShape(1, 1, 2, 2),
		legacyControlShape(3, 3, 5, 4),
		legacyControlShape(6, 6, 7, 7),
	))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "ControlState"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.FormControls) != 3 {
		t.Fatalf("len(sheet.FormControls) = %d, want 3", len(sheet.FormControls))
	}
	if got := sheet.FormControls[0]; got.Type != "Checkbox" || !got.Checked || got.Anchor != "0, 0, 0, 0, 1, 0, 1, 0" {
		t.Fatalf("checkbox control = %#v", got)
	}
	if got := sheet.FormControls[1]; got.Type != "Spin" || got.Min != 1 || got.Max != 9 || got.Val != 3 || got.Anchor != "2, 0, 2, 0, 3, 0, 4, 0" {
		t.Fatalf("spin control = %#v", got)
	}
	if got := sheet.FormControls[2]; got.Type != "Checkbox" || got.Checked || !got.Mixed || got.Anchor != "5, 0, 5, 0, 6, 0, 6, 0" {
		t.Fatalf("mixed checkbox control = %#v", got)
	}
}

func TestParseLegacyPivotCacheSources(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffSXStreamID, []byte{0x11, 0x00})
	data = appendBiffRecord(data, biffSXVS, []byte{0x01, 0x00})
	data = appendBiffRecord(data, biffDConRef, dconRefPayload(0, 3, 0, 2, "Data"))
	data = appendBiffRecord(data, biffSXStreamID, []byte{0x12, 0x00})
	data = appendBiffRecord(data, biffSXVS, []byte{0x01, 0x00})
	data = appendBiffRecord(data, biffDConName, dconNamePayload("PivotData", ""))
	data = appendBiffRecord(data, biffEOF, nil)

	caches := parseLegacyPivotCacheSources(data)
	if len(caches) != 2 {
		t.Fatalf("len(caches) = %d, want 2", len(caches))
	}
	if got := caches[0]; got.StreamID != 0x0011 || got.SourceSheet != "Data" || got.SourceRef != "A1:C4" || got.SourceName != "" {
		t.Fatalf("caches[0] = %#v", got)
	}
	if got := caches[1]; got.StreamID != 0x0012 || got.SourceName != "PivotData" || got.SourceSheet != "" || got.SourceRef != "" {
		t.Fatalf("caches[1] = %#v", got)
	}
}

func TestParseWorksheetExtractsLegacyPivotTableView(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffSXView, pivotViewPayload(2, 7, 0, 3, 0, 2, 1, 1, 1, true, true, "OrdersPivot", "Values"))
	data = appendBiffRecord(data, biffSXVD, pivotViewFieldPayload(0x0001, "Region"))
	data = appendBiffRecord(data, biffSXVD, pivotViewFieldPayload(0x0002, "Quarter"))
	data = appendBiffRecord(data, biffSXVD, pivotViewFieldPayload(0x0004, "Channel"))
	data = appendBiffRecord(data, biffSXVD, pivotViewFieldPayload(0x0008, "Sales"))
	data = appendBiffRecord(data, biffSXIvd, pivotAxisOrderPayload(0))
	data = appendBiffRecord(data, biffSXIvd, pivotAxisOrderPayload(1))
	data = appendBiffRecord(data, biffSXPI, pivotPageFieldsPayload(2))
	data = appendBiffRecord(data, biffSXDI, pivotDataFieldPayload(3, 0, 4, "Sum of Sales"))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheetWithDrawingStore(data, boundSheet{offset: 0, name: "Pivot"}, nil, nil, nil, legacyFormulaWorkbookContext{
		pivotCaches: []legacyPivotCacheSource{{
			SourceSheet: "Data",
			SourceRef:   "A1:D20",
		}},
	})
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.PivotTables) != 1 {
		t.Fatalf("len(sheet.PivotTables) = %d, want 1", len(sheet.PivotTables))
	}
	pivot := sheet.PivotTables[0]
	if pivot.Name != "OrdersPivot" || pivot.LocationRef != "A3:D8" || pivot.DataCaption != "Values" {
		t.Fatalf("pivot identity = %#v", pivot)
	}
	if pivot.SourceSheet != "Data" || pivot.SourceRef != "A1:D20" {
		t.Fatalf("pivot source = %#v", pivot)
	}
	if !pivot.RowGrandTotals || !pivot.ColGrandTotals || !pivot.ShowHeaders {
		t.Fatalf("pivot layout flags = %#v", pivot)
	}
	if len(pivot.RowFields) != 1 || pivot.RowFields[0] != "Region" {
		t.Fatalf("pivot row fields = %#v", pivot.RowFields)
	}
	if len(pivot.ColFields) != 1 || pivot.ColFields[0] != "Quarter" {
		t.Fatalf("pivot col fields = %#v", pivot.ColFields)
	}
	if len(pivot.PageFields) != 1 || pivot.PageFields[0] != "Channel" {
		t.Fatalf("pivot page fields = %#v", pivot.PageFields)
	}
	if len(pivot.DataFields) != 1 {
		t.Fatalf("len(pivot.DataFields) = %d, want 1", len(pivot.DataFields))
	}
	if got := pivot.DataFields[0]; got.Name != "Sum of Sales" || got.SourceFieldName != "Sales" || got.Subtotal != "sum" || got.NumFmtID != 4 {
		t.Fatalf("pivot data field = %#v", got)
	}
}

func TestParseWorksheetPreservesLegacyPivotItemsAndPageSelection(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffSXView, pivotViewPayload(2, 7, 0, 3, 0, 1, 0, 1, 0, true, true, "FilterPivot", "Values"))
	data = appendBiffRecord(data, biffSXVD, pivotViewFieldPayload(0x0004, "Region"))
	data = appendBiffRecord(data, biffSXVI, pivotItemPayload(0, false, false, "East"))
	data = appendBiffRecord(data, biffSXVI, pivotItemPayload(1, true, true, "West"))
	data = appendBiffRecord(data, biffSXPI, pivotPageFieldSelectionPayload(0, 1))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheetWithDrawingStore(data, boundSheet{offset: 0, name: "Pivot"}, nil, nil, nil, legacyFormulaWorkbookContext{})
	if sheet == nil || len(sheet.PivotTables) != 1 {
		t.Fatalf("pivot tables = %#v", sheet)
	}
	pivot := sheet.PivotTables[0]
	if len(pivot.Fields) != 1 || len(pivot.Fields[0].Items) != 2 {
		t.Fatalf("pivot fields = %#v", pivot.Fields)
	}
	if got := pivot.Fields[0].Items[1]; got.Name != "West" || !got.Hidden || !got.HideDetail || got.CacheIndex != 1 {
		t.Fatalf("second item = %#v", got)
	}
	if len(pivot.PageFieldSelections) != 1 || pivot.PageFieldSelections[0].FieldName != "Region" || pivot.PageFieldSelections[0].ItemIndex != 1 {
		t.Fatalf("page selection = %#v", pivot.PageFieldSelections)
	}
}

func TestParseWorksheetFallsBackToNoteAuthorWhenCommentTextMissing(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffNote, notePayload(2, 2, 0, "Reviewer"))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "CommentsAuthor"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.Comments["C3"] != "Reviewer" {
		t.Fatalf("C3 comment = %q, want Reviewer", sheet.Comments["C3"])
	}
}

func TestApplyFrozenPaneSetsTopLeftCellAndActivePane(t *testing.T) {
	window2Payload := make([]byte, 18)
	binary.LittleEndian.PutUint16(window2Payload[0:2], 0x0008) // freeze bit

	// Pane: xSplit=1, ySplit=2, topRowInBottomPane=2 (0-based→row 3), leftColInRightPane=1 (0-based→col B), activePane=0 (bottomRight).
	panePayload := make([]byte, 10)
	binary.LittleEndian.PutUint16(panePayload[0:2], 1)
	binary.LittleEndian.PutUint16(panePayload[2:4], 2)
	binary.LittleEndian.PutUint16(panePayload[4:6], 2)  // topRow (0-based)
	binary.LittleEndian.PutUint16(panePayload[6:8], 1)  // leftCol (0-based)
	binary.LittleEndian.PutUint16(panePayload[8:10], 0) // bottomRight

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffWindow2, window2Payload)
	data = appendBiffRecord(data, biffPane, panePayload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "FreezeTopLeft"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.TopLeftCell != "B3" {
		t.Fatalf("TopLeftCell = %q, want B3", sheet.TopLeftCell)
	}
	if sheet.ActivePane != "bottomRight" {
		t.Fatalf("ActivePane = %q, want bottomRight", sheet.ActivePane)
	}
}

func TestParseWorksheetCapturesSplitPaneMetadata(t *testing.T) {
	window2Payload := make([]byte, 18)
	binary.LittleEndian.PutUint16(window2Payload[0:2], 0x06B6) // freeze bit NOT set

	panePayload := make([]byte, 10)
	binary.LittleEndian.PutUint16(panePayload[0:2], 1200) // split position in twips; not used as count
	binary.LittleEndian.PutUint16(panePayload[2:4], 2400) // split position in twips; not used as count
	binary.LittleEndian.PutUint16(panePayload[4:6], 5)    // first visible row in bottom pane = row 6
	binary.LittleEndian.PutUint16(panePayload[6:8], 3)    // first visible col in right pane = col D
	binary.LittleEndian.PutUint16(panePayload[8:10], 3)   // topLeft

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffWindow2, window2Payload)
	data = appendBiffRecord(data, biffPane, panePayload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Split"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.FrozenCols != 0 || sheet.FrozenRows != 0 {
		t.Fatalf("FrozenCols=%d FrozenRows=%d, want 0 and 0 for split pane", sheet.FrozenCols, sheet.FrozenRows)
	}
	if sheet.SplitCols != 3 {
		t.Fatalf("SplitCols = %d, want 3", sheet.SplitCols)
	}
	if sheet.SplitRows != 5 {
		t.Fatalf("SplitRows = %d, want 5", sheet.SplitRows)
	}
	if sheet.TopLeftCell != "D6" {
		t.Fatalf("TopLeftCell = %q, want D6", sheet.TopLeftCell)
	}
	if sheet.ActivePane != "topLeft" {
		t.Fatalf("ActivePane = %q, want topLeft", sheet.ActivePane)
	}
}

func TestParseWorksheetReadsFrozenPanesFromWindow2AndPane(t *testing.T) {
	// Window2: options word with bit 3 set (0x0008 = freeze panes).
	window2Payload := make([]byte, 18)
	binary.LittleEndian.PutUint16(window2Payload[0:2], 0x06B6|0x0008) // options with freeze bit

	// Pane: xSplit=2 (freeze 2 cols), ySplit=3 (freeze 3 rows).
	panePayload := make([]byte, 10)
	binary.LittleEndian.PutUint16(panePayload[0:2], 2)
	binary.LittleEndian.PutUint16(panePayload[2:4], 3)

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffWindow2, window2Payload)
	data = appendBiffRecord(data, biffPane, panePayload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Freeze"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.FrozenCols != 2 {
		t.Fatalf("FrozenCols = %d, want 2", sheet.FrozenCols)
	}
	if sheet.FrozenRows != 3 {
		t.Fatalf("FrozenRows = %d, want 3", sheet.FrozenRows)
	}
}

func TestParseWorksheetIgnoresPaneWithoutWindow2FreezeBit(t *testing.T) {
	// Window2 without freeze bit — Pane should not set FrozenCols/FrozenRows.
	window2Payload := make([]byte, 18)
	binary.LittleEndian.PutUint16(window2Payload[0:2], 0x06B6) // freeze bit NOT set

	panePayload := make([]byte, 4)
	binary.LittleEndian.PutUint16(panePayload[0:2], 2)
	binary.LittleEndian.PutUint16(panePayload[2:4], 3)

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffWindow2, window2Payload)
	data = appendBiffRecord(data, biffPane, panePayload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Split"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.FrozenCols != 0 || sheet.FrozenRows != 0 {
		t.Fatalf("FrozenCols=%d FrozenRows=%d, want 0 and 0 for split pane", sheet.FrozenCols, sheet.FrozenRows)
	}
}

func TestParseWorksheetAssemblesFormulaCellStringAcrossContinueRecords(t *testing.T) {
	formulaPayload := []byte{
		0, 0, // row 0
		0, 0, // col 0
		0, 0, // XF index 0
		0x00, 0, 0, 0, 0, 0, 0xFF, 0xFF, // result: string-follows marker (POI STRING type)
		0, 0, 0, 0, 0, 0, // remaining formula fields
	}
	// biffString: first chunk – charCount=11, flags=0x00, partial data "Hello"
	firstChunk := []byte{0x0B, 0x00, 0x00, 'H', 'e', 'l', 'l', 'o'}
	// biffContinue: rest of string data " World"
	restChunk := []byte{' ', 'W', 'o', 'r', 'l', 'd'}

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffFormula, formulaPayload)
	data = appendBiffRecord(data, biffString, firstChunk)
	data = appendBiffRecord(data, biffContinue, restChunk)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "FormulaStr"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Rows) != 1 || len(sheet.Rows[0].Cells) != 1 {
		t.Fatalf("expected 1 row with 1 cell, got %v", sheet.Rows)
	}
	cell := sheet.Rows[0].Cells[0]
	if cell.Ref != "A1" {
		t.Fatalf("cell ref = %q, want A1", cell.Ref)
	}
	if cell.StrValue != "Hello World" {
		t.Fatalf("cell StrValue = %q, want \"Hello World\"", cell.StrValue)
	}
}

func TestParseWorksheetDecompilesBIFFFormulaCell(t *testing.T) {
	formulaBytes := []byte{
		0x24,
		0xFF, 0xFF,
		0xFF, 0xC0,
		0x1E, 0x01, 0x00,
		0x03,
	}
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffFormula, numberFormulaPayload(1, 1, 0, 2, formulaBytes))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Formula"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Rows) != 1 || len(sheet.Rows[0].Cells) != 1 {
		t.Fatalf("expected 1 row with 1 cell, got %v", sheet.Rows)
	}
	cell := sheet.Rows[0].Cells[0]
	if cell.Ref != "B2" {
		t.Fatalf("cell ref = %q, want B2", cell.Ref)
	}
	if cell.Formula != "A1+1" {
		t.Fatalf("cell.Formula = %q, want A1+1", cell.Formula)
	}
	if cell.NumValue != 2 {
		t.Fatalf("cell.NumValue = %v, want 2", cell.NumValue)
	}
}

func TestParseWorksheetRequiresImmediateStringRecordForFormulaStringResults(t *testing.T) {
	formulaBytes := []byte{0x1E, 0x2A, 0x00}
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffFormula, emptyFormulaPayload(0, 0, 0, formulaBytes))
	data = appendBiffRecord(data, biffNumber, numberCellPayload(0, 1, 0, 7))
	data = appendBiffRecord(data, biffString, []byte{0x05, 0x00, 0x00, 'W', 'r', 'o', 'n', 'g'})
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "FormulaStringOrder"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Rows) != 1 || len(sheet.Rows[0].Cells) != 2 {
		t.Fatalf("expected 1 row with 2 cells, got %+v", sheet.Rows)
	}
	formulaCell := sheet.Rows[0].Cells[0]
	if formulaCell.Ref != "A1" {
		t.Fatalf("formula cell ref = %q, want A1", formulaCell.Ref)
	}
	if formulaCell.Formula != "42" {
		t.Fatalf("formula cell formula = %q, want 42", formulaCell.Formula)
	}
	if formulaCell.StrValue != "" {
		t.Fatalf("formula cell StrValue = %q, want empty string", formulaCell.StrValue)
	}
	numberCell := sheet.Rows[0].Cells[1]
	if numberCell.Ref != "B1" || numberCell.NumValue != 7 {
		t.Fatalf("number cell = %+v, want B1=7", numberCell)
	}
}

func TestParseWorksheetDecompilesOldBIFF3FormulaCell(t *testing.T) {
	formulaBytes := []byte{
		0x24,
		0xFF, 0xFF,
		0xFF, 0xC0,
		0x1E, 0x01, 0x00,
		0x03,
	}
	data := appendBiffRecord(nil, biffBOF3, []byte{0x02, 0x00, 0x10, 0x00})
	data = appendBiffRecord(data, biffFormula3, oldNumberFormulaPayload(1, 1, 0, 2, formulaBytes, false))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "OldFormula3"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Rows) != 1 || len(sheet.Rows[0].Cells) != 1 {
		t.Fatalf("expected 1 row with 1 cell, got %v", sheet.Rows)
	}
	cell := sheet.Rows[0].Cells[0]
	if cell.Ref != "B2" {
		t.Fatalf("cell ref = %q, want B2", cell.Ref)
	}
	if cell.Formula != "A1+1" {
		t.Fatalf("cell.Formula = %q, want A1+1", cell.Formula)
	}
	if cell.NumValue != 2 {
		t.Fatalf("cell.NumValue = %v, want 2", cell.NumValue)
	}
}

func TestParseWorksheetCapturesOldBIFF4FormulaStringResult(t *testing.T) {
	formulaBytes := []byte{0x24, 0x00, 0x00, 0x00, 0x00}
	data := appendBiffRecord(nil, biffBOF4, []byte{0x04, 0x00, 0x10, 0x00})
	data = appendBiffRecord(data, biffFormula4, oldStringFormulaPayload(0, 0, 0, formulaBytes, false))
	data = appendBiffRecord(data, biffString, []byte{0x06, 0x00, 'L', 'e', 'g', 'a', 'c', 'y'})
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "OldFormula4String"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Rows) != 1 || len(sheet.Rows[0].Cells) != 1 {
		t.Fatalf("expected 1 row with 1 cell, got %v", sheet.Rows)
	}
	cell := sheet.Rows[0].Cells[0]
	if cell.Ref != "A1" {
		t.Fatalf("cell ref = %q, want A1", cell.Ref)
	}
	if cell.StrValue != "Legacy" {
		t.Fatalf("cell.StrValue = %q, want Legacy", cell.StrValue)
	}
}

func TestParseWorksheetTreatsBIFF2FormulaSIDAsOldFormula(t *testing.T) {
	formulaBytes := []byte{0x1E, 0x2A, 0x00}
	data := appendBiffRecord(nil, biffBOFOld, []byte{0x01, 0x00, 0x10, 0x00})
	data = appendBiffRecord(data, biffFormula, oldNumberFormulaPayload(0, 0, 0, 42, formulaBytes, true))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "OldFormula2"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Rows) != 1 || len(sheet.Rows[0].Cells) != 1 {
		t.Fatalf("expected 1 row with 1 cell, got %v", sheet.Rows)
	}
	cell := sheet.Rows[0].Cells[0]
	if cell.Ref != "A1" {
		t.Fatalf("cell ref = %q, want A1", cell.Ref)
	}
	if cell.Formula != "42" {
		t.Fatalf("cell.Formula = %q, want 42", cell.Formula)
	}
	if cell.NumValue != 42 {
		t.Fatalf("cell.NumValue = %v, want 42", cell.NumValue)
	}
}

func TestDecodeBIFFFormulaExpandedFunctions(t *testing.T) {
	// Verify that expanded function table entries decode via ptgFuncVar (0x22).
	// Each sub-test encodes: ptgRef(A1), ptgFuncVar(argc=1, iftab=N).
	cases := []struct {
		iftab uint16
		want  string
	}{
		{65, "DATE($A$1)"},
		{74, "NOW($A$1)"},
		{76, "ROWS($A$1)"},
		{77, "COLUMNS($A$1)"},
		{82, "SEARCH($A$1)"},
		{86, "TYPE($A$1)"},
		{87, "ISTEXT($A$1)"},
		{88, "ISNUMBER($A$1)"},
		{89, "ISBLANK($A$1)"},
		{97, "ATAN2($A$1)"},
		{101, "HLOOKUP($A$1)"},
		{102, "VLOOKUP($A$1)"},
		{109, "LOG($A$1)"},
		{110, "EXEC($A$1)"},
		{115, "LEFT($A$1)"},
		{168, "PPMT($A$1)"},
		{169, "COUNTA($A$1)"},
		{228, "SUMPRODUCT($A$1)"},
		{346, "COUNTIF($A$1)"},
		{347, "COUNTBLANK($A$1)"},
	}
	for _, tc := range cases {
		// ptgRef $A$1 (absolute): 0x24, row=0, col=0x8000|0xC000 means absolute row+col
		// Simpler: use relative ref A1 (row=0, col=0 with row/col relative bits set)
		ptgRef := []byte{0x24, 0x00, 0x00, 0x00, 0x00} // relative A1
		ptgFuncVar := []byte{0x22, 0x01, 0, 0}         // argc=1
		binary.LittleEndian.PutUint16(ptgFuncVar[2:4], tc.iftab)
		formula := append(ptgRef, ptgFuncVar...)
		ctx := legacyFormulaDecodeContext{row: 1, col: 1}
		got, ok := decodeBIFFFormula(formula, ctx)
		if !ok {
			t.Errorf("iftab=%d: decode failed, want %q", tc.iftab, tc.want)
			continue
		}
		if got != tc.want {
			t.Errorf("iftab=%d: got %q, want %q", tc.iftab, got, tc.want)
		}
	}
}

func TestDecodeBIFFFormulaPtgArrayPlaceholder(t *testing.T) {
	// ptgArray (0x20) body is 8 bytes (1 token + 7 reserved).
	// Decompiler should push a placeholder and continue.
	ptgArray := []byte{0x20, 0, 0, 0, 0, 0, 0, 0} // 8 bytes
	// Then SUM({...}) via ptgFuncVar argc=1 iftab=4
	ptgFuncVar := []byte{0x22, 0x01, 0x04, 0x00}
	formula := append(ptgArray, ptgFuncVar...)
	ctx := legacyFormulaDecodeContext{row: 1, col: 1}
	got, ok := decodeBIFFFormula(formula, ctx)
	if !ok {
		t.Fatalf("ptgArray decode failed, got empty formula")
	}
	if got != "SUM({...})" {
		t.Fatalf("ptgArray: got %q, want SUM({...})", got)
	}
}

func TestParseWorksheetCapturesPageSetupFromSetupRecord(t *testing.T) {
	payload := make([]byte, 34)
	binary.LittleEndian.PutUint16(payload[0:2], 9)        // A4 paper
	binary.LittleEndian.PutUint16(payload[2:4], 75)       // 75% scale
	binary.LittleEndian.PutUint16(payload[6:8], 2)        // fitWidth (not active since scale>0)
	binary.LittleEndian.PutUint16(payload[8:10], 3)       // fitHeight (not active since scale>0)
	binary.LittleEndian.PutUint16(payload[10:12], 0x0002) // landscape bit
	binary.LittleEndian.PutUint64(payload[16:24], math.Float64bits(0.3))
	binary.LittleEndian.PutUint64(payload[24:32], math.Float64bits(0.4))

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffSetup, payload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Setup"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.PageSetup == nil {
		t.Fatal("expected PageSetup to be set")
	}
	if sheet.PageSetup.PaperSize != 9 {
		t.Fatalf("PaperSize = %d, want 9", sheet.PageSetup.PaperSize)
	}
	if sheet.PageSetup.Scale != 75 {
		t.Fatalf("Scale = %d, want 75", sheet.PageSetup.Scale)
	}
	if sheet.PageSetup.FitToPage {
		t.Fatalf("FitToPage = true, want false when scale > 0")
	}
	if sheet.PageSetup.Orientation != "landscape" {
		t.Fatalf("Orientation = %q, want landscape", sheet.PageSetup.Orientation)
	}
	if sheet.PageSetup.MarginHeader != 0.3 {
		t.Fatalf("MarginHeader = %v, want 0.3", sheet.PageSetup.MarginHeader)
	}
	if sheet.PageSetup.MarginFooter != 0.4 {
		t.Fatalf("MarginFooter = %v, want 0.4", sheet.PageSetup.MarginFooter)
	}
}

func TestParseWorksheetCapturesFitToPageFromSetupRecord(t *testing.T) {
	payload := make([]byte, 12)
	binary.LittleEndian.PutUint16(payload[0:2], 1)   // letter paper
	binary.LittleEndian.PutUint16(payload[2:4], 0)   // scale=0 → fit to page
	binary.LittleEndian.PutUint16(payload[6:8], 1)   // fitWidth = 1 page wide
	binary.LittleEndian.PutUint16(payload[8:10], 2)  // fitHeight = 2 pages tall
	binary.LittleEndian.PutUint16(payload[10:12], 0) // portrait (landscape bit clear)

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffSetup, payload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "FitToPage"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.PageSetup == nil {
		t.Fatal("expected PageSetup to be set")
	}
	if !sheet.PageSetup.FitToPage {
		t.Fatalf("FitToPage = false, want true when scale == 0")
	}
	if sheet.PageSetup.FitToWidth != 1 {
		t.Fatalf("FitToWidth = %d, want 1", sheet.PageSetup.FitToWidth)
	}
	if sheet.PageSetup.FitToHeight != 2 {
		t.Fatalf("FitToHeight = %d, want 2", sheet.PageSetup.FitToHeight)
	}
	if sheet.PageSetup.Orientation != "portrait" {
		t.Fatalf("Orientation = %q, want portrait", sheet.PageSetup.Orientation)
	}
}

func TestParseWorksheetCapturesMarginRecords(t *testing.T) {
	makeMargin := func(inches float64) []byte {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, math.Float64bits(inches))
		return b
	}

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffLeftMargin, makeMargin(0.75))
	data = appendBiffRecord(data, biffRightMargin, makeMargin(0.75))
	data = appendBiffRecord(data, biffTopMargin, makeMargin(1.0))
	data = appendBiffRecord(data, biffBottomMargin, makeMargin(1.0))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Margins"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.PageSetup == nil {
		t.Fatal("expected PageSetup to be set by margin records")
	}
	if sheet.PageSetup.MarginLeft != 0.75 {
		t.Fatalf("MarginLeft = %v, want 0.75", sheet.PageSetup.MarginLeft)
	}
	if sheet.PageSetup.MarginRight != 0.75 {
		t.Fatalf("MarginRight = %v, want 0.75", sheet.PageSetup.MarginRight)
	}
	if sheet.PageSetup.MarginTop != 1.0 {
		t.Fatalf("MarginTop = %v, want 1.0", sheet.PageSetup.MarginTop)
	}
	if sheet.PageSetup.MarginBottom != 1.0 {
		t.Fatalf("MarginBottom = %v, want 1.0", sheet.PageSetup.MarginBottom)
	}
}

func TestParseWorksheetCapturesPrintFlagRecords(t *testing.T) {
	flagPayload := func(set bool) []byte {
		b := make([]byte, 2)
		if set {
			binary.LittleEndian.PutUint16(b, 1)
		}
		return b
	}

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x10, 0x00})
	data = appendBiffRecord(data, biffHCenter, flagPayload(true))
	data = appendBiffRecord(data, biffVCenter, flagPayload(false))
	data = appendBiffRecord(data, biffPrintHeaders, flagPayload(true))
	data = appendBiffRecord(data, biffPrintGridlines, flagPayload(true))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Print"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.PageSetup == nil {
		t.Fatal("expected PageSetup to be set by print-flag records")
	}
	if !sheet.PageSetup.CenterHorizontally {
		t.Fatalf("CenterHorizontally = false, want true (HCENTER flag = 1)")
	}
	if sheet.PageSetup.CenterVertically {
		t.Fatalf("CenterVertically = true, want false (VCENTER flag = 0)")
	}
	if !sheet.PageSetup.PrintHeadings {
		t.Fatalf("PrintHeadings = false, want true (PRINTHEADERS flag = 1)")
	}
	if !sheet.PageSetup.PrintGridlines {
		t.Fatalf("PrintGridlines = false, want true (PRINTGRIDLINES flag = 1)")
	}
}

func TestParseWorksheetCapturesSCLZoomScale(t *testing.T) {
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint16(payload[0:2], 3) // numerator
	binary.LittleEndian.PutUint16(payload[2:4], 4) // denominator => 75%

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffSCL, payload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Zoom"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.ZoomScale != 75 {
		t.Fatalf("ZoomScale = %d, want 75", sheet.ZoomScale)
	}
}

func TestApplySCLRecordRejectsInvalidPayloadAndZeroDenominator(t *testing.T) {
	sheet := &sheetir.Sheet{}
	applySCLRecord(sheet, []byte{0x01, 0x00, 0x00}) // short payload
	if sheet.ZoomScale != 0 {
		t.Fatalf("ZoomScale = %d, want 0 for short payload", sheet.ZoomScale)
	}
	applySCLRecord(sheet, []byte{0x10, 0x00, 0x00, 0x00}) // denominator = 0
	if sheet.ZoomScale != 0 {
		t.Fatalf("ZoomScale = %d, want 0 for denominator=0", sheet.ZoomScale)
	}
}

func TestApplyPrintFlagRecordRejectsShortPayload(t *testing.T) {
	sheet := &sheetir.Sheet{}
	applyPrintFlagRecord(sheet, "hcenter", []byte{0x01}) // only 1 byte
	if sheet.PageSetup != nil {
		t.Fatalf("PageSetup = %+v, want nil for short payload", sheet.PageSetup)
	}
}

func TestApplyPrintFlagRecordIgnoresUnknownKind(t *testing.T) {
	sheet := &sheetir.Sheet{}
	applyPrintFlagRecord(sheet, "bogus", []byte{0x01, 0x00})
	// Page setup is created but no flag is touched; this is the "unknown kind"
	// safety net so future record additions can land without silently flipping
	// existing fields.
	if sheet.PageSetup == nil {
		t.Fatal("PageSetup = nil, want non-nil (helper still allocates the struct)")
	}
	if sheet.PageSetup.CenterHorizontally || sheet.PageSetup.CenterVertically ||
		sheet.PageSetup.PrintHeadings || sheet.PageSetup.PrintGridlines {
		t.Fatalf("PageSetup = %+v, want all flags false for unknown kind", sheet.PageSetup)
	}
}

func TestParsePageBreakRecord(t *testing.T) {
	build := func(breaks ...uint16) []byte {
		out := make([]byte, 2, 2+len(breaks)*6)
		binary.LittleEndian.PutUint16(out[0:2], uint16(len(breaks)))
		for _, main := range breaks {
			b := make([]byte, 6)
			binary.LittleEndian.PutUint16(b[0:2], main)
			// subFrom and subTo are deliberately left at 0; the parser ignores them.
			out = append(out, b...)
		}
		return out
	}

	cases := []struct {
		name    string
		payload []byte
		want    []int
	}{
		{"empty payload", nil, nil},
		{"count=0", []byte{0x00, 0x00}, nil},
		{"single break before row 5", build(5), []int{5}},
		{"three breaks preserve order", build(2, 9, 21), []int{2, 9, 21}},
		{"main=0 entries are skipped as invalid", build(0, 7, 0), []int{7}},
		{"truncated trailing entry is ignored", append(build(3), 0x09), []int{3}},
		{"count larger than payload allows", append([]byte{0x05, 0x00}, build(1, 2, 3)[2:]...), []int{1, 2, 3}},
	}
	for _, c := range cases {
		got := parsePageBreakRecord(c.payload)
		if len(got) != len(c.want) {
			t.Errorf("%s: parsePageBreakRecord len = %d, want %d (got %v)", c.name, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: parsePageBreakRecord[%d] = %d, want %d", c.name, i, got[i], c.want[i])
			}
		}
	}
}

func TestParseWorksheetCapturesPageBreakRecords(t *testing.T) {
	build := func(breaks ...uint16) []byte {
		out := make([]byte, 2, 2+len(breaks)*6)
		binary.LittleEndian.PutUint16(out[0:2], uint16(len(breaks)))
		for _, main := range breaks {
			b := make([]byte, 6)
			binary.LittleEndian.PutUint16(b[0:2], main)
			out = append(out, b...)
		}
		return out
	}

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x10, 0x00})
	data = appendBiffRecord(data, biffHorizPageBreak, build(20, 40))
	data = appendBiffRecord(data, biffVertPageBreak, build(8))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Breaks"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if want := []int{20, 40}; !equalIntSlice(sheet.RowPageBreaks, want) {
		t.Fatalf("RowPageBreaks = %v, want %v", sheet.RowPageBreaks, want)
	}
	if want := []int{8}; !equalIntSlice(sheet.ColPageBreaks, want) {
		t.Fatalf("ColPageBreaks = %v, want %v", sheet.ColPageBreaks, want)
	}
}

func TestParseWorksheetAggregatesContiguousMergeCellsRecords(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffMergeCells, mergeCellsPayload(
		[4]int{0, 0, 0, 1}, // A1:B1
	))
	data = appendBiffRecord(data, biffMergeCells, mergeCellsPayload(
		[4]int{2, 3, 2, 2}, // C3:C4
		[4]int{5, 5, 4, 6}, // E6:G6
	))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Merged"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.MergedCells) != 3 {
		t.Fatalf("MergedCells = %d, want 3", len(sheet.MergedCells))
	}
	got := []string{
		fmt.Sprintf("%d:%d:%d:%d", sheet.MergedCells[0].TopRow, sheet.MergedCells[0].BottomRow, sheet.MergedCells[0].LeftCol, sheet.MergedCells[0].RightCol),
		fmt.Sprintf("%d:%d:%d:%d", sheet.MergedCells[1].TopRow, sheet.MergedCells[1].BottomRow, sheet.MergedCells[1].LeftCol, sheet.MergedCells[1].RightCol),
		fmt.Sprintf("%d:%d:%d:%d", sheet.MergedCells[2].TopRow, sheet.MergedCells[2].BottomRow, sheet.MergedCells[2].LeftCol, sheet.MergedCells[2].RightCol),
	}
	want := []string{"1:1:1:2", "3:4:3:3", "6:6:5:7"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MergedCells[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestGatherContiguousMergeCellsStopsAtFirstNonMergeRecord(t *testing.T) {
	data := appendBiffRecord(nil, biffMergeCells, mergeCellsPayload([4]int{0, 0, 0, 0}))
	data = appendBiffRecord(data, biffMergeCells, mergeCellsPayload([4]int{1, 1, 1, 1}))
	data = appendBiffRecord(data, biffRow, rowPayload(0, 0))
	data = appendBiffRecord(data, biffMergeCells, mergeCellsPayload([4]int{2, 2, 2, 2}))

	merged, nextPos := gatherContiguousMergeCells(data, 0)
	if len(merged) != 2 {
		t.Fatalf("len(merged) = %d, want 2", len(merged))
	}
	id, _, _, ok := readBiffRecord(data, nextPos)
	if !ok {
		t.Fatal("expected record at returned offset")
	}
	if id != biffRow {
		t.Fatalf("record at returned offset = 0x%04X, want ROW", id)
	}
}

func TestSkipCustomViewSettingsRecordAggregateStopsAtEndRecord(t *testing.T) {
	data := appendBiffRecord(nil, biffUserSViewBegin, nil)
	data = appendBiffRecord(data, biffHeader, []byte{0x01, 0x00, 0x00, 'X'})
	data = appendBiffRecord(data, biffUserSViewEnd, nil)
	data = appendBiffRecord(data, biffRow, rowPayload(0, 0))

	nextPos := skipCustomViewSettingsRecordAggregate(data, 0)
	id, _, _, ok := readBiffRecord(data, nextPos)
	if !ok {
		t.Fatal("expected record at returned offset")
	}
	if id != biffRow {
		t.Fatalf("record at returned offset = 0x%04X, want ROW", id)
	}
}

func TestParseWorksheetSkipsCustomViewSettingsAggregateRecords(t *testing.T) {
	buildHeaderFooter := func(text string) []byte {
		out := make([]byte, 3, 3+len(text))
		binary.LittleEndian.PutUint16(out[0:2], uint16(len(text)))
		out[2] = 0x00
		return append(out, []byte(text)...)
	}

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x10, 0x00})
	data = appendBiffRecord(data, biffUserSViewBegin, nil)
	data = appendBiffRecord(data, biffHeader, buildHeaderFooter("View Header"))
	data = appendBiffRecord(data, biffFooter, buildHeaderFooter("View Footer"))
	data = appendBiffRecord(data, biffUserSViewEnd, nil)
	data = appendBiffRecord(data, biffHeader, buildHeaderFooter("Live Header"))
	data = appendBiffRecord(data, biffFooter, buildHeaderFooter("Live Footer"))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "CustomView"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.HeaderFooter == nil {
		t.Fatal("expected HeaderFooter from live sheet records")
	}
	if sheet.HeaderFooter.OddHeader != "Live Header" {
		t.Fatalf("OddHeader = %q, want Live Header", sheet.HeaderFooter.OddHeader)
	}
	if sheet.HeaderFooter.OddFooter != "Live Footer" {
		t.Fatalf("OddFooter = %q, want Live Footer", sheet.HeaderFooter.OddFooter)
	}
}

func TestSkipChartSubstreamRecordAggregateStopsAtEOF(t *testing.T) {
	chartBOF := []byte{
		0x00, 0x06, // version = BIFF8
		0x20, 0x00, // type = chart
	}
	data := appendBiffRecord(nil, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffEOF, nil)
	data = appendBiffRecord(data, biffRow, rowPayload(0, 0))

	nextPos := skipChartSubstreamRecordAggregate(data, 0)
	id, _, _, ok := readBiffRecord(data, nextPos)
	if !ok {
		t.Fatal("expected record at returned offset")
	}
	if id != biffRow {
		t.Fatalf("record at returned offset = 0x%04X, want ROW", id)
	}
}

func TestParseWorksheetSkipsChartSubstreamAggregateRecords(t *testing.T) {
	sheetBOF := []byte{
		0x00, 0x06, // version = BIFF8
		0x10, 0x00, // type = worksheet
	}
	chartBOF := []byte{
		0x00, 0x06, // version = BIFF8
		0x20, 0x00, // type = chart
	}

	data := appendBiffRecord(nil, biffBOF, sheetBOF)
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffEOF, nil) // chart EOF
	data = appendBiffRecord(data, biffNumber, numberCellPayload(0, 0, 0, 7))
	data = appendBiffRecord(data, biffEOF, nil) // sheet EOF

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "ChartHost"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Rows) != 1 || len(sheet.Rows[0].Cells) != 1 {
		t.Fatalf("rows/cells = %+v, want one parsed cell after chart substream", sheet.Rows)
	}
	cell := sheet.Rows[0].Cells[0]
	if cell.Ref != "A1" || cell.NumValue != 7 {
		t.Fatalf("cell = %+v, want A1=7", cell)
	}
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseBIFFHeaderFooterText(t *testing.T) {
	// Helper: build a HeaderRecord-style body. cch is in chars; flag controls
	// the hasMultibyte bit; text is the literal bytes (compressed) or UTF-16LE.
	build := func(cch int, flag byte, body []byte) []byte {
		out := make([]byte, 3, 3+len(body))
		binary.LittleEndian.PutUint16(out[0:2], uint16(cch))
		out[2] = flag
		return append(out, body...)
	}

	cases := []struct {
		name    string
		payload []byte
		want    string
	}{
		{"empty body", nil, ""},
		{"truncated to cch only", []byte{0x05, 0x00}, ""}, // cch but no grbit (POI 61287 edge)
		{"cch=0", []byte{0x00, 0x00, 0x00}, ""},
		{"compressed unicode", build(5, 0x00, []byte("Hello")), "Hello"},
		{"full unicode", build(2, 0x01, []byte{0x4B, 0x00, 0x76, 0x00}), "Kv"},
		{"cch larger than body clamps", build(99, 0x00, []byte("Hi")), "Hi"},
	}
	for _, c := range cases {
		if got := parseBIFFHeaderFooterText(c.payload); got != c.want {
			t.Errorf("%s: parseBIFFHeaderFooterText(% x) = %q, want %q", c.name, c.payload, got, c.want)
		}
	}
}

func TestParseWorksheetCapturesHeaderFooterRecords(t *testing.T) {
	build := func(text string) []byte {
		out := make([]byte, 3, 3+len(text))
		binary.LittleEndian.PutUint16(out[0:2], uint16(len(text)))
		out[2] = 0x00 // compressed Unicode
		return append(out, []byte(text)...)
	}

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x10, 0x00})
	data = appendBiffRecord(data, biffHeader, build("&LPage Header"))
	data = appendBiffRecord(data, biffFooter, build("&CPage &P of &N"))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "HF"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.HeaderFooter == nil {
		t.Fatal("expected HeaderFooter to be set by HEADER/FOOTER records")
	}
	if sheet.HeaderFooter.OddHeader != "&LPage Header" {
		t.Fatalf("OddHeader = %q, want %q", sheet.HeaderFooter.OddHeader, "&LPage Header")
	}
	if sheet.HeaderFooter.OddFooter != "&CPage &P of &N" {
		t.Fatalf("OddFooter = %q, want %q", sheet.HeaderFooter.OddFooter, "&CPage &P of &N")
	}
}

func TestApplyHeaderFooterRecordSkipsEmptyText(t *testing.T) {
	sheet := &sheetir.Sheet{}
	applyHeaderFooterRecord(sheet, "header", nil)
	applyHeaderFooterRecord(sheet, "footer", []byte{0x00, 0x00, 0x00})
	if sheet.HeaderFooter != nil {
		t.Fatalf("HeaderFooter = %+v, want nil for empty payloads", sheet.HeaderFooter)
	}
}

func TestApplyHeaderFooterExtRecordCapturesOddEvenFirstStrings(t *testing.T) {
	encodeXLUnicode := func(s string) []byte {
		b := make([]byte, 3, 3+len(s))
		binary.LittleEndian.PutUint16(b[0:2], uint16(len(s)))
		b[2] = 0x00 // compressed
		return append(b, []byte(s)...)
	}
	payload := make([]byte, 28) // 12-byte future header + 16-byte GUID (all zero=current sheet)
	for _, s := range []string{"OH", "OF", "EH", "EF", "FH", "FF"} {
		payload = append(payload, encodeXLUnicode(s)...)
	}
	sheet := &sheetir.Sheet{}
	applyHeaderFooterExtRecord(sheet, payload)
	if sheet.HeaderFooter == nil {
		t.Fatal("expected HeaderFooter")
	}
	if sheet.HeaderFooter.OddHeader != "OH" || sheet.HeaderFooter.OddFooter != "OF" {
		t.Fatalf("odd header/footer = %q/%q, want OH/OF", sheet.HeaderFooter.OddHeader, sheet.HeaderFooter.OddFooter)
	}
	if sheet.HeaderFooter.EvenHeader != "EH" || sheet.HeaderFooter.EvenFooter != "EF" {
		t.Fatalf("even header/footer = %q/%q, want EH/EF", sheet.HeaderFooter.EvenHeader, sheet.HeaderFooter.EvenFooter)
	}
	if sheet.HeaderFooter.FirstHeader != "FH" || sheet.HeaderFooter.FirstFooter != "FF" {
		t.Fatalf("first header/footer = %q/%q, want FH/FF", sheet.HeaderFooter.FirstHeader, sheet.HeaderFooter.FirstFooter)
	}
}

func TestApplyHeaderFooterExtRecordSkipsNonCurrentSheetGUID(t *testing.T) {
	payload := make([]byte, 28)
	payload[12] = 0x01 // non-zero GUID byte => custom view
	sheet := &sheetir.Sheet{}
	applyHeaderFooterExtRecord(sheet, payload)
	if sheet.HeaderFooter != nil {
		t.Fatalf("HeaderFooter = %+v, want nil for non-current-sheet GUID", sheet.HeaderFooter)
	}
}

func TestParseNoteAuthorRejectsNonLetterPayload(t *testing.T) {
	// A NOTE payload whose author bytes contain no letters — should produce no author text.
	payload := make([]byte, 10)
	payload[8] = 2    // cch = 2 (1-byte length)
	payload[9] = 0x00 // flags: 8-bit chars
	// chars: 0x01 and 0x02 — control bytes, no letters
	payload = append(payload, 0x01, 0x02)

	got := parseNoteAuthor(payload)
	if got != "" {
		t.Fatalf("parseNoteAuthor = %q, want empty for non-letter payload", got)
	}
}

func TestParseTxOCommentCharCountPrefersValidCandidate(t *testing.T) {
	payload := make([]byte, 12)
	binary.LittleEndian.PutUint16(payload[4:6], 7)
	binary.LittleEndian.PutUint16(payload[10:12], 9)
	if got := parseTxOCommentCharCount(payload); got != 7 {
		t.Fatalf("parseTxOCommentCharCount = %d, want 7", got)
	}
}

func TestParseExternSheetCapturesInternalSheetRefs(t *testing.T) {
	refs := parseExternSheet([]byte{0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x03, 0x00}, []supBookRef{{kind: supBookKindInternal}})
	if len(refs) != 1 {
		t.Fatalf("len(refs) = %d, want 1", len(refs))
	}
	if refs[0].firstSheet != 2 || refs[0].lastSheet != 3 {
		t.Fatalf("refs[0] = %+v, want firstSheet=2 lastSheet=3", refs[0])
	}
	if refs[0].sourceKind != supBookKindInternal {
		t.Fatalf("refs[0].sourceKind = %q, want internal", refs[0].sourceKind)
	}
}

func TestParseSupBookDecodesExternalURLAndSheetNames(t *testing.T) {
	payload := make([]byte, 0, 64)
	payload = append(payload, 0x02, 0x00) // two sheets
	payload = append(payload, xlUnicodeStringPayload("\x00book.xls")...)
	payload = append(payload, xlUnicodeStringPayload("Sheet1")...)
	payload = append(payload, xlUnicodeStringPayload("Sheet2")...)

	sb := parseSupBook(payload)
	if sb.kind != supBookKindExternal {
		t.Fatalf("kind = %q, want %q", sb.kind, supBookKindExternal)
	}
	if sb.url != "book.xls" {
		t.Fatalf("url = %q, want %q", sb.url, "book.xls")
	}
	if len(sb.sheetNames) != 2 || sb.sheetNames[0] != "Sheet1" || sb.sheetNames[1] != "Sheet2" {
		t.Fatalf("sheetNames = %#v, want [Sheet1 Sheet2]", sb.sheetNames)
	}
}

func TestParseExternalNameRecordCapturesTextAndIxals(t *testing.T) {
	payload := make([]byte, 0, 32)
	payload = append(payload,
		0x00, 0x00, // options
		0x02, 0x00, // ixals
		0x00, 0x00, // unused
		0x09, // name len
		0x00, // compressed unicode
	)
	payload = append(payload, []byte("Rate_Date")...)
	payload = append(payload, 0x00, 0x00) // formula len

	name, ok := parseExternalNameRecord(payload)
	if !ok {
		t.Fatal("parseExternalNameRecord returned !ok")
	}
	if name.text != "Rate_Date" {
		t.Fatalf("text = %q, want Rate_Date", name.text)
	}
	if name.ixals != 2 {
		t.Fatalf("ixals = %d, want 2", name.ixals)
	}
}

func TestParseWorkbookGlobalsHydratesExternSheetExternalNames(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x05, 0x00, 0x05, 0x00})
	supBook := make([]byte, 0, 64)
	supBook = append(supBook, 0x01, 0x00) // one sheet
	supBook = append(supBook, xlUnicodeStringPayload("\x00book.xls")...)
	supBook = append(supBook, xlUnicodeStringPayload("Sheet1")...)
	data = appendBiffRecord(data, biffSupBook, supBook)
	data = appendBiffRecord(data, biffExternalName, []byte{
		0x00, 0x00, // options
		0x00, 0x00, // ixals
		0x00, 0x00, // unused
		0x09, // name len
		0x00, // compressed unicode
		'R', 'a', 't', 'e', '_', 'D', 'a', 't', 'e',
		0x00, 0x00, // formula len
	})
	data = appendBiffRecord(data, biffExternSheet, []byte{
		0x01, 0x00, // count
		0x00, 0x00, // iSupBook
		0x00, 0x00, // first sheet
		0x00, 0x00, // last sheet
	})
	data = appendBiffRecord(data, biffEOF, nil)

	_, _, _, _, externSheets, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(externSheets) != 1 {
		t.Fatalf("len(externSheets) = %d, want 1", len(externSheets))
	}
	if len(externSheets[0].externalNames) != 1 {
		t.Fatalf("len(externSheets[0].externalNames) = %d, want 1", len(externSheets[0].externalNames))
	}
	if externSheets[0].externalNames[0].text != "Rate_Date" {
		t.Fatalf("externalNames[0].text = %q, want Rate_Date", externSheets[0].externalNames[0].text)
	}
}

func TestParseCRNCountRecordHandlesSignedCount(t *testing.T) {
	count, sheetIndex, ok := parseCRNCountRecord([]byte{
		0xFE, 0xFF, // signed -2 => two CRN records
		0x03, 0x00, // sheet index
	})
	if !ok {
		t.Fatal("parseCRNCountRecord returned !ok")
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if sheetIndex != 3 {
		t.Fatalf("sheetIndex = %d, want 3", sheetIndex)
	}
}

func TestParseLegacyExternalLinksCapturesCachedExternalCells(t *testing.T) {
	numberCRNValue := func(v float64) []byte {
		payload := make([]byte, 9)
		payload[0] = 0x01
		binary.LittleEndian.PutUint64(payload[1:9], math.Float64bits(v))
		return payload
	}
	stringCRNValue := func(v string) []byte {
		payload := []byte{0x02}
		return append(payload, xlUnicodeStringPayload(v)...)
	}
	boolCRNValue := func(v bool) []byte {
		payload := make([]byte, 9)
		payload[0] = 0x04
		if v {
			payload[1] = 0x01
		}
		return payload
	}
	errorCRNValue := func(code byte) []byte {
		payload := make([]byte, 9)
		payload[0] = 0x10
		payload[1] = code
		return payload
	}

	data := appendBiffRecord(nil, biffBOF, []byte{0x05, 0x00, 0x05, 0x00})
	supBook := make([]byte, 0, 64)
	supBook = append(supBook, 0x01, 0x00) // one sheet
	supBook = append(supBook, xlUnicodeStringPayload("\x00book.xls")...)
	supBook = append(supBook, xlUnicodeStringPayload("Prices")...)
	data = appendBiffRecord(data, biffSupBook, supBook)
	data = appendBiffRecord(data, biffExternalName, []byte{
		0x00, 0x00, // options
		0x00, 0x00, // ixals
		0x00, 0x00, // unused
		0x09, // name len
		0x00, // compressed unicode
		'R', 'a', 't', 'e', '_', 'D', 'a', 't', 'e',
		0x00, 0x00, // formula len
	})
	data = appendBiffRecord(data, biffCRNCount, []byte{
		0x01, 0x00, // one following CRN record
		0x00, 0x00, // first external sheet
	})
	crn := []byte{
		0x03,       // last col
		0x00,       // first col
		0x00, 0x00, // row 0
	}
	crn = append(crn, numberCRNValue(7)...)
	crn = append(crn, stringCRNValue("North")...)
	crn = append(crn, boolCRNValue(true)...)
	crn = append(crn, errorCRNValue(0x07)...)
	data = appendBiffRecord(data, biffCRN, crn)
	data = appendBiffRecord(data, biffEOF, nil)

	links := parseLegacyExternalLinks(data)
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	link := links[0]
	if link.Target != "book.xls" {
		t.Fatalf("Target = %q, want book.xls", link.Target)
	}
	if len(link.SheetNames) != 1 || link.SheetNames[0] != "Prices" {
		t.Fatalf("SheetNames = %#v, want [Prices]", link.SheetNames)
	}
	if len(link.DefinedNames) != 1 || link.DefinedNames[0].Name != "Rate_Date" {
		t.Fatalf("DefinedNames = %+v, want Rate_Date", link.DefinedNames)
	}
	if len(link.CachedSheets) != 1 {
		t.Fatalf("len(CachedSheets) = %d, want 1", len(link.CachedSheets))
	}
	sheet := link.CachedSheets[0]
	if sheet.ID != 0 || sheet.Name != "Prices" {
		t.Fatalf("cached sheet = %+v, want ID=0 Name=Prices", sheet)
	}
	if len(sheet.Cells) != 4 {
		t.Fatalf("len(sheet.Cells) = %d, want 4", len(sheet.Cells))
	}
	want := []struct {
		ref   string
		kind  string
		value string
	}{
		{ref: "A1", kind: "n", value: "7"},
		{ref: "B1", kind: "str", value: "North"},
		{ref: "C1", kind: "b", value: "1"},
		{ref: "D1", kind: "e", value: "#DIV/0!"},
	}
	for i, cell := range sheet.Cells {
		if cell.Ref != want[i].ref || cell.Type != want[i].kind || cell.Value != want[i].value {
			t.Fatalf("sheet.Cells[%d] = %+v, want ref=%s type=%s value=%s", i, cell, want[i].ref, want[i].kind, want[i].value)
		}
	}
}

func TestParseWorkbookGlobalsKeepsExternalNameScopedToContiguousSupBookBlock(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x05, 0x00, 0x05, 0x00})
	supBook := make([]byte, 0, 64)
	supBook = append(supBook, 0x01, 0x00) // one sheet
	supBook = append(supBook, xlUnicodeStringPayload("\x00book.xls")...)
	supBook = append(supBook, xlUnicodeStringPayload("Sheet1")...)
	data = appendBiffRecord(data, biffSupBook, supBook)
	data = appendBiffRecord(data, biffCodepage, []byte{0xE4, 0x04}) // break the contiguous SUPBOOK block
	data = appendBiffRecord(data, biffExternalName, []byte{
		0x00, 0x00, // options
		0x00, 0x00, // ixals
		0x00, 0x00, // unused
		0x09, // name len
		0x00, // compressed unicode
		'R', 'a', 't', 'e', '_', 'D', 'a', 't', 'e',
		0x00, 0x00, // formula len
	})
	data = appendBiffRecord(data, biffExternSheet, []byte{
		0x01, 0x00, // count
		0x00, 0x00, // iSupBook
		0x00, 0x00, // first sheet
		0x00, 0x00, // last sheet
	})
	data = appendBiffRecord(data, biffEOF, nil)

	_, _, _, _, externSheets, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(externSheets) != 1 {
		t.Fatalf("len(externSheets) = %d, want 1", len(externSheets))
	}
	if len(externSheets[0].externalNames) != 0 {
		t.Fatalf("externalNames = %+v, want none because EXTERNALNAME was not contiguous with SUPBOOK", externSheets[0].externalNames)
	}
}

func TestParseLegacyExternalLinksKeepsCRNsScopedToContiguousSupBookBlock(t *testing.T) {
	numberCRNValue := func(v float64) []byte {
		payload := make([]byte, 9)
		payload[0] = 0x01
		binary.LittleEndian.PutUint64(payload[1:9], math.Float64bits(v))
		return payload
	}

	data := appendBiffRecord(nil, biffBOF, []byte{0x05, 0x00, 0x05, 0x00})
	supBook := make([]byte, 0, 64)
	supBook = append(supBook, 0x01, 0x00) // one sheet
	supBook = append(supBook, xlUnicodeStringPayload("\x00book.xls")...)
	supBook = append(supBook, xlUnicodeStringPayload("Prices")...)
	data = appendBiffRecord(data, biffSupBook, supBook)
	data = appendBiffRecord(data, biffCodepage, []byte{0xE4, 0x04}) // break the contiguous SUPBOOK block
	data = appendBiffRecord(data, biffCRNCount, []byte{
		0x01, 0x00, // one following CRN record
		0x00, 0x00, // sheet index
	})
	crn := []byte{
		0x00,       // last col
		0x00,       // first col
		0x00, 0x00, // row 0
	}
	crn = append(crn, numberCRNValue(7)...)
	data = appendBiffRecord(data, biffCRN, crn)
	data = appendBiffRecord(data, biffEOF, nil)

	links := parseLegacyExternalLinks(data)
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	if len(links[0].CachedSheets) != 0 {
		t.Fatalf("CachedSheets = %+v, want none because CRN block was not contiguous with SUPBOOK", links[0].CachedSheets)
	}
}

func TestDecodeBIFFFormulaResolvesNameXPtgText(t *testing.T) {
	formula := []byte{
		0x39,
		0x00, 0x00, // extern sheet ref index
		0x01, 0x00, // external name index (1-based)
		0x00, 0x00, // reserved
	}
	got, ok := decodeBIFFFormula(formula, legacyFormulaDecodeContext{
		workbook: legacyFormulaWorkbookContext{
			externSheets: []externSheetRef{{
				supBookIndex: 0,
				sourceKind:   supBookKindExternal,
				externalNames: []legacyExternalName{
					{text: "Rate_Date"},
				},
			}},
		},
		row: 1,
		col: 1,
	})
	if !ok {
		t.Fatal("decodeBIFFFormula returned !ok")
	}
	if got != "Rate_Date" {
		t.Fatalf("formula = %q, want Rate_Date", got)
	}
}

func TestParseSupBookClassifiesKinds(t *testing.T) {
	if got := parseSupBook([]byte{0x03, 0x00, 0x01, 0x04}); got.kind != supBookKindInternal || got.sheetCount != 3 {
		t.Fatalf("internal supbook = %+v, want kind=internal sheetCount=3", got)
	}
	if got := parseSupBook([]byte{0x00, 0x00, 0x01, 0x3A}); got.kind != supBookKindAddIn {
		t.Fatalf("addin supbook = %+v, want kind=addin", got)
	}
	if got := parseSupBook([]byte{0x01, 0x00, 0x08, 0x00, 0x00, 0x00}); got.kind != supBookKindExternal {
		t.Fatalf("external supbook = %+v, want kind=external", got)
	}
}

func TestResolveExternSheetIndexRejectsExternalSupBookRefs(t *testing.T) {
	refs := parseExternSheet([]byte{
		0x02, 0x00, // count
		0x00, 0x00, 0x02, 0x00, 0x03, 0x00, // iSupBook=0, first=2, last=3
		0x01, 0x00, 0x00, 0x00, 0x00, 0x00, // iSupBook=1, first=0, last=0
	}, []supBookRef{{kind: supBookKindInternal}, {kind: supBookKindExternal}})

	if got, ok := resolveExternSheetIndex(0, refs, -1); !ok || got != 2 {
		t.Fatalf("resolve internal ref = (%d,%v), want (2,true)", got, ok)
	}
	if _, ok := resolveExternSheetIndex(1, refs, -1); ok {
		t.Fatal("expected external ref resolution to fail")
	}
}

func TestParseChartAIReferenceDecodesPtgArea3d(t *testing.T) {
	payload := []byte{0x01, 0x02, 0x00, 0x00, 0x00, 0x00, 0x0B, 0x00, 0x3B, 0x00, 0x00, 0x00, 0x00, 0x05, 0x00, 0x01, 0x00, 0x01, 0x00}
	ref, ok := parseChartAIReference(payload, 0, []externSheetRef{{firstSheet: 0, lastSheet: 0}})
	if !ok {
		t.Fatal("expected ptgArea3d reference")
	}
	if ref.sheetIndex != 0 || ref.firstRow != 0 || ref.lastRow != 5 || ref.firstCol != 1 || ref.lastCol != 1 {
		t.Fatalf("ref = %+v, want Sheet1!B1:B6", ref)
	}
}

func TestParseChartAIReferenceDecodesPtgArea3dWithSelfReference(t *testing.T) {
	// Area3D PTG with extern sheet index 0xFFFF (self-reference / same sheet)
	payload := []byte{0x01, 0x02, 0x00, 0x00, 0x00, 0x00, 0x0B, 0x00, 0x3B, 0xFF, 0xFF, 0x00, 0x00, 0x05, 0x00, 0x01, 0x00, 0x01, 0x00}
	ref, ok := parseChartAIReference(payload, 2, nil)
	if !ok {
		t.Fatal("expected ptgArea3d self-reference")
	}
	if ref.sheetIndex != 2 || ref.firstRow != 0 || ref.lastRow != 5 || ref.firstCol != 1 || ref.lastCol != 1 {
		t.Fatalf("ref = %+v, want Sheet3!B1:B6", ref)
	}
}

func TestParseXlsExtractsEmbeddedChartSeriesFromWorksheetRecords(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "xls", "apache-WithChart.xls")
	workbook, err := ParseXls(path)
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(workbook.Sheets) != 3 {
		t.Fatalf("len(workbook.Sheets) = %d, want 3", len(workbook.Sheets))
	}
	sheet := workbook.Sheets[1]
	if sheet.Name != "Sheet2" {
		t.Fatalf("sheet.Name = %q, want Sheet2", sheet.Name)
	}
	if len(sheet.Drawings) != 1 {
		t.Fatalf("len(sheet.Drawings) = %d, want 1", len(sheet.Drawings))
	}
	drawing := sheet.Drawings[0]
	if drawing.Type != sheetir.DrawingTypeChart {
		t.Fatalf("drawing.Type = %q, want %q", drawing.Type, sheetir.DrawingTypeChart)
	}
	chart, ok := drawing.Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatalf("drawing chart = %T, want *chart.Chart", drawing.Chart.Chart)
	}
	if chart.ChartType != chartpkg.ChartLine {
		t.Fatalf("chart.ChartType = %v, want %v", chart.ChartType, chartpkg.ChartLine)
	}
	if len(chart.Series) != 2 {
		t.Fatalf("len(chart.Series) = %d, want 2", len(chart.Series))
	}
	if chart.Series[0].Name != "1st Column" {
		t.Fatalf("chart.Series[0].Name = %q, want %q", chart.Series[0].Name, "1st Column")
	}
	if chart.Series[1].Name != "2nd Column" {
		t.Fatalf("chart.Series[1].Name = %q, want %q", chart.Series[1].Name, "2nd Column")
	}
	if got := chart.Series[0].Values; len(got) != 6 || got[0] != 1 || got[5] != 6 {
		t.Fatalf("chart.Series[0].Values = %v, want [1 2 3 4 5 6]", got)
	}
	if got := chart.Series[1].Values; len(got) != 6 || got[0] != 10 || got[5] != 17 {
		t.Fatalf("chart.Series[1].Values = %v, want [10 12 14 9 15 17]", got)
	}
}

func TestParseXlsExtractsLibreChartxEmbeddedChartAsColumnChart(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "xls", "libre-chartx.xls")
	workbook, err := ParseXls(path)
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(workbook.Sheets) != 2 {
		t.Fatalf("len(workbook.Sheets) = %d, want 2", len(workbook.Sheets))
	}
	sheet := workbook.Sheets[0]
	if len(sheet.Drawings) != 1 {
		t.Fatalf("len(sheet.Drawings) = %d, want 1", len(sheet.Drawings))
	}
	drawing := sheet.Drawings[0]
	if drawing.Type != sheetir.DrawingTypeChart {
		t.Fatalf("drawing.Type = %q, want %q", drawing.Type, sheetir.DrawingTypeChart)
	}
	chart, ok := drawing.Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatalf("drawing chart = %T, want *chart.Chart", drawing.Chart.Chart)
	}
	if chart.ChartType != chartpkg.ChartColumn {
		t.Fatalf("chart.ChartType = %v, want %v", chart.ChartType, chartpkg.ChartColumn)
	}
	if chart.Legend.Show {
		t.Fatal("chart.Legend.Show = true, want false")
	}
	if chart.PlotArea.FillColor != "E3E3E3" {
		t.Fatalf("chart.PlotArea.FillColor = %q, want %q", chart.PlotArea.FillColor, "E3E3E3")
	}
	if chart.PlotArea.BorderColor != "808080" {
		t.Fatalf("chart.PlotArea.BorderColor = %q, want %q", chart.PlotArea.BorderColor, "808080")
	}
	if len(chart.Series) != 2 {
		t.Fatalf("len(chart.Series) = %d, want 2", len(chart.Series))
	}
	if chart.Series[0].ChartType != chartpkg.ChartColumn {
		t.Fatalf("chart.Series[0].ChartType = %v, want %v", chart.Series[0].ChartType, chartpkg.ChartColumn)
	}
	if chart.Series[0].Name != "Series 1" {
		t.Fatalf("chart.Series[0].Name = %q, want %q", chart.Series[0].Name, "Series 1")
	}
	if chart.Series[0].FillColor != "339933" {
		t.Fatalf("chart.Series[0].FillColor = %q, want %q", chart.Series[0].FillColor, "339933")
	}
	if chart.Series[1].ChartType != chartpkg.ChartLine {
		t.Fatalf("chart.Series[1].ChartType = %v, want %v", chart.Series[1].ChartType, chartpkg.ChartLine)
	}
	if chart.Series[1].Name != "Series 2" {
		t.Fatalf("chart.Series[1].Name = %q, want %q", chart.Series[1].Name, "Series 2")
	}
	if chart.Series[1].MarkerColor != "FF0000" {
		t.Fatalf("chart.Series[1].MarkerColor = %q, want %q", chart.Series[1].MarkerColor, "FF0000")
	}
	if chart.Series[1].MarkerShape != "diamond" {
		t.Fatalf("chart.Series[1].MarkerShape = %q, want %q", chart.Series[1].MarkerShape, "diamond")
	}
	if chart.Series[1].LinePattern != 5 {
		t.Fatalf("chart.Series[1].LinePattern = %d, want 5", chart.Series[1].LinePattern)
	}
	if got := chart.Series[0].Values; len(got) != 4 || got[0] != 17 || got[3] != 85 {
		t.Fatalf("chart.Series[0].Values = %v, want [17 26 11 85]", got)
	}
	if got := chart.Series[1].Values; len(got) != 4 || got[0] != 18 || got[3] != 42 {
		t.Fatalf("chart.Series[1].Values = %v, want [18 20 33 42]", got)
	}
	if got := chart.Categories; len(got) != 4 || got[0] != "1" || got[3] != "4" {
		t.Fatalf("chart.Categories = %v, want [1 2 3 4]", got)
	}
}

func TestParseXlsExtractsWorksheetImagesFromOfficeArt(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "xls", "apache-SimpleWithImages.xls")
	workbook, err := ParseXls(path)
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(workbook.Sheets) != 3 {
		t.Fatalf("len(workbook.Sheets) = %d, want 3", len(workbook.Sheets))
	}
	sheet := workbook.Sheets[0]
	if sheet.Name != "Sheet1" {
		t.Fatalf("sheet.Name = %q, want Sheet1", sheet.Name)
	}
	want := map[string]string{
		"A1:D11":  "image/jpeg",
		"E2:J19":  "image/png",
		"B21:D32": "image/x-wmf",
		"F25:I33": "image/x-emf",
	}
	seen := map[string]string{}
	for _, drawing := range sheet.Drawings {
		if drawing.Type != sheetir.DrawingTypeImage {
			t.Fatalf("drawing.Type = %q, want %q", drawing.Type, sheetir.DrawingTypeImage)
		}
		if drawing.Image == nil || len(drawing.Image.Data) == 0 {
			t.Fatal("drawing missing image payload")
		}
		if drawing.Position.Anchor != sheetir.AnchorTwoCell {
			t.Fatalf("drawing.Position.Anchor = %q, want %q", drawing.Position.Anchor, sheetir.AnchorTwoCell)
		}
		anchor := formatA1Ref(drawing.Position.FromCell.Row, drawing.Position.FromCell.Col) + ":" + formatA1Ref(drawing.Position.ToCell.Row, drawing.Position.ToCell.Col)
		seen[anchor] = drawing.Image.ContentType
	}
	if len(seen) != len(want) {
		t.Fatalf("len(sheet.Drawings) = %d, seen anchors = %v, want %v", len(sheet.Drawings), seen, want)
	}
	for anchor, wantContentType := range want {
		if got := seen[anchor]; got != wantContentType {
			t.Fatalf("drawing %s content type = %q, want %q", anchor, got, wantContentType)
		}
	}
}

func TestParseWorksheetExtractsLegacyOfficeArtShapes(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffObj, objPayload(legacyObjTypeButton, 1))
	data = appendBiffRecord(data, biffMsoDrawing, legacyControlDrawing(
		legacyControlShape(1, 1, 2, 2),
		legacyOfficeArtShape(3, 3, 3, 6, 7, 0x0000FF00, 0x00FF0000, 25400),
		legacyOfficeArtShape(34, 8, 2, 8, 6, 0, 0x000000FF, 12700),
	))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Shapes"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.FormControls) != 1 {
		t.Fatalf("len(sheet.FormControls) = %d, want 1", len(sheet.FormControls))
	}
	if got := sheet.FormControls[0].Anchor; got != "0, 0, 0, 0, 1, 0, 1, 0" {
		t.Fatalf("control anchor = %q, want first non-picture OfficeArt anchor", got)
	}
	if len(sheet.Drawings) != 2 {
		t.Fatalf("len(sheet.Drawings) = %d, want 2", len(sheet.Drawings))
	}
	if got := sheet.Drawings[0]; got.Type != sheetir.DrawingTypeShape || got.Shape == nil || got.Shape.Geometry != "ellipse" || got.Shape.FillColor != "00FF00" || got.Shape.LineColor != "0000FF" {
		t.Fatalf("shape drawing = %#v", got)
	}
	if got := sheet.Drawings[0].Shape.LineWidthPt; math.Abs(got-2.0) > 1e-9 {
		t.Fatalf("shape line width = %v, want 2", got)
	}
	if got := sheet.Drawings[0].Position.FromCell; got.Row != 3 || got.Col != 3 {
		t.Fatalf("shape from anchor = %#v, want row 3 col 3", got)
	}
	if got := sheet.Drawings[1]; got.Type != sheetir.DrawingTypeConnector || got.Shape == nil || got.Shape.Geometry != "straightConnector1" || got.Shape.LineColor != "FF0000" {
		t.Fatalf("connector drawing = %#v", got)
	}
}

func TestParseWorksheetRetainsOBJBackedLegacyShapeTextAndLineSemantics(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	// Object type 0x0006 is deliberately not a form control. Historically its
	// OfficeArt anchor was consumed by OBJ ordering and the shape was dropped.
	data = appendBiffRecord(data, biffObj, objPayload(0x0006, 42))
	data = appendBiffRecord(data, biffTxO, txoPayload(12))
	data = appendBiffRecord(data, biffContinue, append([]byte{0x00}, []byte("Legacy shape")...))
	props := []byte{}
	props = append(props, officeArtProperty(legacyOfficeArtPropAdjustValue, 10800)...)
	props = append(props, officeArtProperty(legacyOfficeArtPropFillColor, 0x0000FF00)...)
	props = append(props, officeArtProperty(legacyOfficeArtPropLineColor, 0x00FF0000)...)
	props = append(props, officeArtProperty(legacyOfficeArtPropLineWidth, 25400)...)
	props = append(props, officeArtProperty(legacyOfficeArtPropLineDashing, 8)...)
	props = append(props, officeArtProperty(legacyOfficeArtPropLineStartArrow, 1)...)
	props = append(props, officeArtProperty(legacyOfficeArtPropLineEndArrow, 5)...)
	props = append(props, officeArtProperty(legacyOfficeArtPropLineStartWidth, 0)...)
	props = append(props, officeArtProperty(legacyOfficeArtPropLineEndLength, 2)...)
	data = appendBiffRecord(data, biffMsoDrawing, legacyOfficeArtShapeWithProps(34, 2, 2, 4, 6, props))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Shapes"}, nil, nil)
	if sheet == nil || len(sheet.Drawings) != 1 {
		t.Fatalf("drawings = %#v, want one retained OBJ-backed shape", sheet)
	}
	got := sheet.Drawings[0]
	if got.Name != "Shapes Object 42" || got.Type != sheetir.DrawingTypeConnector || got.Shape == nil {
		t.Fatalf("drawing = %#v", got)
	}
	if got.Shape.Text != "Legacy shape" {
		t.Fatalf("shape text = %q, want Legacy shape", got.Shape.Text)
	}
	if len(got.Shape.Adjustments) != 1 || math.Abs(got.Shape.Adjustments[0]-0.5) > 1e-9 {
		t.Fatalf("adjustments = %v, want [0.5]", got.Shape.Adjustments)
	}
	line := got.Shape.LineStyle
	if line == nil || line.Dash != drawing.DashPatternDashDot || line.Color != "0000FF" || math.Abs(line.Width-2) > 1e-9 {
		t.Fatalf("line style = %#v", line)
	}
	if line.HeadEnd == nil || line.HeadEnd.Type != drawing.ArrowheadTriangle || line.HeadEnd.Size != drawing.ArrowheadSizeSmall {
		t.Fatalf("head end = %#v", line.HeadEnd)
	}
	if line.TailEnd == nil || line.TailEnd.Type != drawing.ArrowheadArrow || line.TailEnd.Length != drawing.ArrowheadSizeLarge {
		t.Fatalf("tail end = %#v", line.TailEnd)
	}
}

func TestLegacyOfficeArtDashAndArrowMappings(t *testing.T) {
	if got := legacyOfficeArtDashPattern(10); got != drawing.DashPatternLongDashDotDot {
		t.Fatalf("dash pattern = %q", got)
	}
	for raw, want := range map[uint32]drawing.ArrowheadType{
		1: drawing.ArrowheadTriangle,
		2: drawing.ArrowheadStealth,
		3: drawing.ArrowheadDiamond,
		4: drawing.ArrowheadOval,
		5: drawing.ArrowheadArrow,
	} {
		if got := legacyOfficeArtArrowhead(raw); got == nil || got.Type != want {
			t.Fatalf("arrow %d = %#v, want %q", raw, got, want)
		}
	}
}

func TestParseXlsExtractsRasterWorksheetImagesFromLibreFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "xls", "libre-file-with-png-image.xls")
	workbook, err := ParseXls(path)
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(workbook.Sheets) == 0 {
		t.Fatal("expected sheets")
	}
	sheet := workbook.Sheets[0]
	if len(sheet.Drawings) != 1 {
		t.Fatalf("len(sheet.Drawings) = %d, want 1", len(sheet.Drawings))
	}
	drawing := sheet.Drawings[0]
	if drawing.Type != sheetir.DrawingTypeImage {
		t.Fatalf("drawing.Type = %q, want %q", drawing.Type, sheetir.DrawingTypeImage)
	}
	if drawing.Image == nil || drawing.Image.ContentType != "image/png" || len(drawing.Image.Data) == 0 {
		t.Fatalf("unexpected drawing image payload: %#v", drawing.Image)
	}
	if gotFrom := formatA1Ref(drawing.Position.FromCell.Row, drawing.Position.FromCell.Col); gotFrom != "A1" {
		t.Fatalf("drawing from = %s, want A1", gotFrom)
	}
	if gotTo := formatA1Ref(drawing.Position.ToCell.Row, drawing.Position.ToCell.Col); gotTo != "H11" {
		t.Fatalf("drawing to = %s, want H11", gotTo)
	}
}

func TestParseXlsHandlesSupBookAndExternSheetFixtures(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "xls", "libre-external-ref.xls")
	workbook, err := ParseXls(path)
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(workbook.Sheets) == 0 {
		t.Fatal("expected sheets from external-ref fixture")
	}
}

func TestParseXlsHandlesRawWorksheetFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "xls", "libre-tdf144732.xls")
	workbook, err := ParseXls(path)
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(workbook.Sheets) != 1 {
		t.Fatalf("len(workbook.Sheets) = %d, want 1", len(workbook.Sheets))
	}
	sheet := workbook.Sheets[0]
	if sheet.Name != "Sheet1" {
		t.Fatalf("sheet.Name = %q, want Sheet1", sheet.Name)
	}
	if len(sheet.Rows) != 6 {
		t.Fatalf("len(sheet.Rows) = %d, want 6", len(sheet.Rows))
	}
	if sheet.Rows[0].Index != 1 || sheet.Rows[5].Index != 6 {
		t.Fatalf("row indexes = %d..%d, want 1..6", sheet.Rows[0].Index, sheet.Rows[5].Index)
	}
	if len(sheet.Rows[5].Cells) != 1 {
		t.Fatalf("len(sheet.Rows[5].Cells) = %d, want 1", len(sheet.Rows[5].Cells))
	}
}

func TestParseXlsHandlesRawBIFF2WorksheetFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "xls", "libre-tdf72470.xls")
	workbook, err := ParseXls(path)
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(workbook.Sheets) != 1 {
		t.Fatalf("len(workbook.Sheets) = %d, want 1", len(workbook.Sheets))
	}
	sheet := workbook.Sheets[0]
	if sheet.Name != "Sheet1" {
		t.Fatalf("sheet.Name = %q, want Sheet1", sheet.Name)
	}
	if len(sheet.Rows) != 3 {
		t.Fatalf("len(sheet.Rows) = %d, want 3", len(sheet.Rows))
	}
	if len(sheet.Rows[0].Cells) != 1 || sheet.Rows[0].Cells[0].StrValue != "name" {
		t.Fatalf("first row cells = %#v, want one cell with value %q", sheet.Rows[0].Cells, "name")
	}
}

func TestParseArrayFormulaRecordDecodesRange(t *testing.T) {
	group, ok := parseArrayFormulaRecord(arrayPayload(0, 1, 0, 1), legacyFormulaWorkbookContext{})
	if !ok {
		t.Fatal("expected array formula group")
	}
	if group.AnchorRef != "A1" {
		t.Fatalf("group.AnchorRef = %q, want A1", group.AnchorRef)
	}
	if group.RangeRef != "A1:B2" {
		t.Fatalf("group.RangeRef = %q, want A1:B2", group.RangeRef)
	}
	if group.TopRow != 1 || group.BottomRow != 2 || group.LeftCol != 1 || group.RightCol != 2 {
		t.Fatalf("group bounds = (%d,%d)-(%d,%d), want (1,1)-(2,2)", group.TopRow, group.LeftCol, group.BottomRow, group.RightCol)
	}
}

func TestParseWorksheetMarksArrayFormulaGroupMembers(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffBlank, blankCellPayload(0, 0, 0))
	data = appendBiffRecord(data, biffBlank, blankCellPayload(0, 1, 0))
	data = appendBiffRecord(data, biffArray, arrayPayload(0, 0, 0, 1))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Array"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.ArrayFormulas) != 1 {
		t.Fatalf("len(sheet.ArrayFormulas) = %d, want 1", len(sheet.ArrayFormulas))
	}
	group := sheet.ArrayFormulas[0]
	if group.AnchorRef != "A1" || group.RangeRef != "A1:B1" {
		t.Fatalf("group = %#v, want anchor A1 and range A1:B1", group)
	}
	if len(sheet.Rows) != 1 || len(sheet.Rows[0].Cells) != 2 {
		t.Fatalf("sheet rows/cells = %d/%d, want 1/2", len(sheet.Rows), len(sheet.Rows[0].Cells))
	}
	for _, cell := range sheet.Rows[0].Cells {
		if cell.ArrayFormulaRange != "A1:B1" {
			t.Fatalf("cell %s array range = %q, want A1:B1", cell.Ref, cell.ArrayFormulaRange)
		}
		if cell.ArrayFormulaAnchor != "A1" {
			t.Fatalf("cell %s array anchor = %q, want A1", cell.Ref, cell.ArrayFormulaAnchor)
		}
	}
}

func TestParseWorksheetDecompilesArrayFormulaRecord(t *testing.T) {
	formulaBytes := []byte{
		0x25,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0xC0,
		0x01, 0xC0,
		0x19, 0x10, 0x00, 0x00,
	}
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffBlank, blankCellPayload(0, 0, 0))
	data = appendBiffRecord(data, biffBlank, blankCellPayload(0, 1, 0))
	data = appendBiffRecord(data, biffArray, arrayFormulaPayload(0, 0, 0, 1, formulaBytes))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Array"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.ArrayFormulas) != 1 {
		t.Fatalf("len(sheet.ArrayFormulas) = %d, want 1", len(sheet.ArrayFormulas))
	}
	if got := sheet.ArrayFormulas[0].Formula; got != "SUM(A1:B1)" {
		t.Fatalf("array formula = %q, want SUM(A1:B1)", got)
	}
}

func TestParseWorksheetResolvesPtgExpArrayFormulaMembers(t *testing.T) {
	formulaBytes := []byte{
		0x25,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x01, 0x00,
	}
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffNumber, numberCellPayload(0, 0, 0, 1))
	data = appendBiffRecord(data, biffNumber, numberCellPayload(0, 1, 0, 2))
	data = appendBiffRecord(data, biffFormula, numberFormulaPayload(0, 2, 0, 0, expFormulaPayload(0, 2)))
	data = appendBiffRecord(data, biffFormula, numberFormulaPayload(0, 3, 0, 0, expFormulaPayload(0, 2)))
	data = appendBiffRecord(data, biffArray, arrayFormulaPayload(0, 0, 2, 3, formulaBytes))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "ArrayExp"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	row := sheet.Rows[0]
	if got := row.Cells[2].Formula; got != "$A$1:$B$1" {
		t.Fatalf("anchor formula = %q, want $A$1:$B$1", got)
	}
	if got := row.Cells[3].Formula; got != "$A$1:$B$1" {
		t.Fatalf("member formula = %q, want $A$1:$B$1", got)
	}
	for _, cell := range row.Cells[2:4] {
		if cell.ArrayFormulaRange != "C1:D1" {
			t.Fatalf("cell %s array range = %q, want C1:D1", cell.Ref, cell.ArrayFormulaRange)
		}
		if cell.ArrayFormulaAnchor != "C1" {
			t.Fatalf("cell %s array anchor = %q, want C1", cell.Ref, cell.ArrayFormulaAnchor)
		}
	}
}

func TestParseWorksheetResolvesSharedFormulaPtgExpMembers(t *testing.T) {
	formulaBytes := []byte{
		0x24,
		0x00, 0x00,
		0xFE, 0xC0,
		0x24,
		0x00, 0x00,
		0xFF, 0xC0,
		0x03,
	}
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffNumber, numberCellPayload(0, 0, 0, 1))
	data = appendBiffRecord(data, biffNumber, numberCellPayload(0, 1, 0, 2))
	data = appendBiffRecord(data, biffFormula, numberFormulaPayload(0, 2, 0, 3, expFormulaPayload(0, 2)))
	data = appendBiffRecord(data, biffShrFmla, sharedFormulaPayload(0, 0, 2, 3, formulaBytes))
	data = appendBiffRecord(data, biffFormula, numberFormulaPayload(0, 3, 0, 5, expFormulaPayload(0, 2)))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "SharedExp"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	row := sheet.Rows[0]
	if got := row.Cells[2].Formula; got != "A1+B1" {
		t.Fatalf("anchor shared formula = %q, want A1+B1", got)
	}
	if got := row.Cells[3].Formula; got != "B1+C1" {
		t.Fatalf("member shared formula = %q, want B1+C1", got)
	}
	if got := row.Cells[2].NumValue; got != 3 {
		t.Fatalf("anchor shared cached value = %v, want 3", got)
	}
	if got := row.Cells[3].NumValue; got != 5 {
		t.Fatalf("member shared cached value = %v, want 5", got)
	}
}

func TestParseWorksheetSkipsMemoryControlPtgsInFormulaDecompilation(t *testing.T) {
	formulaBytes := []byte{
		0x29, 0x09, 0x00,
		0x25,
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0xC0,
		0x01, 0xC0,
		0x19, 0x10, 0x00, 0x00,
	}
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffFormula, emptyFormulaPayload(0, 0, 0, formulaBytes))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "MemFunc"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.Rows) != 1 || len(sheet.Rows[0].Cells) != 1 {
		t.Fatalf("sheet rows/cells = %d/%d, want 1/1", len(sheet.Rows), len(sheet.Rows[0].Cells))
	}
	if got := sheet.Rows[0].Cells[0].Formula; got != "SUM(A1:B1)" {
		t.Fatalf("formula = %q, want SUM(A1:B1)", got)
	}
}

func TestDecodeBIFFFormulaSkipsChooseAttrJumpTables(t *testing.T) {
	formulaBytes := []byte{
		0x1E, 0x01, 0x00,
		0x19, 0x04, 0x03, 0x00, 0x08, 0x00, 0x11, 0x00, 0x1A, 0x00, 0x23, 0x00,
		0x1E, 0x0A, 0x00,
		0x1E, 0x14, 0x00,
		0x1E, 0x1E, 0x00,
		0x22, 0x04, 0x64, 0x00,
	}
	got, ok := decodeBIFFFormula(formulaBytes, legacyFormulaDecodeContext{})
	if !ok {
		t.Fatal("decodeBIFFFormula() failed for tAttrChoose formula")
	}
	if got != "CHOOSE(1,10,20,30)" {
		t.Fatalf("decodeBIFFFormula() = %q, want CHOOSE(1,10,20,30)", got)
	}
}

func TestDecodeBIFFFormulaReconstructsIfAttrControlFlow(t *testing.T) {
	formulaBytes := []byte{
		0x1E, 0x01, 0x00,
		0x19, 0x02, 0x07, 0x00,
		0x1E, 0x0A, 0x00,
		0x19, 0x08, 0x03, 0x00,
		0x1E, 0x14, 0x00,
	}
	got, ok := decodeBIFFFormula(formulaBytes, legacyFormulaDecodeContext{})
	if !ok {
		t.Fatal("decodeBIFFFormula() failed for optimized IF attr formula")
	}
	if got != "IF(1,10,20)" {
		t.Fatalf("decodeBIFFFormula() = %q, want IF(1,10,20)", got)
	}
}

func TestDecodeBIFFFormulaSupportsAdditionalFixedDatabaseFunctions(t *testing.T) {
	tests := []struct {
		name    string
		iftab   uint16
		formula string
	}{
		{name: "DPRODUCT", iftab: 189, formula: "DPRODUCT(1,2,3)"},
		{name: "DSTDEVP", iftab: 195, formula: "DSTDEVP(1,2,3)"},
		{name: "DVARP", iftab: 196, formula: "DVARP(1,2,3)"},
		{name: "DCOUNTA", iftab: 199, formula: "DCOUNTA(1,2,3)"},
		{name: "DGET", iftab: 235, formula: "DGET(1,2,3)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			formulaBytes := []byte{
				0x1E, 0x01, 0x00,
				0x1E, 0x02, 0x00,
				0x1E, 0x03, 0x00,
				0x21, byte(tc.iftab), byte(tc.iftab >> 8),
			}
			got, ok := decodeBIFFFormula(formulaBytes, legacyFormulaDecodeContext{})
			if !ok {
				t.Fatalf("decodeBIFFFormula() failed for iftab %d", tc.iftab)
			}
			if got != tc.formula {
				t.Fatalf("decodeBIFFFormula() = %q, want %q", got, tc.formula)
			}
		})
	}
}

func TestParseXlsDecodesOffsetBuiltinsFromApacheFormulaCorpus(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "xls", "apache-FormulaEvalTestData.xls")
	workbook, err := ParseXls(path)
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(workbook.Sheets) == 0 {
		t.Fatal("expected sheets from formula corpus fixture")
	}
	var target *sheetir.Cell
	for _, sheet := range workbook.Sheets {
		if sheet == nil || sheet.Name != "EverythingTests" {
			continue
		}
		for _, row := range sheet.Rows {
			for _, cell := range row.Cells {
				if cell != nil && cell.Ref == "H1044" {
					target = cell
					break
				}
			}
			if target != nil {
				break
			}
		}
	}
	if target == nil {
		t.Fatal("expected to find EverythingTests!H1044")
	}
	if got := target.Formula; got != "SUM(OFFSET(R1050:S1051,0,0,1,1))" {
		t.Fatalf("EverythingTests!H1044 formula = %q, want SUM(OFFSET(R1050:S1051,0,0,1,1))", got)
	}
}

func TestParseXlsDecodesMatrixArrayFormulaFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "xls", "apache-MatrixFormulaEvalTestData.xls")
	workbook, err := ParseXls(path)
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(workbook.Sheets) != 1 {
		t.Fatalf("len(workbook.Sheets) = %d, want 1", len(workbook.Sheets))
	}
	sheet := workbook.Sheets[0]
	if len(sheet.ArrayFormulas) == 0 {
		t.Fatal("expected array formulas from matrix fixture")
	}
	checks := map[string]string{
		"H2":  "B2:D4+E2:G4",
		"H6":  "TRANSPOSE(B6:D8)",
		"H10": "MDETERM(B10:D12)",
		"H14": "MINVERSE(B14:D16)",
		"H18": "MMULT(B18:D20,E18:G20)",
	}
	gotByAnchor := map[string]string{}
	for _, group := range sheet.ArrayFormulas {
		if group == nil {
			continue
		}
		gotByAnchor[group.AnchorRef] = group.Formula
	}
	for anchor, want := range checks {
		if got := gotByAnchor[anchor]; got != want {
			t.Fatalf("array formula %s = %q, want %q", anchor, got, want)
		}
	}
}

func TestParseXlsRendersScalarFormulaThroughSharedEngine(t *testing.T) {
	formulaBytes := []byte{
		0x24,
		0xFF, 0xFF,
		0xFF, 0xC0,
		0x1E, 0x01, 0x00,
		0x03,
	}
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffNumber, numberCellPayload(0, 0, 0, 1))
	data = appendBiffRecord(data, biffFormula, emptyFormulaPayload(1, 1, 0, formulaBytes))
	data = appendBiffRecord(data, biffEOF, nil)

	path := filepath.Join(t.TempDir(), "array-exp.xls")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write temp xls: %v", err)
	}
	wb, err := ParseXls(path)
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	out, err := sheetrender.HTML(wb)
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	if !strings.Contains(out, `data-formula="A1+1"`) {
		t.Fatalf("expected decompiled array formula text in output:\n%s", out)
	}
	if !strings.Contains(out, `>2</td>`) {
		t.Fatalf("expected computed formula value from shared engine in output:\n%s", out)
	}
	if strings.Contains(out, `data-array-formula-range=`) {
		t.Fatalf("expected scalar formula render, not array metadata:\n%s", out)
	}
}

func TestParseXlsRendersSharedFormulaThroughSharedEngine(t *testing.T) {
	formulaBytes := []byte{
		0x24,
		0x00, 0x00,
		0xFE, 0xC0,
		0x24,
		0x00, 0x00,
		0xFF, 0xC0,
		0x03,
	}
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffNumber, numberCellPayload(0, 0, 0, 1))
	data = appendBiffRecord(data, biffNumber, numberCellPayload(0, 1, 0, 2))
	data = appendBiffRecord(data, biffFormula, emptyFormulaPayload(0, 2, 0, expFormulaPayload(0, 2)))
	data = appendBiffRecord(data, biffShrFmla, sharedFormulaPayload(0, 0, 2, 3, formulaBytes))
	data = appendBiffRecord(data, biffFormula, emptyFormulaPayload(0, 3, 0, expFormulaPayload(0, 2)))
	data = appendBiffRecord(data, biffEOF, nil)

	path := filepath.Join(t.TempDir(), "shared-exp.xls")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write temp xls: %v", err)
	}
	wb, err := ParseXls(path)
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	out, err := sheetrender.HTML(wb)
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	if !strings.Contains(out, `data-formula="A1+B1"`) {
		t.Fatalf("expected shared formula anchor text in output:\n%s", out)
	}
	if !strings.Contains(out, `data-formula="B1+C1"`) {
		t.Fatalf("expected shifted shared formula member text in output:\n%s", out)
	}
	if !strings.Contains(out, `>3</td>`) || !strings.Contains(out, `>5</td>`) {
		t.Fatalf("expected computed shared-formula values from shared engine in output:\n%s", out)
	}
}

func appendBiffRecord(data []byte, id uint16, payload []byte) []byte {
	buf := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint16(buf[0:2], id)
	binary.LittleEndian.PutUint16(buf[2:4], uint16(len(payload)))
	copy(buf[4:], payload)
	return append(data, buf...)
}

func xlUnicodeStringPayload(s string) []byte {
	payload := make([]byte, 0, 3+len(s))
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(len(s)))
	payload = append(payload, buf...)
	payload = append(payload, 0x00) // compressed unicode
	payload = append(payload, []byte(s)...)
	return payload
}

func xlUnicodeStringNoCchPayload(s string) []byte {
	payload := make([]byte, 0, 1+len(s))
	payload = append(payload, 0x00)
	payload = append(payload, []byte(s)...)
	return payload
}

func pivotViewPayload(firstRow, lastRow, firstCol, lastCol, cacheIndex, rowFields, colFields, pageFields, dataFields int, rowGrand, colGrand bool, name, dataCaption string) []byte {
	namePayload := xlUnicodeStringNoCchPayload(name)
	dataPayload := xlUnicodeStringNoCchPayload(dataCaption)
	payload := make([]byte, 42)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(firstRow))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(lastRow))
	binary.LittleEndian.PutUint16(payload[4:6], uint16(firstCol))
	binary.LittleEndian.PutUint16(payload[6:8], uint16(lastCol))
	binary.LittleEndian.PutUint16(payload[8:10], uint16(firstRow))
	binary.LittleEndian.PutUint16(payload[10:12], uint16(firstRow+colFields))
	binary.LittleEndian.PutUint16(payload[12:14], uint16(firstCol+rowFields))
	binary.LittleEndian.PutUint16(payload[14:16], uint16(cacheIndex))
	binary.LittleEndian.PutUint16(payload[20:22], uint16(rowFields+colFields+pageFields+dataFields))
	binary.LittleEndian.PutUint16(payload[22:24], uint16(rowFields))
	binary.LittleEndian.PutUint16(payload[24:26], uint16(colFields))
	binary.LittleEndian.PutUint16(payload[26:28], uint16(pageFields))
	binary.LittleEndian.PutUint16(payload[28:30], uint16(dataFields))
	flags := uint16(0)
	if rowGrand {
		flags |= 0x0001
	}
	if colGrand {
		flags |= 0x0002
	}
	binary.LittleEndian.PutUint16(payload[34:36], flags)
	binary.LittleEndian.PutUint16(payload[38:40], uint16(len(name)))
	binary.LittleEndian.PutUint16(payload[40:42], uint16(len(dataCaption)))
	payload = append(payload, namePayload...)
	payload = append(payload, dataPayload...)
	return payload
}

func pivotViewFieldPayload(axis uint16, name string) []byte {
	payload := make([]byte, 10)
	binary.LittleEndian.PutUint16(payload[0:2], axis)
	binary.LittleEndian.PutUint16(payload[8:10], uint16(len(name)))
	payload = append(payload, xlUnicodeStringNoCchPayload(name)...)
	return payload
}

func pivotItemPayload(cacheIndex int, hidden, hideDetail bool, name string) []byte {
	payload := make([]byte, 8)
	flags := uint16(0)
	if hidden {
		flags |= 0x0001
	}
	if hideDetail {
		flags |= 0x0002
	}
	binary.LittleEndian.PutUint16(payload[2:4], flags)
	binary.LittleEndian.PutUint16(payload[4:6], uint16(cacheIndex))
	binary.LittleEndian.PutUint16(payload[6:8], uint16(len(name)))
	payload = append(payload, xlUnicodeStringNoCchPayload(name)...)
	return payload
}

func pivotPageFieldSelectionPayload(fieldIndex, itemIndex int) []byte {
	payload := make([]byte, 6)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(fieldIndex))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(itemIndex))
	return payload
}

func pivotAxisOrderPayload(indices ...int) []byte {
	payload := make([]byte, len(indices)*2)
	for i, index := range indices {
		binary.LittleEndian.PutUint16(payload[i*2:i*2+2], uint16(index))
	}
	return payload
}

func pivotPageFieldsPayload(indices ...int) []byte {
	payload := make([]byte, len(indices)*6)
	for i, index := range indices {
		offset := i * 6
		binary.LittleEndian.PutUint16(payload[offset:offset+2], uint16(index))
	}
	return payload
}

func pivotDataFieldPayload(fieldIndex int, subtotal uint16, numFmtID int, name string) []byte {
	payload := make([]byte, 14)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(fieldIndex))
	binary.LittleEndian.PutUint16(payload[2:4], subtotal)
	binary.LittleEndian.PutUint16(payload[10:12], uint16(numFmtID))
	binary.LittleEndian.PutUint16(payload[12:14], uint16(len(name)))
	payload = append(payload, xlUnicodeStringNoCchPayload(name)...)
	return payload
}

func dconRefPayload(firstRow, lastRow, firstCol, lastCol int, sheet string) []byte {
	file := string(rune(0x0002)) + sheet
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(firstRow))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(lastRow))
	payload[4] = byte(firstCol)
	payload[5] = byte(lastCol)
	binary.LittleEndian.PutUint16(payload[6:8], uint16(len(file)))
	payload = append(payload, xlUnicodeStringNoCchPayload(file)...)
	payload = append(payload, 0x00)
	return payload
}

func dconNamePayload(name, sheet string) []byte {
	payload := xlUnicodeStringPayload(name)
	if sheet == "" {
		payload = append(payload, 0x00, 0x00)
		return payload
	}
	file := string(rune(0x0002)) + sheet
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, uint16(len(file)))
	payload = append(payload, buf...)
	payload = append(payload, xlUnicodeStringNoCchPayload(file)...)
	payload = append(payload, 0x00)
	return payload
}

func blankCellPayload(row, col int, xfIndex uint16) []byte {
	payload := make([]byte, 6)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(col))
	binary.LittleEndian.PutUint16(payload[4:6], xfIndex)
	return payload
}

func mulBlankPayload(row, firstCol int, xfIndexes []uint16) []byte {
	payload := make([]byte, 4+len(xfIndexes)*2+2)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(firstCol))
	offset := 4
	for _, xfIndex := range xfIndexes {
		binary.LittleEndian.PutUint16(payload[offset:offset+2], xfIndex)
		offset += 2
	}
	binary.LittleEndian.PutUint16(payload[offset:offset+2], uint16(firstCol+len(xfIndexes)-1))
	return payload
}

func mergeCellsPayload(ranges ...[4]int) []byte {
	payload := make([]byte, 2+len(ranges)*8)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(len(ranges)))
	offset := 2
	for _, r := range ranges {
		binary.LittleEndian.PutUint16(payload[offset:offset+2], uint16(r[0]))
		binary.LittleEndian.PutUint16(payload[offset+2:offset+4], uint16(r[1]))
		binary.LittleEndian.PutUint16(payload[offset+4:offset+6], uint16(r[2]))
		binary.LittleEndian.PutUint16(payload[offset+6:offset+8], uint16(r[3]))
		offset += 8
	}
	return payload
}

func rowPayload(row int, flags uint16) []byte {
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
	binary.LittleEndian.PutUint16(payload[12:14], flags)
	return payload
}

func arrayPayload(firstRow, lastRow, firstCol, lastCol int) []byte {
	payload := make([]byte, 14)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(firstRow))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(lastRow))
	payload[4] = byte(firstCol)
	payload[5] = byte(lastCol)
	return payload
}

func numberFormulaPayload(row, col int, xfIndex uint16, result float64, formula []byte) []byte {
	payload := make([]byte, 22+len(formula))
	binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(col))
	binary.LittleEndian.PutUint16(payload[4:6], xfIndex)
	binary.LittleEndian.PutUint64(payload[6:14], math.Float64bits(result))
	binary.LittleEndian.PutUint16(payload[20:22], uint16(len(formula)))
	copy(payload[22:], formula)
	return payload
}

func oldNumberFormulaPayload(row, col int, xfIndex uint16, result float64, formula []byte, biff2 bool) []byte {
	if biff2 {
		payload := make([]byte, 18+len(formula))
		binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
		binary.LittleEndian.PutUint16(payload[2:4], uint16(col))
		binary.LittleEndian.PutUint64(payload[7:15], math.Float64bits(result))
		binary.LittleEndian.PutUint16(payload[16:18], uint16(len(formula)))
		copy(payload[18:], formula)
		return payload
	}
	payload := make([]byte, 18+len(formula))
	binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(col))
	binary.LittleEndian.PutUint16(payload[4:6], xfIndex)
	binary.LittleEndian.PutUint64(payload[6:14], math.Float64bits(result))
	binary.LittleEndian.PutUint16(payload[16:18], uint16(len(formula)))
	copy(payload[18:], formula)
	return payload
}

func oldStringFormulaPayload(row, col int, xfIndex uint16, formula []byte, biff2 bool) []byte {
	payload := oldNumberFormulaPayload(row, col, xfIndex, 0, formula, biff2)
	resultOffset := 6
	if biff2 {
		resultOffset = 7
	}
	binary.LittleEndian.PutUint16(payload[resultOffset+6:resultOffset+8], 0xFFFF)
	payload[resultOffset] = 0x00
	return payload
}

func emptyFormulaPayload(row, col int, xfIndex uint16, formula []byte) []byte {
	payload := make([]byte, 22+len(formula))
	binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(col))
	binary.LittleEndian.PutUint16(payload[4:6], xfIndex)
	binary.LittleEndian.PutUint16(payload[12:14], 0xFFFF)
	payload[6] = 0x03
	binary.LittleEndian.PutUint16(payload[20:22], uint16(len(formula)))
	copy(payload[22:], formula)
	return payload
}

func arrayFormulaPayload(firstRow, lastRow, firstCol, lastCol int, formula []byte) []byte {
	payload := arrayPayload(firstRow, lastRow, firstCol, lastCol)
	binary.LittleEndian.PutUint16(payload[12:14], uint16(len(formula)))
	return append(payload, formula...)
}

func sharedFormulaPayload(firstRow, lastRow, firstCol, lastCol int, formula []byte) []byte {
	payload := make([]byte, 10)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(firstRow))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(lastRow))
	payload[4] = byte(firstCol)
	payload[5] = byte(lastCol)
	binary.LittleEndian.PutUint16(payload[8:10], uint16(len(formula)))
	return append(payload, formula...)
}

func expFormulaPayload(anchorRow, anchorCol int) []byte {
	payload := make([]byte, 5)
	payload[0] = 0x01
	binary.LittleEndian.PutUint16(payload[1:3], uint16(anchorRow))
	binary.LittleEndian.PutUint16(payload[3:5], uint16(anchorCol))
	return payload
}

func numberCellPayload(row, col int, xfIndex uint16, value float64) []byte {
	payload := make([]byte, 14)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(col))
	binary.LittleEndian.PutUint16(payload[4:6], xfIndex)
	binary.LittleEndian.PutUint64(payload[6:14], math.Float64bits(value))
	return payload
}

func objPayload(objType, objID uint16) []byte {
	payload := make([]byte, 22)
	binary.LittleEndian.PutUint16(payload[0:2], 0x0015) // FtCmo.ft
	binary.LittleEndian.PutUint16(payload[2:4], 0x0012) // FtCmo.cb
	binary.LittleEndian.PutUint16(payload[4:6], objType)
	binary.LittleEndian.PutUint16(payload[6:8], objID)
	return payload
}

func txoPayload(charCount uint16) []byte {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint16(payload[4:6], charCount)
	return payload
}

func txoPayloadWithLink(charCount uint16, formula []byte) []byte {
	payload := make([]byte, 24+len(formula))
	binary.LittleEndian.PutUint16(payload[10:12], charCount)
	binary.LittleEndian.PutUint16(payload[18:20], uint16(len(formula)))
	copy(payload[24:], formula)
	return payload
}

func legacyControlDrawing(shapes ...[]byte) []byte {
	return bytes.Join(shapes, nil)
}

func legacyControlShape(fromRow, fromCol, toRow, toCol int) []byte {
	spBody := make([]byte, 8)
	anchorBody := make([]byte, 18)
	binary.LittleEndian.PutUint16(anchorBody[2:4], uint16(fromCol-1))
	binary.LittleEndian.PutUint16(anchorBody[6:8], uint16(fromRow-1))
	binary.LittleEndian.PutUint16(anchorBody[10:12], uint16(toCol-1))
	binary.LittleEndian.PutUint16(anchorBody[14:16], uint16(toRow-1))
	return officeArtRecord(legacyOfficeArtSpContainer, 0, 0x000F,
		append(
			officeArtRecord(legacyOfficeArtSp, 2, 0x0002, spBody),
			officeArtRecord(legacyOfficeArtClientAnchor, 0, 0x0000, anchorBody)...,
		),
	)
}

func legacyOfficeArtShape(shapeType uint16, fromRow, fromCol, toRow, toCol int, fillColorRaw, lineColorRaw, lineWidth uint32) []byte {
	spBody := make([]byte, 8)
	anchorBody := make([]byte, 18)
	binary.LittleEndian.PutUint16(anchorBody[2:4], uint16(fromCol-1))
	binary.LittleEndian.PutUint16(anchorBody[6:8], uint16(fromRow-1))
	binary.LittleEndian.PutUint16(anchorBody[10:12], uint16(toCol-1))
	binary.LittleEndian.PutUint16(anchorBody[14:16], uint16(toRow-1))

	props := make([]byte, 0, 18)
	if fillColorRaw != 0 {
		props = append(props, officeArtProperty(legacyOfficeArtPropFillColor, fillColorRaw)...)
	}
	if lineColorRaw != 0 {
		props = append(props, officeArtProperty(legacyOfficeArtPropLineColor, lineColorRaw)...)
	}
	if lineWidth != 0 {
		props = append(props, officeArtProperty(legacyOfficeArtPropLineWidth, lineWidth)...)
	}
	return officeArtRecord(legacyOfficeArtSpContainer, 0, 0x000F,
		append(
			append(
				officeArtRecord(legacyOfficeArtSp, shapeType, 0x0002, spBody),
				officeArtRecord(legacyOfficeArtFOPT, uint16(len(props)/6), 0x0003, props)...,
			),
			officeArtRecord(legacyOfficeArtClientAnchor, 0, 0x0000, anchorBody)...,
		),
	)
}

func legacyOfficeArtShapeWithProps(shapeType uint16, fromRow, fromCol, toRow, toCol int, props []byte) []byte {
	spBody := make([]byte, 8)
	anchorBody := make([]byte, 18)
	binary.LittleEndian.PutUint16(anchorBody[2:4], uint16(fromCol-1))
	binary.LittleEndian.PutUint16(anchorBody[6:8], uint16(fromRow-1))
	binary.LittleEndian.PutUint16(anchorBody[10:12], uint16(toCol-1))
	binary.LittleEndian.PutUint16(anchorBody[14:16], uint16(toRow-1))
	return officeArtRecord(legacyOfficeArtSpContainer, 0, 0x000F,
		append(
			append(
				officeArtRecord(legacyOfficeArtSp, shapeType, 0x0002, spBody),
				officeArtRecord(legacyOfficeArtFOPT, uint16(len(props)/6), 0x0003, props)...,
			),
			officeArtRecord(legacyOfficeArtClientAnchor, 0, 0x0000, anchorBody)...,
		),
	)
}

func officeArtProperty(opid uint16, value uint32) []byte {
	payload := make([]byte, 6)
	binary.LittleEndian.PutUint16(payload[0:2], opid)
	binary.LittleEndian.PutUint32(payload[2:6], value)
	return payload
}

func officeArtRecord(recType, inst, version uint16, body []byte) []byte {
	record := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint16(record[0:2], (inst<<4)|(version&0x000F))
	binary.LittleEndian.PutUint16(record[2:4], recType)
	binary.LittleEndian.PutUint32(record[4:8], uint32(len(body)))
	copy(record[8:], body)
	return record
}

func objPayloadWithLbsData(objType, objID uint16, lbsData []byte) []byte {
	payload := objPayload(objType, objID)
	if len(lbsData) == 0 {
		return payload
	}
	return appendObjSubRecord(payload, 0x0013, lbsData)
}

func objPayloadWithLinkFormulaAndLbsData(objType, objID uint16, linkFormula, lbsData []byte) []byte {
	payload := objPayload(objType, objID)
	if len(linkFormula) > 0 {
		payload = appendObjSubRecord(payload, 0x000E, linkFormula)
	}
	if len(lbsData) > 0 {
		payload = appendObjSubRecord(payload, 0x0013, lbsData)
	}
	return payload
}

func objPayloadWithCheckboxData(objType, objID uint16, checkBoxData []byte) []byte {
	payload := objPayload(objType, objID)
	if len(checkBoxData) == 0 {
		return payload
	}
	return appendObjSubRecord(payload, 0x0012, checkBoxData)
}

func objPayloadWithSbsData(objType, objID uint16, sbsData []byte) []byte {
	payload := objPayload(objType, objID)
	if len(sbsData) == 0 {
		return payload
	}
	return appendObjSubRecord(payload, 0x000C, sbsData)
}

func appendObjSubRecord(payload []byte, ft uint16, data []byte) []byte {
	record := make([]byte, 4+len(data))
	binary.LittleEndian.PutUint16(record[0:2], ft)
	binary.LittleEndian.PutUint16(record[2:4], uint16(len(data)))
	copy(record[4:], data)
	return append(payload, record...)
}

func checkboxDataPayload(state uint16) []byte {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint16(payload[0:2], state)
	return payload
}

func sbsDataPayload(value, min, max int) []byte {
	payload := make([]byte, 20)
	binary.LittleEndian.PutUint16(payload[4:6], uint16(int16(value)))
	binary.LittleEndian.PutUint16(payload[6:8], uint16(int16(min)))
	binary.LittleEndian.PutUint16(payload[8:10], uint16(int16(max)))
	return payload
}

func objLinkFormulaPayload(formula []byte) []byte {
	payload := make([]byte, 8+len(formula))
	binary.LittleEndian.PutUint16(payload[0:2], uint16(len(formula)+6))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(len(formula)))
	copy(payload[8:], formula)
	return payload
}

func lbsDataPayloadWithRangeAndSelection(formula []byte, lineCount, selected int, includeDropData bool) []byte {
	return lbsDataPayloadWithSelectionModeAndEntries(formula, lineCount, selected, "single", nil, includeDropData)
}

func lbsDataPayloadWithSelectionModeAndEntries(formula []byte, lineCount, selected int, selectionMode string, selectedEntries []int, includeDropData bool) []byte {
	payload := make([]byte, 0, 32+len(formula))
	if len(formula) > 0 {
		header := make([]byte, 8)
		binary.LittleEndian.PutUint16(header[0:2], uint16(len(formula)+6))
		binary.LittleEndian.PutUint16(header[2:4], uint16(len(formula)))
		payload = append(payload, header...)
		payload = append(payload, formula...)
	} else {
		payload = append(payload, 0x00, 0x00)
	}
	state := make([]byte, 8)
	binary.LittleEndian.PutUint16(state[0:2], uint16(lineCount))
	binary.LittleEndian.PutUint16(state[2:4], uint16(selected))
	switch selectionMode {
	case "multi":
		binary.LittleEndian.PutUint16(state[4:6], 0x0010)
	case "extended":
		binary.LittleEndian.PutUint16(state[4:6], 0x0020)
	}
	payload = append(payload, state...)
	if includeDropData {
		dropData := []byte{
			0x02, 0x00, // simple dropdown
			0x08, 0x00, // 8 lines
			0x00, 0x00, // dxMin
			0x00, 0x00, // cch
			0x00, // fHighByte
			0x00, // pad to even boundary
		}
		payload = append(payload, dropData...)
	}
	if selectionMode == "multi" || selectionMode == "extended" {
		bsels := make([]byte, lineCount)
		for _, entry := range selectedEntries {
			if entry <= 0 || entry > lineCount {
				continue
			}
			bsels[entry-1] = 1
		}
		payload = append(payload, bsels...)
	}
	return payload
}

func ptgRefFormula(rowZeroBased, colZeroBased int) []byte {
	return []byte{
		0x24,
		byte(rowZeroBased),
		byte(rowZeroBased >> 8),
		byte(colZeroBased),
		byte(colZeroBased >> 8),
	}
}

func ptgAreaFormula(firstRowZeroBased, lastRowZeroBased, firstColZeroBased, lastColZeroBased int) []byte {
	return []byte{
		0x25,
		byte(firstRowZeroBased), byte(firstRowZeroBased >> 8),
		byte(lastRowZeroBased), byte(lastRowZeroBased >> 8),
		byte(firstColZeroBased), byte(firstColZeroBased >> 8),
		byte(lastColZeroBased), byte(lastColZeroBased >> 8),
	}
}

func notePayload(row, col int, objID uint16, author string) []byte {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(col))
	binary.LittleEndian.PutUint16(payload[6:8], objID)
	authorBytes := append([]byte{byte(len(author)), 0x00}, []byte(author)...)
	return append(payload, authorBytes...)
}

func makeFontPayload(heightTwips uint16, bold bool, italic bool, underline bool, strike bool, name string) []byte {
	bls := uint16(400)
	if bold {
		bls = 700
	}
	grbit := uint16(0)
	if italic {
		grbit |= 0x0002
	}
	if strike {
		grbit |= 0x0008
	}
	uls := uint8(0)
	if underline {
		uls = 1
	}
	p := make([]byte, 16+len(name))
	binary.LittleEndian.PutUint16(p[0:2], heightTwips)
	binary.LittleEndian.PutUint16(p[2:4], grbit)
	binary.LittleEndian.PutUint16(p[4:6], 0x7FFF) // automatic color
	binary.LittleEndian.PutUint16(p[6:8], bls)
	p[10] = uls
	p[14] = byte(len(name))
	p[15] = 0 // Latin-1
	copy(p[16:], name)
	return p
}

func makeXFPayload(ifnt uint16, ifmt uint16, halign uint8, valign uint8, wrap bool, fillPattern uint8, foreIdx uint16, backIdx uint16) []byte {
	p := make([]byte, 20)
	binary.LittleEndian.PutUint16(p[0:2], ifnt)
	binary.LittleEndian.PutUint16(p[2:4], ifmt)
	alc := halign & 0x07
	if wrap {
		alc |= 0x08
	}
	alc |= (valign & 0x07) << 4
	p[6] = alc
	// fill pattern in bits 26-31 of uint32 at offset 14
	b2 := uint32(fillPattern) << 26
	binary.LittleEndian.PutUint32(p[14:18], b2)
	// foreground/background in uint16 at offset 18
	colors := uint16(foreIdx&0x7F) | (uint16(backIdx&0x7F) << 7)
	binary.LittleEndian.PutUint16(p[18:20], colors)
	return p
}

func TestParseBIFFFontExtractsProperties(t *testing.T) {
	payload := makeFontPayload(240, true, true, true, false, "Arial")
	font := parseBIFFFont(payload)
	if font.Name != "Arial" {
		t.Fatalf("Name = %q, want Arial", font.Name)
	}
	if !font.Bold {
		t.Fatalf("Bold = false, want true")
	}
	if !font.Italic {
		t.Fatalf("Italic = false, want true")
	}
	if !font.Underline {
		t.Fatalf("Underline = false, want true")
	}
	wantPt := 240.0 / 20.0
	if font.HeightPt != wantPt {
		t.Fatalf("HeightPt = %v, want %v", font.HeightPt, wantPt)
	}
}

func TestParseBIFFXFResolvesFont(t *testing.T) {
	fonts := []legacyFont{
		parseBIFFFont(makeFontPayload(200, false, false, false, false, "Calibri")),
		parseBIFFFont(makeFontPayload(240, true, true, false, false, "Arial")),
	}
	fmts := defaultBIFFFormatCodes()
	// ifnt=1 → fonts[1] = Arial bold italic
	xf := parseBIFFXF(makeXFPayload(1, 0, 2, 0, false, 0, 0, 0), fmts, fonts, defaultBIFFPalette)
	if xf.FontName != "Arial" {
		t.Fatalf("FontName = %q, want Arial", xf.FontName)
	}
	if !xf.Bold {
		t.Fatalf("Bold = false, want true")
	}
	if !xf.Italic {
		t.Fatalf("Italic = false, want true")
	}
	if xf.HAlign != "center" {
		t.Fatalf("HAlign = %q, want center", xf.HAlign)
	}
}

func TestParseBIFFXFResolvesFillColor(t *testing.T) {
	fmts := defaultBIFFFormatCodes()
	// fillPattern=1 (solid), foreIdx=0x0A (red)
	xf := parseBIFFXF(makeXFPayload(0, 0, 0, 0, false, 1, 0x0A, 0x09), fmts, nil, defaultBIFFPalette)
	if xf.BgColor != "FF0000" {
		t.Fatalf("BgColor = %q, want FF0000 (red solid fill)", xf.BgColor)
	}
}

func TestStyleFromXFIndexPopulatesAllFields(t *testing.T) {
	fonts := []legacyFont{
		parseBIFFFont(makeFontPayload(280, true, false, true, false, "Times New Roman")),
	}
	fmts := defaultBIFFFormatCodes()
	xf := parseBIFFXF(makeXFPayload(0, 0, 1, 2, true, 0, 0, 0), fmts, fonts, defaultBIFFPalette)
	xfs := []legacyXF{xf}
	style := styleFromXFIndex(0, xfs)
	if !style.Bold {
		t.Fatalf("Bold = false, want true")
	}
	if style.FontFamily != "Times New Roman" {
		t.Fatalf("FontFamily = %q, want Times New Roman", style.FontFamily)
	}
	if !style.Underline {
		t.Fatalf("Underline = false, want true")
	}
	if style.HAlign != "left" {
		t.Fatalf("HAlign = %q, want left", style.HAlign)
	}
	if !style.WrapText {
		t.Fatalf("WrapText = false, want true")
	}
	wantPt := 280.0 / 20.0
	if style.FontSizePt != wantPt {
		t.Fatalf("FontSizePt = %v, want %v", style.FontSizePt, wantPt)
	}
}

// --- Text rotation ---

func TestParseBIFFXFTextRotation(t *testing.T) {
	fmts := defaultBIFFFormatCodes()
	// Build a 20-byte XF payload; trot is at byte 8.
	p := make([]byte, 20)
	p[8] = 45 // 45° CCW
	xf := parseBIFFXF(p, fmts, nil, defaultBIFFPalette)
	if xf.TextRotation != 45 {
		t.Fatalf("TextRotation = %d, want 45", xf.TextRotation)
	}
	style := styleFromXFIndex(0, []legacyXF{xf})
	if style.TextRotation != 45 {
		t.Fatalf("style.TextRotation = %d, want 45", style.TextRotation)
	}
}

func TestParseBIFFXFTextRotationZeroIgnored(t *testing.T) {
	fmts := defaultBIFFFormatCodes()
	p := make([]byte, 20) // all zeros
	xf := parseBIFFXF(p, fmts, nil, defaultBIFFPalette)
	if xf.TextRotation != 0 {
		t.Fatalf("TextRotation = %d, want 0 for horizontal text", xf.TextRotation)
	}
}

func TestParseBIFFXFTextRotationStacked(t *testing.T) {
	fmts := defaultBIFFFormatCodes()
	p := make([]byte, 20)
	p[8] = 255 // stacked
	xf := parseBIFFXF(p, fmts, nil, defaultBIFFPalette)
	if xf.TextRotation != 255 {
		t.Fatalf("TextRotation = %d, want 255 for stacked text", xf.TextRotation)
	}
}

// --- Conditional formatting ---

func TestBiffRef8UToSqRef(t *testing.T) {
	// Ref8U: firstRow=1, lastRow=3, firstCol=1, lastCol=2 (0-based) → B2:C4
	b := make([]byte, 8)
	binary.LittleEndian.PutUint16(b[0:2], 1) // firstRow=1 → row 2
	binary.LittleEndian.PutUint16(b[2:4], 3) // lastRow=3  → row 4
	binary.LittleEndian.PutUint16(b[4:6], 1) // firstCol=1 → col B
	binary.LittleEndian.PutUint16(b[6:8], 2) // lastCol=2  → col C
	got := biffRef8UToSqRef(b)
	if got != "B2:C4" {
		t.Fatalf("biffRef8UToSqRef = %q, want B2:C4", got)
	}
}

func TestParseCONDFMTSqRef(t *testing.T) {
	// Build a minimal CONDFMT payload: cCF=2, fToughRecalc=0, refBound (8 bytes),
	// ccell=1, one Ref8U for A1:A5.
	p := make([]byte, 22)
	binary.LittleEndian.PutUint16(p[0:2], 2)   // cCF=2
	binary.LittleEndian.PutUint16(p[12:14], 1) // ccell=1
	// Ref8U at offset 14: rows 0–4, cols 0–0 → A1:A5
	binary.LittleEndian.PutUint16(p[14:16], 0)
	binary.LittleEndian.PutUint16(p[16:18], 4)
	binary.LittleEndian.PutUint16(p[18:20], 0)
	binary.LittleEndian.PutUint16(p[20:22], 0)
	sqref, count := parseCONDFMTSqRef(p)
	if count != 2 {
		t.Fatalf("cfCount = %d, want 2", count)
	}
	if sqref != "A1:A5" {
		t.Fatalf("sqref = %q, want A1:A5", sqref)
	}
}

func TestParseCFRecordCellIs(t *testing.T) {
	// ct=1 (cellIs), op=3 (equal), cbFormula1=2, cbFormula2=0, 22-byte DxfN, 2 formula bytes.
	p := make([]byte, 30)
	p[0] = 1                                 // ct=cellIs
	p[1] = 3                                 // op=equal
	binary.LittleEndian.PutUint16(p[2:4], 2) // cbFormula1=2
	binary.LittleEndian.PutUint16(p[4:6], 0) // cbFormula2=0
	// 22 bytes DxfN at p[6..27]
	p[28] = 0x1E // formula byte 1
	p[29] = 0x00 // formula byte 2
	rule := parseCFRecord(p)
	if rule == nil {
		t.Fatal("parseCFRecord returned nil")
	}
	if rule.Type != "cellIs" {
		t.Fatalf("Type = %q, want cellIs", rule.Type)
	}
	if rule.Operator != "equal" {
		t.Fatalf("Operator = %q, want equal", rule.Operator)
	}
	if len(rule.Formula) != 1 {
		t.Fatalf("Formula len = %d, want 1", len(rule.Formula))
	}
}

func TestParseCONDFMT12SqRef(t *testing.T) {
	inner := make([]byte, 22)
	binary.LittleEndian.PutUint16(inner[0:2], 1)   // cCF
	binary.LittleEndian.PutUint16(inner[12:14], 1) // ccell
	// Ref8U A1:B2
	binary.LittleEndian.PutUint16(inner[14:16], 0)
	binary.LittleEndian.PutUint16(inner[16:18], 1)
	binary.LittleEndian.PutUint16(inner[18:20], 0)
	binary.LittleEndian.PutUint16(inner[20:22], 1)
	payload := append(make([]byte, 12), inner...)
	sqref, count := parseCONDFMT12SqRef(payload)
	if count != 1 {
		t.Fatalf("cfCount = %d, want 1", count)
	}
	if sqref != "A1:B2" {
		t.Fatalf("sqref = %q, want A1:B2", sqref)
	}
}

func TestParseCF12RecordTypeMappings(t *testing.T) {
	mk := func(ct byte) []byte {
		p := make([]byte, 24)
		p[12] = ct // condition type after 12-byte future header
		// ext_formatting_length = 0, reserved at 22:24 already zero
		return p
	}
	cases := []struct {
		ct   byte
		want string
	}{
		{1, "cellIs"},
		{2, "expression"},
		{3, "colorScale"},
		{4, "dataBar"},
		{5, "filter"},
		{6, "iconSet"},
	}
	for _, c := range cases {
		rule := parseCF12Record(mk(c.ct), nil)
		if rule == nil {
			t.Fatalf("ct=%d: parseCF12Record returned nil", c.ct)
		}
		if rule.Type != c.want {
			t.Fatalf("ct=%d: Type = %q, want %q", c.ct, rule.Type, c.want)
		}
	}
}

func TestParseWorksheetCapturesCF12Rules(t *testing.T) {
	// CFHEADER12: 12-byte future header + CONDFMT body with cCF=1 and sqref A1:A1.
	headerBody := make([]byte, 22)
	binary.LittleEndian.PutUint16(headerBody[0:2], 1)   // cCF
	binary.LittleEndian.PutUint16(headerBody[12:14], 1) // ccell
	// Ref8U A1:A1
	binary.LittleEndian.PutUint16(headerBody[14:16], 0)
	binary.LittleEndian.PutUint16(headerBody[16:18], 0)
	binary.LittleEndian.PutUint16(headerBody[18:20], 0)
	binary.LittleEndian.PutUint16(headerBody[20:22], 0)
	cfHeader12 := append(make([]byte, 12), headerBody...)

	// CFRULE12 with type colorScale (ct=3), minimal extFmtLen=0 shape.
	cfRule12 := make([]byte, 24)
	cfRule12[12] = 3

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffCFHeader12, cfHeader12)
	data = appendBiffRecord(data, biffCFRule12, cfRule12)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "CF12"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.CFRules) != 1 {
		t.Fatalf("len(CFRules) = %d, want 1", len(sheet.CFRules))
	}
	if sheet.CFRules[0].SqRef != "A1" {
		t.Fatalf("sqref = %q, want A1", sheet.CFRules[0].SqRef)
	}
	if sheet.CFRules[0].Type != "colorScale" {
		t.Fatalf("type = %q, want colorScale", sheet.CFRules[0].Type)
	}
}

// TestParseWorksheetCapturesCFRules verifies that a basic CONDFMT + CF record
// pair is captured in the worksheet walker (the non-CF12 path).
func TestParseWorksheetCapturesCFRules(t *testing.T) {
	// CONDFMT with cCF=1, sqref A1:B2.
	headerBody := make([]byte, 22)
	binary.LittleEndian.PutUint16(headerBody[0:2], 1)   // cCF
	binary.LittleEndian.PutUint16(headerBody[12:14], 1) // ccell
	binary.LittleEndian.PutUint16(headerBody[14:16], 0) // firstRow
	binary.LittleEndian.PutUint16(headerBody[16:18], 1) // lastRow
	binary.LittleEndian.PutUint16(headerBody[18:20], 0) // firstCol
	binary.LittleEndian.PutUint16(headerBody[20:22], 1) // lastCol

	// CF record with ct=1 (cellIs), op=1 (between), cbFormula1=2, cbFormula2=2, 22-byte DxfN, 4 formula bytes.
	cfPayload := make([]byte, 32)
	cfPayload[0] = 1                                 // ct=cellIs
	cfPayload[1] = 1                                 // op=between
	binary.LittleEndian.PutUint16(cfPayload[2:4], 2) // cbFormula1=2
	binary.LittleEndian.PutUint16(cfPayload[4:6], 2) // cbFormula2=2
	// 22 bytes DxfN at cfPayload[6..27]
	cfPayload[28] = 0x1E // formula1 byte
	cfPayload[29] = 0x00
	cfPayload[30] = 0x2E // formula2 byte
	cfPayload[31] = 0x00

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffCondFmt, headerBody)
	data = appendBiffRecord(data, biffCF, cfPayload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "CF"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.CFRules) != 1 {
		t.Fatalf("len(CFRules) = %d, want 1", len(sheet.CFRules))
	}
	if sheet.CFRules[0].SqRef != "A1:B2" {
		t.Fatalf("sqref = %q, want A1:B2", sheet.CFRules[0].SqRef)
	}
	if sheet.CFRules[0].Type != "cellIs" {
		t.Fatalf("type = %q, want cellIs", sheet.CFRules[0].Type)
	}
	if sheet.CFRules[0].Operator != "between" {
		t.Fatalf("operator = %q, want between", sheet.CFRules[0].Operator)
	}
}

// TestParseWorksheetCapturesMultipleCONDFMTBlocks verifies that multiple
// CONDFMT blocks with different sqrefs each produce their own CF rules.
func TestParseWorksheetCapturesMultipleCONDFMTBlocks(t *testing.T) {
	// CONDFMT #1: cCF=1, sqref A1:A1.
	h1 := make([]byte, 22)
	binary.LittleEndian.PutUint16(h1[0:2], 1)   // cCF
	binary.LittleEndian.PutUint16(h1[12:14], 1) // ccell
	binary.LittleEndian.PutUint16(h1[14:16], 0) // firstRow
	binary.LittleEndian.PutUint16(h1[16:18], 0) // lastRow
	binary.LittleEndian.PutUint16(h1[18:20], 0) // firstCol
	binary.LittleEndian.PutUint16(h1[20:22], 0) // lastCol

	// CONDFMT #2: cCF=1, sqref C3:D4.
	h2 := make([]byte, 22)
	binary.LittleEndian.PutUint16(h2[0:2], 1)   // cCF
	binary.LittleEndian.PutUint16(h2[12:14], 1) // ccell
	binary.LittleEndian.PutUint16(h2[14:16], 2) // firstRow
	binary.LittleEndian.PutUint16(h2[16:18], 3) // lastRow
	binary.LittleEndian.PutUint16(h2[18:20], 2) // firstCol
	binary.LittleEndian.PutUint16(h2[20:22], 3) // lastCol

	// CF record: ct=1 (cellIs), op=3 (equal), cbFormula1=2, cbFormula2=0.
	cf1 := make([]byte, 30)
	cf1[0] = 1
	cf1[1] = 3
	binary.LittleEndian.PutUint16(cf1[2:4], 2)
	binary.LittleEndian.PutUint16(cf1[4:6], 0)
	cf1[28] = 0x1E
	cf1[29] = 0x00

	// CF record #2: ct=2 (expression), cbFormula1=2, cbFormula2=0.
	cf2 := make([]byte, 30)
	cf2[0] = 2
	binary.LittleEndian.PutUint16(cf2[2:4], 2)
	binary.LittleEndian.PutUint16(cf2[4:6], 0)
	cf2[28] = 0x2E
	cf2[29] = 0x00

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffCondFmt, h1)
	data = appendBiffRecord(data, biffCF, cf1)
	data = appendBiffRecord(data, biffCondFmt, h2)
	data = appendBiffRecord(data, biffCF, cf2)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "MultiCF"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.CFRules) != 2 {
		t.Fatalf("len(CFRules) = %d, want 2", len(sheet.CFRules))
	}
	if sheet.CFRules[0].SqRef != "A1" {
		t.Fatalf("rule[0] sqref = %q, want A1", sheet.CFRules[0].SqRef)
	}
	if sheet.CFRules[0].Type != "cellIs" {
		t.Fatalf("rule[0] type = %q, want cellIs", sheet.CFRules[0].Type)
	}
	if sheet.CFRules[1].SqRef != "C3:D4" {
		t.Fatalf("rule[1] sqref = %q, want C3:D4", sheet.CFRules[1].SqRef)
	}
	if sheet.CFRules[1].Type != "expression" {
		t.Fatalf("rule[1] type = %q, want expression", sheet.CFRules[1].Type)
	}
}

// TestParseWorksheetIgnoresExtraCFRecords verifies that CF records beyond the
// count declared in CONDFMT are silently ignored (matching POI's behavior).
func TestParseWorksheetIgnoresExtraCFRecords(t *testing.T) {
	// CONDFMT with cCF=1, sqref A1:A1.
	headerBody := make([]byte, 22)
	binary.LittleEndian.PutUint16(headerBody[0:2], 1)   // cCF
	binary.LittleEndian.PutUint16(headerBody[12:14], 1) // ccell
	binary.LittleEndian.PutUint16(headerBody[14:16], 0)
	binary.LittleEndian.PutUint16(headerBody[16:18], 0)
	binary.LittleEndian.PutUint16(headerBody[18:20], 0)
	binary.LittleEndian.PutUint16(headerBody[20:22], 0)

	// CF record #1: ct=1 (cellIs), op=3 (equal).
	cf1 := make([]byte, 30)
	cf1[0] = 1
	cf1[1] = 3
	binary.LittleEndian.PutUint16(cf1[2:4], 2)
	binary.LittleEndian.PutUint16(cf1[4:6], 0)
	cf1[28] = 0x1E
	cf1[29] = 0x00

	// CF record #2: extra — should be ignored since cCF=1.
	cf2 := make([]byte, 30)
	cf2[0] = 1
	cf2[1] = 3
	binary.LittleEndian.PutUint16(cf2[2:4], 2)
	binary.LittleEndian.PutUint16(cf2[4:6], 0)
	cf2[28] = 0x3E
	cf2[29] = 0x00

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffCondFmt, headerBody)
	data = appendBiffRecord(data, biffCF, cf1)
	data = appendBiffRecord(data, biffCF, cf2) // extra — should be ignored
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "ExtraCF"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.CFRules) != 1 {
		t.Fatalf("len(CFRules) = %d, want 1 (extra CF record should be ignored)", len(sheet.CFRules))
	}
}

// TestParseWorksheetRejectsCFTypeMismatch verifies that a CF rule under a
// CF12 header (or vice versa) is rejected, matching POI's checkRuleType enforcement.
func TestParseWorksheetRejectsCFTypeMismatch(t *testing.T) {
	// CFHEADER12 with cCF=1, sqref A1:A1.
	headerBody := make([]byte, 22)
	binary.LittleEndian.PutUint16(headerBody[0:2], 1)   // cCF
	binary.LittleEndian.PutUint16(headerBody[12:14], 1) // ccell
	binary.LittleEndian.PutUint16(headerBody[14:16], 0)
	binary.LittleEndian.PutUint16(headerBody[16:18], 0)
	binary.LittleEndian.PutUint16(headerBody[18:20], 0)
	binary.LittleEndian.PutUint16(headerBody[20:22], 0)
	cfHeader12 := append(make([]byte, 12), headerBody...)

	// CF record (not CF12) — should be rejected under CF12 header.
	cfPayload := make([]byte, 30)
	cfPayload[0] = 1
	cfPayload[1] = 3
	binary.LittleEndian.PutUint16(cfPayload[2:4], 2)
	binary.LittleEndian.PutUint16(cfPayload[4:6], 0)
	cfPayload[28] = 0x1E
	cfPayload[29] = 0x00

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffCFHeader12, cfHeader12)
	data = appendBiffRecord(data, biffCF, cfPayload) // CF under CF12 header — should be rejected
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Mismatch"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.CFRules) != 0 {
		t.Fatalf("len(CFRules) = %d, want 0 (CF rule under CF12 header should be rejected)", len(sheet.CFRules))
	}
}

// TestParseCFRecordExpression verifies that a CF record with ct=2 (expression)
// is decoded correctly.
func TestParseCFRecordExpression(t *testing.T) {
	p := make([]byte, 30)
	p[0] = 2                                 // ct=expression
	binary.LittleEndian.PutUint16(p[2:4], 2) // cbFormula1=2
	binary.LittleEndian.PutUint16(p[4:6], 0) // cbFormula2=0
	// 22 bytes DxfN at p[6..27]
	p[28] = 0x1E // formula byte 1
	p[29] = 0x00 // formula byte 2
	rule := parseCFRecord(p)
	if rule == nil {
		t.Fatal("parseCFRecord returned nil")
	}
	if rule.Type != "expression" {
		t.Fatalf("Type = %q, want expression", rule.Type)
	}
	if rule.Operator != "" {
		t.Fatalf("Operator = %q, want empty for expression type", rule.Operator)
	}
	if len(rule.Formula) != 1 {
		t.Fatalf("Formula len = %d, want 1", len(rule.Formula))
	}
}

// TestParseCF12RecordColorScale verifies that a CF12 record with ct=3 (colorScale)
// is decoded correctly.
func TestParseCF12RecordColorScale(t *testing.T) {
	p := make([]byte, 24)
	p[12] = 3 // ct=colorScale after 12-byte future header
	rule := parseCF12Record(p, nil)
	if rule == nil {
		t.Fatal("parseCF12Record returned nil")
	}
	if rule.Type != "colorScale" {
		t.Fatalf("Type = %q, want colorScale", rule.Type)
	}
}

// TestParseCF12RecordDataBar verifies that a CF12 record with ct=4 (dataBar)
// is decoded correctly.
func TestParseCF12RecordDataBar(t *testing.T) {
	p := make([]byte, 24)
	p[12] = 4 // ct=dataBar after 12-byte future header
	rule := parseCF12Record(p, nil)
	if rule == nil {
		t.Fatal("parseCF12Record returned nil")
	}
	if rule.Type != "dataBar" {
		t.Fatalf("Type = %q, want dataBar", rule.Type)
	}
}

// TestParseCF12RecordIconSet verifies that a CF12 record with ct=6 (iconSet)
// is decoded correctly.
func TestParseCF12RecordIconSet(t *testing.T) {
	p := make([]byte, 24)
	p[12] = 6 // ct=iconSet after 12-byte future header
	rule := parseCF12Record(p, nil)
	if rule == nil {
		t.Fatal("parseCF12Record returned nil")
	}
	if rule.Type != "iconSet" {
		t.Fatalf("Type = %q, want iconSet", rule.Type)
	}
}

// TestParseCF12RecordFilter verifies that a CF12 record with ct=5 (filter)
// is decoded correctly.
func TestParseCF12RecordFilter(t *testing.T) {
	p := make([]byte, 24)
	p[12] = 5 // ct=filter after 12-byte future header
	rule := parseCF12Record(p, nil)
	if rule == nil {
		t.Fatal("parseCF12Record returned nil")
	}
	if rule.Type != "filter" {
		t.Fatalf("Type = %q, want filter", rule.Type)
	}
}

// TestParseCFRecordShortPayload verifies that a CF record with a payload
// shorter than 28 bytes returns nil.
func TestParseCFRecordShortPayload(t *testing.T) {
	p := make([]byte, 27)
	rule := parseCFRecord(p)
	if rule != nil {
		t.Fatal("parseCFRecord should return nil for short payload")
	}
}

// TestParseCF12RecordShortPayload verifies that a CF12 record with a payload
// shorter than 22 bytes returns nil.
func TestParseCF12RecordShortPayload(t *testing.T) {
	p := make([]byte, 21)
	rule := parseCF12Record(p, nil)
	if rule != nil {
		t.Fatal("parseCF12Record should return nil for short payload")
	}
}

// TestParseCF12RecordUnknownType verifies that a CF12 record with an unknown
// condition type returns nil.
func TestParseCF12RecordUnknownType(t *testing.T) {
	p := make([]byte, 24)
	p[12] = 99 // unknown condition type
	rule := parseCF12Record(p, nil)
	if rule != nil {
		t.Fatal("parseCF12Record should return nil for unknown condition type")
	}
}

// TestParseCONDFMTSqRefShortPayload verifies that a CONDFMT payload shorter
// than 14 bytes returns empty strings.
func TestParseCONDFMTSqRefShortPayload(t *testing.T) {
	p := make([]byte, 13)
	sqref, count := parseCONDFMTSqRef(p)
	if sqref != "" || count != 0 {
		t.Fatalf("short payload: sqref=%q count=%d, want empty/0", sqref, count)
	}
}

// TestParseCONDFMT12SqRefShortPayload verifies that a CFHEADER12 payload
// shorter than 26 bytes returns empty strings.
func TestParseCONDFMT12SqRefShortPayload(t *testing.T) {
	p := make([]byte, 25)
	sqref, count := parseCONDFMT12SqRef(p)
	if sqref != "" || count != 0 {
		t.Fatalf("short payload: sqref=%q count=%d, want empty/0", sqref, count)
	}
}

// --- HLINK record ---

// encodeUTF16LE converts a Go string (including a trailing null if desired)
// to a UTF-16LE byte slice, one byte per rune (ASCII-only helper for tests).
// to a UTF-16LE byte slice, one byte per rune (ASCII-only helper for tests).
func encodeUTF16LE(s string) []byte {
	out := make([]byte, len(s)*2)
	for i, c := range s {
		out[i*2] = byte(c)
		out[i*2+1] = 0
	}
	return out
}

func TestParseHLINKRecordURLSavedAsStr(t *testing.T) {
	// Build a minimal HLINK payload:
	//   ref8       8 bytes (rows 0-0, cols 1-1)
	//   CLSID     16 bytes (zeros – unused)
	//   streamVer  4 bytes = 2
	//   hlstmf     4 bytes = HasMoniker | MonikerSavedAsStr
	//   count      4 bytes = number of wchars (including null terminator)
	//   url       len*2 bytes (UTF-16LE)
	const (
		hlstmfHasMoniker        = 0x0001
		hlstmfMonikerSavedAsStr = 0x0100
	)
	url := "https://example.com"
	urlWithNull := url + "\x00"
	urlUTF16 := encodeUTF16LE(urlWithNull)

	header := make([]byte, 32)
	binary.LittleEndian.PutUint16(header[0:2], 0) // firstRow
	binary.LittleEndian.PutUint16(header[2:4], 0) // lastRow
	binary.LittleEndian.PutUint16(header[4:6], 1) // firstCol
	binary.LittleEndian.PutUint16(header[6:8], 1) // lastCol
	// bytes 8-23: CLSID zeros (already zero)
	binary.LittleEndian.PutUint32(header[24:28], 2) // streamVersion
	binary.LittleEndian.PutUint32(header[28:32], hlstmfHasMoniker|hlstmfMonikerSavedAsStr)

	count := make([]byte, 4)
	binary.LittleEndian.PutUint32(count, uint32(len(urlWithNull)))

	payload := append(header, count...)
	payload = append(payload, urlUTF16...)

	hl, ok := parseHLINKRecord(payload)
	if !ok {
		t.Fatal("parseHLINKRecord returned ok=false")
	}
	if hl.url != url {
		t.Fatalf("url = %q, want %q", hl.url, url)
	}
	if hl.firstRow != 0 || hl.lastRow != 0 {
		t.Fatalf("row range = [%d..%d], want [0..0]", hl.firstRow, hl.lastRow)
	}
	if hl.firstCol != 1 || hl.lastCol != 1 {
		t.Fatalf("col range = [%d..%d], want [1..1]", hl.firstCol, hl.lastCol)
	}
}

func TestParseHLINKRecordURLMoniker(t *testing.T) {
	// URL moniker CLSID {79EAC9D0-BAF9-11CE-8C82-00AA004BA90B}
	clsidURL := []byte{0xD0, 0xC9, 0xEA, 0x79, 0xF9, 0xBA, 0xCE, 0x11, 0x8C, 0x82, 0x00, 0xAA, 0x00, 0x4B, 0xA9, 0x0B}
	url := "https://example.com/path"
	urlUTF16 := encodeUTF16LE(url)

	header := make([]byte, 32)
	binary.LittleEndian.PutUint16(header[0:2], 0)        // firstRow
	binary.LittleEndian.PutUint16(header[2:4], 0)        // lastRow
	binary.LittleEndian.PutUint16(header[4:6], 2)        // firstCol
	binary.LittleEndian.PutUint16(header[6:8], 2)        // lastCol
	binary.LittleEndian.PutUint32(header[24:28], 2)      // streamVersion
	binary.LittleEndian.PutUint32(header[28:32], 0x0001) // hlstmfHasMoniker

	payload := append(header, clsidURL...)
	cbSize := make([]byte, 4)
	binary.LittleEndian.PutUint32(cbSize, uint32(len(urlUTF16)))
	payload = append(payload, cbSize...)
	payload = append(payload, urlUTF16...)

	hl, ok := parseHLINKRecord(payload)
	if !ok {
		t.Fatal("parseHLINKRecord returned ok=false")
	}
	if hl.url != url {
		t.Fatalf("url = %q, want %q", hl.url, url)
	}
	if hl.firstCol != 2 || hl.lastCol != 2 {
		t.Fatalf("col range = [%d..%d], want [2..2]", hl.firstCol, hl.lastCol)
	}
}

func TestParseHLINKRecordFILEMoniker(t *testing.T) {
	// FILE moniker CLSID {00000303-0000-0000-C000-000000000046}
	clsidFILE := []byte{0x03, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}
	shortName := "C:\\path\\to\\file.xlsx"
	longName := "\\\\server\\share\\long\\path\\file.xlsx"

	header := make([]byte, 32)
	binary.LittleEndian.PutUint16(header[0:2], 0)        // firstRow
	binary.LittleEndian.PutUint16(header[2:4], 0)        // lastRow
	binary.LittleEndian.PutUint16(header[4:6], 3)        // firstCol
	binary.LittleEndian.PutUint16(header[6:8], 3)        // lastCol
	binary.LittleEndian.PutUint32(header[24:28], 2)      // streamVersion
	binary.LittleEndian.PutUint32(header[28:32], 0x0001) // hlstmfHasMoniker

	payload := append(header, clsidFILE...)

	// fileOpts (2 bytes)
	payload = append(payload, 0x00, 0x00)

	// cchShort (4 bytes) + short filename bytes (compressed)
	shortLen := len(shortName)
	cchShort := make([]byte, 4)
	binary.LittleEndian.PutUint32(cchShort, uint32(shortLen))
	payload = append(payload, cchShort...)
	payload = append(payload, []byte(shortName)...)

	// 24-byte tail (FILE_TAIL)
	tail := make([]byte, 24)
	tail[0] = 0xFF
	tail[1] = 0xFF
	tail[2] = 0xAD
	tail[3] = 0xDE
	payload = append(payload, tail...)

	// Optional long address: size(4) + charDataSize(4) + usKeyValue(2) + UTF-16LE chars
	longUTF16 := encodeUTF16LE(longName)
	charDataSize := len(longUTF16)
	addrSize := 4 + 4 + 2 + charDataSize // charDataSize + usKeyValue + size fields
	addrSizeField := make([]byte, 4)
	binary.LittleEndian.PutUint32(addrSizeField, uint32(addrSize))
	payload = append(payload, addrSizeField...)

	charDataSizeField := make([]byte, 4)
	binary.LittleEndian.PutUint32(charDataSizeField, uint32(charDataSize))
	payload = append(payload, charDataSizeField...)

	// usKeyValue = 3
	usKeyValue := make([]byte, 2)
	binary.LittleEndian.PutUint16(usKeyValue, 3)
	payload = append(payload, usKeyValue...)

	payload = append(payload, longUTF16...)

	hl, ok := parseHLINKRecord(payload)
	if !ok {
		t.Fatal("parseHLINKRecord returned ok=false")
	}
	// Should prefer the long address over the short filename.
	if hl.url != longName {
		t.Fatalf("url = %q, want %q", hl.url, longName)
	}
	if hl.firstCol != 3 || hl.lastCol != 3 {
		t.Fatalf("col range = [%d..%d], want [3..3]", hl.firstCol, hl.lastCol)
	}
}

func TestParseHLINKRecordSTDMoniker(t *testing.T) {
	// STD moniker CLSID {79EAC9F0-BAF9-11CE-8C82-00AA004BA90B}
	clsidSTD := []byte{0xF0, 0xC9, 0xEA, 0x79, 0xF9, 0xBA, 0xCE, 0x11, 0x8C, 0x82, 0x00, 0xAA, 0x00, 0x4B, 0xA9, 0x0B}
	path := "/path/to/document"

	header := make([]byte, 32)
	binary.LittleEndian.PutUint16(header[0:2], 0)        // firstRow
	binary.LittleEndian.PutUint16(header[2:4], 0)        // lastRow
	binary.LittleEndian.PutUint16(header[4:6], 4)        // firstCol
	binary.LittleEndian.PutUint16(header[6:8], 4)        // lastCol
	binary.LittleEndian.PutUint32(header[24:28], 2)      // streamVersion
	binary.LittleEndian.PutUint32(header[28:32], 0x0001) // hlstmfHasMoniker

	payload := append(header, clsidSTD...)

	// fileOpts (2 bytes)
	payload = append(payload, 0x00, 0x00)

	// pathLen (4 bytes) + UTF-8 path bytes
	pathLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(pathLen, uint32(len(path)))
	payload = append(payload, pathLen...)
	payload = append(payload, []byte(path)...)

	hl, ok := parseHLINKRecord(payload)
	if !ok {
		t.Fatal("parseHLINKRecord returned ok=false")
	}
	if hl.url != path {
		t.Fatalf("url = %q, want %q", hl.url, path)
	}
	if hl.firstCol != 4 || hl.lastCol != 4 {
		t.Fatalf("col range = [%d..%d], want [4..4]", hl.firstCol, hl.lastCol)
	}
}

func TestParseHLINKRecordUNCPath(t *testing.T) {
	// UNC path: hlstmfHasMoniker | hlstmfUNC, no CLSID, just count + UTF-16LE string.
	const (
		hlstmfHasMoniker = 0x0001
		hlstmfUNC        = 0x0200
	)
	uncPath := "\\\\server\\share\\folder"
	uncWithNull := uncPath + "\x00"
	uncUTF16 := encodeUTF16LE(uncWithNull)

	header := make([]byte, 32)
	binary.LittleEndian.PutUint16(header[0:2], 0)   // firstRow
	binary.LittleEndian.PutUint16(header[2:4], 0)   // lastRow
	binary.LittleEndian.PutUint16(header[4:6], 5)   // firstCol
	binary.LittleEndian.PutUint16(header[6:8], 5)   // lastCol
	binary.LittleEndian.PutUint32(header[24:28], 2) // streamVersion
	binary.LittleEndian.PutUint32(header[28:32], hlstmfHasMoniker|hlstmfUNC)

	count := make([]byte, 4)
	binary.LittleEndian.PutUint32(count, uint32(len(uncWithNull)))

	payload := append(header, count...)
	payload = append(payload, uncUTF16...)

	hl, ok := parseHLINKRecord(payload)
	if !ok {
		t.Fatal("parseHLINKRecord returned ok=false")
	}
	if hl.url != uncPath {
		t.Fatalf("url = %q, want %q", hl.url, uncPath)
	}
	if hl.firstCol != 5 || hl.lastCol != 5 {
		t.Fatalf("col range = [%d..%d], want [5..5]", hl.firstCol, hl.lastCol)
	}
}

func TestParseBIFFNameWorkbookScope(t *testing.T) {
	// Build a minimal NAME record for workbook-scoped name "SalesTax".
	// grbit=0x0000 (not hidden, not built-in), chKey=0, cch=8, cce=4, reserved=0, itab=0, reserved2=0000
	// Then fHighByte=0 + 8 ASCII chars + 4 formula bytes.
	payload := make([]byte, 14+1+8+4)
	binary.LittleEndian.PutUint16(payload[0:2], 0x0000) // grbit
	payload[2] = 0                                      // chKey
	payload[3] = 8                                      // cch
	binary.LittleEndian.PutUint16(payload[4:6], 4)      // cce
	// bytes 6-9: reserved
	binary.LittleEndian.PutUint16(payload[8:10], 0) // itab=0 (workbook scope)
	// bytes 10-13: reserved
	payload[14] = 0 // fHighByte=0 (single-byte chars)
	copy(payload[15:23], "SalesTax")
	// formula bytes: 0x3E 0x00 0x01 0x00 (simple ref — ignored)
	payload[23] = 0x3E
	payload[24] = 0x00
	payload[25] = 0x01
	payload[26] = 0x00

	dn, ok := parseBIFFName(payload)
	if !ok {
		t.Fatal("parseBIFFName returned ok=false")
	}
	if dn.Name != "SalesTax" {
		t.Fatalf("Name = %q, want SalesTax", dn.Name)
	}
	if dn.LocalSheetID != nil {
		t.Fatalf("LocalSheetID = %v, want nil (workbook scope)", dn.LocalSheetID)
	}
	if dn.Hidden {
		t.Fatal("Hidden = true, want false")
	}
	// Value should be the hex encoding of the 4 formula bytes.
	if dn.Value != "3e000100" {
		t.Fatalf("Value = %q, want 3e000100", dn.Value)
	}
}

func TestParseBIFFNameSheetScope(t *testing.T) {
	// itab=2 → LocalSheetID should be 1 (0-based).
	payload := make([]byte, 14+1+3)
	payload[3] = 3                                  // cch
	binary.LittleEndian.PutUint16(payload[8:10], 2) // itab=2
	payload[14] = 0                                 // fHighByte=0
	copy(payload[15:18], "Foo")

	dn, ok := parseBIFFName(payload)
	if !ok {
		t.Fatal("parseBIFFName returned ok=false")
	}
	if dn.Name != "Foo" {
		t.Fatalf("Name = %q, want Foo", dn.Name)
	}
	if dn.LocalSheetID == nil || *dn.LocalSheetID != 1 {
		got := "<nil>"
		if dn.LocalSheetID != nil {
			got = fmt.Sprintf("%d", *dn.LocalSheetID)
		}
		t.Fatalf("LocalSheetID = %s, want 1", got)
	}
}

func TestParseBIFFNameHiddenFlag(t *testing.T) {
	payload := make([]byte, 14+1+5)
	binary.LittleEndian.PutUint16(payload[0:2], 0x0020) // grbit bit 5 = fHidden
	payload[3] = 5
	payload[14] = 0
	copy(payload[15:20], "MyRng")

	dn, ok := parseBIFFName(payload)
	if !ok {
		t.Fatal("parseBIFFName returned ok=false")
	}
	if !dn.Hidden {
		t.Fatal("Hidden = false, want true")
	}
}

func TestParseBIFFNameBuiltInPrintTitlesDecodesArea3D(t *testing.T) {
	formula := []byte{0x3B, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0xFF, 0x00}
	payload := makeBuiltInNamePayload(0x07, 1, formula)
	dn, ok := parseBIFFNameWithContext(payload, []boundSheet{{name: "Sheet1"}}, []externSheetRef{{firstSheet: 0, lastSheet: 0, sourceKind: supBookKindInternal}})
	if !ok {
		t.Fatal("parseBIFFNameWithContext returned ok=false")
	}
	if dn.Name != "_xlnm.Print_Titles" {
		t.Fatalf("Name = %q, want _xlnm.Print_Titles", dn.Name)
	}
	if dn.Value != "Sheet1!$1:$2" {
		t.Fatalf("Value = %q, want Sheet1!$1:$2", dn.Value)
	}
	if dn.LocalSheetID == nil || *dn.LocalSheetID != 0 {
		got := "<nil>"
		if dn.LocalSheetID != nil {
			got = fmt.Sprintf("%d", *dn.LocalSheetID)
		}
		t.Fatalf("LocalSheetID = %s, want 0", got)
	}
}

func TestParseBIFFNameBuiltInPrintAreaDecodesAbsoluteRange(t *testing.T) {
	formula := []byte{0x3B, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x01, 0x00}
	payload := makeBuiltInNamePayload(0x06, 1, formula)
	dn, ok := parseBIFFNameWithContext(payload, []boundSheet{{name: "Sheet1"}}, []externSheetRef{{firstSheet: 0, lastSheet: 0, sourceKind: supBookKindInternal}})
	if !ok {
		t.Fatal("parseBIFFNameWithContext returned ok=false")
	}
	if dn.Name != "_xlnm.Print_Area" {
		t.Fatalf("Name = %q, want _xlnm.Print_Area", dn.Name)
	}
	if dn.Value != "Sheet1!$A$1:$B$3" {
		t.Fatalf("Value = %q, want Sheet1!$A$1:$B$3", dn.Value)
	}
}

func TestApplyLegacyPrintTitlesSetsSheetMetadata(t *testing.T) {
	workbook := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{{Name: "Sheet1"}},
		DefinedNames: []*sheetir.DefinedName{{
			Name:         "_xlnm.Print_Titles",
			LocalSheetID: intPtr(0),
			Value:        "Sheet1!$1:$2,Sheet1!$A:$B",
		}},
	}
	applyLegacyPrintTitles(workbook)
	if workbook.Sheets[0].PrintTitleRows != "$1:$2" {
		t.Fatalf("PrintTitleRows = %q, want $1:$2", workbook.Sheets[0].PrintTitleRows)
	}
	if workbook.Sheets[0].PrintTitleCols != "$A:$B" {
		t.Fatalf("PrintTitleCols = %q, want $A:$B", workbook.Sheets[0].PrintTitleCols)
	}
}

func TestApplyLegacyPrintAreasSetsSheetMetadata(t *testing.T) {
	workbook := &sheetir.Workbook{
		Sheets: []*sheetir.Sheet{{Name: "Sheet1"}},
		DefinedNames: []*sheetir.DefinedName{{
			Name:         "_xlnm.Print_Area",
			LocalSheetID: intPtr(0),
			Value:        "Sheet1!$A$1:$D$20,Sheet1!$F$1:$F$10",
		}},
	}
	applyLegacyPrintAreas(workbook)
	if workbook.Sheets[0].PrintArea != "$A$1:$D$20,$F$1:$F$10" {
		t.Fatalf("PrintArea = %q, want $A$1:$D$20,$F$1:$F$10", workbook.Sheets[0].PrintArea)
	}
}

func TestIsDateNumFmt(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{"dd/mm/yyyy", true},
		{"yyyy-mm-dd", true},
		{"h:mm AM/PM", true},
		{"m/d/yy", true},
		{"0.00", false},
		{"#,##0", false},
		{"@", false},
		{`"text"`, false},
		{`"hello" 0`, false},
		// quoted d should not match
		{`"date" 0.00`, false},
		// unquoted d after quotes: match
		{`"No." dd/mm/yyyy`, true},
	}
	for _, c := range cases {
		got := isDateNumFmt(c.code)
		if got != c.want {
			t.Errorf("isDateNumFmt(%q) = %v, want %v", c.code, got, c.want)
		}
	}
}

func makeBuiltInNamePayload(builtInID byte, itab uint16, formula []byte) []byte {
	payload := make([]byte, 15+len(formula))
	binary.LittleEndian.PutUint16(payload[0:2], 0x0001) // fBuiltin
	payload[3] = 1                                      // cch (built-in name id byte count)
	binary.LittleEndian.PutUint16(payload[4:6], uint16(len(formula)))
	binary.LittleEndian.PutUint16(payload[8:10], itab)
	payload[14] = builtInID
	copy(payload[15:], formula)
	return payload
}

func intPtr(v int) *int { return &v }

func TestExcelSerialToDate(t *testing.T) {
	cases := []struct {
		serial   float64
		date1904 bool
		want     string
	}{
		// 1900 mode (default).
		{1, false, "1900-01-01"},
		{2, false, "1900-01-02"},
		{59, false, "1900-02-28"},
		{60, false, "1900-02-28"}, // phantom Lotus day → clamp to Feb 28
		{61, false, "1900-03-01"},
		{44927, false, "2023-01-01"},
		{44927.5, false, "2023-01-01 12:00:00"},
		{0.5, false, ""}, // below minimum serial
		{0, false, ""},
		// 1904 mode: serial 0 = 1904-01-01, no phantom-day fudge.
		{0, true, "1904-01-01"},
		{1, true, "1904-01-02"},
		{59, true, "1904-02-29"},    // 1904 is a real leap year
		{60, true, "1904-03-01"},    // no Lotus skip
		{43465, true, "2023-01-01"}, // same calendar date is 1462 days lower
		{43465.5, true, "2023-01-01 12:00:00"},
		{-0.5, true, ""}, // below 1904 minimum
	}
	for _, c := range cases {
		got := excelSerialToDate(c.serial, c.date1904)
		if got != c.want {
			t.Errorf("excelSerialToDate(%v, date1904=%v) = %q, want %q", c.serial, c.date1904, got, c.want)
		}
	}
}

func TestParseNumberCellPromotedToDate(t *testing.T) {
	// Build a NUMBER record for row 0, col 0 with XF index 1.
	// Serial 44927 = 2023-01-01.
	payload := make([]byte, 14)
	binary.LittleEndian.PutUint16(payload[0:2], 0) // row
	binary.LittleEndian.PutUint16(payload[2:4], 0) // col
	binary.LittleEndian.PutUint16(payload[4:6], 1) // xfIndex
	binary.LittleEndian.PutUint64(payload[6:14], math.Float64bits(44927))

	xfs := []legacyXF{{}, {NumFmtCode: "dd/mm/yyyy"}}
	cell := parseNumberCell(payload, xfs)
	if cell == nil {
		t.Fatal("parseNumberCell returned nil")
	}
	promoteNumericCellToDate(cell, false)
	if cell.Kind != sheetir.CellDate {
		t.Fatalf("Kind = %v, want CellDate", cell.Kind)
	}
	if cell.StrValue != "2023-01-01" {
		t.Fatalf("StrValue = %q, want 2023-01-01", cell.StrValue)
	}
}

func TestParseBIFFXFProtectionLockedHidden(t *testing.T) {
	// Build an XF payload with grbit at offset 4-5 having bit 2 (fLocked) and bit 3 (fHidden) set.
	// grbit = 0x000C (bits 2 and 3)
	payload := make([]byte, 20)
	binary.LittleEndian.PutUint16(payload[4:6], 0x000C) // fLocked | fHidden
	xf := parseBIFFXF(payload, nil, nil, defaultBIFFPalette)
	if !xf.Locked {
		t.Error("expected Locked=true")
	}
	if !xf.FormulaHidden {
		t.Error("expected FormulaHidden=true")
	}
}

func TestParseBIFFXFProtectionLockedOnly(t *testing.T) {
	payload := make([]byte, 20)
	binary.LittleEndian.PutUint16(payload[4:6], 0x0004) // bit 2 only = fLocked
	xf := parseBIFFXF(payload, nil, nil, defaultBIFFPalette)
	if !xf.Locked {
		t.Error("expected Locked=true")
	}
	if xf.FormulaHidden {
		t.Error("expected FormulaHidden=false")
	}
}

func TestParseBIFFXFProtectionNone(t *testing.T) {
	payload := make([]byte, 20)
	// grbit = 0x0000: no protection flags
	xf := parseBIFFXF(payload, nil, nil, defaultBIFFPalette)
	if xf.Locked {
		t.Error("expected Locked=false")
	}
	if xf.FormulaHidden {
		t.Error("expected FormulaHidden=false")
	}
}

// HSSF PaletteRecord (sid 0x0092) lets a workbook redefine the palette colors
// at indices 0x08..0x3F. parseBIFFPalette decodes the record and biffColorHex
// then resolves icv against the override.
//
// Reference: refs/apache-poi/poi/src/main/java/org/apache/poi/hssf/record/PaletteRecord.java
func TestParseBIFFPaletteOverridesDefaultColors(t *testing.T) {
	// Build a synthetic PaletteRecord payload: numColors=2, then RGBA quads.
	// Palette[0] (icv 0x08, normally 000000) → magenta FF00FF.
	// Palette[1] (icv 0x09, normally FFFFFF) → forest 228B22.
	payload := []byte{
		0x02, 0x00, // numColors = 2
		0xFF, 0x00, 0xFF, 0x00, // palette[0] = R=FF G=00 B=FF (unused byte)
		0x22, 0x8B, 0x22, 0x00, // palette[1] = R=22 G=8B B=22
	}
	palette := parseBIFFPalette(payload)
	if got := biffColorHex(0x08, palette); got != "FF00FF" {
		t.Fatalf("icv=0x08 = %q, want FF00FF (palette override)", got)
	}
	if got := biffColorHex(0x09, palette); got != "228B22" {
		t.Fatalf("icv=0x09 = %q, want 228B22 (palette override)", got)
	}
	// Indices not overridden in the record fall back to the default palette.
	if got := biffColorHex(0x0A, palette); got != "FF0000" {
		t.Fatalf("icv=0x0A = %q, want FF0000 (default red)", got)
	}
	// Indices outside the palette range remain "".
	if got := biffColorHex(0x40, palette); got != "" {
		t.Fatalf("icv=0x40 = %q, want \"\" (theme sentinel)", got)
	}
}

func TestParseBIFFPaletteIgnoresShortPayload(t *testing.T) {
	// numColors=1 but only one byte of color data — should keep defaults.
	short := []byte{0x01, 0x00, 0xFF}
	palette := parseBIFFPalette(short)
	if got := biffColorHex(0x08, palette); got != defaultBIFFPalette[0] {
		t.Fatalf("short-payload palette[0] mutated to %q, want default %q", got, defaultBIFFPalette[0])
	}
	// Empty payload returns the default palette.
	empty := parseBIFFPalette(nil)
	if got := biffColorHex(0x08, empty); got != defaultBIFFPalette[0] {
		t.Fatalf("nil-payload palette[0] = %q, want default %q", got, defaultBIFFPalette[0])
	}
}

func TestBiffColorHexUsesDefaultPaletteWhenUnchanged(t *testing.T) {
	// All 56 default entries should round-trip through biffColorHex unchanged.
	for icv := uint16(0x08); icv <= 0x3F; icv++ {
		want := defaultBIFFPalette[icv-0x08]
		if got := biffColorHex(icv, defaultBIFFPalette); got != want {
			t.Fatalf("icv=0x%02X = %q, want %q", icv, got, want)
		}
	}
}

func TestParseWorkbookGlobalsAppliesPaletteOverrideToXFs(t *testing.T) {
	// Synthetic workbook globals stream: BOF, FONT (default), PALETTE
	// overriding icv=0x0A (red) to forest, XF (font icv→nothing here, but
	// we can route through fill index 0x0A), EOF.
	var b []byte
	writeRecord := func(id uint16, payload []byte) {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		b = append(b, payload...)
	}
	// BIFF8 BOF (16-byte payload is fine; layout is not validated by parser).
	writeRecord(biffBOF, make([]byte, 16))
	// One default font so XF.fontIdx=0 resolves cleanly.
	writeRecord(biffFont, makeFontPayload(220, false, false, false, false, "Arial"))
	// Palette: override icv=0x0A only. PaletteRecord begins palette[0]=icv 0x08;
	// to retarget icv=0x0A we rewrite slots 0..2 (with slots 0/1 set to defaults).
	palettePayload := []byte{0x03, 0x00}
	for _, c := range []struct{ r, g, b byte }{
		{0x00, 0x00, 0x00}, // icv 0x08 = black (default)
		{0xFF, 0xFF, 0xFF}, // icv 0x09 = white (default)
		{0x22, 0x8B, 0x22}, // icv 0x0A overridden = forest green
	} {
		palettePayload = append(palettePayload, c.r, c.g, c.b, 0x00)
	}
	writeRecord(biffPalette, palettePayload)
	// XF: fillPattern=1 (solid), foreIdx=0x0A (now forest green via palette).
	writeRecord(biffXF, makeXFPayload(0, 0, 0, 0, false, 1, 0x0A, 0x09))
	writeRecord(biffEOF, nil)

	_, _, xfs, _, _, _, _, _, _, err := parseWorkbookGlobals(b)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(xfs) != 1 {
		t.Fatalf("xfs = %d, want 1", len(xfs))
	}
	if xfs[0].BgColor != "228B22" {
		t.Fatalf("BgColor = %q, want 228B22 (palette-overridden icv=0x0A)", xfs[0].BgColor)
	}
}

func TestParseWorkbookGlobalsResolvesXFsAgainstLatePaletteRecord(t *testing.T) {
	// PaletteRecord can appear AFTER XF records in HSSF's RecordOrderer.
	// Our parser buffers XF payloads and resolves them at EOF, so the
	// palette override applies regardless of record order.
	var b []byte
	writeRecord := func(id uint16, payload []byte) {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		b = append(b, payload...)
	}
	writeRecord(biffBOF, make([]byte, 16))
	writeRecord(biffFont, makeFontPayload(220, false, false, false, false, "Arial"))
	// XF FIRST.
	writeRecord(biffXF, makeXFPayload(0, 0, 0, 0, false, 1, 0x0A, 0x09))
	// PALETTE AFTER, overriding icv=0x0A.
	palettePayload := []byte{0x03, 0x00}
	for _, c := range []struct{ r, g, b byte }{
		{0x00, 0x00, 0x00},
		{0xFF, 0xFF, 0xFF},
		{0x80, 0x00, 0x00}, // icv 0x0A → maroon
	} {
		palettePayload = append(palettePayload, c.r, c.g, c.b, 0x00)
	}
	writeRecord(biffPalette, palettePayload)
	writeRecord(biffEOF, nil)

	_, _, xfs, _, _, _, _, _, _, err := parseWorkbookGlobals(b)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(xfs) != 1 {
		t.Fatalf("xfs = %d, want 1", len(xfs))
	}
	if xfs[0].BgColor != "800000" {
		t.Fatalf("BgColor = %q, want 800000 (late-palette-overridden icv=0x0A)", xfs[0].BgColor)
	}
}

func TestParseDateWindow1904(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"empty payload defaults to 1900", nil, false},
		{"flag=0 keeps 1900 epoch", []byte{0x00, 0x00}, false},
		{"flag=1 selects 1904 epoch", []byte{0x01, 0x00}, true},
		{"any nonzero selects 1904", []byte{0xFF, 0x00}, true},
	}
	for _, c := range cases {
		if got := parseDateWindow1904(c.payload); got != c.want {
			t.Errorf("%s: parseDateWindow1904(% x) = %v, want %v", c.name, c.payload, got, c.want)
		}
	}
}

func TestParseWorkbookGlobalsCapturesDate1904Flag(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		return append(b, payload...)
	}

	t.Run("default 1900 mode when DATEMODE absent", func(t *testing.T) {
		var b []byte
		b = writeRecord(b, biffBOF, make([]byte, 16))
		b = writeRecord(b, biffEOF, nil)
		_, _, _, _, _, _, date1904, _, _, err := parseWorkbookGlobals(b)
		if err != nil {
			t.Fatalf("parseWorkbookGlobals: %v", err)
		}
		if date1904 {
			t.Fatalf("date1904 = true, want false (DATEMODE not present)")
		}
	})

	t.Run("DATEMODE=1 selects 1904 mode", func(t *testing.T) {
		var b []byte
		b = writeRecord(b, biffBOF, make([]byte, 16))
		b = writeRecord(b, biffDateWindow1904, []byte{0x01, 0x00})
		b = writeRecord(b, biffEOF, nil)
		_, _, _, _, _, _, date1904, _, _, err := parseWorkbookGlobals(b)
		if err != nil {
			t.Fatalf("parseWorkbookGlobals: %v", err)
		}
		if !date1904 {
			t.Fatalf("date1904 = false, want true (DATEMODE flag = 1)")
		}
	})
}

func TestParseNumberCellPromotedToDateHonors1904Mode(t *testing.T) {
	// Same serial 43465 in 1904 mode = 2023-01-01; the corresponding 1900-mode
	// serial would be 44927. This proves promotion threads the workbook flag.
	payload := make([]byte, 14)
	binary.LittleEndian.PutUint16(payload[0:2], 0)
	binary.LittleEndian.PutUint16(payload[2:4], 0)
	binary.LittleEndian.PutUint16(payload[4:6], 1)
	binary.LittleEndian.PutUint64(payload[6:14], math.Float64bits(43465))

	xfs := []legacyXF{{}, {NumFmtCode: "yyyy-mm-dd"}}
	cell := parseNumberCell(payload, xfs)
	if cell == nil {
		t.Fatal("parseNumberCell returned nil")
	}
	promoteNumericCellToDate(cell, true)
	if cell.Kind != sheetir.CellDate {
		t.Fatalf("Kind = %v, want CellDate", cell.Kind)
	}
	if cell.StrValue != "2023-01-01" {
		t.Fatalf("StrValue = %q, want 2023-01-01 (1904-mode serial 43465)", cell.StrValue)
	}
}

// --- STYLE record (0x0293) ---

func makeBiffStylePayloadBuiltin(xfIndex uint16, builtinID byte, level byte) []byte {
	p := make([]byte, 4)
	binary.LittleEndian.PutUint16(p[0:2], xfIndex|0x8000) // bit 15 = built-in
	p[2] = builtinID
	p[3] = level
	return p
}

func makeBiffStylePayloadUser(xfIndex uint16, name string) []byte {
	p := make([]byte, 5+len(name))
	binary.LittleEndian.PutUint16(p[0:2], xfIndex&0x0FFF) // bit 15 = 0 → user
	binary.LittleEndian.PutUint16(p[2:4], uint16(len(name)))
	p[4] = 0x00 // compressed ASCII
	copy(p[5:], name)
	return p
}

func TestBuildStyleNamesBuiltinStyles(t *testing.T) {
	cases := []struct {
		id    byte
		level byte
		want  string
	}{
		{0, 0, "Normal"},
		{1, 0, "RowLevel_1"}, // level 0 → display "1"
		{1, 2, "RowLevel_3"}, // level 2 → display "3"
		{2, 1, "ColLevel_2"},
		{3, 0, "Comma"},
		{4, 0, "Currency"},
		{5, 0, "Percent"},
		{6, 0, "Comma [0]"},
		{7, 0, "Currency [0]"},
	}
	for _, c := range cases {
		payload := makeBiffStylePayloadBuiltin(5, c.id, c.level)
		got := buildStyleNames([][]byte{payload})
		if got[5] != c.want {
			t.Errorf("id=%d level=%d: got %q, want %q", c.id, c.level, got[5], c.want)
		}
	}
}

func TestBuildStyleNamesUserDefined(t *testing.T) {
	payload := makeBiffStylePayloadUser(2, "MyStyle")
	got := buildStyleNames([][]byte{payload})
	if got[2] != "MyStyle" {
		t.Errorf("user-defined: got %q, want MyStyle", got[2])
	}
}

func TestBuildStyleNamesRejectsShortPayloads(t *testing.T) {
	cases := [][]byte{
		nil,
		{0x00},
		{0x00, 0x80, 0x00}, // built-in, len=3 (need 4)
	}
	for _, p := range cases {
		got := buildStyleNames([][]byte{p})
		if len(got) != 0 {
			t.Errorf("short payload % x: got non-empty map %v", p, got)
		}
	}
}

func TestBuildStyleNamesIgnoresUnknownBuiltinID(t *testing.T) {
	payload := makeBiffStylePayloadBuiltin(0, 0xFF, 0)
	got := buildStyleNames([][]byte{payload})
	if len(got) != 0 {
		t.Errorf("unknown builtin id 0xFF: expected empty map, got %v", got)
	}
}

func TestParseWorkbookGlobalsCapturesStyleNames(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		return append(b, payload...)
	}
	var b []byte
	b = writeRecord(b, biffBOF, make([]byte, 16))
	// Built-in Normal at xf 0.
	b = writeRecord(b, biffStyle, makeBiffStylePayloadBuiltin(0, 0, 0))
	// User-defined at xf 3.
	b = writeRecord(b, biffStyle, makeBiffStylePayloadUser(3, "Heading 1"))
	b = writeRecord(b, biffEOF, nil)

	_, _, _, _, _, _, _, styleNames, _, err := parseWorkbookGlobals(b)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if styleNames[0] != "Normal" {
		t.Errorf("xf[0] = %q, want Normal", styleNames[0])
	}
	if styleNames[3] != "Heading 1" {
		t.Errorf("xf[3] = %q, want Heading 1", styleNames[3])
	}
}

// --- DefaultRowHeightRecord grbit flag bits ---

func makeDefRowHeightPayload(grbit uint16, heightTwips uint16) []byte {
	p := make([]byte, 4)
	binary.LittleEndian.PutUint16(p[0:2], grbit)
	binary.LittleEndian.PutUint16(p[2:4], heightTwips)
	return p
}

func TestApplyDefaultRowHeightParsesGrbitFlags(t *testing.T) {
	cases := []struct {
		name           string
		grbit          uint16
		wantUnsynced   bool
		wantZeroHeight bool
		wantExtraAbove bool
		wantExtraBelow bool
	}{
		{"no flags", 0x0000, false, false, false, false},
		{"fUnsynced only", 0x0001, true, false, false, false},
		{"fDyZero only", 0x0002, false, true, false, false},
		{"fExAsc only", 0x0004, false, false, true, false},
		{"fExDsc only", 0x0008, false, false, false, true},
		{"all flags", 0x000F, true, true, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sheet := &sheetir.Sheet{}
			applyDefaultRowHeight(sheet, makeDefRowHeightPayload(c.grbit, 255))
			if sheet.DefaultRowHeightUnsynced != c.wantUnsynced {
				t.Errorf("Unsynced = %v, want %v", sheet.DefaultRowHeightUnsynced, c.wantUnsynced)
			}
			if sheet.DefaultRowHeightZeroHeight != c.wantZeroHeight {
				t.Errorf("ZeroHeight = %v, want %v", sheet.DefaultRowHeightZeroHeight, c.wantZeroHeight)
			}
			if sheet.DefaultRowHeightExtraSpaceAbove != c.wantExtraAbove {
				t.Errorf("ExtraAbove = %v, want %v", sheet.DefaultRowHeightExtraSpaceAbove, c.wantExtraAbove)
			}
			if sheet.DefaultRowHeightExtraSpaceBelow != c.wantExtraBelow {
				t.Errorf("ExtraBelow = %v, want %v", sheet.DefaultRowHeightExtraSpaceBelow, c.wantExtraBelow)
			}
		})
	}
}

func TestApplyDefaultRowHeightStillSetsHeightPt(t *testing.T) {
	sheet := &sheetir.Sheet{}
	applyDefaultRowHeight(sheet, makeDefRowHeightPayload(0x0001, 300)) // 300 twips = 15 pt
	if sheet.DefaultRowHeightPt != 15.0 {
		t.Errorf("DefaultRowHeightPt = %.2f, want 15.00", sheet.DefaultRowHeightPt)
	}
}

// --- Row-level XF (fGhostDirty + ixfe) ---

// makeRowPayloadWithXF builds a 16-byte ROW payload with fGhostDirty set in grbit
// and the given XF index in ixfe (bytes 14:16).
func makeRowPayloadWithXF(row int, xfIdx uint16) []byte {
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(row))
	grbit := uint16(0x0080) // fGhostDirty = bit 7
	binary.LittleEndian.PutUint16(payload[12:14], grbit)
	binary.LittleEndian.PutUint16(payload[14:16], xfIdx)
	return payload
}

func TestApplyRowRecordCapturesRowXFStyle(t *testing.T) {
	// Build a minimal XF list: index 0 = default, index 1 = blue background
	fmts := defaultBIFFFormatCodes()
	xfDefault := parseBIFFXF(makeXFPayload(0, 0, 0, 0, false, 0, 0, 0), fmts, nil, defaultBIFFPalette)
	xfBlue := parseBIFFXF(makeXFPayload(0, 0, 0, 0, false, 1, 0x000C, 0x0009), fmts, nil, defaultBIFFPalette)
	xfs := []legacyXF{xfDefault, xfBlue}

	rows := make(map[int]*sheetir.Row)
	applyRowRecord(rows, makeRowPayloadWithXF(0, 1), xfs)
	row := rows[1]
	if row == nil {
		t.Fatal("row 1 not created")
	}
	if row.Style == nil {
		t.Fatal("row.Style is nil, want non-nil (fGhostDirty set)")
	}
	if row.Style.BgColor == "" {
		t.Errorf("row.Style.BgColor is empty, expected a color from XF index 1")
	}
}

func TestApplyRowRecordNoStyleWhenFGhostDirtyNotSet(t *testing.T) {
	fmts := defaultBIFFFormatCodes()
	xfBlue := parseBIFFXF(makeXFPayload(0, 0, 0, 0, false, 1, 0x000C, 0x0009), fmts, nil, defaultBIFFPalette)
	xfs := []legacyXF{{}, xfBlue}

	rows := make(map[int]*sheetir.Row)
	// fGhostDirty NOT set (grbit = 0)
	applyRowRecord(rows, rowPayload(0, 0x0000), xfs)
	row := rows[1]
	if row != nil && row.Style != nil {
		t.Errorf("row.Style = %+v, want nil when fGhostDirty is not set", row.Style)
	}
}

func TestApplyRowRecordNoStyleWhenXFIndexIsZero(t *testing.T) {
	fmts := defaultBIFFFormatCodes()
	xfs := []legacyXF{parseBIFFXF(makeXFPayload(0, 0, 0, 0, false, 1, 0x000C, 0x0009), fmts, nil, defaultBIFFPalette)}

	rows := make(map[int]*sheetir.Row)
	// fGhostDirty set but XF index = 0 (default → no style applied)
	applyRowRecord(rows, makeRowPayloadWithXF(0, 0), xfs)
	row := rows[1]
	if row != nil && row.Style != nil {
		t.Errorf("row.Style = %+v, want nil when XF index is 0", row.Style)
	}
}

// ---------------------------------------------------------------------------
// Protection record tests
// ---------------------------------------------------------------------------

// makeProtectPayload builds a minimal PROTECT / OBJPROTECT / SCENPROTECT payload.
func makeProtectPayload(protected bool) []byte {
	b := make([]byte, 2)
	if protected {
		binary.LittleEndian.PutUint16(b, 1)
	}
	return b
}

// makePasswordPayload builds a minimal PASSWORD record payload with the given hash.
func makePasswordPayload(hash uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, hash)
	return b
}

func makeFileSharingPayload(readOnly bool, hash uint16, username string) []byte {
	payload := make([]byte, 6)
	if readOnly {
		binary.LittleEndian.PutUint16(payload[0:2], 1)
	}
	binary.LittleEndian.PutUint16(payload[2:4], hash)
	binary.LittleEndian.PutUint16(payload[4:6], uint16(len(username)))
	if username == "" {
		return payload
	}
	payload = append(payload, 0x00) // compressed unicode
	payload = append(payload, []byte(username)...)
	return payload
}

func TestParseWorksheetProtectionSetWhenProtectRecord(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, make([]byte, 8))
	data = appendBiffRecord(data, biffProtect, makeProtectPayload(true))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Protected"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.Protection == nil {
		t.Fatal("sheet.Protection is nil, want non-nil")
	}
	if !sheet.Protection.Sheet {
		t.Error("Protection.Sheet = false, want true")
	}
}

func TestParseWorksheetNoProtectionWhenProtectFalse(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, make([]byte, 8))
	data = appendBiffRecord(data, biffProtect, makeProtectPayload(false))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Open"}, nil, nil)
	if sheet != nil && sheet.Protection != nil {
		t.Errorf("Protection = %+v, want nil when protect flag is 0", sheet.Protection)
	}
}

func TestParseWorksheetProtectionCapturesObjectsAndScenarios(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, make([]byte, 8))
	data = appendBiffRecord(data, biffProtect, makeProtectPayload(true))
	data = appendBiffRecord(data, biffObjProtect, makeProtectPayload(true))
	data = appendBiffRecord(data, biffScenProtect, makeProtectPayload(true))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Full"}, nil, nil)
	if sheet.Protection == nil {
		t.Fatal("sheet.Protection is nil")
	}
	if !sheet.Protection.Objects {
		t.Error("Protection.Objects = false, want true")
	}
	if !sheet.Protection.Scenarios {
		t.Error("Protection.Scenarios = false, want true")
	}
}

func TestParseWorksheetProtectionCapturesPasswordHash(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, make([]byte, 8))
	data = appendBiffRecord(data, biffProtect, makeProtectPayload(true))
	data = appendBiffRecord(data, biffPassword, makePasswordPayload(0xABCD))
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "HasPwd"}, nil, nil)
	if sheet.Protection == nil {
		t.Fatal("sheet.Protection is nil")
	}
	if sheet.Protection.Password != "ABCD" {
		t.Errorf("Password = %q, want ABCD", sheet.Protection.Password)
	}
}

func TestBuildWorkbookProtectionNilWhenAllZero(t *testing.T) {
	if p := buildWorkbookProtection(false, false, false, 0); p != nil {
		t.Errorf("want nil, got %+v", p)
	}
}

func TestBuildWorkbookProtectionSetsLockStructure(t *testing.T) {
	p := buildWorkbookProtection(true, false, false, 0)
	if p == nil {
		t.Fatal("want non-nil")
	}
	if !p.LockStructure {
		t.Error("LockStructure = false, want true")
	}
	if p.LockRevision {
		t.Error("LockRevision = true, want false")
	}
}

func TestBuildWorkbookProtectionSetsLockWindows(t *testing.T) {
	p := buildWorkbookProtection(false, true, false, 0)
	if p == nil {
		t.Fatal("want non-nil")
	}
	if !p.LockWindows {
		t.Error("LockWindows = false, want true")
	}
	if p.LockStructure {
		t.Error("LockStructure = true, want false")
	}
}

func TestBuildWorkbookProtectionSetsLockRevision(t *testing.T) {
	p := buildWorkbookProtection(false, false, true, 0)
	if p == nil {
		t.Fatal("want non-nil")
	}
	if !p.LockRevision {
		t.Error("LockRevision = false, want true")
	}
}

func TestBuildWorkbookProtectionSetsPasswordHash(t *testing.T) {
	p := buildWorkbookProtection(true, false, false, 0x1234)
	if p == nil {
		t.Fatal("want non-nil")
	}
	if p.WorkbookPassword != "1234" {
		t.Errorf("WorkbookPassword = %q, want 1234", p.WorkbookPassword)
	}
}

func TestParseWindowProtectRecord(t *testing.T) {
	if parseWindowProtectRecord(nil) {
		t.Fatal("parseWindowProtectRecord(nil) = true, want false")
	}
	if !parseWindowProtectRecord([]byte{0x01, 0x00}) {
		t.Fatal("parseWindowProtectRecord(protected) = false, want true")
	}
	if parseWindowProtectRecord([]byte{0x00, 0x00}) {
		t.Fatal("parseWindowProtectRecord(unprotected) = true, want false")
	}
}

func TestParseFileSharingRecordCapturesWriteReservation(t *testing.T) {
	info, ok := parseFileSharingRecord(makeFileSharingPayload(true, 0xBEEF, "Alice"))
	if !ok {
		t.Fatal("parseFileSharingRecord returned !ok")
	}
	if !info.readOnly {
		t.Error("readOnly = false, want true")
	}
	if info.passwordHash != 0xBEEF {
		t.Errorf("passwordHash = %04X, want BEEF", info.passwordHash)
	}
	if info.username != "Alice" {
		t.Errorf("username = %q, want Alice", info.username)
	}
}

func TestFinalizeWorkbookProtectionAddsWriteReservationMetadata(t *testing.T) {
	p := finalizeWorkbookProtection(false, false, false, 0, 0xCAFE, true, 0xBEEF, "Alice")
	if p == nil {
		t.Fatal("want non-nil")
	}
	if !p.WriteProtected {
		t.Error("WriteProtected = false, want true")
	}
	if p.RevisionsPassword != "CAFE" {
		t.Errorf("RevisionsPassword = %q, want CAFE", p.RevisionsPassword)
	}
	if p.WriteResPassword != "BEEF" {
		t.Errorf("WriteResPassword = %q, want BEEF", p.WriteResPassword)
	}
	if p.WriteResUser != "Alice" {
		t.Errorf("WriteResUser = %q, want Alice", p.WriteResUser)
	}
}

func TestParseWorkbookGlobalsCapturesWorkbookProtection(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		return append(b, payload...)
	}
	var b []byte
	b = writeRecord(b, biffBOF, make([]byte, 16))
	b = writeRecord(b, biffProtect, makeProtectPayload(true))
	b = writeRecord(b, biffPassword, makePasswordPayload(0xBEEF))
	b = writeRecord(b, biffEOF, nil)

	_, _, _, _, _, _, _, _, wp, err := parseWorkbookGlobals(b)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if wp == nil {
		t.Fatal("WorkbookProtection is nil, want non-nil")
	}
	if !wp.LockStructure {
		t.Error("LockStructure = false, want true")
	}
	if wp.WorkbookPassword != "BEEF" {
		t.Errorf("WorkbookPassword = %q, want BEEF", wp.WorkbookPassword)
	}
}

func TestParseWorkbookGlobalsCapturesWindowProtection(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		return append(b, payload...)
	}
	var b []byte
	b = writeRecord(b, biffBOF, make([]byte, 16))
	b = writeRecord(b, biffWindowProtect, []byte{0x01, 0x00})
	b = writeRecord(b, biffEOF, nil)

	_, _, _, _, _, _, _, _, wp, err := parseWorkbookGlobals(b)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if wp == nil {
		t.Fatal("WorkbookProtection is nil, want non-nil")
	}
	if !wp.LockWindows {
		t.Error("LockWindows = false, want true")
	}
}

func TestParseWorkbookGlobalsCapturesWriteProtectionMetadata(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		return append(b, payload...)
	}
	var b []byte
	b = writeRecord(b, biffBOF, make([]byte, 16))
	b = writeRecord(b, biffWriteProtect, nil)
	b = writeRecord(b, biffPasswordRev4, makePasswordPayload(0xCAFE))
	b = writeRecord(b, biffFileSharing, makeFileSharingPayload(true, 0xBEEF, "Alice"))
	b = writeRecord(b, biffEOF, nil)

	_, _, _, _, _, _, _, _, wp, err := parseWorkbookGlobals(b)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if wp == nil {
		t.Fatal("WorkbookProtection is nil, want non-nil")
	}
	if !wp.WriteProtected {
		t.Error("WriteProtected = false, want true")
	}
	if wp.RevisionsPassword != "CAFE" {
		t.Errorf("RevisionsPassword = %q, want CAFE", wp.RevisionsPassword)
	}
	if wp.WriteResPassword != "BEEF" {
		t.Errorf("WriteResPassword = %q, want BEEF", wp.WriteResPassword)
	}
	if wp.WriteResUser != "Alice" {
		t.Errorf("WriteResUser = %q, want Alice", wp.WriteResUser)
	}
}

// makeAutoFilterPayload constructs a minimal AUTOFILTER (0x9E) record payload.
// col is the 0-based column index within the AF range.
// grbit: use 0 for normal conditions; set bit 2 (0x0004) for top-N mode.
// vt1/sgn1 + doper1 describe condition 1; vt2/sgn2 + doper2 describe condition 2.
// grbitAnd: 0=OR, 1=AND.
// str1, str2: optional string values (used when vt==4).
func makeAutoFilterPayload(col uint16, grbit uint16, vt1, sgn1 byte, doper1 [8]byte, vt2, sgn2 byte, doper2 [8]byte, grbitAnd byte, str1, str2 string) []byte {
	p := make([]byte, 29)
	binary.LittleEndian.PutUint16(p[0:2], col)
	binary.LittleEndian.PutUint16(p[2:4], grbit)
	p[4] = vt1
	p[5] = sgn1
	copy(p[6:14], doper1[:])
	p[14] = vt2
	p[15] = sgn2
	copy(p[16:24], doper2[:])
	p[24] = grbitAnd
	binary.LittleEndian.PutUint16(p[25:27], uint16(len(str1)))
	binary.LittleEndian.PutUint16(p[27:29], uint16(len(str2)))
	for _, s := range []string{str1, str2} {
		if len(s) > 0 {
			p = append(p, byte(len(s)))
			p = append(p, 0x00)
			p = append(p, []byte(s)...)
		}
	}
	return p
}

func TestParseAutoFilterRecordShortPayloadReturnsNil(t *testing.T) {
	if got := parseAutoFilterRecord([]byte{0x00, 0x00}); got != nil {
		t.Fatalf("expected nil for short payload, got %+v", got)
	}
}

func TestParseAutoFilterRecordNoCriteria(t *testing.T) {
	var d1, d2 [8]byte
	payload := makeAutoFilterPayload(0, 0, 0x00, 0x02, d1, 0x00, 0x02, d2, 0, "", "")
	if got := parseAutoFilterRecord(payload); got != nil {
		t.Fatalf("expected nil for no-criteria payload, got %+v", got)
	}
}

func TestParseAutoFilterRecordSingleEqualityStringFilter(t *testing.T) {
	var d1, d2 [8]byte
	payload := makeAutoFilterPayload(2, 0, 0x04, 0x02, d1, 0x00, 0x00, d2, 0, "Alpha", "")
	fc := parseAutoFilterRecord(payload)
	if fc == nil {
		t.Fatal("expected non-nil FilterColumn")
	}
	if fc.ColIndex != 2 {
		t.Errorf("ColIndex = %d, want 2", fc.ColIndex)
	}
	if len(fc.Filters) != 1 || fc.Filters[0] != "Alpha" {
		t.Errorf("Filters = %v, want [Alpha]", fc.Filters)
	}
	if len(fc.CustomFilters) != 0 {
		t.Errorf("expected no CustomFilters, got %v", fc.CustomFilters)
	}
}

func TestParseAutoFilterRecordCustomComparisonFilter(t *testing.T) {
	var d1, d2 [8]byte
	binary.LittleEndian.PutUint64(d1[:], math.Float64bits(7.5))
	payload := makeAutoFilterPayload(0, 0, 0x02, 0x04, d1, 0x00, 0x00, d2, 0, "", "")
	fc := parseAutoFilterRecord(payload)
	if fc == nil {
		t.Fatal("expected non-nil FilterColumn")
	}
	if len(fc.CustomFilters) != 1 {
		t.Fatalf("CustomFilters len = %d, want 1", len(fc.CustomFilters))
	}
	cf := fc.CustomFilters[0]
	if cf.Operator != "greaterThan" {
		t.Errorf("Operator = %q, want greaterThan", cf.Operator)
	}
	if cf.Val != "7.5" {
		t.Errorf("Val = %q, want 7.5", cf.Val)
	}
}

func TestParseAutoFilterRecordTwoConditionAnd(t *testing.T) {
	var d1, d2 [8]byte
	binary.LittleEndian.PutUint64(d1[:], math.Float64bits(5))
	binary.LittleEndian.PutUint64(d2[:], math.Float64bits(10))
	payload := makeAutoFilterPayload(1, 0, 0x02, 0x06, d1, 0x02, 0x03, d2, 1, "", "")
	fc := parseAutoFilterRecord(payload)
	if fc == nil {
		t.Fatal("expected non-nil FilterColumn")
	}
	if len(fc.CustomFilters) != 2 {
		t.Fatalf("CustomFilters len = %d, want 2", len(fc.CustomFilters))
	}
	if !fc.CustomAnd {
		t.Errorf("CustomAnd = false, want true")
	}
	if fc.CustomFilters[0].Operator != "greaterThanOrEqual" {
		t.Errorf("cond1 op = %q, want greaterThanOrEqual", fc.CustomFilters[0].Operator)
	}
	if fc.CustomFilters[1].Operator != "lessThanOrEqual" {
		t.Errorf("cond2 op = %q, want lessThanOrEqual", fc.CustomFilters[1].Operator)
	}
}

func TestParseAutoFilterRecordTopNFilter(t *testing.T) {
	grbit := uint16(0x0004) | (1 << 3) | (5 << 5)
	var d1, d2 [8]byte
	payload := makeAutoFilterPayload(3, grbit, 0x00, 0x00, d1, 0x00, 0x00, d2, 0, "", "")
	fc := parseAutoFilterRecord(payload)
	if fc == nil {
		t.Fatal("expected non-nil FilterColumn for top-N")
	}
	if fc.ColIndex != 3 {
		t.Errorf("ColIndex = %d, want 3", fc.ColIndex)
	}
	if fc.Top10 == nil {
		t.Fatal("Top10 is nil")
	}
	if !fc.Top10.Top {
		t.Errorf("Top10.Top = false, want true")
	}
	if fc.Top10.Val != 5 {
		t.Errorf("Top10.Val = %v, want 5", fc.Top10.Val)
	}
}

func TestParseAutoFilterRecordBlankFilter(t *testing.T) {
	var d1, d2 [8]byte
	payload := makeAutoFilterPayload(0, 0, 0x0E, 0x02, d1, 0x00, 0x00, d2, 0, "", "")
	fc := parseAutoFilterRecord(payload)
	if fc == nil {
		t.Fatal("expected non-nil FilterColumn for blank filter")
	}
	if !fc.ShowBlanks {
		t.Errorf("ShowBlanks = false, want true")
	}
}

func TestParseWorksheetCollectsAutoFilterRecords(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	afiPayload := []byte{0x01, 0x00}
	data = appendBiffRecord(data, biffAutoFilterInfo, afiPayload)
	var d1, d2 [8]byte
	afPayload := makeAutoFilterPayload(0, 0, 0x04, 0x02, d1, 0x00, 0x00, d2, 0, "Beta", "")
	data = appendBiffRecord(data, biffAutoFilter, afPayload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "AF"}, nil, []legacyXF{{}})
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if len(sheet.FilterColumns) != 1 {
		t.Fatalf("FilterColumns len = %d, want 1", len(sheet.FilterColumns))
	}
	fc := sheet.FilterColumns[0]
	if fc.ColIndex != 0 {
		t.Errorf("ColIndex = %d, want 0", fc.ColIndex)
	}
	if len(fc.Filters) != 1 || fc.Filters[0] != "Beta" {
		t.Errorf("Filters = %v, want [Beta]", fc.Filters)
	}
}

// makeDimensionsPayload builds a minimal DIMENSIONS (0x0200) record payload.
// firstRow/lastRow are 0-based; lastRow is exclusive (one past last used row).
// firstCol/lastCol are 0-based; lastCol is exclusive (one past last used col).
func makeDimensionsPayload(firstRow, lastRow uint32, firstCol, lastCol uint16) []byte {
	p := make([]byte, 14)
	binary.LittleEndian.PutUint32(p[0:4], firstRow)
	binary.LittleEndian.PutUint32(p[4:8], lastRow)
	binary.LittleEndian.PutUint16(p[8:10], firstCol)
	binary.LittleEndian.PutUint16(p[10:12], lastCol)
	// bytes 12-14 are reserved (must be 0)
	return p
}

func TestApplyDimensionsRecordPopulatesSheetRange(t *testing.T) {
	sheet := &sheetir.Sheet{}
	payload := makeDimensionsPayload(2, 10, 3, 8) // rows 3-9, cols 4-7 (0-based → 1-based inclusive)
	applyDimensionsRecord(sheet, payload)
	if sheet.DimFirstRow != 3 {
		t.Errorf("DimFirstRow = %d, want 3", sheet.DimFirstRow)
	}
	if sheet.DimLastRow != 10 {
		t.Errorf("DimLastRow = %d, want 10", sheet.DimLastRow)
	}
	if sheet.DimFirstCol != 4 {
		t.Errorf("DimFirstCol = %d, want 4", sheet.DimFirstCol)
	}
	if sheet.DimLastCol != 8 {
		t.Errorf("DimLastCol = %d, want 8", sheet.DimLastCol)
	}
}

func TestApplyDimensionsRecordEmptySheetLeavesZero(t *testing.T) {
	sheet := &sheetir.Sheet{}
	payload := makeDimensionsPayload(0, 0, 0, 0) // empty sheet
	applyDimensionsRecord(sheet, payload)
	if sheet.DimFirstRow != 1 || sheet.DimLastRow != 1 || sheet.DimFirstCol != 1 || sheet.DimLastCol != 1 {
		t.Errorf("empty sheet: got firstRow=%d lastRow=%d firstCol=%d lastCol=%d, want all 1",
			sheet.DimFirstRow, sheet.DimLastRow, sheet.DimFirstCol, sheet.DimLastCol)
	}
}

func TestApplyDimensionsRecordRejectsShortPayload(t *testing.T) {
	sheet := &sheetir.Sheet{DimFirstRow: 5, DimLastRow: 10, DimFirstCol: 1, DimLastCol: 5}
	applyDimensionsRecord(sheet, []byte{0x00, 0x00}) // too short
	if sheet.DimFirstRow != 5 || sheet.DimLastRow != 10 {
		t.Errorf("short payload mutated sheet: firstRow=%d lastRow=%d", sheet.DimFirstRow, sheet.DimLastRow)
	}
}

func TestApplyDimensionsRecordNilSheetNoPanic(t *testing.T) {
	applyDimensionsRecord(nil, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
}

func TestApplyFilterModeRecordSetsFilterMode(t *testing.T) {
	sheet := &sheetir.Sheet{}
	applyFilterModeRecord(sheet)
	if !sheet.FilterMode {
		t.Error("FilterMode = false, want true")
	}
}

func TestApplyFilterModeRecordNilSheetNoPanic(t *testing.T) {
	applyFilterModeRecord(nil)
}

func TestParseWorksheetCapturesDimensionsRecord(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	dimPayload := makeDimensionsPayload(0, 5, 0, 3) // rows 1-4, cols 1-2 (0-based → 1-based inclusive)
	data = appendBiffRecord(data, biffDimensions, dimPayload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Dim"}, nil, []legacyXF{{}})
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if sheet.DimFirstRow != 1 {
		t.Errorf("DimFirstRow = %d, want 1", sheet.DimFirstRow)
	}
	if sheet.DimLastRow != 5 {
		t.Errorf("DimLastRow = %d, want 5", sheet.DimLastRow)
	}
	if sheet.DimFirstCol != 1 {
		t.Errorf("DimFirstCol = %d, want 1", sheet.DimFirstCol)
	}
	if sheet.DimLastCol != 3 {
		t.Errorf("DimLastCol = %d, want 3", sheet.DimLastCol)
	}
}

func TestParseWorksheetCapturesFilterModeRecord(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffFilterMode, nil)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "FM"}, nil, []legacyXF{{}})
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if !sheet.FilterMode {
		t.Error("FilterMode = false, want true after FILTERMODE record")
	}
}

func TestParseWorksheetCapturesFilterModeWithAutoFilter(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffFilterMode, nil)
	afiPayload := []byte{0x01, 0x00}
	data = appendBiffRecord(data, biffAutoFilterInfo, afiPayload)
	var d1, d2 [8]byte
	afPayload := makeAutoFilterPayload(0, 0, 0x04, 0x02, d1, 0x00, 0x00, d2, 0, "Alpha", "")
	data = appendBiffRecord(data, biffAutoFilter, afPayload)
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "AF+FM"}, nil, []legacyXF{{}})
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	if !sheet.FilterMode {
		t.Error("FilterMode = false, want true")
	}
	if len(sheet.FilterColumns) != 1 {
		t.Fatalf("FilterColumns len = %d, want 1", len(sheet.FilterColumns))
	}
	if len(sheet.FilterColumns[0].Filters) != 1 || sheet.FilterColumns[0].Filters[0] != "Alpha" {
		t.Errorf("Filters = %v, want [Alpha]", sheet.FilterColumns[0].Filters)
	}
}

// TestParseWorksheetSkipsPageSettingsBlockRecords verifies that the
// PageSettingsBlock records PRINTSIZE (0x0033), PLS (0x004D), and BITMAP
// (0x00E9) are silently skipped without error, matching POI's UnknownRecord
// pass-through behavior.  These records carry opaque printer-specific data
// that POI stores and re-serializes but never interprets.
func TestParseWorksheetSkipsPageSettingsBlockRecords(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	// PRINTSIZE (0x0033) — typical payload { 0x03, 0x00 } per OOO source.
	data = appendBiffRecord(data, biffPrintSize, []byte{0x03, 0x00})
	// PLS (0x004D) — environment-specific print settings.
	data = appendBiffRecord(data, biffPLS, []byte{0x01, 0x00, 0x02, 0x00, 0x03, 0x00})
	// BITMAP (0x00E9) — print bitmap.
	data = appendBiffRecord(data, biffBitmap, []byte{0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04, 0x00})
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "PSB"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	// The sheet should be empty but valid — no panic, no error.
	if len(sheet.Rows) != 0 {
		t.Errorf("Rows = %d, want 0", len(sheet.Rows))
	}
}

// TestParseWorksheetSkipsIndexAndDBCellRecords verifies that the INDEX
// (0x020B) and DBCELL (0x00D7) records are silently skipped without error,
// matching POI's UnknownRecord pass-through behavior.  These records carry
// row-block and cell-block offset indices that POI uses for offset validation
// but are safe to ignore for parsing.
// ---------------------------------------------------------------------------
// CodepageRecord (0x0042) tests
// ---------------------------------------------------------------------------

func TestParseCodepageRecordReturnsZeroForShortPayload(t *testing.T) {
	if got := parseCodepageRecord(nil); got != 0 {
		t.Errorf("parseCodepageRecord(nil) = %d, want 0", got)
	}
	if got := parseCodepageRecord([]byte{0x01}); got != 0 {
		t.Errorf("parseCodepageRecord(short) = %d, want 0", got)
	}
}

func TestParseCodepageRecordReturnsCodePage(t *testing.T) {
	// 0x04E4 = 1252 (Windows Latin 1 / ANSI)
	payload := []byte{0xE4, 0x04}
	if got := parseCodepageRecord(payload); got != 1252 {
		t.Errorf("parseCodepageRecord = %d, want 1252", got)
	}
	// 0x01B5 = 437 (IBM PC / US)
	payload = []byte{0xB5, 0x01}
	if got := parseCodepageRecord(payload); got != 437 {
		t.Errorf("parseCodepageRecord = %d, want 437", got)
	}
}

func TestParseCodepageRecordReturnsMacintoshCodePage(t *testing.T) {
	// 0x8000 = 32768 (Apple Macintosh)
	payload := []byte{0x00, 0x80}
	if got := parseCodepageRecord(payload); got != 32768 {
		t.Errorf("parseCodepageRecord = %d, want 32768", got)
	}
}

// ---------------------------------------------------------------------------
// CountryRecord (0x008C) tests
// ---------------------------------------------------------------------------

func TestParseCountryRecordReturnsZeroForShortPayload(t *testing.T) {
	def, cur := parseCountryRecord(nil)
	if def != 0 || cur != 0 {
		t.Errorf("parseCountryRecord(nil) = (%d,%d), want (0,0)", def, cur)
	}
	def, cur = parseCountryRecord([]byte{0x01})
	if def != 0 || cur != 0 {
		t.Errorf("parseCountryRecord(short) = (%d,%d), want (0,0)", def, cur)
	}
}

func TestParseCountryRecordReturnsDefaultCountry(t *testing.T) {
	// 0x0001 = USA
	payload := []byte{0x01, 0x00}
	def, cur := parseCountryRecord(payload)
	if def != 1 {
		t.Errorf("defaultCountry = %d, want 1", def)
	}
	if cur != 1 {
		t.Errorf("currentCountry = %d, want 1 (default fallback)", cur)
	}
}

func TestParseCountryRecordReturnsDefaultAndCurrent(t *testing.T) {
	// default=0x0001 (USA), current=0x0033 (France)
	payload := []byte{0x01, 0x00, 0x33, 0x00}
	def, cur := parseCountryRecord(payload)
	if def != 1 {
		t.Errorf("defaultCountry = %d, want 1", def)
	}
	if cur != 51 {
		t.Errorf("currentCountry = %d, want 51 (0x0033)", cur)
	}
}

func TestParseCountryRecordReturnsJapan(t *testing.T) {
	// 0x00F0 = Japan
	payload := []byte{0xF0, 0x00}
	def, cur := parseCountryRecord(payload)
	if def != 240 {
		t.Errorf("defaultCountry = %d, want 240", def)
	}
	if cur != 240 {
		t.Errorf("currentCountry = %d, want 240", cur)
	}
}

// ---------------------------------------------------------------------------
// Workbook globals integration tests for CodepageRecord and CountryRecord
// ---------------------------------------------------------------------------

func TestParseWorkbookGlobalsSkipsCodepageRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		return append(b, payload...)
	}
	var b []byte
	b = writeRecord(b, biffBOF, make([]byte, 16))
	// CODEPAGE record with 1252 (Windows Latin 1)
	b = writeRecord(b, biffCodepage, []byte{0xE4, 0x04})
	b = writeRecord(b, biffEOF, nil)

	_, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(b)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals with CODEPAGE: %v", err)
	}
}

func TestParseWorkbookGlobalsSkipsCountryRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		return append(b, payload...)
	}
	var b []byte
	b = writeRecord(b, biffBOF, make([]byte, 16))
	// COUNTRY record with USA default, France current
	b = writeRecord(b, biffCountry, []byte{0x01, 0x00, 0x33, 0x00})
	b = writeRecord(b, biffEOF, nil)

	_, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(b)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals with COUNTRY: %v", err)
	}
}

func TestParseWorkbookGlobalsSkipsCodepageAndCountryRecords(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		return append(b, payload...)
	}
	var b []byte
	b = writeRecord(b, biffBOF, make([]byte, 16))
	// Both CODEPAGE and COUNTRY records
	b = writeRecord(b, biffCodepage, []byte{0xE4, 0x04})
	b = writeRecord(b, biffCountry, []byte{0x01, 0x00, 0x33, 0x00})
	b = writeRecord(b, biffEOF, nil)

	_, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(b)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals with CODEPAGE+COUNTRY: %v", err)
	}
}

func TestParseWorkbookGlobalsSkipsCodepageRecordWithMacintosh(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		return append(b, payload...)
	}
	var b []byte
	b = writeRecord(b, biffBOF, make([]byte, 16))
	// CODEPAGE record with Macintosh code page
	b = writeRecord(b, biffCodepage, []byte{0x00, 0x80})
	b = writeRecord(b, biffEOF, nil)

	_, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(b)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals with Macintosh CODEPAGE: %v", err)
	}
}

func TestParseWorkbookGlobalsSkipsCountryRecordWithJapan(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], id)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(payload)))
		b = append(b, hdr[:]...)
		return append(b, payload...)
	}
	var b []byte
	b = writeRecord(b, biffBOF, make([]byte, 16))
	// COUNTRY record with Japan
	b = writeRecord(b, biffCountry, []byte{0xF0, 0x00})
	b = writeRecord(b, biffEOF, nil)

	_, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(b)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals with Japan COUNTRY: %v", err)
	}
}

func TestParseWindow1Record(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    int
	}{
		{
			name:    "empty payload",
			payload: nil,
			want:    0,
		},
		{
			name:    "short payload",
			payload: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			want:    0,
		},
		{
			name:    "active sheet 0",
			payload: []byte{0, 0, 0, 0, 100, 0, 200, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			want:    0,
		},
		{
			name:    "active sheet 2",
			payload: []byte{0, 0, 0, 0, 100, 0, 200, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0},
			want:    2,
		},
		{
			name:    "active sheet 5",
			payload: []byte{0, 0, 0, 0, 100, 0, 200, 0, 0, 0, 5, 0, 0, 0, 0, 0, 0, 0},
			want:    5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWindow1Record(tt.payload)
			if got != tt.want {
				t.Errorf("parseWindow1Record = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseWriteAccessRecord(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    string
	}{
		{
			name:    "empty payload",
			payload: nil,
			want:    "",
		},
		{
			name:    "short payload",
			payload: []byte{0, 0},
			want:    "",
		},
		{
			name: "compressed username",
			payload: func() []byte {
				// cch=5, fHighByte=0, then "admin" padded with spaces
				b := []byte{5, 0, 0}
				b = append(b, []byte("admin")...)
				for len(b) < 112 {
					b = append(b, ' ')
				}
				return b
			}(),
			want: "admin",
		},
		{
			name: "unicode username",
			payload: func() []byte {
				// cch=5, fHighByte=1, then UTF-16LE "admin"
				b := []byte{5, 0, 1}
				b = append(b, []byte("a\x00d\x00m\x00i\x00n\x00")...)
				for len(b) < 112 {
					b = append(b, ' ')
				}
				return b
			}(),
			want: "admin",
		},
		{
			name: "zero char count",
			payload: func() []byte {
				b := []byte{0, 0, 0}
				for len(b) < 112 {
					b = append(b, ' ')
				}
				return b
			}(),
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWriteAccessRecord(tt.payload)
			if got != tt.want {
				t.Errorf("parseWriteAccessRecord = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseBackupRecord(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"empty payload", nil, false},
		{"short payload", []byte{0}, false},
		{"backup off", []byte{0, 0}, false},
		{"backup on", []byte{1, 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBackupRecord(tt.payload)
			if got != tt.want {
				t.Errorf("parseBackupRecord = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseBookBoolRecord(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"empty payload", nil, false},
		{"short payload", []byte{0}, false},
		{"save off", []byte{0, 0}, false},
		{"save on", []byte{1, 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBookBoolRecord(tt.payload)
			if got != tt.want {
				t.Errorf("parseBookBoolRecord = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCalcModeRecord(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    int
	}{
		{"empty payload", nil, 1},
		{"short payload", []byte{0}, 1},
		{"manual", []byte{0, 0}, 0},
		{"automatic", []byte{1, 0}, 1},
		{"auto except tables", []byte{0xFF, 0xFF}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCalcModeRecord(tt.payload)
			if got != tt.want {
				t.Errorf("parseCalcModeRecord = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseCalcCountRecord(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    int
	}{
		{"empty payload", nil, 100},
		{"short payload", []byte{0}, 100},
		{"count 10", []byte{10, 0}, 10},
		{"count 100", []byte{100, 0}, 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCalcCountRecord(tt.payload)
			if got != tt.want {
				t.Errorf("parseCalcCountRecord = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseRefreshAllRecord(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"empty payload", nil, false},
		{"short payload", []byte{0}, false},
		{"refresh off", []byte{0, 0}, false},
		{"refresh on", []byte{1, 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRefreshAllRecord(tt.payload)
			if got != tt.want {
				t.Errorf("parseRefreshAllRecord = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseIterationRecord(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"empty payload", nil, false},
		{"short payload", []byte{0}, false},
		{"iteration off", []byte{0, 0}, false},
		{"iteration on", []byte{1, 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIterationRecord(tt.payload)
			if got != tt.want {
				t.Errorf("parseIterationRecord = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDeltaRecord(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    float64
	}{
		{"empty payload", nil, 0.001},
		{"short payload", []byte{0, 0, 0, 0, 0, 0, 0}, 0.001},
		{"delta 0.001", func() []byte {
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, math.Float64bits(0.001))
			return b
		}(), 0.001},
		{"delta 0.01", func() []byte {
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, math.Float64bits(0.01))
			return b
		}(), 0.01},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDeltaRecord(tt.payload)
			if math.Abs(got-tt.want) > 1e-12 {
				t.Errorf("parseDeltaRecord = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePrecisionRecord(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"empty payload", nil, false},
		{"short payload", []byte{0}, false},
		{"full precision", []byte{0, 0}, false},
		{"precision as displayed", []byte{1, 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePrecisionRecord(tt.payload)
			if got != tt.want {
				t.Errorf("parsePrecisionRecord = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSaveRecalcRecord(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"empty payload", nil, false},
		{"short payload", []byte{0}, false},
		{"recalc off", []byte{0, 0}, false},
		{"recalc on", []byte{1, 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSaveRecalcRecord(tt.payload)
			if got != tt.want {
				t.Errorf("parseSaveRecalcRecord = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseWorkbookGlobalsSkipsWindow1Record(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		return appendBiffRecord(b, id, payload)
	}
	data := writeRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	// WINDOW1 record with active sheet index 2
	data = writeRecord(data, biffWindow1, []byte{
		0x00, 0x00, // h_hold
		0x00, 0x00, // v_hold
		0x64, 0x00, // width
		0xC8, 0x00, // height
		0x00, 0x00, // options
		0x02, 0x00, // active_sheet = 2
		0x00, 0x00, // first_visible_tab
		0x00, 0x00, // num_selected_tabs
		0x00, 0x00, // tab_width_ratio
	})
	data = writeRecord(data, biffEOF, nil)
	sheets, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(sheets) != 0 {
		t.Errorf("sheets = %d, want 0", len(sheets))
	}
}

func TestParseWorkbookGlobalsSkipsWriteAccessRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		return appendBiffRecord(b, id, payload)
	}
	data := writeRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	// WRITEACCESS record with username "admin"
	waPayload := []byte{4, 0, 0}
	waPayload = append(waPayload, []byte("admin")...)
	for len(waPayload) < 112 {
		waPayload = append(waPayload, ' ')
	}
	data = writeRecord(data, biffWriteAccess, waPayload)
	data = writeRecord(data, biffEOF, nil)
	sheets, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(sheets) != 0 {
		t.Errorf("sheets = %d, want 0", len(sheets))
	}
}

func TestParseWorkbookGlobalsSkipsBackupRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		return appendBiffRecord(b, id, payload)
	}
	data := writeRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = writeRecord(data, biffBackup, []byte{1, 0})
	data = writeRecord(data, biffEOF, nil)
	sheets, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(sheets) != 0 {
		t.Errorf("sheets = %d, want 0", len(sheets))
	}
}

func TestParseWorkbookGlobalsSkipsBookBoolRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		return appendBiffRecord(b, id, payload)
	}
	data := writeRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = writeRecord(data, biffBookBool, []byte{1, 0})
	data = writeRecord(data, biffEOF, nil)
	sheets, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(sheets) != 0 {
		t.Errorf("sheets = %d, want 0", len(sheets))
	}
}

func TestParseWorkbookGlobalsSkipsCalcModeRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		return appendBiffRecord(b, id, payload)
	}
	data := writeRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = writeRecord(data, biffCalcMode, []byte{0, 0}) // manual
	data = writeRecord(data, biffEOF, nil)
	sheets, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(sheets) != 0 {
		t.Errorf("sheets = %d, want 0", len(sheets))
	}
}

func TestParseWorkbookGlobalsSkipsCalcCountRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		return appendBiffRecord(b, id, payload)
	}
	data := writeRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = writeRecord(data, biffCalcCount, []byte{10, 0})
	data = writeRecord(data, biffEOF, nil)
	sheets, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(sheets) != 0 {
		t.Errorf("sheets = %d, want 0", len(sheets))
	}
}

func TestParseWorkbookGlobalsSkipsRefreshAllRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		return appendBiffRecord(b, id, payload)
	}
	data := writeRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = writeRecord(data, biffRefreshAll, []byte{1, 0})
	data = writeRecord(data, biffEOF, nil)
	sheets, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(sheets) != 0 {
		t.Errorf("sheets = %d, want 0", len(sheets))
	}
}

func TestParseWorkbookGlobalsSkipsIterationRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		return appendBiffRecord(b, id, payload)
	}
	data := writeRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = writeRecord(data, biffIteration, []byte{1, 0})
	data = writeRecord(data, biffEOF, nil)
	sheets, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(sheets) != 0 {
		t.Errorf("sheets = %d, want 0", len(sheets))
	}
}

func TestParseWorkbookGlobalsSkipsDeltaRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		return appendBiffRecord(b, id, payload)
	}
	data := writeRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	deltaPayload := make([]byte, 8)
	binary.LittleEndian.PutUint64(deltaPayload, math.Float64bits(0.01))
	data = writeRecord(data, biffDelta, deltaPayload)
	data = writeRecord(data, biffEOF, nil)
	sheets, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(sheets) != 0 {
		t.Errorf("sheets = %d, want 0", len(sheets))
	}
}

func TestParseWorkbookGlobalsSkipsPrecisionRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		return appendBiffRecord(b, id, payload)
	}
	data := writeRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = writeRecord(data, biffPrecision, []byte{1, 0})
	data = writeRecord(data, biffEOF, nil)
	sheets, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(sheets) != 0 {
		t.Errorf("sheets = %d, want 0", len(sheets))
	}
}

func TestParseWorkbookGlobalsSkipsSaveRecalcRecord(t *testing.T) {
	writeRecord := func(b []byte, id uint16, payload []byte) []byte {
		return appendBiffRecord(b, id, payload)
	}
	data := writeRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = writeRecord(data, biffSaveRecalc, []byte{1, 0})
	data = writeRecord(data, biffEOF, nil)
	sheets, _, _, _, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals: %v", err)
	}
	if len(sheets) != 0 {
		t.Errorf("sheets = %d, want 0", len(sheets))
	}
}

func TestParseLegacyWorkbookMetadataCapturesWorkbookGlobals(t *testing.T) {
	window1 := make([]byte, 18)
	binary.LittleEndian.PutUint16(window1[10:12], 2)
	writeAccess := []byte{5, 0, 0}
	writeAccess = append(writeAccess, []byte("admin")...)
	for len(writeAccess) < 112 {
		writeAccess = append(writeAccess, ' ')
	}
	delta := make([]byte, 8)
	binary.LittleEndian.PutUint64(delta, math.Float64bits(0.25))

	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	data = appendBiffRecord(data, biffWindow1, window1)
	data = appendBiffRecord(data, biffWriteAccess, writeAccess)
	data = appendBiffRecord(data, biffBackup, []byte{1, 0})
	data = appendBiffRecord(data, biffBookBool, []byte{1, 0})
	data = appendBiffRecord(data, biffCalcMode, []byte{0, 0})
	data = appendBiffRecord(data, biffCalcCount, []byte{9, 0})
	data = appendBiffRecord(data, biffRefreshAll, []byte{1, 0})
	data = appendBiffRecord(data, biffIteration, []byte{1, 0})
	data = appendBiffRecord(data, biffDelta, delta)
	data = appendBiffRecord(data, biffPrecision, []byte{1, 0})
	data = appendBiffRecord(data, biffSaveRecalc, []byte{0, 0})
	data = appendBiffRecord(data, biffEOF, nil)

	meta := parseLegacyWorkbookMetadata(data)
	if meta.ActiveSheetIndex == nil || *meta.ActiveSheetIndex != 2 {
		t.Fatalf("ActiveSheetIndex = %v, want 2", meta.ActiveSheetIndex)
	}
	if meta.WriteAccessUser != "admin" {
		t.Fatalf("WriteAccessUser = %q, want admin", meta.WriteAccessUser)
	}
	if meta.BackupFile == nil || !*meta.BackupFile {
		t.Fatalf("BackupFile = %v, want true", meta.BackupFile)
	}
	if meta.SaveExternalLinkValues == nil || !*meta.SaveExternalLinkValues {
		t.Fatalf("SaveExternalLinkValues = %v, want true", meta.SaveExternalLinkValues)
	}
	if meta.Calc == nil {
		t.Fatal("Calc = nil, want non-nil")
	}
	if meta.Calc.Mode == nil || *meta.Calc.Mode != 0 {
		t.Fatalf("Calc.Mode = %v, want 0", meta.Calc.Mode)
	}
	if meta.Calc.IterationCount == nil || *meta.Calc.IterationCount != 9 {
		t.Fatalf("Calc.IterationCount = %v, want 9", meta.Calc.IterationCount)
	}
	if meta.Calc.RefreshAll == nil || !*meta.Calc.RefreshAll {
		t.Fatalf("Calc.RefreshAll = %v, want true", meta.Calc.RefreshAll)
	}
	if meta.Calc.Iterate == nil || !*meta.Calc.Iterate {
		t.Fatalf("Calc.Iterate = %v, want true", meta.Calc.Iterate)
	}
	if meta.Calc.Delta == nil || *meta.Calc.Delta != 0.25 {
		t.Fatalf("Calc.Delta = %v, want 0.25", meta.Calc.Delta)
	}
	if meta.Calc.PrecisionAsDisplayed == nil || !*meta.Calc.PrecisionAsDisplayed {
		t.Fatalf("Calc.PrecisionAsDisplayed = %v, want true", meta.Calc.PrecisionAsDisplayed)
	}
	if meta.Calc.SaveRecalc == nil || *meta.Calc.SaveRecalc {
		t.Fatalf("Calc.SaveRecalc = %v, want false", meta.Calc.SaveRecalc)
	}
}

func TestParseWorkbookPreservesWorkbookGlobalsAndHiddenSheet(t *testing.T) {
	buildBoundSheetPayload := func(offset uint32, state byte, name string) []byte {
		payload := make([]byte, 8+len(name))
		binary.LittleEndian.PutUint32(payload[0:4], offset)
		payload[4] = state
		payload[5] = 0x00
		payload[6] = byte(len(name))
		payload[7] = 0x00
		copy(payload[8:], []byte(name))
		return payload
	}
	buildWorkbookGlobals := func(boundSheetPayload []byte) []byte {
		window1 := make([]byte, 18)
		binary.LittleEndian.PutUint16(window1[10:12], 0)
		data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
		data = appendBiffRecord(data, biffWindow1, window1)
		data = appendBiffRecord(data, biffBoundSheet, boundSheetPayload)
		data = appendBiffRecord(data, biffEOF, nil)
		return data
	}

	sheetStream := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	sheetStream = appendBiffRecord(sheetStream, biffEOF, nil)
	workbookGlobals := buildWorkbookGlobals(buildBoundSheetPayload(0, 0x01, "Hidden"))
	workbookGlobals = buildWorkbookGlobals(buildBoundSheetPayload(uint32(len(workbookGlobals)), 0x01, "Hidden"))
	data := append(workbookGlobals, sheetStream...)

	wb, err := parseWorkbook(data, "hidden.xls")
	if err != nil {
		t.Fatalf("parseWorkbook: %v", err)
	}
	if wb.ActiveSheetIndex == nil || *wb.ActiveSheetIndex != 0 {
		t.Fatalf("ActiveSheetIndex = %v, want 0", wb.ActiveSheetIndex)
	}
	if len(wb.Sheets) != 1 {
		t.Fatalf("len(Sheets) = %d, want 1", len(wb.Sheets))
	}
	if !wb.Sheets[0].Hidden {
		t.Fatal("Sheets[0].Hidden = false, want true")
	}
	if wb.Sheets[0].Visibility != "hidden" {
		t.Fatalf("Sheets[0].Visibility = %q, want hidden", wb.Sheets[0].Visibility)
	}
}

func TestParseWorkbookPreservesVeryHiddenSheet(t *testing.T) {
	buildBoundSheetPayload := func(offset uint32, state byte, name string) []byte {
		payload := make([]byte, 8+len(name))
		binary.LittleEndian.PutUint32(payload[0:4], offset)
		payload[4] = state
		payload[5] = 0x00
		payload[6] = byte(len(name))
		payload[7] = 0x00
		copy(payload[8:], []byte(name))
		return payload
	}
	buildWorkbookGlobals := func(boundSheetPayload []byte) []byte {
		window1 := make([]byte, 18)
		binary.LittleEndian.PutUint16(window1[10:12], 0)
		data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
		data = appendBiffRecord(data, biffWindow1, window1)
		data = appendBiffRecord(data, biffBoundSheet, boundSheetPayload)
		data = appendBiffRecord(data, biffEOF, nil)
		return data
	}

	sheetStream := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	sheetStream = appendBiffRecord(sheetStream, biffEOF, nil)
	workbookGlobals := buildWorkbookGlobals(buildBoundSheetPayload(0, 0x02, "Secret"))
	workbookGlobals = buildWorkbookGlobals(buildBoundSheetPayload(uint32(len(workbookGlobals)), 0x02, "Secret"))
	data := append(workbookGlobals, sheetStream...)

	wb, err := parseWorkbook(data, "very-hidden.xls")
	if err != nil {
		t.Fatalf("parseWorkbook: %v", err)
	}
	if len(wb.Sheets) != 1 {
		t.Fatalf("len(Sheets) = %d, want 1", len(wb.Sheets))
	}
	if !wb.Sheets[0].Hidden {
		t.Fatal("Sheets[0].Hidden = false, want true")
	}
	if wb.Sheets[0].Visibility != "veryHidden" {
		t.Fatalf("Sheets[0].Visibility = %q, want veryHidden", wb.Sheets[0].Visibility)
	}
}

func TestParseWorksheetSkipsIndexAndDBCellRecords(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{0x10, 0x00, 0x06, 0x00})
	// INDEX (0x020B) — row-block index; typical payload includes
	// reserved(4) + nRows(4) + nBlock(4) + blockOffsets(nBlock*4).
	data = appendBiffRecord(data, biffIndex, []byte{
		0x00, 0x00, 0x00, 0x00, // reserved
		0x00, 0x00, 0x00, 0x00, // nRows (0 = no rows)
		0x01, 0x00, 0x00, 0x00, // nBlock (1 block)
		0x00, 0x00, 0x00, 0x00, // block offset
	})
	// DBCELL (0x00D7) — cell-block offset index; typical payload includes
	// rowOffset(4) + cellOffsets(variable).
	data = appendBiffRecord(data, biffDBCell, []byte{
		0x00, 0x00, 0x00, 0x00, // row offset
	})
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "IndexDBCell"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet")
	}
	// The sheet should be empty but valid — no panic, no error.
	if len(sheet.Rows) != 0 {
		t.Errorf("Rows = %d, want 0", len(sheet.Rows))
	}
}

// --- BOFRecord tests (HSSF BOFRecord adoption) ---

func TestParseBIFFBOFDecodesVersionAndType(t *testing.T) {
	// BIFF8 BOF: version=0x0600, type=0x10 (worksheet), build=0x1E11, buildYear=1997,
	// history=0x00000001, rversion=0x00000600
	payload := []byte{
		0x00, 0x06, // version = BIFF8
		0x10, 0x00, // type = worksheet
		0x11, 0x1E, // build
		0xC9, 0x07, // build year = 0x07C9 = 1993 (LE)
		0x01, 0x00, 0x00, 0x00, // history
		0x00, 0x06, 0x00, 0x00, // required version
	}
	version, typ, build, buildYear, history, rversion := parseBIFFBOF(payload)
	if version != biffVersion8 {
		t.Errorf("version = 0x%04X, want 0x%04X (BIFF8)", version, biffVersion8)
	}
	if typ != biffBOFTypeWorksheet {
		t.Errorf("type = 0x%04X, want 0x%04X (worksheet)", typ, biffBOFTypeWorksheet)
	}
	if build != 0x1E11 {
		t.Errorf("build = 0x%04X, want 0x1E11", build)
	}
	if buildYear != 1993 {
		t.Errorf("buildYear = %d, want 1993", buildYear)
	}
	if history != 1 {
		t.Errorf("history = %d, want 1", history)
	}
	if rversion != 0x0600 {
		t.Errorf("rversion = 0x%08X, want 0x0600", rversion)
	}
}

func TestParseBIFFBOFHandlesShortPayload(t *testing.T) {
	// Only 2 bytes: version only, no type.
	// parseBIFFBOF returns all zeros for payloads < 4 bytes.
	payload := []byte{0x00, 0x06}
	version, typ, build, buildYear, history, rversion := parseBIFFBOF(payload)
	if version != 0 {
		t.Errorf("version = 0x%04X, want 0 (payload < 4 bytes)", version)
	}
	if typ != 0 {
		t.Errorf("type = 0x%04X, want 0", typ)
	}
	if build != 0 {
		t.Errorf("build = %d, want 0", build)
	}
	if buildYear != 0 {
		t.Errorf("buildYear = %d, want 0", buildYear)
	}
	if history != 0 {
		t.Errorf("history = %d, want 0", history)
	}
	if rversion != 0 {
		t.Errorf("rversion = %d, want 0", rversion)
	}
}

func TestParseBIFFBOFHandlesPartialFields(t *testing.T) {
	// 6 bytes: version + type + build, no build year or beyond.
	payload := []byte{
		0x00, 0x06, // version = BIFF8
		0x10, 0x00, // type = worksheet
		0x11, 0x1E, // build
	}
	version, typ, build, buildYear, history, rversion := parseBIFFBOF(payload)
	if version != biffVersion8 {
		t.Errorf("version = 0x%04X, want 0x%04X", version, biffVersion8)
	}
	if typ != biffBOFTypeWorksheet {
		t.Errorf("type = 0x%04X, want 0x%04X", typ, biffBOFTypeWorksheet)
	}
	if build != 0x1E11 {
		t.Errorf("build = 0x%04X, want 0x1E11", build)
	}
	if buildYear != 0 {
		t.Errorf("buildYear = %d, want 0", buildYear)
	}
	if history != 0 {
		t.Errorf("history = %d, want 0", history)
	}
	if rversion != 0 {
		t.Errorf("rversion = %d, want 0", rversion)
	}
}

func TestParseBIFFBOFHandlesEmptyPayload(t *testing.T) {
	version, typ, build, buildYear, history, rversion := parseBIFFBOF(nil)
	if version != 0 || typ != 0 || build != 0 || buildYear != 0 || history != 0 || rversion != 0 {
		t.Errorf("expected all zeros, got version=0x%04X type=0x%04X build=%d buildYear=%d history=%d rversion=%d",
			version, typ, build, buildYear, history, rversion)
	}
}

func TestParseBIFFBOFHandlesChartType(t *testing.T) {
	payload := []byte{
		0x00, 0x06, // version = BIFF8
		0x20, 0x00, // type = chart
	}
	_, typ, _, _, _, _ := parseBIFFBOF(payload)
	if typ != biffBOFTypeChart {
		t.Errorf("type = 0x%04X, want 0x%04X (chart)", typ, biffBOFTypeChart)
	}
}

func TestParseBIFFBOFHandlesVBModuleType(t *testing.T) {
	payload := []byte{
		0x00, 0x06, // version = BIFF8
		0x06, 0x00, // type = VBA module
	}
	_, typ, _, _, _, _ := parseBIFFBOF(payload)
	if typ != biffBOFTypeVBModule {
		t.Errorf("type = 0x%04X, want 0x%04X (VBA module)", typ, biffBOFTypeVBModule)
	}
}

func TestParseBIFFBOFHandlesExcel4MacroType(t *testing.T) {
	payload := []byte{
		0x00, 0x06, // version = BIFF8
		0x40, 0x00, // type = Excel 4 macro
	}
	_, typ, _, _, _, _ := parseBIFFBOF(payload)
	if typ != biffBOFTypeExcel4Macro {
		t.Errorf("type = 0x%04X, want 0x%04X (Excel 4 macro)", typ, biffBOFTypeExcel4Macro)
	}
}

func TestParseBIFFBOFHandlesWorkspaceFileType(t *testing.T) {
	payload := []byte{
		0x00, 0x06, // version = BIFF8
		0x00, 0x01, // type = workspace file
	}
	_, typ, _, _, _, _ := parseBIFFBOF(payload)
	if typ != biffBOFTypeWorkspaceFile {
		t.Errorf("type = 0x%04X, want 0x%04X (workspace file)", typ, biffBOFTypeWorkspaceFile)
	}
}

func TestIsBIFFBOFRecordAcceptsBIFF3AndBIFF4(t *testing.T) {
	tests := []struct {
		sid  uint16
		want bool
	}{
		{biffBOF, true},    // BIFF5/8 BOF
		{biffBOFOld, true}, // BIFF2 BOF
		{biffBOF3, true},   // BIFF3 BOF
		{biffBOF4, true},   // BIFF4 BOF
		{0x080A, false},    // EOF
		{0x0006, false},    // FORMULA
		{0x00FC, false},    // SST
	}
	for _, tc := range tests {
		got := isBIFFBOFRecord(tc.sid)
		if got != tc.want {
			t.Errorf("isBIFFBOFRecord(0x%04X) = %v, want %v", tc.sid, got, tc.want)
		}
	}
}

func TestBiffBOFTypeName(t *testing.T) {
	tests := []struct {
		typ  uint16
		want string
	}{
		{biffBOFTypeWorkbook, "workbook"},
		{biffBOFTypeVBModule, "VBA module"},
		{biffBOFTypeWorksheet, "worksheet"},
		{biffBOFTypeChart, "chart"},
		{biffBOFTypeExcel4Macro, "Excel 4 macro"},
		{biffBOFTypeWorkspaceFile, "workspace file"},
		{0xFFFF, "unknown (0xFFFF)"},
	}
	for _, tc := range tests {
		got := biffBOFTypeName(tc.typ)
		if got != tc.want {
			t.Errorf("biffBOFTypeName(0x%04X) = %q, want %q", tc.typ, got, tc.want)
		}
	}
}

func TestParseWorkbookGlobalsAcceptsBIFF5BOF(t *testing.T) {
	// BIFF5 BOF (0x0809) with version=0x0500, type=0x05 (workbook).
	data := appendBiffRecord(nil, biffBOF, []byte{
		0x00, 0x05, // version = BIFF5
		0x05, 0x00, // type = workbook
	})
	data = appendBiffRecord(data, biffEOF, nil)

	sheets, sst, xfs, definedNames, externSheets, drawingStore, date1904, styleNames, wbProtection, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals failed for BIFF5 BOF: %v", err)
	}
	_ = sheets
	_ = sst
	_ = xfs
	_ = definedNames
	_ = externSheets
	_ = drawingStore
	_ = date1904
	_ = styleNames
	_ = wbProtection
}

func TestParseEmbeddedChartsAcceptsBIFFBOFOld(t *testing.T) {
	// BIFF2 BOF (0x0009) for chart substream.
	data := appendBiffRecord(nil, biffBOFOld, []byte{
		0x01, 0x00, // version = BIFF2
		0x20, 0x00, // type = chart
	})
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "ChartSheet"}, 0, nil, nil)
	// No chart records means no drawings, but no panic.
	if len(drawings) != 0 {
		t.Errorf("drawings = %d, want 0 (no chart records)", len(drawings))
	}
}

func TestParseChartLegendRecordDecodesPosition(t *testing.T) {
	tests := []struct {
		payload  []byte
		wantPos  string
		wantShow bool
	}{
		// Type bottom
		{[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, "bottom", true},
		// Type top
		{[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2}, "top", true},
		// Type right
		{[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3}, "right", true},
		// Type left
		{[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4}, "left", true},
		// Type corner
		{[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, "corner", true},
		// Undocked
		{[]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 7}, "right", true},
		// Short payload
		{[]byte{0, 0, 0, 0}, "none", false},
	}
	for _, tt := range tests {
		pos, show := parseChartLegendRecord(tt.payload)
		if pos != tt.wantPos || show != tt.wantShow {
			t.Errorf("parseChartLegendRecord(%x) = (%q, %v), want (%q, %v)", tt.payload, pos, show, tt.wantPos, tt.wantShow)
		}
	}
}

func TestParseChartValueRangeRecordDecodesExplicitScale(t *testing.T) {
	// Build payload: min=1.0, max=10.0, major=2.0, minor=0.5, cross=0.0, options=0x00 (none automatic)
	payload := make([]byte, 34)
	binary.LittleEndian.PutUint64(payload[0:8], math.Float64bits(1.0))
	binary.LittleEndian.PutUint64(payload[8:16], math.Float64bits(10.0))
	binary.LittleEndian.PutUint64(payload[16:24], math.Float64bits(2.0))
	binary.LittleEndian.PutUint64(payload[24:32], math.Float64bits(0.5))
	binary.LittleEndian.PutUint16(payload[32:34], 0x0000)

	min, max, major, hasMin, hasMax, hasMajor := parseChartValueRangeRecord(payload)
	if !hasMin || min != 1.0 {
		t.Errorf("min = %v, hasMin=%v, want 1.0/true", min, hasMin)
	}
	if !hasMax || max != 10.0 {
		t.Errorf("max = %v, hasMax=%v, want 10.0/true", max, hasMax)
	}
	if !hasMajor || major != 2.0 {
		t.Errorf("major = %v, hasMajor=%v, want 2.0/true", major, hasMajor)
	}
}

func TestParseChartValueRangeRecordIgnoresAutomaticScale(t *testing.T) {
	payload := make([]byte, 34)
	binary.LittleEndian.PutUint64(payload[0:8], math.Float64bits(1.0))
	binary.LittleEndian.PutUint64(payload[8:16], math.Float64bits(10.0))
	binary.LittleEndian.PutUint64(payload[16:24], math.Float64bits(2.0))
	binary.LittleEndian.PutUint16(payload[32:34], 0x0007) // auto min+max+major

	_, _, _, hasMin, hasMax, hasMajor := parseChartValueRangeRecord(payload)
	if hasMin || hasMax || hasMajor {
		t.Errorf("hasMin=%v, hasMax=%v, hasMajor=%v, want all false", hasMin, hasMax, hasMajor)
	}
}

func TestParseChartFrameRecordDetectsBorder(t *testing.T) {
	if !parseChartFrameRecord([]byte{0x00, 0x00, 0x00, 0x00}) {
		t.Error("expected regular border (type 0) to return true")
	}
	if parseChartFrameRecord([]byte{0x01, 0x00, 0x00, 0x00}) {
		t.Error("expected shadow border (type 1) to return false")
	}
	if parseChartFrameRecord([]byte{0x00}) {
		t.Error("expected short payload to return false")
	}
}

func TestParseEmbeddedChartsReadsLegendAxisAndTitleRecords(t *testing.T) {
	// Build a synthetic chart substream with legend, axis, value range, object link + title text
	data := appendBiffRecord(nil, biffBOF, []byte{
		0x00, 0x06, // version = BIFF8
		0x20, 0x00, // type = chart
	})
	data = appendBiffRecord(data, biffChart, nil)
	// Series so the chart isn't discarded
	data = appendBiffRecord(data, biffChartSeries, nil)
	// LegendRecord: position = top (type=2)
	legendPayload := make([]byte, 22)
	legendPayload[16] = 2 // TYPE_TOP
	data = appendBiffRecord(data, biffChartLegend, legendPayload)
	// AxisRecord: type = value (1)
	data = appendBiffRecord(data, biffChartAxis, []byte{0x01, 0x00, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	// ValueRangeRecord: min=5.0, max=100.0, major=10.0, options=0 (all explicit)
	vrPayload := make([]byte, 34)
	binary.LittleEndian.PutUint64(vrPayload[0:8], math.Float64bits(5.0))
	binary.LittleEndian.PutUint64(vrPayload[8:16], math.Float64bits(100.0))
	binary.LittleEndian.PutUint64(vrPayload[16:24], math.Float64bits(10.0))
	binary.LittleEndian.PutUint16(vrPayload[32:34], 0x0000)
	data = appendBiffRecord(data, biffChartValueRange, vrPayload)
	// ObjectLinkRecord: anchorId = 1 (chart title)
	data = appendBiffRecord(data, biffChartObjectLink, []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00})
	// SeriesTextRecord with "My Chart Title"
	titlePayload := []byte{0x00, 0x00, 0x0E, 0x00} // id=0, len=14, flags=0
	titlePayload = append(titlePayload, []byte("My Chart Title")...)
	data = appendBiffRecord(data, biffChartSeriesText, titlePayload)
	// FrameRecord: regular border
	data = appendBiffRecord(data, biffChartFrame, []byte{0x00, 0x00, 0x00, 0x00})
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "TestSheet"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("len(drawings) = %d, want 1", len(drawings))
	}
	chart, ok := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatalf("chart type = %T, want *chartpkg.Chart", drawings[0].Chart.Chart)
	}
	if chart.Legend.Position != "top" || !chart.Legend.Show {
		t.Errorf("legend = (%q, %v), want (top, true)", chart.Legend.Position, chart.Legend.Show)
	}
	if chart.Title != "My Chart Title" {
		t.Errorf("title = %q, want %q", chart.Title, "My Chart Title")
	}
	if chart.YAxis.Min != 5.0 {
		t.Errorf("YAxis.Min = %v, want 5.0", chart.YAxis.Min)
	}
	if chart.YAxis.Max != 100.0 {
		t.Errorf("YAxis.Max = %v, want 100.0", chart.YAxis.Max)
	}
	if chart.YAxis.MajorUnit != 10.0 {
		t.Errorf("YAxis.MajorUnit = %v, want 10.0", chart.YAxis.MajorUnit)
	}
	if !chart.PlotArea.ShowBorder {
		t.Error("PlotArea.ShowBorder = false, want true")
	}
}

func TestParseExtendedColorAutoReturnsEmpty(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:4], 0) // TYPE_AUTO
	if got := parseExtendedColor(data, nil); got != "" {
		t.Errorf("parseExtendedColor(TYPE_AUTO) = %q, want empty", got)
	}
}

func TestParseExtendedColorIndexedResolvesPalette(t *testing.T) {
	palette := &[56]string{}
	palette[0] = "FF0000" // index 8 → slot 0
	palette[1] = "00FF00" // index 9 → slot 1
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:4], 1) // TYPE_INDEXED
	binary.LittleEndian.PutUint32(data[4:8], 9) // index 9
	if got := parseExtendedColor(data, palette); got != "00FF00" {
		t.Errorf("parseExtendedColor(INDEXED 9) = %q, want 00FF00", got)
	}
}

func TestParseExtendedColorIndexedFallsBackToDefaultPalette(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:4], 1) // TYPE_INDEXED
	binary.LittleEndian.PutUint32(data[4:8], 8) // index 8
	want := defaultBIFFPalette[0]
	if got := parseExtendedColor(data, nil); got != want {
		t.Errorf("parseExtendedColor(INDEXED 8) = %q, want %q", got, want)
	}
}

func TestParseExtendedColorIndexedAutomaticReturnsEmpty(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:4], 1)  // TYPE_INDEXED
	binary.LittleEndian.PutUint32(data[4:8], 64) // automatic
	if got := parseExtendedColor(data, nil); got != "" {
		t.Errorf("parseExtendedColor(INDEXED 64) = %q, want empty", got)
	}
}

func TestParseExtendedColorRGBReturnsARGB(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:4], 2) // TYPE_RGB
	data[4] = 0x11                              // R
	data[5] = 0x22                              // G
	data[6] = 0x33                              // B
	data[7] = 0xFF                              // A
	if got := parseExtendedColor(data, nil); got != "FF112233" {
		t.Errorf("parseExtendedColor(RGB) = %q, want FF112233", got)
	}
}

func TestParseExtendedColorThemedResolvesWithTint(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:4], 3) // TYPE_THEMED
	binary.LittleEndian.PutUint32(data[4:8], 4) // theme index 4 (accent1 = 4472C4)
	// tint = 0.5 (lighten)
	binary.LittleEndian.PutUint64(data[8:16], math.Float64bits(0.5))
	got := parseExtendedColor(data, nil)
	if got == "" || got == "4472C4" {
		t.Errorf("parseExtendedColor(THEMED 4 tint=0.5) = %q, want tinted value", got)
	}
}

func TestParseExtendedColorThemedNoTint(t *testing.T) {
	data := make([]byte, 16)
	binary.LittleEndian.PutUint32(data[0:4], 3) // TYPE_THEMED
	binary.LittleEndian.PutUint32(data[4:8], 1) // theme index 1 (white)
	binary.LittleEndian.PutUint64(data[8:16], math.Float64bits(0.0))
	if got := parseExtendedColor(data, nil); got != "FFFFFF" {
		t.Errorf("parseExtendedColor(THEMED 1 tint=0) = %q, want FFFFFF", got)
	}
}

func TestParseExtendedColorShortPayload(t *testing.T) {
	if got := parseExtendedColor([]byte{1, 2, 3}, nil); got != "" {
		t.Errorf("parseExtendedColor(short) = %q, want empty", got)
	}
}

func TestApplyTintToHexLightens(t *testing.T) {
	got := applyTintToHex("808080", 0.5)
	want := "BFBFBF"
	if got != want {
		t.Errorf("applyTintToHex(808080, 0.5) = %q, want %q", got, want)
	}
}

func TestApplyTintToHexDarkens(t *testing.T) {
	got := applyTintToHex("808080", -0.5)
	want := "404040"
	if got != want {
		t.Errorf("applyTintToHex(808080, -0.5) = %q, want %q", got, want)
	}
}

func TestParseChartAxisOptionsRecordDecodesExplicitValues(t *testing.T) {
	payload := make([]byte, 18)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(1))  // minimumCategory = 1
	binary.LittleEndian.PutUint16(payload[2:4], uint16(10)) // maximumCategory = 10
	binary.LittleEndian.PutUint16(payload[4:6], uint16(2))  // majorUnitValue = 2
	binary.LittleEndian.PutUint16(payload[16:18], 0x0000)   // none automatic
	min, max, major, hasMin, hasMax, hasMajor := parseChartAxisOptionsRecord(payload)
	if !hasMin || min != 1.0 {
		t.Errorf("min = %v, hasMin=%v, want 1.0/true", min, hasMin)
	}
	if !hasMax || max != 10.0 {
		t.Errorf("max = %v, hasMax=%v, want 10.0/true", max, hasMax)
	}
	if !hasMajor || major != 2.0 {
		t.Errorf("major = %v, hasMajor=%v, want 2.0/true", major, hasMajor)
	}
}

func TestParseChartAxisOptionsRecordIgnoresAutomaticValues(t *testing.T) {
	payload := make([]byte, 18)
	binary.LittleEndian.PutUint16(payload[0:2], uint16(1))
	binary.LittleEndian.PutUint16(payload[2:4], uint16(10))
	binary.LittleEndian.PutUint16(payload[4:6], uint16(2))
	binary.LittleEndian.PutUint16(payload[16:18], 0x0007) // auto min+max+major
	_, _, _, hasMin, hasMax, hasMajor := parseChartAxisOptionsRecord(payload)
	if hasMin || hasMax || hasMajor {
		t.Errorf("hasMin=%v, hasMax=%v, hasMajor=%v, want all false", hasMin, hasMax, hasMajor)
	}
}

func TestParseEmbeddedChartsReadsCategoryAxisOptions(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{
		0x00, 0x06, // version = BIFF8
		0x20, 0x00, // type = chart
	})
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartSeries, nil)
	// AxisRecord: type = category (0)
	data = appendBiffRecord(data, biffChartAxis, []byte{0x00, 0x00, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	// AxisOptionsRecord: min=0, max=5, major=1, options=0 (all explicit)
	aoPayload := make([]byte, 18)
	binary.LittleEndian.PutUint16(aoPayload[0:2], uint16(0))
	binary.LittleEndian.PutUint16(aoPayload[2:4], uint16(5))
	binary.LittleEndian.PutUint16(aoPayload[4:6], uint16(1))
	binary.LittleEndian.PutUint16(aoPayload[16:18], 0x0000)
	data = appendBiffRecord(data, biffChartAxisOptions, aoPayload)
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "TestSheet"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("len(drawings) = %d, want 1", len(drawings))
	}
	chart := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if chart.XAxis.Min != 0.0 {
		t.Errorf("XAxis.Min = %v, want 0.0", chart.XAxis.Min)
	}
	if chart.XAxis.Max != 5.0 {
		t.Errorf("XAxis.Max = %v, want 5.0", chart.XAxis.Max)
	}
	if chart.XAxis.MajorUnit != 1.0 {
		t.Errorf("XAxis.MajorUnit = %v, want 1.0", chart.XAxis.MajorUnit)
	}
}

func TestParseWSBoolRecordDetectsDialogSheet(t *testing.T) {
	if !parseWSBoolRecord([]byte{0x10, 0x00}) {
		t.Error("expected dialog flag (bit 4) to be set")
	}
	if parseWSBoolRecord([]byte{0x0F, 0x00}) {
		t.Error("expected dialog flag to be clear")
	}
	if parseWSBoolRecord([]byte{0x00}) {
		t.Error("expected short payload to return false")
	}
}

func TestParseWorksheetSetsDialogSheetFlag(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{
		0x00, 0x06, // version = BIFF8
		0x10, 0x00, // type = worksheet
	})
	// WSBOOL with dialog flag set
	data = appendBiffRecord(data, biffWSBool, []byte{0x10, 0x00})
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Dialog1"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet, got nil")
	}
	if !sheet.DialogSheet {
		t.Error("expected sheet.DialogSheet = true")
	}
}

func TestParseWorksheetLeavesDialogSheetFalse(t *testing.T) {
	data := appendBiffRecord(nil, biffBOF, []byte{
		0x00, 0x06, // version = BIFF8
		0x10, 0x00, // type = worksheet
	})
	// WSBOOL with dialog flag clear
	data = appendBiffRecord(data, biffWSBool, []byte{0x00, 0x00})
	data = appendBiffRecord(data, biffEOF, nil)

	sheet := parseWorksheet(data, boundSheet{offset: 0, name: "Sheet1"}, nil, nil)
	if sheet == nil {
		t.Fatal("expected sheet, got nil")
	}
	if sheet.DialogSheet {
		t.Error("expected sheet.DialogSheet = false")
	}
}

func TestParseNameCommentRecordDecodesNameAndComment(t *testing.T) {
	// POI TestNameCommentRecord test data (payload only):
	// 94 08 00 00 00 00 00 00 00 00 00 00 04 00 07 00 00 6E 61 6D 65 00 63 6F 6D 6D 65 6E 74
	payload := []byte{
		0x94, 0x08, // record type
		0x00, 0x00, // frt cell ref flag
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // reserved
		0x04, 0x00, // name length
		0x07, 0x00, // comment length
		0x00,                   // name multibyte = false
		0x6E, 0x61, 0x6D, 0x65, // "name"
		0x00,                                     // comment multibyte = false
		0x63, 0x6F, 0x6D, 0x6D, 0x65, 0x6E, 0x74, // "comment"
	}
	name, comment, ok := parseNameCommentRecord(payload)
	if !ok {
		t.Fatal("expected parseNameCommentRecord to succeed")
	}
	if name != "name" {
		t.Errorf("name = %q, want %q", name, "name")
	}
	if comment != "comment" {
		t.Errorf("comment = %q, want %q", comment, "comment")
	}
}

func TestParseNameCommentRecordReturnsFalseForShortPayload(t *testing.T) {
	if _, _, ok := parseNameCommentRecord([]byte{0x00, 0x00}); ok {
		t.Error("expected parseNameCommentRecord to fail for short payload")
	}
}

func TestParseWorkbookGlobalsAppliesNameComment(t *testing.T) {
	// Build a minimal workbook stream with a NAME record and a NAMECOMMENT record.
	data := appendBiffRecord(nil, biffBOF, []byte{0x05, 0x00, 0x05, 0x00})

	// NAME record for "MyName" with no formula.
	namePayload := make([]byte, 21)
	// grbit = 0, chKey = 0, cch = 6
	namePayload[3] = 6
	// cce = 0 (bytes 4-5 already zero)
	// itab = 0 (bytes 8-9 already zero)
	// fHighByte = 0 at byte 14
	// name = "MyName" at bytes 15-20
	copy(namePayload[15:], "MyName")
	data = appendBiffRecord(data, biffName, namePayload)

	// NAMECOMMENT record for "MyName" → "MyComment".
	ncPayload := make([]byte, 33)
	binary.LittleEndian.PutUint16(ncPayload[0:2], 0x0894) // record type
	// frt cell ref flag = 0, reserved = 0
	binary.LittleEndian.PutUint16(ncPayload[12:14], 6) // name length
	binary.LittleEndian.PutUint16(ncPayload[14:16], 9) // comment length
	ncPayload[16] = 0                                  // name multibyte = false
	copy(ncPayload[17:], "MyName")                     // 6 bytes
	ncPayload[23] = 0                                  // comment multibyte = false
	copy(ncPayload[24:], "MyComment")                  // 9 bytes
	data = appendBiffRecord(data, biffNameComment, ncPayload)

	data = appendBiffRecord(data, biffEOF, nil)

	_, _, _, definedNames, _, _, _, _, _, err := parseWorkbookGlobals(data)
	if err != nil {
		t.Fatalf("parseWorkbookGlobals error: %v", err)
	}
	if len(definedNames) != 1 {
		t.Fatalf("expected 1 defined name, got %d", len(definedNames))
	}
	if definedNames[0].Name != "MyName" {
		t.Errorf("Name = %q, want %q", definedNames[0].Name, "MyName")
	}
	if definedNames[0].Comment != "MyComment" {
		t.Errorf("Comment = %q, want %q", definedNames[0].Comment, "MyComment")
	}
}

func TestParseChartLineFormatRecordDecodesColor(t *testing.T) {
	// LineFormatRecord payload: color=0x00FF0000 (red in BBGGRR), pattern=0, weight=1, format=0, paletteIdx=0
	payload := make([]byte, 12)
	binary.LittleEndian.PutUint32(payload[0:4], 0x000000FF) // red
	binary.LittleEndian.PutUint16(payload[4:6], 0)          // pattern solid
	binary.LittleEndian.PutUint16(payload[6:8], 1)          // weight medium
	binary.LittleEndian.PutUint16(payload[8:10], 0)         // format
	binary.LittleEndian.PutUint16(payload[10:12], 0)        // paletteIdx

	color, pattern, weight, ok := parseChartLineFormatRecord(payload)
	if !ok {
		t.Fatal("expected parseChartLineFormatRecord to succeed")
	}
	if color != "FF0000" {
		t.Errorf("color = %q, want %q", color, "FF0000")
	}
	if pattern != 0 {
		t.Errorf("pattern = %d, want 0", pattern)
	}
	if weight != 1 {
		t.Errorf("weight = %d, want 1", weight)
	}
}

func TestParseChartLineFormatRecordReturnsFalseForShortPayload(t *testing.T) {
	if _, _, _, ok := parseChartLineFormatRecord([]byte{0x00}); ok {
		t.Error("expected parseChartLineFormatRecord to fail for short payload")
	}
}

func TestParseChartLineFormatRecordAcceptsBIFF5Payload(t *testing.T) {
	payload := []byte{0x80, 0x80, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	color, pattern, weight, ok := parseChartLineFormatRecord(payload)
	if !ok {
		t.Fatal("expected BIFF5 line format payload to succeed")
	}
	if color != "808080" || pattern != 0 || weight != 0 {
		t.Fatalf("got color=%q pattern=%d weight=%d, want 808080/0/0", color, pattern, weight)
	}
}

func TestParseChartAreaFormatRecordDecodesColors(t *testing.T) {
	// AreaFormatRecord payload: fg=0x0000FF00 (green), bg=0x00FF0000 (red), pattern=1, flags=0, foreIdx=0, backIdx=0
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint32(payload[0:4], 0x0000FF00) // green
	binary.LittleEndian.PutUint32(payload[4:8], 0x000000FF) // red
	binary.LittleEndian.PutUint16(payload[8:10], 1)         // pattern
	binary.LittleEndian.PutUint16(payload[10:12], 0)        // flags
	binary.LittleEndian.PutUint16(payload[12:14], 0)        // foreIdx
	binary.LittleEndian.PutUint16(payload[14:16], 0)        // backIdx

	fg, bg, pattern, ok := parseChartAreaFormatRecord(payload)
	if !ok {
		t.Fatal("expected parseChartAreaFormatRecord to succeed")
	}
	if fg != "00FF00" {
		t.Errorf("fgColor = %q, want %q", fg, "00FF00")
	}
	if bg != "FF0000" {
		t.Errorf("bgColor = %q, want %q", bg, "FF0000")
	}
	if pattern != 1 {
		t.Errorf("pattern = %d, want 1", pattern)
	}
}

func TestParseChartAreaFormatRecordReturnsFalseForShortPayload(t *testing.T) {
	if _, _, _, ok := parseChartAreaFormatRecord([]byte{0x00}); ok {
		t.Error("expected parseChartAreaFormatRecord to fail for short payload")
	}
}

func TestParseChartAreaFormatRecordAcceptsBIFF5Payload(t *testing.T) {
	payload := []byte{0x33, 0x99, 0x33, 0x00, 0xFF, 0xFF, 0xFF, 0x00, 0x01, 0x00, 0x00, 0x00}
	fg, bg, pattern, ok := parseChartAreaFormatRecord(payload)
	if !ok {
		t.Fatal("expected BIFF5 area format payload to succeed")
	}
	if fg != "339933" || bg != "FFFFFF" || pattern != 1 {
		t.Fatalf("got fg=%q bg=%q pattern=%d, want 339933/FFFFFF/1", fg, bg, pattern)
	}
}

func TestParseChartMarkerFormatRecordDecodesDiamondMarker(t *testing.T) {
	payload := []byte{0xFF, 0x00, 0x00, 0x00, 0xFF, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00}
	color, shape, ok := parseChartMarkerFormatRecord(payload)
	if !ok {
		t.Fatal("expected marker format to succeed")
	}
	if color != "FF0000" || shape != "diamond" {
		t.Fatalf("got color=%q shape=%q, want FF0000/diamond", color, shape)
	}
}

func TestResolveChartColor(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0xFF000000, ""},       // automatic
		{0, ""},                // automatic (zero fallback)
		{0x08, "000000"},       // palette index 0x08 → default black
		{0x09, "FFFFFF"},       // palette index 0x09 → default white
		{0x3F, "333333"},       // palette index 0x3F → default dark gray
		{0x07, "070000"},       // below palette range → treat as raw RGB 0x00000007 → R=07 G=00 B=00
		{0x000000FF, "FF0000"}, // pure red (BBGGRR)
		{0x0000FF00, "00FF00"}, // pure green
		{0x00FF0000, "0000FF"}, // pure blue
	}
	for _, tt := range tests {
		got := resolveChartColor(tt.input)
		if got != tt.want {
			t.Errorf("resolveChartColor(0x%08X) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseEmbeddedChartsReadsLineAndAreaFormat(t *testing.T) {
	// Build a minimal chart substream with Series + LineFormat + AreaFormat.
	chartBOF := []byte{0x00, 0x06, 0x20, 0x00}
	var data []byte
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// Series record
	data = appendBiffRecord(data, biffChartSeries, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// LineFormat: red line
	linePayload := make([]byte, 12)
	binary.LittleEndian.PutUint32(linePayload[0:4], 0x000000FF)
	binary.LittleEndian.PutUint16(linePayload[4:6], 0)
	binary.LittleEndian.PutUint16(linePayload[6:8], 1)
	binary.LittleEndian.PutUint16(linePayload[8:10], 0)
	binary.LittleEndian.PutUint16(linePayload[10:12], 0)
	data = appendBiffRecord(data, biffChartLineFormat, linePayload)

	// AreaFormat: green fill
	areaPayload := make([]byte, 16)
	binary.LittleEndian.PutUint32(areaPayload[0:4], 0x0000FF00)
	binary.LittleEndian.PutUint32(areaPayload[4:8], 0x00000000)
	binary.LittleEndian.PutUint16(areaPayload[8:10], 1)
	binary.LittleEndian.PutUint16(areaPayload[10:12], 0)
	binary.LittleEndian.PutUint16(areaPayload[12:14], 0)
	binary.LittleEndian.PutUint16(areaPayload[14:16], 0)
	data = appendBiffRecord(data, biffChartAreaFormat, areaPayload)

	data = appendBiffRecord(data, biffChartEnd, nil) // end series block
	data = appendBiffRecord(data, biffChartEnd, nil) // end chart block
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "Sheet1"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("expected 1 drawing, got %d", len(drawings))
	}
	chart, ok := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatal("expected *chartpkg.Chart")
	}
	if len(chart.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(chart.Series))
	}
	if chart.Series[0].LineColor != "FF0000" {
		t.Errorf("Series[0].LineColor = %q, want %q", chart.Series[0].LineColor, "FF0000")
	}
	if chart.Series[0].FillColor != "00FF00" {
		t.Errorf("Series[0].FillColor = %q, want %q", chart.Series[0].FillColor, "00FF00")
	}
}

func TestParseEmbeddedChartsAppliesLineAreaToPlotAreaWhenOutsideSeries(t *testing.T) {
	// Build a chart substream where LineFormat/AreaFormat appear after PlotArea (no active series).
	chartBOF := []byte{0x00, 0x06, 0x20, 0x00}
	var data []byte
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// Series block (closed properly)
	data = appendBiffRecord(data, biffChartSeries, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)
	data = appendBiffRecord(data, biffChartEnd, nil)

	// PlotArea block
	data = appendBiffRecord(data, biffChartPlotArea, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// LineFormat inside plot area
	linePayload := make([]byte, 12)
	binary.LittleEndian.PutUint32(linePayload[0:4], 0x00FF00FF) // magenta
	binary.LittleEndian.PutUint16(linePayload[4:6], 0)
	binary.LittleEndian.PutUint16(linePayload[6:8], 1)
	binary.LittleEndian.PutUint16(linePayload[8:10], 0)
	binary.LittleEndian.PutUint16(linePayload[10:12], 0)
	data = appendBiffRecord(data, biffChartLineFormat, linePayload)

	// AreaFormat inside plot area
	areaPayload := make([]byte, 16)
	binary.LittleEndian.PutUint32(areaPayload[0:4], 0x00FFFF00) // cyan
	binary.LittleEndian.PutUint32(areaPayload[4:8], 0x00000000)
	binary.LittleEndian.PutUint16(areaPayload[8:10], 1)
	binary.LittleEndian.PutUint16(areaPayload[10:12], 0)
	binary.LittleEndian.PutUint16(areaPayload[12:14], 0)
	binary.LittleEndian.PutUint16(areaPayload[14:16], 0)
	data = appendBiffRecord(data, biffChartAreaFormat, areaPayload)

	data = appendBiffRecord(data, biffChartEnd, nil) // end plot area
	data = appendBiffRecord(data, biffChartEnd, nil) // end chart
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "Sheet1"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("expected 1 drawing, got %d", len(drawings))
	}
	chart, ok := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatal("expected *chartpkg.Chart")
	}
	if chart.PlotArea.BorderColor != "FF00FF" {
		t.Errorf("PlotArea.BorderColor = %q, want %q", chart.PlotArea.BorderColor, "FF00FF")
	}
	if chart.PlotArea.FillColor != "00FFFF" {
		t.Errorf("PlotArea.FillColor = %q, want %q", chart.PlotArea.FillColor, "00FFFF")
	}
}

func TestParseChartBarRecordDecodesHorizontalAndStacked(t *testing.T) {
	// BarRecord payload: barSpace=0, categorySpace=0, flags=horizontal|stacked
	payload := make([]byte, 6)
	binary.LittleEndian.PutUint16(payload[0:2], 0)    // barSpace
	binary.LittleEndian.PutUint16(payload[2:4], 0)    // categorySpace
	binary.LittleEndian.PutUint16(payload[4:6], 0x03) // horizontal=true, stacked=true

	horizontal, stacked := parseChartBarRecord(payload)
	if !horizontal {
		t.Error("expected horizontal = true")
	}
	if !stacked {
		t.Error("expected stacked = true")
	}

	// flags=0 → horizontal=false, stacked=false
	binary.LittleEndian.PutUint16(payload[4:6], 0)
	horizontal, stacked = parseChartBarRecord(payload)
	if horizontal {
		t.Error("expected horizontal = false")
	}
	if stacked {
		t.Error("expected stacked = false")
	}
}

func TestParseChartBarRecordReturnsFalseForShortPayload(t *testing.T) {
	horizontal, stacked := parseChartBarRecord([]byte{0x00})
	if horizontal || stacked {
		t.Error("expected parseChartBarRecord to return false for short payload")
	}
}

func TestParseChartDataFormatRecordDecodesSeriesIndex(t *testing.T) {
	// DataFormatRecord payload: pointNumber=-1, seriesIndex=2, seriesNumber=2, flags=0
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint16(payload[0:2], 0xFFFF) // pointNumber = -1
	binary.LittleEndian.PutUint16(payload[2:4], 2)      // seriesIndex = 2
	binary.LittleEndian.PutUint16(payload[4:6], 2)      // seriesNumber = 2
	binary.LittleEndian.PutUint16(payload[6:8], 0)      // flags

	idx := parseChartDataFormatRecord(payload)
	if idx != 2 {
		t.Errorf("seriesIndex = %d, want 2", idx)
	}
}

func TestParseChartDataFormatRecordReturnsMinusOneForShortPayload(t *testing.T) {
	if idx := parseChartDataFormatRecord([]byte{0x00}); idx != -1 {
		t.Errorf("expected -1 for short payload, got %d", idx)
	}
}

func TestParseEmbeddedChartsReadsBarRecordDirectionAndStacked(t *testing.T) {
	chartBOF := []byte{0x00, 0x06, 0x20, 0x00}
	var data []byte
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// BarRecord: horizontal=false (column), stacked=true
	barPayload := make([]byte, 6)
	binary.LittleEndian.PutUint16(barPayload[4:6], 0x02) // stacked only
	data = appendBiffRecord(data, biffChartBar, barPayload)

	// Add a dummy series so the chart isn't discarded
	data = appendBiffRecord(data, biffChartSeries, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)
	data = appendBiffRecord(data, biffChartEnd, nil)

	data = appendBiffRecord(data, biffChartEnd, nil)
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "Sheet1"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("expected 1 drawing, got %d", len(drawings))
	}
	chart, ok := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatal("expected *chartpkg.Chart")
	}
	if chart.ChartType != chartpkg.ChartColumn {
		t.Errorf("ChartType = %v, want %v", chart.ChartType, chartpkg.ChartColumn)
	}
	if !chart.Stacked {
		t.Error("expected Stacked = true")
	}
}

func TestParseEmbeddedChartsUsesDataFormatSeriesIndexForColors(t *testing.T) {
	chartBOF := []byte{0x00, 0x06, 0x20, 0x00}
	var data []byte
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// Two series
	data = appendBiffRecord(data, biffChartSeries, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)
	data = appendBiffRecord(data, biffChartEnd, nil)

	data = appendBiffRecord(data, biffChartSeries, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)
	data = appendBiffRecord(data, biffChartEnd, nil)

	// DataFormat for series 0 (not the current series)
	dfPayload := make([]byte, 8)
	binary.LittleEndian.PutUint16(dfPayload[0:2], 0xFFFF) // pointNumber = -1
	binary.LittleEndian.PutUint16(dfPayload[2:4], 0)      // seriesIndex = 0
	binary.LittleEndian.PutUint16(dfPayload[4:6], 0)      // seriesNumber = 0
	binary.LittleEndian.PutUint16(dfPayload[6:8], 0)      // flags
	data = appendBiffRecord(data, biffChartDataFormat, dfPayload)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// LineFormat inside DataFormat block
	linePayload := make([]byte, 12)
	binary.LittleEndian.PutUint32(linePayload[0:4], 0x000000FF) // red
	binary.LittleEndian.PutUint16(linePayload[4:6], 0)
	binary.LittleEndian.PutUint16(linePayload[6:8], 1)
	binary.LittleEndian.PutUint16(linePayload[8:10], 0)
	binary.LittleEndian.PutUint16(linePayload[10:12], 0)
	data = appendBiffRecord(data, biffChartLineFormat, linePayload)

	data = appendBiffRecord(data, biffChartEnd, nil) // end DataFormat block
	data = appendBiffRecord(data, biffChartEnd, nil) // end chart
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "Sheet1"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("expected 1 drawing, got %d", len(drawings))
	}
	chart, ok := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatal("expected *chartpkg.Chart")
	}
	if len(chart.Series) != 2 {
		t.Fatalf("expected 2 series, got %d", len(chart.Series))
	}
	if chart.Series[0].LineColor != "FF0000" {
		t.Errorf("Series[0].LineColor = %q, want %q", chart.Series[0].LineColor, "FF0000")
	}
	if chart.Series[1].LineColor != "" {
		t.Errorf("Series[1].LineColor = %q, want empty", chart.Series[1].LineColor)
	}
}

func TestParseChartTickRecordDecodesFields(t *testing.T) {
	payload := make([]byte, 30)
	payload[0] = 1                                          // majorTickType = outside
	payload[1] = 2                                          // minorTickType = inside
	payload[2] = 3                                          // labelPosition = next to axis
	binary.LittleEndian.PutUint32(payload[4:8], 0x00FF0000) // labelColorRgb
	binary.LittleEndian.PutUint16(payload[24:26], 0x0020)   // autorotate + normal rotation
	binary.LittleEndian.PutUint16(payload[26:28], 0x0004)   // tickColor

	major, minor, labelPos, ok := parseChartTickRecord(payload)
	if !ok {
		t.Fatal("expected ok")
	}
	if major != 1 {
		t.Errorf("majorTickType = %d, want 1", major)
	}
	if minor != 2 {
		t.Errorf("minorTickType = %d, want 2", minor)
	}
	if labelPos != 3 {
		t.Errorf("labelPosition = %d, want 3", labelPos)
	}
}

func TestParseChartTickRecordReturnsFalseForShortPayload(t *testing.T) {
	_, _, _, ok := parseChartTickRecord([]byte{1, 2, 3})
	if ok {
		t.Error("expected false for short payload")
	}
}

func TestParseEmbeddedChartsReadsTickRecordOnValueAxis(t *testing.T) {
	chartBOF := []byte{0x00, 0x06, 0x20, 0x00}
	var data []byte
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// Series so the chart isn't discarded
	data = appendBiffRecord(data, biffChartSeries, nil)

	// Value axis block with TICK record
	data = appendBiffRecord(data, biffChartAxis, []byte{0x01, 0x00}) // type = value
	data = appendBiffRecord(data, biffChartBegin, nil)
	tickPayload := make([]byte, 30)
	tickPayload[0] = 3 // majorTickType = cross
	tickPayload[1] = 1 // minorTickType = outside
	tickPayload[2] = 2 // labelPosition = high
	data = appendBiffRecord(data, biffChartTick, tickPayload)
	data = appendBiffRecord(data, biffChartEnd, nil)

	data = appendBiffRecord(data, biffChartEnd, nil) // end chart
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "Sheet1"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("expected 1 drawing, got %d", len(drawings))
	}
	chart, ok := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatal("expected *chartpkg.Chart")
	}
	if chart.YAxis.MajorTickType != 3 {
		t.Errorf("YAxis.MajorTickType = %d, want 3", chart.YAxis.MajorTickType)
	}
	if chart.YAxis.MinorTickType != 1 {
		t.Errorf("YAxis.MinorTickType = %d, want 1", chart.YAxis.MinorTickType)
	}
	if chart.YAxis.LabelPosition != 2 {
		t.Errorf("YAxis.LabelPosition = %d, want 2", chart.YAxis.LabelPosition)
	}
}

func TestParseEmbeddedChartsReadsTickRecordOnCategoryAxis(t *testing.T) {
	chartBOF := []byte{0x00, 0x06, 0x20, 0x00}
	var data []byte
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// Series so the chart isn't discarded
	data = appendBiffRecord(data, biffChartSeries, nil)

	// Category axis block with TICK record
	data = appendBiffRecord(data, biffChartAxis, []byte{0x00, 0x00}) // type = category
	data = appendBiffRecord(data, biffChartBegin, nil)
	tickPayload := make([]byte, 30)
	tickPayload[0] = 1 // majorTickType = outside
	tickPayload[1] = 0 // minorTickType = none
	tickPayload[2] = 3 // labelPosition = next to axis
	data = appendBiffRecord(data, biffChartTick, tickPayload)
	data = appendBiffRecord(data, biffChartEnd, nil)

	data = appendBiffRecord(data, biffChartEnd, nil) // end chart
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "Sheet1"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("expected 1 drawing, got %d", len(drawings))
	}
	chart, ok := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatal("expected *chartpkg.Chart")
	}
	if chart.XAxis.MajorTickType != 1 {
		t.Errorf("XAxis.MajorTickType = %d, want 1", chart.XAxis.MajorTickType)
	}
	if chart.XAxis.MinorTickType != 0 {
		t.Errorf("XAxis.MinorTickType = %d, want 0", chart.XAxis.MinorTickType)
	}
	if chart.XAxis.LabelPosition != 3 {
		t.Errorf("XAxis.LabelPosition = %d, want 3", chart.XAxis.LabelPosition)
	}
}

func TestParseChartCategorySeriesAxisRecordDecodesFields(t *testing.T) {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint16(payload[0:2], 0x0005) // crossingPoint = 5
	binary.LittleEndian.PutUint16(payload[2:4], 0x0002) // labelFrequency = 2
	binary.LittleEndian.PutUint16(payload[4:6], 0x0001) // tickMarkFrequency = 1
	binary.LittleEndian.PutUint16(payload[6:8], 0x0004) // reversed = true

	crossing, labelFreq, tickFreq, reversed, ok := parseChartCategorySeriesAxisRecord(payload)
	if !ok {
		t.Fatal("expected ok")
	}
	if crossing != 5 {
		t.Errorf("crossingPoint = %d, want 5", crossing)
	}
	if labelFreq != 2 {
		t.Errorf("labelFrequency = %d, want 2", labelFreq)
	}
	if tickFreq != 1 {
		t.Errorf("tickMarkFrequency = %d, want 1", tickFreq)
	}
	if !reversed {
		t.Error("expected reversed = true")
	}
}

func TestParseChartCategorySeriesAxisRecordReturnsFalseForShortPayload(t *testing.T) {
	_, _, _, _, ok := parseChartCategorySeriesAxisRecord([]byte{1, 2, 3})
	if ok {
		t.Error("expected false for short payload")
	}
}

func TestParseEmbeddedChartsReadsCategorySeriesAxisRecord(t *testing.T) {
	chartBOF := []byte{0x00, 0x06, 0x20, 0x00}
	var data []byte
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// Series so the chart isn't discarded
	data = appendBiffRecord(data, biffChartSeries, nil)

	// Category axis block with CATEGORYSERIESAXIS record
	data = appendBiffRecord(data, biffChartAxis, []byte{0x00, 0x00}) // type = category
	data = appendBiffRecord(data, biffChartBegin, nil)
	csPayload := make([]byte, 8)
	binary.LittleEndian.PutUint16(csPayload[0:2], 0x0003) // crossingPoint = 3
	binary.LittleEndian.PutUint16(csPayload[2:4], 0x0002) // labelFrequency = 2
	binary.LittleEndian.PutUint16(csPayload[4:6], 0x0001) // tickMarkFrequency = 1
	binary.LittleEndian.PutUint16(csPayload[6:8], 0x0004) // reversed = true
	data = appendBiffRecord(data, biffChartCategorySeriesAxis, csPayload)
	data = appendBiffRecord(data, biffChartEnd, nil)

	data = appendBiffRecord(data, biffChartEnd, nil) // end chart
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "Sheet1"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("expected 1 drawing, got %d", len(drawings))
	}
	chart, ok := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatal("expected *chartpkg.Chart")
	}
	if chart.XAxis.CrossingPoint != 3 {
		t.Errorf("XAxis.CrossingPoint = %d, want 3", chart.XAxis.CrossingPoint)
	}
	if chart.XAxis.LabelFrequency != 2 {
		t.Errorf("XAxis.LabelFrequency = %d, want 2", chart.XAxis.LabelFrequency)
	}
	if chart.XAxis.TickMarkFrequency != 1 {
		t.Errorf("XAxis.TickMarkFrequency = %d, want 1", chart.XAxis.TickMarkFrequency)
	}
	if !chart.XAxis.Reversed {
		t.Error("expected XAxis.Reversed = true")
	}
}

func TestParseChartAxisLineFormatRecordDecodesType(t *testing.T) {
	tests := []struct {
		input    uint16
		wantType int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 3},
	}
	for _, tt := range tests {
		payload := make([]byte, 2)
		binary.LittleEndian.PutUint16(payload[0:2], tt.input)
		axisType, ok := parseChartAxisLineFormatRecord(payload)
		if !ok {
			t.Fatalf("expected ok for input %d", tt.input)
		}
		if axisType != tt.wantType {
			t.Errorf("input %d: axisType = %d, want %d", tt.input, axisType, tt.wantType)
		}
	}
}

func TestParseChartAxisLineFormatRecordReturnsFalseForShortPayload(t *testing.T) {
	_, ok := parseChartAxisLineFormatRecord([]byte{0})
	if ok {
		t.Error("expected false for short payload")
	}
}

func TestParseEmbeddedChartsRoutesLineFormatViaAxisLineFormat(t *testing.T) {
	chartBOF := []byte{0x00, 0x06, 0x20, 0x00}
	var data []byte
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// Series so the chart isn't discarded
	data = appendBiffRecord(data, biffChartSeries, nil)

	// Value axis block with AxisLineFormat + LineFormat for major grid lines
	data = appendBiffRecord(data, biffChartAxis, []byte{0x01, 0x00}) // type = value
	data = appendBiffRecord(data, biffChartBegin, nil)
	alPayload := make([]byte, 2)
	binary.LittleEndian.PutUint16(alPayload[0:2], 1) // major grid line
	data = appendBiffRecord(data, biffChartAxisLineFormat, alPayload)
	linePayload := make([]byte, 12)
	binary.LittleEndian.PutUint32(linePayload[0:4], 0x0000FF00) // green
	binary.LittleEndian.PutUint16(linePayload[4:6], 0)
	binary.LittleEndian.PutUint16(linePayload[6:8], 0)
	binary.LittleEndian.PutUint16(linePayload[8:10], 0)
	binary.LittleEndian.PutUint16(linePayload[10:12], 0)
	data = appendBiffRecord(data, biffChartLineFormat, linePayload)
	data = appendBiffRecord(data, biffChartEnd, nil)

	data = appendBiffRecord(data, biffChartEnd, nil) // end chart
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "Sheet1"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("expected 1 drawing, got %d", len(drawings))
	}
	chart, ok := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatal("expected *chartpkg.Chart")
	}
	if !chart.YAxis.ShowGridlines {
		t.Error("expected YAxis.ShowGridlines = true")
	}
	if chart.YAxis.GridlineColor != "00FF00" {
		t.Errorf("YAxis.GridlineColor = %q, want %q", chart.YAxis.GridlineColor, "00FF00")
	}
}

func TestParseEmbeddedChartsAxisLineFormatDoesNotAffectSeries(t *testing.T) {
	chartBOF := []byte{0x00, 0x06, 0x20, 0x00}
	var data []byte
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)

	// Series
	data = appendBiffRecord(data, biffChartSeries, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)
	linePayload := make([]byte, 12)
	binary.LittleEndian.PutUint32(linePayload[0:4], 0x000000FF) // red
	data = appendBiffRecord(data, biffChartLineFormat, linePayload)
	data = appendBiffRecord(data, biffChartEnd, nil)

	data = appendBiffRecord(data, biffChartEnd, nil) // end chart
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "Sheet1"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("expected 1 drawing, got %d", len(drawings))
	}
	chart, ok := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatal("expected *chartpkg.Chart")
	}
	if len(chart.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(chart.Series))
	}
	if chart.Series[0].LineColor != "FF0000" {
		t.Errorf("Series[0].LineColor = %q, want %q", chart.Series[0].LineColor, "FF0000")
	}
	if chart.YAxis.GridlineColor != "" {
		t.Errorf("YAxis.GridlineColor = %q, want empty", chart.YAxis.GridlineColor)
	}
}

func TestParseEmbeddedChartsPreservesPointFormattingAndDataTable(t *testing.T) {
	chartBOF := []byte{0x00, 0x06, 0x20, 0x00}
	var data []byte
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)
	data = appendBiffRecord(data, biffChartSeries, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)
	data = appendBiffRecord(data, biffChartEnd, nil)

	df := make([]byte, 8)
	binary.LittleEndian.PutUint16(df[0:2], 2)
	binary.LittleEndian.PutUint16(df[2:4], 0)
	data = appendBiffRecord(data, biffChartDataFormat, df)
	data = appendBiffRecord(data, biffChartBegin, nil)
	area := make([]byte, 16)
	binary.LittleEndian.PutUint32(area[0:4], 0x0000FF00)
	data = appendBiffRecord(data, biffChartAreaFormat, area)
	line := make([]byte, 12)
	binary.LittleEndian.PutUint32(line[0:4], 0x00FF0000)
	data = appendBiffRecord(data, biffChartLineFormat, line)
	data = appendBiffRecord(data, biffChartEnd, nil)

	dat := make([]byte, 2)
	binary.LittleEndian.PutUint16(dat, 0x000D) // horizontal + outline + keys
	data = appendBiffRecord(data, biffChartDat, dat)
	data = appendBiffRecord(data, biffChartEnd, nil)
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "Sheet1"}, 0, nil, nil)
	if len(drawings) != 1 {
		t.Fatalf("drawings=%d, want 1", len(drawings))
	}
	chart := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if len(chart.Series) != 1 || len(chart.Series[0].DataPoints) != 1 {
		t.Fatalf("data points=%#v", chart.Series)
	}
	point := chart.Series[0].DataPoints[0]
	if point.Index != 2 || point.FillColor != "00FF00" || point.LineColor != "0000FF" {
		t.Fatalf("point=%#v", point)
	}
	if chart.DataTable == nil || !chart.DataTable.ShowHorzBorder || chart.DataTable.ShowVertBorder || !chart.DataTable.ShowOutline || !chart.DataTable.ShowKeys {
		t.Fatalf("data table=%#v", chart.DataTable)
	}
}

func TestParseEmbeddedChartsUsesSerToCrtForComboSeries(t *testing.T) {
	chartBOF := []byte{0x00, 0x06, 0x20, 0x00}
	var data []byte
	data = appendBiffRecord(data, biffBOF, chartBOF)
	data = appendBiffRecord(data, biffChart, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)
	data = appendBiffRecord(data, biffChartBar, make([]byte, 6))
	data = appendBiffRecord(data, biffChartLine, nil)

	data = appendBiffRecord(data, biffChartSeries, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)
	group1 := make([]byte, 2)
	binary.LittleEndian.PutUint16(group1, 1)
	data = appendBiffRecord(data, biffChartSeriesChartGroupIndex, group1)
	data = appendBiffRecord(data, biffChartEnd, nil)

	data = appendBiffRecord(data, biffChartSeries, nil)
	data = appendBiffRecord(data, biffChartBegin, nil)
	group0 := make([]byte, 2)
	binary.LittleEndian.PutUint16(group0, 0)
	data = appendBiffRecord(data, biffChartSeriesChartGroupIndex, group0)
	data = appendBiffRecord(data, biffChartEnd, nil)
	data = appendBiffRecord(data, biffChartEnd, nil)
	data = appendBiffRecord(data, biffEOF, nil)

	drawings := parseEmbeddedCharts(data, boundSheet{offset: 0, name: "Sheet1"}, 0, nil, nil)
	chart := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if chart.Series[0].ChartType != chartpkg.ChartLine || chart.Series[1].ChartType != chartpkg.ChartColumn {
		t.Fatalf("series chart types=%v,%v", chart.Series[0].ChartType, chart.Series[1].ChartType)
	}
}

func TestParseChartDatRecord(t *testing.T) {
	payload := []byte{0x0F, 0x00}
	table, ok := parseChartDatRecord(payload)
	if !ok || table == nil || !table.ShowHorzBorder || !table.ShowVertBorder || !table.ShowOutline || !table.ShowKeys {
		t.Fatalf("table=%#v ok=%v", table, ok)
	}
}

func TestParseLegacySbsControlStatePreservesStepsAndOrientation(t *testing.T) {
	payload := make([]byte, 20)
	binary.LittleEndian.PutUint16(payload[4:6], uint16(7))
	binary.LittleEndian.PutUint16(payload[6:8], uint16(1))
	binary.LittleEndian.PutUint16(payload[8:10], uint16(20))
	binary.LittleEndian.PutUint16(payload[10:12], uint16(2))
	binary.LittleEndian.PutUint16(payload[12:14], uint16(5))
	binary.LittleEndian.PutUint16(payload[14:16], uint16(1))
	min, max, val, inc, page, horizontal, ok := parseLegacySbsControlState(payload, legacyObjTypeScroll)
	if !ok || min != 1 || max != 20 || val != 7 || inc != 2 || page != 5 || !horizontal {
		t.Fatalf("got min=%d max=%d val=%d inc=%d page=%d horizontal=%v ok=%v", min, max, val, inc, page, horizontal, ok)
	}
}

func TestParseLegacyRadioFirstButtonUsesFtRboData(t *testing.T) {
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint16(payload[0:2], 42)
	binary.LittleEndian.PutUint16(payload[2:4], 1)
	first, ok := parseLegacyRadioFirstButton(payload, legacyObjTypeRadio)
	if !ok || !first {
		t.Fatalf("first=%v ok=%v", first, ok)
	}
}

func TestParseChartPieRecordDistinguishesDoughnutAndLeaderLines(t *testing.T) {
	pie := make([]byte, 6)
	binary.LittleEndian.PutUint16(pie[2:4], 0)
	typ, hole, leaders := parseChartPieRecord(pie)
	if typ != chartpkg.ChartPie || hole != 0 || leaders {
		t.Fatalf("pie = type %v hole %d leaders %v", typ, hole, leaders)
	}

	doughnut := make([]byte, 6)
	binary.LittleEndian.PutUint16(doughnut[2:4], 55)
	binary.LittleEndian.PutUint16(doughnut[4:6], 0x0002)
	typ, hole, leaders = parseChartPieRecord(doughnut)
	if typ != chartpkg.ChartDoughnut || hole != 55 || !leaders {
		t.Fatalf("doughnut = type %v hole %d leaders %v", typ, hole, leaders)
	}
}

func TestLegacyChartLineWeightEMU(t *testing.T) {
	for weight, want := range map[int]int{0xffff: 6350, 0: 12700, 1: 25400, 2: 38100} {
		if got := legacyChartLineWeightEMU(weight); got != want {
			t.Fatalf("legacyChartLineWeightEMU(%#x) = %d, want %d", weight, got, want)
		}
	}
}

func TestLegacyOfficeArtShapeRotationFlipAndShadow(t *testing.T) {
	shape := legacyWorksheetShape{}
	props := make([]byte, 6*6)
	putProp := func(i int, id uint16, value uint32) {
		binary.LittleEndian.PutUint16(props[i*6:i*6+2], id)
		binary.LittleEndian.PutUint32(props[i*6+2:i*6+6], value)
	}
	putProp(0, legacyOfficeArtPropRotation, uint32(int32(45*65536)))
	putProp(1, legacyOfficeArtPropShadowColor, 0x00332211)
	putProp(2, legacyOfficeArtPropShadowOpacity, 0x00008000)
	putProp(3, legacyOfficeArtPropShadowOffsetX, uint32(int32(25400)))
	putProp(4, legacyOfficeArtPropShadowOffsetY, uint32(int32(12700)))
	putProp(5, legacyOfficeArtPropShadowSoftness, uint32(int32(38100)))
	parseLegacyWorksheetShapeProperties(props, 6, &shape)
	if shape.rotationDeg != 45 {
		t.Fatalf("rotation = %v, want 45", shape.rotationDeg)
	}
	if shape.shadow == nil {
		t.Fatal("expected shadow")
	}
	if shape.shadow.Color != "112233" || math.Abs(shape.shadow.Alpha-0.5) > 0.001 {
		t.Fatalf("shadow = %#v", shape.shadow)
	}
	if math.Abs(shape.shadow.Blur-3) > 0.01 {
		t.Fatalf("blur = %v, want 3pt", shape.shadow.Blur)
	}

	spBody := make([]byte, 8)
	binary.LittleEndian.PutUint32(spBody[4:8], 0x40|0x80)
	container := appendOfficeArtTestRecord(nil, uint16(1<<4)|2, legacyOfficeArtSp, spBody)
	anchor := make([]byte, 18)
	binary.LittleEndian.PutUint16(anchor[10:12], 1)
	binary.LittleEndian.PutUint16(anchor[14:16], 1)
	container = appendOfficeArtTestRecord(container, 0, legacyOfficeArtClientAnchor, anchor)
	sheet := &sheetir.Sheet{Name: "S", ColumnWidths: map[int]float64{1: 8, 2: 8}}
	got, ok := parseLegacyNonPictureShapeContainer(container, sheet, map[int]*sheetir.Row{})
	if !ok || !got.flipH || !got.flipV {
		t.Fatalf("flips = h:%t v:%t ok:%t", got.flipH, got.flipV, ok)
	}
}

func appendOfficeArtTestRecord(dst []byte, verInst, recType uint16, body []byte) []byte {
	header := make([]byte, 8)
	binary.LittleEndian.PutUint16(header[0:2], verInst)
	binary.LittleEndian.PutUint16(header[2:4], recType)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(body)))
	dst = append(dst, header...)
	return append(dst, body...)
}

func TestLegacyChartRadarSurfaceAnd3DRecords(t *testing.T) {
	var data []byte
	appendBIFF := func(id uint16, payload []byte) {
		rec := make([]byte, 4+len(payload))
		binary.LittleEndian.PutUint16(rec[0:2], id)
		binary.LittleEndian.PutUint16(rec[2:4], uint16(len(payload)))
		copy(rec[4:], payload)
		data = append(data, rec...)
	}
	appendBIFF(0x0809, make([]byte, 16))
	appendBIFF(biffChart, make([]byte, 16))
	appendBIFF(biffChartSeries, make([]byte, 12))
	appendBIFF(biffChartRadar, make([]byte, 4))
	chart3d := make([]byte, 14)
	binary.LittleEndian.PutUint16(chart3d[0:2], 30)
	binary.LittleEndian.PutUint16(chart3d[2:4], 20)
	binary.LittleEndian.PutUint16(chart3d[4:6], 25)
	binary.LittleEndian.PutUint16(chart3d[8:10], 120)
	appendBIFF(biffChartChart3D, chart3d)
	appendBIFF(biffEOF, nil)
	drawings := parseEmbeddedCharts(data, boundSheet{name: "S"}, 0, nil, nil)
	if len(drawings) != 1 || drawings[0].Chart == nil {
		t.Fatalf("drawings = %#v", drawings)
	}
	chart, ok := drawings[0].Chart.Chart.(*chartpkg.Chart)
	if !ok || chart == nil {
		t.Fatalf("chart = %T", drawings[0].Chart.Chart)
	}
	if chart.ChartType != chartpkg.ChartRadar || chart.View3D == nil {
		t.Fatalf("type/view3d = %v %#v", chart.ChartType, chart.View3D)
	}
	if chart.View3D.RotX != 20 || chart.View3D.RotY != 30 || chart.View3D.DepthPercent != 120 {
		t.Fatalf("view3d = %#v", chart.View3D)
	}

	data = nil
	appendBIFF(0x0809, make([]byte, 16))
	appendBIFF(biffChart, make([]byte, 16))
	appendBIFF(biffChartSeries, make([]byte, 12))
	appendBIFF(biffChartSurface, make([]byte, 4))
	appendBIFF(biffEOF, nil)
	drawings = parseEmbeddedCharts(data, boundSheet{name: "S"}, 0, nil, nil)
	chart = drawings[0].Chart.Chart.(*chartpkg.Chart)
	if chart.ChartType != chartpkg.ChartSurface {
		t.Fatalf("surface type = %v", chart.ChartType)
	}
}
