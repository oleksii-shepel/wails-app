package legacy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"

	"office/pkg/ooxml/cfb"
	dml "office/pkg/ooxml/drawing"
	wordir "office/pkg/word/ir"
)

func TestBuildLegacyWriteTextPreservesParagraphsTablesAndFields(t *testing.T) {
	field := wordir.Run{Kind: "run", Text: "Example", FieldKind: "HYPERLINK", FieldInstr: `HYPERLINK "https://example.com"`}
	p1 := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: "Before "}, field}}
	cellA := wordir.TableCell{Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Kind: "run", Text: "A"}}}}}
	cellB := wordir.TableCell{Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Kind: "run", Text: "B"}}}}}
	table := wordir.Table{Rows: []wordir.TableRow{{Cells: []wordir.TableCell{cellA, cellB}}}}
	p2 := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: "After"}, {Kind: "break", BreakType: "page"}, {Kind: "run", Text: "Page"}}}
	doc := &wordir.Document{Blocks: []wordir.Block{
		{Kind: "paragraph", Paragraph: &p1},
		{Kind: "table", Table: &table},
		{Kind: "paragraph", Paragraph: &p2},
	}}
	got := buildLegacyWriteText(doc)
	for _, want := range []string{
		"Before \x13HYPERLINK \"https://example.com\"\x14Example\x15\r",
		"A\x07B\x07\r",
		"After\fPage\r",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("legacy text %q does not contain %q", got, want)
		}
	}
}

func TestWriteDocumentUsesUnicodePieceAndKeepsSemanticText(t *testing.T) {
	p := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: "Привіт 世界 😀"}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	file, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("open CFB: %v", err)
	}
	wordStream, ok := file.Stream("WordDocument")
	if !ok {
		t.Fatal("missing WordDocument stream")
	}
	tableStream, ok := file.Stream("1Table")
	if !ok {
		t.Fatal("missing 1Table stream")
	}
	got := extractDocTextFromTable(wordStream, tableStream)
	if !strings.Contains(got, "Привіт 世界 😀") {
		t.Fatalf("round-trip text = %q", got)
	}
}

func TestBuildPieceTableAtUsesPCDLayoutExpectedByReader(t *testing.T) {
	plc := buildPieceTableAt(5, 700, false)
	if len(plc) != 16 {
		t.Fatalf("piece table length = %d, want 16", len(plc))
	}
	// The parser reads fcCompressed from bytes 2..6 of the PCD, after the
	// two-byte PCD flags. A previous writer placed fc at the start of the PCD.
	pcd := plc[8:16]
	if got := uint32(pcd[2]) | uint32(pcd[3])<<8 | uint32(pcd[4])<<16 | uint32(pcd[5])<<24; got != 700 {
		t.Fatalf("fcCompressed = %d, want 700", got)
	}
}

func TestLegacyWriterEncodesCoreCHPXFormatting(t *testing.T) {
	on := true
	size := 28
	format := wordir.RunFormat{Bold: &on, Italic: &on, Strike: &on, Underline: "single", FontSize: &size, VertAlignVal: "superscript"}
	grp := encodeChpxGrpprl(format)
	got := parseChpxGrpprl(grp)
	if got.Bold == nil || !*got.Bold || got.Italic == nil || !*got.Italic || got.Strike == nil || !*got.Strike {
		t.Fatalf("toggle formatting = %+v", got)
	}
	if got.Underline != "single" || got.FontSize == nil || *got.FontSize != 28 || got.VertAlignVal != "superscript" {
		t.Fatalf("character formatting = %+v", got)
	}
}

func TestLegacyWriterEncodesCorePAPXFormatting(t *testing.T) {
	left, right, first, before, after := 720, 360, 240, 120, 180
	format := wordir.ParagraphFormat{
		Alignment: "center", KeepLines: true, KeepNext: true, PageBreakBefore: true,
		IndentLeft: &left, IndentRight: &right, IndentFirstLine: &first,
		SpacingBefore: &before, SpacingAfter: &after,
	}
	grp := encodePapxGrpprl(format)
	got := parsePapxGrpprl(grp)
	if got.Alignment != "center" || !got.KeepLines || !got.KeepNext || !got.PageBreakBefore {
		t.Fatalf("paragraph flags = %+v", got)
	}
	if got.IndentLeft == nil || *got.IndentLeft != left || got.IndentRight == nil || *got.IndentRight != right ||
		got.IndentFirstLine == nil || *got.IndentFirstLine != first || got.SpacingBefore == nil || *got.SpacingBefore != before ||
		got.SpacingAfter == nil || *got.SpacingAfter != after {
		t.Fatalf("paragraph dimensions = %+v", got)
	}
}

func TestLegacyWriterFKPPagesRoundTripFormatting(t *testing.T) {
	on := true
	size := 32
	charRuns := []legacyCharacterWriteRun{
		{StartCP: 0, EndCP: 4, Format: wordir.RunFormat{Bold: &on, FontSize: &size}},
		{StartCP: 4, EndCP: 8, Format: wordir.RunFormat{Italic: &on}},
	}
	chpx := parseChpxFKPPage(buildChpxFKPPage(charRuns, 700, legacyFontWriteCatalog{}))
	if len(chpx) != 2 || chpx[0].Format.Bold == nil || !*chpx[0].Format.Bold || chpx[0].Format.FontSize == nil || *chpx[0].Format.FontSize != 32 ||
		chpx[1].Format.Italic == nil || !*chpx[1].Format.Italic {
		t.Fatalf("CHPX FKP = %+v", chpx)
	}
	left := 720
	paras := []legacyParagraphWriteRun{{StartCP: 0, EndCP: 8, Format: wordir.ParagraphFormat{Alignment: "right", IndentLeft: &left}}}
	papx := parsePapxFKPPage(buildPapxFKPPage(paras, 700))
	if len(papx) != 1 || papx[0].Format.Alignment != "right" || papx[0].Format.IndentLeft == nil || *papx[0].Format.IndentLeft != 720 {
		t.Fatalf("PAPX FKP = %+v", papx)
	}
}

func TestWriteDocumentExposesFormattingToLegacyReader(t *testing.T) {
	on := true
	size := 30
	left := 720
	paragraph := wordir.Paragraph{
		Format: wordir.ParagraphFormat{Alignment: "center", IndentLeft: &left},
		Runs: []wordir.Run{
			{Kind: "run", Text: "Bold", Format: wordir.RunFormat{Bold: &on, FontSize: &size}},
			{Kind: "run", Text: " Italic", Format: wordir.RunFormat{Italic: &on}},
		},
	}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &paragraph}}}
	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	file, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("open CFB: %v", err)
	}
	ws, _ := file.Stream("WordDocument")
	ts, _ := file.Stream("1Table")
	// Reuse the reader's own FIB navigation to prove the generated indexes/pages
	// are consumable, rather than testing only the standalone encoders.
	pos := 32
	csw := int(uint16(ws[pos]) | uint16(ws[pos+1])<<8)
	pos += 2 + csw*2
	cslw := int(uint16(ws[pos]) | uint16(ws[pos+1])<<8)
	pos += 2 + cslw*4
	cb := int(uint16(ws[pos]) | uint16(ws[pos+1])<<8)
	fibBase := pos + 2
	if cb > 136 || fibBase+14*8 > len(ws) {
		t.Fatalf("bad FIB navigation: csw=%d cslw=%d cb=%d pos=%d fibBase=%d", csw, cslw, cb, pos, fibBase)
	}

	chpx := loadChpxEntries(ws, ts, fibBase, cb)
	papx := loadPapxEntries(ws, ts, fibBase, cb)
	if len(chpx) < 2 || chpx[0].Format.Bold == nil || !*chpx[0].Format.Bold || chpx[0].Format.FontSize == nil || *chpx[0].Format.FontSize != 30 ||
		chpx[1].Format.Italic == nil || !*chpx[1].Format.Italic {
		t.Fatalf("reader CHPX entries = %+v", chpx)
	}
	if len(papx) < 1 || papx[0].Format.Alignment != "center" || papx[0].Format.IndentLeft == nil || *papx[0].Format.IndentLeft != 720 {
		t.Fatalf("reader PAPX entries = %+v", papx)
	}
}

func TestLegacyWriterEncodesTableRowProperties(t *testing.T) {
	width := 3600
	cellAWidth := 1200
	cellBWidth := 2400
	rowHeight := 480
	meta := legacyTableWriteMeta{
		TableFormat: wordir.TableFormat{
			Width: &width, WidthType: "dxa", Alignment: "center",
			Indent: 240, CellSpacing: 80, Layout: "fixed",
			ColumnWidths: []int{1200, 1200, 1200},
			CellPadTop:   80, CellPadBottom: 80, CellPadLeft: 100, CellPadRight: 100,
			Borders: &wordir.TableBorders{
				Top:     &wordir.BorderSide{Val: "single", SizePt: 1, Color: "FF0000"},
				InsideV: &wordir.BorderSide{Val: "dotted", SizePt: 1, Color: "00FFFF"},
			},
		},
		RowFormat: wordir.TableRowFormat{
			Height: &rowHeight, HeightRule: "exact", IsHeader: true, CantSplit: true,
		},
		Cells: []wordir.TableCell{
			{Format: wordir.TableCellFormat{Width: &cellAWidth, WidthType: "dxa", GridSpan: 2, VertAlign: "center", Shading: "FFF2CC", TextDirection: "tbRl"}},
			{Format: wordir.TableCellFormat{Width: &cellBWidth, WidthType: "dxa", VMerge: "restart", VertAlign: "bottom"}},
		},
	}
	grp := encodeLegacyTableGrpprl(meta)
	if !bytes.Contains(grp, []byte{0x2d, 0xd6}) || !bytes.Contains(grp, []byte{0x29, 0x76}) {
		t.Fatalf("missing shading/text-flow SPRMs in encoded TAPX: % x", grp)
	}
	_, parsed := parsePapxGrpprlWithMetadata(grp)
	fkpEntries := parsePapxFKPPage(buildPapxFKPPage([]legacyParagraphWriteRun{{StartCP: 0, EndCP: 4, Format: wordir.ParagraphFormat{InTable: true, IsTableTrailingParagraph: true}, Table: &meta}}, 700))
	if len(fkpEntries) != 1 || fkpEntries[0].Meta.TableRowProps == nil ||
		fkpEntries[0].Meta.TableRowProps.cellFormat(0).Shading != "FFF2CC" ||
		fkpEntries[0].Meta.TableRowProps.cellFormat(0).TextDirection != "tbRl" {
		t.Fatalf("FKP lost table cell visual metadata: %+v", fkpEntries)
	}
	if parsed.TableRowProps == nil {
		t.Fatal("missing decoded table row props")
	}
	props := parsed.TableRowProps
	if props.tableJc != "center" || props.tableDxaLeft == nil || *props.tableDxaLeft != 240 ||
		props.tableDxaGap == nil || *props.tableDxaGap != 40 {
		t.Fatalf("table geometry props = %+v", props)
	}
	if props.rowHeight == nil || *props.rowHeight != -480 || props.tableHeader == nil || !*props.tableHeader ||
		props.cantSplit == nil || !*props.cantSplit {
		t.Fatalf("row props = %+v", props)
	}
	if props.tableWidth == nil || *props.tableWidth != 3600 || props.tableWidthType != "dxa" {
		t.Fatalf("table width = %v/%q", props.tableWidth, props.tableWidthType)
	}
	if state := props.hMergeState(0); state != 2 {
		t.Fatalf("grid span start merge state = %d, want 2", state)
	}
	if state := props.hMergeState(1); state != 1 {
		t.Fatalf("grid span continuation merge state = %d, want 1", state)
	}
	if state := props.vMergeState(2); state != 3 {
		t.Fatalf("vertical merge state = %d, want 3", state)
	}
	if got := props.cellFormat(0).VertAlign; got != "center" {
		t.Fatalf("cell 0 vert align = %q", got)
	}
	if got := props.cellFormat(2).VertAlign; got != "bottom" {
		t.Fatalf("cell 2 vert align = %q", got)
	}
	if props.tableBorders == nil || props.tableBorders.Top == nil || props.tableBorders.Top.Color != "FF0000" ||
		props.tableBorders.InsideV == nil || props.tableBorders.InsideV.Val != "dotted" {
		t.Fatalf("table borders = %+v", props.tableBorders)
	}
	if props.cellPadTop == nil || *props.cellPadTop != 80 || props.cellPadLeft == nil || *props.cellPadLeft != 100 {
		t.Fatalf("table cell padding = top:%v left:%v", props.cellPadTop, props.cellPadLeft)
	}
	if got := props.cellFormat(0).Shading; got != "FFF2CC" {
		t.Fatalf("cell 0 shading = %q", got)
	}
	if got := props.cellFormat(0).TextDirection; got != "tbRl" {
		t.Fatalf("cell 0 text direction = %q", got)
	}
}

