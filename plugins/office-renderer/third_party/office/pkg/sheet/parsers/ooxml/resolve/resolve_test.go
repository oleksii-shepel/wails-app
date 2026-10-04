package resolve_test

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	"office/pkg/sheet/parsers/ooxml/parts"
	"office/pkg/sheet/parsers/ooxml/resolve"
)

func TestXlsxCorpusResolve(t *testing.T) {
	fixturesDir := repoPath(t, "testdata", "xlsx")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	var count int
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".xlsx") {
			continue
		}
		// Excel drops an owner lock file next to an open workbook. It is not a
		// ZIP package (and may be locked by the running Office process), so a
		// fixture left open must not fail the corpus run.
		if strings.HasPrefix(entry.Name(), "~$") {
			continue
		}
		count++
		path := filepath.Join(fixturesDir, entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			pkg, err := opc.Open(path)
			if err != nil {
				if opc.IsEncryptedPackageError(err) {
					t.Skip("encrypted package requires password")
				}
				t.Fatalf("open: %v", err)
			}
			xlPkg, err := parts.LoadPackage(pkg)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			wb, err := resolve.ResolvePackage(xlPkg)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if wb == nil {
				t.Fatal("workbook is nil")
			}
			if len(wb.Sheets) == 0 {
				t.Fatal("no sheets resolved")
			}
			for _, s := range wb.Sheets {
				if s.Name == "" {
					t.Errorf("sheet has no name")
				}
				// Every sheet should have at least one row with cells in the corpus.
				hasCells := false
				for _, r := range s.Rows {
					if len(r.Cells) > 0 {
						hasCells = true
						break
					}
				}
				if !hasCells {
					t.Errorf("sheet %q has no cells", s.Name)
				}
			}
		})
	}

	if count == 0 {
		t.Fatal("no xlsx fixtures found")
	}
}

