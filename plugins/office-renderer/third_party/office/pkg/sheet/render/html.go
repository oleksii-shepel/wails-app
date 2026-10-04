package render

import (
	"encoding/base64"
	"fmt"
	"math"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"office/pkg/fontmetrics"
	"office/pkg/ooxml/chart"
	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/htmlasset"
	"office/pkg/ooxml/smartart"
	smartartRender "office/pkg/ooxml/smartart/render"
	drawingcss "office/pkg/render/drawingcss"
	"office/pkg/sheet/parsers/ooxml/formula"
	"office/pkg/sheet/parsers/ooxml/resolve"
)

type HTMLImageMode string

type HTMLFontMode string

const (
	HTMLImageModeInline   HTMLImageMode = "inline"
	HTMLImageModeExternal HTMLImageMode = "external"

	HTMLFontModeInline   HTMLFontMode = "inline"
	HTMLFontModeExternal HTMLFontMode = "external"

	// Character-unit to pixel conversion used for worksheet layout. Excel's
	// column unit is the advance of "0" in the default font, so a column sized
	// for N characters must be able to display N of them: the Calibri 11pt zero
	// advances 7.5px at 96 DPI, not the 7px integer an "MDW=7" reading suggests.
	// Using 7 made every column ~7% narrower than Excel renders it, so cell
	// content overflowed columns that Excel displays in full.
	charUnitToPx = 7.5
	// Default worksheet column width in pixels when not specified.
	defaultColPx = 64
	// Default worksheet row height in pixels.
	defaultRowPx = 18
)

type HTMLAsset = htmlasset.Asset

type HTMLOptions struct {
	ImageMode                HTMLImageMode
	ExternalImageDir         string
	FontMode                 HTMLFontMode
	ExternalFontDir          string
	ExternalWorkbookResolver resolve.ExternalWorkbookResolver
}

type htmlRenderer struct {
	wb               *resolve.Workbook
	options          HTMLOptions
	embeddedFamilies map[string]struct{}
	assets           []HTMLAsset
	nextImageIndex   int
	nextFontIndex    int
	// printPages is set when sheets are rendered as slices of physical print pages.
	printPages bool
}

type dataValidationState struct {
	Evaluated bool
	Valid     bool
}

// HTML renders a resolved Workbook as an HTML document.
// Each sheet becomes a section with a heading and a grid table that grows to
// the worksheet's full height, sized to cover anchored drawings and panels, so
// the page scrolls rather than the sheet box.
func HTML(wb *resolve.Workbook) (string, error) {
	html, _, err := HTMLWithOptions(wb, HTMLOptions{})
	return html, err
}

// HTMLWithOptions renders a resolved Workbook and returns any sidecar assets.
func HTMLWithOptions(wb *resolve.Workbook, opts HTMLOptions) (string, []HTMLAsset, error) {
	if wb == nil {
		return "", nil, fmt.Errorf("sheet render: workbook is nil")
	}
	if err := opts.validate(); err != nil {
		return "", nil, err
	}
	if opts.ExternalWorkbookResolver != nil {
		if err := resolve.RefreshExternalWorkbooks(wb, opts.ExternalWorkbookResolver); err != nil {
			return "", nil, err
		}
	}
	r := &htmlRenderer{
		wb:               wb,
		options:          opts.normalized(),
		embeddedFamilies: buildEmbeddedFamilyIndex(wb),
	}
	return r.render()
}

func (r *htmlRenderer) render() (string, []HTMLAsset, error) {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<style>\n")
	r.writeEmbeddedFontCSS(&b)
	b.WriteString(sheetCSS)
	b.WriteString("</style>\n<script>\n")
	b.WriteString(sheetJS)
	b.WriteString("</script>\n</head>\n<body>\n")

	r.renderWorkbookExternalLinks(&b)
	r.renderWorkbookConnections(&b)

	for i, sheet := range r.wb.Sheets {
		r.renderSheet(&b, sheet, i)
	}

	r.renderDiagnostics(&b)

	b.WriteString("</body>\n</html>")
	return b.String(), r.assetsCopy(), nil
}

func (r *htmlRenderer) renderWorkbookExternalLinks(b *strings.Builder) {
	if r == nil || r.wb == nil || len(r.wb.ExternalLinks) == 0 {
		return
	}
	b.WriteString("<section class=\"workbook-external-links\" aria-label=\"External workbook references\">\n")
	b.WriteString("<h2 class=\"workbook-external-links-title\">External References</h2>\n")
	b.WriteString("<p class=\"workbook-external-links-note\">External link metadata is preserved, and direct external cell or range refs can use cached external workbook values when present, but external workbooks are not opened or refreshed during HTML rendering.</p>\n")
	b.WriteString("<table class=\"workbook-external-links-table\">\n")
	b.WriteString("<thead><tr><th>Workbook</th><th>Sheets</th><th>Defined Names</th></tr></thead>\n<tbody>\n")
	for _, link := range r.wb.ExternalLinks {
		if link == nil {
			continue
		}
		fmt.Fprintf(b, "<tr><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			escapeHTML(link.Target),
			escapeHTML(strings.Join(link.SheetNames, ", ")),
			escapeHTML(workbookExternalDefinedNamesSummary(link.DefinedNames)),
		)
	}
	b.WriteString("</tbody></table>\n</section>\n")
}

func workbookExternalDefinedNamesSummary(names []*resolve.WorkbookExternalDefinedName) string {
	if len(names) == 0 {
		return ""
	}
	parts := make([]string, 0, len(names))
	for _, name := range names {
		if name == nil || strings.TrimSpace(name.Name) == "" {
			continue
		}
		label := name.Name
		if strings.TrimSpace(name.RefersTo) != "" {
			label += "=" + name.RefersTo
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, ", ")
}

func (r *htmlRenderer) renderWorkbookConnections(b *strings.Builder) {
	if r == nil || r.wb == nil || len(r.wb.Connections) == 0 {
		return
	}
	b.WriteString("<section class=\"workbook-connections\" aria-label=\"External data connections\">\n")
	b.WriteString("<h2 class=\"workbook-connections-title\">External Data Connections</h2>\n")
	b.WriteString("<p class=\"workbook-connections-note\">Connection metadata is preserved, but external queries are not executed during HTML rendering.</p>\n")
	b.WriteString("<table class=\"workbook-connections-table\">\n")
	b.WriteString("<thead><tr><th>Name</th><th>Kind</th><th>Target</th><th>Options</th></tr></thead>\n<tbody>\n")
	for _, conn := range r.wb.Connections {
		if conn == nil {
			continue
		}
		target := workbookConnectionTarget(conn)
		options := workbookConnectionOptions(conn)
		fmt.Fprintf(b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			escapeHTML(conn.Name),
			escapeHTML(conn.Kind),
			escapeHTML(target),
			escapeHTML(options),
		)
	}
	b.WriteString("</tbody></table>\n</section>\n")
}

func workbookConnectionTarget(conn *resolve.WorkbookConnection) string {
	if conn == nil {
		return ""
	}
	for _, target := range []string{conn.URL, conn.TextFile, conn.SourceFile, conn.ODCFile, conn.Connection, conn.LocalCube} {
		if strings.TrimSpace(target) != "" {
			return target
		}
	}
	return conn.Description
}

func workbookConnectionOptions(conn *resolve.WorkbookConnection) string {
	if conn == nil {
		return ""
	}
	var options []string
	if conn.RefreshOnLoad {
		options = append(options, "refreshOnLoad")
	}
	if conn.Background {
		options = append(options, "background")
	}
	if conn.SaveData {
		options = append(options, "saveData")
	}
	if conn.Command != "" {
		options = append(options, conn.Command)
	}
	if conn.WebPost != "" {
		options = append(options, "post="+conn.WebPost)
	}
	if conn.WebHTMLTables {
		options = append(options, "htmlTables")
	}
	if conn.WebSourceData {
		options = append(options, "sourceData")
	}
	if conn.WebParsePre {
		options = append(options, "parsePre")
	}
	if conn.WebConsecutive {
		options = append(options, "consecutive")
	}
	if conn.WebFirstRow {
		options = append(options, "firstRow")
	}
	if conn.WebTextDates {
		options = append(options, "textDates")
	}
	if conn.TextDelimiter != "" {
		options = append(options, "delimiter="+conn.TextDelimiter)
	}
	return strings.Join(options, ", ")
}

func (r *htmlRenderer) renderSheet(b *strings.Builder, sheet *resolve.Sheet, sheetIndex int) {
	if r.printPages {
		b.WriteString("<section class=\"sheet\">\n")
	} else {
		fmt.Fprintf(b, "<div class=\"sheet-num\">Sheet %d</div>\n", sheetIndex+1)
		fmt.Fprintf(b, "<section class=\"sheet\" id=\"sheet-%d\" data-sheet=\"%d\" data-sheet-name=\"%s\">\n", sheetIndex+1, sheetIndex+1, escapeHTML(sheet.Name))
	}
	fmt.Fprintf(b, "<h2 class=\"sheet-name\">%s</h2>\n", escapeHTML(sheet.Name))
	rowOutlineMax := maxOutlineLevel(sheet.OutlineRows)
	colOutlineMax := maxOutlineLevel(sheet.OutlineCols)
	if rowOutlineMax > 0 || colOutlineMax > 0 {
		renderOutlineControls(b, rowOutlineMax, colOutlineMax)
	}

	// Render pane metadata if frozen or split panes are present
	if sheet.FrozenRows > 0 || sheet.FrozenCols > 0 || sheet.SplitRows > 0 || sheet.SplitCols > 0 ||
		sheet.TopLeftCell != "" || sheet.ActivePane != "" {
		renderPaneMetadata(b, sheet)
	}

	var metaAttrs []string
	metaAttrs = append(metaAttrs, "class=\"sheet-meta\"", "hidden")
	if sheet.PageSetup != nil {
		ps := sheet.PageSetup
		if ps.PaperSize > 0 {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-page-paper-size=%q", strconv.Itoa(ps.PaperSize)))
		}
		if ps.Orientation != "" {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-page-orientation=%q", ps.Orientation))
		}
		if ps.Scale > 0 {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-page-scale=%q", strconv.Itoa(ps.Scale)))
		}
		if ps.FitToPage {
			metaAttrs = append(metaAttrs, "data-page-fit-to-page=\"true\"")
		}
		if ps.FitToWidth > 0 {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-page-fit-to-width=%q", strconv.Itoa(ps.FitToWidth)))
		}
		if ps.FitToHeight > 0 {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-page-fit-to-height=%q", strconv.Itoa(ps.FitToHeight)))
		}
		if ps.MarginLeft > 0 {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-margin-left=%q", strconv.FormatFloat(ps.MarginLeft, 'f', -1, 64)))
		}
		if ps.MarginRight > 0 {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-margin-right=%q", strconv.FormatFloat(ps.MarginRight, 'f', -1, 64)))
		}
		if ps.MarginTop > 0 {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-margin-top=%q", strconv.FormatFloat(ps.MarginTop, 'f', -1, 64)))
		}
		if ps.MarginBottom > 0 {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-margin-bottom=%q", strconv.FormatFloat(ps.MarginBottom, 'f', -1, 64)))
		}
		if ps.MarginHeader > 0 {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-margin-header=%q", strconv.FormatFloat(ps.MarginHeader, 'f', -1, 64)))
		}
		if ps.MarginFooter > 0 {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-margin-footer=%q", strconv.FormatFloat(ps.MarginFooter, 'f', -1, 64)))
		}
		if sheet.PrintTitleRows != "" {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-print-title-rows=%q", sheet.PrintTitleRows))
		}
		if sheet.PrintTitleCols != "" {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-print-title-cols=%q", sheet.PrintTitleCols))
		}
	} else if sheet.PrintTitleRows != "" || sheet.PrintTitleCols != "" {
		if sheet.PrintTitleRows != "" {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-print-title-rows=%q", sheet.PrintTitleRows))
		}
		if sheet.PrintTitleCols != "" {
			metaAttrs = append(metaAttrs, fmt.Sprintf("data-print-title-cols=%q", sheet.PrintTitleCols))
		}
	}
	if sheet.ZoomScale > 0 {
		metaAttrs = append(metaAttrs, fmt.Sprintf("data-zoom-scale=%q", strconv.Itoa(sheet.ZoomScale)))
	}
	if sheet.RowGutter > 0 {
		metaAttrs = append(metaAttrs, fmt.Sprintf("data-row-gutter=%q", strconv.Itoa(sheet.RowGutter)))
	}
	if sheet.ColGutter > 0 {
		metaAttrs = append(metaAttrs, fmt.Sprintf("data-col-gutter=%q", strconv.Itoa(sheet.ColGutter)))
	}
	if sheet.RowOutlineLevelMax > 0 {
		metaAttrs = append(metaAttrs, fmt.Sprintf("data-row-outline-max=%q", strconv.Itoa(sheet.RowOutlineLevelMax)))
	}
	if sheet.ColOutlineLevelMax > 0 {
		metaAttrs = append(metaAttrs, fmt.Sprintf("data-col-outline-max=%q", strconv.Itoa(sheet.ColOutlineLevelMax)))
	}
	if sheet.DefaultRowHeightUnsynced {
		metaAttrs = append(metaAttrs, "data-row-height-unsynced=\"true\"")
	}
	if sheet.DefaultRowHeightZeroHeight {
		metaAttrs = append(metaAttrs, "data-row-height-zero-height=\"true\"")
	}
	if sheet.DefaultRowHeightExtraSpaceAbove {
		metaAttrs = append(metaAttrs, "data-row-height-extra-above=\"true\"")
	}
	if sheet.DefaultRowHeightExtraSpaceBelow {
		metaAttrs = append(metaAttrs, "data-row-height-extra-below=\"true\"")
	}
	metaAttrs = appendProtectionMetaAttrs(metaAttrs, r.wb.Protection, sheet.Protection)
	if len(metaAttrs) > 2 {
		fmt.Fprintf(b, "<div %s></div>\n", strings.Join(metaAttrs, " "))
	}
	renderProtectionNotice(b, r.wb.Protection, sheet.Protection)

	// Anchored drawings and form controls are absolutely positioned and add no
	// height to grid-wrap on their own, so the wrapper is sized to the lowest
	// overlay edge; shapes and instruction panels that reach below the authored
	// rows stay visible without scrolling inside the box. The reserved space is
	// the wrapper border — border-box sizing counts it inside min-height while
	// overlays are positioned from the padding box — plus the horizontal
	// scrollbar a wide sheet needs, which would otherwise eat into the panel's
	// last pixels. The sheet script then pads the grid with cells to fill
	// whatever box the viewer shows, so no space is reserved beyond this.
	hasOverlay := len(sheet.Drawings) > 0 || len(sheet.FormControls) > 0
	var overlay drawingOverlay
	if hasOverlay {
		overlay = r.buildDrawingOverlay(sheet, sheetIndex)
	}
	if overlay.bottomPx > 0 {
		minHeight := overlay.bottomPx + 2*gridWrapBorderPx + gridWrapScrollbarPx
		fmt.Fprintf(b, "<div class=\"grid-wrap\" style=\"min-height:%dpx\">\n", minHeight)
	} else {
		b.WriteString("<div class=\"grid-wrap\">\n")
	}
	// The column model drives the table for every sheet: cells are sized from
	// the same widths the drawing layer is positioned with, so anchored objects
	// land on top of cells rather than beside them.
	gridClasses := []string{"grid", "grid--fixed"}
	gridClassAttr := strings.Join(gridClasses, " ")

	// Seed a workbook-aware formula engine for formula display and validation evaluation.
	workbookCtx := buildWorkbookFormulaContext(r.wb)
	formulaEngine := buildFormulaEngine(sheet, r.wb, sheetIndex)
	if workbookCtx != nil {
		if eng := workbookCtx.engineForSheet(sheetIndex); eng != nil {
			formulaEngine = eng
		}
	}

	// Determine the physical column envelope once from every worksheet feature
	// that occupies grid coordinates. Sparse drawings and controls are part of
	// the worksheet geometry even when there are no populated cells beneath them.
	maxCol := worksheetDataColumns(sheet)
	for _, group := range formulaEngine.ArrayFormulaGroups() {
		if group.RightCol > maxCol {
			maxCol = group.RightCol
		}
	}
	// Cells cover the whole sheet envelope, so panels sit on the grid instead
	// of beside it and the sheet is not cropped to its populated range.
	maxCol = extendColumnsToOverlay(sheet, maxCol, overlay)

	// Emit <colgroup> for explicit/default column widths, hidden columns, and outline levels.
	var colgroup strings.Builder
	gridWidthPx := 0
	{
		defaultColPxV := worksheetDefaultColumnPx(sheet)
		colgroup.WriteString("<colgroup>\n")
		for c := 1; c <= maxCol; c++ {
			var colAttrs []string
			if level := sheet.OutlineCols[c]; level > 0 {
				colAttrs = append(colAttrs, fmt.Sprintf("data-outline-level=%q", strconv.Itoa(level)))
				colAttrs = append(colAttrs, fmt.Sprintf("data-outline-col-level=%q", strconv.Itoa(level)))
			}
			if sheet.CollapsedCols[c] {
				colAttrs = append(colAttrs, "data-outline-collapsed=\"true\"")
			}
			if sheet.HiddenCols[c] {
				colAttrs = append(colAttrs, "data-base-hidden=\"true\"")
				colAttrs = append(colAttrs, "style=\"visibility:collapse\"")
			} else {
				width := worksheetColumnVisualWidthPx(sheet, c, defaultColPxV)
				colAttrs = append(colAttrs, fmt.Sprintf("style=\"width:%dpx\"", width))
				gridWidthPx += width
			}
			if len(colAttrs) == 0 {
				colgroup.WriteString("<col>\n")
			} else {
				fmt.Fprintf(&colgroup, "<col %s>\n", strings.Join(colAttrs, " "))
			}
		}
		colgroup.WriteString("</colgroup>\n")
	}
	// Pin the table to the modelled column widths. With table-layout:fixed and an
	// auto table width, browsers size columns from cell content, which stretches
	// the grid past the column model the drawing layer is positioned with, so
	// anchored panels and pictures land on top of the cells.
	// data-col-px / data-row-px carry the modelled default cell size so the
	// sheet script can pad the grid with cells of the right size.
	cellSizeAttrs := fmt.Sprintf("data-col-px=%q data-row-px=%q",
		strconv.Itoa(worksheetDefaultColumnPx(sheet)), strconv.Itoa(worksheetDefaultRowPx(sheet)))
	if gridWidthPx > 0 {
		fmt.Fprintf(b, "<table class=%q style=%q %s>\n", gridClassAttr, fmt.Sprintf("width:%dpx", gridWidthPx), cellSizeAttrs)
	} else {
		fmt.Fprintf(b, "<table class=%q %s>\n", gridClassAttr, cellSizeAttrs)
	}
	b.WriteString(colgroup.String())

	arrayFormulaCells, arrayFormulaRows := buildArrayFormulaRenderMap(sheet, formulaEngine)
	// Build CF style map: cell ref → winning DxfStyle (lowest priority wins, evaluated per cell).
	cfMap := evaluateCFRules(sheet, arrayFormulaCells, formulaEngine)
	// Build CF visual map: cell ref → data bars / icon sets / color scales.
	cfVisualMap := buildCFVisualMap(sheet, arrayFormulaCells, formulaEngine)
	// Build data-validation map: cell ref → DataValidation.
	dvMap := buildDVMap(sheet.DataValidations)
	// Determine autoFilter header row (top row of the filter range).
	afHeaderRow, afColMin, afColMax := parseAutoFilterHeader(sheet.AutoFilterRef)
	// Build the set of 0-based-within-range column indices that have active filter criteria.
	afActiveSet := buildAFActiveSet(sheet.FilterColumns)
	// Evaluate formula cells that have no cached display value.
	computedValues := buildComputedValues(sheet, arrayFormulaCells, formulaEngine)
	formulaSparklines := buildFormulaSparklines(sheet, formulaEngine)
	computedHyperlinks := buildComputedHyperlinks(sheet, arrayFormulaCells, formulaEngine)
	// Build data-validation state map: cell ref → current pass/fail state.
	dvStateMap := buildDVStateMap(r.wb, sheetIndex, sheet.DataValidations)
	// Build table cell visual flags.
	tableFlags := buildTableCellFlags(sheet.Tables)

	// Build a sparse row map for O(1) lookup.
	rowMap := make(map[int]*resolve.Row, len(sheet.Rows))
	minRow, maxRow := math.MaxInt32, 0
	for _, r := range sheet.Rows {
		rowMap[r.Index] = r
		if r.Index < minRow {
			minRow = r.Index
		}
		if r.Index > maxRow {
			maxRow = r.Index
		}
	}
	mergeLayout := buildMergeLayout(sheet.MergedCells)
	if mergeLayout.minRow > 0 && mergeLayout.minRow < minRow {
		minRow = mergeLayout.minRow
	}
	if mergeLayout.maxRow > maxRow {
		maxRow = mergeLayout.maxRow
	}
	for row := range arrayFormulaRows {
		if row < minRow {
			minRow = row
		}
		if row > maxRow {
			maxRow = row
		}
	}
	// Rows fill the sheet envelope the same way columns do, so panels sit on
	// the grid rather than past its last row.
	maxRow = extendRowsToOverlay(sheet, rowMap, maxRow, overlay)
	// Empty rows still occupy grid height; drawings and comments are anchored
	// to a grid that starts at row 1.
	if maxRow > 0 {
		minRow = 1
	}

	for ri := minRow; ri <= maxRow; ri++ {
		row, ok := rowMap[ri]
		if !ok {
			row = &resolve.Row{Index: ri}
		}

		var rowStyles []string
		if row.HeightPt > 0 {
			rowStyles = append(rowStyles, fmt.Sprintf("height:%.2fpt", row.HeightPt))
		} else if sheet.DefaultRowHeightPt > 0 {
			rowStyles = append(rowStyles, fmt.Sprintf("height:%.2fpt", sheet.DefaultRowHeightPt))
		}
		if row.Hidden {
			rowStyles = append(rowStyles, "display:none")
		}
		if row.Style != nil && row.Style.BgColor != "" {
			rowStyles = append(rowStyles, fmt.Sprintf("background:#%s", row.Style.BgColor))
		}

		isFrozen := ri <= sheet.FrozenRows
		rowClass := "row"
		if isFrozen {
			rowClass = "row row--frozen"
		}
		var rowAttrs []string
		rowAttrs = append(rowAttrs, fmt.Sprintf("class=%q", rowClass))
		if row.OutlineLevel > 0 {
			rowAttrs = append(rowAttrs, fmt.Sprintf("data-outline-level=%q", strconv.Itoa(row.OutlineLevel)))
		}
		if row.Collapsed {
			rowAttrs = append(rowAttrs, "data-outline-collapsed=\"true\"")
		}
		if row.Hidden {
			rowAttrs = append(rowAttrs, "data-base-hidden=\"true\"")
		}
		if len(rowStyles) > 0 {
			rowAttrs = append(rowAttrs, fmt.Sprintf("style=%q", strings.Join(rowStyles, ";")))
		}
		fmt.Fprintf(b, "<tr %s>\n", strings.Join(rowAttrs, " "))

		// Cell maps for this row and the row below. The lower row lets an
		// authored border win the collapse conflict against the default grid
		// drawn by an unstyled cell above it.
		cellMap := cellsByColumn(rowMap[ri])
		belowRow := rowMap[ri+1]
		belowCells := cellsByColumn(belowRow)
		belowRowPainted := belowRow == nil || !belowRow.Hidden

		for ci := 1; ci <= maxCol; ci++ {
			if mergeLayout.isCovered(ri, ci) {
				continue
			}
			isFrozenCol := ci <= sheet.FrozenCols
			ref := colLetterFromIndex(ci) + strconv.Itoa(ri)
			cell := cellMap[ci]
			if spillCell := arrayFormulaCells[ref]; spillCell != nil {
				if cell == nil {
					cell = spillCell
				} else if cell.ArrayFormulaRange == "" || cell.ArrayFormulaAnchor == "" {
					cellCopy := *cell
					if cellCopy.Formula == "" {
						cellCopy.Formula = spillCell.Formula
					}
					if cellCopy.ArrayFormulaRange == "" {
						cellCopy.ArrayFormulaRange = spillCell.ArrayFormulaRange
					}
					if cellCopy.ArrayFormulaAnchor == "" {
						cellCopy.ArrayFormulaAnchor = spillCell.ArrayFormulaAnchor
					}
					cell = &cellCopy
				}
			}
			cf, hasCF := cfMap[ref]
			dv := dvMap[ref]
			comment := sheet.Comments[ref]
			isAFHeader := ri == afHeaderRow && ci >= afColMin && ci <= afColMax
			isAFActive := isAFHeader && afActiveSet[ci-afColMin]
			tf := tableFlags[ref]
			merged := mergeLayout.anchorAt(ri, ci)
			display := buildCellDisplayGeometry(sheet, cellMap, ci, merged)
			// Border-collapse resolves a shared edge in favour of the cell
			// further left (and further up), so an unstyled cell would hide the
			// authored border of the cell to its right or below. Drop the
			// default grid edge there so the authored border shows.
			edges := gridBorderEdges{
				suppressRight: !sheet.HiddenCols[ci+1] && !mergeLayout.isCovered(ri, ci+1) &&
					authorsBorder(cellMap[ci+1], borderSideLeft),
				suppressBottom: belowRowPainted && !mergeLayout.isCovered(ri+1, ci) &&
					authorsBorder(belowCells[ci], borderSideTop),
			}
			computedValue, hasComputedValue := computedValues[ref]
			renderCell(b, cell, ci, sheet.OutlineCols[ci], isFrozenCol, merged, r.embeddedFamilies, sheet.HiddenCols[ci], hasCF, cf, cfVisualMap[ref], dv, dvStateMap[ref], comment, isAFHeader, isAFActive, computedValue, hasComputedValue, computedHyperlinks[ref], formulaSparklines[ref], tf, display, edges)
		}
		b.WriteString("</tr>\n")
	}

	b.WriteString("</table>\n")

	// Render the drawings overlay inside the grid wrapper so drawings scroll
	// horizontally with wide sheets and stay positioned relative to the grid.
	if hasOverlay {
		r.renderDrawingsLayer(b, overlay)
	}

	b.WriteString("</div>\n")

	b.WriteString("</section>\n")
}

type cellDisplayGeometry struct {
	widthPx             int
	hasExplicitGeometry bool
	nextCellOccupied    bool
	merged              bool
}

// buildCellDisplayGeometry is the single worksheet-cell box calculation used by
// display decisions. Merged anchors own the sum of all visible columns in the
// merged range; hidden columns contribute no visual width.
func buildCellDisplayGeometry(sheet *resolve.Sheet, rowCells map[int]*resolve.Cell, col int, merged *resolve.MergedCell) cellDisplayGeometry {
	if sheet == nil || col <= 0 {
		return cellDisplayGeometry{}
	}
	defaultWidth := defaultColPx
	if sheet.DefaultColWidthChars > 0 {
		defaultWidth = worksheetColumnWidthPx(sheet.DefaultColWidthChars)
	}
	right := col
	if merged != nil && merged.RightCol >= col {
		right = merged.RightCol
	}
	width := 0
	for c := col; c <= right; c++ {
		width += worksheetColumnVisualWidthPx(sheet, c, defaultWidth)
	}
	_, nextOccupied := rowCells[right+1]
	return cellDisplayGeometry{
		widthPx:             width,
		hasExplicitGeometry: hasGridColumnLayout(sheet),
		nextCellOccupied:    nextOccupied,
		merged:              right > col,
	}
}

func shouldRenderNumericOverflowHashes(cell *resolve.Cell, text string, display cellDisplayGeometry) bool {
	if cell == nil || (cell.Kind != resolve.CellNumber && cell.Kind != resolve.CellDate) || text == "" || !display.hasExplicitGeometry || display.widthPx <= 0 {
		return false
	}
	if cell.Style.WrapText || cell.Style.ShrinkToFit {
		return false
	}
	fontPt := cell.Style.FontSizePt
	if fontPt <= 0 {
		fontPt = 11
	}
	contentPx := float64(display.widthPx - 8) // renderer cell horizontal padding
	if contentPx <= 0 {
		return false
	}
	if textPx, ok := excelTextWidthPx(text, cell.Style.FontFamily, cell.Style.Bold, cell.Style.Italic, fontPt); ok {
		// Excel keeps a value while it fits the column less its ~3px cell margins.
		return textPx > float64(display.widthPx-3)
	}
	// Without metrics for the family, approximate the glyph advance and only
	// emit hashes when the value clearly cannot fit the authored box.
	avgGlyphPx := fontPt * 4.0 / 3.0 * 0.52
	return float64(len([]rune(text)))*avgGlyphPx > contentPx+avgGlyphPx
}

var bundledFontCatalog = sync.OnceValue(fontmetrics.BundledCatalog)

// excelTextWidthPx measures text the way Excel's GDI layout does at 96 DPI:
// each glyph advance is hinted to whole pixels.
func excelTextWidthPx(text, family string, bold, italic bool, fontPt float64) (float64, bool) {
	catalog := bundledFontCatalog()
	sizePx := fontPt * 4 / 3
	total := 0.0
	for _, r := range text {
		advance, ok := catalog.MeasureText(string(r), family, bold, italic, sizePx)
		if !ok {
			return 0, false
		}
		total += math.Round(advance)
	}
	return total, true
}

func numericOverflowHashes(widthPx int) string {
	n := (widthPx - 8) / 7
	if n < 1 {
		n = 1
	}
	return strings.Repeat("#", n)
}

func shouldAllowTextOverflow(cell *resolve.Cell, text string, display cellDisplayGeometry) bool {
	if cell == nil || cell.Kind != resolve.CellString || text == "" || !display.hasExplicitGeometry || display.merged {
		return false
	}
	if cell.Style.WrapText || cell.Style.ShrinkToFit || display.nextCellOccupied {
		return false
	}
	align := cell.Style.HAlign
	return align == "" || align == "general" || align == "left"
}

// gridBorderEdges marks the default grid edges a cell must not draw because the
// neighbouring cell authors its own border on the shared edge.
type gridBorderEdges struct {
	suppressRight  bool
	suppressBottom bool
}

// borderSide selects one edge of a cell style for authorsBorder.
type borderSide int

const (
	borderSideLeft borderSide = iota
	borderSideTop
)

// authorsBorder reports whether a cell carries an authored border on the given
// edge, which has to survive the border-collapse conflict with a neighbour's
// default grid line.
func authorsBorder(cell *resolve.Cell, side borderSide) bool {
	if cell == nil {
		return false
	}
	switch side {
	case borderSideLeft:
		return cell.Style.BorderLeft.Style != ""
	default:
		return cell.Style.BorderTop.Style != ""
	}
}

// cellsByColumn indexes a row's cells for O(1) neighbour lookups. Missing rows
// yield a nil map so absent neighbours read as unstyled.
func cellsByColumn(row *resolve.Row) map[int]*resolve.Cell {
	if row == nil || len(row.Cells) == 0 {
		return nil
	}
	cells := make(map[int]*resolve.Cell, len(row.Cells))
	for _, cell := range row.Cells {
		cells[cell.Col] = cell
	}
	return cells
}

func renderCell(b *strings.Builder, cell *resolve.Cell, colIndex int, outlineColLevel int, frozen bool, merged *resolve.MergedCell, embeddedFamilies map[string]struct{}, hiddenCol bool, hasCF bool, cfStyle resolve.DxfStyle, cfVisual cfVisualData, dv *resolve.DataValidation, dvState dataValidationState, commentText string, isAFHeader bool, isAFActive bool, computedValue string, hasComputedValue bool, computedHyperlink string, formulaSparkline string, tf tableCellFlags, display cellDisplayGeometry, edges gridBorderEdges) {
	if hiddenCol {
		var attrs []string
		attrs = append(attrs, "style=\"visibility:collapse\"", "data-base-hidden=\"true\"")
		if outlineColLevel > 0 {
			attrs = append(attrs, fmt.Sprintf("data-outline-col-level=%q", strconv.Itoa(outlineColLevel)))
		}
		if colIndex > 0 {
			attrs = append(attrs, fmt.Sprintf("data-col-index=%q", strconv.Itoa(colIndex)))
		}
		fmt.Fprintf(b, "<td %s></td>\n", strings.Join(attrs, " "))
		return
	}
	tag := "td"
	class := "cell"
	if frozen {
		class = "cell cell--frozen-col"
	}
	if isAFHeader {
		class += " cell--af-header"
	}
	if isAFActive {
		class += " cell--af-active"
	}
	if commentText != "" {
		class += " cell--has-comment"
	}
	if cell != nil && cell.Formula != "" {
		class += " cell--formula"
	}
	if formulaSparkline != "" {
		class += " cell--sparkline"
	}
	if cell != nil && cell.ArrayFormulaRange != "" {
		class += " cell--array-formula"
	}
	if dvState.Evaluated {
		if dvState.Valid {
			class += " cell--validation-valid"
		} else {
			class += " cell--validation-invalid"
		}
	}
	if tf.isTableHeader {
		class += " cell--table-header"
	}
	if tf.isTableTotal {
		class += " cell--table-total"
	}
	if tf.isBandedRow {
		class += " cell--table-banded-row"
	}
	if tf.isBandedCol {
		class += " cell--table-banded-col"
	}
	if tf.isFirstCol {
		class += " cell--table-first-col"
	}
	if tf.isLastCol {
		class += " cell--table-last-col"
	}

	var styles []string
	var text string
	var align string

	// Default grid edges the neighbouring cell overrides with an authored
	// border. Authored and conditional-format borders appended below win over
	// these, so the suppression only removes the fallback grid line.
	if edges.suppressRight {
		styles = append(styles, "border-right:none")
	}
	if edges.suppressBottom {
		styles = append(styles, "border-bottom:none")
	}

	if cell != nil {
		s := cell.Style
		if shouldOverrideDefaultGridBorder(s) {
			styles = append(styles, "border:none")
		}
		if s.Bold {
			styles = append(styles, "font-weight:bold")
		}
		if s.Italic {
			styles = append(styles, "font-style:italic")
		}
		var decorations []string
		if s.Underline {
			decorations = append(decorations, "underline")
		}
		if s.Strike {
			decorations = append(decorations, "line-through")
		}
		if len(decorations) > 0 {
			styles = append(styles, "text-decoration:"+strings.Join(decorations, " "))
			appendUnderlineVariantStyles(&styles, s.UnderlineType)
		}
		if s.FontVertAlign == "superscript" || s.FontVertAlign == "subscript" {
			styles = append(styles, "font-variant-position:"+strings.TrimSuffix(s.FontVertAlign, "script"))
		}
		if s.FontSizePt > 0 {
			styles = append(styles, fmt.Sprintf("font-size:%.2fpt", s.FontSizePt))
		}
		if s.FontFamily != "" {
			styles = append(styles, "font-family:"+cssFontFamily(s.FontFamily, !isEmbeddedFont(embeddedFamilies, s.FontFamily)))
		}
		if s.FontColor != "" {
			styles = append(styles, "color:#"+s.FontColor)
		}
		appendFillStyles(&styles, s)
		appendBorderStyles(&styles, s)
		if s.WrapText {
			styles = append(styles, "white-space:normal")
		}
		if s.ShrinkToFit {
			styles = append(styles, "overflow:hidden", "text-overflow:clip")
		}
		if s.Indent > 0 {
			styles = append(styles, fmt.Sprintf("padding-left:%.2fem", float64(s.Indent)))
		}
		if s.VAlign != "" {
			styles = append(styles, "vertical-align:"+s.VAlign)
		}
		align = s.HAlign
		if align == "" || align == "general" {
			align = defaultAlign(cell.Kind)
		}

		text = formatCellValue(cell)
		// Prefer formula-engine output when it differs from a stale cached display.
		if hasComputedValue {
			text = computedValue
		}
		if hasCF && cfStyle.NumFmtCode != "" && (cell.Kind == resolve.CellNumber || cell.Kind == resolve.CellDate) {
			text = FormatValue(cell.NumValue, cfStyle.NumFmtCode)
		}
		if shouldRenderNumericOverflowHashes(cell, text, display) {
			text = numericOverflowHashes(display.widthPx)
		}
		if shouldAllowTextOverflow(cell, text, display) {
			class += " cell--text-overflow"
		}
	}
	hyperlinkURL := ""
	if cell != nil {
		hyperlinkURL = cell.HyperlinkURL
	}
	if hyperlinkURL == "" && computedHyperlink != "" {
		hyperlinkURL = computedHyperlink
	}

	// Apply conditional formatting overrides on top of base style.
	if hasCF {
		if cfStyle.BgColor != "" {
			// Remove any prior background and add the CF one.
			styles = removeStyleProp(styles, "background")
			styles = append(styles, "background:#"+cfStyle.BgColor)
		}
		if cfStyle.FontColor != "" {
			styles = removeStyleProp(styles, "color")
			styles = append(styles, "color:#"+cfStyle.FontColor)
		}
		if cfStyle.Bold != nil {
			styles = removeStyleProp(styles, "font-weight")
			if *cfStyle.Bold {
				styles = append(styles, "font-weight:bold")
			}
		}
		if cfStyle.Italic != nil {
			styles = removeStyleProp(styles, "font-style")
			if *cfStyle.Italic {
				styles = append(styles, "font-style:italic")
			}
		}
		underline, strike := false, false
		if cell != nil {
			underline = cell.Style.Underline
			strike = cell.Style.Strike
		}
		if cfStyle.Underline != nil {
			underline = *cfStyle.Underline
		}
		if cfStyle.Strike != nil {
			strike = *cfStyle.Strike
		}
		styles = removeStyleProp(styles, "text-decoration")
		var cfDecorations []string
		if underline {
			cfDecorations = append(cfDecorations, "underline")
		}
		if strike {
			cfDecorations = append(cfDecorations, "line-through")
		}
		if len(cfDecorations) > 0 {
			styles = append(styles, "text-decoration:"+strings.Join(cfDecorations, " "))
		}

		applyCFBorderOverride(&styles, "left", cfStyle.BorderLeft, cfStyle.LeftBorderColor)
		applyCFBorderOverride(&styles, "right", cfStyle.BorderRight, cfStyle.RightBorderColor)
		applyCFBorderOverride(&styles, "top", cfStyle.BorderTop, cfStyle.TopBorderColor)
		applyCFBorderOverride(&styles, "bottom", cfStyle.BorderBottom, cfStyle.BottomBorderColor)
	}

	// Apply visual conditional formatting: color scale sets background color.
	cellNumVal := 0.0
	if cell != nil {
		cellNumVal = cell.NumValue
	}
	if cfVisual.HasNumericValue {
		cellNumVal = cfVisual.NumericValue
	}
	if cfVisual.ColorScale != nil {
		if bg := renderColorScale(cfVisual.ColorScale, cfVisual.ColorScaleThresholds, cellNumVal); bg != "" {
			styles = removeStyleProp(styles, "background")
			styles = append(styles, "background:"+bg)
		}
	}

	var attrParts []string
	attrParts = append(attrParts, fmt.Sprintf("class=%q", class))
	if colIndex > 0 {
		attrParts = append(attrParts, fmt.Sprintf("data-col-index=%q", strconv.Itoa(colIndex)))
	}
	if outlineColLevel > 0 {
		attrParts = append(attrParts, fmt.Sprintf("data-outline-col-level=%q", strconv.Itoa(outlineColLevel)))
	}
	if merged != nil {
		if rowSpan := merged.RowSpan(); rowSpan > 1 {
			attrParts = append(attrParts, fmt.Sprintf("rowspan=%q", strconv.Itoa(rowSpan)))
		}
		if colSpan := merged.ColSpan(); colSpan > 1 {
			attrParts = append(attrParts, fmt.Sprintf("colspan=%q", strconv.Itoa(colSpan)))
		}
	}
	if align != "" {
		attrParts = append(attrParts, fmt.Sprintf("data-align=%q", align))
		if cssAlign, extra := cssCellHAlign(align); cssAlign != "" {
			styles = append(styles, "text-align:"+cssAlign)
			styles = append(styles, extra...)
		}
	}
	if cell != nil && cell.Formula != "" {
		attrParts = append(attrParts, fmt.Sprintf("data-formula=%q", escapeHTMLAttr(cell.Formula)))
	}
	if cell != nil && cell.ArrayFormulaRange != "" {
		attrParts = append(attrParts, fmt.Sprintf("data-array-formula-range=%q", escapeHTMLAttr(cell.ArrayFormulaRange)))
	}
	if cell != nil && cell.ArrayFormulaAnchor != "" {
		attrParts = append(attrParts, fmt.Sprintf("data-array-formula-anchor=%q", escapeHTMLAttr(cell.ArrayFormulaAnchor)))
	}

	if len(styles) > 0 {
		attrParts = append(attrParts, fmt.Sprintf("style=%q", strings.Join(styles, ";")))
	}

	// Data validation metadata and tooltip.
	if dv != nil {
		if tip := dataValidationTooltip(dv, dvState); tip != "" {
			attrParts = append(attrParts, fmt.Sprintf("title=%q", escapeHTMLAttr(tip)))
		}
		attrParts = append(attrParts, dataValidationAttrs(dv, dvState)...)
	}

	openTag := fmt.Sprintf("<%s %s>", tag, strings.Join(attrParts, " "))
	closeTag := fmt.Sprintf("</%s>\n", tag)

	// AutoFilter header: append a small drop-down indicator after the cell text.
	// Active-filter columns get a filled funnel icon (▾ with af-active class).
	afSuffix := ""
	if isAFActive {
		afSuffix = "<span class=\"af-arrow af-arrow--active\" aria-hidden=\"true\">\u25bc</span>"
	} else if isAFHeader {
		afSuffix = "<span class=\"af-arrow\" aria-hidden=\"true\">\u25be</span>"
	}
	commentSuffix := ""
	if commentText != "" {
		commentSuffix = fmt.Sprintf("<span class=\"comment-mark\" title=%q aria-hidden=\"true\">&#9650;</span>", escapeHTMLAttr(commentText))
	}
	// Data bar and icon set visual overlays.
	iconPrefix := renderIconSet(cfVisual.IconSet, cfVisual.IconSetThresholds, cellNumVal)
	dataBarSuffix := renderDataBar(cfVisual.DataBar, cfVisual.DataBarThresholds, cellNumVal)
	cellSuffix := dataBarSuffix + afSuffix + commentSuffix

	if formulaSparkline != "" {
		b.WriteString(openTag)
		b.WriteString(formulaSparkline)
		b.WriteString(cellSuffix)
		b.WriteString(closeTag)
		return
	}

	// Rich-text runs: render styled spans instead of plain text.
	if cell != nil && len(cell.RichTextRuns) > 0 {
		b.WriteString(openTag)
		if iconPrefix != "" {
			b.WriteString(iconPrefix)
		}
		var inner strings.Builder
		renderRichTextRuns(&inner, cell.RichTextRuns, embeddedFamilies)
		richText := inner.String()
		if cell.Style.TextRotation != 0 {
			richText = wrapRotatedCellHTML(cell.Style.TextRotation, richText)
		}
		if hyperlinkURL != "" {
			fmt.Fprintf(b, "<a href=%q>%s</a>%s", hyperlinkURL, richText, cellSuffix)
		} else {
			b.WriteString(richText)
			b.WriteString(cellSuffix)
		}
		b.WriteString(closeTag)
		return
	}

	if cell != nil && cell.Style.TextRotation != 0 && text != "" {
		rotSpan := wrapRotatedCellHTML(cell.Style.TextRotation, escapeHTML(text))
		if hyperlinkURL != "" {
			fmt.Fprintf(b, "%s%s<a href=%q>%s</a>%s%s", openTag, iconPrefix, hyperlinkURL, rotSpan, cellSuffix, closeTag)
		} else {
			fmt.Fprintf(b, "%s%s%s%s%s", openTag, iconPrefix, rotSpan, cellSuffix, closeTag)
		}
		return
	}

	if hyperlinkURL != "" {
		fmt.Fprintf(b, "%s%s<a href=%q>%s</a>%s%s", openTag, iconPrefix, hyperlinkURL, escapeHTML(text), cellSuffix, closeTag)
		return
	}

	fmt.Fprintf(b, "%s%s%s%s%s", openTag, iconPrefix, escapeHTML(text), cellSuffix, closeTag)
}

type cellCoord struct {
	row int
	col int
}

func appendUnderlineVariantStyles(styles *[]string, underlineType string) {
	switch strings.TrimSpace(underlineType) {
	case "double", "doubleAccounting":
		*styles = append(*styles, "text-decoration-style:double")
	}
	switch strings.TrimSpace(underlineType) {
	case "singleAccounting", "doubleAccounting":
		*styles = append(*styles, "text-underline-position:under")
	}
}

// renderRichTextRuns writes styled <span> elements for each rich-text run.
func renderRichTextRuns(b *strings.Builder, runs []*resolve.RichTextRun, embeddedFamilies map[string]struct{}) {
	for _, run := range runs {
		if run == nil || run.Text == "" {
			continue
		}
		var spanStyles []string
		if run.Bold {
			spanStyles = append(spanStyles, "font-weight:bold")
		}
		if run.Italic {
			spanStyles = append(spanStyles, "font-style:italic")
		}
		var decorations []string
		if run.Underline {
			decorations = append(decorations, "underline")
		}
		if run.Strike {
			decorations = append(decorations, "line-through")
		}
		if len(decorations) > 0 {
			spanStyles = append(spanStyles, "text-decoration:"+strings.Join(decorations, " "))
			appendUnderlineVariantStyles(&spanStyles, run.UnderlineType)
		}
		if run.VertAlign == "superscript" {
			spanStyles = append(spanStyles, "vertical-align:super", "font-size:smaller")
		} else if run.VertAlign == "subscript" {
			spanStyles = append(spanStyles, "vertical-align:sub", "font-size:smaller")
		}
		if run.FontSizePt > 0 {
			spanStyles = append(spanStyles, fmt.Sprintf("font-size:%.2fpt", run.FontSizePt))
		}
		if run.FontFamily != "" {
			spanStyles = append(spanStyles, "font-family:"+cssFontFamily(run.FontFamily, !isEmbeddedFont(embeddedFamilies, run.FontFamily)))
		}
		if run.FontColor != "" {
			spanStyles = append(spanStyles, "color:#"+run.FontColor)
		}
		if len(spanStyles) > 0 {
			fmt.Fprintf(b, "<span style=%q>%s</span>", strings.Join(spanStyles, ";"), escapeHTML(run.Text))
		} else {
			b.WriteString(escapeHTML(run.Text))
		}
	}
}

func dataValidationAttrs(dv *resolve.DataValidation, state dataValidationState) []string {
	if dv == nil {
		return nil
	}
	var attrs []string
	attrs = append(attrs, fmt.Sprintf("data-validation-type=%q", dv.Type))
	if dv.Operator != "" {
		attrs = append(attrs, fmt.Sprintf("data-validation-operator=%q", dv.Operator))
	}
	if dv.Formula1 != "" {
		attrs = append(attrs, fmt.Sprintf("data-validation-formula1=%q", escapeHTMLAttr(dv.Formula1)))
	}
	if dv.Formula2 != "" {
		attrs = append(attrs, fmt.Sprintf("data-validation-formula2=%q", escapeHTMLAttr(dv.Formula2)))
	}
	if len(dv.ListValues) > 0 {
		attrs = append(attrs, fmt.Sprintf("data-validation-list-values=%q", escapeHTMLAttr(strings.Join(dv.ListValues, "\n"))))
	}
	if dv.PromptTitle != "" {
		attrs = append(attrs, fmt.Sprintf("data-validation-prompt-title=%q", escapeHTMLAttr(dv.PromptTitle)))
	}
	if dv.ErrorTitle != "" {
		attrs = append(attrs, fmt.Sprintf("data-validation-error-title=%q", escapeHTMLAttr(dv.ErrorTitle)))
	}
	if dv.Error != "" {
		attrs = append(attrs, fmt.Sprintf("data-validation-error=%q", escapeHTMLAttr(dv.Error)))
	}
	attrs = append(attrs,
		fmt.Sprintf("data-validation-allow-blank=%q", strconv.FormatBool(dv.AllowBlank)),
		fmt.Sprintf("data-validation-show-dropdown=%q", strconv.FormatBool(dv.ShowDropDown)),
		fmt.Sprintf("data-validation-show-input-message=%q", strconv.FormatBool(dv.ShowInputMessage)),
		fmt.Sprintf("data-validation-show-error-message=%q", strconv.FormatBool(dv.ShowErrorMessage)),
	)
	if state.Evaluated {
		attrs = append(attrs,
			`data-validation-evaluated="true"`,
			fmt.Sprintf("data-validation-valid=%q", strconv.FormatBool(state.Valid)),
		)
	}
	return attrs
}

func dataValidationTooltip(dv *resolve.DataValidation, state dataValidationState) string {
	if dv == nil {
		return ""
	}
	var parts []string
	if dv.PromptTitle != "" {
		parts = append(parts, dv.PromptTitle)
	}
	if dv.Prompt != "" {
		parts = append(parts, dv.Prompt)
	}
	if desc := dataValidationRuleDescription(dv); desc != "" {
		parts = append(parts, desc)
	}
	if dv.ErrorTitle != "" {
		parts = append(parts, "Error: "+dv.ErrorTitle)
	}
	if dv.Error != "" {
		parts = append(parts, dv.Error)
	}
	if state.Evaluated {
		if state.Valid {
			parts = append(parts, "Current value: valid")
		} else {
			parts = append(parts, "Current value: invalid")
		}
	}
	return strings.Join(parts, " | ")
}

// worksheetGeometryBounds returns the worksheet coordinate envelope owned by
// semantic grid content and anchored objects. Source <dimension> metadata is
// intentionally not used here: Excel files commonly retain stale dimensions,
// including whole-sheet ranges, which must not materialize millions of empty
// HTML cells.
// worksheetDataColumns returns the right-most column owned by cell content,
// merged ranges or array formulas. Anchored objects are deliberately excluded:
// the rendered overlay decides how far the grid reaches for them, so a banner
// trimmed back to the table does not leave a stray empty column behind.
func worksheetDataColumns(sheet *resolve.Sheet) int {
	if sheet == nil {
		return 0
	}
	maxCol := 0
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell != nil && cell.Col > maxCol {
				maxCol = cell.Col
			}
		}
	}
	for _, merged := range sheet.MergedCells {
		if merged != nil && merged.RightCol > maxCol {
			maxCol = merged.RightCol
		}
	}
	for _, af := range sheet.ArrayFormulas {
		if af != nil && af.RightCol > maxCol {
			maxCol = af.RightCol
		}
	}
	// Keep a single trailing column recorded by the source worksheet dimension.
	// Legacy BIFF commonly represents the authored Home-sheet envelope this way
	// even when that final column has no cell record of its own. OOXML tends to
	// materialize the corresponding blank cell, so ignoring DIMENSIONS here made
	// XLS stop at D while the equivalent XLSX correctly reached E.  Only accept
	// the immediately adjacent column: larger/stale dimensions (including the
	// notorious whole-sheet ranges) remain intentionally ignored.
	if maxCol > 0 && sheet.DimLastCol == maxCol+1 {
		maxCol = sheet.DimLastCol
	}
	return maxCol
}

