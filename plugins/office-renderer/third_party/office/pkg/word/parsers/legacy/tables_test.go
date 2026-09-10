package legacy

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
)

func TestBuildLegacyDocFromRunsBinaryTableUsesRowMergeMetadata(t *testing.T) {
	inTable := wordir.ParagraphFormat{InTable: true}
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	props := &legacyTableRowProps{}
	props.setHMerge(1, 3)
	runs := []binaryWordRun{
		{Text: "A\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "B\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "C\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "\a", ParaFormat: rowMark, TableProps: props.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: props.canonical()},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	row := doc.Tables[0].Rows[0]
	if len(row.Cells) != 2 {
		t.Fatalf("row cells = %d, want 2 after merge metadata", len(row.Cells))
	}
	if got := row.Cells[1].Text; got != "B" {
		t.Fatalf("merged cell text = %q, want B", got)
	}
	if got := row.Cells[1].Format.GridSpan; got != 2 {
		t.Fatalf("merged cell GridSpan = %d, want 2", got)
	}
	if !row.Cells[1].Format.HMerge {
		t.Fatal("merged cell HMerge = false, want true")
	}
}

func TestBuildLegacyDocFromRunsBinaryTableUsesMetadataWithoutInTableFlag(t *testing.T) {
	rowProps := &legacyTableRowProps{}
	width := 1200
	rowProps.setCellFormat(0, wordir.TableCellFormat{Width: &width, WidthType: "dxa"})
	rowProps.setCellFormat(1, wordir.TableCellFormat{Width: &width, WidthType: "dxa"})
	runs := []binaryWordRun{
		{Text: "A\a", NativeCellMarks: true, TableProps: rowProps.canonical()},
		{Text: "\r", TableProps: rowProps.canonical()},
		{Text: "B\a", NativeCellMarks: true, TableProps: rowProps.canonical()},
		{Text: "\r", TableProps: rowProps.canonical()},
		{Text: "\a", NativeCellMarks: true, TableProps: rowProps.canonical()},
		{Text: "\r", TableProps: rowProps.canonical()},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(doc.Tables[0].Rows))
	}
	if len(doc.Tables[0].Rows[0].Cells) != 2 {
		t.Fatalf("cells = %d, want 2", len(doc.Tables[0].Rows[0].Cells))
	}
	if got := doc.Tables[0].Rows[0].Cells[0].Text; got != "A" {
		t.Fatalf("cell[0] text = %q, want A", got)
	}
	if got := doc.Tables[0].Rows[0].Cells[1].Text; got != "B" {
		t.Fatalf("cell[1] text = %q, want B", got)
	}
}

func TestCollectLegacyBinaryTableRowGroupsSplitsMetadataOnlyRowsOnRowPropChange(t *testing.T) {
	width := 1200
	firstRowProps := &legacyTableRowProps{}
	firstRowProps.setCellFormat(0, wordir.TableCellFormat{Width: &width, WidthType: "dxa"})
	firstRowProps.setCellFormat(1, wordir.TableCellFormat{Width: &width, WidthType: "dxa"})
	secondRowProps := firstRowProps.clone()
	height := 240
	secondRowProps.rowHeight = &height

	lineSlices := [][]binaryWordRun{
		{{Text: "A\a", NativeCellMarks: true, TableProps: firstRowProps.canonical()}},
		{{Text: "B\a", NativeCellMarks: true, TableProps: firstRowProps.canonical()}},
		{{Text: "C\a", NativeCellMarks: true, TableProps: secondRowProps.canonical()}},
		{{Text: "D\a", NativeCellMarks: true, TableProps: secondRowProps.canonical()}},
	}

	rowGroups := collectLegacyBinaryTableRowGroups(lineSlices)
	if len(rowGroups) != 2 {
		t.Fatalf("row groups = %d, want 2", len(rowGroups))
	}
	if len(rowGroups[0].LineSlices) != 2 {
		t.Fatalf("row 0 lines = %d, want 2", len(rowGroups[0].LineSlices))
	}
	if len(rowGroups[1].LineSlices) != 2 {
		t.Fatalf("row 1 lines = %d, want 2", len(rowGroups[1].LineSlices))
	}
	if got := rowGroups[0].LineSlices[0][0].Text; got != "A\a" {
		t.Fatalf("row 0 first line = %q, want A\\a", got)
	}
	if got := rowGroups[1].LineSlices[0][0].Text; got != "C\a" {
		t.Fatalf("row 1 first line = %q, want C\\a", got)
	}
}

func TestCollectLegacyBinaryTableRowGroupsKeepsCompatibleMetadataInSameRow(t *testing.T) {
	lineAProps := &legacyTableRowProps{tableJc: "center"}
	indent := 360
	lineBProps := &legacyTableRowProps{tableDxaLeft: &indent}
	lineSlices := [][]binaryWordRun{
		{{Text: "A\a", NativeCellMarks: true, TableProps: lineAProps.canonical()}},
		{{Text: "B\a", NativeCellMarks: true, TableProps: lineBProps.canonical()}},
	}

	rowGroups := collectLegacyBinaryTableRowGroups(lineSlices)
	if len(rowGroups) != 1 {
		t.Fatalf("row groups = %d, want 1 (compatible metadata should merge)", len(rowGroups))
	}
	if len(rowGroups[0].LineSlices) != 2 {
		t.Fatalf("row 0 lines = %d, want 2", len(rowGroups[0].LineSlices))
	}
}

func TestCollectLegacyBinaryTableRowGroupsSplitsOnConflictingRowMetadata(t *testing.T) {
	header := true
	notHeader := false
	lineAProps := &legacyTableRowProps{tableHeader: &header}
	lineBProps := &legacyTableRowProps{tableHeader: &notHeader}
	lineSlices := [][]binaryWordRun{
		{{Text: "A\a", NativeCellMarks: true, TableProps: lineAProps.canonical()}},
		{{Text: "B\a", NativeCellMarks: true, TableProps: lineBProps.canonical()}},
	}

	rowGroups := collectLegacyBinaryTableRowGroups(lineSlices)
	if len(rowGroups) != 2 {
		t.Fatalf("row groups = %d, want 2 (conflicting row metadata should split)", len(rowGroups))
	}
}

func TestBuildLegacyDocFromRunsBinaryTableMergesRowMarkAndLineProps(t *testing.T) {
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	header := true
	rowMarkProps := &legacyTableRowProps{tableHeader: &header}
	indent := 720
	gapHalf := 24
	lineProps := &legacyTableRowProps{tableJc: "center", tableDxaLeft: &indent, tableDxaGap: &gapHalf}
	runs := []binaryWordRun{
		{Text: "A\a", NativeCellMarks: true, TableProps: lineProps.canonical()},
		{Text: "\r", TableProps: lineProps.canonical()},
		{Text: "\a", ParaFormat: rowMark, TableProps: rowMarkProps.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: rowMarkProps.canonical()},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	table := doc.Tables[0]
	if got := table.Format.Alignment; got != "center" {
		t.Fatalf("table alignment = %q, want center", got)
	}
	if got := table.Format.Indent; got != 720 {
		t.Fatalf("table indent = %d, want 720", got)
	}
	if got := table.Format.CellSpacing; got != 48 {
		t.Fatalf("table cell spacing = %d, want 48", got)
	}
	if len(table.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(table.Rows))
	}
	if !table.Rows[0].Format.IsHeader {
		t.Fatal("row 0 IsHeader = false, want true from row-mark metadata")
	}
}

func TestBuildLegacyDocFromRunsBinaryTablePromotesRowMarkPropertyRevision(t *testing.T) {
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	rowProps := &legacyTableRowProps{}
	width := 1200
	rowProps.setCellFormat(0, wordir.TableCellFormat{Width: &width, WidthType: "dxa"})
	revision := &wordir.RevisionMark{Author: "Alice", Date: "2026-05-08"}
	runs := []binaryWordRun{
		{Text: "A\a", NativeCellMarks: true, TableProps: rowProps.canonical()},
		{Text: "\r", TableProps: rowProps.canonical()},
		{Text: "\a", ParaFormat: rowMark, TableProps: rowProps.canonical(), PropertyRevision: revision},
		{Text: "\r", ParaFormat: rowMark, TableProps: rowProps.canonical(), PropertyRevision: revision},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(doc.Tables[0].Rows))
	}
	rowRevision := doc.Tables[0].Rows[0].PropertyRevision
	if rowRevision == nil {
		t.Fatal("row PropertyRevision = nil, want metadata from row mark PAPX")
	}
	if got := rowRevision.Author; got != "Alice" {
		t.Fatalf("row PropertyRevision.Author = %q, want Alice", got)
	}
	if got := rowRevision.Date; got != "2026-05-08" {
		t.Fatalf("row PropertyRevision.Date = %q, want 2026-05-08", got)
	}
}

func TestBuildLegacyDocFromRunsBinaryTablePrefersTapxPropertyRevision(t *testing.T) {
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	rowProps := &legacyTableRowProps{}
	width := 1200
	rowProps.setCellFormat(0, wordir.TableCellFormat{Width: &width, WidthType: "dxa"})
	// resolvedPropertyRevision from sprmTPropRMark (TAPx) should take precedence
	// over the paragraph-level PropertyRevision on the row-mark runs.
	rowProps.resolvedPropertyRevision = &wordir.RevisionMark{Author: "TAPxAuthor", Date: "2026-01-01"}
	papxRevision := &wordir.RevisionMark{Author: "PAPxAuthor", Date: "2026-02-02"}
	runs := []binaryWordRun{
		{Text: "A\a", NativeCellMarks: true, TableProps: rowProps.canonical()},
		{Text: "\r", TableProps: rowProps.canonical()},
		{Text: "\a", ParaFormat: rowMark, TableProps: rowProps.canonical(), PropertyRevision: papxRevision},
		{Text: "\r", ParaFormat: rowMark, TableProps: rowProps.canonical(), PropertyRevision: papxRevision},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(doc.Tables[0].Rows))
	}
	rowRevision := doc.Tables[0].Rows[0].PropertyRevision
	if rowRevision == nil {
		t.Fatal("row PropertyRevision = nil")
	}
	if got := rowRevision.Author; got != "TAPxAuthor" {
		t.Fatalf("row PropertyRevision.Author = %q, want TAPxAuthor", got)
	}
	if got := rowRevision.Date; got != "2026-01-01" {
		t.Fatalf("row PropertyRevision.Date = %q, want 2026-01-01", got)
	}
}

func TestExtractDocRunsViaPreWord97FIBWord95ErrPreservesNativeCellMarks(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := filepath.Join("..", "..", "..", "..", "testdata", "doc", "word95_cyrillic.doc")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatal("missing WordDocument")
	}
	data := extractDocRunsViaPreWord97FIB(wordStream, f)
	if data == nil || len(data.Assembly.Runs) == 0 {
		t.Fatal("extractDocRunsViaPreWord97FIB = nil")
	}
	for _, run := range data.Assembly.Runs {
		if run.NativeCellMarks {
			return
		}
	}
	t.Fatal("no pre-97 runs preserved native cell-mark signal")
}

