package legacytoooxml

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

// mapPivotTables converts IR pivot table summaries into OOXML pivot table/cache
// parts that can be serialized by sheetparts.WritePackage.
func (c *converter) mapPivotTables(sheet *sheetir.Sheet, wb *sheetir.Workbook) []*sheetparts.PivotTableWithCache {
	if len(sheet.PivotTables) == 0 {
		return nil
	}
	out := make([]*sheetparts.PivotTableWithCache, 0, len(sheet.PivotTables))
	for _, pt := range sheet.PivotTables {
		ptc, err := buildPivotTable(pt, wb, sheet.Name, c.nextPivotCacheID)
		if err != nil {
			continue
		}
		out = append(out, ptc)
	}
	return out
}

func buildPivotTable(pt *sheetir.PivotTable, wb *sheetir.Workbook, pivotSheet string, nextCacheID func() int) (*sheetparts.PivotTableWithCache, error) {
	sourceSheet, sourceRef, worksheetSource, err := resolvePivotWorksheetSource(pt, wb, pivotSheet)
	if err != nil {
		return nil, err
	}

	src := sheetByName(sourceSheet, wb)
	if src == nil {
		return nil, fmt.Errorf("pivot table source sheet %q not found", sourceSheet)
	}

	fr, fc, lr, lc, ok := parseRange(sourceRef)
	if !ok || fr < 1 || fc < 1 || lr < fr || lc < fc {
		return nil, fmt.Errorf("invalid pivot source range %q", sourceRef)
	}

	fieldCount := lc - fc + 1
	fieldNames := make([]string, fieldCount)
	fieldIndex := make(map[string]int, fieldCount)
	for col := fc; col <= lc; col++ {
		name := cellStringValue(src, fr, col)
		if name == "" {
			name = fmt.Sprintf("Field%d", col-fc+1)
		}
		idx := col - fc
		fieldNames[idx] = name
		fieldIndex[name] = idx
	}

	dataFieldSet := make(map[int]bool)
	dataFieldDefs := make(map[int]*sheetir.PivotDataField)
	for _, df := range pt.DataFields {
		idx, ok := fieldIndex[df.SourceFieldName]
		if !ok {
			continue
		}
		dataFieldSet[idx] = true
		dataFieldDefs[idx] = df
	}

	axisMap := make(map[string]string)
	for _, name := range pt.RowFields {
		axisMap[name] = "axisRow"
	}
	for _, name := range pt.ColFields {
		axisMap[name] = "axisCol"
	}
	for _, name := range pt.PageFields {
		axisMap[name] = "axisPage"
	}

	// Determine field types and collect shared items for string fields.
	fieldTypes := make([]string, fieldCount)
	sharedItemMaps := make([]map[string]int, fieldCount)
	sharedItemLists := make([][]sheetparts.SharedItem, fieldCount)
	for i := range sharedItemMaps {
		sharedItemMaps[i] = make(map[string]int)
	}

	for row := fr + 1; row <= lr; row++ {
		for fi := 0; fi < fieldCount; fi++ {
			cell := cellAt(src, row, fc+fi)
			typ, val := cacheCellValue(cell)
			if typ == "" {
				continue
			}
			if fieldTypes[fi] == "" {
				fieldTypes[fi] = typ
			} else if fieldTypes[fi] != typ {
				// Mixed types fall back to string.
				fieldTypes[fi] = "s"
			}
			if typ == "s" {
				if _, ok := sharedItemMaps[fi][val]; !ok {
					sharedItemMaps[fi][val] = len(sharedItemLists[fi])
					sharedItemLists[fi] = append(sharedItemLists[fi], sheetparts.SharedItem{Type: "s", Value: val})
				}
			}
		}
	}

	// Build cache records.
	records := make([]sheetparts.PivotCacheRow, 0, lr-fr)
	for row := fr + 1; row <= lr; row++ {
		cacheRow := sheetparts.PivotCacheRow{}
		for fi := 0; fi < fieldCount; fi++ {
			cell := cellAt(src, row, fc+fi)
			_, val := cacheCellValue(cell)
			var pc sheetparts.PivotCacheCell
			switch fieldTypes[fi] {
			case "n":
				pc = sheetparts.PivotCacheCell{Type: "n", Value: val}
			case "b":
				pc = sheetparts.PivotCacheCell{Type: "b", Value: val}
			case "e":
				pc = sheetparts.PivotCacheCell{Type: "e", Value: val}
			case "s":
				idx, ok := sharedItemMaps[fi][val]
				if !ok {
					idx = len(sharedItemLists[fi])
					sharedItemMaps[fi][val] = idx
					sharedItemLists[fi] = append(sharedItemLists[fi], sheetparts.SharedItem{Type: "s", Value: val})
				}
				pc = sheetparts.PivotCacheCell{Type: "s", Value: strconv.Itoa(idx)}
			default:
				pc = sheetparts.PivotCacheCell{Value: val}
			}
			cacheRow.Cells = append(cacheRow.Cells, pc)
		}
		records = append(records, cacheRow)
	}

	// Build cache fields.
	cacheFields := make([]sheetparts.CacheField, fieldCount)
	for fi, name := range fieldNames {
		cf := sheetparts.CacheField{Name: name}
		if fieldTypes[fi] == "s" && len(sharedItemLists[fi]) > 0 {
			cf.SharedItems = &sheetparts.SharedItems{
				Count: len(sharedItemLists[fi]),
				Items: sharedItemLists[fi],
			}
		}
		cacheFields[fi] = cf
	}

	// Build pivot fields and axis references.
	semanticFields := make(map[string]*sheetir.PivotField, len(pt.Fields))
	for _, field := range pt.Fields {
		if field != nil {
			semanticFields[strings.ToLower(field.Name)] = field
		}
	}
	pivotFields := make([]sheetparts.PivotField, fieldCount)
	for fi, name := range fieldNames {
		pf := sheetparts.PivotField{Name: name, ShowAll: true}
		if semantic := semanticFields[strings.ToLower(name)]; semantic != nil {
			pf.Compact, pf.Outline, pf.SubtotalTop, pf.ShowAll = semantic.Compact, semantic.Outline, semantic.SubtotalTop, semantic.ShowAll
			if len(semantic.Items) > 0 {
				items := &sheetparts.PivotItems{}
				for _, item := range semantic.Items {
					if item == nil {
						continue
					}
					sd := !item.HideDetail
					typ := item.Type
					if typ == "data" {
						typ = ""
					}
					items.Items = append(items.Items, sheetparts.PivotItem{X: item.CacheIndex, Type: typ, Hidden: item.Hidden, HideDetail: &sd, Name: item.Name})
				}
				items.Count = len(items.Items)
				pf.Items = items
			}
		}
		if dataFieldSet[fi] {
			pf.DataField = true
		}
		if axis, ok := axisMap[name]; ok {
			pf.Axis = axis
		}
		pivotFields[fi] = pf
	}

	makeFieldIndex := func(name string) int {
		if idx, ok := fieldIndex[name]; ok {
			return idx
		}
		return -1
	}

	rowFields := make([]sheetparts.PivotFieldIndex, 0, len(pt.RowFields))
	for _, name := range pt.RowFields {
		if idx := makeFieldIndex(name); idx >= 0 {
			rowFields = append(rowFields, sheetparts.PivotFieldIndex{Index: idx})
		}
	}
	colFields := make([]sheetparts.PivotFieldIndex, 0, len(pt.ColFields))
	for _, name := range pt.ColFields {
		if idx := makeFieldIndex(name); idx >= 0 {
			colFields = append(colFields, sheetparts.PivotFieldIndex{Index: idx})
		}
	}
	pageSelection := make(map[string]int, len(pt.PageFieldSelections))
	for _, selection := range pt.PageFieldSelections {
		if selection != nil {
			pageSelection[strings.ToLower(selection.FieldName)] = selection.ItemIndex
		}
	}
	pageFields := make([]sheetparts.PageField, 0, len(pt.PageFields))
	for _, name := range pt.PageFields {
		if idx := makeFieldIndex(name); idx >= 0 {
			entry := sheetparts.PageField{Field: idx, Name: name}
			if item, ok := pageSelection[strings.ToLower(name)]; ok {
				value := item
				entry.Item = &value
			}
			pageFields = append(pageFields, entry)
		}
	}
	dataFields := make([]sheetparts.DataField, 0, len(pt.DataFields))
	for _, df := range pt.DataFields {
		if idx := makeFieldIndex(df.SourceFieldName); idx >= 0 {
			dataFields = append(dataFields, sheetparts.DataField{
				Name:     df.Name,
				Field:    idx,
				Subtotal: df.Subtotal,
				NumFmtID: df.NumFmtID,
			})
		}
	}

	cacheID := nextCacheID()
	table := &sheetparts.PivotTableDefinition{
		Name:           pt.Name,
		CacheID:        cacheID,
		DataCaption:    pt.DataCaption,
		RowGrandTotals: boolAttr(pt.RowGrandTotals),
		ColGrandTotals: boolAttr(pt.ColGrandTotals),
		ShowHeaders:    boolAttr(pt.ShowHeaders),
		Location: sheetparts.PivotLocation{
			Ref:            pt.LocationRef,
			FirstHeaderRow: 0,
			FirstDataRow:   1,
			FirstDataCol:   0,
		},
		PivotFields: pivotFields,
		RowFields:   rowFields,
		ColFields:   colFields,
		PageFields:  pageFields,
		DataFields:  dataFields,
	}

	cache := &sheetparts.PivotCacheDefinition{
		RefreshedBy:           "office",
		RefreshedDate:         time.Now().UTC().Format(time.RFC3339),
		CreatedVersion:        6,
		RefreshedVersion:      6,
		MinRefreshableVersion: 6,
		RecordCount:           len(records),
		WorksheetSource:       worksheetSource,
		CacheFields: &sheetparts.CacheFields{
			Count:  len(cacheFields),
			Fields: cacheFields,
		},
	}

	recordsPart := &sheetparts.PivotCacheRecords{
		Count: len(records),
		Rows:  records,
	}

	return &sheetparts.PivotTableWithCache{
		Table:   table,
		Cache:   cache,
		Records: recordsPart,
	}, nil
}

