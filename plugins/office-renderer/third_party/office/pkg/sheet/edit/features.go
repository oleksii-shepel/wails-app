package edit

import (
	"fmt"
	"strings"

	sheetir "office/pkg/sheet/ir"
)

// DefinedNames returns workbook defined names in storage order.
func (w *Workbook) DefinedNames() []*sheetir.DefinedName {
	if w == nil || w.ir == nil {
		return nil
	}
	return append([]*sheetir.DefinedName(nil), w.ir.DefinedNames...)
}

// SetDefinedName creates or replaces a workbook or sheet-local defined name.
func (w *Workbook) SetDefinedName(name, value string, localSheetIndex *int) (*sheetir.DefinedName, error) {
	return w.SetDefinedNameWithOptions(name, value, localSheetIndex, false, "")
}

// SetDefinedNameWithOptions creates or replaces a defined name with metadata.
func (w *Workbook) SetDefinedNameWithOptions(name, value string, localSheetIndex *int, hidden bool, comment string) (*sheetir.DefinedName, error) {
	if w == nil || w.ir == nil {
		return nil, fmt.Errorf("sheet edit: nil workbook")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("sheet edit: defined name is empty")
	}
	if localSheetIndex != nil {
		if err := w.checkSheetIndex(*localSheetIndex); err != nil {
			return nil, err
		}
	}
	if dn, ok := w.DefinedName(name, localSheetIndex); ok {
		dn.Value = value
		dn.Hidden = hidden
		dn.Comment = comment
		w.markAllPivotDirty()
		return dn, nil
	}
	dn := &sheetir.DefinedName{
		Name:         name,
		LocalSheetID: cloneIntPtr(localSheetIndex),
		Hidden:       hidden,
		Value:        value,
		Comment:      comment,
	}
	w.ir.DefinedNames = append(w.ir.DefinedNames, dn)
	w.markAllPivotDirty()
	return dn, nil
}

// DefinedName looks up a workbook or sheet-local defined name.
func (w *Workbook) DefinedName(name string, localSheetIndex *int) (*sheetir.DefinedName, bool) {
	if w == nil || w.ir == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	for _, dn := range w.ir.DefinedNames {
		if !strings.EqualFold(dn.Name, name) {
			continue
		}
		if sameLocalSheet(dn.LocalSheetID, localSheetIndex) {
			return dn, true
		}
	}
	return nil, false
}

// RemoveDefinedName removes a workbook or sheet-local defined name.
func (w *Workbook) RemoveDefinedName(name string, localSheetIndex *int) bool {
	if w == nil || w.ir == nil {
		return false
	}
	name = strings.TrimSpace(name)
	for i, dn := range w.ir.DefinedNames {
		if strings.EqualFold(dn.Name, name) && sameLocalSheet(dn.LocalSheetID, localSheetIndex) {
			w.ir.DefinedNames = append(w.ir.DefinedNames[:i], w.ir.DefinedNames[i+1:]...)
			w.markAllPivotDirty()
			return true
		}
	}
	return false
}

// Tables returns worksheet structured tables in storage order.
func (s *Sheet) Tables() []*sheetir.Table {
	if s == nil || s.sheet == nil {
		return nil
	}
	return append([]*sheetir.Table(nil), s.sheet.Tables...)
}

// AddTable creates a structured table over ref.
func (s *Sheet) AddTable(name, ref string, headers []string) (*sheetir.Table, error) {
	if s == nil || s.wb == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	name = strings.TrimSpace(name)
	ref = strings.TrimSpace(ref)
	if name == "" {
		name = s.nextTableName()
	}
	if ref == "" {
		return nil, fmt.Errorf("sheet edit: table range is empty")
	}
	if _, ok := s.Table(name); ok {
		return nil, fmt.Errorf("sheet edit: table name %q already exists", name)
	}
	table := &sheetir.Table{
		ID:             s.wb.nextTableID(),
		Name:           name,
		DisplayName:    name,
		Ref:            ref,
		HeaderRowCount: 1,
		AutoFilterRef:  ref,
		TableStyle:     &sheetir.TableStyleInfo{Name: "TableStyleMedium2", ShowRowStripes: true},
		Columns:        make([]*sheetir.TableColumn, 0, len(headers)),
		TotalsRowShown: false,
		TotalsRowCount: 0,
	}
	for i, header := range headers {
		table.Columns = append(table.Columns, &sheetir.TableColumn{
			ID:   i + 1,
			Name: header,
		})
	}
	s.sheet.Tables = append(s.sheet.Tables, table)
	return table, nil
}

// Table returns a structured table by name or display name.
func (s *Sheet) Table(name string) (*sheetir.Table, bool) {
	if s == nil || s.sheet == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	for _, table := range s.sheet.Tables {
		if strings.EqualFold(table.Name, name) || strings.EqualFold(table.DisplayName, name) {
			return table, true
		}
	}
	return nil, false
}