func worksheetGeometryBounds(sheet *resolve.Sheet) (minRow, maxRow, minCol, maxCol int) {
	if sheet == nil {
		return 0, 0, 0, 0
	}
	include := func(top, bottom, left, right int) {
		if top <= 0 || bottom <= 0 || left <= 0 || right <= 0 {
			return
		}
		if minRow == 0 || top < minRow {
			minRow = top
		}
		if bottom > maxRow {
			maxRow = bottom
		}
		if minCol == 0 || left < minCol {
			minCol = left
		}
		if right > maxCol {
			maxCol = right
		}
	}
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell != nil {
				include(row.Index, row.Index, cell.Col, cell.Col)
			}
		}
	}
	for _, merged := range sheet.MergedCells {
		if merged != nil {
			include(merged.TopRow, merged.BottomRow, merged.LeftCol, merged.RightCol)
		}
	}
	for _, af := range sheet.ArrayFormulas {
		if af != nil {
			include(af.TopRow, af.BottomRow, af.LeftCol, af.RightCol)
		}
	}
	for _, d := range sheet.Drawings {
		if d == nil {
			continue
		}
		p := d.Position
		switch p.Anchor {
		case resolve.AnchorOneCell:
			include(p.FromCell.Row, p.FromCell.Row, p.FromCell.Col, p.FromCell.Col)
		case resolve.AnchorTwoCell:
			include(p.FromCell.Row, p.ToCell.Row, p.FromCell.Col, p.ToCell.Col)
		}
	}
	for _, control := range sheet.FormControls {
		if control == nil {
			continue
		}
		if col, row, ok := vmlAnchorFarCell(control.Anchor); ok {
			include(row, row, col, col)
		}
	}
	return minRow, maxRow, minCol, maxCol
}

func hasGridColumnLayout(sheet *resolve.Sheet) bool {
	if sheet == nil {
		return false
	}
	return len(sheet.ColumnWidths) > 0 ||
		sheet.DefaultColWidthChars > 0 ||
		len(sheet.HiddenCols) > 0 ||
		len(sheet.OutlineCols) > 0
}

func dataValidationRuleDescription(dv *resolve.DataValidation) string {
	if dv == nil {
		return ""
	}
	switch dv.Type {
	case "list":
		if len(dv.ListValues) > 0 {
			return "Allowed values: " + strings.Join(dv.ListValues, ", ")
		}
		if dv.Formula1 != "" {
			return "Allowed values: " + dv.Formula1
		}
		return "List validation"
	case "custom":
		if dv.Formula1 != "" {
			return "Custom formula: " + dv.Formula1
		}
		return "Custom validation"
	case "whole", "decimal", "date", "time", "textLength":
		label := dv.Type
		switch dv.Type {
		case "whole":
			label = "Whole number"
		case "decimal":
			label = "Decimal"
		case "date":
			label = "Date"
		case "time":
			label = "Time"
		case "textLength":
			label = "Text length"
		}
		return label + validationOperatorSuffix(dv.Operator, dv.Formula1, dv.Formula2)
	default:
		if dv.Formula1 != "" || dv.Formula2 != "" {
			return strings.TrimSpace(strings.Join([]string{dv.Type, dv.Formula1, dv.Formula2}, " "))
		}
	}
	return ""
}

func validationOperatorSuffix(operator, formula1, formula2 string) string {
	switch operator {
	case "between":
		if formula1 != "" || formula2 != "" {
			return fmt.Sprintf(" between %s and %s", formula1, formula2)
		}
	case "notBetween":
		if formula1 != "" || formula2 != "" {
			return fmt.Sprintf(" not between %s and %s", formula1, formula2)
		}
	case "equal":
		return " equal to " + formula1
	case "notEqual":
		return " not equal to " + formula1
	case "greaterThan":
		return " greater than " + formula1
	case "lessThan":
		return " less than " + formula1
	case "greaterThanOrEqual":
		return " greater than or equal to " + formula1
	case "lessThanOrEqual":
		return " less than or equal to " + formula1
	}
	if formula1 != "" && formula2 != "" {
		return fmt.Sprintf(" (%s, %s)", formula1, formula2)
	}
	if formula1 != "" {
		return " " + formula1
	}
	return ""
}

func renderOutlineControls(b *strings.Builder, rowMax, colMax int) {
	b.WriteString("<div class=\"outline-controls\">\n")
	if rowMax > 0 {
		renderOutlineGroup(b, "rows", rowMax)
	}
	if colMax > 0 {
		renderOutlineGroup(b, "cols", colMax)
	}
	b.WriteString("</div>\n")
}

// renderPaneMetadata emits a hidden div with pane configuration metadata.
func renderPaneMetadata(b *strings.Builder, sheet *resolve.Sheet) {
	var attrs []string
	attrs = append(attrs, "class=\"pane-meta\"", "hidden")
	if sheet.FrozenRows > 0 {
		attrs = append(attrs, fmt.Sprintf("data-frozen-rows=%q", strconv.Itoa(sheet.FrozenRows)))
	}
	if sheet.FrozenCols > 0 {
		attrs = append(attrs, fmt.Sprintf("data-frozen-cols=%q", strconv.Itoa(sheet.FrozenCols)))
	}
	if sheet.SplitRows > 0 {
		attrs = append(attrs, fmt.Sprintf("data-split-rows=%q", strconv.Itoa(sheet.SplitRows)))
	}
	if sheet.SplitCols > 0 {
		attrs = append(attrs, fmt.Sprintf("data-split-cols=%q", strconv.Itoa(sheet.SplitCols)))
	}
	if sheet.TopLeftCell != "" {
		attrs = append(attrs, fmt.Sprintf("data-top-left-cell=%q", sheet.TopLeftCell))
	}
	if sheet.ActivePane != "" {
		attrs = append(attrs, fmt.Sprintf("data-active-pane=%q", sheet.ActivePane))
	}
	fmt.Fprintf(b, "<div %s></div>\n", strings.Join(attrs, " "))
}

func buildRenderedFormControls(wb *resolve.Workbook, sheet *resolve.Sheet, sheetIndex int, dl drawingLayout) []renderedFormControl {
	var controls []*resolve.FormControl
	if sheet != nil {
		controls = sheet.FormControls
	}
	out := make([]renderedFormControl, 0, len(controls))
	groupBoxes := collectRadioGroupBoxes(controls, dl)
	linkedValues := formControlLinkedCellValues(sheet)
	nextGroupID := 0
	lastWasRadio := false
	lastGroupKey := ""
	groupPosition := 0
	for _, fc := range controls {
		rendered := renderedFormControl{control: fc}
		if fc == nil {
			out = append(out, rendered)
			lastWasRadio = false
			lastGroupKey = ""
			groupPosition = 0
			continue
		}
		rendered.options = formControlOptionValues(wb, sheetIndex, fc)
		rendered.selected = formControlSelectedIndexes(fc, linkedValues)
		rendered.multiple = fc.SelectionType == "multi" || fc.SelectionType == "extended"
		if fc.DropLines > 0 {
			rendered.visibleOptions = fc.DropLines
		} else if fc.Type == "List" {
			rendered.visibleOptions = 4
		}
		if fc.Type != "Radio" {
			out = append(out, rendered)
			lastWasRadio = false
			lastGroupKey = ""
			groupPosition = 0
			continue
		}
		groupKey := effectiveRadioGroupKey(fc, dl, groupBoxes)
		if fc.FirstButton || !lastWasRadio || groupKey != lastGroupKey {
			nextGroupID++
			groupPosition = 1
		} else {
			groupPosition++
		}
		rendered.radioName = fmt.Sprintf("sheet-radio-%d", nextGroupID)
		if value, ok := linkedValues[groupKey]; ok {
			checked := value == groupPosition
			rendered.checked = &checked
		}
		out = append(out, rendered)
		lastWasRadio = true
		lastGroupKey = groupKey
	}
	return out
}

func formControlOptionValues(wb *resolve.Workbook, sheetIndex int, fc *resolve.FormControl) []string {
	if fc == nil {
		return nil
	}
	formula := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(fc.FmlaRange), "="))
	if formula == "" {
		return nil
	}
	if resolved := resolveFormControlDefinedNameValue(wb, sheetIndex, formula); resolved != "" && !strings.EqualFold(resolved, formula) {
		formula = resolved
	}
	return resolveFormControlRangeValues(wb, sheetIndex, formula)
}

func formControlSelectedIndexes(fc *resolve.FormControl, linkedValues map[string]int) map[int]bool {
	if fc == nil {
		return nil
	}
	selected := make(map[int]bool)
	multiple := fc.SelectionType == "multi" || fc.SelectionType == "extended"
	if multiple {
		for _, idx := range fc.Selected {
			if idx > 0 {
				selected[idx] = true
			}
		}
	}
	if link := normalizeFormControlLink(fc.FmlaLink); link != "" {
		if idx, ok := linkedValues["link:"+link]; ok && idx > 0 {
			if !multiple {
				return map[int]bool{idx: true}
			}
			if len(selected) == 0 {
				selected[idx] = true
			}
		}
	}
	if len(selected) == 0 {
		switch {
		case fc.Val > 0:
			selected[fc.Val] = true
		case !multiple && len(fc.Selected) > 0 && fc.Selected[0] > 0:
			selected[fc.Selected[0]] = true
		}
	}
	if len(selected) == 0 {
		return nil
	}
	return selected
}