func resolvePivotWorksheetSource(pt *sheetir.PivotTable, wb *sheetir.Workbook, pivotSheet string) (string, string, *sheetparts.WorksheetSource, error) {
	if pt == nil {
		return "", "", nil, fmt.Errorf("nil pivot table")
	}
	if strings.TrimSpace(pt.SourceSheet) != "" && strings.TrimSpace(pt.SourceRef) != "" {
		return pt.SourceSheet, pt.SourceRef, &sheetparts.WorksheetSource{
			Sheet: pt.SourceSheet,
			Ref:   pt.SourceRef,
		}, nil
	}
	sourceName := strings.TrimSpace(pt.SourceName)
	if sourceName == "" {
		return "", "", nil, fmt.Errorf("pivot table %q missing source sheet/ref or source name", pt.Name)
	}
	sourceSheet, sourceRef, ok := resolvePivotDefinedNameRange(wb, pivotSheet, sourceName)
	if !ok {
		return "", "", nil, fmt.Errorf("pivot table %q source name %q not found or not range-backed", pt.Name, sourceName)
	}
	return sourceSheet, sourceRef, &sheetparts.WorksheetSource{Name: sourceName}, nil
}

func sheetByName(name string, wb *sheetir.Workbook) *sheetir.Sheet {
	if wb == nil {
		return nil
	}
	for _, s := range wb.Sheets {
		if s != nil && s.Name == name {
			return s
		}
	}
	return nil
}