func TestResolvePackageCapturesMergedCells(t *testing.T) {
	pkg, err := opc.Open(repoPath(t, "testdata", "xlsx", "sample-5.xlsx"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	xlPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	wb, err := resolve.ResolvePackage(xlPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	var yearSheet *resolve.Sheet
	for _, sheet := range wb.Sheets {
		if sheet.Name == "Year" {
			yearSheet = sheet
			break
		}
	}
	if yearSheet == nil {
		t.Fatal("Year sheet not found")
	}
	if len(yearSheet.MergedCells) == 0 {
		t.Fatal("expected merged cells on Year sheet")
	}
	if len(yearSheet.Rows) == 0 || len(yearSheet.Rows[0].Cells) == 0 {
		t.Fatal("expected resolved cells on Year sheet")
	}

	var topLeft *resolve.Cell
	for _, cell := range yearSheet.Rows[0].Cells {
		if cell.Ref == "A1" {
			topLeft = cell
			break
		}
	}
	if topLeft == nil {
		t.Fatal("expected styled empty cell A1 to be preserved")
	}
	if topLeft.Style.BgColor == "" {
		t.Fatal("expected styled empty cell A1 to preserve background color")
	}

	assertHasMerge := func(topRow, leftCol, bottomRow, rightCol int) {
		t.Helper()
		for _, merged := range yearSheet.MergedCells {
			if merged.TopRow == topRow && merged.LeftCol == leftCol &&
				merged.BottomRow == bottomRow && merged.RightCol == rightCol {
				return
			}
		}
		t.Fatalf("missing merged range rows %d-%d cols %d-%d", topRow, bottomRow, leftCol, rightCol)
	}

	assertHasMerge(2, 2, 2, 8)    // B2:H2
	assertHasMerge(1, 26, 37, 26) // Z1:Z37
	assertHasMerge(1, 27, 37, 27) // AA1:AA37
}

func repoPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}

func TestResolveHyperlinkURL(t *testing.T) {
	ws := &parts.Worksheet{
		Rows: []parts.Row{{
			Index: 1,
			Cells: []parts.Cell{{Ref: "A1", Type: "s", Value: "0"}},
		}},
		Hyperlinks: []parts.Hyperlink{
			{Ref: "A1", RelID: "rId1"},
		},
	}
	entry := &parts.WorksheetEntry{
		SheetID:       1,
		Name:          "Sheet1",
		Sheet:         ws,
		HyperlinkURLs: map[string]string{"rId1": "https://example.com"},
	}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
		SharedStrings: &parts.SharedStrings{
			Items: []parts.RichText{{PlainText: "Click here"}},
		},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if len(wb.Sheets) == 0 || len(wb.Sheets[0].Rows) == 0 {
		t.Fatal("expected sheet with rows")
	}
	cell := wb.Sheets[0].Rows[0].Cells[0]
	if cell.HyperlinkURL != "https://example.com" {
		t.Fatalf("HyperlinkURL = %q, want %q", cell.HyperlinkURL, "https://example.com")
	}
}

func TestResolveInternalHyperlink(t *testing.T) {
	ws := &parts.Worksheet{
		Rows: []parts.Row{{
			Index: 1,
			Cells: []parts.Cell{{Ref: "B2", Type: "s", Value: "0"}},
		}},
		Hyperlinks: []parts.Hyperlink{
			{Ref: "B2", Location: "Sheet2!A1"},
		},
	}
	entry := &parts.WorksheetEntry{
		SheetID: 1,
		Name:    "Sheet1",
		Sheet:   ws,
	}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
		SharedStrings: &parts.SharedStrings{
			Items: []parts.RichText{{PlainText: "Go to Sheet2"}},
		},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	cell := wb.Sheets[0].Rows[0].Cells[0]
	if cell.HyperlinkURL != "#Sheet2!A1" {
		t.Fatalf("HyperlinkURL = %q, want %q", cell.HyperlinkURL, "#Sheet2!A1")
	}
}

func TestResolveRichTextRuns(t *testing.T) {
	ws := &parts.Worksheet{
		Rows: []parts.Row{{
			Index: 1,
			Cells: []parts.Cell{{Ref: "A1", Type: "s", Value: "0"}},
		}},
	}
	entry := &parts.WorksheetEntry{
		SheetID: 1,
		Name:    "Sheet1",
		Sheet:   ws,
	}
	bold := struct{}{}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
		SharedStrings: &parts.SharedStrings{
			Items: []parts.RichText{{
				Runs: []parts.RTRun{
					{Text: "Bold", Properties: &parts.RTProps{Bold: &bold}},
					{Text: " Normal"},
				},
			}},
		},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	cell := wb.Sheets[0].Rows[0].Cells[0]
	if len(cell.RichTextRuns) != 2 {
		t.Fatalf("RichTextRuns len = %d, want 2", len(cell.RichTextRuns))
	}
	if !cell.RichTextRuns[0].Bold {
		t.Fatalf("first run should be bold")
	}
	if cell.RichTextRuns[0].Text != "Bold" {
		t.Fatalf("first run text = %q, want %q", cell.RichTextRuns[0].Text, "Bold")
	}
	if cell.RichTextRuns[1].Bold {
		t.Fatalf("second run should not be bold")
	}
}

func TestResolveHiddenColumns(t *testing.T) {
	ws := &parts.Worksheet{
		Cols: []parts.ColRange{
			{Min: 2, Max: 3, Hidden: true},
			{Min: 5, Max: 5, Width: 10},
		},
		Rows: []parts.Row{{
			Index: 1,
			Cells: []parts.Cell{{Ref: "A1", Value: "1"}},
		}},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if !sheet.HiddenCols[2] {
		t.Fatalf("expected column 2 to be hidden")
	}
	if !sheet.HiddenCols[3] {
		t.Fatalf("expected column 3 to be hidden")
	}
	if sheet.HiddenCols[5] {
		t.Fatalf("column 5 should not be hidden (it has a width)")
	}
	if _, ok := sheet.ColumnWidths[5]; !ok {
		t.Fatalf("column 5 should have an explicit width")
	}
}

func TestResolveHiddenColumnPreservesAuthoredWidth(t *testing.T) {
	ws := &parts.Worksheet{
		Cols: []parts.ColRange{{Min: 2, Max: 2, Width: 17.25, CustomWidth: true, Hidden: true}},
		Rows: []parts.Row{{Index: 1, Cells: []parts.Cell{{Ref: "A1", Value: "1"}}}},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if !sheet.HiddenCols[2] {
		t.Fatal("column B should remain hidden")
	}
	if got := sheet.ColumnWidths[2]; got != 17.25 {
		t.Fatalf("hidden column B width = %v, want 17.25", got)
	}
}

func TestResolveComments(t *testing.T) {
	ws := &parts.Worksheet{
		Rows: []parts.Row{{
			Index: 1,
			Cells: []parts.Cell{{Ref: "A1", Value: "1"}},
		}},
	}
	entry := &parts.WorksheetEntry{
		SheetID: 1,
		Name:    "Sheet1",
		Sheet:   ws,
		Comments: &parts.Comments{CommentList: []parts.CommentEntry{
			{Ref: "A1", Text: parts.RichText{PlainText: "Needs review"}},
			{Ref: "B2", Text: parts.RichText{PlainText: ""}},
		}},
	}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if sheet.Comments["A1"] != "Needs review" {
		t.Fatalf("expected comment for A1, got %q", sheet.Comments["A1"])
	}
	if _, ok := sheet.Comments["B2"]; ok {
		t.Fatalf("did not expect empty comment entry for B2")
	}
}

func TestResolveOutlineLevels(t *testing.T) {
	ws := &parts.Worksheet{
		Cols: []parts.ColRange{{Min: 2, Max: 4, OutlineLevel: 2, Collapsed: true}},
		Rows: []parts.Row{
			{Index: 1, OutlineLevel: 1, Collapsed: true, Cells: []parts.Cell{{Ref: "A1", Value: "x"}}},
			{Index: 2, Cells: []parts.Cell{{Ref: "A2", Value: "y"}}},
		},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if sheet.OutlineRows[1] != 1 {
		t.Fatalf("expected row 1 outline level 1, got %d", sheet.OutlineRows[1])
	}
	if sheet.OutlineCols[2] != 2 || sheet.OutlineCols[4] != 2 {
		t.Fatalf("expected outline level 2 for columns 2..4")
	}
	if level := sheet.Rows[0].OutlineLevel; level != 1 {
		t.Fatalf("expected resolved row outline level 1, got %d", level)
	}
	if !sheet.CollapsedRows[1] {
		t.Fatalf("expected row 1 collapsed outline marker")
	}
	if !sheet.CollapsedCols[2] || !sheet.CollapsedCols[4] {
		t.Fatalf("expected collapsed markers for columns 2..4")
	}
	if !sheet.Rows[0].Collapsed {
		t.Fatalf("expected resolved row collapsed flag")
	}
}

func TestResolvePageSetupAndMargins(t *testing.T) {
	ws := &parts.Worksheet{
		PrintOptions: &parts.PrintOptions{
			HorizontalCentered: true,
			VerticalCentered:   true,
			Headings:           true,
			GridLines:          true,
		},
		PageSetup: &parts.PageSetup{
			PaperSize:   9,
			Orientation: "landscape",
			FitToPage:   true,
			FitToWidth:  1,
			FitToHeight: 0,
			Scale:       90,
		},
		PageMargins: &parts.PageMargins{
			Left:   0.7,
			Right:  0.7,
			Top:    0.75,
			Bottom: 0.75,
			Header: 0.3,
			Footer: 0.3,
		},
		Rows: []parts.Row{{Index: 1, Cells: []parts.Cell{{Ref: "A1", Value: "1"}}}},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	ps := wb.Sheets[0].PageSetup
	if ps == nil {
		t.Fatal("expected PageSetup to be resolved")
	}
	if ps.PaperSize != 9 || ps.Orientation != "landscape" || !ps.FitToPage {
		t.Fatalf("unexpected page setup values: %+v", *ps)
	}
	if ps.MarginLeft != 0.7 || ps.MarginTop != 0.75 || ps.MarginHeader != 0.3 {
		t.Fatalf("unexpected page margins values: %+v", *ps)
	}
	if !ps.CenterHorizontally || !ps.CenterVertically || !ps.PrintHeadings || !ps.PrintGridlines {
		t.Fatalf("unexpected print options values: %+v", *ps)
	}
}

func TestResolveProtectionMetadata(t *testing.T) {
	ws := &parts.Worksheet{
		SheetProtection: &parts.SheetProtection{
			Sheet:               true,
			Objects:             true,
			Sort:                true,
			AutoFilter:          true,
			SelectLockedCells:   true,
			SelectUnlockedCells: true,
			AlgorithmName:       "SHA-512",
			SpinCount:           25000,
		},
		Rows: []parts.Row{{Index: 1, Cells: []parts.Cell{{Ref: "A1", Value: "1"}}}},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Protected", Sheet: ws}
	pkg := &parts.Package{
		Workbook: &parts.Workbook{
			Protection: &parts.WorkbookProtection{
				LockStructure: true,
				LockWindows:   true,
				AlgorithmName: "SHA-512",
				SpinCount:     100000,
			},
			Sheets: []parts.SheetRef{{Name: "Protected", SheetID: 1}},
		},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if wb.Protection == nil {
		t.Fatal("expected workbook protection metadata")
	}
	if !wb.Protection.LockStructure || !wb.Protection.LockWindows || wb.Protection.AlgorithmName != "SHA-512" || wb.Protection.SpinCount != 100000 {
		t.Fatalf("unexpected workbook protection: %+v", wb.Protection)
	}
	if len(wb.Sheets) != 1 || wb.Sheets[0].Protection == nil {
		t.Fatalf("expected worksheet protection metadata, got %+v", wb.Sheets)
	}
	sp := wb.Sheets[0].Protection
	if !sp.Sheet || !sp.Objects || !sp.Sort || !sp.AutoFilter || !sp.SelectLockedCells || !sp.SelectUnlockedCells {
		t.Fatalf("unexpected worksheet protection flags: %+v", sp)
	}
	if sp.AlgorithmName != "SHA-512" || sp.SpinCount != 25000 {
		t.Fatalf("unexpected worksheet protection metadata: %+v", sp)
	}
}

func TestResolveWorkbookConnections(t *testing.T) {
	ws := &parts.Worksheet{
		Rows: []parts.Row{{Index: 1, Cells: []parts.Cell{{Ref: "A1", Value: "1"}}}},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook: &parts.Workbook{
			Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}},
		},
		Connections: &parts.Connections{Connections: []parts.Connection{
			{
				ID:            1,
				Name:          "Sales DB",
				Description:   "Warehouse import",
				RefreshOnLoad: true,
				Background:    true,
				SaveData:      true,
				SourceFile:    "sales.odc",
				DBPr: &parts.ConnectionDBPr{
					Connection:  "Provider=SQLOLEDB;Data Source=server;Initial Catalog=sales",
					Command:     "SELECT * FROM Sales",
					CommandType: "2",
				},
			},
			{
				ID:   2,
				Name: "Rates Feed",
				WebPr: &parts.ConnectionWebPr{
					URL:        "https://example.com/rates.csv",
					Post:       "region=us",
					HtmlTables: true,
					SourceData: true,
					FirstRow:   true,
					TextDates:  true,
				},
			},
		}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if len(wb.Connections) != 2 {
		t.Fatalf("expected 2 workbook connections, got %+v", wb.Connections)
	}
	if wb.Connections[0].Kind != "database" || wb.Connections[0].Command != "SELECT * FROM Sales" || wb.Connections[0].SourceFile != "sales.odc" {
		t.Fatalf("unexpected first resolved connection: %+v", wb.Connections[0])
	}
	if wb.Connections[1].Kind != "web" || wb.Connections[1].URL != "https://example.com/rates.csv" || wb.Connections[1].WebPost != "region=us" || !wb.Connections[1].WebHTMLTables || !wb.Connections[1].WebSourceData || !wb.Connections[1].WebFirstRow || !wb.Connections[1].WebTextDates {
		t.Fatalf("unexpected second resolved connection: %+v", wb.Connections[1])
	}
}

func TestResolveWorkbookExternalLinks(t *testing.T) {
	ws := &parts.Worksheet{
		Rows: []parts.Row{{Index: 1, Cells: []parts.Cell{{Ref: "A1", Value: "1"}}}},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook: &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		ExternalLinks: []*parts.ExternalLinkEntry{{
			Target: "file:///C:/workbooks/source.xlsx",
			Link: &parts.ExternalLink{ExternalBook: &parts.ExternalBook{
				SheetNames:   []parts.ExternalSheetName{{Val: "Prices"}},
				DefinedNames: []parts.ExternalDefinedName{{Name: "ExtRange", RefersTo: "Prices!$A$1:$B$2"}},
				SheetDataSet: &parts.ExternalSheetDataSet{Sheets: []parts.ExternalSheetData{{
					SheetID: 0,
					Rows: []parts.ExternalSheetRow{{
						Index: 1,
						Cells: []parts.ExternalSheetCell{{Ref: "A1", Type: "n", Value: "7"}, {Ref: "B1", Type: "str", Value: "North"}},
					}},
				}}},
			}},
		}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if len(wb.ExternalLinks) != 1 {
		t.Fatalf("expected 1 resolved external link, got %+v", wb.ExternalLinks)
	}
	link := wb.ExternalLinks[0]
	if link.Target != "file:///C:/workbooks/source.xlsx" {
		t.Fatalf("unexpected external link target: %+v", link)
	}
	if len(link.SheetNames) != 1 || link.SheetNames[0] != "Prices" {
		t.Fatalf("unexpected external link sheet names: %+v", link.SheetNames)
	}
	if len(link.DefinedNames) != 1 || link.DefinedNames[0].Name != "ExtRange" || link.DefinedNames[0].RefersTo != "Prices!$A$1:$B$2" {
		t.Fatalf("unexpected external link defined names: %+v", link.DefinedNames)
	}
	if len(link.CachedSheets) != 1 || link.CachedSheets[0].Name != "Prices" {
		t.Fatalf("unexpected external cached sheets: %+v", link.CachedSheets)
	}
	if len(link.CachedSheets[0].Cells) != 2 || link.CachedSheets[0].Cells[0].Ref != "A1" || link.CachedSheets[0].Cells[0].Value != "7" {
		t.Fatalf("unexpected external cached cells: %+v", link.CachedSheets[0].Cells)
	}
}

func TestResolvePrintTitles(t *testing.T) {
	ws := &parts.Worksheet{
		Rows: []parts.Row{{Index: 1, Cells: []parts.Cell{{Ref: "A1", Value: "1"}}}},
	}
	entry := &parts.WorksheetEntry{SheetID: 7, Name: "Sheet1", Sheet: ws}
	localSheetID := 0
	pkg := &parts.Package{
		Workbook: &parts.Workbook{
			Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 7}},
			DefinedNames: []parts.DefinedName{{
				Name:         "_xlnm.Print_Titles",
				LocalSheetID: &localSheetID,
				Value:        "Sheet1!$1:$2,Sheet1!$A:$B",
			}},
		},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if sheet.PrintTitleRows != "$1:$2" {
		t.Fatalf("expected print title rows $1:$2, got %q", sheet.PrintTitleRows)
	}
	if sheet.PrintTitleCols != "$A:$B" {
		t.Fatalf("expected print title cols $A:$B, got %q", sheet.PrintTitleCols)
	}
}

func TestResolvePrintArea(t *testing.T) {
	ws := &parts.Worksheet{
		Rows: []parts.Row{{Index: 1, Cells: []parts.Cell{{Ref: "A1", Value: "1"}}}},
	}
	entry := &parts.WorksheetEntry{SheetID: 7, Name: "Sheet1", Sheet: ws}
	localSheetID := 0
	pkg := &parts.Package{
		Workbook: &parts.Workbook{
			Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 7}},
			DefinedNames: []parts.DefinedName{{
				Name:         "_xlnm.Print_Area",
				LocalSheetID: &localSheetID,
				Value:        "Sheet1!$A$1:$D$20,Sheet1!$F$1:$F$10",
			}},
		},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if wb.Sheets[0].PrintArea != "$A$1:$D$20,$F$1:$F$10" {
		t.Fatalf("expected print area $A$1:$D$20,$F$1:$F$10, got %q", wb.Sheets[0].PrintArea)
	}
}

func TestResolveHiddenSheetState(t *testing.T) {
	ws := &parts.Worksheet{
		Rows: []parts.Row{{Index: 1, Cells: []parts.Cell{{Ref: "A1", Value: "1"}}}},
	}
	entry := &parts.WorksheetEntry{SheetID: 7, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook: &parts.Workbook{
			Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 7, State: "veryHidden"}},
		},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if !wb.Sheets[0].Hidden {
		t.Fatal("expected hidden sheet state to be preserved")
	}
	if wb.Sheets[0].Visibility != "veryHidden" {
		t.Fatalf("Visibility = %q, want veryHidden", wb.Sheets[0].Visibility)
	}
}

func TestResolveZoomScale(t *testing.T) {
	ws := &parts.Worksheet{
		Rows: []parts.Row{{Index: 1, Cells: []parts.Cell{{Ref: "A1", Value: "1"}}}},
		SheetViews: []parts.SheetView{{
			WorkbookViewID: 0,
			ZoomScale:      135,
		}},
	}
	entry := &parts.WorksheetEntry{SheetID: 7, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 7}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if wb.Sheets[0].ZoomScale != 135 {
		t.Fatalf("expected zoom scale 135, got %d", wb.Sheets[0].ZoomScale)
	}
}

func TestResolveGridLinesVisibility(t *testing.T) {
	showGridLines := false
	ws := &parts.Worksheet{
		Rows: []parts.Row{{Index: 1, Cells: []parts.Cell{{Ref: "A1", Value: "1"}}}},
		SheetViews: []parts.SheetView{{
			WorkbookViewID: 0,
			ShowGridLines:  &showGridLines,
		}},
	}
	entry := &parts.WorksheetEntry{SheetID: 7, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 7}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if wb.Sheets[0].ShowGridLines == nil || *wb.Sheets[0].ShowGridLines {
		t.Fatalf("expected explicit showGridLines=false, got %v", wb.Sheets[0].ShowGridLines)
	}
}

func TestResolveSharedFormulaExpansion(t *testing.T) {
	sharedIndex := 40
	ws := &parts.Worksheet{
		Rows: []parts.Row{
			{Index: 23, Cells: []parts.Cell{
				{Ref: "B23", Value: "3", Formula: &parts.Formula{Type: "shared", Ref: "B23:B27", SharedIndex: &sharedIndex, Text: `IF(OR(H22="",VALUE(H22)>31),"",H22+1)`}},
				{Ref: "B24", Value: "10", Formula: &parts.Formula{Type: "shared", SharedIndex: &sharedIndex}},
			}},
		},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	rows := wb.Sheets[0].Rows
	if len(rows) != 1 || len(rows[0].Cells) != 2 {
		t.Fatalf("expected one resolved row with two cells")
	}
	if rows[0].Cells[0].Formula != `IF(OR(H22="",VALUE(H22)>31),"",H22+1)` {
		t.Fatalf("unexpected master formula %q", rows[0].Cells[0].Formula)
	}
	if rows[0].Cells[1].Formula != `IF(OR(H23="",VALUE(H23)>31),"",H23+1)` {
		t.Fatalf("unexpected expanded shared formula %q", rows[0].Cells[1].Formula)
	}
	if rows[0].Cells[1].Kind != resolve.CellNumber || rows[0].Cells[1].NumValue != 10 {
		t.Fatalf("expected cached numeric result to be preserved, got kind=%v num=%v", rows[0].Cells[1].Kind, rows[0].Cells[1].NumValue)
	}
}

func TestResolveArrayFormulaPreservesRangeAndMemberMetadata(t *testing.T) {
	ws := &parts.Worksheet{
		Rows: []parts.Row{{
			Index: 1,
			Cells: []parts.Cell{
				{Ref: "A1", Value: "10", Formula: &parts.Formula{Type: "array", Ref: "A1:B1", Text: "ROW(A1:B1)"}},
				{Ref: "B1", Value: "20"},
			},
		}},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if len(sheet.ArrayFormulas) != 1 {
		t.Fatalf("expected 1 array formula group, got %d", len(sheet.ArrayFormulas))
	}
	group := sheet.ArrayFormulas[0]
	if group.AnchorRef != "A1" || group.RangeRef != "A1:B1" || group.Formula != "ROW(A1:B1)" {
		t.Fatalf("unexpected array formula group: %#v", group)
	}
	rows := sheet.Rows
	if len(rows) != 1 || len(rows[0].Cells) != 2 {
		t.Fatalf("expected one resolved row with two cells")
	}
	anchor := rows[0].Cells[0]
	member := rows[0].Cells[1]
	if anchor.ArrayFormulaAnchor != "A1" || anchor.ArrayFormulaRange != "A1:B1" {
		t.Fatalf("unexpected anchor array metadata: %#v", anchor)
	}
	if member.ArrayFormulaAnchor != "A1" || member.ArrayFormulaRange != "A1:B1" {
		t.Fatalf("unexpected member array metadata: %#v", member)
	}
	if member.Formula != "ROW(A1:B1)" {
		t.Fatalf("unexpected member formula text %q", member.Formula)
	}
	if member.Kind != resolve.CellNumber || member.NumValue != 20 {
		t.Fatalf("expected cached member value to be preserved, got kind=%v num=%v", member.Kind, member.NumValue)
	}
}

func TestResolveAutoFilter(t *testing.T) {
	ws := &parts.Worksheet{
		AutoFilter: &parts.AutoFilter{Ref: "A1:E10"},
		Rows: []parts.Row{{
			Index: 1,
			Cells: []parts.Cell{{Ref: "A1", Value: "Header"}},
		}},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if sheet.AutoFilterRef != "A1:E10" {
		t.Errorf("expected AutoFilterRef 'A1:E10', got %q", sheet.AutoFilterRef)
	}
}

func TestResolveDataValidation(t *testing.T) {
	ws := &parts.Worksheet{
		DataValidations: []parts.DataValidation{
			{
				SqRef:            "B2:B10",
				Type:             "list",
				AllowBlank:       true,
				ShowInputMessage: true,
				Prompt:           "Pick one",
				Formula1:         "ListMonths",
			},
		},
		Rows: []parts.Row{{
			Index: 1,
			Cells: []parts.Cell{{Ref: "A1", Value: "Header"}},
		}},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if len(sheet.DataValidations) != 1 {
		t.Fatalf("expected 1 DataValidation, got %d", len(sheet.DataValidations))
	}
	dv := sheet.DataValidations[0]
	if dv.SqRef != "B2:B10" {
		t.Errorf("expected SqRef 'B2:B10', got %q", dv.SqRef)
	}
	if dv.Type != "list" {
		t.Errorf("expected Type 'list', got %q", dv.Type)
	}
	if dv.Prompt != "Pick one" {
		t.Errorf("expected Prompt 'Pick one', got %q", dv.Prompt)
	}
	if dv.Formula1 != "ListMonths" {
		t.Errorf("expected Formula1 'ListMonths', got %q", dv.Formula1)
	}
}

func TestResolveDataValidationListValuesFromDefinedName(t *testing.T) {
	localSheetID := 0
	pkg := &parts.Package{
		Workbook: &parts.Workbook{
			Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}},
			DefinedNames: []parts.DefinedName{{
				Name:         "ListMonths",
				LocalSheetID: &localSheetID,
				Value:        "Sheet1!$D$1:$D$3",
			}},
		},
		SharedStrings: &parts.SharedStrings{
			Items: []parts.RichText{
				{PlainText: "January"},
				{PlainText: "February"},
				{PlainText: "March"},
			},
		},
		Worksheets: []*parts.WorksheetEntry{{
			SheetID: 1,
			Name:    "Sheet1",
			Sheet: &parts.Worksheet{
				DataValidations: []parts.DataValidation{{
					SqRef:    "B2:B10",
					Type:     "list",
					Formula1: "ListMonths",
				}},
				Rows: []parts.Row{{
					Index: 1,
					Cells: []parts.Cell{
						{Ref: "D1", Type: "s", Value: "0"},
						{Ref: "D2", Type: "s", Value: "1"},
						{Ref: "D3", Type: "s", Value: "2"},
					},
				}},
			},
		}},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	dv := wb.Sheets[0].DataValidations[0]
	want := []string{"January", "February", "March"}
	if !reflect.DeepEqual(dv.ListValues, want) {
		t.Fatalf("ListValues = %#v, want %#v", dv.ListValues, want)
	}
}

func TestResolveCFRules(t *testing.T) {
	bgRGB := "FFFFCC00"
	themeIdx := 4
	indexedIdx := 10
	ws := &parts.Worksheet{
		ConditionalFmts: []parts.ConditionalFmt{
			{
				SqRef: "A1:C3",
				Rules: []parts.CFRule{
					{Type: "cellIs", Operator: "lessThan", DxfID: 0, Priority: 1, Formula: []string{"0"}},
					{Type: "top10", DxfID: 0, Priority: 2, Percent: true, Bottom: true, Rank: 25},
					{
						Type:     "colorScale",
						Priority: 3,
						ColorScale: &parts.ColorScale{
							Cfvo: []parts.Cfvo{{Type: "min"}, {Type: "percentile", Val: "50"}, {Type: "max"}},
							Colors: []parts.ColorScaleColor{
								{Theme: &themeIdx, Tint: 0.5},
								{Indexed: &indexedIdx},
								{RGB: "FF112233"},
							},
						},
					},
				},
			},
		},
		Rows: []parts.Row{{
			Index: 1,
			Cells: []parts.Cell{{Ref: "A1", Value: "-1"}},
		}},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook: &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Theme: &parts.Theme{
			Elements: parts.ThemeElements{
				ClrScheme: parts.ColorScheme{
					Accent1: &parts.ThemeColor{SrgbClr: &parts.SrgbColor{Val: "4472C4"}},
				},
			},
		},
		Styles: &parts.StyleSheet{
			Colors: &parts.StyleColors{
				IndexedColors: &parts.IndexedColors{
					RGBColors: []parts.IndexedRGBColor{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{RGB: "FF102030"},
					},
				},
			},
			Dxfs: []parts.Dxf{
				{Fill: &parts.DxfFill{PatternFill: &parts.PatternFill{FgColor: &parts.ColorVal{RGB: bgRGB}}}},
			},
		},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if len(sheet.CFRules) != 3 {
		t.Fatalf("expected 3 CFRules, got %d", len(sheet.CFRules))
	}
	cr := sheet.CFRules[0]
	if cr.SqRef != "A1:C3" {
		t.Errorf("expected SqRef 'A1:C3', got %q", cr.SqRef)
	}
	if cr.DxfStyle.BgColor != "FFCC00" {
		t.Errorf("expected BgColor 'FFCC00', got %q", cr.DxfStyle.BgColor)
	}
	if sheet.CFRules[1].Type != "top10" {
		t.Fatalf("expected second rule type 'top10', got %q", sheet.CFRules[1].Type)
	}
	if !sheet.CFRules[1].Percent {
		t.Fatal("expected second rule percent flag to be preserved")
	}
	if !sheet.CFRules[1].Bottom {
		t.Fatal("expected second rule bottom flag to be preserved")
	}
	if sheet.CFRules[1].Rank != 25 {
		t.Fatalf("expected second rule rank 25, got %d", sheet.CFRules[1].Rank)
	}
	if sheet.CFRules[2].ColorScale == nil {
		t.Fatal("expected third rule color scale")
	}
	wantColors := []string{"A1B8E1", "102030", "112233"}
	if !reflect.DeepEqual(sheet.CFRules[2].ColorScale.Colors, wantColors) {
		t.Fatalf("color scale colors = %#v, want %#v", sheet.CFRules[2].ColorScale.Colors, wantColors)
	}
}

func TestResolvePivotTablesIntegration(t *testing.T) {
	// Build a synthetic parts.Package with one worksheet that has a pivot table.
	// This exercises resolvePivotTables without needing a real XLSX file.
	rowGrandTotals := false
	colGrandTotals := true
	showHeaders := false
	pt := &parts.PivotTableDefinition{
		Name:           "Sales Pivot",
		DataCaption:    "Measures",
		RowGrandTotals: &rowGrandTotals,
		ColGrandTotals: &colGrandTotals,
		ShowHeaders:    &showHeaders,
		Location: parts.PivotLocation{
			Ref:            "A3:D10",
			FirstHeaderRow: 1,
			FirstDataRow:   2,
			FirstDataCol:   1,
		},
		PivotFields: []parts.PivotField{
			{Name: "Region", Axis: "axisRow", Items: &parts.PivotItems{Count: 2, Items: []parts.PivotItem{{X: 0}, {X: 1, Hidden: true}}}},
			{Name: "Product"},
			{Name: "Quarter"},
			{Name: "Revenue"},
		},
		RowFields:  []parts.PivotFieldIndex{{Index: 0}, {Index: 1}},
		ColFields:  []parts.PivotFieldIndex{{Index: 2}},
		PageFields: []parts.PageField{{Field: 3, Name: "", Item: intPtrForPivotTest(1)}},
		DataFields: []parts.DataField{
			{Name: "Sum of Revenue", Field: 3, Subtotal: "sum", NumFmtID: 4},
		},
	}
	cache := &parts.PivotCacheDefinition{
		CacheFields: &parts.CacheFields{Count: 4, Fields: []parts.CacheField{
			{Name: "Region", SharedItems: &parts.SharedItems{Count: 2, Items: []parts.SharedItem{{Type: "s", Value: "East"}, {Type: "s", Value: "West"}}}},
			{Name: "Product"}, {Name: "Quarter"}, {Name: "Revenue"},
		}},
		WorksheetSource: &parts.WorksheetSource{
			Sheet: "Data",
			Ref:   "A1:D100",
		},
	}

	pkg := &parts.Package{
		Workbook: &parts.Workbook{
			Sheets: []parts.SheetRef{{Name: "Pivot", SheetID: 1, RelID: "rId1"}},
		},
		Worksheets: []*parts.WorksheetEntry{
			{
				SheetID: 1,
				Name:    "Pivot",
				Sheet:   &parts.Worksheet{},
				PivotTables: []*parts.PivotTableWithCache{
					{Table: pt, Cache: cache},
				},
			},
		},
	}

	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	if len(wb.Sheets) != 1 {
		t.Fatalf("expected 1 sheet, got %d", len(wb.Sheets))
	}
	sheet := wb.Sheets[0]
	if len(sheet.PivotTables) != 1 {
		t.Fatalf("expected 1 pivot table, got %d", len(sheet.PivotTables))
	}
	resolved := sheet.PivotTables[0]
	if resolved.Name != "Sales Pivot" {
		t.Errorf("Name: got %q, want %q", resolved.Name, "Sales Pivot")
	}
	if resolved.LocationRef != "A3:D10" {
		t.Errorf("LocationRef: got %q, want %q", resolved.LocationRef, "A3:D10")
	}
	if resolved.DataCaption != "Measures" || resolved.RowGrandTotals || !resolved.ColGrandTotals || resolved.ShowHeaders {
		t.Errorf("display metadata = %#v", resolved)
	}
	if resolved.SourceSheet != "Data" {
		t.Errorf("SourceSheet: got %q, want %q", resolved.SourceSheet, "Data")
	}
	if resolved.SourceRef != "A1:D100" {
		t.Errorf("SourceRef: got %q, want %q", resolved.SourceRef, "A1:D100")
	}
	if !reflect.DeepEqual(resolved.RowFields, []string{"Region", "Product"}) {
		t.Errorf("RowFields: got %v, want [Region Product]", resolved.RowFields)
	}
	if !reflect.DeepEqual(resolved.ColFields, []string{"Quarter"}) {
		t.Errorf("ColFields: got %v, want [Quarter]", resolved.ColFields)
	}
	if len(resolved.PageFields) != 1 || resolved.PageFields[0] != "Revenue" {
		t.Errorf("PageFields: got %v, want [Revenue]", resolved.PageFields)
	}
	if len(resolved.Fields) != 4 || len(resolved.Fields[0].Items) != 2 || resolved.Fields[0].Items[0].Name != "" || !resolved.Fields[0].Items[1].Hidden {
		t.Errorf("pivot field items = %#v", resolved.Fields)
	}
	if got := pivotCacheItemDisplayValueForIntegrationTest(cache, 0, resolved.Fields[0].Items[0].CacheIndex); got != "East" {
		t.Errorf("pivot cache display value = %q, want East", got)
	}
	if len(resolved.PageFieldSelections) != 1 || resolved.PageFieldSelections[0].FieldName != "Revenue" || resolved.PageFieldSelections[0].ItemIndex != 1 {
		t.Errorf("page selection = %#v", resolved.PageFieldSelections)
	}
	if len(resolved.DataFields) != 1 {
		t.Fatalf("expected 1 data field, got %d", len(resolved.DataFields))
	}
	df := resolved.DataFields[0]
	if df.Name != "Sum of Revenue" {
		t.Errorf("DataField.Name: got %q, want %q", df.Name, "Sum of Revenue")
	}
	if df.Subtotal != "sum" {
		t.Errorf("DataField.Subtotal: got %q, want %q", df.Subtotal, "sum")
	}
	if df.NumFmtID != 4 {
		t.Errorf("DataField.NumFmtID: got %d, want %d", df.NumFmtID, 4)
	}
}

func pivotCacheItemDisplayValueForIntegrationTest(cache *parts.PivotCacheDefinition, fieldIndex, itemIndex int) string {
	if cache == nil || cache.CacheFields == nil || fieldIndex < 0 || fieldIndex >= len(cache.CacheFields.Fields) {
		return ""
	}
	items := cache.CacheFields.Fields[fieldIndex].SharedItems
	if items == nil || itemIndex < 0 || itemIndex >= len(items.Items) {
		return ""
	}
	return items.Items[itemIndex].Value
}

func intPtrForPivotTest(v int) *int { return &v }

func TestResolveFilterColumnsDiscreteFilters(t *testing.T) {
	ws := &parts.Worksheet{
		AutoFilter: &parts.AutoFilter{
			Ref: "A1:C1",
			FilterColumns: []parts.FilterColumn{
				{
					ColID: 1,
					Filters: &parts.FilterList{
						Filters: []parts.Filter{{Val: "Apples"}, {Val: "Bananas"}},
					},
				},
			},
		},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if sheet.AutoFilterRef != "A1:C1" {
		t.Errorf("AutoFilterRef = %q, want A1:C1", sheet.AutoFilterRef)
	}
	if len(sheet.FilterColumns) != 1 {
		t.Fatalf("FilterColumns len = %d, want 1", len(sheet.FilterColumns))
	}
	fc := sheet.FilterColumns[0]
	if fc.ColIndex != 1 {
		t.Errorf("ColIndex = %d, want 1", fc.ColIndex)
	}
	wantFilters := []string{"Apples", "Bananas"}
	if !reflect.DeepEqual(fc.Filters, wantFilters) {
		t.Errorf("Filters = %v, want %v", fc.Filters, wantFilters)
	}
}

func TestResolveFilterColumnsCustomFilter(t *testing.T) {
	ws := &parts.Worksheet{
		AutoFilter: &parts.AutoFilter{
			Ref: "B1:D10",
			FilterColumns: []parts.FilterColumn{
				{
					ColID: 0,
					CustomFilters: &parts.CustomFilters{
						And: true,
						Filter: []parts.CustomFilter{
							{Operator: "greaterThanOrEqual", Val: "10"},
							{Operator: "lessThan", Val: "100"},
						},
					},
				},
			},
		},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if len(sheet.FilterColumns) != 1 {
		t.Fatalf("FilterColumns len = %d, want 1", len(sheet.FilterColumns))
	}
	fc := sheet.FilterColumns[0]
	if !fc.CustomAnd {
		t.Errorf("CustomAnd = false, want true")
	}
	if len(fc.CustomFilters) != 2 {
		t.Fatalf("CustomFilters len = %d, want 2", len(fc.CustomFilters))
	}
	if fc.CustomFilters[0].Operator != "greaterThanOrEqual" || fc.CustomFilters[0].Val != "10" {
		t.Errorf("cond0 = %+v, want {greaterThanOrEqual 10}", fc.CustomFilters[0])
	}
	if fc.CustomFilters[1].Operator != "lessThan" || fc.CustomFilters[1].Val != "100" {
		t.Errorf("cond1 = %+v, want {lessThan 100}", fc.CustomFilters[1])
	}
}

func TestResolveFilterColumnsTop10(t *testing.T) {
	ws := &parts.Worksheet{
		AutoFilter: &parts.AutoFilter{
			Ref: "A1:E1",
			FilterColumns: []parts.FilterColumn{
				{
					ColID: 2,
					Top10: &parts.FilterTop10{Top: true, Percent: false, Val: 5},
				},
			},
		},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if len(sheet.FilterColumns) != 1 {
		t.Fatalf("FilterColumns len = %d, want 1", len(sheet.FilterColumns))
	}
	fc := sheet.FilterColumns[0]
	if fc.ColIndex != 2 {
		t.Errorf("ColIndex = %d, want 2", fc.ColIndex)
	}
	if fc.Top10 == nil {
		t.Fatal("Top10 is nil")
	}
	if !fc.Top10.Top || fc.Top10.Val != 5 {
		t.Errorf("Top10 = %+v, want {Top:true Val:5}", fc.Top10)
	}
}

func TestResolveFilterColumnsDynamicAndSortState(t *testing.T) {
	ws := &parts.Worksheet{
		AutoFilter: &parts.AutoFilter{
			Ref: "A1:C10",
			FilterColumns: []parts.FilterColumn{
				{
					ColID:         0,
					DynamicFilter: &parts.DynamicFilter{Type: "lastMonth"},
				},
			},
			SortState: &parts.SortState{
				Ref:           "A1:C10",
				CaseSensitive: true,
				Conditions: []parts.SortCondition{
					{Ref: "C2:C10", Descending: true},
					{Ref: "A2:A10"},
				},
			},
		},
	}
	entry := &parts.WorksheetEntry{SheetID: 1, Name: "Sheet1", Sheet: ws}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if len(sheet.FilterColumns) != 1 {
		t.Fatalf("FilterColumns len = %d, want 1", len(sheet.FilterColumns))
	}
	if sheet.FilterColumns[0].Dynamic == nil || sheet.FilterColumns[0].Dynamic.Type != "lastMonth" {
		t.Fatalf("Dynamic = %#v", sheet.FilterColumns[0].Dynamic)
	}
	if sheet.SortState == nil {
		t.Fatal("SortState is nil")
	}
	if sheet.SortState.Ref != "A1:C10" || !sheet.SortState.CaseSensitive {
		t.Fatalf("SortState = %#v", sheet.SortState)
	}
	if len(sheet.SortState.Conditions) != 2 {
		t.Fatalf("SortState.Conditions len = %d, want 2", len(sheet.SortState.Conditions))
	}
	if sheet.SortState.Conditions[0].Ref != "C2:C10" || !sheet.SortState.Conditions[0].Descending {
		t.Fatalf("condition0 = %#v", sheet.SortState.Conditions[0])
	}
	if sheet.SortState.Conditions[1].Ref != "A2:A10" {
		t.Fatalf("condition1 = %#v", sheet.SortState.Conditions[1])
	}
}

func TestResolvePreservesLegacyDrawingVMLForCanonicalRoundTrip(t *testing.T) {
	raw := []byte(`<xml xmlns:v="urn:schemas-microsoft-com:vml"><v:shape id="custom" data-unknown="keep-me"/></xml>`)
	entry := &parts.WorksheetEntry{
		SheetID:          1,
		Name:             "Sheet1",
		Sheet:            &parts.Worksheet{},
		LegacyDrawingVML: raw,
		VMLObjects: []parts.VMLObject{{
			ID: "custom", Type: "#custom", Style: "width:20pt", Text: "Hello",
		}},
	}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if string(sheet.LegacyDrawingVML) != string(raw) {
		t.Fatalf("legacy VML = %q, want exact source bytes %q", sheet.LegacyDrawingVML, raw)
	}
	if len(sheet.VMLObjects) != 1 || sheet.VMLObjects[0].ID != "custom" || sheet.VMLObjects[0].Text != "Hello" {
		t.Fatalf("VML objects = %#v", sheet.VMLObjects)
	}
}

func TestResolvePreservesHyperlinkAndCommentMetadata(t *testing.T) {
	entry := &parts.WorksheetEntry{
		SheetID: 1, Name: "Sheet1",
		Sheet: &parts.Worksheet{
			Rows:       []parts.Row{{Index: 1, Cells: []parts.Cell{{Ref: "A1", Type: "inlineStr", InlineStr: &parts.RichText{PlainText: "link"}}}}},
			Hyperlinks: []parts.Hyperlink{{Ref: "A1:B1", RelID: "rId1", Display: "Open docs", Tooltip: "Documentation"}},
		},
		HyperlinkURLs: map[string]string{"rId1": "https://example.com/docs"},
		Comments: &parts.Comments{
			Authors: []string{"Ada"},
			CommentList: []parts.CommentEntry{{
				Ref: "A1", AuthorID: 0,
				Text: parts.RichText{Runs: []parts.RTRun{{Text: "Review ", Properties: &parts.RTProps{Bold: &struct{}{}}}, {Text: "this"}}},
			}},
		},
	}
	pkg := &parts.Package{
		Workbook:   &parts.Workbook{Sheets: []parts.SheetRef{{Name: "Sheet1", SheetID: 1}}},
		Worksheets: []*parts.WorksheetEntry{entry},
	}
	wb, err := resolve.ResolvePackage(pkg)
	if err != nil {
		t.Fatalf("ResolvePackage: %v", err)
	}
	sheet := wb.Sheets[0]
	if len(sheet.Hyperlinks) != 1 {
		t.Fatalf("hyperlinks = %d, want 1", len(sheet.Hyperlinks))
	}
	hl := sheet.Hyperlinks[0]
	if hl.Ref != "A1:B1" || hl.URL != "https://example.com/docs" || hl.Display != "Open docs" || hl.Tooltip != "Documentation" {
		t.Fatalf("hyperlink metadata = %#v", hl)
	}
	if sheet.Rows[0].Cells[0].HyperlinkURL != "https://example.com/docs" {
		t.Fatalf("cell hyperlink compatibility view lost")
	}
	note := sheet.Notes["A1"]
	if note == nil || note.Author != "Ada" || note.Text != "Review this" || len(note.RichText) != 2 || !note.RichText[0].Bold {
		t.Fatalf("note metadata = %#v", note)
	}
	if sheet.Comments["A1"] != "Review this" {
		t.Fatalf("plain comment compatibility view lost")
	}
}
