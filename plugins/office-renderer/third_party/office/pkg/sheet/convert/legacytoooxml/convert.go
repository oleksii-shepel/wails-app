// Package legacytoooxml converts legacy Excel workbooks (.xls) to OOXML
// SpreadsheetML packages (.xlsx).
package legacytoooxml

import (
	"fmt"
	"sort"
	"strconv"

	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

// Convert transforms a legacy workbook IR into an OOXML parts.Package
// that can be serialized to a .xlsx file via sheetparts.WritePackage.
func Convert(wb *sheetir.Workbook) (*sheetparts.Package, error) {
	if wb == nil {
		return nil, fmt.Errorf("nil workbook")
	}

	c := &converter{
		pkg: &sheetparts.Package{
			Workbook:      &sheetparts.Workbook{},
			SharedStrings: &sheetparts.SharedStrings{},
			Styles: &sheetparts.StyleSheet{
				Fonts:        []sheetparts.Font{{}},
				Fills:        []sheetparts.Fill{{PatternFill: &sheetparts.PatternFill{PatternType: "none"}}, {PatternFill: &sheetparts.PatternFill{PatternType: "gray125"}}},
				Borders:      []sheetparts.Border{{}},
				CellStyleXfs: []sheetparts.Xf{{}},
				CellXfs:      []sheetparts.Xf{{XfID: 0}},
				CellStyles:   []sheetparts.CellStyle{{Name: "Normal", XfID: 0, BuiltinID: 0}},
			},
		},
		stringIndex: make(map[string]int),
		irWb:        wb,
	}

	if wb.ActiveSheetIndex != nil {
		active := *wb.ActiveSheetIndex
		c.pkg.Workbook.BookViews = []sheetparts.WorkbookView{{ActiveTab: &active}}
	}

	c.mapWorkbookProtection(wb)
	c.initNamedCellStyles()

	// Map defined names first so they're available.
	c.mapDefinedNames(wb)

	// Map workbook-level connections and external links.
	c.mapConnections(wb)
	c.mapExternalLinks(wb)

	// Convert each sheet.
	for i, sheet := range wb.Sheets {
		if err := c.convertSheet(sheet, i+1); err != nil {
			return nil, fmt.Errorf("sheet %q: %w", sheet.Name, err)
		}
	}

	return c.pkg, nil
}

type converter struct {
	pkg               *sheetparts.Package
	stringIndex       map[string]int
	irWb              *sheetir.Workbook
	nextRelID         int
	nextMediaNum      int
	nextPivotCacheNum int
}

func (c *converter) nextRID() string {
	c.nextRelID++
	return fmt.Sprintf("rId%d", c.nextRelID)
}

func (c *converter) mapExternalLinks(wb *sheetir.Workbook) {
	if len(wb.ExternalLinks) == 0 {
		return
	}
	for i, ie := range wb.ExternalLinks {
		entry := &sheetparts.ExternalLinkEntry{
			RelID:    fmt.Sprintf("rIdExtLink%d", i+1),
			PartName: fmt.Sprintf("xl/externalLinks/externalLink%d.xml", i+1),
			Target:   ie.Target,
			Link: &sheetparts.ExternalLink{
				ExternalBook: &sheetparts.ExternalBook{
					RelID: "rId1",
				},
			},
		}
		for _, name := range ie.SheetNames {
			entry.Link.ExternalBook.SheetNames = append(entry.Link.ExternalBook.SheetNames, sheetparts.ExternalSheetName{Val: name})
		}
		for _, dn := range ie.DefinedNames {
			entry.Link.ExternalBook.DefinedNames = append(entry.Link.ExternalBook.DefinedNames, sheetparts.ExternalDefinedName{
				Name:     dn.Name,
				RefersTo: dn.RefersTo,
				SheetID:  dn.SheetID,
			})
		}
		if len(ie.CachedSheets) > 0 {
			sds := &sheetparts.ExternalSheetDataSet{}
			for _, s := range ie.CachedSheets {
				esd := sheetparts.ExternalSheetData{SheetID: s.ID, RefreshError: s.RefreshError}
				rowMap := make(map[int][]sheetparts.ExternalSheetCell)
				for _, cell := range s.Cells {
					rowMap[rowFromRef(cell.Ref)] = append(rowMap[rowFromRef(cell.Ref)], sheetparts.ExternalSheetCell{
						Ref:   cell.Ref,
						Type:  cell.Type,
						Value: cell.Value,
					})
				}
				rows := make([]int, 0, len(rowMap))
				for r := range rowMap {
					rows = append(rows, r)
				}
				sort.Ints(rows)
				for _, r := range rows {
					esd.Rows = append(esd.Rows, sheetparts.ExternalSheetRow{Index: r, Cells: rowMap[r]})
				}
				sds.Sheets = append(sds.Sheets, esd)
			}
			entry.Link.ExternalBook.SheetDataSet = sds
		}
		c.pkg.ExternalLinks = append(c.pkg.ExternalLinks, entry)
	}
}

func rowFromRef(ref string) int {
	i := 0
	for i < len(ref) && (ref[i] < '0' || ref[i] > '9') {
		i++
	}
	if i >= len(ref) {
		return 0
	}
	n, _ := strconv.Atoi(ref[i:])
	return n
}

func (c *converter) nextMediaName(contentType string) string {
	c.nextMediaNum++
	return fmt.Sprintf("xl/media/image%d.%s", c.nextMediaNum, mediaExtension(contentType))
}

func (c *converter) nextPivotCacheID() int {
	c.nextPivotCacheNum++
	return c.nextPivotCacheNum
}

func (c *converter) mapConnections(wb *sheetir.Workbook) {
	if len(wb.Connections) == 0 {
		return
	}
	conn := &sheetparts.Connections{}
	for _, ic := range wb.Connections {
		pc := sheetparts.Connection{
			ID:            ic.ID,
			Name:          ic.Name,
			Type:          ic.Kind,
			Description:   ic.Description,
			RefreshOnLoad: ic.RefreshOnLoad,
			Background:    ic.Background,
			SaveData:      ic.SaveData,
			SourceFile:    ic.SourceFile,
			ODCFile:       ic.ODCFile,
		}
		if ic.Connection != "" || ic.Command != "" || ic.CommandType != "" {
			pc.DBPr = &sheetparts.ConnectionDBPr{
				Connection:  ic.Connection,
				Command:     ic.Command,
				CommandType: ic.CommandType,
			}
		}
		if ic.LocalCube != "" {
			pc.OLAPPr = &sheetparts.ConnectionOLAPPr{LocalConnection: ic.LocalCube}
		}
		if ic.TextFile != "" || ic.TextDelimiter != "" {
			pc.TextPr = &sheetparts.ConnectionTextPr{
				SourceFile: ic.TextFile,
				Delimiter:  ic.TextDelimiter,
			}
		}
		if ic.URL != "" || ic.WebPost != "" {
			pc.WebPr = &sheetparts.ConnectionWebPr{
				URL:         ic.URL,
				Post:        ic.WebPost,
				HtmlTables:  ic.WebHTMLTables,
				SourceData:  ic.WebSourceData,
				ParsePre:    ic.WebParsePre,
				Consecutive: ic.WebConsecutive,
				FirstRow:    ic.WebFirstRow,
				TextDates:   ic.WebTextDates,
			}
		}
		conn.Connections = append(conn.Connections, pc)
	}
	c.pkg.Connections = conn
}

func (c *converter) addString(s string) int {
	if idx, ok := c.stringIndex[s]; ok {
		return idx
	}
	idx := len(c.pkg.SharedStrings.Items)
	c.pkg.SharedStrings.Items = append(c.pkg.SharedStrings.Items, sheetparts.RichText{PlainText: s})
	c.stringIndex[s] = idx
	return idx
}

func (c *converter) convertSheet(sheet *sheetir.Sheet, sheetNum int) error {
	relID := c.nextRID()
	partName := fmt.Sprintf("xl/worksheets/sheet%d.xml", sheetNum)

	ws := &sheetparts.Worksheet{
		SheetFormatPr: &sheetparts.SheetFormatPr{},
	}

	// Default row/col heights.
	if sheet.DefaultRowHeightPt > 0 {
		ws.SheetFormatPr.DefaultRowHeight = sheet.DefaultRowHeightPt
	}
	if sheet.DefaultColWidthChars > 0 {
		ws.SheetFormatPr.DefaultColWidth = sheet.DefaultColWidthChars
	}

	// Dimension.
	if sheet.DimFirstRow > 0 && sheet.DimFirstCol > 0 {
		ws.Dimension = &sheetparts.SheetDimension{
			Ref: fmt.Sprintf("%s%d:%s%d",
				colName(sheet.DimFirstCol), sheet.DimFirstRow,
				colName(sheet.DimLastCol), sheet.DimLastRow),
		}
	}

	// Column metadata.
	c.mapColumns(sheet, ws)

	// Rows and cells.
	c.mapRowsAndCells(sheet, ws)

	// Merged cells.
	c.mapMergedCells(sheet, ws)

	// Hyperlinks and filters.
	hyperlinkURLs := c.mapHyperlinks(sheet, ws)
	c.mapAutoFilter(sheet, ws)

	// Page setup / header-footer.
	c.mapPageSetup(sheet, ws)

	// Protection.
	c.mapSheetProtection(sheet, ws)

	// Data validations.
	c.mapDataValidations(sheet, ws)

	// Conditional formatting.
	c.mapConditionalFormatting(sheet, ws)

	// Freeze/split panes.
	c.mapPanes(sheet, ws)

	// Comments.
	comments := c.mapComments(sheet)

	// VML form controls.
	formControls, legacyDrawingVML := c.mapFormControls(sheet)

	// Tables.
	tables := c.mapTables(sheet)

	// Pivot tables.
	pivotTables := c.mapPivotTables(sheet, c.irWb)

	// Drawings.
	drawing, drawingMedia, drawingCharts, drawingDiagrams, err := c.mapDrawings(sheet)
	if err != nil {
		return err
	}

	oleObjects, oleParts := mapOLEObjects(sheet)
	ws.OLEObjects = oleObjects

	entry := &sheetparts.WorksheetEntry{
		SheetID:             sheetNum,
		RelID:               relID,
		Name:                sheet.Name,
		PartName:            partName,
		Sheet:               ws,
		HyperlinkURLs:       hyperlinkURLs,
		Comments:            comments,
		FormControls:        formControls,
		LegacyDrawingVML:    legacyDrawingVML,
		Tables:              tables,
		PivotTables:         pivotTables,
		Drawing:             drawing,
		DrawingDiagramParts: drawingDiagrams,
		OLEParts:            oleParts,
	}
	if drawingMedia != nil {
		entry.DrawingMedia = drawingMedia
	}
	if drawingCharts != nil {
		entry.DrawingChartParts = drawingCharts
	}

	c.pkg.Workbook.Sheets = append(c.pkg.Workbook.Sheets, sheetparts.SheetRef{
		Name:    sheet.Name,
		SheetID: sheetNum,
		RelID:   relID,
		State:   sheetVisibilityState(sheet),
	})
	c.pkg.Worksheets = append(c.pkg.Worksheets, entry)

	return nil
}

func colName(col int) string {
	var s string
	for col > 0 {
		col--
		s = string(rune('A'+col%26)) + s
		col /= 26
	}
	return s
}

func cellRef(row, col int) string {
	return colName(col) + strconv.Itoa(row)
}

func sheetVisibilityState(sheet *sheetir.Sheet) string {
	if sheet == nil {
		return ""
	}
	switch sheet.Visibility {
	case "hidden", "veryHidden":
		return sheet.Visibility
	}
	if sheet.Hidden {
		return "hidden"
	}
	return ""
}