func workbookSheetIndex(wb *sheetir.Workbook, name string) int {
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

func resolvePivotDefinedNameRange(wb *sheetir.Workbook, pivotSheet, sourceName string) (string, string, bool) {
	if wb == nil {
		return "", "", false
	}
	sourceName = strings.TrimSpace(sourceName)
	pivotSheetIndex := workbookSheetIndex(wb, pivotSheet)
	for _, dn := range wb.DefinedNames {
		if dn == nil || dn.Hidden || !strings.EqualFold(strings.TrimSpace(dn.Name), sourceName) {
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
		if dn == nil || dn.Hidden || dn.LocalSheetID != nil || !strings.EqualFold(strings.TrimSpace(dn.Name), sourceName) {
			continue
		}
		if sheetName, ref, ok := parseDefinedNameRange(dn.Value, pivotSheet); ok {
			return sheetName, ref, true
		}
	}
	return "", "", false
}

func cellStringValue(sheet *sheetir.Sheet, row, col int) string {
	cell := cellAt(sheet, row, col)
	if cell == nil {
		return ""
	}
	switch cell.Kind {
	case sheetir.CellString:
		return cell.StrValue
	case sheetir.CellNumber:
		return strconv.FormatFloat(cell.NumValue, 'f', -1, 64)
	case sheetir.CellBool:
		return strconv.FormatBool(cell.NumValue != 0)
	case sheetir.CellError:
		return cell.StrValue
	default:
		return ""
	}
}

func cacheCellValue(cell *sheetir.Cell) (typ, val string) {
	if cell == nil {
		return "", ""
	}
	switch cell.Kind {
	case sheetir.CellNumber:
		return "n", strconv.FormatFloat(cell.NumValue, 'f', -1, 64)
	case sheetir.CellBool:
		return "b", strconv.FormatBool(cell.NumValue != 0)
	case sheetir.CellError:
		return "e", cell.StrValue
	case sheetir.CellString:
		return "s", cell.StrValue
	default:
		return "s", ""
	}
}

func cellAt(sheet *sheetir.Sheet, row, col int) *sheetir.Cell {
	if sheet == nil {
		return nil
	}
	for _, r := range sheet.Rows {
		if r != nil && r.Index == row {
			for _, c := range r.Cells {
				if c != nil && c.Col == col {
					return c
				}
			}
		}
	}
	return nil
}

func parseRange(ref string) (firstRow, firstCol, lastRow, lastCol int, ok bool) {
	parts := strings.Split(ref, ":")
	if len(parts) != 2 {
		return 0, 0, 0, 0, false
	}
	fr, fc, ok1 := parseCellRef(parts[0])
	lr, lc, ok2 := parseCellRef(parts[1])
	if !ok1 || !ok2 {
		return 0, 0, 0, 0, false
	}
	return fr, fc, lr, lc, true
}

func parseDefinedNameRange(value, defaultSheet string) (string, string, bool) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "="))
	if value == "" {
		return "", "", false
	}
	if idx := strings.LastIndex(value, "!"); idx >= 0 {
		sheetName := unquoteSheetName(value[:idx])
		ref := normalizeRangeRef(value[idx+1:])
		if sheetName == "" || ref == "" {
			return "", "", false
		}
		return sheetName, ref, true
	}
	ref := normalizeRangeRef(value)
	if ref == "" {
		return "", "", false
	}
	return defaultSheet, ref, true
}

func parseCellRef(ref string) (row, col int, ok bool) {
	i := 0
	for i < len(ref) && (ref[i] >= 'A' && ref[i] <= 'Z' || ref[i] >= 'a' && ref[i] <= 'z') {
		i++
	}
	if i == 0 || i >= len(ref) {
		return 0, 0, false
	}
	col = 0
	for j := 0; j < i; j++ {
		col = col*26 + int(unicodeUpper(ref[j])-'A') + 1
	}
	r, err := strconv.Atoi(ref[i:])
	if err != nil || r < 1 {
		return 0, 0, false
	}
	return r, col, true
}

func unicodeUpper(ch byte) byte {
	if ch >= 'a' && ch <= 'z' {
		return ch - ('a' - 'A')
	}
	return ch
}

func normalizeRangeRef(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.ReplaceAll(ref, "$", "")
	return strings.ToUpper(ref)
}

func unquoteSheetName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		name = strings.ReplaceAll(name[1:len(name)-1], "''", "'")
	}
	return name
}

func boolAttr(value bool) *bool {
	v := value
	return &v
}
