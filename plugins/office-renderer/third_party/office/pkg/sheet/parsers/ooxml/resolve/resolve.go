package resolve

import (
	"fmt"
	"math"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	officeartir "office/pkg/officeart/ir"
	officeartdml "office/pkg/officeart/ooxml/drawingml"
	"office/pkg/ooxml/chart"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/media"
	"office/pkg/ooxml/smartart"
	"office/pkg/sheet/parsers/ooxml/formula"
	"office/pkg/sheet/parsers/ooxml/parts"
)

// ResolvePackage converts a fully loaded parts.Package into a resolved Workbook IR.
// It remains as the full-parser oracle for tests and non-production tooling.
func ResolvePackage(pkg *parts.Package) (*Workbook, error) {
	return resolvePackage(pkg)
}

// ResolveMetadataPackage projects package-level metadata and the worksheet
// identity skeleton into canonical IR. Worksheet content is populated by SAX
// and relationship-heavy objects are loaded through their explicit boundary.
func ResolveMetadataPackage(pkg *parts.Package) (*Workbook, error) {
	return resolvePackage(pkg)
}

func resolvePackage(pkg *parts.Package) (*Workbook, error) {
	if pkg == nil {
		return nil, nil
	}

	sst := buildSST(pkg.SharedStrings)
	sstRich := buildSSTRich(pkg.SharedStrings)
	themeColors := buildThemeColorIndex(pkg.Theme)
	drawingThemeColors := buildDrawingThemeColorIndex(pkg.Theme)
	indexedColors := buildIndexedColorIndex(pkg.Styles)
	styles := buildStyleTable(pkg.Styles, themeColors, indexedColors)
	namedStyleXFs, namedStyles := buildNamedCellStyles(pkg.Styles, themeColors, indexedColors)
	dxfs := buildDxfTable(pkg.Styles, themeColors, indexedColors)

	wb := &Workbook{
		EmbeddedFonts:     resolveEmbeddedFonts(pkg.EmbeddedFonts),
		Diagnostics:       pkg.Diagnostics,
		NamedCellStyleXFs: namedStyleXFs,
		NamedCellStyles:   namedStyles,
	}
	if pkg.Workbook != nil {
		if pkg.Workbook.Protection != nil {
			wp := pkg.Workbook.Protection
			wb.Protection = &WorkbookProtection{
				WorkbookPassword:  wp.WorkbookPassword,
				RevisionsPassword: wp.RevisionsPassword,
				LockStructure:     wp.LockStructure,
				LockWindows:       wp.LockWindows,
				LockRevision:      wp.LockRevision,
				AlgorithmName:     wp.AlgorithmName,
				HashValue:         wp.HashValue,
				SaltValue:         wp.SaltValue,
				SpinCount:         wp.SpinCount,
			}
		}
		for i := range pkg.Workbook.DefinedNames {
			dn := pkg.Workbook.DefinedNames[i]
			wb.DefinedNames = append(wb.DefinedNames, &DefinedName{
				Name:         dn.Name,
				LocalSheetID: dn.LocalSheetID,
				Hidden:       dn.Hidden,
				Value:        strings.TrimSpace(dn.Value),
			})
		}
		if wp := pkg.Workbook.WorkbookPr; wp != nil {
			if wp.BackupFile != nil {
				v := *wp.BackupFile
				wb.BackupFile = &v
			}
			if wp.SaveExternalLinkValues != nil {
				v := *wp.SaveExternalLinkValues
				wb.SaveExternalLinkValues = &v
			}
		}
		for _, view := range pkg.Workbook.BookViews {
			if view.ActiveTab != nil {
				v := *view.ActiveTab
				wb.ActiveSheetIndex = &v
				break
			}
		}
		if fs := pkg.Workbook.FileSharing; fs != nil {
			wb.WriteAccessUser = strings.TrimSpace(fs.UserName)
		}
		if cp := pkg.Workbook.CalcPr; cp != nil {
			wb.Calc = resolveWorkbookCalcPr(cp)
		}
	}
	if pkg.Connections != nil {
		for i := range pkg.Connections.Connections {
			conn := pkg.Connections.Connections[i]
			wb.Connections = append(wb.Connections, resolveWorkbookConnection(conn))
		}
	}
	for _, entry := range pkg.ExternalLinks {
		if entry == nil {
			continue
		}
		wb.ExternalLinks = append(wb.ExternalLinks, resolveWorkbookExternalLink(entry))
	}
	printTitlesBySheetID := buildPrintTitleMap(pkg.Workbook)
	printAreasBySheetID := buildPrintAreaMap(pkg.Workbook)
	sheetVisibilityByID := buildSheetVisibilityByID(pkg.Workbook)
	for _, entry := range pkg.Worksheets {
		sheet := resolveWorksheet(entry, sst, sstRich, styles, dxfs, themeColors, indexedColors, entry.DrawingChartParts, drawingThemeColors)
		sheet.Visibility = sheetVisibilityByID[entry.SheetID]
		sheet.Hidden = sheet.Visibility != ""
		if area, ok := printAreasBySheetID[entry.SheetID]; ok {
			sheet.PrintArea = area
		}
		if pt, ok := printTitlesBySheetID[entry.SheetID]; ok {
			sheet.PrintTitleRows = pt.rows
			sheet.PrintTitleCols = pt.cols
		}
		wb.Sheets = append(wb.Sheets, sheet)
	}
	hydrateWorkbookCharts(wb)
	resolveDataValidationLists(wb)
	return wb, nil
}

// ApplyStreamingPrintMetadata projects print/page and worksheet-protection
// metadata emitted by the streaming parser into canonical Sheet IR.
func ApplyStreamingPrintMetadata(sheet *Sheet, printOptions *parts.PrintOptions, margins *parts.PageMargins, setup *parts.PageSetup, rowBreaks, colBreaks *parts.PageBreaks, protection *parts.SheetProtection) {
	if sheet == nil {
		return
	}
	if setup != nil || margins != nil || printOptions != nil {
		ps := &PageSetup{}
		if setup != nil {
			ps.PaperSize = setup.PaperSize
			ps.Orientation = setup.Orientation
			ps.Scale = setup.Scale
			ps.FitToPage = setup.FitToPage
			ps.FitToWidth = setup.FitToWidth
			ps.FitToHeight = setup.FitToHeight
			ps.FirstPageNumber = setup.FirstPageNumber
			ps.UseFirstPageNumber = setup.UseFirstPageNumber
			ps.PageOrder = setup.PageOrder
			ps.BlackAndWhite = setup.BlackAndWhite
			ps.Draft = setup.Draft
			ps.HorizontalDPI = setup.HorizontalDPI
			ps.VerticalDPI = setup.VerticalDPI
			ps.Copies = setup.Copies
		}
		if margins != nil {
			ps.MarginLeft = margins.Left
			ps.MarginRight = margins.Right
			ps.MarginTop = margins.Top
			ps.MarginBottom = margins.Bottom
			ps.MarginHeader = margins.Header
			ps.MarginFooter = margins.Footer
		}
		if printOptions != nil {
			ps.CenterHorizontally = printOptions.HorizontalCentered
			ps.CenterVertically = printOptions.VerticalCentered
			ps.PrintHeadings = printOptions.Headings
			ps.PrintGridlines = printOptions.GridLines
		}
		sheet.PageSetup = ps
	}
	sheet.RowPageBreaks = nil
	if rowBreaks != nil {
		for _, brk := range rowBreaks.Breaks {
			if brk.ID > 0 {
				sheet.RowPageBreaks = append(sheet.RowPageBreaks, brk.ID)
			}
		}
	}
	sheet.ColPageBreaks = nil
	if colBreaks != nil {
		for _, brk := range colBreaks.Breaks {
			if brk.ID > 0 {
				sheet.ColPageBreaks = append(sheet.ColPageBreaks, brk.ID)
			}
		}
	}
	if protection != nil {
		sp := protection
		sheet.Protection = &SheetProtection{Password: sp.Password, Sheet: sp.Sheet, Objects: sp.Objects, Scenarios: sp.Scenarios, FormatCells: sp.FormatCells, FormatColumns: sp.FormatColumns, FormatRows: sp.FormatRows, InsertColumns: sp.InsertColumns, InsertRows: sp.InsertRows, InsertHyperlinks: sp.InsertHyperlinks, DeleteColumns: sp.DeleteColumns, DeleteRows: sp.DeleteRows, SelectLockedCells: sp.SelectLockedCells, Sort: sp.Sort, AutoFilter: sp.AutoFilter, PivotTables: sp.PivotTables, SelectUnlockedCells: sp.SelectUnlockedCells, AlgorithmName: sp.AlgorithmName, HashValue: sp.HashValue, SaltValue: sp.SaltValue, SpinCount: sp.SpinCount}
	} else {
		sheet.Protection = nil
	}
}

func resolveWorkbookCalcPr(cp *parts.CalcPr) *WorkbookCalcSettings {
	out := &WorkbookCalcSettings{}
	any := false
	switch strings.TrimSpace(cp.CalcMode) {
	case "auto":
		v := 1
		out.Mode = &v
		any = true
	case "manual":
		v := 0
		out.Mode = &v
		any = true
	case "autoNoTable":
		v := -1
		out.Mode = &v
		any = true
	}
	if mode := strings.TrimSpace(cp.RefMode); mode == "A1" || mode == "R1C1" {
		out.ReferenceMode = mode
		any = true
	}
	if cp.CalcID != nil {
		v := *cp.CalcID
		out.RecalcID = &v
		any = true
	}
	if cp.Iterate != nil {
		v := *cp.Iterate
		out.Iterate = &v
		any = true
	}
	if cp.IterateCount != nil {
		v := *cp.IterateCount
		out.IterationCount = &v
		any = true
	}
	if cp.IterateDelta != nil {
		v := *cp.IterateDelta
		out.Delta = &v
		any = true
	}
	if cp.CalcOnSave != nil {
		v := *cp.CalcOnSave
		out.SaveRecalc = &v
		any = true
	}
	if cp.FullPrecision != nil {
		// fullPrecision="false" means "use displayed precision" — invert.
		v := !*cp.FullPrecision
		out.PrecisionAsDisplayed = &v
		any = true
	}
	if cp.FullCalcOnLoad != nil {
		v := *cp.FullCalcOnLoad
		out.FullCalcOnLoad = &v
		any = true
	}
	if cp.ForceFullCalc != nil {
		v := *cp.ForceFullCalc
		out.ForceFullCalc = &v
		any = true
	}
	if !any {
		return nil
	}
	return out
}

func resolveWorkbookConnection(conn parts.Connection) *WorkbookConnection {
	resolved := &WorkbookConnection{
		ID:            conn.ID,
		Name:          strings.TrimSpace(conn.Name),
		Kind:          resolveWorkbookConnectionKind(conn),
		Description:   strings.TrimSpace(conn.Description),
		RefreshOnLoad: conn.RefreshOnLoad,
		Background:    conn.Background,
		SaveData:      conn.SaveData,
		SourceFile:    strings.TrimSpace(conn.SourceFile),
		ODCFile:       strings.TrimSpace(conn.ODCFile),
	}
	if conn.DBPr != nil {
		resolved.Connection = strings.TrimSpace(conn.DBPr.Connection)
		resolved.Command = strings.TrimSpace(conn.DBPr.Command)
		resolved.CommandType = strings.TrimSpace(conn.DBPr.CommandType)
	}
	if conn.WebPr != nil {
		resolved.URL = strings.TrimSpace(conn.WebPr.URL)
		resolved.WebPost = strings.TrimSpace(conn.WebPr.Post)
		resolved.WebHTMLTables = conn.WebPr.HtmlTables
		resolved.WebSourceData = conn.WebPr.SourceData
		resolved.WebParsePre = conn.WebPr.ParsePre
		resolved.WebConsecutive = conn.WebPr.Consecutive
		resolved.WebFirstRow = conn.WebPr.FirstRow
		resolved.WebTextDates = conn.WebPr.TextDates
	}
	if conn.TextPr != nil {
		resolved.TextFile = strings.TrimSpace(conn.TextPr.SourceFile)
		resolved.TextDelimiter = strings.TrimSpace(conn.TextPr.Delimiter)
	}
	if conn.OLAPPr != nil {
		resolved.LocalCube = strings.TrimSpace(conn.OLAPPr.LocalConnection)
	}
	return resolved
}

func resolveWorkbookConnectionKind(conn parts.Connection) string {
	switch {
	case conn.WebPr != nil:
		return "web"
	case conn.TextPr != nil:
		return "text"
	case conn.DBPr != nil:
		return "database"
	case conn.OLAPPr != nil:
		return "olap"
	case strings.TrimSpace(conn.Type) != "":
		return strings.TrimSpace(conn.Type)
	default:
		return "unknown"
	}
}

func resolveWorkbookExternalLink(entry *parts.ExternalLinkEntry) *WorkbookExternalLink {
	resolved := &WorkbookExternalLink{Target: strings.TrimSpace(entry.Target)}
	if entry == nil || entry.Link == nil || entry.Link.ExternalBook == nil {
		return resolved
	}
	book := entry.Link.ExternalBook
	for _, sheet := range book.SheetNames {
		name := strings.TrimSpace(sheet.Val)
		if name != "" {
			resolved.SheetNames = append(resolved.SheetNames, name)
		}
	}
	for _, dn := range book.DefinedNames {
		resolved.DefinedNames = append(resolved.DefinedNames, &WorkbookExternalDefinedName{
			Name:     strings.TrimSpace(dn.Name),
			RefersTo: strings.TrimSpace(dn.RefersTo),
			SheetID:  dn.SheetID,
		})
	}
	if book.SheetDataSet != nil {
		for _, sheet := range book.SheetDataSet.Sheets {
			resolvedSheet := &WorkbookExternalSheet{
				ID:           sheet.SheetID,
				Name:         resolveExternalSheetName(book, sheet.SheetID),
				RefreshError: sheet.RefreshError,
			}
			for _, row := range sheet.Rows {
				for _, cell := range row.Cells {
					ref := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(cell.Ref), "$", ""))
					if ref == "" {
						continue
					}
					resolvedSheet.Cells = append(resolvedSheet.Cells, &WorkbookExternalCell{
						Ref:   ref,
						Type:  strings.TrimSpace(cell.Type),
						Value: strings.TrimSpace(cell.Value),
					})
				}
			}
			if resolvedSheet.Name != "" || len(resolvedSheet.Cells) > 0 {
				resolved.CachedSheets = append(resolved.CachedSheets, resolvedSheet)
			}
		}
	}
	return resolved
}

func resolveExternalSheetName(book *parts.ExternalBook, sheetID int) string {
	if book == nil {
		return ""
	}
	if sheetID >= 0 && sheetID < len(book.SheetNames) {
		return strings.TrimSpace(book.SheetNames[sheetID].Val)
	}
	if sheetID > 0 && sheetID-1 < len(book.SheetNames) {
		return strings.TrimSpace(book.SheetNames[sheetID-1].Val)
	}
	if len(book.SheetNames) == 1 {
		return strings.TrimSpace(book.SheetNames[0].Val)
	}
	return ""
}

func resolveDataValidationLists(wb *Workbook) {
	if wb == nil || len(wb.Sheets) == 0 {
		return
	}
	for sheetIndex, sheet := range wb.Sheets {
		if sheet == nil || len(sheet.DataValidations) == 0 {
			continue
		}
		cellValues := buildSheetCellValueMap(sheet)
		for _, dv := range sheet.DataValidations {
			if dv == nil || dv.Type != "list" || strings.TrimSpace(dv.Formula1) == "" {
				continue
			}
			dv.ListValues = resolveValidationListValues(wb, sheetIndex, cellValues, dv.Formula1)
		}
	}
}

func buildSheetCellValueMap(sheet *Sheet) map[string]string {
	if sheet == nil || len(sheet.Rows) == 0 {
		return nil
	}
	values := make(map[string]string)
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil || cell.Ref == "" {
				continue
			}
			text := resolvedCellText(cell)
			if text == "" {
				continue
			}
			values[strings.ToUpper(cell.Ref)] = text
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func resolvedCellText(cell *Cell) string {
	if cell == nil {
		return ""
	}
	switch cell.Kind {
	case CellString, CellError, CellFormula:
		if cell.StrValue != "" {
			return cell.StrValue
		}
	case CellNumber, CellBool:
		return strconv.FormatFloat(cell.NumValue, 'f', -1, 64)
	}
	if len(cell.RichTextRuns) > 0 {
		var b strings.Builder
		for _, run := range cell.RichTextRuns {
			b.WriteString(run.Text)
		}
		return b.String()
	}
	return ""
}

func resolveValidationListValues(wb *Workbook, sheetIndex int, cellValues map[string]string, formula string) []string {
	formula = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(formula), "="))
	if formula == "" {
		return nil
	}
	if values := parseValidationInlineList(formula); len(values) > 0 {
		return values
	}
	if resolved := resolveDefinedNameValue(wb, sheetIndex, formula); resolved != "" && !strings.EqualFold(resolved, formula) {
		formula = resolved
		if values := parseValidationInlineList(formula); len(values) > 0 {
			return values
		}
	}
	return resolveValidationRangeValues(wb, sheetIndex, cellValues, formula)
}

func parseValidationInlineList(formula string) []string {
	if len(formula) < 2 || formula[0] != '"' || formula[len(formula)-1] != '"' {
		return nil
	}
	raw := formula[1 : len(formula)-1]
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.ReplaceAll(part, `""`, `"`))
		if part == "" {
			continue
		}
		values = append(values, part)
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func resolveDefinedNameValue(wb *Workbook, sheetIndex int, name string) string {
	if wb == nil || len(wb.DefinedNames) == 0 || name == "" {
		return ""
	}
	for _, dn := range wb.DefinedNames {
		if dn == nil || !strings.EqualFold(dn.Name, name) {
			continue
		}
		if dn.LocalSheetID != nil && *dn.LocalSheetID != sheetIndex {
			continue
		}
		if value := strings.TrimSpace(dn.Value); value != "" {
			return value
		}
	}
	for _, dn := range wb.DefinedNames {
		if dn == nil || dn.LocalSheetID != nil || !strings.EqualFold(dn.Name, name) {
			continue
		}
		if value := strings.TrimSpace(dn.Value); value != "" {
			return value
		}
	}
	return ""
}

func resolveValidationRangeValues(wb *Workbook, currentSheetIndex int, currentSheetValues map[string]string, formula string) []string {
	sheetValues := currentSheetValues
	refExpr := strings.TrimSpace(formula)
	if bang := strings.LastIndex(refExpr, "!"); bang >= 0 {
		sheetName := normalizeSheetName(refExpr[:bang])
		refExpr = refExpr[bang+1:]
		if wb == nil {
			return nil
		}
		targetIndex := currentSheetIndex
		found := false
		for i, sheet := range wb.Sheets {
			if sheet != nil && strings.EqualFold(sheet.Name, sheetName) {
				targetIndex = i
				found = true
				break
			}
		}
		if !found {
			return nil
		}
		sheetValues = buildSheetCellValueMap(wb.Sheets[targetIndex])
	}
	refs := expandValidationListRefs(refExpr)
	if len(refs) == 0 || len(sheetValues) == 0 {
		return nil
	}
	values := make([]string, 0, len(refs))
	for _, ref := range refs {
		value := sheetValues[ref]
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func normalizeSheetName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		name = strings.ReplaceAll(name[1:len(name)-1], "''", "'")
	}
	return name
}

