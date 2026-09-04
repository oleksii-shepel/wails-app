package edit

import (
	"fmt"
	"strings"

	sheetir "office/pkg/sheet/ir"
)

// PivotTables returns worksheet pivot tables in storage order.
func (s *Sheet) PivotTables() []*sheetir.PivotTable {
	if s == nil || s.sheet == nil {
		return nil
	}
	return append([]*sheetir.PivotTable(nil), s.sheet.PivotTables...)
}

// PivotTable returns a worksheet pivot table by name.
func (s *Sheet) PivotTable(name string) (*sheetir.PivotTable, bool) {
	if s == nil || s.sheet == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	for _, pivot := range s.sheet.PivotTables {
		if pivot != nil && strings.EqualFold(pivot.Name, name) {
			return pivot, true
		}
	}
	return nil, false
}

// AddPivotTable creates a pivot table backed by a worksheet source range.
func (s *Sheet) AddPivotTable(name, sourceSheet, sourceRef, locationRef string) (*sheetir.PivotTable, error) {
	if s == nil || s.wb == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = s.nextPivotTableName()
	}
	if _, ok := s.PivotTable(name); ok {
		return nil, fmt.Errorf("sheet edit: pivot table %q already exists", name)
	}
	sourceName, err := s.resolvePivotSourceSheetName(sourceSheet)
	if err != nil {
		return nil, err
	}
	sourceRef = strings.TrimSpace(sourceRef)
	if sourceRef == "" {
		return nil, fmt.Errorf("sheet edit: pivot source range is empty")
	}
	locationRef = strings.TrimSpace(locationRef)
	if locationRef == "" {
		return nil, fmt.Errorf("sheet edit: pivot location range is empty")
	}
	pivot := &sheetir.PivotTable{
		Name:           name,
		SourceSheet:    sourceName,
		SourceRef:      sourceRef,
		LocationRef:    locationRef,
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowHeaders:    true,
	}
	s.sheet.PivotTables = append(s.sheet.PivotTables, pivot)
	s.wb.markPivotDirty(s.sheet.ID)
	return pivot, nil
}

// SetPivotSource updates the worksheet range backing a pivot table.
func (s *Sheet) SetPivotSource(name, sourceSheet, sourceRef string) error {
	pivot, ok := s.PivotTable(name)
	if !ok {
		return fmt.Errorf("sheet edit: pivot table %q not found", name)
	}
	sourceName, err := s.resolvePivotSourceSheetName(sourceSheet)
	if err != nil {
		return err
	}
	sourceRef = strings.TrimSpace(sourceRef)
	if sourceRef == "" {
		return fmt.Errorf("sheet edit: pivot source range is empty")
	}
	pivot.SourceSheet = sourceName
	pivot.SourceRef = sourceRef
	pivot.SourceName = ""
	s.wb.markPivotDirty(s.sheet.ID)
	return nil
}

// SetPivotSourceName updates the named range backing a pivot table.
func (s *Sheet) SetPivotSourceName(name, sourceName string) error {
	pivot, ok := s.PivotTable(name)
	if !ok {
		return fmt.Errorf("sheet edit: pivot table %q not found", name)
	}
	sourceName, err := s.resolvePivotDefinedName(sourceName)
	if err != nil {
		return err
	}
	pivot.SourceSheet = ""
	pivot.SourceRef = ""
	pivot.SourceName = sourceName
	s.wb.markPivotDirty(s.sheet.ID)
	return nil
}

// SetPivotLocation updates the worksheet range occupied by a pivot table.
func (s *Sheet) SetPivotLocation(name, locationRef string) error {
	pivot, ok := s.PivotTable(name)
	if !ok {
		return fmt.Errorf("sheet edit: pivot table %q not found", name)
	}
	locationRef = strings.TrimSpace(locationRef)
	if locationRef == "" {
		return fmt.Errorf("sheet edit: pivot location range is empty")
	}
	pivot.LocationRef = locationRef
	s.wb.markPivotDirty(s.sheet.ID)
	return nil
}

// SetPivotDataCaption updates the Values-area caption for a pivot table.
func (s *Sheet) SetPivotDataCaption(name, caption string) error {
	pivot, err := s.requirePivotTable(name)
	if err != nil {
		return err
	}
	pivot.DataCaption = strings.TrimSpace(caption)
	s.wb.markPivotDirty(s.sheet.ID)
	return nil
}