func TestParseLegacyTDefTableOperandDecodesHorizontalMergeFlags(t *testing.T) {
	op := make([]byte, 71)
	op[0] = 0x46 // cb = 70, so total operand size is 71 bytes
	op[2] = 3    // NumberOfColumns
	// rgdxaCenter occupies (columns+1)*2 bytes and can stay zeroed for this test.
	tcOffset := 3 + (int(op[2])+1)*2
	// Cell 1 starts a horizontal merge; cell 2 continues it.
	op[tcOffset+20] = 0x02
	op[tcOffset+40] = 0x01

	props := parseLegacyTDefTableOperand(op)
	if props == nil {
		t.Fatal("parseLegacyTDefTableOperand = nil, want merge metadata")
	}
	if got := props.hMergeState(1); got != 2 {
		t.Fatalf("cell 1 hMerge = %d, want 2 (start)", got)
	}
	if got := props.hMergeState(2); got != 1 {
		t.Fatalf("cell 2 hMerge = %d, want 1 (continue)", got)
	}
}

func TestParseLegacyTDefTableOperandDecodesCellFormatting(t *testing.T) {
	op := make([]byte, 27)
	op[0] = 0x1A // cb = 26, so total operand size is 27 bytes
	op[2] = 1    // NumberOfColumns
	tcOffset := 3 + (int(op[2])+1)*2

	tcgrf := uint16((1 << 2) | (1 << 7) | (3 << 9)) // textFlow=tbRl, valign=center, widthType=dxa
	op[tcOffset+0] = byte(tcgrf)
	op[tcOffset+1] = byte(tcgrf >> 8)
	op[tcOffset+2] = 0xA0 // width = 1440 twips
	op[tcOffset+3] = 0x05
	copy(op[tcOffset+4:tcOffset+8], []byte{8, 1, 6, 0}) // single red top border
	for _, off := range []int{8, 12, 16} {
		copy(op[tcOffset+off:tcOffset+off+4], []byte{0xFF, 0xFF, 0xFF, 0xFF})
	}

	props := parseLegacyTDefTableOperand(op)
	if props == nil {
		t.Fatal("parseLegacyTDefTableOperand = nil, want cell format metadata")
	}
	format := props.cellFormat(0)
	if format.TextDirection != "tbRl" {
		t.Fatalf("cell textDirection = %q, want tbRl", format.TextDirection)
	}
	if format.VertAlign != "center" {
		t.Fatalf("cell valign = %q, want center", format.VertAlign)
	}
	if format.WidthType != "dxa" {
		t.Fatalf("cell widthType = %q, want dxa", format.WidthType)
	}
	if format.Width == nil || *format.Width != 1440 {
		t.Fatalf("cell width = %v, want 1440 twips", format.Width)
	}
	if format.Borders == nil || format.Borders.Top == nil {
		t.Fatalf("cell borders = %#v, want top border", format.Borders)
	}
	if format.Borders.Top.Val != "single" {
		t.Fatalf("top border style = %q, want single", format.Borders.Top.Val)
	}
	if format.Borders.Top.Color != "FF0000" {
		t.Fatalf("top border color = %q, want FF0000", format.Borders.Top.Color)
	}
}