func TestWriteDocumentReconstructsLegacyTableThroughReader(t *testing.T) {
	tableWidth := 3600
	firstWidth := 2400
	secondWidth := 1200
	rowHeight := 400
	cellA := wordir.TableCell{
		Format:     wordir.TableCellFormat{Width: &firstWidth, WidthType: "dxa", GridSpan: 2, VertAlign: "center", Shading: "FFF2CC", TextDirection: "tbRl"},
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Kind: "run", Text: "Merged"}}}},
	}
	cellB := wordir.TableCell{
		Format:     wordir.TableCellFormat{Width: &secondWidth, WidthType: "dxa", VMerge: "restart"},
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Kind: "run", Text: "Side"}}}},
	}
	table := wordir.Table{
		Format: wordir.TableFormat{
			Width: &tableWidth, WidthType: "dxa", Alignment: "right",
			Indent: 360, CellSpacing: 60, Layout: "fixed",
			ColumnWidths: []int{1200, 1200, 1200},
			Borders:      &wordir.TableBorders{Top: &wordir.BorderSide{Val: "single", SizePt: 1, Color: "FF0000"}},
		},
		Rows: []wordir.TableRow{{
			Format: wordir.TableRowFormat{Height: &rowHeight, HeightRule: "exact", IsHeader: true, CantSplit: true},
			Cells:  []wordir.TableCell{cellA, cellB},
		}},
	}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "table", Table: &table}}}

	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	file, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("open CFB: %v", err)
	}
	got, err := parseDocFile(file, "roundtrip.doc")
	if err != nil {
		t.Fatalf("parseDocFile: %v", err)
	}
	if len(got.Tables) != 1 || len(got.Tables[0].Rows) != 1 {
		t.Fatalf("tables = %+v", got.Tables)
	}
	rt := got.Tables[0]
	if rt.Format.Alignment != "right" || rt.Format.Indent != 360 || rt.Format.CellSpacing != 60 {
		t.Fatalf("table format = %+v", rt.Format)
	}
	row := rt.Rows[0]
	if !row.Format.IsHeader || !row.Format.CantSplit || row.Format.Height == nil || *row.Format.Height != 400 || row.Format.HeightRule != "exact" {
		t.Fatalf("row format = %+v", row.Format)
	}
	if len(row.Cells) != 2 {
		t.Fatalf("row cells = %d, want 2: %+v", len(row.Cells), row.Cells)
	}
	if row.Cells[0].Text != "Merged" || row.Cells[0].Format.GridSpan != 2 || !row.Cells[0].Format.HMerge {
		t.Fatalf("merged cell = %+v", row.Cells[0])
	}
	if row.Cells[0].Format.Shading != "FFF2CC" || row.Cells[0].Format.TextDirection != "tbRl" {
		t.Fatalf("merged cell visual format = %+v", row.Cells[0].Format)
	}
	if rt.Format.Borders == nil || rt.Format.Borders.Top == nil || rt.Format.Borders.Top.Color != "FF0000" {
		t.Fatalf("table borders = %+v", rt.Format.Borders)
	}
	if row.Cells[1].Text != "Side" || row.Cells[1].Format.VMerge != "restart" {
		t.Fatalf("side cell = %+v", row.Cells[1])
	}
}

func TestLegacyWriterRoundTripsParagraphStyles(t *testing.T) {
	on := true
	normalAfter := 160
	headingSize := 32
	accentColor := "FF0000"

	doc := &wordir.Document{
		DefaultParagraphStyleID: "Normal",
		DefaultParagraphFormat:  wordir.ParagraphFormat{SpacingAfter: &normalAfter},
		Styles: map[string]wordir.Style{
			"Normal": {
				ID: "Normal", Name: "Normal", Default: true,
				RawFormat: wordir.ParagraphFormat{SpacingAfter: &normalAfter},
			},
			"Heading1": {
				ID: "Heading1", Name: "Heading 1", BasedOn: "Normal", LinkedCharID: "Heading1Char",
				RawFormat:    wordir.ParagraphFormat{Alignment: "center", KeepNext: true},
				RawRunFormat: wordir.RunFormat{Bold: &on, FontSize: &headingSize},
			},
			"CustomBody": {
				ID: "CustomBody", Name: "Custom Body", BasedOn: "Normal",
				RawFormat:    wordir.ParagraphFormat{Alignment: "both"},
				RawRunFormat: wordir.RunFormat{Color: accentColor},
			},
		},
		CharacterStyles: map[string]wordir.CharacterStyle{
			"Heading1Char": {
				ID: "Heading1Char", Name: "Heading 1 Char", BasedOn: "DefaultParagraphFont",
				RawRunFormat: wordir.RunFormat{Bold: &on, FontSize: &headingSize},
			},
			"DefaultParagraphFont": {
				ID: "DefaultParagraphFont", Name: "Default Paragraph Font",
			},
		},
	}
	p1 := wordir.Paragraph{StyleID: "Heading1", Runs: []wordir.Run{{Kind: "run", Text: "Title"}}}
	p2 := wordir.Paragraph{StyleID: "CustomBody", Runs: []wordir.Run{{Kind: "run", Text: "Body"}}}
	doc.Blocks = []wordir.Block{
		{Kind: "paragraph", Paragraph: &p1},
		{Kind: "paragraph", Paragraph: &p2},
	}

	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	file, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("open CFB: %v", err)
	}
	got, err := parseDocFile(file, "styles.doc")
	if err != nil {
		t.Fatalf("parseDocFile: %v", err)
	}
	if got.DefaultParagraphStyleID != "Normal" {
		t.Fatalf("default paragraph style = %q", got.DefaultParagraphStyleID)
	}
	heading, ok := got.Styles["Heading1"]
	if !ok {
		t.Fatalf("Heading1 missing from round-tripped styles: %#v", got.Styles)
	}
	if heading.BasedOn != "Normal" || (heading.LinkedCharID != "Heading1Char" && heading.LinkedCharID != "Heading 1 Char") {
		t.Fatalf("Heading1 relationships = %+v", heading)
	}
	if heading.RawFormat.Alignment != "center" || !heading.RawFormat.KeepNext ||
		heading.RawRunFormat.Bold == nil || !*heading.RawRunFormat.Bold ||
		heading.RawRunFormat.FontSize == nil || *heading.RawRunFormat.FontSize != headingSize {
		t.Fatalf("Heading1 formatting = %+v / %+v", heading.RawFormat, heading.RawRunFormat)
	}
	custom, ok := got.Styles["Custom Body"]
	if !ok {
		// User-defined legacy style IDs are authored names.
		custom, ok = got.Styles["CustomBody"]
	}
	if !ok {
		t.Fatalf("custom style missing: %#v", got.Styles)
	}
	if custom.BasedOn != "Normal" || custom.RawFormat.Alignment != "both" {
		t.Fatalf("custom style = %+v", custom)
	}
	if _, ok := got.CharacterStyles["Heading 1 Char"]; !ok {
		if _, ok = got.CharacterStyles["Heading1Char"]; !ok {
			t.Fatalf("Heading1Char missing: %#v", got.CharacterStyles)
		}
	}
	if len(got.Paragraphs) < 2 {
		t.Fatalf("paragraph count = %d", len(got.Paragraphs))
	}
	if got.Paragraphs[0].StyleID != "Heading1" {
		t.Fatalf("paragraph[0] style = %q/%q", got.Paragraphs[0].StyleID, got.Paragraphs[0].StyleName)
	}
	if got.Paragraphs[1].StyleName != "Custom Body" {
		t.Fatalf("paragraph[1] style = %q/%q", got.Paragraphs[1].StyleID, got.Paragraphs[1].StyleName)
	}
}

func TestLegacyStyleSheetPreservesRawSprmsBeforeSemanticOverrides(t *testing.T) {
	style := wordir.Style{
		ID: "Custom", Name: "Custom",
		RawFormat: wordir.ParagraphFormat{KeepNext: true},
		Source:    &wordir.StyleSource{RawSprms: []byte{0x05, 0x24, 0x00}}, // sprmPFKeepLines=false
	}
	doc := &wordir.Document{Styles: map[string]wordir.Style{"Custom": style}}
	catalog := buildLegacyStyleWriteCatalog(doc)
	data := buildLegacyStyleSheetData(catalog, legacyFontWriteCatalog{})
	if len(data) == 0 {
		t.Fatal("empty STSH")
	}
	// The emitted style sheet must be readable by the same STD parser and retain
	// both raw provenance and the appended semantic override.
	cbStshi := int(binary.LittleEndian.Uint16(data[0:2]))
	pos := 2 + cbStshi
	found := false
	for index := 0; pos+2 <= len(data) && index < len(catalog.Entries); index++ {
		cbStd := int(binary.LittleEndian.Uint16(data[pos:]))
		pos += 2
		if cbStd == 0 || pos+cbStd > len(data) {
			break
		}
		std := data[pos : pos+cbStd]
		pos += cbStd
		if pos&1 != 0 {
			pos++
		}
		raw, ok := parseLegacySTD(std, index, 12)
		if !ok || raw.Name != "Custom" {
			continue
		}
		found = true
		if !bytes.Contains(raw.ParaSprms, []byte{0x05, 0x24, 0x00}) ||
			!bytes.Contains(raw.ParaSprms, []byte{0x06, 0x24, 0x01}) {
			t.Fatalf("style PAPX provenance/override = % x", raw.ParaSprms)
		}
	}
	if !found {
		t.Fatal("Custom style not decoded from emitted STSH")
	}
}

