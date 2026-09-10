package resolve

import (
	"testing"

	"office/pkg/ooxml/drawingxml"
	"office/pkg/sheet/parsers/ooxml/parts"
)

func TestBuildThemeColorIndexesApplyWorkbookThemeTransforms(t *testing.T) {
	theme := &parts.Theme{
		Elements: parts.ThemeElements{
			ClrScheme: parts.ColorScheme{
				LT1: &parts.ThemeColor{
					SrgbClr: &parts.SrgbColor{
						Val: "808080",
						ThemeColorTransforms: parts.ThemeColorTransforms{
							Tint: &drawingxml.ColorTransform{Val: 50000},
						},
					},
				},
				DK1: &parts.ThemeColor{
					SysClr: &parts.SysColor{
						LastClr: "808080",
						ThemeColorTransforms: parts.ThemeColorTransforms{
							Shade: &drawingxml.ColorTransform{Val: 50000},
						},
					},
				},
			},
		},
	}

	cellTheme := buildThemeColorIndex(theme)
	if got := cellTheme[0]; got != "C0C0C0" {
		t.Fatalf("theme index 0 = %q, want %q", got, "C0C0C0")
	}
	if got := cellTheme[1]; got != "404040" {
		t.Fatalf("theme index 1 = %q, want %q", got, "404040")
	}

	drawingTheme := buildDrawingThemeColorIndex(theme)
	if got := drawingTheme["lt1"]; got != "C0C0C0" {
		t.Fatalf("drawing lt1 = %q, want %q", got, "C0C0C0")
	}
	if got := drawingTheme["dk1"]; got != "404040" {
		t.Fatalf("drawing dk1 = %q, want %q", got, "404040")
	}
}

func TestResolveColorValAppliesSpreadsheetTint(t *testing.T) {
	themeIdx := 4

	if got := resolveColorVal(&parts.ColorVal{Theme: &themeIdx, Tint: 0.5}, map[int]string{4: "4472C4"}, nil); got != "A1B8E1" {
		t.Fatalf("resolveColorVal(theme+tint) = %q, want %q", got, "A1B8E1")
	}
	if got := resolveColorVal(&parts.ColorVal{RGB: "FFAABBCC", Tint: -0.25}, nil, nil); got != "7F8C99" {
		t.Fatalf("resolveColorVal(rgb+tint) = %q, want %q", got, "7F8C99")
	}
}

func TestBuildIndexedColorIndexUsesDefaultsAndCustomOverrides(t *testing.T) {
	styles := &parts.StyleSheet{
		Colors: &parts.StyleColors{
			IndexedColors: &parts.IndexedColors{
				RGBColors: []parts.IndexedRGBColor{
					{RGB: "FF102030"},
					{RGB: "FF405060"},
				},
			},
		},
	}

	indexed := buildIndexedColorIndex(styles)
	if got := indexed[0]; got != "102030" {
		t.Fatalf("indexed[0] = %q, want %q", got, "102030")
	}
	if got := indexed[1]; got != "405060" {
		t.Fatalf("indexed[1] = %q, want %q", got, "405060")
	}
	if got := indexed[10]; got != "FF0000" {
		t.Fatalf("indexed[10] = %q, want %q", got, "FF0000")
	}
}

func TestResolveColorValResolvesIndexedColors(t *testing.T) {
	defaultIdx := 12
	if got := resolveColorVal(&parts.ColorVal{Indexed: &defaultIdx}, nil, nil); got != "0000FF" {
		t.Fatalf("resolveColorVal(default indexed) = %q, want %q", got, "0000FF")
	}

	customIdx := 10
	if got := resolveColorVal(&parts.ColorVal{Indexed: &customIdx, Tint: -0.5}, nil, map[int]string{10: "AABBCC"}); got != "555D66" {
		t.Fatalf("resolveColorVal(custom indexed+tint) = %q, want %q", got, "555D66")
	}

	autoIdx := 64
	if got := resolveColorVal(&parts.ColorVal{Indexed: &autoIdx}, nil, nil); got != "" {
		t.Fatalf("resolveColorVal(automatic indexed) = %q, want empty", got)
	}
}