// SetPivotLayout updates header and grand-total visibility metadata.
func (s *Sheet) SetPivotLayout(name string, showHeaders, rowGrandTotals, colGrandTotals bool) error {
	pivot, err := s.requirePivotTable(name)
	if err != nil {
		return err
	}
	pivot.ShowHeaders = showHeaders
	pivot.RowGrandTotals = rowGrandTotals
	pivot.ColGrandTotals = colGrandTotals
	s.wb.markPivotDirty(s.sheet.ID)
	return nil
}

// AddPivotRowField appends a row-axis field to a pivot table.
func (s *Sheet) AddPivotRowField(tableName, fieldName string) error {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return err
	}
	pivot.RowFields = appendUniqueString(pivot.RowFields, fieldName)
	s.wb.markPivotDirty(s.sheet.ID)
	return nil
}

// RemovePivotRowField removes a row-axis field by name.
func (s *Sheet) RemovePivotRowField(tableName, fieldName string) bool {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return false
	}
	updated, removed := removeFoldStringValue(pivot.RowFields, fieldName)
	if removed {
		pivot.RowFields = updated
		s.wb.markPivotDirty(s.sheet.ID)
	}
	return removed
}

// ClearPivotRowFields removes all row-axis fields.
func (s *Sheet) ClearPivotRowFields(tableName string) int {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return 0
	}
	removed := len(pivot.RowFields)
	pivot.RowFields = nil
	if removed > 0 {
		s.wb.markPivotDirty(s.sheet.ID)
	}
	return removed
}

// AddPivotColumnField appends a column-axis field to a pivot table.
func (s *Sheet) AddPivotColumnField(tableName, fieldName string) error {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return err
	}
	pivot.ColFields = appendUniqueString(pivot.ColFields, fieldName)
	s.wb.markPivotDirty(s.sheet.ID)
	return nil
}

// RemovePivotColumnField removes a column-axis field by name.
func (s *Sheet) RemovePivotColumnField(tableName, fieldName string) bool {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return false
	}
	updated, removed := removeFoldStringValue(pivot.ColFields, fieldName)
	if removed {
		pivot.ColFields = updated
		s.wb.markPivotDirty(s.sheet.ID)
	}
	return removed
}

// ClearPivotColumnFields removes all column-axis fields.
func (s *Sheet) ClearPivotColumnFields(tableName string) int {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return 0
	}
	removed := len(pivot.ColFields)
	pivot.ColFields = nil
	if removed > 0 {
		s.wb.markPivotDirty(s.sheet.ID)
	}
	return removed
}

// AddPivotPageField appends a report-filter field to a pivot table.
func (s *Sheet) AddPivotPageField(tableName, fieldName string) error {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return err
	}
	pivot.PageFields = appendUniqueString(pivot.PageFields, fieldName)
	s.wb.markPivotDirty(s.sheet.ID)
	return nil
}

// RemovePivotPageField removes a report-filter field by name.
func (s *Sheet) RemovePivotPageField(tableName, fieldName string) bool {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return false
	}
	updated, removed := removeFoldStringValue(pivot.PageFields, fieldName)
	if removed {
		pivot.PageFields = updated
		s.wb.markPivotDirty(s.sheet.ID)
	}
	return removed
}

// ClearPivotPageFields removes all report-filter fields.
func (s *Sheet) ClearPivotPageFields(tableName string) int {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return 0
	}
	removed := len(pivot.PageFields)
	pivot.PageFields = nil
	if removed > 0 {
		s.wb.markPivotDirty(s.sheet.ID)
	}
	return removed
}

// AddPivotDataField appends a value field to a pivot table.
func (s *Sheet) AddPivotDataField(tableName, sourceFieldName, name, subtotal string) error {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return err
	}
	sourceFieldName = strings.TrimSpace(sourceFieldName)
	if sourceFieldName == "" {
		return fmt.Errorf("sheet edit: pivot data source field is empty")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Sum of " + sourceFieldName
	}
	subtotal = strings.TrimSpace(subtotal)
	if subtotal == "" {
		subtotal = "sum"
	}
	for _, field := range pivot.DataFields {
		if field != nil && strings.EqualFold(field.SourceFieldName, sourceFieldName) && strings.EqualFold(field.Name, name) {
			field.Subtotal = subtotal
			s.wb.markPivotDirty(s.sheet.ID)
			return nil
		}
	}
	pivot.DataFields = append(pivot.DataFields, &sheetir.PivotDataField{
		Name:            name,
		SourceFieldName: sourceFieldName,
		Subtotal:        subtotal,
	})
	s.wb.markPivotDirty(s.sheet.ID)
	return nil
}