func TestLegacyWriterEncodesExtendedRunAndParagraphFormatting(t *testing.T) {
	on := true
	size := 26
	sizeCS := 24
	spacing := 18
	kern := 20
	scale := 115
	position := 4
	run := wordir.RunFormat{
		Bold: &on, SmallCaps: &on, Caps: &on, DoubleStrike: &on,
		Outline: &on, Shadow: &on, Emboss: &on, Imprint: &on,
		Vanish: &on, NoProof: &on, RTL: &on,
		Underline: "wave", UnderlineColor: "00FF00",
		Color: "336699", Highlight: "yellow", Background: "FFF2CC",
		FontSize: &size, FontSizeCS: &sizeCS,
		CharacterSpacing: &spacing, Kerning: &kern, CharScale: &scale,
		Position: &position, Lang: "uk-UA",
	}
	gotRun := parseChpxGrpprl(encodeChpxGrpprl(run))
	if gotRun.SmallCaps == nil || !*gotRun.SmallCaps || gotRun.Caps == nil || !*gotRun.Caps ||
		gotRun.DoubleStrike == nil || !*gotRun.DoubleStrike ||
		gotRun.Outline == nil || !*gotRun.Outline || gotRun.Shadow == nil || !*gotRun.Shadow ||
		gotRun.Emboss == nil || !*gotRun.Emboss || gotRun.Imprint == nil || !*gotRun.Imprint {
		t.Fatalf("extended toggles = %+v", gotRun)
	}
	if gotRun.Color != "336699" || gotRun.Highlight != "yellow" || gotRun.Background != "FFF2CC" ||
		gotRun.Underline != "wave" || gotRun.UnderlineColor != "00FF00" || gotRun.Lang != "uk-UA" {
		t.Fatalf("extended colors/language = %+v", gotRun)
	}
	if gotRun.CharacterSpacing == nil || *gotRun.CharacterSpacing != 18 ||
		gotRun.Kerning == nil || *gotRun.Kerning != 20 ||
		gotRun.CharScale == nil || *gotRun.CharScale != 115 ||
		gotRun.Position == nil || *gotRun.Position != 4 {
		t.Fatalf("extended run dimensions = %+v", gotRun)
	}

	left := 540
	outline := 2
	wordWrap := true
	para := wordir.ParagraphFormat{
		Alignment: "both", IndentLeft: &left, WidowControl: true,
		ContextualSpacing: true, NoLineNumber: true, NoAutoHyphenation: true,
		Kinsoku: true, WordWrap: &wordWrap, OverflowPunct: true, TopLinePunct: true,
		AutoSpaceDE: true, AutoSpaceDN: true, Bidi: true, AdjustRightInd: true,
		MirrorIndents: true, NoAllowOverlap: true, SpacingBeforeAuto: true,
		SpacingAfterAuto: true, OutlineLevel: &outline, Shading: "DDEBF7",
	}
	gotPara := parsePapxGrpprl(encodePapxGrpprl(para))
	if gotPara.Alignment != "both" || gotPara.IndentLeft == nil || *gotPara.IndentLeft != 540 ||
		!gotPara.WidowControl || !gotPara.ContextualSpacing || !gotPara.NoLineNumber ||
		!gotPara.NoAutoHyphenation || !gotPara.Kinsoku || gotPara.WordWrap == nil || !*gotPara.WordWrap ||
		!gotPara.OverflowPunct || !gotPara.TopLinePunct || !gotPara.AutoSpaceDE || !gotPara.AutoSpaceDN ||
		!gotPara.Bidi || !gotPara.AdjustRightInd || !gotPara.MirrorIndents || !gotPara.NoAllowOverlap ||
		!gotPara.SpacingBeforeAuto || !gotPara.SpacingAfterAuto ||
		gotPara.OutlineLevel == nil || *gotPara.OutlineLevel != 2 || gotPara.Shading != "DDEBF7" {
		t.Fatalf("extended paragraph formatting = %+v", gotPara)
	}
}

func TestLegacyWriterRoundTripsStructuredNumbering(t *testing.T) {
	bold := true
	markerSize := 20
	doc := &wordir.Document{
		NumberingDefinitions: []wordir.NumberingDefinition{
			{
				NumID: 7, AbstractNumID: 700,
				Levels: []wordir.NumberingLevel{
					{Level: 0, Start: 3, Format: "decimal", Text: "%1.", IndentLeft: 720, RunFormat: &wordir.RunFormat{Bold: &bold, FontSize: &markerSize}},
					{Level: 1, Start: 1, Format: "lowerLetter", Text: "%1.%2.", IndentLeft: 1440},
				},
			},
		},
	}
	p1 := wordir.Paragraph{
		Numbering: &wordir.Numbering{NumID: 7, AbstractNumID: 700, Level: 0, Start: 3, Format: "decimal", Text: "3.", IndentLeft: 720},
		List:      &wordir.ListMarker{Kind: "ordered", Level: 0, Start: 3, Format: "decimal", Template: "%1."},
		Runs:      []wordir.Run{{Kind: "run", Text: "Third"}},
	}
	p2 := wordir.Paragraph{
		Numbering: &wordir.Numbering{NumID: 7, AbstractNumID: 700, Level: 0, Start: 4, Format: "decimal", Text: "4.", IndentLeft: 720},
		List:      &wordir.ListMarker{Kind: "ordered", Level: 0, Start: 4, Format: "decimal", Template: "%1."},
		Runs:      []wordir.Run{{Kind: "run", Text: "Fourth"}},
	}
	p3 := wordir.Paragraph{
		Numbering: &wordir.Numbering{NumID: 7, AbstractNumID: 700, Level: 1, Start: 1, Format: "lowerLetter", Text: "3.a.", IndentLeft: 1440},
		List:      &wordir.ListMarker{Kind: "ordered", Level: 1, Start: 1, Format: "lowerLetter", Template: "%1.%2."},
		Runs:      []wordir.Run{{Kind: "run", Text: "Nested"}},
	}
	doc.Blocks = []wordir.Block{
		{Kind: "paragraph", Paragraph: &p1},
		{Kind: "paragraph", Paragraph: &p2},
		{Kind: "paragraph", Paragraph: &p3},
	}

	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	file, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("open CFB: %v", err)
	}
	got, err := parseDocFile(file, "numbering.doc")
	if err != nil {
		t.Fatalf("parseDocFile: %v", err)
	}
	if len(got.Paragraphs) < 3 {
		t.Fatalf("paragraphs=%d", len(got.Paragraphs))
	}
	for i := 0; i < 3; i++ {
		if got.Paragraphs[i].List == nil || got.Paragraphs[i].Numbering == nil {
			t.Fatalf("paragraph[%d] missing numbering: %+v", i, got.Paragraphs[i])
		}
	}
	if got.Paragraphs[0].List.Start != 3 || got.Paragraphs[0].List.Format != "decimal" ||
		got.Paragraphs[0].List.Template != "%1." {
		t.Fatalf("level0 first=%+v", got.Paragraphs[0].List)
	}
	if got.Paragraphs[1].List.Start != 4 {
		t.Fatalf("level0 continuation=%+v", got.Paragraphs[1].List)
	}
	if got.Paragraphs[2].List.Level != 1 || got.Paragraphs[2].List.Format != "lowerLetter" ||
		got.Paragraphs[2].List.Template != "%1.%2." {
		t.Fatalf("level1=%+v", got.Paragraphs[2].List)
	}
	if got.Paragraphs[0].Numbering.AbstractNumID != 700 || got.Paragraphs[0].Numbering.NumID != 1 {
		// NumID in legacy IR is ilfo, while AbstractNumID is the authored LSID.
		t.Fatalf("numbering identity=%+v", got.Paragraphs[0].Numbering)
	}
	if got.Paragraphs[0].Numbering.RunFormat == nil ||
		got.Paragraphs[0].Numbering.RunFormat.Bold == nil || !*got.Paragraphs[0].Numbering.RunFormat.Bold ||
		got.Paragraphs[0].Numbering.RunFormat.FontSize == nil || *got.Paragraphs[0].Numbering.RunFormat.FontSize != 20 {
		t.Fatalf("marker run format=%+v", got.Paragraphs[0].Numbering.RunFormat)
	}
}

func TestLegacyWriterRoundTripsBulletNumbering(t *testing.T) {
	doc := &wordir.Document{
		NumberingDefinitions: []wordir.NumberingDefinition{{
			NumID: 2, AbstractNumID: 202,
			Levels: []wordir.NumberingLevel{{Level: 0, Start: 1, Format: "bullet", Text: "•", IndentLeft: 720}},
		}},
	}
	p1 := wordir.Paragraph{
		Numbering: &wordir.Numbering{NumID: 2, AbstractNumID: 202, Level: 0, Start: 1, Format: "bullet", Text: "•", IndentLeft: 720},
		List:      &wordir.ListMarker{Kind: "bullet", Level: 0, Start: 1, Template: "•"},
		Runs:      []wordir.Run{{Kind: "run", Text: "One"}},
	}
	p2 := wordir.Paragraph{
		Numbering: &wordir.Numbering{NumID: 2, AbstractNumID: 202, Level: 0, Start: 1, Format: "bullet", Text: "•", IndentLeft: 720},
		List:      &wordir.ListMarker{Kind: "bullet", Level: 0, Start: 1, Template: "•"},
		Runs:      []wordir.Run{{Kind: "run", Text: "Two"}},
	}
	doc.Blocks = []wordir.Block{{Kind: "paragraph", Paragraph: &p1}, {Kind: "paragraph", Paragraph: &p2}}
	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	file, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("open CFB: %v", err)
	}
	got, err := parseDocFile(file, "bullet.doc")
	if err != nil {
		t.Fatalf("parseDocFile: %v", err)
	}
	if len(got.Paragraphs) < 2 || got.Paragraphs[0].List == nil || got.Paragraphs[1].List == nil {
		t.Fatalf("bullet paragraphs=%+v", got.Paragraphs)
	}
	for i := 0; i < 2; i++ {
		if got.Paragraphs[i].List.Kind != "bullet" || got.Paragraphs[i].List.Template != "•" {
			t.Fatalf("paragraph[%d] bullet=%+v", i, got.Paragraphs[i].List)
		}
	}
}

func TestLegacyWriterSynthesizesNumberingDefinitionsFromParagraphs(t *testing.T) {
	p := wordir.Paragraph{
		Numbering: &wordir.Numbering{NumID: 11, AbstractNumID: 1111, Level: 0, Start: 5, Format: "upperRoman", Text: "V.", IndentLeft: 900},
		List:      &wordir.ListMarker{Kind: "ordered", Level: 0, Start: 5, Format: "upperRoman", Template: "%1."},
		Runs:      []wordir.Run{{Kind: "run", Text: "Five"}},
	}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	catalog := buildLegacyNumberingWriteCatalog(doc)
	if len(catalog.Definitions) != 1 || catalog.Definitions[0].NumID != 11 || catalog.Definitions[0].AbstractNumID != 1111 {
		t.Fatalf("synthesized catalog=%+v", catalog)
	}
	lst := buildLegacyPlfLstData(catalog)
	lfo := buildLegacyPlfLfoData(catalog)
	if levels, ok := parseLegacyStructuredPlfLstBlob(lst); !ok || levels[1111].start[0] != 5 ||
		legacyNfcToListFormat(levels[1111].format[0]) != "upperRoman" {
		t.Fatalf("synthesized PlfLst parse=%v ok=%v", levels, ok)
	}
	lsids, _, _, _, _, ok := parseLegacyStructuredPlfLfoData(lfo)
	if !ok || lsids[1] != 1111 {
		t.Fatalf("synthesized PlfLfo=%v ok=%v", lsids, ok)
	}
}

func TestLegacyListTemplateEncodingPreservesMultilevelPlaceholders(t *testing.T) {
	xst, rgb := encodeLegacyListTemplateXst("%1.%2)")
	header := make([]byte, legacyLvlfSize)
	copy(header[6:15], rgb[:])
	if got := decodeLegacyLvlfTemplate(header, xst); got != "%1.%2)" {
		t.Fatalf("template=%q", got)
	}
}

func TestLegacyPlfLfoWriterPreservesInstanceOverrides(t *testing.T) {
	start := 7
	restart := true
	restartOn := 0
	c := legacyNumberingWriteCatalog{
		Definitions: []legacyNumberingWriteDefinition{{
			NumID: 3, AbstractNumID: 303, LSID: 303,
			Levels: []legacyNumberingWriteLevel{{Level: 0, Start: 1, Format: "decimal", Text: "%1."}},
			Overrides: map[int]wordir.LegacyNumberingOverride{
				0: {Start: &start, Format: "upperRoman", Restart: &restart, RestartOnLevel: &restartOn},
			},
		}},
		IlfoByNumID: map[int]int{3: 1},
	}
	blob := buildLegacyPlfLfoData(c)
	lsids, starts, formats, restarts, restartOnLevels, ok := parseLegacyStructuredPlfLfoData(blob)
	if !ok {
		t.Fatal("structured PlfLfo did not parse")
	}
	if lsids[1] != 303 {
		t.Fatalf("lsid=%d", lsids[1])
	}
	key := legacyListKey(1, 0)
	if starts[key] != 7 || formats[key] != "upperRoman" || restarts[key] != 1 {
		t.Fatalf("override start/format/restart = %v %v %v", starts, formats, restarts)
	}
	if _, exists := restartOnLevels[key]; exists {
		t.Fatalf("restart threshold should be cleared by explicit restart: %v", restartOnLevels)
	}
}

