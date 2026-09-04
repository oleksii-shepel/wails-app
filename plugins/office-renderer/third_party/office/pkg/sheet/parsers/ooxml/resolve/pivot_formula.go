package resolve

import (
	"strconv"
	"strings"

	"office/pkg/sheet/parsers/ooxml/formula"
)

// LookupPivotData resolves GETPIVOTDATA against resolved workbook metadata and
// source-sheet rows. It uses source ranges or defined-name sources already
// present in the workbook model and optionally evaluates formula source cells
// through the provided engine lookup.
func LookupPivotData(wb *Workbook, currentSheet string, engineForSheet func(string) *formula.Engine, dataField, pivotTableRef string, pivotTableValue formula.Value, criteria []formula.PivotDataCriterion) (formula.Value, bool) {
	if wb == nil {
		return formula.Value{}, false
	}
	pivotSheet, pivot := findPivotTableForGetPivotData(wb, currentSheet, pivotTableRef, pivotTableValue)
	if pivot == nil {
		return formula.Value{}, false
	}
	sourceSheetName, sourceRef, ok := pivotSourceRangeForGetPivotData(wb, pivotSheet, pivot)
	if !ok {
		return formula.Value{}, false
	}
	sourceSheet := workbookSheetByName(wb, sourceSheetName)
	if sourceSheet == nil {
		return formula.Value{}, false
	}
	firstRow, firstCol, lastRow, lastCol, ok := parsePivotFormulaRange(sourceRef)
	if !ok || firstRow < 1 || firstCol < 1 || lastRow <= firstRow || lastCol < firstCol {
		return formula.Value{}, false
	}
	headers := buildPivotSourceHeaders(sourceSheet, firstRow, firstCol, lastCol, engineForSheet)
	if len(headers) == 0 {
		return formula.Value{}, false
	}
	dataCol, subtotal, ok := resolvePivotDataField(headers, pivot, dataField)
	if !ok {
		return formula.Value{}, false
	}
	filterCols, ok := resolvePivotCriteriaColumns(headers, criteria)
	if !ok {
		return formula.Value{}, false
	}
	cells := buildPivotCellLookup(sourceSheet)
	var eng *formula.Engine
	if engineForSheet != nil {
		eng = engineForSheet(sourceSheetName)
	}
	matchedRows := 0
	numericCount := 0
	sum := 0.0
	countA := 0
	minSet := false
	minVal := 0.0
	maxVal := 0.0
	product := 1.0
	productCount := 0
	for row := firstRow + 1; row <= lastRow; row++ {
		if !pivotRowMatchesCriteria(cells, eng, row, filterCols, criteria) {
			continue
		}
		matchedRows++
		value := pivotCellFormulaValue(cells, eng, row, dataCol)
		if !value.IsEmpty() {
			countA++
		}
		if num, err := value.ToNumber(); err == nil {
			numericCount++
			sum += num
			if !minSet || num < minVal {
				minVal = num
			}
			if !minSet || num > maxVal {
				maxVal = num
			}
			minSet = true
			product *= num
			productCount++
		}
	}
	if matchedRows == 0 {
		return formula.Value{}, false
	}
	switch strings.ToLower(strings.TrimSpace(subtotal)) {
	case "", "sum":
		return formula.Number(sum), true
	case "count":
		return formula.Number(float64(numericCount)), true
	case "counta":
		return formula.Number(float64(countA)), true
	case "average", "avg":
		if numericCount == 0 {
			return formula.Error(formula.ErrDiv0), true
		}
		return formula.Number(sum / float64(numericCount)), true
	case "min":
		if !minSet {
			return formula.Value{}, false
		}
		return formula.Number(minVal), true
	case "max":
		if !minSet {
			return formula.Value{}, false
		}
		return formula.Number(maxVal), true
	case "product":
		if productCount == 0 {
			return formula.Number(0), true
		}
		return formula.Number(product), true
	default:
		return formula.Number(sum), true
	}
}