// SetPivotDataField inserts or replaces a pivot data field.
func (s *Sheet) SetPivotDataField(tableName string, field sheetir.PivotDataField) (*sheetir.PivotDataField, error) {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizePivotDataField(field)
	if err != nil {
		return nil, err
	}
	exactIndex := -1
	firstSourceIndex := -1
	sourceMatches := 0
	for i, existing := range pivot.DataFields {
		if existing == nil || !strings.EqualFold(existing.SourceFieldName, normalized.SourceFieldName) {
			continue
		}
		sourceMatches++
		if firstSourceIndex < 0 {
			firstSourceIndex = i
		}
		if strings.EqualFold(existing.Name, normalized.Name) {
			exactIndex = i
			break
		}
	}
	switch {
	case exactIndex >= 0:
		*pivot.DataFields[exactIndex] = normalized
		s.wb.markPivotDirty(s.sheet.ID)
		return pivot.DataFields[exactIndex], nil
	case sourceMatches == 1 && firstSourceIndex >= 0:
		*pivot.DataFields[firstSourceIndex] = normalized
		s.wb.markPivotDirty(s.sheet.ID)
		return pivot.DataFields[firstSourceIndex], nil
	default:
		pivot.DataFields = append(pivot.DataFields, &normalized)
		s.wb.markPivotDirty(s.sheet.ID)
		return pivot.DataFields[len(pivot.DataFields)-1], nil
	}
}

// RemovePivotDataFields removes data fields backed by a source field name.
func (s *Sheet) RemovePivotDataFields(tableName, sourceFieldName string) int {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return 0
	}
	sourceFieldName = strings.TrimSpace(sourceFieldName)
	if sourceFieldName == "" {
		return 0
	}
	filtered := pivot.DataFields[:0]
	removed := 0
	for _, field := range pivot.DataFields {
		if field != nil && strings.EqualFold(field.SourceFieldName, sourceFieldName) {
			removed++
			continue
		}
		filtered = append(filtered, field)
	}
	pivot.DataFields = filtered
	if removed > 0 {
		s.wb.markPivotDirty(s.sheet.ID)
	}
	return removed
}

// ClearPivotDataFields removes all data fields from a pivot table.
func (s *Sheet) ClearPivotDataFields(tableName string) int {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return 0
	}
	removed := len(pivot.DataFields)
	pivot.DataFields = nil
	if removed > 0 {
		s.wb.markPivotDirty(s.sheet.ID)
	}
	return removed
}

// SetPivotItemHidden updates one pivot item visibility by caption. It preserves
// the semantic filter through pivot-cache regeneration on save.
func (s *Sheet) SetPivotItemHidden(tableName, fieldName, itemName string, hidden bool) error {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return err
	}
	field := findPivotField(pivot, fieldName)
	if field == nil {
		return fmt.Errorf("sheet edit: pivot field %q not found", fieldName)
	}
	itemName = strings.TrimSpace(itemName)
	for _, item := range field.Items {
		if item != nil && strings.EqualFold(item.Name, itemName) {
			item.Hidden = hidden
			s.wb.markPivotDirty(s.sheet.ID)
			return nil
		}
	}
	return fmt.Errorf("sheet edit: pivot item %q not found in field %q", itemName, fieldName)
}

// SetPivotPageSelection sets the selected pivot item for a report-filter field.
func (s *Sheet) SetPivotPageSelection(tableName, fieldName string, itemIndex int) error {
	pivot, err := s.requirePivotTable(tableName)
	if err != nil {
		return err
	}
	if !containsFoldString(pivot.PageFields, fieldName) {
		return fmt.Errorf("sheet edit: pivot page field %q not found", fieldName)
	}
	for _, selection := range pivot.PageFieldSelections {
		if selection != nil && strings.EqualFold(selection.FieldName, fieldName) {
			selection.ItemIndex = itemIndex
			s.wb.markPivotDirty(s.sheet.ID)
			return nil
		}
	}
	pivot.PageFieldSelections = append(pivot.PageFieldSelections, &sheetir.PivotPageFieldSelection{FieldName: strings.TrimSpace(fieldName), ItemIndex: itemIndex})
	s.wb.markPivotDirty(s.sheet.ID)
	return nil
}