func TestLegacyPlfLfoWriterPreservesRestartThreshold(t *testing.T) {
	restart := false
	restartOn := 1
	c := legacyNumberingWriteCatalog{
		Definitions: []legacyNumberingWriteDefinition{{
			NumID: 4, AbstractNumID: 404, LSID: 404,
			Levels: []legacyNumberingWriteLevel{{Level: 2, Start: 1, Format: "lowerRoman", Text: "%1.%2.%3."}},
			Overrides: map[int]wordir.LegacyNumberingOverride{
				2: {Restart: &restart, RestartOnLevel: &restartOn},
			},
		}},
		IlfoByNumID: map[int]int{4: 1},
	}
	blob := buildLegacyPlfLfoData(c)
	_, _, _, restarts, restartOnLevels, ok := parseLegacyStructuredPlfLfoData(blob)
	if !ok {
		t.Fatal("structured PlfLfo did not parse")
	}
	key := legacyListKey(1, 2)
	if restarts[key] != legacyListOverrideClear {
		t.Fatalf("restart override=%d want clear", restarts[key])
	}
	if restartOnLevels[key] != 1 {
		t.Fatalf("restartOnLevel=%d want 1", restartOnLevels[key])
	}
}

func TestLegacyNumberingCatalogCarriesParagraphInstanceOverride(t *testing.T) {
	start := 9
	restart := true
	p := wordir.Paragraph{
		Numbering: &wordir.Numbering{
			NumID: 5, AbstractNumID: 505, Level: 0, Start: 9, Format: "decimal",
			LegacyOverride: &wordir.LegacyNumberingOverride{Start: &start, Restart: &restart},
		},
		List: &wordir.ListMarker{Kind: "ordered", Level: 0, Start: 9, Format: "decimal", Template: "%1."},
		Runs: []wordir.Run{{Kind: "run", Text: "Nine"}},
	}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	c := buildLegacyNumberingWriteCatalog(doc)
	if len(c.Definitions) != 1 {
		t.Fatalf("definitions=%d", len(c.Definitions))
	}
	o, ok := c.Definitions[0].Overrides[0]
	if !ok || o.Start == nil || *o.Start != 9 || o.Restart == nil || !*o.Restart {
		t.Fatalf("override=%+v ok=%v", o, ok)
	}
}

func TestLegacyWriterPreservesInlineImagePayloads(t *testing.T) {
	png := testLegacyPNG(2, 3)
	image := &wordir.Image{
		MIMEType: "image/png",
		Data:     png,
		Width:    19050,
		Height:   28575,
		Alt:      "tiny",
	}
	paragraph := wordir.Paragraph{Runs: []wordir.Run{
		{Kind: "run", Text: "Before "},
		{Kind: "image", Image: image},
		{Kind: "run", Text: " after"},
	}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &paragraph}}}

	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	file, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("open CFB: %v", err)
	}
	data, ok := file.Stream("Data")
	if !ok {
		t.Fatal("missing Data stream")
	}
	if !bytes.Contains(data, png) {
		t.Fatal("Data stream does not contain image payload")
	}

	got, err := parseDocFile(file, "inline-image.doc")
	if err != nil {
		t.Fatalf("parseDocFile: %v", err)
	}
	var found *wordir.Image
	for _, paragraph := range got.Paragraphs {
		for _, run := range paragraph.Runs {
			if run.Kind == "image" && run.Image != nil {
				found = run.Image
				break
			}
		}
	}
	if found == nil {
		t.Fatalf("round-tripped document has no image: %+v", got.Paragraphs)
	}
	if found.MIMEType != "image/png" || !bytes.Equal(found.Data, png) {
		t.Fatalf("round-tripped image = mime:%q bytes:%d", found.MIMEType, len(found.Data))
	}
	if found.Width <= 0 || found.Height <= 0 {
		t.Fatalf("round-tripped dimensions = %d x %d", found.Width, found.Height)
	}
}

func TestLegacyWriterMarksImageAnchorAsSpecialCHPX(t *testing.T) {
	png := testLegacyPNG(1, 1)
	paragraph := wordir.Paragraph{Runs: []wordir.Run{{Kind: "image", Image: &wordir.Image{MIMEType: "image/png", Data: png}}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &paragraph}}}
	plan := buildLegacyWritePlan(doc, buildLegacyStyleWriteCatalog(doc), buildLegacyNumberingWriteCatalog(doc))
	if plan.Text != "\x01\r" {
		t.Fatalf("legacy image text = %q", plan.Text)
	}
	if len(plan.Images) != 1 || len(plan.Characters) < 1 || !plan.Characters[0].Spec {
		t.Fatalf("image plan = images:%d characters:%+v", len(plan.Images), plan.Characters)
	}
	grp := encodeChpxWriteRun(plan.Characters[0], legacyFontWriteCatalog{})
	_, meta := parseChpxGrpprlWithMetadata(grp)
	if !meta.Spec {
		t.Fatalf("image CHPX missing sprmCFSpec: % x", grp)
	}
}

func TestLegacyWriterKeepsMultipleImageOrder(t *testing.T) {
	first := testLegacyPNG(1, 2)
	second := testLegacyPNG(3, 4)
	p := wordir.Paragraph{Runs: []wordir.Run{
		{Kind: "image", Image: &wordir.Image{MIMEType: "image/png", Data: first}},
		{Kind: "run", Text: " "},
		{Kind: "image", Image: &wordir.Image{MIMEType: "image/png", Data: second}},
	}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	plan := buildLegacyWritePlan(doc, buildLegacyStyleWriteCatalog(doc), buildLegacyNumberingWriteCatalog(doc))
	data := buildLegacyImageDataStream(plan.Images)
	images := scanLegacyImageBlobs(data)
	if len(images) != 2 {
		t.Fatalf("scanned images=%d", len(images))
	}
	w1, h1 := detectLegacyImageDimensions(images[0], "image/png")
	w2, h2 := detectLegacyImageDimensions(images[1], "image/png")
	if w1 != 1 || h1 != 2 || w2 != 3 || h2 != 4 {
		t.Fatalf("image order/dims = %dx%d, %dx%d", w1, h1, w2, h2)
	}
}

func testLegacyPNG(width, height uint32) []byte {
	var b bytes.Buffer
	b.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	_ = binary.Write(&b, binary.BigEndian, uint32(13))
	b.WriteString("IHDR")
	_ = binary.Write(&b, binary.BigEndian, width)
	_ = binary.Write(&b, binary.BigEndian, height)
	b.Write([]byte{8, 2, 0, 0, 0})
	b.Write(make([]byte, 4)) // ignored CRC in the legacy scanner
	_ = binary.Write(&b, binary.BigEndian, uint32(0))
	b.WriteString("IEND")
	b.Write(make([]byte, 4))
	return b.Bytes()
}

func TestLegacyWriterBuildsFloatingImageFSPAAndOfficeArt(t *testing.T) {
	png := testLegacyPNG(4, 5)
	image := &wordir.Image{
		MIMEType:    "image/png",
		Data:        png,
		Floating:    true,
		XRelative:   "margin",
		YRelative:   "page",
		X:           120 * 635,
		Y:           240 * 635,
		Width:       360 * 635,
		Height:      480 * 635,
		WrapType:    "tight",
		WrapSide:    "right",
		RotationDeg: 27.5,
		FlipH:       true,
		FlipV:       true,
		CropTop:     0.10,
		CropBottom:  0.20,
		CropLeft:    0.15,
		CropRight:   0.25,
	}
	paragraph := wordir.Paragraph{Runs: []wordir.Run{
		{Kind: "run", Text: "A"},
		{Kind: "image", Image: image},
		{Kind: "run", Text: "B"},
	}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &paragraph}}}
	plan := buildLegacyWritePlan(doc, buildLegacyStyleWriteCatalog(doc), buildLegacyNumberingWriteCatalog(doc))
	if plan.Text != "A\bB\r" || len(plan.FloatingImages) != 1 || len(plan.Images) != 0 {
		t.Fatalf("floating plan text=%q floating=%d inline=%d", plan.Text, len(plan.FloatingImages), len(plan.Images))
	}
	if len(plan.Characters) < 2 || !plan.Characters[1].Spec {
		t.Fatalf("drawing anchor CHPX plan=%+v", plan.Characters)
	}

	spa := buildLegacyPlcfSpaData(plan.FloatingImages, len([]rune(plan.Text)))
	anchors, ok := parseLegacyPlcfSpa(spa, len([]rune(plan.Text)))
	if !ok || len(anchors) != 1 {
		t.Fatalf("PlcfSpa parse ok=%v anchors=%+v", ok, anchors)
	}
	a := anchors[0]
	if a.CP != 1 || a.Left != 120 || a.Top != 240 || a.Right != 480 || a.Bottom != 720 {
		t.Fatalf("FSPA geometry=%+v", a)
	}
	if legacyWrapTypeFromSpaFlags(a.Flags) != "tight" || legacyWrapSideFromSpaFlags(a.Flags) != "right" ||
		legacyAnchorRelativeFromSpaFlags(a.Flags, false) != "margin" ||
		legacyAnchorRelativeFromSpaFlags(a.Flags, true) != "page" {
		t.Fatalf("FSPA flags=%#x", a.Flags)
	}

	office := buildLegacyFloatingImageOfficeArt(plan.FloatingImages)
	images := extractLegacyDrawingImages(office)
	props := extractLegacyDrawingShapeProps(office)
	if len(images) != 1 || !bytes.Equal(images[0].Data, png) {
		t.Fatalf("OfficeArt images=%d", len(images))
	}
	prop, ok := props[plan.FloatingImages[0].Shape]
	if !ok {
		t.Fatalf("shape props missing: %+v", props)
	}
	if !prop.FlipH || !prop.FlipV || !prop.HasRotation || math.Abs(prop.Rotation-27.5) > 0.01 ||
		math.Abs(prop.CropTop-0.10) > 0.001 || math.Abs(prop.CropBottom-0.20) > 0.001 ||
		math.Abs(prop.CropLeft-0.15) > 0.001 || math.Abs(prop.CropRight-0.25) > 0.001 {
		t.Fatalf("shape props=%+v", prop)
	}

	cloned := legacyCloneDrawingImage(images[0], a, props)
	if cloned == nil || !cloned.Floating || cloned.XRelative != "margin" || cloned.YRelative != "page" ||
		cloned.WrapType != "tight" || cloned.WrapSide != "right" ||
		cloned.X != 120*635 || cloned.Y != 240*635 || cloned.Width != 360*635 || cloned.Height != 480*635 ||
		!cloned.FlipH || !cloned.FlipV || math.Abs(cloned.RotationDeg-27.5) > 0.01 {
		t.Fatalf("reconstructed floating image=%+v", cloned)
	}
}

func TestLegacyWriterPreservesFloatingImageBehindText(t *testing.T) {
	image := &wordir.Image{
		MIMEType: "image/png", Data: testLegacyPNG(1, 1), Floating: true,
		XRelative: "page", YRelative: "paragraph",
		X: 10 * 635, Y: 20 * 635, Width: 30 * 635, Height: 40 * 635,
		WrapType: "none", BehindDoc: true,
	}
	plan := legacyWritePlan{Text: "\b\r", FloatingImages: []legacyFloatingImageWrite{{CP: 0, Shape: 0x501, Image: image}}}
	spa := buildLegacyPlcfSpaData(plan.FloatingImages, len([]rune(plan.Text)))
	anchors, ok := parseLegacyPlcfSpa(spa, len([]rune(plan.Text)))
	if !ok || len(anchors) != 1 {
		t.Fatalf("anchors=%+v ok=%v", anchors, ok)
	}
	if anchors[0].Flags&0x4000 == 0 || legacyWrapTypeFromSpaFlags(anchors[0].Flags) != "none" {
		t.Fatalf("behind-text FSPA flags=%#x", anchors[0].Flags)
	}
	cloned := legacyCloneDrawingImage(image, anchors[0], nil)
	if cloned == nil || !cloned.BehindDoc {
		t.Fatalf("reconstructed behind-doc=%+v", cloned)
	}
}

