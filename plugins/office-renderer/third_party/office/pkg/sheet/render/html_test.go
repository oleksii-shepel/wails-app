package render_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"office/pkg/diag"
	"office/pkg/ooxml/chart"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/opc"
	"office/pkg/sheet/parsers/legacy"
	"office/pkg/sheet/parsers/ooxml/parts"
	"office/pkg/sheet/parsers/ooxml/resolve"
	"office/pkg/sheet/render"
)

func TestXlsxCorpusRender(t *testing.T) {
	fixturesDir := repoPath(t, "testdata", "xlsx")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	var count, totalDiags int
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".xlsx") {
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
			totalDiags += countUnexpectedDiagnostics(wb.Diagnostics)
			html, err := render.HTML(wb)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, "<!DOCTYPE html>") || !strings.Contains(html, "<table") {
				t.Fatalf("unexpected HTML output")
			}
			if !strings.Contains(html, "<td") {
				t.Fatalf("no cells rendered")
			}
		})
	}

	if count == 0 {
		t.Fatal("no xlsx fixtures found")
	}
	if totalDiags != 0 {
		t.Fatalf("total unexpected corpus diagnostics = %d, want 0", totalDiags)
	}
}

func TestXlsCorpusRender(t *testing.T) {
	fixturesDir := repoPath(t, "testdata", "xls")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	var count, totalDiags int
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".xls") {
			continue
		}
		count++
		path := filepath.Join(fixturesDir, entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			wb, err := legacy.ParseXls(path)
			if err != nil {
				if isLegacyEncryptedWorkbookError(err) {
					t.Skip("encrypted workbook requires password")
				}
				t.Fatalf("parse legacy XLS: %v", err)
			}
			totalDiags += countUnexpectedDiagnostics(wb.Diagnostics)

			html, err := render.HTML(wb)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if !strings.Contains(html, "<!DOCTYPE html>") || !strings.Contains(html, "<table") {
				t.Fatalf("unexpected HTML output")
			}

			// Corpus rendering is also a determinism gate: canonical XLS IR must
			// produce byte-identical HTML when rendered repeatedly.
			again, err := render.HTML(wb)
			if err != nil {
				t.Fatalf("second render: %v", err)
			}
			if html != again {
				t.Fatal("legacy XLS render is not deterministic")
			}
		})
	}

	if count == 0 {
		t.Fatal("no xls fixtures found")
	}
	if totalDiags != 0 {
		t.Fatalf("total unexpected legacy corpus diagnostics = %d, want 0", totalDiags)
	}
}

func TestHTMLIncludesDiagnosticsDetails(t *testing.T) {
	wb := &resolve.Workbook{
		Diagnostics: diag.List{{
			Severity: diag.SeverityWarning,
			Code:     "sheet/unsupported-smartart",
			Message:  "SmartArt content is not supported and was skipped",
		}},
		Sheets: []*resolve.Sheet{{Name: "Sheet1"}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "<details class=\"diagnostics\">") {
		t.Fatalf("expected diagnostics details block in output: %s", out)
	}
	if !strings.Contains(out, "[sheet/unsupported-smartart]") {
		t.Fatalf("expected diagnostic code in output: %s", out)
	}
}

func TestHTMLRendersWorkbookExternalLinks(t *testing.T) {
	wb := &resolve.Workbook{
		ExternalLinks: []*resolve.WorkbookExternalLink{{
			Target:     "file:///C:/workbooks/source.xlsx",
			SheetNames: []string{"Prices"},
			DefinedNames: []*resolve.WorkbookExternalDefinedName{{
				Name:     "ExtRange",
				RefersTo: "Prices!$A$1:$B$2",
			}},
		}},
		Sheets: []*resolve.Sheet{{Name: "Sheet1"}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		"External References",
		"External link metadata is preserved, and direct external cell or range refs can use cached external workbook values when present, but external workbooks are not opened or refreshed during HTML rendering.",
		"file:///C:/workbooks/source.xlsx",
		"Prices",
		"ExtRange=Prices!$A$1:$B$2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected external reference output %q in HTML:\n%s", want, out)
		}
	}
}

func TestHTMLComputesFormulaFromCachedExternalWorkbookValues(t *testing.T) {
	wb := &resolve.Workbook{
		ExternalLinks: []*resolve.WorkbookExternalLink{{
			Target: "file:///C:/workbooks/source.xlsx",
			CachedSheets: []*resolve.WorkbookExternalSheet{{
				ID:   0,
				Name: "Prices",
				Cells: []*resolve.WorkbookExternalCell{
					{Ref: "A1", Type: "n", Value: "7"},
					{Ref: "B1", Type: "n", Value: "3"},
				},
			}},
		}},
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellFormula, Formula: `SUM([1]Prices!A1:B1)`,
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">10<") {
		t.Fatalf("expected computed external formula result in HTML:\n%s", out)
	}
}

func TestHTMLComputesGetPivotDataFromWorkbookPivotSource(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{
			{
				Name: "Data",
				Rows: []*resolve.Row{
					{Index: 1, Cells: []*resolve.Cell{
						{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "Region"},
						{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellString, StrValue: "Sales"},
					}},
					{Index: 2, Cells: []*resolve.Cell{
						{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellString, StrValue: "North"},
						{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 10},
					}},
					{Index: 3, Cells: []*resolve.Cell{
						{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellString, StrValue: "South"},
						{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 20},
					}},
					{Index: 4, Cells: []*resolve.Cell{
						{Row: 4, Col: 1, Ref: "A4", Kind: resolve.CellString, StrValue: "North"},
						{Row: 4, Col: 2, Ref: "B4", Kind: resolve.CellNumber, NumValue: 5},
					}},
				},
			},
			{
				Name: "Pivot",
				Rows: []*resolve.Row{{Index: 3, Cells: []*resolve.Cell{{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellString, StrValue: "Pivot"}}}},
				PivotTables: []*resolve.PivotTable{{
					Name:        "SalesPivot",
					LocationRef: "A3:C8",
					SourceSheet: "Data",
					SourceRef:   "A1:B4",
					RowFields:   []string{"Region"},
					DataFields: []*resolve.PivotDataField{{
						Name:            "Sum of Sales",
						SourceFieldName: "Sales",
						Subtotal:        "sum",
					}},
				}},
			},
			{
				Name: "Summary",
				Rows: []*resolve.Row{
					{Index: 1, Cells: []*resolve.Cell{{
						Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellFormula,
						Formula: `GETPIVOTDATA("Sum of Sales",'Pivot'!A3,"Region","North")`,
					}}},
					{Index: 2, Cells: []*resolve.Cell{{
						Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellFormula,
						Formula: `GETPIVOTDATA("Sum of Sales",'Pivot'!A3)`,
					}}},
				},
			},
		},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{">15<", ">35<"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected computed GETPIVOTDATA result %q in HTML:\n%s", want, out)
		}
	}
}

func TestHTMLComputesStructuredTableColumnReferences(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Tables: []*resolve.Table{{
				ID:             1,
				Name:           "SalesTable",
				DisplayName:    "SalesTable",
				Ref:            "A1:B4",
				HeaderRowCount: 1,
				Columns: []*resolve.TableColumn{
					{ID: 1, Name: "Region"},
					{ID: 2, Name: "Sales"},
				},
			}},
			Rows: []*resolve.Row{
				{Index: 1, Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "Region"},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellString, StrValue: "Sales"},
				}},
				{Index: 2, Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellString, StrValue: "North"},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 2, Col: 4, Ref: "D2", Kind: resolve.CellFormula, Formula: `SUM(SalesTable[Sales])`},
				}},
				{Index: 3, Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellString, StrValue: "South"},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 20},
				}},
				{Index: 4, Cells: []*resolve.Cell{
					{Row: 4, Col: 1, Ref: "A4", Kind: resolve.CellString, StrValue: "North"},
					{Row: 4, Col: 2, Ref: "B4", Kind: resolve.CellNumber, NumValue: 5},
					{Row: 4, Col: 4, Ref: "D4", Kind: resolve.CellFormula, Formula: `ROWS(SalesTable[#All])`},
				}},
			},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{">35<", ">4<"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected computed structured-table result %q in HTML:\n%s", want, out)
		}
	}
}

func TestHTMLRendersWorkbookConnections(t *testing.T) {
	wb := &resolve.Workbook{
		Connections: []*resolve.WorkbookConnection{
			{
				Name:          "Sales DB",
				Kind:          "database",
				RefreshOnLoad: true,
				Background:    true,
				SaveData:      true,
				SourceFile:    "sales.odc",
				Command:       "SELECT * FROM Sales",
			},
			{
				Name:          "Rates Feed",
				Kind:          "web",
				URL:           "https://example.com/rates.csv",
				WebPost:       "region=us",
				WebHTMLTables: true,
				WebSourceData: true,
				WebFirstRow:   true,
				WebTextDates:  true,
			},
		},
		Sheets: []*resolve.Sheet{{Name: "Sheet1"}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		"External Data Connections",
		"Connection metadata is preserved, but external queries are not executed during HTML rendering.",
		"Sales DB",
		"database",
		"sales.odc",
		"refreshOnLoad, background, saveData, SELECT * FROM Sales",
		"Rates Feed",
		"https://example.com/rates.csv",
		"post=region=us, htmlTables, sourceData, firstRow, textDates",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected workbook connection output %q in HTML:\n%s", want, out)
		}
	}
}

func TestHTMLExternalizesImagesAndFontsWhenConfigured(t *testing.T) {
	fontContentType, fontData := sampleFontBytes(t, "ppt/fonts/Oswald-regular.fntdata")
	wb := &resolve.Workbook{
		EmbeddedFonts: []*resolve.EmbeddedFont{{
			Family: "Oswald",
			Faces: []*resolve.EmbeddedFontFace{{
				Family:      "Oswald",
				ContentType: fontContentType,
				Data:        fontData,
			}},
		}},
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "Hello",
					Style: resolve.CellStyle{
						FontFamily: "Source Code Pro",
					},
				}},
			}},
			Drawings: []*resolve.Drawing{{
				Name: "Logo",
				Type: resolve.DrawingTypeImage,
				Position: resolve.DrawingPosition{
					Anchor: resolve.AnchorAbsolute,
					X:      0,
					Y:      0,
					Width:  914400,
					Height: 914400,
				},
				Image: &resolve.DrawingImage{
					ContentType: "image/png",
					Data:        []byte("image-bytes"),
					AltText:     "Logo",
				},
			}},
		}},
	}

	html, assets, err := render.HTMLWithOptions(wb, render.HTMLOptions{
		ImageMode:        render.HTMLImageModeExternal,
		ExternalImageDir: "sample_files",
		FontMode:         render.HTMLFontModeExternal,
		ExternalFontDir:  "sample_fonts",
	})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if strings.Contains(html, "data:image/") || strings.Contains(html, "data:font/") {
		t.Fatalf("expected external assets in html: %s", html)
	}
	if !strings.Contains(html, `src="sample_files/image-001.png"`) {
		t.Fatalf("expected external image asset path in html: %s", html)
	}
	if !strings.Contains(html, `url("sample_fonts/font-001.`) {
		t.Fatalf("expected external font asset path in html: %s", html)
	}
	if len(assets) < 2 {
		t.Fatalf("expected sidecar assets, got %d", len(assets))
	}
}

func TestHTMLUsesPlaceholderForNonBrowserDrawingImages(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "Vector",
				}},
			}},
			Drawings: []*resolve.Drawing{{
				Name: "VectorLogo",
				Type: resolve.DrawingTypeImage,
				Position: resolve.DrawingPosition{
					Anchor: resolve.AnchorAbsolute,
					X:      0,
					Y:      0,
					Width:  914400,
					Height: 914400,
				},
				Image: &resolve.DrawingImage{
					ContentType: "image/x-emf",
					Data:        []byte("vector-bytes"),
				},
			}},
		}},
	}

	html, assets, err := render.HTMLWithOptions(wb, render.HTMLOptions{
		ImageMode:        render.HTMLImageModeExternal,
		ExternalImageDir: "sample_files",
	})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if strings.Contains(html, "<img") {
		t.Fatalf("expected placeholder instead of img tag: %s", html)
	}
	if !strings.Contains(html, "drawing--image-placeholder") || !strings.Contains(html, "[image/x-emf]") {
		t.Fatalf("expected emf placeholder in html: %s", html)
	}
	if len(assets) != 0 {
		t.Fatalf("expected no sidecar assets for non-browser vector placeholder, got %d", len(assets))
	}
}

func TestHTMLGroupsRadioControlsByFirstButtonAndGroupBoxLink(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 8, Ref: "H1", Kind: resolve.CellNumber, NumValue: 2},
				},
			}},
			FormControls: []*resolve.FormControl{
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
				},
			},
		}},
	}

	html, _, err := render.HTMLWithOptions(wb, render.HTMLOptions{})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if !strings.Contains(html, `name="sheet-radio-1" disabled> North`) {
		t.Fatalf("expected first radio to be cleared by linked cell index 2 in group 1: %s", html)
	}
	if !strings.Contains(html, `name="sheet-radio-1" checked disabled> South`) {
		t.Fatalf("expected second radio to be selected by linked cell index 2 via group-box link override: %s", html)
	}
	if !strings.Contains(html, `name="sheet-radio-2" checked disabled> West`) {
		t.Fatalf("expected third radio to start group 2 and be selected by linked cell index 1: %s", html)
	}
	if !strings.Contains(html, `name="sheet-radio-2" disabled> East`) {
		t.Fatalf("expected fourth radio to stay in group 2: %s", html)
	}
}