func TestParseLegacyTDefTableOperandMapsTableCellTextFlow(t *testing.T) {
	tests := []struct {
		name string
		flow uint16
		want string
	}{
		{name: "top to bottom right to left", flow: 1, want: "tbRl"},
		{name: "bottom to top left to right", flow: 3, want: "btLr"},
		{name: "vertical east asian", flow: 5, want: "tbRlV"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op := make([]byte, 27)
			op[0] = 0x1A
			op[2] = 1
			tcOffset := 3 + (int(op[2])+1)*2
			tcgrf := tc.flow << 2
			op[tcOffset+0] = byte(tcgrf)
			op[tcOffset+1] = byte(tcgrf >> 8)

			props := parseLegacyTDefTableOperand(op)
			if props == nil {
				t.Fatal("parseLegacyTDefTableOperand = nil, want cell format metadata")
			}
			if got := props.cellFormat(0).TextDirection; got != tc.want {
				t.Fatalf("cell textDirection = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildLegacyTableFlattensExplicitRowIntoBalancedGridAndPeelsTrailingProse(t *testing.T) {
	lines := []string{
		"Alpha\tBravo\tCharlie\tDelta\tEcho\tFoxtrot\tGolf\tHotel\tIndia\tJuliet\tKilo\tLima",
		"This is an embedded 3 x 4 Excel spreadsheet.\tIt has been stretched horizontally and vertically so the font is funny looking.\tIt has also been centered.",
	}

	table, trailing := buildLegacyTable(0, lines, 0)
	if table == nil {
		t.Fatal("buildLegacyTable = nil, want 4x3 table")
	}
	if len(table.Rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(table.Rows))
	}
	for rowIndex, row := range table.Rows {
		if len(row.Cells) != 3 {
			t.Fatalf("row %d cells = %d, want 3", rowIndex, len(row.Cells))
		}
	}
	if strings.Contains(table.Text, "embedded 3 x 4 Excel spreadsheet") {
		t.Fatalf("table text unexpectedly includes trailing prose: %q", table.Text)
	}
	if len(trailing) != 3 {
		t.Fatalf("trailing = %d entries, want 3", len(trailing))
	}
	if trailing[0] != "This is an embedded 3 x 4 Excel spreadsheet." {
		t.Fatalf("trailing[0] = %q, want first prose sentence", trailing[0])
	}
}

func TestParseDocFloatingPicturesEmbeddedSpreadsheetPreservesFourByThreeTable(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := filepath.Join("..", "..", "..", "..", "testdata", "doc", "apache-FloatingPictures.doc")
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}

	for _, table := range collectLegacyTablesRecursive(doc.Tables) {
		if table.Text != "Alpha\nBravo\nCharlie\nDelta\nEcho\nFoxtrot\nGolf\nHotel\nIndia\nJuliet\nKilo\nLima" {
			continue
		}
		if len(table.Rows) != 4 {
			t.Fatalf("rows = %d, want 4", len(table.Rows))
		}
		for rowIndex, row := range table.Rows {
			if len(row.Cells) != 3 {
				t.Fatalf("row %d cells = %d, want 3", rowIndex, len(row.Cells))
			}
		}
		return
	}

	t.Fatal("embedded spreadsheet table not found")
}

func collectLegacyTablesRecursive(tables []wordir.Table) []wordir.Table {
	out := append([]wordir.Table(nil), tables...)
	for _, table := range tables {
		for _, row := range table.Rows {
			for _, cell := range row.Cells {
				out = append(out, collectLegacyTablesRecursive(cell.Tables)...)
			}
		}
	}
	return out
}

func TestParsePapxGrpprlTableCellFormattingSprms(t *testing.T) {
	grpprl := []byte{
		0x2C, 0xD6, 0x03, 0x00, 0x02, 0x01, // sprmTVertAlign: cells 0..1 => center
		0x29, 0x76, 0x00, 0x02, 0x01, 0x00, // sprmTTextFlow: cells 0..1 => tbRl
		0x35, 0xD6, 0x05, 0x00, 0x01, 0x03, 0xB0, 0x04, // sprmTCellWidth: cell 0 => 1200 twips
		0x2D, 0xD6, 0x0C, 0x01, 0x02, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, // sprmTSetShd: cell 1 => solid red
		0x20, 0xD6, 0x07, 0x00, 0x02, 0x01, 0x08, 0x01, 0x02, 0x00, // sprmTSetBrc80: cells 0..1 top border => blue
	}

	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.TableRowProps == nil {
		t.Fatal("TableRowProps is nil, want TAPx cell metadata")
	}

	first := meta.TableRowProps.cellFormat(0)
	if first.TextDirection != "tbRl" {
		t.Fatalf("cell 0 textDirection = %q, want tbRl", first.TextDirection)
	}
	if first.VertAlign != "center" {
		t.Fatalf("cell 0 valign = %q, want center", first.VertAlign)
	}
	if first.WidthType != "dxa" {
		t.Fatalf("cell 0 widthType = %q, want dxa", first.WidthType)
	}
	if first.Width == nil || *first.Width != 1200 {
		t.Fatalf("cell 0 width = %v, want 1200 twips", first.Width)
	}
	if first.Borders == nil || first.Borders.Top == nil {
		t.Fatalf("cell 0 borders = %#v, want top border", first.Borders)
	}
	if first.Borders.Top.Color != "0000FF" {
		t.Fatalf("cell 0 top border color = %q, want 0000FF", first.Borders.Top.Color)
	}

	second := meta.TableRowProps.cellFormat(1)
	if second.TextDirection != "tbRl" {
		t.Fatalf("cell 1 textDirection = %q, want tbRl", second.TextDirection)
	}
	if second.VertAlign != "center" {
		t.Fatalf("cell 1 valign = %q, want center", second.VertAlign)
	}
	if second.Shading != "FF0000" {
		t.Fatalf("cell 1 shading = %q, want FF0000", second.Shading)
	}
	if second.Borders == nil || second.Borders.Top == nil {
		t.Fatalf("cell 1 borders = %#v, want top border", second.Borders)
	}
	if second.Borders.Top.Color != "0000FF" {
		t.Fatalf("cell 1 top border color = %q, want 0000FF", second.Borders.Top.Color)
	}
}

func TestParsePapxGrpprlTableVerticalMergeSprm(t *testing.T) {
	grpprl := []byte{
		0x2B, 0xD6, 0x02, 0x01, 0x03, // sprmTVertMerge: cell 1 => restart
	}

	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.TableRowProps == nil {
		t.Fatal("TableRowProps is nil, want vertical merge metadata")
	}
	if got := meta.TableRowProps.vMergeState(1); got != 3 {
		t.Fatalf("cell 1 vMerge = %d, want 3 (restart)", got)
	}
}

func TestParsePapxGrpprlTableRowHeightSprm(t *testing.T) {
	grpprl := []byte{
		0x07, 0x94, 0x70, 0xFE, // sprmTDyaRowHeight: -400 twips => exact 20pt
	}

	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.TableRowProps == nil || meta.TableRowProps.rowHeight == nil {
		t.Fatal("rowHeight metadata is nil, want parsed sprmTDyaRowHeight")
	}
	if got := *meta.TableRowProps.rowHeight; got != -400 {
		t.Fatalf("rowHeight = %d, want -400", got)
	}
	if !meta.TableRowProps.hasData() {
		t.Fatal("TableRowProps.hasData() = false, want true for row height metadata")
	}
}

func TestParsePapxGrpprlTableDefaultCellPaddingSprm(t *testing.T) {
	grpprl := []byte{
		0x34, 0xD6, 0x06, 0x00, 0x03, 0x0F, 0x03, 0x78, 0x00, // sprmTCellPaddingDefault: all sides = 120 twips
	}

	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.TableRowProps == nil {
		t.Fatal("TableRowProps is nil, want default cell padding metadata")
	}
	if meta.TableRowProps.cellPadTop == nil || *meta.TableRowProps.cellPadTop != 120 {
		t.Fatalf("cellPadTop = %v, want 120", meta.TableRowProps.cellPadTop)
	}
	if meta.TableRowProps.cellPadBottom == nil || *meta.TableRowProps.cellPadBottom != 120 {
		t.Fatalf("cellPadBottom = %v, want 120", meta.TableRowProps.cellPadBottom)
	}
	if meta.TableRowProps.cellPadLeft == nil || *meta.TableRowProps.cellPadLeft != 120 {
		t.Fatalf("cellPadLeft = %v, want 120", meta.TableRowProps.cellPadLeft)
	}
	if meta.TableRowProps.cellPadRight == nil || *meta.TableRowProps.cellPadRight != 120 {
		t.Fatalf("cellPadRight = %v, want 120", meta.TableRowProps.cellPadRight)
	}
}

func TestParseLegacyTDefTableOperandDecodesVerticalMergeFlags(t *testing.T) {
	op := make([]byte, 71)
	op[0] = 0x46 // cb = 70, so total operand size is 71 bytes
	op[2] = 3    // NumberOfColumns
	tcOffset := 3 + (int(op[2])+1)*2

	// Cell 0 => restart, cell 1 => continue.
	tcgrfRestart := uint16(3 << 5)
	tcgrfContinue := uint16(1 << 5)
	op[tcOffset+0] = byte(tcgrfRestart)
	op[tcOffset+1] = byte(tcgrfRestart >> 8)
	op[tcOffset+20] = byte(tcgrfContinue)
	op[tcOffset+21] = byte(tcgrfContinue >> 8)

	props := parseLegacyTDefTableOperand(op)
	if props == nil {
		t.Fatal("parseLegacyTDefTableOperand = nil, want vertical merge metadata")
	}
	if got := props.vMergeState(0); got != 3 {
		t.Fatalf("cell 0 vMerge = %d, want 3 (restart)", got)
	}
	if got := props.vMergeState(1); got != 1 {
		t.Fatalf("cell 1 vMerge = %d, want 1 (continue)", got)
	}
}

func TestBuildLegacyDocFromRunsBinaryTableAppliesCellFormattingMetadata(t *testing.T) {
	inTable := wordir.ParagraphFormat{InTable: true}
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	props := &legacyTableRowProps{}
	width := 1440
	props.setCellFormat(0, wordir.TableCellFormat{
		Width:         &width,
		WidthType:     "dxa",
		VertAlign:     "center",
		TextDirection: "btLr",
		Shading:       "FF0000",
		Borders:       &wordir.CellBorders{Top: &wordir.BorderSide{Val: "single", SizePt: 1, Color: "0000FF"}},
	})
	props.setCellFormat(1, wordir.TableCellFormat{
		Shading: "00FF00",
	})
	runs := []binaryWordRun{
		{Text: "A\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "B\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "\a", ParaFormat: rowMark, TableProps: props.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: props.canonical()},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	row := doc.Tables[0].Rows[0]
	if len(row.Cells) != 2 {
		t.Fatalf("row cells = %d, want 2", len(row.Cells))
	}
	first := row.Cells[0].Format
	if first.Width == nil || *first.Width != 1440 || first.WidthType != "dxa" {
		t.Fatalf("cell 0 width = (%v,%q), want (1440,dxa)", first.Width, first.WidthType)
	}
	if first.VertAlign != "center" {
		t.Fatalf("cell 0 valign = %q, want center", first.VertAlign)
	}
	if first.TextDirection != "btLr" {
		t.Fatalf("cell 0 textDirection = %q, want btLr", first.TextDirection)
	}
	if first.Shading != "FF0000" {
		t.Fatalf("cell 0 shading = %q, want FF0000", first.Shading)
	}
	if first.Borders == nil || first.Borders.Top == nil || first.Borders.Top.Color != "0000FF" {
		t.Fatalf("cell 0 borders = %#v, want blue top border", first.Borders)
	}
	if got := row.Cells[1].Format.Shading; got != "00FF00" {
		t.Fatalf("cell 1 shading = %q, want 00FF00", got)
	}
}

func TestBuildLegacyDocFromRunsBinaryTableUsesVerticalMergeMetadata(t *testing.T) {
	inTable := wordir.ParagraphFormat{InTable: true}
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	firstRowProps := &legacyTableRowProps{}
	secondRowProps := &legacyTableRowProps{}
	firstRowProps.setVMerge(0, 3)
	secondRowProps.setVMerge(0, 1)

	runs := []binaryWordRun{
		{Text: "Top\\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "\\a", ParaFormat: rowMark, TableProps: firstRowProps.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: firstRowProps.canonical()},
		{Text: "Bottom\\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "\\a", ParaFormat: rowMark, TableProps: secondRowProps.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: secondRowProps.canonical()},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	table := doc.Tables[0]
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(table.Rows))
	}
	if len(table.Rows[0].Cells) != 1 || len(table.Rows[1].Cells) != 1 {
		t.Fatalf("row cell counts = (%d,%d), want (1,1)", len(table.Rows[0].Cells), len(table.Rows[1].Cells))
	}
	if got := table.Rows[0].Cells[0].Format.VMerge; got != "restart" {
		t.Fatalf("row 0 cell 0 vMerge = %q, want restart", got)
	}
	if got := table.Rows[1].Cells[0].Format.VMerge; got != "continue" {
		t.Fatalf("row 1 cell 0 vMerge = %q, want continue", got)
	}
}

func TestBuildLegacyDocFromRunsBinaryTableUsesRowHeightMetadata(t *testing.T) {
	inTable := wordir.ParagraphFormat{InTable: true}
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	exactRowProps := &legacyTableRowProps{}
	atLeastRowProps := &legacyTableRowProps{}
	exactHeight := -400
	atLeastHeight := 240
	exactRowProps.rowHeight = &exactHeight
	atLeastRowProps.rowHeight = &atLeastHeight

	runs := []binaryWordRun{
		{Text: "Exact\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "\a", ParaFormat: rowMark, TableProps: exactRowProps.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: exactRowProps.canonical()},
		{Text: "AtLeast\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "\a", ParaFormat: rowMark, TableProps: atLeastRowProps.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: atLeastRowProps.canonical()},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	table := doc.Tables[0]
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(table.Rows))
	}
	if table.Rows[0].Format.Height == nil || *table.Rows[0].Format.Height != 400 {
		t.Fatalf("row 0 height = %v, want 400 twips", table.Rows[0].Format.Height)
	}
	if got := table.Rows[0].Format.HeightRule; got != "exact" {
		t.Fatalf("row 0 height rule = %q, want exact", got)
	}
	if table.Rows[1].Format.Height == nil || *table.Rows[1].Format.Height != 240 {
		t.Fatalf("row 1 height = %v, want 240 twips", table.Rows[1].Format.Height)
	}
	if got := table.Rows[1].Format.HeightRule; got != "atLeast" {
		t.Fatalf("row 1 height rule = %q, want atLeast", got)
	}
}

func TestBuildLegacyDocFromRunsBinaryTableUsesDefaultCellPaddingMetadata(t *testing.T) {
	inTable := wordir.ParagraphFormat{InTable: true}
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	padding := 120
	props := &legacyTableRowProps{
		cellPadTop:    &padding,
		cellPadBottom: &padding,
		cellPadLeft:   &padding,
		cellPadRight:  &padding,
	}
	runs := []binaryWordRun{
		{Text: "A\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "\a", ParaFormat: rowMark, TableProps: props.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: props.canonical()},
	}

	doc := buildLegacyDocFromRuns("padding.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	format := doc.Tables[0].Format
	if format.CellPadTop != 120 || format.CellPadBottom != 120 || format.CellPadLeft != 120 || format.CellPadRight != 120 {
		t.Fatalf("table cell padding = top:%d bottom:%d left:%d right:%d, want 120 on all sides", format.CellPadTop, format.CellPadBottom, format.CellPadLeft, format.CellPadRight)
	}
}

func TestBuildLegacyBinaryTablePreservesNestedTablesByDepth(t *testing.T) {
	inTable := wordir.ParagraphFormat{InTable: true}
	outerRowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	innerCell := wordir.ParagraphFormat{InTable: true, IsTableCellTrailingParagraph: true}
	innerRow := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	rowGroups := []legacyBinaryTableRow{{
		LineSlices: [][]binaryWordRun{
			{{Text: "1", ParaFormat: innerCell, TableDepth: 2}},
			{{Text: "", ParaFormat: innerRow, TableDepth: 2}},
			{{Text: "2", ParaFormat: innerCell, TableDepth: 2}},
			{{Text: "", ParaFormat: innerRow, TableDepth: 2}},
			{{Text: "3", ParaFormat: innerCell, TableDepth: 2}},
			{{Text: "4", ParaFormat: innerCell, TableDepth: 2}},
			{{Text: "", ParaFormat: innerRow, TableDepth: 2}},
		},
		RowMark: []binaryWordRun{{Text: "\t\t", ParaFormat: outerRowMark, TableDepth: 1}},
	}}

	table, _ := buildLegacyBinaryTable(0, rowGroups, 0)
	if table == nil {
		t.Fatal("buildLegacyBinaryTable = nil, want outer table")
	}
	if len(table.Rows) != 1 {
		t.Fatalf("rows = %d, want 1 outer row", len(table.Rows))
	}
	row := table.Rows[0]
	if len(row.Cells) != 3 {
		t.Fatalf("outer row cells = %d, want 3", len(row.Cells))
	}
	for index, cell := range row.Cells {
		if len(cell.Tables) != 1 {
			t.Fatalf("outer cell %d nested tables = %d, want 1", index, len(cell.Tables))
		}
	}
	if got := row.Cells[0].Tables[0].Text; got != "1" {
		t.Fatalf("first nested table text = %q, want 1", got)
	}
	if got := row.Cells[1].Tables[0].Text; got != "2" {
		t.Fatalf("second nested table text = %q, want 2", got)
	}
	if got := row.Cells[2].Tables[0].Text; got != "3\n4" {
		t.Fatalf("third nested table text = %q, want lines 3 and 4", got)
	}
	_ = inTable
}

func TestBuildLegacyDocFromRunsBinaryTableRespectsNestedRowMarkDepth(t *testing.T) {
	outerCell := wordir.ParagraphFormat{InTable: true}
	outerRowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	innerCell := wordir.ParagraphFormat{InTable: true, IsTableCellTrailingParagraph: true}
	innerRow := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	runs := []binaryWordRun{
		{Text: "1", ParaFormat: innerCell, TableDepth: 2},
		{Text: "\r", ParaFormat: innerCell, TableDepth: 2},
		{Text: "", ParaFormat: innerRow, TableDepth: 2},
		{Text: "\r", ParaFormat: innerRow, TableDepth: 2},
		{Text: "2", ParaFormat: innerCell, TableDepth: 2},
		{Text: "\r", ParaFormat: innerCell, TableDepth: 2},
		{Text: "", ParaFormat: innerRow, TableDepth: 2},
		{Text: "\r", ParaFormat: innerRow, TableDepth: 2},
		{Text: "3", ParaFormat: innerCell, TableDepth: 2},
		{Text: "\r", ParaFormat: innerCell, TableDepth: 2},
		{Text: "4", ParaFormat: innerCell, TableDepth: 2},
		{Text: "\r", ParaFormat: innerCell, TableDepth: 2},
		{Text: "", ParaFormat: innerRow, TableDepth: 2},
		{Text: "\r", ParaFormat: innerRow, TableDepth: 2},
		{Text: "\t\t", ParaFormat: outerRowMark, TableDepth: 1},
		{Text: "\r", ParaFormat: outerRowMark, TableDepth: 1},
		{Text: "after", ParaFormat: outerCell},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1 outer table", len(doc.Tables))
	}
	outer := doc.Tables[0]
	if len(outer.Rows) != 1 {
		t.Fatalf("outer rows = %d, want 1", len(outer.Rows))
	}
	if len(outer.Rows[0].Cells) != 3 {
		t.Fatalf("outer cells = %d, want 3", len(outer.Rows[0].Cells))
	}
	wantTexts := []string{"1", "2", "3\n4"}
	for index, want := range wantTexts {
		cell := outer.Rows[0].Cells[index]
		if len(cell.Tables) != 1 {
			t.Fatalf("outer cell %d nested tables = %d, want 1", index, len(cell.Tables))
		}
		if got := cell.Tables[0].Text; got != want {
			t.Fatalf("outer cell %d nested table text = %q, want %q", index, got, want)
		}
	}
}

func TestBuildLegacyBinaryTableFallsBackWhenRowPropsHideNestedToken(t *testing.T) {
	props := &legacyTableRowProps{hMerge: map[int]byte{}}
	props.hMerge[0] = 1
	props.hMerge[1] = 2
	props.hMerge[2] = 2
	props.hMerge[3] = 2
	rowGroups := []legacyBinaryTableRow{{
		LineSlices: [][]binaryWordRun{
			{{Text: "1", ParaFormat: wordir.ParagraphFormat{InTable: true, IsTableCellTrailingParagraph: true}, TableDepth: 2}},
		},
		RowMark: []binaryWordRun{{Text: "", ParaFormat: wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}, TableDepth: 2, TableProps: props.canonical()}},
	}}

	table, _ := buildLegacyBinaryTable(0, rowGroups, 0)
	if table == nil {
		t.Fatal("buildLegacyBinaryTable = nil, want fallback row without merge props")
	}
	if len(table.Rows) != 1 || len(table.Rows[0].Cells) != 1 {
		t.Fatalf("table shape = %#v, want 1x1 fallback table", table)
	}
	if got := table.Rows[0].Cells[0].Text; got != "1" {
		t.Fatalf("fallback cell text = %q, want %q", got, "1")
	}
}

func TestParseDocCuratedMergedTableFixturePreservesNativeGridGeometry(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/apache-table-merges.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	table := doc.Tables[0]
	if len(table.Rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(table.Rows))
	}
	wantWidths := []int{1062, 4676, 1134, 1276, 1154}
	if !reflect.DeepEqual(table.Format.ColumnWidths, wantWidths) {
		t.Fatalf("ColumnWidths = %v, want %v", table.Format.ColumnWidths, wantWidths)
	}

	wantRows := []struct {
		texts []string
		spans []int
	}{
		{texts: []string{"A", "B"}, spans: []int{3, 2}},
		{texts: []string{"C", "D", "E", "F"}, spans: []int{1, 1, 2, 1}},
		{texts: []string{"", "G", "H", "I\nJ"}, spans: []int{1, 1, 2, 1}},
		{texts: []string{"K"}, spans: []int{5}},
	}
	for rowIndex, want := range wantRows {
		row := table.Rows[rowIndex]
		if row.Format.GridBefore != 0 || row.Format.GridAfter != 0 {
			t.Fatalf("row %d grid gaps = before %d after %d, want 0/0", rowIndex, row.Format.GridBefore, row.Format.GridAfter)
		}
		if len(row.Cells) != len(want.texts) {
			t.Fatalf("row %d cells = %d, want %d", rowIndex, len(row.Cells), len(want.texts))
		}
		for cellIndex, cell := range row.Cells {
			if cell.Text != want.texts[cellIndex] {
				t.Fatalf("row %d cell %d text = %q, want %q", rowIndex, cellIndex, cell.Text, want.texts[cellIndex])
			}
			span := cell.Format.GridSpan
			if span <= 0 {
				span = 1
			}
			if span != want.spans[cellIndex] {
				t.Fatalf("row %d cell %d GridSpan = %d, want %d", rowIndex, cellIndex, span, want.spans[cellIndex])
			}
		}
	}
	if got := len(table.Rows[2].Cells[3].Paragraphs); got != 2 {
		t.Fatalf("I/J cell paragraphs = %d, want 2", got)
	}
}

func TestParseDocCuratedNestedTableFixturePreservesNestedTables(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/libre-tdf106799.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("top-level tables = %d, want 1 outer table", len(doc.Tables))
	}
	outer := doc.Tables[0]
	if len(outer.Rows) != 1 {
		t.Fatalf("outer rows = %d, want 1", len(outer.Rows))
	}
	if len(outer.Rows[0].Cells) != 3 {
		t.Fatalf("outer cells = %d, want 3", len(outer.Rows[0].Cells))
	}
	wantTexts := []string{"1", "2", "3\n4\n5\n6"}
	for index, want := range wantTexts {
		cell := outer.Rows[0].Cells[index]
		if len(cell.Tables) != 1 {
			t.Fatalf("outer cell %d nested tables = %d, want 1", index, len(cell.Tables))
		}
		if got := cell.Tables[0].Text; got != want {
			t.Fatalf("outer cell %d nested table text = %q, want %q", index, got, want)
		}
	}
}

// sprmTJc / sprmTJc90 (0x549A / 0x5400): table justification (0=left, 1=center, 2=right).
// sprmTDxaLeft (0x9601): signed 16-bit table indent in twips.
// sprmTDxaGapHalf (0x9602): signed 16-bit half-gap between cells in twips.
// sprmTFCantSplit (0x3403): 1-byte toggle, keep row across page break.
// sprmTFTableHeader (0x3404): 1-byte toggle, repeat row as header.
func TestParsePapxGrpprlTableLevelAndRowLevelTAPxSprms(t *testing.T) {
	grpprl := []byte{
		0x00, 0x54, 0x01, 0x00, // sprmTJc90 (op=0x5400) operand=1 (center) - 2-byte
		0x01, 0x96, 0x70, 0x01, // sprmTDxaLeft (op=0x9601) operand=0x0170 (368 twips) - 2-byte signed
		0x02, 0x96, 0x20, 0x00, // sprmTDxaGapHalf (op=0x9602) operand=0x0020 (32 twips) - 2-byte signed
		0x03, 0x34, 0x01, // sprmTFCantSplit (op=0x3403) operand=1 - 1-byte
		0x04, 0x34, 0x01, // sprmTFTableHeader (op=0x3404) operand=1 - 1-byte
	}
	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.TableRowProps == nil {
		t.Fatal("TableRowProps = nil, want populated TAPx props")
	}
	props := meta.TableRowProps
	if got := props.tableJc; got != "center" {
		t.Fatalf("tableJc = %q, want center", got)
	}
	if props.tableDxaLeft == nil || *props.tableDxaLeft != 368 {
		t.Fatalf("tableDxaLeft = %v, want 368", props.tableDxaLeft)
	}
	if props.tableDxaGap == nil || *props.tableDxaGap != 32 {
		t.Fatalf("tableDxaGap = %v, want 32", props.tableDxaGap)
	}
	if props.cantSplit == nil || !*props.cantSplit {
		t.Fatalf("cantSplit = %v, want true", props.cantSplit)
	}
	if props.tableHeader == nil || !*props.tableHeader {
		t.Fatalf("tableHeader = %v, want true", props.tableHeader)
	}
	if !props.hasData() {
		t.Fatal("hasData() = false, want true with row/table-level props set")
	}
}

func TestParsePapxGrpprlTableWidthAutofitBidiSprms(t *testing.T) {
	grpprl := []byte{
		0x14, 0xF6, 0x03, 0x90, 0x01, // sprmTTableWidth (0xF614): ftsDxa=3, wWidth=0x0190 (400 twips)
		0x15, 0x36, 0x01, // sprmTFAutofit (0x3615): true
		0x0B, 0x56, 0x01, 0x00, // sprmTFBiDi (0x560B): true (Bool16)
		0x0A, 0x74, 0x00, 0x00, 0x2F, 0x00, // sprmTTlp (0x740A): opaque, parsed but not mapped to IR Look
	}
	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.TableRowProps == nil {
		t.Fatal("TableRowProps = nil")
	}
	props := meta.TableRowProps
	if props.tableWidth == nil || *props.tableWidth != 400 {
		t.Fatalf("tableWidth = %v, want 400", props.tableWidth)
	}
	if props.tableWidthType != "dxa" {
		t.Fatalf("tableWidthType = %q, want dxa", props.tableWidthType)
	}
	if props.autoFit == nil || !*props.autoFit {
		t.Fatalf("autoFit = %v, want true", props.autoFit)
	}
	if props.bidiVisual == nil || !*props.bidiVisual {
		t.Fatalf("bidiVisual = %v, want true", props.bidiVisual)
	}
}

func TestParsePapxGrpprlTableBorderSprm(t *testing.T) {
	grpprl := []byte{
		0x05, 0xD6, 0x18, // sprmTTableBorders (op=0xD605), 24-byte payload follows
		8, 1, 2, 0, // top: single blue
		8, 1, 6, 0, // left: single red
		8, 3, 4, 0, // bottom: double green
		8, 4, 1, 0, // right: dotted black
		8, 6, 5, 0, // insideH: dashed magenta
		8, 1, 3, 0, // insideV: single cyan
	}
	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.TableRowProps == nil || meta.TableRowProps.tableBorders == nil {
		t.Fatal("TableRowProps.tableBorders = nil, want decoded TAP row borders")
	}
	borders := meta.TableRowProps.tableBorders
	if borders.Top == nil || borders.Top.Val != "single" || borders.Top.Color != "0000FF" {
		t.Fatalf("top border = %#v, want single blue", borders.Top)
	}
	if borders.Left == nil || borders.Left.Color != "FF0000" {
		t.Fatalf("left border = %#v, want red", borders.Left)
	}
	if borders.Bottom == nil || borders.Bottom.Val != "double" {
		t.Fatalf("bottom border = %#v, want double", borders.Bottom)
	}
	if borders.Right == nil || borders.Right.Val != "dotted" {
		t.Fatalf("right border = %#v, want dotted", borders.Right)
	}
	if borders.InsideH == nil || borders.InsideH.Val != "dashed" {
		t.Fatalf("insideH border = %#v, want dashed", borders.InsideH)
	}
	if borders.InsideV == nil || borders.InsideV.Color != "00FFFF" {
		t.Fatalf("insideV border = %#v, want cyan", borders.InsideV)
	}
}

func TestBuildLegacyDocFromRunsBinaryTableAppliesRowAndTableLevelProps(t *testing.T) {
	inTable := wordir.ParagraphFormat{InTable: true}
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}

	indent := 720
	gapHalf := 40
	headerTrue := true
	cantSplitTrue := true
	headerProps := &legacyTableRowProps{
		tableJc:      "right",
		tableDxaLeft: &indent,
		tableDxaGap:  &gapHalf,
		tableHeader:  &headerTrue,
		cantSplit:    &cantSplitTrue,
	}
	secondRowProps := &legacyTableRowProps{}
	runs := []binaryWordRun{
		{Text: "Header\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "\a", ParaFormat: rowMark, TableProps: headerProps.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: headerProps.canonical()},
		{Text: "Body\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "\a", ParaFormat: rowMark, TableProps: secondRowProps.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: secondRowProps.canonical()},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	table := doc.Tables[0]
	if got := table.Format.Alignment; got != "right" {
		t.Fatalf("table alignment = %q, want right (sprmTJc)", got)
	}
	if got := table.Format.Indent; got != 720 {
		t.Fatalf("table indent = %d, want 720 (sprmTDxaLeft)", got)
	}
	if got := table.Format.CellSpacing; got != 80 {
		t.Fatalf("table cell spacing = %d, want 80 (2*sprmTDxaGapHalf)", got)
	}
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(table.Rows))
	}
	if !table.Rows[0].Format.IsHeader {
		t.Fatal("row 0 IsHeader = false, want true (sprmTFTableHeader)")
	}
	if !table.Rows[0].Format.CantSplit {
		t.Fatal("row 0 CantSplit = false, want true (sprmTFCantSplit)")
	}
	if table.Rows[1].Format.IsHeader {
		t.Fatal("row 1 IsHeader = true, want false (no sprmTFTableHeader)")
	}
	if table.Rows[1].Format.CantSplit {
		t.Fatal("row 1 CantSplit = true, want false (no sprmTFCantSplit)")
	}
}

func TestBuildLegacyDocFromRunsBinaryTableAppliesPropsWithoutRowMarkRuns(t *testing.T) {
	rowProps := &legacyTableRowProps{}
	indent := 360
	gapHalf := 30
	header := true
	cantSplit := true
	rowProps.tableJc = "center"
	rowProps.tableDxaLeft = &indent
	rowProps.tableDxaGap = &gapHalf
	rowProps.tableHeader = &header
	rowProps.cantSplit = &cantSplit

	runs := []binaryWordRun{
		{Text: "A\a", NativeCellMarks: true, TableProps: rowProps.canonical()},
		{Text: "\r", TableProps: rowProps.canonical()},
		{Text: "B\a", NativeCellMarks: true, TableProps: rowProps.canonical()},
		{Text: "\r", TableProps: rowProps.canonical()},
		{Text: "\a", NativeCellMarks: true, TableProps: rowProps.canonical()},
		{Text: "\r", TableProps: rowProps.canonical()},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	table := doc.Tables[0]
	if got := table.Format.Alignment; got != "center" {
		t.Fatalf("table alignment = %q, want center", got)
	}
	if got := table.Format.Indent; got != 360 {
		t.Fatalf("table indent = %d, want 360", got)
	}
	if got := table.Format.CellSpacing; got != 60 {
		t.Fatalf("table cell spacing = %d, want 60", got)
	}
	if len(table.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(table.Rows))
	}
	if !table.Rows[0].Format.IsHeader {
		t.Fatal("row 0 IsHeader = false, want true")
	}
	if !table.Rows[0].Format.CantSplit {
		t.Fatal("row 0 CantSplit = false, want true")
	}
}

func TestBuildLegacyDocFromRunsBinaryTableSynthesizesColumnWidthsAndWidth(t *testing.T) {
	rowProps := &legacyTableRowProps{}
	left := 1200
	right := 2400
	rowProps.setCellFormat(0, wordir.TableCellFormat{Width: &left, WidthType: "dxa"})
	rowProps.setCellFormat(1, wordir.TableCellFormat{Width: &right, WidthType: "dxa"})

	runs := []binaryWordRun{
		{Text: "A\a", NativeCellMarks: true, TableProps: rowProps.canonical()},
		{Text: "\r", TableProps: rowProps.canonical()},
		{Text: "B\a", NativeCellMarks: true, TableProps: rowProps.canonical()},
		{Text: "\r", TableProps: rowProps.canonical()},
		{Text: "\a", NativeCellMarks: true, TableProps: rowProps.canonical()},
		{Text: "\r", TableProps: rowProps.canonical()},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	table := doc.Tables[0]
	if got := table.Format.ColumnWidths; len(got) != 2 || got[0] != 1200 || got[1] != 2400 {
		t.Fatalf("table column widths = %#v, want [1200 2400]", got)
	}
	if table.Format.Width == nil || *table.Format.Width != 3600 {
		t.Fatalf("table width = %v, want 3600", table.Format.Width)
	}
	if got := table.Format.WidthType; got != "dxa" {
		t.Fatalf("table widthType = %q, want dxa", got)
	}
}

func TestBuildLegacyDocFromRunsBinaryTableIgnoresRunawayMetadataWidthOnTokenRows(t *testing.T) {
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	runawayProps := &legacyTableRowProps{}
	runawayProps.setHMerge(200, 201)

	runs := []binaryWordRun{
		{Text: "A\aB\a", NativeCellMarks: true, TableProps: runawayProps.canonical()},
		{Text: "\r", TableProps: runawayProps.canonical()},
		{Text: "\a", ParaFormat: rowMark, NativeCellMarks: true, TableProps: runawayProps.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: runawayProps.canonical()},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(doc.Tables[0].Rows))
	}
	row := doc.Tables[0].Rows[0]
	if got := len(row.Cells); got != 2 {
		t.Fatalf("cells = %d, want 2 (token width, not runaway metadata width)", got)
	}
	if got := row.Format.GridAfter; got != 0 {
		t.Fatalf("gridAfter = %d, want 0", got)
	}
}

func TestBuildLegacyTableFromGridLinesIgnoresTrailingEmptyTabRunaway(t *testing.T) {
	lines := []string{
		"A\tB" + strings.Repeat("\t", 200),
		"C\tD",
	}
	table := buildLegacyTableFromGridLines(0, lines, 0)
	if table == nil {
		t.Fatal("buildLegacyTableFromGridLines = nil, want table")
	}
	if len(table.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(table.Rows))
	}
	if got := len(table.Rows[0].Cells); got != 2 {
		t.Fatalf("row 0 cells = %d, want 2", got)
	}
	if got := len(table.Rows[1].Cells); got != 2 {
		t.Fatalf("row 1 cells = %d, want 2", got)
	}
	if got := table.Rows[1].Format.GridAfter; got != 0 {
		t.Fatalf("row 1 gridAfter = %d, want 0", got)
	}
}

func TestApplyLegacyHeuristicTablePropsPreservesMetadata(t *testing.T) {
	lines := []string{"A\tB", "C\tD"}
	table := buildLegacyTableFromGridLines(0, lines, 0)
	if table == nil {
		t.Fatal("buildLegacyTableFromGridLines = nil, want table")
	}
	// Grid-line reconstruction supplies inferred dimensions. Clear them so this
	// test verifies that the explicit binary row properties take precedence.
	table.Format.Width = nil
	table.Format.WidthType = ""
	table.Format.ColumnWidths = nil

	indent := 720
	gapHalf := 18
	padding := 90
	rowHeight := -240
	header := true
	cantSplit := true
	leftWidth := 1200
	rightWidth := 1800
	rowRevision := &wordir.RevisionMark{Author: "Alice", Date: "2026-05-08"}
	rowProps := &legacyTableRowProps{
		tableJc:       "center",
		tableDxaLeft:  &indent,
		tableDxaGap:   &gapHalf,
		rowHeight:     &rowHeight,
		tableHeader:   &header,
		cantSplit:     &cantSplit,
		cellPadTop:    &padding,
		cellPadBottom: &padding,
		cellPadLeft:   &padding,
		cellPadRight:  &padding,
		tableBorders: &wordir.TableBorders{
			Top: &wordir.BorderSide{Val: "single", SizePt: 1},
		},
	}
	rowProps.setCellFormat(0, wordir.TableCellFormat{
		Width:         &leftWidth,
		WidthType:     "dxa",
		VertAlign:     "center",
		TextDirection: "tbRl",
		Shading:       "FFAA00",
		Borders: &wordir.CellBorders{
			Left: &wordir.BorderSide{Val: "double", SizePt: 2},
		},
	})
	rowProps.setCellFormat(1, wordir.TableCellFormat{
		Width:     &rightWidth,
		WidthType: "dxa",
		Shading:   "00AACC",
	})

	lineSlices := [][]binaryWordRun{
		{{Text: "A\tB", TableProps: rowProps.canonical(), PropertyRevision: rowRevision}},
		{{Text: "C\tD"}},
	}

	applyLegacyHeuristicTableProps(table, lineSlices)

	if got := table.Format.Alignment; got != "center" {
		t.Fatalf("table alignment = %q, want center", got)
	}
	if got := table.Format.Indent; got != 720 {
		t.Fatalf("table indent = %d, want 720", got)
	}
	if got := table.Format.CellSpacing; got != 36 {
		t.Fatalf("table cell spacing = %d, want 36", got)
	}
	if table.Format.Borders == nil || table.Format.Borders.Top == nil || table.Format.Borders.Top.Val != "single" {
		t.Fatalf("table top border = %#v, want single", table.Format.Borders)
	}
	if got := table.Format.CellPadTop; got != 90 {
		t.Fatalf("table cell pad top = %d, want 90", got)
	}
	if got := table.Format.CellPadBottom; got != 90 {
		t.Fatalf("table cell pad bottom = %d, want 90", got)
	}
	if got := table.Format.CellPadLeft; got != 90 {
		t.Fatalf("table cell pad left = %d, want 90", got)
	}
	if got := table.Format.CellPadRight; got != 90 {
		t.Fatalf("table cell pad right = %d, want 90", got)
	}
	if table.Format.Width == nil || *table.Format.Width != 3000 {
		t.Fatalf("table width = %v, want 3000", table.Format.Width)
	}
	if got := table.Format.WidthType; got != "dxa" {
		t.Fatalf("table width type = %q, want dxa", got)
	}
	if got := table.Format.ColumnWidths; len(got) != 2 || got[0] != 1200 || got[1] != 1800 {
		t.Fatalf("table column widths = %#v, want [1200 1800]", got)
	}

	row := table.Rows[0]
	if row.PropertyRevision == nil || row.PropertyRevision.Author != "Alice" || row.PropertyRevision.Date != "2026-05-08" {
		t.Fatalf("row PropertyRevision = %#v, want Alice/2026-05-08", row.PropertyRevision)
	}
	if row.Format.Height == nil || *row.Format.Height != 240 {
		t.Fatalf("row height = %v, want 240", row.Format.Height)
	}
	if got := row.Format.HeightRule; got != "exact" {
		t.Fatalf("row height rule = %q, want exact", got)
	}
	if !row.Format.IsHeader {
		t.Fatal("row IsHeader = false, want true")
	}
	if !row.Format.CantSplit {
		t.Fatal("row CantSplit = false, want true")
	}

	firstCell := row.Cells[0]
	if firstCell.PropertyRevision == nil || firstCell.PropertyRevision.Author != "Alice" || firstCell.PropertyRevision.Date != "2026-05-08" {
		t.Fatalf("first cell PropertyRevision = %#v, want Alice/2026-05-08", firstCell.PropertyRevision)
	}
	if len(firstCell.Paragraphs) != 1 || firstCell.Paragraphs[0].PropertyRevision == nil || firstCell.Paragraphs[0].PropertyRevision.Author != "Alice" {
		t.Fatalf("first cell paragraph PropertyRevision = %#v, want Alice/2026-05-08", firstCell.Paragraphs)
	}
	if firstCell.Format.Width == nil || *firstCell.Format.Width != 1200 {
		t.Fatalf("first cell width = %v, want 1200", firstCell.Format.Width)
	}
	if got := firstCell.Format.WidthType; got != "dxa" {
		t.Fatalf("first cell width type = %q, want dxa", got)
	}
	if got := firstCell.Format.VertAlign; got != "center" {
		t.Fatalf("first cell vert align = %q, want center", got)
	}
	if got := firstCell.Format.TextDirection; got != "tbRl" {
		t.Fatalf("first cell text direction = %q, want tbRl", got)
	}
	if got := firstCell.Format.Shading; got != "FFAA00" {
		t.Fatalf("first cell shading = %q, want FFAA00", got)
	}
	if firstCell.Format.Borders == nil || firstCell.Format.Borders.Left == nil || firstCell.Format.Borders.Left.Val != "double" {
		t.Fatalf("first cell left border = %#v, want double", firstCell.Format.Borders)
	}
	if got := row.Cells[1].Format.Shading; got != "00AACC" {
		t.Fatalf("second cell shading = %q, want 00AACC", got)
	}
}

func TestParsePapxGrpprlIgnoresObservedUnmappedTapOpcodes(t *testing.T) {
	grpprl := []byte{
		0x0A, 0x74, 0x01, 0x02, 0x03, 0x04, // 0x740A (size=4)
		0x13, 0xD6, 0x01, 0xAA, // 0xD613 (var: len=1 + payload)
		0x14, 0xF6, 0x11, 0x22, 0x33, // 0xF614 (size=3)
		0x15, 0x36, 0x44, // 0x3615 (size=1)
		0x17, 0xF6, 0x55, 0x66, 0x77, // 0xF617 (size=3)
		0x18, 0xF6, 0x88, 0x99, 0xAA, // 0xF618 (size=3)
		0x1A, 0xD6, 0x01, 0xBB, // 0xD61A (var: len=1 + payload)
		0x1B, 0xD6, 0x01, 0xCC, // 0xD61B (var: len=1 + payload)
		0x1C, 0xD6, 0x01, 0xDD, // 0xD61C (var: len=1 + payload)
		0x1D, 0xD6, 0x01, 0xEE, // 0xD61D (var: len=1 + payload)
		0x61, 0xF6, 0x10, 0x20, 0x30, // 0xF661 (size=3)
		// Known controls prove the parser remains aligned after unmapped opcodes.
		0x00, 0x54, 0x02, 0x00, // sprmTJc90 => right
		0x01, 0x96, 0xE0, 0x00, // sprmTDxaLeft => 224 twips
	}

	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.TableRowProps == nil {
		t.Fatal("TableRowProps = nil, want parsed known TAPx props after unmapped opcodes")
	}
	props := meta.TableRowProps
	if got := props.tableJc; got != "right" {
		t.Fatalf("tableJc = %q, want right", got)
	}
	if props.tableDxaLeft == nil || *props.tableDxaLeft != 224 {
		t.Fatalf("tableDxaLeft = %v, want 224", props.tableDxaLeft)
	}
	if props.tableDxaGap != nil {
		t.Fatalf("tableDxaGap = %v, want nil (no 0x9602 in grpprl)", props.tableDxaGap)
	}
}

func TestBuildLegacyDocFromRunsBinaryTableAppliesTableBorders(t *testing.T) {
	inTable := wordir.ParagraphFormat{InTable: true}
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	borderProps := &legacyTableRowProps{
		tableBorders: &wordir.TableBorders{
			Top:     &wordir.BorderSide{Val: "single", SizePt: 1, Color: "FF0000"},
			Left:    &wordir.BorderSide{Val: "single", SizePt: 1, Color: "00FF00"},
			Bottom:  &wordir.BorderSide{Val: "single", SizePt: 1, Color: "0000FF"},
			Right:   &wordir.BorderSide{Val: "single", SizePt: 1, Color: "000000"},
			InsideH: &wordir.BorderSide{Val: "dashed", SizePt: 1, Color: "FF00FF"},
			InsideV: &wordir.BorderSide{Val: "dotted", SizePt: 1, Color: "00FFFF"},
		},
	}
	runs := []binaryWordRun{
		{Text: "A\aB\a", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "\a", ParaFormat: rowMark, TableProps: borderProps.canonical()},
		{Text: "\r", ParaFormat: rowMark, TableProps: borderProps.canonical()},
	}

	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if doc.Tables[0].Format.Borders == nil {
		t.Fatal("table format borders = nil, want propagated row/table borders")
	}
	if got := doc.Tables[0].Format.Borders.InsideV; got == nil || got.Val != "dotted" {
		t.Fatalf("insideV = %#v, want dotted", got)
	}
	if got := doc.Tables[0].Format.Borders.Top; got == nil || got.Color != "FF0000" {
		t.Fatalf("top = %#v, want red", got)
	}
}

// loadLegacyDocDefaultTabStop reads dxaTab from DOP at FibRgFcLcb97[31].
// MS-DOC §2.7.5: dxaTab is a SHORT at offset 0x0A in the DOP record.
func TestParseDocFloatingPicturesPopulatesDefaultTabStopFromDop(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := filepath.Join("..", "..", "..", "..", "testdata", "doc", "apache-FloatingPictures.doc")
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	// The fixture stores the Word default of 720 twips (0.5 inch).
	if got := doc.DefaultTabStop; got != 720 {
		t.Fatalf("DefaultTabStop = %d, want 720 (DOP.dxaTab)", got)
	}
}

// CHPx sprmCIbstRMark (0x4804) carries an author index into SttbfRMark.
// CHPx sprmCDttmRMark (0x6805) carries a packed DTTM date.
func TestParseChpxGrpprlDecodesRevisionAuthorAndDate(t *testing.T) {
	// Insertion revision toggle, then ibstRMark=2, then a DTTM packed value
	// representing 2026-04-29 (year=126, mon=4, dom=29; minute/hour/wday=0).
	dttm := uint32(0)
	dttm |= uint32(29) << 11
	dttm |= uint32(4) << 16
	dttm |= uint32(126) << 20
	grpprl := []byte{
		0x01, 0x08, 0x01, // sprmCFRMarkIns: on
		0x04, 0x48, 0x02, 0x00, // sprmCIbstRMark: ibst=2
		0x05, 0x68, byte(dttm), byte(dttm >> 8), byte(dttm >> 16), byte(dttm >> 24), // sprmCDttmRMark
	}
	_, meta := parseChpxGrpprlWithMetadata(grpprl)
	if meta.Revision != "inserted" {
		t.Fatalf("Revision = %q, want inserted", meta.Revision)
	}
	if !meta.HasIbstRMark || meta.IbstRMark != 2 {
		t.Fatalf("IbstRMark = (%v, %d), want (true, 2)", meta.HasIbstRMark, meta.IbstRMark)
	}
	if !meta.HasDttmRMark {
		t.Fatal("HasDttmRMark = false, want true")
	}
	if got := formatLegacyRevisionDate(meta.DttmRMark); got != "2026-04-29" {
		t.Fatalf("formatted DTTM = %q, want 2026-04-29", got)
	}
}

func TestResolveLegacyPropertyRevisionMarkDecodesAuthorAndDate(t *testing.T) {
	dttm := uint32(0)
	dttm |= uint32(29) << 11
	dttm |= uint32(4) << 16
	dttm |= uint32(126) << 20
	got := resolveLegacyPropertyRevisionMark([]string{"Alice", "Bob", "Carol"}, true, 2, true, dttm, true)
	if got == nil {
		t.Fatal("resolveLegacyPropertyRevisionMark = nil, want metadata")
	}
	if got.Author != "Carol" {
		t.Fatalf("Author = %q, want Carol", got.Author)
	}
	if got.Date != "2026-04-29" {
		t.Fatalf("Date = %q, want 2026-04-29", got.Date)
	}
}

func TestLoadLegacySttbfRMarkDecodesUnicodeAuthorTable(t *testing.T) {
	const sttbfRMarkFibIndex = 51
	cbRgFcLcb := sttbfRMarkFibIndex + 1
	wordStream := make([]byte, cbRgFcLcb*8)

	authors := []string{"Unknown", "Alice", "Bob"}
	// Build extended STTB: 0xFFFF, cData=3, cbExtra=0, then for each author
	// cchData (uint16) followed by UTF-16LE chars.
	buf := []byte{0xFF, 0xFF, 0x03, 0x00, 0x00, 0x00}
	for _, name := range authors {
		buf = append(buf, byte(len(name)), 0x00)
		for _, r := range name {
			buf = append(buf, byte(r), byte(r>>8))
		}
	}

	// Point FibRgFcLcb97[51] at table-stream offset 0, length len(buf).
	wordStream[sttbfRMarkFibIndex*8] = 0
	for i := 0; i < 4; i++ {
		wordStream[sttbfRMarkFibIndex*8+4+i] = byte(len(buf) >> (8 * i))
	}

	got := loadLegacySttbfRMark(wordStream, buf, 0, cbRgFcLcb)
	if len(got) != len(authors) {
		t.Fatalf("authors = %v (len %d), want %v (len %d)", got, len(got), authors, len(authors))
	}
	for i, want := range authors {
		if got[i] != want {
			t.Fatalf("author[%d] = %q, want %q", i, got[i], want)
		}
	}
}

func TestParseDocFloatingPicturesParsesWithoutTrackedChanges(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := filepath.Join("..", "..", "..", "..", "testdata", "doc", "apache-FloatingPictures.doc")
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	// This fixture does not contain tracked-change runs (no sprmCFRMarkIns/
	// sprmCFRMarkDel in CHPX). Verify that parsing succeeds and no run
	// incorrectly carries a revision flag.
	for _, p := range doc.Paragraphs {
		for _, run := range p.Runs {
			if run.Revision != "" {
				t.Fatalf("unexpected revision on run: Kind=%q Revision=%q", run.Kind, run.Revision)
			}
		}
	}
}

// HWPFOldDocument FIB offset map (HWPFOldDocument.java:91-106) anchors the
// Word 6 / Word 95 table-stream pointers used for future structured CHP/PAP/
// SED parsing. Repo-local apache-Word95.doc and apache-Word6.doc carry the
// shapes documented in WORD95_ASSESSMENT.md; this test locks them so any
// drift surfaces immediately.
func TestInspectPreWord97BodySpanCarriesHWPFOffsetMap(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	for _, tc := range []struct {
		path                                                             string
		wantSed, wantSedSize, wantChp, wantChpSize, wantPap, wantPapSize int
		wantFont, wantFontSize                                           int
		wantFontEntries                                                  []string
	}{
		{
			path:    "../../../../testdata/doc/apache-Word95.doc",
			wantSed: 0x96C, wantSedSize: 20,
			wantChp: 0x980, wantChpSize: 10,
			wantPap: 0x98A, wantPapSize: 10,
			wantFont: 0x994, wantFontSize: 124,
			wantFontEntries: []string{"Times New Roman", "Symbol", "Arial", "Nimbus Roman No9 L", "Times New Roman", "Arial"},
		},
		{
			path:    "../../../../testdata/doc/apache-Word6.doc",
			wantSed: 0x944, wantSedSize: 20,
			wantChp: 0x958, wantChpSize: 10,
			wantPap: 0x962, wantPapSize: 10,
			wantFont: 0x96C, wantFontSize: 83,
			wantFontEntries: []string{"Times New Roman", "Symbol", "Arial", "Times New Roman", "Arial"},
		},
	} {
		t.Run(filepath.Base(tc.path), func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read %s: %v", tc.path, err)
			}
			f, err := cfb.OpenBytes(data)
			if err != nil {
				t.Fatalf("open cfb: %v", err)
			}
			ws, ok := f.Stream("WordDocument")
			if !ok {
				t.Fatal("missing WordDocument stream")
			}
			span, ok := InspectPreWord97BodySpan(ws)
			if !ok {
				t.Fatal("InspectPreWord97BodySpan returned false")
			}
			if span.SedTableOffset != tc.wantSed || span.SedTableSize != tc.wantSedSize {
				t.Fatalf("SED = (0x%X, %d), want (0x%X, %d)", span.SedTableOffset, span.SedTableSize, tc.wantSed, tc.wantSedSize)
			}
			if span.ChpTableOffset != tc.wantChp || span.ChpTableSize != tc.wantChpSize {
				t.Fatalf("CHP = (0x%X, %d), want (0x%X, %d)", span.ChpTableOffset, span.ChpTableSize, tc.wantChp, tc.wantChpSize)
			}
			if span.PapTableOffset != tc.wantPap || span.PapTableSize != tc.wantPapSize {
				t.Fatalf("PAP = (0x%X, %d), want (0x%X, %d)", span.PapTableOffset, span.PapTableSize, tc.wantPap, tc.wantPapSize)
			}
			if span.FontTableOffset != tc.wantFont || span.FontTableSize != tc.wantFontSize {
				t.Fatalf("Font = (0x%X, %d), want (0x%X, %d)", span.FontTableOffset, span.FontTableSize, tc.wantFont, tc.wantFontSize)
			}
			fonts := LoadPreWord97FontTable(ws, span.FontTableOffset, span.FontTableSize)
			if len(fonts) != len(tc.wantFontEntries) {
				t.Fatalf("font entries = %d, want %d", len(fonts), len(tc.wantFontEntries))
			}
			for i, want := range tc.wantFontEntries {
				if fonts[i].Name != want {
					t.Fatalf("font[%d] name = %q, want %q", i, fonts[i].Name, want)
				}
			}
			if span.FComplex {
				t.Fatal("FComplex = true, want false (these fixtures are not fast-saved)")
			}
		})
	}
}

// TestCollectLegacyBinaryTableRowGroupsUsesPreWord97RowMarkAsDelimiter verifies
// that a pre-97 row mark (IsTableTrailingParagraph=true, LegacyRowMarkHint=true)
// correctly acts as a row delimiter in collectLegacyBinaryTableRowGroups. This
// locks the binary-path behavior for pre-97 0x19-toggle row marks.
func TestCollectLegacyBinaryTableRowGroupsUsesPreWord97RowMarkAsDelimiter(t *testing.T) {
	pre97RowMark := wordir.ParagraphFormat{IsTableTrailingParagraph: true}
	width := 1200
	rowProps := &legacyTableRowProps{}
	rowProps.setCellFormat(0, wordir.TableCellFormat{Width: &width, WidthType: "dxa"})
	rowProps.setCellFormat(1, wordir.TableCellFormat{Width: &width, WidthType: "dxa"})

	// Two content lines followed by a pre-97 row mark that terminates the first
	// row, then two more content lines.
	lineSlices := [][]binaryWordRun{
		{{Text: "A\t", NativeCellMarks: true, TableProps: rowProps.canonical()}},
		{{Text: "B\t", NativeCellMarks: true, TableProps: rowProps.canonical()}},
		// Pre-97 row mark: LegacyRowMarkHint distinguishes it from Word 97+ marks.
		{{Text: "", ParaFormat: pre97RowMark, LegacyRowMarkHint: true, TableProps: rowProps.canonical()}},
		{{Text: "C\t", NativeCellMarks: true, TableProps: rowProps.canonical()}},
		{{Text: "D\t", NativeCellMarks: true, TableProps: rowProps.canonical()}},
	}

	rowGroups := collectLegacyBinaryTableRowGroups(lineSlices)
	if len(rowGroups) != 2 {
		t.Fatalf("row groups = %d, want 2 (pre-97 row mark should end the first row)", len(rowGroups))
	}
	if len(rowGroups[0].LineSlices) != 2 {
		t.Fatalf("row 0 line slices = %d, want 2", len(rowGroups[0].LineSlices))
	}
	if len(rowGroups[1].LineSlices) != 2 {
		t.Fatalf("row 1 line slices = %d, want 2", len(rowGroups[1].LineSlices))
	}
	// Verify the pre-97 row mark is captured in the RowMark field.
	if rowGroups[0].RowMark == nil {
		t.Fatal("row group 0 RowMark = nil, want the pre-97 row mark slice")
	}
	if !lineSliceHasPreWord97RowMark(rowGroups[0].RowMark) {
		t.Fatal("row group 0 RowMark should be identified as a pre-97 row mark")
	}
}

func TestBuildLegacyBinaryTablePromotesPreWord97ContentRowMarkPropertyRevision(t *testing.T) {
	revision := &wordir.RevisionMark{Author: "Bob", Date: "2026-05-09"}
	rowGroups := []legacyBinaryTableRow{{
		LineSlices: [][]binaryWordRun{
			{{Text: "A", NativeCellMarks: true, TableDepth: 1}},
			{{Text: "B", LegacyRowMarkHint: true, NativeCellMarks: true, TableDepth: 1, PropertyRevision: revision}},
		},
	}}

	table, _ := buildLegacyBinaryTable(0, rowGroups, 0)
	if table == nil {
		t.Fatal("buildLegacyBinaryTable = nil, want one-row table")
	}
	if len(table.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(table.Rows))
	}
	rowRevision := table.Rows[0].PropertyRevision
	if rowRevision == nil {
		t.Fatal("row PropertyRevision = nil, want metadata from pre-97 content-bearing row mark")
	}
	if got := rowRevision.Author; got != "Bob" {
		t.Fatalf("row PropertyRevision.Author = %q, want Bob", got)
	}
	if got := rowRevision.Date; got != "2026-05-09" {
		t.Fatalf("row PropertyRevision.Date = %q, want 2026-05-09", got)
	}
}