func TestResolveShapeTextRunUsesWorkbookThemeAndIndexedColors(t *testing.T) {
	themeIdx := 4
	run := parts.XDRTxRun{
		T: "Theme text",
		Properties: &parts.RTProps{
			Color: &parts.ColorVal{Theme: &themeIdx},
		},
	}

	resolved := resolveShapeTextRun(run, map[int]string{4: "4472C4"}, nil)
	if resolved.FontColor != "4472C4" {
		t.Fatalf("resolveShapeTextRun(theme) = %q, want %q", resolved.FontColor, "4472C4")
	}

	indexedIdx := 9
	run.Properties.Color = &parts.ColorVal{Indexed: &indexedIdx}
	resolved = resolveShapeTextRun(run, nil, map[int]string{9: "ABCDEF"})
	if resolved.FontColor != "ABCDEF" {
		t.Fatalf("resolveShapeTextRun(indexed) = %q, want %q", resolved.FontColor, "ABCDEF")
	}
}

func TestNormalizeFormControlLinkedCellStateForRadioGroups(t *testing.T) {
	sheet := &Sheet{
		Rows: []*Row{{
			Index: 1,
			Cells: []*Cell{
				{Row: 1, Col: 4, Ref: "D1", Kind: CellNumber, NumValue: 1},
				{Row: 1, Col: 8, Ref: "H1", Kind: CellNumber, NumValue: 2},
			},
		}},
		FormControls: []*FormControl{
			{
				Type:      "GBox",
				AltText:   "Region",
				Anchor:    "0, 0, 0, 0, 4, 0, 6, 0",
				FmlaGroup: "$H$1",
			},
			{
				Type:        "Radio",
				AltText:     "North",
				Anchor:      "1, 0, 1, 0, 2, 0, 2, 0",
				Checked:     true,
				FirstButton: true,
				FmlaLink:    "$B$1",
			},
			{
				Type:     "Radio",
				AltText:  "South",
				Anchor:   "1, 0, 2, 0, 2, 0, 3, 0",
				FmlaLink: "$C$1",
			},
			{
				Type:        "Radio",
				AltText:     "West",
				Anchor:      "5, 0, 1, 0, 6, 0, 2, 0",
				FirstButton: true,
				FmlaLink:    "$D$1",
			},
			{
				Type:     "Radio",
				AltText:  "East",
				Anchor:   "5, 0, 2, 0, 6, 0, 3, 0",
				FmlaLink: "$D$1",
				Checked:  true,
			},
		},
	}

	normalizeFormControlLinkedCellState(sheet)

	if sheet.FormControls[1].Checked {
		t.Fatalf("expected first grouped radio to be cleared by H1=2, got %#v", sheet.FormControls[1])
	}
	if !sheet.FormControls[2].Checked {
		t.Fatalf("expected second grouped radio to be selected by H1=2, got %#v", sheet.FormControls[2])
	}
	if !sheet.FormControls[3].Checked {
		t.Fatalf("expected third radio to be selected by D1=1, got %#v", sheet.FormControls[3])
	}
	if sheet.FormControls[4].Checked {
		t.Fatalf("expected fourth radio to be cleared by D1=1, got %#v", sheet.FormControls[4])
	}
}