func TestHTMLRendersFormControlOptionsAndSelections(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{
				{
					Index: 1,
					Cells: []*resolve.Cell{
						{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 2},
						{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellString, StrValue: "Open"},
						{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellString, StrValue: "North"},
					},
				},
				{
					Index: 2,
					Cells: []*resolve.Cell{
						{Row: 2, Col: 4, Ref: "D2", Kind: resolve.CellString, StrValue: "Closed"},
						{Row: 2, Col: 5, Ref: "E2", Kind: resolve.CellString, StrValue: "South"},
					},
				},
				{
					Index: 3,
					Cells: []*resolve.Cell{
						{Row: 3, Col: 4, Ref: "D3", Kind: resolve.CellString, StrValue: "Pending"},
						{Row: 3, Col: 5, Ref: "E3", Kind: resolve.CellString, StrValue: "East"},
					},
				},
				{
					Index: 4,
					Cells: []*resolve.Cell{
						{Row: 4, Col: 5, Ref: "E4", Kind: resolve.CellString, StrValue: "West"},
					},
				},
			},
			FormControls: []*resolve.FormControl{
				{
					Type:      "Drop",
					AltText:   "Status",
					Anchor:    "0, 0, 0, 0, 2, 0, 1, 0",
					FmlaLink:  "$B$1",
					FmlaRange: "$D$1:$D$3",
				},
				{
					Type:          "List",
					AltText:       "Regions",
					Anchor:        "3, 0, 0, 0, 5, 0, 4, 0",
					FmlaRange:     "$E$1:$E$4",
					SelectionType: "extended",
					Selected:      []int{2, 4},
					DropLines:     5,
				},
			},
		}},
	}

	html, _, err := render.HTMLWithOptions(wb, render.HTMLOptions{})
	if err != nil {
		t.Fatalf("HTMLWithOptions: %v", err)
	}
	if !strings.Contains(html, `<option>Open</option><option selected>Closed</option><option>Pending</option>`) {
		t.Fatalf("expected dropdown options with linked-cell-selected second item: %s", html)
	}
	if !strings.Contains(html, `<select class="form-control form-control--list" size="5"`) {
		t.Fatalf("expected list control with drop-lines-derived size: %s", html)
	}
	if !strings.Contains(html, `<option>North</option><option selected>South</option><option>East</option><option selected>West</option>`) {
		t.Fatalf("expected list control to render source-range options with explicit selections: %s", html)
	}
}

func TestHTMLRendersChartsUsingDrawingSize(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "Chart",
				}},
			}},
			Drawings: []*resolve.Drawing{{
				Name: "Market Cap",
				Type: resolve.DrawingTypeChart,
				Position: resolve.DrawingPosition{
					Anchor: resolve.AnchorAbsolute,
					Width:  800 * 9525,
					Height: 300 * 9525,
				},
				Chart: &resolve.DrawingChart{
					Chart: &chart.Chart{
						ChartType:  chart.ChartArea,
						Categories: []string{"Dec", "Jan", "Feb", "Mar"},
						Series: []chart.Series{{
							Name:   "Close",
							Values: []float64{145, 130, 138, 122},
						}},
						Legend: chart.Legend{Show: true},
					},
					SVG: `<svg xmlns="http://www.w3.org/2000/svg" width="600" height="400" viewBox="0 0 600 400"></svg>`,
				},
			}},
		}},
	}

	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `width="800" height="300" viewBox="0 0 800 300"`) {
		t.Fatalf("expected chart svg sized to drawing bounds, got: %s", html)
	}
	if strings.Contains(html, `viewBox="0 0 600 400"`) {
		t.Fatalf("expected chart svg not to use fixed 600x400 viewBox, got: %s", html)
	}
}

func TestHTMLRendersFallbackChartSVGUsingOriginalViewBox(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "Chart",
				}},
			}},
			Drawings: []*resolve.Drawing{{
				Name: "Market Cap",
				Type: resolve.DrawingTypeChart,
				Position: resolve.DrawingPosition{
					Anchor: resolve.AnchorAbsolute,
					Width:  800 * 9525,
					Height: 300 * 9525,
				},
				Chart: &resolve.DrawingChart{
					SVG: `<svg xmlns="http://www.w3.org/2000/svg" width="600" height="400" viewBox="0 0 600 400"><path d="M50,300 L550,100"/></svg>`,
				},
			}},
		}},
	}

	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `width="800" height="300" viewBox="0 0 600 400"`) {
		t.Fatalf("expected fallback chart svg to keep its original viewBox while resizing, got: %s", html)
	}
	if strings.Contains(html, `viewBox="0 0 800 300"`) {
		t.Fatalf("expected fallback chart svg not to rewrite its viewBox, got: %s", html)
	}
}

func TestHTMLRendersAdvancedShapeDrawingsAsSVG(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "shape",
				}},
			}},
			Drawings: []*resolve.Drawing{{
				Name: "Process",
				Type: resolve.DrawingTypeShape,
				Position: resolve.DrawingPosition{
					Anchor: resolve.AnchorAbsolute,
					Width:  200 * 9525,
					Height: 100 * 9525,
				},
				Shape: &resolve.DrawingShape{
					Geometry:    "rightArrowCallout",
					FillColor:   "FF7043",
					LineColor:   "1E1E1E",
					LineWidthPt: 1.5,
				},
			}},
		}},
	}

	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `drawing--shape-vector`) {
		t.Fatalf("expected vector shape wrapper in output: %s", html)
	}
	if !strings.Contains(html, `<svg class="drawing__shape-vector" viewBox="0 0 200.0000 100.0000"`) {
		t.Fatalf("expected shape svg sized to drawing bounds, got: %s", html)
	}
	if !strings.Contains(html, `L 200.0000 50.0000`) {
		t.Fatalf("expected right-arrow callout tip in svg path, got: %s", html)
	}
	if strings.Contains(html, `background:#FF7043`) {
		t.Fatalf("did not expect CSS box fallback for advanced shape, got: %s", html)
	}
}

func TestHTMLAppliesSheetShapeEffects(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "shape",
				}},
			}},
			Drawings: []*resolve.Drawing{{
				Name: "Glowing shape",
				Type: resolve.DrawingTypeShape,
				Position: resolve.DrawingPosition{
					Anchor: resolve.AnchorAbsolute,
					Width:  200 * 9525,
					Height: 100 * 9525,
				},
				Shape: &resolve.DrawingShape{
					Geometry:  "rightArrowCallout",
					FillColor: "FF7043",
					Effects: drawing.EffectList{Effects: []drawing.Effect{
						drawing.Glow{Color: "FF0000", Alpha: 1.0, Radius: 4.0},
						drawing.Reflection{Alpha: 0.25, Blur: 2.0, Distance: 3.0, Direction: 90.0},
					}},
				},
			}},
		}},
	}

	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `overflow:visible`) {
		t.Fatalf("expected effect wrapper overflow override, got: %s", html)
	}
	if !strings.Contains(html, `filter:drop-shadow(0 0 4.0pt #FF0000)`) {
		t.Fatalf("expected glow filter in output, got: %s", html)
	}
	if !strings.Contains(html, `box-shadow:`) {
		t.Fatalf("expected reflection box-shadow in output, got: %s", html)
	}
}

func TestHTMLAppliesSheetShapeThreeDEffects(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "shape",
				}},
			}},
			Drawings: []*resolve.Drawing{{
				Name: "3D shape",
				Type: resolve.DrawingTypeShape,
				Position: resolve.DrawingPosition{
					Anchor: resolve.AnchorAbsolute,
					Width:  200 * 9525,
					Height: 100 * 9525,
				},
				Shape: &resolve.DrawingShape{
					Geometry:  "rightArrowCallout",
					FillColor: "FF7043",
					Effects: drawing.EffectList{Effects: []drawing.Effect{
						drawing.ThreeD{CameraPreset: "legacyPerspectiveTopLeft", LightRig: "threePt", LightDirection: "tl", Material: "metal", RotationX: -15, RotationY: -10, ExtrusionDepth: 6, ContourWidth: 2, BevelTopHeight: 1, ExtrusionColor: "595959", ContourColor: "D9D9D9"},
					}},
				},
			}},
		}},
	}

	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `perspective(960px)`) {
		t.Fatalf("expected 3D perspective transform in output, got: %s", html)
	}
	if !strings.Contains(html, `transform-style:preserve-3d`) {
		t.Fatalf("expected preserve-3d style in output, got: %s", html)
	}
	if !strings.Contains(html, `drop-shadow(`) {
		t.Fatalf("expected 3D drop-shadow approximation in output, got: %s", html)
	}
	if !strings.Contains(html, `mix-blend-mode:screen`) || !strings.Contains(html, `mix-blend-mode:multiply`) {
		t.Fatalf("expected 3D scene-light overlays in output, got: %s", html)
	}
	if !strings.Contains(html, `shape-fill-overlay-0`) || !strings.Contains(html, `shape-fill-overlay-1`) {
		t.Fatalf("expected SVG defs for 3D light gradients in output, got: %s", html)
	}
}

func TestHTMLRoutesBasicSheetThreeDShapeThroughVectorRenderer(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "shape",
				}},
			}},
			Drawings: []*resolve.Drawing{{
				Name: "3D ellipse",
				Type: resolve.DrawingTypeShape,
				Position: resolve.DrawingPosition{
					Anchor: resolve.AnchorAbsolute,
					Width:  200 * 9525,
					Height: 100 * 9525,
				},
				Shape: &resolve.DrawingShape{
					Geometry:  "ellipse",
					FillColor: "FF7043",
					Effects: drawing.EffectList{Effects: []drawing.Effect{
						drawing.ThreeD{CameraPreset: "legacyPerspectiveTopLeft", LightRig: "threePt", LightDirection: "tl", Material: "metal", ExtrusionDepth: 6, ContourWidth: 2, BevelTopHeight: 1, ExtrusionColor: "595959", ContourColor: "D9D9D9"},
					}},
				},
			}},
		}},
	}

	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `drawing--shape-vector`) {
		t.Fatalf("expected basic 3D shape to use vector renderer, got: %s", html)
	}
	if !strings.Contains(html, `<svg class="drawing__shape-vector"`) {
		t.Fatalf("expected SVG output for basic 3D shape, got: %s", html)
	}
	if !strings.Contains(html, `mix-blend-mode:screen`) || !strings.Contains(html, `mix-blend-mode:multiply`) {
		t.Fatalf("expected scene-light overlays on basic 3D shape, got: %s", html)
	}
	if strings.Contains(html, `border-radius:50%`) {
		t.Fatalf("did not expect div fallback border-radius for basic 3D shape, got: %s", html)
	}
}

func TestHTMLRendersConnectorDrawingsAsSVG(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "connector",
				}},
			}},
			Drawings: []*resolve.Drawing{{
				Name: "Flow",
				Type: resolve.DrawingTypeConnector,
				Position: resolve.DrawingPosition{
					Anchor: resolve.AnchorAbsolute,
					Width:  200 * 9525,
					Height: 100 * 9525,
				},
				Shape: &resolve.DrawingShape{
					Geometry:      "line",
					ConnectorType: "bentConnector3",
					LineColor:     "00AAFF",
					LineWidthPt:   3,
					LineStyle: &drawing.LineStyle{
						Width: 3,
						Color: "00AAFF",
						Dash:  drawing.DashPatternDashDot,
						Cap:   drawing.LineCapRound,
						Join:  drawing.LineJoinBevel,
						HeadEnd: &drawing.Arrowhead{
							Type:   drawing.ArrowheadTriangle,
							Size:   drawing.ArrowheadSizeMedium,
							Length: drawing.ArrowheadSizeMedium,
						},
					},
				},
			}},
		}},
	}

	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `drawing--connector`) {
		t.Fatalf("expected connector wrapper in output: %s", html)
	}
	if !strings.Contains(html, `<svg class="drawing__connector-vector" viewBox="0 0 200.0000 100.0000"`) {
		t.Fatalf("expected connector svg sized to drawing bounds, got: %s", html)
	}
	if !strings.Contains(html, `stroke-dasharray="3, 3, 1, 3"`) {
		t.Fatalf("expected connector dash array in output, got: %s", html)
	}
	if !strings.Contains(html, `stroke-linecap="round"`) || !strings.Contains(html, `stroke-linejoin="bevel"`) {
		t.Fatalf("expected connector cap/join styles in output, got: %s", html)
	}
	if !strings.Contains(html, `<polygon points=`) {
		t.Fatalf("expected arrowhead polygon in connector output, got: %s", html)
	}
}

func isLegacyEncryptedWorkbookError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "encrypted workbook") &&
		(strings.Contains(message, "invalid password") || strings.Contains(message, "password"))
}

func countUnexpectedDiagnostics(diagnostics diag.List) int {
	total := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "sheet/unsupported-chart" || diagnostic.Code == "sheet/unsupported-vml" {
			continue
		}
		total++
	}
	return total
}

// DiagnosticCategory groups diagnostics by feature area for tracking coverage
type DiagnosticCategory string

const (
	CatChart     DiagnosticCategory = "chart"
	CatDrawing   DiagnosticCategory = "drawing"
	CatFormula   DiagnosticCategory = "formula"
	CatHyperlink DiagnosticCategory = "hyperlink"
	CatStyle     DiagnosticCategory = "style"
	CatOther     DiagnosticCategory = "other"
)

// categorizeDiagnostic assigns a diagnostic code to a category
func categorizeDiagnostic(code string) DiagnosticCategory {
	switch {
	case contains(code, "chart"):
		return CatChart
	case contains(code, "drawing", "vml", "shape", "image"):
		return CatDrawing
	case contains(code, "formula", "calc", "shared"):
		return CatFormula
	case contains(code, "hyperlink", "link"):
		return CatHyperlink
	case contains(code, "style", "font", "color", "border"):
		return CatStyle
	default:
		return CatOther
	}
}

func contains(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// CategorizeDiagnostics groups diagnostics by feature area
func CategorizeDiagnostics(diagnostics diag.List) map[DiagnosticCategory]diag.List {
	categorized := make(map[DiagnosticCategory]diag.List)
	for _, d := range diagnostics {
		cat := categorizeDiagnostic(d.Code)
		categorized[cat] = append(categorized[cat], d)
	}
	return categorized
}

func TestHTMLRendersMergedCells(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Merged",
			Rows: []*resolve.Row{
				{
					Index: 1,
					Cells: []*resolve.Cell{
						{
							Row:      1,
							Col:      1,
							Ref:      "A1",
							Kind:     resolve.CellString,
							StrValue: "Header",
						},
						{
							Row:      1,
							Col:      3,
							Ref:      "C1",
							Kind:     resolve.CellString,
							StrValue: "Side",
						},
					},
				},
				{
					Index: 2,
					Cells: []*resolve.Cell{{
						Row:      2,
						Col:      3,
						Ref:      "C2",
						Kind:     resolve.CellString,
						StrValue: "Tail",
					}},
				},
			},
			MergedCells: []*resolve.MergedCell{{
				TopRow:    1,
				LeftCol:   1,
				BottomRow: 2,
				RightCol:  2,
			}},
		}},
	}

	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `class="cell" data-col-index="1" rowspan="2" colspan="2"`) &&
		!strings.Contains(html, `class="cell" data-col-index="1" colspan="2" rowspan="2"`) &&
		!strings.Contains(html, `<td class="cell" colspan="2" rowspan="2"`) &&
		!strings.Contains(html, `<td class="cell" rowspan="2" colspan="2"`) {
		t.Fatalf("expected merged cell with rowspan and colspan: %s", html)
	}
	if strings.Count(html, ">Header</td>") != 1 {
		t.Fatalf("expected merged anchor cell once: %s", html)
	}
	if strings.Contains(html, "<td class=\"cell\"></td>\n<td class=\"cell\">Header</td>") {
		t.Fatalf("unexpected placeholder cell before merged anchor: %s", html)
	}
}