// TestLineSliceIsPreWord97RowMarkContentDetectsContentBearingRowMarks verifies
// that lineSliceIsPreWord97RowMarkContent correctly identifies line slices from
// pre-97 row mark paragraphs (0x19 toggle) that carry actual cell content. Such
// slices have LegacyRowMarkHint=true and NativeCellMarks=true but may not have
// IsTableTrailingParagraph=true (because their text ends in \r, not \a).
func TestLineSliceIsPreWord97RowMarkContentDetectsContentBearingRowMarks(t *testing.T) {
	// Content-bearing row mark: LegacyRowMarkHint=true, NativeCellMarks=true,
	// IsTableTrailingParagraph=false (ends in \r not \a after PAP collapsing).
	contentRowMarkSlice := []binaryWordRun{
		{Text: "Cell1\tCell2", LegacyRowMarkHint: true, NativeCellMarks: true},
	}
	if !lineSliceIsPreWord97RowMarkContent(contentRowMarkSlice) {
		t.Fatal("lineSliceIsPreWord97RowMarkContent = false, want true for content-bearing pre-97 row mark")
	}

	// Explicit row mark (ends in \a): IsTableTrailingParagraph=true, LegacyRowMarkHint=true.
	// Also matches lineSliceIsPreWord97RowMarkContent (NativeCellMarks is set).
	explicitRowMarkSlice := []binaryWordRun{
		{
			Text:              "",
			ParaFormat:        wordir.ParagraphFormat{IsTableTrailingParagraph: true},
			LegacyRowMarkHint: true,
			NativeCellMarks:   true,
		},
	}
	if !lineSliceIsPreWord97RowMarkContent(explicitRowMarkSlice) {
		t.Fatal("lineSliceIsPreWord97RowMarkContent = false, want true for explicit pre-97 row mark with NativeCellMarks")
	}

	// Word 97+ row mark: IsTableTrailingParagraph=true, LegacyRowMarkHint=false.
	// Should NOT match lineSliceIsPreWord97RowMarkContent.
	word97RowMarkSlice := []binaryWordRun{
		{
			Text:       "",
			ParaFormat: wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true},
		},
	}
	if lineSliceIsPreWord97RowMarkContent(word97RowMarkSlice) {
		t.Fatal("lineSliceIsPreWord97RowMarkContent = true, want false for Word 97+ row mark")
	}

	// Regular in-table line without row mark hint.
	regularSlice := []binaryWordRun{
		{Text: "A\tB", NativeCellMarks: true, LegacyInTableHint: true},
	}
	if lineSliceIsPreWord97RowMarkContent(regularSlice) {
		t.Fatal("lineSliceIsPreWord97RowMarkContent = true, want false for regular in-table line")
	}
}

