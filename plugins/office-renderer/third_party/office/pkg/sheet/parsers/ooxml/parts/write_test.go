package parts

import (
	"bytes"
	"testing"

	"office/pkg/ooxml/opc"
)

func TestWritePackageRoundTrip(t *testing.T) {
	// Build a minimal spreadsheet programmatically.
	pkg := &Package{
		Workbook: &Workbook{
			Sheets: []SheetRef{
				{SheetID: 1, Name: "Sheet1", RelID: "rId1"},
			},
		},
		Worksheets: []*WorksheetEntry{
			{
				SheetID:  1,
				RelID:    "rId1",
				Name:     "Sheet1",
				PartName: "xl/worksheets/sheet1.xml",
				Sheet: &Worksheet{
					Rows: []Row{
						{
							Index: 1,
							Cells: []Cell{
								{Ref: "A1", Type: "s", Value: "0"},
								{Ref: "B1", Value: "42"},
							},
						},
					},
				},
			},
		},
		SharedStrings: &SharedStrings{
			Count:       1,
			UniqueCount: 1,
			Items:       []RichText{{PlainText: "hello"}},
		},
		Styles: &StyleSheet{
			Fonts:   []Font{{}},
			Fills:   []Fill{{PatternFill: &PatternFill{PatternType: "none"}}},
			Borders: []Border{{}},
			CellXfs: []Xf{{}},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	// Read back.
	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	if got := opcPkg.PackageType(); got != opc.PackageTypeSpreadsheet {
		t.Fatalf("package type = %q, want spreadsheet", got)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(loaded.Worksheets) != 1 {
		t.Fatalf("worksheets = %d, want 1", len(loaded.Worksheets))
	}

	ws := loaded.Worksheets[0].Sheet
	if len(ws.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(ws.Rows))
	}
	if len(ws.Rows[0].Cells) != 2 {
		t.Fatalf("cells = %d, want 2", len(ws.Rows[0].Cells))
	}
	if ws.Rows[0].Cells[0].Value != "0" {
		t.Fatalf("A1 value = %q, want 0", ws.Rows[0].Cells[0].Value)
	}
	if ws.Rows[0].Cells[1].Value != "42" {
		t.Fatalf("B1 value = %q, want 42", ws.Rows[0].Cells[1].Value)
	}

	if loaded.SharedStrings == nil || len(loaded.SharedStrings.Items) != 1 {
		t.Fatal("shared strings missing or wrong count after round-trip")
	}
	if loaded.SharedStrings.Items[0].PlainValue() != "hello" {
		t.Fatalf("shared string = %q, want hello", loaded.SharedStrings.Items[0].PlainValue())
	}
}

func TestWritePackageRoundTripWithTables(t *testing.T) {
	pkg := &Package{
		Workbook: &Workbook{
			Sheets: []SheetRef{{SheetID: 1, Name: "Sheet1", RelID: "rId1"}},
		},
		Worksheets: []*WorksheetEntry{
			{
				SheetID:  1,
				RelID:    "rId1",
				Name:     "Sheet1",
				PartName: "xl/worksheets/sheet1.xml",
				Sheet: &Worksheet{
					Rows: []Row{{
						Index: 1,
						Cells: []Cell{
							{Ref: "A1", Type: "s", Value: "0"},
							{Ref: "B1", Value: "42"},
						},
					}},
				},
				Tables: []*Table{
					{
						ID:          1,
						Name:        "Table1",
						DisplayName: "Table1",
						Ref:         "A1:B2",
						Columns: []TableColumn{
							{ID: 1, Name: "Column1"},
							{ID: 2, Name: "Column2"},
						},
					},
				},
			},
		},
		SharedStrings: &SharedStrings{
			Count:       1,
			UniqueCount: 1,
			Items:       []RichText{{PlainText: "hello"}},
		},
		Styles: &StyleSheet{
			Fonts:   []Font{{}},
			Fills:   []Fill{{PatternFill: &PatternFill{PatternType: "none"}}},
			Borders: []Border{{}},
			CellXfs: []Xf{{}},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(loaded.Worksheets) != 1 {
		t.Fatalf("worksheets = %d, want 1", len(loaded.Worksheets))
	}
	ws := loaded.Worksheets[0].Sheet
	if ws.TableParts == nil || len(ws.TableParts.Tables) != 1 {
		t.Fatalf("tableParts = %+v, want 1 table", ws.TableParts)
	}
	if loaded.Worksheets[0].Tables == nil || len(loaded.Worksheets[0].Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(loaded.Worksheets[0].Tables))
	}
	tbl := loaded.Worksheets[0].Tables[0]
	if tbl.Name != "Table1" {
		t.Fatalf("table name = %q, want Table1", tbl.Name)
	}
	if len(tbl.Columns) != 2 {
		t.Fatalf("table columns = %d, want 2", len(tbl.Columns))
	}
}

func TestWritePackageEmptyWorkbook(t *testing.T) {
	pkg := &Package{
		Workbook: &Workbook{
			Sheets: []SheetRef{
				{SheetID: 1, Name: "Empty", RelID: "rId1"},
			},
		},
		Worksheets: []*WorksheetEntry{
			{
				SheetID:  1,
				RelID:    "rId1",
				Name:     "Empty",
				PartName: "xl/worksheets/sheet1.xml",
				Sheet:    &Worksheet{},
			},
		},
		Styles: &StyleSheet{
			Fonts:   []Font{{}},
			Fills:   []Fill{{PatternFill: &PatternFill{PatternType: "none"}}},
			Borders: []Border{{}},
			CellXfs: []Xf{{}},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(loaded.Worksheets) != 1 {
		t.Fatalf("worksheets = %d, want 1", len(loaded.Worksheets))
	}
}

func TestWritePackageMultipleSheetsNoDuplicateRels(t *testing.T) {
	pkg := &Package{
		Workbook: &Workbook{
			Sheets: []SheetRef{
				{SheetID: 1, Name: "Sheet1", RelID: "rId1"},
				{SheetID: 2, Name: "Sheet2", RelID: "rId2"},
			},
		},
		Worksheets: []*WorksheetEntry{
			{
				SheetID:  1,
				RelID:    "rId1",
				Name:     "Sheet1",
				PartName: "xl/worksheets/sheet1.xml",
				Sheet: &Worksheet{
					Rows: []Row{{
						Index: 1,
						Cells: []Cell{{Ref: "A1", Value: "1"}},
					}},
				},
			},
			{
				SheetID:  2,
				RelID:    "rId2",
				Name:     "Sheet2",
				PartName: "xl/worksheets/sheet2.xml",
				Sheet: &Worksheet{
					Rows: []Row{{
						Index: 1,
						Cells: []Cell{{Ref: "A1", Type: "s", Value: "0"}},
					}},
				},
			},
		},
		SharedStrings: &SharedStrings{
			Count:       1,
			UniqueCount: 1,
			Items:       []RichText{{PlainText: "shared"}},
		},
		Styles: &StyleSheet{
			Fonts:   []Font{{}},
			Fills:   []Fill{{PatternFill: &PatternFill{PatternType: "none"}}},
			Borders: []Border{{}},
			CellXfs: []Xf{{}},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(loaded.Worksheets) != 2 {
		t.Fatalf("worksheets = %d, want 2", len(loaded.Worksheets))
	}
	if loaded.Worksheets[0].Name != "Sheet1" {
		t.Errorf("sheet[0] name = %q, want Sheet1", loaded.Worksheets[0].Name)
	}
	if loaded.Worksheets[1].Name != "Sheet2" {
		t.Errorf("sheet[1] name = %q, want Sheet2", loaded.Worksheets[1].Name)
	}
}

func TestWritePackageRoundTripWithComments(t *testing.T) {
	pkg := &Package{
		Workbook: &Workbook{
			Sheets: []SheetRef{{SheetID: 1, Name: "Sheet1", RelID: "rId1"}},
		},
		Worksheets: []*WorksheetEntry{
			{
				SheetID:  1,
				RelID:    "rId1",
				Name:     "Sheet1",
				PartName: "xl/worksheets/sheet1.xml",
				Sheet: &Worksheet{
					Rows: []Row{{
						Index: 1,
						Cells: []Cell{{Ref: "A1", Value: "42"}},
					}},
				},
				Comments: &Comments{
					Authors: []string{"Author One"},
					CommentList: []CommentEntry{
						{
							Ref:      "A1",
							AuthorID: 0,
							Text:     RichText{PlainText: "Hello comment"},
						},
					},
				},
			},
		},
		Styles: &StyleSheet{
			Fonts:   []Font{{}},
			Fills:   []Fill{{PatternFill: &PatternFill{PatternType: "none"}}},
			Borders: []Border{{}},
			CellXfs: []Xf{{}},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(loaded.Worksheets) != 1 {
		t.Fatalf("worksheets = %d, want 1", len(loaded.Worksheets))
	}
	if loaded.Worksheets[0].Comments == nil {
		t.Fatal("comments missing after round-trip")
	}
	comments := loaded.Worksheets[0].Comments
	if len(comments.Authors) != 1 || comments.Authors[0] != "Author One" {
		t.Fatalf("authors = %v, want [Author One]", comments.Authors)
	}
	if len(comments.CommentList) != 1 {
		t.Fatalf("comment list = %d, want 1", len(comments.CommentList))
	}
	c := comments.CommentList[0]
	if c.Ref != "A1" {
		t.Errorf("comment ref = %q, want A1", c.Ref)
	}
	if c.Text.PlainValue() != "Hello comment" {
		t.Errorf("comment text = %q, want Hello comment", c.Text.PlainValue())
	}
	if len(loaded.Worksheets[0].LegacyDrawingVML) == 0 {
		t.Fatal("expected synthesized legacy drawing VML for comments")
	}
	if loaded.Worksheets[0].Sheet == nil || loaded.Worksheets[0].Sheet.LegacyDrawing == nil {
		t.Fatal("expected worksheet legacyDrawing reference for comments")
	}
	if note := loaded.Worksheets[0].LegacyNotes["A1"]; note.Ref != "A1" {
		t.Fatalf("expected legacy note metadata for A1, got %#v", loaded.Worksheets[0].LegacyNotes)
	}
}

func TestWritePackageRoundTripWithConnections(t *testing.T) {
	pkg := &Package{
		Workbook: &Workbook{
			Sheets: []SheetRef{{SheetID: 1, Name: "Sheet1", RelID: "rId1"}},
		},
		Worksheets: []*WorksheetEntry{
			{
				SheetID:  1,
				RelID:    "rId1",
				Name:     "Sheet1",
				PartName: "xl/worksheets/sheet1.xml",
				Sheet:    &Worksheet{},
			},
		},
		Connections: &Connections{
			Connections: []Connection{
				{
					ID:   1,
					Name: "Conn1",
					Type: "1",
					DBPr: &ConnectionDBPr{
						Connection: "Provider=SQLOLEDB",
						Command:    "SELECT * FROM data",
					},
				},
			},
		},
		Styles: &StyleSheet{
			Fonts:   []Font{{}},
			Fills:   []Fill{{PatternFill: &PatternFill{PatternType: "none"}}},
			Borders: []Border{{}},
			CellXfs: []Xf{{}},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if loaded.Connections == nil {
		t.Fatal("connections missing after round-trip")
	}
	if len(loaded.Connections.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(loaded.Connections.Connections))
	}
	conn := loaded.Connections.Connections[0]
	if conn.Name != "Conn1" {
		t.Errorf("connection name = %q, want Conn1", conn.Name)
	}
	if conn.DBPr == nil || conn.DBPr.Command != "SELECT * FROM data" {
		t.Errorf("connection command = %q, want SELECT * FROM data", conn.DBPr.Command)
	}
}

func TestWritePackageRoundTripWithExternalLinks(t *testing.T) {
	pkg := &Package{
		Workbook: &Workbook{
			Sheets: []SheetRef{{SheetID: 1, Name: "Sheet1", RelID: "rId1"}},
		},
		Worksheets: []*WorksheetEntry{
			{
				SheetID:  1,
				RelID:    "rId1",
				Name:     "Sheet1",
				PartName: "xl/worksheets/sheet1.xml",
				Sheet:    &Worksheet{},
			},
		},
		ExternalLinks: []*ExternalLinkEntry{
			{
				RelID:    "rIdExtLink1",
				PartName: "xl/externalLinks/externalLink1.xml",
				Target:   "external.xlsx",
				Link: &ExternalLink{
					ExternalBook: &ExternalBook{
						RelID:      "rId1",
						SheetNames: []ExternalSheetName{{Val: "Data"}},
					},
				},
			},
		},
		Styles: &StyleSheet{
			Fonts:   []Font{{}},
			Fills:   []Fill{{PatternFill: &PatternFill{PatternType: "none"}}},
			Borders: []Border{{}},
			CellXfs: []Xf{{}},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(loaded.ExternalLinks) != 1 {
		t.Fatalf("external links = %d, want 1", len(loaded.ExternalLinks))
	}
	if loaded.ExternalLinks[0].Target != "external.xlsx" {
		t.Errorf("target = %q, want external.xlsx", loaded.ExternalLinks[0].Target)
	}
	if len(loaded.Workbook.ExternalReferences) != 1 {
		t.Errorf("external references = %d, want 1", len(loaded.Workbook.ExternalReferences))
	}
}

func TestWritePackageRoundTripWithPivotTables(t *testing.T) {
	pkg := &Package{
		Workbook: &Workbook{
			Sheets: []SheetRef{{SheetID: 1, Name: "Sheet1", RelID: "rId1"}},
		},
		Worksheets: []*WorksheetEntry{
			{
				SheetID:  1,
				RelID:    "rId1",
				Name:     "Sheet1",
				PartName: "xl/worksheets/sheet1.xml",
				Sheet:    &Worksheet{},
				PivotTables: []*PivotTableWithCache{
					{
						Table: &PivotTableDefinition{
							Name:    "PivotTable1",
							CacheID: 5,
							Raw:     []byte(`<pivotTableDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" name="PivotTable1" cacheId="5"><location ref="A3:C5" firstHeaderRow="1" firstDataRow="2" firstDataCol="1"/><pivotFields count="0"/><rowFields count="0"/><dataFields count="0"/></pivotTableDefinition>`),
						},
						Cache: &PivotCacheDefinition{
							Raw: []byte(`<pivotCacheDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" refreshedBy="Excel" refreshedDate="2026-01-01T00:00:00Z" recordCount="1"><cacheSource type="worksheet"><worksheetSource sheet="Data" ref="A1:C10"/></cacheSource></pivotCacheDefinition>`),
						},
						Records: &PivotCacheRecords{
							Raw: []byte(`<pivotCacheRecords xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="1"><r><c v="1"/></r></pivotCacheRecords>`),
						},
					},
				},
			},
		},
		Styles: &StyleSheet{
			Fonts:   []Font{{}},
			Fills:   []Fill{{PatternFill: &PatternFill{PatternType: "none"}}},
			Borders: []Border{{}},
			CellXfs: []Xf{{}},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(loaded.Worksheets[0].PivotTables) != 1 {
		t.Fatalf("pivot tables = %d, want 1", len(loaded.Worksheets[0].PivotTables))
	}
	ptc := loaded.Worksheets[0].PivotTables[0]
	if ptc.Table == nil || ptc.Table.Name != "PivotTable1" {
		t.Errorf("table name = %q, want PivotTable1", ptc.Table.Name)
	}
	if ptc.Cache == nil {
		t.Fatal("expected pivot cache after round-trip")
	}
	if ptc.Records == nil {
		t.Fatal("expected pivot cache records after round-trip")
	}
	if len(ptc.Records.Rows) != 1 || len(ptc.Records.Rows[0].Cells) != 1 || ptc.Records.Rows[0].Cells[0].Value != "1" {
		t.Fatalf("pivot cache records = %#v, want one row with value 1", ptc.Records.Rows)
	}
	if ptc.Cache.WorksheetSource == nil || ptc.Cache.WorksheetSource.Sheet != "Data" || ptc.Cache.WorksheetSource.Ref != "A1:C10" {
		t.Fatalf("pivot cache worksheet source = %#v, want Data!A1:C10", ptc.Cache.WorksheetSource)
	}
	if len(loaded.Workbook.PivotCaches) != 1 {
		t.Fatalf("workbook pivot caches = %d, want 1", len(loaded.Workbook.PivotCaches))
	}
	if loaded.Workbook.PivotCaches[0].CacheID != 5 {
		t.Errorf("cache id = %d, want 5", loaded.Workbook.PivotCaches[0].CacheID)
	}
	if loaded.Worksheets[0].Sheet == nil || loaded.Worksheets[0].Sheet.PivotTables == nil {
		t.Fatal("expected worksheet pivotTables element")
	}
	if loaded.Worksheets[0].Sheet.PivotTables.Count != 1 {
		t.Errorf("worksheet pivotTables count = %d, want 1", loaded.Worksheets[0].Sheet.PivotTables.Count)
	}
}

func TestWritePackageRoundTripWithVMLDrawing(t *testing.T) {
	vml := []byte(`<xml xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office"><v:shapetype id="_x0000_t202" coordsize="21600,21600" o:spt="202" path="m,l,21600,21600,21600,21600,xe"><v:stroke joinstyle="miter"/></v:shapetype><v:shape id="RoundedRectangle" type="#_x0000_t202" style="position:absolute;margin-left:0pt;margin-top:0pt;width:100pt;height:50pt;z-index:1" fillcolor="yellow"/></xml>`)
	pkg := &Package{
		Workbook: &Workbook{
			Sheets: []SheetRef{{SheetID: 1, Name: "Sheet1", RelID: "rId1"}},
		},
		Worksheets: []*WorksheetEntry{
			{
				SheetID:  1,
				RelID:    "rId1",
				Name:     "Sheet1",
				PartName: "xl/worksheets/sheet1.xml",
				Sheet: &Worksheet{
					LegacyDrawing: &DrawingRef{RelID: "rIdVML1"},
				},
				LegacyDrawingVML: vml,
			},
		},
		Styles: &StyleSheet{
			Fonts:   []Font{{}},
			Fills:   []Fill{{PatternFill: &PatternFill{PatternType: "none"}}},
			Borders: []Border{{}},
			CellXfs: []Xf{{}},
		},
	}

	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatalf("write package: %v", err)
	}

	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("open written bytes: %v", err)
	}

	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatalf("load package: %v", err)
	}

	if len(loaded.Worksheets[0].LegacyDrawingVML) == 0 {
		t.Fatal("legacy drawing VML missing after round-trip")
	}
	if loaded.Worksheets[0].Sheet == nil || loaded.Worksheets[0].Sheet.LegacyDrawing == nil {
		t.Fatal("expected worksheet legacyDrawing reference after round-trip")
	}
}