func collectRadioGroupBoxes(controls []*resolve.FormControl, dl drawingLayout) []radioGroupBox {
	out := make([]radioGroupBox, 0, len(controls))
	for _, fc := range controls {
		if fc == nil || fc.Type != "GBox" {
			continue
		}
		link := strings.TrimSpace(fc.FmlaGroup)
		if link == "" {
			continue
		}
		left, top, width, height := vmlAnchorToPixels(fc.Anchor, dl)
		if width <= 0 || height <= 0 {
			continue
		}
		out = append(out, radioGroupBox{
			link:   link,
			left:   left,
			top:    top,
			right:  left + width,
			bottom: top + height,
		})
	}
	return out
}

func effectiveRadioGroupKey(fc *resolve.FormControl, dl drawingLayout, groupBoxes []radioGroupBox) string {
	if fc == nil {
		return ""
	}
	if link := containingRadioGroupBoxLink(fc, dl, groupBoxes); link != "" {
		return "link:" + normalizeFormControlLink(link)
	}
	if link := strings.TrimSpace(fc.FmlaLink); link != "" {
		return "link:" + normalizeFormControlLink(link)
	}
	return strings.TrimSpace(fc.AltText)
}

func containingRadioGroupBoxLink(fc *resolve.FormControl, dl drawingLayout, groupBoxes []radioGroupBox) string {
	if fc == nil || len(groupBoxes) == 0 {
		return ""
	}
	left, top, width, height := vmlAnchorToPixels(fc.Anchor, dl)
	if width <= 0 || height <= 0 {
		return ""
	}
	centerX := left + width/2
	centerY := top + height/2
	bestArea := 0
	bestLink := ""
	for _, box := range groupBoxes {
		if centerX < box.left || centerX > box.right || centerY < box.top || centerY > box.bottom {
			continue
		}
		area := (box.right - box.left) * (box.bottom - box.top)
		if bestLink == "" || area < bestArea {
			bestLink = box.link
			bestArea = area
		}
	}
	return bestLink
}

func formControlLinkedCellValues(sheet *resolve.Sheet) map[string]int {
	if sheet == nil {
		return nil
	}
	values := make(map[string]int)
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			ref := normalizeFormControlLink(cell.Ref)
			if ref == "" {
				continue
			}
			if value, ok := formControlLinkedCellIndex(cell); ok {
				values["link:"+ref] = value
			}
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func formControlLinkedCellIndex(cell *resolve.Cell) (int, bool) {
	if cell == nil {
		return 0, false
	}
	switch cell.Kind {
	case resolve.CellNumber, resolve.CellBool:
		if cell.NumValue < 1 {
			return 0, false
		}
		rounded := math.Round(cell.NumValue)
		if math.Abs(cell.NumValue-rounded) > 1e-9 {
			return 0, false
		}
		return int(rounded), true
	case resolve.CellString, resolve.CellFormula:
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

func normalizeFormControlLink(ref string) string {
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

func resolveFormControlDefinedNameValue(wb *resolve.Workbook, sheetIndex int, name string) string {
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

func resolveFormControlRangeValues(wb *resolve.Workbook, currentSheetIndex int, formula string) []string {
	if wb == nil || currentSheetIndex < 0 || currentSheetIndex >= len(wb.Sheets) {
		return nil
	}
	sheetValues := buildFormControlCellValueMap(wb.Sheets[currentSheetIndex])
	refExpr := strings.TrimSpace(formula)
	if bang := strings.LastIndex(refExpr, "!"); bang >= 0 {
		sheetName := normalizeFormControlSheetName(refExpr[:bang])
		refExpr = refExpr[bang+1:]
		targetIndex := -1
		for i, sheet := range wb.Sheets {
			if sheet != nil && strings.EqualFold(sheet.Name, sheetName) {
				targetIndex = i
				break
			}
		}
		if targetIndex < 0 {
			return nil
		}
		sheetValues = buildFormControlCellValueMap(wb.Sheets[targetIndex])
	}
	refs := expandFormControlRangeRefs(refExpr)
	if len(refs) == 0 {
		return nil
	}
	values := make([]string, 0, len(refs))
	for _, ref := range refs {
		values = append(values, sheetValues[ref])
	}
	return values
}

func buildFormControlCellValueMap(sheet *resolve.Sheet) map[string]string {
	values := make(map[string]string)
	if sheet == nil {
		return values
	}
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil {
				continue
			}
			ref := strings.ToUpper(strings.TrimSpace(cell.Ref))
			if ref == "" {
				continue
			}
			values[ref] = formatCellValue(cell)
		}
	}
	return values
}

func normalizeFormControlSheetName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		name = strings.ReplaceAll(name[1:len(name)-1], "''", "'")
	}
	return name
}

func expandFormControlRangeRefs(expr string) []string {
	parts := strings.Fields(strings.ReplaceAll(expr, ",", " "))
	if len(parts) == 0 {
		return nil
	}
	var out []string
	for _, part := range parts {
		part = normalizeFormControlRangeRef(part)
		if part == "" {
			continue
		}
		if colon := strings.Index(part, ":"); colon >= 0 {
			startRef := part[:colon]
			endRef := part[colon+1:]
			sr, sc, ok1 := parseFormControlA1(startRef)
			er, ec, ok2 := parseFormControlA1(endRef)
			if !ok1 || !ok2 || er < sr || ec < sc {
				continue
			}
			for row := sr; row <= er; row++ {
				for col := sc; col <= ec; col++ {
					out = append(out, colLetterFromIndex(col)+strconv.Itoa(row))
				}
			}
			continue
		}
		if _, _, ok := parseFormControlA1(part); ok {
			out = append(out, part)
		}
	}
	return out
}

func normalizeFormControlRangeRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	ref = strings.ReplaceAll(ref, "$", "")
	if bang := strings.LastIndex(ref, "!"); bang >= 0 {
		ref = ref[bang+1:]
	}
	return strings.ToUpper(ref)
}

func parseFormControlA1(ref string) (row, col int, ok bool) {
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		i++
	}
	if i == 0 || i >= len(ref) {
		return 0, 0, false
	}
	col = 0
	for j := 0; j < i; j++ {
		col = col*26 + int(ref[j]-'A') + 1
	}
	r, err := strconv.Atoi(ref[i:])
	if err != nil || r < 1 {
		return 0, 0, false
	}
	return r, col, true
}

// renderFormControl emits a disabled HTML form element for a VML-based Excel form control.
func renderFormControl(b *strings.Builder, rendered renderedFormControl, dl drawingLayout) {
	fc := rendered.control
	if fc == nil {
		return
	}
	x, y, w, h := vmlAnchorToPixels(fc.Anchor, dl)
	posStyle := fmt.Sprintf("position:absolute;left:%dpx;top:%dpx;width:%dpx;height:%dpx", x, y, w, h)
	caption := escapeHTML(fc.AltText)
	switch fc.Type {
	case "Button":
		fmt.Fprintf(b, "<button class=\"form-control form-control--button\" style=%q disabled>%s</button>\n",
			posStyle, caption)
	case "Checkbox":
		checked := ""
		ariaChecked := "false"
		extraClass := ""
		if fc.Mixed {
			checked = " checked"
			ariaChecked = "mixed"
			extraClass = " form-control--mixed"
		} else if fc.Checked {
			checked = " checked"
			ariaChecked = "true"
		}
		if caption != "" {
			fmt.Fprintf(b, "<label class=\"form-control form-control--checkbox\" style=%q>"+
				"<input type=\"checkbox\" class=\"form-control form-control--checkbox%s\"%s aria-checked=%q disabled> %s</label>\n", posStyle, extraClass, checked, ariaChecked, caption)
		} else {
			fmt.Fprintf(b, "<input type=\"checkbox\" class=\"form-control form-control--checkbox%s\" style=%q%s aria-checked=%q disabled>\n",
				extraClass, posStyle, checked, ariaChecked)
		}
	case "Radio":
		checked := ""
		nameAttr := ""
		isChecked := fc.Checked
		if rendered.checked != nil {
			isChecked = *rendered.checked
		}
		if isChecked {
			checked = " checked"
		}
		if rendered.radioName != "" {
			nameAttr = fmt.Sprintf(" name=%q", rendered.radioName)
		}
		if caption != "" {
			fmt.Fprintf(b, "<label class=\"form-control form-control--radio\" style=%q>"+
				"<input type=\"radio\"%s%s disabled> %s</label>\n", posStyle, nameAttr, checked, caption)
		} else {
			fmt.Fprintf(b, "<input type=\"radio\" class=\"form-control form-control--radio\" style=%q%s%s disabled>\n",
				posStyle, nameAttr, checked)
		}
	case "Drop":
		fmt.Fprintf(b, "<select class=\"form-control form-control--dropdown\" style=%q disabled>", posStyle)
		renderFormControlOptions(b, rendered)
		b.WriteString("</select>\n")
	case "List":
		multiple := ""
		if rendered.multiple {
			multiple = " multiple"
		}
		size := rendered.visibleOptions
		if size <= 0 {
			size = 4
		}
		fmt.Fprintf(b, "<select class=\"form-control form-control--list\" size=%q style=%q%s disabled>", strconv.Itoa(size), posStyle, multiple)
		renderFormControlOptions(b, rendered)
		b.WriteString("</select>\n")
	case "Edit":
		fmt.Fprintf(b, "<input type=\"text\" class=\"form-control form-control--edit\" style=%q disabled>\n", posStyle)
	case "Label", "GBox":
		if caption != "" {
			fmt.Fprintf(b, "<span class=\"form-control form-control--label\" style=%q>%s</span>\n", posStyle, caption)
		}
		// Spin / Scroll: render a simple range input for visual indication.
	case "Spin", "Scroll":
		min, max, val := fc.Min, fc.Max, fc.Val
		if max == 0 && min == 0 {
			max = 100
		}
		step := fc.Increment
		if step <= 0 {
			step = 1
		}
		className := "form-control form-control--spin"
		style := posStyle
		if fc.Type == "Scroll" {
			className = "form-control form-control--scroll"
			if fc.Horizontal {
				className += " form-control--scroll-horizontal"
			} else {
				className += " form-control--scroll-vertical"
				style += ";writing-mode:vertical-lr;direction:rtl"
			}
		}
		fmt.Fprintf(b, "<input type=\"range\" class=%q min=%d max=%d value=%d step=%d data-page-step=%d style=%q disabled>\n",
			className, min, max, val, step, fc.PageStep, style)
	}
}

func renderFormControlOptions(b *strings.Builder, rendered renderedFormControl) {
	if len(rendered.options) == 0 {
		return
	}
	for i, option := range rendered.options {
		selected := ""
		if rendered.selected[i+1] {
			selected = " selected"
		}
		fmt.Fprintf(b, "<option%s>%s</option>", selected, escapeHTML(option))
	}
}

// vmlAnchorToPixels converts a VML anchor string to pixel position and size.
// The VML anchor format is: "col1, colOff, row1, rowOff, col2, colOff2, row2, rowOff2"
// where col/row are 0-based indices and offsets are in pixels.
func vmlAnchorToPixels(anchor string, dl drawingLayout) (x, y, w, h int) {
	parts := strings.Split(anchor, ",")
	if len(parts) != 8 {
		return 0, 0, 80, 20
	}
	parse := func(s string) int {
		v, _ := strconv.Atoi(strings.TrimSpace(s))
		return v
	}
	col1 := parse(parts[0]) + 1 // convert 0-based to 1-based
	colOff1 := parse(parts[1])
	row1 := parse(parts[2]) + 1
	rowOff1 := parse(parts[3])
	col2 := parse(parts[4]) + 1
	colOff2 := parse(parts[5])
	row2 := parse(parts[6]) + 1
	rowOff2 := parse(parts[7])

	x1 := dl.anchorXPx(col1, 0) + colOff1
	y1 := dl.anchorYPx(row1, 0) + rowOff1
	x2 := dl.anchorXPx(col2, 0) + colOff2
	y2 := dl.anchorYPx(row2, 0) + rowOff2
	if x2 <= x1 {
		x2 = x1 + 80
	}
	if y2 <= y1 {
		y2 = y1 + 20
	}
	return x1, y1, x2 - x1, y2 - y1
}

// renderTable emits a metadata block for a structured table.
func renderTable(b *strings.Builder, tbl *resolve.Table) {
	if tbl == nil {
		return
	}
	name := escapeHTML(tbl.Name)
	b.WriteString("<div class=\"structured-table\" aria-label=\"Table: " + name + "\">\n")
	fmt.Fprintf(b, "<h3 class=\"structured-table-name\">%s</h3>\n", name)

	// Metadata table
	b.WriteString("<table class=\"structured-table-meta\">\n")
	b.WriteString("<thead><tr><th>Property</th><th>Value</th></tr></thead>\n")
	b.WriteString("<tbody>\n")
	fmt.Fprintf(b, "<tr><th>Ref</th><td>%s</td></tr>\n", escapeHTML(tbl.Ref))
	if tbl.DisplayName != "" && tbl.DisplayName != tbl.Name {
		fmt.Fprintf(b, "<tr><th>Display Name</th><td>%s</td></tr>\n", escapeHTML(tbl.DisplayName))
	}
	if tbl.HeaderRowCount > 0 {
		fmt.Fprintf(b, "<tr><th>Header Rows</th><td>%d</td></tr>\n", tbl.HeaderRowCount)
	}
	if tbl.TotalsRowCount > 0 {
		fmt.Fprintf(b, "<tr><th>Totals Rows</th><td>%d</td></tr>\n", tbl.TotalsRowCount)
	}
	if tbl.TotalsRowShown {
		fmt.Fprintf(b, "<tr><th>Totals Row Shown</th><td>true</td></tr>\n")
	}
	if tbl.AutoFilterRef != "" {
		fmt.Fprintf(b, "<tr><th>AutoFilter</th><td>%s</td></tr>\n", escapeHTML(tbl.AutoFilterRef))
	}
	if tbl.TableStyle != nil {
		style := tbl.TableStyle
		fmt.Fprintf(b, "<tr><th>Style</th><td>%s</td></tr>\n", escapeHTML(style.Name))
		var flags []string
		if style.ShowFirstColumn {
			flags = append(flags, "firstColumn")
		}
		if style.ShowLastColumn {
			flags = append(flags, "lastColumn")
		}
		if style.ShowRowStripes {
			flags = append(flags, "rowStripes")
		}
		if style.ShowColumnStripes {
			flags = append(flags, "columnStripes")
		}
		if len(flags) > 0 {
			fmt.Fprintf(b, "<tr><th>Style Flags</th><td>%s</td></tr>\n", escapeHTML(strings.Join(flags, ", ")))
		}
	}
	if len(tbl.Columns) > 0 {
		var colNames []string
		for _, col := range tbl.Columns {
			colNames = append(colNames, col.Name)
		}
		fmt.Fprintf(b, "<tr><th>Columns</th><td>%s</td></tr>\n", escapeHTML(strings.Join(colNames, ", ")))
		for _, col := range tbl.Columns {
			if col.CalculatedColumnFormula != "" {
				fmt.Fprintf(b, "<tr><th>Col %s Formula</th><td>%s</td></tr>\n", escapeHTML(col.Name), escapeHTML(col.CalculatedColumnFormula))
			}
			if col.TotalsRowFunction != "" {
				fmt.Fprintf(b, "<tr><th>Col %s Totals</th><td>%s</td></tr>\n", escapeHTML(col.Name), escapeHTML(col.TotalsRowFunction))
			}
			if col.TotalsRowLabel != "" {
				fmt.Fprintf(b, "<tr><th>Col %s Totals Label</th><td>%s</td></tr>\n", escapeHTML(col.Name), escapeHTML(col.TotalsRowLabel))
			}
			if col.TotalsRowFormula != "" {
				fmt.Fprintf(b, "<tr><th>Col %s Totals Formula</th><td>%s</td></tr>\n", escapeHTML(col.Name), escapeHTML(col.TotalsRowFormula))
			}
		}
	}
	b.WriteString("</tbody></table>\n")
	b.WriteString("</div>\n")
}

// renderPivotTable emits a metadata block and summary table for a pivot table.
func renderPivotTable(b *strings.Builder, pt *resolve.PivotTable) {
	if pt == nil {
		return
	}
	name := escapeHTML(pt.Name)
	b.WriteString("<div class=\"pivot-table\" aria-label=\"Pivot table: " + name + "\">\n")
	fmt.Fprintf(b, "<h3 class=\"pivot-table-name\">%s</h3>\n", name)

	// Source info
	if pt.SourceRef != "" || pt.SourceName != "" {
		src := pt.SourceRef
		if src == "" {
			src = pt.SourceName
		}
		if pt.SourceSheet != "" {
			src = escapeHTML(pt.SourceSheet) + "!" + escapeHTML(src)
		} else {
			src = escapeHTML(src)
		}
		fmt.Fprintf(b, "<p class=\"pivot-source\">Source: <code>%s</code></p>\n", src)
	}

	// Metadata table
	b.WriteString("<table class=\"pivot-meta\">\n")
	b.WriteString("<thead><tr><th>Axis</th><th>Fields</th></tr></thead>\n")
	b.WriteString("<tbody>\n")
	if len(pt.PageFields) > 0 {
		fmt.Fprintf(b, "<tr><th>Filters</th><td>%s</td></tr>\n", escapeHTML(strings.Join(pt.PageFields, ", ")))
	}
	if len(pt.ColFields) > 0 {
		fmt.Fprintf(b, "<tr><th>Columns</th><td>%s</td></tr>\n", escapeHTML(strings.Join(pt.ColFields, ", ")))
	}
	if len(pt.RowFields) > 0 {
		fmt.Fprintf(b, "<tr><th>Rows</th><td>%s</td></tr>\n", escapeHTML(strings.Join(pt.RowFields, ", ")))
	}
	if len(pt.DataFields) > 0 {
		var dfNames []string
		for _, df := range pt.DataFields {
			label := df.Name
			if label == "" && df.SourceFieldName != "" {
				subtotal := df.Subtotal
				if len(subtotal) > 0 {
					subtotal = strings.ToUpper(subtotal[:1]) + subtotal[1:]
				}
				label = subtotal + " of " + df.SourceFieldName
			}
			if label == "" {
				label = df.SourceFieldName
			}
			dfNames = append(dfNames, label)
		}
		fmt.Fprintf(b, "<tr><th>Values</th><td>%s</td></tr>\n", escapeHTML(strings.Join(dfNames, ", ")))
	}
	b.WriteString("</tbody></table>\n")
	b.WriteString("</div>\n")
}

func appendProtectionMetaAttrs(attrs []string, workbookProtection *resolve.WorkbookProtection, sheetProtection *resolve.SheetProtection) []string {
	if workbookProtection != nil {
		attrs = append(attrs, `data-workbook-protection="true"`)
		attrs = appendBoolMetaAttr(attrs, "data-workbook-lock-structure", workbookProtection.LockStructure)
		attrs = appendBoolMetaAttr(attrs, "data-workbook-lock-windows", workbookProtection.LockWindows)
		attrs = appendBoolMetaAttr(attrs, "data-workbook-lock-revision", workbookProtection.LockRevision)
		if workbookProtection.AlgorithmName != "" {
			attrs = append(attrs, fmt.Sprintf("data-workbook-protection-algorithm=%q", workbookProtection.AlgorithmName))
		}
	}
	if sheetProtection != nil {
		attrs = append(attrs, `data-sheet-protection="true"`)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-sheet", sheetProtection.Sheet)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-objects", sheetProtection.Objects)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-scenarios", sheetProtection.Scenarios)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-format-cells", sheetProtection.FormatCells)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-format-columns", sheetProtection.FormatColumns)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-format-rows", sheetProtection.FormatRows)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-insert-columns", sheetProtection.InsertColumns)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-insert-rows", sheetProtection.InsertRows)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-insert-hyperlinks", sheetProtection.InsertHyperlinks)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-delete-columns", sheetProtection.DeleteColumns)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-delete-rows", sheetProtection.DeleteRows)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-select-locked-cells", sheetProtection.SelectLockedCells)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-sort", sheetProtection.Sort)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-auto-filter", sheetProtection.AutoFilter)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-pivot-tables", sheetProtection.PivotTables)
		attrs = appendBoolMetaAttr(attrs, "data-sheet-protection-select-unlocked-cells", sheetProtection.SelectUnlockedCells)
		if sheetProtection.AlgorithmName != "" {
			attrs = append(attrs, fmt.Sprintf("data-sheet-protection-algorithm=%q", sheetProtection.AlgorithmName))
		}
	}
	return attrs
}

func appendBoolMetaAttr(attrs []string, name string, value bool) []string {
	if value {
		return append(attrs, fmt.Sprintf("%s=\"true\"", name))
	}
	return attrs
}

func renderProtectionNotice(b *strings.Builder, workbookProtection *resolve.WorkbookProtection, sheetProtection *resolve.SheetProtection) {
	if workbookProtection == nil && sheetProtection == nil {
		return
	}
	b.WriteString("<div class=\"sheet-protection\" aria-label=\"Protection metadata\">\n")
	b.WriteString("<p class=\"sheet-protection-title\">Protection metadata preserved; restrictions are not enforced during HTML rendering.</p>\n")
	if flags := workbookProtectionFlags(workbookProtection); len(flags) > 0 {
		fmt.Fprintf(b, "<p class=\"sheet-protection-detail\"><strong>Workbook flags:</strong> %s</p>\n", escapeHTML(strings.Join(flags, ", ")))
	}
	if flags := sheetProtectionFlags(sheetProtection); len(flags) > 0 {
		fmt.Fprintf(b, "<p class=\"sheet-protection-detail\"><strong>Worksheet flags:</strong> %s</p>\n", escapeHTML(strings.Join(flags, ", ")))
	}
	if workbookProtection != nil && workbookProtection.AlgorithmName != "" {
		fmt.Fprintf(b, "<p class=\"sheet-protection-detail\"><strong>Workbook algorithm:</strong> %s</p>\n", escapeHTML(workbookProtection.AlgorithmName))
	}
	if sheetProtection != nil && sheetProtection.AlgorithmName != "" {
		fmt.Fprintf(b, "<p class=\"sheet-protection-detail\"><strong>Worksheet algorithm:</strong> %s</p>\n", escapeHTML(sheetProtection.AlgorithmName))
	}
	b.WriteString("</div>\n")
}

func workbookProtectionFlags(protection *resolve.WorkbookProtection) []string {
	if protection == nil {
		return nil
	}
	var flags []string
	if protection.LockStructure {
		flags = append(flags, "lockStructure")
	}
	if protection.LockWindows {
		flags = append(flags, "lockWindows")
	}
	if protection.LockRevision {
		flags = append(flags, "lockRevision")
	}
	return flags
}

func sheetProtectionFlags(protection *resolve.SheetProtection) []string {
	if protection == nil {
		return nil
	}
	table := []struct {
		name    string
		enabled bool
	}{
		{name: "sheet", enabled: protection.Sheet},
		{name: "objects", enabled: protection.Objects},
		{name: "scenarios", enabled: protection.Scenarios},
		{name: "formatCells", enabled: protection.FormatCells},
		{name: "formatColumns", enabled: protection.FormatColumns},
		{name: "formatRows", enabled: protection.FormatRows},
		{name: "insertColumns", enabled: protection.InsertColumns},
		{name: "insertRows", enabled: protection.InsertRows},
		{name: "insertHyperlinks", enabled: protection.InsertHyperlinks},
		{name: "deleteColumns", enabled: protection.DeleteColumns},
		{name: "deleteRows", enabled: protection.DeleteRows},
		{name: "selectLockedCells", enabled: protection.SelectLockedCells},
		{name: "sort", enabled: protection.Sort},
		{name: "autoFilter", enabled: protection.AutoFilter},
		{name: "pivotTables", enabled: protection.PivotTables},
		{name: "selectUnlockedCells", enabled: protection.SelectUnlockedCells},
	}
	flags := make([]string, 0, len(table))
	for _, entry := range table {
		if entry.enabled {
			flags = append(flags, entry.name)
		}
	}
	return flags
}

func renderOutlineGroup(b *strings.Builder, axis string, maxLevel int) {
	label := "Rows"
	if axis == "cols" {
		label = "Columns"
	}
	fmt.Fprintf(b, "<div class=\"outline-group\"><span class=\"outline-label\">%s</span>", label)
	fmt.Fprintf(b, "<button type=\"button\" class=\"outline-btn\" data-axis=%q data-level=\"all\" aria-pressed=\"true\" onclick=\"sheetSetOutline(this)\">All</button>", axis)
	for level := 1; level <= maxLevel; level++ {
		fmt.Fprintf(b, "<button type=\"button\" class=\"outline-btn\" data-axis=%q data-level=%q aria-pressed=\"false\" onclick=\"sheetSetOutline(this)\">%d</button>", axis, strconv.Itoa(level), level)
	}
	b.WriteString("</div>\n")
}

func maxOutlineLevel(levels map[int]int) int {
	maxLevel := 0
	for _, level := range levels {
		if level > maxLevel {
			maxLevel = level
		}
	}
	return maxLevel
}

type mergeLayout struct {
	anchors map[cellCoord]*resolve.MergedCell
	covered map[cellCoord]struct{}
	rows    map[int]struct{}
	minRow  int
	maxRow  int
}

func buildMergeLayout(mergedCells []*resolve.MergedCell) mergeLayout {
	layout := mergeLayout{
		anchors: make(map[cellCoord]*resolve.MergedCell, len(mergedCells)),
		covered: make(map[cellCoord]struct{}),
		rows:    make(map[int]struct{}),
		minRow:  math.MaxInt32,
	}
	for _, merged := range mergedCells {
		if merged == nil || merged.TopRow <= 0 || merged.LeftCol <= 0 {
			continue
		}
		layout.anchors[cellCoord{row: merged.TopRow, col: merged.LeftCol}] = merged
		for row := merged.TopRow; row <= merged.BottomRow; row++ {
			layout.rows[row] = struct{}{}
			if row < layout.minRow {
				layout.minRow = row
			}
			if row > layout.maxRow {
				layout.maxRow = row
			}
			for col := merged.LeftCol; col <= merged.RightCol; col++ {
				if row == merged.TopRow && col == merged.LeftCol {
					continue
				}
				layout.covered[cellCoord{row: row, col: col}] = struct{}{}
			}
		}
	}
	if len(layout.rows) == 0 {
		layout.minRow = 0
	}
	return layout
}

func (ml mergeLayout) anchorAt(row, col int) *resolve.MergedCell {
	return ml.anchors[cellCoord{row: row, col: col}]
}

func (ml mergeLayout) isCovered(row, col int) bool {
	_, ok := ml.covered[cellCoord{row: row, col: col}]
	return ok
}

// shouldOverrideDefaultGridBorder reports whether the cell draws decoration of
// its own, in which case the fallback grid line is dropped. It has to stay in
// step with appendFillStyles and appendBorderStyles: a style that emits nothing
// (for example Excel's "none" fill pattern, which legacy XLS styles commonly
// carry) must keep the grid, or the cell renders with no borders at all.
func shouldOverrideDefaultGridBorder(style resolve.CellStyle) bool {
	return fillDrawsBackground(style) ||
		style.BorderTop.Style != "" ||
		style.BorderRight.Style != "" ||
		style.BorderBottom.Style != "" ||
		style.BorderLeft.Style != "" ||
		style.BorderDiagonal.Style != "" ||
		style.TextRotation != 0
}

// fillDrawsBackground mirrors appendFillStyles' decision to emit a background.
func fillDrawsBackground(style resolve.CellStyle) bool {
	switch strings.TrimSpace(style.FillPattern) {
	case "":
		return style.BgColor != ""
	case "none":
		return false
	case "solid":
		return style.BgColor != ""
	default:
		return style.BgColor != "" || style.FillPatternBgColor != ""
	}
}

func applyCFBorderOverride(styles *[]string, side string, code *int, color string) {
	if styles == nil || code == nil {
		return
	}
	*styles = removeStyleProp(*styles, "border-"+side)
	if *code == 0 {
		return
	}
	style := cfBorderStyleName(*code)
	if style == "" {
		return
	}
	appendBorderStyle(styles, side, resolve.BorderStyle{Style: style, Color: color})
}

func cfBorderStyleName(code int) string {
	switch code {
	case 1:
		return "thin"
	case 2:
		return "medium"
	case 3:
		return "dashed"
	case 4:
		return "hair"
	case 5:
		return "thick"
	case 6:
		return "double"
	case 7:
		return "dotted"
	case 8:
		return "mediumDashed"
	case 9:
		return "dashDot"
	case 10:
		return "mediumDashDot"
	case 11:
		return "dashDotDot"
	case 12:
		return "mediumDashDotDot"
	case 13:
		return "slantDashDot"
	default:
		return ""
	}
}

func cssCellHAlign(align string) (string, []string) {
	switch align {
	case "left", "center", "right":
		return align, nil
	case "centerContinuous":
		// HTML has no cross-cell center-continuous primitive; preserve the
		// semantic value in data-align and use the closest visual alignment.
		return "center", nil
	case "justify":
		return "justify", nil
	case "distributed":
		return "justify", []string{"text-align-last:justify"}
	case "fill":
		// Repetition depends on measured cell width. Preserve the semantic value
		// in data-align without emitting an invalid CSS text-align value.
		return "", nil
	default:
		return "", nil
	}
}

func appendFillStyles(styles *[]string, style resolve.CellStyle) {
	if styles == nil {
		return
	}
	pattern := strings.TrimSpace(style.FillPattern)
	if pattern == "" {
		if style.BgColor != "" {
			*styles = append(*styles, "background:#"+style.BgColor)
		}
		return
	}
	if pattern == "none" {
		return
	}
	fg := style.BgColor
	bg := style.FillPatternBgColor
	if pattern == "solid" {
		if fg != "" {
			*styles = append(*styles, "background:#"+fg)
		}
		return
	}
	if bg != "" {
		*styles = append(*styles, "background-color:#"+bg)
	} else if fg != "" {
		// Pattern fills without an explicit background still need a stable base.
		*styles = append(*styles, "background-color:#FFFFFF")
	}
	if fg == "" {
		return
	}
	if image, size := cssPatternFill(pattern, fg); image != "" {
		*styles = append(*styles, "background-image:"+image)
		if size != "" {
			*styles = append(*styles, "background-size:"+size)
		}
	}
}

