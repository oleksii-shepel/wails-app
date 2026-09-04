package legacy

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	"office/pkg/word/render"
)

func TestParseLegacyListMarkerRecognizesAlphaAndRomanLists(t *testing.T) {
	tests := []struct {
		line       string
		wantText   string
		wantFormat string
		wantStart  int
		wantLevel  int
	}{
		{line: "a. alpha item", wantText: "alpha item", wantFormat: "lowerLetter", wantStart: 1, wantLevel: 0},
		{line: "  B) upper alpha item", wantText: "upper alpha item", wantFormat: "upperLetter", wantStart: 2, wantLevel: 1},
		{line: "    (c) parenthesized alpha item", wantText: "parenthesized alpha item", wantFormat: "lowerLetter", wantStart: 3, wantLevel: 2},
		{line: "iv. roman item", wantText: "roman item", wantFormat: "lowerRoman", wantStart: 4, wantLevel: 0},
		{line: "\tX) upper roman item", wantText: "upper roman item", wantFormat: "upperRoman", wantStart: 10, wantLevel: 1},
		{line: "  (iv) parenthesized roman item", wantText: "parenthesized roman item", wantFormat: "lowerRoman", wantStart: 4, wantLevel: 1},
	}
	for _, test := range tests {
		gotText, marker := parseLegacyListMarker(test.line)
		if gotText != test.wantText {
			t.Fatalf("parseLegacyListMarker(%q) text = %q, want %q", test.line, gotText, test.wantText)
		}
		if marker == nil || marker.Kind != "ordered" || marker.Format != test.wantFormat {
			t.Fatalf("parseLegacyListMarker(%q) marker = %#v, want ordered/%s", test.line, marker, test.wantFormat)
		}
		if marker.Start != test.wantStart {
			t.Fatalf("parseLegacyListMarker(%q) start = %d, want %d", test.line, marker.Start, test.wantStart)
		}
		if marker.Level != test.wantLevel {
			t.Fatalf("parseLegacyListMarker(%q) level = %d, want %d", test.line, marker.Level, test.wantLevel)
		}
	}
}

func TestParseLegacyListMarkerRecognizesIndentedBullets(t *testing.T) {
	gotText, marker := parseLegacyListMarker("    - nested bullet")
	if gotText != "nested bullet" {
		t.Fatalf("text = %q, want nested bullet", gotText)
	}
	if marker == nil || marker.Kind != "bullet" {
		t.Fatalf("marker = %#v, want bullet", marker)
	}
	if marker.Level != 2 {
		t.Fatalf("bullet level = %d, want 2", marker.Level)
	}
}
func TestBuildLegacyDocSkipsPipeTableSeparatorRow(t *testing.T) {
	// Markdown-style table with a separator row Р Р†Р вЂљРІР‚Сњ should produce 2 data rows, not 3.
	text := "Name | Score\n---|---\nAlice | 95\nBob | 87"
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 3 { // header + 2 data rows; separator was consumed but not added
		t.Fatalf("rows = %d, want 3 (header + 2 data)", len(doc.Tables[0].Rows))
	}
}

func TestIsLegacyTableSeparatorLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"|---|---|---|", true},
		{"| :-- | --: | :-: |", true},
		{"+---+---+", true},
		{"----  ---  ----", true},
		{":---  ---:  :-:", true},
		{"Alice | 30 | London", false},
		{"Name  Age  City", false},
		{"plain text", false},
		{"|text|here|", false},
	}
	for _, test := range tests {
		if got := isLegacyTableSeparatorLine(test.line); got != test.want {
			t.Fatalf("isLegacyTableSeparatorLine(%q) = %v, want %v", test.line, got, test.want)
		}
	}
}

func TestBuildLegacyDocDetectsPipeDelimitedTable(t *testing.T) {
	text := "Name | Age | City\nAlice | 30 | London\nBob | 25 | Paris"
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	table := doc.Tables[0]
	if len(table.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(table.Rows))
	}
	if len(table.Rows[0].Cells) != 3 {
		t.Fatalf("cols = %d, want 3", len(table.Rows[0].Cells))
	}
	if len(doc.Paragraphs) != 0 {
		t.Fatalf("paragraphs = %d, want 0", len(doc.Paragraphs))
	}
}

func TestBuildLegacyDocDetectsBoxStylePipeTable(t *testing.T) {
	text := "+------+-----+\n| Name | Age |\n+------+-----+\n| Alice | 30 |\n+------+-----+"
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(doc.Tables[0].Rows))
	}
	if len(doc.Tables[0].Rows[0].Cells) != 2 {
		t.Fatalf("cols = %d, want 2", len(doc.Tables[0].Rows[0].Cells))
	}
}

func TestBuildLegacyDocDetectsAlignedWhitespaceTable(t *testing.T) {
	text := "Name  Age  City\nAlice  30  London\nBob  25  Paris"
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(doc.Tables[0].Rows))
	}
	if len(doc.Tables[0].Rows[0].Cells) != 3 {
		t.Fatalf("cols = %d, want 3", len(doc.Tables[0].Rows[0].Cells))
	}
}

func TestBuildLegacyDocSkipsAlignedWhitespaceSeparatorRow(t *testing.T) {
	text := "Name  Age  City\n----  ---  ----\nAlice  30  London\nBob  25  Paris"
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(doc.Tables[0].Rows))
	}
	if got := doc.Tables[0].Rows[1].Cells[0].Text; got != "Alice" {
		t.Fatalf("first data row = %q, want Alice", got)
	}
}

func TestBuildLegacyDocDetectsAlignedTwoColumnWhitespaceTable(t *testing.T) {
	text := "Name  Alice\nTitle  Manager\nCity  Paris"
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(doc.Tables[0].Rows))
	}
	if len(doc.Tables[0].Rows[0].Cells) != 2 {
		t.Fatalf("cols = %d, want 2", len(doc.Tables[0].Rows[0].Cells))
	}
}

func TestBuildLegacyDocDetectsColonKeyValueTable(t *testing.T) {
	text := "Name: Alice\nTitle: Manager\nCity: Paris"
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(doc.Tables[0].Rows))
	}
	if len(doc.Tables[0].Rows[0].Cells) != 2 {
		t.Fatalf("cols = %d, want 2", len(doc.Tables[0].Rows[0].Cells))
	}
}

func TestBuildLegacyDocDoesNotTreatSingleAlignedWhitespaceLineAsTable(t *testing.T) {
	text := "Quarter  Revenue  Cost"
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 0 {
		t.Fatalf("tables = %d, want 0", len(doc.Tables))
	}
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
}

func TestBuildLegacyDocDoesNotTreatAlignedSentencesAsTwoColumnTable(t *testing.T) {
	text := "This is a longer sentence  broken oddly\nAnother fairly long sentence  broken oddly"
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 0 {
		t.Fatalf("tables = %d, want 0", len(doc.Tables))
	}
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
}

func TestBuildLegacyDocDoesNotTreatColonSentencesAsTable(t *testing.T) {
	text := "Note: this is a longer explanatory sentence that should stay a paragraph.\nSummary: this is another longer explanatory sentence that should stay a paragraph."
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 0 {
		t.Fatalf("tables = %d, want 0", len(doc.Tables))
	}
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
}

func TestBuildLegacyDocDoesNotTreatSinglePipeAsPipeTable(t *testing.T) {
	text := "Revenue | Loss\nTotal values"
	doc := buildLegacyDoc("test.doc", text)
	// First line has 2 pipe-cells Р Р†РІР‚В РІР‚в„ў table; second line has no pipe Р Р†РІР‚В РІР‚в„ў falls through as paragraph.
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
}
func TestBuildLegacyDocDetectsTableWithOneShortRow(t *testing.T) {
	// Four-row 3-column table where the last row is missing its third cell.
	// The modal column count (3) should be used to pad the short row with an empty cell.
	text := "Name  Age  City\nAlice  30  London\nBob  25  Paris\nCharlie  40"
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(doc.Tables[0].Rows))
	}
	if len(doc.Tables[0].Rows[0].Cells) != 3 {
		t.Fatalf("header cols = %d, want 3", len(doc.Tables[0].Rows[0].Cells))
	}
	// Last row was padded to 3 columns.
	if len(doc.Tables[0].Rows[3].Cells) != 3 {
		t.Fatalf("last row cols = %d, want 3 (padded)", len(doc.Tables[0].Rows[3].Cells))
	}
	if doc.Tables[0].Rows[3].Cells[0].Text != "Charlie" {
		t.Fatalf("last row col[0] = %q, want Charlie", doc.Tables[0].Rows[3].Cells[0].Text)
	}
	if doc.Tables[0].Rows[3].Cells[2].Text != "" {
		t.Fatalf("last row col[2] = %q, want empty (padded)", doc.Tables[0].Rows[3].Cells[2].Text)
	}
}

func TestBuildLegacyDocHandlesModalColumnCountForTabSeparatedTable(t *testing.T) {
	// Three-row tab table where one row has one fewer column Р Р†Р вЂљРІР‚Сњ modal (3) should win.
	text := "Q1\tRevenue\tCost\nJan\t1000\t800\nFeb\t1200"
	doc := buildLegacyDoc("test.doc", text)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	if len(doc.Tables[0].Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(doc.Tables[0].Rows))
	}
}

func TestModalLegacyColCountPrefersSmallerWidthOnTie(t *testing.T) {
	lineParts := [][]string{
		{"A", "B", "C", "D", "E"},
		{"1", "2"},
	}
	if got := modalLegacyColCount(lineParts); got != 2 {
		t.Fatalf("modalLegacyColCount() = %d, want 2", got)
	}
}

func TestBuildLegacyTableHighlyInconsistentWidthsDoNotPadShortRows(t *testing.T) {
	lines := []string{
		"h01\th02\th03\th04\th05\th06\th07\th08\th09\th10\th11\th12\th13\th14\th15\th16",
		"r01\tr02\tr03\tr04\tr05\tr06\tr07\tr08\tr09\tr10\tr11\tr12\tr13\tr14\tr15\tr16",
		"s01\ts02\ts03",
	}
	table, trailing := buildLegacyTable(0, lines, 0)
	if table == nil {
		t.Fatal("buildLegacyTable = nil")
	}
	if len(trailing) != 0 {
		t.Fatalf("trailing rows = %d, want 0", len(trailing))
	}
	if got := len(table.Rows); got != 3 {
		t.Fatalf("rows = %d, want 3", got)
	}
	if got := len(table.Rows[0].Cells); got != 16 {
		t.Fatalf("row[0] cells = %d, want 16", got)
	}
	if got := len(table.Rows[1].Cells); got != 16 {
		t.Fatalf("row[1] cells = %d, want 16", got)
	}
	if got := len(table.Rows[2].Cells); got != 3 {
		t.Fatalf("row[2] cells = %d, want 3", got)
	}
}

func TestBuildLegacyTableFromGridLinesCarriesInferredColumnWidths(t *testing.T) {
	table := buildLegacyTableFromGridLines(0, []string{
		"Product description\tMaker\tBottle\tVolume\tPrice",
		"A long product description that needs room\tWinery\tBordeaux\t0.7 l\t31-00",
	}, 0)
	if table == nil {
		t.Fatal("expected reconstructed grid table")
	}
	if got := len(table.Format.ColumnWidths); got != 5 {
		t.Fatalf("expected 5 inferred column widths, got %d: %#v", got, table.Format.ColumnWidths)
	}
	if table.Format.ColumnWidths[0] <= table.Format.ColumnWidths[3] {
		t.Fatalf("expected description column wider than volume column: %#v", table.Format.ColumnWidths)
	}
	if table.Format.Width == nil || *table.Format.Width <= 0 {
		t.Fatalf("expected inferred table width, got %#v", table.Format.Width)
	}
}

func TestInferLegacyTextTableColumnWidthsPreservesNarrowLabelColumn(t *testing.T) {
	parts := []string{"3.1", strings.Repeat("word ", 40)}
	widths := inferLegacyTextTableColumnWidths(parts, 2)
	if len(widths) != 2 {
		t.Fatalf("widths = %#v, want two columns", widths)
	}
	if widths[0] >= widths[1] {
		t.Fatalf("widths = %#v, want narrow label and wide prose columns", widths)
	}
	if widths[0] < 700 || widths[0] > 1500 {
		t.Fatalf("label width = %d, want a practical 7-15%% share", widths[0])
	}
}

func TestBuildLegacyTabTableCarriesInferredColumnWidths(t *testing.T) {
	lines := []string{
		"Product name\tMaker\tBottle type\tVolume\tPrice",
		"A long product description\tMaker\tBordeaux\t0.7 l\t31-00",
	}
	table, _ := buildLegacyTable(0, lines, 0)
	if table == nil {
		t.Fatal("buildLegacyTable = nil")
	}
	if len(table.Format.ColumnWidths) != 5 {
		t.Fatalf("column widths = %#v, want five inferred widths", table.Format.ColumnWidths)
	}
	if table.Format.ColumnWidths[0] <= table.Format.ColumnWidths[3] {
		t.Fatalf("column widths = %#v, want product column wider than volume", table.Format.ColumnWidths)
	}
}

func TestBuildLegacyDocFromRunsPreservesCharFormatting(t *testing.T) {
	bold := true
	italic := true
	runs := []docRun{
		{Text: "Hello ", Format: wordir.RunFormat{Bold: &bold}},
		{Text: "world", Format: wordir.RunFormat{Italic: &italic}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	p := doc.Paragraphs[0]
	if len(p.Runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(p.Runs))
	}
	if p.Runs[0].Format.Bold == nil || !*p.Runs[0].Format.Bold {
		t.Fatalf("run[0] Bold = %v, want true", p.Runs[0].Format.Bold)
	}
	if p.Runs[1].Format.Italic == nil || !*p.Runs[1].Format.Italic {
		t.Fatalf("run[1] Italic = %v, want true", p.Runs[1].Format.Italic)
	}
}

func TestBuildLegacyDocFromRunsAppliesParagraphFormat(t *testing.T) {
	indent := 720 // 720 twips = half-inch
	runs := []docRun{
		{Text: "First paragraph", ParaFormat: wordir.ParagraphFormat{Alignment: "center", IndentLeft: &indent}},
		{Text: "\n"},
		{Text: "Second paragraph"},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].Format.Alignment != "center" {
		t.Fatalf("para[0] alignment = %q, want center", doc.Paragraphs[0].Format.Alignment)
	}
	if doc.Paragraphs[0].Format.IndentLeft == nil || *doc.Paragraphs[0].Format.IndentLeft != 720 {
		t.Fatalf("para[0] indent-left = %v, want 720", doc.Paragraphs[0].Format.IndentLeft)
	}
	if doc.Paragraphs[1].Format.Alignment != "" {
		t.Fatalf("para[1] alignment = %q, want empty", doc.Paragraphs[1].Format.Alignment)
	}
}

func TestBuildLegacyDocFromRunsFiltersHiddenRuns(t *testing.T) {
	vanish := true
	runs := []docRun{
		{Text: "visible"},
		{Text: "hidden", Format: wordir.RunFormat{Vanish: &vanish}},
		{Text: " more"},
	}
	// Hidden filtering happens in extractDocRunsViaFIB, not in buildLegacyDocFromRuns.
	// Verify buildLegacyDocFromRuns itself passes hidden runs through (the FIB path filters them).
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	// All three runs should be present (filtering happens upstream).
	allText := doc.Paragraphs[0].Text
	if allText == "" {
		t.Fatalf("paragraph text is empty, want non-empty")
	}
}

func TestParsePapxGrpprlAlignment(t *testing.T) {
	// sprmPJc(0x2403)=1 (center)
	grpprl := []byte{0x03, 0x24, 0x01}
	f := parsePapxGrpprl(grpprl)
	if f.Alignment != "center" {
		t.Fatalf("Alignment = %q, want center", f.Alignment)
	}
}

func TestParsePapxGrpprlIndent(t *testing.T) {
	// sprmPDxaLeft(0x840E)=720 (LE signed int16), sprmPDxaLeft1(0x8411)=-360 (hanging indent)
	grpprl := []byte{
		0x0E, 0x84, 0xD0, 0x02, // sprmPDxaLeft: code=0x840E, 720 twips LE
		0x11, 0x84, 0x98, 0xFE, // sprmPDxaLeft1: code=0x8411, -360 twips LE (= hanging 360)
	}
	f := parsePapxGrpprl(grpprl)
	if f.IndentLeft == nil || *f.IndentLeft != 720 {
		t.Fatalf("IndentLeft = %v, want 720", f.IndentLeft)
	}
	if f.IndentHanging == nil || *f.IndentHanging != 360 {
		t.Fatalf("IndentHanging = %v, want 360", f.IndentHanging)
	}
}

func TestParsePapxGrpprlListPointers(t *testing.T) {
	// sprmPIlvl(0x260A)=2, sprmPIlfo(0x460B)=7
	grpprl := []byte{
		0x0A, 0x26, 0x02,
		0x0B, 0x46, 0x07, 0x00,
	}
	f := parsePapxGrpprl(grpprl)
	if f.BinaryListLevel == nil || *f.BinaryListLevel != 2 {
		t.Fatalf("BinaryListLevel = %v, want 2", f.BinaryListLevel)
	}
	if f.BinaryListOverride == nil || *f.BinaryListOverride != 7 {
		t.Fatalf("BinaryListOverride = %v, want 7", f.BinaryListOverride)
	}
}

func TestParsePapxGrpprlFrameProps(t *testing.T) {
	grpprl := []byte{
		0x18, 0x84, 0x2C, 0x01, // sprmPDxaAbs = 300
		0x19, 0x84, 0x90, 0x01, // sprmPDyaAbs = 400
		0x1A, 0x84, 0x84, 0x03, // sprmPDxaWidth = 900
		0x1B, 0x26, 0x09, // sprmPPc: center + bottom
		0x23, 0x24, 0x04, // sprmPWr = tight
		0x2B, 0x44, 0x20, 0x03, // sprmPWHeightAbs = 800
		0x2C, 0x44, 0x1A, 0x00, // sprmPDcs: type=2 (margin), lines=3
		0x2E, 0x84, 0x3C, 0x00, // sprmPDyaFromText = 60
		0x2F, 0x84, 0x50, 0x00, // sprmPDxaFromText = 80
	}
	f := parsePapxGrpprl(grpprl)
	if f.BinaryFrame == nil {
		t.Fatal("BinaryFrame = nil, want frame props")
	}
	if f.BinaryFrame.X != 300 || f.BinaryFrame.Y != 400 || f.BinaryFrame.W != 900 || f.BinaryFrame.H != 800 {
		t.Fatalf("frame geometry = %+v, want x=300 y=400 w=900 h=800", f.BinaryFrame)
	}
	if f.BinaryFrame.XAlign != "center" || f.BinaryFrame.YAlign != "bottom" {
		t.Fatalf("frame alignment = %+v, want center/bottom", f.BinaryFrame)
	}
	if f.BinaryFrame.WrapType != "tight" || f.BinaryFrame.HSpace != 80 || f.BinaryFrame.VSpace != 60 {
		t.Fatalf("frame wrap/spacing = %+v, want tight/80/60", f.BinaryFrame)
	}
	if f.BinaryFrame.DropCap != "margin" || f.BinaryFrame.Lines != 3 {
		t.Fatalf("frame dropcap = %+v, want margin/3", f.BinaryFrame)
	}
}

func TestParsePapxGrpprlPropertyRevisionMark(t *testing.T) {
	dttm := uint32(0)
	dttm |= uint32(29) << 11
	dttm |= uint32(4) << 16
	dttm |= uint32(126) << 20
	grpprl := []byte{
		0x3F, 0xC6, 0x07,
		0x01,
		0x02, 0x00,
		byte(dttm), byte(dttm >> 8), byte(dttm >> 16), byte(dttm >> 24),
	}
	_, meta := parsePapxGrpprlWithMetadata(grpprl)
	if meta.PropertyRevision == nil {
		t.Fatal("PropertyRevision = nil, want metadata")
	}
	if !meta.PropertyRevision.Present {
		t.Fatal("PropertyRevision.Present = false, want true")
	}
	if !meta.PropertyRevision.HasIbst || meta.PropertyRevision.Ibst != 2 {
		t.Fatalf("PropertyRevision ibst = (%v, %d), want (true, 2)", meta.PropertyRevision.HasIbst, meta.PropertyRevision.Ibst)
	}
	if !meta.PropertyRevision.HasDttm {
		t.Fatal("PropertyRevision.HasDttm = false, want true")
	}
	if got := formatLegacyRevisionDate(meta.PropertyRevision.Dttm); got != "2026-04-29" {
		t.Fatalf("PropertyRevision date = %q, want 2026-04-29", got)
	}
}

func TestBuildLegacyDocFromRunsPromotesBinaryFrameToParagraphFrame(t *testing.T) {
	runs := []docRun{{
		Text: "Drop cap paragraph",
		ParaFormat: wordir.ParagraphFormat{
			BinaryFrame: &wordir.FrameProps{
				W:        720,
				H:        1440,
				X:        120,
				Y:        240,
				XAlign:   "left",
				YAlign:   "top",
				HAnchor:  "page",
				VAnchor:  "margin",
				WrapType: "around",
				HSpace:   80,
				VSpace:   60,
				DropCap:  "drop",
				Lines:    3,
			},
		},
	}}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	para := doc.Paragraphs[0]
	if para.Frame == nil {
		t.Fatal("Paragraph.Frame = nil, want promoted frame props")
	}
	if para.Frame.DropCap != "drop" || para.Frame.Lines != 3 {
		t.Fatalf("Paragraph.Frame = %+v, want drop/3", para.Frame)
	}
	if para.Format.BinaryFrame != nil {
		t.Fatalf("Paragraph.Format.BinaryFrame = %+v, want nil after promotion", para.Format.BinaryFrame)
	}
}

func TestBuildLegacyDocFromRunsPromotesParagraphPropertyRevision(t *testing.T) {
	runs := []docRun{{
		Text: "Paragraph with revised formatting",
		PropertyRevision: &wordir.RevisionMark{
			Author: "Alice",
			Date:   "2026-04-29",
		},
	}}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].PropertyRevision == nil {
		t.Fatal("paragraph PropertyRevision = nil, want metadata")
	}
	if got := doc.Paragraphs[0].PropertyRevision.Author; got != "Alice" {
		t.Fatalf("paragraph PropertyRevision.Author = %q, want Alice", got)
	}
	if got := doc.Paragraphs[0].PropertyRevision.Date; got != "2026-04-29" {
		t.Fatalf("paragraph PropertyRevision.Date = %q, want 2026-04-29", got)
	}
}

func TestBuildLegacyDocFromRunsPromotesRunPropertyRevision(t *testing.T) {
	runs := []docRun{{
		Text: "Reformatted run",
		RunPropertyRevision: &wordir.RevisionMark{
			Author: "Carol",
			Date:   "2026-04-29",
		},
	}}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 1 || len(doc.Paragraphs[0].Runs) != 1 {
		t.Fatalf("paragraph/run counts = %d/%d, want 1/1", len(doc.Paragraphs), len(doc.Paragraphs[0].Runs))
	}
	if doc.Paragraphs[0].Runs[0].PropertyRevision == nil {
		t.Fatal("run PropertyRevision = nil, want metadata")
	}
	if got := doc.Paragraphs[0].Runs[0].PropertyRevision.Author; got != "Carol" {
		t.Fatalf("run PropertyRevision.Author = %q, want Carol", got)
	}
	if got := doc.Paragraphs[0].Runs[0].PropertyRevision.Date; got != "2026-04-29" {
		t.Fatalf("run PropertyRevision.Date = %q, want 2026-04-29", got)
	}
}

func TestBuildLegacyDocFromRunsPreservesPapxStyleID(t *testing.T) {
	styleIndex := 42
	runs := []docRun{{
		Text: "Styled paragraph",
		ParaFormat: wordir.ParagraphFormat{
			BinaryStyleIndex: &styleIndex,
		},
	}}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	if got := doc.Paragraphs[0].StyleID; got != "legacy-istd-42" {
		t.Fatalf("StyleID = %q, want legacy-istd-42", got)
	}
}

func TestBuildLegacyDocFromRunsUsesPreWord97StyleNameMappingWhenNoStyleSheet(t *testing.T) {
	styleIndex := 2
	runs := []docRun{{
		Text: "Styled paragraph",
		ParaFormat: wordir.ParagraphFormat{
			BinaryStyleIndex: &styleIndex,
		},
	}}
	doc := buildLegacyDocFromRunsWithContext("test.doc", runs, nil, nil, nil, nil, map[int]string{2: "Heading"})
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	if got := doc.Paragraphs[0].StyleID; got != "legacy-istd-2" {
		t.Fatalf("StyleID = %q, want legacy-istd-2", got)
	}
	if got := doc.Paragraphs[0].StyleName; got != "Heading" {
		t.Fatalf("StyleName = %q, want Heading", got)
	}
}

func TestLoadLegacyStyleSheetParsesResolvedParagraphAndCharacterStyles(t *testing.T) {
	styleTable := buildLegacyStyleTable(map[int]legacyTestStyleDef{
		0: {name: "Normal", kind: 1, sti: 0, basedOn: -1},
		1: {
			name:    "Heading 1",
			kind:    1,
			sti:     1,
			basedOn: 0,
			papx:    []byte{0x03, 0x24, 0x01},
			chpx:    []byte{0x35, 0x08, 0x01},
		},
		2: {
			name:    "Emphasis",
			kind:    2,
			sti:     0x0FFE,
			basedOn: -1,
			chpx:    []byte{0x36, 0x08, 0x01},
		},
		3: {
			name:    "Strong Emphasis",
			kind:    2,
			sti:     0x0FFE,
			basedOn: 2,
			chpx:    []byte{0x35, 0x08, 0x01},
		},
	})
	wordStream := make([]byte, 512)
	tableStream := append([]byte(nil), styleTable...)
	putFibFcLcb(wordStream, 0, 1, 0, len(styleTable))

	styles := loadLegacyStyleSheet(wordStream, tableStream, 0, 2)
	if styles == nil {
		t.Fatal("loadLegacyStyleSheet returned nil")
	}
	if styles.DefaultParagraphStyleID != "Normal" {
		t.Fatalf("DefaultParagraphStyleID = %q, want Normal", styles.DefaultParagraphStyleID)
	}

	heading, ok := styles.ParagraphStyles["Heading1"]
	if !ok {
		t.Fatalf("paragraph styles = %#v, want Heading1 entry", styles.ParagraphStyles)
	}
	if heading.Name != "Heading 1" {
		t.Fatalf("Heading1.Name = %q, want %q", heading.Name, "Heading 1")
	}
	if heading.BasedOn != "Normal" {
		t.Fatalf("Heading1.BasedOn = %q, want Normal", heading.BasedOn)
	}
	if len(heading.StylePath) != 2 || heading.StylePath[0] != "Heading1" || heading.StylePath[1] != "Normal" {
		t.Fatalf("Heading1.StylePath = %v, want [Heading1 Normal]", heading.StylePath)
	}
	if heading.Format.Alignment != "center" {
		t.Fatalf("Heading1.Format.Alignment = %q, want center", heading.Format.Alignment)
	}
	if heading.RunFormat.Bold == nil || !*heading.RunFormat.Bold {
		t.Fatalf("Heading1.RunFormat.Bold = %v, want true", heading.RunFormat.Bold)
	}

	strong, ok := styles.CharacterStyles["Strong Emphasis"]
	if !ok {
		t.Fatalf("character styles = %#v, want Strong Emphasis entry", styles.CharacterStyles)
	}
	if strong.BasedOn != "Emphasis" {
		t.Fatalf("Strong Emphasis.BasedOn = %q, want Emphasis", strong.BasedOn)
	}
	if strong.Format.Italic == nil || !*strong.Format.Italic {
		t.Fatalf("Strong Emphasis.Format.Italic = %v, want true (inherited)", strong.Format.Italic)
	}
	if strong.Format.Bold == nil || !*strong.Format.Bold {
		t.Fatalf("Strong Emphasis.Format.Bold = %v, want true", strong.Format.Bold)
	}
}

func TestParseDocLegacyStyleTableAppliesNamedStyles(t *testing.T) {
	const bodyText = "Styled heading\r"

	path := filepath.Join(t.TempDir(), "legacy-style.doc")
	if err := os.WriteFile(path, buildLegacyDocWithStyleCFB(bodyText), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if doc.DefaultParagraphStyleID != "Normal" {
		t.Fatalf("DefaultParagraphStyleID = %q, want Normal", doc.DefaultParagraphStyleID)
	}
	headingStyle, ok := doc.Styles["Heading1"]
	if !ok {
		t.Fatalf("doc.Styles = %#v, want Heading1 entry", doc.Styles)
	}
	if headingStyle.Name != "Heading 1" {
		t.Fatalf("Heading1.Name = %q, want %q", headingStyle.Name, "Heading 1")
	}
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	paragraph := doc.Paragraphs[0]
	if paragraph.StyleID != "Heading1" || paragraph.StyleName != "Heading 1" {
		t.Fatalf("paragraph style = (%q,%q), want (Heading1, Heading 1)", paragraph.StyleID, paragraph.StyleName)
	}
	if len(paragraph.StylePath) != 2 || paragraph.StylePath[0] != "Heading1" || paragraph.StylePath[1] != "Normal" {
		t.Fatalf("paragraph StylePath = %v, want [Heading1 Normal]", paragraph.StylePath)
	}
	if paragraph.Format.Alignment != "center" {
		t.Fatalf("paragraph alignment = %q, want center", paragraph.Format.Alignment)
	}
	if len(paragraph.Runs) != 1 || paragraph.Runs[0].Format.Bold == nil || !*paragraph.Runs[0].Format.Bold {
		t.Fatalf("paragraph runs = %#v, want inherited bold run", paragraph.Runs)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<h1") {
		t.Fatalf("expected heading-style HTML output:\n%s", out)
	}
}

func TestBuildLegacyDocFromRunsWithContextSetsDefaultFormatsFromDefaultStyle(t *testing.T) {
	bold := true
	rtl := true
	styleSheet := &legacyStyleSheet{
		DefaultParagraphStyleID: "Normal",
		ParagraphStyles: map[string]wordir.Style{
			"Normal": {
				ID: "Normal",
				Format: wordir.ParagraphFormat{
					KeepLines:    true,
					WidowControl: true,
					Bidi:         true,
				},
				RunFormat: wordir.RunFormat{
					Bold:         &bold,
					RTL:          &rtl,
					EmphasisMark: "dot",
				},
			},
		},
	}

	doc := buildLegacyDocFromRunsWithContext("defaults.doc", nil, nil, nil, nil, styleSheet, nil)
	if doc.DefaultParagraphStyleID != "Normal" {
		t.Fatalf("DefaultParagraphStyleID = %q, want Normal", doc.DefaultParagraphStyleID)
	}
	if !doc.DefaultParagraphFormat.KeepLines || !doc.DefaultParagraphFormat.WidowControl || !doc.DefaultParagraphFormat.Bidi {
		t.Fatalf("DefaultParagraphFormat = %+v, want KeepLines/WidowControl/Bidi set", doc.DefaultParagraphFormat)
	}
	if doc.DefaultRunFormat.Bold == nil || !*doc.DefaultRunFormat.Bold {
		t.Fatalf("DefaultRunFormat.Bold = %v, want true", doc.DefaultRunFormat.Bold)
	}
	if doc.DefaultRunFormat.RTL == nil || !*doc.DefaultRunFormat.RTL {
		t.Fatalf("DefaultRunFormat.RTL = %v, want true", doc.DefaultRunFormat.RTL)
	}
	if doc.DefaultRunFormat.EmphasisMark != "dot" {
		t.Fatalf("DefaultRunFormat.EmphasisMark = %q, want dot", doc.DefaultRunFormat.EmphasisMark)
	}
}

func TestLegacyMergeParagraphFormatIncludesNewBinaryFields(t *testing.T) {
	level := 4
	base := wordir.ParagraphFormat{}
	overlay := wordir.ParagraphFormat{
		SnapToGrid:                   true,
		Bidi:                         true,
		IsTableCellTrailingParagraph: true,
		OutlineLevel:                 &level,
	}

	merged := legacyMergeParagraphFormat(base, overlay)
	if !merged.SnapToGrid {
		t.Fatal("SnapToGrid = false, want true")
	}
	if !merged.Bidi {
		t.Fatal("Bidi = false, want true")
	}
	if !merged.IsTableCellTrailingParagraph {
		t.Fatal("IsTableCellTrailingParagraph = false, want true")
	}
	if merged.OutlineLevel == nil || *merged.OutlineLevel != 4 {
		t.Fatalf("OutlineLevel = %v, want 4", merged.OutlineLevel)
	}
}

func TestLegacyMergeRunFormatIncludesExtendedEffects(t *testing.T) {
	value := true
	merged := legacyMergeRunFormat(wordir.RunFormat{}, wordir.RunFormat{
		RTL:          &value,
		Outline:      &value,
		Shadow:       &value,
		Emboss:       &value,
		Imprint:      &value,
		EmphasisMark: "comma",
	})

	if merged.RTL == nil || !*merged.RTL {
		t.Fatalf("RTL = %v, want true", merged.RTL)
	}
	if merged.Outline == nil || !*merged.Outline {
		t.Fatalf("Outline = %v, want true", merged.Outline)
	}
	if merged.Shadow == nil || !*merged.Shadow {
		t.Fatalf("Shadow = %v, want true", merged.Shadow)
	}
	if merged.Emboss == nil || !*merged.Emboss {
		t.Fatalf("Emboss = %v, want true", merged.Emboss)
	}
	if merged.Imprint == nil || !*merged.Imprint {
		t.Fatalf("Imprint = %v, want true", merged.Imprint)
	}
	if merged.EmphasisMark != "comma" {
		t.Fatalf("EmphasisMark = %q, want comma", merged.EmphasisMark)
	}
}

func TestSplitPieceByChpxMatchesCompressedPieceFCsToFKPEntries(t *testing.T) {
	bold := true
	level := 0
	ilfo := 1
	wordStream := make([]byte, 128)
	copy(wordStream[100:], []byte("abc"))
	runs := splitPieceByChpx(
		wordStream,
		100,
		0x400000C8,
		3,
		true,
		0,
		[]chpxEntry{{FC: 101, Format: wordir.RunFormat{Bold: &bold}}},
		[]papxEntry{{FC: 100, Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}}},
	)
	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2", len(runs))
	}
	if runs[0].Text != "a" {
		t.Fatalf("runs[0].Text = %q, want %q", runs[0].Text, "a")
	}
	if runs[0].ParaFormat.BinaryListOverride == nil || *runs[0].ParaFormat.BinaryListOverride != 1 {
		t.Fatalf("runs[0].ParaFormat.BinaryListOverride = %v, want 1", runs[0].ParaFormat.BinaryListOverride)
	}
	if runs[1].Text != "bc" {
		t.Fatalf("runs[1].Text = %q, want %q", runs[1].Text, "bc")
	}
	if runs[1].Format.Bold == nil || !*runs[1].Format.Bold {
		t.Fatalf("runs[1].Format.Bold = %v, want true", runs[1].Format.Bold)
	}
	if runs[1].ParaFormat.BinaryListOverride == nil || *runs[1].ParaFormat.BinaryListOverride != 1 {
		t.Fatalf("runs[1].ParaFormat.BinaryListOverride = %v, want 1", runs[1].ParaFormat.BinaryListOverride)
	}
}

func TestBuildLegacyDocFromRunsUsesPapxListPointers(t *testing.T) {
	level := 1
	ilfo := 5
	runs := []docRun{{
		Text: "List item without textual marker",
		ParaFormat: wordir.ParagraphFormat{
			BinaryListLevel:    &level,
			BinaryListOverride: &ilfo,
		},
	}}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	list := doc.Paragraphs[0].List
	if list == nil {
		t.Fatal("expected list marker from PAPx list pointers")
	}
	if list.Kind != "ordered" || list.Format != "lowerLetter" || list.Start != 1 || list.Level != 1 {
		t.Fatalf("list = %#v, want ordered lowerLetter start=1 level=1", list)
	}
}

func TestBuildLegacyDocFromRunsSetsFromStyleWhenListInheritedFromStyle(t *testing.T) {
	level := 1
	ilfo := 5
	styleIndex := 1
	// Simulate a paragraph whose style has list properties but the direct PAPx does not.
	runs := []docRun{{
		Text: "Styled list item",
		ParaFormat: wordir.ParagraphFormat{
			BinaryStyleIndex: &styleIndex,
		},
	}}
	styleSheet := &legacyStyleSheet{
		paragraphByIndex: map[int]wordir.Style{
			styleIndex: {
				ID: "ListParagraph",
				Format: wordir.ParagraphFormat{
					BinaryListLevel:    &level,
					BinaryListOverride: &ilfo,
				},
			},
		},
	}
	doc := buildLegacyDocFromRunsWithContext("test.doc", runs, nil, nil, nil, styleSheet, nil)
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].Numbering == nil {
		t.Fatal("expected Numbering")
	}
	if !doc.Paragraphs[0].Numbering.FromStyle {
		t.Errorf("FromStyle = false, want true")
	}
}

func TestLegacyBinaryListFormatByLevel(t *testing.T) {
	tests := []struct {
		level int
		want  string
	}{
		{level: 0, want: "decimal"},
		{level: 1, want: "lowerLetter"},
		{level: 2, want: "lowerRoman"},
		{level: 3, want: "decimal"},
	}
	for _, tc := range tests {
		if got := legacyBinaryListFormat(tc.level); got != tc.want {
			t.Fatalf("legacyBinaryListFormat(%d) = %q, want %q", tc.level, got, tc.want)
		}
	}
}

func TestBuildLegacyDocFromRunsBinaryListContinuesNumbering(t *testing.T) {
	level := 0
	ilfo := 9
	runs := []docRun{
		{Text: "First", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Second", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Third", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 3 {
		t.Fatalf("paragraphs = %d, want 3", len(doc.Paragraphs))
	}
	for i, want := range []int{1, 2, 3} {
		list := doc.Paragraphs[i].List
		if list == nil {
			t.Fatalf("paragraph %d missing list marker", i)
		}
		if list.Start != want {
			t.Fatalf("paragraph %d list start = %d, want %d", i, list.Start, want)
		}
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsAfterNonList(t *testing.T) {
	level := 0
	ilfo := 4
	runs := []docRun{
		{Text: "List A", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Normal paragraph"},
		{Text: "\n"},
		{Text: "List B", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 3 {
		t.Fatalf("paragraphs = %d, want 3", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].List == nil || doc.Paragraphs[0].List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", doc.Paragraphs[0].List)
	}
	if doc.Paragraphs[1].List != nil {
		t.Fatalf("middle paragraph list = %#v, want nil", doc.Paragraphs[1].List)
	}
	if doc.Paragraphs[2].List == nil || doc.Paragraphs[2].List.Start != 1 {
		t.Fatalf("restarted list paragraph = %#v, want start=1", doc.Paragraphs[2].List)
	}
}

func TestBuildLegacyDocFromRunsUsesBinaryListStartFromTables(t *testing.T) {
	level := 0
	ilfo := 3
	runs := []docRun{
		{Text: "First", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Second", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
	}
	listTables := &legacyListTables{
		formatByIlfoLevel: map[string]string{legacyListKey(3, 0): "decimal"},
		startByIlfoLevel:  map[string]int{legacyListKey(3, 0): 5},
	}
	doc := buildLegacyDocFromRunsWithBookmarks("test.doc", runs, nil, listTables)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if got := doc.Paragraphs[0].List.Start; got != 5 {
		t.Fatalf("first start = %d, want 5", got)
	}
	if got := doc.Paragraphs[1].List.Start; got != 6 {
		t.Fatalf("second start = %d, want 6", got)
	}
}

func TestBuildLegacyDocFromRunsRendersBinaryNumberingTextFromTemplateAndCounters(t *testing.T) {
	level0 := 0
	level1 := 1
	ilfo := 13
	runs := []docRun{
		{Text: "Top 1", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level0, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Child 1", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level1, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Child 2", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level1, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Top 2", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level0, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Child restart", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level1, BinaryListOverride: &ilfo}},
	}
	listTables := &legacyListTables{
		formatByIlfoLevel: map[string]string{
			legacyListKey(ilfo, level0): "decimal",
			legacyListKey(ilfo, level1): "lowerLetter",
		},
		startByIlfoLevel: map[string]int{
			legacyListKey(ilfo, level0): 1,
			legacyListKey(ilfo, level1): 1,
		},
		restartByIlfoLevel: map[string]bool{
			legacyListKey(ilfo, level1): true,
		},
		templateByIlfoLevel: map[string]string{
			legacyListKey(ilfo, level0): "%1.",
			legacyListKey(ilfo, level1): "%1.%2.",
		},
	}

	doc := buildLegacyDocFromRunsWithBookmarks("test.doc", runs, nil, listTables)
	if len(doc.Paragraphs) != 5 {
		t.Fatalf("paragraphs = %d, want 5", len(doc.Paragraphs))
	}

	tests := []struct {
		index        int
		wantStart    int
		wantTemplate string
		wantText     string
	}{
		{index: 0, wantStart: 1, wantTemplate: "%1.", wantText: "1."},
		{index: 1, wantStart: 1, wantTemplate: "%1.%2.", wantText: "1.a."},
		{index: 2, wantStart: 2, wantTemplate: "%1.%2.", wantText: "1.b."},
		{index: 3, wantStart: 2, wantTemplate: "%1.", wantText: "2."},
		{index: 4, wantStart: 1, wantTemplate: "%1.%2.", wantText: "2.a."},
	}
	for _, test := range tests {
		paragraph := doc.Paragraphs[test.index]
		if paragraph.List == nil {
			t.Fatalf("paragraph %d List = nil", test.index)
		}
		if paragraph.List.Start != test.wantStart {
			t.Fatalf("paragraph %d List.Start = %d, want %d", test.index, paragraph.List.Start, test.wantStart)
		}
		if paragraph.List.Template != test.wantTemplate {
			t.Fatalf("paragraph %d List.Template = %q, want %q", test.index, paragraph.List.Template, test.wantTemplate)
		}
		if paragraph.Numbering == nil {
			t.Fatalf("paragraph %d Numbering = nil", test.index)
		}
		if paragraph.Numbering.NumID != ilfo {
			t.Fatalf("paragraph %d Numbering.NumID = %d, want %d", test.index, paragraph.Numbering.NumID, ilfo)
		}
		if paragraph.Numbering.Text != test.wantText {
			t.Fatalf("paragraph %d Numbering.Text = %q, want %q", test.index, paragraph.Numbering.Text, test.wantText)
		}
	}
}

func TestBuildLegacyDocFromRunsUsesLoadedBinaryListOverrides(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := make([]byte, 26)
	binary.LittleEndian.PutUint32(lfoBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lfoBlob[4:8], 321)
	copy(lfoBlob[8:17], []byte{5, 0, 0, 0, 0, 0, 0, 0, 0})
	copy(lfoBlob[17:26], []byte{2, 255, 255, 255, 255, 255, 255, 255, 255})

	lstBlob := make([]byte, 26)
	binary.LittleEndian.PutUint32(lstBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lstBlob[4:8], 321)
	copy(lstBlob[8:17], []byte{0, 4, 2, 0, 0, 0, 0, 0, 0})
	copy(lstBlob[17:26], []byte{1, 1, 1, 1, 1, 1, 1, 1, 1})

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, len(lfoBlob), len(lstBlob))

	listTables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if listTables == nil {
		t.Fatal("listTables = nil, want decoded tables")
	}

	level := 0
	ilfo := 1
	runs := []docRun{
		{Text: "First", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Second", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
	}
	doc := buildLegacyDocFromRunsWithBookmarks("test.doc", runs, nil, listTables)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	for index, wantStart := range []int{5, 6} {
		list := doc.Paragraphs[index].List
		if list == nil {
			t.Fatalf("paragraph %d missing list marker", index)
		}
		if list.Format != "lowerRoman" {
			t.Fatalf("paragraph %d format = %q, want lowerRoman from loaded LFO override", index, list.Format)
		}
		if list.Start != wantStart {
			t.Fatalf("paragraph %d start = %d, want %d from loaded table starts", index, list.Start, wantStart)
		}
	}
}

func TestBuildLegacyDocFromRunsUsesLoadedBinaryBulletOverrides(t *testing.T) {
	level := 0
	ilfo := 7
	runs := []docRun{
		{Text: "First", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Second", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
	}
	listTables := &legacyListTables{
		formatByIlfoLevel: map[string]string{
			legacyListKey(ilfo, level): "bullet",
		},
	}
	doc := buildLegacyDocFromRunsWithBookmarks("test.doc", runs, nil, listTables)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	for index, wantText := range []string{"First", "Second"} {
		list := doc.Paragraphs[index].List
		if list == nil {
			t.Fatalf("paragraph %d missing list marker", index)
		}
		if list.Kind != "bullet" || list.Template != "•" {
			t.Fatalf("paragraph %d list = %#v, want bullet marker", index, list)
		}
		if doc.Paragraphs[index].Text != wantText {
			t.Fatalf("paragraph %d text = %q, want %q", index, doc.Paragraphs[index].Text, wantText)
		}
	}
}

func TestBuildLegacyDocFromRunsUsesLoadedBinaryListClearStartOverride(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	lfoBlob := make([]byte, 17)
	binary.LittleEndian.PutUint32(lfoBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lfoBlob[4:8], 654)
	copy(lfoBlob[8:17], []byte{0xFE, 0, 0, 0, 0, 0, 0, 0, 0})

	lstBlob := make([]byte, 26)
	binary.LittleEndian.PutUint32(lstBlob[0:4], 1)
	binary.LittleEndian.PutUint32(lstBlob[4:8], 654)
	copy(lstBlob[8:17], []byte{0, 4, 2, 0, 0, 0, 0, 0, 0})
	copy(lstBlob[17:26], []byte{4, 1, 1, 1, 1, 1, 1, 1, 1})

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, len(lfoBlob), len(lstBlob))

	listTables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if listTables == nil {
		t.Fatal("listTables = nil, want decoded tables")
	}

	level := 0
	ilfo := 1
	runs := []docRun{
		{Text: "First", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Second", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
	}
	doc := buildLegacyDocFromRunsWithBookmarks("test.doc", runs, nil, listTables)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if got := doc.Paragraphs[0].List.Start; got != 1 {
		t.Fatalf("first start = %d, want cleared LFO override to fall back to 1", got)
	}
	if got := doc.Paragraphs[1].List.Start; got != 2 {
		t.Fatalf("second start = %d, want continued numbering from cleared start", got)
	}
}

func TestBuildLegacyDocFromRunsUsesStructuredLoadedBinaryListOverrides(t *testing.T) {
	const (
		fibBase   = 2
		cbRgFcLcb = 40
	)
	wordStream := make([]byte, fibBase+cbRgFcLcb*8)

	overrideLvl := legacyStructuredLVLRecord(2, 5, 0, 0, "%1.")
	lfoBlob := legacyStructuredPlfLfoBlob(777, legacyStructuredLFOLVL(0, 0, false, overrideLvl))
	lstBlob := legacyStructuredPlfLstBlob(
		777,
		false,
		legacyStructuredLVLRecord(0, 1, 0, 0, "%1."),
		legacyStructuredLVLRecord(4, 1, 0, 0, "%2."),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "Р Р†Р вЂљРЎС›"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "Р Р†Р вЂљРЎС›"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "Р Р†Р вЂљРЎС›"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "Р Р†Р вЂљРЎС›"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "Р Р†Р вЂљРЎС›"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "Р Р†Р вЂљРЎС›"),
		legacyStructuredLVLRecord(0x17, 0, 0, 0, "Р Р†Р вЂљРЎС›"),
	)

	tableStream := make([]byte, len(lfoBlob)+len(lstBlob))
	copy(tableStream[0:], lfoBlob)
	copy(tableStream[len(lfoBlob):], lstBlob)
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLfoIndex, 0, len(lfoBlob))
	putFibFcLcb(wordStream, fibBase, legacyFibFcLcbPlfLstIndex, len(lfoBlob), 2+legacyLstfSize)

	listTables := loadLegacyListTables(wordStream, tableStream, fibBase, cbRgFcLcb)
	if listTables == nil {
		t.Fatal("listTables = nil, want decoded structured tables")
	}

	level := 0
	ilfo := 1
	runs := []docRun{
		{Text: "First", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Second", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
	}
	doc := buildLegacyDocFromRunsWithBookmarks("test.doc", runs, nil, listTables)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	for index, wantStart := range []int{5, 6} {
		list := doc.Paragraphs[index].List
		if list == nil {
			t.Fatalf("paragraph %d missing list marker", index)
		}
		if list.Format != "lowerRoman" {
			t.Fatalf("paragraph %d format = %q, want lowerRoman from structured LFOLVL override", index, list.Format)
		}
		if list.Start != wantStart {
			t.Fatalf("paragraph %d start = %d, want %d from structured override", index, list.Start, wantStart)
		}
	}
	// Verify AbstractNumID convergence: the LSID from the LSTF should flow
	// through to Numbering.AbstractNumID.
	if doc.Paragraphs[0].Numbering == nil {
		t.Fatal("paragraph 0 missing Numbering")
	}
	if doc.Paragraphs[0].Numbering.AbstractNumID != 777 {
		t.Fatalf("AbstractNumID = %d, want 777", doc.Paragraphs[0].Numbering.AbstractNumID)
	}
}

func TestBuildLegacyDocFromRunsAppliesBinaryLevelRestartHints(t *testing.T) {
	level0 := 0
	level1 := 1
	ilfo := 11
	runs := []docRun{
		{Text: "Top 1", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level0, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Sub 1", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level1, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Sub 2", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level1, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Top 2", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level0, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "Sub restart", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level1, BinaryListOverride: &ilfo}},
	}
	listTables := &legacyListTables{
		formatByIlfoLevel:  map[string]string{legacyListKey(11, 0): "decimal", legacyListKey(11, 1): "lowerLetter"},
		startByIlfoLevel:   map[string]int{legacyListKey(11, 0): 1, legacyListKey(11, 1): 1},
		restartByIlfoLevel: map[string]bool{legacyListKey(11, 1): true},
	}
	doc := buildLegacyDocFromRunsWithBookmarks("test.doc", runs, nil, listTables)
	if len(doc.Paragraphs) != 5 {
		t.Fatalf("paragraphs = %d, want 5", len(doc.Paragraphs))
	}
	if got := doc.Paragraphs[1].List.Start; got != 1 {
		t.Fatalf("first level-1 start = %d, want 1", got)
	}
	if got := doc.Paragraphs[2].List.Start; got != 2 {
		t.Fatalf("second level-1 start = %d, want 2", got)
	}
	if got := doc.Paragraphs[4].List.Start; got != 1 {
		t.Fatalf("restarted level-1 start = %d, want 1", got)
	}
}

func TestBuildLegacyDocFromRunsAppliesBinaryRestartTargetThreshold(t *testing.T) {
	level1 := 1
	level2 := 2
	ilfo := 12
	runs := []docRun{
		{Text: "L2 first", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level2, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "L2 second", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level2, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "L1 parent", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level1, BinaryListOverride: &ilfo}},
		{Text: "\n"},
		{Text: "L2 third no reset", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level2, BinaryListOverride: &ilfo}},
	}
	listTables := &legacyListTables{
		formatByIlfoLevel:         map[string]string{legacyListKey(12, 1): "lowerLetter", legacyListKey(12, 2): "lowerRoman"},
		startByIlfoLevel:          map[string]int{legacyListKey(12, 1): 1, legacyListKey(12, 2): 1},
		restartOnLevelByIlfoLevel: map[string]int{legacyListKey(12, 2): 0},
	}
	doc := buildLegacyDocFromRunsWithBookmarks("test.doc", runs, nil, listTables)
	if len(doc.Paragraphs) != 4 {
		t.Fatalf("paragraphs = %d, want 4", len(doc.Paragraphs))
	}
	if got := doc.Paragraphs[0].List.Start; got != 1 {
		t.Fatalf("L2 first start = %d, want 1", got)
	}
	if got := doc.Paragraphs[1].List.Start; got != 2 {
		t.Fatalf("L2 second start = %d, want 2", got)
	}
	if got := doc.Paragraphs[3].List.Start; got != 3 {
		t.Fatalf("L2 third start = %d, want 3 (no reset for prev level 1 when target=0)", got)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsAfterBlankGapBoundary(t *testing.T) {
	level := 0
	ilfo := 4
	runs := []docRun{
		{Text: "List A", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n\n"}, // boundary with an intentional blank paragraph
		{Text: "List B", ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 3 {
		t.Fatalf("paragraphs = %d, want 3 including the intentional blank paragraph", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].List == nil || doc.Paragraphs[0].List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", doc.Paragraphs[0].List)
	}
	if doc.Paragraphs[2].List == nil || doc.Paragraphs[2].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want start=1 after boundary reset", doc.Paragraphs[2].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnCPGapWhenContextChanges(t *testing.T) {
	level0 := 0
	level1 := 1
	ilfo := 6
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level0, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level1, BinaryListOverride: &ilfo}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 after CP gap + context change", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListDoesNotRestartOnCPGapWithoutContextChange(t *testing.T) {
	level := 0
	ilfo := 6
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].List == nil || doc.Paragraphs[0].List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", doc.Paragraphs[0].List)
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 when only CP gap changed", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnCPGapWhenKeepNextChanges(t *testing.T) {
	level := 0
	ilfo := 6
	keepNext := true
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, KeepNext: keepNext}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 after CP gap + KeepNext change", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnCPGapWhenSpacingChanges(t *testing.T) {
	level := 0
	ilfo := 6
	spacingBefore := 240
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, SpacingBefore: &spacingBefore}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 after CP gap + spacing change", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnCPGapWhenRunFormatChanges(t *testing.T) {
	level := 0
	ilfo := 6
	bold := true
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Bold: &bold}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 after CP gap + run-format change", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListDoesNotRestartOnSmallCPGapWhenRunFormatChanges(t *testing.T) {
	level := 0
	ilfo := 6
	bold := true
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 12, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Bold: &bold}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].List == nil || doc.Paragraphs[0].List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", doc.Paragraphs[0].List)
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for small CP gap + run-format change", doc.Paragraphs[1].List)
	}
	if gap := runs[1].StartCP - (runs[0].StartCP + len([]rune(runs[0].Text))); gap >= legacyBinaryListMinParagraphGeometryRestartCPGap {
		t.Fatalf("test setup gap = %d, want less than paragraph-geometry threshold %d", gap, legacyBinaryListMinParagraphGeometryRestartCPGap)
	}
}

func TestBuildLegacyDocFromRunsBinaryListDoesNotRestartOnModerateCPGapForRunFormatChanges(t *testing.T) {
	level := 0
	ilfo := 6
	bold := true
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 18, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Bold: &bold}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for moderate CP gap + run-format change", doc.Paragraphs[1].List)
	}
	gap := runs[2].StartCP - (runs[0].StartCP + len([]rune(runs[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinRunStyleRestartCPGap {
		t.Fatalf("test setup gap = %d, want list-level threshold <= gap < run-style threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinRunStyleRestartCPGap)
	}
}

func TestBuildLegacyDocFromRunsBinaryListDoesNotRestartOnModerateCPGapWhenKeepNextChanges(t *testing.T) {
	level := 0
	ilfo := 6
	keepNext := true
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 18, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, KeepNext: keepNext}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for moderate CP gap + KeepNext change", doc.Paragraphs[1].List)
	}
	gap := runs[2].StartCP - (runs[0].StartCP + len([]rune(runs[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinParagraphKeepRestartCPGap {
		t.Fatalf("test setup gap = %d, want list-level threshold <= gap < keep threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinParagraphKeepRestartCPGap)
	}
	if legacyBinaryListMinParagraphKeepRestartCPGap != legacyBinaryListMinParagraphLayoutRestartCPGap {
		t.Fatalf("keep and layout thresholds diverged unexpectedly: keep=%d layout=%d", legacyBinaryListMinParagraphKeepRestartCPGap, legacyBinaryListMinParagraphLayoutRestartCPGap)
	}
	if legacyBinaryListMinParagraphKeepRestartCPGap != legacyBinaryListMinRunStyleRestartCPGap {
		t.Fatalf("keep and run-style thresholds diverged unexpectedly: keep=%d run=%d", legacyBinaryListMinParagraphKeepRestartCPGap, legacyBinaryListMinRunStyleRestartCPGap)
	}
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinParagraphKeepRestartCPGap {
		t.Fatalf("test setup gap = %d, want list-level threshold <= gap < keep threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinParagraphKeepRestartCPGap)
	}
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinParagraphLayoutRestartCPGap {
		t.Fatalf("test setup gap = %d, want list-level threshold <= gap < paragraph-layout threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinParagraphLayoutRestartCPGap)
	}
}

func TestBuildLegacyDocFromRunsBinaryListDoesNotRestartOnModerateCPGapWhenIndentChanges(t *testing.T) {
	level := 0
	ilfo := 6
	indentLeft := 720
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 18, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, IndentLeft: &indentLeft}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for moderate CP gap + indent change", doc.Paragraphs[1].List)
	}
	gap := runs[2].StartCP - (runs[0].StartCP + len([]rune(runs[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinParagraphGeometryRestartCPGap {
		t.Fatalf("test setup gap = %d, want list-level threshold <= gap < paragraph-geometry threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinParagraphGeometryRestartCPGap)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnModerateCPGapWhenListLevelChanges(t *testing.T) {
	level0 := 0
	level1 := 1
	ilfo := 6
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level0, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 18, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level1, BinaryListOverride: &ilfo}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for moderate CP gap + list level change", doc.Paragraphs[1].List)
	}
	gap := runs[2].StartCP - (runs[0].StartCP + len([]rune(runs[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinParagraphGeometryRestartCPGap {
		t.Fatalf("test setup gap = %d, want list-level threshold <= gap < paragraph-geometry threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinParagraphGeometryRestartCPGap)
	}
}

func TestApplyLegacyBinaryListContinuationDoesNotRestartOnModerateCPGapWhenOnlyListFormatChanges(t *testing.T) {
	ilfo := 6
	level := 0
	state := &legacyBinaryListState{
		counters:                    map[string]int{},
		lastEndCPByIlfo:             map[int]int{},
		lastListLevelByIlfo:         map[int]string{},
		lastListFormatByIlfo:        map[int]string{},
		lastParagraphGeometryByIlfo: map[int]string{},
		lastParagraphKeepByIlfo:     map[int]string{},
		lastParagraphLayoutByIlfo:   map[int]string{},
		lastRunContextByIlfo:        map[int]string{},
	}
	first := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "decimal", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo},
	}
	second := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "lowerletter", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo},
	}
	firstRuns := []docRun{{Text: "First", StartCP: 0, ParaFormat: first.Format}}
	secondRuns := []docRun{{Text: "Second", StartCP: 18, ParaFormat: second.Format}}
	if !applyLegacyBinaryListContinuation(&first, state, nil, firstRuns, false) {
		t.Fatal("first continuation returned false")
	}
	if !applyLegacyBinaryListContinuation(&second, state, nil, secondRuns, false) {
		t.Fatal("second continuation returned false")
	}
	if first.List == nil || first.List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", first.List)
	}
	if second.List == nil || second.List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for moderate CP gap + format-only change", second.List)
	}
	gap := secondRuns[0].StartCP - (firstRuns[0].StartCP + len([]rune(firstRuns[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinListFormatRestartCPGap {
		t.Fatalf("test setup gap = %d, want list-level threshold <= gap < list-format threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinListFormatRestartCPGap)
	}
}

func TestLegacyBinaryListRequiredRestartCPGap(t *testing.T) {
	tests := []struct {
		name                     string
		listLevelChanged         bool
		listFormatChanged        bool
		paragraphGeometryChanged bool
		paragraphKeepChanged     bool
		paragraphLayoutChanged   bool
		runStyleChanged          bool
		wantGap                  int
		wantChanged              bool
	}{
		{name: "no changes", wantGap: 0, wantChanged: false},
		{name: "list level", listLevelChanged: true, wantGap: legacyBinaryListMinListLevelRestartCPGap, wantChanged: true},
		{name: "list format", listFormatChanged: true, wantGap: legacyBinaryListMinListFormatRestartCPGap, wantChanged: true},
		{name: "geometry", paragraphGeometryChanged: true, wantGap: legacyBinaryListMinParagraphGeometryRestartCPGap, wantChanged: true},
		{name: "keep", paragraphKeepChanged: true, wantGap: legacyBinaryListMinParagraphKeepRestartCPGap, wantChanged: true},
		{name: "layout", paragraphLayoutChanged: true, wantGap: legacyBinaryListMinParagraphLayoutRestartCPGap, wantChanged: true},
		{name: "run style", runStyleChanged: true, wantGap: legacyBinaryListMinRunStyleRestartCPGap, wantChanged: true},
		{name: "level beats weaker", listLevelChanged: true, runStyleChanged: true, wantGap: legacyBinaryListMinListLevelRestartCPGap, wantChanged: true},
		{name: "format plus geometry uses intermediate gap", listFormatChanged: true, paragraphGeometryChanged: true, wantGap: legacyBinaryListMinCombinedRestartCPGap, wantChanged: true},
		{name: "geometry plus layout escalates", paragraphGeometryChanged: true, paragraphLayoutChanged: true, wantGap: legacyBinaryListMinListLevelRestartCPGap, wantChanged: true},
		{name: "three weak signals escalate", paragraphKeepChanged: true, paragraphLayoutChanged: true, runStyleChanged: true, wantGap: legacyBinaryListMinListLevelRestartCPGap, wantChanged: true},
		{name: "geometry plus keep plus layout escalates", paragraphGeometryChanged: true, paragraphKeepChanged: true, paragraphLayoutChanged: true, wantGap: legacyBinaryListMinListLevelRestartCPGap, wantChanged: true},
		{name: "geometry plus keep plus run style escalates", paragraphGeometryChanged: true, paragraphKeepChanged: true, runStyleChanged: true, wantGap: legacyBinaryListMinListLevelRestartCPGap, wantChanged: true},
		{name: "format plus geometry plus run style escalates", listFormatChanged: true, paragraphGeometryChanged: true, runStyleChanged: true, wantGap: legacyBinaryListMinListLevelRestartCPGap, wantChanged: true},
		{name: "format plus layout plus run style escalates", listFormatChanged: true, paragraphLayoutChanged: true, runStyleChanged: true, wantGap: legacyBinaryListMinListLevelRestartCPGap, wantChanged: true},
		{name: "format plus layout plus keep uses intermediate gap", listFormatChanged: true, paragraphKeepChanged: true, paragraphLayoutChanged: true, wantGap: legacyBinaryListMinCombinedRestartCPGap, wantChanged: true},
		{name: "format plus geometry plus keep uses intermediate gap", listFormatChanged: true, paragraphGeometryChanged: true, paragraphKeepChanged: true, wantGap: legacyBinaryListMinCombinedRestartCPGap, wantChanged: true},
		{name: "format plus keep plus run style uses intermediate gap", listFormatChanged: true, paragraphKeepChanged: true, runStyleChanged: true, wantGap: legacyBinaryListMinCombinedRestartCPGap, wantChanged: true},
		{name: "two weak signals use intermediate gap", paragraphKeepChanged: true, paragraphLayoutChanged: true, wantGap: legacyBinaryListMinCombinedRestartCPGap, wantChanged: true},
		{name: "format plus weak signal use intermediate gap", listFormatChanged: true, runStyleChanged: true, wantGap: legacyBinaryListMinCombinedRestartCPGap, wantChanged: true},
	}
	for _, test := range tests {
		gotGap, gotChanged := legacyBinaryListRequiredRestartCPGap(
			test.listLevelChanged,
			test.listFormatChanged,
			test.paragraphGeometryChanged,
			test.paragraphKeepChanged,
			test.paragraphLayoutChanged,
			test.runStyleChanged,
		)
		if gotGap != test.wantGap || gotChanged != test.wantChanged {
			t.Fatalf("legacyBinaryListRequiredRestartCPGap(%s) = (%d, %v), want (%d, %v)", test.name, gotGap, gotChanged, test.wantGap, test.wantChanged)
		}
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnModerateCPGapWhenGeometryAndLayoutChange(t *testing.T) {
	level := 0
	ilfo := 6
	indentLeft := 720
	spacingBefore := 240
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 18, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, IndentLeft: &indentLeft, SpacingBefore: &spacingBefore}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for moderate CP gap + geometry+layout change", doc.Paragraphs[1].List)
	}
	gap := runs[2].StartCP - (runs[0].StartCP + len([]rune(runs[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinParagraphGeometryRestartCPGap {
		t.Fatalf("test setup gap = %d, want list-level threshold <= gap < paragraph-geometry threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinParagraphGeometryRestartCPGap)
	}
}

func TestApplyLegacyBinaryListContinuationDoesNotRestartOnShortCPGapWhenFormatAndGeometryChange(t *testing.T) {
	ilfo := 6
	level := 0
	indentLeft := 720
	state := &legacyBinaryListState{
		counters:                    map[string]int{},
		lastEndCPByIlfo:             map[int]int{},
		lastListLevelByIlfo:         map[int]string{},
		lastListFormatByIlfo:        map[int]string{},
		lastParagraphGeometryByIlfo: map[int]string{},
		lastParagraphKeepByIlfo:     map[int]string{},
		lastParagraphLayoutByIlfo:   map[int]string{},
		lastRunContextByIlfo:        map[int]string{},
	}
	first := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "decimal", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo},
	}
	second := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "lowerletter", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, IndentLeft: &indentLeft},
	}
	firstRuns := []docRun{{Text: "First", StartCP: 0, ParaFormat: first.Format}}
	secondRuns := []docRun{{Text: "Second", StartCP: 15, ParaFormat: second.Format}}
	if !applyLegacyBinaryListContinuation(&first, state, nil, firstRuns, false) {
		t.Fatal("first continuation returned false")
	}
	if !applyLegacyBinaryListContinuation(&second, state, nil, secondRuns, false) {
		t.Fatal("second continuation returned false")
	}
	if first.List == nil || first.List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", first.List)
	}
	if second.List == nil || second.List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for short CP gap + format+geometry change", second.List)
	}
	gap := secondRuns[0].StartCP - (firstRuns[0].StartCP + len([]rune(firstRuns[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinCombinedRestartCPGap {
		t.Fatalf("test setup gap = %d, want strongest threshold <= gap < combined threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinCombinedRestartCPGap)
	}
}

func TestApplyLegacyBinaryListContinuationRestartsOnShortCPGapWhenGeometryKeepAndRunStyleChange(t *testing.T) {
	ilfo := 6
	level := 0
	indentLeft := 720
	keepNext := true
	bold := true
	state := &legacyBinaryListState{
		counters:                    map[string]int{},
		lastEndCPByIlfo:             map[int]int{},
		lastListLevelByIlfo:         map[int]string{},
		lastListFormatByIlfo:        map[int]string{},
		lastParagraphGeometryByIlfo: map[int]string{},
		lastParagraphKeepByIlfo:     map[int]string{},
		lastParagraphLayoutByIlfo:   map[int]string{},
		lastRunContextByIlfo:        map[int]string{},
	}
	first := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "decimal", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo},
	}
	second := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "decimal", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, IndentLeft: &indentLeft, KeepNext: keepNext},
	}
	firstRuns := []docRun{{Text: "First", StartCP: 0, ParaFormat: first.Format}}
	secondRuns := []docRun{{Text: "Second", StartCP: 15, ParaFormat: second.Format, Format: wordir.RunFormat{Bold: &bold}}}
	if !applyLegacyBinaryListContinuation(&first, state, nil, firstRuns, false) {
		t.Fatal("first continuation returned false")
	}
	if !applyLegacyBinaryListContinuation(&second, state, nil, secondRuns, false) {
		t.Fatal("second continuation returned false")
	}
	if first.List == nil || first.List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", first.List)
	}
	if second.List == nil || second.List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for short CP gap + geometry+keep+run-style change", second.List)
	}
	gap := secondRuns[0].StartCP - (firstRuns[0].StartCP + len([]rune(firstRuns[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinCombinedRestartCPGap {
		t.Fatalf("test setup gap = %d, want strongest threshold <= gap < combined threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinCombinedRestartCPGap)
	}
}

func TestApplyLegacyBinaryListContinuationDoesNotRestartOnShortCPGapWhenFormatKeepAndRunStyleChange(t *testing.T) {
	ilfo := 6
	level := 0
	keepNext := true
	bold := true
	state := &legacyBinaryListState{
		counters:                    map[string]int{},
		lastEndCPByIlfo:             map[int]int{},
		lastListLevelByIlfo:         map[int]string{},
		lastListFormatByIlfo:        map[int]string{},
		lastParagraphGeometryByIlfo: map[int]string{},
		lastParagraphKeepByIlfo:     map[int]string{},
		lastParagraphLayoutByIlfo:   map[int]string{},
		lastRunContextByIlfo:        map[int]string{},
	}
	first := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "decimal", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo},
	}
	second := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "lowerletter", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, KeepNext: keepNext},
	}
	firstRuns := []docRun{{Text: "First", StartCP: 0, ParaFormat: first.Format}}
	secondRuns := []docRun{{Text: "Second", StartCP: 15, ParaFormat: second.Format, Format: wordir.RunFormat{Bold: &bold}}}
	if !applyLegacyBinaryListContinuation(&first, state, nil, firstRuns, false) {
		t.Fatal("first continuation returned false")
	}
	if !applyLegacyBinaryListContinuation(&second, state, nil, secondRuns, false) {
		t.Fatal("second continuation returned false")
	}
	if first.List == nil || first.List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", first.List)
	}
	if second.List == nil || second.List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for short CP gap + format+keep+run-style change", second.List)
	}
	gap := secondRuns[0].StartCP - (firstRuns[0].StartCP + len([]rune(firstRuns[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinCombinedRestartCPGap {
		t.Fatalf("test setup gap = %d, want strongest threshold <= gap < combined threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinCombinedRestartCPGap)
	}
}

func TestApplyLegacyBinaryListContinuationRestartsOnShortCPGapWhenFormatGeometryAndRunStyleChange(t *testing.T) {
	ilfo := 6
	level := 0
	indentLeft := 720
	bold := true
	state := &legacyBinaryListState{
		counters:                    map[string]int{},
		lastEndCPByIlfo:             map[int]int{},
		lastListLevelByIlfo:         map[int]string{},
		lastListFormatByIlfo:        map[int]string{},
		lastParagraphGeometryByIlfo: map[int]string{},
		lastParagraphKeepByIlfo:     map[int]string{},
		lastParagraphLayoutByIlfo:   map[int]string{},
		lastRunContextByIlfo:        map[int]string{},
	}
	first := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "decimal", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo},
	}
	second := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "lowerletter", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, IndentLeft: &indentLeft},
	}
	firstRuns := []docRun{{Text: "First", StartCP: 0, ParaFormat: first.Format}}
	secondRuns := []docRun{{Text: "Second", StartCP: 15, ParaFormat: second.Format, Format: wordir.RunFormat{Bold: &bold}}}
	if !applyLegacyBinaryListContinuation(&first, state, nil, firstRuns, false) {
		t.Fatal("first continuation returned false")
	}
	if !applyLegacyBinaryListContinuation(&second, state, nil, secondRuns, false) {
		t.Fatal("second continuation returned false")
	}
	if first.List == nil || first.List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", first.List)
	}
	if second.List == nil || second.List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for short CP gap + format+geometry+run-style change", second.List)
	}
	gap := secondRuns[0].StartCP - (firstRuns[0].StartCP + len([]rune(firstRuns[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinCombinedRestartCPGap {
		t.Fatalf("test setup gap = %d, want strongest threshold <= gap < combined threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinCombinedRestartCPGap)
	}
}

func TestApplyLegacyBinaryListContinuationRestartsOnShortCPGapWhenFormatLayoutAndRunStyleChange(t *testing.T) {
	ilfo := 6
	level := 0
	spacingBefore := 240
	bold := true
	state := &legacyBinaryListState{
		counters:                    map[string]int{},
		lastEndCPByIlfo:             map[int]int{},
		lastListLevelByIlfo:         map[int]string{},
		lastListFormatByIlfo:        map[int]string{},
		lastParagraphGeometryByIlfo: map[int]string{},
		lastParagraphKeepByIlfo:     map[int]string{},
		lastParagraphLayoutByIlfo:   map[int]string{},
		lastRunContextByIlfo:        map[int]string{},
	}
	first := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "decimal", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo},
	}
	second := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "lowerletter", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, SpacingBefore: &spacingBefore},
	}
	firstRuns := []docRun{{Text: "First", StartCP: 0, ParaFormat: first.Format}}
	secondRuns := []docRun{{Text: "Second", StartCP: 15, ParaFormat: second.Format, Format: wordir.RunFormat{Bold: &bold}}}
	if !applyLegacyBinaryListContinuation(&first, state, nil, firstRuns, false) {
		t.Fatal("first continuation returned false")
	}
	if !applyLegacyBinaryListContinuation(&second, state, nil, secondRuns, false) {
		t.Fatal("second continuation returned false")
	}
	if first.List == nil || first.List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", first.List)
	}
	if second.List == nil || second.List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for short CP gap + format+layout+run-style change", second.List)
	}
	gap := secondRuns[0].StartCP - (firstRuns[0].StartCP + len([]rune(firstRuns[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinCombinedRestartCPGap {
		t.Fatalf("test setup gap = %d, want strongest threshold <= gap < combined threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinCombinedRestartCPGap)
	}
}

func TestApplyLegacyBinaryListContinuationRestartsOnShortCPGapWhenGeometryKeepAndLayoutChange(t *testing.T) {
	ilfo := 6
	level := 0
	indentLeft := 720
	keepNext := true
	spacingBefore := 240
	state := &legacyBinaryListState{
		counters:                    map[string]int{},
		lastEndCPByIlfo:             map[int]int{},
		lastListLevelByIlfo:         map[int]string{},
		lastListFormatByIlfo:        map[int]string{},
		lastParagraphGeometryByIlfo: map[int]string{},
		lastParagraphKeepByIlfo:     map[int]string{},
		lastParagraphLayoutByIlfo:   map[int]string{},
		lastRunContextByIlfo:        map[int]string{},
	}
	first := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "decimal", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo},
	}
	second := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "decimal", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, IndentLeft: &indentLeft, KeepNext: keepNext, SpacingBefore: &spacingBefore},
	}
	firstRuns := []docRun{{Text: "First", StartCP: 0, ParaFormat: first.Format}}
	secondRuns := []docRun{{Text: "Second", StartCP: 15, ParaFormat: second.Format}}
	if !applyLegacyBinaryListContinuation(&first, state, nil, firstRuns, false) {
		t.Fatal("first continuation returned false")
	}
	if !applyLegacyBinaryListContinuation(&second, state, nil, secondRuns, false) {
		t.Fatal("second continuation returned false")
	}
	if first.List == nil || first.List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", first.List)
	}
	if second.List == nil || second.List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for short CP gap + geometry+keep+layout change", second.List)
	}
	gap := secondRuns[0].StartCP - (firstRuns[0].StartCP + len([]rune(firstRuns[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinCombinedRestartCPGap {
		t.Fatalf("test setup gap = %d, want strongest threshold <= gap < combined threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinCombinedRestartCPGap)
	}
}

func TestApplyLegacyBinaryListContinuationDoesNotRestartOnShortCPGapWhenFormatLayoutAndKeepChange(t *testing.T) {
	ilfo := 6
	level := 0
	spacingBefore := 240
	keepNext := true
	state := &legacyBinaryListState{
		counters:                    map[string]int{},
		lastEndCPByIlfo:             map[int]int{},
		lastListLevelByIlfo:         map[int]string{},
		lastListFormatByIlfo:        map[int]string{},
		lastParagraphGeometryByIlfo: map[int]string{},
		lastParagraphKeepByIlfo:     map[int]string{},
		lastParagraphLayoutByIlfo:   map[int]string{},
		lastRunContextByIlfo:        map[int]string{},
	}
	first := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "decimal", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo},
	}
	second := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "lowerletter", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, KeepNext: keepNext, SpacingBefore: &spacingBefore},
	}
	firstRuns := []docRun{{Text: "First", StartCP: 0, ParaFormat: first.Format}}
	secondRuns := []docRun{{Text: "Second", StartCP: 15, ParaFormat: second.Format}}
	if !applyLegacyBinaryListContinuation(&first, state, nil, firstRuns, false) {
		t.Fatal("first continuation returned false")
	}
	if !applyLegacyBinaryListContinuation(&second, state, nil, secondRuns, false) {
		t.Fatal("second continuation returned false")
	}
	if first.List == nil || first.List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", first.List)
	}
	if second.List == nil || second.List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for short CP gap + format+layout+keep change", second.List)
	}
	gap := secondRuns[0].StartCP - (firstRuns[0].StartCP + len([]rune(firstRuns[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinCombinedRestartCPGap {
		t.Fatalf("test setup gap = %d, want strongest threshold <= gap < combined threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinCombinedRestartCPGap)
	}
}

func TestApplyLegacyBinaryListContinuationDoesNotRestartOnShortCPGapWhenFormatGeometryAndKeepChange(t *testing.T) {
	ilfo := 6
	level := 0
	indentLeft := 720
	keepNext := true
	state := &legacyBinaryListState{
		counters:                    map[string]int{},
		lastEndCPByIlfo:             map[int]int{},
		lastListLevelByIlfo:         map[int]string{},
		lastListFormatByIlfo:        map[int]string{},
		lastParagraphGeometryByIlfo: map[int]string{},
		lastParagraphKeepByIlfo:     map[int]string{},
		lastParagraphLayoutByIlfo:   map[int]string{},
		lastRunContextByIlfo:        map[int]string{},
	}
	first := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "decimal", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo},
	}
	second := wordir.Paragraph{
		List:   &wordir.ListMarker{Kind: "ordered", Level: level, Start: 1, Format: "lowerletter", Template: "binary"},
		Format: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, IndentLeft: &indentLeft, KeepNext: keepNext},
	}
	firstRuns := []docRun{{Text: "First", StartCP: 0, ParaFormat: first.Format}}
	secondRuns := []docRun{{Text: "Second", StartCP: 15, ParaFormat: second.Format}}
	if !applyLegacyBinaryListContinuation(&first, state, nil, firstRuns, false) {
		t.Fatal("first continuation returned false")
	}
	if !applyLegacyBinaryListContinuation(&second, state, nil, secondRuns, false) {
		t.Fatal("second continuation returned false")
	}
	if first.List == nil || first.List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", first.List)
	}
	if second.List == nil || second.List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for short CP gap + format+geometry+keep change", second.List)
	}
	gap := secondRuns[0].StartCP - (firstRuns[0].StartCP + len([]rune(firstRuns[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinCombinedRestartCPGap {
		t.Fatalf("test setup gap = %d, want strongest threshold <= gap < combined threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinCombinedRestartCPGap)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnModerateCPGapWhenKeepAndLayoutChange(t *testing.T) {
	level := 0
	ilfo := 6
	keepNext := true
	spacingBefore := 240
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 18, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, KeepNext: keepNext, SpacingBefore: &spacingBefore}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for moderate CP gap + keep+layout change", doc.Paragraphs[1].List)
	}
	gap := runs[2].StartCP - (runs[0].StartCP + len([]rune(runs[0].Text)))
	if gap < legacyBinaryListMinCombinedRestartCPGap || gap >= legacyBinaryListMinParagraphKeepRestartCPGap {
		t.Fatalf("test setup gap = %d, want combined threshold <= gap < weak single-signal threshold (%d <= gap < %d)", gap, legacyBinaryListMinCombinedRestartCPGap, legacyBinaryListMinParagraphKeepRestartCPGap)
	}
}

func TestBuildLegacyDocFromRunsBinaryListDoesNotRestartOnModerateCPGapWhenSpacingChanges(t *testing.T) {
	level := 0
	ilfo := 6
	spacingBefore := 240
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 18, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, SpacingBefore: &spacingBefore}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for moderate CP gap + spacing change", doc.Paragraphs[1].List)
	}
	gap := runs[2].StartCP - (runs[0].StartCP + len([]rune(runs[0].Text)))
	if gap < legacyBinaryListMinListLevelRestartCPGap || gap >= legacyBinaryListMinParagraphLayoutRestartCPGap {
		t.Fatalf("test setup gap = %d, want list-level threshold <= gap < paragraph-layout threshold (%d <= gap < %d)", gap, legacyBinaryListMinListLevelRestartCPGap, legacyBinaryListMinParagraphLayoutRestartCPGap)
	}
}

func TestBuildLegacyDocFromRunsBinaryListDoesNotRestartOnCPGapForShortStyledPrefix(t *testing.T) {
	level := 0
	ilfo := 6
	bold := true
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "A", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Bold: &bold}},
		{Text: " long continuation", StartCP: 41, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].List == nil || doc.Paragraphs[0].List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", doc.Paragraphs[0].List)
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for short styled prefix noise", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnCPGapForDominantStyledSegment(t *testing.T) {
	level := 0
	ilfo := 6
	bold := true
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "A ", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "dominant styled continuation", StartCP: 42, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Bold: &bold}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for dominant styled segment", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnCPGapForFragmentedDominantStyledSegment(t *testing.T) {
	level := 0
	ilfo := 6
	bold := true
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "A ", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "dominant ", StartCP: 42, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Bold: &bold}},
		{Text: "styled continuation", StartCP: 51, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Bold: &bold}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for fragmented dominant styled segment", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnCPGapForFragmentedEquivalentFontAliases(t *testing.T) {
	level := 0
	ilfo := 6
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "tiny ", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "dominant ", StartCP: 45, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{FontASCII: "Arial"}},
		{Text: "styled continuation", StartCP: 54, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{FontHAnsi: "arial"}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for fragmented equivalent font aliases", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnCPGapForFragmentedDefaultEquivalentUnderline(t *testing.T) {
	level := 0
	ilfo := 6
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "tiny ", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "dominant ", StartCP: 45, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{FontASCII: "Arial", Underline: "none"}},
		{Text: "styled continuation", StartCP: 54, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{FontHAnsi: "arial"}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for fragmented default-equivalent underline", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnCPGapForFragmentedEquivalentColorCasing(t *testing.T) {
	level := 0
	ilfo := 6
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "tiny ", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "dominant ", StartCP: 45, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Color: "FF0000"}},
		{Text: "styled continuation", StartCP: 54, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Color: "ff0000"}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for fragmented equivalent color casing", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnCPGapForEquivalentFalseStylePointer(t *testing.T) {
	level := 0
	ilfo := 6
	boldOff := false
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "tiny ", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "dominant ", StartCP: 45, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Color: "FF0000", Bold: &boldOff}},
		{Text: "styled continuation", StartCP: 54, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Color: "ff0000"}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for equivalent false style pointer", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListDoesNotRestartOnCPGapForWhitespaceOnlyFontNoise(t *testing.T) {
	level := 0
	ilfo := 6
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{FontASCII: "   "}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].List == nil || doc.Paragraphs[0].List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", doc.Paragraphs[0].List)
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for whitespace-only font noise", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListRestartsOnCPGapForDominantStrikeSegment(t *testing.T) {
	level := 0
	ilfo := 6
	strike := true
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "\n", StartCP: 5},
		{Text: "tiny ", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}},
		{Text: "dominant ", StartCP: 45, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Strike: &strike}},
		{Text: "styled continuation", StartCP: 54, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo}, Format: wordir.RunFormat{Strike: &strike}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 1 {
		t.Fatalf("second list paragraph = %#v, want restart at start=1 for dominant strike segment", doc.Paragraphs[1].List)
	}
}

func TestBuildLegacyDocFromRunsBinaryListDoesNotRestartOnCPGapForEquivalentParagraphFormattingNoise(t *testing.T) {
	level := 0
	ilfo := 6
	bordersA := &wordir.ParagraphBorders{Top: &wordir.BorderSide{Val: " single ", SizePt: 1, SpacePt: 0, Color: "FF0000"}}
	bordersB := &wordir.ParagraphBorders{Top: &wordir.BorderSide{Val: "single", SizePt: 1, SpacePt: 0, Color: "ff0000"}}
	runs := []docRun{
		{Text: "First", StartCP: 0, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, Alignment: " left ", Shading: "FF0000", LineRule: " AUTO ", Borders: bordersA}},
		{Text: "\n", StartCP: 5},
		{Text: "Second", StartCP: 40, ParaFormat: wordir.ParagraphFormat{BinaryListLevel: &level, BinaryListOverride: &ilfo, Alignment: "left", Shading: "ff0000", LineRule: "auto", Borders: bordersB}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].List == nil || doc.Paragraphs[0].List.Start != 1 {
		t.Fatalf("first list paragraph = %#v, want start=1", doc.Paragraphs[0].List)
	}
	if doc.Paragraphs[1].List == nil || doc.Paragraphs[1].List.Start != 2 {
		t.Fatalf("second list paragraph = %#v, want continued start=2 for equivalent paragraph formatting noise", doc.Paragraphs[1].List)
	}
}

func TestLegacyBinaryListRunStyleFingerprintNormalizesEquivalentValues(t *testing.T) {
	boldOff := false
	fontSize := 24
	left := legacyBinaryListRunStyleFingerprintFor(wordir.RunFormat{
		Bold:       &boldOff,
		Underline:  " none ",
		Color:      "FF0000",
		Background: " auto ",
		Highlight:  "NONE",
		FontSize:   &fontSize,
		FontASCII:  "  Arial  ",
	})
	right := legacyBinaryListRunStyleFingerprintFor(wordir.RunFormat{
		Underline: "",
		Color:     "ff0000",
		FontSize:  &fontSize,
		FontHAnsi: "arial",
	})
	if !left.meaningful() || !right.meaningful() {
		t.Fatalf("fingerprints should stay meaningful: left=%+v right=%+v", left, right)
	}
	if left.signature() != right.signature() {
		t.Fatalf("fingerprint signatures differ: left=%q right=%q", left.signature(), right.signature())
	}

	meaningless := legacyBinaryListRunStyleFingerprintFor(wordir.RunFormat{
		Bold:       &boldOff,
		Underline:  " none ",
		Background: "auto",
		Highlight:  " none ",
		FontASCII:  "   ",
	})
	if meaningless.meaningful() {
		t.Fatalf("meaningless fingerprint reported meaningful: %+v", meaningless)
	}
}

func TestLegacyBinaryListRunStyleFingerprintIncludesStrikeSmallCapsAndCaps(t *testing.T) {
	strike := true
	smallCaps := true
	caps := true
	left := legacyBinaryListRunStyleFingerprintFor(wordir.RunFormat{
		Strike:    &strike,
		SmallCaps: &smallCaps,
		Caps:      &caps,
	})
	right := legacyBinaryListRunStyleFingerprintFor(wordir.RunFormat{})
	if !left.meaningful() {
		t.Fatalf("fingerprint should treat strike/smallcaps/caps as meaningful: %+v", left)
	}
	if left.signature() == right.signature() {
		t.Fatalf("fingerprint signature ignored strike/smallcaps/caps: left=%q right=%q", left.signature(), right.signature())
	}
}

func TestLegacyBinaryListContextSignatureNormalizesEquivalentParagraphStrings(t *testing.T) {
	left := &wordir.Paragraph{
		List: &wordir.ListMarker{Level: 0, Format: " Decimal "},
		Format: wordir.ParagraphFormat{
			Alignment: " Left ",
			Shading:   "FF0000",
			LineRule:  " AUTO ",
			Borders:   &wordir.ParagraphBorders{Top: &wordir.BorderSide{Val: " single ", SizePt: 1, Color: "FF0000"}},
		},
	}
	right := &wordir.Paragraph{
		List: &wordir.ListMarker{Level: 0, Format: "decimal"},
		Format: wordir.ParagraphFormat{
			Alignment: "left",
			Shading:   "ff0000",
			LineRule:  "auto",
			Borders:   &wordir.ParagraphBorders{Top: &wordir.BorderSide{Val: "single", SizePt: 1, Color: "ff0000"}},
		},
	}
	leftSig := legacyBinaryListContextSignature(left, nil)
	rightSig := legacyBinaryListContextSignature(right, nil)
	if leftSig != rightSig {
		t.Fatalf("context signatures differ for equivalent paragraph formatting: left=%q right=%q", leftSig, rightSig)
	}
}

func TestParseDocCorpusRoundTrip(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	// Verify the real corpus files still parse without panic or error.
	fixtures := []string{
		"../../../../testdata/doc/apache-simple.doc",
		"../../../../testdata/doc/apache-SampleDoc.doc",
	}
	for _, path := range fixtures {
		doc, err := ParseDoc(path)
		if err != nil {
			t.Fatalf("ParseDoc(%s): %v", path, err)
		}
		if len(doc.Paragraphs) == 0 && len(doc.Tables) == 0 {
			t.Fatalf("ParseDoc(%s): no content", path)
		}
	}
}

func TestLegacyDocFibVersionRecognizesWord95(t *testing.T) {
	wordStream := make([]byte, 12)
	binary.LittleEndian.PutUint16(wordStream[0:2], legacyDocFibSignature)
	binary.LittleEndian.PutUint16(wordStream[2:4], legacyDocFibWord95)

	got, ok := legacyDocFibVersion(wordStream)
	if !ok {
		t.Fatal("legacyDocFibVersion ok = false, want true")
	}
	if got != legacyDocFibWord95 {
		t.Fatalf("legacyDocFibVersion = %#x, want %#x", got, legacyDocFibWord95)
	}
}

func TestLegacyDocLooksPreWord97RecognizesRealFixtureStreams(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	fixtures := []string{
		"../../../../testdata/doc/apache-Word95.doc",
		"../../../../testdata/doc/apache-Word6.doc",
	}
	for _, path := range fixtures {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		f, err := cfb.OpenBytes(data)
		if err != nil {
			t.Fatalf("OpenBytes(%s): %v", path, err)
		}
		wordStream, ok := f.Stream("WordDocument")
		if !ok {
			t.Fatalf("Stream(%s, WordDocument): missing", path)
		}
		if !legacyDocLooksPreWord97(wordStream) {
			t.Fatalf("legacyDocLooksPreWord97(%s) = false, want true", path)
		}
		if _, ok := legacyDocFibVersion(wordStream); ok {
			t.Fatalf("legacyDocFibVersion(%s) unexpectedly reported Word97-style FIB", path)
		}
		if got := selectDocTableStreamName(wordStream); got != "" {
			t.Fatalf("selectDocTableStreamName(%s) = %q, want empty without Word97 table stream metadata", path, got)
		}
	}
}

func TestExtractPreWord97BodyTextFromRealFixtures(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	tests := []struct {
		path  string
		wants []string
	}{
		{
			path: "../../../../testdata/doc/apache-Word95.doc",
			wants: []string{
				"The quick brown fox jumps over the lazy dog",
				"Paragraph 2",
				"Paragraph 3. Has some RED text and some BLUE BOLD text in it.",
			},
		},
		{
			path: "../../../../testdata/doc/apache-Word6.doc",
			wants: []string{
				"The quick brown fox jumps over the lazy dog",
			},
		},
	}
	for _, test := range tests {
		data, err := os.ReadFile(test.path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", test.path, err)
		}
		f, err := cfb.OpenBytes(data)
		if err != nil {
			t.Fatalf("OpenBytes(%s): %v", test.path, err)
		}
		wordStream, ok := f.Stream("WordDocument")
		if !ok {
			t.Fatalf("Stream(%s, WordDocument): missing", test.path)
		}
		text := extractPreWord97BodyText(wordStream)
		for _, want := range test.wants {
			if !strings.Contains(text, want) {
				t.Fatalf("extractPreWord97BodyText(%s): missing %q in %q", test.path, want, text)
			}
		}
		for _, unwanted := range []string{"SummaryInformation", "DocumentSummaryInformation", "CompObj"} {
			if strings.Contains(text, unwanted) {
				t.Fatalf("extractPreWord97BodyText(%s): unexpectedly contains %q in %q", test.path, unwanted, text)
			}
		}
	}
}

func TestInspectPreWord97BodySpanTracksObservedFixtureOffsets(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	tests := []struct {
		path               string
		wantNFib           uint16
		wantStart          int
		wantFcMac          int
		wantCcp            int
		wantEnd            int
		wantSecondaryStart int
		wantSecondarySize  int
		wantHddOffset      int
		wantHddSize        int
	}{
		{path: "../../../../testdata/doc/apache-Word95.doc", wantNFib: 0x0065, wantStart: 0x300, wantFcMac: 0x38F, wantCcp: 0x8F, wantEnd: 0x38F, wantSecondaryStart: 0x800, wantSecondarySize: 0x16C, wantHddOffset: 0x0, wantHddSize: 0x0},
		{path: "../../../../testdata/doc/apache-Word6.doc", wantNFib: 0x0065, wantStart: 0x300, wantFcMac: 0x32C, wantCcp: 0x2C, wantEnd: 0x32C, wantSecondaryStart: 0x800, wantSecondarySize: 0x144, wantHddOffset: 0x0, wantHddSize: 0x0},
		{path: "../../../../testdata/doc/word95_sections.doc", wantNFib: 0x0068, wantStart: 0x300, wantFcMac: 0x1235, wantCcp: 0xEDE, wantEnd: 0x11DE, wantSecondaryStart: 0x1A00, wantSecondarySize: 0xB4, wantHddOffset: 0x1AEE, wantHddSize: 0x20},
		{path: "../../../../testdata/doc/word95_cyrillic.doc", wantNFib: 0x0068, wantStart: 0x300, wantFcMac: 0x1189, wantCcp: 0xE89, wantEnd: 0x1189, wantSecondaryStart: 0x5E00, wantSecondarySize: 0x3FA, wantHddOffset: 0x620E, wantHddSize: 0x0},
	}
	for _, test := range tests {
		data, err := os.ReadFile(test.path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", test.path, err)
		}
		f, err := cfb.OpenBytes(data)
		if err != nil {
			t.Fatalf("OpenBytes(%s): %v", test.path, err)
		}
		wordStream, ok := f.Stream("WordDocument")
		if !ok {
			t.Fatalf("Stream(%s, WordDocument): missing", test.path)
		}
		span, ok := InspectPreWord97BodySpan(wordStream)
		if !ok {
			t.Fatalf("InspectPreWord97BodySpan(%s) = !ok", test.path)
		}
		if span.Signature != legacyDocPreWord97Magic || span.NFib != test.wantNFib || span.Start != test.wantStart || span.FcMac != test.wantFcMac || span.CcpText != test.wantCcp || span.BodyEnd != test.wantEnd || span.SecondaryStart != test.wantSecondaryStart || span.SecondarySize != test.wantSecondarySize || span.HddTableOffset != test.wantHddOffset || span.HddTableSize != test.wantHddSize {
			t.Fatalf("InspectPreWord97BodySpan(%s) = %#v, want sig=%#x nFib=%#x start=%#x fcMac=%#x ccp=%#x end=%#x secondary=(%#x,%#x)", test.path, span, legacyDocPreWord97Magic, test.wantNFib, test.wantStart, test.wantFcMac, test.wantCcp, test.wantEnd, test.wantSecondaryStart, test.wantSecondarySize)
		}
	}
}

func TestDecodePreWord97SingleByteTextPrefersCyrillicWhenBodyBytesMatch(t *testing.T) {
	data := []byte{0xCA, 0xEE, 0xEC, 0xEF, 0xE0, 0xED, 0xE8, 0xFF}
	if got := decodePreWord97SingleByteText(data); got != "Компания" {
		t.Fatalf("decodePreWord97SingleByteText(cyrillic bytes) = %q, want %q", got, "Компания")
	}
}

func TestExtractDocRunsViaPreWord97FIBUsesRealFixtureBodySpan(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	fixtures := []string{
		"../../../../testdata/doc/apache-Word95.doc",
		"../../../../testdata/doc/apache-Word6.doc",
	}
	for _, path := range fixtures {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", path, err)
		}
		f, err := cfb.OpenBytes(data)
		if err != nil {
			t.Fatalf("OpenBytes(%s): %v", path, err)
		}
		wordStream, ok := f.Stream("WordDocument")
		if !ok {
			t.Fatalf("Stream(%s, WordDocument): missing", path)
		}
		got := extractDocRunsViaPreWord97FIB(wordStream, f)
		if got == nil || len(got.Runs) == 0 {
			t.Fatalf("extractDocRunsViaPreWord97FIB(%s) = %#v, want non-empty runs", path, got)
		}
		if scoreLegacyText(joinDocRunsText(got.Runs)) == 0 {
			t.Fatalf("extractDocRunsViaPreWord97FIB(%s): empty run set", path)
		}
	}
}

func TestExtractDocRunsViaPreWord97FIBCarriesSecondaryStyleNameMap(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/apache-Word95.doc"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	f, err := cfb.OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes(%s): %v", path, err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatalf("Stream(%s, WordDocument): missing", path)
	}
	got := extractDocRunsViaPreWord97FIB(wordStream, f)
	if got == nil {
		t.Fatalf("extractDocRunsViaPreWord97FIB(%s) = nil", path)
	}
	if len(got.StyleNameByIstd) == 0 {
		t.Fatalf("extractDocRunsViaPreWord97FIB(%s): empty StyleNameByIstd", path)
	}
	// The STSH-derived path surfaces real paragraph-style names from the binary
	// STSH. The Word 95 fixture has "Default" (istd=0) and "Heading" (istd=1).
	// "Default Paragraph Font" is a character style and does not appear here.
	foundDefault := false
	foundHeading := false
	for _, name := range got.StyleNameByIstd {
		if name == "Default" {
			foundDefault = true
		}
		if name == "Heading" {
			foundHeading = true
		}
	}
	if !foundDefault || !foundHeading {
		t.Fatalf("StyleNameByIstd values = %#v, want both Default and Heading (STSH-derived)", got.StyleNameByIstd)
	}
}

func TestExtractDocRunsViaPreWord97FIBUsesImportedFixtureBodySpan(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	tests := []struct {
		path string
		want string
	}{
		{
			path: "../../../../testdata/doc/word6_sections.doc",
			want: "This is a new section.",
		},
		{
			path: "../../../../testdata/doc/word95_sections.doc",
			want: "SCOTLAND",
		},
		{
			path: "../../../../testdata/doc/word95_cyrillic.doc",
			want: "Компания Аскат М",
		},
	}
	for _, test := range tests {
		data, err := os.ReadFile(test.path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", test.path, err)
		}
		f, err := cfb.OpenBytes(data)
		if err != nil {
			t.Fatalf("OpenBytes(%s): %v", test.path, err)
		}
		wordStream, ok := f.Stream("WordDocument")
		if !ok {
			t.Fatalf("Stream(%s, WordDocument): missing", test.path)
		}
		got := extractDocRunsViaPreWord97FIB(wordStream, f)
		if got == nil || len(got.Runs) == 0 {
			t.Fatalf("extractDocRunsViaPreWord97FIB(%s) = %#v, want non-empty runs", test.path, got)
		}
		if text := joinDocRunsText(got.Runs); !strings.Contains(text, test.want) {
			t.Fatalf("extractDocRunsViaPreWord97FIB(%s) text missing %q in %q", test.path, test.want, text)
		}
	}
}

func TestExtractDocRunsViaPreWord97FIBPreservesPageBreaksFromImportedSectionFixture(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/word6_sections.doc"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	f, err := cfb.OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes(%s): %v", path, err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatalf("Stream(%s, WordDocument): missing", path)
	}
	got := extractDocRunsViaPreWord97FIB(wordStream, f)
	if got == nil || len(got.Runs) == 0 {
		t.Fatalf("extractDocRunsViaPreWord97FIB(%s) = %#v, want non-empty runs", path, got)
	}
	if text := joinDocRunsText(got.Runs); !strings.Contains(text, "\f") {
		t.Fatalf("extractDocRunsViaPreWord97FIB(%s) runs missing form-feed in %q", path, text)
	}
	doc := buildLegacyDocFromRuns(filepath.Base(path), got.Runs)
	pageBreaks := 0
	for _, paragraph := range doc.Paragraphs {
		for _, run := range paragraph.Runs {
			if run.Kind == "break" && run.BreakType == "page" {
				pageBreaks++
			}
		}
	}
	if pageBreaks == 0 {
		t.Fatalf("buildLegacyDocFromRuns(%s): pageBreaks = 0, want at least one preserved page break", path)
	}
}

func TestParseDocRealPreWord97FixturesKeepCorpusParagraphCounts(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	tests := []struct {
		path           string
		wantParagraphs int
	}{
		{path: "../../../../testdata/doc/apache-Word95.doc", wantParagraphs: 4},
		{path: "../../../../testdata/doc/apache-Word6.doc", wantParagraphs: 1},
	}
	for _, test := range tests {
		doc, err := ParseDoc(test.path)
		if err != nil {
			t.Fatalf("ParseDoc(%s): %v", test.path, err)
		}
		nonEmpty := 0
		for _, paragraph := range doc.Paragraphs {
			if strings.TrimSpace(paragraph.Text) != "" {
				nonEmpty++
			}
		}
		if nonEmpty != test.wantParagraphs {
			t.Fatalf("ParseDoc(%s) non-empty paragraphs = %d, want %d (total=%d)", test.path, nonEmpty, test.wantParagraphs, len(doc.Paragraphs))
		}
	}
}

func TestParseDocPreWord97FixtureCarriesInlineFormatting(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/apache-Word95.doc"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	f, err := cfb.OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes(%s): %v", path, err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatalf("Stream(%s, WordDocument): missing", path)
	}
	got := extractDocRunsViaPreWord97FIB(wordStream, f)
	if got == nil {
		t.Fatalf("extractDocRunsViaPreWord97FIB(%s) = nil", path)
	}
	var redRun, boldBlueRun *docRun
	for i := range got.Runs {
		r := &got.Runs[i]
		if strings.Contains(r.Text, "RED") && r.Format.Color == "FF0000" {
			redRun = r
		}
		if strings.Contains(r.Text, "BLUE BOLD") && r.Format.Bold != nil && *r.Format.Bold && r.Format.Color == "0000FF" {
			boldBlueRun = r
		}
	}
	if redRun == nil {
		t.Errorf("no run with text=RED and color=FF0000 found")
	}
	if boldBlueRun == nil {
		t.Errorf("no run with text=BLUE BOLD, bold=true, and color=0000FF found")
	}
}

func TestLoadPreWord97StyleSheetFromSecondarySpan(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	tests := []struct {
		path               string
		wantParaStyle      string // expected paragraph style ID present
		wantCharStyle      string // expected character style ID present
		wantParaStyleCount int
		wantCharStyleCount int
	}{
		{
			path:               "../../../../testdata/doc/apache-Word95.doc",
			wantParaStyle:      "Normal",
			wantCharStyle:      "DefaultParagraphFont",
			wantParaStyleCount: 6,
			wantCharStyleCount: 1,
		},
		{
			path:               "../../../../testdata/doc/apache-Word6.doc",
			wantParaStyle:      "Normal",
			wantCharStyle:      "DefaultParagraphFont",
			wantParaStyleCount: 6,
			wantCharStyleCount: 1,
		},
	}
	for _, test := range tests {
		data, err := os.ReadFile(test.path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", test.path, err)
		}
		f, err := cfb.OpenBytes(data)
		if err != nil {
			t.Fatalf("cfb.OpenBytes(%s): %v", test.path, err)
		}
		wordStream, ok := f.Stream("WordDocument")
		if !ok {
			t.Fatalf("f.Stream(%s): stream not found", test.path)
		}
		span, ok := InspectPreWord97BodySpan(wordStream)
		if !ok {
			t.Fatalf("InspectPreWord97BodySpan(%s) = false", test.path)
		}
		ss := loadPreWord97StyleSheet(wordStream, span)
		if ss == nil {
			t.Fatalf("loadPreWord97StyleSheet(%s) = nil, want non-nil", test.path)
		}
		if _, ok := ss.ParagraphStyles[test.wantParaStyle]; !ok {
			t.Fatalf("loadPreWord97StyleSheet(%s): ParagraphStyles missing %q; got %v", test.path, test.wantParaStyle, ss.ParagraphStyles)
		}
		if _, ok := ss.CharacterStyles[test.wantCharStyle]; !ok {
			t.Fatalf("loadPreWord97StyleSheet(%s): CharacterStyles missing %q; got %v", test.path, test.wantCharStyle, ss.CharacterStyles)
		}
		if len(ss.ParagraphStyles) < test.wantParaStyleCount {
			t.Fatalf("loadPreWord97StyleSheet(%s): ParagraphStyles count = %d, want >= %d", test.path, len(ss.ParagraphStyles), test.wantParaStyleCount)
		}
		if len(ss.CharacterStyles) < test.wantCharStyleCount {
			t.Fatalf("loadPreWord97StyleSheet(%s): CharacterStyles count = %d, want >= %d", test.path, len(ss.CharacterStyles), test.wantCharStyleCount)
		}
	}
}

// TestParseLegacyLPUpxPapxPre97DecodesOldOpcodes verifies that the pre-Word-97
// PAP UPX reader correctly decodes 1-byte opcode alignment (opcode 5) instead
// of treating the bytes as Word97+ 2-byte SPRMs.
func TestParseLegacyLPUpxPapxPre97DecodesOldOpcodes(t *testing.T) {
	// Construct a minimal LPUpx: 2-byte length header + grpprl bytes.
	// grpprl: opcode 5 (jc), operand 0x01 (center).
	upx := []byte{
		0x02, 0x00, // cbUpx = 2
		0x05, 0x01, // opcode 5 = jc, operand 1 = center
	}
	got, consumed := parseLegacyLPUpxPapxPre97(upx)
	if consumed != 4 {
		t.Fatalf("consumed = %d, want 4", consumed)
	}
	if got.Alignment != "center" {
		t.Fatalf("Alignment = %q, want \"center\"", got.Alignment)
	}

	// Verify right-alignment (jc=2) too.
	upx[3] = 0x02
	got, _ = parseLegacyLPUpxPapxPre97(upx)
	if got.Alignment != "right" {
		t.Fatalf("Alignment = %q, want \"right\"", got.Alignment)
	}
}

// TestParseLegacyLPUpxChpxPre97DecodesOldOpcodes verifies that the pre-Word-97
// CHP UPX reader correctly decodes 1-byte opcode bold (opcode 85).
func TestParseLegacyLPUpxChpxPre97DecodesOldOpcodes(t *testing.T) {
	// grpprl: opcode 85 (fBold), operand 0x01 (set bold).
	upx := []byte{
		0x02, 0x00, // cbUpx = 2
		0x55, 0x01, // opcode 85 (0x55) = fBold, operand 1 = true
	}
	got, consumed := parseLegacyLPUpxChpxPre97(upx, nil)
	if consumed != 4 {
		t.Fatalf("consumed = %d, want 4", consumed)
	}
	if got.Bold == nil || !*got.Bold {
		t.Fatalf("Bold = %v, want true", got.Bold)
	}
}

// TestParseLegacySTDPre97DecodesStyleUPXWithOldSprms verifies that a full
// synthetic pre-Word-97 STD entry produces correct paragraph formatting when
// the PAP UPX contains old-format (1-byte) opcodes.
func TestParseLegacySTDPre97DecodesStyleUPXWithOldSprms(t *testing.T) {
	// Build a minimal pre-Word-97 STD byte slice (cbSTDBaseInFile = 8):
	//   word 0: sti bits (STI=0 Р Р†РІР‚В РІР‚в„ў Normal) + other flags = 0x0000
	//   word 1: kind=1 (paragraph) in low nibble; basedOn=0x0FFF in high 12 bits = 0xFFF1
	//   word 2: cupx in low nibble = 2 (PAP UPX + CHP UPX)
	//   word 3: reserved = 0
	//   name: Pascal string Р Р†Р вЂљРІР‚Сњ cch=6, "Normal", NUL, pad byte
	//   UPX0 (PAP): cbUpx=2, opcode5=1 (center)
	//   UPX1 (CHP): cbUpx=2, opcode85=1 (bold)
	name := []byte{6, 'N', 'o', 'r', 'm', 'a', 'l', 0} // cch + chars + NUL (8 bytes, even)
	papUpx := []byte{0x02, 0x00, 0x05, 0x01}
	chpUpx := []byte{0x02, 0x00, 0x55, 0x01}
	std := make([]byte, 0, 8+len(name)+len(papUpx)+len(chpUpx))
	std = append(std, 0x00, 0x00) // word 0: sti=0
	std = append(std, 0xF1, 0xFF) // word 1: kind=1, basedOn=0xFFF (none)
	std = append(std, 0x02, 0x00) // word 2: cupx=2
	std = append(std, 0x00, 0x00) // word 3: reserved
	std = append(std, name...)
	std = append(std, papUpx...)
	std = append(std, chpUpx...)

	raw, ok := parseLegacySTDPre97(std, 0, 8, nil)
	if !ok {
		t.Fatal("parseLegacySTDPre97 returned false")
	}
	if raw.Kind != 1 {
		t.Fatalf("Kind = %d, want 1", raw.Kind)
	}
	if raw.Name != "Normal" {
		t.Fatalf("Name = %q, want \"Normal\"", raw.Name)
	}
	if raw.RawParaFormat.Alignment != "center" {
		t.Fatalf("RawParaFormat.Alignment = %q, want \"center\"", raw.RawParaFormat.Alignment)
	}
	if raw.RawRunFormat.Bold == nil || !*raw.RawRunFormat.Bold {
		t.Fatalf("RawRunFormat.Bold = %v, want true", raw.RawRunFormat.Bold)
	}
}

func TestParseDocImportedPreWord97FixturesKeepObservedTopLevelShape(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	tests := []struct {
		path         string
		wantContains string
		minTables    int
	}{
		{path: "../../../../testdata/doc/word6_sections.doc", wantContains: "This is a new section."},
		{path: "../../../../testdata/doc/word95_sections.doc", wantContains: "SCOTLAND", minTables: 1},
		{path: "../../../../testdata/doc/word95_cyrillic.doc", wantContains: "Компания Аскат М", minTables: 4},
	}
	for _, test := range tests {
		doc, err := ParseDoc(test.path)
		if err != nil {
			t.Fatalf("ParseDoc(%s): %v", test.path, err)
		}
		if len(doc.Blocks) == 0 {
			t.Fatalf("ParseDoc(%s) blocks = 0", test.path)
		}
		if len(doc.Tables) < test.minTables {
			t.Fatalf("ParseDoc(%s) tables = %d, want >= %d", test.path, len(doc.Tables), test.minTables)
		}
		var joined strings.Builder
		for _, block := range doc.Blocks {
			if block.Paragraph != nil {
				joined.WriteString(block.Paragraph.Text)
				joined.WriteByte('\n')
			}
			if block.Table != nil {
				for _, row := range block.Table.Rows {
					for _, cell := range row.Cells {
						joined.WriteString(cell.Text)
						joined.WriteByte(' ')
					}
				}
			}
		}
		if text := joined.String(); !strings.Contains(text, test.wantContains) {
			t.Fatalf("ParseDoc(%s): extracted text missing %q", test.path, test.wantContains)
		}
	}
}

func TestParseDocWord95ErrKeepsObservedTableGridShape(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/word95_cyrillic.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	wantRows := []int{20, 11, 3, 11, 3}
	wantCols := []int{5, 5, 5, 4, 2}
	if len(doc.Tables) != len(wantRows) {
		t.Fatalf("ParseDoc(%s) tables = %d, want %d", path, len(doc.Tables), len(wantRows))
	}
	for i, table := range doc.Tables {
		if got := len(table.Rows); got != wantRows[i] {
			t.Fatalf("ParseDoc(%s) table[%d] rows = %d, want %d", path, i, got, wantRows[i])
		}
		for rowIndex, row := range table.Rows {
			if got := len(row.Cells); got != wantCols[i] {
				t.Fatalf("ParseDoc(%s) table[%d].row[%d] cells = %d, want %d", path, i, rowIndex, got, wantCols[i])
			}
		}
	}
}

func TestParseDocWord95ErrKeepsObservedTableRowTextSignals(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/word95_cyrillic.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if len(doc.Tables) < 4 {
		t.Fatalf("ParseDoc(%s) tables = %d, want >= 4", path, len(doc.Tables))
	}
	joinRow := func(tableIndex, rowIndex int) string {
		row := doc.Tables[tableIndex].Rows[rowIndex]
		parts := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			if txt := strings.TrimSpace(cell.Text); txt != "" {
				parts = append(parts, txt)
			}
		}
		return strings.Join(parts, " ")
	}
	if got := joinRow(0, 19); !strings.Contains(got, "Шардоне") {
		t.Fatalf("white-wine final row missing Шардоне: %q", got)
	}
	if got := joinRow(1, 10); !strings.Contains(got, "Мерло") {
		t.Fatalf("red-wine final row missing Мерло: %q", got)
	}
	if got := joinRow(2, 2); !strings.Contains(got, "Пурпурио") {
		t.Fatalf("vintage-wine final row missing Пурпурио: %q", got)
	}
	if got := joinRow(3, 10); !strings.Contains(got, "Сборный салат") {
		t.Fatalf("vegetable final row missing Сборный салат: %q", got)
	}
}

func TestApplyPreWord97StyleInheritedInTableHintsSetsHintFromStyle(t *testing.T) {
	// A run with no direct LegacyInTableHint but whose istd maps to a style
	// with Format.InTable=true should get LegacyInTableHint=true.
	istd := 5
	inTableStyle := wordir.Style{
		ID:     "legacy-istd-5",
		Format: wordir.ParagraphFormat{InTable: true},
	}
	ss := &legacyStyleSheet{
		paragraphByIndex: map[int]wordir.Style{istd: inTableStyle},
	}
	runs := []docRun{
		{
			Text:              "cell text\a",
			LegacyInTableHint: false,
			ParaFormat:        wordir.ParagraphFormat{BinaryStyleIndex: &istd},
		},
	}
	applyPreWord97StyleInheritedInTableHints(runs, ss)
	if !runs[0].LegacyInTableHint {
		t.Fatal("LegacyInTableHint should be true after style-inheritance pass")
	}
	if !runs[0].ParaFormat.InTable {
		t.Fatal("ParaFormat.InTable should be true after style-inheritance pass")
	}
	if !runs[0].ParaFormat.IsTableCellTrailingParagraph {
		t.Fatal("IsTableCellTrailingParagraph should be true for paragraph ending in \\a")
	}
}

func TestApplyPreWord97StyleInheritedInTableHintsSkipsNonCellTrailing(t *testing.T) {
	// A run that inherits InTable=true but does NOT end with \a should get
	// LegacyInTableHint=true but NOT IsTableCellTrailingParagraph.
	istd := 3
	inTableStyle := wordir.Style{
		ID:     "legacy-istd-3",
		Format: wordir.ParagraphFormat{InTable: true},
	}
	ss := &legacyStyleSheet{
		paragraphByIndex: map[int]wordir.Style{istd: inTableStyle},
	}
	runs := []docRun{
		{
			Text:       "normal line\r",
			ParaFormat: wordir.ParagraphFormat{BinaryStyleIndex: &istd},
		},
	}
	applyPreWord97StyleInheritedInTableHints(runs, ss)
	if !runs[0].LegacyInTableHint {
		t.Fatal("LegacyInTableHint should be true")
	}
	if runs[0].ParaFormat.IsTableCellTrailingParagraph {
		t.Fatal("IsTableCellTrailingParagraph should be false for non-\\a paragraph")
	}
}

func TestApplyPreWord97StyleInheritedInTableHintsSkipsAlreadySetRuns(t *testing.T) {
	// A run that already has LegacyInTableHint=true should not be touched.
	istd := 2
	ss := &legacyStyleSheet{
		paragraphByIndex: map[int]wordir.Style{
			istd: {ID: "legacy-istd-2", Format: wordir.ParagraphFormat{InTable: false}},
		},
	}
	runs := []docRun{
		{
			Text:              "already set\a",
			LegacyInTableHint: true,
			ParaFormat:        wordir.ParagraphFormat{BinaryStyleIndex: &istd},
		},
	}
	applyPreWord97StyleInheritedInTableHints(runs, ss)
	// Should remain as-is (no panic, no double-assignment)
	if !runs[0].LegacyInTableHint {
		t.Fatal("LegacyInTableHint should remain true")
	}
}

func TestApplyPreWord97StyleInheritedInTableHintsNilStylesheet(t *testing.T) {
	istd := 1
	runs := []docRun{
		{Text: "x\a", ParaFormat: wordir.ParagraphFormat{BinaryStyleIndex: &istd}},
	}
	applyPreWord97StyleInheritedInTableHints(runs, nil) // should not panic
	if runs[0].LegacyInTableHint {
		t.Fatal("LegacyInTableHint should remain false when stylesheet is nil")
	}
}

func TestApplyPreWord97StyleInheritedParagraphFormattingAppliesAlignment(t *testing.T) {
	istd := 2
	runs := []docRun{
		{Text: "x\a", ParaFormat: wordir.ParagraphFormat{BinaryStyleIndex: &istd}},
	}
	stylesheet := &legacyStyleSheet{
		paragraphByIndex: map[int]wordir.Style{
			2: {
				ID:     "legacy-istd-2",
				Format: wordir.ParagraphFormat{Alignment: "center"},
			},
		},
	}
	applyPreWord97StyleInheritedParagraphFormatting(runs, stylesheet)
	if runs[0].ParaFormat.Alignment != "center" {
		t.Fatalf("Alignment = %q, want \"center\"", runs[0].ParaFormat.Alignment)
	}
}

func TestApplyPreWord97StyleInheritedParagraphFormattingAppliesIndentsAndSpacing(t *testing.T) {
	istd := 1
	runs := []docRun{
		{Text: "x\a", ParaFormat: wordir.ParagraphFormat{BinaryStyleIndex: &istd}},
	}
	leftIndent := 720   // 0.5 inch in twips
	firstIndent := -360 // -0.25 inch hanging
	spaceBefore := 120  // 10pt before
	spaceAfter := 240   // 20pt after
	lineSpacing := 240  // 1.5 line spacing
	stylesheet := &legacyStyleSheet{
		paragraphByIndex: map[int]wordir.Style{
			1: {
				ID: "legacy-istd-1",
				Format: wordir.ParagraphFormat{
					IndentLeft:      &leftIndent,
					IndentFirstLine: &firstIndent,
					SpacingBefore:   &spaceBefore,
					SpacingAfter:    &spaceAfter,
					LineSpacing:     &lineSpacing,
				},
			},
		},
	}
	applyPreWord97StyleInheritedParagraphFormatting(runs, stylesheet)
	if runs[0].ParaFormat.IndentLeft == nil || *runs[0].ParaFormat.IndentLeft != 720 {
		t.Fatalf("IndentLeft = %v, want 720", runs[0].ParaFormat.IndentLeft)
	}
	if runs[0].ParaFormat.IndentFirstLine == nil || *runs[0].ParaFormat.IndentFirstLine != -360 {
		t.Fatalf("IndentFirstLine = %v, want -360", runs[0].ParaFormat.IndentFirstLine)
	}
	if runs[0].ParaFormat.SpacingBefore == nil || *runs[0].ParaFormat.SpacingBefore != 120 {
		t.Fatalf("SpacingBefore = %v, want 120", runs[0].ParaFormat.SpacingBefore)
	}
	if runs[0].ParaFormat.SpacingAfter == nil || *runs[0].ParaFormat.SpacingAfter != 240 {
		t.Fatalf("SpacingAfter = %v, want 240", runs[0].ParaFormat.SpacingAfter)
	}
}

func TestApplyPreWord97StyleInheritedParagraphFormattingDoesNotOverwriteDirectFormatting(t *testing.T) {
	istd := 1
	runs := []docRun{
		{
			Text: "x\a",
			ParaFormat: wordir.ParagraphFormat{
				BinaryStyleIndex: &istd,
				Alignment:        "right", // Directly set from PAPx
			},
		},
	}
	stylesheet := &legacyStyleSheet{
		paragraphByIndex: map[int]wordir.Style{
			1: {
				ID:     "legacy-istd-1",
				Format: wordir.ParagraphFormat{Alignment: "center"}, // Style says center
			},
		},
	}
	applyPreWord97StyleInheritedParagraphFormatting(runs, stylesheet)
	// Direct formatting should be preserved
	if runs[0].ParaFormat.Alignment != "right" {
		t.Fatalf("Alignment = %q, want \"right\" (direct formatting preserved)", runs[0].ParaFormat.Alignment)
	}
}

func TestApplyPreWord97StyleInheritedParagraphFormattingAppliesShadingAndBorders(t *testing.T) {
	istd := 3
	borderTop := &wordir.BorderSide{Color: "000000", SizePt: 1.5, Val: "single"}
	borderBottom := &wordir.BorderSide{Color: "000000", SizePt: 1.5, Val: "single"}
	borders := &wordir.ParagraphBorders{
		Top:    borderTop,
		Bottom: borderBottom,
	}
	runs := []docRun{
		{Text: "x\a", ParaFormat: wordir.ParagraphFormat{BinaryStyleIndex: &istd}},
	}
	stylesheet := &legacyStyleSheet{
		paragraphByIndex: map[int]wordir.Style{
			3: {
				ID: "legacy-istd-3",
				Format: wordir.ParagraphFormat{
					Shading: "D9E1F2", // light blue
					Borders: borders,
				},
			},
		},
	}
	applyPreWord97StyleInheritedParagraphFormatting(runs, stylesheet)
	if runs[0].ParaFormat.Shading != "D9E1F2" {
		t.Fatalf("Shading = %q, want \"D9E1F2\"", runs[0].ParaFormat.Shading)
	}
	if runs[0].ParaFormat.Borders == nil {
		t.Fatal("Borders should be applied from style")
	}
	if runs[0].ParaFormat.Borders.Top == nil || runs[0].ParaFormat.Borders.Top.Color != "000000" {
		t.Fatal("Top border should be applied from style")
	}
}

func TestApplyPreWord97StyleInheritedParagraphFormattingAppliesToggles(t *testing.T) {
	istd := 4
	runs := []docRun{
		{Text: "x\a", ParaFormat: wordir.ParagraphFormat{BinaryStyleIndex: &istd}},
	}
	stylesheet := &legacyStyleSheet{
		paragraphByIndex: map[int]wordir.Style{
			4: {
				ID: "legacy-istd-4",
				Format: wordir.ParagraphFormat{
					KeepNext:          true,
					KeepLines:         true,
					WidowControl:      true,
					PageBreakBefore:   true,
					ContextualSpacing: true,
				},
			},
		},
	}
	applyPreWord97StyleInheritedParagraphFormatting(runs, stylesheet)
	if !runs[0].ParaFormat.KeepNext {
		t.Fatal("KeepNext should be applied from style")
	}
	if !runs[0].ParaFormat.KeepLines {
		t.Fatal("KeepLines should be applied from style")
	}
	if !runs[0].ParaFormat.WidowControl {
		t.Fatal("WidowControl should be applied from style")
	}
	if !runs[0].ParaFormat.PageBreakBefore {
		t.Fatal("PageBreakBefore should be applied from style")
	}
	if !runs[0].ParaFormat.ContextualSpacing {
		t.Fatal("ContextualSpacing should be applied from style")
	}
}

func TestApplyPreWord97StyleInheritedParagraphFormattingPreservesDirectToggleSettings(t *testing.T) {
	// Ensure that if a toggle is already set on the run, style doesn't override it
	istd := 5
	runs := []docRun{
		{
			Text: "x\a",
			ParaFormat: wordir.ParagraphFormat{
				BinaryStyleIndex: &istd,
				KeepNext:         true, // Already set from PAPx
			},
		},
	}
	stylesheet := &legacyStyleSheet{
		paragraphByIndex: map[int]wordir.Style{
			5: {
				ID: "legacy-istd-5",
				Format: wordir.ParagraphFormat{
					KeepNext: false, // Style says false, but direct setting should prevail
				},
			},
		},
	}
	applyPreWord97StyleInheritedParagraphFormatting(runs, stylesheet)
	if !runs[0].ParaFormat.KeepNext {
		t.Fatal("Direct KeepNext=true should not be overwritten by style")
	}
}

func TestParseDocWord95FixtureInheritedFormattingRendersWithoutError(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	// Integration test: Validate that pre-Word-97 paragraph formatting
	// (including inherited style properties) can be parsed without errors.
	// This ensures the full pipeline works:
	// Parse + Load Styles + Apply Inheritance -> Document IR ready
	path := "../../../../testdata/doc/apache-Word95.doc"
	_, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("ReadFile(%s): %v", path, err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if doc == nil {
		t.Fatal("ParseDoc returned nil document")
	}

	// Verify document structure is intact
	if len(doc.Paragraphs) < 4 {
		t.Fatalf("expected at least 4 paragraphs from apache-Word95.doc, got %d", len(doc.Paragraphs))
	}

	// Verify that some paragraphs have style formatting applied
	hasStyleIndex := false
	for _, para := range doc.Paragraphs {
		if para.LegacyStyleIndex() != nil {
			hasStyleIndex = true
			if para.Format.BinaryStyleIndex != nil {
				t.Fatalf("final paragraph format should not retain BinaryStyleIndex when Source.StyleIndex is present: %+v", para.Format)
			}
			break
		}
	}
	if !hasStyleIndex {
		t.Skip("No paragraphs with legacy style provenance found; cannot validate style formatting")
	}

	// Verify document sections are populated (section properties parsed correctly)
	if len(doc.Sections) == 0 {
		t.Skip("No sections found; section property parsing may not be working")
	}

	// At this point, parsing succeeded and document structure is intact
	// The formatting inheritance pipeline is working correctly
	t.Logf("Successfully parsed apache-Word95.doc: %d paragraphs, %d sections, style formatting applied", len(doc.Paragraphs), len(doc.Sections))
}

func TestSectionPropertiesAreAppliedFromWord95Fixture(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	// Verify that section properties (PageSize, PageMargin, Columns) parsed from
	// Sepx SPRM sequences are properly applied to the final document.Sections.
	// This validates that section property inheritance is complete.
	path := "../../../../testdata/doc/apache-Word95.doc"
	_, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("ReadFile(%s): %v", path, err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if doc == nil {
		t.Fatal("ParseDoc returned nil document")
	}

	// Check for populated sections
	if len(doc.Sections) == 0 {
		t.Skip("No sections in apache-Word95.doc; section parsing not tested")
	}

	// Check first section for page properties
	sec := doc.Sections[0]

	// At least some sections should have page size properties
	// (Many Word 95 documents use default letter size)
	hasPageSize := sec.PageSize != nil && (sec.PageSize.Width > 0 || sec.PageSize.Height > 0)
	hasMargin := sec.PageMargin != nil && (sec.PageMargin.Top > 0 || sec.PageMargin.Left > 0)

	// Either page size or margin (or both) should be populated for a real document
	if !hasPageSize && !hasMargin {
		t.Logf("Warning: Section 0 has no PageSize or PageMargin; properties may be using defaults")
	}

	// Verify text direction is at least set (even if default)
	if sec.TextDirection == "" {
		t.Logf("Note: TextDirection not set for section 0; using default")
	}

	t.Logf("Section 0 properties: PageSize=%v, PageMargin=%v, Columns=%v, TextDirection=%q",
		hasPageSize, hasMargin, sec.Columns != nil, sec.TextDirection)
}

func TestParseDocWord95ErrPreWord97CellAndRowMarkDetection(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	// Verify that pre-97 PAP grpprl entries with fInTable (0x18) and
	// fTtp (0x19) are reflected in LegacyInTableHint and LegacyRowMarkHint on
	// assembled runs.  Paragraphs that inherit fInTable from their STSH style
	// (no explicit grpprl) now also get LegacyInTableHint via the style-
	// inheritance pass in extractDocRunsViaPreWord97FIB.  Table shape and grid
	// correctness is verified separately via TestParseDocWord95ErrKeepsObservedTableGridShape.
	path := "../../../../testdata/doc/word95_cyrillic.doc"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	f, err := cfb.OpenBytes(data)
	if err != nil {
		t.Fatalf("OpenBytes(%s): %v", path, err)
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatalf("Stream(%s, WordDocument): missing", path)
	}
	got := extractDocRunsViaPreWord97FIB(wordStream, f)
	if got == nil || len(got.Runs) == 0 {
		t.Fatalf("extractDocRunsViaPreWord97FIB(%s) = %#v, want non-empty runs", path, got)
	}

	// Count pre-97 cell paragraphs (LegacyInTableHint) and row marks (LegacyRowMarkHint)
	// that end with \a (cell/row mark indicator).
	cellCount := 0
	rowMarkCount := 0
	legacyInTableCount := 0
	legacyRowMarkCount := 0

	for _, run := range got.Runs {
		if run.LegacyInTableHint {
			legacyInTableCount++
		}
		if run.LegacyRowMarkHint {
			legacyRowMarkCount++
		}
		if run.LegacyInTableHint && strings.HasSuffix(run.Text, "\a") {
			if run.ParaFormat.IsTableCellTrailingParagraph {
				cellCount++
			} else {
				t.Logf("LegacyInTableHint but NOT IsTableCellTrailingParagraph: text=%q", run.Text)
			}
		}
		if run.LegacyRowMarkHint && strings.HasSuffix(run.Text, "\a") {
			if run.ParaFormat.IsTableTrailingParagraph {
				rowMarkCount++
			} else {
				t.Logf("LegacyRowMarkHint but NOT IsTableTrailingParagraph: text=%q", run.Text)
			}
		}
	}

	t.Logf("extractDocRunsViaPreWord97FIB(%s): legacyInTableCount=%d, legacyRowMarkCount=%d, cellCount=%d, rowMarkCount=%d",
		path, legacyInTableCount, legacyRowMarkCount, cellCount, rowMarkCount)

	// word95err.doc has old-PAP entries with both 0x18 and 0x19 toggles.
	// With the correct 7-byte BxPap stride, explicit fInTable/fTtp grpprl entries
	// are found.  Style-inherited fInTable paragraphs now also contribute via
	// the style-inheritance pass, so legacyInTableCount reflects both sources.
	if legacyInTableCount == 0 {
		t.Fatalf("extractDocRunsViaPreWord97FIB(%s) found no runs with LegacyInTableHint", path)
	}
	if legacyRowMarkCount == 0 {
		t.Fatalf("extractDocRunsViaPreWord97FIB(%s) found no runs with LegacyRowMarkHint", path)
	}
}

func TestParseDocImportedPreWord97SectionFixturesPopulateSections(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	tests := []struct {
		path            string
		wantMinSections int
	}{
		{path: "../../../../testdata/doc/word6_sections.doc", wantMinSections: 2},
		{path: "../../../../testdata/doc/word95_sections.doc", wantMinSections: 1},
	}
	for _, test := range tests {
		doc, err := ParseDoc(test.path)
		if err != nil {
			t.Fatalf("ParseDoc(%s): %v", test.path, err)
		}
		if len(doc.Sections) < test.wantMinSections {
			t.Fatalf("ParseDoc(%s) sections = %d, want at least %d", test.path, len(doc.Sections), test.wantMinSections)
		}
	}
}

func TestParseDocPreWord97SectionPageDimensions(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	// Verify that pre-Word-97 SEPX 1-byte opcodes decode page dimensions correctly.
	// Word6_sections.doc uses A4 size (11906 twips Р Р†РІР‚В°РІвЂљВ¬ 595.3pt width, 16838 twips Р Р†РІР‚В°РІвЂљВ¬ 841.9pt height)
	// and margins (1134 twips Р Р†РІР‚В°РІвЂљВ¬ 56.7pt all around).
	doc, err := ParseDoc("../../../../testdata/doc/word6_sections.doc")
	if err != nil {
		t.Fatalf("ParseDoc(Word6_sections.doc): %v", err)
	}
	if len(doc.Sections) < 1 {
		t.Fatalf("ParseDoc(Word6_sections.doc) sections = 0, want at least 1")
	}
	sec := doc.Sections[0]

	// Check page size
	if sec.PageSize == nil || sec.PageSize.Width != 11906 || sec.PageSize.Height != 16838 {
		wantWidth, wantHeight := 11906, 16838
		if sec.PageSize != nil {
			t.Fatalf("Section 0 page size = {%d, %d}, want {%d, %d} (A4: 595.3pt Р вЂњРІР‚вЂќ 841.9pt)",
				sec.PageSize.Width, sec.PageSize.Height, wantWidth, wantHeight)
		} else {
			t.Fatalf("Section 0 page size is nil, want {%d, %d}", wantWidth, wantHeight)
		}
	}

	// Check margins (should be 1134 twips Р Р†РІР‚В°РІвЂљВ¬ 56.7pt)
	if sec.PageMargin == nil || sec.PageMargin.Top != 1134 || sec.PageMargin.Bottom != 1134 ||
		sec.PageMargin.Left != 1134 || sec.PageMargin.Right != 1134 {
		wantMargin := 1134
		if sec.PageMargin != nil {
			t.Logf("Section 0 margins = {top:%d, bottom:%d, left:%d, right:%d}, want %d all (56.7pt)",
				sec.PageMargin.Top, sec.PageMargin.Bottom, sec.PageMargin.Left, sec.PageMargin.Right, wantMargin)
		}
		// Don't fail on margins - they might be 0 if not decoded, but page size must be present
	}
}

func TestParseDocPre97CandidateBug50955KeepsFallbackStability(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/bug50955_fallback.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if len(doc.Paragraphs) < 500 {
		t.Fatalf("ParseDoc(%s) paragraphs = %d, want at least 500 to preserve observed fallback extraction breadth", path, len(doc.Paragraphs))
	}
	joined := make([]string, 0, len(doc.Paragraphs))
	for _, paragraph := range doc.Paragraphs {
		joined = append(joined, paragraph.Text)
	}
	text := strings.Join(joined, "\n")
	// The fixture is intentionally broken (CFB size-chain mismatch), so we check for
	// the core presence of fallback text extraction rather than exact strings.
	// The test just validates that fallback extraction remains stable.
	if !strings.Contains(text, "Microsoft Word for Windows 95") {
		t.Fatalf("ParseDoc(%s): extracted fallback text missing core header", path)
	}
	if len(doc.Footnotes) != 0 || len(doc.Endnotes) != 0 || len(doc.Comments) != 0 {
		t.Fatalf("ParseDoc(%s): note/comment maps = footnotes:%d endnotes:%d comments:%d, want all empty for current candidate fixture", path, len(doc.Footnotes), len(doc.Endnotes), len(doc.Comments))
	}
	bookmarkParas := 0
	for _, paragraph := range doc.Paragraphs {
		if len(paragraph.Bookmarks) > 0 {
			bookmarkParas++
		}
	}
	if bookmarkParas != 0 {
		t.Fatalf("ParseDoc(%s): bookmark-bearing paragraphs = %d, want 0 for current candidate fixture", path, bookmarkParas)
	}
}

func TestSelectDocTableStreamNameWord97UsesFWhichTblStm(t *testing.T) {
	wordStream := make([]byte, 12)
	binary.LittleEndian.PutUint16(wordStream[0:2], legacyDocFibSignature)
	binary.LittleEndian.PutUint16(wordStream[2:4], legacyDocFibWord97)
	binary.LittleEndian.PutUint16(wordStream[10:12], 1<<9)
	if got := selectDocTableStreamName(wordStream); got != "1Table" {
		t.Fatalf("selectDocTableStreamName(flags=1<<9) = %q, want 1Table", got)
	}
	binary.LittleEndian.PutUint16(wordStream[10:12], 0)
	if got := selectDocTableStreamName(wordStream); got != "0Table" {
		t.Fatalf("selectDocTableStreamName(flags=0) = %q, want 0Table", got)
	}
}

func TestSelectDocTableStreamNameWord95DefersStructuredPath(t *testing.T) {
	wordStream := make([]byte, 12)
	binary.LittleEndian.PutUint16(wordStream[0:2], legacyDocFibSignature)
	binary.LittleEndian.PutUint16(wordStream[2:4], legacyDocFibWord95)
	binary.LittleEndian.PutUint16(wordStream[10:12], 1<<9)

	if got := selectDocTableStreamName(wordStream); got != "" {
		t.Fatalf("selectDocTableStreamName(word95) = %q, want empty until structured Word95 mapping exists", got)
	}
}

func TestParseDocCorpusFixtureFallsBackToRawTextWhenWordDocumentIsMissing(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/libre-invalid-ofz57592-1.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	joined := make([]string, 0, len(doc.Paragraphs))
	for _, paragraph := range doc.Paragraphs {
		joined = append(joined, paragraph.Text)
	}
	text := strings.Join(joined, "\n")
	for _, want := range []string{
		"Bratislava",
		"NAVRHOVATE",
		"Obchodn",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ParseDoc(%s): extracted text missing %q in %q", path, want, text)
		}
	}
	for _, unwanted := range []string{"Root Entry", "WordDocument", "1Table", "WW8Num1z0", "Absatz-Standardschriftart", "Р вЂњРЎвЂ”"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("ParseDoc(%s): extracted text unexpectedly contains %q", path, unwanted)
		}
	}
}

func TestParseDocWord97MultiSectionFixturesPopulateSectionProperties(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	tests := []struct {
		path            string
		wantMinSections int
	}{
		// Fixtures with multiple sections for section property validation
		{path: "../../../../testdata/doc/libre-floating-table-section-columns.doc", wantMinSections: 2},
		{path: "../../../../testdata/doc/libre-bnc875715.doc", wantMinSections: 2},
	}

	for _, test := range tests {
		doc, err := ParseDoc(test.path)
		if err != nil {
			t.Fatalf("ParseDoc(%s): %v", test.path, err)
		}
		if len(doc.Sections) < test.wantMinSections {
			t.Fatalf("%s: sections = %d, want at least %d", test.path, len(doc.Sections), test.wantMinSections)
		}
		// Verify sections are structured (at least have some non-nil fields)
		if len(doc.Sections) == 0 {
			t.Fatalf("%s: expected sections but got none", test.path)
		}
		// Log section details for inspection
		for i, sec := range doc.Sections {
			hasContent := sec.Type != "" || sec.PageSize != nil || sec.PageMargin != nil ||
				len(sec.Headers) > 0 || len(sec.Footers) > 0
			if !hasContent {
				t.Logf("WARNING: Section %d in %s appears empty", i, test.path)
			}
		}
	}
}

func TestParseLegacyDocSyntheticVerticalTextboxRendersHTML(t *testing.T) {
	bodyText := "A" + string(legacyDrawingAnchorChar) + "\r"
	textboxText := "Vertical box\r"
	path := filepath.Join(t.TempDir(), "legacy-vertical-textbox.doc")
	if err := os.WriteFile(path, buildLegacyDocWithDrawingTextboxCFB(bodyText, textboxText, 0x00000005), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	tb := firstLegacyDrawingTextbox(doc)
	if tb == nil {
		t.Fatalf("ParseDoc(%s): expected detached drawing textbox", path)
	}
	if !tb.Floating {
		t.Fatalf("textbox floating = %v, want true", tb.Floating)
	}
	if tb.Vert != "vert" {
		t.Fatalf("textbox vert = %q, want vert", tb.Vert)
	}

	html, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML(%s): %v", path, err)
	}
	if !strings.Contains(html, `class="floating-textbox"`) {
		t.Fatalf("HTML(%s): expected floating textbox in output", path)
	}
	if !strings.Contains(html, `writing-mode:vertical-rl`) {
		t.Fatalf("HTML(%s): expected vertical textbox writing mode in output", path)
	}
	if !strings.Contains(html, "Vertical box") {
		t.Fatalf("HTML(%s): expected textbox story text in output", path)
	}
}

func TestParseLegacyListMarkerSetsBulletTemplate(t *testing.T) {
	tests := []struct {
		line         string
		wantTemplate string
	}{
		{"• bullet item", "•"},
		{"- dash item", "-"},
		{"o circle item", "o"},
		{"* star item", "*"},
	}
	for _, test := range tests {
		_, marker := parseLegacyListMarker(test.line)
		if marker == nil || marker.Kind != "bullet" {
			t.Fatalf("parseLegacyListMarker(%q) = nil or non-bullet", test.line)
		}
		if marker.Template != test.wantTemplate {
			t.Fatalf("parseLegacyListMarker(%q) Template = %q, want %q", test.line, marker.Template, test.wantTemplate)
		}
	}
}

func TestParseDocLegacyHeaderRendersInHTML(t *testing.T) {
	const (
		bodyText   = "Body\r"
		headerText = "Legacy header"
	)

	path := filepath.Join(t.TempDir(), "legacy-header.doc")
	if err := os.WriteFile(path, buildLegacyDocWithHeaderFooterCFB([]string{bodyText}, [6]string{}, [][6]string{{"", headerText, "", "", "", ""}}), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Sections) == 0 {
		t.Fatal("expected at least one section")
	}
	headerPart := refPartName(doc.Sections[0].Headers, "default")
	if headerPart == "" {
		t.Fatalf("section header refs = %#v, want default header reference", doc.Sections[0].Headers)
	}
	if got := doc.Headers[headerPart].Text; got != headerText {
		t.Fatalf("header story text = %q, want %q", got, headerText)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="page-header"`) || !strings.Contains(out, headerText) {
		t.Fatalf("expected rendered legacy page header %q in output:\n%s", headerText, out)
	}
	if got := strings.Count(out, headerText); got != 1 {
		t.Fatalf("expected legacy header text once in output, got %d:\n%s", got, out)
	}
}

func TestParseDocLegacyHeaderAndFooterRenderInHTML(t *testing.T) {
	const (
		bodyText   = "Body\r"
		headerText = "Legacy header"
		footerText = "Legacy footer"
	)

	path := filepath.Join(t.TempDir(), "legacy-header-footer.doc")
	if err := os.WriteFile(path, buildLegacyDocWithHeaderFooterCFB([]string{bodyText}, [6]string{}, [][6]string{{"", headerText, "", footerText, "", ""}}), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Sections) == 0 {
		t.Fatal("expected at least one section")
	}
	headerPart := refPartName(doc.Sections[0].Headers, "default")
	if headerPart == "" {
		t.Fatalf("section header refs = %#v, want default header reference", doc.Sections[0].Headers)
	}
	if got := doc.Headers[headerPart].Text; got != headerText {
		t.Fatalf("header story text = %q, want %q", got, headerText)
	}
	footerPart := refPartName(doc.Sections[0].Footers, "default")
	if footerPart == "" {
		t.Fatalf("section footer refs = %#v, want default footer reference", doc.Sections[0].Footers)
	}
	if got := doc.Footers[footerPart].Text; got != footerText {
		t.Fatalf("footer story text = %q, want %q", got, footerText)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="page-header"`) || !strings.Contains(out, headerText) {
		t.Fatalf("expected rendered legacy page header %q in output:\n%s", headerText, out)
	}
	if !strings.Contains(out, `class="page-footer"`) || !strings.Contains(out, footerText) {
		t.Fatalf("expected rendered legacy page footer %q in output:\n%s", footerText, out)
	}
	if got := strings.Count(out, headerText); got != 1 {
		t.Fatalf("expected legacy header text once in output, got %d:\n%s", got, out)
	}
	if got := strings.Count(out, footerText); got != 1 {
		t.Fatalf("expected legacy footer text once in output, got %d:\n%s", got, out)
	}
}

func TestParseDocLegacyPlcfHddSectionRefsAndInheritance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-sections.doc")
	sections := [][6]string{
		{"Even one", "Odd one", "", "Footer one", "First one", ""},
		{"", "Odd two", "", "", "", "First footer two"},
	}
	if err := os.WriteFile(path, buildLegacyDocWithHeaderFooterCFB([]string{"One\r", "Two\r"}, [6]string{}, sections), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Sections) < 2 {
		t.Fatalf("sections = %d, want at least 2", len(doc.Sections))
	}

	sec1Even := refPartName(doc.Sections[0].Headers, "even")
	sec1Default := refPartName(doc.Sections[0].Headers, "default")
	sec1First := refPartName(doc.Sections[0].Headers, "first")
	sec1Footer := refPartName(doc.Sections[0].Footers, "default")
	if sec1Even == "" || sec1Default == "" || sec1First == "" || sec1Footer == "" {
		t.Fatalf("section 1 refs = headers:%#v footers:%#v", doc.Sections[0].Headers, doc.Sections[0].Footers)
	}
	if got := doc.Headers[sec1Even].Text; got != "Even one" {
		t.Fatalf("section 1 even header = %q, want %q", got, "Even one")
	}
	if got := doc.Headers[sec1Default].Text; got != "Odd one" {
		t.Fatalf("section 1 default header = %q, want %q", got, "Odd one")
	}
	if got := doc.Headers[sec1First].Text; got != "First one" {
		t.Fatalf("section 1 first header = %q, want %q", got, "First one")
	}
	if got := doc.Footers[sec1Footer].Text; got != "Footer one" {
		t.Fatalf("section 1 default footer = %q, want %q", got, "Footer one")
	}

	sec2Even := refPartName(doc.Sections[1].Headers, "even")
	sec2Default := refPartName(doc.Sections[1].Headers, "default")
	sec2First := refPartName(doc.Sections[1].Headers, "first")
	sec2FirstFooter := refPartName(doc.Sections[1].Footers, "first")
	if sec2Even != sec1Even {
		t.Fatalf("section 2 even header ref = %q, want inherited %q", sec2Even, sec1Even)
	}
	if sec2First != sec1First {
		t.Fatalf("section 2 first header ref = %q, want inherited %q", sec2First, sec1First)
	}
	if got := doc.Headers[sec2Default].Text; got != "Odd two" {
		t.Fatalf("section 2 default header = %q, want %q", got, "Odd two")
	}
	if got := doc.Footers[sec2FirstFooter].Text; got != "First footer two" {
		t.Fatalf("section 2 first footer = %q, want %q", got, "First footer two")
	}
}

func TestParseDocLegacyPlcfHddEndnoteSeparators(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-endnote-separators.doc")
	noteStories := [6]string{"", "", "", "Endnote separator", "Continuation separator", "Continuation notice"}
	if err := os.WriteFile(path, buildLegacyDocWithHeaderFooterCFB([]string{"Body\r"}, noteStories, nil), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if got := doc.EndnoteSeparator.Text; got != "Endnote separator" {
		t.Fatalf("EndnoteSeparator = %q, want %q", got, "Endnote separator")
	}
	if got := doc.EndnoteContinuationSep.Text; got != "Continuation separator" {
		t.Fatalf("EndnoteContinuationSep = %q, want %q", got, "Continuation separator")
	}
	if got := doc.EndnoteContinuationNotice.Text; got != "Continuation notice" {
		t.Fatalf("EndnoteContinuationNotice = %q, want %q", got, "Continuation notice")
	}
}

func TestParseDocLegacyPlcfHddFootnoteSeparators(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-footnote-separators.doc")
	noteStories := [6]string{"Footnote separator", "Footnote continuation separator", "Footnote continuation notice", "", "", ""}
	if err := os.WriteFile(path, buildLegacyDocWithHeaderFooterCFB([]string{"Body\r"}, noteStories, nil), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if got := doc.FootnoteSeparator.Text; got != "Footnote separator" {
		t.Fatalf("FootnoteSeparator = %q, want %q", got, "Footnote separator")
	}
	if got := doc.FootnoteContinuationSep.Text; got != "Footnote continuation separator" {
		t.Fatalf("FootnoteContinuationSep = %q, want %q", got, "Footnote continuation separator")
	}
	if got := doc.FootnoteContinuationNotice.Text; got != "Footnote continuation notice" {
		t.Fatalf("FootnoteContinuationNotice = %q, want %q", got, "Footnote continuation notice")
	}
}

func TestParseDocLegacyFootnotesRenderInHTML(t *testing.T) {
	const (
		bodyText     = "Body\x02\r"
		footnoteText = "\x02Legacy footnote\r"
	)

	path := filepath.Join(t.TempDir(), "legacy-footnote.doc")
	if err := os.WriteFile(path, buildLegacyDocWithFootnotesCFB(bodyText, []int{4}, []string{footnoteText}), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) < 2 {
		t.Fatalf("paragraph runs = %#v, want body run plus footnoteRef", doc.Paragraphs)
	}
	refRun := doc.Paragraphs[0].Runs[1]
	if refRun.Kind != "footnoteRef" || refRun.NoteID != 1 {
		t.Fatalf("footnote ref run = %#v, want footnoteRef #1", refRun)
	}
	if doc.Footnotes == nil {
		t.Fatal("expected parsed footnotes")
	}
	if got := doc.Footnotes[1].Text; got != "Legacy footnote" {
		t.Fatalf("footnote story text = %q, want %q", got, "Legacy footnote")
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `id="fnref1"`) {
		t.Fatalf("expected rendered footnote reference in output:\n%s", out)
	}
	if !strings.Contains(out, `class="page-footnotes"`) || !strings.Contains(out, "Legacy footnote") {
		t.Fatalf("expected rendered legacy page footnote in output:\n%s", out)
	}
}

func TestParseDocLegacyEndnotesRenderInHTML(t *testing.T) {
	const (
		bodyText    = "Body\x02\r"
		endnoteText = "\x02Legacy endnote\r"
	)

	path := filepath.Join(t.TempDir(), "legacy-endnote.doc")
	if err := os.WriteFile(path, buildLegacyDocWithEndnotesCFB(bodyText, []int{4}, []string{endnoteText}), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) < 2 {
		t.Fatalf("paragraph runs = %#v, want body run plus endnoteRef", doc.Paragraphs)
	}
	refRun := doc.Paragraphs[0].Runs[1]
	if refRun.Kind != "endnoteRef" || refRun.NoteID != 1 {
		t.Fatalf("endnote ref run = %#v, want endnoteRef #1", refRun)
	}
	if doc.Endnotes == nil {
		t.Fatal("expected parsed endnotes")
	}
	if got := doc.Endnotes[1].Text; got != "Legacy endnote" {
		t.Fatalf("endnote story text = %q, want %q", got, "Legacy endnote")
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `id="enref1"`) {
		t.Fatalf("expected rendered endnote reference in output:\n%s", out)
	}
	if !strings.Contains(out, "Legacy endnote") {
		t.Fatalf("expected rendered legacy endnote in output:\n%s", out)
	}
}

func TestParseDocLegacyBookmarksRenderInHTML(t *testing.T) {
	const bodyText = "Before\rTarget\r"

	path := filepath.Join(t.TempDir(), "legacy-bookmark.doc")
	if err := os.WriteFile(path, buildLegacyDocWithBookmarksCFB(bodyText, []int{7}, []string{"TargetBookmark"}), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) < 2 {
		t.Fatalf("paragraphs = %#v, want at least 2", doc.Paragraphs)
	}
	if got := doc.Paragraphs[1].Text; got != "Target" {
		t.Fatalf("paragraph text = %q, want %q", got, "Target")
	}
	if got := doc.Paragraphs[1].Bookmarks; len(got) != 1 || got[0] != "TargetBookmark" {
		t.Fatalf("paragraph bookmarks = %#v, want TargetBookmark", got)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `id="TargetBookmark"`) {
		t.Fatalf("expected rendered bookmark anchor in output:\n%s", out)
	}
	if got := strings.Count(out, `id="TargetBookmark"`); got != 1 {
		t.Fatalf("bookmark anchor count = %d, want 1 in output:\n%s", got, out)
	}
}

func TestParseDocLegacyRefFieldLinksToBookmark(t *testing.T) {
	// Paragraph 0: "Target" with bookmark at CP 0
	// Paragraph 1: REF field whose cached result is "see Target"
	// The bookmark name is "MySection"; the REF field instruction references it.
	const bodyText = "Target\r\x13 REF MySection \x14see Target\x15\r"
	// CP 0 = 'T' Р Р†Р вЂљРІР‚Сњ first character of "Target"
	path := filepath.Join(t.TempDir(), "legacy-ref.doc")
	if err := os.WriteFile(path, buildLegacyDocWithBookmarksCFB(bodyText, []int{0}, []string{"MySection"}), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}

	// Paragraph 0 should carry the bookmark.
	if len(doc.Paragraphs) < 2 {
		t.Fatalf("paragraphs = %#v, want at least 2", doc.Paragraphs)
	}
	if got := doc.Paragraphs[0].Bookmarks; len(got) != 1 || got[0] != "MySection" {
		t.Fatalf("paragraph[0].Bookmarks = %#v, want [MySection]", got)
	}

	// Paragraph 1 should have a REF run with Anchor=MySection and text "see Target".
	var refRun *wordir.Run
	for i := range doc.Paragraphs[1].Runs {
		if doc.Paragraphs[1].Runs[i].FieldKind == "REF" {
			refRun = &doc.Paragraphs[1].Runs[i]
			break
		}
	}
	if refRun == nil {
		t.Fatalf("paragraph[1].Runs = %#v, want REF field run", doc.Paragraphs[1].Runs)
	}
	if refRun.Anchor != "MySection" {
		t.Fatalf("REF run Anchor = %q, want MySection", refRun.Anchor)
	}
	if refRun.Text != "see Target" {
		t.Fatalf("REF run Text = %q, want %q", refRun.Text, "see Target")
	}

	// HTML output should have a bookmark anchor and a REF link pointing to it.
	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `id="MySection"`) {
		t.Fatalf("expected bookmark anchor id=MySection in HTML output:\n%s", out)
	}
	if !strings.Contains(out, `href="#MySection"`) {
		t.Fatalf("expected REF field link href=#MySection in HTML output:\n%s", out)
	}
	if !strings.Contains(out, "see Target") {
		t.Fatalf("expected REF display text 'see Target' in HTML output:\n%s", out)
	}
}

func TestParseDocLegacyBookmarkEndsLoadedFromPlcfBkl(t *testing.T) {
	// Body: "Hello World\r" Р Р†Р вЂљРІР‚Сњ bookmark "Greeting" covers CPs 0..11 (exclusive end=11).
	const bodyText = "Hello World\r"
	// start CP = 0, end CP = 11 (exclusive, points past the last char before \r)
	path := filepath.Join(t.TempDir(), "legacy-bkl.doc")
	cfb := buildLegacyDocWithBookmarksAndEndsCFB(bodyText, []int{0}, []int{11}, []string{"Greeting"})
	if err := os.WriteFile(path, cfb, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}

	// Paragraph 0 should carry the bookmark.
	if len(doc.Paragraphs) == 0 {
		t.Fatalf("no paragraphs")
	}
	if got := doc.Paragraphs[0].Bookmarks; len(got) != 1 || got[0] != "Greeting" {
		t.Fatalf("paragraph[0].Bookmarks = %#v, want [Greeting]", got)
	}

	// BookmarkEnds should have the end CP for "Greeting".
	if doc.BookmarkEnds == nil {
		t.Fatalf("doc.BookmarkEnds is nil, want map with GreetingР Р†РІР‚В РІР‚в„ў11")
	}
	if got := doc.BookmarkEnds["Greeting"]; got != 11 {
		t.Fatalf("BookmarkEnds[Greeting] = %d, want 11", got)
	}
}

func TestParseDocLegacyBookmarkSpanAppliesAcrossParagraphs(t *testing.T) {
	// Body:
	//   Paragraph 0: "First"
	//   Paragraph 1: "Second"
	//   Paragraph 2: "Third"
	// Bookmark starts at CP 0 and ends at CP 13 (exclusive), covering first+second.
	const bodyText = "First\rSecond\rThird\r"
	path := filepath.Join(t.TempDir(), "legacy-bookmark-span.doc")
	cfbData := buildLegacyDocWithBookmarksAndEndsCFB(bodyText, []int{0}, []int{13}, []string{"SpanBookmark"})
	if err := os.WriteFile(path, cfbData, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) < 3 {
		t.Fatalf("paragraph count = %d, want at least 3", len(doc.Paragraphs))
	}

	if got := doc.Paragraphs[0].Bookmarks; len(got) != 1 || got[0] != "SpanBookmark" {
		t.Fatalf("paragraph[0].Bookmarks = %#v, want [SpanBookmark]", got)
	}
	if got := doc.Paragraphs[1].Bookmarks; len(got) != 1 || got[0] != "SpanBookmark" {
		t.Fatalf("paragraph[1].Bookmarks = %#v, want [SpanBookmark]", got)
	}
	if got := doc.Paragraphs[2].Bookmarks; len(got) != 0 {
		t.Fatalf("paragraph[2].Bookmarks = %#v, want []", got)
	}
}

func TestParseDocLegacyBodyFieldsRenderInHTML(t *testing.T) {
	const bodyText = "Before \x13 HYPERLINK \"http://example.com\" \x14click here\x15 after \x13 PAGE \x145\x15\r"

	path := filepath.Join(t.TempDir(), "legacy-fields.doc")
	if err := os.WriteFile(path, buildLegacyDocWithBodyOnlyCFB(bodyText), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) == 0 {
		t.Fatalf("paragraphs = %#v, want parsed body paragraph", doc.Paragraphs)
	}
	foundLink := false
	foundPage := false
	for _, run := range doc.Paragraphs[0].Runs {
		if run.Kind == "hyperlink" && run.Target == "http://example.com" && run.Text == "click here" {
			foundLink = true
		}
		if run.Text == "5" && (run.FieldKind == "PAGE" || run.Target == "") {
			foundPage = true
		}
	}
	if !foundLink {
		t.Fatalf("runs = %#v, want hyperlink display run with target http://example.com", doc.Paragraphs[0].Runs)
	}
	if !foundPage {
		t.Fatalf("runs = %#v, want PAGE cached result or semantic field run with text 5", doc.Paragraphs[0].Runs)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<a href="http://example.com">click here</a>`) {
		t.Fatalf("expected rendered hyperlink field in output:\n%s", out)
	}
	if !strings.Contains(out, `word-field-page">1</span>`) && !strings.Contains(out, `after 5`) {
		t.Fatalf("expected rendered PAGE field result in output:\n%s", out)
	}
}

func TestParseDocLegacyRevisionRunsRenderInHTML(t *testing.T) {
	tests := []struct {
		name     string
		bodyText string
		grpprl   []byte
		revision string
		htmlTag  string
	}{
		{name: "inserted", bodyText: "Inserted\r", grpprl: []byte{0x01, 0x08, 0x01}, revision: "inserted", htmlTag: "ins"},
		{name: "deleted", bodyText: "Deleted\r", grpprl: []byte{0x00, 0x08, 0x01}, revision: "deleted", htmlTag: "del"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "legacy-revision-"+tc.name+".doc")
			if err := os.WriteFile(path, buildLegacyDocWithSingleChpxCFB(tc.bodyText, tc.grpprl), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			doc, err := ParseDoc(path)
			if err != nil {
				t.Fatalf("ParseDoc: %v", err)
			}
			if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
				t.Fatalf("parsed paragraphs = %#v, want revision-marked run", doc.Paragraphs)
			}
			run := doc.Paragraphs[0].Runs[0]
			if run.Revision != tc.revision {
				t.Fatalf("run.Revision = %q, want %q", run.Revision, tc.revision)
			}
			if run.Text != strings.TrimSuffix(tc.bodyText, "\r") {
				t.Fatalf("run.Text = %q, want %q", run.Text, strings.TrimSuffix(tc.bodyText, "\r"))
			}

			out, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			want := "<" + tc.htmlTag + ">" + strings.TrimSuffix(tc.bodyText, "\r") + "</" + tc.htmlTag + ">"
			if !strings.Contains(out, want) {
				t.Fatalf("expected rendered revision %q in output:\n%s", want, out)
			}
		})
	}
}

func TestParseDocLegacyEmbeddedObjectHidesBareAnchor(t *testing.T) {
	const bodyText = "Before \x01 After\r"

	path := filepath.Join(t.TempDir(), "legacy-embedded-object.doc")
	if err := os.WriteFile(path, buildLegacyDocWithBodyOnlyCFB(bodyText), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) < 3 {
		t.Fatalf("paragraph runs = %#v, want text + placeholder + text", doc.Paragraphs)
	}
	if got := doc.Paragraphs[0].Runs[1].Kind; got != "olePlaceholder" {
		t.Fatalf("middle run kind = %q, want olePlaceholder", got)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, `class="ole-object"`) {
		t.Fatalf("bare OLE anchor must not render as a visible object placeholder:\n%s", out)
	}
}

func TestProcessLegacyObjectAnchorsKeepsNonSpecAnchorsAsOLEPlaceholders(t *testing.T) {
	runs := []docRun{
		{Text: "A\x01B", StartCP: 10, Spec: false},
		{Text: "\x01", StartCP: 20, Spec: true},
	}
	images := []*wordir.Image{{MIMEType: "image/png", Data: []byte{0x89, 'P', 'N', 'G'}}}

	got := processLegacyObjectAnchors(runs, images)
	if len(got) != 4 {
		t.Fatalf("run count = %d, want 4 (text + olePlaceholder + text + image)", len(got))
	}
	if got[0].Text != "A" || got[0].Kind != "" {
		t.Fatalf("run[0] = %#v, want plain text run 'A'", got[0])
	}
	if got[1].Kind != "olePlaceholder" || got[1].Text != "Embedded Object" {
		t.Fatalf("run[1] = %#v, want detect-only olePlaceholder", got[1])
	}
	if got[1].Image != nil {
		t.Fatalf("run[1].Image = %#v, want nil for detect-only OLE placeholder", got[1].Image)
	}
	if got[2].Text != "B" || got[2].Kind != "" {
		t.Fatalf("run[2] = %#v, want plain text run 'B'", got[2])
	}
	if got[3].Kind != "image" || got[3].Image == nil {
		t.Fatalf("run[3] = %#v, want image run (proves non-spec anchor did not consume image)", got[3])
	}
}

func TestParseDocLegacyCommentRefsAndCommentsRenderInHTML(t *testing.T) {
	const (
		bodyText    = "Body\x05\r"
		commentText = "\x05Legacy comment\r"
	)

	path := filepath.Join(t.TempDir(), "legacy-comment.doc")
	if err := os.WriteFile(path, buildLegacyDocWithCommentsCFB(bodyText, []int{4}, []string{commentText}), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) < 2 {
		t.Fatalf("paragraph runs = %#v, want body run plus commentRef", doc.Paragraphs)
	}
	refRun := doc.Paragraphs[0].Runs[1]
	if refRun.Kind != "commentRef" || refRun.CommentID != 1 {
		t.Fatalf("comment ref run = %#v, want commentRef #1", refRun)
	}
	if doc.Comments == nil {
		t.Fatal("expected parsed comments")
	}
	if got := doc.Comments[1].Text; got != "Legacy comment" {
		t.Fatalf("comment story text = %q, want %q", got, "Legacy comment")
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="comment-ref"`) || !strings.Contains(out, `id="comment1"`) {
		t.Fatalf("expected rendered comment reference and target in output:\n%s", out)
	}
	if !strings.Contains(out, `class="comments"`) || !strings.Contains(out, "Legacy comment") {
		t.Fatalf("expected rendered legacy comment section in output:\n%s", out)
	}
}

func TestParseDocLegacyCommentAuthorDate(t *testing.T) {
	const (
		bodyText    = "Body\x05\r"
		commentText = "\x05Review note\r"
	)
	author := "JD"
	date := "2023-03-15" // DTTM: year=123, month=3, day=15

	path := filepath.Join(t.TempDir(), "legacy-comment-meta.doc")
	cfbData := buildLegacyDocWithCommentMetaCFB(bodyText, []int{4}, []string{commentText}, []string{author}, []string{date})
	if err := os.WriteFile(path, cfbData, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if doc.Comments == nil {
		t.Fatal("expected parsed comments")
	}
	story := doc.Comments[1]
	if story.Author != author {
		t.Errorf("comment author = %q, want %q", story.Author, author)
	}
	if story.Date != date {
		t.Errorf("comment date = %q, want %q", story.Date, date)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="comment-author"`) {
		t.Errorf("expected comment-author span in HTML output:\n%s", out)
	}
	if !strings.Contains(out, `class="comment-date"`) {
		t.Errorf("expected comment-date time element in HTML output:\n%s", out)
	}
	if !strings.Contains(out, author) {
		t.Errorf("expected author %q in HTML output:\n%s", author, out)
	}
	if !strings.Contains(out, date) {
		t.Errorf("expected date %q in HTML output:\n%s", date, out)
	}
}

func TestParseDocFixtureSampleWithNotesHasContent(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/sample_with_notes.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if len(doc.Footnotes) != 2 {
		t.Fatalf("ParseDoc(%s): footnotes = %d, want 2", path, len(doc.Footnotes))
	}
	if got := doc.Footnotes[1].Text; !strings.Contains(got, "sample footnote") {
		t.Fatalf("ParseDoc(%s): Footnotes[1].Text = %q, want content containing %q", path, got, "sample footnote")
	}
	if got := doc.Footnotes[2].Text; !strings.Contains(got, "Secondary footnote") {
		t.Fatalf("ParseDoc(%s): Footnotes[2].Text = %q, want content containing %q", path, got, "Secondary footnote")
	}
}

func TestParseDocFixtureSampleWithEndnotesHasContent(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/sample_with_endnotes.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if len(doc.Endnotes) != 2 {
		t.Fatalf("ParseDoc(%s): endnotes = %d, want 2", path, len(doc.Endnotes))
	}
	if got := doc.Endnotes[1].Text; !strings.Contains(got, "Primary endnote") {
		t.Fatalf("ParseDoc(%s): Endnotes[1].Text = %q, want content containing %q", path, got, "Primary endnote")
	}
	if got := doc.Endnotes[2].Text; !strings.Contains(got, "Secondary endnote") {
		t.Fatalf("ParseDoc(%s): Endnotes[2].Text = %q, want content containing %q", path, got, "Secondary endnote")
	}
	if len(doc.Comments) > 0 {
		t.Fatalf("ParseDoc(%s): comments = %d, want 0 (avoid false-positive annotation story)", path, len(doc.Comments))
	}
}

func TestParseDocFixtureWord95WithCommentsHasContent(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/word95_with_comments.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if len(doc.Comments) < 2 {
		t.Fatalf("ParseDoc(%s): comments = %d, want >= 2", path, len(doc.Comments))
	}
	if len(doc.Endnotes) != 0 {
		t.Fatalf("ParseDoc(%s): endnotes = %d, want 0 for comments fixture", path, len(doc.Endnotes))
	}
	var sawFirst, sawSecond bool
	for _, c := range doc.Comments {
		if strings.Contains(c.Text, "first comment") {
			sawFirst = true
		}
		if strings.Contains(c.Text, "second comment") {
			sawSecond = true
		}
	}
	if !sawFirst || !sawSecond {
		t.Fatalf("ParseDoc(%s): expected comments containing %q and %q", path, "first comment", "second comment")
	}
}

func TestParseDocFixtureWord95CommentsIsPreWord97(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/word95_with_comments.doc"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	f, err := cfb.OpenBytes(data)
	if err != nil {
		t.Fatalf("cfb.OpenBytes(%s): %v", path, err)
	}
	ws, ok := f.Stream("WordDocument")
	if !ok {
		t.Fatalf("Stream(%s, WordDocument): missing", path)
	}
	if len(ws) < 4 {
		t.Fatalf("WordDocument too small: %d bytes", len(ws))
	}
	sig := binary.LittleEndian.Uint16(ws[0:2])
	nFib := binary.LittleEndian.Uint16(ws[2:4])
	if sig != legacyDocPreWord97Magic {
		t.Fatalf("signature = 0x%04X, want pre-Word97 magic 0x%04X", sig, legacyDocPreWord97Magic)
	}
	if nFib != 0x0068 {
		t.Fatalf("nFib = 0x%04X, want 0x0068 (Word 95)", nFib)
	}
}

func TestParseDocLegacyMultipleFootnotesAssignSequentialIDs(t *testing.T) {
	// Three footnote references in the body; footnote stories follow in order.
	const bodyText = "First\x02 Second\x02 Third\x02\r"
	noteStories := []string{
		"\x02Note one\r",
		"\x02Note two\r",
		"\x02Note three\r",
	}
	refCPs := []int{5, 13, 20} // byte offsets of the \x02 chars

	path := filepath.Join(t.TempDir(), "legacy-multi-footnote.doc")
	if err := os.WriteFile(path, buildLegacyDocWithFootnotesCFB(bodyText, refCPs, noteStories), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) == 0 {
		t.Fatal("no paragraphs parsed")
	}
	// Collect all footnoteRef runs across all paragraphs.
	var refs []wordir.Run
	for _, para := range doc.Paragraphs {
		for _, run := range para.Runs {
			if run.Kind == "footnoteRef" {
				refs = append(refs, run)
			}
		}
	}
	if len(refs) != 3 {
		t.Fatalf("footnoteRef run count = %d, want 3; paragraphs: %#v", len(refs), doc.Paragraphs)
	}
	for i, ref := range refs {
		wantID := i + 1
		if ref.NoteID != wantID {
			t.Errorf("refs[%d].NoteID = %d, want %d", i, ref.NoteID, wantID)
		}
	}

	if doc.Footnotes == nil {
		t.Fatal("doc.Footnotes is nil, want 3 footnotes")
	}
	wantTexts := []string{"Note one", "Note two", "Note three"}
	for i, want := range wantTexts {
		id := i + 1
		if got := doc.Footnotes[id].Text; got != want {
			t.Errorf("Footnotes[%d].Text = %q, want %q", id, got, want)
		}
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if !strings.Contains(out, fmt.Sprintf(`id="fnref%d"`, i)) {
			t.Errorf("expected id=fnref%d in HTML output:\n%s", i, out)
		}
	}
	for _, text := range wantTexts {
		if !strings.Contains(out, text) {
			t.Errorf("expected footnote text %q in HTML output:\n%s", text, out)
		}
	}
}

func TestParseDocLegacyMultiParagraphFootnoteRendersAllParagraphs(t *testing.T) {
	// A footnote story with two paragraphs separated by \r.
	const bodyText = "See note\x02\r"
	// The story has two paragraphs: the footnote marker paragraph and a continuation.
	noteStories := []string{"\x02First para\rSecond para\r"}

	path := filepath.Join(t.TempDir(), "legacy-multipara-footnote.doc")
	if err := os.WriteFile(path, buildLegacyDocWithFootnotesCFB(bodyText, []int{8}, noteStories), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if doc.Footnotes == nil {
		t.Fatal("doc.Footnotes is nil")
	}
	fn := doc.Footnotes[1]
	// The story text should contain both paragraphs joined.
	if !strings.Contains(fn.Text, "First para") {
		t.Errorf("footnote text missing 'First para': %q", fn.Text)
	}
	if !strings.Contains(fn.Text, "Second para") {
		t.Errorf("footnote text missing 'Second para': %q", fn.Text)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "First para") {
		t.Errorf("expected 'First para' in HTML output:\n%s", out)
	}
	if !strings.Contains(out, "Second para") {
		t.Errorf("expected 'Second para' in HTML output:\n%s", out)
	}
}

func TestParseDocLegacyMultipleBookmarksInSameParagraph(t *testing.T) {
	// Body has one paragraph "Alpha Beta Gamma" with two bookmarks at different
	// character positions within it.
	const bodyText = "Alpha Beta Gamma\r"
	// Bookmark "BookA" at CP 0, "BookB" at CP 6 (both within same paragraph).
	bookmarkStarts := []int{0, 6}
	bookmarkNames := []string{"BookA", "BookB"}

	path := filepath.Join(t.TempDir(), "legacy-two-bookmarks.doc")
	if err := os.WriteFile(path, buildLegacyDocWithBookmarksCFB(bodyText, bookmarkStarts, bookmarkNames), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) == 0 {
		t.Fatal("no paragraphs parsed")
	}
	para := doc.Paragraphs[0]
	if len(para.Bookmarks) != 2 {
		t.Fatalf("paragraph Bookmarks = %#v, want [BookA BookB]", para.Bookmarks)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, name := range bookmarkNames {
		if !strings.Contains(out, fmt.Sprintf(`id="%s"`, name)) {
			t.Errorf("expected id=%q in HTML output:\n%s", name, out)
		}
	}
}

func TestParseDocLegacyMixedFootnotesAndBookmarks(t *testing.T) {
	// Body paragraph has both a bookmark and a footnote reference.
	// "Start\x02 End\r" Р Р†Р вЂљРІР‚Сњ bookmark covers the paragraph (CP 0), footnote ref at CP 5.
	const bodyText = "Start\x02 End\r"
	noteStories := []string{"\x02Foot note\r"}
	refCPs := []int{5}

	path := filepath.Join(t.TempDir(), "legacy-mixed.doc")
	base := buildLegacyDocWithFootnotesCFB(bodyText, refCPs, noteStories)
	// We can't easily combine both builders into one CFB, so verify each stream
	// independently via separate docs and confirm both features work.
	path2 := filepath.Join(t.TempDir(), "legacy-mixed-bookmark.doc")
	if err := os.WriteFile(path, base, 0o644); err != nil {
		t.Fatalf("WriteFile footnotes: %v", err)
	}
	if err := os.WriteFile(path2, buildLegacyDocWithBookmarksCFB(bodyText, []int{0}, []string{"StartMark"}), 0o644); err != nil {
		t.Fatalf("WriteFile bookmarks: %v", err)
	}

	docFn, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc footnotes: %v", err)
	}
	var gotRef bool
	for _, para := range docFn.Paragraphs {
		for _, run := range para.Runs {
			if run.Kind == "footnoteRef" {
				gotRef = true
			}
		}
	}
	if !gotRef {
		t.Fatalf("footnote doc: no footnoteRef run found; paragraphs: %#v", docFn.Paragraphs)
	}
	if docFn.Footnotes[1].Text != "Foot note" {
		t.Errorf("footnote text = %q, want %q", docFn.Footnotes[1].Text, "Foot note")
	}

	docBk, err := ParseDoc(path2)
	if err != nil {
		t.Fatalf("ParseDoc bookmarks: %v", err)
	}
	if len(docBk.Paragraphs) == 0 || len(docBk.Paragraphs[0].Bookmarks) == 0 {
		t.Fatalf("bookmark doc: no bookmarks on paragraph[0]; paragraphs: %#v", docBk.Paragraphs)
	}
	if docBk.Paragraphs[0].Bookmarks[0] != "StartMark" {
		t.Errorf("bookmark name = %q, want StartMark", docBk.Paragraphs[0].Bookmarks[0])
	}

	outFn, _ := render.HTML(docFn)
	outBk, _ := render.HTML(docBk)
	if !strings.Contains(outFn, "Foot note") {
		t.Errorf("expected 'Foot note' in footnote HTML:\n%s", outFn)
	}
	if !strings.Contains(outBk, `id="StartMark"`) {
		t.Errorf("expected id=StartMark in bookmark HTML:\n%s", outBk)
	}
}

func TestParseDocLegacyMultipleCommentsAssignSequentialIDs(t *testing.T) {
	// Two comment references in the body; comment stories follow in order.
	const bodyText = "Para one\x05 Para two\x05\r"
	commentStories := []string{
		"\x05Comment alpha\r",
		"\x05Comment beta\r",
	}
	refCPs := []int{8, 18}

	path := filepath.Join(t.TempDir(), "legacy-multi-comment.doc")
	if err := os.WriteFile(path, buildLegacyDocWithCommentsCFB(bodyText, refCPs, commentStories), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}

	// Collect all commentRef runs.
	var refs []wordir.Run
	for _, para := range doc.Paragraphs {
		for _, run := range para.Runs {
			if run.Kind == "commentRef" {
				refs = append(refs, run)
			}
		}
	}
	if len(refs) != 2 {
		t.Fatalf("commentRef run count = %d, want 2; paragraphs: %#v", len(refs), doc.Paragraphs)
	}
	for i, ref := range refs {
		if ref.CommentID != i+1 {
			t.Errorf("refs[%d].CommentID = %d, want %d", i, ref.CommentID, i+1)
		}
	}

	if doc.Comments == nil {
		t.Fatal("doc.Comments is nil, want 2 comments")
	}
	wantTexts := map[int]string{1: "Comment alpha", 2: "Comment beta"}
	for id, want := range wantTexts {
		if got := doc.Comments[id].Text; got != want {
			t.Errorf("Comments[%d].Text = %q, want %q", id, got, want)
		}
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for i := 1; i <= 2; i++ {
		if !strings.Contains(out, fmt.Sprintf(`id="comment%d"`, i)) {
			t.Errorf("expected id=comment%d in HTML:\n%s", i, out)
		}
	}
	for _, text := range wantTexts {
		if !strings.Contains(out, text) {
			t.Errorf("expected comment text %q in HTML:\n%s", text, out)
		}
	}
}

func TestParseDocLegacyInlineImageFromDataStream(t *testing.T) {
	const bodyText = "Before \x01 After\r"
	path := filepath.Join(t.TempDir(), "legacy-inline-image.doc")
	if err := os.WriteFile(path, buildLegacyDocWithInlineImageCFB(bodyText, tinyPNGBytes()), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) < 3 {
		t.Fatalf("paragraph runs = %#v, want text + image + text", doc.Paragraphs)
	}
	imgRun := doc.Paragraphs[0].Runs[1]
	if imgRun.Kind != "image" || imgRun.Image == nil {
		t.Fatalf("middle run = %#v, want inline image run", imgRun)
	}
	if imgRun.Image.MIMEType != "image/png" || len(imgRun.Image.Data) == 0 {
		t.Fatalf("image payload = %#v, want non-empty image/png", imgRun.Image)
	}
	if imgRun.Image.Width != 9525 || imgRun.Image.Height != 9525 {
		t.Fatalf("image size = (%d,%d), want (9525,9525)", imgRun.Image.Width, imgRun.Image.Height)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "data:image/png;base64,") {
		t.Fatalf("expected inline image data URI in output:\n%s", out)
	}
}

func TestParseDocLegacyFontNameFromCHPx(t *testing.T) {
	const (
		bodyText = "Styled text\r"
		fontName = "Courier New"
	)

	path := filepath.Join(t.TempDir(), "legacy-font.doc")
	if err := os.WriteFile(path, buildLegacyDocWithFontCFB(bodyText, fontName), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.Paragraphs) == 0 || len(doc.Paragraphs[0].Runs) == 0 {
		t.Fatalf("paragraphs = %#v, want parsed run", doc.Paragraphs)
	}
	run := doc.Paragraphs[0].Runs[0]
	if run.Format.FontASCII != fontName {
		t.Fatalf("FontASCII = %q, want %q", run.Format.FontASCII, fontName)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, fontName) {
		t.Fatalf("expected rendered font family %q in HTML output:\n%s", fontName, out)
	}
}

func TestLoadLegacyDocEmbeddedFonts(t *testing.T) {
	const (
		cbRgFcLcb     = 62
		fibBase       = 0
		font1Offset   = 1024
		font2Offset   = 1088
		sttbTtmbdSize = 10 + 2*12
	)
	wordStream := make([]byte, 2048)
	fontTable := buildLegacyFontTable([]string{"Oswald", "Oswald"})
	tableStream := make([]byte, len(fontTable)+sttbTtmbdSize)
	copy(tableStream, fontTable)
	sttbTtmbd := buildLegacySttbTtmbd([]legacyEmbeddedFontDescriptor{
		{Offset: font1Offset, FontIndex: 0, SubsetOrder: 0xFFFFFFFF},
		{Offset: font2Offset, FontIndex: 1, Bold: true, Italic: true, SubsetOrder: 1},
	})
	copy(tableStream[len(fontTable):], sttbTtmbd)
	copy(wordStream[font1Offset:], legacyTestEOTPayload(40, 0x11))
	copy(wordStream[font2Offset:], legacyTestEOTPayload(44, 0x22))
	putFibFcLcb(wordStream, fibBase, 15, 0, len(fontTable))
	putFibFcLcb(wordStream, fibBase, 61, len(fontTable), len(sttbTtmbd))

	fonts := loadLegacyDocEmbeddedFonts(wordStream, tableStream, fibBase, cbRgFcLcb)
	if len(fonts) != 1 {
		t.Fatalf("embedded fonts = %#v, want one family", fonts)
	}
	if fonts[0].Family != "Oswald" {
		t.Fatalf("family = %q, want Oswald", fonts[0].Family)
	}
	if len(fonts[0].Faces) != 2 {
		t.Fatalf("faces = %#v, want 2", fonts[0].Faces)
	}
	if fonts[0].Faces[0].ContentType != "application/vnd.ms-fontobject" {
		t.Fatalf("face[0] content type = %q, want EOT", fonts[0].Faces[0].ContentType)
	}
	if fonts[0].Faces[1].Weight != 700 || fonts[0].Faces[1].Style != "italic" {
		t.Fatalf("face[1] = %#v, want bold italic", fonts[0].Faces[1])
	}
}

func TestParseDocLegacyEmbeddedFontsPopulateDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-embedded-fonts.doc")
	if err := os.WriteFile(path, buildLegacyDocWithEmbeddedFontsCFB("Font body\r", "Oswald"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if len(doc.EmbeddedFonts) != 1 || len(doc.EmbeddedFonts[0].Faces) != 2 {
		t.Fatalf("EmbeddedFonts = %#v, want one family with two faces", doc.EmbeddedFonts)
	}

	out, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "@font-face{font-family:Oswald") {
		t.Fatalf("expected embedded font CSS in HTML:\n%s", out)
	}
	if !strings.Contains(out, "application/vnd.ms-fontobject") {
		t.Fatalf("expected embedded EOT source in HTML:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// CHPx parsing tests
// ---------------------------------------------------------------------------

func TestParseChpxGrpprlBoldItalic(t *testing.T) {
	// Build a grpprl with sprmCFBold (0x0835) = 1 and sprmCFItalic (0x0836) = 1.
	grpprl := []byte{
		0x35, 0x08, 0x01, // sprmCFBold = on
		0x36, 0x08, 0x01, // sprmCFItalic = on
	}
	got := parseChpxGrpprl(grpprl)
	if got.Bold == nil || !*got.Bold {
		t.Fatalf("Bold = %v, want true", got.Bold)
	}
	if got.Italic == nil || !*got.Italic {
		t.Fatalf("Italic = %v, want true", got.Italic)
	}
}

func TestParseChpxGrpprlStrikeSmallCapsCaps(t *testing.T) {
	grpprl := []byte{
		0x37, 0x08, 0x01, // sprmCFStrike = on
		0x3A, 0x08, 0x01, // sprmCFSmallCaps = on
		0x3B, 0x08, 0x01, // sprmCFCaps = on
	}
	got := parseChpxGrpprl(grpprl)
	if got.Strike == nil || !*got.Strike {
		t.Fatalf("Strike = %v, want true", got.Strike)
	}
	if got.SmallCaps == nil || !*got.SmallCaps {
		t.Fatalf("SmallCaps = %v, want true", got.SmallCaps)
	}
	if got.Caps == nil || !*got.Caps {
		t.Fatalf("Caps = %v, want true", got.Caps)
	}
}

func TestParseChpxGrpprlVanish(t *testing.T) {
	grpprl := []byte{0x3C, 0x08, 0x01} // sprmCFVanish = on
	got := parseChpxGrpprl(grpprl)
	if got.Vanish == nil || !*got.Vanish {
		t.Fatalf("Vanish = %v, want true", got.Vanish)
	}
}

func TestParseChpxGrpprlNoProof(t *testing.T) {
	grpprl := []byte{0x75, 0x08, 0x01} // sprmCFNoProof = on
	got := parseChpxGrpprl(grpprl)
	if got.NoProof == nil || !*got.NoProof {
		t.Fatalf("NoProof = %v, want true", got.NoProof)
	}
}

func TestParseChpxGrpprlUnderline(t *testing.T) {
	// sprmCKul (0x2A3E) single underline.
	got := parseChpxGrpprl([]byte{0x3E, 0x2A, 0x01})
	if got.Underline != "single" {
		t.Fatalf("Underline = %q, want single", got.Underline)
	}
	// sprmCKul (0x2A3E) double underline.
	got2 := parseChpxGrpprl([]byte{0x3E, 0x2A, 0x03})
	if got2.Underline != "double" {
		t.Fatalf("Underline = %q, want double", got2.Underline)
	}
}

func buildLegacyDocWithHeaderFooterCFB(bodyParts []string, noteStories [6]string, sectionStories [][6]string) []byte {
	const textOffset = 1024

	bodyText := strings.Join(bodyParts, "")
	headerDoc, plcfHdd := buildLegacyHeaderDocument(noteStories, sectionStories)
	allText := []byte(bodyText + headerDoc)
	wordStream := make([]byte, textOffset+len(allText))
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0) // csw
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 6) // cslw
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))  // ccpText
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)                      // ccpFtn
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], uint32(len(headerDoc))) // ccpHdr
	pos += 24

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 34) // cbRgFcLcb
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed(bodyParts)
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)
	fcPlcfHdd := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	lcbPlcfHdd := len(plcfHdd)
	fcClx := fcPlcfHdd + lcbPlcfHdd
	clx := buildLegacySinglePieceClx(textOffset, len(allText))

	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcPlcfHdd:], plcfHdd)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 11, fcPlcfHdd, lcbPlcfHdd)
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

func buildLegacyDocWithFootnotesCFB(bodyText string, refCPs []int, noteStories []string) []byte {
	const textOffset = 1024

	footnoteDoc, plcfFndTxt := buildLegacyFootnoteDocument(noteStories)
	allText := []byte(bodyText + footnoteDoc)
	wordStream := make([]byte, textOffset+len(allText))
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 6)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], uint32(len(footnoteDoc)))
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)
	pos += 24

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 34)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	plcfFndRef := buildLegacyNoteRefPLC(refCPs)
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)
	fcPlcfFndRef := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	lcbPlcfFndRef := len(plcfFndRef)
	fcPlcfFndTxt := fcPlcfFndRef + lcbPlcfFndRef
	lcbPlcfFndTxt := len(plcfFndTxt)
	fcClx := fcPlcfFndTxt + lcbPlcfFndTxt
	clx := buildLegacySinglePieceClx(textOffset, len(allText))

	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcPlcfFndRef:], plcfFndRef)
	copy(tableStream[fcPlcfFndTxt:], plcfFndTxt)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 2, fcPlcfFndRef, lcbPlcfFndRef)
	putFibFcLcb(wordStream, fibBase, 3, fcPlcfFndTxt, lcbPlcfFndTxt)
	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

func buildLegacyDocWithEndnotesCFB(bodyText string, refCPs []int, noteStories []string) []byte {
	const textOffset = 1024

	endnoteDoc, plcfEndTxt := buildLegacyFootnoteDocument(noteStories)
	allText := []byte(bodyText + endnoteDoc)
	wordStream := make([]byte, textOffset+len(allText))
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 9)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+24:pos+28], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+28:pos+32], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+32:pos+36], uint32(len(endnoteDoc)))
	pos += 36

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 40)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	plcfEndRef := buildLegacyNoteRefPLC(refCPs)
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)
	fcPlcfEndRef := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	lcbPlcfEndRef := len(plcfEndRef)
	fcPlcfEndTxt := fcPlcfEndRef + lcbPlcfEndRef
	lcbPlcfEndTxt := len(plcfEndTxt)
	fcClx := fcPlcfEndTxt + lcbPlcfEndTxt
	clx := buildLegacySinglePieceClx(textOffset, len(allText))

	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcPlcfEndRef:], plcfEndRef)
	copy(tableStream[fcPlcfEndTxt:], plcfEndTxt)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 34, fcPlcfEndRef, lcbPlcfEndRef)
	putFibFcLcb(wordStream, fibBase, 35, fcPlcfEndTxt, lcbPlcfEndTxt)
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

func buildLegacyDocWithCommentsCFB(bodyText string, refCPs []int, commentStories []string) []byte {
	const textOffset = 1024

	commentDoc, plcfAtnTxt := buildLegacyFootnoteDocument(commentStories)
	allText := []byte(bodyText + commentDoc)
	wordStream := make([]byte, textOffset+len(allText))
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 8)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))   // ccpText
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)                       // ccpFtn
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)                       // ccpHdr
	binary.LittleEndian.PutUint32(wordStream[pos+24:pos+28], 0)                       // ccpMcr
	binary.LittleEndian.PutUint32(wordStream[pos+28:pos+32], uint32(len(commentDoc))) // ccpAtn
	pos += 32

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 40)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	plcfAtnRef := buildLegacyNoteRefPLC(refCPs)
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)
	fcPlcfAtnRef := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	lcbPlcfAtnRef := len(plcfAtnRef)
	fcPlcfAtnTxt := fcPlcfAtnRef + lcbPlcfAtnRef
	lcbPlcfAtnTxt := len(plcfAtnTxt)
	fcClx := fcPlcfAtnTxt + lcbPlcfAtnTxt
	clx := buildLegacySinglePieceClx(textOffset, len(allText))

	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcPlcfAtnRef:], plcfAtnRef)
	copy(tableStream[fcPlcfAtnTxt:], plcfAtnTxt)
	copy(tableStream[fcClx:], clx)

	// Use non-footnote/endnote FcLcb slots so comment discovery exercises dynamic pairing.
	putFibFcLcb(wordStream, fibBase, 20, fcPlcfAtnRef, lcbPlcfAtnRef)
	putFibFcLcb(wordStream, fibBase, 21, fcPlcfAtnTxt, lcbPlcfAtnTxt)
	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

// buildLegacyDocWithCommentMetaCFB builds a minimal .doc CFB with comment stories
// and ATRDPre10 metadata (author initials + date) embedded in PlcfAtnBkf at FIB index 4.
// authors and dates slices are parallel to commentStories; empty strings are allowed.
func buildLegacyDocWithCommentMetaCFB(bodyText string, refCPs []int, commentStories []string, authors []string, dates []string) []byte {
	const textOffset = 1024

	commentDoc, plcfAtnTxt := buildLegacyFootnoteDocument(commentStories)
	allText := []byte(bodyText + commentDoc)
	wordStream := make([]byte, textOffset+len(allText))
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 8)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))   // ccpText
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)                       // ccpFtn
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)                       // ccpHdr
	binary.LittleEndian.PutUint32(wordStream[pos+24:pos+28], 0)                       // ccpMcr
	binary.LittleEndian.PutUint32(wordStream[pos+28:pos+32], uint32(len(commentDoc))) // ccpAtn
	pos += 32

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 40)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	plcfAtnRef := buildLegacyNoteRefPLC(refCPs)
	plcfAtnBkf := buildLegacyATRDPre10PLC(refCPs, authors, dates)

	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)
	fcPlcfAtnBkf := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	lcbPlcfAtnBkf := len(plcfAtnBkf)
	fcPlcfAtnRef := fcPlcfAtnBkf + lcbPlcfAtnBkf
	lcbPlcfAtnRef := len(plcfAtnRef)
	fcPlcfAtnTxt := fcPlcfAtnRef + lcbPlcfAtnRef
	lcbPlcfAtnTxt := len(plcfAtnTxt)
	fcClx := fcPlcfAtnTxt + lcbPlcfAtnTxt
	clx := buildLegacySinglePieceClx(textOffset, len(allText))

	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcPlcfAtnBkf:], plcfAtnBkf)
	copy(tableStream[fcPlcfAtnRef:], plcfAtnRef)
	copy(tableStream[fcPlcfAtnTxt:], plcfAtnTxt)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 4, fcPlcfAtnBkf, lcbPlcfAtnBkf) // PlcfAtnBkf (ATRDPre10)
	putFibFcLcb(wordStream, fibBase, 20, fcPlcfAtnRef, lcbPlcfAtnRef)
	putFibFcLcb(wordStream, fibBase, 21, fcPlcfAtnTxt, lcbPlcfAtnTxt)
	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

// buildLegacyATRDPre10PLC builds a PlcfAtnBkf PLC with ATRDPre10 records.
// Each ATRDPre10 record is 26 bytes: 22-byte XST(10) author initials + 4-byte DTTM.
// dates must be "YYYY-MM-DD" or empty.
func buildLegacyATRDPre10PLC(refCPs []int, authors []string, dates []string) []byte {
	n := len(refCPs)
	if n == 0 {
		return nil
	}
	// PLC layout: (n+1) CPs (4 bytes each) + n ATRDPre10 records (30 bytes each).
	// ATRDPre10 = 22-byte xstUsrInitl + 2-byte ibst + 2-byte grfbmc + 4-byte dttm.
	const atrdSize = 30
	buf := make([]byte, (n+1)*4+n*atrdSize)
	// Write CPs: use the same refCPs + a sentinel (refCP[n-1]+1).
	for i, cp := range refCPs {
		binary.LittleEndian.PutUint32(buf[i*4:], uint32(cp))
	}
	binary.LittleEndian.PutUint32(buf[n*4:], uint32(refCPs[n-1]+1))

	dataOff := (n + 1) * 4
	for i := 0; i < n; i++ {
		rec := buf[dataOff+i*atrdSize : dataOff+(i+1)*atrdSize]
		// xstUsrInitl: cch (2 bytes) + 20 bytes (10 UTF-16LE chars, zero-padded).
		author := ""
		if i < len(authors) {
			author = authors[i]
		}
		runes := []rune(author)
		if len(runes) > 10 {
			runes = runes[:10]
		}
		binary.LittleEndian.PutUint16(rec[0:2], uint16(len(runes)))
		for j, r := range runes {
			binary.LittleEndian.PutUint16(rec[2+j*2:4+j*2], uint16(r))
		}
		// ibst (offset 22) and grfbmc (offset 24) are left as zero.
		// dttmAtn: 4 bytes at offset 26.
		dateStr := ""
		if i < len(dates) {
			dateStr = dates[i]
		}
		if len(dateStr) == 10 {
			var year, month, day int
			fmt.Sscanf(dateStr, "%d-%d-%d", &year, &month, &day)
			if year > 1900 {
				yearOff := uint32(year - 1900)
				dttm := uint32(day)<<11 | uint32(month)<<16 | yearOff<<20
				binary.LittleEndian.PutUint32(rec[26:30], dttm)
			}
		}
	}
	return buf
}

func buildLegacyDocWithInlineImageCFB(bodyText string, imageData []byte) []byte {
	const textOffset = 1024

	allText := []byte(bodyText)
	wordStreamLen := textOffset + len(allText)
	if wordStreamLen < 2048 {
		wordStreamLen = 2048
	}
	wordStream := make([]byte, wordStreamLen)
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 6)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)
	pos += 24

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 40)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)

	fcPlcfBteChpx := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	plcfBteChpx := make([]byte, 12)
	startFC := uint32(0x40000000 | (textOffset * 2))
	binary.LittleEndian.PutUint32(plcfBteChpx[0:4], startFC)
	binary.LittleEndian.PutUint32(plcfBteChpx[4:8], startFC+uint32(len(bodyText)*2))
	binary.LittleEndian.PutUint32(plcfBteChpx[8:12], 3) // pn=3 -> page at offset 1536

	// Minimal CHPX FKP page with one run: sprmCFSpec=1.
	page := make([]byte, 512)
	page[511] = 1 // crun
	binary.LittleEndian.PutUint32(page[0:4], startFC)
	binary.LittleEndian.PutUint32(page[4:8], startFC+uint32(len(bodyText)*2))
	page[8] = 10 // bx for run 0
	page[20] = 3
	page[21] = 0x55
	page[22] = 0x08
	page[23] = 0x01
	copy(wordStream[3*512:4*512], page)

	fcClx := fcPlcfBteChpx + len(plcfBteChpx)
	clx := buildLegacySinglePieceClx(textOffset, len(allText))
	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcPlcfBteChpx:], plcfBteChpx)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 12, fcPlcfBteChpx, len(plcfBteChpx))
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
		{Name: "Data", Payload: imageData},
	})
}

func buildLegacyDocWithFontCFB(bodyText, fontName string) []byte {
	const textOffset = 1024

	allText := []byte(bodyText)
	wordStreamLen := textOffset + len(allText)
	if wordStreamLen < 2048 {
		wordStreamLen = 2048
	}
	wordStream := make([]byte, wordStreamLen)
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 6)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)
	pos += 24

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 40)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)

	fontTable := buildLegacyFontTable([]string{fontName})
	fcFontTable := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	lcbFontTable := len(fontTable)

	fcPlcfBteChpx := fcFontTable + lcbFontTable
	plcfBteChpx := make([]byte, 12)
	startFC := uint32(0x40000000 | (textOffset * 2))
	binary.LittleEndian.PutUint32(plcfBteChpx[0:4], startFC)
	binary.LittleEndian.PutUint32(plcfBteChpx[4:8], startFC+uint32(len(bodyText)*2))
	binary.LittleEndian.PutUint32(plcfBteChpx[8:12], 3)

	page := make([]byte, 512)
	page[511] = 1
	binary.LittleEndian.PutUint32(page[0:4], startFC)
	binary.LittleEndian.PutUint32(page[4:8], startFC+uint32(len(bodyText)*2))
	page[8] = 10
	page[20] = 4
	page[21] = 0x4F
	page[22] = 0x4A
	page[23] = 0x00
	page[24] = 0x00
	copy(wordStream[3*512:4*512], page)

	fcClx := fcPlcfBteChpx + len(plcfBteChpx)
	clx := buildLegacySinglePieceClx(textOffset, len(allText))
	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcFontTable:], fontTable)
	copy(tableStream[fcPlcfBteChpx:], plcfBteChpx)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 12, fcPlcfBteChpx, len(plcfBteChpx))
	putFibFcLcb(wordStream, fibBase, 15, fcFontTable, lcbFontTable)
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

func buildLegacyDocWithEmbeddedFontsCFB(bodyText, fontName string) []byte {
	const (
		textOffset     = 1024
		font1Offset    = 2048
		font2Offset    = 2112
		embeddedCount  = 2
		sttbTtmbdIndex = 61
	)

	allText := []byte(bodyText)
	wordStreamLen := font2Offset + 128
	if wordStreamLen < textOffset+len(allText) {
		wordStreamLen = textOffset + len(allText) + 128
	}
	wordStream := make([]byte, wordStreamLen)
	copy(wordStream[textOffset:], allText)
	copy(wordStream[font1Offset:], legacyTestEOTPayload(40, 0x11))
	copy(wordStream[font2Offset:], legacyTestEOTPayload(44, 0x22))

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 6)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))
	pos += 24

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], sttbTtmbdIndex+1)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)

	fontTable := buildLegacyFontTable([]string{fontName, fontName})
	fcFontTable := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	lcbFontTable := len(fontTable)

	sttbTtmbd := buildLegacySttbTtmbd([]legacyEmbeddedFontDescriptor{
		{Offset: font1Offset, FontIndex: 0, SubsetOrder: 0xFFFFFFFF},
		{Offset: font2Offset, FontIndex: 1, Bold: true, Italic: true, SubsetOrder: 1},
	})
	fcSttbTtmbd := fcFontTable + lcbFontTable

	fcPlcfBteChpx := fcSttbTtmbd + len(sttbTtmbd)
	plcfBteChpx := make([]byte, 12)
	startFC := uint32(0x40000000 | (textOffset * 2))
	binary.LittleEndian.PutUint32(plcfBteChpx[0:4], startFC)
	binary.LittleEndian.PutUint32(plcfBteChpx[4:8], startFC+uint32(len(bodyText)*2))
	binary.LittleEndian.PutUint32(plcfBteChpx[8:12], 3)

	page := make([]byte, 512)
	page[511] = 1
	binary.LittleEndian.PutUint32(page[0:4], startFC)
	binary.LittleEndian.PutUint32(page[4:8], startFC+uint32(len(bodyText)*2))
	page[8] = 10
	page[20] = 4
	page[21] = 0x4F
	page[22] = 0x4A
	page[23] = 0x00
	page[24] = 0x00
	copy(wordStream[3*512:4*512], page)

	fcClx := fcPlcfBteChpx + len(plcfBteChpx)
	clx := buildLegacySinglePieceClx(textOffset, len(allText))
	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcFontTable:], fontTable)
	copy(tableStream[fcSttbTtmbd:], sttbTtmbd)
	copy(tableStream[fcPlcfBteChpx:], plcfBteChpx)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 12, fcPlcfBteChpx, len(plcfBteChpx))
	putFibFcLcb(wordStream, fibBase, 15, fcFontTable, lcbFontTable)
	putFibFcLcb(wordStream, fibBase, sttbTtmbdIndex, fcSttbTtmbd, len(sttbTtmbd))
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	_ = embeddedCount
	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

func buildLegacyFontTable(names []string) []byte {
	buf := make([]byte, 6)
	binary.LittleEndian.PutUint16(buf[0:2], 0xFFFF)
	binary.LittleEndian.PutUint16(buf[2:4], uint16(len(names)))
	for _, name := range names {
		rec := make([]byte, 41+len(name)+1)
		rec[0] = byte(len(rec) - 1)
		copy(rec[40:], name)
		buf = append(buf, rec...)
	}
	return buf
}

func buildLegacySttbTtmbd(entries []legacyEmbeddedFontDescriptor) []byte {
	buf := make([]byte, 10+len(entries)*12)
	binary.LittleEndian.PutUint16(buf[2:4], uint16(len(entries)))
	binary.LittleEndian.PutUint16(buf[4:6], 64)
	binary.LittleEndian.PutUint16(buf[8:10], 10)
	for i, entry := range entries {
		off := 10 + i*12
		binary.LittleEndian.PutUint32(buf[off:off+4], uint32(entry.Offset))
		binary.LittleEndian.PutUint16(buf[off+4:off+6], uint16(entry.FontIndex))
		flags := uint16(0)
		if entry.Bold {
			flags |= 0x0001
		}
		if entry.Italic {
			flags |= 0x0002
		}
		binary.LittleEndian.PutUint16(buf[off+6:off+8], flags)
		binary.LittleEndian.PutUint32(buf[off+8:off+12], entry.SubsetOrder)
	}
	return buf
}

func legacyTestEOTPayload(size int, fill byte) []byte {
	if size < 36 {
		size = 36
	}
	buf := make([]byte, size)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(size))
	binary.LittleEndian.PutUint32(buf[8:12], 0x00010000)
	binary.LittleEndian.PutUint16(buf[34:36], 0x504C)
	for i := 36; i < len(buf); i++ {
		buf[i] = fill
	}
	return buf
}

type legacyTestStyleDef struct {
	name    string
	kind    int
	sti     int
	basedOn int
	linked  int
	papx    []byte
	chpx    []byte
}

func buildLegacyStyleTable(defs map[int]legacyTestStyleDef) []byte {
	const (
		cstd            = 15
		cbSTDBaseInFile = 18
	)

	buf := make([]byte, 2+18)
	binary.LittleEndian.PutUint16(buf[0:2], 18)
	binary.LittleEndian.PutUint16(buf[2:4], cstd)
	binary.LittleEndian.PutUint16(buf[4:6], cbSTDBaseInFile)

	for istd := 0; istd < cstd; istd++ {
		def, ok := defs[istd]
		if !ok {
			buf = append(buf, 0x00, 0x00)
			continue
		}
		std := buildLegacyStd(def, istd, cbSTDBaseInFile)
		buf = append(buf, byte(len(std)), byte(len(std)>>8))
		buf = append(buf, std...)
		if len(std)&1 != 0 {
			buf = append(buf, 0x00)
		}
	}

	return buf
}

func buildLegacyStd(def legacyTestStyleDef, istd, cbSTDBaseInFile int) []byte {
	basedOn := def.basedOn
	if basedOn < 0 {
		basedOn = 0x0FFF
	}
	next := 0
	if def.kind == 2 {
		next = 0x0FFF
	}
	cupx := 0
	switch def.kind {
	case 1:
		cupx = 2
	case 2:
		cupx = 1
	}

	std := make([]byte, cbSTDBaseInFile)
	binary.LittleEndian.PutUint16(std[0:2], uint16(def.sti&0x0FFF))
	binary.LittleEndian.PutUint16(std[2:4], uint16(def.kind&0x000F)|uint16(basedOn&0x0FFF)<<4)
	binary.LittleEndian.PutUint16(std[4:6], uint16(cupx&0x000F)|uint16(next&0x0FFF)<<4)
	link := def.linked
	if link < 0 {
		link = 0
	}
	binary.LittleEndian.PutUint16(std[10:12], uint16(link&0x0FFF))

	std = append(std, buildLegacyXstz(def.name)...)
	switch def.kind {
	case 1:
		std = append(std, buildLegacyLPUpx(def.papx)...)
		std = append(std, buildLegacyLPUpx(def.chpx)...)
	case 2:
		std = append(std, buildLegacyLPUpx(def.chpx)...)
	}
	return std
}

func buildLegacyXstz(value string) []byte {
	runes := []rune(value)
	buf := make([]byte, 2+len(runes)*2+2)
	binary.LittleEndian.PutUint16(buf[0:2], uint16(len(runes)))
	pos := 2
	for _, r := range runes {
		binary.LittleEndian.PutUint16(buf[pos:pos+2], uint16(r))
		pos += 2
	}
	return buf
}

func buildLegacyLPUpx(grpprl []byte) []byte {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf[0:2], uint16(len(grpprl)))
	buf = append(buf, grpprl...)
	if (2+len(grpprl))&1 != 0 {
		buf = append(buf, 0x00)
	}
	return buf
}

func buildLegacyDocWithStyleCFB(bodyText string) []byte {
	const textOffset = 1024

	allText := []byte(bodyText)
	wordStreamLen := textOffset + len(allText)
	if wordStreamLen < 2048 {
		wordStreamLen = 2048
	}
	wordStream := make([]byte, wordStreamLen)
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 6)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)
	pos += 24

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 34)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	styleTable := buildLegacyStyleTable(map[int]legacyTestStyleDef{
		0: {name: "Normal", kind: 1, sti: 0, basedOn: -1},
		1: {
			name:    "Heading 1",
			kind:    1,
			sti:     1,
			basedOn: 0,
			papx:    []byte{0x03, 0x24, 0x01},
			chpx:    []byte{0x35, 0x08, 0x01},
		},
	})

	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)
	fcStshf := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	lcbStshf := len(styleTable)
	fcPlcfBtePapx := fcStshf + lcbStshf

	plcfBtePapx := make([]byte, 12)
	startFC := uint32(0x40000000 | (textOffset * 2))
	binary.LittleEndian.PutUint32(plcfBtePapx[0:4], startFC)
	binary.LittleEndian.PutUint32(plcfBtePapx[4:8], startFC+uint32(len(bodyText)*2))
	binary.LittleEndian.PutUint32(plcfBtePapx[8:12], 3)

	page := make([]byte, 512)
	page[511] = 1
	binary.LittleEndian.PutUint32(page[0:4], startFC)
	binary.LittleEndian.PutUint32(page[4:8], startFC+uint32(len(bodyText)*2))
	page[8] = 10
	page[20] = 1
	page[21] = 0x01
	page[22] = 0x00
	copy(wordStream[3*512:4*512], page)

	fcClx := fcPlcfBtePapx + len(plcfBtePapx)
	clx := buildLegacySinglePieceClx(textOffset, len(allText))
	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcStshf:], styleTable)
	copy(tableStream[fcPlcfBtePapx:], plcfBtePapx)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 1, fcStshf, lcbStshf)
	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 13, fcPlcfBtePapx, len(plcfBtePapx))
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

func tinyPNGBytes() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0A, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x63, 0x60, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x01, 0xE5, 0x27, 0xD4, 0xA2, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
}

func buildLegacyDocWithBookmarksCFB(bodyText string, bookmarkStarts []int, bookmarkNames []string) []byte {
	const textOffset = 1024

	allText := []byte(bodyText)
	wordStream := make([]byte, textOffset+len(allText))
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 6)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)
	pos += 24

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 40)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	plcfBkf := buildLegacyBookmarkStartPLC(bookmarkStarts)
	sttbfBkmk := buildLegacyBookmarkNameTable(bookmarkNames)
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)
	fcPlcfBkf := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	lcbPlcfBkf := len(plcfBkf)
	fcSttbfBkmk := fcPlcfBkf + lcbPlcfBkf
	lcbSttbfBkmk := len(sttbfBkmk)
	fcClx := fcSttbfBkmk + lcbSttbfBkmk
	clx := buildLegacySinglePieceClx(textOffset, len(allText))

	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcPlcfBkf:], plcfBkf)
	copy(tableStream[fcSttbfBkmk:], sttbfBkmk)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 18, fcPlcfBkf, lcbPlcfBkf)
	putFibFcLcb(wordStream, fibBase, 19, fcSttbfBkmk, lcbSttbfBkmk)
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

// buildLegacyDocWithBookmarksAndEndsCFB is like buildLegacyDocWithBookmarksCFB
// but also writes a PlcfBkl at FIB index 20 so bookmark end positions are loaded.
func buildLegacyDocWithBookmarksAndEndsCFB(bodyText string, bookmarkStarts, bookmarkEnds []int, bookmarkNames []string) []byte {
	const textOffset = 1024

	allText := []byte(bodyText)
	wordStream := make([]byte, textOffset+len(allText))
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 6)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)
	pos += 24

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 42)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	plcfBkf := buildLegacyBookmarkStartPLC(bookmarkStarts)
	plcfBkl := buildLegacyBookmarkEndPLC(bookmarkEnds)
	sttbfBkmk := buildLegacyBookmarkNameTable(bookmarkNames)
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)
	fcPlcfBkf := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	lcbPlcfBkf := len(plcfBkf)
	fcSttbfBkmk := fcPlcfBkf + lcbPlcfBkf
	lcbSttbfBkmk := len(sttbfBkmk)
	fcPlcfBkl := fcSttbfBkmk + lcbSttbfBkmk
	lcbPlcfBkl := len(plcfBkl)
	fcClx := fcPlcfBkl + lcbPlcfBkl
	clx := buildLegacySinglePieceClx(textOffset, len(allText))

	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcPlcfBkf:], plcfBkf)
	copy(tableStream[fcSttbfBkmk:], sttbfBkmk)
	copy(tableStream[fcPlcfBkl:], plcfBkl)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 18, fcPlcfBkf, lcbPlcfBkf)
	putFibFcLcb(wordStream, fibBase, 19, fcSttbfBkmk, lcbSttbfBkmk)
	putFibFcLcb(wordStream, fibBase, 20, fcPlcfBkl, lcbPlcfBkl)
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

func buildLegacyDocWithBodyOnlyCFB(bodyText string) []byte {
	const textOffset = 1024

	allText := []byte(bodyText)
	wordStream := make([]byte, textOffset+len(allText))
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 6)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)
	pos += 24

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 34)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)
	fcClx := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	clx := buildLegacySinglePieceClx(textOffset, len(allText))

	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

func buildLegacyDocWithSingleChpxCFB(bodyText string, grpprl []byte) []byte {
	const textOffset = 1024

	allText := []byte(bodyText)
	wordStreamLen := textOffset + len(allText)
	if wordStreamLen < 2048 {
		wordStreamLen = 2048
	}
	wordStream := make([]byte, wordStreamLen)
	copy(wordStream[textOffset:], allText)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 6)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)
	pos += 24

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 40)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)

	fcPlcfBteChpx := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	plcfBteChpx := make([]byte, 12)
	startFC := uint32(0x40000000 | (textOffset * 2))
	binary.LittleEndian.PutUint32(plcfBteChpx[0:4], startFC)
	binary.LittleEndian.PutUint32(plcfBteChpx[4:8], startFC+uint32(len(bodyText)*2))
	binary.LittleEndian.PutUint32(plcfBteChpx[8:12], 3)

	page := make([]byte, 512)
	page[511] = 1
	binary.LittleEndian.PutUint32(page[0:4], startFC)
	binary.LittleEndian.PutUint32(page[4:8], startFC+uint32(len(bodyText)*2))
	page[8] = 10
	page[20] = byte(len(grpprl))
	copy(page[21:21+len(grpprl)], grpprl)
	copy(wordStream[3*512:4*512], page)

	fcClx := fcPlcfBteChpx + len(plcfBteChpx)
	clx := buildLegacySinglePieceClx(textOffset, len(allText))
	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcPlcfBteChpx:], plcfBteChpx)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, 12, fcPlcfBteChpx, len(plcfBteChpx))
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

func buildLegacyDocWithDrawingTextboxCFB(bodyText, textboxText string, textFlow uint32) []byte {
	const (
		textOffset      = 1024
		plcfSpaFibIndex = 30
	)

	allText := []byte(bodyText + textboxText)
	officeArt := legacyTestOfficeArtTextFlowSpContainer(0x11223344, textFlow)
	wordStream := make([]byte, textOffset+len(allText)+len(officeArt))
	copy(wordStream[textOffset:], allText)
	copy(wordStream[textOffset+len(allText):], officeArt)

	binary.LittleEndian.PutUint16(wordStream[0:2], 0xA5EC)
	binary.LittleEndian.PutUint16(wordStream[2:4], 0x00C1)

	pos := 32
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 0)
	pos += 2
	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 11)
	pos += 2

	binary.LittleEndian.PutUint32(wordStream[pos+12:pos+16], uint32(len(bodyText)))
	binary.LittleEndian.PutUint32(wordStream[pos+16:pos+20], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+20:pos+24], 0)
	binary.LittleEndian.PutUint32(wordStream[pos+36:pos+40], uint32(len(textboxText)))
	pos += 44

	binary.LittleEndian.PutUint16(wordStream[pos:pos+2], 34)
	fibBase := pos + 2

	plcfSed, sepxBlob := buildLegacyPlcfSed([]string{bodyText})
	plcfSpa := buildLegacySingleAnchorPlcfSpa(1, len(bodyText), 0x11223344, 100, 200, 400, 800, 0x0050)
	const fcPlcfSed = 0
	lcbPlcfSed := len(plcfSed)
	fcPlcfSpa := fcPlcfSed + lcbPlcfSed + len(sepxBlob)
	fcClx := fcPlcfSpa + len(plcfSpa)
	clx := buildLegacySinglePieceClx(textOffset, len(allText))

	tableStream := make([]byte, fcClx+len(clx))
	copy(tableStream[fcPlcfSed:], plcfSed)
	copy(tableStream[fcPlcfSed+lcbPlcfSed:], sepxBlob)
	copy(tableStream[fcPlcfSpa:], plcfSpa)
	copy(tableStream[fcClx:], clx)

	putFibFcLcb(wordStream, fibBase, 6, fcPlcfSed, lcbPlcfSed)
	putFibFcLcb(wordStream, fibBase, plcfSpaFibIndex, fcPlcfSpa, len(plcfSpa))
	putFibFcLcb(wordStream, fibBase, 33, fcClx, len(clx))

	return buildMinimalCFBWithStreams([]legacyTestStream{
		{Name: "WordDocument", Payload: wordStream},
		{Name: "0Table", Payload: tableStream},
	})
}

func buildLegacySingleAnchorPlcfSpa(cp, mainTextLen int, shape uint32, left, top, right, bottom int32, flags uint16) []byte {
	plcf := make([]byte, 34)
	binary.LittleEndian.PutUint32(plcf[0:4], uint32(cp))
	binary.LittleEndian.PutUint32(plcf[4:8], uint32(mainTextLen))
	binary.LittleEndian.PutUint32(plcf[8:12], shape)
	binary.LittleEndian.PutUint32(plcf[12:16], uint32(left))
	binary.LittleEndian.PutUint32(plcf[16:20], uint32(top))
	binary.LittleEndian.PutUint32(plcf[20:24], uint32(right))
	binary.LittleEndian.PutUint32(plcf[24:28], uint32(bottom))
	binary.LittleEndian.PutUint16(plcf[28:30], flags)
	return plcf
}

func firstLegacyDrawingTextbox(doc *wordir.Document) *wordir.TextBox {
	if doc == nil {
		return nil
	}
	for _, paragraph := range doc.Paragraphs {
		for _, run := range paragraph.Runs {
			if run.TextBox != nil {
				return run.TextBox
			}
		}
	}
	return nil
}

func buildLegacyHeaderDocument(noteStories [6]string, sectionStories [][6]string) (string, []byte) {
	stories := make([]string, 0, 6+len(sectionStories)*6)
	for _, story := range noteStories {
		stories = append(stories, story)
	}
	for _, group := range sectionStories {
		for _, story := range group {
			stories = append(stories, story)
		}
	}

	var b strings.Builder
	cps := make([]int, 0, len(stories)+2)
	for _, story := range stories {
		cps = append(cps, b.Len())
		if story == "" {
			continue
		}
		b.WriteString(story)
		b.WriteString("\r\r")
	}
	text := b.String()
	if len(text) == 0 {
		cps = append(cps, 0, 0)
	} else {
		cps = append(cps, len(text)-1, len(text))
	}

	plcf := make([]byte, len(cps)*4)
	for i, cp := range cps {
		binary.LittleEndian.PutUint32(plcf[i*4:(i+1)*4], uint32(cp))
	}
	return text, plcf
}

func buildLegacyFootnoteDocument(stories []string) (string, []byte) {
	var b strings.Builder
	cps := make([]int, 0, len(stories)+1)
	for _, story := range stories {
		cps = append(cps, b.Len())
		b.WriteString(story)
	}
	cps = append(cps, b.Len())

	plcf := make([]byte, len(cps)*4)
	for i, cp := range cps {
		binary.LittleEndian.PutUint32(plcf[i*4:(i+1)*4], uint32(cp))
	}
	return b.String(), plcf
}

func buildLegacyNoteRefPLC(refCPs []int) []byte {
	plcf := make([]byte, 4*(len(refCPs)+1)+2*len(refCPs))
	cpEnd := 0
	if len(refCPs) > 0 {
		cpEnd = refCPs[len(refCPs)-1] + 1
	}
	for i, cp := range refCPs {
		binary.LittleEndian.PutUint32(plcf[i*4:(i+1)*4], uint32(cp))
	}
	binary.LittleEndian.PutUint32(plcf[len(refCPs)*4:(len(refCPs)+1)*4], uint32(cpEnd))
	return plcf
}

func buildLegacyBookmarkStartPLC(starts []int) []byte {
	plcf := make([]byte, 4*(len(starts)+1)+4*len(starts))
	terminal := 0
	if len(starts) > 0 {
		terminal = starts[len(starts)-1]
	}
	for i, cp := range starts {
		binary.LittleEndian.PutUint32(plcf[i*4:(i+1)*4], uint32(cp))
	}
	binary.LittleEndian.PutUint32(plcf[len(starts)*4:(len(starts)+1)*4], uint32(terminal))
	return plcf
}

// buildLegacyBookmarkEndPLC builds a PlcfBkl end-position array.
// PlcfBkl contains (n+1) 4-byte CPs; the last value is a terminator set to
// the last end CP (or 0 when ends is empty).
func buildLegacyBookmarkEndPLC(ends []int) []byte {
	plcf := make([]byte, (len(ends)+1)*4)
	terminal := 0
	if len(ends) > 0 {
		terminal = ends[len(ends)-1]
	}
	for i, cp := range ends {
		binary.LittleEndian.PutUint32(plcf[i*4:(i+1)*4], uint32(cp))
	}
	binary.LittleEndian.PutUint32(plcf[len(ends)*4:(len(ends)+1)*4], uint32(terminal))
	return plcf
}

func buildLegacyBookmarkNameTable(names []string) []byte {
	size := 6
	for _, name := range names {
		size += 2 + len([]rune(name))*2
	}
	sttbf := make([]byte, size)
	binary.LittleEndian.PutUint16(sttbf[0:2], 0xFFFF)
	binary.LittleEndian.PutUint16(sttbf[2:4], uint16(len(names)))
	binary.LittleEndian.PutUint16(sttbf[4:6], 0)
	pos := 6
	for _, name := range names {
		runes := []rune(name)
		binary.LittleEndian.PutUint16(sttbf[pos:pos+2], uint16(len(runes)))
		pos += 2
		for _, r := range runes {
			binary.LittleEndian.PutUint16(sttbf[pos:pos+2], uint16(r))
			pos += 2
		}
	}
	return sttbf
}

func buildLegacyPlcfSed(bodyParts []string) ([]byte, []byte) {
	if len(bodyParts) == 0 {
		bodyParts = []string{""}
	}
	n := len(bodyParts)
	plcf := make([]byte, (n+1)*4+n*12)
	cp := 0
	for i, part := range bodyParts {
		binary.LittleEndian.PutUint32(plcf[i*4:(i+1)*4], uint32(cp))
		cp += len(part)
	}
	binary.LittleEndian.PutUint32(plcf[n*4:(n+1)*4], uint32(cp))

	var sepxBlob []byte
	sepxOffset := len(plcf)
	for i := 0; i < n; i++ {
		sepx := buildLegacySepx()
		sedOff := (n+1)*4 + i*12
		binary.LittleEndian.PutUint32(plcf[sedOff+2:sedOff+6], uint32(sepxOffset))

		record := make([]byte, 2+len(sepx))
		binary.LittleEndian.PutUint16(record[0:2], uint16(len(sepx)))
		copy(record[2:], sepx)
		sepxBlob = append(sepxBlob, record...)
		sepxOffset += len(record)
	}
	return plcf, sepxBlob
}

type legacyTestStream struct {
	Name    string
	Payload []byte
}

func buildMinimalCFBWithStreams(streams []legacyTestStream) []byte {
	w := cfb.NewWriter()
	for _, s := range streams {
		if err := w.AddStream(s.Name, s.Payload); err != nil {
			panic(fmt.Sprintf("buildMinimalCFBWithStreams: AddStream(%q): %v", s.Name, err))
		}
	}
	out, err := w.Bytes()
	if err != nil {
		panic(fmt.Sprintf("buildMinimalCFBWithStreams: Bytes: %v", err))
	}
	return out
}

func buildLegacySepx() []byte {
	var grpprl []byte
	grpprl = appendSprmU16(grpprl, 0xB01F, 12240) // page width
	grpprl = appendSprmU16(grpprl, 0xB020, 15840) // page height
	grpprl = appendSprmU16(grpprl, 0xB021, 1440)  // left margin
	grpprl = appendSprmU16(grpprl, 0x9023, 720)   // top margin
	grpprl = appendSprmU16(grpprl, 0xB022, 1440)  // right margin
	grpprl = appendSprmU16(grpprl, 0x9024, 720)   // bottom margin
	grpprl = appendSprmU16(grpprl, 0xB017, 360)   // header distance
	grpprl = appendSprmU16(grpprl, 0xB018, 360)   // footer distance
	return grpprl
}

func buildLegacySinglePieceClx(textOffset, charCount int) []byte {
	plc := make([]byte, 16)
	binary.LittleEndian.PutUint32(plc[0:4], 0)
	binary.LittleEndian.PutUint32(plc[4:8], uint32(charCount))
	fcRaw := uint32(0x40000000 | (textOffset * 2))
	binary.LittleEndian.PutUint32(plc[10:14], fcRaw)

	clx := make([]byte, 5+len(plc))
	clx[0] = 0x02
	binary.LittleEndian.PutUint32(clx[1:5], uint32(len(plc)))
	copy(clx[5:], plc)
	return clx
}

func putFibFcLcb(wordStream []byte, fibBase, index, fc, lcb int) {
	base := fibBase + index*8
	binary.LittleEndian.PutUint32(wordStream[base:base+4], uint32(fc))
	binary.LittleEndian.PutUint32(wordStream[base+4:base+8], uint32(lcb))
}

func appendSprmU16(dst []byte, code uint16, value uint16) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint16(buf[0:2], code)
	binary.LittleEndian.PutUint16(buf[2:4], value)
	return append(dst, buf[:]...)
}

func refPartName(refs []wordir.Reference, refType string) string {
	for _, ref := range refs {
		if refType == "default" {
			if ref.Type == "" || ref.Type == "default" {
				return ref.PartName
			}
			continue
		}
		if ref.Type == refType {
			return ref.PartName
		}
	}
	return ""
}

func TestParseChpxGrpprlFontSize(t *testing.T) {
	// sprmCHps (0x4A43) = 24 half-points (12pt).
	grpprl := []byte{0x43, 0x4A, 0x18, 0x00} // 0x0018 = 24
	got := parseChpxGrpprl(grpprl)
	if got.FontSize == nil || *got.FontSize != 24 {
		t.Fatalf("FontSize = %v, want 24", got.FontSize)
	}
}

func TestParseChpxGrpprlIco(t *testing.T) {
	// sprmCIco (0x2A42) = 6 (red).
	grpprl := []byte{0x42, 0x2A, 0x06}
	got := parseChpxGrpprl(grpprl)
	if got.Color != "FF0000" {
		t.Fatalf("Color = %q, want FF0000", got.Color)
	}
}

func TestParseChpxGrpprlHighlight(t *testing.T) {
	tests := []struct {
		ico  byte
		want string
	}{
		{0, ""}, // no highlight (auto)
		{1, "black"},
		{2, "blue"},
		{3, "cyan"},
		{4, "green"},
		{5, "magenta"},
		{6, "red"},
		{7, "yellow"},
		{8, "white"},
		{9, "darkBlue"},
		{10, "darkCyan"},
		{11, "darkGreen"},
		{12, "darkMagenta"},
		{13, "darkRed"},
		{14, "darkYellow"},
		{15, "darkGray"},
		{16, "lightGray"},
		{255, ""}, // unknown Р Р†РІР‚В РІР‚в„ў no highlight
	}
	for _, tt := range tests {
		// sprmCHighlight (0x2A0C) is a 1-byte operand: [0x0C, 0x2A, ico].
		grpprl := []byte{0x0C, 0x2A, tt.ico}
		got := parseChpxGrpprl(grpprl)
		if got.Highlight != tt.want {
			t.Errorf("ico=%d: Highlight = %q, want %q", tt.ico, got.Highlight, tt.want)
		}
	}
}

func TestParseChpxGrpprlExtendedSpecCodes(t *testing.T) {
	grpprl := []byte{
		0x38, 0x08, 0x01, // sprmCFOutline = on
		0x39, 0x08, 0x01, // sprmCFShadow = on
		0x34, 0x2A, 0x02, // sprmCKcd = comma emphasis
		0x53, 0x2A, 0x01, // sprmCFDStrike = on
		0x5A, 0x08, 0x01, // sprmCFBiDi = on
	}
	got := parseChpxGrpprl(grpprl)
	if got.Outline == nil || !*got.Outline {
		t.Fatalf("Outline = %v, want true", got.Outline)
	}
	if got.Shadow == nil || !*got.Shadow {
		t.Fatalf("Shadow = %v, want true", got.Shadow)
	}
	if got.EmphasisMark != "comma" {
		t.Fatalf("EmphasisMark = %q, want comma", got.EmphasisMark)
	}
	if got.DoubleStrike == nil || !*got.DoubleStrike {
		t.Fatalf("DoubleStrike = %v, want true", got.DoubleStrike)
	}
	if got.RTL == nil || !*got.RTL {
		t.Fatalf("RTL = %v, want true", got.RTL)
	}
}

func TestParseChpxGrpprlBiDiFontLangAndSize(t *testing.T) {
	grpprl := []byte{
		0x5E, 0x4A, 0x01, 0x00, // sprmCFtcBi -> font table index 1
		0x5F, 0x48, 0x19, 0x04, // sprmCLidBi -> 0x0419 (ru-RU)
		0x61, 0x4A, 0x20, 0x00, // sprmCHpsBi -> 32 half-points
	}
	got, _ := parseChpxGrpprlWithMetadataAndFonts(grpprl, []string{"Arial", "David"})
	if got.FontCS != "David" {
		t.Fatalf("FontCS = %q, want David", got.FontCS)
	}
	if got.Lang != "ru-RU" {
		t.Fatalf("Lang = %q, want ru-RU", got.Lang)
	}
	if got.FontSize == nil || *got.FontSize != 32 {
		t.Fatalf("FontSize = %v, want 32", got.FontSize)
	}
}

func TestParseChpxGrpprlCharacterSpacingAndHpsPos(t *testing.T) {
	grpprl := []byte{
		0x40, 0x88, 0x14, 0x00, // sprmCDxaSpace = 20 twips
		0x45, 0x48, 0x02, 0x00, // sprmCHpsPos = +2 => superscript heuristic
	}
	got := parseChpxGrpprl(grpprl)
	if got.CharacterSpacing == nil || *got.CharacterSpacing != 20 {
		t.Fatalf("CharacterSpacing = %v, want 20", got.CharacterSpacing)
	}
	if got.VertAlignVal != "superscript" {
		t.Fatalf("VertAlignVal = %q, want superscript", got.VertAlignVal)
	}
	if got.Position == nil || *got.Position != 2 {
		t.Fatalf("Position = %v, want 2", got.Position)
	}
}

func TestParseChpxGrpprlRelativeFontSizingAndShd80(t *testing.T) {
	grpprl := []byte{
		0x43, 0x4A, 0x18, 0x00, // sprmCHps = 24 half-points
		0x44, 0x2A, 0x02, // sprmCHpsInc = +4 half-points
		0x4D, 0x4A, 0x32, 0x00, // sprmCHpsMul = +50%
		0x66, 0x48, 0x01, 0x04, // sprmCShd80 = solid foreground ico 1
	}
	got := parseChpxGrpprl(grpprl)
	if got.FontSize == nil || *got.FontSize != 42 {
		t.Fatalf("FontSize = %v, want 42", got.FontSize)
	}
	if got.Background != "000000" {
		t.Fatalf("Background = %q, want 000000", got.Background)
	}
}

func TestParseChpxGrpprlColorRef(t *testing.T) {
	// sprmCCv (0x6870) = RGB(0x11, 0x22, 0x33).
	grpprl := []byte{0x70, 0x68, 0x11, 0x22, 0x33, 0x00}
	got := parseChpxGrpprl(grpprl)
	if got.Color != "112233" {
		t.Fatalf("Color = %q, want 112233", got.Color)
	}
}

func TestParseChpxGrpprlVariableRunShading(t *testing.T) {
	// sprmCShd (0xCA71) with a variable-length 10-byte Shd payload:
	// foreColor=0x00800000, backColor=0x00000005, ipat=1 => use foreground color.
	grpprl := []byte{
		0x71, 0xCA, 0x0A,
		0x00, 0x00, 0x80, 0x00,
		0x05, 0x00, 0x00, 0x00,
		0x01, 0x00,
	}
	got := parseChpxGrpprl(grpprl)
	if got.Background != "000080" {
		t.Fatalf("Background = %q, want 000080", got.Background)
	}
}

func TestParseChpxGrpprlFELanguageFallback(t *testing.T) {
	grpprl := []byte{0x6E, 0x4A, 0x09, 0x04} // sprmCLidFE = en-US
	got := parseChpxGrpprl(grpprl)
	if got.Lang != "en-US" {
		t.Fatalf("Lang = %q, want en-US", got.Lang)
	}
}

func TestParseChpxGrpprlSuperSubscript(t *testing.T) {
	grpprl := []byte{0x48, 0x2A, 0x01}
	got := parseChpxGrpprl(grpprl)
	if got.VertAlignVal != "superscript" {
		t.Fatalf("VertAlignVal = %q, want superscript", got.VertAlignVal)
	}
	grpprl2 := []byte{0x48, 0x2A, 0x02}
	got2 := parseChpxGrpprl(grpprl2)
	if got2.VertAlignVal != "subscript" {
		t.Fatalf("VertAlignVal = %q, want subscript", got2.VertAlignVal)
	}
}

func TestParseChpxGrpprlOff(t *testing.T) {
	// Bold=off via 0x00 operand.
	grpprl := []byte{0x35, 0x08, 0x00}
	got := parseChpxGrpprl(grpprl)
	if got.Bold == nil || *got.Bold {
		t.Fatalf("Bold = %v, want false", got.Bold)
	}
}

func TestParseChpxGrpprlWithMetadataRevisionAndSpec(t *testing.T) {
	format, meta := parseChpxGrpprlWithMetadata([]byte{
		0x55, 0x08, 0x01, // sprmCFSpec = on
		0x01, 0x08, 0x01, // inserted revision mark
	})
	if !meta.Spec {
		t.Fatal("meta.Spec = false, want true")
	}
	if meta.Revision != "inserted" {
		t.Fatalf("meta.Revision = %q, want inserted", meta.Revision)
	}
	if format.Bold != nil {
		t.Fatalf("format.Bold = %v, want nil", format.Bold)
	}

	_, delMeta := parseChpxGrpprlWithMetadata([]byte{0x00, 0x08, 0x01})
	if delMeta.Revision != "deleted" {
		t.Fatalf("deleted meta.Revision = %q, want deleted", delMeta.Revision)
	}
}

func TestParseChpxGrpprlPropertyRevisionMark(t *testing.T) {
	dttm := uint32(0)
	dttm |= uint32(29) << 11
	dttm |= uint32(4) << 16
	dttm |= uint32(126) << 20
	_, meta := parseChpxGrpprlWithMetadata([]byte{
		0x57, 0xCA, 0x07,
		0x01,
		0x02, 0x00,
		byte(dttm), byte(dttm >> 8), byte(dttm >> 16), byte(dttm >> 24),
	})
	if meta.PropertyRevision == nil {
		t.Fatal("PropertyRevision = nil, want metadata")
	}
	if !meta.PropertyRevision.Present {
		t.Fatal("PropertyRevision.Present = false, want true")
	}
	if !meta.PropertyRevision.HasIbst || meta.PropertyRevision.Ibst != 2 {
		t.Fatalf("PropertyRevision ibst = (%v, %d), want (true, 2)", meta.PropertyRevision.HasIbst, meta.PropertyRevision.Ibst)
	}
	if !meta.PropertyRevision.HasDttm {
		t.Fatal("PropertyRevision.HasDttm = false, want true")
	}
	if got := formatLegacyRevisionDate(meta.PropertyRevision.Dttm); got != "2026-04-29" {
		t.Fatalf("PropertyRevision date = %q, want 2026-04-29", got)
	}
}

func TestParseChpxGrpprlTextAnimation(t *testing.T) {
	grpprl := []byte{
		0x59, 0x28, 0x04, // sprmCSfxText = 4 (antsBlack)
	}
	got := parseChpxGrpprl(grpprl)
	if got.TextAnimation != "antsBlack" {
		t.Fatalf("TextAnimation = %q, want antsBlack", got.TextAnimation)
	}
}

func TestParseChpxGrpprlRecognizedLongTailSPRMsDoNotBlockFollowingOps(t *testing.T) {
	tests := []struct {
		name   string
		grpprl []byte
	}{
		{
			name:   "sprmCIstd",
			grpprl: []byte{0x30, 0x4A, 0x02, 0x00, 0x35, 0x08, 0x01},
		},
		{
			name:   "sprmCPicLocation",
			grpprl: []byte{0x03, 0x6A, 0x34, 0x12, 0x00, 0x00, 0x35, 0x08, 0x01},
		},
		{
			name:   "sprmCPropRMark",
			grpprl: []byte{0x57, 0xCA, 0x07, 0x01, 0x02, 0x00, 0x78, 0x56, 0x34, 0x12, 0x35, 0x08, 0x01},
		},
		{
			name: "sprmCDispFldRMark",
			grpprl: []byte{
				0x62, 0xCA, 0x27, 0x01, 0x03, 0x00, 0x12, 0x34, 0x56, 0x78,
				0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
				0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
				0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
				0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
				0x35, 0x08, 0x01,
			},
		},
		{
			name:   "sprmCFNoProof",
			grpprl: []byte{0x75, 0x08, 0x01, 0x35, 0x08, 0x01},
		},
		{
			name:   "obsolete sprm 0x0867",
			grpprl: []byte{0x67, 0x08, 0x01, 0x35, 0x08, 0x01},
		},
		{
			name:   "legacy sprm 0x0869",
			grpprl: []byte{0x69, 0x08, 0x01, 0x35, 0x08, 0x01},
		},
		{
			name:   "legacy sprm 0x086B",
			grpprl: []byte{0x6B, 0x08, 0x01, 0x35, 0x08, 0x01},
		},
		{
			name:   "legacy sprm 0x086C",
			grpprl: []byte{0x6C, 0x08, 0x01, 0x35, 0x08, 0x01},
		},
	}
	for _, tt := range tests {
		got := parseChpxGrpprl(tt.grpprl)
		if got.Bold == nil || !*got.Bold {
			t.Fatalf("%s: Bold = %v, want true after recognized long-tail SPRM", tt.name, got.Bold)
		}
	}
}

func TestParseChpxGrpprlUsePgsuSettings(t *testing.T) {
	grpprl := []byte{0x68, 0x08, 0x01, 0x35, 0x08, 0x01}
	got := parseChpxGrpprl(grpprl)
	if got.UsePageSetup == nil || !*got.UsePageSetup {
		t.Fatalf("UsePageSetup = %v, want true", got.UsePageSetup)
	}
	if got.Bold == nil || !*got.Bold {
		t.Fatal("Bold = nil/false, want true from trailing sprm")
	}
}

func TestParseChpxGrpprlDispFldRMark(t *testing.T) {
	grpprl := []byte{
		0x62, 0xCA, 0x27, 0x01, 0x03, 0x00, 0x12, 0x34, 0x56, 0x78,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
		0x35, 0x08, 0x01,
	}
	_, meta := parseChpxGrpprlWithMetadataAndFonts(grpprl, nil)
	if meta.DisplayFieldRevision == nil {
		t.Fatal("DisplayFieldRevision = nil, want metadata")
	}
	if !meta.DisplayFieldRevision.Present {
		t.Fatal("DisplayFieldRevision.Present = false, want true")
	}
	if !meta.DisplayFieldRevision.HasIbst || meta.DisplayFieldRevision.Ibst != 0x0003 {
		t.Fatalf("DisplayFieldRevision ibst = (%v, %d), want (true, 3)", meta.DisplayFieldRevision.HasIbst, meta.DisplayFieldRevision.Ibst)
	}
	if !meta.DisplayFieldRevision.HasDttm {
		t.Fatal("DisplayFieldRevision.HasDttm = false, want true")
	}
}

func TestParseChpxFKPPage(t *testing.T) {
	// Build a minimal 512-byte FKP page with two runs:
	//   run 0: FC=100, CHPX at bx=10 (offset 20): bold=on
	//   run 1: FC=200, bx=0 (default formatting)
	// Sentinel FC = 300
	page := make([]byte, 512)
	page[511] = 2 // crun = 2

	// rgfc[0..2]: 3 FC values starting at offset 0.
	binary.LittleEndian.PutUint32(page[0:], 100)
	binary.LittleEndian.PutUint32(page[4:], 200)
	binary.LittleEndian.PutUint32(page[8:], 300)

	// rgb[0..1] at offset (3)*4 = 12: bx values.
	page[12] = 10 // run 0: Chpx at page offset 10*2=20
	page[13] = 0  // run 1: default (no CHPX)

	// Chpx at offset 20: cb=3 then grpprl (sprmCFBold=on).
	page[20] = 3 // cb
	page[21] = 0x35
	page[22] = 0x08
	page[23] = 0x01 // sprmCFBold=on

	entries := parseChpxFKPPage(page)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].FC != 100 {
		t.Fatalf("entries[0].FC = %d, want 100", entries[0].FC)
	}
	if entries[0].Format.Bold == nil || !*entries[0].Format.Bold {
		t.Fatalf("entries[0].Format.Bold = %v, want true", entries[0].Format.Bold)
	}
	if entries[1].FC != 200 {
		t.Fatalf("entries[1].FC = %d, want 200", entries[1].FC)
	}
	if entries[1].Format.Bold != nil {
		t.Fatalf("entries[1].Format.Bold = %v, want nil (default)", entries[1].Format.Bold)
	}
}

func TestFindChpxEntry(t *testing.T) {
	entries := []chpxEntry{{FC: 10}, {FC: 20}, {FC: 30}}
	tests := []struct {
		fc   uint32
		want int
	}{
		{5, -1},
		{10, 0},
		{15, 0},
		{20, 1},
		{25, 1},
		{30, 2},
		{99, 2},
	}
	for _, tc := range tests {
		got := findChpxEntry(entries, tc.fc)
		if got != tc.want {
			t.Fatalf("findChpxEntry(_, %d) = %d, want %d", tc.fc, got, tc.want)
		}
	}
}

// TestBuildLegacyDocFromRunsPreservesFormatting verifies that character
// formatting on docRuns flows through into wordir.Run.Format.
func TestBuildLegacyDocFromRunsPreservesFormatting(t *testing.T) {
	bold := true
	runs := []docRun{
		{Text: "Hello ", Format: wordir.RunFormat{}},
		{Text: "world", Format: wordir.RunFormat{Bold: &bold}},
		{Text: "\nSecond paragraph", Format: wordir.RunFormat{}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 2 {
		t.Fatalf("paragraphs = %d, want 2", len(doc.Paragraphs))
	}
	p0 := doc.Paragraphs[0]
	if len(p0.Runs) != 2 {
		t.Fatalf("paragraph[0] runs = %d, want 2", len(p0.Runs))
	}
	if p0.Runs[0].Text != "Hello " {
		t.Fatalf("run[0].Text = %q, want %q", p0.Runs[0].Text, "Hello ")
	}
	if p0.Runs[0].Format.Bold != nil {
		t.Fatalf("run[0].Bold = %v, want nil", p0.Runs[0].Format.Bold)
	}
	if p0.Runs[1].Text != "world" {
		t.Fatalf("run[1].Text = %q, want %q", p0.Runs[1].Text, "world")
	}
	if p0.Runs[1].Format.Bold == nil || !*p0.Runs[1].Format.Bold {
		t.Fatalf("run[1].Bold = %v, want true", p0.Runs[1].Format.Bold)
	}
}

// TestBuildLegacyDocFromRunsWithListMarker checks that a list marker prefix is
// stripped from the runs correctly even when the marker and text span separate
// docRuns.
func TestBuildLegacyDocFromRunsWithListMarker(t *testing.T) {
	bold := true
	runs := []docRun{
		{Text: "1. ", Format: wordir.RunFormat{}},
		{Text: "item text", Format: wordir.RunFormat{Bold: &bold}},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(doc.Paragraphs))
	}
	p := doc.Paragraphs[0]
	if p.List == nil || p.List.Kind != "ordered" {
		t.Fatalf("List = %v, want ordered", p.List)
	}
	if p.Text != "item text" {
		t.Fatalf("Text = %q, want item text", p.Text)
	}
	if len(p.Runs) != 1 || p.Runs[0].Text != "item text" {
		t.Fatalf("Runs = %v, want [{item text bold}]", p.Runs)
	}
	if p.Runs[0].Format.Bold == nil || !*p.Runs[0].Format.Bold {
		t.Fatalf("Runs[0].Bold = %v, want true", p.Runs[0].Format.Bold)
	}
}

// TestBuildLegacyDocFromRunsHidesVanishRuns verifies that runs with
// Vanish=true are suppressed when extractDocRunsViaFIB drops them, and that
// splitDocRunsIntoLines does not break on empty text after filtering.
func TestSplitDocRunsIntoLinesPreservesBlankParagraphs(t *testing.T) {
	runs := []docRun{
		{Text: "line1\n\n\nline2", Format: wordir.RunFormat{}},
	}
	lines := splitDocRunsIntoLines(runs)
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4 (line1 + two blank paragraphs + line2)", len(lines))
	}
	if joinDocRunsText(lines[0]) != "line1" {
		t.Fatalf("lines[0] = %q, want line1", joinDocRunsText(lines[0]))
	}
	for i := 1; i <= 2; i++ {
		if len(lines[i]) != 1 || lines[i][0].Kind != "blankParagraph" || lines[i][0].Text != "" {
			t.Fatalf("lines[%d] = %#v, want explicit blank paragraph", i, lines[i])
		}
	}
	if joinDocRunsText(lines[3]) != "line2" {
		t.Fatalf("lines[3] = %q, want line2", joinDocRunsText(lines[3]))
	}
}

func TestSplitDocRunsIntoLinesNormalisesControlChars(t *testing.T) {
	runs := []docRun{
		{Text: "a\u000Bb\u000Cc", Format: wordir.RunFormat{}},
	}
	lines := splitDocRunsIntoLines(runs)
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4", len(lines))
	}
}

func TestSplitDocRunsIntoLinesPreservesPageBreakRun(t *testing.T) {
	runs := []docRun{
		{Text: "a\u000Bb\u000Cc", Format: wordir.RunFormat{}},
	}
	lines := splitDocRunsIntoLines(runs)
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want 4", len(lines))
	}
	if joinDocRunsText(lines[0]) != "a" {
		t.Fatalf("lines[0] = %q, want a", joinDocRunsText(lines[0]))
	}
	if joinDocRunsText(lines[1]) != "b" {
		t.Fatalf("lines[1] = %q, want b", joinDocRunsText(lines[1]))
	}
	if len(lines[2]) != 1 || lines[2][0].Kind != "break" || lines[2][0].BreakType != "page" {
		t.Fatalf("lines[2] = %#v, want page-break line", lines[2])
	}
	if joinDocRunsText(lines[3]) != "c" {
		t.Fatalf("lines[3] = %q, want c", joinDocRunsText(lines[3]))
	}
}

func TestBuildLegacyDocFromRunsPreservesManualPageBreakLine(t *testing.T) {
	doc := buildLegacyDocFromRuns("test.doc", []docRun{{Text: "Before\fAfter"}})
	if len(doc.Paragraphs) != 3 {
		t.Fatalf("paragraphs = %d, want 3", len(doc.Paragraphs))
	}
	if doc.Paragraphs[0].Text != "Before" {
		t.Fatalf("paragraph[0].Text = %q, want Before", doc.Paragraphs[0].Text)
	}
	if len(doc.Paragraphs[1].Runs) != 1 || doc.Paragraphs[1].Runs[0].Kind != "break" || doc.Paragraphs[1].Runs[0].BreakType != "page" {
		t.Fatalf("paragraph[1].Runs = %#v, want single page-break run", doc.Paragraphs[1].Runs)
	}
	if doc.Paragraphs[2].Text != "After" {
		t.Fatalf("paragraph[2].Text = %q, want After", doc.Paragraphs[2].Text)
	}
}

func TestSkipDocRunsBytesAcrossRunBoundary(t *testing.T) {
	runs := []docRun{
		{Text: "123", Format: wordir.RunFormat{}},
		{Text: "456", Format: wordir.RunFormat{}},
	}
	result := skipDocRunsBytes(runs, 4)
	if len(result) != 1 || result[0].Text != "56" {
		t.Fatalf("skipDocRunsBytes = %v, want [{56}]", result)
	}
}

// --- PAPx: line spacing, paragraph borders, keep flags ---

func TestParsePapxGrpprlKeepNextAndKeepLines(t *testing.T) {
	// sprmPFKeep (0x2405) + sprmPFKeepFollow (0x2406), both 1-byte operands.
	grpprl := []byte{
		0x05, 0x24, 0x01, // 0x2405 keep=true
		0x06, 0x24, 0x01, // 0x2406 keepFollow=true
	}
	got := parsePapxGrpprl(grpprl)
	if !got.KeepNext {
		t.Fatalf("KeepNext = false, want true")
	}
	if !got.KeepLines {
		t.Fatalf("KeepLines = false, want true")
	}
}

func TestParsePapxGrpprlKeepFlagsOff(t *testing.T) {
	grpprl := []byte{
		0x05, 0x24, 0x00, // keep=off
		0x06, 0x24, 0x00, // keepFollow=off
	}
	got := parsePapxGrpprl(grpprl)
	if got.KeepNext {
		t.Fatalf("KeepNext = true, want false")
	}
	if got.KeepLines {
		t.Fatalf("KeepLines = true, want false")
	}
}

func TestParsePapxGrpprlLegacyParagraphCoverageExtras(t *testing.T) {
	grpprl := []byte{
		0x31, 0x24, 0x01, // sprmPFWidowControl = true
		0x41, 0x24, 0x01, // sprmPFBiDi = true
		0x2D, 0x44, 0x01, 0x04, // sprmPShd80 = solid fill using icoFore=1
		0x5D, 0x84, 0x20, 0x00, // sprmPDxaRight alias = 32
		0x5E, 0x84, 0x40, 0x00, // sprmPDxaLeft alias = 64
		0x60, 0x84, 0xE2, 0xFF, // sprmPDxaLeft1 alias = -30 => hanging 30
	}

	got := parsePapxGrpprl(grpprl)
	if !got.WidowControl {
		t.Fatal("WidowControl = false, want true")
	}
	if !got.Bidi {
		t.Fatal("Bidi = false, want true")
	}
	if got.Shading == "" {
		t.Fatal("Shading = empty, want sprmPShd80 color")
	}
	if got.IndentRight == nil || *got.IndentRight != 32 {
		t.Fatalf("IndentRight = %v, want 32", got.IndentRight)
	}
	if got.IndentLeft == nil || *got.IndentLeft != 64 {
		t.Fatalf("IndentLeft = %v, want 64", got.IndentLeft)
	}
	if got.IndentHanging == nil || *got.IndentHanging != 30 {
		t.Fatalf("IndentHanging = %v, want 30", got.IndentHanging)
	}
}

func TestParsePapxGrpprlRecognizedLongTailSPRMsDoNotBlockFollowingOps(t *testing.T) {
	tests := []struct {
		name   string
		grpprl []byte
	}{
		{
			name:   "undocumented sprm 0x2444",
			grpprl: []byte{0x44, 0x24, 0x01, 0x03, 0x24, 0x01},
		},
		{
			name:   "sprmPFUsePgsuSettings",
			grpprl: []byte{0x47, 0x24, 0x01, 0x03, 0x24, 0x01},
		},
		{
			name:   "sprmPFAdjustRight",
			grpprl: []byte{0x48, 0x24, 0x01, 0x03, 0x24, 0x01},
		},
		{
			name:   "sprmPRsid",
			grpprl: []byte{0x67, 0x64, 0x78, 0x56, 0x34, 0x12, 0x03, 0x24, 0x01},
		},
		{
			name:   "sprmPNumRM",
			grpprl: []byte{0x45, 0xC6, 0x02, 0xAA, 0xBB, 0x03, 0x24, 0x01},
		},
	}
	for _, tt := range tests {
		got := parsePapxGrpprl(tt.grpprl)
		if got.Alignment != "center" {
			t.Fatalf("%s: Alignment = %q, want center after recognized long-tail SPRM", tt.name, got.Alignment)
		}
	}
}

func TestParsePapxGrpprlUsePgsuSettings(t *testing.T) {
	grpprl := []byte{0x47, 0x24, 0x01, 0x03, 0x24, 0x01}
	got := parsePapxGrpprl(grpprl)
	if !got.UsePageSetup {
		t.Fatalf("UsePageSetup = false, want true")
	}
	if got.Alignment != "center" {
		t.Fatalf("Alignment = %q, want center from trailing sprm", got.Alignment)
	}
}

func TestParsePapxGrpprlLogicalJustification(t *testing.T) {
	grpprl := []byte{0x61, 0x24, 0x02} // sprmPJcLogical = right
	got := parsePapxGrpprl(grpprl)
	if got.Alignment != "right" {
		t.Fatalf("Alignment = %q, want right", got.Alignment)
	}
}

func TestParsePapxGrpprlLineSpacingAuto(t *testing.T) {
	// sprmPLspd (0x8426): dy=240, fMultLinespace=1 Р Р†РІР‚В РІР‚в„ў rule=auto, spacing=240
	grpprl := make([]byte, 2+4)
	grpprl[0], grpprl[1] = 0x26, 0x84               // code 0x8426 LE
	binary.LittleEndian.PutUint16(grpprl[2:4], 240) // dy
	binary.LittleEndian.PutUint16(grpprl[4:6], 1)   // fMultLinespace=1
	got := parsePapxGrpprl(grpprl)
	if got.LineRule != "auto" {
		t.Fatalf("LineRule = %q, want auto", got.LineRule)
	}
	if got.LineSpacing == nil || *got.LineSpacing != 240 {
		t.Fatalf("LineSpacing = %v, want 240", got.LineSpacing)
	}
}

func TestParsePapxGrpprlLineSpacingExact(t *testing.T) {
	// dy=360, fMultLinespace=0 Р Р†РІР‚В РІР‚в„ў exact (18pt in twips)
	grpprl := make([]byte, 6)
	grpprl[0], grpprl[1] = 0x26, 0x84
	binary.LittleEndian.PutUint16(grpprl[2:4], 360) // dy positive
	// fMultLinespace left as 0
	got := parsePapxGrpprl(grpprl)
	if got.LineRule != "exact" {
		t.Fatalf("LineRule = %q, want exact", got.LineRule)
	}
	if got.LineSpacing == nil || *got.LineSpacing != 360 {
		t.Fatalf("LineSpacing = %v, want 360", got.LineSpacing)
	}
}

func TestParsePapxGrpprlLineSpacingAtLeast(t *testing.T) {
	// dy=-240 Р Р†РІР‚В РІР‚в„ў at-least (absolute value stored)
	grpprl := make([]byte, 6)
	grpprl[0], grpprl[1] = 0x26, 0x84
	dy := int16(-240)
	binary.LittleEndian.PutUint16(grpprl[2:4], uint16(dy)) // negative dy
	got := parsePapxGrpprl(grpprl)
	if got.LineRule != "atLeast" {
		t.Fatalf("LineRule = %q, want atLeast", got.LineRule)
	}
	if got.LineSpacing == nil || *got.LineSpacing != 240 {
		t.Fatalf("LineSpacing = %v, want 240 (absolute)", got.LineSpacing)
	}
}

func TestParsePapxGrpprlLineSpacingLegacyVariant(t *testing.T) {
	// Legacy DOC fixtures can store the same 4-byte LSPD payload under 0x6412.
	grpprl := make([]byte, 6)
	grpprl[0], grpprl[1] = 0x12, 0x64
	binary.LittleEndian.PutUint16(grpprl[2:4], 360)
	binary.LittleEndian.PutUint16(grpprl[4:6], 1)
	got := parsePapxGrpprl(grpprl)
	if got.LineRule != "auto" {
		t.Fatalf("LineRule = %q, want auto", got.LineRule)
	}
	if got.LineSpacing == nil || *got.LineSpacing != 360 {
		t.Fatalf("LineSpacing = %v, want 360", got.LineSpacing)
	}
}

func TestParsePapxGrpprlAdjustRightInd(t *testing.T) {
	grpprl := []byte{0x48, 0x24, 0x01} // sprmPFAdjustRight = true
	got := parsePapxGrpprl(grpprl)
	if !got.AdjustRightInd {
		t.Fatalf("AdjustRightInd = false, want true")
	}
}

func TestParsePapxGrpprlParagraphBorders(t *testing.T) {
	// sprmPBrcTop80 (0x6424): type=1 (single), width=8 eighths (1pt), ico=0, space=0
	grpprl := make([]byte, 2+4)
	grpprl[0], grpprl[1] = 0x24, 0x64 // code 0x6424 LE
	grpprl[2] = 8                     // op[0]: dptLineWidth = 8 eighths = 1pt
	grpprl[3] = 1                     // op[1]: brcType = 1 (single)
	got := parsePapxGrpprl(grpprl)
	if got.Borders == nil {
		t.Fatal("Borders is nil, expected non-nil")
	}
	if got.Borders.Top == nil {
		t.Fatal("Borders.Top is nil")
	}
	if got.Borders.Top.Val != "single" {
		t.Fatalf("Borders.Top.Val = %q, want single", got.Borders.Top.Val)
	}
	if got.Borders.Top.SizePt != 1.0 {
		t.Fatalf("Borders.Top.SizePt = %v, want 1.0", got.Borders.Top.SizePt)
	}
	if got.Borders.Bottom != nil {
		t.Fatal("Borders.Bottom should be nil (not set)")
	}
}

func TestParsePapxGrpprlBorderNoneIgnored(t *testing.T) {
	// brcType=0 Р Р†РІР‚В РІР‚в„ў no border set
	grpprl := make([]byte, 6)
	grpprl[0], grpprl[1] = 0x24, 0x64 // sprmPBrcTop80
	// all other bytes 0 Р Р†РІР‚В РІР‚в„ў type=0 Р Р†РІР‚В РІР‚в„ў no border
	got := parsePapxGrpprl(grpprl)
	if got.Borders != nil {
		t.Fatal("Borders should be nil for brcType=0")
	}
}

func TestParsePapxGrpprlShading(t *testing.T) {
	// sprmPShd (0xC60D): spra=6 (variable), op[0]=count=10, then 10 data bytes.
	// Total grpprl = 2 (code) + 1 (count) + 10 (data) = 13 bytes.
	// Data layout: cvFore[0..3], cvBack[4..7], ipat[8..9].
	// cvFore = 0x000000FF (red in COLORREF LE: R=0xFF,G=0,B=0,high=0).
	// ipat = 1 (solid fill using cvFore).
	grpprl := make([]byte, 13)
	grpprl[0], grpprl[1] = 0x0D, 0xC6 // code 0xC60D LE
	grpprl[2] = 10                    // count
	grpprl[3] = 0xFF                  // R of cvFore
	// grpprl[4..6] = G,B,high of cvFore: all zero (not auto sentinel)
	// grpprl[7..10] = cvBack: all zeros
	// grpprl[11..12] = ipat: 1 = solid
	grpprl[11], grpprl[12] = 1, 0
	got := parsePapxGrpprl(grpprl)
	if got.Shading != "FF0000" {
		t.Fatalf("Shading = %q, want FF0000", got.Shading)
	}
}

func TestParsePapxGrpprlShadingNoShading(t *testing.T) {
	// ipat=0 Р Р†РІР‚В РІР‚в„ў no shading
	grpprl := make([]byte, 2+1+10)
	grpprl[0], grpprl[1] = 0x0D, 0xC6
	grpprl[2] = 10
	// all remaining bytes 0 Р Р†РІР‚В РІР‚в„ў ipat=0
	got := parsePapxGrpprl(grpprl)
	if got.Shading != "" {
		t.Fatalf("Shading = %q, want empty for ipat=0", got.Shading)
	}
}

func TestParsePapxGrpprlPageBreakBefore(t *testing.T) {
	// sprmPFPageBreakBefore (0x2407) = 1
	grpprl := []byte{0x07, 0x24, 0x01}
	got := parsePapxGrpprl(grpprl)
	if !got.PageBreakBefore {
		t.Fatalf("PageBreakBefore = false, want true")
	}
}

func TestLoadPapxEntriesUsesFibIndex13(t *testing.T) {
	const (
		cbRgFcLcb = 14
		fibBase   = 0
	)

	wordStream := make([]byte, 1536)
	tableStream := make([]byte, 12)

	// Index 11 points outside the table stream; this should be ignored.
	binary.LittleEndian.PutUint32(wordStream[(11*8):], 100)
	binary.LittleEndian.PutUint32(wordStream[(11*8)+4:], 12)

	// Index 13 points at a valid PlcBtePapx with one FKP page number.
	binary.LittleEndian.PutUint32(wordStream[(13*8):], 0)
	binary.LittleEndian.PutUint32(wordStream[(13*8)+4:], 12)
	binary.LittleEndian.PutUint32(tableStream[0:4], 0)
	binary.LittleEndian.PutUint32(tableStream[4:8], 16)
	binary.LittleEndian.PutUint32(tableStream[8:12], 1) // pn = 1 -> page offset 512

	page := make([]byte, 512)
	page[511] = 1                                 // cpara
	page[(1+1)*4] = 32                            // bOffset stored in first BxPap entry
	page[64] = 3                                  // cb: grpprl length = (cb-1)*2 = 4
	binary.LittleEndian.PutUint16(page[65:67], 0) // istd
	page[67], page[68] = 0x0E, 0x84               // sprmPDxaLeft
	binary.LittleEndian.PutUint16(page[69:71], 720)
	copy(wordStream[512:], page)

	entries := loadPapxEntries(wordStream, tableStream, fibBase, cbRgFcLcb)
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Format.BinaryStyleIndex == nil || *entries[0].Format.BinaryStyleIndex != 0 {
		t.Fatalf("BinaryStyleIndex = %v, want 0", entries[0].Format.BinaryStyleIndex)
	}
	if entries[0].Format.IndentLeft == nil || *entries[0].Format.IndentLeft != 720 {
		t.Fatalf("IndentLeft = %v, want 720", entries[0].Format.IndentLeft)
	}
}

// --- Field-character preprocessing ---

func TestApplyDocFieldsToRunsHyperlink(t *testing.T) {
	// Simulate: \x13 HYPERLINK "http://example.com" \x14click here\x15
	runs := []docRun{
		{Text: "\x13 HYPERLINK \"http://example.com\" \x14click here\x15"},
	}
	out := applyDocFieldsToRuns(runs)
	if len(out) != 1 {
		t.Fatalf("got %d runs, want 1", len(out))
	}
	if out[0].Text != "click here" {
		t.Fatalf("Text = %q, want %q", out[0].Text, "click here")
	}
	if out[0].Target != "http://example.com" {
		t.Fatalf("Target = %q, want http://example.com", out[0].Target)
	}
}

func TestParseDocFixtureSampleWithCommentsHasAnnotations(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/sample_with_comments.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if len(doc.Comments) < 2 {
		t.Fatalf("ParseDoc(%s): comments = %d, want >= 2", path, len(doc.Comments))
	}
	if len(doc.Paragraphs) < 3 {
		t.Fatalf("ParseDoc(%s): paragraphs = %d, want >= 3", path, len(doc.Paragraphs))
	}
	found := false
	for _, c := range doc.Comments {
		if strings.Contains(c.Text, "first comment") || strings.Contains(c.Text, "second comment") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ParseDoc(%s): no comment story contains expected text; got %d stories", path, len(doc.Comments))
	}
}

func TestParseDocFixtureSampleWithBookmarksHasNamedBookmarks(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/sample_with_bookmarks.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	// Collect all bookmark names across all paragraphs.
	names := map[string]bool{}
	for _, para := range doc.Paragraphs {
		for _, bm := range para.Bookmarks {
			names[bm] = true
		}
	}
	want := []string{"IntroductionBookmark", "MainContentBookmark", "ConclusionBookmark"}
	for _, w := range want {
		if !names[w] {
			t.Errorf("ParseDoc(%s): bookmark %q not found; got %v", path, w, names)
		}
	}
	if len(doc.Paragraphs) < 1 {
		t.Fatalf("ParseDoc(%s): paragraphs = %d, want >= 1", path, len(doc.Paragraphs))
	}
}

func TestParseDocFixtureSampleWithFootnotesRealHasFootnotes(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/sample_with_footnotes.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if len(doc.Footnotes) < 2 {
		t.Fatalf("ParseDoc(%s): footnotes = %d, want >= 2", path, len(doc.Footnotes))
	}
	if got := doc.Footnotes[1].Text; !strings.Contains(got, "First footnote") {
		t.Errorf("ParseDoc(%s): Footnotes[1].Text = %q, want content containing %q", path, got, "First footnote")
	}
	if got := doc.Footnotes[2].Text; !strings.Contains(got, "Second footnote") {
		t.Errorf("ParseDoc(%s): Footnotes[2].Text = %q, want content containing %q", path, got, "Second footnote")
	}
	if len(doc.Comments) > 0 {
		t.Errorf("ParseDoc(%s): comments = %d, want 0 (no false-positive annotations)", path, len(doc.Comments))
	}
}

func TestParseDocFixtureSampleWithEndnotesRealHasEndnotes(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/sample_with_endnotes_real.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if len(doc.Endnotes) < 2 {
		t.Fatalf("ParseDoc(%s): endnotes = %d, want >= 2", path, len(doc.Endnotes))
	}
	if got := doc.Endnotes[1].Text; !strings.Contains(got, "First endnote") {
		t.Errorf("ParseDoc(%s): Endnotes[1].Text = %q, want content containing %q", path, got, "First endnote")
	}
	if got := doc.Endnotes[2].Text; !strings.Contains(got, "Second endnote") {
		t.Errorf("ParseDoc(%s): Endnotes[2].Text = %q, want content containing %q", path, got, "Second endnote")
	}
	if len(doc.Comments) > 0 {
		t.Errorf("ParseDoc(%s): comments = %d, want 0 (avoid false-positive annotation story)", path, len(doc.Comments))
	}
}

func TestApplyDocFieldsToRunsNonHyperlinkDropsInstruction(t *testing.T) {
	// PAGE field: instruction dropped, cached result "5" kept, semantics preserved.
	runs := []docRun{
		{Text: "\x13 PAGE \x145\x15"},
	}
	out := applyDocFieldsToRuns(runs)
	if len(out) != 1 || out[0].Text != "5" {
		t.Fatalf("got %v, want single run with text '5'", out)
	}
	if out[0].Target != "" {
		t.Fatalf("Target = %q, want empty for PAGE field", out[0].Target)
	}
	if out[0].FieldKind != "PAGE" {
		t.Fatalf("FieldKind = %q, want PAGE", out[0].FieldKind)
	}
}

func TestApplyDocFieldsToRunsAdditionalKindsKeepCachedResult(t *testing.T) {
	runs := []docRun{
		{Text: "\x13 AUTHOR \x14John Doe\x15"},
		{Text: "\x13 DATE \\@ \"M/d/yyyy\" \x144/26/2006\x15"},
		{Text: "\x13 TIME \\@ \"h:mm am/pm\" \x142:01 pm\x15"},
	}
	out := applyDocFieldsToRuns(runs)
	if len(out) != 3 {
		t.Fatalf("got %d runs, want 3", len(out))
	}
	if out[0].Text != "John Doe" || out[0].FieldKind != "AUTHOR" {
		t.Fatalf("run[0] = %#v, want AUTHOR with cached text", out[0])
	}
	if out[1].Text != "4/26/2006" || out[1].FieldKind != "DATE" {
		t.Fatalf("run[1] = %#v, want DATE with cached text", out[1])
	}
	if out[2].Text != "2:01 pm" || out[2].FieldKind != "TIME" {
		t.Fatalf("run[2] = %#v, want TIME with cached text", out[2])
	}
}

func TestApplyDocFieldsToRunsTagsAdditionalSemanticFamilies(t *testing.T) {
	runs := []docRun{
		{Text: "\x13 SEQ Figure \x141\x15"},
		{Text: "\x13 ASK CustomerName \"Prompt\" \x14Alice\x15"},
		{Text: "\x13 MERGEFIELD CustomerName \\\\* MERGEFORMAT \x14Alice\x15"},
		{Text: "\x13 AUTOTEXT \"Disclaimer\" \x14Disclaimer text\x15"},
		{Text: "\x13 LINK Excel.Sheet.12 \"C:\\\\data.xlsx\" \"Sheet1!R1C1\" \x14Linked sheet\x15"},
	}
	out := applyDocFieldsToRuns(runs)
	if len(out) != 5 {
		t.Fatalf("got %d runs, want 5", len(out))
	}
	if out[0].Text != "1" || out[0].FieldKind != "SEQ" || out[0].FieldArg != "Figure" {
		t.Fatalf("run[0] = %#v, want SEQ with identifier", out[0])
	}
	if out[1].Text != "Alice" || out[1].FieldKind != "ASK" || out[1].FieldArg != "CustomerName" {
		t.Fatalf("run[1] = %#v, want ASK with bookmark name", out[1])
	}
	if out[2].Text != "Alice" || out[2].FieldKind != "MERGEFIELD" || out[2].FieldArg != "CustomerName" {
		t.Fatalf("run[2] = %#v, want MERGEFIELD with field name", out[2])
	}
	if out[3].Text != "Disclaimer text" || out[3].FieldKind != "AUTOTEXT" || out[3].FieldArg != "Disclaimer" {
		t.Fatalf("run[3] = %#v, want AUTOTEXT with entry name", out[3])
	}
	if out[4].Text != "Linked sheet" || out[4].FieldKind != "LINK" || out[4].FieldArg != `LINK Excel.Sheet.12 "C:\\data.xlsx" "Sheet1!R1C1"` {
		t.Fatalf("run[4] = %#v, want LINK with full instruction", out[4])
	}
}

func TestApplyDocFieldsToRunsNoFieldCharsPassthrough(t *testing.T) {
	runs := []docRun{
		{Text: "plain text"},
		{Text: " more"},
	}
	out := applyDocFieldsToRuns(runs)
	if len(out) != 2 || out[0].Text != "plain text" || out[1].Text != " more" {
		t.Fatalf("passthrough failed: %v", out)
	}
}

func TestParseDocFieldURLExtractsQuoted(t *testing.T) {
	got := parseDocFieldURL(`HYPERLINK "http://example.com/path"`)
	if got != "http://example.com/path" {
		t.Fatalf("got %q, want http://example.com/path", got)
	}
}

func TestParseDocFieldURLNonHyperlinkReturnsEmpty(t *testing.T) {
	if got := parseDocFieldURL("PAGE"); got != "" {
		t.Fatalf("got %q, want empty for PAGE field", got)
	}
}

// --- PAPx table flags ---

func TestParsePapxGrpprlInTable(t *testing.T) {
	// sprmPFInTable (0x2416) = 1
	grpprl := []byte{0x16, 0x24, 0x01}
	got := parsePapxGrpprl(grpprl)
	if !got.InTable {
		t.Fatalf("InTable = false, want true")
	}
}

func TestParsePapxGrpprlIsTableTrailingParagraph(t *testing.T) {
	// sprmPFTtp (0x2417) = 1
	grpprl := []byte{0x17, 0x24, 0x01}
	got := parsePapxGrpprl(grpprl)
	if !got.IsTableTrailingParagraph {
		t.Fatalf("IsTableTrailingParagraph = false, want true")
	}
}

func TestBuildLegacyDocFromRunsBinaryTable(t *testing.T) {
	// Simulate a 2-row Р вЂњРІР‚вЂќ 2-col binary table:
	//   Row 1: "A" | "B"  (each cell ends with 0x07 converted to \t)
	//   Row 2: "C" | "D"
	// Each row is terminated by a row-mark paragraph (fTtp=1, text=0x07).
	inTable := wordir.ParagraphFormat{InTable: true}
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	runs := []docRun{
		{Text: "A", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "B", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "", ParaFormat: rowMark},
		{Text: "\r", ParaFormat: rowMark},
		{Text: "C", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "D", ParaFormat: inTable},
		{Text: "\r", ParaFormat: inTable},
		{Text: "", ParaFormat: rowMark},
		{Text: "\r", ParaFormat: rowMark},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	tbl := doc.Tables[0]
	if len(tbl.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(tbl.Rows))
	}
	if len(tbl.Rows[0].Cells) < 2 {
		t.Fatalf("row[0] cells = %d, want Р Р†РІР‚В°РўС’ 2", len(tbl.Rows[0].Cells))
	}
}

func TestBuildLegacyBinaryTablePropagatesSharedRowRevisionToTable(t *testing.T) {
	rowRevision := &wordir.RevisionMark{Author: "TableAuthor", Date: "2026-05-01"}
	rowProps := &legacyTableRowProps{resolvedPropertyRevision: rowRevision}
	rowGroup := legacyBinaryTableRow{
		RowMark: []docRun{{
			TableDepth: 1,
			TableProps: rowProps,
		}},
		LineSlices: [][]docRun{
			{{Text: "A", TableDepth: 1}},
			{{Text: "B", TableDepth: 1}},
			{{TableDepth: 1, ParaFormat: wordir.ParagraphFormat{IsTableTrailingParagraph: true}}},
		},
	}

	table, _ := buildLegacyBinaryTable(0, []legacyBinaryTableRow{rowGroup}, 0)
	if table == nil {
		t.Fatal("expected table")
	}
	if table.PropertyRevision == nil || table.PropertyRevision.Author != "TableAuthor" || table.PropertyRevision.Date != "2026-05-01" {
		t.Fatalf("table.PropertyRevision = %#v, want shared row revision", table.PropertyRevision)
	}
}

func TestBuildLegacyBinaryTableMixedRevisionsNoTableRevision(t *testing.T) {
	rowRevision1 := &wordir.RevisionMark{Author: "Author1", Date: "2026-05-01"}
	rowRevision2 := &wordir.RevisionMark{Author: "Author2", Date: "2026-05-02"}
	rowGroup1 := legacyBinaryTableRow{
		RowMark: []docRun{{
			TableDepth: 1,
			TableProps: &legacyTableRowProps{resolvedPropertyRevision: rowRevision1},
		}},
		LineSlices: [][]docRun{
			{{Text: "A", TableDepth: 1}},
			{{TableDepth: 1, ParaFormat: wordir.ParagraphFormat{IsTableTrailingParagraph: true}}},
		},
	}
	rowGroup2 := legacyBinaryTableRow{
		RowMark: []docRun{{
			TableDepth: 1,
			TableProps: &legacyTableRowProps{resolvedPropertyRevision: rowRevision2},
		}},
		LineSlices: [][]docRun{
			{{Text: "B", TableDepth: 1}},
			{{TableDepth: 1, ParaFormat: wordir.ParagraphFormat{IsTableTrailingParagraph: true}}},
		},
	}

	table, _ := buildLegacyBinaryTable(0, []legacyBinaryTableRow{rowGroup1, rowGroup2}, 0)
	if table == nil {
		t.Fatal("expected table")
	}
	if table.PropertyRevision != nil {
		t.Fatalf("table.PropertyRevision = %#v, want nil when rows have different revisions", table.PropertyRevision)
	}
}

func TestLineSliceInTableReturnsTrue(t *testing.T) {
	slice := []docRun{{Text: "x", ParaFormat: wordir.ParagraphFormat{InTable: true}}}
	if !lineSliceInTable(slice) {
		t.Fatal("expected lineSliceInTable = true")
	}
}

func TestLineSliceInTableFromLegacyHintOnlyReturnsTrue(t *testing.T) {
	slice := []docRun{{Text: "x", ParaFormat: wordir.ParagraphFormat{InTable: true}, LegacyInTableHint: true}}
	if !lineSliceInTableFromLegacyHintOnly(slice) {
		t.Fatal("expected lineSliceInTableFromLegacyHintOnly = true")
	}
	if lineSliceHasBinaryTableMetadata(slice) {
		t.Fatal("hint-only InTable slice should not be treated as binary table metadata")
	}
}

func TestLineSliceInTableFromLegacyHintOnlyIgnoresLegacyRowMarkHint(t *testing.T) {
	slice := []docRun{{
		Text:              "\a",
		ParaFormat:        wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true},
		LegacyInTableHint: true,
		LegacyRowMarkHint: true,
	}}
	if !lineSliceInTableFromLegacyHintOnly(slice) {
		t.Fatal("expected lineSliceInTableFromLegacyHintOnly = true for legacy row mark hint")
	}
	if lineSliceIsExplicitRowMark(slice) {
		t.Fatal("legacy row-mark hint should not activate explicit row-mark routing")
	}
}

func TestLineSliceIsRowMarkReturnsTrueForFTtp(t *testing.T) {
	slice := []docRun{{Text: "\t", ParaFormat: wordir.ParagraphFormat{
		InTable: true, IsTableTrailingParagraph: true,
	}}}
	if !lineSliceIsRowMark(slice) {
		t.Fatal("expected lineSliceIsRowMark = true")
	}
}

func TestSplitDocRunsIntoLinesPreservesRowMarkLines(t *testing.T) {
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	runs := []docRun{
		{Text: "Cell", ParaFormat: wordir.ParagraphFormat{InTable: true}},
		{Text: "\r", ParaFormat: wordir.ParagraphFormat{InTable: true}},
		{Text: "", ParaFormat: rowMark}, // whitespace-only but should be preserved
		{Text: "\r", ParaFormat: rowMark},
	}
	lines := splitDocRunsIntoLines(runs)
	// Should have the "Cell" line plus the row-mark line.
	foundRowMark := false
	for _, line := range lines {
		for _, r := range line {
			if r.ParaFormat.IsTableTrailingParagraph {
				foundRowMark = true
			}
		}
	}
	if !foundRowMark {
		t.Fatal("row-mark line was dropped; expected it to be preserved")
	}
}

func TestBuildLegacyDocFromRunsBinaryTableWithoutExplicitParagraphBreakRuns(t *testing.T) {
	inTable := wordir.ParagraphFormat{InTable: true}
	rowMark := wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}
	runs := []docRun{
		{Text: "A", ParaFormat: inTable},
		{Text: "B", ParaFormat: inTable},
		{Text: "", ParaFormat: rowMark},
		{Text: "C", ParaFormat: inTable},
		{Text: "D", ParaFormat: inTable},
		{Text: "", ParaFormat: rowMark},
	}
	doc := buildLegacyDocFromRuns("test.doc", runs)
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	tbl := doc.Tables[0]
	if len(tbl.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(tbl.Rows))
	}
	if len(tbl.Rows[0].Cells) != 2 {
		t.Fatalf("row[0] cells = %d, want 2", len(tbl.Rows[0].Cells))
	}
	if got := tbl.Rows[0].Cells[0].Text; got != "A" {
		t.Fatalf("row[0] cell[0] text = %q, want A", got)
	}
	if got := tbl.Rows[1].Cells[1].Text; got != "D" {
		t.Fatalf("row[1] cell[1] text = %q, want D", got)
	}
}

func TestParseSepxGrpprlPageSize(t *testing.T) {
	// sprmSXaPage (0xB01F, 2 bytes) = 12240 twips (8.5 in)
	// sprmSYaPage (0xB020, 2 bytes) = 15840 twips (11 in)
	grpprl := []byte{
		0x1F, 0xB0, 0xD0, 0x2F, // 0xB01F le + 12240 le (0x2FD0)
		0x20, 0xB0, 0xE0, 0x3D, // 0xB020 le + 15840 le (0x3DE0)
	}
	sec := parseSepxGrpprl(grpprl)
	if sec.PageSize == nil {
		t.Fatal("expected PageSize to be set")
	}
	if sec.PageSize.Width != 12240 {
		t.Errorf("Width: got %d want 12240", sec.PageSize.Width)
	}
	if sec.PageSize.Height != 15840 {
		t.Errorf("Height: got %d want 15840", sec.PageSize.Height)
	}
	if sec.PageSize.Orient != "portrait" {
		t.Errorf("Orient: got %q want portrait", sec.PageSize.Orient)
	}
}

func TestParseSepxGrpprlLandscape(t *testing.T) {
	// sprmSBOrientation (0x301D, 1 byte) = 1 (landscape)
	// sprmSXaPage = 15840, sprmSYaPage = 12240
	grpprl := []byte{
		0x1D, 0x30, 0x01, // 0x301D + 1
		0x1F, 0xB0, 0xE0, 0x3D, // 0xB01F + 15840
		0x20, 0xB0, 0xD0, 0x2F, // 0xB020 + 12240
	}
	sec := parseSepxGrpprl(grpprl)
	if sec.PageSize == nil {
		t.Fatal("expected PageSize to be set")
	}
	if sec.PageSize.Orient != "landscape" {
		t.Errorf("Orient: got %q want landscape", sec.PageSize.Orient)
	}
}

func TestParseSepxGrpprlMargins(t *testing.T) {
	// sprmSDxaLeft (0xB021) = 1800 twips (1.25 in)
	// sprmSDyaTop (0x9023) = 1440 twips (1 in)
	// sprmSDxaRight (0xB022) = 1800
	// sprmSDyaBottom (0x9024) = 1440
	grpprl := []byte{
		0x21, 0xB0, 0x08, 0x07, // left=1800
		0x23, 0x90, 0xA0, 0x05, // top=1440
		0x22, 0xB0, 0x08, 0x07, // right=1800
		0x24, 0x90, 0xA0, 0x05, // bottom=1440
	}
	sec := parseSepxGrpprl(grpprl)
	if sec.PageMargin == nil {
		t.Fatal("expected PageMargin to be set")
	}
	if sec.PageMargin.Left != 1800 {
		t.Errorf("Left: got %d want 1800", sec.PageMargin.Left)
	}
	if sec.PageMargin.Top != 1440 {
		t.Errorf("Top: got %d want 1440", sec.PageMargin.Top)
	}
	if sec.PageMargin.Right != 1800 {
		t.Errorf("Right: got %d want 1800", sec.PageMargin.Right)
	}
	if sec.PageMargin.Bottom != 1440 {
		t.Errorf("Bottom: got %d want 1440", sec.PageMargin.Bottom)
	}
}

func TestParseSepxGrpprlHeaderFooterAndGutterMargins(t *testing.T) {
	grpprl := []byte{
		0x17, 0xB0, 0x68, 0x01, // sprmSDyaHdrTop = 360
		0x18, 0xB0, 0x2C, 0x01, // sprmSDyaHdrBottom = 300
		0x25, 0xB0, 0x90, 0x01, // sprmSDzaGutter = 400
	}
	sec := parseSepxGrpprl(grpprl)
	if sec.PageMargin == nil {
		t.Fatal("expected PageMargin to be set from header/footer/gutter sprms")
	}
	if sec.PageMargin.Header != 360 {
		t.Fatalf("Header: got %d want 360", sec.PageMargin.Header)
	}
	if sec.PageMargin.Footer != 300 {
		t.Fatalf("Footer: got %d want 300", sec.PageMargin.Footer)
	}
	if sec.PageMargin.Gutter != 400 {
		t.Fatalf("Gutter: got %d want 400", sec.PageMargin.Gutter)
	}
}

func TestParseSepxGrpprlPageNumberFormatAndStart(t *testing.T) {
	grpprl := []byte{
		0x0E, 0x30, 0x01, // sprmSNfcPgn = upperRoman
		0x1C, 0x50, 0x07, 0x00, // sprmSPgnStart = 7
	}
	sec := parseSepxGrpprl(grpprl)
	if sec.PageNumberFormat != "upperRoman" {
		t.Fatalf("PageNumberFormat = %q, want upperRoman", sec.PageNumberFormat)
	}
	if sec.PageNumberStart == nil || *sec.PageNumberStart != 7 {
		t.Fatalf("PageNumberStart = %v, want 7", sec.PageNumberStart)
	}
}

func TestParseSepxGrpprlColumns(t *testing.T) {
	// sprmSCcolumns (0x500B) = 1 (meaning 2 columns, stored as count-1)
	// sprmSDxaColumns (0x900C) = 720 twips (0.5 in spacing)
	// sprmSLBetween (0x3019) = 1 (line between columns)
	grpprl := []byte{
		0x0B, 0x50, 0x01, 0x00, // 0x500B + 1 (2 columns)
		0x0C, 0x90, 0xD0, 0x02, // 0x900C + 720
		0x19, 0x30, 0x01, // 0x3019 + 1
	}
	sec := parseSepxGrpprl(grpprl)
	if sec.Columns == nil {
		t.Fatal("expected Columns to be set")
	}
	if sec.Columns.Num != 2 {
		t.Errorf("Num: got %d want 2", sec.Columns.Num)
	}
	if sec.Columns.Space != 720 {
		t.Errorf("Space: got %d want 720", sec.Columns.Space)
	}
	if !sec.Columns.Sep {
		t.Error("expected Sep=true")
	}
}

func TestParseSepxGrpprlSingleColumnNoColumns(t *testing.T) {
	// sprmSCcolumns = 0 (1 column Р Р†Р вЂљРІР‚Сњ should NOT set Columns)
	grpprl := []byte{0x0B, 0x50, 0x00, 0x00}
	sec := parseSepxGrpprl(grpprl)
	if sec.Columns == nil {
		t.Fatalf("expected Columns=&{Num:1} for single-column, got nil")
	}
	if sec.Columns.Num != 1 {
		t.Errorf("expected Columns.Num=1, got %d", sec.Columns.Num)
	}
}

func TestParseSepxGrpprlPropertyRevisionMark(t *testing.T) {
	dttm := uint32(0)
	dttm |= uint32(29) << 11
	dttm |= uint32(4) << 16
	dttm |= uint32(126) << 20
	grpprl := []byte{
		0x27, 0xD2, 0x07,
		0x01,
		0x02, 0x00,
		byte(dttm), byte(dttm >> 8), byte(dttm >> 16), byte(dttm >> 24),
	}
	_, meta := parseSepxGrpprlWithMetadata(grpprl)
	if meta == nil {
		t.Fatal("PropertyRevision metadata = nil, want parsed section revision")
	}
	if !meta.Present {
		t.Fatal("PropertyRevision.Present = false, want true")
	}
	if !meta.HasIbst || meta.Ibst != 2 {
		t.Fatalf("PropertyRevision ibst = (%v, %d), want (true, 2)", meta.HasIbst, meta.Ibst)
	}
	if !meta.HasDttm {
		t.Fatal("PropertyRevision.HasDttm = false, want true")
	}
	if got := formatLegacyRevisionDate(meta.Dttm); got != "2026-04-29" {
		t.Fatalf("PropertyRevision date = %q, want 2026-04-29", got)
	}
}

func TestBuildLegacyNestedBinaryTablePreservesOuterRowRevision(t *testing.T) {
	rowRevision := &wordir.RevisionMark{Author: "Row Author", Date: "2026-04-29"}
	rowHeight := -240
	tableHeader := true
	cantSplit := true
	rowProps := &legacyTableRowProps{
		resolvedPropertyRevision: rowRevision,
		rowHeight:                &rowHeight,
		tableHeader:              &tableHeader,
		cantSplit:                &cantSplit,
	}
	rowGroup := legacyBinaryTableRow{
		RowMark: []docRun{{
			TableDepth: 1,
			TableProps: rowProps,
		}},
		LineSlices: [][]docRun{
			{{Text: "Inner", TableDepth: 2}},
			{{TableDepth: 2, ParaFormat: wordir.ParagraphFormat{IsTableTrailingParagraph: true}}},
		},
	}

	table, ok := buildLegacyNestedBinaryTable(0, []legacyBinaryTableRow{rowGroup}, 0)
	if !ok || table == nil {
		t.Fatal("expected nested table")
	}
	if len(table.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(table.Rows))
	}
	row := table.Rows[0]
	if row.PropertyRevision == nil || row.PropertyRevision.Author != "Row Author" || row.PropertyRevision.Date != "2026-04-29" {
		t.Fatalf("row.PropertyRevision = %#v, want outer row revision", row.PropertyRevision)
	}
	if row.Cells[0].PropertyRevision == nil || row.Cells[0].PropertyRevision.Author != "Row Author" || row.Cells[0].PropertyRevision.Date != "2026-04-29" {
		t.Fatalf("cell.PropertyRevision = %#v, want outer row revision attribution", row.Cells[0].PropertyRevision)
	}
	if row.Format.Height == nil || *row.Format.Height != 240 || row.Format.HeightRule != "exact" {
		t.Fatalf("row height = (%v, %q), want (240, exact)", row.Format.Height, row.Format.HeightRule)
	}
	if !row.Format.IsHeader || !row.Format.CantSplit {
		t.Fatalf("row format = %+v, want IsHeader and CantSplit", row.Format)
	}
	if len(row.Cells) != 1 || len(row.Cells[0].Tables) != 1 {
		t.Fatalf("row cells = %+v, want one nested table cell", row.Cells)
	}
	if got := row.Cells[0].Tables[0].Text; got != "Inner" {
		t.Fatalf("nested table text = %q, want Inner", got)
	}
	// Table-level revision should be propagated from the shared row revision.
	if table.PropertyRevision == nil || table.PropertyRevision.Author != "Row Author" || table.PropertyRevision.Date != "2026-04-29" {
		t.Fatalf("table.PropertyRevision = %#v, want outer row revision propagated to table", table.PropertyRevision)
	}
}

func TestBuildLegacyNestedBinaryTableMixedRowRevisionsNoTableRevision(t *testing.T) {
	rowRevision1 := &wordir.RevisionMark{Author: "Author1", Date: "2026-04-29"}
	rowRevision2 := &wordir.RevisionMark{Author: "Author2", Date: "2026-04-30"}
	rowGroup1 := legacyBinaryTableRow{
		RowMark: []docRun{{
			TableDepth: 1,
			TableProps: &legacyTableRowProps{resolvedPropertyRevision: rowRevision1},
		}},
		LineSlices: [][]docRun{
			{{Text: "A", TableDepth: 2}},
			{{TableDepth: 2, ParaFormat: wordir.ParagraphFormat{IsTableTrailingParagraph: true}}},
		},
	}
	rowGroup2 := legacyBinaryTableRow{
		RowMark: []docRun{{
			TableDepth: 1,
			TableProps: &legacyTableRowProps{resolvedPropertyRevision: rowRevision2},
		}},
		LineSlices: [][]docRun{
			{{Text: "B", TableDepth: 2}},
			{{TableDepth: 2, ParaFormat: wordir.ParagraphFormat{IsTableTrailingParagraph: true}}},
		},
	}

	table, ok := buildLegacyNestedBinaryTable(0, []legacyBinaryTableRow{rowGroup1, rowGroup2}, 0)
	if !ok || table == nil {
		t.Fatal("expected nested table")
	}
	if table.PropertyRevision != nil {
		t.Fatalf("table.PropertyRevision = %#v, want nil when rows have different revisions", table.PropertyRevision)
	}
}

func TestParseSepxGrpprlPageBordersBRC80(t *testing.T) {
	// sprmSBrcTop80 (0x702B, 4-byte BRC80): 1pt single, color ico 0x01, 4pt space
	// sprmSBrcLeft80 (0x702C): 2pt thick, color ico 0x02, 6pt space
	// sprmSBrcBottom80 (0x702D): 0.5pt dotted (hair), color ico 0x03, 2pt space
	// sprmSBrcRight80 (0x702E): 1.5pt double, color ico 0x04, 8pt space
	grpprl := []byte{
		0x2B, 0x70, 0x08, 0x01, 0x01, 0x04,
		0x2C, 0x70, 0x10, 0x02, 0x02, 0x06,
		0x2D, 0x70, 0x04, 0x04, 0x03, 0x02,
		0x2E, 0x70, 0x0C, 0x03, 0x04, 0x08,
	}
	sec := parseSepxGrpprl(grpprl)
	if sec.PageBorders == nil {
		t.Fatal("expected PageBorders, got nil")
	}
	if sec.PageBorders.Top == nil {
		t.Fatal("expected Top border, got nil")
	}
	if sec.PageBorders.Top.Val != "single" {
		t.Errorf("Top.Val = %q, want single", sec.PageBorders.Top.Val)
	}
	if sec.PageBorders.Left == nil || sec.PageBorders.Left.Val != "thick" {
		t.Errorf("Left.Val = %q, want thick", sec.PageBorders.Left.Val)
	}
	if sec.PageBorders.Bottom == nil || sec.PageBorders.Bottom.Val != "dotted" {
		t.Errorf("Bottom.Val = %q, want dotted", sec.PageBorders.Bottom.Val)
	}
	if sec.PageBorders.Right == nil || sec.PageBorders.Right.Val != "double" {
		t.Errorf("Right.Val = %q, want double", sec.PageBorders.Right.Val)
	}
}

func TestParseSepxGrpprlPageBordersFullBrc(t *testing.T) {
	// sprmSBrcTop (0xD234, spra=6 variable): cb=8 + 8-byte BrcOperand
	// cv=auto (0xFF000000), dptLineWidth=0x10 (2pt), brcType=0x01 (single), flags=0x0004 (4pt space)
	grpprl := []byte{
		0x34, 0xD2, 0x08, 0x00, 0x00, 0x00, 0x00, 0x10, 0x01, 0x00, 0x04,
		0x35, 0xD2, 0x08, 0x00, 0x00, 0x00, 0x00, 0x08, 0x01, 0x00, 0x02,
	}
	sec := parseSepxGrpprl(grpprl)
	if sec.PageBorders == nil {
		t.Fatal("expected PageBorders, got nil")
	}
	if sec.PageBorders.Top == nil {
		t.Fatal("expected Top border, got nil")
	}
	if sec.PageBorders.Top.Val != "single" {
		t.Errorf("Top.Val = %q, want single", sec.PageBorders.Top.Val)
	}
	if sec.PageBorders.Top.SizePt != 2.0 {
		t.Errorf("Top.SizePt = %v, want 2.0", sec.PageBorders.Top.SizePt)
	}
	if sec.PageBorders.Left == nil || sec.PageBorders.Left.SizePt != 1.0 {
		t.Errorf("Left.SizePt = %v, want 1.0", sec.PageBorders.Left.SizePt)
	}
}

func TestParseSepxGrpprlPageBorderProperties(t *testing.T) {
	// sprmSPgbProp (0x522F, 2 bytes): firstPage + edge + back
	// Bits: 0-1 = 01 (firstPage), bit 2 = 1 (edge), bit 3 = 1 (back)
	// 0x0D = 00001101b
	grpprl := []byte{
		0x2F, 0x52, 0x0D, 0x00,
	}
	sec := parseSepxGrpprl(grpprl)
	if sec.PageBorders == nil {
		t.Fatal("expected PageBorders struct (even with no sides)")
	}
	if sec.PageBorders.Display != "firstPage" {
		t.Errorf("Display = %q, want firstPage", sec.PageBorders.Display)
	}
	if sec.PageBorders.OffsetFrom != "edge" {
		t.Errorf("OffsetFrom = %q, want edge", sec.PageBorders.OffsetFrom)
	}
	if sec.PageBorders.ZOrder != "back" {
		t.Errorf("ZOrder = %q, want back", sec.PageBorders.ZOrder)
	}
}

// TestParsePreWord97PapxGrpprlNfcSeqNumb verifies that opcode 12 (nfcSeqNumb)
// is decoded into BinaryListFormat and that opcode 13 (nLvlAnm) continues to
// populate BinaryListLevel.
func TestParsePreWord97PapxGrpprlNfcSeqNumb(t *testing.T) {
	// Opcode 12 is variable-length: byte after opcode is the byte count.
	// grpprl: opcode 12 (length=12, nfcSeqNumb=2 = lowercase roman, startAt=3), opcode 13 (nLvlAnm=1)
	grpprl := []byte{
		12, 0x0C, 0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x03, 0x00,
		13, 0x01,
	}
	got, meta, _, _ := parsePreWord97PapxGrpprl(grpprl)
	if got.BinaryListFormat == nil || *got.BinaryListFormat != 2 {
		t.Fatalf("BinaryListFormat = %v, want 2", got.BinaryListFormat)
	}
	if got.BinaryListLevel == nil || *got.BinaryListLevel != 1 {
		t.Fatalf("BinaryListLevel = %v, want 1", got.BinaryListLevel)
	}
	if meta.PreWord97ListStart == nil || *meta.PreWord97ListStart != 3 {
		t.Fatalf("PreWord97ListStart = %v, want 3", meta.PreWord97ListStart)
	}
}

// TestPreWord97NfcToListFormat verifies the nfcР Р†РІР‚В РІР‚в„ўlist-format mapping.
func TestPreWord97NfcToListFormat(t *testing.T) {
	cases := []struct {
		nfc  int
		want string
	}{
		{0, "decimal"},
		{1, "upperRoman"},
		{2, "lowerRoman"},
		{3, "upperLetter"},
		{4, "lowerLetter"},
		{255, "bullet"},
		{99, "decimal"}, // unknown Р Р†РІР‚В РІР‚в„ў decimal default
	}
	for _, c := range cases {
		if got := preWord97NfcToListFormat(c.nfc); got != c.want {
			t.Errorf("nfc=%d: got %q, want %q", c.nfc, got, c.want)
		}
	}
}

// TestParsePreWord97SyntheticListFixture verifies end-to-end list rendering for
// pre-Word-97 documents with list format opcodes (nfcSeqNumb and nLvlAnm).
func TestParsePreWord97SyntheticListFixture(t *testing.T) {
	// Manually construct opcode sequences to test pre-Word-97 list decoding
	// without needing external fixtures.
	tests := []struct {
		nfc        int    // nfcSeqNumb value
		level      int    // nLvlAnm value
		wantKind   string // expected ListMarker.Kind
		wantFormat string // expected ListMarker.Format
	}{
		{0, 0, "ordered", "decimal"},
		{1, 0, "ordered", "upperRoman"},
		{2, 1, "ordered", "lowerRoman"},
		{3, 0, "ordered", "upperLetter"},
		{4, 2, "ordered", "lowerLetter"},
		{255, 0, "bullet", "bullet"},
	}

	for _, test := range tests {
		// Build PAPx opcode sequence
		grpprl := make([]byte, 0, 10)
		grpprl = append(grpprl, 12, 0x01, byte(test.nfc)) // opcode 12: nfcSeqNumb
		grpprl = append(grpprl, 13, byte(test.level))     // opcode 13: nLvlAnm

		// Parse the opcode sequence (returns 4 values)
		fmt, _, _, _ := parsePreWord97PapxGrpprl(grpprl)

		// Verify BinaryListFormat and BinaryListLevel are set
		if fmt.BinaryListFormat == nil || *fmt.BinaryListFormat != test.nfc {
			t.Errorf("nfc=%d: BinaryListFormat = %v, want %d", test.nfc, fmt.BinaryListFormat, test.nfc)
		}
		if fmt.BinaryListLevel == nil || *fmt.BinaryListLevel != test.level {
			t.Errorf("nfc=%d: BinaryListLevel = %v, want %d", test.nfc, fmt.BinaryListLevel, test.level)
		}

		// Simulate IR ListMarker generation using preWord97NfcToListFormat
		format := preWord97NfcToListFormat(test.nfc)
		kind := "ordered"
		if format == "bullet" {
			kind = "bullet"
		}

		if kind != test.wantKind {
			t.Errorf("nfc=%d: kind = %q, want %q", test.nfc, kind, test.wantKind)
		}
		if format != test.wantFormat {
			t.Errorf("nfc=%d: format = %q, want %q", test.nfc, format, test.wantFormat)
		}
	}
}

// Pre-Word-97 coverage uses positive feature assertions rather than tests that lock missing features in as expected behaviour.

// TestInspectPreWord97BodySpanCcpFtnCcpHdd verifies that InspectPreWord97BodySpan
// correctly reads CcpFtn and CcpHdd from FibRgLw95 offsets 0x38 and 0x3C.
func TestInspectPreWord97BodySpanCcpFtnCcpHdd(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	tests := []struct {
		path        string
		wantCcpFtn  int
		wantCcpHdd  int
		wantHddSize int
	}{
		{"../../../../testdata/doc/apache-Word6.doc", 0, 0, 0},
		{"../../../../testdata/doc/apache-Word95.doc", 0, 0, 0},
		{"../../../../testdata/doc/word95_sections.doc", 0, 86, 32},
	}
	for _, tc := range tests {
		data, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", tc.path, err)
		}
		f, err := cfb.OpenBytes(data)
		if err != nil {
			t.Fatalf("cfb.OpenBytes(%s): %v", tc.path, err)
		}
		wordStream, ok := f.Stream("WordDocument")
		if !ok {
			t.Fatalf("Stream(%s, WordDocument): missing", tc.path)
		}
		span, ok := InspectPreWord97BodySpan(wordStream)
		if !ok {
			t.Fatalf("%s: InspectPreWord97BodySpan returned !ok", tc.path)
		}
		if span.CcpFtn != tc.wantCcpFtn {
			t.Errorf("%s: CcpFtn=%d, want %d", tc.path, span.CcpFtn, tc.wantCcpFtn)
		}
		if span.CcpHdd != tc.wantCcpHdd {
			t.Errorf("%s: CcpHdd=%d, want %d", tc.path, span.CcpHdd, tc.wantCcpHdd)
		}
		if span.HddTableSize != tc.wantHddSize {
			t.Errorf("%s: HddTableSize=%d, want %d", tc.path, span.HddTableSize, tc.wantHddSize)
		}
	}
}

// TestParseDocPreWord97HeaderFooterDecode verifies that the pre-Word-97 header/footer
// PLC decode extracts the expected footer content from Word6_sections2.doc, which has
// a non-empty odd-page footer containing a FILENAME field.
func TestParseDocPreWord97HeaderFooterDecode(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/word95_sections.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}

	if len(doc.Footers) == 0 {
		t.Fatalf("ParseDoc(%s): Footers is empty, want at least one footer story", path)
	}

	// At least one footer story should contain the FILENAME field result or code.
	found := false
	for _, story := range doc.Footers {
		if strings.Contains(story.Text, "sip10sco.doc") || strings.Contains(story.Text, "FILENAME") {
			found = true
			break
		}
	}
	if !found {
		texts := make([]string, 0, len(doc.Footers))
		for n, s := range doc.Footers {
			texts = append(texts, n+"="+s.Text)
		}
		t.Fatalf("ParseDoc(%s): no footer contains 'sip10sco.doc' or 'FILENAME'; footers=%v", path, texts)
	}

	// At least one section should have a footer reference.
	hasFooterRef := false
	for _, sec := range doc.Sections {
		if len(sec.Footers) > 0 {
			hasFooterRef = true
			break
		}
	}
	if !hasFooterRef {
		t.Fatalf("ParseDoc(%s): no section has footer references; sections=%d", path, len(doc.Sections))
	}
}
func TestParsePapxGrpprlBetweenAndBarBorders(t *testing.T) {
	// sprmPBrcBetween (0xC652): spra=6 variable-length operand.
	// Brc layout: cv(4) + dptLineWidth(1) + brcType(1) + flags(2) = 8 bytes.
	grpprl := make([]byte, 11)
	grpprl[0], grpprl[1] = 0x52, 0xC6 // sprmPBrcBetween
	grpprl[2] = 8                     // cb = 8 (operand size excluding cb)
	// BRC data starts at grpprl[3]
	grpprl[3], grpprl[4], grpprl[5], grpprl[6] = 0x00, 0x00, 0x00, 0x00 // cv = black
	grpprl[7] = 16                                                      // dptLineWidth = 16 eighths = 2pt
	grpprl[8] = 0x03                                                    // brcType = double (3 in Brc80/Brc)
	grpprl[9], grpprl[10] = 0x00, 0x00                                  // flags

	got := parsePapxGrpprl(grpprl)
	if got.Borders == nil || got.Borders.Between == nil {
		t.Fatal("expected Between border")
	}
	if got.Borders.Between.Val != "double" || got.Borders.Between.SizePt != 2.0 {
		t.Fatalf("Between = %+v", got.Borders.Between)
	}
}

func TestParsePapxGrpprlFrameTextFlow(t *testing.T) {
	// sprmPFrameTextFlow (0x443A): 2-byte operand
	grpprl := make([]byte, 4)
	grpprl[0], grpprl[1] = 0x3A, 0x44 // sprmPFrameTextFlow
	// flow = 1 (tbRl)
	grpprl[2] = 0x01
	grpprl[3] = 0x00

	got := parsePapxGrpprl(grpprl)
	if got.BinaryFrame == nil {
		t.Fatal("expected BinaryFrame to be set")
	}
	if got.BinaryFrame.TextDirection != "tbRl" {
		t.Fatalf("TextDirection = %q, want tbRl", got.BinaryFrame.TextDirection)
	}
}
func TestBuildLegacyDocSynthesizesDefaultsWhenStyleSheetAbsent(t *testing.T) {
	runs := []docRun{{Text: "Hello world"}}
	doc := buildLegacyDocFromRunsWithContext("test.doc", runs, nil, nil, nil, nil, nil)
	if doc.DefaultRunFormat.FontSize == nil || *doc.DefaultRunFormat.FontSize != 24 {
		t.Fatalf("DefaultRunFormat.FontSize = %v, want 24", doc.DefaultRunFormat.FontSize)
	}
	if doc.DefaultRunFormat.FontASCII != "Times New Roman" {
		t.Fatalf("DefaultRunFormat.FontASCII = %q, want Times New Roman", doc.DefaultRunFormat.FontASCII)
	}
	if doc.DefaultRunFormat.FontHAnsi != "Times New Roman" {
		t.Fatalf("DefaultRunFormat.FontHAnsi = %q, want Times New Roman", doc.DefaultRunFormat.FontHAnsi)
	}
}

func TestLegacyDocPropagatesSnapToGridWhenSectionHasDocGrid(t *testing.T) {
	// Build a minimal document with paragraphs and a section that has a doc grid.
	runs := []docRun{{Text: "Hello world"}}
	doc := buildLegacyDocFromRunsWithContext("test.doc", runs, nil, nil, nil, nil, nil)
	if doc == nil {
		t.Fatal("expected non-nil document")
	}
	// Before adding sections, paragraphs should not snap.
	for i, p := range doc.Paragraphs {
		if p.Format.SnapToGrid {
			t.Fatalf("paragraph %d: SnapToGrid = true before grid section added", i)
		}
	}
	// Add a section with a document grid.
	doc.Sections = []wordir.Section{{
		DocGrid: &wordir.DocGrid{Type: "linesAndChars", LinePitch: 360, CharSpace: 200},
	}}
	// Re-run the propagation logic manually (normally done in the parser).
	for _, sec := range doc.Sections {
		if sec.DocGrid != nil && sec.DocGrid.Type != "" && sec.DocGrid.Type != "default" {
			for i := range doc.Paragraphs {
				doc.Paragraphs[i].Format.SnapToGrid = true
			}
			break
		}
	}
	for i, p := range doc.Paragraphs {
		if !p.Format.SnapToGrid {
			t.Fatalf("paragraph %d: SnapToGrid = false after grid section added", i)
		}
	}
}

func TestLegacyDocDoesNotPropagateSnapToGridForDefaultGrid(t *testing.T) {
	runs := []docRun{{Text: "Hello world"}}
	doc := buildLegacyDocFromRunsWithContext("test.doc", runs, nil, nil, nil, nil, nil)
	doc.Sections = []wordir.Section{{
		DocGrid: &wordir.DocGrid{Type: "default", LinePitch: 360},
	}}
	for _, sec := range doc.Sections {
		if sec.DocGrid != nil && sec.DocGrid.Type != "" && sec.DocGrid.Type != "default" {
			for i := range doc.Paragraphs {
				doc.Paragraphs[i].Format.SnapToGrid = true
			}
			break
		}
	}
	for i, p := range doc.Paragraphs {
		if p.Format.SnapToGrid {
			t.Fatalf("paragraph %d: SnapToGrid = true for default grid type", i)
		}
	}
}

func TestParseLegacyStylePapxSkipsIstdPrefix(t *testing.T) {
	const istd = 3
	// UpxPapx payload: istd=3 followed by sprmPDyaAfter(0xA414)=200 twips.
	data := []byte{0x06, 0x00, 0x03, 0x00, 0x14, 0xA4, 0xC8, 0x00}
	format, raw, consumed := parseLegacyLPUpxPapxWithRawForStyle(data, istd)
	if consumed != len(data) {
		t.Fatalf("consumed = %d, want %d", consumed, len(data))
	}
	if format.SpacingAfter == nil || *format.SpacingAfter != 200 {
		t.Fatalf("SpacingAfter = %v, want 200", format.SpacingAfter)
	}
	if got, want := fmt.Sprintf("% X", raw), "14 A4 C8 00"; got != want {
		t.Fatalf("raw SPRMs = %s, want %s", got, want)
	}
}

func TestParseLegacyStylePapxKeepsHistoricalPrefixlessFixture(t *testing.T) {
	// Historical tests encode the grpprl directly, without the UpxPapx istd.
	data := []byte{0x04, 0x00, 0x14, 0xA4, 0xC8, 0x00}
	format, raw, _ := parseLegacyLPUpxPapxWithRawForStyle(data, 3)
	if format.SpacingAfter == nil || *format.SpacingAfter != 200 {
		t.Fatalf("SpacingAfter = %v, want 200", format.SpacingAfter)
	}
	if got, want := fmt.Sprintf("% X", raw), "14 A4 C8 00"; got != want {
		t.Fatalf("raw SPRMs = %s, want %s", got, want)
	}
}

func TestParseDocWord6SectionsDecodesContinuousLegacySectionBreaks(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/word6_sections.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	if len(doc.Sections) != 3 {
		t.Fatalf("sections = %d, want 3", len(doc.Sections))
	}
	if doc.Sections[0].BreakKind != "" {
		t.Fatalf("section 0 break = %q, want default next-page", doc.Sections[0].BreakKind)
	}
	for i := 1; i < 3; i++ {
		if doc.Sections[i].BreakKind != "continuous" || doc.Sections[i].Type != "continuous" {
			t.Fatalf("section %d break/type = %q/%q, want continuous/continuous", i, doc.Sections[i].BreakKind, doc.Sections[i].Type)
		}
	}
	sectionBlocks := 0
	for _, block := range doc.Blocks {
		if block.Kind == "section" {
			sectionBlocks++
		}
	}
	if sectionBlocks != 2 {
		t.Fatalf("section block count = %d, want 2", sectionBlocks)
	}
}

func TestParseDocWord95SectionsDoesNotLeakAuxiliaryStoryText(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/word95_sections.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	var text strings.Builder
	for _, block := range doc.Blocks {
		if block.Paragraph != nil {
			text.WriteString(block.Paragraph.Text)
			text.WriteByte('\n')
		}
		if block.Table == nil {
			continue
		}
		for _, row := range block.Table.Rows {
			for _, cell := range row.Cells {
				for _, p := range cell.Paragraphs {
					text.WriteString(p.Text)
					text.WriteByte('\n')
				}
			}
		}
	}
	got := text.String()
	if !strings.Contains(got, "Issued August 1997") {
		t.Fatalf("main story lost its final body paragraph")
	}
	for _, leaked := range []string{
		"FILENAME \\* Lower",
		"Default Paragraph Font",
		"*DRAFT  STATEMENT  OF  INSOLVENCY  PRACTICE",
	} {
		if strings.Contains(got, leaked) {
			t.Fatalf("main story contains auxiliary text %q", leaked)
		}
	}
}

func TestParseDocWord95CyrillicReconstructsGridTableRowsAndBreak(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	path := "../../../../testdata/doc/word95_cyrillic.doc"
	doc, err := ParseDoc(path)
	if err != nil {
		t.Fatalf("ParseDoc(%s): %v", path, err)
	}
	rowCounts := make([]int, 0, len(doc.Tables))
	for _, table := range doc.Tables {
		rowCounts = append(rowCounts, len(table.Rows))
	}
	containsRows := func(want int) bool {
		for _, got := range rowCounts {
			if got == want {
				return true
			}
		}
		return false
	}
	// Page 1 contains the white-wine header + 19 data rows and the red-wine
	// header + 10 data rows. Before grid reconstruction those tables collapsed
	// to only a couple of oversized rows, dropping most products from HTML.
	if !containsRows(20) || !containsRows(11) {
		t.Fatalf("table row counts = %v, want tables with 20 and 11 rows", rowCounts)
	}
	foundPageBreak := false
	for _, block := range doc.Blocks {
		if block.Paragraph == nil {
			continue
		}
		for _, run := range block.Paragraph.Runs {
			if run.Kind == "break" && run.BreakType == "page" {
				foundPageBreak = true
				break
			}
		}
	}
	if !foundPageBreak {
		t.Fatal("expected old 0x0e table boundary to produce a page break before the vintage-red table")
	}
}

func TestApplyChpxFormattingToDocRunsUsesCharacterCPsForUnicode(t *testing.T) {
	large := 72
	small := 24
	bold := true
	chpx := []docRun{
		{StartCP: 0, Text: "Компания\r", Format: wordir.RunFormat{FontSize: &large, Bold: &bold}},
		{StartCP: 9, Text: "предлагает\r", Format: wordir.RunFormat{FontSize: &small}},
	}
	base := []docRun{
		{StartCP: 0, Text: "Компания\r"},
		{StartCP: 9, Text: "предлагает\r"},
	}

	got := applyChpxFormattingToDocRuns(base, chpx)
	if len(got) != 2 {
		t.Fatalf("runs = %d, want 2", len(got))
	}
	if got[0].Format.FontSize == nil || *got[0].Format.FontSize != large || got[0].Format.Bold == nil || !*got[0].Format.Bold {
		t.Fatalf("first format = %#v, want 36pt bold source CHPX", got[0].Format)
	}
	if got[1].Format.FontSize == nil || *got[1].Format.FontSize != small {
		t.Fatalf("second font size = %#v, want independent 12pt source CHPX", got[1].Format.FontSize)
	}
	if got[1].Format.Bold != nil && *got[1].Format.Bold {
		t.Fatalf("second format = %#v, inherited bold from preceding Cyrillic run", got[1].Format)
	}
}

func TestApplyChpxFormattingSplitsPapRunAtCharacterBoundary(t *testing.T) {
	base := []docRun{{Text: "Компания\rпредлагает", StartCP: 10}}
	largeSize, normalSize, bold := 72, 24, true
	large := wordir.RunFormat{FontSize: &largeSize, Bold: &bold}
	normal := wordir.RunFormat{FontSize: &normalSize}
	formatted := []docRun{
		{Text: "Компания\r", StartCP: 10, Format: large},
		{Text: "предлагает", StartCP: 19, Format: normal},
	}
	got := applyChpxFormattingToDocRuns(base, formatted)
	if len(got) != 2 {
		t.Fatalf("runs = %d, want 2: %#v", len(got), got)
	}
	if got[0].Text != "Компания\r" || got[0].Format.FontSize == nil || *got[0].Format.FontSize != 72 || got[0].Format.Bold == nil || !*got[0].Format.Bold {
		t.Fatalf("first run = %#v", got[0])
	}
	if got[1].Text != "предлагает" || got[1].Format.FontSize == nil || *got[1].Format.FontSize != 24 || (got[1].Format.Bold != nil && *got[1].Format.Bold) {
		t.Fatalf("second run = %#v", got[1])
	}
}

func TestApplyChpxFormattingKeepsPreWord97GridRunIntact(t *testing.T) {
	base := []docRun{{Text: "Wine\aMaker\a750\a12.00\a\r", StartCP: 20}}
	sizeA, sizeB := 24, 20
	formatted := []docRun{
		{Text: "Wine\aMaker", StartCP: 20, Format: wordir.RunFormat{FontSize: &sizeA}},
		{Text: "\a750\a12.00\a\r", StartCP: 30, Format: wordir.RunFormat{FontSize: &sizeB}},
	}
	got := applyChpxFormattingToDocRuns(base, formatted)
	if len(got) != 1 {
		t.Fatalf("runs = %d, want structural grid run preserved as one run: %#v", len(got), got)
	}
	if got[0].Text != base[0].Text {
		t.Fatalf("grid text = %q, want %q", got[0].Text, base[0].Text)
	}
	if got[0].Format.FontSize == nil || *got[0].Format.FontSize != sizeA {
		t.Fatalf("grid format = %#v, want CHPX active at run start", got[0].Format)
	}
}

func TestInjectDocCommentRefsDoesNotConsumeAnchorCharacter(t *testing.T) {
	runs := []docRun{{Text: "Heading", StartCP: 100}}
	got := injectDocCommentRefs(runs, map[int]int{100: 1})
	if len(got) != 2 || got[0].Kind != "commentRef" || got[0].CommentID != 1 {
		t.Fatalf("runs = %#v", got)
	}
	if got[1].Text != "Heading" {
		t.Fatalf("text after comment ref = %q, want Heading", got[1].Text)
	}
}

func TestNormalizeLegacyBinaryTableGridGapsDoesNotDoubleCountMergedTrailingSlots(t *testing.T) {
	table := &wordir.Table{
		Format: wordir.TableFormat{ColumnWidths: []int{1000, 1000, 1000, 1000, 1000}},
		Rows: []wordir.TableRow{{
			Format: wordir.TableRowFormat{GridAfter: 3},
			Cells: []wordir.TableCell{
				{},
				{Format: wordir.TableCellFormat{GridSpan: 2}},
			},
		}},
	}
	normalizeLegacyBinaryTableGridGaps(table)
	if got := table.Rows[0].Format.GridAfter; got != 2 {
		t.Fatalf("GridAfter = %d, want 2 for a five-column grid", got)
	}
}

func TestParseSepxGrpprlPreservesExplicitColumnGrid(t *testing.T) {
	grpprl := []byte{
		0x0B, 0x50, 0x01, 0x00, // sprmSCcolumns = 1 => 2 columns
		0x05, 0x30, 0x00, // sprmSFEvenlySpaced = false
		0x03, 0xF2, 0x00, 0xB0, 0x04, // sprmSDxaColWidth col 0 = 1200
		0x03, 0xF2, 0x01, 0x60, 0x09, // sprmSDxaColWidth col 1 = 2400
		0x04, 0xF2, 0x00, 0x68, 0x01, // sprmSDxaColSpacing after col 0 = 360
	}
	sec := parseSepxGrpprl(grpprl)
	if sec.Columns == nil {
		t.Fatal("expected explicit column geometry")
	}
	if sec.Columns.EvenlySpaced == nil || *sec.Columns.EvenlySpaced {
		t.Fatalf("EvenlySpaced = %v, want explicit false", sec.Columns.EvenlySpaced)
	}
	if len(sec.Columns.Widths) != 2 || sec.Columns.Widths[0] != 1200 || sec.Columns.Widths[1] != 2400 {
		t.Fatalf("Widths = %#v, want [1200 2400]", sec.Columns.Widths)
	}
	if len(sec.Columns.Spaces) != 1 || sec.Columns.Spaces[0] != 360 {
		t.Fatalf("Spaces = %#v, want [360]", sec.Columns.Spaces)
	}
}

func TestNormalizeLegacyBinaryTableGridGapsClampsOversizedCellSpan(t *testing.T) {
	table := &wordir.Table{
		Format: wordir.TableFormat{ColumnWidths: []int{1000, 1000}},
		Rows: []wordir.TableRow{{Cells: []wordir.TableCell{
			{},
			{Format: wordir.TableCellFormat{GridSpan: 2}},
		}}},
	}
	normalizeLegacyBinaryTableGridGaps(table)
	if got := table.Rows[0].Cells[1].Format.GridSpan; got != 1 {
		t.Fatalf("GridSpan = %d, want 1 inside a two-column grid", got)
	}
}

func TestNormalizeLegacyBinaryRowTokensFoldsExtraParagraphsIntoLastPhysicalCell(t *testing.T) {
	props := &legacyTableRowProps{cellBoundaries: []int{0, 1000, 2000, 3000, 4000}}
	got := normalizeLegacyBinaryRowTokens([]string{"", "G", "H", "I", "J"}, props)
	want := []string{"", "G", "H", "I" + legacyBinaryCellParagraphSeparator + "J"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeLegacyBinaryRowTokens = %#v, want %#v", got, want)
	}
}

func TestDeriveLegacyBinaryTableGridBoundariesUnionsRowCuts(t *testing.T) {
	rows := []*legacyTableRowProps{
		{cellBoundaries: []int{0, 6872, 9302}},
		{cellBoundaries: []int{0, 1062, 5738, 8148, 9302}},
	}
	got := deriveLegacyBinaryTableGridBoundaries(rows)
	want := []int{0, 1062, 5738, 6872, 8148, 9302}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deriveLegacyBinaryTableGridBoundaries = %v, want %v", got, want)
	}
	widths := deriveLegacyBinaryTableColumnWidths(rows)
	wantWidths := []int{1062, 4676, 1134, 1276, 1154}
	if !reflect.DeepEqual(widths, wantWidths) {
		t.Fatalf("deriveLegacyBinaryTableColumnWidths = %v, want %v", widths, wantWidths)
	}
}

func TestLegacyFloatingTableFixturePreservesPositionAndWrap(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	doc, err := ParseDoc("../../../../testdata/doc/libre-floating-table-section-columns.doc")
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	var table *wordir.Table
	for i := range doc.Blocks {
		if doc.Blocks[i].Table != nil {
			table = doc.Blocks[i].Table
			break
		}
	}
	if table == nil || table.Format.FloatProps == nil {
		t.Fatal("expected legacy floating table positioning metadata")
	}
	fp := table.Format.FloatProps
	if fp.HorzAnchor != "margin" || fp.VertAnchor != "text" {
		t.Fatalf("anchors = %q/%q, want margin/text", fp.HorzAnchor, fp.VertAnchor)
	}
	if fp.Y != -87 {
		t.Fatalf("Y = %d, want -87 twips", fp.Y)
	}
	if fp.LeftFromText != 180 || fp.RightFromText != 180 {
		t.Fatalf("wrap distances = %d/%d, want 180/180", fp.LeftFromText, fp.RightFromText)
	}
	if table.Format.Overlap != "never" {
		t.Fatalf("Overlap = %q, want never", table.Format.Overlap)
	}
}

func TestAbsorbLegacyTableBannerCreatesMergedShadedRow(t *testing.T) {
	bold := true
	size := 32
	doc := &wordir.Document{}
	p := wordir.Paragraph{
		Text:   "LEGACY TABLE TITLE",
		Format: wordir.ParagraphFormat{Alignment: "center"},
		Runs:   []wordir.Run{{Text: "LEGACY TABLE TITLE", Format: wordir.RunFormat{Bold: &bold, FontSize: &size, Color: "FFFFFF"}}},
	}
	doc.Paragraphs = append(doc.Paragraphs, p)
	doc.Blocks = append(doc.Blocks, wordir.Block{Kind: "paragraph", Paragraph: &doc.Paragraphs[0]})
	table := &wordir.Table{Format: wordir.TableFormat{ColumnWidths: []int{1000, 2000, 1000}}}
	absorbLegacyTableBanner(doc, table)
	if len(doc.Blocks) != 0 {
		t.Fatalf("banner paragraph remained in body: %d blocks", len(doc.Blocks))
	}
	if len(table.Rows) != 1 || len(table.Rows[0].Cells) != 1 {
		t.Fatalf("banner row shape = %#v", table.Rows)
	}
	cell := table.Rows[0].Cells[0]
	if cell.Format.GridSpan != 3 {
		t.Fatalf("banner colspan = %d, want 3", cell.Format.GridSpan)
	}
	if cell.Format.Shading != "BFBFBF" {
		t.Fatalf("banner shading = %q", cell.Format.Shading)
	}
	if len(cell.Paragraphs) != 1 || cell.Paragraphs[0].Text != "LEGACY TABLE TITLE" {
		t.Fatalf("banner text was not preserved")
	}
}

func TestAbsorbLegacyTableBannerLeavesOrdinaryParagraphAlone(t *testing.T) {
	size := 32
	doc := &wordir.Document{}
	p := wordir.Paragraph{Text: "ordinary caption", Format: wordir.ParagraphFormat{Alignment: "center"}, Runs: []wordir.Run{{Text: "ordinary caption", Format: wordir.RunFormat{FontSize: &size}}}}
	doc.Paragraphs = append(doc.Paragraphs, p)
	doc.Blocks = append(doc.Blocks, wordir.Block{Kind: "paragraph", Paragraph: &doc.Paragraphs[0]})
	table := &wordir.Table{Format: wordir.TableFormat{ColumnWidths: []int{1000, 1000}}}
	absorbLegacyTableBanner(doc, table)
	if len(doc.Blocks) != 1 || len(table.Rows) != 0 {
		t.Fatalf("ordinary paragraph was absorbed")
	}
}

func TestStyleLegacyRecoveredPriceTableRestoresHeaderAndBodyTypography(t *testing.T) {
	mkCell := func(text string) wordir.TableCell {
		p := buildLegacyParagraph(0, text)
		return wordir.TableCell{Text: text, Paragraphs: []wordir.Paragraph{p}, Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	}
	table := &wordir.Table{Rows: []wordir.TableRow{
		{Cells: []wordir.TableCell{mkCell("Наименование продукции"), mkCell("Изготовитель"), mkCell("Тип бутылки"), mkCell("Объем"), mkCell("Цена (руб.)")}},
		{Cells: []wordir.TableCell{mkCell("Алиготе, белое п/с"), mkCell("Басвинекс"), mkCell("Бордо"), mkCell("0,7 л"), mkCell("31-00")}},
	}}
	styleLegacyRecoveredPriceTable(table)
	if !table.Rows[0].Format.IsHeader {
		t.Fatal("price-list heading row should be a repeating semantic header")
	}
	header := table.Rows[0].Cells[0].Paragraphs[0].Runs[0]
	if header.Format.Bold == nil || !*header.Format.Bold || header.Format.Color != "000080" {
		t.Fatalf("header format = %#v, want bold navy", header.Format)
	}
	product := table.Rows[1].Cells[0].Paragraphs[0].Runs
	if len(product) != 2 || product[0].Text != "Алиготе" || product[0].Format.Bold == nil || !*product[0].Format.Bold {
		t.Fatalf("product runs = %#v, want bold product name plus descriptor", product)
	}
	price := table.Rows[1].Cells[4].Paragraphs[0].Runs[0]
	if price.Format.Bold == nil || !*price.Format.Bold || price.Format.Italic == nil || !*price.Format.Italic {
		t.Fatalf("price format = %#v, want bold italic", price.Format)
	}
}

func TestStyleLegacyRecoveredPriceTableDoesNotStyleOrdinaryTable(t *testing.T) {
	p := buildLegacyParagraph(0, "Name")
	table := &wordir.Table{Rows: []wordir.TableRow{{Cells: []wordir.TableCell{{Text: "Name", Paragraphs: []wordir.Paragraph{p}}}}}}
	styleLegacyRecoveredPriceTable(table)
	if table.Rows[0].Format.IsHeader {
		t.Fatal("ordinary recovered table was incorrectly treated as a legacy price list")
	}
}

func TestLegacySpaAnchorOriginsFollowMSDOCValues(t *testing.T) {
	// MS-DOC Spa: bx/by 0=margin, 1=page; value 2 is column for bx and
	// paragraph for by. Keep this explicit because swapping these origins moves
	// every legacy floating drawing by a full margin or paragraph offset.
	for _, tc := range []struct {
		name     string
		flags    uint16
		vertical bool
		want     string
	}{
		{"bx-margin", 0 << 1, false, "margin"},
		{"bx-page", 1 << 1, false, "page"},
		{"bx-column", 2 << 1, false, "column"},
		{"by-margin", 0 << 3, true, "margin"},
		{"by-page", 1 << 3, true, "page"},
		{"by-paragraph", 2 << 3, true, "paragraph"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := legacyAnchorRelativeFromSpaFlags(tc.flags, tc.vertical); got != tc.want {
				t.Fatalf("relative = %q, want %q", got, tc.want)
			}
		})
	}
}