func TestHTMLPreservesStyledEmptyCellsAndThemeColors(t *testing.T) {
	html := renderXlsxHTML(t, "sample-5.xlsx")
	if !strings.Contains(html, "background:#004561") {
		t.Fatalf("expected theme background color in html")
	}
	if !strings.Contains(html, "border-bottom:1px solid #EEF1F1") {
		t.Fatalf("expected resolved worksheet border styling in html")
	}
	if !strings.Contains(html, "rotate(-90deg)") {
		t.Fatalf("expected rotated text styling in html")
	}
}

func TestHTMLUsesWorkbookColumnLayoutForRotatedSampleCells(t *testing.T) {
	html := renderXlsxHTML(t, "sample-5.xlsx")
	if !strings.Contains(html, `<table class="grid grid--fixed">`) {
		t.Fatalf("expected fixed-layout grid for workbook-sized columns")
	}
	if strings.Contains(html, "width:37.63ch") {
		t.Fatalf("expected workbook column widths to be emitted in pixels, not CSS ch units")
	}
	if !strings.Contains(html, `width:263px`) {
		t.Fatalf("expected converted workbook width for the rotated year column: %s", html)
	}
	if !strings.Contains(html, `position:absolute;left:50%;top:50%;transform:translate(-50%,-50%) rotate(-90deg)`) {
		t.Fatalf("expected rotated text to be absolutely positioned inside its cell: %s", html)
	}
}

func TestHTMLRendersStackedCellTextWithVerticalWritingMode(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "STACK",
					Style: resolve.CellStyle{
						TextRotation: 255,
					},
				}},
			}},
		}},
	}

	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, "writing-mode:vertical-rl") {
		t.Fatalf("expected stacked text to use vertical writing mode: %s", html)
	}
	if strings.Contains(html, "rotate(-255deg)") {
		t.Fatalf("expected stacked text not to be rendered as a raw rotation: %s", html)
	}
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

func sampleFontBytes(t *testing.T, partName string) (string, []byte) {
	t.Helper()
	pkg, err := opc.Open(repoPath(t, "testdata", "pptx", "sample-4.pptx"))
	if err != nil {
		t.Fatalf("open sample pptx: %v", err)
	}
	part, ok := pkg.Part(partName)
	if !ok {
		t.Fatalf("missing font part %s", partName)
	}
	return part.ContentType, part.Data
}

func renderXlsxHTML(t *testing.T, filename string) string {
	t.Helper()
	path := repoPath(t, "testdata", "xlsx", filename)
	pkg, err := opc.Open(path)
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
	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("render HTML: %v", err)
	}
	return html
}

func TestHTMLRendersHyperlinkAsAnchor(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:          1,
					Col:          1,
					Ref:          "A1",
					Kind:         resolve.CellString,
					StrValue:     "Click here",
					HyperlinkURL: "https://example.com",
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<a href="https://example.com">Click here</a>`) {
		t.Fatalf("expected hyperlink anchor in output:\n%s", out)
	}
}

func TestHTMLRendersInternalHyperlinkAsAnchor(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:          1,
					Col:          1,
					Ref:          "A1",
					Kind:         resolve.CellString,
					StrValue:     "Go to Sheet2",
					HyperlinkURL: "#Sheet2!A1",
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<a href="#Sheet2!A1">Go to Sheet2</a>`) {
		t.Fatalf("expected internal hyperlink anchor in output:\n%s", out)
	}
}

func TestHTMLRendersRichTextRuns(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "Hello World",
					RichTextRuns: []*resolve.RichTextRun{
						{Text: "Hello ", Bold: true, FontColor: "FF0000"},
						{Text: "World", Italic: true},
					},
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "font-weight:bold") {
		t.Fatalf("expected bold style in rich-text run:\n%s", out)
	}
	if !strings.Contains(out, "color:#FF0000") {
		t.Fatalf("expected color in rich-text run:\n%s", out)
	}
	if !strings.Contains(out, "font-style:italic") {
		t.Fatalf("expected italic style in rich-text run:\n%s", out)
	}
	if !strings.Contains(out, "Hello ") || !strings.Contains(out, "World") {
		t.Fatalf("expected run text in output:\n%s", out)
	}
}

func TestHTMLRendersGroupedDrawings(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Drawings: []*resolve.Drawing{{
				Name: "Cluster",
				Type: resolve.DrawingTypeGroup,
				Position: resolve.DrawingPosition{
					Anchor: resolve.AnchorAbsolute,
					X:      0,
					Y:      0,
					Width:  952500,
					Height: 476250,
				},
				Group: &resolve.DrawingGroup{
					Children: []*resolve.Drawing{
						{
							Name: "Box",
							Type: resolve.DrawingTypeShape,
							Position: resolve.DrawingPosition{
								Anchor: resolve.AnchorAbsolute,
								X:      95250,
								Y:      47625,
								Width:  381000,
								Height: 190500,
							},
							Shape: &resolve.DrawingShape{
								Geometry:  "rect",
								Text:      "Inner",
								FillColor: "FFF4CC",
								LineColor: "000000",
							},
						},
						{
							Name: "Arrow",
							Type: resolve.DrawingTypeConnector,
							Position: resolve.DrawingPosition{
								Anchor: resolve.AnchorAbsolute,
								X:      476250,
								Y:      95250,
								Width:  381000,
								Height: 142875,
							},
							Shape: &resolve.DrawingShape{
								Geometry:      "line",
								ConnectorType: "straightConnector1",
								LineColor:     "0088CC",
								LineWidthPt:   2,
							},
						},
					},
				},
			}},
		}},
	}

	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="drawing drawing--group"`) {
		t.Fatalf("expected grouped drawing wrapper:\n%s", out)
	}
	if !strings.Contains(out, `left:10px;top:5px;width:40px;height:20px`) {
		t.Fatalf("expected grouped child geometry to be scaled into group pixels:\n%s", out)
	}
	if !strings.Contains(out, `class="drawing drawing--connector"`) {
		t.Fatalf("expected grouped connector to render:\n%s", out)
	}
	if !strings.Contains(out, "Inner") {
		t.Fatalf("expected grouped shape text in output:\n%s", out)
	}
}

func TestHTMLHiddenColumnsCollapsed(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name:       "Sheet1",
			HiddenCols: map[int]bool{2: true},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "Visible"},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellString, StrValue: "Hidden"},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellString, StrValue: "Also Visible"},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "visibility:collapse") {
		t.Fatalf("expected visibility:collapse for hidden column in output:\n%s", out)
	}
	// The colgroup col for column 2 should have visibility:collapse.
	if !strings.Contains(out, `style="visibility:collapse"`) {
		t.Fatalf("expected collapsed col element in colgroup:\n%s", out)
	}
}

func TestHTMLRowOutlineAndHiddenRow(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name:        "Sheet1",
			OutlineRows: map[int]int{2: 2},
			Rows: []*resolve.Row{{
				Index:        2,
				Hidden:       true,
				OutlineLevel: 2,
				Collapsed:    true,
				Cells: []*resolve.Cell{{
					Row:      2,
					Col:      1,
					Ref:      "A2",
					Kind:     resolve.CellString,
					StrValue: "Hidden grouped row",
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-outline-level="2"`) {
		t.Fatalf("expected row outline level attribute in output:\n%s", out)
	}
	if !strings.Contains(out, `display:none`) {
		t.Fatalf("expected hidden row style in output:\n%s", out)
	}
	if !strings.Contains(out, `data-outline-collapsed="true"`) {
		t.Fatalf("expected collapsed row metadata in output:\n%s", out)
	}
	if !strings.Contains(out, "outline-controls") || !strings.Contains(out, `data-axis="rows"`) {
		t.Fatalf("expected outline controls in output:\n%s", out)
	}
}

func TestHTMLCommentIndicatorAndTooltip(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name:     "Sheet1",
			Comments: map[string]string{"A1": "Needs review"},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellString,
					StrValue: "Hello",
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "cell--has-comment") {
		t.Fatalf("expected cell--has-comment class in output:\n%s", out)
	}
	if !strings.Contains(out, "comment-mark") {
		t.Fatalf("expected comment marker in output:\n%s", out)
	}
	if !strings.Contains(out, "Needs review") {
		t.Fatalf("expected comment tooltip text in output:\n%s", out)
	}
}

func TestHTMLPageSetupMetadata(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			PageSetup: &resolve.PageSetup{
				PaperSize:    9,
				Orientation:  "landscape",
				FitToPage:    true,
				FitToWidth:   1,
				Scale:        90,
				MarginLeft:   0.7,
				MarginRight:  0.7,
				MarginTop:    0.75,
				MarginBottom: 0.75,
			},
			PrintTitleRows: "$1:$2",
			PrintTitleCols: "$A:$B",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "x"}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "class=\"sheet-meta\"") {
		t.Fatalf("expected sheet meta container in output:\n%s", out)
	}
	if !strings.Contains(out, `data-page-paper-size="9"`) {
		t.Fatalf("expected paper size metadata in output:\n%s", out)
	}
	if !strings.Contains(out, `data-page-orientation="landscape"`) {
		t.Fatalf("expected orientation metadata in output:\n%s", out)
	}
	if !strings.Contains(out, `data-print-title-rows="$1:$2"`) {
		t.Fatalf("expected print title row metadata in output:\n%s", out)
	}
	if !strings.Contains(out, `data-print-title-cols="$A:$B"`) {
		t.Fatalf("expected print title col metadata in output:\n%s", out)
	}
}

func TestHTMLPrintTitleMetadataWithoutPageSetup(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name:           "Sheet1",
			PrintTitleRows: "$1:$1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "x"}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-print-title-rows="$1:$1"`) {
		t.Fatalf("expected print title metadata without page setup in output:\n%s", out)
	}
}

func TestHTMLProtectionMetadataAndNotice(t *testing.T) {
	wb := &resolve.Workbook{
		Protection: &resolve.WorkbookProtection{
			LockStructure: true,
			LockWindows:   true,
			AlgorithmName: "SHA-512",
		},
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Protection: &resolve.SheetProtection{
				Sheet:               true,
				Objects:             true,
				Sort:                true,
				AutoFilter:          true,
				SelectLockedCells:   true,
				SelectUnlockedCells: true,
				AlgorithmName:       "SHA-512",
			},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "x"}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		`data-workbook-protection="true"`,
		`data-workbook-lock-structure="true"`,
		`data-sheet-protection="true"`,
		`data-sheet-protection-sheet="true"`,
		`data-sheet-protection-sort="true"`,
		`data-sheet-protection-auto-filter="true"`,
		`Protection metadata preserved; restrictions are not enforced during HTML rendering.`,
		`Workbook flags:</strong> lockStructure, lockWindows`,
		`Worksheet flags:</strong> sheet, objects, selectLockedCells, sort, autoFilter, selectUnlockedCells`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected protection output %q in HTML:\n%s", want, out)
		}
	}
}

func TestHTMLOutlineColumnMetadata(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name:          "Sheet1",
			OutlineCols:   map[int]int{2: 1},
			CollapsedCols: map[int]bool{2: true},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "A"},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellString, StrValue: "B"},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-outline-level="1"`) {
		t.Fatalf("expected outline-level metadata on column in output:\n%s", out)
	}
	if !strings.Contains(out, `data-outline-collapsed="true"`) {
		t.Fatalf("expected collapsed metadata on column in output:\n%s", out)
	}
	if !strings.Contains(out, `data-outline-col-level="1"`) {
		t.Fatalf("expected outline control metadata on cells/cols in output:\n%s", out)
	}
}

func TestHTMLFormulaMetadata(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:      1,
					Col:      1,
					Ref:      "A1",
					Kind:     resolve.CellNumber,
					NumValue: 10,
					Formula:  "SUM(B1:B3)",
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-formula="SUM(B1:B3)"`) {
		t.Fatalf("expected formula metadata in output:\n%s", out)
	}
	if !strings.Contains(out, "cell--formula") {
		t.Fatalf("expected formula css class in output:\n%s", out)
	}
}

func TestHTMLAutoFilterHeader(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name:          "Sheet1",
			AutoFilterRef: "A1:C1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "Name"},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellString, StrValue: "Age"},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellString, StrValue: "City"},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// All three header cells should carry the af-header class.
	if !strings.Contains(out, "cell--af-header") {
		t.Fatalf("expected cell--af-header class in output:\n%s", out)
	}
	// The drop-down arrow indicator should be present.
	if !strings.Contains(out, "af-arrow") {
		t.Fatalf("expected af-arrow span in output:\n%s", out)
	}
}

func TestHTMLDataValidationTooltip(t *testing.T) {
	trueVal := true
	_ = trueVal
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			DataValidations: []*resolve.DataValidation{
				{SqRef: "B2", Type: "list", Prompt: "Choose a month", Formula1: "ListMonths", ShowInputMessage: true},
			},
			Rows: []*resolve.Row{
				{Index: 1, Cells: []*resolve.Cell{{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellString, StrValue: "Month"}}},
				{Index: 2, Cells: []*resolve.Cell{{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellString, StrValue: "January"}}},
			},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "data-validation-type") {
		t.Fatalf("expected data-validation-type attribute:\n%s", out)
	}
	if !strings.Contains(out, "Choose a month") {
		t.Fatalf("expected tooltip text 'Choose a month' in output:\n%s", out)
	}
	if !strings.Contains(out, `data-validation-formula1="ListMonths"`) {
		t.Fatalf("expected formula metadata in output:\n%s", out)
	}
}