func cssPatternFill(pattern, fg string) (image, size string) {
	c := "#" + fg
	switch strings.ToLower(strings.TrimSpace(pattern)) {
	case "gray125", "gray0625":
		return "radial-gradient(" + c + " 0.7px,transparent 0.8px)", "4px 4px"
	case "darkgray", "mediumgray", "lightgray":
		return "repeating-linear-gradient(45deg," + c + " 0 1px,transparent 1px 3px)", ""
	case "darkhorizontal", "lighthorizontal":
		return "repeating-linear-gradient(0deg," + c + " 0 1px,transparent 1px 4px)", ""
	case "darkvertical", "lightvertical":
		return "repeating-linear-gradient(90deg," + c + " 0 1px,transparent 1px 4px)", ""
	case "darkdown", "lightdown":
		return "repeating-linear-gradient(45deg," + c + " 0 1px,transparent 1px 5px)", ""
	case "darkup", "lightup":
		return "repeating-linear-gradient(135deg," + c + " 0 1px,transparent 1px 5px)", ""
	case "darkgrid", "lightgrid":
		return "repeating-linear-gradient(0deg," + c + " 0 1px,transparent 1px 5px),repeating-linear-gradient(90deg," + c + " 0 1px,transparent 1px 5px)", ""
	case "darktrellis", "lighttrellis":
		return "repeating-linear-gradient(45deg," + c + " 0 1px,transparent 1px 5px),repeating-linear-gradient(135deg," + c + " 0 1px,transparent 1px 5px)", ""
	default:
		return "", ""
	}
}

func appendBorderStyles(styles *[]string, style resolve.CellStyle) {
	if styles == nil {
		return
	}
	appendBorderStyle(styles, "top", style.BorderTop)
	appendBorderStyle(styles, "right", style.BorderRight)
	appendBorderStyle(styles, "bottom", style.BorderBottom)
	appendBorderStyle(styles, "left", style.BorderLeft)
	appendDiagonalBorderStyles(styles, style)
}

func appendDiagonalBorderStyles(styles *[]string, style resolve.CellStyle) {
	if styles == nil || style.BorderDiagonal.Style == "" || (!style.BorderDiagonalUp && !style.BorderDiagonalDown) {
		return
	}
	width, _ := cssBorderStyle(style.BorderDiagonal.Style)
	color := "#000000"
	if style.BorderDiagonal.Color != "" {
		color = "#" + style.BorderDiagonal.Color
	}
	// CSS has no diagonal-border primitive. Layer a one-stroke gradient over
	// the cell while retaining the canonical Excel direction flags in IR.
	stroke := "1px"
	if width == "2px" || width == "3px" {
		stroke = width
	}
	var images []string
	if style.BorderDiagonalUp {
		images = append(images, fmt.Sprintf("linear-gradient(to top right,transparent calc(50%% - %s),%s 50%%,transparent calc(50%% + %s))", stroke, color, stroke))
	}
	if style.BorderDiagonalDown {
		images = append(images, fmt.Sprintf("linear-gradient(to bottom right,transparent calc(50%% - %s),%s 50%%,transparent calc(50%% + %s))", stroke, color, stroke))
	}
	if len(images) > 0 {
		*styles = append(*styles, "background-image:"+strings.Join(images, ","))
	}
}

func appendBorderStyle(styles *[]string, side string, border resolve.BorderStyle) {
	if styles == nil || border.Style == "" {
		return
	}
	width, styleName := cssBorderStyle(border.Style)
	if styleName == "" {
		return
	}
	color := "#000000"
	if border.Color != "" {
		color = "#" + border.Color
	}
	*styles = append(*styles, fmt.Sprintf("border-%s:%s %s %s", side, width, styleName, color))
}

func cssBorderStyle(style string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "thin", "hair":
		return "1px", "solid"
	case "medium":
		return "2px", "solid"
	case "thick":
		return "3px", "solid"
	case "dotted":
		return "1px", "dotted"
	case "dashed", "mediumdashed", "slantdashdot":
		return "1px", "dashed"
	case "double":
		return "3px", "double"
	default:
		return "1px", "solid"
	}
}

func cellTextRotationCSS(rotation int) string {
	switch rotation {
	case 255:
		return "display:inline-block;writing-mode:vertical-rl;text-orientation:upright;white-space:nowrap;line-height:1"
	case 90:
		return "display:inline-block;position:absolute;left:50%;top:50%;transform:translate(-50%,-50%) rotate(-90deg);transform-origin:center center;white-space:nowrap;line-height:1"
	case 180:
		return "display:inline-block;position:absolute;left:50%;top:50%;transform:translate(-50%,-50%) rotate(180deg);transform-origin:center center;white-space:nowrap;line-height:1"
	default:
		return fmt.Sprintf("display:inline-block;position:absolute;left:50%%;top:50%%;transform:translate(-50%%,-50%%) rotate(%ddeg);transform-origin:center center;white-space:nowrap;line-height:1", -rotation)
	}
}

func wrapRotatedCellHTML(rotation int, innerHTML string) string {
	if rotation == 255 {
		return fmt.Sprintf("<span style=%q>%s</span>", cellTextRotationCSS(rotation), innerHTML)
	}
	return fmt.Sprintf("<span style=%q><span style=%q>%s</span></span>",
		"display:inline-block;position:relative;width:0;height:0;line-height:0;overflow:visible;vertical-align:middle",
		cellTextRotationCSS(rotation),
		innerHTML,
	)
}

func cssFontFamily(name string, includeFallbacks bool) string {
	if name == "" {
		return ""
	}
	families := []string{name}
	if includeFallbacks {
		families = append(families, bestEffortFontFallbacks(name)...)
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(families))
	for _, candidate := range families {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := normalizeFontFamily(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if isGenericFontFamily(candidate) {
			out = append(out, candidate)
			continue
		}
		out = append(out, quoteCSSFontFamilyName(candidate))
	}
	return strings.Join(out, ",")
}

func bestEffortFontFallbacks(name string) []string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "source code pro":
		return []string{"Consolas", "Courier New", "monospace"}
	case "oswald":
		return []string{"Arial Narrow", "Arial", "sans-serif"}
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "code") || strings.Contains(lower, "mono") {
		return []string{"Consolas", "Courier New", "monospace"}
	}
	if strings.Contains(lower, "display") || strings.Contains(lower, "condensed") {
		return []string{"Arial Narrow", "Arial", "sans-serif"}
	}
	return []string{"Arial", "sans-serif"}
}

func normalizeFontFamily(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isGenericFontFamily(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "serif", "sans-serif", "monospace", "cursive", "fantasy", "system-ui", "math", "emoji", "fangsong":
		return true
	default:
		return false
	}
}

func quoteCSSFontFamilyName(name string) string {
	if name == "" {
		return name
	}
	if strings.ContainsAny(name, " ,\"'") {
		return "'" + strings.ReplaceAll(name, "'", "\\'") + "'"
	}
	return name
}

func buildEmbeddedFamilyIndex(wb *resolve.Workbook) map[string]struct{} {
	index := map[string]struct{}{}
	if wb == nil {
		return index
	}
	for _, font := range wb.EmbeddedFonts {
		if font == nil || strings.TrimSpace(font.Family) == "" {
			continue
		}
		index[normalizeFontFamily(font.Family)] = struct{}{}
	}
	return index
}

func isEmbeddedFont(embeddedFamilies map[string]struct{}, name string) bool {
	if len(embeddedFamilies) == 0 {
		return false
	}
	_, ok := embeddedFamilies[normalizeFontFamily(name)]
	return ok
}

func (r *htmlRenderer) writeEmbeddedFontCSS(b *strings.Builder) {
	if b == nil || r == nil || r.wb == nil {
		return
	}
	for _, font := range r.wb.EmbeddedFonts {
		if font == nil || strings.TrimSpace(font.Family) == "" {
			continue
		}
		for _, face := range font.Faces {
			if face == nil || len(face.Data) == 0 {
				continue
			}
			sources := fontdata.EmbeddedSources(face.Data, face.ContentType, face.Family)
			if len(sources) == 0 {
				continue
			}
			fmt.Fprintf(b, "@font-face{font-family:%s;font-style:%s;font-weight:%d;font-display:swap;src:",
				quoteCSSFontFamilyName(face.Family),
				fontStyleValue(face.Style),
				fontWeightValue(face.Weight),
			)
			for i, src := range sources {
				if i > 0 {
					b.WriteByte(',')
				}
				if r.options.FontMode == HTMLFontModeExternal {
					fmt.Fprintf(b, "url(%q) format(%q)", r.addFontAsset(src), src.Format)
					continue
				}
				encoded := base64.StdEncoding.EncodeToString(src.Data)
				fmt.Fprintf(b, "url(data:%s;base64,%s) format(%q)", src.MIME, encoded, src.Format)
			}
			b.WriteString(";}\n")
		}
	}
}

func (r *htmlRenderer) imageSource(img *resolve.DrawingImage) string {
	if r != nil && r.options.ImageMode == HTMLImageModeExternal {
		return r.addImageAsset(img)
	}
	return "data:" + img.ContentType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
}

func (r *htmlRenderer) addImageAsset(img *resolve.DrawingImage) string {
	if r == nil || img == nil {
		return ""
	}
	r.nextImageIndex++
	assetPath := htmlasset.AssetPath(r.options.ExternalImageDir, "image", r.nextImageIndex, htmlasset.ImageFileExtension(img.ContentType))
	r.assets = append(r.assets, HTMLAsset{
		Path:        assetPath,
		ContentType: img.ContentType,
		Data:        append([]byte(nil), img.Data...),
	})
	return assetPath
}

func (r *htmlRenderer) addFontAsset(src fontdata.Source) string {
	if r == nil {
		return ""
	}
	r.nextFontIndex++
	assetPath := htmlasset.AssetPath(r.options.ExternalFontDir, "font", r.nextFontIndex, htmlasset.FontFileExtension(src.MIME, src.Format))
	r.assets = append(r.assets, HTMLAsset{
		Path:        assetPath,
		ContentType: src.MIME,
		Data:        append([]byte(nil), src.Data...),
	})
	return assetPath
}

func (opts HTMLOptions) normalized() HTMLOptions {
	imageMode := opts.ImageMode
	if imageMode == "" {
		imageMode = HTMLImageModeInline
	}
	fontMode := opts.FontMode
	if fontMode == "" {
		fontMode = HTMLFontModeInline
	}
	imageDir := htmlasset.NormalizeDir(opts.ExternalImageDir)
	if imageMode == HTMLImageModeExternal && imageDir == "" {
		imageDir = "images"
	}
	fontDir := htmlasset.NormalizeDir(opts.ExternalFontDir)
	if fontMode == HTMLFontModeExternal && fontDir == "" {
		fontDir = "fonts"
	}
	return HTMLOptions{
		ImageMode:        imageMode,
		ExternalImageDir: imageDir,
		FontMode:         fontMode,
		ExternalFontDir:  fontDir,
	}
}

func (opts HTMLOptions) validate() error {
	normalized := opts.normalized()
	switch normalized.ImageMode {
	case HTMLImageModeInline, HTMLImageModeExternal:
	default:
		return fmt.Errorf("sheet render: unsupported html image mode %q", normalized.ImageMode)
	}
	switch normalized.FontMode {
	case HTMLFontModeInline, HTMLFontModeExternal:
		return nil
	default:
		return fmt.Errorf("sheet render: unsupported html font mode %q", normalized.FontMode)
	}
}

func (r *htmlRenderer) assetsCopy() []HTMLAsset {
	if r == nil || len(r.assets) == 0 {
		return nil
	}
	assets := make([]HTMLAsset, len(r.assets))
	copy(assets, r.assets)
	return assets
}

func (r *htmlRenderer) renderDiagnostics(b *strings.Builder) {
	if r == nil || r.wb == nil {
		return
	}

	if len(r.wb.Diagnostics) == 0 {
		return
	}

	b.WriteString("<details class=\"diagnostics\">\n")
	b.WriteString("<summary>Unsupported Features</summary>\n")
	b.WriteString("<ul>\n")

	for _, d := range r.wb.Diagnostics {
		severityClass := ""
		switch d.Severity {
		case "error":
			severityClass = "diagnostic-error"
		case "warning":
			severityClass = "diagnostic-warning"
		case "info":
			severityClass = "diagnostic-info"
		}

		message := escapeHTML(d.Message)
		if d.Code != "" {
			message = fmt.Sprintf("[%s] %s", escapeHTML(d.Code), message)
		}
		if d.Location.Part != "" {
			message = fmt.Sprintf("%s (%s)", message, escapeHTML(d.Location.Part))
		}

		fmt.Fprintf(b, "<li class=\"%s\">%s</li>\n", severityClass, message)
	}

	b.WriteString("</ul>\n")
	b.WriteString("</details>\n")
}

func fontStyleValue(style string) string {
	if style == "italic" {
		return "italic"
	}
	return "normal"
}

func fontWeightValue(weight int) int {
	if weight >= 700 {
		return 700
	}
	return 400
}

// buildFormulaEngine seeds a formula engine with the worksheet's known values
// and formulas so it can be reused by both display and validation helpers.
func buildFormulaEngine(sheet *resolve.Sheet, wb *resolve.Workbook, sheetIndex int) *formula.Engine {
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
	rows := sheet.Rows
	for _, row := range rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil || cell.Ref == "" {
				continue
			}
			ref := strings.ToUpper(cell.Ref)
			switch cell.Kind {
			case resolve.CellNumber, resolve.CellDate:
				eng.SetCell(ref, formula.Number(cell.NumValue))
			case resolve.CellBool:
				eng.SetCell(ref, formula.Bool(cell.NumValue != 0))
			case resolve.CellString:
				eng.SetCell(ref, formula.Text(cell.StrValue))
			case resolve.CellFormula:
				if cell.StrValue != "" {
					eng.SetCell(ref, formula.Text(cell.StrValue))
				}
			}
		}
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil || cell.Ref == "" || cell.Formula == "" {
				continue
			}
			ref := strings.ToUpper(cell.Ref)
			if cell.ArrayFormulaRange == "" || strings.EqualFold(cell.ArrayFormulaAnchor, ref) {
				eng.SetFormula(ref, resolve.ExpandStructuredTableRefs(resolveValidationNamedRanges(cell.Formula, wb, sheetIndex), wb, sheetIndex))
			}
		}
	}
	return eng
}

func configureWorkbookFormulaResolvers(ctx *validationWorkbookContext) {
	if ctx == nil {
		return
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
				return resolve.LookupPivotData(ctx.wb, currentSheet, func(sheetName string) *formula.Engine {
					return ctx.engineForSheet(ctx.sheetIndexByName(sheetName))
				}, dataField, pivotTableRef, pivotTableValue, criteria)
			},
		)
	}
}

type formulaSparklineSpec struct {
	chartType string
	color     string
	values    []float64
	max       float64
}

// buildFormulaSparklines recognizes the compatibility encoding used when
// Google Sheets SPARKLINE formulas are stored in XLSX as __xludf.DUMMYFUNCTION.
// They are a visual cell primitive, so fidelity rendering emits the visual
// instead of exposing the evaluator's #VALUE! for the unsupported UDF.
func buildFormulaSparklines(sheet *resolve.Sheet, eng *formula.Engine) map[string]string {
	result := make(map[string]string)
	if sheet == nil || eng == nil {
		return result
	}
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil || cell.Ref == "" || cell.Formula == "" {
				continue
			}
			spec, ok := parseFormulaSparkline(cell.Formula, eng)
			if !ok {
				continue
			}
			result[strings.ToUpper(cell.Ref)] = renderFormulaSparklineSVG(spec)
		}
	}
	return result
}

func parseFormulaSparkline(expr string, eng *formula.Engine) (formulaSparklineSpec, bool) {
	normalized := strings.ReplaceAll(expr, `""`, `"`)
	upper := strings.ToUpper(normalized)
	start := strings.Index(upper, "SPARKLINE(")
	if start < 0 {
		return formulaSparklineSpec{}, false
	}
	body := normalized[start+len("SPARKLINE("):]
	arg, rest, ok := splitFormulaArgument(body)
	if !ok {
		return formulaSparklineSpec{}, false
	}
	values := evaluateSparklineValues(strings.TrimSpace(arg), eng)
	if len(values) == 0 {
		return formulaSparklineSpec{}, false
	}
	spec := formulaSparklineSpec{chartType: "line", color: "#4a5568", values: values}
	options := strings.ToLower(rest)
	if v := sparklineQuotedOption(rest, options, "charttype"); v != "" {
		spec.chartType = strings.ToLower(v)
	}
	for _, key := range []string{"color1", "firstcolor", "color"} {
		if v := sparklineQuotedOption(rest, options, key); v != "" {
			spec.color = v
			break
		}
	}
	for _, key := range []string{"max", "ymax"} {
		if raw := sparklineOptionExpression(rest, options, key); raw != "" {
			if v, err := eng.Evaluate(raw); err == nil && v.IsNumber() {
				spec.max = v.Num
				break
			}
		}
	}
	return spec, true
}

func splitFormulaArgument(body string) (string, string, bool) {
	depth := 0
	inString := false
	for i, r := range body {
		switch r {
		case '"':
			inString = !inString
		case '(':
			if !inString {
				depth++
			}
		case ')':
			if !inString && depth > 0 {
				depth--
			}
		case ',':
			if !inString && depth == 0 {
				return body[:i], body[i+1:], true
			}
		}
	}
	return "", "", false
}

func evaluateSparklineValues(expr string, eng *formula.Engine) []float64 {
	expr = strings.ReplaceAll(strings.TrimSpace(expr), "$", "")
	if parts := strings.Split(expr, ":"); len(parts) == 2 {
		vals := eng.GetRange(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		out := make([]float64, 0, len(vals))
		for _, v := range vals {
			if v.IsNumber() {
				out = append(out, v.Num)
			}
		}
		return out
	}
	v, err := eng.Evaluate(expr)
	if err == nil && v.IsNumber() {
		return []float64{v.Num}
	}
	return nil
}

func sparklineQuotedOption(original, lower, key string) string {
	needle := `"` + strings.ToLower(key) + `"`
	i := strings.Index(lower, needle)
	if i < 0 {
		return ""
	}
	rest := strings.TrimSpace(original[i+len(needle):])
	if len(rest) == 0 || rest[0] != ',' {
		return ""
	}
	rest = strings.TrimSpace(rest[1:])
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	if end := strings.Index(rest[1:], `"`); end >= 0 {
		return rest[1 : 1+end]
	}
	return ""
}

func sparklineOptionExpression(original, lower, key string) string {
	needle := `"` + strings.ToLower(key) + `"`
	i := strings.Index(lower, needle)
	if i < 0 {
		return ""
	}
	rest := strings.TrimSpace(original[i+len(needle):])
	if len(rest) == 0 || rest[0] != ',' {
		return ""
	}
	rest = strings.TrimSpace(rest[1:])
	depth := 0
	for i, r := range rest {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ';', '}':
			if depth == 0 {
				return strings.TrimSpace(rest[:i])
			}
		}
	}
	return strings.TrimSpace(rest)
}