func TestLegacyWriterFallsBackUnsupportedFloatingImageToInline(t *testing.T) {
	gif := []byte{'G', 'I', 'F', '8', '9', 'a', 1, 0, 1, 0, 0, 0, 0, 0x3b}
	image := &wordir.Image{MIMEType: "image/gif", Data: gif, Floating: true}
	p := wordir.Paragraph{Runs: []wordir.Run{{Kind: "image", Image: image}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	plan := buildLegacyWritePlan(doc, buildLegacyStyleWriteCatalog(doc), buildLegacyNumberingWriteCatalog(doc))
	if plan.Text != "\x01\r" || len(plan.Images) != 1 || len(plan.FloatingImages) != 0 {
		t.Fatalf("unsupported floating fallback text=%q inline=%d floating=%d", plan.Text, len(plan.Images), len(plan.FloatingImages))
	}
}

func TestLegacyWriterPreservesFloatingOfficeArtShape(t *testing.T) {
	shape := &wordir.Shape{
		Kind: "triangle", Floating: true,
		XRelative: "margin", YRelative: "page",
		X: 100 * 635, Y: 200 * 635, Width: 300 * 635, Height: 600 * 635,
		WrapType: "through", WrapSide: "largest",
		FillColor: "FF0000", StrokeColor: "0000FF", StrokeWidth: 25400,
		RotationDeg: 30, FlipH: true,
	}
	paragraph := wordir.Paragraph{Runs: []wordir.Run{
		{Kind: "run", Text: "A"},
		{Kind: "shape", Shape: shape},
		{Kind: "run", Text: "B"},
	}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &paragraph}}}
	plan := buildLegacyWritePlan(doc, buildLegacyStyleWriteCatalog(doc), buildLegacyNumberingWriteCatalog(doc))
	if plan.Text != "A\bB\r" || len(plan.FloatingShapes) != 1 {
		t.Fatalf("shape plan text=%q shapes=%d", plan.Text, len(plan.FloatingShapes))
	}
	if len(plan.Characters) < 2 || !plan.Characters[1].Spec {
		t.Fatalf("shape anchor CHPX plan=%+v", plan.Characters)
	}

	spa := buildLegacyPlcfSpaDataWithShapes(nil, plan.FloatingShapes, len([]rune(plan.Text)))
	anchors, ok := parseLegacyPlcfSpa(spa, len([]rune(plan.Text)))
	if !ok || len(anchors) != 1 {
		t.Fatalf("shape anchors=%+v ok=%v", anchors, ok)
	}
	anchor := anchors[0]
	if anchor.CP != 1 || anchor.Left != 100 || anchor.Top != 200 || anchor.Right != 400 || anchor.Bottom != 800 {
		t.Fatalf("shape anchor=%+v", anchor)
	}
	if legacyWrapTypeFromSpaFlags(anchor.Flags) != "through" || legacyWrapSideFromSpaFlags(anchor.Flags) != "largest" {
		t.Fatalf("shape anchor flags=%#x", anchor.Flags)
	}

	officeArt := buildLegacyFloatingShapeOfficeArt(plan.FloatingShapes)
	props := extractLegacyDrawingShapeProps(officeArt)
	property, ok := props[plan.FloatingShapes[0].ShapeID]
	if !ok {
		t.Fatalf("shape property missing: %+v", props)
	}
	if property.ShapeType != 5 || property.FillColor != "FF0000" || property.StrokeColor != "0000FF" ||
		property.StrokeWidth != 25400 || !property.FlipH || property.FlipV ||
		!property.HasRotation || math.Abs(property.Rotation-30) > 0.01 {
		t.Fatalf("shape property=%+v", property)
	}

	roundTripped := legacyCloneDrawingShape(anchor, props)
	if roundTripped == nil || roundTripped.Kind != "triangle" || roundTripped.FillColor != "FF0000" ||
		roundTripped.StrokeColor != "0000FF" || roundTripped.X != 100*635 || roundTripped.Y != 200*635 ||
		roundTripped.Width != 300*635 || roundTripped.Height != 600*635 || !roundTripped.FlipH {
		t.Fatalf("round-tripped shape=%+v", roundTripped)
	}
}

func TestLegacyWriterFallsBackUnknownOfficeArtShapeToRectangle(t *testing.T) {
	shape := &wordir.Shape{Kind: "unsupportedCustomGeometry", Floating: true, Width: 100 * 635, Height: 100 * 635}
	record := buildLegacyShapeSpContainer(0x901, shape)
	props := extractLegacyDrawingShapeProps(record)
	if props[0x901].ShapeType != 1 {
		t.Fatalf("fallback shape type=%d, want rect (1)", props[0x901].ShapeType)
	}
}

func TestWriteDocumentExposesOfficeArtShapeToLegacyReader(t *testing.T) {
	shape := &wordir.Shape{
		Kind: "diamond", Floating: true,
		X: 20 * 635, Y: 30 * 635, Width: 200 * 635, Height: 120 * 635,
		WrapType: "square", FillColor: "12AB34", StrokeColor: "9876EF",
	}
	paragraph := wordir.Paragraph{Runs: []wordir.Run{{Kind: "shape", Shape: shape}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &paragraph}}}

	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	file, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("open CFB: %v", err)
	}
	parsed, err := parseDocFile(file, "shape.doc")
	if err != nil {
		t.Fatalf("parseDocFile: %v", err)
	}
	for _, candidate := range parsed.Paragraphs {
		for _, run := range candidate.Runs {
			if run.Kind == "shape" && run.Shape != nil {
				if run.Shape.Kind != "diamond" || run.Shape.FillColor != "12AB34" || run.Shape.StrokeColor != "9876EF" {
					t.Fatalf("round-tripped shape=%+v", run.Shape)
				}
				return
			}
		}
	}
	t.Fatalf("round-tripped document has no shape: %+v", parsed.Paragraphs)
}

func TestLegacyWriterRoundTripsCharacterStyleAssignment(t *testing.T) {
	italic := true
	paragraph := wordir.Paragraph{Runs: []wordir.Run{{
		Kind: "run", Text: "Emphasized", StyleID: "Emphasis",
	}}}
	doc := &wordir.Document{
		Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &paragraph}},
		CharacterStyles: map[string]wordir.CharacterStyle{
			"Emphasis": {ID: "Emphasis", Name: "Emphasis", Format: wordir.RunFormat{Italic: &italic}},
		},
	}
	catalog := buildLegacyStyleWriteCatalog(doc)
	plan := buildLegacyWritePlan(doc, catalog, buildLegacyNumberingWriteCatalog(doc))
	if len(plan.Characters) == 0 || plan.Characters[0].StyleIndex == nil {
		t.Fatalf("character plan=%+v, want style index", plan.Characters)
	}
	parsedCHPX := parseChpxGrpprl(encodeChpxWriteRun(plan.Characters[0], legacyFontWriteCatalog{}))
	if parsedCHPX.BinaryStyleIndex == nil || *parsedCHPX.BinaryStyleIndex != int(*plan.Characters[0].StyleIndex) {
		t.Fatalf("CHPX style index=%v, want %d", parsedCHPX.BinaryStyleIndex, *plan.Characters[0].StyleIndex)
	}

	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	file, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("open CFB: %v", err)
	}
	roundTripped, err := parseDocFile(file, "character-style.doc")
	if err != nil {
		t.Fatalf("parseDocFile: %v", err)
	}
	if len(roundTripped.Paragraphs) == 0 || len(roundTripped.Paragraphs[0].Runs) == 0 {
		t.Fatalf("round-tripped paragraphs=%+v", roundTripped.Paragraphs)
	}
	run := roundTripped.Paragraphs[0].Runs[0]
	if run.StyleID != "Emphasis" {
		t.Fatalf("run StyleID=%q, want Emphasis", run.StyleID)
	}
	if run.Source == nil || run.Source.StyleIndex == nil {
		t.Fatalf("run source=%+v, want raw style provenance", run.Source)
	}
}

func TestLegacyChpxWriterPreservesAdvancedRunFormatting(t *testing.T) {
	webHidden, complexScript, usePageSetup := true, true, true
	fit := 720
	border := &wordir.BorderSide{Val: "double", SizePt: 1.5, SpacePt: 2, Color: "000000"}
	format := wordir.RunFormat{
		WebHidden: &webHidden, ComplexScript: &complexScript, UsePageSetup: &usePageSetup,
		FitTextWidth: &fit,
		Borders:      &wordir.RunBorders{Top: border, Left: border, Bottom: border, Right: border},
	}
	got := parseChpxGrpprl(encodeChpxGrpprl(format))
	if got.WebHidden == nil || !*got.WebHidden || got.ComplexScript == nil || !*got.ComplexScript || got.UsePageSetup == nil || !*got.UsePageSetup {
		t.Fatalf("advanced toggles=%+v", got)
	}
	if got.FitTextWidth == nil || *got.FitTextWidth != fit {
		t.Fatalf("fitText=%v want %d", got.FitTextWidth, fit)
	}
	if got.Borders == nil || got.Borders.Top == nil || got.Borders.Top.Val != "double" {
		t.Fatalf("run borders=%+v", got.Borders)
	}
}

func TestLegacyPapxWriterPreservesLineBordersAndFrame(t *testing.T) {
	line := 360
	border := &wordir.BorderSide{Val: "dashed", SizePt: 1, SpacePt: 3, Color: "000000"}
	format := wordir.ParagraphFormat{
		LineSpacing: &line, LineRule: "atLeast",
		Borders: &wordir.ParagraphBorders{Top: border, Left: border, Bottom: border, Right: border},
		BinaryFrame: &wordir.FrameProps{
			W: 900, H: 800, X: 300, Y: 400, XAlign: "center", YAlign: "bottom",
			WrapType: "tight", HSpace: 80, VSpace: 60, DropCap: "margin", Lines: 3,
			TextDirection: "tbRl", AnchorLock: true,
		},
	}
	got := parsePapxGrpprl(encodePapxGrpprl(format))
	if got.LineSpacing == nil || *got.LineSpacing != line || got.LineRule != "atLeast" {
		t.Fatalf("line spacing=%v/%q", got.LineSpacing, got.LineRule)
	}
	if got.Borders == nil || got.Borders.Top == nil || got.Borders.Top.Val != "dashed" {
		t.Fatalf("paragraph borders=%+v", got.Borders)
	}
	if got.BinaryFrame == nil {
		t.Fatal("binary frame missing")
	}
	frame := got.BinaryFrame
	if frame.W != 900 || frame.H != 800 || frame.X != 300 || frame.Y != 400 || frame.XAlign != "center" || frame.YAlign != "bottom" ||
		frame.WrapType != "tight" || frame.HSpace != 80 || frame.VSpace != 60 || frame.DropCap != "margin" || frame.Lines != 3 ||
		frame.TextDirection != "tbRl" || !frame.AnchorLock {
		t.Fatalf("binary frame=%+v", frame)
	}
}

