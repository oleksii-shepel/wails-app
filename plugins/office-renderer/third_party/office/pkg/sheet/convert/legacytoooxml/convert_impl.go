package legacytoooxml

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	chartmodel "office/pkg/ooxml/chart"
	"office/pkg/ooxml/drawing"
	drawingxml "office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/media"
	"office/pkg/ooxml/smartart"
	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

// colInfo holds per-column metadata for grouping into ColRanges.
type colInfo struct {
	width     float64
	hidden    bool
	outline   int
	collapsed bool
}

// ── Defined Names ────────────────────────────────────────────────────────────

func (c *converter) mapDefinedNames(wb *sheetir.Workbook) {
	for _, dn := range wb.DefinedNames {
		pdn := sheetparts.DefinedName{
			Name:   dn.Name,
			Hidden: dn.Hidden,
			Value:  dn.Value,
		}
		if dn.LocalSheetID != nil {
			id := *dn.LocalSheetID
			pdn.LocalSheetID = &id
		}
		c.pkg.Workbook.DefinedNames = append(c.pkg.Workbook.DefinedNames, pdn)
	}
	for i, sheet := range wb.Sheets {
		if sheet != nil && sheet.PrintArea != "" && !c.hasDefinedName("_xlnm.Print_Area", i) {
			value := buildPrintAreaDefinedNameValue(sheet.Name, sheet.PrintArea)
			if value != "" {
				localSheetID := i
				c.pkg.Workbook.DefinedNames = append(c.pkg.Workbook.DefinedNames, sheetparts.DefinedName{
					Name:         "_xlnm.Print_Area",
					LocalSheetID: &localSheetID,
					Value:        value,
				})
			}
		}
		if sheet == nil || (sheet.PrintTitleRows == "" && sheet.PrintTitleCols == "") {
			continue
		}
		if c.hasDefinedName("_xlnm.Print_Titles", i) {
			continue
		}
		value := buildPrintTitlesDefinedNameValue(sheet.Name, sheet.PrintTitleRows, sheet.PrintTitleCols)
		if value == "" {
			continue
		}
		localSheetID := i
		c.pkg.Workbook.DefinedNames = append(c.pkg.Workbook.DefinedNames, sheetparts.DefinedName{
			Name:         "_xlnm.Print_Titles",
			LocalSheetID: &localSheetID,
			Value:        value,
		})
	}
}

func (c *converter) hasDefinedName(name string, localSheetID int) bool {
	for _, dn := range c.pkg.Workbook.DefinedNames {
		if !strings.EqualFold(dn.Name, name) {
			continue
		}
		if dn.LocalSheetID != nil && *dn.LocalSheetID == localSheetID {
			return true
		}
	}
	return false
}

func buildPrintTitlesDefinedNameValue(sheetName, rows, cols string) string {
	sheetRef := quoteDefinedNameSheetName(sheetName) + "!"
	var parts []string
	if rows != "" {
		parts = append(parts, sheetRef+rows)
	}
	if cols != "" {
		parts = append(parts, sheetRef+cols)
	}
	return strings.Join(parts, ",")
}

func buildPrintAreaDefinedNameValue(sheetName, area string) string {
	sheetRef := quoteDefinedNameSheetName(sheetName) + "!"
	parts := strings.Split(area, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, sheetRef+part)
	}
	return strings.Join(out, ",")
}

func quoteDefinedNameSheetName(name string) string {
	name = strings.ReplaceAll(name, "'", "''")
	return "'" + name + "'"
}