func expandValidationListRefs(expr string) []string {
	parts := strings.Fields(strings.ReplaceAll(expr, ",", " "))
	if len(parts) == 0 {
		return nil
	}
	var out []string
	for _, part := range parts {
		part = normalizeValidationRef(part)
		if part == "" {
			continue
		}
		if colon := strings.Index(part, ":"); colon >= 0 {
			startRef := part[:colon]
			endRef := part[colon+1:]
			sr, sc, ok1 := parseValidationA1(startRef)
			er, ec, ok2 := parseValidationA1(endRef)
			if !ok1 || !ok2 {
				continue
			}
			for row := sr; row <= er; row++ {
				for col := sc; col <= ec; col++ {
					out = append(out, colLettersFromIndex(col)+strconv.Itoa(row))
				}
			}
			continue
		}
		if _, _, ok := parseValidationA1(part); ok {
			out = append(out, strings.ToUpper(part))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeValidationRef(ref string) string {
	ref = strings.TrimSpace(strings.ToUpper(ref))
	ref = strings.ReplaceAll(ref, "$", "")
	return ref
}

func parseValidationA1(ref string) (row, col int, ok bool) {
	ref = normalizeValidationRef(ref)
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		i++
	}
	if i == 0 || i >= len(ref) {
		return 0, 0, false
	}
	r, err := strconv.Atoi(ref[i:])
	if err != nil || r < 1 {
		return 0, 0, false
	}
	c := 0
	for _, ch := range ref[:i] {
		c = c*26 + int(ch-'A') + 1
	}
	return r, c, true
}

type printTitles struct {
	rows string
	cols string
}

func buildSheetVisibilityByID(wb *parts.Workbook) map[int]string {
	if wb == nil || len(wb.Sheets) == 0 {
		return nil
	}
	out := make(map[int]string)
	for _, sheet := range wb.Sheets {
		switch strings.TrimSpace(sheet.State) {
		case "hidden", "veryHidden":
			out[sheet.SheetID] = strings.TrimSpace(sheet.State)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildPrintAreaMap(wb *parts.Workbook) map[int]string {
	if wb == nil || len(wb.DefinedNames) == 0 || len(wb.Sheets) == 0 {
		return nil
	}
	out := make(map[int]string)
	for _, dn := range wb.DefinedNames {
		if dn.Name != "_xlnm.Print_Area" || dn.LocalSheetID == nil {
			continue
		}
		localSheetID := *dn.LocalSheetID
		if localSheetID < 0 || localSheetID >= len(wb.Sheets) {
			continue
		}
		parsed := parsePrintArea(dn.Value)
		if parsed == "" {
			continue
		}
		sheetID := wb.Sheets[localSheetID].SheetID
		out[sheetID] = parsed
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildPrintTitleMap(wb *parts.Workbook) map[int]printTitles {
	if wb == nil || len(wb.DefinedNames) == 0 || len(wb.Sheets) == 0 {
		return nil
	}
	out := make(map[int]printTitles)
	for _, dn := range wb.DefinedNames {
		if dn.Name != "_xlnm.Print_Titles" || dn.LocalSheetID == nil {
			continue
		}
		localSheetID := *dn.LocalSheetID
		if localSheetID < 0 || localSheetID >= len(wb.Sheets) {
			continue
		}
		parsed := parsePrintTitles(dn.Value)
		if parsed.rows == "" && parsed.cols == "" {
			continue
		}
		sheetID := wb.Sheets[localSheetID].SheetID
		existing := out[sheetID]
		if parsed.rows != "" {
			existing.rows = parsed.rows
		}
		if parsed.cols != "" {
			existing.cols = parsed.cols
		}
		out[sheetID] = existing
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parsePrintArea(value string) string {
	parts := splitDefinedNameLocalRanges(value)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ",")
}

func parsePrintTitles(value string) printTitles {
	var out printTitles
	for _, rangePart := range splitDefinedNameLocalRanges(value) {
		if strings.Contains(rangePart, "$0") {
			continue
		}
		if isPrintTitleRowRange(rangePart) {
			out.rows = rangePart
			continue
		}
		if isPrintTitleColRange(rangePart) {
			out.cols = rangePart
		}
	}
	return out
}

func splitDefinedNameLocalRanges(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		bang := strings.LastIndex(part, "!")
		if bang < 0 || bang == len(part)-1 {
			continue
		}
		rangePart := strings.TrimSpace(part[bang+1:])
		if rangePart == "" {
			continue
		}
		out = append(out, rangePart)
	}
	return out
}

func isPrintTitleRowRange(v string) bool {
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if len(part) < 2 || part[0] != '$' {
			return false
		}
		for _, r := range part[1:] {
			if !unicode.IsDigit(r) {
				return false
			}
		}
	}
	return true
}

func isPrintTitleColRange(v string) bool {
	parts := strings.Split(v, ":")
	if len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if len(part) < 2 || part[0] != '$' {
			return false
		}
		for _, r := range part[1:] {
			if !unicode.IsLetter(r) {
				return false
			}
		}
	}
	return true
}

// buildSST returns a flat slice of plain-text shared strings indexed by position.
func buildSST(ss *parts.SharedStrings) []string {
	if ss == nil {
		return nil
	}
	out := make([]string, len(ss.Items))
	for i, item := range ss.Items {
		out[i] = item.PlainValue()
	}
	return out
}

// buildSSTRich returns the full rich-text items from the shared string table.
// Entries with no runs return nil; entries with runs return the original RichText.
func buildSSTRich(ss *parts.SharedStrings) []*parts.RichText {
	if ss == nil {
		return nil
	}
	out := make([]*parts.RichText, len(ss.Items))
	for i := range ss.Items {
		if len(ss.Items[i].Runs) > 0 {
			item := ss.Items[i]
			out[i] = &item
		}
	}
	return out
}

// styleEntry holds pre-resolved cell style properties indexed by cellXfs position.
type styleEntry struct {
	bold               bool
	italic             bool
	underline          bool
	underlineType      string
	strike             bool
	fontVertAlign      string
	fontFamily         string
	hAlign             string
	vAlign             string
	wrapText           bool
	shrinkToFit        bool
	indent             int
	textRotation       int
	fontSizePt         float64
	fontColor          string
	bgColor            string
	fillPattern        string
	fillPatternBgColor string
	borderTop          BorderStyle
	borderRight        BorderStyle
	borderBottom       BorderStyle
	borderLeft         BorderStyle
	borderDiagonal     BorderStyle
	borderDiagonalUp   bool
	borderDiagonalDown bool
	borderVertical     BorderStyle
	borderHorizontal   BorderStyle
	numFmtCode         string
	locked             bool
	formulaHidden      bool
}

// ResolveCellStyleTable exposes the canonical styles.xml resolution used by the
// OOXML reader. The streaming parser owns style indices; this resolver converts
// those indices to the canonical IR CellStyle representation.
func ResolveCellStyleTable(ss *parts.StyleSheet, theme *parts.Theme) []CellStyle {
	themeColors := buildThemeColorIndex(theme)
	indexedColors := buildIndexedColorIndex(ss)
	entries := buildStyleTable(ss, themeColors, indexedColors)
	out := make([]CellStyle, len(entries))
	for i, entry := range entries {
		out[i] = styleEntryToCellStyle(entry)
	}
	return out
}

func buildStyleTable(ss *parts.StyleSheet, themeColors, indexedColors map[int]string) []styleEntry {
	if ss == nil {
		return nil
	}

	base := buildStyleEntries(ss, ss.CellStyleXfs, themeColors, indexedColors)
	direct := buildStyleEntries(ss, ss.CellXfs, themeColors, indexedColors)
	out := make([]styleEntry, len(direct))
	for i, entry := range direct {
		xf := ss.CellXfs[i]
		if xf.XfID >= 0 && xf.XfID < len(base) {
			out[i] = inheritCellXf(base[xf.XfID], entry, xf)
		} else {
			out[i] = entry
		}
	}
	return out
}

// inheritCellXf applies a cellXf on top of its referenced cellStyleXf. Formatting
// components are groups in OOXML: when a group is applied, the cellXf replaces
// that whole group, including false/empty values. Non-zero component IDs remain
// an interoperability fallback for producers that omit the corresponding apply*
// attribute while still authoring a direct format.
func inheritCellXf(base, direct styleEntry, xf parts.Xf) styleEntry {
	out := base

	if xf.ApplyFont || xf.FontID != 0 {
		out.bold = direct.bold
		out.italic = direct.italic
		out.underline = direct.underline
		out.underlineType = direct.underlineType
		out.strike = direct.strike
		out.fontVertAlign = direct.fontVertAlign
		out.fontFamily = direct.fontFamily
		out.fontSizePt = direct.fontSizePt
		out.fontColor = direct.fontColor
	}
	if xf.ApplyFill || xf.FillID != 0 {
		out.bgColor = direct.bgColor
		out.fillPattern = direct.fillPattern
		out.fillPatternBgColor = direct.fillPatternBgColor
	}
	if xf.ApplyBorder || xf.BorderID != 0 {
		out.borderTop = direct.borderTop
		out.borderRight = direct.borderRight
		out.borderBottom = direct.borderBottom
		out.borderLeft = direct.borderLeft
		out.borderDiagonal = direct.borderDiagonal
		out.borderDiagonalUp = direct.borderDiagonalUp
		out.borderDiagonalDown = direct.borderDiagonalDown
		out.borderVertical = direct.borderVertical
		out.borderHorizontal = direct.borderHorizontal
	}
	if xf.ApplyNumberFormat || xf.NumFmtID != 0 {
		out.numFmtCode = direct.numFmtCode
	}
	if xf.ApplyAlignment || xf.Alignment != nil {
		out.hAlign = direct.hAlign
		out.vAlign = direct.vAlign
		out.wrapText = direct.wrapText
		out.shrinkToFit = direct.shrinkToFit
		out.indent = direct.indent
		out.textRotation = direct.textRotation
	}
	if xf.ApplyProtection || xf.Protection != nil {
		out.locked = direct.locked
		out.formulaHidden = direct.formulaHidden
	}
	return out
}

func buildNamedCellStyles(ss *parts.StyleSheet, themeColors, indexedColors map[int]string) ([]CellStyle, []*NamedCellStyle) {
	if ss == nil {
		return nil, nil
	}
	styleXFs := buildStyleEntries(ss, ss.CellStyleXfs, themeColors, indexedColors)
	resolvedXFs := make([]CellStyle, len(styleXFs))
	for i, entry := range styleXFs {
		resolvedXFs[i] = styleEntryToCellStyle(entry)
	}
	out := make([]*NamedCellStyle, 0, len(ss.CellStyles))
	for i := range ss.CellStyles {
		style := ss.CellStyles[i]
		named := &NamedCellStyle{
			Name:         strings.TrimSpace(style.Name),
			StyleXFIndex: style.XfID,
		}
		if style.BuiltinID != 0 || strings.EqualFold(named.Name, "Normal") {
			id := style.BuiltinID
			named.BuiltinID = &id
		}
		if style.XfID >= 0 && style.XfID < len(resolvedXFs) {
			named.Style = resolvedXFs[style.XfID]
		}
		out = append(out, named)
	}
	return resolvedXFs, out
}

func buildStyleEntries(ss *parts.StyleSheet, xfs []parts.Xf, themeColors, indexedColors map[int]string) []styleEntry {
	if ss == nil {
		return nil
	}

	// Build numFmt lookup: id -> format code (includes built-in formats).
	numFmts := builtinNumFmts()
	for _, nf := range ss.NumFmts {
		numFmts[nf.ID] = nf.FormatCode
	}

	out := make([]styleEntry, len(xfs))
	for i, xf := range xfs {
		var e styleEntry

		if xf.FontID >= 0 && xf.FontID < len(ss.Fonts) {
			f := ss.Fonts[xf.FontID]
			e.bold = f.Bold != nil
			e.italic = f.Italic != nil
			e.underline = f.Underline != nil
			if f.Underline != nil {
				e.underlineType = strings.TrimSpace(f.Underline.Val)
			}
			e.strike = f.Strike != nil
			if f.VertAlign != nil {
				e.fontVertAlign = strings.TrimSpace(f.VertAlign.Val)
			}
			if f.Name != nil {
				e.fontFamily = f.Name.Val
			}
			if f.Size != nil {
				e.fontSizePt = f.Size.Val
			}
			if f.Color != nil {
				e.fontColor = resolveColorVal(f.Color, themeColors, indexedColors)
			}
		}

		if xf.FillID >= 2 && xf.FillID < len(ss.Fills) {
			fill := ss.Fills[xf.FillID]
			if fill.PatternFill != nil {
				pattern := strings.TrimSpace(fill.PatternFill.PatternType)
				if pattern != "" && pattern != "none" {
					e.fillPattern = pattern
					e.bgColor = resolveColorVal(fill.PatternFill.FgColor, themeColors, indexedColors)
					e.fillPatternBgColor = resolveColorVal(fill.PatternFill.BgColor, themeColors, indexedColors)
				}
			}
		}
		if xf.BorderID >= 0 && xf.BorderID < len(ss.Borders) {
			border := ss.Borders[xf.BorderID]
			e.borderTop = resolveBorderSide(border.Top, themeColors, indexedColors)
			e.borderRight = resolveBorderSide(border.Right, themeColors, indexedColors)
			e.borderBottom = resolveBorderSide(border.Bottom, themeColors, indexedColors)
			e.borderLeft = resolveBorderSide(border.Left, themeColors, indexedColors)
			e.borderDiagonal = resolveBorderSide(border.Diagonal, themeColors, indexedColors)
			e.borderDiagonalUp = border.DiagonalUp
			e.borderDiagonalDown = border.DiagonalDown
			e.borderVertical = resolveBorderSide(border.Vertical, themeColors, indexedColors)
			e.borderHorizontal = resolveBorderSide(border.Horizontal, themeColors, indexedColors)
		}

		if xf.Alignment != nil {
			e.wrapText = xf.Alignment.WrapText
			e.shrinkToFit = xf.Alignment.ShrinkToFit
			e.indent = xf.Alignment.Indent
			e.textRotation = xf.Alignment.TextRotation
			switch xf.Alignment.Horizontal {
			case "left", "center", "right", "general", "fill", "justify", "centerContinuous", "distributed":
				e.hAlign = mapHAlign(xf.Alignment.Horizontal)
			}
			switch xf.Alignment.Vertical {
			case "top":
				e.vAlign = "top"
			case "center":
				e.vAlign = "middle"
			case "bottom":
				e.vAlign = "bottom"
			}
		}

		if code, ok := numFmts[xf.NumFmtID]; ok && xf.NumFmtID != 0 {
			e.numFmtCode = code
		}
		if xf.Protection != nil {
			e.locked = xf.Protection.Locked
			e.formulaHidden = xf.Protection.FormulaHidden
		}

		out[i] = e
	}
	return out
}

func styleEntryToCellStyle(e styleEntry) CellStyle {
	return CellStyle{
		Bold:               e.bold,
		Italic:             e.italic,
		Underline:          e.underline,
		UnderlineType:      e.underlineType,
		Strike:             e.strike,
		FontVertAlign:      e.fontVertAlign,
		FontFamily:         e.fontFamily,
		HAlign:             e.hAlign,
		VAlign:             e.vAlign,
		WrapText:           e.wrapText,
		ShrinkToFit:        e.shrinkToFit,
		Indent:             e.indent,
		TextRotation:       e.textRotation,
		FontSizePt:         e.fontSizePt,
		FontColor:          e.fontColor,
		BgColor:            e.bgColor,
		FillPattern:        e.fillPattern,
		FillPatternBgColor: e.fillPatternBgColor,
		BorderTop:          e.borderTop,
		BorderRight:        e.borderRight,
		BorderBottom:       e.borderBottom,
		BorderLeft:         e.borderLeft,
		BorderDiagonal:     e.borderDiagonal,
		BorderDiagonalUp:   e.borderDiagonalUp,
		BorderDiagonalDown: e.borderDiagonalDown,
		BorderVertical:     e.borderVertical,
		BorderHorizontal:   e.borderHorizontal,
		NumFmtCode:         e.numFmtCode,
		Locked:             e.locked,
		FormulaHidden:      e.formulaHidden,
	}
}

// buildDxfTable resolves the differential format (dxf) list from the stylesheet
// into DxfStyle values indexed by dxfId.
func buildDxfTable(ss *parts.StyleSheet, themeColors, indexedColors map[int]string) []DxfStyle {
	if ss == nil {
		return nil
	}
	out := make([]DxfStyle, len(ss.Dxfs))
	for i, dxf := range ss.Dxfs {
		var ds DxfStyle
		if dxf.Font != nil {
			if dxf.Font.Bold != nil {
				b := true
				ds.Bold = &b
			}
			if dxf.Font.Italic != nil {
				v := true
				ds.Italic = &v
			}
			if dxf.Font.Underline != nil {
				v := true
				ds.Underline = &v
			}
			if dxf.Font.Strike != nil {
				v := true
				ds.Strike = &v
			}
			if dxf.Font.Color != nil {
				ds.FontColor = resolveColorVal(dxf.Font.Color, themeColors, indexedColors)
			}
		}
		if dxf.NumFmt != nil {
			ds.NumFmtCode = dxf.NumFmt.FormatCode
			if ds.NumFmtCode == "" {
				ds.NumFmtCode = builtinNumFmts()[dxf.NumFmt.ID]
			}
		}
		if dxf.Fill != nil && dxf.Fill.PatternFill != nil {
			pf := dxf.Fill.PatternFill
			fg := resolveColorVal(pf.FgColor, themeColors, indexedColors)
			bg := resolveColorVal(pf.BgColor, themeColors, indexedColors)
			// A differential solid fill paints its bgColor (fgColor only as a
			// fallback); patterned fills keep fgColor as the pattern colour.
			if pf.PatternType == "" || pf.PatternType == "solid" {
				ds.BgColor = bg
				if ds.BgColor == "" {
					ds.BgColor = fg
				}
			} else {
				ds.BgColor = fg
				ds.FillForegroundColor = fg
				ds.FillBackgroundColor = bg
			}
		}
		if dxf.Border != nil {
			setDxfBorderSide(&ds.BorderLeft, &ds.LeftBorderColor, dxf.Border.Left, themeColors, indexedColors)
			setDxfBorderSide(&ds.BorderRight, &ds.RightBorderColor, dxf.Border.Right, themeColors, indexedColors)
			setDxfBorderSide(&ds.BorderTop, &ds.TopBorderColor, dxf.Border.Top, themeColors, indexedColors)
			setDxfBorderSide(&ds.BorderBottom, &ds.BottomBorderColor, dxf.Border.Bottom, themeColors, indexedColors)
		}
		out[i] = ds
	}
	return out
}

func setDxfBorderSide(dst **int, color *string, side *parts.BorderSide, themeColors, indexedColors map[int]string) {
	if side == nil || side.Style == "" {
		return
	}
	if code, ok := dxfBorderStyleCode(side.Style); ok {
		v := code
		*dst = &v
	}
	if side.Color != nil {
		*color = resolveColorVal(side.Color, themeColors, indexedColors)
	}
}

func dxfBorderStyleCode(style string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "none":
		return 0, true
	case "thin":
		return 1, true
	case "medium":
		return 2, true
	case "dashed":
		return 3, true
	case "hair":
		return 4, true
	case "thick":
		return 5, true
	case "double":
		return 6, true
	case "dotted":
		return 7, true
	case "mediumdashed":
		return 8, true
	case "dashdot":
		return 9, true
	case "mediumdashdot":
		return 10, true
	case "dashdotdot":
		return 11, true
	case "mediumdashdotdot":
		return 12, true
	case "slantdashdot":
		return 13, true
	default:
		return 0, false
	}
}

func mapHAlign(v string) string {
	switch v {
	case "left", "center", "right", "fill", "justify", "centerContinuous", "distributed":
		return v
	default:
		return ""
	}
}

func resolveColorVal(c *parts.ColorVal, themeColors, indexedColors map[int]string) string {
	if c == nil {
		return ""
	}
	base := ""
	if c.RGB != "" {
		base = normalizeSpreadsheetHex(c.RGB)
	} else if c.Theme != nil {
		if hex, ok := themeColors[*c.Theme]; ok {
			base = hex
		}
	} else if c.Indexed != nil {
		base = resolveIndexedColorHex(*c.Indexed, indexedColors)
	}
	return applySpreadsheetTintToHex(base, c.Tint)
}

func resolveBorderSide(side *parts.BorderSide, themeColors, indexedColors map[int]string) BorderStyle {
	if side == nil || side.Style == "" {
		return BorderStyle{}
	}
	return BorderStyle{
		Style: side.Style,
		Color: resolveColorVal(side.Color, themeColors, indexedColors),
	}
}

func buildIndexedColorIndex(ss *parts.StyleSheet) map[int]string {
	colors := make(map[int]string, len(defaultSpreadsheetIndexedColors))
	for idx, hex := range defaultSpreadsheetIndexedColors {
		if hex != "" {
			colors[idx] = hex
		}
	}
	if ss == nil || ss.Colors == nil || ss.Colors.IndexedColors == nil {
		return colors
	}
	for idx, color := range ss.Colors.IndexedColors.RGBColors {
		if hex := normalizeSpreadsheetHex(color.RGB); hex != "" {
			colors[idx] = hex
		}
	}
	return colors
}

func resolveIndexedColorHex(index int, indexedColors map[int]string) string {
	if indexedColors != nil {
		if hex, ok := indexedColors[index]; ok {
			return hex
		}
	}
	if index < 0 || index >= len(defaultSpreadsheetIndexedColors) {
		return ""
	}
	return defaultSpreadsheetIndexedColors[index]
}

func normalizeSpreadsheetHex(hex string) string {
	hex = strings.ToUpper(strings.TrimSpace(hex))
	if len(hex) == 8 {
		return hex[2:]
	}
	return hex
}

var defaultSpreadsheetIndexedColors = [...]string{
	"000000", "FFFFFF", "FF0000", "00FF00", "0000FF", "FFFF00", "FF00FF", "00FFFF",
	"000000", "FFFFFF", "FF0000", "00FF00", "0000FF", "FFFF00", "FF00FF", "00FFFF",
	"800000", "008000", "000080", "808000", "800080", "008080", "C0C0C0", "808080",
	"9999FF", "993366", "FFFFCC", "CCFFFF", "660066", "FF8080", "0066CC", "CCCCFF",
	"000080", "FF00FF", "FFFF00", "00FFFF", "800080", "800000", "008080", "0000FF",
	"00CCFF", "CCFFFF", "CCFFCC", "FFFF99", "99CCFF", "FF99CC", "CC99FF", "FFCC99",
	"3366FF", "33CCCC", "99CC00", "FFCC00", "FF9900", "FF6600", "666699", "969696",
	"003366", "339966", "003300", "333300", "993300", "993366", "333399", "333333",
	"",
}

func buildThemeColorIndex(theme *parts.Theme) map[int]string {
	colors := map[int]string{}
	if theme == nil {
		return colors
	}
	scheme := theme.Elements.ClrScheme
	add := func(idx int, color *parts.ThemeColor) {
		if hex := resolveThemeColorHex(color); hex != "" {
			colors[idx] = hex
		}
	}
	// SpreadsheetML theme indexes follow the workbook theme color scheme order.
	add(0, scheme.LT1)
	add(1, scheme.DK1)
	add(2, scheme.LT2)
	add(3, scheme.DK2)
	add(4, scheme.Accent1)
	add(5, scheme.Accent2)
	add(6, scheme.Accent3)
	add(7, scheme.Accent4)
	add(8, scheme.Accent5)
	add(9, scheme.Accent6)
	add(10, scheme.HLink)
	add(11, scheme.FolHLink)
	return colors
}

func buildDrawingThemeColorIndex(theme *parts.Theme) map[string]string {
	colors := map[string]string{}
	if theme == nil {
		return colors
	}
	scheme := theme.Elements.ClrScheme
	add := func(name string, color *parts.ThemeColor) {
		if hex := resolveThemeColorHex(color); hex != "" {
			colors[name] = hex
		}
	}
	add("lt1", scheme.LT1)
	add("dk1", scheme.DK1)
	add("lt2", scheme.LT2)
	add("dk2", scheme.DK2)
	add("accent1", scheme.Accent1)
	add("accent2", scheme.Accent2)
	add("accent3", scheme.Accent3)
	add("accent4", scheme.Accent4)
	add("accent5", scheme.Accent5)
	add("accent6", scheme.Accent6)
	add("hlink", scheme.HLink)
	add("folHlink", scheme.FolHLink)
	// SpreadsheetML has no per-sheet clrMap, so shapes use the default mapping.
	for alias, name := range map[string]string{"tx1": "dk1", "bg1": "lt1", "tx2": "dk2", "bg2": "lt2"} {
		if hex, ok := colors[name]; ok {
			colors[alias] = hex
		}
	}
	return colors
}

func resolveThemeColorHex(color *parts.ThemeColor) string {
	if color == nil {
		return ""
	}
	switch {
	case color.SrgbClr != nil && color.SrgbClr.Val != "":
		return applyThemeColorTransforms(color.SrgbClr.Val, &color.SrgbClr.ThemeColorTransforms)
	case color.SysClr != nil && color.SysClr.LastClr != "":
		return applyThemeColorTransforms(color.SysClr.LastClr, &color.SysClr.ThemeColorTransforms)
	default:
		return ""
	}
}

func applyThemeColorTransforms(base string, transforms *parts.ThemeColorTransforms) string {
	base = strings.ToUpper(strings.TrimSpace(base))
	if base == "" || transforms == nil {
		return base
	}
	fill := &drawing.SolidFill{}
	if transforms.Alpha != nil {
		fill.Alpha = float64(transforms.Alpha.Val) / 100000.0
	}
	if transforms.AlphaMod != nil {
		fill.AlphaMod = float64(transforms.AlphaMod.Val) / 100000.0
	}
	if transforms.AlphaOff != nil {
		fill.AlphaOff = float64(transforms.AlphaOff.Val) / 100000.0
	}
	if transforms.Shade != nil {
		fill.Shade = float64(transforms.Shade.Val) / 100000.0
		fill.HasShade = true
	}
	if transforms.Tint != nil {
		fill.Tint = float64(transforms.Tint.Val) / 100000.0
		fill.HasTint = true
	}
	if transforms.Red != nil {
		fill.Red = float64(transforms.Red.Val) / 100000.0
		fill.HasRed = true
	}
	if transforms.RedMod != nil {
		fill.RedMod = float64(transforms.RedMod.Val) / 100000.0
		fill.HasRedMod = true
	}
	if transforms.RedOff != nil {
		fill.RedOff = float64(transforms.RedOff.Val) / 100000.0
		fill.HasRedOff = true
	}
	if transforms.Green != nil {
		fill.Green = float64(transforms.Green.Val) / 100000.0
		fill.HasGreen = true
	}
	if transforms.GreenMod != nil {
		fill.GreenMod = float64(transforms.GreenMod.Val) / 100000.0
		fill.HasGreenMod = true
	}
	if transforms.GreenOff != nil {
		fill.GreenOff = float64(transforms.GreenOff.Val) / 100000.0
		fill.HasGreenOff = true
	}
	if transforms.Blue != nil {
		fill.Blue = float64(transforms.Blue.Val) / 100000.0
		fill.HasBlue = true
	}
	if transforms.BlueMod != nil {
		fill.BlueMod = float64(transforms.BlueMod.Val) / 100000.0
		fill.HasBlueMod = true
	}
	if transforms.BlueOff != nil {
		fill.BlueOff = float64(transforms.BlueOff.Val) / 100000.0
		fill.HasBlueOff = true
	}
	if transforms.Hue != nil {
		fill.Hue = float64(transforms.Hue.Val) / 60000.0
		fill.HasHue = true
	}
	if transforms.Sat != nil {
		fill.Sat = float64(transforms.Sat.Val) / 100000.0
		fill.HasSat = true
	}
	if transforms.Lum != nil {
		fill.Lum = float64(transforms.Lum.Val) / 100000.0
		fill.HasLum = true
	}
	if transforms.LumMod != nil {
		fill.LumMod = float64(transforms.LumMod.Val) / 100000.0
	}
	if transforms.LumOff != nil {
		fill.LumOff = float64(transforms.LumOff.Val) / 100000.0
	}
	if transforms.SatMod != nil {
		fill.SatMod = float64(transforms.SatMod.Val) / 100000.0
	}
	if transforms.SatOff != nil {
		fill.SatOff = float64(transforms.SatOff.Val) / 100000.0
	}
	if transforms.HueMod != nil {
		fill.HueMod = float64(transforms.HueMod.Val) / 60000.0
	}
	if transforms.HueOff != nil {
		fill.HueOff = float64(transforms.HueOff.Val) / 60000.0
	}
	fill.Gray = transforms.Gray != nil
	fill.Comp = transforms.Comp != nil
	fill.Inv = transforms.Inv != nil
	fill.Gamma = transforms.Gamma != nil
	fill.InvGamma = transforms.InvGamma != nil
	return fill.ApplyColorTransforms(base)
}

func applySpreadsheetTintToHex(rgb string, tint float64) string {
	rgb = strings.ToUpper(strings.TrimSpace(rgb))
	if rgb == "" {
		return ""
	}
	if len(rgb) < 6 {
		return rgb
	}
	if tint == 0 {
		return rgb
	}
	if tint > 1 {
		tint = 1
	} else if tint < -1 {
		tint = -1
	}
	r, err1 := strconv.ParseInt(rgb[0:2], 16, 64)
	g, err2 := strconv.ParseInt(rgb[2:4], 16, 64)
	b, err3 := strconv.ParseInt(rgb[4:6], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return rgb
	}
	return fmt.Sprintf("%02X%02X%02X", applySpreadsheetTintChannel(r, tint), applySpreadsheetTintChannel(g, tint), applySpreadsheetTintChannel(b, tint))
}

func applySpreadsheetTintChannel(channel int64, tint float64) int64 {
	switch {
	case tint > 0:
		channel = int64(float64(channel)*(1.0-tint) + (255.0 - 255.0*(1.0-tint)))
	case tint < 0:
		channel = int64(float64(channel) * (1.0 + tint))
	}
	if channel < 0 {
		return 0
	}
	if channel > 255 {
		return 255
	}
	return channel
}

// builtinNumFmts returns the ECMA-376 §18.8.30 built-in number format codes.
func builtinNumFmts() map[int]string {
	return map[int]string{
		1:  "0",
		2:  "0.00",
		3:  "#,##0",
		4:  "#,##0.00",
		9:  "0%",
		10: "0.00%",
		11: "0.00E+00",
		12: "# ?/?",
		13: "# ??/??",
		14: "m/d/yyyy",
		15: "d-mmm-yy",
		16: "d-mmm",
		17: "mmm-yy",
		18: "h:mm AM/PM",
		19: "h:mm:ss AM/PM",
		20: "h:mm",
		21: "h:mm:ss",
		22: "m/d/yyyy h:mm",
		37: "#,##0 ;(#,##0)",
		38: "#,##0 ;[Red](#,##0)",
		39: "#,##0.00;(#,##0.00)",
		40: "#,##0.00;[Red](#,##0.00)",
		45: "mm:ss",
		46: "[h]:mm:ss",
		47: "mmss.0",
		48: "##0.0E+0",
		49: "@",
	}
}

func resolveWorksheet(entry *parts.WorksheetEntry, sst []string, sstRich []*parts.RichText, styles []styleEntry, dxfs []DxfStyle, themeColors, indexedColors map[int]string, chartParts map[string][]byte, drawingThemeColors map[string]string) *Sheet {
	sheet := &Sheet{
		Name:         entry.Name,
		ID:           entry.SheetID,
		ColumnWidths: make(map[int]float64),
	}
	if entry.Sheet == nil {
		return sheet
	}
	ws := entry.Sheet

	if ws.SheetFormatPr != nil {
		sheet.DefaultColWidthChars = ws.SheetFormatPr.DefaultColWidth
		sheet.DefaultRowHeightPt = ptFromRowHeight(ws.SheetFormatPr.DefaultRowHeight)
		sheet.DefaultRowHeightUnsynced = ws.SheetFormatPr.CustomHeight
		sheet.DefaultRowHeightZeroHeight = ws.SheetFormatPr.ZeroHeight
		sheet.RowOutlineLevelMax = ws.SheetFormatPr.OutlineLevelRow
		sheet.ColOutlineLevelMax = ws.SheetFormatPr.OutlineLevelCol
	}

	for _, col := range ws.Cols {
		// Width and visibility are independent worksheet properties. A hidden
		// column can still carry an authored width which must survive resolution
		// so unhiding/round-tripping and geometry calculations retain the source.
		if col.Width > 0 {
			for c := col.Min; c <= col.Max; c++ {
				sheet.ColumnWidths[c] = col.Width
			}
		}
		if col.Hidden {
			if sheet.HiddenCols == nil {
				sheet.HiddenCols = make(map[int]bool)
			}
			for c := col.Min; c <= col.Max; c++ {
				sheet.HiddenCols[c] = true
			}
		}
	}

	if len(ws.SheetViews) > 0 {
		sv := ws.SheetViews[0]
		sheet.ZoomScale = sv.ZoomScale
		sheet.ShowGridLines = sv.ShowGridLines
		if sv.Pane != nil {
			sheet.PaneState = sv.Pane.State
			sheet.SplitX = sv.Pane.XSplit
			sheet.SplitY = sv.Pane.YSplit
			switch sv.Pane.State {
			case "frozen", "frozenSplit":
				sheet.FrozenRows = int(sv.Pane.YSplit)
				sheet.FrozenCols = int(sv.Pane.XSplit)
			case "split":
				sheet.SplitRows = int(sv.Pane.YSplit)
				sheet.SplitCols = int(sv.Pane.XSplit)
			}
			sheet.TopLeftCell = sv.Pane.TopLeftCell
			sheet.ActivePane = sv.Pane.ActivePane
		}
		for _, sel := range sv.Selections {
			sheet.PaneSelections = append(sheet.PaneSelections, PaneSelection{
				Pane: sel.Pane, ActiveCell: sel.ActiveCell, ActiveCellID: sel.ActiveCellID, SqRef: sel.SqRef,
			})
		}
	}

	for _, merge := range ws.MergeCells {
		if merged, ok := parseMergeRef(merge.Ref); ok {
			sheet.MergedCells = append(sheet.MergedCells, merged)
		}
	}

	// Dimension (used cell range)
	if ws.Dimension != nil && ws.Dimension.Ref != "" {
		if firstRow, lastRow, firstCol, lastCol, ok := parseA1Range(ws.Dimension.Ref); ok {
			sheet.DimFirstRow = firstRow
			sheet.DimLastRow = lastRow
			sheet.DimFirstCol = firstCol
			sheet.DimLastCol = lastCol
		}
	}

	// AutoFilter
	if ws.AutoFilter != nil {
		sheet.AutoFilterRef = ws.AutoFilter.Ref
		sheet.FilterMode = true
		for _, fc := range ws.AutoFilter.FilterColumns {
			ifc := resolveFilterColumn(&fc)
			if ifc != nil {
				sheet.FilterColumns = append(sheet.FilterColumns, ifc)
			}
		}
		sheet.SortState = resolveSortState(ws.AutoFilter.SortState)
	}

	// Data validations
	for i := range ws.DataValidations {
		dv := &ws.DataValidations[i]
		sheet.DataValidations = append(sheet.DataValidations, &DataValidation{
			SqRef:            dv.SqRef,
			Type:             dv.Type,
			Operator:         dv.Operator,
			ErrorStyle:       dv.ErrorStyle,
			IMEMode:          dv.IMEMode,
			AllowBlank:       dv.AllowBlank,
			ShowDropDown:     dv.ShowDropDown,
			ShowInputMessage: dv.ShowInputMessage,
			ShowErrorMessage: dv.ShowErrorMessage,
			PromptTitle:      dv.PromptTitle,
			Prompt:           dv.Prompt,
			ErrorTitle:       dv.ErrorTitle,
			Error:            dv.Error,
			Formula1:         dv.Formula1,
			Formula2:         dv.Formula2,
		})
	}

	// Conditional formatting rules
	for i := range ws.ConditionalFmts {
		cf := &ws.ConditionalFmts[i]
		for j := range cf.Rules {
			rule := &cf.Rules[j]
			cr := &CFRule{
				SqRef:      cf.SqRef,
				Type:       rule.Type,
				Operator:   rule.Operator,
				Priority:   rule.Priority,
				Percent:    rule.Percent,
				Bottom:     rule.Bottom,
				Rank:       rule.Rank,
				Formula:    rule.Formula,
				Text:       rule.Text,
				TimePeriod: rule.TimePeriod,
			}
			if rule.DxfID >= 0 && rule.DxfID < len(dxfs) {
				cr.DxfStyle = dxfs[rule.DxfID]
			}

			// Copy visual formatting for data bars, icon sets, and color scales
			if rule.DataBar != nil {
				cr.DataBar = &CFDataBar{
					MinLength: rule.DataBar.MinLength,
					MaxLength: rule.DataBar.MaxLength,
					ShowValue: rule.DataBar.ShowValue,
					Color:     resolveColorVal(rule.DataBar.Color, themeColors, indexedColors),
					Cfvo:      copyCfvo(rule.DataBar.Cfvo),
				}
			}
			if rule.IconSet != nil {
				cr.IconSet = &CFIconSet{
					Set:       rule.IconSet.Set,
					ShowValue: rule.IconSet.ShowValue,
					Reverse:   rule.IconSet.Reverse,
					Cfvo:      copyCfvo(rule.IconSet.Cfvo),
				}
			}
			if rule.ColorScale != nil {
				colors := make([]string, len(rule.ColorScale.Colors))
				for k, c := range rule.ColorScale.Colors {
					colors[k] = resolveColorVal(c.ColorVal(), themeColors, indexedColors)
				}
				cr.ColorScale = &CFColorScale{
					Cfvo:   copyCfvo(rule.ColorScale.Cfvo),
					Colors: colors,
				}
			}

			sheet.CFRules = append(sheet.CFRules, cr)
		}
	}

	hyperlinks := buildHyperlinkMap(ws.Hyperlinks, entry.HyperlinkURLs)
	for _, hl := range ws.Hyperlinks {
		url := resolveHyperlinkTarget(hl, entry.HyperlinkURLs)
		if url == "" || strings.TrimSpace(hl.Ref) == "" {
			continue
		}
		sheet.Hyperlinks = append(sheet.Hyperlinks, &Hyperlink{
			Ref: strings.ToUpper(strings.TrimSpace(hl.Ref)), URL: url,
			Display: hl.Display, Tooltip: hl.Tooltip,
		})
	}
	arrayFormulas, arrayFormulaCells := buildArrayFormulaTable(ws)
	if len(arrayFormulas) > 0 {
		sheet.ArrayFormulas = arrayFormulas
	}

	// Comments/notes: retain the historical plain-text map while preserving
	// author identity and rich runs in the canonical note model.
	if entry.Comments != nil {
		for _, c := range entry.Comments.CommentList {
			ref := strings.ToUpper(strings.TrimSpace(c.Ref))
			if ref == "" {
				continue
			}
			text := c.Text.PlainValue()
			if text == "" {
				continue
			}
			author := ""
			if c.AuthorID >= 0 && c.AuthorID < len(entry.Comments.Authors) {
				author = entry.Comments.Authors[c.AuthorID]
			}
			if sheet.Comments == nil {
				sheet.Comments = make(map[string]string)
			}
			if sheet.Notes == nil {
				sheet.Notes = make(map[string]*CommentNote)
			}
			sheet.Comments[ref] = text
			sheet.Notes[ref] = &CommentNote{
				Ref: ref, Author: author, Text: text,
				RichText: resolveRichTextRuns(&c.Text, themeColors, indexedColors),
			}
		}
	}

	// Page setup and margins.
	if ws.PageSetup != nil || ws.PageMargins != nil || ws.PrintOptions != nil {
		ps := &PageSetup{}
		if ws.PageSetup != nil {
			ps.PaperSize = ws.PageSetup.PaperSize
			ps.Orientation = ws.PageSetup.Orientation
			ps.Scale = ws.PageSetup.Scale
			ps.FitToPage = ws.PageSetup.FitToPage
			ps.FitToWidth = ws.PageSetup.FitToWidth
			ps.FitToHeight = ws.PageSetup.FitToHeight
			ps.FirstPageNumber = ws.PageSetup.FirstPageNumber
			ps.UseFirstPageNumber = ws.PageSetup.UseFirstPageNumber
			ps.PageOrder = ws.PageSetup.PageOrder
			ps.BlackAndWhite = ws.PageSetup.BlackAndWhite
			ps.Draft = ws.PageSetup.Draft
			ps.HorizontalDPI = ws.PageSetup.HorizontalDPI
			ps.VerticalDPI = ws.PageSetup.VerticalDPI
			ps.Copies = ws.PageSetup.Copies
		}
		if ws.PageMargins != nil {
			ps.MarginLeft = ws.PageMargins.Left
			ps.MarginRight = ws.PageMargins.Right
			ps.MarginTop = ws.PageMargins.Top
			ps.MarginBottom = ws.PageMargins.Bottom
			ps.MarginHeader = ws.PageMargins.Header
			ps.MarginFooter = ws.PageMargins.Footer
		}
		if ws.PrintOptions != nil {
			ps.CenterHorizontally = ws.PrintOptions.HorizontalCentered
			ps.CenterVertically = ws.PrintOptions.VerticalCentered
			ps.PrintHeadings = ws.PrintOptions.Headings
			ps.PrintGridlines = ws.PrintOptions.GridLines
		}
		sheet.PageSetup = ps
	}
	if ws.RowBreaks != nil {
		for _, brk := range ws.RowBreaks.Breaks {
			if brk.ID > 0 {
				sheet.RowPageBreaks = append(sheet.RowPageBreaks, brk.ID)
			}
		}
	}
	if ws.ColBreaks != nil {
		for _, brk := range ws.ColBreaks.Breaks {
			if brk.ID > 0 {
				sheet.ColPageBreaks = append(sheet.ColPageBreaks, brk.ID)
			}
		}
	}

	if ws.SheetProtection != nil {
		sp := ws.SheetProtection
		sheet.Protection = &SheetProtection{
			Password:            sp.Password,
			Sheet:               sp.Sheet,
			Objects:             sp.Objects,
			Scenarios:           sp.Scenarios,
			FormatCells:         sp.FormatCells,
			FormatColumns:       sp.FormatColumns,
			FormatRows:          sp.FormatRows,
			InsertColumns:       sp.InsertColumns,
			InsertRows:          sp.InsertRows,
			InsertHyperlinks:    sp.InsertHyperlinks,
			DeleteColumns:       sp.DeleteColumns,
			DeleteRows:          sp.DeleteRows,
			SelectLockedCells:   sp.SelectLockedCells,
			Sort:                sp.Sort,
			AutoFilter:          sp.AutoFilter,
			PivotTables:         sp.PivotTables,
			SelectUnlockedCells: sp.SelectUnlockedCells,
			AlgorithmName:       sp.AlgorithmName,
			HashValue:           sp.HashValue,
			SaltValue:           sp.SaltValue,
			SpinCount:           sp.SpinCount,
		}
	}

	// Header/footer text.
	if hf := ws.HeaderFooter; hf != nil {
		if hf.OddHeader != "" || hf.OddFooter != "" || hf.EvenHeader != "" ||
			hf.EvenFooter != "" || hf.FirstHeader != "" || hf.FirstFooter != "" ||
			hf.DifferentOddEven || hf.DifferentFirst || hf.ScaleWithDoc || hf.AlignWithMargins {
			resolved := &HeaderFooter{
				OddHeader:        hf.OddHeader,
				OddFooter:        hf.OddFooter,
				EvenHeader:       hf.EvenHeader,
				EvenFooter:       hf.EvenFooter,
				FirstHeader:      hf.FirstHeader,
				FirstFooter:      hf.FirstFooter,
				DifferentOddEven: hf.DifferentOddEven,
				DifferentFirst:   hf.DifferentFirst,
				ScaleWithDoc:     hf.ScaleWithDoc,
				AlignWithMargins: hf.AlignWithMargins,
			}
			for _, img := range entry.HFImages {
				resolved.Images = append(resolved.Images, HFImage{
					ContentType: img.ContentType,
					Data:        img.Data,
				})
			}
			sheet.HeaderFooter = resolved
		}
	}

	// Outline/grouping levels for columns.
	for _, col := range ws.Cols {
		if col.OutlineLevel > 0 {
			if sheet.OutlineCols == nil {
				sheet.OutlineCols = make(map[int]int)
			}
			for c := col.Min; c <= col.Max; c++ {
				sheet.OutlineCols[c] = col.OutlineLevel
			}
		}
		if col.Collapsed {
			if sheet.CollapsedCols == nil {
				sheet.CollapsedCols = make(map[int]bool)
			}
			for c := col.Min; c <= col.Max; c++ {
				sheet.CollapsedCols[c] = true
			}
		}
	}

	sharedFormulaGroups, sharedFormulas := buildSharedFormulaTable(ws)
	if len(sharedFormulaGroups) > 0 {
		sheet.SharedFormulas = sharedFormulaGroups
	}

	for _, row := range ws.Rows {
		r := resolveRow(row, sst, sstRich, styles, hyperlinks, sharedFormulas, arrayFormulaCells, themeColors, indexedColors)
		if r != nil {
			sheet.Rows = append(sheet.Rows, r)
			if row.OutlineLevel > 0 {
				if sheet.OutlineRows == nil {
					sheet.OutlineRows = make(map[int]int)
				}
				sheet.OutlineRows[row.Index] = row.OutlineLevel
			}
			if row.Collapsed {
				if sheet.CollapsedRows == nil {
					sheet.CollapsedRows = make(map[int]bool)
				}
				sheet.CollapsedRows[row.Index] = true
			}
		}
	}

	if entry.Drawing != nil {
		sheet.Drawings = resolveDrawings(entry.Drawing, entry.DrawingMedia, chartParts, entry.DrawingDiagramParts, drawingThemeColors, themeColors, indexedColors)
	}
	if entry.Sheet != nil && entry.Sheet.OLEObjects != nil {
		for _, obj := range entry.Sheet.OLEObjects.Objects {
			relID := normalizeRelID(obj.RelID)
			preserved := entry.OLEParts[relID]
			sheet.Drawings = append(sheet.Drawings, &Drawing{Name: obj.ProgID, Type: DrawingTypeOLE, OLE: &DrawingOLE{ProgID: obj.ProgID, Link: obj.Link, RelID: relID, ShapeID: obj.ShapeID, Data: append([]byte(nil), preserved.Data...), ContentType: preserved.ContentType, External: preserved.External}})
		}
	}

	if len(entry.PivotTables) > 0 {
		sheet.PivotTables = resolvePivotTables(entry.PivotTables)
	}

	if len(entry.Tables) > 0 {
		sheet.Tables = resolveTables(entry.Tables)
	}

	if len(entry.LegacyDrawingVML) > 0 {
		sheet.LegacyDrawingVML = append([]byte(nil), entry.LegacyDrawingVML...)
	}

	for _, obj := range entry.VMLObjects {
		copyObj := obj
		sheet.VMLObjects = append(sheet.VMLObjects, &VMLObject{
			ID: copyObj.ID, Type: copyObj.Type, Style: copyObj.Style, FillColor: copyObj.FillColor,
			StrokeColor: copyObj.StrokeColor, Text: copyObj.Text, Anchor: copyObj.Anchor,
		})
	}

	for _, fc := range entry.FormControls {
		sheet.FormControls = append(sheet.FormControls, &FormControl{
			Type:          fc.Type,
			AltText:       fc.AltText,
			Anchor:        fc.Anchor,
			Checked:       fc.Checked,
			Mixed:         fc.Mixed,
			FirstButton:   fc.FirstButton,
			FmlaLink:      fc.FmlaLink,
			FmlaGroup:     fc.FmlaGroup,
			FmlaRange:     fc.FmlaRange,
			SelectionType: fc.SelectionType,
			Selected:      append([]int(nil), fc.Selected...),
			Min:           fc.Min,
			Max:           fc.Max,
			Val:           fc.Val,
			Increment:     fc.Increment,
			PageStep:      fc.PageStep,
			Horizontal:    fc.Horizontal,
			DropLines:     fc.DropLines,
		})
	}
	normalizeFormControlLinkedCellState(sheet)

	return sheet
}

func normalizeFormControlLinkedCellState(sheet *Sheet) {
	if sheet == nil || len(sheet.FormControls) == 0 {
		return
	}
	linkedValues := resolveFormControlLinkedCellValues(sheet)
	if len(linkedValues) == 0 {
		return
	}
	groupBoxes := collectResolveRadioGroupBoxes(sheet.FormControls)
	lastWasRadio := false
	lastGroupKey := ""
	groupPosition := 0
	for _, fc := range sheet.FormControls {
		if fc == nil || fc.Type != "Radio" {
			lastWasRadio = false
			lastGroupKey = ""
			groupPosition = 0
			continue
		}
		groupKey := effectiveResolveRadioGroupKey(fc, groupBoxes)
		if fc.FirstButton || !lastWasRadio || groupKey != lastGroupKey {
			groupPosition = 1
		} else {
			groupPosition++
		}
		if value, ok := linkedValues[groupKey]; ok {
			fc.Checked = value == groupPosition
		}
		lastWasRadio = true
		lastGroupKey = groupKey
	}
}

type resolveRadioGroupBox struct {
	link               string
	startCol, startRow int
	endCol, endRow     int
}

func collectResolveRadioGroupBoxes(controls []*FormControl) []resolveRadioGroupBox {
	out := make([]resolveRadioGroupBox, 0, len(controls))
	for _, fc := range controls {
		if fc == nil || fc.Type != "GBox" {
			continue
		}
		link := strings.TrimSpace(fc.FmlaGroup)
		startCol, startRow, endCol, endRow, ok := parseFormControlAnchorBounds(fc.Anchor)
		if link == "" || !ok {
			continue
		}
		out = append(out, resolveRadioGroupBox{
			link:     link,
			startCol: startCol,
			startRow: startRow,
			endCol:   endCol,
			endRow:   endRow,
		})
	}
	return out
}

func effectiveResolveRadioGroupKey(fc *FormControl, groupBoxes []resolveRadioGroupBox) string {
	if fc == nil {
		return ""
	}
	if link := containingResolveRadioGroupBoxLink(fc, groupBoxes); link != "" {
		return "link:" + normalizeResolveFormControlLink(link)
	}
	if link := strings.TrimSpace(fc.FmlaLink); link != "" {
		return "link:" + normalizeResolveFormControlLink(link)
	}
	return strings.TrimSpace(fc.AltText)
}

func containingResolveRadioGroupBoxLink(fc *FormControl, groupBoxes []resolveRadioGroupBox) string {
	if fc == nil || len(groupBoxes) == 0 {
		return ""
	}
	startCol, startRow, endCol, endRow, ok := parseFormControlAnchorBounds(fc.Anchor)
	if !ok {
		return ""
	}
	bestArea := 0
	bestLink := ""
	for _, box := range groupBoxes {
		if startCol < box.startCol || endCol > box.endCol || startRow < box.startRow || endRow > box.endRow {
			continue
		}
		area := (box.endCol - box.startCol + 1) * (box.endRow - box.startRow + 1)
		if bestLink == "" || area < bestArea {
			bestLink = box.link
			bestArea = area
		}
	}
	return bestLink
}

func resolveFormControlLinkedCellValues(sheet *Sheet) map[string]int {
	values := make(map[string]int)
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			ref := normalizeResolveFormControlLink(cell.Ref)
			if ref == "" {
				continue
			}
			if value, ok := resolveFormControlLinkedCellIndex(cell); ok {
				values["link:"+ref] = value
			}
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func resolveFormControlLinkedCellIndex(cell *Cell) (int, bool) {
	if cell == nil {
		return 0, false
	}
	switch cell.Kind {
	case CellNumber, CellBool:
		if cell.NumValue < 1 {
			return 0, false
		}
		rounded := math.Round(cell.NumValue)
		if math.Abs(cell.NumValue-rounded) > 1e-9 {
			return 0, false
		}
		return int(rounded), true
	case CellString, CellFormula:
		text := strings.TrimSpace(cell.StrValue)
		if text == "" {
			return 0, false
		}
		value, err := strconv.Atoi(text)
		if err != nil || value < 1 {
			return 0, false
		}
		return value, true
	default:
		return 0, false
	}
}

func normalizeResolveFormControlLink(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if idx := strings.LastIndex(ref, "!"); idx >= 0 {
		ref = ref[idx+1:]
	}
	ref = strings.ReplaceAll(ref, "$", "")
	return strings.ToUpper(strings.TrimSpace(ref))
}

func parseFormControlAnchorBounds(anchor string) (startCol, startRow, endCol, endRow int, ok bool) {
	parts := strings.Split(anchor, ",")
	if len(parts) != 8 {
		return 0, 0, 0, 0, false
	}
	values := make([]int, 0, 8)
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return 0, 0, 0, 0, false
		}
		values = append(values, value)
	}
	return values[0], values[2], values[4], values[6], true
}

// resolvePivotTables converts parts pivot table data into IR PivotTable values.
func resolvePivotTables(ptcs []*parts.PivotTableWithCache) []*PivotTable {
	out := make([]*PivotTable, 0, len(ptcs))
	for _, ptc := range ptcs {
		if ptc == nil || ptc.Table == nil {
			continue
		}
		pt := ptc.Table
		resolved := &PivotTable{
			Name: pt.Name, LocationRef: pt.Location.Ref, DataCaption: pt.DataCaption,
			RowGrandTotals: pivotBoolValue(pt.RowGrandTotals, true),
			ColGrandTotals: pivotBoolValue(pt.ColGrandTotals, true),
			ShowHeaders:    pivotBoolValue(pt.ShowHeaders, true),
		}
		if ptc.Cache != nil && ptc.Cache.WorksheetSource != nil {
			src := ptc.Cache.WorksheetSource
			resolved.SourceSheet, resolved.SourceRef, resolved.SourceName = src.Sheet, src.Ref, src.Name
		}
		fieldLabel := func(index int) string {
			if index >= 0 && index < len(pt.PivotFields) && pt.PivotFields[index].Name != "" {
				return pt.PivotFields[index].Name
			}
			if ptc.Cache != nil && ptc.Cache.CacheFields != nil && index >= 0 && index < len(ptc.Cache.CacheFields.Fields) {
				if name := ptc.Cache.CacheFields.Fields[index].Name; name != "" {
					return name
				}
			}
			if index >= 0 && index < len(pt.PivotFields) {
				return pivotFieldLabel(pt.PivotFields[index], index)
			}
			return fmt.Sprintf("Field%d", index+1)
		}
		for i, pf := range pt.PivotFields {
			field := &PivotField{Name: fieldLabel(i), Axis: pf.Axis, DataField: pf.DataField, Compact: pf.Compact, Outline: pf.Outline, SubtotalTop: pf.SubtotalTop, ShowAll: pf.ShowAll}
			if pf.Items != nil {
				for _, item := range pf.Items.Items {
					cacheIndex := item.X
					// PivotItem.Name is explicit item metadata, not the cache display
					// value. Keep an omitted name omitted in canonical IR; consumers
					// can resolve the display value from the pivot cache when needed.
					name := item.Name
					hideDetail := false
					if item.HideDetail != nil {
						hideDetail = !*item.HideDetail
					}
					itemType := item.Type
					if itemType == "" {
						itemType = "data"
					}
					field.Items = append(field.Items, &PivotItem{CacheIndex: cacheIndex, Type: itemType, Name: name, Hidden: item.Hidden, HideDetail: hideDetail})
				}
			}
			resolved.Fields = append(resolved.Fields, field)
		}
		for _, fi := range pt.RowFields {
			if fi.Index >= 0 && fi.Index < len(pt.PivotFields) {
				resolved.RowFields = append(resolved.RowFields, fieldLabel(fi.Index))
			}
		}
		for _, fi := range pt.ColFields {
			if fi.Index >= 0 && fi.Index < len(pt.PivotFields) {
				resolved.ColFields = append(resolved.ColFields, fieldLabel(fi.Index))
			}
		}
		for _, pf := range pt.PageFields {
			if pf.Field >= 0 && pf.Field < len(pt.PivotFields) {
				label := pf.Name
				if label == "" {
					label = fieldLabel(pf.Field)
				}
				resolved.PageFields = append(resolved.PageFields, label)
				if pf.Item != nil {
					resolved.PageFieldSelections = append(resolved.PageFieldSelections, &PivotPageFieldSelection{FieldName: label, ItemIndex: *pf.Item})
				}
			}
		}
		for _, df := range pt.DataFields {
			name, sourceFieldName := df.Name, ""
			if df.Field >= 0 && df.Field < len(pt.PivotFields) {
				sourceFieldName = fieldLabel(df.Field)
			}
			subtotal := df.Subtotal
			if subtotal == "" {
				subtotal = "sum"
			}
			resolved.DataFields = append(resolved.DataFields, &PivotDataField{Name: name, SourceFieldName: sourceFieldName, Subtotal: subtotal, NumFmtID: df.NumFmtID})
		}
		out = append(out, resolved)
	}
	return out
}

func pivotCacheItemDisplayValue(cache *parts.PivotCacheDefinition, fieldIndex, itemIndex int) string {
	if cache == nil || cache.CacheFields == nil || fieldIndex < 0 || fieldIndex >= len(cache.CacheFields.Fields) {
		return ""
	}
	items := cache.CacheFields.Fields[fieldIndex].SharedItems
	if items == nil || itemIndex < 0 || itemIndex >= len(items.Items) {
		return ""
	}
	return items.Items[itemIndex].Value
}

func pivotBoolValue(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

// resolveTables converts parts table data into IR Table values.
func resolveTables(tables []*parts.Table) []*Table {
	out := make([]*Table, 0, len(tables))
	for _, tbl := range tables {
		if tbl == nil {
			continue
		}
		resolved := &Table{
			ID:             tbl.ID,
			Name:           tbl.Name,
			DisplayName:    tbl.DisplayName,
			Ref:            tbl.Ref,
			HeaderRowCount: tbl.HeaderRowCount,
			TotalsRowCount: tbl.TotalsRowCount,
			TotalsRowShown: tbl.TotalsRowShown,
		}
		if tbl.AutoFilter != nil {
			resolved.AutoFilterRef = tbl.AutoFilter.Ref
		}
		if tbl.TableStyleInfo != nil {
			resolved.TableStyle = &TableStyleInfo{
				Name:              tbl.TableStyleInfo.Name,
				ShowFirstColumn:   tbl.TableStyleInfo.ShowFirstColumn,
				ShowLastColumn:    tbl.TableStyleInfo.ShowLastColumn,
				ShowRowStripes:    tbl.TableStyleInfo.ShowRowStripes,
				ShowColumnStripes: tbl.TableStyleInfo.ShowColumnStripes,
			}
		}
		for _, col := range tbl.Columns {
			resolved.Columns = append(resolved.Columns, &TableColumn{
				ID:                      col.ID,
				Name:                    col.Name,
				CalculatedColumnFormula: col.CalculatedColumnFormula,
				TotalsRowFormula:        col.TotalsRowFormula,
				TotalsRowFunction:       col.TotalsRowFunction,
				TotalsRowLabel:          col.TotalsRowLabel,
			})
		}
		out = append(out, resolved)
	}
	return out
}

// pivotFieldLabel returns the display label for a pivot field.
// Uses the explicit Name attribute when available, else "Field N".
func pivotFieldLabel(f parts.PivotField, index int) string {
	if f.Name != "" {
		return f.Name
	}
	return "Field " + strconv.Itoa(index)
}

func resolveDrawings(sd *parts.SpreadsheetDrawing, media map[string]media.Part, chartParts map[string][]byte, diagramParts map[string]parts.PreservedPart, drawingThemeColors map[string]string, themeColors, indexedColors map[int]string) []*Drawing {
	if sd == nil {
		return nil
	}
	var out []*Drawing
	for i := range sd.OneCellAnchors {
		a := &sd.OneCellAnchors[i]
		d := resolveAnchorContent(a.Pic, a.GraphicFrame, a.Shape, a.GroupShape, media, chartParts, diagramParts, drawingThemeColors, themeColors, indexedColors)
		if d == nil {
			continue
		}
		d.ZOrder = a.DocumentOrder
		d.Position = DrawingPosition{
			Anchor:          AnchorOneCell,
			LocksWithSheet:  anchorBool(a.ClientData, true),
			PrintsWithSheet: anchorBool(a.ClientData, false),
			FromCell:        DrawingCellAnchor{Row: a.From.Row + 1, Col: a.From.Col + 1, RowOffset: a.From.RowOff, ColOffset: a.From.ColOff},
			Width:           a.Ext.CX,
			Height:          a.Ext.CY,
		}
		applyGroupDrawingExtent(d, groupShapeTransform(a.GroupShape))
		syncSheetOfficeArtDrawing(d)
		out = append(out, d)
	}
	for i := range sd.TwoCellAnchors {
		a := &sd.TwoCellAnchors[i]
		d := resolveAnchorContent(a.Pic, a.GraphicFrame, a.Shape, a.GroupShape, media, chartParts, diagramParts, drawingThemeColors, themeColors, indexedColors)
		if d == nil {
			continue
		}
		d.ZOrder = a.DocumentOrder
		d.Position = DrawingPosition{
			Anchor:          AnchorTwoCell,
			Behavior:        resolveAnchorBehavior(a.EditAs),
			LocksWithSheet:  anchorBool(a.ClientData, true),
			PrintsWithSheet: anchorBool(a.ClientData, false),
			FromCell:        DrawingCellAnchor{Row: a.From.Row + 1, Col: a.From.Col + 1, RowOffset: a.From.RowOff, ColOffset: a.From.ColOff},
			ToCell:          DrawingCellAnchor{Row: a.To.Row + 1, Col: a.To.Col + 1, RowOffset: a.To.RowOff, ColOffset: a.To.ColOff},
		}
		applyGroupDrawingExtent(d, groupShapeTransform(a.GroupShape))
		syncSheetOfficeArtDrawing(d)
		out = append(out, d)
	}
	for i := range sd.AbsoluteAnchors {
		a := &sd.AbsoluteAnchors[i]
		d := resolveAnchorContent(a.Pic, a.GraphicFrame, a.Shape, a.GroupShape, media, chartParts, diagramParts, drawingThemeColors, themeColors, indexedColors)
		if d == nil {
			continue
		}
		d.ZOrder = a.DocumentOrder
		d.Position = DrawingPosition{
			Anchor:          AnchorAbsolute,
			LocksWithSheet:  anchorBool(a.ClientData, true),
			PrintsWithSheet: anchorBool(a.ClientData, false),
			X:               a.Pos.X,
			Y:               a.Pos.Y,
			Width:           a.Ext.CX,
			Height:          a.Ext.CY,
		}
		applyGroupDrawingExtent(d, groupShapeTransform(a.GroupShape))
		syncSheetOfficeArtDrawing(d)
		out = append(out, d)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ZOrder < out[j].ZOrder })
	return out
}

func resolveAnchorBehavior(editAs string) DrawingAnchorBehavior {
	switch strings.TrimSpace(editAs) {
	case "oneCell":
		return AnchorBehaviorMoveOnly
	case "absolute":
		return AnchorBehaviorFixed
	case "twoCell":
		return AnchorBehaviorMoveAndSize
	default:
		return ""
	}
}

func anchorBool(data *parts.XDRClientData, locks bool) *bool {
	if data == nil {
		return nil
	}
	if locks {
		return data.LocksWithSheet
	}
	return data.PrintsWithSheet
}

func syncSheetOfficeArtDrawing(d *Drawing) *officeartir.Shape {
	if d == nil {
		return nil
	}
	if d.Shape != nil {
		spec := officeartdml.ShapeSpec{
			Name:        d.Name,
			Kind:        sheetOfficeArtKind(d.Type),
			Bounds:      officeartir.Rect{X: d.Position.X, Y: d.Position.Y, W: d.Position.Width, H: d.Position.Height},
			RotationDeg: d.Shape.RotationDeg,
			FlipH:       d.Shape.FlipH,
			FlipV:       d.Shape.FlipV,
			Fill:        d.Shape.Fill,
			Line:        d.Shape.LineStyle,
			Effects:     d.Shape.Effects,
			Source:      officeartir.SourceRef{Format: "xlsx"},
		}
		if d.Shape.Geometry != "" {
			spec.Preset = &drawing.PresetGeometry{
				Name:        d.Shape.Geometry,
				Adjustments: append([]float64(nil), d.Shape.Adjustments...),
			}
		}
		if d.Shape.ConnectorType != "" {
			adjustments := make([]int, len(d.Shape.Adjustments))
			for i, value := range d.Shape.Adjustments {
				adjustments[i] = int(math.Round(value * 100000))
			}
			spec.Connector = &officeartdml.ConnectorSpec{
				Type:        d.Shape.ConnectorType,
				Adjustments: adjustments,
			}
		}
		d.OfficeArt = officeartdml.Shape(spec)
		return d.OfficeArt
	}
	if d.Image != nil {
		d.OfficeArt = officeartdml.Shape(officeartdml.ShapeSpec{
			Name:   d.Name,
			Kind:   officeartir.ShapePicture,
			Bounds: officeartir.Rect{X: d.Position.X, Y: d.Position.Y, W: d.Position.Width, H: d.Position.Height},
			Image: &drawing.PictureFill{
				EmbedID:     d.Image.RelID,
				ImageData:   append([]byte(nil), d.Image.Data...),
				ContentType: d.Image.ContentType,
			},
			Source: officeartir.SourceRef{Format: "xlsx"},
		})
		return d.OfficeArt
	}
	if d.Group != nil {
		root := &officeartir.Shape{
			Name:      d.Name,
			Kind:      officeartir.ShapeGroup,
			Transform: officeartir.Transform{Rect: officeartir.Rect{X: d.Position.X, Y: d.Position.Y, W: d.Position.Width, H: d.Position.Height}},
			Source:    officeartir.SourceRef{Format: "xlsx"},
		}
		for _, child := range d.Group.Children {
			if composed := syncSheetOfficeArtDrawing(child); composed != nil {
				root.Children = append(root.Children, composed)
			}
		}
		d.OfficeArt = root
		return root
	}
	return nil
}

func sheetOfficeArtKind(kind DrawingType) officeartir.ShapeKind {
	switch kind {
	case DrawingTypeImage:
		return officeartir.ShapePicture
	case DrawingTypeConnector:
		return officeartir.ShapeConnector
	case DrawingTypeGroup:
		return officeartir.ShapeGroup
	default:
		return officeartir.ShapeAuto
	}
}

func resolveAnchorContent(pic *parts.XDRPic, gf *parts.XDRGraphicFrame, sp *parts.XDRShape, grp *parts.XDRGroupShape, mediaMap map[string]media.Part, chartParts map[string][]byte, diagramParts map[string]parts.PreservedPart, drawingThemeColors map[string]string, themeColors, indexedColors map[int]string) *Drawing {
	if pic != nil {
		return resolvePictureDrawing(pic, mediaMap, drawingThemeColors)
	}
	if gf != nil {
		return resolveGraphicFrameDrawing(gf, chartParts, diagramParts, drawingThemeColors)
	}
	if sp != nil {
		return resolveShapeDrawing(sp, mediaMap, drawingThemeColors, themeColors, indexedColors)
	}
	if grp != nil {
		return resolveGroupDrawing(grp, mediaMap, chartParts, diagramParts, drawingThemeColors, themeColors, indexedColors)
	}
	return nil
}

func resolvePictureDrawing(pic *parts.XDRPic, mediaMap map[string]media.Part, themeColors map[string]string) *Drawing {
	if pic == nil {
		return nil
	}
	d := &Drawing{
		Name: pic.NvPicPr.CNvPr.Name,
		Type: DrawingTypeImage,
	}
	if title := pic.NvPicPr.CNvPr.Title; title != "" {
		d.Name = title
	}
	relID := normalizeRelID(pic.BlipFill.Blip.Embed)
	img := &DrawingImage{RelID: relID, AltText: pic.NvPicPr.CNvPr.Descr}
	img.Effects = pic.SpPr.DrawingEffectList(themeColors)
	if pic.SpPr.Xfrm != nil {
		img.RotationDeg = float64(pic.SpPr.Xfrm.Rot) / 60000.0
		img.FlipH = pic.SpPr.Xfrm.FlipH
		img.FlipV = pic.SpPr.Xfrm.FlipV
	}
	if crop := pic.BlipFill.SrcRect; crop != nil {
		img.CropLeft = clampDrawingCrop(crop.Left)
		img.CropTop = clampDrawingCrop(crop.Top)
		img.CropRight = clampDrawingCrop(crop.Right)
		img.CropBottom = clampDrawingCrop(crop.Bottom)
	}
	if mp, ok := mediaMap[relID]; ok {
		img.Data = mp.Data
		img.ContentType = mp.ContentType
	}
	d.Image = img
	return d
}

func clampDrawingCrop(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100000 {
		return 100000
	}
	return v
}

func resolveGraphicFrameDrawing(gf *parts.XDRGraphicFrame, chartParts map[string][]byte, diagramParts map[string]parts.PreservedPart, drawingThemeColors map[string]string) *Drawing {
	if gf == nil {
		return nil
	}
	uri := strings.ToLower(strings.TrimSpace(gf.Graphic.GraphicData.URI))
	if strings.Contains(uri, "/diagram") && gf.Graphic.GraphicData.RelIDs != nil {
		r := gf.Graphic.GraphicData.RelIDs
		sa := &DrawingSmartArt{DataRelID: normalizeRelID(r.Data), LayoutRelID: normalizeRelID(r.Layout), QuickStyleRelID: normalizeRelID(r.QuickStyle), ColorsRelID: normalizeRelID(r.Colors), Parts: map[string]DrawingObjectPart{}}
		for _, id := range []string{sa.DataRelID, sa.LayoutRelID, sa.QuickStyleRelID, sa.ColorsRelID} {
			if id == "" {
				continue
			}
			if part, ok := diagramParts[id]; ok {
				sa.Parts[id] = resolvePreservedObjectPart(part)
			}
		}
		resolveSmartArtSemanticModel(sa, drawingThemeColors)
		return &Drawing{Name: graphicFrameNameResolved(gf), Type: DrawingTypeSmartArt, SmartArt: sa}
	}
	if gf.Graphic.GraphicData.Chart == nil && !strings.Contains(uri, "/chart") {
		return nil
	}
	name := graphicFrameNameResolved(gf)

	drawingChart := &DrawingChart{
		AltTitle: strings.TrimSpace(gf.NvGraphicFramePr.CNvPr.Title),
		AltText:  strings.TrimSpace(gf.NvGraphicFramePr.CNvPr.Descr),
	}
	if gf.Graphic.GraphicData.Chart != nil {
		relID := normalizeRelID(gf.Graphic.GraphicData.Chart.RelID)
		drawingChart.RelID = relID
		if chartXML, ok := chartParts[relID]; ok {
			drawingChart.RawXML = append([]byte(nil), chartXML...)
			if c, err := chart.ParseChart(chartXML); err == nil && c != nil {
				c.ThemeColors = drawingThemeColors
				drawingChart.Chart = c
				drawingChart.SVG = c.SVG(600, 400)
			}
		}
	}
	return &Drawing{Name: name, Type: DrawingTypeChart, Chart: drawingChart}
}

func resolveSmartArtSemanticModel(sa *DrawingSmartArt, themeColors map[string]string) {
	if sa == nil {
		return
	}
	dataXML := smartArtPartData(sa, sa.DataRelID)
	if len(dataXML) == 0 {
		return
	}
	diagram, err := smartart.ParseDiagramWithOptionsAndTheme(
		dataXML,
		smartArtPartData(sa, sa.LayoutRelID),
		smartArtPartData(sa, sa.QuickStyleRelID),
		smartArtPartData(sa, sa.ColorsRelID),
		themeColors,
	)
	if err != nil || diagram == nil {
		return
	}
	if diagram.Layout != nil {
		sa.DiagramType = diagram.Layout.Type
	}
	sa.Nodes = make([]*SmartArtNode, 0, len(diagram.Nodes))
	for _, node := range diagram.Nodes {
		if node == nil {
			continue
		}
		irNode := &SmartArtNode{ID: node.ID, Type: node.Type, Text: node.Text, TextColor: node.TextColor}
		switch fill := node.Fill.(type) {
		case drawing.SolidFill:
			irNode.FillColor = fill.Color
		case *drawing.SolidFill:
			if fill != nil {
				irNode.FillColor = fill.Color
			}
		}
		for _, child := range node.Children {
			if child != nil {
				irNode.Children = append(irNode.Children, child.ID)
			}
		}
		sa.Nodes = append(sa.Nodes, irNode)
	}
	sa.Connections = make([]SmartArtConnection, 0, len(diagram.Connections))
	for _, cxn := range diagram.Connections {
		sa.Connections = append(sa.Connections, SmartArtConnection{SrcID: cxn.SrcID, DestID: cxn.DestID})
	}
}

func smartArtPartData(sa *DrawingSmartArt, relID string) []byte {
	if sa == nil || relID == "" {
		return nil
	}
	part, ok := sa.Parts[relID]
	if !ok {
		return nil
	}
	return part.Data
}

func resolvePreservedObjectPart(part parts.PreservedPart) DrawingObjectPart {
	out := DrawingObjectPart{RelID: part.RelID, RelType: part.RelType, PartName: part.PartName, ContentType: part.ContentType, Target: part.Target, Data: append([]byte(nil), part.Data...), External: part.External}
	for _, rel := range part.Relationships {
		out.Relationships = append(out.Relationships, DrawingObjectRelationship{ID: rel.ID, Type: rel.Type, Target: rel.Target, TargetMode: rel.TargetMode})
	}
	if len(part.Children) > 0 {
		out.Children = map[string]DrawingObjectPart{}
		for name, child := range part.Children {
			out.Children[name] = resolvePreservedObjectPart(child)
		}
	}
	return out
}

func graphicFrameNameResolved(gf *parts.XDRGraphicFrame) string {
	if gf == nil {
		return ""
	}
	// Preserve the historical public drawing identity: existing workbook APIs
	// address charts by description first, then title, then DrawingML name.
	// Accessibility metadata is preserved separately on DrawingChart.
	if descr := strings.TrimSpace(gf.NvGraphicFramePr.CNvPr.Descr); descr != "" {
		return descr
	}
	if title := strings.TrimSpace(gf.NvGraphicFramePr.CNvPr.Title); title != "" {
		return title
	}
	return strings.TrimSpace(gf.NvGraphicFramePr.CNvPr.Name)
}

func resolveShapeDrawing(sp *parts.XDRShape, mediaMap map[string]media.Part, drawingThemeColors map[string]string, themeColors, indexedColors map[int]string) *Drawing {
	if sp == nil {
		return nil
	}
	shape := resolveShapeWithColorMaps(sp, mediaMap, drawingThemeColors, themeColors, indexedColors)
	drawingType := DrawingTypeShape
	if strings.EqualFold(sp.NvSpPr.CNvPr.Descr, "connector") || shape.Geometry == "line" || (shape.Text == "" && shape.FillColor == "" && shape.LineColor != "") {
		drawingType = DrawingTypeConnector
		if shape.ConnectorType == "" {
			if strings.Contains(strings.ToLower(shape.Geometry), "connector") {
				shape.ConnectorType = shape.Geometry
			} else {
				shape.ConnectorType = "straightConnector1"
			}
		}
	}
	return &Drawing{
		Name:  sp.NvSpPr.CNvPr.Name,
		Type:  drawingType,
		Shape: shape,
	}
}

func resolveGroupDrawing(grp *parts.XDRGroupShape, mediaMap map[string]media.Part, chartParts map[string][]byte, diagramParts map[string]parts.PreservedPart, drawingThemeColors map[string]string, themeColors, indexedColors map[int]string) *Drawing {
	if grp == nil {
		return nil
	}
	group := &Drawing{
		Type:  DrawingTypeGroup,
		Group: &DrawingGroup{},
	}
	if grp.NvGrpSpPr != nil {
		group.Name = grp.NvGrpSpPr.CNvPr.Name
		if title := grp.NvGrpSpPr.CNvPr.Title; title != "" {
			group.Name = title
		}
	}
	var groupXfrm *parts.XDRTransform2D
	if grp.GrpSpPr != nil {
		groupXfrm = grp.GrpSpPr.Xfrm
	}
	for _, item := range groupItems(grp) {
		var child *Drawing
		switch item.Kind {
		case groupItemShape:
			if item.Index >= 0 && item.Index < len(grp.Shapes) {
				child = resolveShapeDrawing(&grp.Shapes[item.Index], mediaMap, drawingThemeColors, themeColors, indexedColors)
				if child != nil {
					child.Position = resolveGroupedChildPosition(grp.Shapes[item.Index].SpPr.Xfrm, groupXfrm)
				}
			}
		case groupItemPicture:
			if item.Index >= 0 && item.Index < len(grp.Pictures) {
				child = resolvePictureDrawing(&grp.Pictures[item.Index], mediaMap, drawingThemeColors)
				if child != nil {
					child.Position = resolveGroupedChildPosition(grp.Pictures[item.Index].SpPr.Xfrm, groupXfrm)
				}
			}
		case groupItemGraphicFrame:
			if item.Index >= 0 && item.Index < len(grp.GraphicFrms) {
				child = resolveGraphicFrameDrawing(&grp.GraphicFrms[item.Index], chartParts, diagramParts, drawingThemeColors)
				if child != nil {
					child.Position = resolveGroupedChildPosition(grp.GraphicFrms[item.Index].Xfrm, groupXfrm)
				}
			}
		case groupItemGroup:
			if item.Index >= 0 && item.Index < len(grp.GroupShapes) {
				child = resolveGroupDrawing(&grp.GroupShapes[item.Index], mediaMap, chartParts, diagramParts, drawingThemeColors, themeColors, indexedColors)
				if child != nil {
					child.Position = resolveGroupedChildPosition(groupShapeTransform(&grp.GroupShapes[item.Index]), groupXfrm)
				}
			}
		}
		if child != nil {
			group.Group.Children = append(group.Group.Children, child)
		}
	}
	return group
}

type groupItem struct {
	Kind  parts.XDRGroupItemKind
	Index int
}

const (
	groupItemShape        = parts.XDRGroupItemKind("sp")
	groupItemPicture      = parts.XDRGroupItemKind("pic")
	groupItemGraphicFrame = parts.XDRGroupItemKind("graphicFrame")
	groupItemGroup        = parts.XDRGroupItemKind("grpSp")
)

func groupItems(grp *parts.XDRGroupShape) []groupItem {
	if grp == nil {
		return nil
	}
	if len(grp.Items) > 0 {
		items := make([]groupItem, 0, len(grp.Items))
		for _, item := range grp.Items {
			items = append(items, groupItem{Kind: item.Kind, Index: item.Index})
		}
		return items
	}
	var items []groupItem
	for i := range grp.Shapes {
		items = append(items, groupItem{Kind: groupItemShape, Index: i})
	}
	for i := range grp.Pictures {
		items = append(items, groupItem{Kind: groupItemPicture, Index: i})
	}
	for i := range grp.GraphicFrms {
		items = append(items, groupItem{Kind: groupItemGraphicFrame, Index: i})
	}
	for i := range grp.GroupShapes {
		items = append(items, groupItem{Kind: groupItemGroup, Index: i})
	}
	return items
}

func groupShapeTransform(grp *parts.XDRGroupShape) *parts.XDRTransform2D {
	if grp == nil || grp.GrpSpPr == nil {
		return nil
	}
	return grp.GrpSpPr.Xfrm
}

func resolveGroupedChildPosition(childXfrm, parentXfrm *parts.XDRTransform2D) DrawingPosition {
	position := DrawingPosition{Anchor: AnchorAbsolute}
	if childXfrm == nil || childXfrm.Off == nil || childXfrm.Ext == nil {
		return position
	}
	position.X = childXfrm.Off.X
	position.Y = childXfrm.Off.Y
	position.Width = childXfrm.Ext.CX
	position.Height = childXfrm.Ext.CY
	if parentXfrm == nil || parentXfrm.Ext == nil {
		return position
	}
	baseX, baseY := int64(0), int64(0)
	scaleW, scaleH := parentXfrm.Ext.CX, parentXfrm.Ext.CY
	if parentXfrm.ChOff != nil {
		baseX = parentXfrm.ChOff.X
		baseY = parentXfrm.ChOff.Y
	}
	if parentXfrm.ChExt != nil {
		if parentXfrm.ChExt.CX > 0 {
			scaleW = parentXfrm.ChExt.CX
		}
		if parentXfrm.ChExt.CY > 0 {
			scaleH = parentXfrm.ChExt.CY
		}
	}
	position.X = scaleGroupCoord(position.X-baseX, parentXfrm.Ext.CX, scaleW)
	position.Y = scaleGroupCoord(position.Y-baseY, parentXfrm.Ext.CY, scaleH)
	position.Width = scaleGroupCoord(position.Width, parentXfrm.Ext.CX, scaleW)
	position.Height = scaleGroupCoord(position.Height, parentXfrm.Ext.CY, scaleH)
	return position
}

func applyGroupDrawingExtent(d *Drawing, xfrm *parts.XDRTransform2D) {
	if d == nil || d.Type != DrawingTypeGroup || xfrm == nil || xfrm.Ext == nil {
		return
	}
	if d.Position.Width <= 0 {
		d.Position.Width = xfrm.Ext.CX
	}
	if d.Position.Height <= 0 {
		d.Position.Height = xfrm.Ext.CY
	}
}

func scaleGroupCoord(value, target, source int64) int64 {
	if source <= 0 || target <= 0 {
		return value
	}
	return value * target / source
}

func resolveShape(sp *parts.XDRShape, mediaMap map[string]media.Part, drawingThemeColors map[string]string) *DrawingShape {
	return resolveShapeWithColorMaps(sp, mediaMap, drawingThemeColors, nil, nil)
}

func resolveShapeWithColorMaps(sp *parts.XDRShape, mediaMap map[string]media.Part, drawingThemeColors map[string]string, themeColors, indexedColors map[int]string) *DrawingShape {
	shape := &DrawingShape{}
	if sp.SpPr.Xfrm != nil {
		shape.RotationDeg = float64(sp.SpPr.Xfrm.Rot) / 60000.0
		shape.FlipH = sp.SpPr.Xfrm.FlipH
		shape.FlipV = sp.SpPr.Xfrm.FlipV
	}
	if pg := sp.SpPr.PrstGeom; pg != nil {
		shape.Geometry = pg.Prst
		dpg := pg.DrawingPresetGeometry()
		if dpg != nil {
			shape.Adjustments = dpg.Adjustments
		}
		switch {
		case strings.EqualFold(pg.Prst, "line"):
			shape.ConnectorType = "straightConnector1"
		case strings.Contains(strings.ToLower(pg.Prst), "connector"):
			shape.ConnectorType = pg.Prst
		}
	}
	shape.Effects = sp.SpPr.DrawingEffectList(drawingThemeColors)
	if sf := sp.SpPr.SolidFill; sf != nil {
		if fill := drawingxml.ResolveSolidFill(sf, drawingThemeColors); fill != nil && fill.Color != "" {
			shape.FillColor = fill.Color
		}
	}
	if bf := sp.SpPr.BlipFill; bf != nil && bf.Blip != nil && bf.Blip.Embed != "" {
		pf := drawing.PictureFill{
			EmbedID: bf.Blip.Embed,
			Alpha:   1.0,
			Tile:    bf.Tile != nil,
			Stretch: bf.Stretch != nil || (bf.Tile == nil && bf.Stretch == nil),
		}
		if bf.Tile != nil {
			pf.TileOffsetX = int64(bf.Tile.Tx)
			pf.TileOffsetY = int64(bf.Tile.Ty)
			pf.TileScaleX = bf.Tile.Sx
			pf.TileScaleY = bf.Tile.Sy
			pf.TileFlip = bf.Tile.Flip
			pf.TileAlign = bf.Tile.Algn
		}
		if mp, ok := mediaMap[bf.Blip.Embed]; ok {
			pf.ImageData = mp.Data
			pf.ContentType = mp.ContentType
		}
		if bf.SrcRect != nil {
			l := clampXDRCropPct(bf.SrcRect.L)
			t := clampXDRCropPct(bf.SrcRect.T)
			r := clampXDRCropPct(bf.SrcRect.R)
			b := clampXDRCropPct(bf.SrcRect.B)
			w := 1 - l - r
			h := 1 - t - b
			if w > 0 && h > 0 {
				pf.Crop = &drawing.Rect{X: l, Y: t, Width: w, Height: h}
			}
		}
		shape.Fill = pf
	}
	if ln := sp.SpPr.Ln; ln != nil {
		shape.LineStyle = ln.ToDrawingLineStyle()
		if fill := drawingxml.ResolveSolidFill(ln.SolidFill, drawingThemeColors); fill != nil && fill.Color != "" {
			shape.LineColor = fill.Color
		}
		if ln.W > 0 {
			shape.LineWidthPt = drawing.EMUToPoints(ln.W)
		}
		if shape.LineStyle != nil {
			if shape.LineColor == "" && shape.LineStyle.Color != "" {
				shape.LineColor = shape.LineStyle.Color
			}
			if shape.LineWidthPt <= 0 && shape.LineStyle.Width > 0 {
				shape.LineWidthPt = shape.LineStyle.Width
			}
		}
	}
	if sp.TxBody != nil {
		var paragraphs []string
		for pi, p := range sp.TxBody.Paragraphs {
			var paragraph strings.Builder
			if pi > 0 {
				shape.RichTextRuns = append(shape.RichTextRuns, &RichTextRun{Text: "\n"})
			}
			for _, r := range p.Runs {
				paragraph.WriteString(r.T)
				shape.RichTextRuns = append(shape.RichTextRuns, resolveShapeTextRun(r, drawingThemeColors, themeColors, indexedColors))
			}
			paragraphs = append(paragraphs, paragraph.String())
		}
		shape.Text = strings.Join(paragraphs, "\n")
	}
	return shape
}

func resolveShapeTextRun(run parts.XDRTxRun, drawingThemeColors map[string]string, themeColors, indexedColors map[int]string) *RichTextRun {
	out := &RichTextRun{Text: run.T}
	if run.Properties == nil {
		return out
	}
	props := run.Properties
	out.Bold = props.Bold != nil || drawingOnOff(props.B)
	out.Italic = props.Italic != nil || drawingOnOff(props.I)
	out.Underline = props.Underline != nil || (props.U != "" && props.U != "none")
	if props.Underline != nil {
		out.UnderlineType = props.Underline.Val
	} else if out.Underline {
		out.UnderlineType = props.U
	}
	out.Strike = props.Strike != nil || (props.StrikeType != "" && props.StrikeType != "noStrike")
	if props.VertAlign != nil {
		out.VertAlign = props.VertAlign.Val
	} else if props.Baseline > 0 {
		out.VertAlign = "superscript"
	} else if props.Baseline < 0 {
		out.VertAlign = "subscript"
	}
	if props.FontSize != nil {
		out.FontSizePt = props.FontSize.Val
	} else if props.SizeHundredths > 0 {
		out.FontSizePt = float64(props.SizeHundredths) / 100
	}
	out.FontColor = resolveColorVal(props.Color, themeColors, indexedColors)
	if out.FontColor == "" {
		if fill := drawingxml.ResolveSolidFill(props.SolidFill, drawingThemeColors); fill != nil {
			out.FontColor = fill.Color
		}
	}
	if props.FontName != nil {
		out.FontFamily = props.FontName.Val
	} else if props.Latin != nil && !strings.HasPrefix(props.Latin.Typeface, "+") {
		out.FontFamily = props.Latin.Typeface
	}
	return out
}

func drawingOnOff(v string) bool {
	return v == "1" || strings.EqualFold(v, "true")
}

func normalizeRelID(id string) string {
	id = strings.TrimSpace(id)
	return id
}

// clampXDRCropPct clamps an a:srcRect edge value (thousandths of a percent,
// range 0–100000) to a normalized float in [0, 1].
func clampXDRCropPct(v int) float64 {
	if v <= 0 {
		return 0
	}
	if v >= 100000 {
		return 1
	}
	return float64(v) / 100000.0
}

type sharedFormulaMaster struct {
	ref     string
	formula string
}

type arrayFormulaBinding struct {
	anchorRef string
	rangeRef  string
	formula   string
}

func buildSharedFormulaTable(ws *parts.Worksheet) ([]*SharedFormula, map[int]sharedFormulaMaster) {
	if ws == nil {
		return nil, nil
	}
	out := make(map[int]sharedFormulaMaster)
	var groups []*SharedFormula
	for _, row := range ws.Rows {
		for _, cell := range row.Cells {
			if cell.Formula == nil || strings.TrimSpace(cell.Formula.Type) != "shared" || cell.Formula.SharedIndex == nil {
				continue
			}
			text := strings.TrimSpace(cell.Formula.Text)
			if text == "" {
				continue
			}
			out[*cell.Formula.SharedIndex] = sharedFormulaMaster{ref: cell.Ref, formula: text}
			rangeRef := strings.TrimSpace(cell.Formula.Ref)
			if rangeRef == "" {
				rangeRef = strings.TrimSpace(cell.Ref)
			}
			bounds, ok := parseMergeRef(rangeRef)
			if !ok {
				continue
			}
			groups = append(groups, &SharedFormula{
				Index:     *cell.Formula.SharedIndex,
				AnchorRef: strings.ToUpper(strings.TrimSpace(cell.Ref)),
				RangeRef:  rangeRef,
				Formula:   text,
				TopRow:    bounds.TopRow,
				LeftCol:   bounds.LeftCol,
				BottomRow: bounds.BottomRow,
				RightCol:  bounds.RightCol,
			})
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return groups, out
}

func buildArrayFormulaTable(ws *parts.Worksheet) ([]*ArrayFormula, map[string]arrayFormulaBinding) {
	if ws == nil {
		return nil, nil
	}
	var groups []*ArrayFormula
	bindings := make(map[string]arrayFormulaBinding)
	for _, row := range ws.Rows {
		for _, cell := range row.Cells {
			if cell.Formula == nil || strings.TrimSpace(cell.Formula.Type) != "array" {
				continue
			}
			formulaText := strings.TrimSpace(cell.Formula.Text)
			rangeRef := strings.TrimSpace(cell.Formula.Ref)
			anchorRef := strings.ToUpper(strings.TrimSpace(cell.Ref))
			if formulaText == "" || rangeRef == "" || anchorRef == "" {
				continue
			}
			bounds, ok := parseMergeRef(rangeRef)
			if !ok {
				continue
			}
			group := &ArrayFormula{
				AnchorRef: anchorRef,
				RangeRef:  rangeRef,
				Formula:   formulaText,
				TopRow:    bounds.TopRow,
				LeftCol:   bounds.LeftCol,
				BottomRow: bounds.BottomRow,
				RightCol:  bounds.RightCol,
			}
			groups = append(groups, group)
			binding := arrayFormulaBinding{anchorRef: anchorRef, rangeRef: rangeRef, formula: formulaText}
			for r := bounds.TopRow; r <= bounds.BottomRow; r++ {
				for c := bounds.LeftCol; c <= bounds.RightCol; c++ {
					bindings[colLettersFromIndex(c)+strconv.Itoa(r)] = binding
				}
			}
		}
	}
	if len(groups) == 0 {
		return nil, nil
	}
	return groups, bindings
}

func resolveRow(row parts.Row, sst []string, sstRich []*parts.RichText, styles []styleEntry, hyperlinks map[string]string, sharedFormulas map[int]sharedFormulaMaster, arrayFormulaCells map[string]arrayFormulaBinding, themeColors, indexedColors map[int]string) *Row {
	r := &Row{
		Index:        row.Index,
		Hidden:       row.Hidden,
		HeightPt:     ptFromRowHeight(row.Height),
		OutlineLevel: row.OutlineLevel,
		Collapsed:    row.Collapsed,
	}
	for _, c := range row.Cells {
		cell := resolveCell(c, row.Index, sst, sstRich, styles, hyperlinks, sharedFormulas, arrayFormulaCells, themeColors, indexedColors)
		if cell != nil {
			r.Cells = append(r.Cells, cell)
		}
	}
	if len(r.Cells) == 0 && !r.Hidden && r.HeightPt == 0 && r.OutlineLevel == 0 && !r.Collapsed {
		return nil
	}
	return r
}

func resolveCell(c parts.Cell, rowIdx int, sst []string, sstRich []*parts.RichText, styles []styleEntry, hyperlinks map[string]string, sharedFormulas map[int]sharedFormulaMaster, arrayFormulaCells map[string]arrayFormulaBinding, themeColors, indexedColors map[int]string) *Cell {
	col := colIndexFromRef(c.Ref)
	if col == 0 {
		return nil
	}
	formulaText := resolveFormulaText(c, sharedFormulas)
	ref := strings.ToUpper(strings.TrimSpace(c.Ref))
	arrayFormula, hasArrayFormula := arrayFormulaCells[ref]
	if formulaText == "" && hasArrayFormula {
		formulaText = arrayFormula.formula
	}
	cell := &Cell{
		Row:            rowIdx,
		Col:            col,
		Ref:            c.Ref,
		Formula:        formulaText,
		HasCachedValue: formulaText != "" && c.Value != "",
	}
	if hasArrayFormula {
		cell.ArrayFormulaAnchor = arrayFormula.anchorRef
		cell.ArrayFormulaRange = arrayFormula.rangeRef
	}

	if c.StyleIndex >= 0 && c.StyleIndex < len(styles) {
		s := styles[c.StyleIndex]
		cell.Style = CellStyle{
			Bold:          s.bold,
			Italic:        s.italic,
			Underline:     s.underline,
			UnderlineType: s.underlineType,
			Strike:        s.strike,
			FontVertAlign: s.fontVertAlign,
			FontFamily:    s.fontFamily,
			HAlign:        s.hAlign,
			VAlign:        s.vAlign,
			WrapText:      s.wrapText,
			ShrinkToFit:   s.shrinkToFit,
			Indent:        s.indent,
			TextRotation:  s.textRotation,
			FontSizePt:    s.fontSizePt,
			FontColor:     s.fontColor,
			BgColor:       s.bgColor,
			BorderTop:     s.borderTop,
			BorderRight:   s.borderRight,
			BorderBottom:  s.borderBottom,
			BorderLeft:    s.borderLeft,
			NumFmtCode:    s.numFmtCode,
			Locked:        s.locked,
			FormulaHidden: s.formulaHidden,
		}
	}

	switch c.Type {
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(c.Value))
		if err == nil && idx >= 0 && idx < len(sst) {
			cell.Kind = CellString
			cell.StrValue = sst[idx]
			if idx < len(sstRich) && sstRich[idx] != nil {
				cell.RichTextRuns = resolveRichTextRuns(sstRich[idx], themeColors, indexedColors)
			}
		}
	case "b":
		cell.Kind = CellBool
		if strings.TrimSpace(c.Value) == "1" {
			cell.NumValue = 1
		}
	case "e":
		cell.Kind = CellError
		cell.StrValue = resolveErrorValue(c.Value)
	case "str":
		// Formula string result - the cached result of a formula that returns text
		cell.Kind = CellFormula
		cell.StrValue = c.Value
	case "inlineStr":
		cell.Kind = CellString
		if c.InlineStr != nil {
			cell.StrValue = c.InlineStr.PlainValue()
			if len(c.InlineStr.Runs) > 0 {
				cell.RichTextRuns = resolveRichTextRuns(c.InlineStr, themeColors, indexedColors)
			}
		}
	default:
		if c.Value == "" && formulaText == "" {
			if cell.Style == (CellStyle{}) {
				return nil
			}
			cell.Kind = CellEmpty
			return cell
		}
		if c.Value != "" {
			if f, err := strconv.ParseFloat(strings.TrimSpace(c.Value), 64); err == nil {
				cell.Kind = CellNumber
				cell.NumValue = f
			} else if formulaText != "" {
				cell.Kind = CellFormula
				cell.StrValue = c.Value
			}
		} else if formulaText != "" {
			cell.Kind = CellFormula
			cell.StrValue = ""
		} else {
			cell.Kind = CellEmpty
		}
	}

	if len(hyperlinks) > 0 {
		if url, ok := hyperlinks[ref]; ok {
			cell.HyperlinkURL = url
		}
	}

	return cell
}

func resolveFormulaText(c parts.Cell, sharedFormulas map[int]sharedFormulaMaster) string {
	if c.Formula == nil {
		return ""
	}
	text := strings.TrimSpace(c.Formula.Text)
	if strings.TrimSpace(c.Formula.Type) != "shared" {
		return text
	}
	if text != "" {
		return text
	}
	if c.Formula.SharedIndex == nil || len(sharedFormulas) == 0 {
		return ""
	}
	master, ok := sharedFormulas[*c.Formula.SharedIndex]
	if !ok || master.formula == "" || master.ref == "" {
		return ""
	}
	masterRow, masterCol, ok := cellCoordsFromRef(master.ref)
	if !ok {
		return master.formula
	}
	targetRow, targetCol, ok := cellCoordsFromRef(c.Ref)
	if !ok {
		return master.formula
	}
	return translateSharedFormula(master.formula, targetRow-masterRow, targetCol-masterCol)
}

func translateSharedFormula(formula string, rowDelta, colDelta int) string {
	if formula == "" || (rowDelta == 0 && colDelta == 0) {
		return formula
	}
	var out strings.Builder
	for i := 0; i < len(formula); {
		if formula[i] == '"' {
			start := i
			i++
			for i < len(formula) {
				if formula[i] == '"' {
					i++
					if i < len(formula) && formula[i] == '"' {
						i++
						continue
					}
					break
				}
				i++
			}
			out.WriteString(formula[start:i])
			continue
		}
		if isCellRefLead(formula, i) {
			token, n, ok := readCellRefToken(formula[i:])
			if ok {
				out.WriteString(shiftCellRefToken(token, rowDelta, colDelta))
				i += n
				continue
			}
		}
		out.WriteByte(formula[i])
		i++
	}
	return out.String()
}

func isCellRefLead(formula string, idx int) bool {
	if idx < 0 || idx >= len(formula) {
		return false
	}
	ch := formula[idx]
	if ch != '$' && !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) {
		return false
	}
	if idx == 0 {
		return true
	}
	prev := formula[idx-1]
	if (prev >= 'A' && prev <= 'Z') || (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') || prev == '_' || prev == '.' {
		return false
	}
	return true
}

func readCellRefToken(s string) (string, int, bool) {
	if s == "" {
		return "", 0, false
	}
	i := 0
	if s[i] == '$' {
		i++
	}
	colStart := i
	for i < len(s) && ((s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z')) {
		i++
	}
	if i == colStart {
		return "", 0, false
	}
	if i < len(s) && s[i] == '$' {
		i++
	}
	rowStart := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == rowStart {
		return "", 0, false
	}
	return s[:i], i, true
}

func shiftCellRefToken(token string, rowDelta, colDelta int) string {
	colAbs := false
	rowAbs := false
	idx := 0
	if token[idx] == '$' {
		colAbs = true
		idx++
	}
	colStart := idx
	for idx < len(token) && ((token[idx] >= 'A' && token[idx] <= 'Z') || (token[idx] >= 'a' && token[idx] <= 'z')) {
		idx++
	}
	colLetters := token[colStart:idx]
	if idx < len(token) && token[idx] == '$' {
		rowAbs = true
		idx++
	}
	rowStart := idx
	rowValue, err := strconv.Atoi(token[rowStart:])
	if err != nil {
		return token
	}
	colValue := colIndexFromRef(colLetters)
	if colValue == 0 {
		return token
	}
	if !colAbs {
		colValue += colDelta
		if colValue < 1 {
			colValue = 1
		}
	}
	if !rowAbs {
		rowValue += rowDelta
		if rowValue < 1 {
			rowValue = 1
		}
	}
	var out strings.Builder
	if colAbs {
		out.WriteByte('$')
	}
	out.WriteString(colLettersFromIndex(colValue))
	if rowAbs {
		out.WriteByte('$')
	}
	out.WriteString(strconv.Itoa(rowValue))
	return out.String()
}

// resolveRichTextRuns converts parts.RTRun entries to resolved RichTextRun values.
func resolveRichTextRuns(rt *parts.RichText, themeColors, indexedColors map[int]string) []*RichTextRun {
	if rt == nil || len(rt.Runs) == 0 {
		return nil
	}
	out := make([]*RichTextRun, 0, len(rt.Runs))
	for _, run := range rt.Runs {
		r := &RichTextRun{Text: run.Text}
		if p := run.Properties; p != nil {
			r.Bold = p.Bold != nil
			r.Italic = p.Italic != nil
			r.Underline = p.Underline != nil
			if p.Underline != nil {
				r.UnderlineType = strings.TrimSpace(p.Underline.Val)
			}
			r.Strike = p.Strike != nil
			if p.VertAlign != nil {
				r.VertAlign = strings.TrimSpace(p.VertAlign.Val)
			}
			if p.FontSize != nil {
				r.FontSizePt = p.FontSize.Val
			}
			if p.FontName != nil {
				r.FontFamily = p.FontName.Val
			}
			if p.Color != nil {
				r.FontColor = resolveColorVal(p.Color, themeColors, indexedColors)
			}
		}
		out = append(out, r)
	}
	return out
}

// buildHyperlinkMap builds a map from uppercase cell ref (e.g. "A1") to resolved URL.
// External URLs come from hyperlinkURLs; internal references are prefixed with "#".
func resolveHyperlinkTarget(hl parts.Hyperlink, hyperlinkURLs map[string]string) string {
	if hl.RelID != "" {
		return hyperlinkURLs[normalizeRelID(hl.RelID)]
	}
	if hl.Location != "" {
		return "#" + hl.Location
	}
	return ""
}

func buildHyperlinkMap(hyperlinks []parts.Hyperlink, hyperlinkURLs map[string]string) map[string]string {
	if len(hyperlinks) == 0 {
		return nil
	}
	m := make(map[string]string, len(hyperlinks))
	for _, hl := range hyperlinks {
		url := resolveHyperlinkTarget(hl, hyperlinkURLs)
		if url == "" {
			continue
		}
		ref := strings.ToUpper(strings.TrimSpace(hl.Ref))
		if ref == "" {
			continue
		}
		if strings.Contains(ref, ":") {
			expandHyperlinkRange(ref, url, m)
		} else {
			m[ref] = url
		}
	}
	return m
}

// expandHyperlinkRange expands a range ref like "A1:C3" into individual cell refs.
func expandHyperlinkRange(rangeRef, url string, m map[string]string) {
	halves := strings.SplitN(rangeRef, ":", 2)
	if len(halves) != 2 {
		m[rangeRef] = url
		return
	}
	startRow, startCol, ok1 := cellCoordsFromRef(halves[0])
	endRow, endCol, ok2 := cellCoordsFromRef(halves[1])
	if !ok1 || !ok2 {
		m[rangeRef] = url
		return
	}
	for r := startRow; r <= endRow; r++ {
		for col := startCol; col <= endCol; col++ {
			m[colLettersFromIndex(col)+strconv.Itoa(r)] = url
		}
	}
}

// colLettersFromIndex converts a 1-based column index to its A1-style letter(s).
func colLettersFromIndex(col int) string {
	if col <= 0 {
		return ""
	}
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}

func colIndexFromRef(ref string) int {
	col := 0
	for _, ch := range ref {
		if !unicode.IsLetter(ch) {
			break
		}
		col = col*26 + int(unicode.ToUpper(ch)-'A'+1)
	}
	return col
}

func ptFromRowHeight(h float64) float64 {
	return h
}

// resolveFilterColumn maps one parts.FilterColumn into the IR equivalent.
// Returns nil when the column carries no recognisable criteria.
// ApplyStreamingWorksheetSemantics resolves SAX-owned worksheet semantic metadata into canonical IR.
func ApplyStreamingWorksheetSemantics(sheet *Sheet, auto *parts.AutoFilter, validations []parts.DataValidation, conditional []parts.ConditionalFmt, styles *parts.StyleSheet, theme *parts.Theme) {
	if sheet == nil {
		return
	}
	if auto != nil {
		sheet.AutoFilterRef = auto.Ref
		sheet.FilterMode = true
		for i := range auto.FilterColumns {
			if fc := resolveFilterColumn(&auto.FilterColumns[i]); fc != nil {
				sheet.FilterColumns = append(sheet.FilterColumns, fc)
			}
		}
		sheet.SortState = resolveSortState(auto.SortState)
	}
	for i := range validations {
		dv := &validations[i]
		sheet.DataValidations = append(sheet.DataValidations, &DataValidation{SqRef: dv.SqRef, Type: dv.Type, Operator: dv.Operator, ErrorStyle: dv.ErrorStyle, IMEMode: dv.IMEMode, AllowBlank: dv.AllowBlank, ShowDropDown: dv.ShowDropDown, ShowInputMessage: dv.ShowInputMessage, ShowErrorMessage: dv.ShowErrorMessage, PromptTitle: dv.PromptTitle, Prompt: dv.Prompt, ErrorTitle: dv.ErrorTitle, Error: dv.Error, Formula1: dv.Formula1, Formula2: dv.Formula2})
	}
	themeColors := buildThemeColorIndex(theme)
	indexedColors := buildIndexedColorIndex(styles)
	dxfs := buildDxfTable(styles, themeColors, indexedColors)
	for i := range conditional {
		cf := &conditional[i]
		for j := range cf.Rules {
			rule := &cf.Rules[j]
			cr := &CFRule{SqRef: cf.SqRef, Type: rule.Type, Operator: rule.Operator, Priority: rule.Priority, Percent: rule.Percent, Bottom: rule.Bottom, Rank: rule.Rank, Formula: rule.Formula, Text: rule.Text, TimePeriod: rule.TimePeriod}
			if rule.DxfID >= 0 && rule.DxfID < len(dxfs) {
				cr.DxfStyle = dxfs[rule.DxfID]
			}
			if rule.DataBar != nil {
				cr.DataBar = &CFDataBar{MinLength: rule.DataBar.MinLength, MaxLength: rule.DataBar.MaxLength, ShowValue: rule.DataBar.ShowValue, Color: resolveColorVal(rule.DataBar.Color, themeColors, indexedColors), Cfvo: copyCfvo(rule.DataBar.Cfvo)}
			}
			if rule.IconSet != nil {
				cr.IconSet = &CFIconSet{Set: rule.IconSet.Set, ShowValue: rule.IconSet.ShowValue, Reverse: rule.IconSet.Reverse, Cfvo: copyCfvo(rule.IconSet.Cfvo)}
			}
			if rule.ColorScale != nil {
				colors := make([]string, len(rule.ColorScale.Colors))
				for k, c := range rule.ColorScale.Colors {
					colors[k] = resolveColorVal(c.ColorVal(), themeColors, indexedColors)
				}
				cr.ColorScale = &CFColorScale{Cfvo: copyCfvo(rule.ColorScale.Cfvo), Colors: colors}
			}
			sheet.CFRules = append(sheet.CFRules, cr)
		}
	}
}

func ResolveStreamingCommentRichText(text *parts.RichText, styles *parts.StyleSheet, theme *parts.Theme) []*RichTextRun {
	if text == nil {
		return nil
	}
	return resolveRichTextRuns(text, buildThemeColorIndex(theme), buildIndexedColorIndex(styles))
}
func ResolveStreamingValidationLists(wb *Workbook) { resolveDataValidationLists(wb) }

func resolveFilterColumn(fc *parts.FilterColumn) *FilterColumn {
	if fc == nil {
		return nil
	}
	out := &FilterColumn{ColIndex: fc.ColID}

	if fc.Filters != nil {
		out.ShowBlanks = fc.Filters.Blank
		for _, f := range fc.Filters.Filters {
			if f.Val != "" {
				out.Filters = append(out.Filters, f.Val)
			}
		}
	}

	if fc.CustomFilters != nil {
		out.CustomAnd = fc.CustomFilters.And
		for _, cf := range fc.CustomFilters.Filter {
			op := cf.Operator
			if op == "" {
				op = "equal"
			}
			out.CustomFilters = append(out.CustomFilters, &CustomFilter{
				Operator: op,
				Val:      cf.Val,
			})
		}
	}

	if fc.Top10 != nil {
		out.Top10 = &FilterTop10{
			Top:     fc.Top10.Top,
			Percent: fc.Top10.Percent,
			Val:     fc.Top10.Val,
		}
	}

	if fc.DynamicFilter != nil && fc.DynamicFilter.Type != "" {
		out.Dynamic = &DynamicFilter{
			Type:      fc.DynamicFilter.Type,
			Val:       cloneFloat64Ptr(fc.DynamicFilter.Val),
			MaxVal:    cloneFloat64Ptr(fc.DynamicFilter.MaxVal),
			ValISO:    fc.DynamicFilter.ValISO,
			MaxValISO: fc.DynamicFilter.MaxValISO,
		}
	}

	// Only return a non-nil entry when at least one criterion is present.
	if len(out.Filters) == 0 && !out.ShowBlanks && len(out.CustomFilters) == 0 && out.Top10 == nil && out.Dynamic == nil {
		return nil
	}
	return out
}

func resolveSortState(state *parts.SortState) *SortState {
	if state == nil {
		return nil
	}
	out := &SortState{
		Ref:           state.Ref,
		CaseSensitive: state.CaseSensitive,
	}
	for _, condition := range state.Conditions {
		out.Conditions = append(out.Conditions, &SortCondition{
			Ref:        condition.Ref,
			Descending: condition.Descending,
			SortBy:     condition.SortBy,
			CustomList: condition.CustomList,
			DxfID:      cloneIntPtr(condition.DxfID),
			IconSet:    condition.IconSet,
			IconID:     cloneIntPtr(condition.IconID),
		})
	}
	if out.Ref == "" && len(out.Conditions) == 0 && !out.CaseSensitive {
		return nil
	}
	return out
}

func cloneFloat64Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

// parseA1Range parses an A1-style range reference (e.g. "A1:C3" or "A1")
// and returns the 1-based row/col bounds. Returns false if the ref is invalid.
func parseA1Range(ref string) (firstRow, lastRow, firstCol, lastCol int, ok bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, 0, 0, 0, false
	}
	parts := strings.SplitN(ref, ":", 2)
	startRow, startCol, ok1 := cellCoordsFromRef(parts[0])
	if !ok1 {
		return 0, 0, 0, 0, false
	}
	endRow, endCol := startRow, startCol
	if len(parts) == 2 {
		var ok2 bool
		endRow, endCol, ok2 = cellCoordsFromRef(parts[1])
		if !ok2 {
			return 0, 0, 0, 0, false
		}
	}
	if endRow < startRow {
		startRow, endRow = endRow, startRow
	}
	if endCol < startCol {
		startCol, endCol = endCol, startCol
	}
	return startRow, endRow, startCol, endCol, true
}

func parseMergeRef(ref string) (*MergedCell, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, false
	}
	parts := strings.SplitN(ref, ":", 2)
	startRow, startCol, ok := cellCoordsFromRef(parts[0])
	if !ok {
		return nil, false
	}
	endRow, endCol := startRow, startCol
	if len(parts) == 2 {
		var endOK bool
		endRow, endCol, endOK = cellCoordsFromRef(parts[1])
		if !endOK {
			return nil, false
		}
	}
	if endRow < startRow {
		startRow, endRow = endRow, startRow
	}
	if endCol < startCol {
		startCol, endCol = endCol, startCol
	}
	return &MergedCell{
		TopRow:    startRow,
		LeftCol:   startCol,
		BottomRow: endRow,
		RightCol:  endCol,
	}, true
}

func cellCoordsFromRef(ref string) (int, int, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, 0, false
	}
	col := colIndexFromRef(ref)
	if col == 0 {
		return 0, 0, false
	}
	rowStart := 0
	for i, ch := range ref {
		if unicode.IsDigit(ch) {
			rowStart = i
			break
		}
	}
	if rowStart == 0 || rowStart >= len(ref) {
		return 0, 0, false
	}
	row, err := strconv.Atoi(ref[rowStart:])
	if err != nil || row <= 0 {
		return 0, 0, false
	}
	return row, col, true
}

func hydrateWorkbookCharts(wb *Workbook) {
	if wb == nil || len(wb.Sheets) == 0 {
		return
	}
	lookup := buildWorkbookCellLookup(wb)
	formulaCtx := buildChartWorkbookFormulaContext(wb)
	for _, sheet := range wb.Sheets {
		if sheet == nil {
			continue
		}
		for _, drawing := range sheet.Drawings {
			if drawing == nil || drawing.Type != DrawingTypeChart || drawing.Chart == nil {
				continue
			}
			parsed, ok := drawing.Chart.Chart.(*chart.Chart)
			if !ok || parsed == nil {
				continue
			}
			if hydrateChartData(parsed, sheet.Name, lookup, formulaCtx) {
				drawing.Chart.SVG = parsed.SVG(600, 400)
			}
		}
	}
}

type chartWorkbookFormulaContext struct {
	wb               *Workbook
	engines          []*formula.Engine
	externalLinks    []*chartExternalWorkbook
	externalResolver ExternalWorkbookResolver
	resolvedExternal map[string]*chartExternalWorkbook
	externalErrors   map[string]error
}

type chartExternalWorkbook struct {
	index      int
	target     string
	targetBase string
	sheets     map[string]map[string]formula.Value
	live       *chartWorkbookFormulaContext
}

func buildChartWorkbookFormulaContext(wb *Workbook) *chartWorkbookFormulaContext {
	if wb == nil || len(wb.Sheets) == 0 {
		return nil
	}
	ctx := &chartWorkbookFormulaContext{
		wb:               wb,
		engines:          make([]*formula.Engine, len(wb.Sheets)),
		externalLinks:    buildChartExternalWorkbooks(wb),
		resolvedExternal: make(map[string]*chartExternalWorkbook),
		externalErrors:   make(map[string]error),
	}
	for i, sheet := range wb.Sheets {
		if sheet == nil {
			continue
		}
		ctx.engines[i] = buildChartFormulaEngine(wb, i, sheet)
	}
	for i, eng := range ctx.engines {
		if eng == nil {
			continue
		}
		sheetIndex := i
		eng.SetExternalResolver(
			func(ref string) formula.Value { return ctx.getCellForSheet(sheetIndex, ref) },
			func(start, end string) []formula.Value { return ctx.getRangeForSheet(sheetIndex, start, end) },
		)
		eng.SetSheetResolver(
			func(token string) (int, bool) { return ctx.resolveSheetOrdinal(sheetIndex, token) },
			func() int { return ctx.sheetCount() },
		)
		eng.SetPivotDataResolver(
			func(dataField, pivotTableRef string, pivotTableValue formula.Value, criteria []formula.PivotDataCriterion) (formula.Value, bool) {
				currentSheet := ""
				if sheetIndex >= 0 && sheetIndex < len(ctx.wb.Sheets) && ctx.wb.Sheets[sheetIndex] != nil {
					currentSheet = ctx.wb.Sheets[sheetIndex].Name
				}
				return LookupPivotData(ctx.wb, currentSheet, ctx.engineForSheet, dataField, pivotTableRef, pivotTableValue, criteria)
			},
		)
	}
	return ctx
}

func buildChartFormulaEngine(wb *Workbook, sheetIndex int, sheet *Sheet) *formula.Engine {
	eng := formula.NewEngine()
	if sheet == nil {
		return eng
	}
	for _, group := range sheet.ArrayFormulas {
		if group == nil || group.AnchorRef == "" || group.RangeRef == "" || group.Formula == "" {
			continue
		}
		eng.SetArrayFormula(group.AnchorRef, group.RangeRef, group.Formula)
	}
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil || cell.Ref == "" {
				continue
			}
			ref := strings.ToUpper(cell.Ref)
			switch cell.Kind {
			case CellNumber:
				eng.SetCell(ref, formula.Number(cell.NumValue))
			case CellBool:
				eng.SetCell(ref, formula.Bool(cell.NumValue != 0))
			case CellString:
				eng.SetCell(ref, formula.Text(cell.StrValue))
			case CellError:
				if errType := formula.ParseError(cell.StrValue); errType != formula.ErrNone {
					eng.SetCell(ref, formula.Error(errType))
				}
			case CellFormula:
				if cell.StrValue != "" {
					eng.SetCell(ref, formula.Text(cell.StrValue))
				}
			}
		}
	}
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil || cell.Ref == "" || cell.Formula == "" {
				continue
			}
			ref := strings.ToUpper(cell.Ref)
			if cell.ArrayFormulaRange == "" || strings.EqualFold(cell.ArrayFormulaAnchor, ref) {
				eng.SetFormula(ref, ExpandStructuredTableRefs(expandWorkbookDefinedNamesInFormula(cell.Formula, wb, sheetIndex), wb, sheetIndex))
			}
		}
	}
	return eng
}