func TestHTMLDataValidationTooltipIncludesCustomFormulaAndError(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			DataValidations: []*resolve.DataValidation{
				{
					SqRef:            "C3",
					Type:             "custom",
					Formula1:         "LEN(C3)<=5",
					ShowErrorMessage: true,
					ErrorTitle:       "Too long",
					Error:            "Use 5 characters or fewer",
				},
			},
			Rows: []*resolve.Row{
				{Index: 3, Cells: []*resolve.Cell{{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellString, StrValue: "abcdef"}}},
			},
		}},
	}

	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-validation-type="custom"`) || !strings.Contains(out, `data-validation-formula1="LEN(C3)&lt;=5"`) {
		t.Fatalf("expected custom validation metadata in output:\n%s", out)
	}
	if !strings.Contains(out, `Custom formula: LEN(C3)&lt;=5`) {
		t.Fatalf("expected custom validation tooltip in output:\n%s", out)
	}
	if !strings.Contains(out, `Error: Too long`) || !strings.Contains(out, `Use 5 characters or fewer`) {
		t.Fatalf("expected custom validation error details in output:\n%s", out)
	}
}

func TestHTMLDataValidationTooltipIncludesBetweenRule(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			DataValidations: []*resolve.DataValidation{
				{
					SqRef:            "D4",
					Type:             "whole",
					Operator:         "between",
					Formula1:         "1",
					Formula2:         "10",
					ShowInputMessage: true,
				},
			},
			Rows: []*resolve.Row{
				{Index: 4, Cells: []*resolve.Cell{{Row: 4, Col: 4, Ref: "D4", Kind: resolve.CellNumber, NumValue: 8}}},
			},
		}},
	}

	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-validation-operator="between"`) || !strings.Contains(out, `data-validation-formula2="10"`) {
		t.Fatalf("expected between-rule metadata in output:\n%s", out)
	}
	if !strings.Contains(out, `Whole number between 1 and 10`) {
		t.Fatalf("expected between-rule tooltip in output:\n%s", out)
	}
	if !strings.Contains(out, `data-validation-evaluated="true"`) || !strings.Contains(out, `data-validation-valid="true"`) {
		t.Fatalf("expected evaluated between-rule state in output:\n%s", out)
	}
	if !strings.Contains(out, `Current value: valid`) {
		t.Fatalf("expected current validity state in tooltip:\n%s", out)
	}
}

func TestHTMLDataValidationTooltipUsesResolvedListValues(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			DataValidations: []*resolve.DataValidation{{
				SqRef:      "B2",
				Type:       "list",
				Formula1:   "ListMonths",
				ListValues: []string{"January", "February", "March"},
			}},
			Rows: []*resolve.Row{{
				Index: 2,
				Cells: []*resolve.Cell{{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellString, StrValue: "January"}},
			}},
		}},
	}

	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `Allowed values: January, February, March`) {
		t.Fatalf("expected resolved list values in tooltip:\n%s", out)
	}
	if !strings.Contains(out, "data-validation-list-values=") {
		t.Fatalf("expected resolved list values metadata:\n%s", out)
	}
}

func TestHTMLDataValidationTextLengthInvalidState(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			DataValidations: []*resolve.DataValidation{{
				SqRef:    "E5",
				Type:     "textLength",
				Operator: "lessThanOrEqual",
				Formula1: "5",
			}},
			Rows: []*resolve.Row{{
				Index: 5,
				Cells: []*resolve.Cell{{Row: 5, Col: 5, Ref: "E5", Kind: resolve.CellString, StrValue: "toolong"}},
			}},
		}},
	}

	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `data-validation-valid="false"`) {
		t.Fatalf("expected invalid text-length state in output:\n%s", out)
	}
	if !strings.Contains(out, `Text length less than or equal to 5`) {
		t.Fatalf("expected text-length rule description in output:\n%s", out)
	}
	if !strings.Contains(out, `Current value: invalid`) {
		t.Fatalf("expected invalid tooltip state in output:\n%s", out)
	}
}

func TestHTMLDataValidationCustomFormulaRelativeEvaluation(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			DataValidations: []*resolve.DataValidation{{
				SqRef:      "C3:C4",
				Type:       "custom",
				Formula1:   `LEN(C3)<=5`,
				ErrorTitle: "Too long",
			}},
			Rows: []*resolve.Row{{
				Index: 3,
				Cells: []*resolve.Cell{{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellString, StrValue: "short"}},
			}, {
				Index: 4,
				Cells: []*resolve.Cell{{Row: 4, Col: 3, Ref: "C4", Kind: resolve.CellString, StrValue: "toolong"}},
			}},
		}},
	}

	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `data-validation-valid="true"`) != 1 {
		t.Fatalf("expected one valid custom-validation cell, got output:\n%s", out)
	}
	if strings.Count(out, `data-validation-valid="false"`) != 1 {
		t.Fatalf("expected one invalid custom-validation cell, got output:\n%s", out)
	}
	if !strings.Contains(out, `Custom formula: LEN(C3)&lt;=5`) {
		t.Fatalf("expected custom formula description in output:\n%s", out)
	}
}

func TestHTMLDataValidationCustomFormulaNamedCrossSheetEvaluation(t *testing.T) {
	localSheetID := 0
	wb := &resolve.Workbook{
		DefinedNames: []*resolve.DefinedName{{
			Name:         "AllowedValues",
			LocalSheetID: &localSheetID,
			Value:        `'Lookup Sheet'!$A$1:$A$3`,
		}},
		Sheets: []*resolve.Sheet{
			{
				Name: "Sheet1",
				DataValidations: []*resolve.DataValidation{{
					SqRef:      "B2:B3",
					Type:       "custom",
					Formula1:   `COUNTIF(AllowedValues,B2)>0`,
					ErrorTitle: "Invalid choice",
				}},
				Rows: []*resolve.Row{{
					Index: 2,
					Cells: []*resolve.Cell{{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellString, StrValue: "January"}},
				}, {
					Index: 3,
					Cells: []*resolve.Cell{{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellString, StrValue: "April"}},
				}},
			},
			{
				Name: "Lookup Sheet",
				Rows: []*resolve.Row{{
					Index: 1,
					Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "January"}},
				}, {
					Index: 2,
					Cells: []*resolve.Cell{{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellString, StrValue: "February"}},
				}, {
					Index: 3,
					Cells: []*resolve.Cell{{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellString, StrValue: "March"}},
				}},
			},
		},
	}

	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `data-validation-valid="true"`) == 0 {
		t.Fatalf("expected at least one valid custom-validation cell, got output:\n%s", out)
	}
	if strings.Count(out, `data-validation-valid="false"`) == 0 {
		t.Fatalf("expected at least one invalid custom-validation cell, got output:\n%s", out)
	}
	if !strings.Contains(out, `COUNTIF(AllowedValues,B2)&gt;0`) {
		t.Fatalf("expected COUNTIF-based custom formula metadata in output:\n%s", out)
	}
}

func TestHTMLDataValidationNumericFormulaCrossSheetReference(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{
			{
				Name: "Sheet1",
				DataValidations: []*resolve.DataValidation{{
					SqRef:      "C2:C3",
					Type:       "whole",
					Operator:   "lessThanOrEqual",
					Formula1:   `'Limits'!$A$1`,
					AllowBlank: false,
				}},
				Rows: []*resolve.Row{{
					Index: 2,
					Cells: []*resolve.Cell{{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 5}},
				}, {
					Index: 3,
					Cells: []*resolve.Cell{{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellNumber, NumValue: 9}},
				}},
			},
			{
				Name: "Limits",
				Rows: []*resolve.Row{{
					Index: 1,
					Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 7}},
				}},
			},
		},
	}

	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `data-validation-valid="true"`) == 0 {
		t.Fatalf("expected a valid cross-sheet numeric-validation cell, got output:\n%s", out)
	}
	if strings.Count(out, `data-validation-valid="false"`) == 0 {
		t.Fatalf("expected an invalid cross-sheet numeric-validation cell, got output:\n%s", out)
	}
}

func TestHTMLCFDataBarRendered(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			CFRules: []*resolve.CFRule{
				{SqRef: "A1:A3", Type: "dataBar", Priority: 1, DataBar: &resolve.CFDataBar{Color: "638EC6", ShowValue: true, Cfvo: []resolve.CFCfvo{{Type: "min"}, {Type: "max"}}}},
			},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 200}},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 300}},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 400}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "cf-databar") {
		t.Fatalf("expected cf-databar SVG in output:\n%s", out)
	}
	if !strings.Contains(out, "638EC6") {
		t.Fatalf("expected data bar color in output:\n%s", out)
	}
	if !strings.Contains(out, `width="50%"`) {
		t.Fatalf("expected middle cell data bar to use range-derived 50%% width, got output:\n%s", out)
	}
}

func TestHTMLCFIconSetRendered(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			CFRules: []*resolve.CFRule{
				{SqRef: "A1:A2", Type: "iconSet", Priority: 1, IconSet: &resolve.CFIconSet{Set: "3TrafficLights1"}},
			},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 90}, // green
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 10}, // red
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "cf-icon") {
		t.Fatalf("expected cf-icon span in output:\n%s", out)
	}
}

func TestHTMLCFColorScaleRendered(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			CFRules: []*resolve.CFRule{
				{SqRef: "A1", Type: "colorScale", Priority: 1, ColorScale: &resolve.CFColorScale{Colors: []string{"FF0000", "00FF00"}}},
			},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 50}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Should have a computed background color (neither pure red nor pure green at 50%).
	if !strings.Contains(out, "background:#") {
		t.Fatalf("expected color scale background in output:\n%s", out)
	}
}

func TestHTMLConditionalFormattingAppliesToComputedArrayFormulaMembers(t *testing.T) {
	bgColor := "FFCC00"
	boolTrue := true
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			ArrayFormulas: []*resolve.ArrayFormula{{
				AnchorRef: "A1",
				RangeRef:  "A1:B1",
				Formula:   "COLUMN(A1:B1)",
				TopRow:    1,
				LeftCol:   1,
				BottomRow: 1,
				RightCol:  2,
			}},
			CFRules: []*resolve.CFRule{{
				SqRef:    "A1:B1",
				Type:     "cellIs",
				Operator: "greaterThan",
				Priority: 1,
				Formula:  []string{"1"},
				DxfStyle: resolve.DxfStyle{BgColor: bgColor, Bold: &boolTrue},
			}},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:                1,
					Col:                1,
					Ref:                "A1",
					Kind:               resolve.CellFormula,
					Formula:            "COLUMN(A1:B1)",
					ArrayFormulaRange:  "A1:B1",
					ArrayFormulaAnchor: "A1",
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, "background:#"+bgColor) != 1 {
		t.Fatalf("expected exactly one CF-styled array member, got output:\n%s", out)
	}
	if !strings.Contains(out, ">2<") {
		t.Fatalf("expected computed second array member in output:\n%s", out)
	}
}

func TestHTMLCFDataBarUsesComputedArrayFormulaMembers(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			ArrayFormulas: []*resolve.ArrayFormula{{
				AnchorRef: "A1",
				RangeRef:  "A1:B1",
				Formula:   "COLUMN(A1:B1)",
				TopRow:    1,
				LeftCol:   1,
				BottomRow: 1,
				RightCol:  2,
			}},
			CFRules: []*resolve.CFRule{{
				SqRef:    "A1:B1",
				Type:     "dataBar",
				Priority: 1,
				DataBar:  &resolve.CFDataBar{Color: "638EC6", ShowValue: true, Cfvo: []resolve.CFCfvo{{Type: "min"}, {Type: "max"}}},
			}},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:                1,
					Col:                1,
					Ref:                "A1",
					Kind:               resolve.CellFormula,
					Formula:            "COLUMN(A1:B1)",
					ArrayFormulaRange:  "A1:B1",
					ArrayFormulaAnchor: "A1",
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, "cf-databar") != 2 {
		t.Fatalf("expected a data bar for both array members, got output:\n%s", out)
	}
	if !strings.Contains(out, `width="100%"`) {
		t.Fatalf("expected computed max-width array data bar, got output:\n%s", out)
	}
}

func TestHTMLConditionalFormatting(t *testing.T) {
	bgColor := "FFCC00"
	boolTrue := true
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			CFRules: []*resolve.CFRule{
				{SqRef: "A1", Type: "cellIs", Operator: "lessThan", Priority: 1, DxfStyle: resolve.DxfStyle{BgColor: bgColor, Bold: &boolTrue}},
			},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: -5}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "#"+bgColor) {
		t.Fatalf("expected CF background color #%s in output:\n%s", bgColor, out)
	}
	if !strings.Contains(out, "font-weight:bold") {
		t.Fatalf("expected font-weight:bold from CF rule:\n%s", out)
	}
}

func TestHTMLConditionalFormattingTimePeriods(t *testing.T) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	prevMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	nextMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)

	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			CFRules: []*resolve.CFRule{
				{SqRef: "A1", Type: "timePeriod", TimePeriod: "today", Priority: 1, DxfStyle: resolve.DxfStyle{BgColor: "AA1101"}},
				{SqRef: "B1", Type: "timePeriod", TimePeriod: "yesterday", Priority: 2, DxfStyle: resolve.DxfStyle{BgColor: "AA1102"}},
				{SqRef: "C1", Type: "timePeriod", TimePeriod: "tomorrow", Priority: 3, DxfStyle: resolve.DxfStyle{BgColor: "AA1103"}},
				{SqRef: "D1", Type: "timePeriod", TimePeriod: "last7Days", Priority: 4, DxfStyle: resolve.DxfStyle{BgColor: "AA1104"}},
				{SqRef: "F1", Type: "timePeriod", TimePeriod: "thisMonth", Priority: 5, DxfStyle: resolve.DxfStyle{BgColor: "AA1106"}},
				{SqRef: "G1", Type: "timePeriod", TimePeriod: "lastMonth", Priority: 6, DxfStyle: resolve.DxfStyle{BgColor: "AA1107"}},
				{SqRef: "H1", Type: "timePeriod", TimePeriod: "nextMonth", Priority: 7, DxfStyle: resolve.DxfStyle{BgColor: "AA1108"}},
				{SqRef: "E1", Type: "timePeriod", TimePeriod: "last7Days", Priority: 8, DxfStyle: resolve.DxfStyle{BgColor: "AA1105"}},
			},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: excelSerialFromTime(today)},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: excelSerialFromTime(today.AddDate(0, 0, -1))},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellNumber, NumValue: excelSerialFromTime(today.AddDate(0, 0, 1))},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellNumber, NumValue: excelSerialFromTime(today.AddDate(0, 0, -3))},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellNumber, NumValue: excelSerialFromTime(today.AddDate(0, 0, -10))},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellNumber, NumValue: excelSerialFromTime(time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC))},
					{Row: 1, Col: 7, Ref: "G1", Kind: resolve.CellNumber, NumValue: excelSerialFromTime(time.Date(prevMonth.Year(), prevMonth.Month(), 15, 0, 0, 0, 0, time.UTC))},
					{Row: 1, Col: 8, Ref: "H1", Kind: resolve.CellNumber, NumValue: excelSerialFromTime(time.Date(nextMonth.Year(), nextMonth.Month(), 15, 0, 0, 0, 0, time.UTC))},
				},
			}},
		}},
	}

	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, color := range []string{"#AA1101", "#AA1102", "#AA1103", "#AA1104", "#AA1106", "#AA1107", "#AA1108"} {
		if !strings.Contains(out, color) {
			t.Fatalf("expected time-period conditional formatting color %s in output:\n%s", color, out)
		}
	}
	if strings.Contains(out, "#AA1105") {
		t.Fatalf("did not expect unmatched last7Days color in output:\n%s", out)
	}
}