func TestLegacyWriterPreservesShapeTextBoxStoryAndTextFlow(t *testing.T) {
	textParagraph := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: "Inside shape"}}}
	shape := &wordir.Shape{
		Kind: "diamond", Floating: true,
		X: 10 * 635, Y: 20 * 635, Width: 120 * 635, Height: 80 * 635,
		FillColor: "ABCDEF", StrokeColor: "123456",
		TextBox: &wordir.TextBox{
			Vert:   "vert270",
			Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &textParagraph}},
		},
	}
	body := wordir.Paragraph{Runs: []wordir.Run{{Kind: "shape", Shape: shape}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &body}}}
	styles := buildLegacyStyleWriteCatalog(doc)
	plan := buildLegacyWritePlan(doc, styles, buildLegacyNumberingWriteCatalog(doc))
	if plan.MainTextLen != 2 || plan.TextBoxText != "Inside shape\r" {
		t.Fatalf("plan main=%d txbx=%q", plan.MainTextLen, plan.TextBoxText)
	}
	if len(plan.FloatingShapes) != 1 || plan.FloatingShapes[0].Shape.TextBox == nil {
		t.Fatalf("floating shapes=%+v", plan.FloatingShapes)
	}
	flows := extractLegacyDrawingTextFlows(buildLegacyFloatingShapeOfficeArt(plan.FloatingShapes))
	if got := flows[plan.FloatingShapes[0].ShapeID]; got != "vert270" {
		t.Fatalf("text flow=%q, want vert270", got)
	}
}

func TestLegacyWriterSplitsMultipleTextBoxStories(t *testing.T) {
	mk := func(text string) *wordir.TextBox {
		p := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: text}}}
		return &wordir.TextBox{Vert: "vert", Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	}
	var plan legacyWritePlan
	appendLegacyTextBoxStory(&plan, mk("one"))
	appendLegacyTextBoxStory(&plan, mk("two"))
	if plan.TextBoxText != "one\r\ftwo\r" {
		t.Fatalf("text box story=%q", plan.TextBoxText)
	}
	runs := []docRun{{Text: plan.TextBoxText, StartCP: 100}}
	groups := splitLegacyTextBoxStoryRuns(runs)
	if len(groups) != 2 || len(groups[0]) != 1 || len(groups[1]) != 1 || groups[0][0].Text != "one\r" || groups[1][0].Text != "two\r" {
		t.Fatalf("split groups=%+v", groups)
	}
}

func TestLegacyWriterEmitsTrueGroupedShapeHierarchy(t *testing.T) {
	group := &wordir.Shape{
		Kind: "group", Floating: true, X: 1000, Y: 2000, Width: 5000, Height: 4000,
		Children: []wordir.Shape{
			{Kind: "rect", X: 10 * 635, Y: 20 * 635, Width: 100 * 635, Height: 50 * 635, FillColor: "FF0000"},
			{Kind: "ellipse", X: 30 * 635, Y: 40 * 635, Width: 80 * 635, Height: 80 * 635, FillColor: "00FF00"},
		},
	}
	body := wordir.Paragraph{Runs: []wordir.Run{{Kind: "shape", Shape: group}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &body}}}
	plan := buildLegacyWritePlan(doc, buildLegacyStyleWriteCatalog(doc), buildLegacyNumberingWriteCatalog(doc))
	if len(plan.FloatingShapes) != 1 {
		t.Fatalf("floating shape count=%d, want one anchored group", len(plan.FloatingShapes))
	}
	officeArt := buildLegacyFloatingShapeOfficeArt(plan.FloatingShapes)
	trees := extractLegacyDrawingShapeTrees(officeArt)
	root, ok := trees[plan.FloatingShapes[0].ShapeID]
	if !ok {
		t.Fatalf("group root %08x missing from OfficeArt tree", plan.FloatingShapes[0].ShapeID)
	}
	if len(root.Children) != 2 {
		t.Fatalf("group child count=%d, want 2", len(root.Children))
	}
	if root.Children[0].Bounds == nil || root.Children[0].Bounds.Left != 10 || root.Children[0].Bounds.Top != 20 {
		t.Fatalf("first child bounds=%+v", root.Children[0].Bounds)
	}
}

func TestLegacyOfficeArtPathInfoUsesMSODRAWBitLayout(t *testing.T) {
	if got := legacyOfficeArtPathInfo(legacyOfficeArtPathMoveTo, 0); got != 0x0002 {
		t.Fatalf("move path info=%#04x, want 0x0002", got)
	}
	if got := legacyOfficeArtPathInfo(legacyOfficeArtPathLineTo, 1); got != 0x0008 {
		t.Fatalf("line path info=%#04x, want 0x0008", got)
	}
	if got := legacyOfficeArtPathInfo(legacyOfficeArtPathCurveTo, 1); got != 0x0009 {
		t.Fatalf("curve path info=%#04x, want 0x0009", got)
	}
	if got := legacyOfficeArtPathInfo(legacyOfficeArtPathClose, 1); got != 0x000B {
		t.Fatalf("close path info=%#04x, want 0x000B", got)
	}
	if got := legacyOfficeArtPathInfo(legacyOfficeArtPathEnd, 0); got != 0x0004 {
		t.Fatalf("end path info=%#04x, want 0x0004", got)
	}
}

func TestLegacyWriterRoundTripsCustomOfficeArtGeometryProperties(t *testing.T) {
	geometry := &dml.Geometry{Type: dml.GeometryTypeCustom, Paths: []dml.Path{
		{Commands: []dml.PathCommand{
			{Type: "M", Points: []dml.Point{{X: 0.1, Y: 0.2}}},
			{Type: "L", Points: []dml.Point{{X: 0.8, Y: 0.2}}},
			{Type: "C", Points: []dml.Point{{X: 0.9, Y: 0.3}, {X: 0.9, Y: 0.8}, {X: 0.5, Y: 0.9}}},
			{Type: "Q", Points: []dml.Point{{X: 0.2, Y: 0.8}, {X: 0.1, Y: 0.5}}},
			{Type: "Z"},
		}},
		{Commands: []dml.PathCommand{
			{Type: "M", Points: []dml.Point{{X: 0.25, Y: 0.25}}},
			{Type: "L", Points: []dml.Point{{X: 0.75, Y: 0.75}}},
		}},
	}}
	shape := &wordir.Shape{Kind: "custom", GeometryData: geometry, FillColor: "AABBCC", StrokeColor: "112233"}
	container := buildLegacyShapeSpContainer(0x12345678, shape, nil, false)
	props := extractLegacyDrawingShapeProps(container)
	got, ok := props[0x12345678]
	if !ok || got.GeometryData == nil {
		t.Fatalf("custom geometry missing after FOPT decode: %+v", got)
	}
	if got.GeometryData.Type != dml.GeometryTypeCustom || len(got.GeometryData.Paths) != 2 {
		t.Fatalf("geometry=%+v", got.GeometryData)
	}
	commands := got.GeometryData.Paths[0].Commands
	if len(commands) < 5 || commands[0].Type != "M" || commands[1].Type != "L" || commands[2].Type != "C" || commands[3].Type != "C" || commands[4].Type != "Z" {
		t.Fatalf("commands=%+v", commands)
	}
	if math.Abs(commands[0].Points[0].X-0.1) > 0.001 || math.Abs(commands[0].Points[0].Y-0.2) > 0.001 {
		t.Fatalf("move point=%+v", commands[0].Points[0])
	}
}

func TestLegacyWriterPreservesNestedGroupTextBoxStories(t *testing.T) {
	makeTextBox := func(text string) *wordir.TextBox {
		p := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: text}}}
		return &wordir.TextBox{Vert: "vert", Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	}
	group := &wordir.Shape{Kind: "group", Width: 1000 * 635, Height: 600 * 635, Children: []wordir.Shape{
		{Kind: "rect", X: 10 * 635, Y: 20 * 635, Width: 200 * 635, Height: 100 * 635, TextBox: makeTextBox("first child")},
		{Kind: "group", X: 250 * 635, Y: 30 * 635, Width: 300 * 635, Height: 200 * 635, Children: []wordir.Shape{
			{Kind: "ellipse", X: 20 * 635, Y: 10 * 635, Width: 80 * 635, Height: 80 * 635, TextBox: makeTextBox("nested child")},
		}},
	}}
	body := wordir.Paragraph{Runs: []wordir.Run{{Kind: "shape", Shape: group}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &body}}}
	plan := buildLegacyWritePlan(doc, buildLegacyStyleWriteCatalog(doc), buildLegacyNumberingWriteCatalog(doc))
	if plan.TextBoxText != "first child\r\fnested child\r" {
		t.Fatalf("textbox story=%q", plan.TextBoxText)
	}
	officeArt := buildLegacyFloatingShapeOfficeArt(plan.FloatingShapes)
	flows := extractLegacyDrawingTextFlows(officeArt)
	if len(flows) != 2 {
		t.Fatalf("text-flow shape count=%d, want 2", len(flows))
	}
}

func TestWriteDocumentRoundTripsTextBearingShape(t *testing.T) {
	inside := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: "Legacy textbox"}}}
	shape := &wordir.Shape{
		Kind: "roundRect", Floating: true,
		X: 200 * 635, Y: 300 * 635, Width: 180 * 635, Height: 90 * 635,
		FillColor: "FFEEDD", StrokeColor: "112233",
		TextBox: &wordir.TextBox{Vert: "vert", Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &inside}}},
	}
	body := wordir.Paragraph{Runs: []wordir.Run{{Kind: "shape", Shape: shape}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &body}}}
	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	file, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("open CFB: %v", err)
	}
	got, err := parseDocFile(file, "shape-text.doc")
	if err != nil {
		t.Fatalf("parseDocFile: %v", err)
	}
	for _, p := range got.Paragraphs {
		for _, r := range p.Runs {
			if r.Kind == "shape" && r.Shape != nil && r.Shape.TextBox != nil {
				if len(r.Shape.TextBox.Blocks) == 0 || r.Shape.TextBox.Blocks[0].Paragraph == nil || !strings.Contains(r.Shape.TextBox.Blocks[0].Paragraph.Text, "Legacy textbox") {
					t.Fatalf("shape textbox blocks=%+v", r.Shape.TextBox.Blocks)
				}
				return
			}
		}
	}
	t.Fatal("round-tripped text-bearing shape not found")
}

func TestWriteDocumentRoundTripsGroupedCustomShapeAndNestedText(t *testing.T) {
	inside := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: "Grouped custom text"}}}
	custom := dml.Geometry{Type: dml.GeometryTypeCustom, Paths: []dml.Path{{Commands: []dml.PathCommand{
		{Type: "M", Points: []dml.Point{{X: 0, Y: 0.5}}},
		{Type: "L", Points: []dml.Point{{X: 0.5, Y: 0}}},
		{Type: "L", Points: []dml.Point{{X: 1, Y: 0.5}}},
		{Type: "L", Points: []dml.Point{{X: 0.5, Y: 1}}},
		{Type: "Z"},
	}}}}
	group := &wordir.Shape{
		Kind: "group", Floating: true, X: 100 * 635, Y: 120 * 635, Width: 400 * 635, Height: 250 * 635,
		Children: []wordir.Shape{
			{Kind: "custom", X: 20 * 635, Y: 30 * 635, Width: 140 * 635, Height: 120 * 635, GeometryData: &custom, FillColor: "CC8844", StrokeColor: "331100", TextBox: &wordir.TextBox{Vert: "vert270", Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &inside}}}},
			{Kind: "ellipse", X: 190 * 635, Y: 40 * 635, Width: 100 * 635, Height: 100 * 635, FillColor: "88AACC"},
		},
	}
	body := wordir.Paragraph{Runs: []wordir.Run{{Kind: "shape", Shape: group}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &body}}}
	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	file, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("open CFB: %v", err)
	}
	got, err := parseDocFile(file, "group-custom.doc")
	if err != nil {
		t.Fatalf("parseDocFile: %v", err)
	}
	for _, p := range got.Paragraphs {
		for _, r := range p.Runs {
			if r.Kind != "shape" || r.Shape == nil || r.Shape.Kind != "group" {
				continue
			}
			if len(r.Shape.Children) != 2 {
				t.Fatalf("group children=%d, want 2", len(r.Shape.Children))
			}
			child := &r.Shape.Children[0]
			if child.GeometryData == nil || child.GeometryData.Type != dml.GeometryTypeCustom {
				t.Fatalf("custom child geometry=%+v", child.GeometryData)
			}
			if child.TextBox == nil || len(child.TextBox.Blocks) == 0 || child.TextBox.Blocks[0].Paragraph == nil || !strings.Contains(child.TextBox.Blocks[0].Paragraph.Text, "Grouped custom text") {
				t.Fatalf("nested textbox=%+v", child.TextBox)
			}
			return
		}
	}
	t.Fatal("round-tripped grouped shape not found")
}