// ResizeTable changes a structured table range and embedded auto-filter range.
func (s *Sheet) ResizeTable(name, ref string) error {
	table, ok := s.Table(name)
	if !ok {
		return fmt.Errorf("sheet edit: table %q not found", name)
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return fmt.Errorf("sheet edit: table range is empty")
	}
	table.Ref = ref
	table.AutoFilterRef = ref
	return nil
}

// SetTableStyle updates structured table style metadata.
func (s *Sheet) SetTableStyle(name, styleName string, showFirstColumn, showLastColumn, showRowStripes, showColumnStripes bool) error {
	table, ok := s.Table(name)
	if !ok {
		return fmt.Errorf("sheet edit: table %q not found", name)
	}
	table.TableStyle = &sheetir.TableStyleInfo{
		Name:              styleName,
		ShowFirstColumn:   showFirstColumn,
		ShowLastColumn:    showLastColumn,
		ShowRowStripes:    showRowStripes,
		ShowColumnStripes: showColumnStripes,
	}
	return nil
}

// SetTableTotalsRow shows or hides a one-row table totals row.
func (s *Sheet) SetTableTotalsRow(name string, shown bool) error {
	table, ok := s.Table(name)
	if !ok {
		return fmt.Errorf("sheet edit: table %q not found", name)
	}
	table.TotalsRowShown = shown
	if shown {
		table.TotalsRowCount = 1
	} else {
		table.TotalsRowCount = 0
	}
	return nil
}

// SetTableColumnCalculatedFormula updates a calculated-column formula.
func (s *Sheet) SetTableColumnCalculatedFormula(tableName, columnName, formula string) error {
	column, err := s.tableColumn(tableName, columnName)
	if err != nil {
		return err
	}
	column.CalculatedColumnFormula = formula
	return nil
}

// SetTableColumnTotalsFormula updates an explicit totals-row formula.
func (s *Sheet) SetTableColumnTotalsFormula(tableName, columnName, formula string) error {
	column, err := s.tableColumn(tableName, columnName)
	if err != nil {
		return err
	}
	column.TotalsRowFormula = formula
	column.TotalsRowFunction = "custom"
	return nil
}

// SetTableColumnTotalsFunction updates a built-in totals-row function.
func (s *Sheet) SetTableColumnTotalsFunction(tableName, columnName, function string) error {
	column, err := s.tableColumn(tableName, columnName)
	if err != nil {
		return err
	}
	column.TotalsRowFunction = function
	if function != "custom" {
		column.TotalsRowFormula = ""
	}
	return nil
}

// SetTableColumnTotalsLabel updates a totals-row label.
func (s *Sheet) SetTableColumnTotalsLabel(tableName, columnName, label string) error {
	column, err := s.tableColumn(tableName, columnName)
	if err != nil {
		return err
	}
	column.TotalsRowLabel = label
	return nil
}

// RemoveTable removes a structured table by name or display name.
func (s *Sheet) RemoveTable(name string) bool {
	if s == nil || s.sheet == nil {
		return false
	}
	name = strings.TrimSpace(name)
	for i, table := range s.sheet.Tables {
		if strings.EqualFold(table.Name, name) || strings.EqualFold(table.DisplayName, name) {
			s.sheet.Tables = append(s.sheet.Tables[:i], s.sheet.Tables[i+1:]...)
			return true
		}
	}
	return false
}

// SetAutoFilterValues sets discrete equality filters for one auto-filter column.
func (s *Sheet) SetAutoFilterValues(colIndex int, values []string, showBlanks bool) error {
	if s.sheet.AutoFilterRef == "" {
		return fmt.Errorf("sheet edit: auto-filter range is not set")
	}
	if colIndex < 0 {
		return fmt.Errorf("sheet edit: auto-filter column index must be >= 0")
	}
	fc := s.ensureFilterColumn(colIndex)
	fc.Filters = append([]string(nil), values...)
	fc.ShowBlanks = showBlanks
	fc.CustomFilters = nil
	fc.Top10 = nil
	return nil
}

// SetAutoFilterCustom sets one or two custom comparison filters for a column.
func (s *Sheet) SetAutoFilterCustom(colIndex int, filters []*sheetir.CustomFilter, and bool) error {
	if s.sheet.AutoFilterRef == "" {
		return fmt.Errorf("sheet edit: auto-filter range is not set")
	}
	if colIndex < 0 {
		return fmt.Errorf("sheet edit: auto-filter column index must be >= 0")
	}
	fc := s.ensureFilterColumn(colIndex)
	fc.CustomFilters = append([]*sheetir.CustomFilter(nil), filters...)
	fc.CustomAnd = and
	fc.Filters = nil
	fc.ShowBlanks = false
	fc.Top10 = nil
	return nil
}

// SetAutoFilterTop10 sets a top/bottom-N filter for one auto-filter column.
func (s *Sheet) SetAutoFilterTop10(colIndex int, top bool, percent bool, value float64) error {
	if s.sheet.AutoFilterRef == "" {
		return fmt.Errorf("sheet edit: auto-filter range is not set")
	}
	if colIndex < 0 {
		return fmt.Errorf("sheet edit: auto-filter column index must be >= 0")
	}
	fc := s.ensureFilterColumn(colIndex)
	fc.Top10 = &sheetir.FilterTop10{Top: top, Percent: percent, Val: value}
	fc.Filters = nil
	fc.ShowBlanks = false
	fc.CustomFilters = nil
	return nil
}