func findPivotTableForGetPivotData(wb *Workbook, currentSheet, pivotTableRef string, pivotTableValue formula.Value) (string, *PivotTable) {
	if sheetName, cellRef, ok := parsePivotTableAnchorRef(currentSheet, pivotTableRef); ok {
		if sheet := workbookSheetByName(wb, sheetName); sheet != nil {
			for _, pivot := range sheet.PivotTables {
				if pivot != nil && pivotRangeContainsCell(pivot.LocationRef, cellRef) {
					return sheet.Name, pivot
				}
			}
		}
	}
	name := strings.TrimSpace(pivotTableValue.ToText())
	if name == "" {
		name = strings.Trim(strings.TrimSpace(pivotTableRef), `"`)
	}
	if name == "" {
		return "", nil
	}
	if sheet := workbookSheetByName(wb, currentSheet); sheet != nil {
		for _, pivot := range sheet.PivotTables {
			if pivot != nil && strings.EqualFold(strings.TrimSpace(pivot.Name), name) {
				return sheet.Name, pivot
			}
		}
	}
	for _, sheet := range wb.Sheets {
		if sheet == nil {
			continue
		}
		for _, pivot := range sheet.PivotTables {
			if pivot != nil && strings.EqualFold(strings.TrimSpace(pivot.Name), name) {
				return sheet.Name, pivot
			}
		}
	}
	return "", nil
}

func pivotSourceRangeForGetPivotData(wb *Workbook, pivotSheet string, pivot *PivotTable) (string, string, bool) {
	if pivot == nil || wb == nil {
		return "", "", false
	}
	if strings.TrimSpace(pivot.SourceSheet) != "" && strings.TrimSpace(pivot.SourceRef) != "" {
		return strings.TrimSpace(pivot.SourceSheet), strings.TrimSpace(pivot.SourceRef), true
	}
	if strings.TrimSpace(pivot.SourceName) == "" {
		return "", "", false
	}
	pivotSheetIndex := workbookSheetIndex(wb, pivotSheet)
	for _, dn := range wb.DefinedNames {
		if dn == nil || dn.Hidden || !strings.EqualFold(strings.TrimSpace(dn.Name), strings.TrimSpace(pivot.SourceName)) {
			continue
		}
		if dn.LocalSheetID != nil && *dn.LocalSheetID != pivotSheetIndex {
			continue
		}
		if sheetName, ref, ok := parseDefinedNameRange(dn.Value, pivotSheet); ok {
			return sheetName, ref, true
		}
	}
	for _, dn := range wb.DefinedNames {
		if dn == nil || dn.Hidden || dn.LocalSheetID != nil || !strings.EqualFold(strings.TrimSpace(dn.Name), strings.TrimSpace(pivot.SourceName)) {
			continue
		}
		if sheetName, ref, ok := parseDefinedNameRange(dn.Value, pivotSheet); ok {
			return sheetName, ref, true
		}
	}
	return "", "", false
}

func resolvePivotDataField(headers map[string]int, pivot *PivotTable, dataField string) (int, string, bool) {
	if pivot == nil {
		return 0, "", false
	}
	for _, field := range pivot.DataFields {
		if field == nil {
			continue
		}
		if strings.EqualFold(field.Name, dataField) || strings.EqualFold(field.SourceFieldName, dataField) {
			if idx, ok := headers[strings.ToUpper(strings.TrimSpace(field.SourceFieldName))]; ok {
				return idx, field.Subtotal, true
			}
		}
	}
	if idx, ok := headers[strings.ToUpper(strings.TrimSpace(dataField))]; ok {
		subtotal := "sum"
		if len(pivot.DataFields) == 1 && pivot.DataFields[0] != nil && strings.EqualFold(pivot.DataFields[0].SourceFieldName, dataField) {
			subtotal = pivot.DataFields[0].Subtotal
		}
		return idx, subtotal, true
	}
	return 0, "", false
}