func excelSerialFromTime(t time.Time) float64 {
	excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	t = time.Date(t.UTC().Year(), t.UTC().Month(), t.UTC().Day(), 0, 0, 0, 0, time.UTC)
	return t.Sub(excelEpoch).Hours() / 24
}

func TestHTMLFrozenPaneMetadata(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name:        "Sheet1",
			FrozenRows:  2,
			FrozenCols:  1,
			TopLeftCell: "B3",
			ActivePane:  "bottomRight",
			Rows: []*resolve.Row{
				{Index: 1, Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "Header1"}}},
				{Index: 2, Cells: []*resolve.Cell{{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellString, StrValue: "Header2"}}},
				{Index: 3, Cells: []*resolve.Cell{{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellString, StrValue: "Data"}}},
			},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "class=\"pane-meta\"") {
		t.Fatalf("expected pane meta container in output:\n%s", out)
	}
	if !strings.Contains(out, `data-frozen-rows="2"`) {
		t.Fatalf("expected frozen rows metadata in output:\n%s", out)
	}
	if !strings.Contains(out, `data-frozen-cols="1"`) {
		t.Fatalf("expected frozen cols metadata in output:\n%s", out)
	}
	if !strings.Contains(out, `data-top-left-cell="B3"`) {
		t.Fatalf("expected top-left cell metadata in output:\n%s", out)
	}
	if !strings.Contains(out, `data-active-pane="bottomRight"`) {
		t.Fatalf("expected active pane metadata in output:\n%s", out)
	}
	// Check that frozen row class is applied
	if !strings.Contains(out, "row--frozen") {
		t.Fatalf("expected row--frozen class in output:\n%s", out)
	}
	// Check that frozen col class is applied
	if !strings.Contains(out, "cell--frozen-col") {
		t.Fatalf("expected cell--frozen-col class in output:\n%s", out)
	}
}

func TestHTMLSplitPaneMetadata(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name:        "Sheet1",
			SplitRows:   5,
			SplitCols:   3,
			TopLeftCell: "D6",
			ActivePane:  "topLeft",
			Rows: []*resolve.Row{
				{Index: 1, Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "Data"}}},
			},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "class=\"pane-meta\"") {
		t.Fatalf("expected pane meta container in output:\n%s", out)
	}
	if !strings.Contains(out, `data-split-rows="5"`) {
		t.Fatalf("expected split rows metadata in output:\n%s", out)
	}
	if !strings.Contains(out, `data-split-cols="3"`) {
		t.Fatalf("expected split cols metadata in output:\n%s", out)
	}
	if !strings.Contains(out, `data-top-left-cell="D6"`) {
		t.Fatalf("expected top-left cell metadata in output:\n%s", out)
	}
	if !strings.Contains(out, `data-active-pane="topLeft"`) {
		t.Fatalf("expected active pane metadata in output:\n%s", out)
	}
}

func TestHTMLFormulaEngineComputesUncachedCells(t *testing.T) {
	// A formula cell with StrValue="" has no cached value; the engine should
	// evaluate the formula and display the computed result.
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 32},
					// C1 has a SUM formula but no cached value (StrValue == "").
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellFormula, Formula: "A1+B1", StrValue: ""},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">42<") {
		t.Fatalf("expected computed formula result '42' in output:\n%s", out)
	}
}

func TestHTMLFormulaEngineRendersComputedHyperlinkCells(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellFormula, Formula: `HYPERLINK("https://example.com","Jump")`, StrValue: "",
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `<a href="https://example.com">Jump</a>`) {
		t.Fatalf("expected computed hyperlink anchor in output:\n%s", out)
	}
}

func TestHTMLFormulaEnginePreservesCachedFormulaValues(t *testing.T) {
	// Fidelity rendering must preserve a cached workbook result. The local
	// evaluator is a fallback for uncached formulas, not an authority that may
	// replace an authored Excel cache when evaluator support is incomplete.
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellFormula, Formula: "1+1", StrValue: "cached", HasCachedValue: true},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, Formula: "1+1", NumValue: 7, HasCachedValue: true},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">cached<") {
		t.Fatalf("expected cached string formula result in output:\n%s", out)
	}
	if !strings.Contains(out, ">7<") {
		t.Fatalf("expected cached numeric formula result in output:\n%s", out)
	}
}

func TestHTMLFormulaEngineRecoversCachedFormulaErrorWhenEvaluationSucceeds(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellError,
					Formula: `IFERROR(AVERAGE(1,2,3))`, StrValue: "#VALUE!", HasCachedValue: true,
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, ">#VALUE!<") {
		t.Fatalf("expected recoverable cached formula error to be replaced by evaluated value:\n%s", out)
	}
	if !strings.Contains(out, ">2<") {
		t.Fatalf("expected evaluated IFERROR/AVERAGE result in output:\n%s", out)
	}
}

func TestHTMLFormulaEngineCanReplaceCachedErrorWithEmptyFallback(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellError,
					Formula:  `IFERROR(__xludf.dummyfunction("unique(Income!A:A)"),"")`,
					StrValue: "#NAME?", HasCachedValue: true,
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, ">#NAME?<") {
		t.Fatalf("expected empty IFERROR fallback to replace cached #NAME?:\n%s", out)
	}
	if !strings.Contains(out, `data-formula="IFERROR(__xludf.dummyfunction`) {
		t.Fatalf("formula cell disappeared from output:\n%s", out)
	}
}

func TestHTMLDoesNotEmitRendererMetadataPanelsForTablesOrPivots(t *testing.T) {
	wb := &resolve.Workbook{Sheets: []*resolve.Sheet{{
		Name:        "Sheet1",
		Rows:        []*resolve.Row{{Index: 1, Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "Visible"}}}},
		Tables:      []*resolve.Table{{Name: "SalesTable", DisplayName: "SalesTable", Ref: "A1:B2"}},
		PivotTables: []*resolve.PivotTable{{Name: "SalesPivot", LocationRef: "D4:F8"}},
	}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Contains(out, `class="structured-table"`) || strings.Contains(out, `class="pivot-table"`) {
		t.Fatalf("worksheet fidelity HTML must not append renderer metadata panels:\n%s", out)
	}
	if !strings.Contains(out, ">Visible<") {
		t.Fatalf("worksheet grid content disappeared:\n%s", out)
	}
}

func TestHTMLRendersComputedArrayFormulaRangeMembers(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			ArrayFormulas: []*resolve.ArrayFormula{{
				AnchorRef: "A1",
				RangeRef:  "A1:B1",
				Formula:   "1+1",
				TopRow:    1,
				LeftCol:   1,
				BottomRow: 1,
				RightCol:  2,
			}},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:                1,
					Col:                1,
					Ref:                "A1",
					Kind:               resolve.CellFormula,
					Formula:            "1+1",
					ArrayFormulaRange:  "A1:B1",
					ArrayFormulaAnchor: "A1",
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, `data-array-formula-range="A1:B1"`) != 2 {
		t.Fatalf("expected two rendered array-formula cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-anchor="A1"`) != 2 {
		t.Fatalf("expected anchor metadata on both array cells, got output:\n%s", out)
	}
	if strings.Count(out, ">2<") != 2 {
		t.Fatalf("expected both array cells to be computed, got output:\n%s", out)
	}
	if !strings.Contains(out, `class="cell cell--formula cell--array-formula"`) {
		t.Fatalf("expected array formula class in output:\n%s", out)
	}
}

func TestHTMLComputesUncachedArrayFormulaMembers(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			ArrayFormulas: []*resolve.ArrayFormula{{
				AnchorRef: "A1",
				RangeRef:  "A1:B1",
				Formula:   "COLUMN(A1:B1)",
				TopRow:    1,
				LeftCol:   1,
				BottomRow: 1,
				RightCol:  2,
			}},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{
					Row:                1,
					Col:                1,
					Ref:                "A1",
					Kind:               resolve.CellFormula,
					Formula:            "COLUMN(A1:B1)",
					ArrayFormulaRange:  "A1:B1",
					ArrayFormulaAnchor: "A1",
				}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if strings.Count(out, ">1<") != 1 || strings.Count(out, ">2<") != 1 {
		t.Fatalf("expected computed array member outputs 1 and 2, got output:\n%s", out)
	}
}

func TestHTMLComputesArrayFormulaRangeArithmeticMembers(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			ArrayFormulas: []*resolve.ArrayFormula{{
				AnchorRef: "C1",
				RangeRef:  "C1:C2",
				Formula:   "A1:A2+B1:B2",
				TopRow:    1,
				LeftCol:   3,
				BottomRow: 2,
				RightCol:  3,
			}},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellFormula, Formula: "A1:A2+B1:B2", ArrayFormulaRange: "C1:C2", ArrayFormulaAnchor: "C1"},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 2},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">11<") || !strings.Contains(out, ">22<") {
		t.Fatalf("expected computed array arithmetic outputs 11 and 22, got output:\n%s", out)
	}
}

func TestHTMLBroadcastsSingletonArrayDimensions(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			ArrayFormulas: []*resolve.ArrayFormula{{
				AnchorRef: "C1",
				RangeRef:  "C1:D2",
				Formula:   "A1:B1+A1:A2",
				TopRow:    1,
				LeftCol:   3,
				BottomRow: 2,
				RightCol:  4,
			}},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellFormula, Formula: "A1:B1+A1:A2", ArrayFormulaRange: "C1:D2", ArrayFormulaAnchor: "C1"},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 30}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">20<", ">30<", ">40<", ">50<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected broadcasted array output %s, got output:\n%s", expected, out)
		}
	}
}

func TestHTMLComputesSumProductFormula(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellNumber, NumValue: 30},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellFormula, Formula: "SUMPRODUCT(A1:B2,C1:D2)"},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 4},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 4, Ref: "D2", Kind: resolve.CellNumber, NumValue: 40},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">300<") {
		t.Fatalf("expected computed SUMPRODUCT output 300, got output:\n%s", out)
	}
}

func TestHTMLComputesRangeBasedLogicalAndTextFunctions(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellBool, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellString, StrValue: "Hello"},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellFormula, Formula: "AND(A1:A2)"},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: "OR(A1:A2)"},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellFormula, Formula: "CONCATENATE(B1:B2)"},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellBool, NumValue: 0},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellString, StrValue: "World"},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">FALSE<", ">TRUE<", ">HelloWorld<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed output %s, got output:\n%s", expected, out)
		}
	}
}

func TestHTMLComputesSumIfFormula(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellFormula, Formula: `SUMIF(A1:A3,">1",B1:B3)`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">50<") {
		t.Fatalf("expected computed SUMIF output 50, got output:\n%s", out)
	}
}

func TestHTMLComputesCriteriaFunctionFamilies(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellString, StrValue: "x"},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `COUNTIFS(A1:A3,">1",C1:C3,"x")`},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellFormula, Formula: `SUMIFS(B1:B3,A1:A3,">1",C1:C3,"x")`},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellFormula, Formula: `AVERAGEIF(A1:A3,">1",B1:B3)`},
					{Row: 1, Col: 7, Ref: "G1", Kind: resolve.CellFormula, Formula: `AVERAGEIFS(B1:B3,A1:A3,">1",C1:C3,"x")`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellString, StrValue: "y"},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
					{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellString, StrValue: "x"},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">1<", ">30<", ">25<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, ">30<") < 2 {
		t.Fatalf("expected both SUMIFS and AVERAGEIFS outputs 30, got output:\n%s", out)
	}
}

func TestHTMLComputesInlineArrayExpressionsInAggregates(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellString, StrValue: "x"},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `SUM(A1:A3*B1:B3)`},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellFormula, Formula: `MAX(A1:A3+B1:B3)`},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellFormula, Formula: `COUNTIF(A1:A3+B1:B3,">15")`},
					{Row: 1, Col: 7, Ref: "G1", Kind: resolve.CellFormula, Formula: `SUM(IF(C1:C3="x",B1:B3,0))`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellString, StrValue: "y"},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
					{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellString, StrValue: "x"},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">140<", ">33<", ">2<", ">40<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed output %s, got output:\n%s", expected, out)
		}
	}
}

func TestHTMLComputesAggregateFunctionFamily(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellFormula, Formula: `PRODUCT(A1:A3)`},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `SUMSQ(A1:A3)`},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellFormula, Formula: `LARGE(B1:B3,2)`},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellFormula, Formula: `SMALL(B1:B3,2)`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 5},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 4},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 9},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">24<", ">29<", ">5<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, ">5<") < 2 {
		t.Fatalf("expected both LARGE and SMALL outputs 5, got output:\n%s", out)
	}
}

func TestHTMLComputesStatFunctionFamily(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellString, StrValue: ""},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `MEDIAN(A1:A5)`},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellFormula, Formula: `RANK(B2,B1:B3)`},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellFormula, Formula: `RANK.EQ(B2,B1:B3)`},
					{Row: 1, Col: 7, Ref: "G1", Kind: resolve.CellFormula, Formula: `COUNTBLANK(C1:C3)`},
					{Row: 1, Col: 8, Ref: "H1", Kind: resolve.CellFormula, Formula: `MODE(A1:A5)`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
					{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellNumber, NumValue: 1},
				},
			}, {
				Index: 4,
				Cells: []*resolve.Cell{{Row: 4, Col: 1, Ref: "A4", Kind: resolve.CellNumber, NumValue: 3}},
			}, {
				Index: 5,
				Cells: []*resolve.Cell{{Row: 5, Col: 1, Ref: "A5", Kind: resolve.CellNumber, NumValue: 4}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">2<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, ">2<") < 5 {
		t.Fatalf("expected all stat formulas to compute 2, got output:\n%s", out)
	}
}

func TestHTMLComputesRoundingFunctionFamily(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 2.3},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellFormula, Formula: `CEILING(A1,1)`},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellFormula, Formula: `FLOOR(A1,1)`},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `ROUNDUP(A1,0)`},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellFormula, Formula: `ROUNDDOWN(A1,0)`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: -2.3},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellFormula, Formula: `ROUNDUP(A2,0)`},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellFormula, Formula: `ROUNDDOWN(A2,0)`},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">3<", ">2<", ">-3<", ">-2<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, ">3<") < 2 {
		t.Fatalf("expected both CEILING and ROUNDUP outputs 3, got output:\n%s", out)
	}
	if strings.Count(out, ">2<") < 2 {
		t.Fatalf("expected both FLOOR and ROUNDDOWN outputs 2, got output:\n%s", out)
	}
}