func TestLegacyFontTableWritePreservesNamesAndMetadata(t *testing.T) {
	catalog := legacyFontWriteCatalog{
		Names: []string{"Aptos", "MS Mincho", "Simplified Arabic"},
		Index: map[string]uint16{"aptos": 0, "ms mincho": 1, "simplified arabic": 2},
		Info: map[string]wordir.FontFamilyInfo{
			"aptos": {Family: "swiss", Pitch: "variable", Charset: "00", Panose1: "020B0604020202020204"},
		},
	}
	blob := buildLegacyFontTableData(catalog)
	entries := parseLegacyFontTableBlobEntries(blob)
	if len(entries) != 3 {
		t.Fatalf("font entries=%d, want 3", len(entries))
	}
	if entries[0].Name != "Aptos" || entries[1].Name != "MS Mincho" || entries[2].Name != "Simplified Arabic" {
		t.Fatalf("font names=%+v", entries)
	}
	if entries[0].Info.Family != "swiss" || entries[0].Info.Pitch != "variable" || entries[0].Info.Panose1 != "020B0604020202020204" {
		t.Fatalf("font metadata=%+v", entries[0].Info)
	}
}

func TestEncodeChpxWriteRunWritesFontIndexesAndEastAsianLayout(t *testing.T) {
	trueValue := true
	format := wordir.RunFormat{
		FontASCII: "Aptos", FontEastAsia: "MS Mincho", FontCS: "Simplified Arabic",
		TextAnimation: "antsBlack", SpecVanish: &trueValue,
		FELayout: &wordir.FELayout{CombineCharacters: true, CombineBrackets: true, Vertical: true, VerticalCompressed: true, CombineCharCount: 3},
	}
	fonts := legacyFontWriteCatalog{Index: map[string]uint16{"aptos": 1, "ms mincho": 2, "simplified arabic": 3}}
	grp := encodeChpxWriteRun(legacyCharacterWriteRun{Format: format}, fonts)
	for _, want := range [][]byte{
		{0x4F, 0x4A, 0x01, 0x00}, {0x50, 0x4A, 0x02, 0x00}, {0x51, 0x4A, 0x03, 0x00},
		{0x59, 0x28, 0x04}, {0x18, 0x08, 0x01}, {0x78, 0xCA, 0x02, 0x0F, 0x03},
	} {
		if !bytes.Contains(grp, want) {
			t.Fatalf("CHPX %x missing %x", grp, want)
		}
	}
}

func TestEncodePapxGrpprlWritesLegacyEastAsianAndNestedTableProperties(t *testing.T) {
	f := wordir.ParagraphFormat{UsePageSetup: true, IsTableCellTrailingParagraph: true, FontAlign: "baseline"}
	grp := encodePapxGrpprl(f)
	for _, want := range [][]byte{{0x47, 0x24, 0x01}, {0x4B, 0x24, 0x01}, {0x39, 0x44, 0x03, 0x00}} {
		if !bytes.Contains(grp, want) {
			t.Fatalf("PAPX %x missing %x", grp, want)
		}
	}
}

func TestLegacyOfficeArtPresetAdjustmentsRoundTripPropertyTable(t *testing.T) {
	shape := &wordir.Shape{Kind: "roundRect", Geometry: &dml.PresetGeometry{Name: "roundRect", Adjustments: []float64{12500, 25000, -5000}}}
	props := legacyShapeOfficeArtProperties(shape)
	fopt := buildLegacyOfficeArtFOPT(props)
	if len(fopt) < 8 {
		t.Fatal("empty FOPT")
	}
	var parsed legacyOfficeArtShapeProps
	legacyApplyOfficeArtPropertyTable(fopt[8:], len(props), &parsed)
	want := []int32{12500, 25000, -5000}
	for i, v := range want {
		if parsed.adjustments[i] == nil || *parsed.adjustments[i] != v {
			t.Fatalf("adjustment[%d]=%v, want %d", i, parsed.adjustments[i], v)
		}
	}
}

func TestWriteDocumentEmitsLegacyFontTableFIBSlot(t *testing.T) {
	p := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: "font", Format: wordir.RunFormat{FontASCII: "Aptos", FontEastAsia: "MS Mincho"}}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}, FontFamilies: map[string]wordir.FontFamilyInfo{"aptos": {Family: "swiss", Pitch: "variable"}}}
	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	cf, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("cfb parse: %v", err)
	}
	wordStream, ok := cf.Stream("WordDocument")
	if !ok {
		t.Fatal("WordDocument missing")
	}
	tableStream, ok := cf.Stream("1Table")
	if !ok {
		t.Fatal("1Table missing")
	}
	// The writer uses the fixed Word 97 FIB layout: Fc/Lcb pairs begin at byte 154.
	names := loadLegacyFontNames(wordStream, tableStream, 154, 136)
	if len(names) < 2 || names[0] != "Aptos" || names[1] != "MS Mincho" {
		t.Fatalf("font names=%v", names)
	}
}

func TestLegacyTabStopsRoundTripAlignmentAndLeader(t *testing.T) {
	want := []wordir.TabStop{
		{Pos: 720, Algn: "left", Leader: "dot"},
		{Pos: 1440, Algn: "center", Leader: "hyphen"},
		{Pos: 2160, Algn: "right", Leader: "underscore"},
		{Pos: 2880, Algn: "decimal", Leader: "middleDot"},
	}
	payload := encodeLegacyTabStops(want)
	got := applyLegacyTabChange(nil, payload, 2)
	if len(got) != len(want) {
		t.Fatalf("tab count=%d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tab[%d]=%+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestLegacyStyleRunGrpprlCarriesFontReference(t *testing.T) {
	fonts := legacyFontWriteCatalog{Index: map[string]uint16{"aptos": 7}}
	grp := encodeLegacyStyleRunGrpprl(wordir.RunFormat{FontASCII: "Aptos"}, fonts)
	if !bytes.Contains(grp, []byte{0x4F, 0x4A, 0x07, 0x00}) {
		t.Fatalf("style CHPX %x missing ASCII font reference", grp)
	}
}

func TestLegacySecondaryStoryPlanCorrelatesNotesCommentsAndHeaders(t *testing.T) {
	body := wordir.Paragraph{Runs: []wordir.Run{
		{Kind: "run", Text: "Body"},
		{Kind: "footnoteRef", NoteID: 1},
		{Kind: "commentRef", CommentID: 1},
		{Kind: "endnoteRef", NoteID: 1},
	}}
	doc := &wordir.Document{
		Blocks:    []wordir.Block{{Kind: "paragraph", Paragraph: &body}},
		Footnotes: map[int]wordir.Story{1: {Text: "Footnote text"}},
		Comments:  map[int]wordir.Story{1: {Text: "Comment text"}},
		Endnotes:  map[int]wordir.Story{1: {Text: "Endnote text"}},
		Headers:   map[string]wordir.Story{"header-default": {Text: "Header text"}},
		Footers:   map[string]wordir.Story{"footer-default": {Text: "Footer text"}},
		Sections: []wordir.Section{{
			Headers: []wordir.Reference{{Type: "default", PartName: "header-default"}},
			Footers: []wordir.Reference{{Type: "default", PartName: "footer-default"}},
		}},
	}
	styles := buildLegacyStyleWriteCatalog(doc)
	plan := buildLegacyWritePlan(doc, styles, buildLegacyNumberingWriteCatalog(doc))
	if len(plan.FootnoteRefs) != 1 || len(plan.CommentRefs) != 1 || len(plan.EndnoteRefs) != 1 {
		t.Fatalf("refs: footnote=%v comment=%v endnote=%v", plan.FootnoteRefs, plan.CommentRefs, plan.EndnoteRefs)
	}
	if !strings.Contains(plan.FootnoteText, "Footnote text") || !strings.Contains(plan.CommentText, "Comment text") || !strings.Contains(plan.EndnoteText, "Endnote text") {
		t.Fatalf("secondary texts missing: ftn=%q atn=%q edn=%q", plan.FootnoteText, plan.CommentText, plan.EndnoteText)
	}
	if !strings.Contains(plan.HeaderText, "Header text") || !strings.Contains(plan.HeaderText, "Footer text") {
		t.Fatalf("header story=%q", plan.HeaderText)
	}
	if len(plan.HeaderBounds) != 13 { // 6 separators + 6 slots + terminal origin
		t.Fatalf("header bounds=%v, want 13 boundaries", plan.HeaderBounds)
	}
}

func TestLegacyNoteAndCommentPLCsHaveCanonicalRecordLayouts(t *testing.T) {
	refs := []int{4, 12}
	note := buildLegacyRefPLC(refs, false)
	if got := parseLegacyNoteRefCPs(note); len(got) != 2 || got[0] != 4 || got[1] != 12 {
		t.Fatalf("note ref PLC parses as %v", got)
	}
	comment := buildLegacyRefPLC(refs, true)
	if got := parseLegacyAnnotationRefCPs(comment); len(got) != 2 || got[0] != 4 || got[1] != 12 {
		t.Fatalf("comment ref PLC parses as %v", got)
	}
	text := buildLegacyTextPLC([]int{0, 7, 19})
	if got := parseLegacyNoteTextCPs(text); len(got) != 3 || got[2] != 19 {
		t.Fatalf("text PLC parses as %v", got)
	}
}

func TestWriteDocumentEmitsSecondaryStoryCountsAndPLCFIBSlots(t *testing.T) {
	body := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: "B"}, {Kind: "footnoteRef", NoteID: 1}, {Kind: "commentRef", CommentID: 1}, {Kind: "endnoteRef", NoteID: 1}}}
	doc := &wordir.Document{
		Blocks:    []wordir.Block{{Kind: "paragraph", Paragraph: &body}},
		Footnotes: map[int]wordir.Story{1: {Text: "FN"}}, Comments: map[int]wordir.Story{1: {Text: "CM"}}, Endnotes: map[int]wordir.Story{1: {Text: "EN"}},
		Headers: map[string]wordir.Story{"h": {Text: "H"}}, Sections: []wordir.Section{{Headers: []wordir.Reference{{Type: "default", PartName: "h"}}}},
	}
	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	cf, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("cfb: %v", err)
	}
	ws, _ := cf.Stream("WordDocument")
	ts, _ := cf.Stream("1Table")
	// FibRgLw97 begins after FibBase + FibRgW; counts are at fixed offsets in our writer.
	pos := 32
	csw := int(binary.LittleEndian.Uint16(ws[pos : pos+2]))
	lw := pos + 2 + csw*2 + 2
	if binary.LittleEndian.Uint32(ws[lw+16:]) == 0 || binary.LittleEndian.Uint32(ws[lw+20:]) == 0 || binary.LittleEndian.Uint32(ws[lw+28:]) == 0 || binary.LittleEndian.Uint32(ws[lw+32:]) == 0 {
		t.Fatalf("secondary story counts not populated")
	}
	cslw := int(binary.LittleEndian.Uint16(ws[pos+2+csw*2:]))
	fibBase := pos + 2 + csw*2 + 2 + cslw*4 + 2
	for _, index := range []int{2, 3, 4, 5, 11, 46, 47} {
		fc := int(binary.LittleEndian.Uint32(ws[fibBase+index*8:]))
		lcb := int(binary.LittleEndian.Uint32(ws[fibBase+index*8+4:]))
		if lcb <= 0 || fc < 0 || fc+lcb > len(ts) {
			t.Fatalf("FIB slot %d invalid fc/lcb=%d/%d", index, fc, lcb)
		}
	}
}