func expandWorkbookDefinedNamesInFormula(expr string, wb *Workbook, sheetIndex int) string {
	return expandWorkbookDefinedNamesInFormulaDepth(expr, wb, sheetIndex, 0)
}

func expandWorkbookDefinedNamesInFormulaDepth(expr string, wb *Workbook, sheetIndex, depth int) string {
	if expr == "" || wb == nil || len(wb.DefinedNames) == 0 || depth > 8 {
		return expr
	}
	var result strings.Builder
	changed := false
	for i := 0; i < len(expr); {
		if expr[i] == '"' {
			start := i
			i++
			for i < len(expr) {
				if expr[i] == '"' {
					i++
					if i < len(expr) && expr[i] == '"' {
						i++
						continue
					}
					break
				}
				i++
			}
			result.WriteString(expr[start:i])
			continue
		}
		if isWorkbookDefinedNameLead(expr, i) {
			name, n := readWorkbookDefinedNameIdentifier(expr[i:])
			if n > 0 {
				end := i + n
				if isWorkbookDefinedNameWholeWordMatch(expr, i, end) &&
					!workbookDefinedNameLooksLikeCellRef(name) &&
					!workbookDefinedNameFollowedByCallOrSheet(expr, end) {
					if value := resolveWorkbookDefinedNameFormula(wb, sheetIndex, name); value != "" {
						result.WriteByte('(')
						result.WriteString(expandWorkbookDefinedNamesInFormulaDepth(value, wb, sheetIndex, depth+1))
						result.WriteByte(')')
						i = end
						changed = true
						continue
					}
				}
				result.WriteString(expr[i:end])
				i = end
				continue
			}
		}
		result.WriteByte(expr[i])
		i++
	}
	if !changed {
		return expr
	}
	return result.String()
}