func TestHTMLRendersImplicitSpillRanges(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `A1:A3+B1:B3`},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellFormula, Formula: `IF(A1:A3>1,B1:B3,0)`},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellFormula, Formula: `A1:A3`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">11<", ">22<", ">33<", ">0<", ">20<", ">30<", ">1<", ">2<", ">3<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed spill output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="D1:D3"`) != 3 {
		t.Fatalf("expected three rendered D spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="E1:E3"`) != 3 {
		t.Fatalf("expected three rendered E spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="F1:F3"`) != 3 {
		t.Fatalf("expected three rendered F spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersSheetQualifiedImplicitSpillRanges(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `Sheet2!A1:A3`}},
			}},
		}, {
			Name: "Sheet2",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 10}},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 20}},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 30}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">10<", ">20<", ">30<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed cross-sheet spill output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="D1:D3"`) != 3 {
		t.Fatalf("expected three rendered D spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersTransposeImplicitSpillRanges(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `TRANSPOSE(A1:B3)`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
				},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">1<", ">2<", ">3<", ">10<", ">20<", ">30<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed transpose spill output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="D1:F2"`) != 6 {
		t.Fatalf("expected six rendered transpose spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersIndexSelectorsAnd2DBroadcastSpills(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellNumber, NumValue: 100},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellNumber, NumValue: 1000},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellFormula, Formula: `INDEX(A1:C3,2,3)`},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellFormula, Formula: `INDEX(A1:C3,0,2)`},
					{Row: 1, Col: 8, Ref: "H1", Kind: resolve.CellFormula, Formula: `INDEX(A1:C3,2,0)+D1:D3`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 200},
					{Row: 2, Col: 4, Ref: "D2", Kind: resolve.CellNumber, NumValue: 2000},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
					{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellNumber, NumValue: 300},
					{Row: 3, Col: 4, Ref: "D3", Kind: resolve.CellNumber, NumValue: 3000},
				},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">200<", ">10<", ">20<", ">30<", ">1002<", ">1020<", ">1200<", ">2002<", ">2020<", ">2200<", ">3002<", ">3020<", ">3200<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed INDEX/broadcast output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="F1:F3"`) != 3 {
		t.Fatalf("expected three rendered INDEX column selector spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="H1:J3"`) != 9 {
		t.Fatalf("expected nine rendered INDEX broadcast spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersOffsetSpillsAndAggregateComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellNumber, NumValue: 100},
					{Row: 1, Col: 11, Ref: "K1", Kind: resolve.CellFormula, Formula: `SUM(INDEX(A1:C3,2,0))`},
					{Row: 1, Col: 12, Ref: "L1", Kind: resolve.CellFormula, Formula: `OFFSET(A1,1,0,2,2)`},
					{Row: 1, Col: 14, Ref: "N1", Kind: resolve.CellFormula, Formula: `SUM(OFFSET(A1,1,0,2,2))`},
					{Row: 1, Col: 15, Ref: "O1", Kind: resolve.CellFormula, Formula: `SUM(TRANSPOSE(A1:B3))`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 200},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
					{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellNumber, NumValue: 300},
				},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">222<", ">2<", ">20<", ">3<", ">30<", ">55<", ">66<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed OFFSET/aggregate output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="L1:M2"`) != 4 {
		t.Fatalf("expected four rendered OFFSET spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersChooseSpillsAndRangeAwareComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellString, StrValue: "Hello"},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellString, StrValue: "One"},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellBool, NumValue: 1},
					{Row: 1, Col: 7, Ref: "G1", Kind: resolve.CellFormula, Formula: `CHOOSE(2,A1:A3,B1:B3)`},
					{Row: 1, Col: 8, Ref: "H1", Kind: resolve.CellFormula, Formula: `COUNTIF(CHOOSE(1,A1:A3,B1:B3),">1")`},
					{Row: 1, Col: 9, Ref: "I1", Kind: resolve.CellFormula, Formula: `CONCATENATE(CHOOSE(2,C1:C2,D1:D2))`},
					{Row: 1, Col: 10, Ref: "J1", Kind: resolve.CellFormula, Formula: `AND(CHOOSE(1,E1:E2,F1:F2))`},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellBool, NumValue: 0},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellString, StrValue: "World"},
					{Row: 2, Col: 4, Ref: "D2", Kind: resolve.CellString, StrValue: "Two"},
					{Row: 2, Col: 5, Ref: "E2", Kind: resolve.CellBool, NumValue: 1},
					{Row: 2, Col: 6, Ref: "F2", Kind: resolve.CellBool, NumValue: 0},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
				},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">10<", ">20<", ">30<", ">2<", ">OneTwo<", ">TRUE<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed CHOOSE output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="G1:G3"`) != 3 {
		t.Fatalf("expected three rendered CHOOSE spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersChooseAxisSelectorsAndScalarComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellNumber, NumValue: 100},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellString, StrValue: "one"},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellString, StrValue: "two"},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellFormula, Formula: `CHOOSECOLS(A1:C3,3,1)`},
					{Row: 1, Col: 9, Ref: "I1", Kind: resolve.CellFormula, Formula: `CHOOSEROWS(D1:E2,2)`},
					{Row: 1, Col: 11, Ref: "K1", Kind: resolve.CellFormula, Formula: `LEN(CHOOSECOLS(D1:E2,2))`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 200},
					{Row: 2, Col: 4, Ref: "D2", Kind: resolve.CellString, StrValue: "three"},
					{Row: 2, Col: 5, Ref: "E2", Kind: resolve.CellString, StrValue: "four"},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
					{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellNumber, NumValue: 300},
					{Row: 3, Col: 9, Ref: "I3", Kind: resolve.CellFormula, Formula: `UPPER(CHOOSEROWS(D1:E2,2))`},
				},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">100<", ">1<", ">200<", ">2<", ">300<", ">3<", ">three<", ">four<", ">THREE<", ">FOUR<", ">3<", ">4<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed CHOOSECOLS/CHOOSEROWS output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="F1:G3"`) != 6 {
		t.Fatalf("expected six rendered CHOOSECOLS spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="I1:J1"`) != 2 {
		t.Fatalf("expected two rendered CHOOSEROWS spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="I3:J3"`) != 2 {
		t.Fatalf("expected two rendered scalar-composed CHOOSEROWS spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="K1:K2"`) != 2 {
		t.Fatalf("expected two rendered scalar-composed CHOOSECOLS spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersTakeDropSelectorsAndScalarComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1.2},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10.4},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellNumber, NumValue: 100.6},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `TAKE(A1:C3,2,2)`},
					{Row: 1, Col: 7, Ref: "G1", Kind: resolve.CellFormula, Formula: `DROP(A1:C3,1,1)`},
					{Row: 1, Col: 10, Ref: "J1", Kind: resolve.CellFormula, Formula: `ROUND(TAKE(A1:C3,2,2),0)`},
					{Row: 1, Col: 12, Ref: "L1", Kind: resolve.CellFormula, Formula: `MOD(DROP(A1:C3,1,1),10)`},
					{Row: 1, Col: 14, Ref: "N1", Kind: resolve.CellFormula, Formula: `TEXT(TAKE(A1:C3,2,1),"0.0")`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2.3},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20.5},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 200.7},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 3.4},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30.6},
					{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellNumber, NumValue: 300.8},
				},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">1.2<", ">10.4<", ">2.3<", ">20.5<", ">200.7<", ">30.6<", ">100.6<", ">1<", ">10<", ">2<", ">21<", ">1.2<", ">2.3<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed TAKE/DROP output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="D1:E2"`) != 4 {
		t.Fatalf("expected four rendered TAKE spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="G1:H2"`) != 4 {
		t.Fatalf("expected four rendered DROP spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="J1:K2"`) != 4 {
		t.Fatalf("expected four rendered scalar-composed TAKE spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="L1:M2"`) != 4 {
		t.Fatalf("expected four rendered scalar-composed DROP spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="N1:N2"`) != 2 {
		t.Fatalf("expected two rendered scalar-composed TAKE text spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersToRowToColAndComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellNumber, NumValue: 100},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `TOROW(A1:C2)`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 200},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 4, Ref: "D3", Kind: resolve.CellFormula, Formula: `TOCOL(A1:C2,0,1)`},
					{Row: 3, Col: 6, Ref: "F3", Kind: resolve.CellFormula, Formula: `SUM(TOROW(A1:C2))`},
				},
			}, {
				Index: 4,
				Cells: []*resolve.Cell{
					{Row: 4, Col: 1, Ref: "A4", Kind: resolve.CellString, StrValue: "one"},
					{Row: 4, Col: 2, Ref: "B4", Kind: resolve.CellString, StrValue: "two"},
					{Row: 4, Col: 7, Ref: "G4", Kind: resolve.CellFormula, Formula: `UPPER(TOROW(A4:B5))`},
				},
			}, {
				Index: 5,
				Cells: []*resolve.Cell{
					{Row: 5, Col: 1, Ref: "A5", Kind: resolve.CellString, StrValue: "three"},
					{Row: 5, Col: 2, Ref: "B5", Kind: resolve.CellString, StrValue: "four"},
				},
			}, {
				Index: 6,
				Cells: []*resolve.Cell{{Row: 6, Col: 7, Ref: "G6", Kind: resolve.CellFormula, Formula: `LEN(TOCOL(A4:B5,0,1))`}},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">333<", ">ONE<", ">TWO<", ">THREE<", ">FOUR<", ">5<", ">4<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed TOROW/TOCOL output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="D1:I1"`) != 6 {
		t.Fatalf("expected six rendered TOROW spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="D3:D8"`) != 6 {
		t.Fatalf("expected six rendered TOCOL spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="G4:J4"`) != 4 {
		t.Fatalf("expected four rendered scalar-composed TOROW spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="G6:G9"`) != 4 {
		t.Fatalf("expected four rendered scalar-composed TOCOL spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersToVectorIgnoreModesAndWrapFunctions(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `TOROW(A1:C2,3)`},
					{Row: 1, Col: 10, Ref: "J1", Kind: resolve.CellFormula, Formula: `WRAPCOLS(TOCOL(A1:C2,3,1),3,0)`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellError, StrValue: "#DIV/0!"},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 20},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 4, Ref: "D3", Kind: resolve.CellFormula, Formula: `TOCOL(A1:C2,3,1)`},
					{Row: 3, Col: 6, Ref: "F3", Kind: resolve.CellFormula, Formula: `SUM(WRAPROWS(TOROW(A1:C2,3),3,0))`},
				},
			}, {
				Index: 5,
				Cells: []*resolve.Cell{{Row: 5, Col: 6, Ref: "F5", Kind: resolve.CellFormula, Formula: `WRAPROWS(TOROW(A1:C2,3),3,"pad")`}},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">33<", ">pad<", ">20<", ">10<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed wrap output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="D1:G1"`) != 4 {
		t.Fatalf("expected four rendered filtered TOROW spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="D3:D6"`) != 4 {
		t.Fatalf("expected four rendered filtered TOCOL spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="F5:H6"`) != 6 {
		t.Fatalf("expected six rendered WRAPROWS spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="J1:K3"`) != 6 {
		t.Fatalf("expected six rendered WRAPCOLS spill cells, got output:\n%s", out)
	}
	if !strings.Contains(out, ">0<") {
		t.Fatalf("expected wrap pad value in output:\n%s", out)
	}
}

func TestHTMLRendersFilterRowsColsAndIfEmpty(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellNumber, NumValue: 100},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellBool, NumValue: 0},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellFormula, Formula: `FILTER(A1:C3,D1:D3)`},
					{Row: 1, Col: 10, Ref: "J1", Kind: resolve.CellFormula, Formula: `FILTER(A1:C3,A5:C5)`},
					{Row: 1, Col: 13, Ref: "M1", Kind: resolve.CellFormula, Formula: `SUM(FILTER(A1:C3,D1:D3))`},
					{Row: 1, Col: 14, Ref: "N1", Kind: resolve.CellString, StrValue: "one"},
					{Row: 1, Col: 15, Ref: "O1", Kind: resolve.CellFormula, Formula: `UPPER(FILTER(N1:N3,D1:D3,"none"))`},
					{Row: 1, Col: 17, Ref: "Q1", Kind: resolve.CellFormula, Formula: `FILTER(A1:C3,D1:D3*0,"none")`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 200},
					{Row: 2, Col: 4, Ref: "D2", Kind: resolve.CellBool, NumValue: 1},
					{Row: 2, Col: 14, Ref: "N2", Kind: resolve.CellString, StrValue: "two"},
					{Row: 2, Col: 17, Ref: "Q2", Kind: resolve.CellFormula, Formula: `FILTER(A1:C3,D1:D3*0)`},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
					{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellNumber, NumValue: 300},
					{Row: 3, Col: 4, Ref: "D3", Kind: resolve.CellBool, NumValue: 1},
					{Row: 3, Col: 14, Ref: "N3", Kind: resolve.CellString, StrValue: "three"},
				},
			}, {
				Index: 5,
				Cells: []*resolve.Cell{
					{Row: 5, Col: 1, Ref: "A5", Kind: resolve.CellBool, NumValue: 1},
					{Row: 5, Col: 2, Ref: "B5", Kind: resolve.CellBool, NumValue: 0},
					{Row: 5, Col: 3, Ref: "C5", Kind: resolve.CellBool, NumValue: 1},
				},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">555<", ">TWO<", ">THREE<", ">none<", ">300<", ">200<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed FILTER output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="F1:H2"`) != 6 {
		t.Fatalf("expected six rendered row FILTER spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="J1:K3"`) != 6 {
		t.Fatalf("expected six rendered column FILTER spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="O1:O2"`) != 2 {
		t.Fatalf("expected two rendered scalar-composed FILTER spill cells, got output:\n%s", out)
	}
	if !strings.Contains(out, "#N/A") {
		t.Fatalf("expected zero-match FILTER fallback error in output:\n%s", out)
	}
}

func TestHTMLRendersUniqueRowsColsAndExactlyOnce(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellString, StrValue: "alpha"},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 1, Col: 8, Ref: "H1", Kind: resolve.CellString, StrValue: "one"},
					{Row: 1, Col: 10, Ref: "J1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 12, Ref: "L1", Kind: resolve.CellFormula, Formula: `UNIQUE(A1:B5)`},
					{Row: 1, Col: 15, Ref: "O1", Kind: resolve.CellFormula, Formula: `UNIQUE(D1:F2,1)`},
					{Row: 1, Col: 18, Ref: "R1", Kind: resolve.CellFormula, Formula: `UPPER(UNIQUE(H1:H6,0,1))`},
					{Row: 1, Col: 19, Ref: "S1", Kind: resolve.CellFormula, Formula: `SUM(UNIQUE(J1:J5,0,1))`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellString, StrValue: "alpha"},
					{Row: 2, Col: 4, Ref: "D2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 5, Ref: "E2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 6, Ref: "F2", Kind: resolve.CellNumber, NumValue: 4},
					{Row: 2, Col: 8, Ref: "H2", Kind: resolve.CellString, StrValue: "one"},
					{Row: 2, Col: 10, Ref: "J2", Kind: resolve.CellNumber, NumValue: 1},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellString, StrValue: "beta"},
					{Row: 3, Col: 8, Ref: "H3", Kind: resolve.CellString, StrValue: "two"},
					{Row: 3, Col: 10, Ref: "J3", Kind: resolve.CellNumber, NumValue: 2},
				},
			}, {
				Index: 4,
				Cells: []*resolve.Cell{
					{Row: 4, Col: 1, Ref: "A4", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 4, Col: 2, Ref: "B4", Kind: resolve.CellString, StrValue: "beta"},
					{Row: 4, Col: 8, Ref: "H4", Kind: resolve.CellString, StrValue: "three"},
					{Row: 4, Col: 10, Ref: "J4", Kind: resolve.CellNumber, NumValue: 3},
				},
			}, {
				Index: 5,
				Cells: []*resolve.Cell{
					{Row: 5, Col: 1, Ref: "A5", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 5, Col: 2, Ref: "B5", Kind: resolve.CellString, StrValue: "beta"},
					{Row: 5, Col: 8, Ref: "H5", Kind: resolve.CellString, StrValue: "three"},
					{Row: 5, Col: 10, Ref: "J5", Kind: resolve.CellNumber, NumValue: 3},
				},
			}, {
				Index: 6,
				Cells: []*resolve.Cell{{Row: 6, Col: 8, Ref: "H6", Kind: resolve.CellString, StrValue: "four"}},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">alpha<", ">beta<", ">TWO<", ">FOUR<", ">3<", ">4<", ">2<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed UNIQUE output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="L1:M3"`) != 6 {
		t.Fatalf("expected six rendered row UNIQUE spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="O1:P2"`) != 4 {
		t.Fatalf("expected four rendered column UNIQUE spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="R1:R2"`) != 2 {
		t.Fatalf("expected two rendered scalar-composed UNIQUE spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersSortRowsColsVectorsAndComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellString, StrValue: "c"},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `SORT(A1:B4)`},
					{Row: 1, Col: 7, Ref: "G1", Kind: resolve.CellFormula, Formula: `SORT(A1:B4,2,-1)`},
					{Row: 1, Col: 12, Ref: "L1", Kind: resolve.CellNumber, NumValue: 4},
					{Row: 1, Col: 13, Ref: "M1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 14, Ref: "N1", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 1, Col: 15, Ref: "O1", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 1, Col: 16, Ref: "P1", Kind: resolve.CellFormula, Formula: `SORT(L1:O1)`},
					{Row: 1, Col: 20, Ref: "T1", Kind: resolve.CellString, StrValue: "one"},
					{Row: 1, Col: 21, Ref: "U1", Kind: resolve.CellFormula, Formula: `UPPER(SORT(T1:T4,1,-1))`},
					{Row: 1, Col: 22, Ref: "V1", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 1, Col: 23, Ref: "W1", Kind: resolve.CellFormula, Formula: `SUM(SORT(V1:V4))`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellString, StrValue: "a"},
					{Row: 2, Col: 20, Ref: "T2", Kind: resolve.CellString, StrValue: "three"},
					{Row: 2, Col: 22, Ref: "V2", Kind: resolve.CellNumber, NumValue: 1},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellString, StrValue: "b"},
					{Row: 3, Col: 12, Ref: "L3", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 3, Col: 13, Ref: "M3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 14, Ref: "N3", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 3, Col: 16, Ref: "P3", Kind: resolve.CellFormula, Formula: `SORT(L3:N4,2,-1,1)`},
					{Row: 3, Col: 20, Ref: "T3", Kind: resolve.CellString, StrValue: "two"},
					{Row: 3, Col: 22, Ref: "V3", Kind: resolve.CellNumber, NumValue: 4},
				},
			}, {
				Index: 4,
				Cells: []*resolve.Cell{
					{Row: 4, Col: 1, Ref: "A4", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 4, Col: 2, Ref: "B4", Kind: resolve.CellString, StrValue: "d"},
					{Row: 4, Col: 12, Ref: "L4", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 4, Col: 13, Ref: "M4", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 4, Col: 14, Ref: "N4", Kind: resolve.CellNumber, NumValue: 4},
					{Row: 4, Col: 20, Ref: "T4", Kind: resolve.CellString, StrValue: "four"},
					{Row: 4, Col: 22, Ref: "V4", Kind: resolve.CellNumber, NumValue: 2},
				},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">10<", ">TWO<", ">THREE<", ">FOUR<", ">a<", ">d<", ">1<", ">2<", ">3<", ">4<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed SORT output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="D1:E4"`) != 8 {
		t.Fatalf("expected eight rendered row SORT spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="G1:H4"`) != 8 {
		t.Fatalf("expected eight rendered keyed SORT spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="P1:S1"`) != 4 {
		t.Fatalf("expected four rendered vector SORT spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="P3:R4"`) != 6 {
		t.Fatalf("expected six rendered column SORT spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="U1:U4"`) != 4 {
		t.Fatalf("expected four rendered scalar-composed SORT spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersSortByRowsColsAndComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellString, StrValue: "c"},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellFormula, Formula: `SORTBY(A1:C4,C1:C4,1,B1:B4,1)`},
					{Row: 1, Col: 10, Ref: "J1", Kind: resolve.CellNumber, NumValue: 4},
					{Row: 1, Col: 11, Ref: "K1", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 1, Col: 12, Ref: "L1", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 1, Col: 13, Ref: "M1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 15, Ref: "O1", Kind: resolve.CellFormula, Formula: `SORTBY(J1:M2,J1:M1)`},
					{Row: 1, Col: 20, Ref: "T1", Kind: resolve.CellString, StrValue: "alpha"},
					{Row: 1, Col: 21, Ref: "U1", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 1, Col: 23, Ref: "W1", Kind: resolve.CellFormula, Formula: `UPPER(SORTBY(T1:T4,U1:U4,-1))`},
					{Row: 1, Col: 25, Ref: "Y1", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 1, Col: 26, Ref: "Z1", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 1, Col: 28, Ref: "AB1", Kind: resolve.CellFormula, Formula: `SUM(SORTBY(Y1:Y4,Z1:Z4,-1))`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellString, StrValue: "a"},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 2, Col: 10, Ref: "J2", Kind: resolve.CellNumber, NumValue: 40},
					{Row: 2, Col: 11, Ref: "K2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 12, Ref: "L2", Kind: resolve.CellNumber, NumValue: 30},
					{Row: 2, Col: 13, Ref: "M2", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 2, Col: 20, Ref: "T2", Kind: resolve.CellString, StrValue: "gamma"},
					{Row: 2, Col: 21, Ref: "U2", Kind: resolve.CellNumber, NumValue: 4},
					{Row: 2, Col: 25, Ref: "Y2", Kind: resolve.CellNumber, NumValue: 4},
					{Row: 2, Col: 26, Ref: "Z2", Kind: resolve.CellNumber, NumValue: 1},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 30},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellString, StrValue: "b"},
					{Row: 3, Col: 3, Ref: "C3", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 3, Col: 20, Ref: "T3", Kind: resolve.CellString, StrValue: "beta"},
					{Row: 3, Col: 21, Ref: "U3", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 3, Col: 25, Ref: "Y3", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 3, Col: 26, Ref: "Z3", Kind: resolve.CellNumber, NumValue: 4},
				},
			}, {
				Index: 4,
				Cells: []*resolve.Cell{
					{Row: 4, Col: 1, Ref: "A4", Kind: resolve.CellNumber, NumValue: 40},
					{Row: 4, Col: 2, Ref: "B4", Kind: resolve.CellString, StrValue: "d"},
					{Row: 4, Col: 3, Ref: "C4", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 4, Col: 20, Ref: "T4", Kind: resolve.CellString, StrValue: "delta"},
					{Row: 4, Col: 21, Ref: "U4", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 4, Col: 25, Ref: "Y4", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 4, Col: 26, Ref: "Z4", Kind: resolve.CellNumber, NumValue: 2},
				},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">10<", ">20<", ">30<", ">40<", ">a<", ">d<", ">GAMMA<", ">DELTA<", ">ALPHA<", ">BETA<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed SORTBY output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="E1:G4"`) != 12 {
		t.Fatalf("expected twelve rendered row SORTBY spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="O1:R2"`) != 8 {
		t.Fatalf("expected eight rendered column SORTBY spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="W1:W4"`) != 4 {
		t.Fatalf("expected four rendered scalar-composed SORTBY spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersSequenceAndComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `SEQUENCE(A1,B1)`},
					{Row: 1, Col: 8, Ref: "H1", Kind: resolve.CellFormula, Formula: `SEQUENCE(1,4,10,-2)`},
					{Row: 1, Col: 13, Ref: "M1", Kind: resolve.CellFormula, Formula: `SUM(SEQUENCE(3,1,1,2))`},
					{Row: 1, Col: 15, Ref: "O1", Kind: resolve.CellFormula, Formula: `TEXT(SEQUENCE(A2,1,1,0.5),"0.0")`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 3}},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">1<", ">2<", ">3<", ">4<", ">5<", ">6<", ">10<", ">8<", ">6<", ">4<", ">9<", ">1.5<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed SEQUENCE output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="D1:F2"`) != 6 {
		t.Fatalf("expected six rendered SEQUENCE spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="H1:K1"`) != 4 {
		t.Fatalf("expected four rendered row SEQUENCE spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="O1:O3"`) != 3 {
		t.Fatalf("expected three rendered scalar-composed SEQUENCE spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersRandArrayAndComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `RANDARRAY(A1,B1)`},
					{Row: 1, Col: 8, Ref: "H1", Kind: resolve.CellFormula, Formula: `RANDARRAY(1,4,10,12,1)`},
					{Row: 1, Col: 13, Ref: "M1", Kind: resolve.CellFormula, Formula: `COUNT(RANDARRAY(3,1,1,2,1))`},
					{Row: 1, Col: 15, Ref: "O1", Kind: resolve.CellFormula, Formula: `ROUND(RANDARRAY(A1,1,1,9),0)`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, ">3<") {
		t.Fatalf("expected COUNT(RANDARRAY(...)) output 3, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="D1:F2"`) != 6 {
		t.Fatalf("expected six rendered RANDARRAY spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="H1:K1"`) != 4 {
		t.Fatalf("expected four rendered whole-number RANDARRAY spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="O1:O2"`) != 2 {
		t.Fatalf("expected two rendered scalar-composed RANDARRAY spill cells, got output:\n%s", out)
	}
	if strings.Contains(out, "#VALUE!") || strings.Contains(out, "#SPILL!") {
		t.Fatalf("expected RANDARRAY output without errors, got output:\n%s", out)
	}
}

func TestHTMLRendersLambdaDynamicArraysAndComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `MAKEARRAY(2,3,LAMBDA(r,c,r*10+c))`},
					{Row: 1, Col: 8, Ref: "H1", Kind: resolve.CellFormula, Formula: `BYROW(A1:C2,LAMBDA(r,SUM(r)))`},
					{Row: 1, Col: 10, Ref: "J1", Kind: resolve.CellFormula, Formula: `BYCOL(A1:C2,LAMBDA(c,MAX(c)))`},
					{Row: 1, Col: 14, Ref: "N1", Kind: resolve.CellFormula, Formula: `MAP(R1:R3,S1:S3,LAMBDA(x,y,x+y))`},
					{Row: 1, Col: 16, Ref: "P1", Kind: resolve.CellFormula, Formula: `SCAN(0,U1:U3,LAMBDA(a,v,a+v))`},
					{Row: 1, Col: 18, Ref: "R1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 19, Ref: "S1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 21, Ref: "U1", Kind: resolve.CellNumber, NumValue: 1},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 4},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 5},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 6},
					{Row: 2, Col: 18, Ref: "R2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 19, Ref: "S2", Kind: resolve.CellNumber, NumValue: 2},
					{Row: 2, Col: 21, Ref: "U2", Kind: resolve.CellNumber, NumValue: 2},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 18, Ref: "R3", Kind: resolve.CellNumber, NumValue: 30},
					{Row: 3, Col: 19, Ref: "S3", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 3, Col: 21, Ref: "U3", Kind: resolve.CellNumber, NumValue: 3},
				},
			}}},
		}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">11<", ">23<", ">6<", ">15<", ">4<", ">5<", ">6<", ">11<", ">22<", ">33<", ">1<", ">3<", ">6<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed lambda spill output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="D1:F2"`) != 6 {
		t.Fatalf("expected six rendered MAKEARRAY spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="H1:H2"`) != 2 {
		t.Fatalf("expected two rendered BYROW spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="J1:L1"`) != 3 {
		t.Fatalf("expected three rendered BYCOL spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="N1:N3"`) != 3 {
		t.Fatalf("expected three rendered MAP spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="P1:P3"`) != 3 {
		t.Fatalf("expected three rendered SCAN spill cells, got output:\n%s", out)
	}
}

func TestHTMLRendersXMatchModesAndComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "alpha"},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 1},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `XMATCH("beta",A1:A3)*10`},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellFormula, Formula: `XMATCH(4,B1:B3,-1)*10`},
					{Row: 1, Col: 6, Ref: "F1", Kind: resolve.CellFormula, Formula: `ABS(XMATCH(3,B1:B3)-1)*10`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellString, StrValue: "beta"},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 2, Col: 4, Ref: "D2", Kind: resolve.CellFormula, Formula: `XMATCH("beta",A1:A3,0,-1)*10`},
					{Row: 2, Col: 5, Ref: "E2", Kind: resolve.CellFormula, Formula: `XMATCH(4,B1:B3,1)*10`},
					{Row: 2, Col: 6, Ref: "F2", Kind: resolve.CellFormula, Formula: `XMATCH("delta",A1:A3)`},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellString, StrValue: "gamma"},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 5},
				},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">20<", ">30<", ">10<", "#N/A"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed XMATCH output %s, got output:\n%s", expected, out)
		}
	}
}