// SetAutoFilterDynamic sets an OOXML dynamic filter for one auto-filter column.
func (s *Sheet) SetAutoFilterDynamic(colIndex int, filter sheetir.DynamicFilter) error {
	if s.sheet.AutoFilterRef == "" {
		return fmt.Errorf("sheet edit: auto-filter range is not set")
	}
	if colIndex < 0 {
		return fmt.Errorf("sheet edit: auto-filter column index must be >= 0")
	}
	filter.Type = strings.TrimSpace(filter.Type)
	if filter.Type == "" {
		return fmt.Errorf("sheet edit: dynamic filter type is empty")
	}
	fc := s.ensureFilterColumn(colIndex)
	fc.Dynamic = cloneDynamicFilter(&filter)
	fc.Filters = nil
	fc.ShowBlanks = false
	fc.CustomFilters = nil
	fc.Top10 = nil
	return nil
}

// ClearAutoFilterColumn removes filter criteria for one auto-filter column.
func (s *Sheet) ClearAutoFilterColumn(colIndex int) bool {
	if s == nil || s.sheet == nil {
		return false
	}
	for i, fc := range s.sheet.FilterColumns {
		if fc != nil && fc.ColIndex == colIndex {
			s.sheet.FilterColumns = append(s.sheet.FilterColumns[:i], s.sheet.FilterColumns[i+1:]...)
			return true
		}
	}
	return false
}

// SortState returns the worksheet auto-filter sort state.
func (s *Sheet) SortState() *sheetir.SortState {
	if s == nil || s.sheet == nil || s.sheet.SortState == nil {
		return nil
	}
	return cloneSortState(s.sheet.SortState)
}

// SetSortState replaces the worksheet auto-filter sort state.
func (s *Sheet) SetSortState(state sheetir.SortState) (*sheetir.SortState, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	if s.sheet.AutoFilterRef == "" {
		return nil, fmt.Errorf("sheet edit: auto-filter range is not set")
	}
	if strings.TrimSpace(state.Ref) == "" {
		state.Ref = s.sheet.AutoFilterRef
	}
	if len(state.Conditions) == 0 {
		return nil, fmt.Errorf("sheet edit: sort state requires at least one condition")
	}
	for _, condition := range state.Conditions {
		if condition == nil || strings.TrimSpace(condition.Ref) == "" {
			return nil, fmt.Errorf("sheet edit: sort condition ref is empty")
		}
	}
	s.sheet.SortState = cloneSortState(&state)
	return s.sheet.SortState, nil
}

// ClearSortState removes the worksheet auto-filter sort state.
func (s *Sheet) ClearSortState() {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.SortState = nil
}

func (s *Sheet) ensureFilterColumn(colIndex int) *sheetir.FilterColumn {
	for _, fc := range s.sheet.FilterColumns {
		if fc != nil && fc.ColIndex == colIndex {
			return fc
		}
	}
	fc := &sheetir.FilterColumn{ColIndex: colIndex}
	s.sheet.FilterColumns = append(s.sheet.FilterColumns, fc)
	return fc
}

func (s *Sheet) nextTableName() string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("Table%d", i)
		if _, ok := s.Table(name); !ok {
			return name
		}
	}
}

func (s *Sheet) tableColumn(tableName, columnName string) (*sheetir.TableColumn, error) {
	table, ok := s.Table(tableName)
	if !ok {
		return nil, fmt.Errorf("sheet edit: table %q not found", tableName)
	}
	columnName = strings.TrimSpace(columnName)
	for _, column := range table.Columns {
		if strings.EqualFold(column.Name, columnName) {
			return column, nil
		}
	}
	return nil, fmt.Errorf("sheet edit: table column %q not found", columnName)
}

func (w *Workbook) nextTableID() int {
	maxID := 0
	for _, sheet := range w.ir.Sheets {
		for _, table := range sheet.Tables {
			if table.ID > maxID {
				maxID = table.ID
			}
		}
	}
	return maxID + 1
}

func sameLocalSheet(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
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

func cloneDynamicFilter(filter *sheetir.DynamicFilter) *sheetir.DynamicFilter {
	if filter == nil {
		return nil
	}
	out := *filter
	out.Val = cloneFloat64Ptr(filter.Val)
	out.MaxVal = cloneFloat64Ptr(filter.MaxVal)
	return &out
}

func cloneSortState(state *sheetir.SortState) *sheetir.SortState {
	if state == nil {
		return nil
	}
	out := &sheetir.SortState{
		Ref:           state.Ref,
		CaseSensitive: state.CaseSensitive,
	}
	for _, condition := range state.Conditions {
		if condition == nil {
			continue
		}
		clone := *condition
		clone.DxfID = cloneIntPtr(condition.DxfID)
		clone.IconID = cloneIntPtr(condition.IconID)
		out.Conditions = append(out.Conditions, &clone)
	}
	return out
}