func resolveWorkbookDefinedNameFormula(wb *Workbook, sheetIndex int, name string) string {
	if wb == nil || name == "" {
		return ""
	}
	for _, dn := range wb.DefinedNames {
		if dn == nil || dn.Hidden || strings.HasPrefix(strings.ToUpper(dn.Name), "_XLNM.") || !strings.EqualFold(dn.Name, name) {
			continue
		}
		if dn.LocalSheetID != nil && *dn.LocalSheetID != sheetIndex {
			continue
		}
		if value := strings.TrimSpace(strings.TrimPrefix(dn.Value, "=")); value != "" {
			return value
		}
	}
	for _, dn := range wb.DefinedNames {
		if dn == nil || dn.Hidden || dn.LocalSheetID != nil || strings.HasPrefix(strings.ToUpper(dn.Name), "_XLNM.") || !strings.EqualFold(dn.Name, name) {
			continue
		}
		if value := strings.TrimSpace(strings.TrimPrefix(dn.Value, "=")); value != "" {
			return value
		}
	}
	return ""
}

func isWorkbookDefinedNameLead(expr string, idx int) bool {
	if idx < 0 || idx >= len(expr) {
		return false
	}
	ch := expr[idx]
	if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_' || ch == '\\') {
		return false
	}
	if idx == 0 {
		return true
	}
	prev := expr[idx-1]
	return !((prev >= 'A' && prev <= 'Z') || (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') || prev == '_' || prev == '.' || prev == '\\')
}

func readWorkbookDefinedNameIdentifier(s string) (string, int) {
	if s == "" {
		return "", 0
	}
	i := 0
	if s[i] == '\\' {
		i++
	}
	if i >= len(s) {
		return "", 0
	}
	ch := s[i]
	if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_') {
		return "", 0
	}
	i++
	for i < len(s) {
		ch = s[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			i++
			continue
		}
		break
	}
	return s[:i], i
}