func TestHTMLRendersXLookupModesAndComposition(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "alpha"},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellString, StrValue: "x"},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellString, StrValue: "y"},
					{Row: 1, Col: 5, Ref: "E1", Kind: resolve.CellString, StrValue: "z"},
					{Row: 1, Col: 7, Ref: "G1", Kind: resolve.CellFormula, Formula: `XLOOKUP("beta",A1:A3,B1:B3)*10`},
					{Row: 1, Col: 8, Ref: "H1", Kind: resolve.CellFormula, Formula: `XLOOKUP(4,C2:E2,C1:E1,"",-1)`},
					{Row: 1, Col: 9, Ref: "I1", Kind: resolve.CellFormula, Formula: `LEN(XLOOKUP("gamma",A1:A3,A1:A3))*10`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellString, StrValue: "beta"},
					{Row: 2, Col: 2, Ref: "B2", Kind: resolve.CellNumber, NumValue: 20},
					{Row: 2, Col: 3, Ref: "C2", Kind: resolve.CellNumber, NumValue: 3},
					{Row: 2, Col: 4, Ref: "D2", Kind: resolve.CellNumber, NumValue: 5},
					{Row: 2, Col: 5, Ref: "E2", Kind: resolve.CellNumber, NumValue: 7},
					{Row: 2, Col: 7, Ref: "G2", Kind: resolve.CellFormula, Formula: `XLOOKUP("delta",A1:A3,B1:B3,"missing")`},
					{Row: 2, Col: 8, Ref: "H2", Kind: resolve.CellFormula, Formula: `XLOOKUP(4,C2:E2,C1:E1,"",1)`},
					{Row: 2, Col: 9, Ref: "I2", Kind: resolve.CellFormula, Formula: `XLOOKUP("beta",A1:A3,B1:B3,0,0,-1)`},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{
					{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellString, StrValue: "gamma"},
					{Row: 3, Col: 2, Ref: "B3", Kind: resolve.CellNumber, NumValue: 30},
				},
			}},
		}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">200<", ">50<", ">20<", ">x<", ">y<", ">missing<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed XLOOKUP output %s, got output:\n%s", expected, out)
		}
	}
}