func resolvePivotCriteriaColumns(headers map[string]int, criteria []formula.PivotDataCriterion) (map[string]int, bool) {
	if len(criteria) == 0 {
		return nil, true
	}
	cols := make(map[string]int, len(criteria))
	for _, criterion := range criteria {
		field := strings.ToUpper(strings.TrimSpace(criterion.Field))
		if field == "" {
			return nil, false
		}
		col, ok := headers[field]
		if !ok {
			return nil, false
		}
		cols[field] = col
	}
	return cols, true
}

func buildPivotSourceHeaders(sheet *Sheet, headerRow, firstCol, lastCol int, engineForSheet func(string) *formula.Engine) map[string]int {
	if sheet == nil {
		return nil
	}
	cells := buildPivotCellLookup(sheet)
	var eng *formula.Engine
	if engineForSheet != nil {
		eng = engineForSheet(sheet.Name)
	}
	headers := make(map[string]int, lastCol-firstCol+1)
	for col := firstCol; col <= lastCol; col++ {
		value := pivotCellFormulaValue(cells, eng, headerRow, col)
		name := strings.TrimSpace(value.ToText())
		if name == "" {
			name = "Field" + strconv.Itoa(col-firstCol+1)
		}
		headers[strings.ToUpper(name)] = col
	}
	return headers
}

func pivotRowMatchesCriteria(cells map[string]*Cell, eng *formula.Engine, row int, filterCols map[string]int, criteria []formula.PivotDataCriterion) bool {
	if len(criteria) == 0 {
		return true
	}
	for _, criterion := range criteria {
		field := strings.ToUpper(strings.TrimSpace(criterion.Field))
		col, ok := filterCols[field]
		if !ok {
			return false
		}
		value := pivotCellFormulaValue(cells, eng, row, col)
		if !pivotCriterionMatches(value, criterion.Item) {
			return false
		}
	}
	return true
}

func pivotCriterionMatches(value, criterion formula.Value) bool {
	if value.IsError() || criterion.IsError() {
		return value.IsError() && criterion.IsError() && value.Err == criterion.Err
	}
	if left, err1 := value.ToNumber(); err1 == nil {
		if right, err2 := criterion.ToNumber(); err2 == nil {
			return left == right
		}
	}
	if value.IsBool() || criterion.IsBool() {
		left, err1 := value.ToBool()
		right, err2 := criterion.ToBool()
		return err1 == nil && err2 == nil && left == right
	}
	return strings.EqualFold(strings.TrimSpace(value.ToText()), strings.TrimSpace(criterion.ToText()))
}

func pivotCellFormulaValue(cells map[string]*Cell, eng *formula.Engine, row, col int) formula.Value {
	ref := colLettersFromIndex(col) + strconv.Itoa(row)
	cell := cells[ref]
	if eng != nil && cell != nil && strings.TrimSpace(cell.Formula) != "" {
		return eng.GetCell(ref)
	}
	return pivotCachedFormulaValue(cell)
}

func pivotCachedFormulaValue(cell *Cell) formula.Value {
	if cell == nil {
		return formula.Empty()
	}
	switch cell.Kind {
	case CellNumber:
		return formula.Number(cell.NumValue)
	case CellBool:
		return formula.Bool(cell.NumValue != 0)
	case CellString:
		return formula.Text(cell.StrValue)
	case CellError:
		return formula.Error(formula.ParseError(cell.StrValue))
	case CellFormula:
		if cell.StrValue != "" {
			if num, err := strconv.ParseFloat(strings.TrimSpace(cell.StrValue), 64); err == nil {
				return formula.Number(num)
			}
			return formula.Text(cell.StrValue)
		}
		return formula.Number(cell.NumValue)
	default:
		return formula.Empty()
	}
}