func TestLineSliceHasPreWord97CellMarkDetectsPreWord97CellMarkSlices(t *testing.T) {
	// A pre-Word97 cell mark has IsTableCellTrailingParagraph=true AND LegacyInTableHint=true.
	cellMarkSlice := []binaryWordRun{
		{
			Text:              "A\t",
			LegacyInTableHint: true,
			ParaFormat:        wordir.ParagraphFormat{IsTableCellTrailingParagraph: true},
		},
	}
	if !lineSliceHasPreWord97CellMark(cellMarkSlice) {
		t.Fatal("lineSliceHasPreWord97CellMark = false, want true for pre-97 cell mark slice")
	}

	// Without LegacyInTableHint the cell mark is NOT pre-Word97.
	word97CellMarkSlice := []binaryWordRun{
		{
			Text:       "B\t",
			ParaFormat: wordir.ParagraphFormat{IsTableCellTrailingParagraph: true},
		},
	}
	if lineSliceHasPreWord97CellMark(word97CellMarkSlice) {
		t.Fatal("lineSliceHasPreWord97CellMark = true, want false for Word-97 cell mark (no LegacyInTableHint)")
	}

	// LegacyInTableHint alone (no IsTableCellTrailingParagraph) is not a cell mark.
	legacyInTableOnly := []binaryWordRun{
		{Text: "C\t", LegacyInTableHint: true},
	}
	if lineSliceHasPreWord97CellMark(legacyInTableOnly) {
		t.Fatal("lineSliceHasPreWord97CellMark = true, want false for LegacyInTableHint-only slice")
	}

	// Empty slice returns false.
	if lineSliceHasPreWord97CellMark(nil) {
		t.Fatal("lineSliceHasPreWord97CellMark = true, want false for nil slice")
	}
}