func TestHTMLRendersScalarFunctionSpillsAndSpillErrors(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: -1.2},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellFormula, Formula: `ABS(A1:A3)`},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellFormula, Formula: `ROUND(A1:A3,0)`},
					{Row: 1, Col: 4, Ref: "D1", Kind: resolve.CellFormula, Formula: `A1:A3`},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{
					{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 2.4},
					{Row: 2, Col: 4, Ref: "D2", Kind: resolve.CellNumber, NumValue: 999},
				},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: -3.6}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">1.2<", ">2.4<", ">3.6<", ">-1<", ">2<", ">-4<", ">#SPILL!<", ">999<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected output %s, got output:\n%s", expected, out)
		}
	}
	if strings.Count(out, `data-array-formula-range="B1:B3"`) != 3 {
		t.Fatalf("expected three rendered B spill cells, got output:\n%s", out)
	}
	if strings.Count(out, `data-array-formula-range="C1:C3"`) != 3 {
		t.Fatalf("expected three rendered C spill cells, got output:\n%s", out)
	}
}

func TestHTMLComputesCrossSheetFormulaReferences(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellFormula, Formula: `Sheet2!A1`},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellFormula, Formula: `SUM(Sheet2!A1:Sheet2!A3)`},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellFormula, Formula: `'Sheet 2'!A1`},
				},
			}},
		}, {
			Name: "Sheet2",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 10}},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 20}},
			}, {
				Index: 3,
				Cells: []*resolve.Cell{{Row: 3, Col: 1, Ref: "A3", Kind: resolve.CellNumber, NumValue: 30}},
			}},
		}, {
			Name: "Sheet 2",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 42}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, expected := range []string{">10<", ">60<", ">42<"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected computed cross-sheet output %s, got output:\n%s", expected, out)
		}
	}
}

func TestHTMLRendersArrayFormulaRangeMismatchError(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			ArrayFormulas: []*resolve.ArrayFormula{{
				AnchorRef: "C1",
				RangeRef:  "C1:C3",
				Formula:   "A1:A2",
				TopRow:    1,
				LeftCol:   3,
				BottomRow: 3,
				RightCol:  3,
			}},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 10},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellFormula, Formula: "A1:A2", ArrayFormulaRange: "C1:C3", ArrayFormulaAnchor: "C1"},
				},
			}, {
				Index: 2,
				Cells: []*resolve.Cell{{Row: 2, Col: 1, Ref: "A2", Kind: resolve.CellNumber, NumValue: 20}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "#N/A") {
		t.Fatalf("expected computed #N/A array mismatch in output:\n%s", out)
	}
}

func TestHTMLCellErrorValue(t *testing.T) {
	// Test that formula errors are properly displayed
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Sheet1",
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellError, StrValue: "#DIV/0!"},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellError, StrValue: "#VALUE!"},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellError, StrValue: "#REF!"},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Error values should be displayed as text
	if !strings.Contains(out, "#DIV/0!") {
		t.Fatalf("expected #DIV/0! error in output:\n%s", out)
	}
	if !strings.Contains(out, "#VALUE!") {
		t.Fatalf("expected #VALUE! error in output:\n%s", out)
	}
	if !strings.Contains(out, "#REF!") {
		t.Fatalf("expected #REF! error in output:\n%s", out)
	}
}
func TestHTMLHeaderFooterRendered(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name: "Summary",
			HeaderFooter: &resolve.HeaderFooter{
				OddHeader: "&LLeft Head&CPage &P&RRight Head",
				OddFooter: "&LConfidential&C&A&RPage &P of &N",
			},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "Data"}},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// Header bar must be present.
	if !strings.Contains(out, "sheet-header") {
		t.Fatalf("expected sheet-header div in output:\n%s", out)
	}
	// Left section text.
	if !strings.Contains(out, "Left Head") {
		t.Fatalf("expected 'Left Head' in header:\n%s", out)
	}
	// Page number placeholder span.
	if !strings.Contains(out, "hf-page-num") {
		t.Fatalf("expected hf-page-num span in header:\n%s", out)
	}
	// Footer bar must be present.
	if !strings.Contains(out, "sheet-footer") {
		t.Fatalf("expected sheet-footer div in output:\n%s", out)
	}
	// &A in footer should resolve to sheet name.
	if !strings.Contains(out, "Summary") {
		t.Fatalf("expected sheet name 'Summary' in footer:\n%s", out)
	}
	// "Confidential" in footer left section.
	if !strings.Contains(out, "Confidential") {
		t.Fatalf("expected 'Confidential' in footer:\n%s", out)
	}
}

func TestHTMLAutoFilterActiveColumn(t *testing.T) {
	wb := &resolve.Workbook{
		Sheets: []*resolve.Sheet{{
			Name:          "Sheet1",
			AutoFilterRef: "A1:C1",
			// Column B (ColIndex=1 within range, i.e. absolute col 2) has active criteria.
			FilterColumns: []*resolve.FilterColumn{
				{ColIndex: 1, Filters: []string{"Sales"}},
			},
			Rows: []*resolve.Row{{
				Index: 1,
				Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "Dept"},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellString, StrValue: "Category"},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellString, StrValue: "Amount"},
				},
			}},
		}},
	}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	// The actively-filtered column B1 should carry cell--af-active class.
	if !strings.Contains(out, "cell--af-active") {
		t.Fatalf("expected cell--af-active class in output:\n%s", out)
	}
	// The active arrow ▼ (U+25BC) should appear in the output.
	if !strings.Contains(out, "\u25bc") {
		t.Fatalf("expected filled arrow ▼ for active column:\n%s", out)
	}
	// The inactive header cells A1 and C1 should NOT have cell--af-active.
	// We can verify by checking the small arrow ▾ (U+25BE) is also present.
	if !strings.Contains(out, "\u25be") {
		t.Fatalf("expected small arrow ▾ for non-active header columns:\n%s", out)
	}
	// The af-arrow--active class should be on the active arrow span.
	if !strings.Contains(out, "af-arrow--active") {
		t.Fatalf("expected af-arrow--active class in output:\n%s", out)
	}
}

func TestHTMLFormulaUsesEvaluatedDefinedNameOverStaleCachedValue(t *testing.T) {
	wb := &resolve.Workbook{
		DefinedNames: []*resolve.DefinedName{{
			Name:  "StartingBalance",
			Value: `'Setup'!$C$13`,
		}},
		Sheets: []*resolve.Sheet{
			{
				Name: "Setup",
				Rows: []*resolve.Row{{
					Index: 13,
					Cells: []*resolve.Cell{{
						Row: 13, Col: 3, Ref: "C13", Kind: resolve.CellNumber, NumValue: 5000,
					}},
				}},
			},
			{
				Name: "Summary",
				Rows: []*resolve.Row{{
					Index: 1,
					Cells: []*resolve.Cell{{
						Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 0,
						Formula: "StartingBalance",
						Style:   resolve.CellStyle{NumFmtCode: "$#,##0"},
					}},
				}},
			},
		},
	}

	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, "$5,000") {
		t.Fatalf("expected evaluated defined-name value in output:\n%s", out)
	}
	if strings.Contains(out, ">$0<") {
		t.Fatalf("expected stale cached zero to be replaced in output:\n%s", out)
	}
}
func TestHTMLRendersScrollableControlStepsAndOrientation(t *testing.T) {
	wb := &resolve.Workbook{Sheets: []*resolve.Sheet{{
		Name: "Sheet1",
		FormControls: []*resolve.FormControl{
			{Type: "Scroll", AltText: "Vertical", Anchor: "0,0,0,0,1,0,4,0", Min: 1, Max: 20, Val: 7, Increment: 2, PageStep: 5},
			{Type: "Scroll", AltText: "Horizontal", Anchor: "2,0,0,0,5,0,1,0", Min: 0, Max: 10, Val: 4, Increment: 1, PageStep: 3, Horizontal: true},
		},
	}}}
	html, _, err := render.HTMLWithOptions(wb, render.HTMLOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`form-control--scroll-vertical`, `step=2`, `data-page-step=5`, `writing-mode:vertical-lr`,
		`form-control--scroll-horizontal`, `step=1`, `data-page-step=3`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in HTML: %s", want, html)
		}
	}
}
func TestHTMLRendersSmartArtAndOLEFallbacks(t *testing.T) {
	wb := &resolve.Workbook{Sheets: []*resolve.Sheet{{Name: "Sheet1", Drawings: []*resolve.Drawing{
		{Name: "Process", Type: resolve.DrawingTypeSmartArt, Position: resolve.DrawingPosition{Anchor: resolve.AnchorAbsolute, Width: 914400, Height: 457200}, SmartArt: &resolve.DrawingSmartArt{}},
		{Name: "Object", Type: resolve.DrawingTypeOLE, Position: resolve.DrawingPosition{Anchor: resolve.AnchorAbsolute, X: 914400, Width: 914400, Height: 457200}, OLE: &resolve.DrawingOLE{ProgID: "Package"}},
	}}}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "drawing--smartart") || !strings.Contains(out, "drawing--ole") || !strings.Contains(out, "Package") {
		t.Fatalf("missing object fallback: %s", out)
	}
}

func TestHTMLAppliesDrawingShapeRotationAndFlips(t *testing.T) {
	wb := &resolve.Workbook{Sheets: []*resolve.Sheet{{Name: "Sheet1", Rows: []*resolve.Row{{Index: 1, Cells: []*resolve.Cell{{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellString, StrValue: "x"}}}}, Drawings: []*resolve.Drawing{{Name: "Rotated", Type: resolve.DrawingTypeShape, Position: resolve.DrawingPosition{Anchor: resolve.AnchorAbsolute, Width: 914400, Height: 457200}, Shape: &resolve.DrawingShape{Geometry: "rect", RotationDeg: 30, FlipH: true, FlipV: true}}}}}}
	html, err := render.HTML(wb)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"rotate(30.0000deg)", "scaleX(-1)", "scaleY(-1)"} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in %s", want, html)
		}
	}
}

func TestHTMLRendersDummyFunctionSparklineAsCellVisual(t *testing.T) {
	wb := &resolve.Workbook{Sheets: []*resolve.Sheet{
		{
			Name: "Dashboard",
			Rows: []*resolve.Row{
				{Index: 1, Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 25},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellNumber, NumValue: 100},
					{Row: 1, Col: 3, Ref: "C1", Kind: resolve.CellFormula, Formula: `IFERROR(__xludf.DUMMYFUNCTION("SPARKLINE(A1,{""charttype"",""bar"";""max"",MAX(A1:B1);""color1"",""#334960""})"),"")`},
				}},
			},
		},
	}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="formula-sparkline"`) {
		t.Fatalf("expected SPARKLINE compatibility formula to render an SVG visual:\n%s", out)
	}
	if !strings.Contains(out, `fill="#334960"`) {
		t.Fatalf("expected authored sparkline color in SVG:\n%s", out)
	}
	if strings.Contains(out, `>#VALUE!<`) {
		t.Fatalf("sparkline visual must replace unsupported-UDF #VALUE! display:\n%s", out)
	}
}

func TestHTMLRendersColumnSparklineWithAuthoredFirstColor(t *testing.T) {
	wb := &resolve.Workbook{Sheets: []*resolve.Sheet{
		{
			Name: "Dashboard",
			Rows: []*resolve.Row{
				{Index: 1, Cells: []*resolve.Cell{
					{Row: 1, Col: 1, Ref: "A1", Kind: resolve.CellNumber, NumValue: 40},
					{Row: 1, Col: 2, Ref: "B1", Kind: resolve.CellFormula, Formula: `IFERROR(__xludf.DUMMYFUNCTION("SPARKLINE(A1,{""charttype"",""column"";""ymin"",0;""ymax"",100;""firstcolor"",""#f46524""})"),"")`},
				}},
			},
		},
	}}
	out, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(out, `class="formula-sparkline"`) || !strings.Contains(out, `fill="#f46524"`) {
		t.Fatalf("expected authored column sparkline visual:\n%s", out)
	}
}