// PivotField returns semantic pivot-field metadata by name.
func (s *Sheet) PivotField(tableName, fieldName string) (*sheetir.PivotField, bool) {
	pivot, ok := s.PivotTable(tableName)
	if !ok {
		return nil, false
	}
	field := findPivotField(pivot, fieldName)
	return field, field != nil
}

func findPivotField(pivot *sheetir.PivotTable, name string) *sheetir.PivotField {
	if pivot == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	for _, field := range pivot.Fields {
		if field != nil && strings.EqualFold(field.Name, name) {
			return field
		}
	}
	return nil
}

func containsFoldString(values []string, value string) bool {
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return true
		}
	}
	return false
}

// RefreshPivotTable marks one pivot table for cache regeneration on save.
func (s *Sheet) RefreshPivotTable(name string) error {
	if _, err := s.requirePivotTable(name); err != nil {
		return err
	}
	s.wb.markPivotDirty(s.sheet.ID)
	return nil
}

// RefreshPivotTables marks all worksheet pivot tables for cache regeneration on save.
func (s *Sheet) RefreshPivotTables() int {
	if s == nil || s.sheet == nil || len(s.sheet.PivotTables) == 0 {
		return 0
	}
	s.wb.markPivotDirty(s.sheet.ID)
	return len(s.sheet.PivotTables)
}

// RemovePivotTable removes a pivot table by name.
func (s *Sheet) RemovePivotTable(name string) bool {
	if s == nil || s.sheet == nil {
		return false
	}
	name = strings.TrimSpace(name)
	for i, pivot := range s.sheet.PivotTables {
		if pivot != nil && strings.EqualFold(pivot.Name, name) {
			s.sheet.PivotTables = append(s.sheet.PivotTables[:i], s.sheet.PivotTables[i+1:]...)
			s.wb.markPivotDirty(s.sheet.ID)
			return true
		}
	}
	return false
}

func (s *Sheet) requirePivotTable(name string) (*sheetir.PivotTable, error) {
	pivot, ok := s.PivotTable(name)
	if !ok {
		return nil, fmt.Errorf("sheet edit: pivot table %q not found", name)
	}
	return pivot, nil
}

func (s *Sheet) resolvePivotSourceSheetName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("sheet edit: pivot source sheet is empty")
	}
	source, ok := s.wb.SheetByName(name)
	if !ok || source == nil || source.sheet == nil {
		return "", fmt.Errorf("sheet edit: pivot source sheet %q not found", name)
	}
	return source.sheet.Name, nil
}

func (s *Sheet) resolvePivotDefinedName(name string) (string, error) {
	if s == nil || s.wb == nil || s.sheet == nil {
		return "", fmt.Errorf("sheet edit: nil sheet")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("sheet edit: pivot source name is empty")
	}
	if strings.Contains(name, "!") {
		return "", fmt.Errorf("sheet edit: pivot source name %q must be an unqualified defined name", name)
	}
	if _, ok := s.wb.DefinedName(name, &s.index); ok {
		return name, nil
	}
	if _, ok := s.wb.DefinedName(name, nil); ok {
		return name, nil
	}
	return "", fmt.Errorf("sheet edit: pivot source name %q not found", name)
}

func (s *Sheet) nextPivotTableName() string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("PivotTable%d", i)
		if _, ok := s.PivotTable(name); !ok {
			return name
		}
	}
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}

func removeFoldStringValue(values []string, value string) ([]string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return values, false
	}
	for i, existing := range values {
		if strings.EqualFold(existing, value) {
			return append(values[:i], values[i+1:]...), true
		}
	}
	return values, false
}

func normalizePivotDataField(field sheetir.PivotDataField) (sheetir.PivotDataField, error) {
	field.SourceFieldName = strings.TrimSpace(field.SourceFieldName)
	if field.SourceFieldName == "" {
		return sheetir.PivotDataField{}, fmt.Errorf("sheet edit: pivot data source field is empty")
	}
	field.Name = strings.TrimSpace(field.Name)
	if field.Name == "" {
		field.Name = "Sum of " + field.SourceFieldName
	}
	field.Subtotal = strings.TrimSpace(field.Subtotal)
	if field.Subtotal == "" {
		field.Subtotal = "sum"
	}
	return field, nil
}