// TestCollectLegacyBinaryTableRowGroupsContentBearingPreWord97RowMarkClosesRow
// verifies that a content-bearing pre-97 row mark (LegacyRowMarkHint=true,
// NativeCellMarks=true, IsTableTrailingParagraph=false) closes the current row
// group in collectLegacyBinaryTableRowGroups while contributing its own text to
// that row's cells. This guards against the case where CHPX-level propagation
// sets LegacyRowMarkHint on non-row-mark runs: only NativeCellMarks-carrying
// slices are treated as implicit row delimiters.
func TestCollectLegacyBinaryTableRowGroupsContentBearingPreWord97RowMarkClosesRow(t *testing.T) {
	width := 1200
	rowProps := &legacyTableRowProps{}
	rowProps.setCellFormat(0, wordir.TableCellFormat{Width: &width, WidthType: "dxa"})
	rowProps.setCellFormat(1, wordir.TableCellFormat{Width: &width, WidthType: "dxa"})

	// Row 1: two cell slices + a content-bearing row mark (ends in \r, not \a).
	// Row 2: two cell slices (no explicit row mark — closed by end-of-input).
	lineSlices := [][]binaryWordRun{
		{{Text: "A\t", NativeCellMarks: true, TableProps: rowProps.canonical()}},
		{{Text: "B\t", NativeCellMarks: true, TableProps: rowProps.canonical()}},
		// Content-bearing pre-97 row mark: LegacyRowMarkHint+NativeCellMarks but
		// no IsTableTrailingParagraph (paragraph ends in \r after PAP collapsing).
		{{Text: "extra\r", LegacyRowMarkHint: true, NativeCellMarks: true, TableProps: rowProps.canonical()}},
		{{Text: "C\t", NativeCellMarks: true, TableProps: rowProps.canonical()}},
		{{Text: "D\t", NativeCellMarks: true, TableProps: rowProps.canonical()}},
	}

	rowGroups := collectLegacyBinaryTableRowGroups(lineSlices)
	if len(rowGroups) != 2 {
		t.Fatalf("row groups = %d, want 2 (content-bearing pre-97 row mark should close row 1)", len(rowGroups))
	}

	// Row 0 has 3 LineSlices: A, B, and the content-bearing row mark itself.
	if len(rowGroups[0].LineSlices) != 3 {
		t.Fatalf("row 0 line slices = %d, want 3 (includes the content-bearing row mark)", len(rowGroups[0].LineSlices))
	}
	// Row 0 has no explicit RowMark (closed by content-bearing mark, not a \a-terminated mark).
	if rowGroups[0].RowMark != nil {
		t.Fatal("row 0 RowMark should be nil when closed by a content-bearing pre-97 row mark")
	}

	// Row 1 has 2 LineSlices: C and D.
	if len(rowGroups[1].LineSlices) != 2 {
		t.Fatalf("row 1 line slices = %d, want 2", len(rowGroups[1].LineSlices))
	}

	// A regular in-table run with LegacyRowMarkHint but no NativeCellMarks must
	// NOT close the row — this is the CHPX-propagation guard.
	legacyHintOnlySlice := []binaryWordRun{
		{Text: "plain\r", LegacyRowMarkHint: true},
	}
	lineSlicesWithFalseHint := [][]binaryWordRun{
		{{Text: "A\t", NativeCellMarks: true, TableProps: rowProps.canonical()}},
		legacyHintOnlySlice,
		{{Text: "B\t", NativeCellMarks: true, TableProps: rowProps.canonical()}},
	}
	rowGroupsFalse := collectLegacyBinaryTableRowGroups(lineSlicesWithFalseHint)
	if len(rowGroupsFalse) != 1 {
		t.Fatalf("guard check: row groups = %d, want 1 (LegacyRowMarkHint without NativeCellMarks must not close rows)", len(rowGroupsFalse))
	}
}