func buildPivotCellLookup(sheet *Sheet) map[string]*Cell {
	if sheet == nil {
		return nil
	}
	lookup := make(map[string]*Cell)
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil || cell.Ref == "" {
				continue
			}
			lookup[strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(cell.Ref, "$", "")))] = cell
		}
	}
	return lookup
}

func workbookSheetByName(wb *Workbook, name string) *Sheet {
	if wb == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	for _, sheet := range wb.Sheets {
		if sheet != nil && strings.EqualFold(sheet.Name, name) {
			return sheet
		}
	}
	return nil
}

func workbookSheetIndex(wb *Workbook, name string) int {
	if wb == nil {
		return -1
	}
	name = strings.TrimSpace(name)
	for i, sheet := range wb.Sheets {
		if sheet != nil && strings.EqualFold(sheet.Name, name) {
			return i
		}
	}
	return -1
}

func parseDefinedNameRange(value, defaultSheet string) (string, string, bool) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "="))
	if value == "" {
		return "", "", false
	}
	if idx := strings.LastIndex(value, "!"); idx >= 0 {
		sheetName := unquotePivotSheetName(value[:idx])
		ref := normalizePivotCellOrRange(value[idx+1:])
		if sheetName == "" || ref == "" {
			return "", "", false
		}
		return sheetName, ref, true
	}
	ref := normalizePivotCellOrRange(value)
	if ref == "" {
		return "", "", false
	}
	return defaultSheet, ref, true
}

func parsePivotTableAnchorRef(defaultSheet, token string) (string, string, bool) {
	token = strings.TrimSpace(token)
	if len(token) >= 2 && token[0] == '"' && token[len(token)-1] == '"' {
		token = token[1 : len(token)-1]
	}
	if token == "" {
		return "", "", false
	}
	if colon := strings.Index(token, ":"); colon >= 0 {
		token = token[:colon]
	}
	sheetName := defaultSheet
	ref := token
	if idx := strings.LastIndex(token, "!"); idx >= 0 {
		sheetName = unquotePivotSheetName(token[:idx])
		ref = token[idx+1:]
	}
	ref = normalizePivotCellOrRange(ref)
	if sheetName == "" || ref == "" {
		return "", "", false
	}
	if _, _, ok := cellCoordsFromRef(ref); !ok {
		return "", "", false
	}
	return sheetName, ref, true
}

func pivotRangeContainsCell(rangeRef, cellRef string) bool {
	firstRow, firstCol, lastRow, lastCol, ok := parsePivotFormulaRange(rangeRef)
	if !ok {
		return false
	}
	row, col, ok := cellCoordsFromRef(normalizePivotCellOrRange(cellRef))
	if !ok {
		return false
	}
	return row >= firstRow && row <= lastRow && col >= firstCol && col <= lastCol
}

func parsePivotFormulaRange(ref string) (firstRow, firstCol, lastRow, lastCol int, ok bool) {
	ref = normalizePivotCellOrRange(ref)
	if ref == "" {
		return 0, 0, 0, 0, false
	}
	parts := strings.SplitN(ref, ":", 2)
	firstRow, firstCol, ok = cellCoordsFromRef(parts[0])
	if !ok {
		return 0, 0, 0, 0, false
	}
	lastRow, lastCol = firstRow, firstCol
	if len(parts) == 2 {
		lastRow, lastCol, ok = cellCoordsFromRef(parts[1])
		if !ok {
			return 0, 0, 0, 0, false
		}
	}
	if lastRow < firstRow {
		firstRow, lastRow = lastRow, firstRow
	}
	if lastCol < firstCol {
		firstCol, lastCol = lastCol, firstCol
	}
	return firstRow, firstCol, lastRow, lastCol, true
}

func normalizePivotCellOrRange(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.ReplaceAll(ref, "$", "")
	return strings.ToUpper(ref)
}

func unquotePivotSheetName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		name = strings.ReplaceAll(name[1:len(name)-1], "''", "'")
	}
	return name
}