func (c *converter) mapWorkbookProtection(wb *sheetir.Workbook) {
	if wb == nil || wb.Protection == nil {
		return
	}
	wp := wb.Protection
	c.pkg.Workbook.Protection = &sheetparts.WorkbookProtection{
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

// ── Columns ──────────────────────────────────────────────────────────────────

func (c *converter) mapColumns(sheet *sheetir.Sheet, ws *sheetparts.Worksheet) {
	infos := make(map[int]colInfo)

	for col, width := range sheet.ColumnWidths {
		infos[col] = colInfo{width: width, hidden: sheet.HiddenCols[col]}
	}
	for col := range sheet.HiddenCols {
		if _, ok := infos[col]; !ok {
			infos[col] = colInfo{hidden: true}
		}
	}
	for col, level := range sheet.OutlineCols {
		info := infos[col]
		info.outline = level
		infos[col] = info
	}
	for col := range sheet.CollapsedCols {
		info := infos[col]
		info.collapsed = true
		infos[col] = info
	}

	if len(infos) == 0 {
		return
	}

	cols := make([]int, 0, len(infos))
	for col := range infos {
		cols = append(cols, col)
	}
	for i := 1; i < len(cols); i++ {
		for j := i; j > 0 && cols[j] < cols[j-1]; j-- {
			cols[j], cols[j-1] = cols[j-1], cols[j]
		}
	}

	var start int
	var cur colInfo
	for i, col := range cols {
		info := infos[col]
		if i == 0 {
			start = col
			cur = info
			continue
		}
		if info != cur {
			ws.Cols = append(ws.Cols, makeColRange(start, cols[i-1], cur))
			start = col
			cur = info
		}
	}
	ws.Cols = append(ws.Cols, makeColRange(start, cols[len(cols)-1], cur))
}

func makeColRange(minCol, maxCol int, info colInfo) sheetparts.ColRange {
	cr := sheetparts.ColRange{
		Min:          minCol,
		Max:          maxCol,
		Width:        info.width,
		CustomWidth:  info.width > 0,
		Hidden:       info.hidden,
		OutlineLevel: info.outline,
		Collapsed:    info.collapsed,
	}
	if info.width == 0 && !info.hidden && info.outline == 0 && !info.collapsed {
		cr.CustomWidth = false
	}
	return cr
}

// ── Rows and Cells ───────────────────────────────────────────────────────────

func (c *converter) mapRowsAndCells(sheet *sheetir.Sheet, ws *sheetparts.Worksheet) {
	formulaOverrides := buildFormulaOverrides(sheet)
	linkedCellOverrides := buildFormControlLinkedCellOverrides(sheet)
	if len(linkedCellOverrides) == 0 {
		for _, row := range sheet.Rows {
			if row == nil {
				continue
			}
			pr := sheetparts.Row{
				Index:        row.Index,
				Height:       row.HeightPt,
				CustomHt:     row.HeightPt > 0,
				Hidden:       row.Hidden,
				OutlineLevel: row.OutlineLevel,
				Collapsed:    row.Collapsed,
			}
			if row.Style != nil {
				pr.StyleIndex = c.getOrAddStyle(row.Style)
			}
			for _, cell := range row.Cells {
				if cell == nil {
					continue
				}
				pr.Cells = append(pr.Cells, c.convertCell(cell, formulaOverrides))
			}
			ws.Rows = append(ws.Rows, pr)
		}
		return
	}

	rowByIndex := make(map[int]*sheetir.Row, len(sheet.Rows))
	rowIndexes := make([]int, 0, len(sheet.Rows)+len(linkedCellOverrides))
	seenRows := make(map[int]bool, len(sheet.Rows)+len(linkedCellOverrides))
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		rowByIndex[row.Index] = row
		if !seenRows[row.Index] {
			rowIndexes = append(rowIndexes, row.Index)
			seenRows[row.Index] = true
		}
	}
	for rowIndex := range linkedCellOverrides {
		if !seenRows[rowIndex] {
			rowIndexes = append(rowIndexes, rowIndex)
			seenRows[rowIndex] = true
		}
	}
	sort.Ints(rowIndexes)

	for _, rowIndex := range rowIndexes {
		row := rowByIndex[rowIndex]
		pr := sheetparts.Row{
			Index: rowIndex,
		}
		if row != nil {
			pr.Height = row.HeightPt
			pr.CustomHt = row.HeightPt > 0
			pr.Hidden = row.Hidden
			pr.OutlineLevel = row.OutlineLevel
			pr.Collapsed = row.Collapsed
			if row.Style != nil {
				pr.StyleIndex = c.getOrAddStyle(row.Style)
			}
		}

		overrideCols := linkedCellOverrides[rowIndex]
		consumedOverrides := make(map[int]bool, len(overrideCols))
		if row != nil {
			for _, cell := range row.Cells {
				if cell == nil {
					continue
				}
				if override := overrideCols[cell.Col]; override != nil {
					pr.Cells = append(pr.Cells, c.convertCellWithFormulaOverride(override, formulaOverrides, false))
					consumedOverrides[cell.Col] = true
					continue
				}
				pr.Cells = append(pr.Cells, c.convertCell(cell, formulaOverrides))
			}
		}

		if len(overrideCols) > len(consumedOverrides) {
			extraCols := make([]int, 0, len(overrideCols)-len(consumedOverrides))
			for col := range overrideCols {
				if !consumedOverrides[col] {
					extraCols = append(extraCols, col)
				}
			}
			sort.Ints(extraCols)
			for _, col := range extraCols {
				pr.Cells = append(pr.Cells, c.convertCellWithFormulaOverride(overrideCols[col], formulaOverrides, false))
			}
		}
		ws.Rows = append(ws.Rows, pr)
	}
}

func (c *converter) convertCell(cell *sheetir.Cell, formulaOverrides map[string]sheetparts.Formula) sheetparts.Cell {
	return c.convertCellWithFormulaOverride(cell, formulaOverrides, true)
}

func (c *converter) convertCellWithFormulaOverride(cell *sheetir.Cell, formulaOverrides map[string]sheetparts.Formula, allowFormulaOverride bool) sheetparts.Cell {
	pc := sheetparts.Cell{
		Ref: cell.Ref,
	}
	if pc.Ref == "" {
		pc.Ref = cellRef(cell.Row, cell.Col)
	}

	if cell.Style != (sheetir.CellStyle{}) {
		pc.StyleIndex = c.getOrAddStyle(&cell.Style)
	}

	switch cell.Kind {
	case sheetir.CellString:
		// Legacy rich-text metadata is occasionally partial: BIFF formatting runs
		// may describe only a suffix of the cell text. Only serialize rich text
		// when its runs reconstruct the complete canonical cell value; otherwise
		// preserve the full value through the shared-string path.
		if len(cell.RichTextRuns) > 0 && richTextRunsText(cell.RichTextRuns) == cell.StrValue {
			pc.Type = "inlineStr"
			pc.InlineStr = c.convertRichText(cell.RichTextRuns)
		} else {
			pc.Type = "s"
			pc.Value = strconv.Itoa(c.addString(cell.StrValue))
		}
	case sheetir.CellNumber, sheetir.CellDate:
		pc.Value = fmt.Sprintf("%v", cell.NumValue)
	case sheetir.CellBool:
		pc.Type = "b"
		if cell.NumValue != 0 {
			pc.Value = "1"
		} else {
			pc.Value = "0"
		}
	case sheetir.CellError:
		pc.Type = "e"
		pc.Value = cell.StrValue
	case sheetir.CellFormula:
		pc.Type = "str"
		pc.Value = cell.StrValue
	}

	if allowFormulaOverride {
		if override, ok := formulaOverrides[strings.ToUpper(pc.Ref)]; ok {
			formula := override
			pc.Formula = &formula
		} else if cell.Formula != "" {
			pc.Formula = &sheetparts.Formula{Text: cell.Formula}
		}
	} else if cell.Formula != "" {
		pc.Formula = &sheetparts.Formula{Text: cell.Formula}
	}

	return pc
}

func buildFormControlLinkedCellOverrides(sheet *sheetir.Sheet) map[int]map[int]*sheetir.Cell {
	linkedValues := collectFormControlLinkedCellValues(sheet)
	if len(linkedValues) == 0 {
		return nil
	}

	existingCells := make(map[string]*sheetir.Cell)
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil {
				continue
			}
			ref := normalizeConverterFormControlLink(cell.Ref)
			if ref != "" {
				existingCells[ref] = cell
			}
		}
	}

	out := make(map[int]map[int]*sheetir.Cell)
	for ref, value := range linkedValues {
		row, col, ok := parseCellRef(ref)
		if !ok {
			continue
		}
		var override sheetir.Cell
		if existing := existingCells[ref]; existing != nil {
			override = *existing
		}
		override.Row = row
		override.Col = col
		override.Ref = cellRef(row, col)
		override.Kind = sheetir.CellNumber
		override.StrValue = ""
		override.NumValue = value
		override.Formula = ""
		override.ArrayFormulaRange = ""
		override.ArrayFormulaAnchor = ""
		override.RichTextRuns = nil
		if out[row] == nil {
			out[row] = make(map[int]*sheetir.Cell)
		}
		cellCopy := override
		out[row][col] = &cellCopy
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func collectFormControlLinkedCellValues(sheet *sheetir.Sheet) map[string]float64 {
	if sheet == nil {
		return nil
	}
	values := make(map[string]float64)
	for ref, selected := range collectRadioLinkedCellSelections(sheet) {
		if selected > 0 {
			values[ref] = float64(selected)
		}
	}
	for _, fc := range sheet.FormControls {
		if fc == nil {
			continue
		}
		link := normalizeConverterFormControlLink(fc.FmlaLink)
		if link == "" {
			continue
		}
		switch fc.Type {
		case "Drop":
			if fc.Val > 0 {
				values[link] = float64(fc.Val)
			}
		case "List":
			if fc.SelectionType == "multi" || fc.SelectionType == "extended" {
				continue
			}
			if selected := firstSelectedFormControlIndex(fc); selected > 0 {
				values[link] = float64(selected)
			}
		case "Spin", "Scroll":
			values[link] = float64(fc.Val)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func firstSelectedFormControlIndex(fc *sheetir.FormControl) int {
	if fc == nil {
		return 0
	}
	for _, idx := range fc.Selected {
		if idx > 0 {
			return idx
		}
	}
	if fc.Val > 0 {
		return fc.Val
	}
	return 0
}

func collectRadioLinkedCellSelections(sheet *sheetir.Sheet) map[string]int {
	if sheet == nil || len(sheet.FormControls) == 0 {
		return nil
	}
	groupBoxes := collectConverterRadioGroupBoxes(sheet.FormControls)
	selections := make(map[string]int)
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
		groupKey := effectiveConverterRadioGroupKey(fc, groupBoxes)
		if fc.FirstButton || !lastWasRadio || groupKey != lastGroupKey {
			groupPosition = 1
		} else {
			groupPosition++
		}
		if fc.Checked {
			if link := effectiveConverterRadioGroupLink(fc, groupBoxes); link != "" {
				if _, exists := selections[link]; !exists {
					selections[link] = groupPosition
				}
			}
		}
		lastWasRadio = true
		lastGroupKey = groupKey
	}
	if len(selections) == 0 {
		return nil
	}
	return selections
}

type converterRadioGroupBox struct {
	link               string
	startCol, startRow int
	endCol, endRow     int
}

func collectConverterRadioGroupBoxes(controls []*sheetir.FormControl) []converterRadioGroupBox {
	out := make([]converterRadioGroupBox, 0, len(controls))
	for _, fc := range controls {
		if fc == nil || fc.Type != "GBox" {
			continue
		}
		link := normalizeConverterFormControlLink(fc.FmlaGroup)
		startCol, startRow, endCol, endRow, ok := parseFormControlAnchorBounds(fc.Anchor)
		if link == "" || !ok {
			continue
		}
		out = append(out, converterRadioGroupBox{
			link:     link,
			startCol: startCol,
			startRow: startRow,
			endCol:   endCol,
			endRow:   endRow,
		})
	}
	return out
}

func effectiveConverterRadioGroupKey(fc *sheetir.FormControl, groupBoxes []converterRadioGroupBox) string {
	if fc == nil {
		return ""
	}
	if link := effectiveConverterRadioGroupLink(fc, groupBoxes); link != "" {
		return "link:" + link
	}
	return strings.TrimSpace(fc.AltText)
}

func effectiveConverterRadioGroupLink(fc *sheetir.FormControl, groupBoxes []converterRadioGroupBox) string {
	if fc == nil {
		return ""
	}
	if link := containingConverterRadioGroupBoxLink(fc, groupBoxes); link != "" {
		return link
	}
	return normalizeConverterFormControlLink(fc.FmlaLink)
}

func containingConverterRadioGroupBoxLink(fc *sheetir.FormControl, groupBoxes []converterRadioGroupBox) string {
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

func normalizeConverterFormControlLink(ref string) string {
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

func buildFormulaOverrides(sheet *sheetir.Sheet) map[string]sheetparts.Formula {
	out := make(map[string]sheetparts.Formula)
	for _, group := range sheet.ArrayFormulas {
		if group == nil || group.AnchorRef == "" || group.RangeRef == "" || group.Formula == "" {
			continue
		}
		out[strings.ToUpper(strings.TrimSpace(group.AnchorRef))] = sheetparts.Formula{
			Type: "array",
			Ref:  group.RangeRef,
			Text: group.Formula,
		}
	}
	for _, group := range sheet.SharedFormulas {
		if group == nil || group.AnchorRef == "" || group.RangeRef == "" || group.Formula == "" {
			continue
		}
		si := group.Index
		out[strings.ToUpper(strings.TrimSpace(group.AnchorRef))] = sheetparts.Formula{
			Type:        "shared",
			Ref:         group.RangeRef,
			SharedIndex: &si,
			Text:        group.Formula,
		}
		for r := group.TopRow; r <= group.BottomRow; r++ {
			for col := group.LeftCol; col <= group.RightCol; col++ {
				ref := cellRef(r, col)
				if strings.EqualFold(ref, group.AnchorRef) {
					continue
				}
				dependentSI := group.Index
				out[strings.ToUpper(ref)] = sheetparts.Formula{
					Type:        "shared",
					SharedIndex: &dependentSI,
				}
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func richTextRunsText(runs []*sheetir.RichTextRun) string {
	var b strings.Builder
	for _, run := range runs {
		if run != nil {
			b.WriteString(run.Text)
		}
	}
	return b.String()
}

func (c *converter) convertRichText(runs []*sheetir.RichTextRun) *sheetparts.RichText {
	rt := &sheetparts.RichText{}
	if len(runs) == 0 {
		return rt
	}
	if len(runs) == 1 {
		rt.PlainText = runs[0].Text
		return rt
	}
	for _, run := range runs {
		pr := sheetparts.RTRun{Text: run.Text}
		if run.Bold || run.Italic || run.Underline || run.Strike || run.VertAlign != "" ||
			run.FontSizePt > 0 || run.FontColor != "" || run.FontFamily != "" {
			pr.Properties = &sheetparts.RTProps{}
			if run.Bold {
				pr.Properties.Bold = &struct{}{}
			}
			if run.Italic {
				pr.Properties.Italic = &struct{}{}
			}
			if run.Underline {
				pr.Properties.Underline = &sheetparts.StringVal{Val: run.UnderlineType}
			}
			if run.VertAlign != "" {
				pr.Properties.VertAlign = &sheetparts.StringVal{Val: run.VertAlign}
			}
			if run.Strike {
				pr.Properties.Strike = &struct{}{}
			}
			if run.FontSizePt > 0 {
				pr.Properties.FontSize = &sheetparts.FloatVal{Val: run.FontSizePt}
			}
			if run.FontColor != "" {
				pr.Properties.Color = &sheetparts.ColorVal{RGB: run.FontColor}
			}
			if run.FontFamily != "" {
				pr.Properties.FontName = &sheetparts.StringVal{Val: run.FontFamily}
			}
		}
		rt.Runs = append(rt.Runs, pr)
	}
	return rt
}

// ── Styles ───────────────────────────────────────────────────────────────────

func (c *converter) initNamedCellStyles() {
	if c == nil || c.pkg == nil || c.pkg.Styles == nil {
		return
	}
	namedXFs := c.irWb.NamedCellStyleXFs
	if len(namedXFs) == 0 {
		namedXFs = []sheetir.CellStyle{{}}
	}
	c.pkg.Styles.CellStyleXfs = make([]sheetparts.Xf, len(namedXFs))
	for i, style := range namedXFs {
		c.pkg.Styles.CellStyleXfs[i] = c.buildXfFromStyle(style, 0)
	}

	namedStyles := c.irWb.NamedCellStyles
	if len(namedStyles) == 0 {
		builtinID := 0
		namedStyles = []*sheetir.NamedCellStyle{{
			Name:         "Normal",
			StyleXFIndex: 0,
			BuiltinID:    &builtinID,
			Style:        namedXFs[0],
		}}
	}
	c.pkg.Styles.CellStyles = make([]sheetparts.CellStyle, 0, len(namedStyles))
	for _, style := range namedStyles {
		if style == nil {
			continue
		}
		entry := sheetparts.CellStyle{
			Name: style.Name,
			XfID: style.StyleXFIndex,
		}
		if style.BuiltinID != nil {
			entry.BuiltinID = *style.BuiltinID
		}
		c.pkg.Styles.CellStyles = append(c.pkg.Styles.CellStyles, entry)
	}
}

func (c *converter) styleXFIndexForStyle(style sheetir.CellStyle) int {
	for i, named := range c.irWb.NamedCellStyleXFs {
		if named == style {
			return i
		}
	}
	return 0
}

func (c *converter) getOrAddStyle(style *sheetir.CellStyle) int {
	xf := c.buildXfFromStyle(*style, c.styleXFIndexForStyle(*style))
	return c.getOrAddXf(xf)
}

func (c *converter) buildXfFromStyle(style sheetir.CellStyle, xfID int) sheetparts.Xf {
	// Build font.
	font := sheetparts.Font{}
	if style.Bold {
		font.Bold = &struct{}{}
	}
	if style.Italic {
		font.Italic = &struct{}{}
	}
	if style.Underline {
		font.Underline = &sheetparts.StringVal{Val: style.UnderlineType}
	}
	if style.FontVertAlign != "" {
		font.VertAlign = &sheetparts.StringVal{Val: style.FontVertAlign}
	}
	if style.Strike {
		font.Strike = &struct{}{}
	}
	if style.FontSizePt > 0 {
		font.Size = &sheetparts.FloatVal{Val: style.FontSizePt}
	}
	if style.FontColor != "" {
		font.Color = &sheetparts.ColorVal{RGB: style.FontColor}
	}
	if style.FontFamily != "" {
		font.Name = &sheetparts.StringVal{Val: style.FontFamily}
	}
	fontID := c.getOrAddFont(font)

	// Build fill. Preserve the canonical pattern instead of flattening every
	// authored fill to solid during legacy -> OOXML conversion.
	fill := sheetparts.Fill{}
	pattern := style.FillPattern
	if pattern == "" && style.BgColor != "" {
		pattern = "solid"
	}
	if pattern == "" {
		pattern = "none"
	}
	fill.PatternFill = &sheetparts.PatternFill{PatternType: pattern}
	if style.BgColor != "" {
		fill.PatternFill.FgColor = &sheetparts.ColorVal{RGB: style.BgColor}
	}
	if style.FillPatternBgColor != "" {
		fill.PatternFill.BgColor = &sheetparts.ColorVal{RGB: style.FillPatternBgColor}
	}
	fillID := c.getOrAddFill(fill)

	// Build border.
	border := sheetparts.Border{}
	if style.BorderTop.Style != "" {
		border.Top = &sheetparts.BorderSide{
			Style: style.BorderTop.Style,
			Color: &sheetparts.ColorVal{RGB: style.BorderTop.Color},
		}
	}
	if style.BorderRight.Style != "" {
		border.Right = &sheetparts.BorderSide{
			Style: style.BorderRight.Style,
			Color: &sheetparts.ColorVal{RGB: style.BorderRight.Color},
		}
	}
	if style.BorderBottom.Style != "" {
		border.Bottom = &sheetparts.BorderSide{
			Style: style.BorderBottom.Style,
			Color: &sheetparts.ColorVal{RGB: style.BorderBottom.Color},
		}
	}
	if style.BorderLeft.Style != "" {
		border.Left = &sheetparts.BorderSide{
			Style: style.BorderLeft.Style,
			Color: &sheetparts.ColorVal{RGB: style.BorderLeft.Color},
		}
	}
	if style.BorderDiagonal.Style != "" {
		border.Diagonal = &sheetparts.BorderSide{Style: style.BorderDiagonal.Style, Color: &sheetparts.ColorVal{RGB: style.BorderDiagonal.Color}}
		border.DiagonalUp = style.BorderDiagonalUp
		border.DiagonalDown = style.BorderDiagonalDown
	}
	if style.BorderVertical.Style != "" {
		border.Vertical = &sheetparts.BorderSide{Style: style.BorderVertical.Style, Color: &sheetparts.ColorVal{RGB: style.BorderVertical.Color}}
	}
	if style.BorderHorizontal.Style != "" {
		border.Horizontal = &sheetparts.BorderSide{Style: style.BorderHorizontal.Style, Color: &sheetparts.ColorVal{RGB: style.BorderHorizontal.Color}}
	}
	borderID := c.getOrAddBorder(border)

	// Number format.
	numFmtID := 0
	if style.NumFmtCode != "" {
		numFmtID = c.getOrAddNumFmt(style.NumFmtCode)
	}

	// Alignment.
	var alignment *sheetparts.Alignment
	if style.HAlign != "" || style.VAlign != "" || style.WrapText || style.ShrinkToFit || style.Indent != 0 || style.TextRotation != 0 {
		alignment = &sheetparts.Alignment{
			Horizontal:   style.HAlign,
			Vertical:     spreadsheetVerticalAlign(style.VAlign),
			WrapText:     style.WrapText,
			ShrinkToFit:  style.ShrinkToFit,
			Indent:       style.Indent,
			TextRotation: style.TextRotation,
		}
	}

	var protection *sheetparts.CellProtection
	if style.Locked || style.FormulaHidden {
		protection = &sheetparts.CellProtection{
			Locked:        style.Locked,
			FormulaHidden: style.FormulaHidden,
		}
	}

	// Build xf. Cell XFs are complete canonical styles. Mark each authored
	// component as applied so a zero component ID can intentionally reset a
	// non-zero named-style component instead of accidentally inheriting it.
	xf := sheetparts.Xf{
		NumFmtID:          numFmtID,
		FontID:            fontID,
		FillID:            fillID,
		BorderID:          borderID,
		XfID:              xfID,
		ApplyNumberFormat: style.NumFmtCode != "",
		ApplyFont:         style.Bold || style.Italic || style.Underline || style.Strike || style.FontVertAlign != "" || style.FontSizePt > 0 || style.FontColor != "" || style.FontFamily != "",
		ApplyFill:         style.BgColor != "" || (style.FillPattern != "" && style.FillPattern != "none") || style.FillPatternBgColor != "",
		ApplyBorder:       style.BorderTop.Style != "" || style.BorderRight.Style != "" || style.BorderBottom.Style != "" || style.BorderLeft.Style != "" || style.BorderDiagonal.Style != "" || style.BorderVertical.Style != "" || style.BorderHorizontal.Style != "",
		ApplyAlignment:    alignment != nil,
		ApplyProtection:   protection != nil,
		Alignment:         alignment,
		Protection:        protection,
	}
	return xf
}

func spreadsheetVerticalAlign(v string) string {
	switch v {
	case "middle":
		return "center"
	default:
		return v
	}
}

func (c *converter) getOrAddFont(font sheetparts.Font) int {
	for i, f := range c.pkg.Styles.Fonts {
		if fontsEqual(f, font) {
			return i
		}
	}
	idx := len(c.pkg.Styles.Fonts)
	c.pkg.Styles.Fonts = append(c.pkg.Styles.Fonts, font)
	return idx
}

func fontsEqual(a, b sheetparts.Font) bool {
	return boolPtrEqual(a.Bold, b.Bold) &&
		boolPtrEqual(a.Italic, b.Italic) &&
		stringValEqual(a.Underline, b.Underline) &&
		boolPtrEqual(a.Strike, b.Strike) &&
		stringValEqual(a.VertAlign, b.VertAlign) &&
		floatValEqual(a.Size, b.Size) &&
		colorValEqual(a.Color, b.Color) &&
		stringValEqual(a.Name, b.Name) &&
		stringValEqual(a.Scheme, b.Scheme)
}

func boolPtrEqual(a, b *struct{}) bool {
	return (a == nil) == (b == nil)
}

func floatValEqual(a, b *sheetparts.FloatVal) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Val == b.Val
}

func stringValEqual(a, b *sheetparts.StringVal) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Val == b.Val
}

func colorValEqual(a, b *sheetparts.ColorVal) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.RGB == b.RGB && a.Tint == b.Tint
}

func (c *converter) getOrAddFill(fill sheetparts.Fill) int {
	for i, f := range c.pkg.Styles.Fills {
		if fillsEqual(f, fill) {
			return i
		}
	}
	idx := len(c.pkg.Styles.Fills)
	c.pkg.Styles.Fills = append(c.pkg.Styles.Fills, fill)
	return idx
}

func fillsEqual(a, b sheetparts.Fill) bool {
	if a.PatternFill == nil || b.PatternFill == nil {
		return a.PatternFill == b.PatternFill
	}
	return a.PatternFill.PatternType == b.PatternFill.PatternType &&
		colorValEqual(a.PatternFill.FgColor, b.PatternFill.FgColor) &&
		colorValEqual(a.PatternFill.BgColor, b.PatternFill.BgColor)
}

func (c *converter) getOrAddBorder(border sheetparts.Border) int {
	for i, b := range c.pkg.Styles.Borders {
		if bordersEqual(b, border) {
			return i
		}
	}
	idx := len(c.pkg.Styles.Borders)
	c.pkg.Styles.Borders = append(c.pkg.Styles.Borders, border)
	return idx
}

func bordersEqual(a, b sheetparts.Border) bool {
	return borderSideEqual(a.Left, b.Left) &&
		borderSideEqual(a.Right, b.Right) &&
		borderSideEqual(a.Top, b.Top) &&
		borderSideEqual(a.Bottom, b.Bottom) &&
		borderSideEqual(a.Diagonal, b.Diagonal) &&
		borderSideEqual(a.Vertical, b.Vertical) &&
		borderSideEqual(a.Horizontal, b.Horizontal) &&
		a.DiagonalUp == b.DiagonalUp &&
		a.DiagonalDown == b.DiagonalDown
}

func borderSideEqual(a, b *sheetparts.BorderSide) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Style == b.Style && colorValEqual(a.Color, b.Color)
}

func (c *converter) getOrAddNumFmt(code string) int {
	for _, nf := range c.pkg.Styles.NumFmts {
		if nf.FormatCode == code {
			return nf.ID
		}
	}
	// Built-in formats: 0-163 are reserved. Use IDs starting from 164.
	id := 164 + len(c.pkg.Styles.NumFmts)
	c.pkg.Styles.NumFmts = append(c.pkg.Styles.NumFmts, sheetparts.NumFmt{
		ID:         id,
		FormatCode: code,
	})
	return id
}

func (c *converter) getOrAddXf(xf sheetparts.Xf) int {
	for i, existing := range c.pkg.Styles.CellXfs {
		if xfsEqual(existing, xf) {
			return i
		}
	}
	idx := len(c.pkg.Styles.CellXfs)
	c.pkg.Styles.CellXfs = append(c.pkg.Styles.CellXfs, xf)
	return idx
}

func xfsEqual(a, b sheetparts.Xf) bool {
	return a.NumFmtID == b.NumFmtID &&
		a.FontID == b.FontID &&
		a.FillID == b.FillID &&
		a.BorderID == b.BorderID &&
		a.XfID == b.XfID &&
		a.ApplyNumberFormat == b.ApplyNumberFormat &&
		a.ApplyFont == b.ApplyFont &&
		a.ApplyFill == b.ApplyFill &&
		a.ApplyBorder == b.ApplyBorder &&
		a.ApplyAlignment == b.ApplyAlignment &&
		a.ApplyProtection == b.ApplyProtection &&
		cellProtectionEqual(a.Protection, b.Protection) &&
		alignmentEqual(a.Alignment, b.Alignment)
}

func alignmentEqual(a, b *sheetparts.Alignment) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Horizontal == b.Horizontal &&
		a.Vertical == b.Vertical &&
		a.WrapText == b.WrapText &&
		a.ShrinkToFit == b.ShrinkToFit &&
		a.Indent == b.Indent &&
		a.TextRotation == b.TextRotation
}

func cellProtectionEqual(a, b *sheetparts.CellProtection) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Locked == b.Locked && a.FormulaHidden == b.FormulaHidden
}

// ── Merged Cells ─────────────────────────────────────────────────────────────

func (c *converter) mapMergedCells(sheet *sheetir.Sheet, ws *sheetparts.Worksheet) {
	for _, mc := range sheet.MergedCells {
		ref := fmt.Sprintf("%s%d:%s%d",
			colName(mc.LeftCol), mc.TopRow,
			colName(mc.RightCol), mc.BottomRow)
		ws.MergeCells = append(ws.MergeCells, sheetparts.MergeCell{Ref: ref})
	}
}

// ── Hyperlinks ───────────────────────────────────────────────────────────────

func (c *converter) mapHyperlinks(sheet *sheetir.Sheet, ws *sheetparts.Worksheet) map[string]string {
	hyperlinkURLs := make(map[string]string)
	nextRelID := 1
	appendHyperlink := func(ref, url, display, tooltip string) {
		ref = strings.TrimSpace(ref)
		if ref == "" || url == "" {
			return
		}
		if strings.HasPrefix(url, "#") {
			ws.Hyperlinks = append(ws.Hyperlinks, sheetparts.Hyperlink{
				Ref: ref, Location: url[1:], Display: display, Tooltip: tooltip,
			})
			return
		}
		relID := fmt.Sprintf("rIdHyperlink%d", nextRelID)
		nextRelID++
		ws.Hyperlinks = append(ws.Hyperlinks, sheetparts.Hyperlink{
			Ref: ref, RelID: relID, Display: display, Tooltip: tooltip,
		})
		hyperlinkURLs[relID] = url
	}

	// Prefer authored hyperlink records so range refs, display text, and tooltip
	// metadata survive round-trip. HyperlinkURL on cells remains the compatibility
	// view used by editors and older parsers.
	if len(sheet.Hyperlinks) > 0 {
		for _, hl := range sheet.Hyperlinks {
			if hl != nil {
				appendHyperlink(hl.Ref, hl.URL, hl.Display, hl.Tooltip)
			}
		}
	} else {
		for _, row := range sheet.Rows {
			for _, cell := range row.Cells {
				if cell.HyperlinkURL == "" {
					continue
				}
				ref := cell.Ref
				if ref == "" {
					ref = cellRef(cell.Row, cell.Col)
				}
				appendHyperlink(ref, cell.HyperlinkURL, "", "")
			}
		}
	}
	if len(hyperlinkURLs) == 0 {
		return nil
	}
	return hyperlinkURLs
}

func (c *converter) mapAutoFilter(sheet *sheetir.Sheet, ws *sheetparts.Worksheet) {
	if sheet.AutoFilterRef == "" {
		return
	}
	filter := &sheetparts.AutoFilter{Ref: sheet.AutoFilterRef}
	for _, fc := range sheet.FilterColumns {
		if fc == nil {
			continue
		}
		pfc := sheetparts.FilterColumn{ColID: fc.ColIndex}
		if len(fc.Filters) > 0 || fc.ShowBlanks {
			pfc.Filters = &sheetparts.FilterList{Blank: fc.ShowBlanks}
			for _, val := range fc.Filters {
				pfc.Filters.Filters = append(pfc.Filters.Filters, sheetparts.Filter{Val: val})
			}
		}
		if len(fc.CustomFilters) > 0 {
			pfc.CustomFilters = &sheetparts.CustomFilters{And: fc.CustomAnd}
			for _, cf := range fc.CustomFilters {
				if cf == nil {
					continue
				}
				pfc.CustomFilters.Filter = append(pfc.CustomFilters.Filter, sheetparts.CustomFilter{
					Operator: cf.Operator,
					Val:      cf.Val,
				})
			}
		}
		if fc.Top10 != nil {
			pfc.Top10 = &sheetparts.FilterTop10{
				Top:     fc.Top10.Top,
				Percent: fc.Top10.Percent,
				Val:     fc.Top10.Val,
			}
		}
		if fc.Dynamic != nil && fc.Dynamic.Type != "" {
			pfc.DynamicFilter = &sheetparts.DynamicFilter{
				Type:      fc.Dynamic.Type,
				Val:       cloneFloat64Ptr(fc.Dynamic.Val),
				MaxVal:    cloneFloat64Ptr(fc.Dynamic.MaxVal),
				ValISO:    fc.Dynamic.ValISO,
				MaxValISO: fc.Dynamic.MaxValISO,
			}
		}
		filter.FilterColumns = append(filter.FilterColumns, pfc)
	}
	if sheet.SortState != nil {
		sortState := &sheetparts.SortState{
			Ref:           sheet.SortState.Ref,
			CaseSensitive: sheet.SortState.CaseSensitive,
		}
		for _, condition := range sheet.SortState.Conditions {
			if condition == nil {
				continue
			}
			sortState.Conditions = append(sortState.Conditions, sheetparts.SortCondition{
				Ref:        condition.Ref,
				Descending: condition.Descending,
				SortBy:     condition.SortBy,
				CustomList: condition.CustomList,
				DxfID:      cloneIntPtr(condition.DxfID),
				IconSet:    condition.IconSet,
				IconID:     cloneIntPtr(condition.IconID),
			})
		}
		filter.SortState = sortState
	}
	ws.AutoFilter = filter
}

// ── Page Setup ───────────────────────────────────────────────────────────────

func (c *converter) mapPageSetup(sheet *sheetir.Sheet, ws *sheetparts.Worksheet) {
	if sheet.PageSetup != nil {
		ps := sheet.PageSetup
		ws.PageSetup = &sheetparts.PageSetup{
			PaperSize:          ps.PaperSize,
			Orientation:        ps.Orientation,
			Scale:              ps.Scale,
			FitToPage:          ps.FitToPage,
			FitToWidth:         ps.FitToWidth,
			FitToHeight:        ps.FitToHeight,
			FirstPageNumber:    ps.FirstPageNumber,
			UseFirstPageNumber: ps.UseFirstPageNumber,
			PageOrder:          ps.PageOrder,
			BlackAndWhite:      ps.BlackAndWhite,
			Draft:              ps.Draft,
			HorizontalDPI:      ps.HorizontalDPI,
			VerticalDPI:        ps.VerticalDPI,
			Copies:             ps.Copies,
		}
		ws.PageMargins = &sheetparts.PageMargins{
			Left:   ps.MarginLeft,
			Right:  ps.MarginRight,
			Top:    ps.MarginTop,
			Bottom: ps.MarginBottom,
			Header: ps.MarginHeader,
			Footer: ps.MarginFooter,
		}
		if ps.CenterHorizontally || ps.CenterVertically || ps.PrintHeadings || ps.PrintGridlines {
			ws.PrintOptions = &sheetparts.PrintOptions{
				HorizontalCentered: ps.CenterHorizontally,
				VerticalCentered:   ps.CenterVertically,
				Headings:           ps.PrintHeadings,
				GridLines:          ps.PrintGridlines,
			}
		}
	}
	if sheet.HeaderFooter != nil {
		hf := sheet.HeaderFooter
		ws.HeaderFooter = &sheetparts.HeaderFooter{
			DifferentOddEven: hf.DifferentOddEven,
			DifferentFirst:   hf.DifferentFirst,
			ScaleWithDoc:     hf.ScaleWithDoc,
			AlignWithMargins: hf.AlignWithMargins,
			OddHeader:        hf.OddHeader,
			OddFooter:        hf.OddFooter,
			EvenHeader:       hf.EvenHeader,
			EvenFooter:       hf.EvenFooter,
			FirstHeader:      hf.FirstHeader,
			FirstFooter:      hf.FirstFooter,
		}
	}
	if len(sheet.RowPageBreaks) > 0 {
		ws.RowBreaks = buildRowPageBreaks(sheet.RowPageBreaks)
	}
	if len(sheet.ColPageBreaks) > 0 {
		ws.ColBreaks = buildColumnPageBreaks(sheet.ColPageBreaks)
	}
}

func buildRowPageBreaks(rows []int) *sheetparts.PageBreaks {
	breaks := &sheetparts.PageBreaks{
		Breaks: make([]sheetparts.PageBreak, 0, len(rows)),
	}
	for _, row := range rows {
		if row < 1 {
			continue
		}
		breaks.Breaks = append(breaks.Breaks, sheetparts.PageBreak{
			ID:     row,
			Min:    0,
			Max:    16383,
			Manual: true,
		})
	}
	if len(breaks.Breaks) == 0 {
		return nil
	}
	breaks.Count = len(breaks.Breaks)
	breaks.ManualBreakCount = len(breaks.Breaks)
	return breaks
}

func buildColumnPageBreaks(cols []int) *sheetparts.PageBreaks {
	breaks := &sheetparts.PageBreaks{
		Breaks: make([]sheetparts.PageBreak, 0, len(cols)),
	}
	for _, col := range cols {
		if col < 1 {
			continue
		}
		breaks.Breaks = append(breaks.Breaks, sheetparts.PageBreak{
			ID:     col,
			Min:    0,
			Max:    1048575,
			Manual: true,
		})
	}
	if len(breaks.Breaks) == 0 {
		return nil
	}
	breaks.Count = len(breaks.Breaks)
	breaks.ManualBreakCount = len(breaks.Breaks)
	return breaks
}

func (c *converter) mapSheetProtection(sheet *sheetir.Sheet, ws *sheetparts.Worksheet) {
	if sheet == nil || sheet.Protection == nil {
		return
	}
	sp := sheet.Protection
	ws.SheetProtection = &sheetparts.SheetProtection{
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

// ── Data Validations ─────────────────────────────────────────────────────────

func (c *converter) mapDataValidations(sheet *sheetir.Sheet, ws *sheetparts.Worksheet) {
	for _, dv := range sheet.DataValidations {
		pdv := sheetparts.DataValidation{
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
		}
		if len(dv.ListValues) > 0 {
			pdv.Formula1 = inlineValidationListFormula(dv.ListValues)
		}
		ws.DataValidations = append(ws.DataValidations, pdv)
	}
}

func inlineValidationListFormula(values []string) string {
	escaped := make([]string, 0, len(values))
	for _, value := range values {
		escaped = append(escaped, strings.ReplaceAll(value, `"`, `""`))
	}
	return `"` + strings.Join(escaped, ",") + `"`
}

// ── Conditional Formatting ───────────────────────────────────────────────────

func (c *converter) mapConditionalFormatting(sheet *sheetir.Sheet, ws *sheetparts.Worksheet) {
	for _, cfr := range sheet.CFRules {
		pcf := sheetparts.CFRule{
			Type:       cfr.Type,
			Operator:   cfr.Operator,
			Priority:   cfr.Priority,
			Percent:    cfr.Percent,
			Bottom:     cfr.Bottom,
			Rank:       cfr.Rank,
			Text:       cfr.Text,
			TimePeriod: cfr.TimePeriod,
			Formula:    append([]string(nil), cfr.Formula...),
		}

		if dxfID := c.addDxfFromCFRule(cfr); dxfID >= 0 {
			pcf.DxfID = dxfID
		}

		if cfr.DataBar != nil {
			pcf.DataBar = &sheetparts.DataBar{
				MinLength: cfr.DataBar.MinLength,
				MaxLength: cfr.DataBar.MaxLength,
				ShowValue: cfr.DataBar.ShowValue,
			}
			if cfr.DataBar.Color != "" {
				pcf.DataBar.Color = &sheetparts.ColorVal{RGB: cfr.DataBar.Color}
			}
			for _, cfvo := range cfr.DataBar.Cfvo {
				pcf.DataBar.Cfvo = append(pcf.DataBar.Cfvo, sheetparts.Cfvo{
					Type: cfvo.Type,
					Val:  cfvo.Val,
				})
			}
		}

		if cfr.IconSet != nil {
			pcf.IconSet = &sheetparts.IconSet{
				Set:       cfr.IconSet.Set,
				ShowValue: cfr.IconSet.ShowValue,
				Reverse:   cfr.IconSet.Reverse,
			}
			for _, cfvo := range cfr.IconSet.Cfvo {
				pcf.IconSet.Cfvo = append(pcf.IconSet.Cfvo, sheetparts.Cfvo{
					Type: cfvo.Type,
					Val:  cfvo.Val,
				})
			}
		}

		if cfr.ColorScale != nil {
			pcf.ColorScale = &sheetparts.ColorScale{}
			for _, cfvo := range cfr.ColorScale.Cfvo {
				pcf.ColorScale.Cfvo = append(pcf.ColorScale.Cfvo, sheetparts.Cfvo{
					Type: cfvo.Type,
					Val:  cfvo.Val,
				})
			}
			for _, color := range cfr.ColorScale.Colors {
				pcf.ColorScale.Colors = append(pcf.ColorScale.Colors, sheetparts.ColorScaleColor{
					RGB: color,
				})
			}
		}

		ws.ConditionalFmts = append(ws.ConditionalFmts, sheetparts.ConditionalFmt{
			SqRef: cfr.SqRef,
			Rules: []sheetparts.CFRule{pcf},
		})
	}
}

func (c *converter) addDxfFromCFRule(cfr *sheetir.CFRule) int {
	ds := cfr.DxfStyle
	if ds.Bold == nil && ds.Italic == nil && ds.Underline == nil && ds.Strike == nil &&
		ds.FontColor == "" && ds.BgColor == "" {
		return -1
	}

	dxf := sheetparts.Dxf{}
	if ds.Bold != nil || ds.Italic != nil || ds.Underline != nil || ds.Strike != nil || ds.FontColor != "" {
		dxf.Font = &sheetparts.DxfFont{}
		if ds.Bold != nil && *ds.Bold {
			dxf.Font.Bold = &struct{}{}
		}
		if ds.Italic != nil && *ds.Italic {
			dxf.Font.Italic = &struct{}{}
		}
		if ds.Underline != nil && *ds.Underline {
			dxf.Font.Underline = &struct{}{}
		}
		if ds.Strike != nil && *ds.Strike {
			dxf.Font.Strike = &struct{}{}
		}
		if ds.FontColor != "" {
			dxf.Font.Color = &sheetparts.ColorVal{RGB: ds.FontColor}
		}
	}
	if ds.BgColor != "" {
		dxf.Fill = &sheetparts.DxfFill{
			PatternFill: &sheetparts.PatternFill{
				PatternType: "solid",
				FgColor:     &sheetparts.ColorVal{RGB: ds.BgColor},
			},
		}
	}

	idx := len(c.pkg.Styles.Dxfs)
	c.pkg.Styles.Dxfs = append(c.pkg.Styles.Dxfs, dxf)
	return idx
}

// ── Panes ────────────────────────────────────────────────────────────────────

func (c *converter) mapPanes(sheet *sheetir.Sheet, ws *sheetparts.Worksheet) {
	var pane *sheetparts.Pane
	state := sheet.PaneState
	if state == "" {
		if sheet.FrozenRows > 0 || sheet.FrozenCols > 0 {
			state = "frozen"
		} else if sheet.SplitRows > 0 || sheet.SplitCols > 0 || sheet.SplitX != 0 || sheet.SplitY != 0 {
			state = "split"
		}
	}
	if state != "" {
		pane = &sheetparts.Pane{State: state}
		if sheet.SplitX != 0 || sheet.SplitY != 0 {
			pane.XSplit, pane.YSplit = sheet.SplitX, sheet.SplitY
		} else if state == "frozen" || state == "frozenSplit" {
			pane.XSplit, pane.YSplit = float64(sheet.FrozenCols), float64(sheet.FrozenRows)
		} else {
			pane.XSplit, pane.YSplit = float64(sheet.SplitCols), float64(sheet.SplitRows)
		}
		pane.TopLeftCell = sheet.TopLeftCell
		pane.ActivePane = sheet.ActivePane
		if pane.TopLeftCell == "" && (state == "frozen" || state == "frozenSplit") {
			pane.TopLeftCell = cellRef(int(pane.YSplit)+1, int(pane.XSplit)+1)
		}
	}
	if pane == nil && sheet.ZoomScale == 0 && sheet.ShowGridLines == nil && len(sheet.PaneSelections) == 0 {
		return
	}
	view := sheetparts.SheetView{WorkbookViewID: 0, ZoomScale: sheet.ZoomScale, ShowGridLines: sheet.ShowGridLines, Pane: pane}
	for _, sel := range sheet.PaneSelections {
		view.Selections = append(view.Selections, sheetparts.Selection{Pane: sel.Pane, ActiveCell: sel.ActiveCell, ActiveCellID: sel.ActiveCellID, SqRef: sel.SqRef})
	}
	ws.SheetViews = append(ws.SheetViews, view)
}

// ── Comments ─────────────────────────────────────────────────────────────────

func (c *converter) mapComments(sheet *sheetir.Sheet) *sheetparts.Comments {
	if len(sheet.Notes) == 0 && len(sheet.Comments) == 0 {
		return nil
	}
	comments := &sheetparts.Comments{}
	authorMap := make(map[string]int)
	appendComment := func(ref, author, text string, rich []*sheetir.RichTextRun) {
		authorID, ok := authorMap[author]
		if !ok {
			authorID = len(comments.Authors)
			comments.Authors = append(comments.Authors, author)
			authorMap[author] = authorID
		}
		rt := sheetparts.RichText{PlainText: text}
		if len(rich) > 0 {
			if converted := c.convertRichText(rich); converted != nil {
				rt = *converted
			}
		}
		comments.CommentList = append(comments.CommentList, sheetparts.CommentEntry{
			Ref: ref, AuthorID: authorID, Text: rt,
		})
	}
	if len(sheet.Notes) > 0 {
		refs := make([]string, 0, len(sheet.Notes))
		for ref := range sheet.Notes {
			refs = append(refs, ref)
		}
		sort.Strings(refs)
		for _, ref := range refs {
			note := sheet.Notes[ref]
			if note == nil {
				continue
			}
			text := note.Text
			if text == "" && len(note.RichText) > 0 {
				var b strings.Builder
				for _, run := range note.RichText {
					if run != nil {
						b.WriteString(run.Text)
					}
				}
				text = b.String()
			}
			appendComment(ref, note.Author, text, note.RichText)
		}
		return comments
	}
	refs := make([]string, 0, len(sheet.Comments))
	for ref := range sheet.Comments {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	for _, ref := range refs {
		appendComment(ref, "", sheet.Comments[ref], nil)
	}
	return comments
}

func (c *converter) mapFormControls(sheet *sheetir.Sheet) ([]sheetparts.FormControl, []byte) {
	if sheet == nil {
		return nil, nil
	}

	// Start from the original VML part when available. VML is extensible and the
	// structured model intentionally covers only the worksheet objects we can edit.
	// Keeping the source part here prevents unrelated/unknown VML from disappearing
	// merely because the workbook passed through the canonical IR.
	legacyDrawingVML := append([]byte(nil), sheet.LegacyDrawingVML...)

	if len(sheet.VMLObjects) > 0 {
		objects := make([]sheetparts.VMLObject, 0, len(sheet.VMLObjects))
		for _, obj := range sheet.VMLObjects {
			if obj == nil {
				continue
			}
			objects = append(objects, sheetparts.VMLObject{
				ID:          obj.ID,
				Type:        obj.Type,
				Style:       obj.Style,
				FillColor:   obj.FillColor,
				StrokeColor: obj.StrokeColor,
				Text:        obj.Text,
				Anchor:      obj.Anchor,
			})
		}
		if len(objects) > 0 {
			legacyDrawingVML = sheetparts.MergeLegacyDrawingVMLObjects(legacyDrawingVML, objects)
		}
	}

	if len(sheet.FormControls) == 0 {
		return nil, legacyDrawingVML
	}
	controls := make([]sheetparts.FormControl, 0, len(sheet.FormControls))
	for _, control := range sheet.FormControls {
		if control == nil {
			continue
		}
		controls = append(controls, sheetparts.FormControl{
			Type:          control.Type,
			AltText:       control.AltText,
			Anchor:        control.Anchor,
			Checked:       control.Checked,
			Mixed:         control.Mixed,
			FirstButton:   control.FirstButton,
			FmlaLink:      control.FmlaLink,
			FmlaGroup:     control.FmlaGroup,
			FmlaRange:     control.FmlaRange,
			SelectionType: control.SelectionType,
			Selected:      append([]int(nil), control.Selected...),
			Min:           control.Min,
			Max:           control.Max,
			Val:           control.Val,
			Increment:     control.Increment,
			PageStep:      control.PageStep,
			Horizontal:    control.Horizontal,
			DropLines:     control.DropLines,
		})
	}
	if len(controls) == 0 {
		return nil, legacyDrawingVML
	}
	return controls, sheetparts.MergeLegacyDrawingVML(legacyDrawingVML, controls)
}

// ── Drawings ─────────────────────────────────────────────────────────────────

func (c *converter) mapDrawings(sheet *sheetir.Sheet) (*sheetparts.SpreadsheetDrawing, map[string]media.Part, map[string][]byte, map[string]sheetparts.PreservedPart, error) {
	if len(sheet.Drawings) == 0 {
		return nil, nil, nil, nil, nil
	}
	sd := &sheetparts.SpreadsheetDrawing{}
	drawingMedia := make(map[string]media.Part)
	drawingCharts := make(map[string][]byte)
	diagramParts := make(map[string]sheetparts.PreservedPart)

	for _, dr := range sheet.Drawings {
		switch dr.Type {
		case sheetir.DrawingTypeImage:
			if dr.Image == nil {
				continue
			}
			c.addImageDrawing(sd, drawingMedia, dr)
		case sheetir.DrawingTypeShape, sheetir.DrawingTypeConnector:
			if dr.Shape == nil {
				continue
			}
			c.addShapeDrawing(sd, dr)
		case sheetir.DrawingTypeGroup:
			if dr.Group == nil {
				continue
			}
			if err := c.addGroupDrawing(sd, drawingMedia, drawingCharts, dr); err != nil {
				return nil, nil, nil, nil, err
			}
		case sheetir.DrawingTypeChart:
			if dr.Chart == nil {
				continue
			}
			if err := c.addChartDrawing(sd, drawingCharts, dr); err != nil {
				return nil, nil, nil, nil, err
			}
		case sheetir.DrawingTypeSmartArt:
			if dr.SmartArt == nil {
				continue
			}
			c.addSmartArtDrawing(sd, diagramParts, dr)
		}
	}

	if len(sd.OneCellAnchors) == 0 && len(sd.TwoCellAnchors) == 0 && len(sd.AbsoluteAnchors) == 0 {
		return nil, nil, nil, nil, nil
	}
	if len(drawingMedia) == 0 {
		drawingMedia = nil
	}
	if len(drawingCharts) == 0 {
		drawingCharts = nil
	}
	if len(diagramParts) == 0 {
		diagramParts = nil
	}
	return sd, drawingMedia, drawingCharts, diagramParts, nil
}

func (c *converter) addSmartArtDrawing(sd *sheetparts.SpreadsheetDrawing, partsOut map[string]sheetparts.PreservedPart, dr *sheetir.Drawing) {
	sa := dr.SmartArt
	if sa == nil {
		return
	}
	frame := &sheetparts.XDRGraphicFrame{Graphic: sheetparts.XDRGraphic{GraphicData: sheetparts.XDRGraphicData{URI: "http://schemas.openxmlformats.org/drawingml/2006/diagram", RelIDs: &sheetparts.XDRDiagramRelIDs{Data: sa.DataRelID, Layout: sa.LayoutRelID, QuickStyle: sa.QuickStyleRelID, Colors: sa.ColorsRelID}}}}
	frame.NvGraphicFramePr.CNvPr.Name = dr.Name
	for id, part := range sa.Parts {
		copyPart := part
		if sa.SemanticDirty && id == sa.DataRelID {
			copyPart = regenerateSmartArtDataPart(sa, copyPart)
		}
		partsOut[id] = convertObjectPart(copyPart)
	}
	switch dr.Position.Anchor {
	case sheetir.AnchorOneCell:
		sd.OneCellAnchors = append(sd.OneCellAnchors, sheetparts.XDROneCellAnchor{ClientData: drawingClientData(dr.Position), From: c.makeMarker(dr.Position.FromCell), Ext: sheetparts.XDRExtent{CX: dr.Position.Width, CY: dr.Position.Height}, GraphicFrame: frame})
	case sheetir.AnchorTwoCell:
		sd.TwoCellAnchors = append(sd.TwoCellAnchors, sheetparts.XDRTwoCellAnchor{EditAs: drawingEditAs(dr.Position.Behavior), ClientData: drawingClientData(dr.Position), From: c.makeMarker(dr.Position.FromCell), To: c.makeMarker(dr.Position.ToCell), GraphicFrame: frame})
	case sheetir.AnchorAbsolute:
		sd.AbsoluteAnchors = append(sd.AbsoluteAnchors, sheetparts.XDRAbsoluteAnchor{ClientData: drawingClientData(dr.Position), Pos: sheetparts.XDRPos{X: dr.Position.X, Y: dr.Position.Y}, Ext: sheetparts.XDRExtent{CX: dr.Position.Width, CY: dr.Position.Height}, GraphicFrame: frame})
	}
}

func regenerateSmartArtDataPart(sa *sheetir.DrawingSmartArt, part sheetir.DrawingObjectPart) sheetir.DrawingObjectPart {
	if sa == nil {
		return part
	}
	diagram, _ := smartart.ParseDiagram(part.Data)
	if diagram == nil {
		diagram = &smartart.Diagram{}
	}
	if diagram.Layout == nil {
		diagram.Layout = &smartart.Layout{}
	}
	if sa.DiagramType != "" {
		diagram.Layout.Type = sa.DiagramType
	}

	original := make(map[string]*smartart.Node, len(diagram.Nodes))
	for _, node := range diagram.Nodes {
		if node != nil {
			original[node.ID] = node
		}
	}
	nodes := make([]*smartart.Node, 0, len(sa.Nodes))
	for _, source := range sa.Nodes {
		if source == nil {
			continue
		}
		var node *smartart.Node
		if existing := original[source.ID]; existing != nil {
			clone := *existing
			node = &clone
			if node.Text != source.Text {
				node.Paragraphs = nil
			}
		} else {
			node = &smartart.Node{ID: source.ID}
		}
		node.ID = source.ID
		node.Type = source.Type
		node.Text = source.Text
		node.TextColor = source.TextColor
		if source.FillColor != "" {
			node.Fill = drawing.SolidFill{Color: source.FillColor}
		}
		node.Parent = nil
		node.Children = nil
		nodes = append(nodes, node)
	}
	diagram.Nodes = nodes
	diagram.Connections = make([]smartart.Connection, 0, len(sa.Connections))
	for _, cxn := range sa.Connections {
		if cxn.SrcID == "" || cxn.DestID == "" {
			continue
		}
		diagram.Connections = append(diagram.Connections, smartart.Connection{SrcID: cxn.SrcID, DestID: cxn.DestID})
	}
	part.Data = smartart.MarshalDataModel(diagram)

	// The generated diagram drawing mirrors an earlier data-model layout. Once
	// the semantic model changes it is safer to let Excel regenerate that drawing
	// than to preserve stale geometry that no longer matches the node tree.
	filteredRels := part.Relationships[:0]
	for _, rel := range part.Relationships {
		if isDiagramDrawingRelationship(rel.Type) {
			continue
		}
		filteredRels = append(filteredRels, rel)
	}
	part.Relationships = filteredRels
	for name, child := range part.Children {
		if isDiagramDrawingRelationship(child.RelType) {
			delete(part.Children, name)
		}
	}
	if len(part.Children) == 0 {
		part.Children = nil
	}
	return part
}

func isDiagramDrawingRelationship(relType string) bool {
	return strings.Contains(strings.ToLower(relType), "diagramdrawing")
}

func convertObjectPart(part sheetir.DrawingObjectPart) sheetparts.PreservedPart {
	out := sheetparts.PreservedPart{RelID: part.RelID, RelType: part.RelType, PartName: part.PartName, ContentType: part.ContentType, Target: part.Target, Data: append([]byte(nil), part.Data...), External: part.External}
	for _, rel := range part.Relationships {
		out.Relationships = append(out.Relationships, sheetparts.PreservedRelationship{ID: rel.ID, Type: rel.Type, Target: rel.Target, TargetMode: rel.TargetMode})
	}
	if len(part.Children) > 0 {
		out.Children = map[string]sheetparts.PreservedPart{}
		for name, child := range part.Children {
			out.Children[name] = convertObjectPart(child)
		}
	}
	return out
}

func mapOLEObjects(sheet *sheetir.Sheet) (*sheetparts.OLEObjects, map[string]sheetparts.PreservedPart) {
	if sheet == nil {
		return nil, nil
	}
	var objects []sheetparts.OLEObject
	preserved := map[string]sheetparts.PreservedPart{}
	for _, dr := range sheet.Drawings {
		if dr == nil || dr.Type != sheetir.DrawingTypeOLE || dr.OLE == nil {
			continue
		}
		o := dr.OLE
		relID := o.RelID
		if relID == "" {
			relID = fmt.Sprintf("rIdOLE%d", len(objects)+1)
		}
		objects = append(objects, sheetparts.OLEObject{ProgID: o.ProgID, Link: o.Link, ShapeID: o.ShapeID, RelID: relID})
		preserved[relID] = sheetparts.PreservedPart{RelID: relID, RelType: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject", Data: append([]byte(nil), o.Data...), ContentType: o.ContentType, External: o.External, Target: o.Link}
	}
	if len(objects) == 0 {
		return nil, nil
	}
	return &sheetparts.OLEObjects{Objects: objects}, preserved
}

func drawingClientData(position sheetir.DrawingPosition) *sheetparts.XDRClientData {
	if position.LocksWithSheet == nil && position.PrintsWithSheet == nil {
		return nil
	}
	return &sheetparts.XDRClientData{LocksWithSheet: position.LocksWithSheet, PrintsWithSheet: position.PrintsWithSheet}
}

func drawingEditAs(behavior sheetir.DrawingAnchorBehavior) string {
	switch behavior {
	case sheetir.AnchorBehaviorMoveOnly:
		return "oneCell"
	case sheetir.AnchorBehaviorFixed:
		return "absolute"
	case sheetir.AnchorBehaviorMoveAndSize:
		return "twoCell"
	default:
		return ""
	}
}

func (c *converter) addImageDrawing(sd *sheetparts.SpreadsheetDrawing, drawingMedia map[string]media.Part, dr *sheetir.Drawing) {
	pic := c.buildImageElement(drawingMedia, dr)

	switch dr.Position.Anchor {
	case sheetir.AnchorOneCell:
		anchor := sheetparts.XDROneCellAnchor{ClientData: drawingClientData(dr.Position),
			From: c.makeMarker(dr.Position.FromCell),
			Ext: sheetparts.XDRExtent{
				CX: dr.Position.Width,
				CY: dr.Position.Height,
			},
			Pic: pic,
		}
		sd.OneCellAnchors = append(sd.OneCellAnchors, anchor)
	case sheetir.AnchorTwoCell:
		anchor := sheetparts.XDRTwoCellAnchor{EditAs: drawingEditAs(dr.Position.Behavior), ClientData: drawingClientData(dr.Position),
			From: c.makeMarker(dr.Position.FromCell),
			To:   c.makeMarker(dr.Position.ToCell),
			Pic:  pic,
		}
		sd.TwoCellAnchors = append(sd.TwoCellAnchors, anchor)
	case sheetir.AnchorAbsolute:
		anchor := sheetparts.XDRAbsoluteAnchor{ClientData: drawingClientData(dr.Position),
			Pos: sheetparts.XDRPos{X: dr.Position.X, Y: dr.Position.Y},
			Ext: sheetparts.XDRExtent{
				CX: dr.Position.Width,
				CY: dr.Position.Height,
			},
			Pic: pic,
		}
		sd.AbsoluteAnchors = append(sd.AbsoluteAnchors, anchor)
	}
}

func (c *converter) addShapeDrawing(sd *sheetparts.SpreadsheetDrawing, dr *sheetir.Drawing) {
	xdrShape := c.buildShapeElement(dr)

	switch dr.Position.Anchor {
	case sheetir.AnchorOneCell:
		anchor := sheetparts.XDROneCellAnchor{ClientData: drawingClientData(dr.Position),
			From:  c.makeMarker(dr.Position.FromCell),
			Ext:   sheetparts.XDRExtent{CX: dr.Position.Width, CY: dr.Position.Height},
			Shape: xdrShape,
		}
		sd.OneCellAnchors = append(sd.OneCellAnchors, anchor)
	case sheetir.AnchorTwoCell:
		anchor := sheetparts.XDRTwoCellAnchor{EditAs: drawingEditAs(dr.Position.Behavior), ClientData: drawingClientData(dr.Position),
			From:  c.makeMarker(dr.Position.FromCell),
			To:    c.makeMarker(dr.Position.ToCell),
			Shape: xdrShape,
		}
		sd.TwoCellAnchors = append(sd.TwoCellAnchors, anchor)
	case sheetir.AnchorAbsolute:
		anchor := sheetparts.XDRAbsoluteAnchor{ClientData: drawingClientData(dr.Position),
			Pos:   sheetparts.XDRPos{X: dr.Position.X, Y: dr.Position.Y},
			Ext:   sheetparts.XDRExtent{CX: dr.Position.Width, CY: dr.Position.Height},
			Shape: xdrShape,
		}
		sd.AbsoluteAnchors = append(sd.AbsoluteAnchors, anchor)
	}
}

func (c *converter) buildShapeElement(dr *sheetir.Drawing) *sheetparts.XDRShape {
	shape := dr.Shape
	spPr := sheetparts.XDRSpPr{}
	if shape.RotationDeg != 0 || shape.FlipH || shape.FlipV {
		spPr.Xfrm = &sheetparts.XDRTransform2D{Rot: int64(math.Round(shape.RotationDeg * 60000)), FlipH: shape.FlipH, FlipV: shape.FlipV}
	}

	geometry := strings.TrimSpace(shape.Geometry)
	if dr.Type == sheetir.DrawingTypeConnector {
		if shape.ConnectorType != "" {
			geometry = shape.ConnectorType
		} else if geometry == "" || strings.EqualFold(geometry, "line") {
			geometry = "straightConnector1"
		}
	}
	if geometry != "" {
		spPr.PrstGeom = &drawingxml.PresetGeometry{Prst: geometry}
	}

	if shape.FillColor != "" {
		spPr.SolidFill = &drawingxml.SolidFill{
			SRGBClr: &drawingxml.SRGBClr{Val: shape.FillColor},
		}
	}

	if line := drawingLineFromShape(shape); line != nil {
		spPr.Ln = line
	}
	if effects := worksheetEffectsFromShape(shape); effects != nil {
		spPr.EffectLst = effects
	}
	if threeD := worksheetThreeDFromShape(shape); threeD != nil {
		spPr.Scene3D = &sheetparts.XDRScene3D{
			Camera:   &sheetparts.XDRSceneCamera{Prst: threeD.CameraPreset},
			LightRig: &sheetparts.XDRSceneLightRig{Rig: threeD.LightRig, Dir: threeD.LightDirection},
		}
		spPr.Shape3D = &sheetparts.XDRShape3D{
			BevelT: &sheetparts.XDRShape3DBevel{
				W:    drawing.PointsToEMU(threeD.BevelTopWidth),
				H:    drawing.PointsToEMU(threeD.BevelTopHeight),
				Prst: threeD.BevelTopPreset,
			},
		}
	}

	xdrShape := &sheetparts.XDRShape{
		NvSpPr: sheetparts.XDRNvSpPr{
			CNvPr: sheetparts.XDRCNvPr{
				ID:   1,
				Name: dr.Name,
			},
		},
		SpPr: spPr,
	}
	if dr.Type == sheetir.DrawingTypeConnector {
		xdrShape.NvSpPr.CNvPr.Descr = "connector"
	}

	if len(shape.RichTextRuns) > 0 || shape.Text != "" {
		xdrShape.TxBody = &sheetparts.XDRTxBody{
			Paragraphs: []sheetparts.XDRTxParagraph{{Runs: c.convertShapeTextRuns(shape)}},
		}
	}
	return xdrShape
}

func (c *converter) addGroupDrawing(sd *sheetparts.SpreadsheetDrawing, drawingMedia map[string]media.Part, drawingCharts map[string][]byte, dr *sheetir.Drawing) error {
	group, err := c.buildGroupElement(dr, 0, 0, dr.Position.Width, dr.Position.Height, drawingMedia, drawingCharts)
	if err != nil {
		return err
	}
	switch dr.Position.Anchor {
	case sheetir.AnchorOneCell:
		anchor := sheetparts.XDROneCellAnchor{ClientData: drawingClientData(dr.Position),
			From:       c.makeMarker(dr.Position.FromCell),
			Ext:        sheetparts.XDRExtent{CX: dr.Position.Width, CY: dr.Position.Height},
			GroupShape: group,
		}
		sd.OneCellAnchors = append(sd.OneCellAnchors, anchor)
	case sheetir.AnchorTwoCell:
		anchor := sheetparts.XDRTwoCellAnchor{EditAs: drawingEditAs(dr.Position.Behavior), ClientData: drawingClientData(dr.Position),
			From:       c.makeMarker(dr.Position.FromCell),
			To:         c.makeMarker(dr.Position.ToCell),
			GroupShape: group,
		}
		sd.TwoCellAnchors = append(sd.TwoCellAnchors, anchor)
	case sheetir.AnchorAbsolute:
		anchor := sheetparts.XDRAbsoluteAnchor{ClientData: drawingClientData(dr.Position),
			Pos:        sheetparts.XDRPos{X: dr.Position.X, Y: dr.Position.Y},
			Ext:        sheetparts.XDRExtent{CX: dr.Position.Width, CY: dr.Position.Height},
			GroupShape: group,
		}
		sd.AbsoluteAnchors = append(sd.AbsoluteAnchors, anchor)
	}
	return nil
}

func (c *converter) buildGroupElement(dr *sheetir.Drawing, x, y, width, height int64, drawingMedia map[string]media.Part, drawingCharts map[string][]byte) (*sheetparts.XDRGroupShape, error) {
	group := &sheetparts.XDRGroupShape{
		NvGrpSpPr: &sheetparts.XDRNvGrpSpPr{
			CNvPr: sheetparts.XDRCNvPr{
				ID:   1,
				Name: dr.Name,
			},
		},
		GrpSpPr: &sheetparts.XDRGrpSpPr{
			Xfrm: groupTransform(x, y, width, height),
		},
	}
	if dr.Group == nil {
		return group, nil
	}
	for _, child := range dr.Group.Children {
		if child == nil {
			continue
		}
		switch child.Type {
		case sheetir.DrawingTypeImage:
			if child.Image == nil {
				continue
			}
			pic := c.buildImageElement(drawingMedia, child)
			pic.SpPr.Xfrm = childTransform(child.Position)
			group.Pictures = append(group.Pictures, *pic)
			group.Items = append(group.Items, sheetparts.XDRGroupItem{
				Kind:  sheetparts.XDRGroupItemKind("pic"),
				Index: len(group.Pictures) - 1,
			})
		case sheetir.DrawingTypeShape, sheetir.DrawingTypeConnector:
			if child.Shape == nil {
				continue
			}
			shape := c.buildShapeElement(child)
			shape.SpPr.Xfrm = childTransform(child.Position)
			group.Shapes = append(group.Shapes, *shape)
			group.Items = append(group.Items, sheetparts.XDRGroupItem{
				Kind:  sheetparts.XDRGroupItemKind("sp"),
				Index: len(group.Shapes) - 1,
			})
		case sheetir.DrawingTypeChart:
			if child.Chart == nil {
				continue
			}
			frame, err := c.buildChartElement(drawingCharts, child)
			if err != nil {
				return nil, err
			}
			frame.Xfrm = childTransform(child.Position)
			group.GraphicFrms = append(group.GraphicFrms, *frame)
			group.Items = append(group.Items, sheetparts.XDRGroupItem{
				Kind:  sheetparts.XDRGroupItemKind("graphicFrame"),
				Index: len(group.GraphicFrms) - 1,
			})
		case sheetir.DrawingTypeGroup:
			if child.Group == nil {
				continue
			}
			nested, err := c.buildGroupElement(child, child.Position.X, child.Position.Y, child.Position.Width, child.Position.Height, drawingMedia, drawingCharts)
			if err != nil {
				return nil, err
			}
			group.GroupShapes = append(group.GroupShapes, *nested)
			group.Items = append(group.Items, sheetparts.XDRGroupItem{
				Kind:  sheetparts.XDRGroupItemKind("grpSp"),
				Index: len(group.GroupShapes) - 1,
			})
		}
	}
	return group, nil
}

func groupTransform(x, y, width, height int64) *sheetparts.XDRTransform2D {
	return &sheetparts.XDRTransform2D{
		Off:   &sheetparts.XDROffset{X: x, Y: y},
		Ext:   &sheetparts.XDRExtent{CX: width, CY: height},
		ChOff: &sheetparts.XDROffset{X: 0, Y: 0},
		ChExt: &sheetparts.XDRExtent{CX: width, CY: height},
	}
}

func childTransform(position sheetir.DrawingPosition) *sheetparts.XDRTransform2D {
	return &sheetparts.XDRTransform2D{
		Off: &sheetparts.XDROffset{
			X: position.X,
			Y: position.Y,
		},
		Ext: &sheetparts.XDRExtent{
			CX: position.Width,
			CY: position.Height,
		},
	}
}

func (c *converter) buildImageElement(drawingMedia map[string]media.Part, dr *sheetir.Drawing) *sheetparts.XDRPic {
	relID := c.nextRID()
	if dr.Image.Data != nil {
		contentType := dr.Image.ContentType
		if contentType == "" {
			contentType = "image/png"
		}
		drawingMedia[relID] = media.Part{
			Name:        c.nextMediaName(contentType),
			Data:        dr.Image.Data,
			ContentType: contentType,
		}
	}
	pic := &sheetparts.XDRPic{
		NvPicPr: sheetparts.XDRNvPicPr{
			CNvPr: sheetparts.XDRCNvPr{
				ID:    1,
				Name:  dr.Name,
				Descr: dr.Image.AltText,
			},
		},
		BlipFill: sheetparts.XDRBlipFill{
			Blip: sheetparts.XDRBlip{Embed: relID},
		},
	}
	if effects := worksheetEffectsFromEffectList(dr.Image.Effects); effects != nil {
		pic.SpPr.EffectLst = effects
	}
	return pic
}

func worksheetEffectsFromShape(shape *sheetir.DrawingShape) *sheetparts.XDREffectList {
	if shape == nil || len(shape.Effects.Effects) == 0 {
		return nil
	}
	return worksheetEffectsFromEffectList(shape.Effects)
}

func worksheetEffectsFromEffectList(effectList drawing.EffectList) *sheetparts.XDREffectList {
	if len(effectList.Effects) == 0 {
		return nil
	}
	out := &sheetparts.XDREffectList{}
	for _, effect := range effectList.Effects {
		switch e := effect.(type) {
		case drawing.Shadow:
			out.OuterShadow = worksheetOuterShadow(e)
		case *drawing.Shadow:
			if e != nil {
				out.OuterShadow = worksheetOuterShadow(*e)
			}
		case drawing.SoftEdge:
			out.SoftEdge = &sheetparts.XDRSoftEdgeEffect{Rad: drawing.PointsToEMU(e.Radius)}
		case *drawing.SoftEdge:
			if e != nil {
				out.SoftEdge = &sheetparts.XDRSoftEdgeEffect{Rad: drawing.PointsToEMU(e.Radius)}
			}
		}
	}
	if out.OuterShadow == nil && out.InnerShadow == nil && out.Glow == nil && out.SoftEdge == nil && out.Reflection == nil && out.Blur == nil {
		return nil
	}
	return out
}

func worksheetThreeDFromShape(shape *sheetir.DrawingShape) *drawing.ThreeD {
	if shape == nil {
		return nil
	}
	for _, effect := range shape.Effects.Effects {
		switch threeD := effect.(type) {
		case drawing.ThreeD:
			return &threeD
		case *drawing.ThreeD:
			return threeD
		}
	}
	return nil
}

func worksheetOuterShadow(sh drawing.Shadow) *sheetparts.XDROuterShadowEffect {
	alpha := sh.Alpha
	if alpha <= 0 {
		alpha = 1
	}
	if alpha > 1 {
		alpha = 1
	}
	color := strings.TrimPrefix(strings.TrimSpace(sh.Color), "#")
	if color == "" {
		color = "808080"
	}
	return &sheetparts.XDROuterShadowEffect{
		BlurRad: drawing.PointsToEMU(sh.Blur),
		Dist:    drawing.PointsToEMU(sh.Distance),
		Dir:     int(math.Round(sh.Angle * 60000)),
		SrgbClr: &sheetparts.XDREffectColorRef{Val: color, Alpha: &sheetparts.XDREffectAlpha{Val: int(math.Round(alpha * 100000))}},
	}
}

func drawingLineFromShape(shape *sheetir.DrawingShape) *drawingxml.Line {
	if shape == nil {
		return nil
	}
	if shape.LineStyle == nil && shape.LineColor == "" && shape.LineWidthPt <= 0 {
		return nil
	}
	ln := &drawingxml.Line{}
	if shape.LineStyle != nil {
		if shape.LineStyle.Width > 0 {
			ln.W = drawing.PointsToEMU(shape.LineStyle.Width)
		}
		if shape.LineStyle.Color != "" {
			ln.SolidFill = &drawingxml.SolidFill{
				SRGBClr: &drawingxml.SRGBClr{Val: shape.LineStyle.Color},
			}
		}
		switch shape.LineStyle.Dash {
		case "", drawing.DashPatternSolid:
		default:
			ln.PrstDash = &drawingxml.PresetDash{Val: string(shape.LineStyle.Dash)}
		}
		switch shape.LineStyle.Cap {
		case drawing.LineCapRound:
			ln.Cap = "rnd"
		case drawing.LineCapSquare:
			ln.Cap = "sq"
		case drawing.LineCapFlat:
			ln.Cap = "flat"
		}
		switch shape.LineStyle.Join {
		case drawing.LineJoinRound:
			ln.Round = &drawingxml.LineRound{}
		case drawing.LineJoinBevel:
			ln.Bevel = &drawingxml.LineBevel{}
		case drawing.LineJoinMiter:
			ln.Miter = &drawingxml.LineMiter{Lim: int(drawing.PointsToEMU(shape.LineStyle.MiterLimit))}
		}
		if shape.LineStyle.CompoundType != "" {
			ln.Cmpd = shape.LineStyle.CompoundType
		}
		if shape.LineStyle.HeadEnd != nil && shape.LineStyle.HeadEnd.Type != drawing.ArrowheadNone {
			ln.HeadEnd = &drawingxml.LineEnd{
				Type: string(shape.LineStyle.HeadEnd.Type),
				W:    string(shape.LineStyle.HeadEnd.Size),
				Len:  string(shape.LineStyle.HeadEnd.Length),
			}
		}
		if shape.LineStyle.TailEnd != nil && shape.LineStyle.TailEnd.Type != drawing.ArrowheadNone {
			ln.TailEnd = &drawingxml.LineTailEnd{
				Type: string(shape.LineStyle.TailEnd.Type),
				W:    string(shape.LineStyle.TailEnd.Size),
				Len:  string(shape.LineStyle.TailEnd.Length),
			}
		}
	}
	if ln.W == 0 && shape.LineWidthPt > 0 {
		ln.W = drawing.PointsToEMU(shape.LineWidthPt)
	}
	if ln.SolidFill == nil && shape.LineColor != "" {
		ln.SolidFill = &drawingxml.SolidFill{
			SRGBClr: &drawingxml.SRGBClr{Val: shape.LineColor},
		}
	}
	return ln
}

func (c *converter) convertShapeTextRuns(shape *sheetir.DrawingShape) []sheetparts.XDRTxRun {
	if len(shape.RichTextRuns) == 0 {
		return []sheetparts.XDRTxRun{{T: shape.Text}}
	}
	runs := make([]sheetparts.XDRTxRun, 0, len(shape.RichTextRuns))
	for _, run := range shape.RichTextRuns {
		if run == nil {
			continue
		}
		xdrRun := sheetparts.XDRTxRun{T: run.Text}
		if run.Bold || run.Italic || run.Underline || run.Strike || run.VertAlign != "" ||
			run.FontSizePt > 0 || run.FontColor != "" || run.FontFamily != "" {
			props := &sheetparts.XDRRunProperties{}
			if run.Bold {
				props.B = "1"
			}
			if run.Italic {
				props.I = "1"
			}
			if run.Underline {
				props.U = drawingUnderlineType(run.UnderlineType)
			}
			switch run.VertAlign {
			case "superscript":
				props.Baseline = 30000
			case "subscript":
				props.Baseline = -25000
			}
			if run.Strike {
				props.StrikeType = "sngStrike"
			}
			if run.FontSizePt > 0 {
				props.SizeHundredths = int(math.Round(run.FontSizePt * 100))
			}
			if run.FontColor != "" {
				props.SolidFill = &drawingxml.SolidFill{SRGBClr: &drawingxml.SRGBClr{Val: run.FontColor}}
			}
			if run.FontFamily != "" {
				props.Latin = &sheetparts.XDRTextFont{Typeface: run.FontFamily}
			}
			xdrRun.Properties = props
		}
		runs = append(runs, xdrRun)
	}
	if len(runs) == 0 {
		return []sheetparts.XDRTxRun{{T: shape.Text}}
	}
	return runs
}

// drawingUnderlineType maps a SpreadsheetML underline variant onto ST_TextUnderlineType.
func drawingUnderlineType(underline string) string {
	switch underline {
	case "double", "doubleAccounting":
		return "dbl"
	default:
		return "sng"
	}
}

func (c *converter) addChartDrawing(sd *sheetparts.SpreadsheetDrawing, drawingCharts map[string][]byte, dr *sheetir.Drawing) error {
	gf, err := c.buildChartElement(drawingCharts, dr)
	if err != nil {
		return err
	}

	switch dr.Position.Anchor {
	case sheetir.AnchorOneCell:
		anchor := sheetparts.XDROneCellAnchor{ClientData: drawingClientData(dr.Position),
			From:         c.makeMarker(dr.Position.FromCell),
			Ext:          sheetparts.XDRExtent{CX: dr.Position.Width, CY: dr.Position.Height},
			GraphicFrame: gf,
		}
		sd.OneCellAnchors = append(sd.OneCellAnchors, anchor)
	case sheetir.AnchorTwoCell:
		anchor := sheetparts.XDRTwoCellAnchor{EditAs: drawingEditAs(dr.Position.Behavior), ClientData: drawingClientData(dr.Position),
			From:         c.makeMarker(dr.Position.FromCell),
			To:           c.makeMarker(dr.Position.ToCell),
			GraphicFrame: gf,
		}
		sd.TwoCellAnchors = append(sd.TwoCellAnchors, anchor)
	case sheetir.AnchorAbsolute:
		anchor := sheetparts.XDRAbsoluteAnchor{ClientData: drawingClientData(dr.Position),
			Pos:          sheetparts.XDRPos{X: dr.Position.X, Y: dr.Position.Y},
			Ext:          sheetparts.XDRExtent{CX: dr.Position.Width, CY: dr.Position.Height},
			GraphicFrame: gf,
		}
		sd.AbsoluteAnchors = append(sd.AbsoluteAnchors, anchor)
	}
	return nil
}

func (c *converter) buildChartElement(drawingCharts map[string][]byte, dr *sheetir.Drawing) (*sheetparts.XDRGraphicFrame, error) {
	relID := c.nextRID()
	chartData, err := marshalDrawingChart(dr.Chart.Chart)
	if err != nil {
		return nil, err
	}
	if len(chartData) == 0 && len(dr.Chart.RawXML) > 0 {
		chartData = append([]byte(nil), dr.Chart.RawXML...)
	}
	if len(chartData) > 0 {
		drawingCharts[relID] = chartData
	}
	return &sheetparts.XDRGraphicFrame{
		NvGraphicFramePr: sheetparts.XDRNvGraphicFramePr{
			CNvPr: sheetparts.XDRCNvPr{
				ID:    1,
				Name:  dr.Name,
				Title: dr.Chart.AltTitle,
				Descr: dr.Chart.AltText,
			},
		},
		Graphic: sheetparts.XDRGraphic{
			GraphicData: sheetparts.XDRGraphicData{
				URI:   "http://schemas.openxmlformats.org/drawingml/2006/chart",
				Chart: &sheetparts.XDRChart{RelID: relID},
			},
		},
	}, nil
}

func marshalDrawingChart(value interface{}) ([]byte, error) {
	switch c := value.(type) {
	case nil:
		return nil, nil
	case *chartmodel.Chart:
		return chartmodel.MarshalChart(c)
	case chartmodel.Chart:
		chart := c
		return chartmodel.MarshalChart(&chart)
	case []byte:
		return append([]byte(nil), c...), nil
	default:
		return nil, fmt.Errorf("unsupported chart payload %T", value)
	}
}

func (c *converter) mapTables(sheet *sheetir.Sheet) []*sheetparts.Table {
	if len(sheet.Tables) == 0 {
		return nil
	}
	out := make([]*sheetparts.Table, 0, len(sheet.Tables))
	for _, t := range sheet.Tables {
		pt := &sheetparts.Table{
			ID:             t.ID,
			Name:           t.Name,
			DisplayName:    t.DisplayName,
			Ref:            t.Ref,
			TotalsRowShown: t.TotalsRowShown,
			HeaderRowCount: t.HeaderRowCount,
			TotalsRowCount: t.TotalsRowCount,
		}
		if t.AutoFilterRef != "" {
			pt.AutoFilter = &sheetparts.TableAutoFilter{Ref: t.AutoFilterRef}
		}
		if t.TableStyle != nil {
			pt.TableStyleInfo = &sheetparts.TableStyleInfoXML{
				Name:              t.TableStyle.Name,
				ShowFirstColumn:   t.TableStyle.ShowFirstColumn,
				ShowLastColumn:    t.TableStyle.ShowLastColumn,
				ShowRowStripes:    t.TableStyle.ShowRowStripes,
				ShowColumnStripes: t.TableStyle.ShowColumnStripes,
			}
		}
		for _, col := range t.Columns {
			pt.Columns = append(pt.Columns, sheetparts.TableColumn{
				ID:                      col.ID,
				Name:                    col.Name,
				TotalsRowFunction:       col.TotalsRowFunction,
				TotalsRowLabel:          col.TotalsRowLabel,
				CalculatedColumnFormula: col.CalculatedColumnFormula,
				TotalsRowFormula:        col.TotalsRowFormula,
			})
		}
		out = append(out, pt)
	}
	return out
}

func (c *converter) makeMarker(anchor sheetir.DrawingCellAnchor) sheetparts.XDRMarker {
	return sheetparts.XDRMarker{
		Col:    anchor.Col - 1,
		ColOff: anchor.ColOffset,
		Row:    anchor.Row - 1,
		RowOff: anchor.RowOffset,
	}
}

// mediaExtension returns a file extension for common image content types.
func mediaExtension(contentType string) string {
	switch strings.ToLower(contentType) {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/bmp":
		return "bmp"
	case "image/tiff":
		return "tiff"
	case "image/webp":
		return "webp"
	case "image/svg+xml":
		return "svg"
	default:
		return "bin"
	}
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func cloneFloat64Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

// buildSheetFormatPr projects canonical worksheet-wide row/column layout
// metadata into OOXML.  Row/column records own the individual outline levels;
// sheetFormatPr owns the worksheet defaults and the maximum outline depths.
func buildSheetFormatPr(sheet *sheetir.Sheet) *sheetparts.SheetFormatPr {
	pr := &sheetparts.SheetFormatPr{}
	if sheet == nil {
		return pr
	}

	pr.DefaultRowHeight = sheet.DefaultRowHeightPt
	pr.DefaultColWidth = sheet.DefaultColWidthChars
	pr.CustomHeight = sheet.DefaultRowHeightUnsynced
	pr.ZeroHeight = sheet.DefaultRowHeightZeroHeight
	pr.OutlineLevelRow = maxOutlineLevel(sheet.RowOutlineLevelMax, sheet.OutlineRows)
	pr.OutlineLevelCol = maxOutlineLevel(sheet.ColOutlineLevelMax, sheet.OutlineCols)
	return pr
}

func maxOutlineLevel(authored int, levels map[int]int) int {
	maxLevel := authored
	for _, level := range levels {
		if level > maxLevel {
			maxLevel = level
		}
	}
	return maxLevel
}