func TestBuildLegacyBinaryTablePreservesCellSpecificRevisionAttribution(t *testing.T) {
	rowRevision := &wordir.RevisionMark{Author: "Row Author", Date: "2026-04-29"}
	cellARevision := &wordir.RevisionMark{Author: "Alice", Date: "2026-05-01"}
	cellBRevision := &wordir.RevisionMark{Author: "Bob", Date: "2026-05-02"}
	props := &legacyTableRowProps{resolvedPropertyRevision: rowRevision}

	group := legacyBinaryTableRow{
		LineSlices: [][]binaryWordRun{{
			{Text: "A", PropertyRevision: cellARevision, Revision: "inserted", RevisionAuthor: "Alice", RevisionDate: "2026-05-01"},
			{Text: "\t"},
			{Text: "B", PropertyRevision: cellBRevision, Revision: "deleted", RevisionAuthor: "Bob", RevisionDate: "2026-05-02"},
		}},
		RowMark: []binaryWordRun{{TableProps: props.canonical()}},
	}

	table, trailing := buildLegacyBinaryTable(0, []legacyBinaryTableRow{group}, 0)
	if table == nil || len(trailing) != 0 || len(table.Rows) != 1 || len(table.Rows[0].Cells) != 2 {
		t.Fatalf("unexpected table result: table=%+v trailing=%v", table, trailing)
	}
	row := table.Rows[0]
	if row.PropertyRevision == nil || row.PropertyRevision.Author != "Row Author" {
		t.Fatalf("row revision = %+v, want Row Author", row.PropertyRevision)
	}
	if row.Cells[0].PropertyRevision == nil || row.Cells[0].PropertyRevision.Author != "Alice" {
		t.Fatalf("cell A property revision = %+v, want Alice", row.Cells[0].PropertyRevision)
	}
	if row.Cells[0].CellIns == nil || row.Cells[0].CellIns.Author != "Alice" || row.Cells[0].CellDel != nil {
		t.Fatalf("cell A tracked state = ins:%+v del:%+v", row.Cells[0].CellIns, row.Cells[0].CellDel)
	}
	if row.Cells[1].PropertyRevision == nil || row.Cells[1].PropertyRevision.Author != "Bob" {
		t.Fatalf("cell B property revision = %+v, want Bob", row.Cells[1].PropertyRevision)
	}
	if row.Cells[1].CellDel == nil || row.Cells[1].CellDel.Author != "Bob" || row.Cells[1].CellIns != nil {
		t.Fatalf("cell B tracked state = ins:%+v del:%+v", row.Cells[1].CellIns, row.Cells[1].CellDel)
	}
	// Different cell property authors must not be collapsed into a table property revision.
	if table.PropertyRevision == nil || table.PropertyRevision.Author != "Row Author" {
		t.Fatalf("table revision = %+v, want shared row revision", table.PropertyRevision)
	}
}