func renderFormulaSparklineSVG(spec formulaSparklineSpec) string {
	if len(spec.values) == 0 {
		return ""
	}
	const w, h = 100.0, 24.0
	maxV := spec.max
	if maxV <= 0 {
		for _, v := range spec.values {
			if v > maxV {
				maxV = v
			}
		}
	}
	if maxV <= 0 {
		maxV = 1
	}
	color := spec.color
	if color == "" {
		color = "#4a5568"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="formula-sparkline" viewBox="0 0 %.0f %.0f" preserveAspectRatio="none" aria-hidden="true">`, w, h)
	switch spec.chartType {
	case "column":
		bw := w / float64(len(spec.values))
		for i, v := range spec.values {
			bh := math.Max(0, math.Min(h, h*v/maxV))
			fmt.Fprintf(&b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="%s"/>`, float64(i)*bw+bw*.12, h-bh, bw*.76, bh, escapeHTMLAttr(color))
		}
	case "bar":
		v := spec.values[0]
		bw := math.Max(0, math.Min(w, w*v/maxV))
		fmt.Fprintf(&b, `<rect x="0" y="5" width="%.2f" height="14" fill="%s"/>`, bw, escapeHTMLAttr(color))
	default:
		if len(spec.values) == 1 {
			y := h - h*spec.values[0]/maxV
			fmt.Fprintf(&b, `<circle cx="50" cy="%.2f" r="2" fill="%s"/>`, y, escapeHTMLAttr(color))
		} else {
			pts := make([]string, 0, len(spec.values))
			for i, v := range spec.values {
				x := float64(i) * w / float64(len(spec.values)-1)
				y := h - h*v/maxV
				pts = append(pts, fmt.Sprintf("%.2f,%.2f", x, y))
			}
			fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="1.5" vector-effect="non-scaling-stroke"/>`, strings.Join(pts, " "), escapeHTMLAttr(color))
		}
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// buildComputedValues evaluates formula cells that have no authoritative cached
// display value and returns a map from upper-case cell reference to computed text.
// Successful source caches remain authoritative for fidelity. Cached formula errors
// are recoverable: when the shared evaluator can produce a successful value, use it
// instead of preserving a stale/error cache from another spreadsheet engine.
func buildComputedValues(sheet *resolve.Sheet, arrayFormulaCells map[string]*resolve.Cell, eng *formula.Engine) map[string]string {
	var rows []*resolve.Row
	if sheet != nil {
		rows = sheet.Rows
	}
	if eng == nil {
		eng = buildFormulaEngine(sheet, nil, -1)
	}
	result := make(map[string]string)
	for _, row := range rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil || cell.Formula == "" || cell.Ref == "" || (cell.HasCachedValue && !cachedFormulaError(cell)) {
				continue
			}
			ref := strings.ToUpper(cell.Ref)
			val := eng.GetCell(ref)
			if val.IsEmpty() {
				continue
			}
			if s := formatComputedFormulaValue(cell, val); s != formatCellValue(cell) {
				result[ref] = s
			}
		}
	}
	for ref, cell := range arrayFormulaCells {
		if cell == nil || cell.Formula == "" || ref == "" || (cell.HasCachedValue && !cachedFormulaError(cell)) {
			continue
		}
		val := eng.GetCell(ref)
		if val.IsEmpty() {
			continue
		}
		if s := formatComputedFormulaValue(cell, val); s != formatCellValue(cell) {
			result[ref] = s
		}
	}
	return result
}

func cachedFormulaError(cell *resolve.Cell) bool {
	if cell == nil || !cell.HasCachedValue || cell.Formula == "" {
		return false
	}
	if cell.Kind == resolve.CellError {
		return true
	}
	text := strings.TrimSpace(formatCellValue(cell))
	return strings.HasPrefix(text, "#")
}

func formatComputedFormulaValue(cell *resolve.Cell, value formula.Value) string {
	if value.IsEmpty() {
		return ""
	}
	if value.IsError() {
		return value.Err.String()
	}
	if value.IsBool() {
		if value.Bool {
			return "TRUE"
		}
		return "FALSE"
	}
	if value.IsNumber() {
		numFmt := ""
		if cell != nil {
			numFmt = cell.Style.NumFmtCode
		}
		return FormatValue(value.Num, numFmt)
	}
	return value.ToText()
}

type computedFormulaContext struct {
	eng        *formula.Engine
	currentRef string
	anchorRef  string
}

func (c *computedFormulaContext) GetCell(ref string) formula.Value {
	if c.eng == nil {
		return formula.Empty()
	}
	return c.eng.GetCell(ref)
}

func (c *computedFormulaContext) GetRange(start, end string) []formula.Value {
	if c.eng == nil {
		return nil
	}
	return c.eng.GetRange(start, end)
}

func (c *computedFormulaContext) CurrentRef() string {
	return c.currentRef
}

func (c *computedFormulaContext) ArrayAnchorRef() string {
	return c.anchorRef
}

func (c *computedFormulaContext) GetFormula(ref string) (string, bool) {
	if c.eng == nil {
		return "", false
	}
	return c.eng.GetFormula(ref)
}

func (c *computedFormulaContext) ResolveSheetOrdinal(token string) (int, bool) {
	if c.eng == nil {
		return 0, false
	}
	return c.eng.ResolveSheetOrdinal(token)
}

func (c *computedFormulaContext) SheetCount() int {
	if c.eng == nil {
		return 0
	}
	return c.eng.SheetCount()
}

func (c *computedFormulaContext) GetSpillRange(ref string) (string, string, bool) {
	if c.eng == nil {
		return "", "", false
	}
	return c.eng.GetSpillRange(ref)
}

func (c *computedFormulaContext) ResolvePivotData(dataField, pivotTableRef string, pivotTableValue formula.Value, criteria []formula.PivotDataCriterion) (formula.Value, bool) {
	if c.eng == nil {
		return formula.Value{}, false
	}
	return c.eng.ResolvePivotData(dataField, pivotTableRef, pivotTableValue, criteria)
}

func buildComputedHyperlinks(sheet *resolve.Sheet, arrayFormulaCells map[string]*resolve.Cell, eng *formula.Engine) map[string]string {
	if eng == nil {
		eng = buildFormulaEngine(sheet, nil, -1)
	}
	result := make(map[string]string)
	var rows []*resolve.Row
	if sheet != nil {
		rows = sheet.Rows
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil || cell.Ref == "" || cell.Formula == "" || cell.HyperlinkURL != "" {
				continue
			}
			if url, ok := computedHyperlinkURL(cell, eng); ok {
				result[strings.ToUpper(cell.Ref)] = url
			}
		}
	}
	for ref, cell := range arrayFormulaCells {
		if cell == nil || ref == "" || cell.Formula == "" || cell.HyperlinkURL != "" {
			continue
		}
		if url, ok := computedHyperlinkURL(cell, eng); ok {
			result[strings.ToUpper(ref)] = url
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func computedHyperlinkURL(cell *resolve.Cell, eng *formula.Engine) (string, bool) {
	if cell == nil || strings.TrimSpace(cell.Formula) == "" || cell.Ref == "" {
		return "", false
	}
	expr, err := formula.Parse(cell.Formula)
	if err != nil {
		return "", false
	}
	call, ok := expr.(*formula.FuncCallExpr)
	if !ok || !strings.EqualFold(call.Name, "HYPERLINK") || len(call.Args) < 1 {
		return "", false
	}
	anchorRef := cell.ArrayFormulaAnchor
	if anchorRef == "" {
		anchorRef = cell.Ref
	}
	ctx := &computedFormulaContext{eng: eng, currentRef: cell.Ref, anchorRef: anchorRef}
	value := call.Args[0].Eval(ctx)
	if value.IsError() || value.IsEmpty() {
		return "", false
	}
	url := value.ToText()
	if strings.TrimSpace(url) == "" {
		return "", false
	}
	return url, true
}

func buildArrayFormulaRenderMap(sheet *resolve.Sheet, eng *formula.Engine) (map[string]*resolve.Cell, map[int]bool) {
	if sheet == nil || eng == nil {
		return nil, nil
	}
	groups := eng.ArrayFormulaGroups()
	if len(groups) == 0 {
		return nil, nil
	}
	arrayCells := make(map[string]*resolve.Cell)
	arrayRows := make(map[int]bool)
	for _, group := range groups {
		for row := group.TopRow; row <= group.BottomRow; row++ {
			arrayRows[row] = true
			for col := group.LeftCol; col <= group.RightCol; col++ {
				ref := strings.ToUpper(colLetterFromIndex(col) + strconv.Itoa(row))
				arrayCells[ref] = &resolve.Cell{
					Row:                row,
					Col:                col,
					Ref:                ref,
					Kind:               resolve.CellFormula,
					Formula:            group.Formula,
					ArrayFormulaRange:  group.RangeRef,
					ArrayFormulaAnchor: group.AnchorRef,
				}
			}
		}
	}
	if len(arrayCells) == 0 && len(arrayRows) == 0 {
		return nil, nil
	}
	return arrayCells, arrayRows
}

type validationWorkbookContext struct {
	wb            *resolve.Workbook
	engines       []*formula.Engine
	externalLinks []*validationExternalWorkbook
}

type validationExternalWorkbook struct {
	index      int
	target     string
	targetBase string
	sheets     map[string]map[string]formula.Value
}

type validationFormulaContext struct {
	workbook          *validationWorkbookContext
	currentSheetIndex int
	currentEngine     *formula.Engine
	cellTargets       map[string]validationRefTarget
}

type validationRefTarget struct {
	sheetIndex int
	ref        string
}

func buildWorkbookFormulaContext(wb *resolve.Workbook) *validationWorkbookContext {
	if wb == nil || len(wb.Sheets) == 0 {
		return nil
	}
	engines := make([]*formula.Engine, len(wb.Sheets))
	for i, sheet := range wb.Sheets {
		if sheet == nil {
			continue
		}
		engines[i] = buildFormulaEngine(sheet, wb, i)
	}
	ctx := &validationWorkbookContext{wb: wb, engines: engines, externalLinks: buildValidationExternalWorkbooks(wb)}
	configureWorkbookFormulaResolvers(ctx)
	return ctx
}

func (ctx *validationWorkbookContext) engineForSheet(sheetIndex int) *formula.Engine {
	if ctx == nil || sheetIndex < 0 || sheetIndex >= len(ctx.engines) {
		return nil
	}
	return ctx.engines[sheetIndex]
}

func (ctx *validationWorkbookContext) getCellForSheet(currentSheetIndex int, ref string) formula.Value {
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
	eng := ctx.engineForSheet(targetSheetIndex)
	if eng == nil {
		return formula.Empty()
	}
	return eng.GetCell(targetRef)
}

func (ctx *validationWorkbookContext) getRangeForSheet(currentSheetIndex int, start, end string) []formula.Value {
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
	if !ok || startSheetIndex != endSheetIndex {
		return nil
	}
	eng := ctx.engineForSheet(startSheetIndex)
	if eng == nil {
		return nil
	}
	return eng.GetRange(startRef, endRef)
}

func (ctx *validationWorkbookContext) resolveSheetRef(currentSheetIndex int, ref string, defaultSheetIndex int) (int, string, bool) {
	if ctx == nil {
		return -1, "", false
	}
	ref = strings.ToUpper(strings.TrimSpace(ref))
	idx := strings.LastIndex(ref, "!")
	if idx < 0 {
		if defaultSheetIndex < 0 {
			defaultSheetIndex = currentSheetIndex
		}
		return defaultSheetIndex, ref, defaultSheetIndex >= 0
	}
	sheetName := normalizeValidationSheetName(ref[:idx])
	targetSheetIndex := ctx.sheetIndexByName(sheetName)
	if targetSheetIndex < 0 {
		return -1, "", false
	}
	return targetSheetIndex, strings.TrimSpace(ref[idx+1:]), true
}

func (ctx *validationWorkbookContext) resolveSheetOrdinal(currentSheetIndex int, token string) (int, bool) {
	if ctx == nil {
		return 0, false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return currentSheetIndex + 1, currentSheetIndex >= 0
	}
	if idx := strings.LastIndex(token, "!"); idx >= 0 {
		targetSheetIndex, _, ok := ctx.resolveSheetRef(currentSheetIndex, token, currentSheetIndex)
		if !ok {
			return 0, false
		}
		return targetSheetIndex + 1, true
	}
	if targetSheetIndex := ctx.sheetIndexByName(token); targetSheetIndex >= 0 {
		return targetSheetIndex + 1, true
	}
	if validationIdentifierLooksLikeCellRef(token) {
		return currentSheetIndex + 1, currentSheetIndex >= 0
	}
	return 0, false
}

func (ctx *validationWorkbookContext) sheetCount() int {
	if ctx == nil || ctx.wb == nil {
		return 0
	}
	return len(ctx.wb.Sheets)
}

func buildValidationExternalWorkbooks(wb *resolve.Workbook) []*validationExternalWorkbook {
	if wb == nil || len(wb.ExternalLinks) == 0 {
		return nil
	}
	var out []*validationExternalWorkbook
	for i, link := range wb.ExternalLinks {
		if link == nil || len(link.CachedSheets) == 0 {
			continue
		}
		cached := &validationExternalWorkbook{
			index:      i + 1,
			target:     strings.ToUpper(strings.TrimSpace(link.Target)),
			targetBase: normalizeExternalWorkbookTargetKey(link.Target),
			sheets:     make(map[string]map[string]formula.Value),
		}
		for _, sheet := range link.CachedSheets {
			if sheet == nil {
				continue
			}
			name := normalizeValidationSheetName(sheet.Name)
			if name == "" {
				continue
			}
			cells := make(map[string]formula.Value)
			for _, cell := range sheet.Cells {
				if cell == nil || strings.TrimSpace(cell.Ref) == "" {
					continue
				}
				cells[strings.ToUpper(strings.TrimSpace(cell.Ref))] = formulaValueFromExternalCell(cell)
			}
			cached.sheets[strings.ToUpper(name)] = cells
		}
		if len(cached.sheets) != 0 {
			out = append(out, cached)
		}
	}
	return out
}

func formulaValueFromExternalCell(cell *resolve.WorkbookExternalCell) formula.Value {
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

func (ctx *validationWorkbookContext) getExternalCell(ref string) (formula.Value, bool) {
	workbookToken, sheetName, cellRef, ok := parseValidationExternalRef(ref, "", "")
	if !ok {
		return formula.Empty(), false
	}
	workbook := ctx.externalWorkbookByToken(workbookToken)
	if workbook == nil {
		return formula.Empty(), false
	}
	value, ok := workbook.cellValue(sheetName, cellRef)
	if !ok {
		return formula.Empty(), false
	}
	return value, true
}

func (ctx *validationWorkbookContext) getExternalRange(start, end string) ([]formula.Value, bool) {
	workbookToken, sheetName, startRef, ok := parseValidationExternalRef(start, "", "")
	if !ok {
		return nil, false
	}
	workbook := ctx.externalWorkbookByToken(workbookToken)
	if workbook == nil {
		return nil, false
	}
	_, endSheetName, endRef, ok := parseValidationExternalRef(end, workbookToken, sheetName)
	if !ok || !strings.EqualFold(sheetName, endSheetName) {
		return nil, false
	}
	values := workbook.rangeValues(sheetName, startRef, endRef)
	if values == nil {
		return nil, false
	}
	return values, true
}

func (ctx *validationWorkbookContext) externalWorkbookByToken(token string) *validationExternalWorkbook {
	if ctx == nil {
		return nil
	}
	token = strings.TrimSpace(strings.Trim(token, "[]"))
	if token == "" {
		return nil
	}
	if index, err := strconv.Atoi(token); err == nil {
		for _, workbook := range ctx.externalLinks {
			if workbook != nil && workbook.index == index {
				return workbook
			}
		}
	}
	normalized := strings.ToUpper(token)
	for _, workbook := range ctx.externalLinks {
		if workbook == nil {
			continue
		}
		if normalized == workbook.target || normalized == workbook.targetBase {
			return workbook
		}
	}
	return nil
}

func (workbook *validationExternalWorkbook) cellValue(sheetName, cellRef string) (formula.Value, bool) {
	if workbook == nil {
		return formula.Empty(), false
	}
	cells := workbook.sheets[strings.ToUpper(normalizeValidationSheetName(sheetName))]
	if len(cells) == 0 {
		return formula.Empty(), false
	}
	value, ok := cells[strings.ToUpper(strings.TrimSpace(cellRef))]
	return value, ok
}

func (workbook *validationExternalWorkbook) rangeValues(sheetName, startRef, endRef string) []formula.Value {
	if workbook == nil {
		return nil
	}
	cells := workbook.sheets[strings.ToUpper(normalizeValidationSheetName(sheetName))]
	if len(cells) == 0 {
		return nil
	}
	startRow, startCol, ok := validationCellRefCoords(startRef)
	if !ok {
		return nil
	}
	endRow, endCol, ok := validationCellRefCoords(endRef)
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
			ref := colLetterFromIndex(col) + strconv.Itoa(row)
			if value, ok := cells[ref]; ok {
				values = append(values, value)
			} else {
				values = append(values, formula.Empty())
			}
		}
	}
	return values
}

func parseValidationExternalRef(ref, defaultWorkbookToken, defaultSheetName string) (string, string, string, bool) {
	ref = strings.ToUpper(strings.TrimSpace(ref))
	idx := strings.LastIndex(ref, "!")
	if idx < 0 {
		if defaultWorkbookToken == "" || defaultSheetName == "" {
			return "", "", "", false
		}
		cellRef := strings.TrimSpace(ref)
		if _, _, ok := readValidationCellRefToken(cellRef); !ok {
			return "", "", "", false
		}
		return defaultWorkbookToken, defaultSheetName, cellRef, true
	}
	sheetQualifier := normalizeValidationSheetName(ref[:idx])
	if !strings.HasPrefix(sheetQualifier, "[") {
		return "", "", "", false
	}
	endBracket := strings.Index(sheetQualifier, "]")
	if endBracket <= 1 || endBracket == len(sheetQualifier)-1 {
		return "", "", "", false
	}
	workbookToken := strings.TrimSpace(sheetQualifier[1:endBracket])
	sheetName := strings.TrimSpace(sheetQualifier[endBracket+1:])
	cellRef := strings.TrimSpace(ref[idx+1:])
	if workbookToken == "" || sheetName == "" {
		return "", "", "", false
	}
	if _, _, ok := readValidationCellRefToken(cellRef); !ok {
		return "", "", "", false
	}
	return workbookToken, sheetName, cellRef, true
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

func buildDVStateMap(wb *resolve.Workbook, sheetIndex int, dvs []*resolve.DataValidation) map[string]dataValidationState {
	workbookCtx := buildWorkbookFormulaContext(wb)
	currentEngine := workbookCtx.engineForSheet(sheetIndex)
	if len(dvs) == 0 || currentEngine == nil {
		return nil
	}
	out := make(map[string]dataValidationState)
	for _, dv := range dvs {
		if dv == nil || strings.TrimSpace(dv.SqRef) == "" {
			continue
		}
		for _, part := range strings.Fields(dv.SqRef) {
			anchorRef := validationAnchorRef(part)
			if anchorRef == "" {
				continue
			}
			for _, ref := range expandSqRef(part) {
				out[ref] = evaluateDataValidationState(workbookCtx, sheetIndex, dv, anchorRef, ref)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func evaluateDataValidationState(workbookCtx *validationWorkbookContext, sheetIndex int, dv *resolve.DataValidation, anchorRef, ref string) dataValidationState {
	eng := workbookCtx.engineForSheet(sheetIndex)
	if dv == nil || eng == nil {
		return dataValidationState{}
	}
	value := eng.GetCell(ref)
	if validationValueIsBlank(value) {
		return dataValidationState{Evaluated: true, Valid: dv.AllowBlank}
	}
	switch dv.Type {
	case "list":
		if len(dv.ListValues) == 0 {
			return dataValidationState{}
		}
		text := value.ToText()
		for _, allowed := range dv.ListValues {
			if text == allowed {
				return dataValidationState{Evaluated: true, Valid: true}
			}
		}
		return dataValidationState{Evaluated: true, Valid: false}
	case "whole":
		num, err := value.ToNumber()
		if err != nil || num != math.Trunc(num) {
			return dataValidationState{Evaluated: true, Valid: false}
		}
		valid, ok := evaluateNumericValidationRule(workbookCtx, sheetIndex, dv.Operator, num, dv.Formula1, dv.Formula2, anchorRef, ref)
		if !ok {
			return dataValidationState{}
		}
		return dataValidationState{Evaluated: true, Valid: valid}
	case "decimal", "date", "time":
		num, err := value.ToNumber()
		if err != nil {
			return dataValidationState{Evaluated: true, Valid: false}
		}
		valid, ok := evaluateNumericValidationRule(workbookCtx, sheetIndex, dv.Operator, num, dv.Formula1, dv.Formula2, anchorRef, ref)
		if !ok {
			return dataValidationState{}
		}
		return dataValidationState{Evaluated: true, Valid: valid}
	case "textLength":
		valid, ok := evaluateNumericValidationRule(workbookCtx, sheetIndex, dv.Operator, float64(utf8.RuneCountInString(value.ToText())), dv.Formula1, dv.Formula2, anchorRef, ref)
		if !ok {
			return dataValidationState{}
		}
		return dataValidationState{Evaluated: true, Valid: valid}
	case "custom":
		result, ok := evaluateValidationFormula(workbookCtx, sheetIndex, dv.Formula1, anchorRef, ref)
		if !ok {
			return dataValidationState{}
		}
		boolVal, err := result.ToBool()
		if err != nil {
			return dataValidationState{Evaluated: true, Valid: false}
		}
		return dataValidationState{Evaluated: true, Valid: boolVal}
	default:
		return dataValidationState{}
	}
}

func validationValueIsBlank(value formula.Value) bool {
	return value.IsEmpty() || (value.IsText() && value.Text == "")
}

func evaluateNumericValidationRule(workbookCtx *validationWorkbookContext, sheetIndex int, operator string, current float64, formula1, formula2, anchorRef, ref string) (bool, bool) {
	v1, ok := evaluateValidationFormula(workbookCtx, sheetIndex, formula1, anchorRef, ref)
	if !ok {
		return false, false
	}
	n1, err := v1.ToNumber()
	if err != nil {
		return false, false
	}
	op := operator
	if op == "" {
		op = "between"
	}
	switch op {
	case "between", "notBetween":
		v2, ok := evaluateValidationFormula(workbookCtx, sheetIndex, formula2, anchorRef, ref)
		if !ok {
			return false, false
		}
		n2, err := v2.ToNumber()
		if err != nil {
			return false, false
		}
		if op == "between" {
			return current >= n1 && current <= n2, true
		}
		return current < n1 || current > n2, true
	case "equal":
		return current == n1, true
	case "notEqual":
		return current != n1, true
	case "greaterThan":
		return current > n1, true
	case "lessThan":
		return current < n1, true
	case "greaterThanOrEqual":
		return current >= n1, true
	case "lessThanOrEqual":
		return current <= n1, true
	default:
		return false, false
	}
}

func evaluateValidationFormula(workbookCtx *validationWorkbookContext, sheetIndex int, expr, anchorRef, ref string) (formula.Value, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" || workbookCtx == nil {
		return formula.Value{}, false
	}
	currentEngine := workbookCtx.engineForSheet(sheetIndex)
	if currentEngine == nil {
		return formula.Value{}, false
	}
	expr = strings.TrimPrefix(expr, "=")
	expr = resolveValidationNamedRanges(expr, workbookCtx.wb, sheetIndex)
	translated := translateValidationFormula(expr, anchorRef, ref)
	formulaCtx := &validationFormulaContext{
		workbook:          workbookCtx,
		currentSheetIndex: sheetIndex,
		currentEngine:     currentEngine,
	}
	translated = formulaCtx.externalizeSheetRefs(translated)
	val, err := formula.EvaluateInContext(translated, formulaCtx.getCell, formulaCtx.getRange)
	if err != nil || val.IsError() {
		return formula.Value{}, false
	}
	return val, true
}

// resolveValidationNamedRanges expands workbook or sheet-local defined names in a
// validation formula into parser-visible formulas or ranges.
func resolveValidationNamedRanges(expr string, wb *resolve.Workbook, sheetIndex int) string {
	return resolveValidationNamedRangesDepth(expr, wb, sheetIndex, 0)
}

func resolveValidationNamedRangesDepth(expr string, wb *resolve.Workbook, sheetIndex, depth int) string {
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
		if isValidationNamedRefLead(expr, i) {
			name, n := readValidationIdentifier(expr[i:])
			if n > 0 {
				end := i + n
				if isValidationWholeWordMatch(expr, i, end) && !validationIdentifierLooksLikeCellRef(name) && !validationIdentifierFollowedByCallOrSheet(expr, end) {
					if value := resolveValidationDefinedName(wb, sheetIndex, name); value != "" {
						result.WriteByte('(')
						result.WriteString(resolveValidationNamedRangesDepth(value, wb, sheetIndex, depth+1))
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

func resolveValidationDefinedName(wb *resolve.Workbook, sheetIndex int, name string) string {
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

// isValidationWholeWordMatch checks that a substring at [start, end) in expr
// is a whole-word match (not preceded/followed by alphanumeric or underscore).
func isValidationWholeWordMatch(expr string, start, end int) bool {
	// Check character before the match.
	if start > 0 {
		ch := expr[start-1]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			return false
		}
	}
	// Check character after the match.
	if end < len(expr) {
		ch := expr[end]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			return false
		}
	}
	return true
}

func isValidationNamedRefLead(expr string, idx int) bool {
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

func readValidationIdentifier(s string) (string, int) {
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

func validationIdentifierLooksLikeCellRef(name string) bool {
	if name == "" {
		return false
	}
	_, _, ok := parseA1(strings.ToUpper(strings.TrimPrefix(name, "\\")))
	return ok
}

func validationIdentifierFollowedByCallOrSheet(expr string, idx int) bool {
	for idx < len(expr) && (expr[idx] == ' ' || expr[idx] == '\t' || expr[idx] == '\n' || expr[idx] == '\r') {
		idx++
	}
	return idx < len(expr) && (expr[idx] == '(' || expr[idx] == '!')
}

func (ctx *validationFormulaContext) externalizeSheetRefs(expr string) string {
	if ctx == nil || expr == "" {
		return expr
	}
	var out strings.Builder
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
			out.WriteString(expr[start:i])
			continue
		}
		sheetName, refToken, n, ok := readValidationSheetQualifiedRef(expr[i:])
		if !ok {
			out.WriteByte(expr[i])
			i++
			continue
		}
		sheetIndex := ctx.workbook.sheetIndexByName(sheetName)
		if sheetIndex < 0 {
			out.WriteString(expr[i : i+n])
			i += n
			continue
		}
		out.WriteString(ctx.addCellTarget(sheetIndex, refToken))
		i += n
		if i < len(expr) && expr[i] == ':' {
			out.WriteByte(':')
			i++
			if nextSheetName, nextRefToken, consumed, ok := readValidationSheetQualifiedRef(expr[i:]); ok {
				nextSheetIndex := ctx.workbook.sheetIndexByName(nextSheetName)
				if nextSheetIndex < 0 {
					out.WriteString(expr[i : i+consumed])
				} else {
					out.WriteString(ctx.addCellTarget(nextSheetIndex, nextRefToken))
				}
				i += consumed
				continue
			}
			if nextRefToken, consumed, ok := readValidationCellRefToken(expr[i:]); ok {
				out.WriteString(ctx.addCellTarget(sheetIndex, nextRefToken))
				i += consumed
				continue
			}
		}
	}
	return out.String()
}

func (ctx *validationFormulaContext) addCellTarget(sheetIndex int, ref string) string {
	if ctx.cellTargets == nil {
		ctx.cellTargets = make(map[string]validationRefTarget)
	}
	key := fmt.Sprintf("_DVREF%d", len(ctx.cellTargets)+1)
	ctx.cellTargets[key] = validationRefTarget{sheetIndex: sheetIndex, ref: strings.ToUpper(ref)}
	return key
}

func (ctx *validationFormulaContext) getCell(ref string) formula.Value {
	if ctx == nil {
		return formula.Empty()
	}
	ref = strings.ToUpper(strings.TrimSpace(ref))
	if target, ok := ctx.cellTargets[ref]; ok {
		if eng := ctx.workbook.engineForSheet(target.sheetIndex); eng != nil {
			return eng.GetCell(target.ref)
		}
		return formula.Empty()
	}
	if ctx.currentEngine != nil {
		return ctx.currentEngine.GetCell(ref)
	}
	return formula.Empty()
}

func (ctx *validationFormulaContext) getRange(start, end string) []formula.Value {
	if ctx == nil {
		return nil
	}
	start = strings.ToUpper(strings.TrimSpace(start))
	end = strings.ToUpper(strings.TrimSpace(end))
	if startTarget, ok := ctx.cellTargets[start]; ok {
		endTarget, ok := ctx.cellTargets[end]
		if !ok || startTarget.sheetIndex != endTarget.sheetIndex {
			return nil
		}
		if eng := ctx.workbook.engineForSheet(startTarget.sheetIndex); eng != nil {
			return eng.GetRange(startTarget.ref, endTarget.ref)
		}
		return nil
	}
	if ctx.currentEngine != nil {
		return ctx.currentEngine.GetRange(start, end)
	}
	return nil
}

func (ctx *validationFormulaContext) ResolvePivotData(dataField, pivotTableRef string, pivotTableValue formula.Value, criteria []formula.PivotDataCriterion) (formula.Value, bool) {
	if ctx == nil || ctx.workbook == nil || ctx.workbook.wb == nil {
		return formula.Value{}, false
	}
	currentSheet := ""
	if ctx.currentSheetIndex >= 0 && ctx.currentSheetIndex < len(ctx.workbook.wb.Sheets) && ctx.workbook.wb.Sheets[ctx.currentSheetIndex] != nil {
		currentSheet = ctx.workbook.wb.Sheets[ctx.currentSheetIndex].Name
	}
	return resolve.LookupPivotData(ctx.workbook.wb, currentSheet, func(sheetName string) *formula.Engine {
		return ctx.workbook.engineForSheet(ctx.workbook.sheetIndexByName(sheetName))
	}, dataField, pivotTableRef, pivotTableValue, criteria)
}

func (ctx *validationWorkbookContext) sheetIndexByName(name string) int {
	if ctx == nil || ctx.wb == nil {
		return -1
	}
	name = normalizeValidationSheetName(name)
	for i, sheet := range ctx.wb.Sheets {
		if sheet != nil && strings.EqualFold(sheet.Name, name) {
			return i
		}
	}
	return -1
}

func normalizeValidationSheetName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		name = strings.ReplaceAll(name[1:len(name)-1], "''", "'")
	}
	return name
}

func readValidationSheetQualifiedRef(s string) (sheetName, ref string, n int, ok bool) {
	if s == "" {
		return "", "", 0, false
	}
	sheetName, consumed, ok := readValidationSheetQualifier(s)
	if !ok || consumed >= len(s) || s[consumed] != '!' {
		return "", "", 0, false
	}
	ref, refLen, ok := readValidationCellRefToken(s[consumed+1:])
	if !ok {
		return "", "", 0, false
	}
	return sheetName, ref, consumed + 1 + refLen, true
}

func readValidationSheetQualifier(s string) (string, int, bool) {
	if s == "" {
		return "", 0, false
	}
	if s[0] == '\'' {
		i := 1
		for i < len(s) {
			if s[i] == '\'' {
				if i+1 < len(s) && s[i+1] == '\'' {
					i += 2
					continue
				}
				return s[:i+1], i + 1, true
			}
			i++
		}
		return "", 0, false
	}
	i := 0
	for i < len(s) {
		ch := s[i]
		if ch == '!' {
			break
		}
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '(' || ch == ')' || ch == ',' || ch == ';' || ch == '+' || ch == '-' || ch == '*' || ch == '/' || ch == '^' || ch == '=' || ch == '<' || ch == '>' || ch == ':' {
			return "", 0, false
		}
		i++
	}
	if i == 0 || i >= len(s) {
		return "", 0, false
	}
	return s[:i], i, true
}

func validationAnchorRef(part string) string {
	part = strings.TrimSpace(part)
	if part == "" {
		return ""
	}
	if colon := strings.Index(part, ":"); colon >= 0 {
		part = part[:colon]
	}
	part = strings.ToUpper(strings.ReplaceAll(part, "$", ""))
	if _, _, ok := parseA1(part); !ok {
		return ""
	}
	return part
}

func translateValidationFormula(expr, anchorRef, ref string) string {
	if expr == "" || anchorRef == "" || ref == "" {
		return expr
	}
	anchorRow, anchorCol, ok1 := parseA1(anchorRef)
	targetRow, targetCol, ok2 := parseA1(ref)
	if !ok1 || !ok2 {
		return expr
	}
	return translateValidationRefs(expr, targetRow-anchorRow, targetCol-anchorCol)
}

func translateValidationRefs(expr string, rowDelta, colDelta int) string {
	if expr == "" || (rowDelta == 0 && colDelta == 0) {
		return expr
	}
	var out strings.Builder
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
			out.WriteString(expr[start:i])
			continue
		}
		if isValidationCellRefLead(expr, i) {
			token, n, ok := readValidationCellRefToken(expr[i:])
			if ok {
				out.WriteString(shiftValidationCellRefToken(token, rowDelta, colDelta))
				i += n
				continue
			}
		}
		out.WriteByte(expr[i])
		i++
	}
	return out.String()
}

func isValidationCellRefLead(expr string, idx int) bool {
	if idx < 0 || idx >= len(expr) {
		return false
	}
	ch := expr[idx]
	if ch != '$' && !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) {
		return false
	}
	if idx == 0 {
		return true
	}
	prev := expr[idx-1]
	if (prev >= 'A' && prev <= 'Z') || (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') || prev == '_' || prev == '.' {
		return false
	}
	return true
}

func readValidationCellRefToken(s string) (string, int, bool) {
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

func validationCellRefCoords(ref string) (int, int, bool) {
	ref = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(ref), "$", ""))
	if ref == "" {
		return 0, 0, false
	}
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		i++
	}
	if i == 0 || i == len(ref) {
		return 0, 0, false
	}
	col, err := colIndexFromLetters(ref[:i])
	if err != nil {
		return 0, 0, false
	}
	row, err := strconv.Atoi(ref[i:])
	if err != nil || row < 1 {
		return 0, 0, false
	}
	return row, col, true
}

func shiftValidationCellRefToken(token string, rowDelta, colDelta int) string {
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
	colValue, err := colIndexFromLetters(colLetters)
	if err != nil {
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
	out.WriteString(colLetterFromIndex(colValue))
	if rowAbs {
		out.WriteByte('$')
	}
	out.WriteString(strconv.Itoa(rowValue))
	return out.String()
}

func colIndexFromLetters(letters string) (int, error) {
	letters = strings.ToUpper(strings.TrimSpace(letters))
	if letters == "" {
		return 0, fmt.Errorf("empty column letters")
	}
	col := 0
	for _, ch := range letters {
		if ch < 'A' || ch > 'Z' {
			return 0, fmt.Errorf("invalid column letters")
		}
		col = col*26 + int(ch-'A') + 1
	}
	return col, nil
}

// formatCellValue converts a resolved cell value to its display string.
func formatCellValue(cell *resolve.Cell) string {
	switch cell.Kind {
	case resolve.CellString, resolve.CellFormula:
		return cell.StrValue
	case resolve.CellError:
		return cell.StrValue
	case resolve.CellBool:
		if cell.NumValue != 0 {
			return "TRUE"
		}
		return "FALSE"
	case resolve.CellNumber, resolve.CellDate:
		return FormatValue(cell.NumValue, cell.Style.NumFmtCode)
	}
	return ""
}

func defaultAlign(kind resolve.CellKind) string {
	switch kind {
	case resolve.CellNumber, resolve.CellBool, resolve.CellDate:
		return "right"
	}
	return "left"
}

// drawingLayout holds precomputed cumulative pixel positions for columns and rows.
// colX[c] is the pixel X of the left edge of column c (1-based).
// colW[c] is the visual width of column c; rowY[r] is the pixel Y of the top
// edge of row r and rowH[r] its visual height.
type drawingLayout struct {
	colX          map[int]int
	colW          map[int]int
	rowY          map[int]int
	rowH          map[int]int
	defaultColPxV int
	defaultRowPxV int
	// dataRightPx is the right edge of the sheet's populated columns, used to
	// keep a title banner from spilling past the table it heads.
	dataRightPx int
}

type renderedFormControl struct {
	control        *resolve.FormControl
	radioName      string
	checked        *bool
	options        []string
	selected       map[int]bool
	multiple       bool
	visibleOptions int
}

type radioGroupBox struct {
	link          string
	left, top     int
	right, bottom int
}

// buildDrawingLayout computes per-column and per-row cumulative pixel offsets.
func buildDrawingLayout(sheet *resolve.Sheet) drawingLayout {
	defaultColPxV := defaultColPx
	defaultRowPxV := worksheetDefaultRowPx(sheet)
	if sheet.DefaultColWidthChars > 0 {
		defaultColPxV = worksheetColumnWidthPx(sheet.DefaultColWidthChars)
	}

	// Determine max col/row needed (from drawings and data rows).
	maxCol, maxRow := 0, 0
	dataMaxCol := 0
	for _, row := range sheet.Rows {
		if row.Index > maxRow {
			maxRow = row.Index
		}
		for _, cell := range row.Cells {
			if cell.Col > maxCol {
				maxCol = cell.Col
			}
			if cell.Col > dataMaxCol {
				dataMaxCol = cell.Col
			}
		}
	}
	for _, d := range sheet.Drawings {
		p := d.Position
		if p.FromCell.Col > maxCol {
			maxCol = p.FromCell.Col
		}
		if p.ToCell.Col > maxCol {
			maxCol = p.ToCell.Col
		}
		if p.FromCell.Row > maxRow {
			maxRow = p.FromCell.Row
		}
		if p.ToCell.Row > maxRow {
			maxRow = p.ToCell.Row
		}
	}
	// VML controls use zero-based cell anchors and may live completely outside
	// the populated cell range. Include their far anchor in the geometry
	// envelope so cumulative offsets exist even on otherwise sparse sheets.
	for _, control := range sheet.FormControls {
		if control == nil {
			continue
		}
		if col, row, ok := vmlAnchorFarCell(control.Anchor); ok {
			if col > maxCol {
				maxCol = col
			}
			if row > maxRow {
				maxRow = row
			}
		}
	}

	// Build row geometry from the same visibility and explicit-height semantics
	// used by the grid. Hidden rows/columns occupy no visual space, while their
	// authored sizes remain in IR for round-trip and later unhiding.
	rowByIndex := make(map[int]*resolve.Row, len(sheet.Rows))
	for _, row := range sheet.Rows {
		if row != nil {
			rowByIndex[row.Index] = row
		}
	}

	// Compute cumulative column X positions.
	colX := make(map[int]int, maxCol)
	colW := make(map[int]int, maxCol)
	x := 0
	for c := 1; c <= maxCol; c++ {
		width := worksheetColumnVisualWidthPx(sheet, c, defaultColPxV)
		colX[c] = x
		colW[c] = width
		x += width
	}

	// Compute cumulative row Y positions.
	rowY := make(map[int]int, maxRow)
	rowH := make(map[int]int, maxRow)
	y := 0
	for rowIndex := 1; rowIndex <= maxRow; rowIndex++ {
		height := worksheetRowVisualHeightPx(sheet, rowByIndex[rowIndex], defaultRowPxV)
		rowY[rowIndex] = y
		rowH[rowIndex] = height
		y += height
	}

	dataRightPx := 0
	if dataMaxCol > 0 {
		dataRightPx = colX[dataMaxCol] + colW[dataMaxCol]
	}
	return drawingLayout{
		colX: colX, colW: colW, rowY: rowY, rowH: rowH,
		defaultColPxV: defaultColPxV, defaultRowPxV: defaultRowPxV,
		dataRightPx: dataRightPx,
	}
}

func worksheetColumnVisualWidthPx(sheet *resolve.Sheet, col int, defaultWidthPx int) int {
	if sheet == nil || col <= 0 {
		return defaultWidthPx
	}
	if sheet.HiddenCols[col] {
		return 0
	}
	if width, ok := sheet.ColumnWidths[col]; ok {
		return worksheetColumnWidthPx(width)
	}
	return defaultWidthPx
}

// worksheetDefaultRowPx returns the pixel height shared by rows that carry no
// explicit height of their own.
func worksheetDefaultRowPx(sheet *resolve.Sheet) int {
	if sheet != nil && sheet.DefaultRowHeightPt > 0 {
		return int(math.Round(sheet.DefaultRowHeightPt * 4 / 3))
	}
	return defaultRowPx
}

// worksheetDefaultColumnPx returns the pixel width shared by columns that carry
// no explicit width of their own.
func worksheetDefaultColumnPx(sheet *resolve.Sheet) int {
	if sheet != nil && sheet.DefaultColWidthChars > 0 {
		return worksheetColumnWidthPx(sheet.DefaultColWidthChars)
	}
	return defaultColPx
}

// extendRowsToOverlay grows the rendered row envelope until the grid covers the
// lowest anchored object, so the panel area keeps its cells, row heights and
// borders instead of falling outside the table. The sheet JS pads the grid
// further to fill whatever box the viewer actually shows.
func extendRowsToOverlay(sheet *resolve.Sheet, rowByIndex map[int]*resolve.Row, maxRow int, overlay drawingOverlay) int {
	if overlay.bottomPx <= 0 {
		return maxRow
	}
	defaultPx := worksheetDefaultRowPx(sheet)
	if defaultPx <= 0 {
		return maxRow
	}
	bottom := 0
	for row := 1; row <= maxRow; row++ {
		bottom += worksheetRowVisualHeightPx(sheet, rowByIndex[row], defaultPx)
	}
	for bottom < overlay.bottomPx {
		bottom += defaultPx
		maxRow++
	}
	return maxRow
}

// extendColumnsToOverlay grows the rendered column envelope until the grid
// covers the right-most anchored object.
func extendColumnsToOverlay(sheet *resolve.Sheet, maxCol int, overlay drawingOverlay) int {
	if overlay.rightPx <= 0 {
		return maxCol
	}
	defaultPx := worksheetDefaultColumnPx(sheet)
	if defaultPx <= 0 {
		return maxCol
	}
	right := 0
	for col := 1; col <= maxCol; col++ {
		right += worksheetColumnVisualWidthPx(sheet, col, defaultPx)
	}
	for right < overlay.rightPx {
		width := worksheetColumnVisualWidthPx(sheet, maxCol+1, defaultPx)
		if width <= 0 {
			break
		}
		right += width
		maxCol++
	}
	return maxCol
}

func worksheetRowVisualHeightPx(sheet *resolve.Sheet, row *resolve.Row, defaultHeightPx int) int {
	if row != nil && row.Hidden {
		return 0
	}
	if row != nil && row.HeightPt > 0 {
		return int(math.Round(row.HeightPt * 4 / 3))
	}
	if sheet != nil && sheet.DefaultRowHeightZeroHeight {
		return 0
	}
	return defaultHeightPx
}

func vmlAnchorFarCell(anchor string) (col, row int, ok bool) {
	parts := strings.Split(anchor, ",")
	if len(parts) != 8 {
		return 0, 0, false
	}
	col0, errCol := strconv.Atoi(strings.TrimSpace(parts[4]))
	row0, errRow := strconv.Atoi(strings.TrimSpace(parts[6]))
	if errCol != nil || errRow != nil || col0 < 0 || row0 < 0 {
		return 0, 0, false
	}
	// drawingLayout is keyed by one-based worksheet coordinates.
	return col0 + 1, row0 + 1, true
}

func worksheetColumnWidthPx(widthChars float64) int {
	if widthChars <= 0 {
		return 0
	}
	// OOXML stores column widths in Excel's character-based units, not CSS ch.
	// Convert using the same MDW=7 approximation already used for drawing layout.
	pixels := int(math.Trunc(((256*widthChars + math.Trunc(128.0/charUnitToPx)) / 256.0) * charUnitToPx))
	if pixels < 1 {
		return 1
	}
	return pixels
}

const emusPerPx = 9525 // 914400 EMU/inch ÷ 96 px/inch

// colRightPx returns the pixel X of the right edge of column col (1-based).
// It reports 0 when the layout has no entry for that column.
func (dl drawingLayout) colRightPx(col int) int {
	left, ok := dl.colX[col]
	if !ok {
		return 0
	}
	return left + dl.colW[col]
}

// rowBottomPx returns the pixel Y of the bottom edge of row row (1-based).
// It reports 0 when the layout has no entry for that row.
func (dl drawingLayout) rowBottomPx(row int) int {
	top, ok := dl.rowY[row]
	if !ok {
		return 0
	}
	return top + dl.rowH[row]
}

func (dl drawingLayout) anchorXPx(col int, colOffEMU int64) int {
	x := 0
	if v, ok := dl.colX[col]; ok {
		x = v
	}
	return x + int(colOffEMU/emusPerPx)
}

func (dl drawingLayout) anchorYPx(row int, rowOffEMU int64) int {
	y := 0
	if v, ok := dl.rowY[row]; ok {
		y = v
	}
	return y + int(rowOffEMU/emusPerPx)
}

// drawingBottomPx returns the bottom Y extent (top + height) of a drawing in pixels.
func drawingBottomPx(d *resolve.Drawing, dl drawingLayout) int {
	_, y, _, h := drawingBoxPx(d, dl)
	return y + h
}

// drawingRightPx returns the right X extent of a drawing in pixels, using the
// same anchor resolution as renderDrawing.
func drawingRightPx(d *resolve.Drawing, dl drawingLayout) int {
	x, _, w, _ := drawingBoxPx(d, dl)
	return x + w
}

// drawingOverlay is a sheet's prepared anchored-object layer: the shared pixel
// layout, the rendered form controls, and the extent of the area the anchored
// objects occupy.
type drawingOverlay struct {
	drawings []*resolve.Drawing
	layout   drawingLayout
	controls []renderedFormControl
	bottomPx int
	rightPx  int
}

// buildDrawingOverlay resolves the anchored drawings and form controls once so
// the grid wrapper can be sized before the layer markup is written.
func (r *htmlRenderer) buildDrawingOverlay(sheet *resolve.Sheet, sheetIndex int) drawingOverlay {
	dl := buildDrawingLayout(sheet)
	controls := buildRenderedFormControls(r.wb, sheet, sheetIndex, dl)
	bottom, right := 0, 0
	for _, d := range sheet.Drawings {
		if b := drawingBottomPx(d, dl); b > bottom {
			bottom = b
		}
		if r := drawingRightPx(d, dl); r > right {
			right = r
		}
	}
	for _, rendered := range controls {
		x, y, w, h := vmlAnchorToPixels(rendered.control.Anchor, dl)
		if y+h > bottom {
			bottom = y + h
		}
		if x+w > right {
			right = x + w
		}
	}
	return drawingOverlay{drawings: sheet.Drawings, layout: dl, controls: controls, bottomPx: bottom, rightPx: right}
}

// renderDrawingsLayer writes the anchored drawings and form controls overlay.
func (r *htmlRenderer) renderDrawingsLayer(b *strings.Builder, overlay drawingOverlay) {
	b.WriteString("<div class=\"drawings-layer\">\n")
	for _, d := range overlay.drawings {
		r.renderDrawing(b, d, overlay.layout)
	}
	for _, rendered := range overlay.controls {
		renderFormControl(b, rendered, overlay.layout)
	}
	b.WriteString("</div>\n")
}

func (r *htmlRenderer) renderDrawing(b *strings.Builder, d *resolve.Drawing, dl drawingLayout) {
	x, y, w, h := drawingBoxPx(d, dl)
	posStyle := fmt.Sprintf("left:%dpx;top:%dpx;width:%dpx;height:%dpx;z-index:%d", x, y, w, h, d.ZOrder+1)
	r.renderDrawingAt(b, d, posStyle, w, h)
}

// drawingBoxPx resolves a drawing's pixel box, using the same anchor math for
// the renderer and the overlay extents.
func drawingBoxPx(d *resolve.Drawing, dl drawingLayout) (x, y, w, h int) {
	p := d.Position
	switch p.Anchor {
	case resolve.AnchorAbsolute:
		x = int(p.X / emusPerPx)
		y = int(p.Y / emusPerPx)
		w = int(p.Width / emusPerPx)
		h = int(p.Height / emusPerPx)
	case resolve.AnchorOneCell:
		x = dl.anchorXPx(p.FromCell.Col, p.FromCell.ColOffset)
		y = dl.anchorYPx(p.FromCell.Row, p.FromCell.RowOffset)
		w = int(p.Width / emusPerPx)
		h = int(p.Height / emusPerPx)
	default: // AnchorTwoCell
		x = dl.anchorXPx(p.FromCell.Col, p.FromCell.ColOffset)
		y = dl.anchorYPx(p.FromCell.Row, p.FromCell.RowOffset)
		x2 := dl.anchorXPx(p.ToCell.Col, p.ToCell.ColOffset)
		y2 := dl.anchorYPx(p.ToCell.Row, p.ToCell.RowOffset)
		// Both corners carry their own sub-cell inset; Excel keeps them literally
		// (the "Book tour" control on the Home sheet measures one row band tall
		// with the authored 3px shift), so the button caption centers in its own
		// box instead of drifting into the row below.
		w = x2 - x
		h = y2 - y
	}
	w = clampBannerWidthPx(d, x, w, dl)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return x, y, w, h
}

// clampBannerWidthPx trims a title banner that starts inside the sheet's
// populated columns and overruns them by less than one default column — an
// anchor that landed a column past the data rather than an intentional
// overhang — so the banner spans the table it heads. Charts, pictures and wider
// shapes keep their authored size.
func clampBannerWidthPx(d *resolve.Drawing, x, w int, dl drawingLayout) int {
	if dl.dataRightPx <= 0 || dl.defaultColPxV <= 0 {
		return w
	}
	if d == nil || d.Type != resolve.DrawingTypeShape || d.Shape == nil || d.Shape.Text == "" {
		return w
	}
	if x >= dl.dataRightPx || x+w <= dl.dataRightPx {
		return w
	}
	if over := x + w - dl.dataRightPx; over > dl.defaultColPxV {
		return w
	}
	// Keep the title inside the sheet's right-side inset. Legacy BIFF client
	// anchors commonly land in the first unused column; clamping to the exact
	// grid edge makes a heading wider than its OOXML counterpart. A quarter of
	// a default column is the grid's existing inner gutter at this boundary.
	innerGutter := (dl.defaultColPxV - 1) / 4
	if width := dl.dataRightPx - x - innerGutter; width > 0 {
		return width
	}
	return dl.dataRightPx - x
}

func (r *htmlRenderer) renderDrawingAt(b *strings.Builder, d *resolve.Drawing, posStyle string, widthPx, heightPx int) {
	switch d.Type {
	case resolve.DrawingTypeImage:
		r.renderDrawingImage(b, d, posStyle)
	case resolve.DrawingTypeConnector:
		renderDrawingConnector(b, d, posStyle, float64(widthPx), float64(heightPx))
	case resolve.DrawingTypeShape:
		renderDrawingShape(b, d, posStyle, float64(widthPx), float64(heightPx))
	case resolve.DrawingTypeChart:
		r.renderDrawingChart(b, d, posStyle, widthPx, heightPx)
	case resolve.DrawingTypeGroup:
		r.renderDrawingGroup(b, d, posStyle, widthPx, heightPx)
	case resolve.DrawingTypeSmartArt:
		r.renderDrawingSmartArt(b, d, posStyle, widthPx, heightPx)
	case resolve.DrawingTypeOLE:
		label := "Embedded object"
		if d.OLE != nil && d.OLE.ProgID != "" {
			label = d.OLE.ProgID
		}
		fmt.Fprintf(b, "<div class=\"drawing drawing--ole\" style=%q title=%q>%s</div>\n", posStyle, escapeHTML(d.Name), escapeHTML(label))
	}
}

func (r *htmlRenderer) renderDrawingSmartArt(b *strings.Builder, d *resolve.Drawing, posStyle string, widthPx, heightPx int) {
	if d == nil || d.SmartArt == nil {
		return
	}
	sa := d.SmartArt
	dataXML := smartArtRenderDataModel(sa)
	if len(dataXML) == 0 {
		fmt.Fprintf(b, "<div class=\"drawing drawing--smartart\" style=%q title=%q>SmartArt</div>\n", posStyle, escapeHTML(d.Name))
		return
	}
	frame := &smartartRender.FrameBounds{W: float64(widthPx) * emusPerPx, H: float64(heightPx) * emusPerPx}
	shapes, err := smartartRender.ProcessDiagramWithOptionsAndFrame(
		dataXML,
		smartArtObjectPartData(sa, sa.LayoutRelID),
		smartArtObjectPartData(sa, sa.QuickStyleRelID),
		smartArtObjectPartData(sa, sa.ColorsRelID),
		findSmartArtDrawingXML(sa),
		frame,
		nil,
	)
	if err != nil || len(shapes) == 0 {
		fmt.Fprintf(b, "<div class=\"drawing drawing--smartart\" style=%q title=%q>SmartArt</div>\n", posStyle, escapeHTML(d.Name))
		return
	}

	fmt.Fprintf(b, "<div class=\"drawing drawing--smartart drawing--smartart-semantic\" style=%q title=%q>\n", posStyle+";overflow:visible", escapeHTML(d.Name))
	shapeByID := make(map[string]*smartartRender.SmartArtShape, len(shapes))
	for _, sh := range shapes {
		if sh != nil {
			shapeByID[sh.ID] = sh
		}
	}
	for _, cxn := range sa.Connections {
		src, sok := shapeByID[cxn.SrcID]
		dst, dok := shapeByID[cxn.DestID]
		if !sok || !dok || src == nil || dst == nil {
			continue
		}
		sx := (src.X + src.Width/2) * 96.0 / 72.0
		sy := (src.Y + src.Height/2) * 96.0 / 72.0
		dx := (dst.X + dst.Width/2) * 96.0 / 72.0
		dy := (dst.Y + dst.Height/2) * 96.0 / 72.0
		left, top := math.Min(sx, dx), math.Min(sy, dy)
		w, h := math.Abs(dx-sx), math.Abs(dy-sy)
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		line := &drawing.LineStyle{Color: "808080", Width: 1.25, Alpha: 1}
		tmp := &resolve.Drawing{Name: "SmartArt connector", Type: resolve.DrawingTypeConnector, Shape: &resolve.DrawingShape{ConnectorType: "straightConnector1", LineColor: line.Color, LineWidthPt: line.Width, LineStyle: line}}
		style := fmt.Sprintf("left:%.2fpx;top:%.2fpx;width:%.2fpx;height:%.2fpx", left, top, w, h)
		renderDrawingConnector(b, tmp, style, w, h)
	}
	for _, sh := range shapes {
		if sh == nil || sh.ID == "__background" {
			continue
		}
		x := sh.X * 96.0 / 72.0
		y := sh.Y * 96.0 / 72.0
		w := sh.Width * 96.0 / 72.0
		h := sh.Height * 96.0 / 72.0
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		sp := &resolve.DrawingShape{Geometry: sh.Type, Adjustments: append([]float64(nil), sh.GeometryAdjustments...), Fill: sh.Fill, Effects: sh.Effect, LineStyle: sh.Line, Text: sh.Text}
		if sh.Text == "" {
			sp.Text = sh.PlaceholderText
		}
		if solid, ok := sh.Fill.(drawing.SolidFill); ok {
			sp.FillColor = solid.Color
		} else if solid, ok := sh.Fill.(*drawing.SolidFill); ok && solid != nil {
			sp.FillColor = solid.Color
		}
		if sh.Line != nil {
			sp.LineColor = sh.Line.Color
			sp.LineWidthPt = sh.Line.Width
		}
		tmp := &resolve.Drawing{Name: sh.ID, Type: resolve.DrawingTypeShape, Shape: sp}
		style := fmt.Sprintf("left:%.2fpx;top:%.2fpx;width:%.2fpx;height:%.2fpx", x, y, w, h)
		renderDrawingShape(b, tmp, style, w, h)
	}
	b.WriteString("</div>\n")
}

func smartArtRenderDataModel(sa *resolve.DrawingSmartArt) []byte {
	if sa == nil {
		return nil
	}
	if !sa.SemanticDirty {
		return smartArtObjectPartData(sa, sa.DataRelID)
	}
	d := &smartart.Diagram{Type: sa.DiagramType, Layout: &smartart.Layout{Type: sa.DiagramType, Direction: "topDown", HSpacing: 50, VSpacing: 80}}
	for _, n := range sa.Nodes {
		if n == nil {
			continue
		}
		node := &smartart.Node{ID: n.ID, Type: n.Type, Text: n.Text, TextColor: n.TextColor}
		if n.FillColor != "" {
			node.Fill = drawing.SolidFill{Color: n.FillColor, Alpha: 1}
		}
		d.Nodes = append(d.Nodes, node)
	}
	for _, c := range sa.Connections {
		d.Connections = append(d.Connections, smartart.Connection{SrcID: c.SrcID, DestID: c.DestID})
	}
	return smartart.MarshalDataModel(d)
}

func smartArtObjectPartData(sa *resolve.DrawingSmartArt, relID string) []byte {
	if sa == nil || relID == "" {
		return nil
	}
	part, ok := sa.Parts[relID]
	if !ok {
		return nil
	}
	return part.Data
}

func findSmartArtDrawingXML(sa *resolve.DrawingSmartArt) []byte {
	if sa == nil || sa.SemanticDirty {
		return nil
	}
	var visit func(resolve.DrawingObjectPart) []byte
	visit = func(part resolve.DrawingObjectPart) []byte {
		if strings.Contains(strings.ToLower(part.RelType), "diagramdrawing") && len(part.Data) > 0 {
			return part.Data
		}
		for _, child := range part.Children {
			if data := visit(child); len(data) > 0 {
				return data
			}
		}
		return nil
	}
	for _, part := range sa.Parts {
		if data := visit(part); len(data) > 0 {
			return data
		}
	}
	return nil
}

func (r *htmlRenderer) renderDrawingChart(b *strings.Builder, d *resolve.Drawing, posStyle string, widthPx, heightPx int) {
	if d.Chart != nil {
		svgContent := d.Chart.SVG
		if parsed, ok := d.Chart.Chart.(*chart.Chart); ok && parsed != nil {
			svgContent = parsed.SVG(widthPx, heightPx)
		} else if svgContent != "" {
			svgContent = normalizeDrawingSVGSize(svgContent, widthPx, heightPx)
		}
		if svgContent != "" {
			fmt.Fprintf(b, "<div class=\"drawing drawing--chart\" style=%q title=%q>\n%s\n</div>\n",
				posStyle, escapeHTML(d.Name), svgContent)
			return
		}
	}
	fmt.Fprintf(b, "<div class=\"drawing drawing--chart\" style=%q title=%q>chart</div>\n",
		posStyle, escapeHTML(d.Name))
}

func normalizeDrawingSVGSize(svg string, widthPx, heightPx int) string {
	start := strings.Index(svg, "<svg")
	if start < 0 {
		return svg
	}
	endRel := strings.IndexByte(svg[start:], '>')
	if endRel < 0 {
		return svg
	}
	end := start + endRel
	tag := svg[start:end]
	tag = replaceSVGDimensionAttribute(tag, "width", widthPx)
	tag = replaceSVGDimensionAttribute(tag, "height", heightPx)
	return svg[:start] + tag + svg[end:]
}

func replaceSVGDimensionAttribute(tag, name string, value int) string {
	needle := name + "=\""
	start := strings.Index(tag, needle)
	if start < 0 {
		return tag + fmt.Sprintf(" %s=\"%d\"", name, value)
	}
	valueStart := start + len(needle)
	valueEndRel := strings.IndexByte(tag[valueStart:], '"')
	if valueEndRel < 0 {
		return tag
	}
	valueEnd := valueStart + valueEndRel
	return tag[:valueStart] + strconv.Itoa(value) + tag[valueEnd:]
}

func (r *htmlRenderer) renderDrawingGroup(b *strings.Builder, d *resolve.Drawing, posStyle string, widthPx, heightPx int) {
	if d.Group == nil {
		return
	}
	fmt.Fprintf(b, "<div class=\"drawing drawing--group\" style=%q title=%q>", strings.Join([]string{posStyle, "overflow:visible"}, ";"), escapeHTML(d.Name))
	parentEMUWidth, parentEMUHeight := drawingGroupExtentEMU(d, widthPx, heightPx)
	for _, child := range d.Group.Children {
		if child == nil {
			continue
		}
		childStyle, childWidthPx, childHeightPx := groupedDrawingStyle(child, parentEMUWidth, parentEMUHeight, widthPx, heightPx)
		r.renderDrawingAt(b, child, childStyle, childWidthPx, childHeightPx)
	}
	b.WriteString("</div>\n")
}

func drawingGroupExtentEMU(d *resolve.Drawing, widthPx, heightPx int) (int64, int64) {
	if d != nil && d.Position.Width > 0 && d.Position.Height > 0 {
		return d.Position.Width, d.Position.Height
	}
	return int64(widthPx) * emusPerPx, int64(heightPx) * emusPerPx
}

func groupedDrawingStyle(child *resolve.Drawing, parentEMUWidth, parentEMUHeight int64, parentWidthPx, parentHeightPx int) (string, int, int) {
	if child == nil {
		return "", 0, 0
	}
	x := scaleGroupDrawingPx(child.Position.X, parentEMUWidth, parentWidthPx)
	y := scaleGroupDrawingPx(child.Position.Y, parentEMUHeight, parentHeightPx)
	width := scaleGroupDrawingPx(child.Position.Width, parentEMUWidth, parentWidthPx)
	height := scaleGroupDrawingPx(child.Position.Height, parentEMUHeight, parentHeightPx)
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	return fmt.Sprintf("left:%dpx;top:%dpx;width:%dpx;height:%dpx", x, y, width, height), width, height
}

func scaleGroupDrawingPx(value, parentEMU int64, parentPx int) int {
	if parentEMU <= 0 || parentPx <= 0 {
		return int(value / emusPerPx)
	}
	return int(float64(value) * float64(parentPx) / float64(parentEMU))
}

func renderDrawingConnector(b *strings.Builder, d *resolve.Drawing, posStyle string, width, height float64) {
	sp := d.Shape
	if sp == nil {
		return
	}
	style := []string{posStyle, "overflow:visible"}
	if transform := sheetShapeTransformStyle(sp); transform != "" {
		style = append(style, transform)
	}
	fmt.Fprintf(b, "<div class=\"drawing drawing--connector\" style=%q title=%q>", strings.Join(style, ";"), escapeHTML(d.Name))
	fmt.Fprintf(b, "<svg class=\"drawing__connector-vector\" viewBox=\"0 0 %.4f %.4f\" preserveAspectRatio=\"none\" aria-hidden=\"true\">", width, height)
	primitives := drawing.BuildConnectorPrimitives(drawing.ConnectorSpec{
		Type:            effectiveSheetConnectorType(sp),
		BoundsX:         0,
		BoundsY:         0,
		BoundsW:         width,
		BoundsH:         height,
		LineStyle:       sp.LineStyle,
		LineColor:       sp.LineColor,
		LineWidthPt:     sp.LineWidthPt,
		ArrowHead:       connectorArrowType(sp.LineStyle, true),
		ArrowHeadWidth:  connectorArrowSize(sp.LineStyle, true),
		ArrowHeadLength: connectorArrowLength(sp.LineStyle, true),
		ArrowTail:       connectorArrowType(sp.LineStyle, false),
		ArrowTailWidth:  connectorArrowSize(sp.LineStyle, false),
		ArrowTailLength: connectorArrowLength(sp.LineStyle, false),
	})
	for _, primitive := range primitives {
		writeSheetSVGPrimitive(b, primitive, "")
	}
	b.WriteString("</svg></div>\n")
}

func (r *htmlRenderer) renderDrawingImage(b *strings.Builder, d *resolve.Drawing, posStyle string) {
	if d.Image != nil {
		posStyle += drawingImageTransformStyle(d.Image)
	}
	if d.Image != nil {
		if effects := drawingcss.EffectList(d.Image.Effects); len(effects) > 0 {
			styles := append([]string{posStyle, "overflow:visible"}, appendSortedSheetStyleProps(nil, effects)...)
			posStyle = strings.Join(styles, ";")
		}
	}
	if d.Image == nil || len(d.Image.Data) == 0 {
		fmt.Fprintf(b, "<div class=\"drawing drawing--image\" style=%q title=%q></div>\n",
			posStyle, escapeHTML(d.Name))
		return
	}
	if !browserRenderableDrawingImage(d.Image.ContentType) {
		label := strings.TrimSpace(d.Image.ContentType)
		if label == "" {
			label = "image"
		}
		fmt.Fprintf(b, "<div class=\"drawing drawing--image drawing--image-placeholder\" style=%q title=%q>[%s]</div>\n",
			posStyle, escapeHTML(d.Name), escapeHTML(label))
		return
	}
	src := r.imageSource(d.Image)
	alt := strings.TrimSpace(d.Image.AltText)
	if alt == "" {
		alt = d.Name
	}
	if drawingImageHasCrop(d.Image) {
		fmt.Fprintf(b, "<div class=\"drawing drawing--image drawing--image-cropped\" style=%q title=%q><img style=%q alt=%q src=%q></div>\n",
			posStyle+";overflow:hidden", escapeHTML(d.Name), drawingImageCropStyle(d.Image), escapeHTML(alt), src)
		return
	}
	fmt.Fprintf(b, "<img class=\"drawing drawing--image\" style=%q title=%q alt=%q src=%q>\n",
		posStyle, escapeHTML(d.Name), escapeHTML(alt), src)
}

func drawingImageTransformStyle(img *resolve.DrawingImage) string {
	if img == nil || (img.RotationDeg == 0 && !img.FlipH && !img.FlipV) {
		return ""
	}
	parts := make([]string, 0, 3)
	if img.RotationDeg != 0 {
		parts = append(parts, fmt.Sprintf("rotate(%.4fdeg)", img.RotationDeg))
	}
	if img.FlipH {
		parts = append(parts, "scaleX(-1)")
	}
	if img.FlipV {
		parts = append(parts, "scaleY(-1)")
	}
	return ";transform-origin:center center;transform:" + strings.Join(parts, " ")
}

func drawingImageHasCrop(img *resolve.DrawingImage) bool {
	return img != nil && (img.CropLeft != 0 || img.CropTop != 0 || img.CropRight != 0 || img.CropBottom != 0)
}

func drawingImageCropStyle(img *resolve.DrawingImage) string {
	if img == nil {
		return "width:100%;height:100%"
	}
	l := math.Max(0, math.Min(99.999, float64(img.CropLeft)/1000))
	t := math.Max(0, math.Min(99.999, float64(img.CropTop)/1000))
	r := math.Max(0, math.Min(99.999, float64(img.CropRight)/1000))
	bt := math.Max(0, math.Min(99.999, float64(img.CropBottom)/1000))
	visibleW := math.Max(0.001, 100-l-r)
	visibleH := math.Max(0.001, 100-t-bt)
	width := 10000 / visibleW
	height := 10000 / visibleH
	left := -l * 100 / visibleW
	top := -t * 100 / visibleH
	return fmt.Sprintf("position:absolute;max-width:none;max-height:none;width:%.4f%%;height:%.4f%%;left:%.4f%%;top:%.4f%%", width, height, left, top)
}

func browserRenderableDrawingImage(contentType string) bool {
	baseType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch baseType {
	case "image/png", "image/jpeg", "image/jpg", "image/gif", "image/webp", "image/svg+xml", "image/bmp", "image/avif":
		return true
	default:
		return false
	}
}

func renderDrawingShape(b *strings.Builder, d *resolve.Drawing, posStyle string, width, height float64) {
	sp := d.Shape
	if sp == nil {
		return
	}
	effectProps := sheetShapeEffectStyleProps(sp)
	if shouldRenderVectorSheetShape(sp) {
		primitives, err := drawing.BuildShapePrimitives(drawing.ShapeGeometrySpec{
			Preset:      &drawing.PresetGeometry{Name: sp.Geometry, Adjustments: sp.Adjustments},
			Width:       width,
			Height:      height,
			Fill:        sp.Fill,
			FillColor:   sp.FillColor,
			StrokeColor: sp.LineColor,
			StrokeWidth: sp.LineWidthPt,
		})
		if err == nil && len(primitives) > 0 {
			fillDefID := ""
			fillDef := ""
			if drawingcss.NeedsSVGDefinition(sp.Fill) {
				fillDefID = "shape-fill"
				fillDef = drawingcss.SVGDefinition(sp.Fill, fillDefID)
			}
			lightOverlays := drawingcss.ThreeDLightOverlays(extractSheetShapeThreeD(sp.Effects))
			overlayRenders := drawingcss.BuildSVGOverlays(lightOverlays, "shape-fill-overlay")
			wrapperStyles := []string{posStyle}
			if transform := sheetShapeTransformStyle(sp); transform != "" {
				wrapperStyles = append(wrapperStyles, transform)
			}
			svgStyles := []string(nil)
			if len(effectProps) > 0 {
				wrapperStyles = append(wrapperStyles, "overflow:visible")
				svgStyles = appendSortedSheetStyleProps(nil, effectProps)
			}
			fmt.Fprintf(b, "<div class=\"drawing drawing--shape drawing--shape-vector\" style=%q title=%q>", strings.Join(wrapperStyles, ";"), escapeHTML(d.Name))
			if len(svgStyles) > 0 {
				fmt.Fprintf(b, "<svg class=\"drawing__shape-vector\" viewBox=\"0 0 %.4f %.4f\" preserveAspectRatio=\"none\" aria-hidden=\"true\" style=%q>", width, height, strings.Join(svgStyles, ";"))
			} else {
				fmt.Fprintf(b, "<svg class=\"drawing__shape-vector\" viewBox=\"0 0 %.4f %.4f\" preserveAspectRatio=\"none\" aria-hidden=\"true\">", width, height)
			}
			if fillDef != "" || len(overlayRenders) > 0 {
				b.WriteString("<defs>")
				if fillDef != "" {
					b.WriteString(fillDef)
				}
				for _, overlay := range overlayRenders {
					if overlay.Def != "" {
						b.WriteString(overlay.Def)
					}
				}
				b.WriteString("</defs>")
			}
			for _, primitive := range primitives {
				writeSheetSVGPrimitive(b, primitive, fillDefID)
			}
			for _, overlay := range overlayRenders {
				if overlay.Fill == nil {
					continue
				}
				if blendStyle := drawingcss.SVGOverlayBlendStyle(overlay.BlendMode); blendStyle != "" {
					fmt.Fprintf(b, "<g style=%q>", blendStyle)
				} else {
					b.WriteString("<g>")
				}
				for _, primitive := range primitives {
					writeSheetSVGPrimitive(b, overlaySheetPrimitiveFill(primitive, overlay.Fill), overlay.DefID)
				}
				b.WriteString("</g>")
			}
			b.WriteString("</svg>")
			renderDrawingShapeText(b, sp)
			b.WriteString("</div>\n")
			return
		}
	}

	// Build shape background styles.
	var bgStyles []string
	if sp.FillColor != "" {
		bgStyles = append(bgStyles, "background:#"+sp.FillColor)
	}
	if sp.LineColor != "" && sp.LineWidthPt > 0 {
		bgStyles = append(bgStyles, fmt.Sprintf("border:%.2fpt solid #%s", sp.LineWidthPt, sp.LineColor))
	}
	switch sp.Geometry {
	case "ellipse":
		bgStyles = append(bgStyles, "border-radius:50%")
	case "roundRect":
		bgStyles = append(bgStyles, "border-radius:10%")
	}

	fullStyles := []string{posStyle}
	if transform := sheetShapeTransformStyle(sp); transform != "" {
		fullStyles = append(fullStyles, transform)
	}
	if len(effectProps) > 0 {
		fullStyles = append(fullStyles, "overflow:visible")
		bgStyles = appendSortedSheetStyleProps(bgStyles, effectProps)
	}

	fmt.Fprintf(b, "<div class=\"drawing drawing--shape\" style=%q title=%q>", strings.Join(fullStyles, ";"), escapeHTML(d.Name))
	if len(bgStyles) > 0 {
		// Effects such as soft edges blur the shape body, never its text.
		fmt.Fprintf(b, "<span class=\"drawing__shape-bg\" style=%q></span>", strings.Join(bgStyles, ";"))
	}
	renderDrawingShapeText(b, sp)
	b.WriteString("</div>\n")
}

func renderDrawingShapeText(b *strings.Builder, shape *resolve.DrawingShape) {
	if b == nil || shape == nil {
		return
	}
	if shape.Text == "" && len(shape.RichTextRuns) == 0 {
		return
	}
	b.WriteString("<span class=\"drawing__text\">")
	defer b.WriteString("</span>")
	if len(shape.RichTextRuns) == 0 {
		b.WriteString(escapeHTML(shape.Text))
		return
	}
	for _, run := range shape.RichTextRuns {
		if run == nil || run.Text == "" {
			continue
		}
		parts := strings.Split(run.Text, "\n")
		for i, part := range parts {
			if i > 0 {
				b.WriteString("<br>")
			}
			if part == "" {
				continue
			}
			copyRun := *run
			copyRun.Text = part
			renderRichTextRuns(b, []*resolve.RichTextRun{&copyRun}, nil)
		}
	}
}

func sheetShapeTransformStyle(shape *resolve.DrawingShape) string {
	if shape == nil || (shape.RotationDeg == 0 && !shape.FlipH && !shape.FlipV) {
		return ""
	}
	parts := make([]string, 0, 3)
	if shape.RotationDeg != 0 {
		parts = append(parts, fmt.Sprintf("rotate(%.4fdeg)", shape.RotationDeg))
	}
	if shape.FlipH {
		parts = append(parts, "scaleX(-1)")
	}
	if shape.FlipV {
		parts = append(parts, "scaleY(-1)")
	}
	return "transform:" + strings.Join(parts, " ") + ";transform-origin:center center"
}

func sheetShapeEffectStyleProps(shape *resolve.DrawingShape) map[string]string {
	if shape == nil || len(shape.Effects.Effects) == 0 {
		return nil
	}
	return drawingcss.EffectList(shape.Effects)
}

func appendSortedSheetStyleProps(styles []string, props map[string]string) []string {
	if len(props) == 0 {
		return styles
	}
	keys := make([]string, 0, len(props))
	for key, value := range props {
		if strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		styles = append(styles, fmt.Sprintf("%s:%s", key, props[key]))
	}
	return styles
}

func shouldRenderVectorSheetShape(shape *resolve.DrawingShape) bool {
	if shape == nil {
		return false
	}
	if extractSheetShapeThreeD(shape.Effects) != nil {
		return true
	}
	if drawingcss.NeedsSVGDefinition(shape.Fill) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(shape.Geometry)) {
	case "", "rect", "roundRect", "ellipse":
		return false
	default:
		return true
	}
}

func writeSheetSVGPrimitive(b *strings.Builder, primitive drawing.VectorPrimitive, fillDefID string) {
	if b == nil {
		return
	}
	switch primitive.Kind {
	case drawing.PrimitivePolyline:
		if len(primitive.Points) == 0 {
			return
		}
		if primitive.Closed {
			fmt.Fprintf(b, "<polygon points=%q fill=%q", sheetDrawingPointsString(primitive.Points), sheetSVGFillValue(primitive.Style, fillDefID))
			if primitive.Style.StrokeColor != "" && primitive.Style.StrokeWidth > 0 {
				writeSheetSVGStrokeAttrs(b, primitive.Style.Stroke, "#"+primitive.Style.StrokeColor, primitive.Style.StrokeWidth)
			}
			b.WriteString("/>")
			return
		}
		if primitive.Style.StrokeColor == "" {
			return
		}
		fmt.Fprintf(b, "<polyline points=%q fill=\"none\"", sheetDrawingPointsString(primitive.Points))
		writeSheetSVGStrokeAttrs(b, primitive.Style.Stroke, "#"+primitive.Style.StrokeColor, primitive.Style.StrokeWidth)
		b.WriteString("/>")
	case drawing.PrimitiveEllipse:
		if primitive.Ellipse == nil {
			return
		}
		fmt.Fprintf(b, "<ellipse cx=\"%.4f\" cy=\"%.4f\" rx=\"%.4f\" ry=\"%.4f\" fill=%q",
			primitive.Ellipse.Center.X, primitive.Ellipse.Center.Y, primitive.Ellipse.RX, primitive.Ellipse.RY, sheetSVGFillValue(primitive.Style, fillDefID))
		if primitive.Style.StrokeColor != "" && primitive.Style.StrokeWidth > 0 {
			writeSheetSVGStrokeAttrs(b, primitive.Style.Stroke, "#"+primitive.Style.StrokeColor, primitive.Style.StrokeWidth)
		}
		b.WriteString("/>")
	case drawing.PrimitivePath:
		if primitive.Path == nil {
			return
		}
		fmt.Fprintf(b, "<path d=%q fill=%q", sheetSVGPathData(*primitive.Path), sheetSVGFillValue(primitive.Style, fillDefID))
		if primitive.Style.StrokeColor != "" && primitive.Style.StrokeWidth > 0 {
			writeSheetSVGStrokeAttrs(b, primitive.Style.Stroke, "#"+primitive.Style.StrokeColor, primitive.Style.StrokeWidth)
		}
		if primitive.Path.FillRule != "" {
			fmt.Fprintf(b, " fill-rule=%q", primitive.Path.FillRule)
		}
		b.WriteString("/>")
	}
}

func extractSheetShapeThreeD(effects drawing.EffectList) *drawing.ThreeD {
	for _, effect := range effects.Effects {
		if threeD, ok := effect.(drawing.ThreeD); ok {
			return &threeD
		}
		if threeD, ok := effect.(*drawing.ThreeD); ok {
			return threeD
		}
	}
	return nil
}

func overlaySheetPrimitiveFill(primitive drawing.VectorPrimitive, fill drawing.Fill) drawing.VectorPrimitive {
	primitive.Style.Fill = fill
	primitive.Style.FillColor = ""
	primitive.Style.Stroke = nil
	primitive.Style.StrokeColor = ""
	primitive.Style.StrokeWidth = 0
	return primitive
}

func sheetSVGFillValue(style drawing.VectorStyle, fillDefID string) string {
	if style.Fill != nil {
		if fillDefID != "" && drawingcss.NeedsSVGDefinition(style.Fill) {
			return "url(#" + fillDefID + ")"
		}
		if fill, ok := drawingcss.Fill(style.Fill)["fill"]; ok && fill != "" {
			return fill
		}
	}
	if style.FillColor != "" {
		return "#" + style.FillColor
	}
	return "none"
}

func writeSheetSVGStrokeAttrs(b *strings.Builder, style *drawing.LineStyle, fallbackColor string, fallbackWidth float64) {
	if b == nil {
		return
	}
	color := fallbackColor
	width := fallbackWidth
	if style != nil {
		if style.Color != "" {
			color = "#" + style.Color
		}
		if style.Width > 0 {
			width = style.Width
		}
	}
	if color == "" || color == "none" || width <= 0 {
		return
	}
	fmt.Fprintf(b, " stroke=%q stroke-width=\"%.2f\"", color, width)
	if style == nil {
		return
	}
	if dashArray := style.DashArray(); dashArray != "" && dashArray != "none" {
		fmt.Fprintf(b, " stroke-dasharray=%q", dashArray)
	}
	switch style.Cap {
	case drawing.LineCapRound:
		b.WriteString(` stroke-linecap="round"`)
	case drawing.LineCapSquare:
		b.WriteString(` stroke-linecap="square"`)
	}
	switch style.Join {
	case drawing.LineJoinRound:
		b.WriteString(` stroke-linejoin="round"`)
	case drawing.LineJoinBevel:
		b.WriteString(` stroke-linejoin="bevel"`)
	}
}

func effectiveSheetConnectorType(shape *resolve.DrawingShape) string {
	if shape == nil {
		return "straightConnector1"
	}
	if shape.ConnectorType != "" {
		return shape.ConnectorType
	}
	if strings.Contains(strings.ToLower(shape.Geometry), "connector") {
		return shape.Geometry
	}
	return "straightConnector1"
}

func connectorArrowType(style *drawing.LineStyle, head bool) string {
	if style == nil {
		return ""
	}
	var arrow *drawing.Arrowhead
	if head {
		arrow = style.HeadEnd
	} else {
		arrow = style.TailEnd
	}
	if arrow == nil || arrow.Type == drawing.ArrowheadNone {
		return ""
	}
	return string(arrow.Type)
}

func connectorArrowSize(style *drawing.LineStyle, head bool) string {
	if style == nil {
		return ""
	}
	var arrow *drawing.Arrowhead
	if head {
		arrow = style.HeadEnd
	} else {
		arrow = style.TailEnd
	}
	if arrow == nil {
		return ""
	}
	return string(arrow.Size)
}

func connectorArrowLength(style *drawing.LineStyle, head bool) string {
	if style == nil {
		return ""
	}
	var arrow *drawing.Arrowhead
	if head {
		arrow = style.HeadEnd
	} else {
		arrow = style.TailEnd
	}
	if arrow == nil {
		return ""
	}
	return string(arrow.Length)
}

func sheetSVGPathData(path drawing.Path) string {
	var parts []string
	for _, cmd := range path.Commands {
		switch cmd.Type {
		case "M", "L":
			if len(cmd.Points) == 0 {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s %.4f %.4f", cmd.Type, cmd.Points[0].X, cmd.Points[0].Y))
		case "C":
			if len(cmd.Points) < 3 {
				continue
			}
			parts = append(parts, fmt.Sprintf("C %.4f %.4f %.4f %.4f %.4f %.4f",
				cmd.Points[0].X, cmd.Points[0].Y,
				cmd.Points[1].X, cmd.Points[1].Y,
				cmd.Points[2].X, cmd.Points[2].Y,
			))
		case "Q":
			if len(cmd.Points) < 2 {
				continue
			}
			parts = append(parts, fmt.Sprintf("Q %.4f %.4f %.4f %.4f",
				cmd.Points[0].X, cmd.Points[0].Y,
				cmd.Points[1].X, cmd.Points[1].Y,
			))
		case "A":
			if len(cmd.Points) == 0 || len(cmd.Args) < 5 {
				continue
			}
			parts = append(parts, fmt.Sprintf("A %.4f %.4f %.4f %.0f %.0f %.4f %.4f",
				cmd.Args[0], cmd.Args[1], cmd.Args[2], cmd.Args[3], cmd.Args[4], cmd.Points[0].X, cmd.Points[0].Y,
			))
		case "Z":
			parts = append(parts, "Z")
		}
	}
	return strings.Join(parts, " ")
}

func sheetDrawingPointsString(points []drawing.Point) string {
	parts := make([]string, 0, len(points))
	for _, pt := range points {
		parts = append(parts, fmt.Sprintf("%.4f,%.4f", pt.X, pt.Y))
	}
	return strings.Join(parts, " ")
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&#34;")
	return s
}

func escapeHTMLAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// removeStyleProp removes any CSS declaration whose property starts with prefix from styles.
func removeStyleProp(styles []string, prop string) []string {
	out := styles[:0]
	for _, s := range styles {
		if !strings.HasPrefix(s, prop) {
			out = append(out, s)
		}
	}
	return out
}

// colLetterFromIndex converts a 1-based column index to an Excel column letter (A, B, …, Z, AA, …).
func colLetterFromIndex(col int) string {
	result := ""
	for col > 0 {
		col--
		result = string(rune('A'+col%26)) + result
		col /= 26
	}
	return result
}

// cfVisualData holds visual conditional formatting for a cell.
type cfVisualData struct {
	DataBar              *resolve.CFDataBar
	DataBarThresholds    []float64
	IconSet              *resolve.CFIconSet
	IconSetThresholds    []float64
	ColorScale           *resolve.CFColorScale
	ColorScaleThresholds []float64
	HasNumericValue      bool
	NumericValue         float64
}

// evaluateCFRules evaluates conditional formatting rules for each cell and returns
// a map of cell reference → winning DxfStyle (only if the rule matches).

func evaluateCFRules(sheet *resolve.Sheet, arrayFormulaCells map[string]*resolve.Cell, eng *formula.Engine) map[string]resolve.DxfStyle {
	var rules []*resolve.CFRule
	if sheet != nil {
		rules = sheet.CFRules
	}
	if len(rules) == 0 {
		return nil
	}

	cellValues := buildCFCellValueMap(sheet, arrayFormulaCells, eng)

	// Map ref → winning rule (lowest priority number wins, but only if rule matches)
	type winner struct {
		priority int
		style    resolve.DxfStyle
	}
	m := make(map[string]winner)

	for _, rule := range rules {
		// Skip visual formatting rules (handled by buildCFVisualMap)
		if rule.DataBar != nil || rule.IconSet != nil || rule.ColorScale != nil {
			continue
		}

		cells := expandSqRef(rule.SqRef)
		for _, ref := range cells {
			// Check if the rule matches this cell
			if ruleMatchesCell(rule, ref, cellValues, eng) {
				if prev, ok := m[ref]; !ok || rule.Priority < prev.priority {
					m[ref] = winner{priority: rule.Priority, style: rule.DxfStyle}
				}
			}
		}
	}

	out := make(map[string]resolve.DxfStyle, len(m))
	for ref, w := range m {
		out[ref] = w.style
	}
	return out
}

// cellValueData holds cell value information for conditional formatting evaluation
type cellValueData struct {
	kind     resolve.CellKind
	numValue float64
	strValue string
}

// ruleMatchesCell determines if a conditional formatting rule matches a given cell
func ruleMatchesCell(rule *resolve.CFRule, ref string, cellValues map[string]cellValueData, eng *formula.Engine) bool {
	cell, ok := cellValues[ref]
	if !ok {
		return false
	}

	switch rule.Type {
	case "cellIs":
		return evaluateCellIsRule(rule, cell)
	case "expression":
		return evaluateExpressionRule(rule, ref, cellValues, eng)
	case "containsText", "notContainsText", "beginsWith", "endsWith":
		return evaluateTextRule(rule, cell)
	case "top10", "bottom10":
		return evaluateTopBottomRule(rule, ref, cellValues, expandSqRef(rule.SqRef))
	case "aboveAverage", "belowAverage":
		return evaluateAverageRule(rule, ref, cellValues, expandSqRef(rule.SqRef))
	case "timePeriod":
		return evaluateTimePeriodRule(rule, cell)
	default:
		// For unsupported rule types, default to matching (old behavior)
		return true
	}
}

// evaluateCellIsRule evaluates a "cellIs" conditional formatting rule
func evaluateCellIsRule(rule *resolve.CFRule, cell cellValueData) bool {
	if cell.kind != resolve.CellNumber && cell.kind != resolve.CellBool {
		return false
	}

	cellValue := cell.numValue

	// Parse formula values (simplified - assumes constant numeric values)
	var formulaValues []float64
	for _, f := range rule.Formula {
		if f == "" {
			continue
		}
		// Try to parse as number
		if val, err := strconv.ParseFloat(f, 64); err == nil {
			formulaValues = append(formulaValues, val)
		}
	}

	// If no formula values, default to comparing against 0
	// This handles test cases where Formula might be empty
	if len(formulaValues) == 0 {
		formulaValues = []float64{0}
	}

	switch rule.Operator {
	case "between":
		if len(formulaValues) >= 2 {
			return cellValue >= formulaValues[0] && cellValue <= formulaValues[1]
		}
	case "notBetween":
		if len(formulaValues) >= 2 {
			return cellValue < formulaValues[0] || cellValue > formulaValues[1]
		}
	case "equal":
		return cellValue == formulaValues[0]
	case "notEqual":
		return cellValue != formulaValues[0]
	case "greaterThan":
		return cellValue > formulaValues[0]
	case "lessThan":
		return cellValue < formulaValues[0]
	case "greaterThanOrEqual":
		return cellValue >= formulaValues[0]
	case "lessThanOrEqual":
		return cellValue <= formulaValues[0]
	}

	return false
}

func conditionalFormattingAnchorRef(sqref string) string {
	for _, part := range strings.Fields(sqref) {
		if anchor := validationAnchorRef(part); anchor != "" {
			return anchor
		}
	}
	return ""
}

// evaluateExpressionRule evaluates an "expression" conditional formatting rule
func evaluateExpressionRule(rule *resolve.CFRule, ref string, cellValues map[string]cellValueData, eng *formula.Engine) bool {
	if len(rule.Formula) == 0 {
		return false
	}

	// Conditional-format expression formulas are authored relative to the
	// top-left cell of the applies-to range. Excel translates relative row/column
	// references for every target cell while preserving $-absolute components.
	// Evaluating the authored expression verbatim makes a rule such as
	// =$A4=TRUE over A4:C51 read A4 for every row, causing one TRUE row to style
	// the whole range.
	formulaStr := rule.Formula[0]
	if formulaStr == "" {
		return false
	}
	if anchorRef := conditionalFormattingAnchorRef(rule.SqRef); anchorRef != "" {
		formulaStr = translateValidationFormula(formulaStr, anchorRef, ref)
	}

	result, err := formula.EvaluateInContext(formulaStr, eng.GetCell, eng.GetRange)
	if err != nil {
		return false
	}

	// Check if result is TRUE
	if result.IsBool() && result.Bool {
		return true
	}

	// Check if result is a number (non-zero is true)
	if result.IsNumber() && result.Num != 0 {
		return true
	}

	return false
}

// evaluateTextRule evaluates text-based conditional formatting rules
func evaluateTextRule(rule *resolve.CFRule, cell cellValueData) bool {
	if cell.kind != resolve.CellString && cell.kind != resolve.CellFormula {
		return false
	}

	cellText := cell.strValue
	targetText := rule.Text

	switch rule.Type {
	case "containsText":
		return strings.Contains(cellText, targetText)
	case "notContainsText":
		return !strings.Contains(cellText, targetText)
	case "beginsWith":
		return strings.HasPrefix(cellText, targetText)
	case "endsWith":
		return strings.HasSuffix(cellText, targetText)
	}

	return false
}

// evaluateTopBottomRule evaluates "top10" and "bottom10" conditional formatting rules
func evaluateTopBottomRule(rule *resolve.CFRule, ref string, cellValues map[string]cellValueData, cellRefs []string) bool {
	cell, ok := cellValues[ref]
	if !ok {
		return false
	}

	if cell.kind != resolve.CellNumber {
		return false
	}

	cellValue := cell.numValue

	// Collect all numeric values in the range
	var values []float64
	for _, cellRef := range cellRefs {
		if c, ok := cellValues[cellRef]; ok && c.kind == resolve.CellNumber {
			values = append(values, c.numValue)
		}
	}

	if len(values) == 0 {
		return false
	}

	limit := rule.Rank
	if limit <= 0 {
		limit = 10
	}
	if rule.Percent {
		limit = int(math.Ceil(float64(len(values)) * float64(limit) / 100.0))
	}
	if limit < 1 {
		limit = 1
	}

	higherCount := 0
	lowerCount := 0
	for _, value := range values {
		if value > cellValue {
			higherCount++
		}
		if value < cellValue {
			lowerCount++
		}
	}

	if rule.Bottom || rule.Type == "bottom10" {
		return lowerCount < limit
	}

	return higherCount < limit
}

// evaluateAverageRule evaluates "aboveAverage" and "belowAverage" conditional formatting rules
func evaluateAverageRule(rule *resolve.CFRule, ref string, cellValues map[string]cellValueData, cellRefs []string) bool {
	cell, ok := cellValues[ref]
	if !ok {
		return false
	}

	if cell.kind != resolve.CellNumber {
		return false
	}

	cellValue := cell.numValue

	// Collect all numeric values in the range
	var values []float64
	for _, cellRef := range cellRefs {
		if c, ok := cellValues[cellRef]; ok && c.kind == resolve.CellNumber {
			values = append(values, c.numValue)
		}
	}

	if len(values) == 0 {
		return false
	}

	// Calculate average
	var sum float64
	for _, v := range values {
		sum += v
	}
	average := sum / float64(len(values))

	// Check for "aboveAverage" or "belowAverage"
	switch rule.Type {
	case "aboveAverage":
		return cellValue > average
	case "belowAverage":
		return cellValue < average
	}

	return false
}

// evaluateTimePeriodRule evaluates date/time period conditional-formatting rules.
// The renderer uses UTC and ISO-style Monday-start weeks so matching stays
// locale-independent and deterministic across environments.
func evaluateTimePeriodRule(rule *resolve.CFRule, cell cellValueData) bool {
	if cell.kind != resolve.CellNumber {
		return false
	}

	cellTime := excelSerialToTime(cell.numValue)
	if cellTime.IsZero() {
		return false
	}

	cellDate := truncateCFDate(cellTime)
	today := truncateCFDate(time.Now().UTC())
	period := strings.ToLower(strings.TrimSpace(rule.TimePeriod))

	switch period {
	case "today":
		return cellDate.Equal(today)
	case "yesterday":
		return cellDate.Equal(today.AddDate(0, 0, -1))
	case "tomorrow":
		return cellDate.Equal(today.AddDate(0, 0, 1))
	case "last7days":
		start := today.AddDate(0, 0, -6)
		return !cellDate.Before(start) && !cellDate.After(today)
	case "thisweek":
		start, end := cfWeekBounds(today)
		return !cellDate.Before(start) && !cellDate.After(end)
	case "lastweek":
		start, _ := cfWeekBounds(today)
		start = start.AddDate(0, 0, -7)
		end := start.AddDate(0, 0, 6)
		return !cellDate.Before(start) && !cellDate.After(end)
	case "nextweek":
		start, _ := cfWeekBounds(today)
		start = start.AddDate(0, 0, 7)
		end := start.AddDate(0, 0, 6)
		return !cellDate.Before(start) && !cellDate.After(end)
	case "thismonth":
		return cellDate.Year() == today.Year() && cellDate.Month() == today.Month()
	case "lastmonth":
		prev := today.AddDate(0, -1, 0)
		return cellDate.Year() == prev.Year() && cellDate.Month() == prev.Month()
	case "nextmonth":
		next := today.AddDate(0, 1, 0)
		return cellDate.Year() == next.Year() && cellDate.Month() == next.Month()
	default:
		return false
	}
}

func truncateCFDate(t time.Time) time.Time {
	return time.Date(t.UTC().Year(), t.UTC().Month(), t.UTC().Day(), 0, 0, 0, 0, time.UTC)
}

func cfWeekBounds(day time.Time) (time.Time, time.Time) {
	day = truncateCFDate(day)
	weekdayOffset := (int(day.Weekday()) + 6) % 7
	start := day.AddDate(0, 0, -weekdayOffset)
	return start, start.AddDate(0, 0, 6)
}

// buildCFVisualMap builds a per-cell map of visual conditional formatting.

func buildCFVisualMap(sheet *resolve.Sheet, arrayFormulaCells map[string]*resolve.Cell, eng *formula.Engine) map[string]cfVisualData {
	var rules []*resolve.CFRule
	if sheet != nil {
		rules = sheet.CFRules
	}
	if len(rules) == 0 {
		return nil
	}
	numericValues := buildCFNumericValueMap(sheet, arrayFormulaCells, eng)
	m := make(map[string]cfVisualData)
	for _, rule := range rules {
		if rule.DataBar == nil && rule.IconSet == nil && rule.ColorScale == nil {
			continue
		}
		cells := expandSqRef(rule.SqRef)
		rangeValues := collectCFRangeValues(cells, numericValues)
		anchorRef := ""
		if len(cells) > 0 {
			anchorRef = cells[0]
		}
		var dataBarThresholds []float64
		if rule.DataBar != nil {
			dataBarThresholds = resolveCFVOThresholds(rule.DataBar.Cfvo, rangeValues, anchorRef, eng)
		}
		var iconSetThresholds []float64
		if rule.IconSet != nil {
			iconSetThresholds = resolveCFVOThresholds(rule.IconSet.Cfvo, rangeValues, anchorRef, eng)
		}
		var colorScaleThresholds []float64
		if rule.ColorScale != nil {
			colorScaleThresholds = resolveCFVOThresholds(rule.ColorScale.Cfvo, rangeValues, anchorRef, eng)
		}
		for _, ref := range cells {
			// Visual CF is additive - different rules can contribute different visual types
			existing := m[ref]
			if value, ok := numericValues[ref]; ok {
				existing.HasNumericValue = true
				existing.NumericValue = value
			}
			if rule.DataBar != nil {
				existing.DataBar = rule.DataBar
				existing.DataBarThresholds = dataBarThresholds
			}
			if rule.IconSet != nil {
				existing.IconSet = rule.IconSet
				existing.IconSetThresholds = iconSetThresholds
			}
			if rule.ColorScale != nil {
				existing.ColorScale = rule.ColorScale
				existing.ColorScaleThresholds = colorScaleThresholds
			}
			m[ref] = existing
		}
	}
	return m
}

func buildCFNumericValueMap(sheet *resolve.Sheet, arrayFormulaCells map[string]*resolve.Cell, eng *formula.Engine) map[string]float64 {
	if sheet == nil && len(arrayFormulaCells) == 0 {
		return nil
	}
	values := make(map[string]float64)
	var rows []*resolve.Row
	if sheet != nil {
		rows = sheet.Rows
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil || cell.Ref == "" {
				continue
			}
			ref := strings.ToUpper(cell.Ref)
			switch cell.Kind {
			case resolve.CellNumber, resolve.CellDate:
				values[ref] = cell.NumValue
			case resolve.CellBool:
				if cell.NumValue != 0 {
					values[ref] = 1
				} else {
					values[ref] = 0
				}
			case resolve.CellFormula:
				if eng == nil {
					continue
				}
				val := eng.GetCell(ref)
				num, err := val.ToNumber()
				if err == nil {
					values[ref] = num
				}
			}
		}
	}
	for ref := range arrayFormulaCells {
		if eng == nil {
			continue
		}
		val := eng.GetCell(ref)
		num, err := val.ToNumber()
		if err == nil {
			values[ref] = num
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func buildCFCellValueMap(sheet *resolve.Sheet, arrayFormulaCells map[string]*resolve.Cell, eng *formula.Engine) map[string]cellValueData {
	if sheet == nil && len(arrayFormulaCells) == 0 {
		return nil
	}
	values := make(map[string]cellValueData)
	if sheet != nil {
		for _, row := range sheet.Rows {
			if row == nil {
				continue
			}
			for _, cell := range row.Cells {
				if cell == nil || cell.Ref == "" {
					continue
				}
				ref := strings.ToUpper(cell.Ref)
				values[ref] = cfCellValueData(cell, ref, eng)
			}
		}
	}
	for ref, cell := range arrayFormulaCells {
		if cell == nil || ref == "" {
			continue
		}
		values[ref] = cfCellValueData(cell, ref, eng)
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func cfCellValueData(cell *resolve.Cell, ref string, eng *formula.Engine) cellValueData {
	if cell == nil {
		return cellValueData{}
	}
	switch cell.Kind {
	case resolve.CellNumber, resolve.CellDate:
		return cellValueData{kind: resolve.CellNumber, numValue: cell.NumValue}
	case resolve.CellBool:
		return cellValueData{kind: resolve.CellBool, numValue: cell.NumValue}
	case resolve.CellString:
		return cellValueData{kind: resolve.CellString, strValue: cell.StrValue}
	case resolve.CellFormula:
		if eng != nil && ref != "" {
			return cfCellValueDataFromFormulaValue(eng.GetCell(ref))
		}
		return cellValueData{kind: resolve.CellFormula, strValue: cell.StrValue}
	default:
		return cellValueData{kind: cell.Kind, numValue: cell.NumValue, strValue: cell.StrValue}
	}
}

func cfCellValueDataFromFormulaValue(value formula.Value) cellValueData {
	switch {
	case value.IsBool():
		if value.Bool {
			return cellValueData{kind: resolve.CellBool, numValue: 1}
		}
		return cellValueData{kind: resolve.CellBool, numValue: 0}
	case value.IsNumber():
		return cellValueData{kind: resolve.CellNumber, numValue: value.Num}
	case value.IsText():
		return cellValueData{kind: resolve.CellString, strValue: value.Text}
	default:
		return cellValueData{kind: resolve.CellFormula, strValue: value.ToText()}
	}
}

func collectCFRangeValues(cells []string, numericValues map[string]float64) []float64 {
	if len(cells) == 0 || len(numericValues) == 0 {
		return nil
	}
	values := make([]float64, 0, len(cells))
	for _, ref := range cells {
		if value, ok := numericValues[ref]; ok {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil
	}
	sort.Float64s(values)
	return values
}

func resolveCFVOThresholds(cfvo []resolve.CFCfvo, rangeValues []float64, anchorRef string, eng *formula.Engine) []float64 {
	if len(cfvo) == 0 {
		return nil
	}
	thresholds := make([]float64, len(cfvo))
	for i, value := range cfvo {
		thresholds[i] = cfvoNumericValue(value, rangeValues, anchorRef, eng)
	}
	return thresholds
}

// renderDataBar generates an inline SVG data bar using cfvo thresholds.
func renderDataBar(bar *resolve.CFDataBar, thresholds []float64, cellValue float64) string {
	if bar == nil {
		return ""
	}
	// Determine min/max from cfvo thresholds, falling back to simplified 0-100 range.
	minVal, maxVal := 0.0, 100.0
	if len(thresholds) >= 2 {
		minVal = thresholds[0]
		maxVal = thresholds[len(thresholds)-1]
	}
	if maxVal <= minVal {
		maxVal = minVal + 100
	}

	// Calculate bar width as percentage between min and max.
	widthPercent := 0.0
	if cellValue > minVal {
		widthPercent = ((cellValue - minVal) / (maxVal - minVal)) * 100
	}
	if widthPercent > 100 {
		widthPercent = 100
	} else if widthPercent < 0 {
		widthPercent = 0
	}

	color := bar.Color
	if color == "" {
		color = "638EC6" // Default Excel blue
	}
	// Ensure 6-digit hex
	if len(color) == 8 {
		color = color[2:] // Strip ARGB alpha
	}

	// Create inline SVG data bar
	return fmt.Sprintf(`<svg class="cf-databar" width="60" height="12" style="vertical-align:middle;margin-left:4px"><rect x="0" y="0" width="%.0f%%" height="12" fill="#%s" rx="1"/></svg>`,
		widthPercent, color)
}

// renderIconSet returns an icon character for the cell value using cfvo thresholds.
func renderIconSet(iconSet *resolve.CFIconSet, thresholds []float64, cellValue float64) string {
	if iconSet == nil {
		return ""
	}
	// Map common icon set names to Unicode characters
	icons := getIconSetIcons(iconSet.Set)
	if len(icons) == 0 {
		return ""
	}

	// Determine which icon to show based on cfvo thresholds.
	iconIndex := 0
	if len(thresholds) >= 2 {
		bucket := 0
		for i := 1; i < len(thresholds) && i < len(icons); i++ {
			if cellValue >= thresholds[i] {
				bucket++
			}
		}
		iconIndex = len(icons) - 1 - bucket
	} else {
		// Fallback: simplified thresholds based on icon count.
		switch len(icons) {
		case 3:
			if cellValue >= 67 {
				iconIndex = 0
			} else if cellValue >= 33 {
				iconIndex = 1
			} else {
				iconIndex = 2
			}
		case 4:
			if cellValue >= 75 {
				iconIndex = 0
			} else if cellValue >= 50 {
				iconIndex = 1
			} else if cellValue >= 25 {
				iconIndex = 2
			} else {
				iconIndex = 3
			}
		case 5:
			if cellValue >= 80 {
				iconIndex = 0
			} else if cellValue >= 60 {
				iconIndex = 1
			} else if cellValue >= 40 {
				iconIndex = 2
			} else if cellValue >= 20 {
				iconIndex = 3
			} else {
				iconIndex = 4
			}
		default:
			iconIndex = 0
		}
	}

	if iconSet.Reverse {
		iconIndex = len(icons) - 1 - iconIndex
	}

	if iconIndex >= 0 && iconIndex < len(icons) {
		return fmt.Sprintf(`<span class="cf-icon" style="margin-right:4px">%s</span>`, icons[iconIndex])
	}
	return ""
}

// getIconSetIcons returns Unicode icons for a given icon set name.
func getIconSetIcons(setName string) []string {
	switch setName {
	case "3TrafficLights1", "3TrafficLights2":
		return []string{"\U0001F7E2", "\U0001F7E1", "\U0001F534"} // 🟢🟡🔴
	case "3Arrows":
		return []string{"\u2191", "\u2192", "\u2193"} // ↑→↓
	case "3ArrowsGray":
		return []string{"\u21E7", "\u21E8", "\u21E9"} // ⇧⇨⇩
	case "3Flags":
		return []string{"\U0001F6A9", "\U0001F3F3", "\U0001F3F4"} // 🚩🏳🏴
	case "3Signs":
		return []string{"+", "\u00B1", "-"} // +±-
	case "3Symbols":
		return []string{"\u2714", "\u26A0", "\u2718"} // ✔⚠✘
	case "3Symbols2":
		return []string{"\u2605", "\u2606", "\u2717"} // ★☆✗
	case "4Arrows":
		return []string{"\u2191", "\u2197", "\u2198", "\u2193"} // ↑↗↘↓
	case "4ArrowsGray":
		return []string{"\u21E7", "\u21E7\u21E9", "\u21E9\u21E7", "\u21E9"} // Simplified
	case "4Rating":
		return []string{"\u2605\u2605\u2605\u2605", "\u2605\u2605\u2605", "\u2605\u2605", "\u2605"} // Stars
	case "5Arrows":
		return []string{"\u2191", "\u2197", "\u2192", "\u2198", "\u2193"} // ↑↗→↘↓
	case "5ArrowsGray":
		return []string{"\u21E7", "\u2197", "\u21E8", "\u2198", "\u21E9"} // ↑↗→↘↓
	case "5Rating":
		return []string{"\u2605\u2605\u2605\u2605\u2605", "\u2605\u2605\u2605\u2605", "\u2605\u2605\u2605", "\u2605\u2605", "\u2605"} // Stars
	case "5Quarters":
		return []string{"\u25cf", "\u25d5", "\u25d1", "\u25d4", "\u25cb"} // \u25cf\u25d5\u25d1\u25d4\u25cb
	case "4TrafficLights":
		return []string{"\U0001F7E2", "\U0001F7E1", "\U0001F7E0", "\U0001F534"} // \ud83d\udfe2\ud83d\udfe1\ud83d\udfe0\ud83d\udd34
	case "4RedToBlack":
		return []string{"\U0001F534", "\U0001F7E5", "\u26ab", "\u2b1b"} // \ud83d\udd34\ud83d\udfe5\u26ab\u2b1b
	default:
		// Default to traffic lights
		return []string{"\U0001F7E2", "\U0001F7E1", "\U0001F534"}
	}
}

// renderColorScale returns a CSS background color for the cell value using cfvo thresholds.
func renderColorScale(colorScale *resolve.CFColorScale, thresholds []float64, cellValue float64) string {
	if colorScale == nil || len(colorScale.Colors) < 2 {
		return ""
	}
	colors := colorScale.Colors

	// Determine threshold values from cfvo, falling back to simplified range.
	if len(thresholds) < len(colors) {
		thresholds = make([]float64, len(colors))
		// Fallback: evenly distribute across 0-100 range.
		for i := range colors {
			thresholds[i] = float64(i) * 100.0 / float64(len(colors)-1)
		}
	}

	if len(colors) == 2 {
		// Two-color scale: interpolate between min and max.
		minVal, maxVal := thresholds[0], thresholds[1]
		if maxVal <= minVal {
			maxVal = minVal + 100
		}
		t := (cellValue - minVal) / (maxVal - minVal)
		return interpolateColor(colors[0], colors[1], t)
	}

	// Multi-color scale: find the segment the cell value falls in.
	for i := 0; i < len(colors)-1; i++ {
		low, high := thresholds[i], thresholds[i+1]
		if high <= low {
			high = low + 100
		}
		if cellValue >= low && cellValue <= high {
			t := (cellValue - low) / (high - low)
			return interpolateColor(colors[i], colors[i+1], t)
		}
	}
	// Clamp to first or last color.
	if cellValue < thresholds[0] {
		return "#" + colors[0]
	}
	return "#" + colors[len(colors)-1]
}

// cfvoNumericValue resolves a CFCfvo threshold using the rule's numeric range values.
func cfvoNumericValue(cfvo resolve.CFCfvo, rangeValues []float64, anchorRef string, eng *formula.Engine) float64 {
	switch cfvo.Type {
	case "min":
		if len(rangeValues) > 0 {
			return rangeValues[0]
		}
		return 0
	case "max":
		if len(rangeValues) > 0 {
			return rangeValues[len(rangeValues)-1]
		}
		return 100
	case "num":
		if v, err := strconv.ParseFloat(cfvo.Val, 64); err == nil {
			return v
		}
	case "percent":
		if v, err := strconv.ParseFloat(cfvo.Val, 64); err == nil {
			if len(rangeValues) == 0 {
				return v
			}
			if v < 0 {
				v = 0
			} else if v > 100 {
				v = 100
			}
			minVal := rangeValues[0]
			maxVal := rangeValues[len(rangeValues)-1]
			return minVal + ((maxVal-minVal)*v)/100.0
		}
	case "percentile":
		if v, err := strconv.ParseFloat(cfvo.Val, 64); err == nil {
			if len(rangeValues) == 0 {
				return v
			}
			return percentileValue(rangeValues, v)
		}
	case "formula":
		if v, ok := evaluateCFVOFormula(cfvo.Val, anchorRef, eng); ok {
			return v
		}
	}
	if len(rangeValues) > 0 {
		return rangeValues[0]
	}
	return 0
}

func evaluateCFVOFormula(expr, anchorRef string, eng *formula.Engine) (float64, bool) {
	expr = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(expr), "="))
	if expr == "" || eng == nil {
		return 0, false
	}
	translated := translateValidationFormula(expr, anchorRef, anchorRef)
	val, err := formula.EvaluateInContext(translated, eng.GetCell, eng.GetRange)
	if err != nil || val.IsError() {
		return 0, false
	}
	num, err := val.ToNumber()
	if err != nil {
		return 0, false
	}
	return num, true
}

func percentileValue(sortedValues []float64, percentile float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	if percentile <= 0 {
		return sortedValues[0]
	}
	if percentile >= 100 {
		return sortedValues[len(sortedValues)-1]
	}
	rank := (percentile / 100.0) * float64(len(sortedValues)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper {
		return sortedValues[lower]
	}
	weight := rank - float64(lower)
	return sortedValues[lower] + (sortedValues[upper]-sortedValues[lower])*weight
}

// interpolateColor interpolates between two hex colors.
func interpolateColor(c1, c2 string, t float64) string {
	// Clamp t to [0,1]
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	// Parse hex colors
	r1, g1, b1 := parseHexColor(c1)
	r2, g2, b2 := parseHexColor(c2)

	// Interpolate
	r := int(float64(r1) + t*float64(r2-r1))
	g := int(float64(g1) + t*float64(g2-g1))
	b := int(float64(b1) + t*float64(b2-b1))

	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// parseHexColor parses a 6-character hex color.
func parseHexColor(hex string) (r, g, b int) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 8 {
		hex = hex[2:] // Strip ARGB alpha
	}
	if len(hex) != 6 {
		return 128, 128, 128 // Default gray
	}

	fmt.Sscanf(hex[0:2], "%02X", &r)
	fmt.Sscanf(hex[2:4], "%02X", &g)
	fmt.Sscanf(hex[4:6], "%02X", &b)
	return
}

// buildDVMap builds a per-cell map of DataValidation rules.
func buildDVMap(dvs []*resolve.DataValidation) map[string]*resolve.DataValidation {
	if len(dvs) == 0 {
		return nil
	}
	m := make(map[string]*resolve.DataValidation)
	for _, dv := range dvs {
		for _, ref := range expandSqRef(dv.SqRef) {
			m[ref] = dv
		}
	}
	return m
}

// expandSqRef expands a space-separated list of A1 ranges into individual cell refs.
func expandSqRef(sqref string) []string {
	var out []string
	for _, part := range strings.Fields(sqref) {
		colon := strings.Index(part, ":")
		if colon < 0 {
			out = append(out, part)
			continue
		}
		// Range like A4:C51
		startRef := part[:colon]
		endRef := part[colon+1:]
		sr, sc, ok1 := parseA1(startRef)
		er, ec, ok2 := parseA1(endRef)
		if !ok1 || !ok2 {
			continue
		}
		for r := sr; r <= er; r++ {
			for c := sc; c <= ec; c++ {
				out = append(out, colLetterFromIndex(c)+strconv.Itoa(r))
			}
		}
	}
	return out
}

// parseA1 parses an A1-style cell reference into 1-based row and col.
func parseA1(ref string) (row, col int, ok bool) {
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		i++
	}
	if i == 0 || i >= len(ref) {
		return 0, 0, false
	}
	letters := ref[:i]
	rowStr := ref[i:]
	r, err := strconv.Atoi(rowStr)
	if err != nil || r < 1 {
		return 0, 0, false
	}
	c := 0
	for _, ch := range letters {
		c = c*26 + int(ch-'A') + 1
	}
	return r, c, true
}

// parseAutoFilterHeader returns the header row, min col, and max col for an autoFilter ref.
// Returns 0,0,0 when the ref is empty.
func parseAutoFilterHeader(ref string) (headerRow, colMin, colMax int) {
	if ref == "" {
		return 0, 0, 0
	}
	colon := strings.Index(ref, ":")
	if colon < 0 {
		r, c, ok := parseA1(ref)
		if !ok {
			return 0, 0, 0
		}
		return r, c, c
	}
	sr, sc, ok1 := parseA1(ref[:colon])
	_, ec, ok2 := parseA1(ref[colon+1:])
	if !ok1 || !ok2 {
		return 0, 0, 0
	}
	return sr, sc, ec
}

// buildAFActiveSet returns the set of 0-based within-range column indices that
// have active filter criteria. The key is (1-based absolute col) - afColMin,
// i.e. 0-based offset within the autoFilter range.
func buildAFActiveSet(filterColumns []*resolve.FilterColumn) map[int]bool {
	if len(filterColumns) == 0 {
		return nil
	}
	set := make(map[int]bool, len(filterColumns))
	for _, fc := range filterColumns {
		if fc != nil {
			set[fc.ColIndex] = true
		}
	}
	return set
}

// tableCellFlags holds visual styling flags for a cell that falls inside a structured table.
type tableCellFlags struct {
	isTableHeader bool
	isTableTotal  bool
	isBandedRow   bool
	isBandedCol   bool
	isFirstCol    bool
	isLastCol     bool
}

// buildTableCellFlags computes, for every cell ref in every table range, what table-visual
// role that cell plays (header, total, banded row, first/last column).
func buildTableCellFlags(tables []*resolve.Table) map[string]tableCellFlags {
	m := make(map[string]tableCellFlags)
	for _, tbl := range tables {
		if tbl == nil || tbl.Ref == "" {
			continue
		}
		minRow, maxRow, minCol, maxCol, ok := parseA1RangeForTable(tbl.Ref)
		if !ok {
			continue
		}
		headerRows := tbl.HeaderRowCount
		if headerRows == 0 {
			headerRows = 1
		}
		totalsRows := tbl.TotalsRowCount
		numCols := maxCol - minCol + 1
		numDataRows := maxRow - minRow + 1 - headerRows - totalsRows
		for r := minRow; r <= maxRow; r++ {
			isHeader := r < minRow+headerRows
			isTotal := r > maxRow-totalsRows
			rowOffset := r - minRow - headerRows // 0-based data row index
			bandedRow := false
			if tbl.TableStyle != nil && tbl.TableStyle.ShowRowStripes && !isHeader && !isTotal && numDataRows > 0 {
				bandedRow = rowOffset%2 == 1
			}
			for c := minCol; c <= maxCol; c++ {
				ref := colLetterFromIndex(c) + strconv.Itoa(r)
				colOffset := c - minCol
				bandedCol := false
				if tbl.TableStyle != nil && tbl.TableStyle.ShowColumnStripes && !isHeader && !isTotal && numCols > 0 {
					bandedCol = colOffset%2 == 1
				}
				firstCol := false
				lastCol := false
				if tbl.TableStyle != nil && numCols > 0 {
					firstCol = tbl.TableStyle.ShowFirstColumn && colOffset == 0
					lastCol = tbl.TableStyle.ShowLastColumn && colOffset == numCols-1
				}
				m[ref] = tableCellFlags{
					isTableHeader: isHeader,
					isTableTotal:  isTotal,
					isBandedRow:   bandedRow,
					isBandedCol:   bandedCol,
					isFirstCol:    firstCol,
					isLastCol:     lastCol,
				}
			}
		}
	}
	return m
}

// parseA1RangeForTable parses an A1 notation range like "G1:I4" into 1-based row/col bounds.
func parseA1RangeForTable(ref string) (minRow, maxRow, minCol, maxCol int, ok bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, 0, 0, 0, false
	}
	parts := strings.SplitN(ref, ":", 2)
	sr, sc, ok1 := parseA1(parts[0])
	if !ok1 {
		return 0, 0, 0, 0, false
	}
	er, ec := sr, sc
	if len(parts) == 2 {
		var ok2 bool
		er, ec, ok2 = parseA1(parts[1])
		if !ok2 {
			return 0, 0, 0, 0, false
		}
	}
	if er < sr {
		sr, er = er, sr
	}
	if ec < sc {
		sc, ec = ec, sc
	}
	return sr, er, sc, ec, true
}

// gridWrapBorderPx is the .grid-wrap border width in sheetCSS. Grid-wrap uses
// border-box sizing, so the border is part of min-height, while the anchored
// overlay is positioned from the padding box; wrapper sizing has to add the
// border back to keep the overlay from overflowing the scroll box.
const gridWrapBorderPx = 1

// gridWrapScrollbarPx is the height a classic horizontal scrollbar takes out of
// grid-wrap when a sheet is wider than the viewer. Reserving it keeps an
// anchored panel clear of the bottom edge — otherwise the panel fits the box
// yet still forces a vertical scrollbar. The value covers the 15px Chrome/Edge
// and 17px Firefox scrollbars; overlay-scrollbar platforms just get a little
// more breathing room below the panel.
const gridWrapScrollbarPx = 18

const sheetCSS = `
html { box-sizing: border-box; }
*, *::before, *::after { box-sizing: inherit; }
body { font-family: sans-serif; margin: 0; padding: 1rem; background: #f5f5f5; }
.sheet { margin-bottom: 2rem; position: relative; }
.sheet-name { font-size: 1rem; font-weight: 600; margin: 0 0 .5rem; }
.sheet-num { font-size: 11px; color: #888; margin-bottom: 4px; user-select: none; }
@media print { .sheet-num { display: none !important; } }
.outline-controls { display: flex; gap: .75rem; flex-wrap: wrap; margin: 0 0 .5rem; }
.outline-group { display: inline-flex; align-items: center; gap: .25rem; padding: .25rem .4rem; background: #fff; border: 1px solid #ddd; border-radius: 999px; }
.outline-label { font-size: 11px; font-weight: 600; color: #666; margin-right: .15rem; }
.outline-btn { border: 0; background: #f0f0f0; color: #333; border-radius: 999px; padding: .15rem .45rem; font-size: 11px; cursor: pointer; }
.outline-btn[aria-pressed="true"] { background: #1f6feb; color: #fff; }
.grid-wrap { overflow: auto; border: 1px solid #ddd; background: #fff; position: relative; }
.drawings-layer { position: absolute; top: 0; left: 0; pointer-events: none; z-index: 10; }
.drawing { position: absolute; box-sizing: border-box; font-size: 11px; overflow: hidden; display: flex; align-items: center; justify-content: center; }
.drawing--image { object-fit: fill; }
.drawing--image-placeholder { background: rgba(245, 239, 222, 0.92); border: 1px solid #9a8855; color: #6b5c34; font-size: 10px; padding: 2px 4px; text-align: center; }
.drawing--shape { color: #333; }
.drawing__shape-vector, .drawing__shape-bg { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; box-sizing: border-box; }
.drawing__text { position: relative; z-index: 1; text-align: center; }
.drawing--chart { background: transparent; border: 0; color: inherit; font-size: 10px; }
.drawing--chart > svg { display: block; width: 100%; height: 100%; }
.grid { border-collapse: collapse; font-size: 12px; }
.grid--fixed { table-layout: fixed; }
.cell { border: 1px solid #e0e0e0; padding: 2px 4px; white-space: nowrap; min-width: 40px; }
.grid--fixed .cell { min-width: 0; overflow: hidden; padding-top: 0; padding-bottom: 0; line-height: 1.2; }
.grid--fixed .cell--text-overflow { overflow: visible; position: relative; z-index: 2; }
.row--frozen .cell { background: #fafafa; }
.cell--frozen-col { position: sticky; left: 0; background: #fafafa; z-index: 1; }
.diagnostics { margin: 1rem 0; padding: 0.5rem; border: 1px solid #ccc; border-radius: 4px; background: #fff; }
.diagnostics summary { font-weight: bold; cursor: pointer; }
.diagnostics ul { margin: 0.5rem 0 0 0; padding-left: 1.5rem; }
.diagnostic-error { color: #c00; }
.diagnostic-warning { color: #c60; }
.diagnostic-info { color: #06c; }
.workbook-connections { margin: 1rem 0; border: 1px solid #c8d4e0; border-radius: 4px; padding: .75rem 1rem; background: #f7fafc; }
.workbook-connections-title { margin: 0 0 .35rem; font-size: 15px; color: #16324a; }
.workbook-connections-note { margin: 0 0 .6rem; font-size: 12px; color: #4b5b6a; }
.workbook-connections-table { border-collapse: collapse; font-size: 12px; width: 100%; }
.workbook-connections-table th, .workbook-connections-table td { border: 1px solid #d7e1ea; padding: 4px 8px; text-align: left; vertical-align: top; }
.workbook-connections-table thead th { background: #e7eef5; }
.workbook-external-links { margin: 1rem 0; border: 1px solid #d7d0bf; border-radius: 4px; padding: .75rem 1rem; background: #fbfaf6; }
.workbook-external-links-title { margin: 0 0 .35rem; font-size: 15px; color: #3b3120; }
.workbook-external-links-note { margin: 0 0 .6rem; font-size: 12px; color: #625845; }
.workbook-external-links-table { border-collapse: collapse; font-size: 12px; width: 100%; }
.workbook-external-links-table th, .workbook-external-links-table td { border: 1px solid #e2ddd1; padding: 4px 8px; text-align: left; vertical-align: top; }
.workbook-external-links-table thead th { background: #f0eadf; }
.sheet-protection { margin: 0.75rem 0; border: 1px solid #d7c58b; border-radius: 4px; padding: .65rem .8rem; background: #fff8e1; color: #5c4a14; }
.sheet-protection-title { margin: 0 0 .35rem; font-size: 12px; font-weight: 600; }
.sheet-protection-detail { margin: .2rem 0 0; font-size: 11px; }
.cell--af-header { position: relative; }
.af-arrow { font-size: 9px; margin-left: 2px; opacity: 0.6; vertical-align: middle; }
.af-arrow--active { opacity: 1; color: #1a6fc4; }
.cell--af-active { background-color: rgba(26, 111, 196, 0.08) !important; }
.cell--has-comment { position: relative; padding-right: 12px; }
.comment-mark { position: absolute; top: 1px; right: 2px; font-size: 8px; line-height: 1; color: #c0392b; }
.cell--formula { box-shadow: inset 0 0 0 1px rgba(31, 111, 235, 0.18); }
.cell--sparkline { padding: 0; overflow: hidden; }
.cell--sparkline svg { display: block; width: 100%; height: 100%; min-height: 12px; }
.cell--table-header { background: #e7e6e6; font-weight: 600; }
.cell--table-total { background: #f2f2f2; font-weight: 600; border-top: 2px solid #bdbdbd; }
.cell--table-banded-row { background: #f8f9fa; }
.cell--table-banded-col { background: #f8f9fa; }
.cell--table-first-col { font-weight: 600; }
.cell--table-last-col { font-weight: 600; }
tr[data-outline-hidden="true"] { display: none; }
col[data-outline-hidden="true"], td[data-outline-hidden="true"] { visibility: collapse; }
.pivot-table { margin: 1rem 0; border: 1px solid #c8d4e0; border-radius: 4px; padding: .75rem 1rem; background: #f5f8fb; }
.pivot-table-name { margin: 0 0 .5rem; font-size: 13px; color: #1a3a5c; }
.pivot-source { margin: 0 0 .5rem; font-size: 11px; color: #555; }
.pivot-source code { background: #e6edf3; border-radius: 3px; padding: 1px 4px; }
.pivot-meta { border-collapse: collapse; font-size: 12px; }
.pivot-meta th, .pivot-meta td { border: 1px solid #c8d4e0; padding: 3px 8px; text-align: left; }
.pivot-meta thead th { background: #dde8f0; }
.structured-table { margin: 1rem 0; border: 1px solid #c8d4e0; border-radius: 4px; padding: .75rem 1rem; background: #f5f8fb; }
.structured-table-name { margin: 0 0 .5rem; font-size: 13px; color: #1a3a5c; }
.structured-table-meta { border-collapse: collapse; font-size: 12px; }
.structured-table-meta th, .structured-table-meta td { border: 1px solid #c8d4e0; padding: 3px 8px; text-align: left; }
.structured-table-meta thead th { background: #dde8f0; }
.form-control { position: absolute; font-size: 11px; box-sizing: border-box; }
.form-control--button { cursor: default; border: 1px solid #999; border-radius: 2px; background: #f0f0f0; padding: 0 6px; white-space: nowrap; overflow: hidden; }
.form-control--checkbox, .form-control--radio { display: inline-flex; align-items: center; gap: 3px; white-space: nowrap; overflow: hidden; }
.form-control--dropdown, .form-control--list, .form-control--edit { border: 1px solid #999; border-radius: 2px; background: #fff; }
.form-control--label { display: inline-block; white-space: nowrap; overflow: hidden; }
.form-control--spin, .form-control--scroll { padding: 0; }
.form-control--scroll-vertical { min-height: 48px; }
`

const sheetJS = `
function sheetSetOutline(button) {
	var sheet = button.closest('.sheet');
	if (!sheet) return;
	var axis = button.getAttribute('data-axis');
	var levelAttr = button.getAttribute('data-level');
	var level = levelAttr === 'all' ? null : parseInt(levelAttr, 10);
	var buttons = sheet.querySelectorAll('.outline-btn[data-axis="' + axis + '"]');
	buttons.forEach(function (btn) {
		btn.setAttribute('aria-pressed', btn === button ? 'true' : 'false');
	});
	if (axis === 'rows') {
		sheet.querySelectorAll('tr[data-outline-level]').forEach(function (row) {
			var baseHidden = row.getAttribute('data-base-hidden') === 'true';
			var rowLevel = parseInt(row.getAttribute('data-outline-level') || '0', 10);
			if (!baseHidden && level !== null && rowLevel > level) {
				row.setAttribute('data-outline-hidden', 'true');
			} else {
				row.removeAttribute('data-outline-hidden');
			}
		});
		return;
	}
	if (axis === 'cols') {
		sheet.querySelectorAll('[data-outline-col-level]').forEach(function (node) {
			var baseHidden = node.getAttribute('data-base-hidden') === 'true';
			var colLevel = parseInt(node.getAttribute('data-outline-col-level') || '0', 10);
			if (!baseHidden && level !== null && colLevel > level) {
				node.setAttribute('data-outline-hidden', 'true');
			} else {
				node.removeAttribute('data-outline-hidden');
			}
		});
	}
}

// sheetGridFit pads each sheet's grid with empty cells until it covers the box
// the viewer actually shows, so a sheet reads as a full spreadsheet surface
// without the rendered document carrying cells no one sees. It only ever adds
// rows and columns that still fit, so no scrollbar appears for the padding.
function sheetGridFit() {
	document.querySelectorAll('.grid-wrap').forEach(function (wrap) {
		var table = wrap.querySelector('table.grid');
		if (!table) return;
		sheetGridFitReset(table);
		var boxWidth = wrap.clientWidth;
		var boxHeight = wrap.clientHeight;
		if (boxWidth <= 0 || boxHeight <= 0) return;
		var tableWidth = table.getBoundingClientRect().width;
		var tableHeight = table.getBoundingClientRect().height;
		var colgroup = table.querySelector('colgroup');
		var rows = Array.prototype.slice.call(table.rows);
		if (!rows.length) return;

		var colCount = sheetGridColumnCount(table, colgroup, rows);
		var unitWidth = parseFloat(table.getAttribute('data-col-px')) || 64;
		while (unitWidth > 0 && tableWidth + unitWidth <= boxWidth) {
			sheetGridAppendColumn(table, colgroup, rows, colCount, unitWidth);
			tableWidth += unitWidth;
			colCount++;
		}

		var unitHeight = parseFloat(table.getAttribute('data-row-px')) || 18;
		var rowParent = table.tBodies.length ? table.tBodies[0] : table;
		while (unitHeight > 0 && tableHeight + unitHeight <= boxHeight) {
			rowParent.appendChild(sheetGridBuildRow(colCount));
			tableHeight += unitHeight;
		}
	});
}

function sheetGridFitReset(table) {
	table.querySelectorAll('tr[data-grid-fit]').forEach(function (row) {
		row.remove();
	});
	table.querySelectorAll('col[data-grid-fit], td[data-grid-fit]').forEach(function (node) {
		node.remove();
	});
}

function sheetGridColumnCount(table, colgroup, rows) {
	if (colgroup) {
		var cols = colgroup.querySelectorAll('col');
		if (cols.length) return cols.length;
	}
	for (var i = rows.length - 1; i >= 0; i--) {
		if (rows[i].children.length) return rows[i].children.length;
	}
	return 0;
}

function sheetGridCellWidth(rows) {
	for (var i = rows.length - 1; i >= 0; i--) {
		var cells = rows[i].children;
		if (cells.length) return cells[cells.length - 1].getBoundingClientRect().width;
	}
	return 0;
}

function sheetGridRowHeight(rows) {
	for (var i = rows.length - 1; i >= 0; i--) {
		var height = rows[i].getBoundingClientRect().height;
		if (height > 0) return height;
	}
	return 0;
}

function sheetGridAppendColumn(table, colgroup, rows, colCount, unitWidth) {
	if (colgroup) {
		var col = document.createElement('col');
		col.setAttribute('data-grid-fit', 'true');
		col.style.width = unitWidth + 'px';
		colgroup.appendChild(col);
	}
	rows.forEach(function (row) {
		var cells = row.children;
		if (!cells.length) return;
		var cell = document.createElement('td');
		cell.className = 'cell';
		cell.setAttribute('data-grid-fit', 'true');
		cell.setAttribute('data-col-index', String(colCount + 1));
		row.appendChild(cell);
	});
}

function sheetGridBuildRow(colCount) {
	var row = document.createElement('tr');
	row.className = 'row';
	row.setAttribute('data-grid-fit', 'true');
	for (var i = 1; i <= colCount; i++) {
		var cell = document.createElement('td');
		cell.className = 'cell';
		cell.setAttribute('data-grid-fit', 'true');
		cell.setAttribute('data-col-index', String(i));
		row.appendChild(cell);
	}
	return row;
}

function sheetGridFitOnLoad() {
	sheetGridFit();
	var pending = false;
	window.addEventListener('resize', function () {
		if (pending) return;
		pending = true;
		window.requestAnimationFrame(function () {
			pending = false;
			sheetGridFit();
		});
	});
}

if (document.readyState === 'loading') {
	document.addEventListener('DOMContentLoaded', sheetGridFitOnLoad);
} else {
	sheetGridFitOnLoad();
}
`