func TestResolveTablesEnrichment(t *testing.T) {
	partsTable := &parts.Table{
		ID:             1,
		Name:           "Table1",
		DisplayName:    "Table1",
		Ref:            "G1:I4",
		TotalsRowShown: false,
		HeaderRowCount: 1,
		TotalsRowCount: 0,
		AutoFilter:     &parts.TableAutoFilter{Ref: "G1:I4"},
		TableStyleInfo: &parts.TableStyleInfoXML{
			Name:              "TableStyleLight1",
			ShowFirstColumn:   false,
			ShowLastColumn:    false,
			ShowRowStripes:    true,
			ShowColumnStripes: false,
		},
		Columns: []parts.TableColumn{
			{ID: 1, Name: "Name"},
			{ID: 2, Name: "Age"},
			{ID: 3, Name: "AgeGroup", CalculatedColumnFormula: "INT(H2/10)"},
		},
	}

	resolved := resolveTables([]*parts.Table{partsTable})
	if len(resolved) != 1 {
		t.Fatalf("expected 1 table, got %d", len(resolved))
	}
	tbl := resolved[0]
	if tbl.AutoFilterRef != "G1:I4" {
		t.Errorf("AutoFilterRef = %q, want G1:I4", tbl.AutoFilterRef)
	}
	if tbl.TableStyle == nil {
		t.Fatal("TableStyle is nil")
	}
	if tbl.TableStyle.Name != "TableStyleLight1" {
		t.Errorf("TableStyle.Name = %q, want TableStyleLight1", tbl.TableStyle.Name)
	}
	if !tbl.TableStyle.ShowRowStripes {
		t.Error("expected ShowRowStripes = true")
	}
	if len(tbl.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(tbl.Columns))
	}
	col := tbl.Columns[2]
	if col.CalculatedColumnFormula != "INT(H2/10)" {
		t.Errorf("CalculatedColumnFormula = %q, want INT(H2/10)", col.CalculatedColumnFormula)
	}
}

func TestPhase5OOXMLCalcPrUsesCanonicalWorkbookCalcSemantics(t *testing.T) {
	id := 191029
	full := true
	force := true
	cp := &parts.CalcPr{RefMode: "R1C1", CalcID: &id, FullCalcOnLoad: &full, ForceFullCalc: &force}
	got := resolveWorkbookCalcPr(cp)
	if got == nil || got.ReferenceMode != "R1C1" {
		t.Fatalf("canonical calc settings = %#v", got)
	}
	if got.RecalcID == nil || *got.RecalcID != id {
		t.Fatalf("RecalcID = %v, want %d", got.RecalcID, id)
	}
	if got.FullCalcOnLoad == nil || !*got.FullCalcOnLoad || got.ForceFullCalc == nil || !*got.ForceFullCalc {
		t.Fatalf("full calculation flags = %#v", got)
	}
}

func TestResolvePivotTablesCanonicalizesDefaultItemTypeAsData(t *testing.T) {
	pt := &parts.PivotTableDefinition{
		Name:     "Pivot1",
		Location: parts.PivotLocation{Ref: "A3:B6"},
		PivotFields: []parts.PivotField{{
			Name:  "Field1",
			Items: &parts.PivotItems{Count: 1, Items: []parts.PivotItem{{X: 0}}},
		}},
	}
	resolved := resolvePivotTables([]*parts.PivotTableWithCache{{Table: pt}})
	if len(resolved) != 1 || len(resolved[0].Fields) != 1 || len(resolved[0].Fields[0].Items) != 1 {
		t.Fatalf("resolved pivot shape = %#v", resolved)
	}
	if got := resolved[0].Fields[0].Items[0].Type; got != "data" {
		t.Fatalf("default pivot item type = %q, want canonical data", got)
	}
}

func TestResolvePivotTablesDoesNotMaterializeCacheDisplayAsItemName(t *testing.T) {
	pt := &parts.PivotTableDefinition{
		Name:     "Pivot1",
		Location: parts.PivotLocation{Ref: "A3:B6"},
		PivotFields: []parts.PivotField{{
			Name:  "Field1",
			Items: &parts.PivotItems{Count: 1, Items: []parts.PivotItem{{X: 0}}},
		}},
	}
	cache := &parts.PivotCacheDefinition{
		CacheFields: &parts.CacheFields{Count: 1, Fields: []parts.CacheField{{
			Name:        "Field1",
			SharedItems: &parts.SharedItems{Count: 1, Items: []parts.SharedItem{{Type: "s", Value: "East"}}},
		}}},
	}
	resolved := resolvePivotTables([]*parts.PivotTableWithCache{{Table: pt, Cache: cache}})
	item := resolved[0].Fields[0].Items[0]
	if item.Name != "" {
		t.Fatalf("canonical explicit pivot item name = %q, want empty; cache display is derived data", item.Name)
	}
	if item.Type != "data" {
		t.Fatalf("canonical default pivot item type = %q, want data", item.Type)
	}
}