func isWorkbookDefinedNameWholeWordMatch(expr string, start, end int) bool {
	if start > 0 {
		ch := expr[start-1]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			return false
		}
	}
	if end < len(expr) {
		ch := expr[end]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			return false
		}
	}
	return true
}

func workbookDefinedNameLooksLikeCellRef(name string) bool {
	if name == "" {
		return false
	}
	_, _, ok := cellCoordsFromRef(strings.ToUpper(strings.TrimPrefix(name, "\\")))
	return ok
}

func workbookDefinedNameFollowedByCallOrSheet(expr string, idx int) bool {
	for idx < len(expr) && unicode.IsSpace(rune(expr[idx])) {
		idx++
	}
	return idx < len(expr) && (expr[idx] == '(' || expr[idx] == '!')
}

func (ctx *chartWorkbookFormulaContext) engineForSheet(sheetName string) *formula.Engine {
	if ctx == nil {
		return nil
	}
	sheetIndex := ctx.sheetIndexByName(sheetName)
	if sheetIndex < 0 || sheetIndex >= len(ctx.engines) {
		return nil
	}
	return ctx.engines[sheetIndex]
}

func (ctx *chartWorkbookFormulaContext) getCellForSheet(currentSheetIndex int, ref string) formula.Value {
	if ctx == nil {
		return formula.Empty()
	}
	if value, ok := ctx.getExternalCell(ref); ok {
		return value
	}
	targetSheetIndex, targetRef, ok := ctx.resolveSheetRef(currentSheetIndex, ref, currentSheetIndex)
	if !ok {
		return formula.Empty()
	}
	if targetSheetIndex < 0 || targetSheetIndex >= len(ctx.engines) {
		return formula.Empty()
	}
	eng := ctx.engines[targetSheetIndex]
	if eng == nil {
		return formula.Empty()
	}
	return eng.GetCell(targetRef)
}