func TestLegacyCellRevisionConsensusDoesNotAttributeMixedContent(t *testing.T) {
	meta := legacyCellRevisionMetadata([]binaryWordRun{
		{Text: "old", PropertyRevision: &wordir.RevisionMark{Author: "Alice", Date: "2026-05-01"}, Revision: "deleted", RevisionAuthor: "Alice", RevisionDate: "2026-05-01"},
		{Text: "new", PropertyRevision: &wordir.RevisionMark{Author: "Bob", Date: "2026-05-02"}, Revision: "inserted", RevisionAuthor: "Bob", RevisionDate: "2026-05-02"},
	})
	if meta.PropertyRevision != nil || meta.CellIns != nil || meta.CellDel != nil {
		t.Fatalf("mixed cell content should not receive whole-cell attribution: %+v", meta)
	}
}

func TestBuildLegacyBinaryTableInfersMergeRevisionFromCellSpecificPropertyRevision(t *testing.T) {
	cellRevision := &wordir.RevisionMark{Author: "Merge Author", Date: "2026-05-03"}
	props := &legacyTableRowProps{}
	props.setHMerge(0, 2)
	group := legacyBinaryTableRow{
		LineSlices: [][]binaryWordRun{{
			{Text: "Merged", PropertyRevision: cellRevision},
			{Text: "\t"},
			{Text: "", PropertyRevision: cellRevision},
		}},
		RowMark: []binaryWordRun{{TableProps: props.canonical()}},
	}
	table, _ := buildLegacyBinaryTable(0, []legacyBinaryTableRow{group}, 0)
	if table == nil || len(table.Rows) != 1 || len(table.Rows[0].Cells) != 1 {
		t.Fatalf("unexpected merged table: %+v", table)
	}
	cell := table.Rows[0].Cells[0]
	if !cell.Format.HMerge || cell.Format.GridSpan != 2 {
		t.Fatalf("merge geometry = %+v", cell.Format)
	}
	if cell.CellMerge == nil || cell.CellMerge.Author != "Merge Author" {
		t.Fatalf("cell merge revision = %+v, want Merge Author", cell.CellMerge)
	}
}