func TestLegacyWritePlanPreservesOLEPlaceholderAsObjectAnchor(t *testing.T) {
	p := wordir.Paragraph{Runs: []wordir.Run{{Kind: "run", Text: "before"}, {Kind: "olePlaceholder", Text: "Embedded Object", OLEProgID: "Excel.Sheet.12"}, {Kind: "run", Text: "after"}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	plan := buildLegacyWritePlan(doc, buildLegacyStyleWriteCatalog(doc), buildLegacyNumberingWriteCatalog(doc))
	if !strings.ContainsRune(plan.Text, '\x01') || strings.Contains(plan.Text, "Embedded Object") {
		t.Fatalf("OLE text=%q, want binary object anchor", plan.Text)
	}
}

func TestLegacyAnnotationPLCWritesAuthorAndDateMetadata(t *testing.T) {
	plc := buildLegacyAnnotationRefPLC([]int{3}, map[int]wordir.Story{1: {Author: "AS", Date: "2026-08-10"}})
	authors, dates := parseLegacyAnnotationMetadataForTest(plc, 1)
	if authors[1] != "AS" || dates[1] != "2026-08-10" {
		t.Fatalf("annotation metadata authors=%v dates=%v", authors, dates)
	}
}

func parseLegacyAnnotationMetadataForTest(plc []byte, count int) (map[int]string, map[int]string) {
	authors := map[int]string{}
	dates := map[int]string{}
	off := (count + 1) * 4
	for i := 0; i < count; i++ {
		rec := plc[off+i*30 : off+(i+1)*30]
		cch := int(binary.LittleEndian.Uint16(rec[:2]))
		if cch > 10 {
			cch = 10
		}
		var rs []rune
		for j := 0; j < cch; j++ {
			rs = append(rs, rune(binary.LittleEndian.Uint16(rec[2+j*2:4+j*2])))
		}
		authors[i+1] = string(rs)
		d := binary.LittleEndian.Uint32(rec[26:30])
		y := int((d>>20)&0x1ff) + 1900
		m := int((d >> 16) & 0xf)
		day := int((d >> 11) & 0x1f)
		if d != 0 {
			dates[i+1] = fmt.Sprintf("%04d-%02d-%02d", y, m, day)
		}
	}
	return authors, dates
}

func TestLegacyHeaderStoryBoundsKeepEmptySlotsZeroWidth(t *testing.T) {
	doc := &wordir.Document{
		Headers:  map[string]wordir.Story{"h": {Text: "Default header"}},
		Sections: []wordir.Section{{Headers: []wordir.Reference{{Type: "default", PartName: "h"}}}},
	}
	text, bounds := buildLegacyHeaderStoryText(doc)
	if !strings.Contains(text, "Default header") {
		t.Fatalf("header text=%q", text)
	}
	if len(bounds) != 13 {
		t.Fatalf("bounds=%v", bounds)
	}
	// Six separator slots and the even-header slot are empty; their boundaries repeat.
	for i := 1; i <= 7; i++ {
		if bounds[i] != 0 {
			t.Fatalf("bounds[%d]=%d, want zero-width empty slot", i, bounds[i])
		}
	}
	if bounds[8] <= bounds[7] {
		t.Fatalf("default header did not advance bounds: %v", bounds)
	}
}

func TestLegacyPreservationMetaRoundTripsOLEPayloadAndRichStories(t *testing.T) {
	body := wordir.Paragraph{Runs: []wordir.Run{{
		Kind:          "olePlaceholder",
		Text:          "Excel Worksheet",
		OLEProgID:     "Excel.Sheet.12",
		OLEData:       []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1},
		OLEMIME:       "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		OLELinkURL:    "https://example.test/book.xlsx",
		OLEType:       "Embed",
		OLEDrawAspect: "Content",
		OLEObjectID:   "_123",
		OLEUpdateMode: "Always",
		OLERelID:      "rOle7",
		OLEPreview:    &wordir.Image{MIMEType: "image/png", Data: []byte{0x89, 0x50, 0x4e, 0x47}, Width: 120, Height: 80},
	}}}
	richHeader := wordir.Story{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &wordir.Paragraph{
		Bookmarks: []string{"header-target"},
		Runs:      []wordir.Run{{Kind: "shape", Shape: &wordir.Shape{}}, {Kind: "run", Text: "Rich header"}},
	}}}}
	source := &wordir.Document{
		Blocks:   []wordir.Block{{Kind: "paragraph", Paragraph: &body}},
		Headers:  map[string]wordir.Story{"authored-header": richHeader},
		Sections: []wordir.Section{{Headers: []wordir.Reference{{Type: "default", PartName: "authored-header"}}}},
	}
	data := buildLegacyPreservationMeta(source)
	if len(data) == 0 {
		t.Fatal("expected preservation metadata")
	}
	cw := cfb.NewWriter()
	if err := cw.AddStream(legacyPreservationMetaStream, data); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	cfbBytes, err := cw.Bytes()
	if err != nil {
		t.Fatalf("cfb bytes: %v", err)
	}
	cf, err := cfb.OpenBytes(cfbBytes)
	if err != nil {
		t.Fatalf("cfb open: %v", err)
	}

	targetBody := wordir.Paragraph{Runs: []wordir.Run{{Kind: "olePlaceholder", Text: "Embedded Object"}}}
	target := &wordir.Document{
		Blocks:   []wordir.Block{{Kind: "paragraph", Paragraph: &targetBody}},
		Headers:  map[string]wordir.Story{"plain-header": {Text: "native plain header"}},
		Sections: []wordir.Section{{}},
	}
	applyLegacyPreservationMeta(cf, target)

	run := target.Blocks[0].Paragraph.Runs[0]
	if run.OLEProgID != "Excel.Sheet.12" || !bytes.Equal(run.OLEData, source.Blocks[0].Paragraph.Runs[0].OLEData) {
		t.Fatalf("OLE metadata not restored: %#v", run)
	}
	if run.OLELinkURL == "" || run.OLEPreview == nil || !bytes.Equal(run.OLEPreview.Data, []byte{0x89, 0x50, 0x4e, 0x47}) {
		t.Fatalf("OLE link/preview not restored: %#v", run)
	}
	if _, ok := target.Headers["plain-header"]; !ok {
		t.Fatal("native plain header was lost while merging metadata")
	}
	gotHeader, ok := target.Headers["authored-header"]
	if !ok || len(gotHeader.Blocks) != 1 || len(gotHeader.Blocks[0].Paragraph.Bookmarks) != 1 {
		t.Fatalf("rich header not restored: %#v", gotHeader)
	}
	if len(target.Sections[0].Headers) != 1 || target.Sections[0].Headers[0].PartName != "authored-header" {
		t.Fatalf("header references not restored: %#v", target.Sections[0].Headers)
	}
}

func TestLegacySecondaryStoryObjectKindsEmitNativeAnchorCharacters(t *testing.T) {
	for _, run := range []wordir.Run{
		{Kind: "image", Image: &wordir.Image{}},
		{Kind: "shape", Shape: &wordir.Shape{}},
		{Kind: "textbox", TextBox: &wordir.TextBox{}},
		{Kind: "olePlaceholder", OLEProgID: "Package"},
	} {
		var out strings.Builder
		writeLegacyRunPayload(&out, &run)
		if got := out.String(); got != "\x01" {
			t.Fatalf("kind %q emitted %q, want object anchor", run.Kind, got)
		}
	}
}

func TestWriteDocumentCarriesOLEPayloadInPreservationStream(t *testing.T) {
	p := wordir.Paragraph{Runs: []wordir.Run{{Kind: "olePlaceholder", OLEProgID: "Package", OLEData: []byte("payload")}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	var out bytes.Buffer
	if err := WriteDocument(doc, &out); err != nil {
		t.Fatalf("WriteDocument: %v", err)
	}
	cf, err := cfb.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatalf("cfb: %v", err)
	}
	meta, ok := cf.Stream(legacyPreservationMetaStream)
	if !ok || len(meta) == 0 || !bytes.Contains(meta, []byte("Package")) {
		t.Fatalf("missing OLE preservation stream: %q", meta)
	}
}

func TestMalformedLegacyPreservationMetaIsNonFatal(t *testing.T) {
	cw := cfb.NewWriter()
	if err := cw.AddStream(legacyPreservationMetaStream, []byte(`{"version":1,"ole":[`)); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	data, err := cw.Bytes()
	if err != nil {
		t.Fatalf("cfb bytes: %v", err)
	}
	cf, err := cfb.OpenBytes(data)
	if err != nil {
		t.Fatalf("cfb open: %v", err)
	}
	doc := &wordir.Document{Title: "still readable"}
	applyLegacyPreservationMeta(cf, doc)
	if doc.Title != "still readable" {
		t.Fatal("malformed private metadata changed native document state")
	}
}

func TestLegacyOLECompoundRecoveryUsesConservativeNativeStreamNames(t *testing.T) {
	cw := cfb.NewWriter()
	if err := cw.AddStream("Ole10Native", []byte("native-payload")); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	if err := cw.AddStream("Unrelated", []byte("ignore-me")); err != nil {
		t.Fatalf("AddStream unrelated: %v", err)
	}
	data, err := cw.Bytes()
	if err != nil {
		t.Fatalf("cfb bytes: %v", err)
	}
	cf, err := cfb.OpenBytes(data)
	if err != nil {
		t.Fatalf("cfb open: %v", err)
	}
	p := wordir.Paragraph{Runs: []wordir.Run{{Kind: "olePlaceholder", Text: "Embedded Object"}}}
	doc := &wordir.Document{Blocks: []wordir.Block{{Kind: "paragraph", Paragraph: &p}}}
	applyLegacyOLECompoundRecovery(cf, doc)
	got := doc.Blocks[0].Paragraph.Runs[0]
	if string(got.OLEData) != "native-payload" || got.OLEType != "Embed" {
		t.Fatalf("OLE recovery=%#v", got)
	}
}

func TestDecodeLegacyLvlfTemplateAcceptsNativeZeroBasedOffsets(t *testing.T) {
	header := make([]byte, legacyLvlfSize)
	for i := 6; i < 15; i++ {
		header[i] = 0xFF
	}
	// xst is "1.2)"; native rgbxchNums offsets point directly at the
	// placeholder UTF-16 code units for levels 1 and 2.
	header[6] = 0
	header[7] = 2
	xst := []byte{4, 0, '1', 0, '.', 0, '2', 0, ')', 0}
	if got := decodeLegacyLvlfTemplate(header, xst); got != "%1.%2)" {
		t.Fatalf("template=%q", got)
	}
}

func TestEncodeLegacyListTemplateUsesNativeZeroBasedOffsets(t *testing.T) {
	_, rgb := encodeLegacyListTemplateXst("%1.%2)")
	if rgb[0] != 0 || rgb[1] != 2 {
		t.Fatalf("rgb offsets=%v, want level offsets 0 and 2", rgb[:2])
	}
	for i := 2; i < len(rgb); i++ {
		if rgb[i] != 0xFF {
			t.Fatalf("rgb[%d]=%#x, want 0xFF sentinel", i, rgb[i])
		}
	}
}

func TestLegacySpaAnchorOriginWriterUsesNativeMSDOCValues(t *testing.T) {
	for _, tc := range []struct {
		name     string
		value    string
		vertical bool
		want     uint16
	}{
		{"bx-margin", "margin", false, 0},
		{"bx-page", "page", false, 1},
		{"bx-column", "column", false, 2},
		{"by-margin", "margin", true, 0},
		{"by-page", "page", true, 1},
		{"by-paragraph", "paragraph", true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := legacyAnchorRelativeBits(tc.value, tc.vertical); got != tc.want {
				t.Fatalf("bits = %d, want %d", got, tc.want)
			}
		})
	}
}