func (ctx *chartWorkbookFormulaContext) getRangeForSheet(currentSheetIndex int, start, end string) []formula.Value {
	if ctx == nil {
		return nil
	}
	if values, ok := ctx.getExternalRange(start, end); ok {
		return values
	}
	startSheetIndex, startRef, ok := ctx.resolveSheetRef(currentSheetIndex, start, currentSheetIndex)
	if !ok {
		return nil
	}
	endSheetIndex, endRef, ok := ctx.resolveSheetRef(currentSheetIndex, end, startSheetIndex)
	if !ok {
		return nil
	}
	if startSheetIndex < 0 || startSheetIndex >= len(ctx.engines) {
		return nil
	}
	if endSheetIndex < 0 || endSheetIndex >= len(ctx.engines) {
		return nil
	}
	if startSheetIndex == endSheetIndex {
		eng := ctx.engines[startSheetIndex]
		if eng == nil {
			return nil
		}
		return eng.GetRange(startRef, endRef)
	}
	startRow, startCol, ok := cellCoordsFromRef(startRef)
	if !ok {
		return nil
	}
	endRow, endCol, ok := cellCoordsFromRef(endRef)
	if !ok {
		return nil
	}
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	if startCol > endCol {
		startCol, endCol = endCol, startCol
	}
	startCell := colLettersFromIndex(startCol) + strconv.Itoa(startRow)
	endCell := colLettersFromIndex(endCol) + strconv.Itoa(endRow)
	if startSheetIndex > endSheetIndex {
		startSheetIndex, endSheetIndex = endSheetIndex, startSheetIndex
	}
	values := make([]formula.Value, 0, (endSheetIndex-startSheetIndex+1)*(endRow-startRow+1)*(endCol-startCol+1))
	for sheetIndex := startSheetIndex; sheetIndex <= endSheetIndex; sheetIndex++ {
		eng := ctx.engines[sheetIndex]
		if eng == nil {
			continue
		}
		values = append(values, eng.GetRange(startCell, endCell)...)
	}
	return values
}

func (ctx *chartWorkbookFormulaContext) resolveSheetRef(currentSheetIndex int, ref string, defaultSheetIndex int) (int, string, bool) {
	if ctx == nil {
		return -1, "", false
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return -1, "", false
	}
	idx := strings.LastIndex(ref, "!")
	if idx < 0 {
		if defaultSheetIndex < 0 {
			defaultSheetIndex = currentSheetIndex
		}
		return defaultSheetIndex, strings.ToUpper(strings.ReplaceAll(ref, "$", "")), defaultSheetIndex >= 0
	}
	sheetIndex := ctx.sheetIndexByName(unquoteChartSheetName(ref[:idx]))
	if sheetIndex < 0 {
		return -1, "", false
	}
	targetRef := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(ref[idx+1:]), "$", ""))
	return sheetIndex, targetRef, targetRef != ""
}

func (ctx *chartWorkbookFormulaContext) resolveSheetOrdinal(currentSheetIndex int, token string) (int, bool) {
	if ctx == nil {
		return 0, false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return currentSheetIndex + 1, currentSheetIndex >= 0
	}
	if idx := strings.LastIndex(token, "!"); idx >= 0 {
		sheetIndex, _, ok := ctx.resolveSheetRef(currentSheetIndex, token, currentSheetIndex)
		if !ok {
			return 0, false
		}
		return sheetIndex + 1, true
	}
	if sheetIndex := ctx.sheetIndexByName(token); sheetIndex >= 0 {
		return sheetIndex + 1, true
	}
	if chartTokenLooksLikeCellRef(token) {
		return currentSheetIndex + 1, currentSheetIndex >= 0
	}
	return 0, false
}

func (ctx *chartWorkbookFormulaContext) sheetCount() int {
	if ctx == nil || ctx.wb == nil {
		return 0
	}
	return len(ctx.wb.Sheets)
}

func buildChartExternalWorkbooks(wb *Workbook) []*chartExternalWorkbook {
	if wb == nil || len(wb.ExternalLinks) == 0 {
		return nil
	}
	var out []*chartExternalWorkbook
	for i, link := range wb.ExternalLinks {
		if link == nil || len(link.CachedSheets) == 0 {
			continue
		}
		cached := &chartExternalWorkbook{
			index:      i + 1,
			target:     strings.ToUpper(strings.TrimSpace(link.Target)),
			targetBase: normalizeExternalWorkbookTargetKey(link.Target),
			sheets:     make(map[string]map[string]formula.Value),
		}
		for _, sheet := range link.CachedSheets {
			if sheet == nil {
				continue
			}
			name := normalizeChartExternalSheetName(sheet.Name)
			if name == "" {
				continue
			}
			cells := make(map[string]formula.Value)
			for _, cell := range sheet.Cells {
				if cell == nil || strings.TrimSpace(cell.Ref) == "" {
					continue
				}
				cells[strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(cell.Ref, "$", "")))] = chartFormulaValueFromExternalCell(cell)
			}
			if len(cells) != 0 {
				cached.sheets[strings.ToUpper(name)] = cells
			}
		}
		if len(cached.sheets) != 0 {
			out = append(out, cached)
		}
	}
	return out
}

func chartFormulaValueFromExternalCell(cell *WorkbookExternalCell) formula.Value {
	if cell == nil {
		return formula.Empty()
	}
	value := strings.TrimSpace(cell.Value)
	switch strings.ToLower(strings.TrimSpace(cell.Type)) {
	case "b":
		return formula.Bool(value == "1" || strings.EqualFold(value, "true"))
	case "e":
		return formula.Error(formula.ParseError(value))
	case "n", "":
		if value == "" {
			return formula.Empty()
		}
		if num, err := strconv.ParseFloat(value, 64); err == nil {
			return formula.Number(num)
		}
	}
	return formula.Text(value)
}

func (ctx *chartWorkbookFormulaContext) getExternalCell(ref string) (formula.Value, bool) {
	workbookToken, sheetName, cellRef, ok := parseChartExternalRef(ref, "", "")
	if !ok {
		return formula.Empty(), false
	}
	workbook := ctx.externalWorkbookByToken(workbookToken)
	if workbook == nil {
		return formula.Empty(), false
	}
	return workbook.cellValue(sheetName, cellRef)
}

func (ctx *chartWorkbookFormulaContext) getExternalRange(start, end string) ([]formula.Value, bool) {
	workbookToken, sheetName, startRef, ok := parseChartExternalRef(start, "", "")
	if !ok {
		return nil, false
	}
	workbook := ctx.externalWorkbookByToken(workbookToken)
	if workbook == nil {
		return nil, false
	}
	_, endSheetName, endRef, ok := parseChartExternalRef(end, workbookToken, sheetName)
	if !ok || !strings.EqualFold(sheetName, endSheetName) {
		return nil, false
	}
	values := workbook.rangeValues(sheetName, startRef, endRef)
	if values == nil {
		return nil, false
	}
	return values, true
}

func (ctx *chartWorkbookFormulaContext) externalWorkbookByToken(token string) *chartExternalWorkbook {
	if ctx == nil {
		return nil
	}
	token = strings.TrimSpace(strings.Trim(token, "[]"))
	if token == "" {
		return nil
	}

	var cached *chartExternalWorkbook
	if index, err := strconv.Atoi(token); err == nil {
		for _, workbook := range ctx.externalLinks {
			if workbook != nil && workbook.index == index {
				cached = workbook
				break
			}
		}
	} else {
		normalized := strings.ToUpper(token)
		for _, workbook := range ctx.externalLinks {
			if workbook == nil {
				continue
			}
			if normalized == workbook.target || normalized == workbook.targetBase {
				cached = workbook
				break
			}
		}
	}

	if ctx.externalResolver == nil {
		return cached
	}
	target := ""
	if ctx.wb != nil {
		if index, err := strconv.Atoi(token); err == nil && index > 0 && index <= len(ctx.wb.ExternalLinks) {
			if link := ctx.wb.ExternalLinks[index-1]; link != nil {
				target = strings.TrimSpace(link.Target)
			}
		}
		if target == "" && cached != nil {
			for _, link := range ctx.wb.ExternalLinks {
				if link == nil {
					continue
				}
				if strings.EqualFold(strings.TrimSpace(link.Target), cached.target) || normalizeExternalWorkbookTargetKey(link.Target) == cached.targetBase {
					target = strings.TrimSpace(link.Target)
					break
				}
			}
		}
	}
	if target == "" && cached != nil {
		target = cached.target
	}
	if target == "" {
		target = token
	}
	key := strings.ToUpper(strings.TrimSpace(target))
	if resolved, ok := ctx.resolvedExternal[key]; ok {
		return resolved
	}
	wb, err := ctx.externalResolver(target)
	if err != nil {
		ctx.externalErrors[target] = err
		return cached
	}
	if wb == nil {
		return cached
	}
	liveCtx := buildChartWorkbookFormulaContext(wb)
	if liveCtx == nil {
		return cached
	}
	liveCtx.externalResolver = ctx.externalResolver
	resolved := &chartExternalWorkbook{target: strings.ToUpper(strings.TrimSpace(target)), targetBase: normalizeExternalWorkbookTargetKey(target), live: liveCtx}
	if cached != nil {
		resolved.index = cached.index
	}
	ctx.resolvedExternal[key] = resolved
	return resolved
}

func (workbook *chartExternalWorkbook) cellValue(sheetName, cellRef string) (formula.Value, bool) {
	if workbook == nil {
		return formula.Empty(), false
	}
	if workbook.live != nil {
		idx := workbook.live.sheetIndexByName(sheetName)
		if idx >= 0 && idx < len(workbook.live.engines) && workbook.live.engines[idx] != nil {
			return workbook.live.engines[idx].GetCell(strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(cellRef), "$", ""))), true
		}
	}
	cells := workbook.sheets[strings.ToUpper(normalizeChartExternalSheetName(sheetName))]
	if len(cells) == 0 {
		return formula.Empty(), false
	}
	value, ok := cells[strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(cellRef, "$", "")))]
	return value, ok
}

func (workbook *chartExternalWorkbook) rangeValues(sheetName, startRef, endRef string) []formula.Value {
	if workbook == nil {
		return nil
	}
	if workbook.live != nil {
		idx := workbook.live.sheetIndexByName(sheetName)
		if idx >= 0 && idx < len(workbook.live.engines) && workbook.live.engines[idx] != nil {
			return workbook.live.engines[idx].GetRange(strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(startRef), "$", "")), strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(endRef), "$", "")))
		}
	}
	cells := workbook.sheets[strings.ToUpper(normalizeChartExternalSheetName(sheetName))]
	if len(cells) == 0 {
		return nil
	}
	startRow, startCol, ok := chartExternalCellRefCoords(startRef)
	if !ok {
		return nil
	}
	endRow, endCol, ok := chartExternalCellRefCoords(endRef)
	if !ok {
		return nil
	}
	if startRow > endRow {
		startRow, endRow = endRow, startRow
	}
	if startCol > endCol {
		startCol, endCol = endCol, startCol
	}
	values := make([]formula.Value, 0, (endRow-startRow+1)*(endCol-startCol+1))
	for row := startRow; row <= endRow; row++ {
		for col := startCol; col <= endCol; col++ {
			ref := colLettersFromIndex(col) + strconv.Itoa(row)
			if value, ok := cells[ref]; ok {
				values = append(values, value)
			} else {
				values = append(values, formula.Empty())
			}
		}
	}
	return values
}

func parseChartExternalRef(ref, defaultWorkbookToken, defaultSheetName string) (string, string, string, bool) {
	ref = strings.ToUpper(strings.TrimSpace(ref))
	idx := strings.LastIndex(ref, "!")
	if idx < 0 {
		if defaultWorkbookToken == "" || defaultSheetName == "" {
			return "", "", "", false
		}
		cellRef := strings.TrimSpace(strings.ReplaceAll(ref, "$", ""))
		if _, _, ok := cellCoordsFromRef(cellRef); !ok {
			return "", "", "", false
		}
		return defaultWorkbookToken, defaultSheetName, cellRef, true
	}
	sheetQualifier := normalizeChartExternalSheetName(ref[:idx])
	if !strings.HasPrefix(sheetQualifier, "[") {
		return "", "", "", false
	}
	endBracket := strings.Index(sheetQualifier, "]")
	if endBracket <= 1 || endBracket == len(sheetQualifier)-1 {
		return "", "", "", false
	}
	workbookToken := strings.TrimSpace(sheetQualifier[1:endBracket])
	sheetName := strings.TrimSpace(sheetQualifier[endBracket+1:])
	cellRef := strings.TrimSpace(strings.ReplaceAll(ref[idx+1:], "$", ""))
	if workbookToken == "" || sheetName == "" {
		return "", "", "", false
	}
	if _, _, ok := cellCoordsFromRef(cellRef); !ok {
		return "", "", "", false
	}
	return workbookToken, sheetName, cellRef, true
}

func normalizeChartExternalSheetName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		name = strings.ReplaceAll(name[1:len(name)-1], "''", "'")
	}
	return name
}

func chartExternalCellRefCoords(ref string) (int, int, bool) {
	return cellCoordsFromRef(strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(ref, "$", ""))))
}

func normalizeExternalWorkbookTargetKey(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if parsed, err := url.Parse(target); err == nil {
		if base := path.Base(parsed.Path); base != "" && base != "." && base != "/" {
			return strings.ToUpper(base)
		}
	}
	if idx := strings.LastIndexAny(target, `/\\`); idx >= 0 && idx+1 < len(target) {
		target = target[idx+1:]
	}
	return strings.ToUpper(target)
}

func (ctx *chartWorkbookFormulaContext) sheetIndexByName(sheetName string) int {
	normalized := normalizeChartSheetName(sheetName)
	if normalized == "" || ctx == nil || ctx.wb == nil {
		return -1
	}
	for i, sheet := range ctx.wb.Sheets {
		if sheet != nil && normalizeChartSheetName(sheet.Name) == normalized {
			return i
		}
	}
	return -1
}

func chartTokenLooksLikeCellRef(token string) bool {
	_, _, ok := cellCoordsFromRef(strings.ReplaceAll(strings.TrimSpace(token), "$", ""))
	return ok
}

func buildWorkbookCellLookup(wb *Workbook) map[string]map[string]*Cell {
	lookup := make(map[string]map[string]*Cell, len(wb.Sheets))
	for _, sheet := range wb.Sheets {
		if sheet == nil {
			continue
		}
		cells := make(map[string]*Cell)
		for _, row := range sheet.Rows {
			if row == nil {
				continue
			}
			for _, cell := range row.Cells {
				if cell == nil || cell.Ref == "" {
					continue
				}
				cells[strings.ToUpper(strings.TrimSpace(cell.Ref))] = cell
			}
		}
		lookup[normalizeChartSheetName(sheet.Name)] = cells
	}
	return lookup
}

func hydrateChartData(c *chart.Chart, currentSheet string, lookup map[string]map[string]*Cell, formulaCtx *chartWorkbookFormulaContext) bool {
	if c == nil || len(c.Series) == 0 {
		return false
	}
	changed := false
	for i := range c.Series {
		series := &c.Series[i]
		if series.NameFormula != "" {
			if name := lookupChartString(series.NameFormula, currentSheet, lookup, formulaCtx); name != "" && name != series.Name {
				series.Name = name
				changed = true
			}
		}
		if series.ValueFormula != "" {
			if values := lookupChartNumbers(series.ValueFormula, currentSheet, lookup, formulaCtx); len(values) > 0 && !chartFloatSlicesEqual(series.Values, values) {
				series.Values = values
				changed = true
			}
		}
		if series.LabelFormula != "" {
			if labels := lookupChartStrings(series.LabelFormula, currentSheet, lookup, formulaCtx); len(labels) > 0 && !chartStringSlicesEqual(series.Labels, labels) {
				series.Labels = labels
				changed = true
			}
		}
	}
	for _, series := range c.Series {
		if len(series.Labels) == 0 {
			continue
		}
		if !chartStringSlicesEqual(c.Categories, series.Labels) {
			c.Categories = append([]string(nil), series.Labels...)
			changed = true
		}
		break
	}
	return changed
}

type chartFormulaRange struct {
	sheetName          string
	startRef           string
	endRef             string
	external           bool
	startRow, startCol int
	endRow, endCol     int
}

func lookupChartString(formula, currentSheet string, lookup map[string]map[string]*Cell, formulaCtx *chartWorkbookFormulaContext) string {
	values := lookupChartStrings(formula, currentSheet, lookup, formulaCtx)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func lookupChartNumbers(formula, currentSheet string, lookup map[string]map[string]*Cell, formulaCtx *chartWorkbookFormulaContext) []float64 {
	r, ok := parseChartFormulaRange(formula, currentSheet)
	if !ok {
		return nil
	}
	if r.external {
		if formulaCtx == nil {
			return nil
		}
		currentSheetIndex := formulaCtx.sheetIndexByName(currentSheet)
		resolved := formulaCtx.getRangeForSheet(currentSheetIndex, r.startRef, r.endRef)
		if len(resolved) == 0 {
			return nil
		}
		values := make([]float64, 0, len(resolved))
		for _, value := range resolved {
			if num, err := value.ToNumber(); err == nil {
				values = append(values, num)
			}
		}
		return values
	}
	cells := lookup[normalizeChartSheetName(r.sheetName)]
	eng := formulaCtx.engineForSheet(r.sheetName)
	if len(cells) == 0 && eng == nil {
		return nil
	}
	var values []float64
	for row := r.startRow; row <= r.endRow; row++ {
		for col := r.startCol; col <= r.endCol; col++ {
			ref := colLettersFromIndex(col) + strconv.Itoa(row)
			cell := cells[ref]
			if value, ok, authoritative := chartEvaluatedNumericValue(ref, eng); authoritative {
				if ok {
					values = append(values, value)
				}
				continue
			}
			if value, ok := chartCachedNumericValue(cell); ok {
				values = append(values, value)
			}
		}
	}
	return values
}

func lookupChartStrings(formula, currentSheet string, lookup map[string]map[string]*Cell, formulaCtx *chartWorkbookFormulaContext) []string {
	r, ok := parseChartFormulaRange(formula, currentSheet)
	if !ok {
		return nil
	}
	if r.external {
		if formulaCtx == nil {
			return nil
		}
		currentSheetIndex := formulaCtx.sheetIndexByName(currentSheet)
		resolved := formulaCtx.getRangeForSheet(currentSheetIndex, r.startRef, r.endRef)
		if len(resolved) == 0 {
			return nil
		}
		values := make([]string, 0, len(resolved))
		for _, value := range resolved {
			if text := value.ToText(); text != "" {
				values = append(values, text)
			}
		}
		return values
	}
	cells := lookup[normalizeChartSheetName(r.sheetName)]
	eng := formulaCtx.engineForSheet(r.sheetName)
	if len(cells) == 0 && eng == nil {
		return nil
	}
	var values []string
	for row := r.startRow; row <= r.endRow; row++ {
		for col := r.startCol; col <= r.endCol; col++ {
			ref := colLettersFromIndex(col) + strconv.Itoa(row)
			cell := cells[ref]
			if text, authoritative := chartEvaluatedStringValue(ref, cell, eng); authoritative {
				if text != "" {
					values = append(values, text)
				}
				continue
			}
			if text := chartCellText(cell); text != "" {
				values = append(values, text)
			}
		}
	}
	return values
}

func parseChartFormulaRange(formula, currentSheet string) (chartFormulaRange, bool) {
	formula = strings.TrimSpace(strings.TrimPrefix(formula, "="))
	if formula == "" {
		return chartFormulaRange{}, false
	}
	r := chartFormulaRange{sheetName: currentSheet}
	rangeRef := formula
	qualifier := ""
	if bang := strings.LastIndex(formula, "!"); bang >= 0 {
		qualifier = strings.TrimSpace(formula[:bang])
		r.sheetName = unquoteChartSheetName(qualifier)
		rangeRef = formula[bang+1:]
	}
	rangeRef = strings.ReplaceAll(strings.TrimSpace(rangeRef), "$", "")
	parts := strings.SplitN(rangeRef, ":", 2)
	startRow, startCol, ok := cellCoordsFromRef(parts[0])
	if !ok {
		return chartFormulaRange{}, false
	}
	endRow, endCol := startRow, startCol
	if len(parts) == 2 {
		var endOK bool
		endRow, endCol, endOK = cellCoordsFromRef(parts[1])
		if !endOK {
			return chartFormulaRange{}, false
		}
	}
	if endRow < startRow {
		startRow, endRow = endRow, startRow
	}
	if endCol < startCol {
		startCol, endCol = endCol, startCol
	}
	r.startRow, r.startCol, r.endRow, r.endCol = startRow, startCol, endRow, endCol
	if qualifier != "" {
		r.startRef = qualifier + "!" + parts[0]
		r.endRef = r.startRef
		if len(parts) == 2 {
			r.endRef = qualifier + "!" + parts[1]
		}
		r.external = strings.HasPrefix(normalizeChartExternalSheetName(qualifier), "[")
	} else {
		r.startRef = parts[0]
		r.endRef = parts[0]
		if len(parts) == 2 {
			r.endRef = parts[1]
		}
	}
	return r, true
}

func unquoteChartSheetName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) >= 2 && strings.HasPrefix(name, "'") && strings.HasSuffix(name, "'") {
		name = strings.ReplaceAll(name[1:len(name)-1], "''", "'")
	}
	return name
}

func normalizeChartSheetName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func chartFloatSlicesEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-9 {
			return false
		}
	}
	return true
}

func chartStringSlicesEqual(a, b []string) bool {
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

func chartEvaluatedNumericValue(ref string, eng *formula.Engine) (float64, bool, bool) {
	if eng == nil {
		return 0, false, false
	}
	value := eng.GetCell(ref)
	switch value.Type {
	case formula.TypeNumber:
		return value.Num, true, true
	case formula.TypeBool:
		if value.Bool {
			return 1, true, true
		}
		return 0, true, true
	case formula.TypeText:
		text := strings.TrimSpace(value.Text)
		if text == "" {
			return 0, false, true
		}
		num, err := strconv.ParseFloat(text, 64)
		return num, err == nil, true
	case formula.TypeEmpty:
		return 0, false, true
	default:
		return 0, false, false
	}
}

func chartCachedNumericValue(cell *Cell) (float64, bool) {
	if cell == nil {
		return 0, false
	}
	switch cell.Kind {
	case CellNumber, CellBool:
		return cell.NumValue, true
	case CellFormula:
		if cell.StrValue == "" {
			return cell.NumValue, true
		}
		if value, err := strconv.ParseFloat(strings.TrimSpace(cell.StrValue), 64); err == nil {
			return value, true
		}
	}
	return 0, false
}

func chartEvaluatedStringValue(ref string, cell *Cell, eng *formula.Engine) (string, bool) {
	if eng == nil {
		return "", false
	}
	value := eng.GetCell(ref)
	if value.IsError() {
		return "", false
	}
	return chartTextFromFormulaValue(value, cell), true
}

func chartTextFromFormulaValue(value formula.Value, cell *Cell) string {
	switch value.Type {
	case formula.TypeText:
		return value.Text
	case formula.TypeBool:
		if value.Bool {
			return "TRUE"
		}
		return "FALSE"
	case formula.TypeError:
		return value.Err.String()
	case formula.TypeNumber:
		if cell != nil && looksLikeChartDateFormat(cell.Style.NumFmtCode) {
			return formatChartExcelDate(value.Num, cell.Style.NumFmtCode)
		}
		return formatChartNumber(value.Num)
	default:
		return ""
	}
}

func chartCellText(cell *Cell) string {
	if cell == nil {
		return ""
	}
	switch cell.Kind {
	case CellString, CellError:
		return cell.StrValue
	case CellFormula:
		if cell.StrValue != "" {
			return cell.StrValue
		}
		if looksLikeChartDateFormat(cell.Style.NumFmtCode) {
			return formatChartExcelDate(cell.NumValue, cell.Style.NumFmtCode)
		}
		return formatChartNumber(cell.NumValue)
	case CellBool:
		if cell.NumValue != 0 {
			return "TRUE"
		}
		return "FALSE"
	case CellNumber:
		if looksLikeChartDateFormat(cell.Style.NumFmtCode) {
			return formatChartExcelDate(cell.NumValue, cell.Style.NumFmtCode)
		}
		return formatChartNumber(cell.NumValue)
	default:
		return cell.StrValue
	}
}

func looksLikeChartDateFormat(numFmt string) bool {
	fmt := strings.ToLower(numFmt)
	// Year, day, or month-name patterns (mmm/mmmm/mmmmm) indicate a date format.
	if strings.Contains(fmt, "y") || strings.Contains(fmt, "d") {
		return true
	}
	// Month-only formats like "mmm", "mmmm" don't contain y/d but are date formats.
	if strings.Contains(fmt, "mmm") {
		return true
	}
	// Numeric month + day patterns (e.g. "m/d", "mm-dd")
	if strings.Contains(fmt, "m") && strings.Contains(fmt, "d") {
		return true
	}
	return false
}

func formatChartExcelDate(serial float64, numFmt string) string {
	day, frac := math.Modf(serial)
	base := time.Date(1899, time.December, 30, 0, 0, 0, 0, time.UTC)
	t := base.AddDate(0, 0, int(day)).Add(time.Duration(frac * 24 * float64(time.Hour)))
	fmt := strings.ToLower(numFmt)

	// Determine which components the format contains.
	hasYear := strings.Contains(fmt, "y")
	hasDay := strings.Contains(fmt, "d")
	hasMonthName := strings.Contains(fmt, "mmm")

	// Full date with year and day.
	if hasYear && hasDay {
		if strings.Contains(fmt, "mmmm") {
			return t.Format("January 2, 2006")
		}
		if strings.Contains(fmt, "mmm") {
			return t.Format("Jan 2, 2006")
		}
		return t.Format("1/2/2006")
	}
	// Month + year (no day).
	if hasYear && !hasDay {
		if strings.Contains(fmt, "mmmm") {
			return t.Format("January 2006")
		}
		if strings.Contains(fmt, "mmm") {
			return t.Format("Jan 2006")
		}
		return t.Format("1/2006")
	}
	// Month name only (no year, no day).
	if hasMonthName && !hasYear && !hasDay {
		if strings.Contains(fmt, "mmmm") {
			return t.Format("January")
		}
		return t.Format("Jan")
	}
	// Fallback for any other date-like format.
	if hasDay {
		return t.Format("1/2/2006")
	}
	return t.Format("1/2/2006")
}

func formatChartNumber(v float64) string {
	if math.Trunc(v) == v {
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// resolveErrorValue normalizes Excel error values to standard error strings.
// Excel stores errors as internal codes; we map them to the familiar #ERROR! format.
func resolveErrorValue(v string) string {
	// Excel error values are stored as special internal strings
	// Common values: "#DIV/0!", "#N/A", "#NAME?", "#NULL!", "#NUM!", "#REF!", "#VALUE!"
	v = strings.TrimSpace(v)
	if v == "" {
		return "#ERROR!"
	}
	// If it's already a proper error format, return it
	if strings.HasPrefix(v, "#") && strings.HasSuffix(v, "!") {
		return v
	}
	// Map common Excel error codes
	switch strings.ToUpper(v) {
	case "#DIV/0!", "#DIV/0":
		return "#DIV/0!"
	case "#N/A", "#N/A!":
		return "#N/A"
	case "#NAME?", "#NAME":
		return "#NAME?"
	case "#NULL!", "#NULL":
		return "#NULL!"
	case "#NUM!", "#NUM":
		return "#NUM!"
	case "#REF!", "#REF":
		return "#REF!"
	case "#VALUE!", "#VALUE":
		return "#VALUE!"
	default:
		// If it looks like an error code but isn't recognized,
		// format it consistently
		if strings.HasPrefix(v, "#") {
			return v
		}
		return "#ERROR!"
	}
}

// copyCfvo copies a slice of parts.Cfvo to a slice of CFCfvo.
func copyCfvo(src []parts.Cfvo) []CFCfvo {
	if len(src) == 0 {
		return nil
	}
	dst := make([]CFCfvo, len(src))
	for i, v := range src {
		dst[i] = CFCfvo{Type: v.Type, Val: v.Val}
	}
	return dst
}

func resolveEmbeddedFonts(fonts []*parts.EmbeddedFontData) []*EmbeddedFont {
	out := make([]*EmbeddedFont, 0, len(fonts))
	for _, font := range fonts {
		if font == nil || strings.TrimSpace(font.Typeface) == "" {
			continue
		}
		resolved := &EmbeddedFont{Family: strings.TrimSpace(font.Typeface)}
		appendFace := func(part *parts.FontPart, weight int, style string) {
			if part == nil || len(part.Data) == 0 {
				return
			}
			resolved.Faces = append(resolved.Faces, &EmbeddedFontFace{
				Family:      resolved.Family,
				Weight:      weight,
				Style:       style,
				ContentType: part.ContentType,
				Data:        part.Data,
			})
		}
		appendFace(font.Regular, 400, "normal")
		appendFace(font.Bold, 700, "normal")
		appendFace(font.Italic, 400, "italic")
		appendFace(font.BoldItalic, 700, "italic")
		if len(resolved.Faces) == 0 {
			continue
		}
		out = append(out, resolved)
	}
	return out
}
