package edit

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	legacytoooxml "office/pkg/sheet/convert/legacytoooxml"
	sheetir "office/pkg/sheet/ir"
	sheetooxml "office/pkg/sheet/parsers/ooxml"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

// Workbook is an editable wrapper around the spreadsheet IR.
type Workbook struct {
	ir               *sheetir.Workbook
	selected         map[int]bool
	styles           map[string]*CellStyle
	formats          map[string]*DataFormat
	seeded           bool
	legacyDrawingVML map[int][]byte
	vmlDirty         map[int]bool
	legacyPivotParts map[int][]*sheetparts.PivotTableWithCache
	pivotDirty       map[int]bool
}

// Sheet is an editable worksheet wrapper.
type Sheet struct {
	wb    *Workbook
	index int
	sheet *sheetir.Sheet
}

// Row is an editable row wrapper.
type Row struct {
	sheet *Sheet
	row   *sheetir.Row
}

// Cell is an editable cell wrapper.
type Cell struct {
	sheet *Sheet
	cell  *sheetir.Cell
}

// NewWorkbook creates an empty editable workbook.
func NewWorkbook() *Workbook {
	return &Workbook{
		ir:               &sheetir.Workbook{},
		selected:         make(map[int]bool),
		styles:           make(map[string]*CellStyle),
		formats:          make(map[string]*DataFormat),
		legacyDrawingVML: make(map[int][]byte),
		vmlDirty:         make(map[int]bool),
		legacyPivotParts: make(map[int][]*sheetparts.PivotTableWithCache),
		pivotDirty:       make(map[int]bool),
	}
}

// OpenIR wraps an existing workbook IR for editing.
func OpenIR(ir *sheetir.Workbook) (*Workbook, error) {
	if ir == nil {
		return nil, fmt.Errorf("sheet edit: nil workbook")
	}
	wb := &Workbook{
		ir:               ir,
		selected:         make(map[int]bool),
		styles:           make(map[string]*CellStyle),
		formats:          make(map[string]*DataFormat),
		legacyDrawingVML: make(map[int][]byte),
		vmlDirty:         make(map[int]bool),
		legacyPivotParts: make(map[int][]*sheetparts.PivotTableWithCache),
		pivotDirty:       make(map[int]bool),
	}
	if ir.ActiveSheetIndex != nil && *ir.ActiveSheetIndex >= 0 && *ir.ActiveSheetIndex < len(ir.Sheets) {
		wb.selected[*ir.ActiveSheetIndex] = true
	}
	return wb, nil
}

// OpenXLSX loads an XLSX package into editable IR.
func OpenXLSX(r io.ReaderAt, size int64) (*Workbook, error) {
	ir, xlPkg, err := sheetooxml.ParseReaderAtWithPackage(r, size)
	if err != nil {
		return nil, err
	}
	wb, err := OpenIR(ir)
	if err != nil {
		return nil, err
	}
	wb.seedLegacyDrawingVML(xlPkg)
	wb.seedLegacyPivotTables(xlPkg)
	return wb, nil
}

// IR returns the underlying workbook IR.
func (w *Workbook) IR() *sheetir.Workbook {
	if w == nil {
		return nil
	}
	return w.ir
}

// SheetCount returns the number of worksheet tabs.
func (w *Workbook) SheetCount() int {
	if w == nil || w.ir == nil {
		return 0
	}
	return len(w.ir.Sheets)
}

// Sheets returns editable wrappers for every worksheet in tab order.
func (w *Workbook) Sheets() []*Sheet {
	out := make([]*Sheet, 0, w.SheetCount())
	for i, sheet := range w.ir.Sheets {
		out = append(out, &Sheet{wb: w, index: i, sheet: sheet})
	}
	return out
}

// Sheet returns the worksheet at index.
func (w *Workbook) Sheet(index int) (*Sheet, error) {
	if err := w.checkSheetIndex(index); err != nil {
		return nil, err
	}
	return &Sheet{wb: w, index: index, sheet: w.ir.Sheets[index]}, nil
}

// SheetByName returns the worksheet matching name, case-insensitively.
func (w *Workbook) SheetByName(name string) (*Sheet, bool) {
	if w == nil || w.ir == nil {
		return nil, false
	}
	for i, sheet := range w.ir.Sheets {
		if strings.EqualFold(sheet.Name, name) {
			return &Sheet{wb: w, index: i, sheet: sheet}, true
		}
	}
	return nil, false
}

// CreateSheet appends a new worksheet.
func (w *Workbook) CreateSheet(name string) (*Sheet, error) {
	if w == nil || w.ir == nil {
		return nil, fmt.Errorf("sheet edit: nil workbook")
	}
	name = strings.TrimSpace(name)
	if strings.TrimSpace(name) == "" {
		name = w.nextSheetName()
	}
	if err := w.validateSheetName(name, -1); err != nil {
		return nil, err
	}
	sheet := &sheetir.Sheet{
		Name:         name,
		ID:           w.nextSheetID(),
		ColumnWidths: make(map[int]float64),
		HiddenCols:   make(map[int]bool),
		Comments:     make(map[string]string),
	}
	w.ir.Sheets = append(w.ir.Sheets, sheet)
	index := len(w.ir.Sheets) - 1
	if w.ir.ActiveSheetIndex == nil {
		if err := w.SetActiveSheet(index); err != nil {
			return nil, err
		}
	}
	return &Sheet{wb: w, index: index, sheet: sheet}, nil
}

// CloneSheet copies a worksheet and inserts the copy after the original.
func (w *Workbook) CloneSheet(index int, name string) (*Sheet, error) {
	if err := w.checkSheetIndex(index); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if strings.TrimSpace(name) == "" {
		name = w.nextSheetName()
	}
	if err := w.validateSheetName(name, -1); err != nil {
		return nil, err
	}
	clone := cloneSheet(w.ir.Sheets[index])
	clone.Name = name
	clone.ID = w.nextSheetID()
	insertAt := index + 1
	w.ir.Sheets = append(w.ir.Sheets, nil)
	copy(w.ir.Sheets[insertAt+1:], w.ir.Sheets[insertAt:])
	w.ir.Sheets[insertAt] = clone
	w.shiftSelectedForInsert(insertAt)
	w.normalizeActiveAfterInsert(insertAt)
	return &Sheet{wb: w, index: insertAt, sheet: clone}, nil
}

// RemoveSheet removes the worksheet at index.
func (w *Workbook) RemoveSheet(index int) error {
	if err := w.checkSheetIndex(index); err != nil {
		return err
	}
	w.ir.Sheets = append(w.ir.Sheets[:index], w.ir.Sheets[index+1:]...)
	w.shiftSelectedForRemove(index)
	if len(w.ir.Sheets) == 0 {
		w.ir.ActiveSheetIndex = nil
		return nil
	}
	active := 0
	if w.ir.ActiveSheetIndex != nil {
		active = *w.ir.ActiveSheetIndex
	}
	switch {
	case active == index:
		if active >= len(w.ir.Sheets) {
			active = len(w.ir.Sheets) - 1
		}
	case active > index:
		active--
	}
	return w.SetActiveSheet(active)
}

// RenameSheet renames the worksheet at index.
func (w *Workbook) RenameSheet(index int, name string) error {
	if err := w.checkSheetIndex(index); err != nil {
		return err
	}
	if err := w.validateSheetName(name, index); err != nil {
		return err
	}
	w.ir.Sheets[index].Name = strings.TrimSpace(name)
	return nil
}

// MoveSheet moves a worksheet from one index to another.
func (w *Workbook) MoveSheet(from, to int) error {
	if err := w.checkSheetIndex(from); err != nil {
		return err
	}
	if err := w.checkSheetIndex(to); err != nil {
		return err
	}
	if from == to {
		return nil
	}
	sheet := w.ir.Sheets[from]
	w.ir.Sheets = append(w.ir.Sheets[:from], w.ir.Sheets[from+1:]...)
	w.ir.Sheets = append(w.ir.Sheets, nil)
	copy(w.ir.Sheets[to+1:], w.ir.Sheets[to:])
	w.ir.Sheets[to] = sheet
	w.remapSelectionAfterMove(from, to)
	if w.ir.ActiveSheetIndex != nil {
		active := remapMovedIndex(*w.ir.ActiveSheetIndex, from, to)
		w.ir.ActiveSheetIndex = &active
	}
	return nil
}

// SetActiveSheet marks the active workbook tab.
func (w *Workbook) SetActiveSheet(index int) error {
	if err := w.checkSheetIndex(index); err != nil {
		return err
	}
	v := index
	w.ir.ActiveSheetIndex = &v
	w.selected[index] = true
	return nil
}

// ActiveSheetIndex returns the active tab index, or -1 when unset.
func (w *Workbook) ActiveSheetIndex() int {
	if w == nil || w.ir == nil || w.ir.ActiveSheetIndex == nil {
		return -1
	}
	return *w.ir.ActiveSheetIndex
}

// SelectSheet updates the in-memory selected-sheet set used by edit callers.
func (w *Workbook) SelectSheet(index int, selected bool) error {
	if err := w.checkSheetIndex(index); err != nil {
		return err
	}
	if selected {
		w.selected[index] = true
	} else {
		delete(w.selected, index)
	}
	return nil
}

// SelectedSheetIndexes returns selected worksheet indexes in ascending order.
func (w *Workbook) SelectedSheetIndexes() []int {
	out := make([]int, 0, len(w.selected))
	for idx := range w.selected {
		if idx >= 0 && idx < w.SheetCount() {
			out = append(out, idx)
		}
	}
	sort.Ints(out)
	return out
}

// SaveAsXLSX writes the edited workbook as an XLSX package.
func (w *Workbook) SaveAsXLSX(out io.Writer) error {
	if w == nil || w.ir == nil {
		return fmt.Errorf("sheet edit: nil workbook")
	}
	if len(w.ir.Sheets) == 0 {
		return fmt.Errorf("sheet edit: workbook has no sheets")
	}
	w.normalizeForSave()
	pkg, err := legacytoooxml.Convert(w.ir)
	if err != nil {
		return err
	}
	w.applyLegacyDrawingVML(pkg)
	w.applyLegacyPivotTables(pkg)
	return sheetparts.WritePackage(pkg, out)
}

// Name returns the worksheet tab name.
func (s *Sheet) Name() string {
	if s == nil || s.sheet == nil {
		return ""
	}
	return s.sheet.Name
}

// Hidden reports whether the worksheet tab is hidden.
func (s *Sheet) Hidden() bool {
	if s == nil || s.sheet == nil {
		return false
	}
	return s.sheet.Hidden
}

// Visibility returns the worksheet tab visibility state: "", "hidden", or "veryHidden".
func (s *Sheet) Visibility() string {
	if s == nil || s.sheet == nil {
		return ""
	}
	return s.sheet.Visibility
}

// SetHidden marks the worksheet tab hidden or visible.
func (s *Sheet) SetHidden(hidden bool) {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.Hidden = hidden
	if hidden {
		s.sheet.Visibility = "hidden"
	} else {
		s.sheet.Visibility = ""
	}
}

// SetVisibility sets the worksheet tab visibility state.
func (s *Sheet) SetVisibility(visibility string) error {
	if s == nil || s.sheet == nil {
		return fmt.Errorf("sheet edit: nil sheet")
	}
	switch visibility {
	case "", "visible":
		s.sheet.Hidden = false
		s.sheet.Visibility = ""
	case "hidden", "veryHidden":
		s.sheet.Hidden = true
		s.sheet.Visibility = visibility
	default:
		return fmt.Errorf("sheet edit: visibility must be visible, hidden, veryHidden, or empty")
	}
	return nil
}

// SetName renames the worksheet.
func (s *Sheet) SetName(name string) error {
	if s == nil || s.wb == nil {
		return fmt.Errorf("sheet edit: nil sheet")
	}
	return s.wb.RenameSheet(s.index, name)
}

// Rows returns editable row wrappers in row order.
func (s *Sheet) Rows() []*Row {
	out := make([]*Row, 0, len(s.sheet.Rows))
	for _, row := range s.sheet.Rows {
		out = append(out, &Row{sheet: s, row: row})
	}
	return out
}

// Row returns the row with the 1-based index.
func (s *Sheet) Row(index int) (*Row, bool) {
	if s == nil || s.sheet == nil {
		return nil, false
	}
	for _, row := range s.sheet.Rows {
		if row.Index == index {
			return &Row{sheet: s, row: row}, true
		}
	}
	return nil, false
}

// CreateRow returns the row with the 1-based index, creating it when needed.
func (s *Sheet) CreateRow(index int) (*Row, error) {
	if index < 1 {
		return nil, fmt.Errorf("sheet edit: row index must be >= 1")
	}
	if row, ok := s.Row(index); ok {
		return row, nil
	}
	row := &sheetir.Row{Index: index}
	s.sheet.Rows = append(s.sheet.Rows, row)
	sortRows(s.sheet.Rows)
	return &Row{sheet: s, row: row}, nil
}

// RemoveRow removes a row by its 1-based index.
func (s *Sheet) RemoveRow(index int) bool {
	for i, row := range s.sheet.Rows {
		if row.Index == index {
			s.sheet.Rows = append(s.sheet.Rows[:i], s.sheet.Rows[i+1:]...)
			return true
		}
	}
	return false
}

// Cell returns a cell by 1-based row and column.
func (s *Sheet) Cell(row, col int) (*Cell, bool) {
	r, ok := s.Row(row)
	if !ok {
		return nil, false
	}
	return r.Cell(col)
}

// CreateCell returns a cell by 1-based row and column, creating row and cell when needed.
func (s *Sheet) CreateCell(row, col int) (*Cell, error) {
	r, err := s.CreateRow(row)
	if err != nil {
		return nil, err
	}
	return r.CreateCell(col)
}

// Dimensions returns the used row and column bounds.
func (s *Sheet) Dimensions() (firstRow, lastRow, firstCol, lastCol int) {
	s.recalculateDimensions()
	return s.sheet.DimFirstRow, s.sheet.DimLastRow, s.sheet.DimFirstCol, s.sheet.DimLastCol
}

// SetColumnWidth sets an explicit column width in Excel character units.
func (s *Sheet) SetColumnWidth(col int, width float64) error {
	if col < 1 {
		return fmt.Errorf("sheet edit: column index must be >= 1")
	}
	if width < 0 {
		return fmt.Errorf("sheet edit: column width must be >= 0")
	}
	if s.sheet.ColumnWidths == nil {
		s.sheet.ColumnWidths = make(map[int]float64)
	}
	if width == 0 {
		delete(s.sheet.ColumnWidths, col)
	} else {
		s.sheet.ColumnWidths[col] = width
	}
	return nil
}

// SetColumnHidden marks a column hidden or visible.
func (s *Sheet) SetColumnHidden(col int, hidden bool) error {
	if col < 1 {
		return fmt.Errorf("sheet edit: column index must be >= 1")
	}
	if s.sheet.HiddenCols == nil {
		s.sheet.HiddenCols = make(map[int]bool)
	}
	if hidden {
		s.sheet.HiddenCols[col] = true
	} else {
		delete(s.sheet.HiddenCols, col)
	}
	return nil
}

// DefaultColumnWidthChars returns the worksheet default column width in Excel character units.
func (s *Sheet) DefaultColumnWidthChars() float64 {
	if s == nil || s.sheet == nil {
		return 0
	}
	return s.sheet.DefaultColWidthChars
}

// SetDefaultColumnWidthChars sets the worksheet default column width in Excel character units.
func (s *Sheet) SetDefaultColumnWidthChars(width float64) error {
	if width < 0 {
		return fmt.Errorf("sheet edit: default column width must be >= 0")
	}
	s.sheet.DefaultColWidthChars = width
	return nil
}

// DefaultRowHeightPt returns the worksheet default row height in points.
func (s *Sheet) DefaultRowHeightPt() float64 {
	if s == nil || s.sheet == nil {
		return 0
	}
	return s.sheet.DefaultRowHeightPt
}

// SetDefaultRowHeightPt sets the worksheet default row height in points.
func (s *Sheet) SetDefaultRowHeightPt(height float64) error {
	if height < 0 {
		return fmt.Errorf("sheet edit: default row height must be >= 0")
	}
	s.sheet.DefaultRowHeightPt = height
	s.sheet.DefaultRowHeightUnsynced = height > 0
	return nil
}

// ZoomScale returns the worksheet zoom percentage, or 0 when unset.
func (s *Sheet) ZoomScale() int {
	if s == nil || s.sheet == nil {
		return 0
	}
	return s.sheet.ZoomScale
}

// SetZoomScale sets the worksheet zoom percentage. Use 0 to clear it.
func (s *Sheet) SetZoomScale(zoom int) error {
	if zoom < 0 || (zoom != 0 && (zoom < 10 || zoom > 400)) {
		return fmt.Errorf("sheet edit: zoom scale must be 0 or 10..400")
	}
	s.sheet.ZoomScale = zoom
	return nil
}

// GridLinesVisible returns the worksheet gridline visibility when explicitly set.
func (s *Sheet) GridLinesVisible() (visible, ok bool) {
	if s == nil || s.sheet == nil || s.sheet.ShowGridLines == nil {
		return false, false
	}
	return *s.sheet.ShowGridLines, true
}

// SetGridLinesVisible sets the worksheet gridline visibility.
func (s *Sheet) SetGridLinesVisible(visible bool) {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.ShowGridLines = &visible
}

// ClearGridLinesVisible removes the explicit worksheet gridline visibility setting.
func (s *Sheet) ClearGridLinesVisible() {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.ShowGridLines = nil
}

// SetRowOutlineLevel sets a row outline level in the range 0..7.
func (s *Sheet) SetRowOutlineLevel(row, level int) error {
	if level < 0 || level > 7 {
		return fmt.Errorf("sheet edit: row outline level must be 0..7")
	}
	r, err := s.CreateRow(row)
	if err != nil {
		return err
	}
	r.row.OutlineLevel = level
	return nil
}

// SetColumnOutlineLevel sets a column outline level in the range 0..7.
func (s *Sheet) SetColumnOutlineLevel(col, level int) error {
	if col < 1 {
		return fmt.Errorf("sheet edit: column index must be >= 1")
	}
	if level < 0 || level > 7 {
		return fmt.Errorf("sheet edit: column outline level must be 0..7")
	}
	if s.sheet.OutlineCols == nil {
		s.sheet.OutlineCols = make(map[int]int)
	}
	if level == 0 {
		delete(s.sheet.OutlineCols, col)
	} else {
		s.sheet.OutlineCols[col] = level
	}
	return nil
}

// FreezePanes configures frozen rows/columns for the default sheet view.
func (s *Sheet) FreezePanes(rows, cols int, topLeftCell, activePane string) error {
	if rows < 0 || cols < 0 {
		return fmt.Errorf("sheet edit: frozen rows/columns must be >= 0")
	}
	s.sheet.FrozenRows = rows
	s.sheet.FrozenCols = cols
	s.sheet.SplitRows = 0
	s.sheet.SplitCols = 0
	s.sheet.TopLeftCell = topLeftCell
	s.sheet.ActivePane = activePane
	return nil
}

// SplitPanes configures split rows/columns for the default sheet view.
func (s *Sheet) SplitPanes(rows, cols int, topLeftCell, activePane string) error {
	if rows < 0 || cols < 0 {
		return fmt.Errorf("sheet edit: split rows/columns must be >= 0")
	}
	s.sheet.SplitRows = rows
	s.sheet.SplitCols = cols
	s.sheet.FrozenRows = 0
	s.sheet.FrozenCols = 0
	s.sheet.TopLeftCell = topLeftCell
	s.sheet.ActivePane = activePane
	return nil
}

// AddMergedRegion appends a merged range by 1-based coordinates.
func (s *Sheet) AddMergedRegion(topRow, leftCol, bottomRow, rightCol int) error {
	if topRow < 1 || leftCol < 1 || bottomRow < topRow || rightCol < leftCol {
		return fmt.Errorf("sheet edit: invalid merged region")
	}
	s.sheet.MergedCells = append(s.sheet.MergedCells, &sheetir.MergedCell{
		TopRow:    topRow,
		LeftCol:   leftCol,
		BottomRow: bottomRow,
		RightCol:  rightCol,
	})
	return nil
}

// ClearMergedRegions removes all merged ranges.
func (s *Sheet) ClearMergedRegions() {
	s.sheet.MergedCells = nil
}

// SetAutoFilter sets or clears the worksheet auto-filter range.
func (s *Sheet) SetAutoFilter(ref string) {
	s.sheet.AutoFilterRef = ref
	if ref == "" {
		s.sheet.FilterColumns = nil
	}
}

// SetPageSetup replaces the worksheet print setup.
func (s *Sheet) SetPageSetup(setup *sheetir.PageSetup) {
	s.sheet.PageSetup = setup
}

// PageSetup returns a copy of the worksheet print setup, or nil when unset.
func (s *Sheet) PageSetup() *sheetir.PageSetup {
	if s == nil || s.sheet == nil || s.sheet.PageSetup == nil {
		return nil
	}
	return clonePageSetup(s.sheet.PageSetup)
}

// ClearPageSetup removes the worksheet print setup.
func (s *Sheet) ClearPageSetup() {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.PageSetup = nil
}

// SetHeaderFooter replaces the worksheet header/footer settings.
func (s *Sheet) SetHeaderFooter(headerFooter *sheetir.HeaderFooter) {
	s.sheet.HeaderFooter = headerFooter
}

// HeaderFooter returns a copy of the worksheet header/footer settings, or nil when unset.
func (s *Sheet) HeaderFooter() *sheetir.HeaderFooter {
	if s == nil || s.sheet == nil || s.sheet.HeaderFooter == nil {
		return nil
	}
	return cloneHeaderFooter(s.sheet.HeaderFooter)
}

// ClearHeaderFooter removes the worksheet header/footer settings.
func (s *Sheet) ClearHeaderFooter() {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.HeaderFooter = nil
}

// PrintArea returns the worksheet print area range or comma-separated ranges.
func (s *Sheet) PrintArea() string {
	if s == nil || s.sheet == nil {
		return ""
	}
	return s.sheet.PrintArea
}

// SetPrintArea updates the worksheet print area range or comma-separated ranges.
func (s *Sheet) SetPrintArea(area string) {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.PrintArea = strings.TrimSpace(area)
}

// ClearPrintArea removes the worksheet print area range.
func (s *Sheet) ClearPrintArea() {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.PrintArea = ""
}

// PrintTitles returns the worksheet row and column repeat ranges used for printing.
func (s *Sheet) PrintTitles() (rows, cols string) {
	if s == nil || s.sheet == nil {
		return "", ""
	}
	return s.sheet.PrintTitleRows, s.sheet.PrintTitleCols
}

// SetPrintTitles updates the worksheet row and column repeat ranges used for printing.
func (s *Sheet) SetPrintTitles(rows, cols string) {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.PrintTitleRows = strings.TrimSpace(rows)
	s.sheet.PrintTitleCols = strings.TrimSpace(cols)
}

// ClearPrintTitles removes the worksheet print-title ranges.
func (s *Sheet) ClearPrintTitles() {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.PrintTitleRows = ""
	s.sheet.PrintTitleCols = ""
}

// AddRowPageBreak adds a manual page break before the 1-based row.
func (s *Sheet) AddRowPageBreak(row int) error {
	if row < 1 {
		return fmt.Errorf("sheet edit: row index must be >= 1")
	}
	s.sheet.RowPageBreaks = appendUniqueInt(s.sheet.RowPageBreaks, row)
	return nil
}

// RowPageBreaks returns the manual row page breaks in ascending order.
func (s *Sheet) RowPageBreaks() []int {
	if s == nil || s.sheet == nil || len(s.sheet.RowPageBreaks) == 0 {
		return nil
	}
	return append([]int(nil), s.sheet.RowPageBreaks...)
}

// RemoveRowPageBreak removes one manual row page break.
func (s *Sheet) RemoveRowPageBreak(row int) bool {
	if s == nil || s.sheet == nil {
		return false
	}
	var removed bool
	s.sheet.RowPageBreaks, removed = removeIntValue(s.sheet.RowPageBreaks, row)
	return removed
}

// ClearRowPageBreaks removes all manual row page breaks.
func (s *Sheet) ClearRowPageBreaks() {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.RowPageBreaks = nil
}

// AddColumnPageBreak adds a manual page break before the 1-based column.
func (s *Sheet) AddColumnPageBreak(col int) error {
	if col < 1 {
		return fmt.Errorf("sheet edit: column index must be >= 1")
	}
	s.sheet.ColPageBreaks = appendUniqueInt(s.sheet.ColPageBreaks, col)
	return nil
}

// ColumnPageBreaks returns the manual column page breaks in ascending order.
func (s *Sheet) ColumnPageBreaks() []int {
	if s == nil || s.sheet == nil || len(s.sheet.ColPageBreaks) == 0 {
		return nil
	}
	return append([]int(nil), s.sheet.ColPageBreaks...)
}

// RemoveColumnPageBreak removes one manual column page break.
func (s *Sheet) RemoveColumnPageBreak(col int) bool {
	if s == nil || s.sheet == nil {
		return false
	}
	var removed bool
	s.sheet.ColPageBreaks, removed = removeIntValue(s.sheet.ColPageBreaks, col)
	return removed
}

// ClearColumnPageBreaks removes all manual column page breaks.
func (s *Sheet) ClearColumnPageBreaks() {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.ColPageBreaks = nil
}

// Index returns the 1-based row index.
func (r *Row) Index() int {
	if r == nil || r.row == nil {
		return 0
	}
	return r.row.Index
}

// Cells returns editable cell wrappers in column order.
func (r *Row) Cells() []*Cell {
	out := make([]*Cell, 0, len(r.row.Cells))
	for _, cell := range r.row.Cells {
		out = append(out, &Cell{sheet: r.sheet, cell: cell})
	}
	return out
}

// Cell returns the cell at a 1-based column.
func (r *Row) Cell(col int) (*Cell, bool) {
	if r == nil || r.row == nil {
		return nil, false
	}
	for _, cell := range r.row.Cells {
		if cell.Col == col {
			return &Cell{sheet: r.sheet, cell: cell}, true
		}
	}
	return nil, false
}

// CreateCell returns the cell at a 1-based column, creating it when needed.
func (r *Row) CreateCell(col int) (*Cell, error) {
	if col < 1 {
		return nil, fmt.Errorf("sheet edit: column index must be >= 1")
	}
	if cell, ok := r.Cell(col); ok {
		return cell, nil
	}
	cell := &sheetir.Cell{
		Row: r.row.Index,
		Col: col,
		Ref: cellRef(r.row.Index, col),
	}
	r.row.Cells = append(r.row.Cells, cell)
	sortCells(r.row.Cells)
	return &Cell{sheet: r.sheet, cell: cell}, nil
}

// RemoveCell removes a cell at a 1-based column.
func (r *Row) RemoveCell(col int) bool {
	for i, cell := range r.row.Cells {
		if cell.Col == col {
			r.row.Cells = append(r.row.Cells[:i], r.row.Cells[i+1:]...)
			return true
		}
	}
	return false
}

// SetHeightPt sets the row height in points.
func (r *Row) SetHeightPt(height float64) error {
	if height < 0 {
		return fmt.Errorf("sheet edit: row height must be >= 0")
	}
	r.row.HeightPt = height
	return nil
}

// SetHidden marks a row hidden or visible.
func (r *Row) SetHidden(hidden bool) {
	r.row.Hidden = hidden
}

// SetCollapsed marks a row as an outline collapse boundary.
func (r *Row) SetCollapsed(collapsed bool) {
	r.row.Collapsed = collapsed
}

// SetOutlineLevel sets the row outline level in the range 0..7.
func (r *Row) SetOutlineLevel(level int) error {
	if level < 0 || level > 7 {
		return fmt.Errorf("sheet edit: row outline level must be 0..7")
	}
	r.row.OutlineLevel = level
	return nil
}

// SetStyle replaces the row-level style.
func (r *Row) SetStyle(style *sheetir.CellStyle) {
	r.row.Style = style
}

// Ref returns the A1-style cell reference.
func (c *Cell) Ref() string {
	if c == nil || c.cell == nil {
		return ""
	}
	if c.cell.Ref == "" {
		c.cell.Ref = cellRef(c.cell.Row, c.cell.Col)
	}
	return c.cell.Ref
}

// SetString stores a plain string value.
func (c *Cell) SetString(value string) {
	c.cell.Kind = sheetir.CellString
	c.cell.StrValue = value
	c.cell.NumValue = 0
	c.cell.Formula = ""
	c.cell.RichTextRuns = nil
}

// SetRichText stores rich text runs as an inline rich-text cell.
func (c *Cell) SetRichText(runs []*sheetir.RichTextRun) {
	c.cell.Kind = sheetir.CellString
	c.cell.RichTextRuns = cloneRichTextRuns(runs)
	c.cell.StrValue = plainTextFromRuns(runs)
	c.cell.NumValue = 0
	c.cell.Formula = ""
}

// SetNumber stores a numeric value.
func (c *Cell) SetNumber(value float64) {
	c.cell.Kind = sheetir.CellNumber
	c.cell.NumValue = value
	c.cell.StrValue = ""
	c.cell.Formula = ""
	c.cell.RichTextRuns = nil
}

// SetBool stores a boolean value.
func (c *Cell) SetBool(value bool) {
	c.cell.Kind = sheetir.CellBool
	if value {
		c.cell.NumValue = 1
	} else {
		c.cell.NumValue = 0
	}
	c.cell.StrValue = ""
	c.cell.Formula = ""
	c.cell.RichTextRuns = nil
}

// SetDate stores a date/time cell using an Excel serial value and ISO display text.
func (c *Cell) SetDate(serial float64, isoValue string) {
	c.cell.Kind = sheetir.CellDate
	c.cell.NumValue = serial
	c.cell.StrValue = isoValue
	c.cell.Formula = ""
	c.cell.RichTextRuns = nil
}

// SetError stores an Excel error value such as #DIV/0!.
func (c *Cell) SetError(value string) {
	c.cell.Kind = sheetir.CellError
	c.cell.StrValue = value
	c.cell.NumValue = 0
	c.cell.Formula = ""
	c.cell.RichTextRuns = nil
}

// SetFormula stores a formula and optional cached string result.
func (c *Cell) SetFormula(formula, cachedString string) {
	c.cell.Kind = sheetir.CellFormula
	c.cell.Formula = formula
	c.cell.StrValue = cachedString
	c.cell.NumValue = 0
	c.cell.RichTextRuns = nil
}

// SetBlank clears the cell value and formula.
func (c *Cell) SetBlank() {
	c.cell.Kind = sheetir.CellEmpty
	c.cell.StrValue = ""
	c.cell.NumValue = 0
	c.cell.Formula = ""
	c.cell.RichTextRuns = nil
}

// SetHyperlink sets or clears the cell hyperlink URL.
func (c *Cell) SetHyperlink(url string) {
	c.cell.HyperlinkURL = url
}

// SetComment sets or clears the cell comment text.
func (c *Cell) SetComment(text string) {
	if c == nil || c.sheet == nil {
		return
	}
	_ = c.sheet.SetComment(c.Ref(), text)
}

// SetStyle replaces the cell style.
func (c *Cell) SetStyle(style sheetir.CellStyle) {
	c.cell.Style = style
}

func (w *Workbook) checkSheetIndex(index int) error {
	if w == nil || w.ir == nil {
		return fmt.Errorf("sheet edit: nil workbook")
	}
	if index < 0 || index >= len(w.ir.Sheets) {
		return fmt.Errorf("sheet edit: sheet index %d out of range", index)
	}
	return nil
}

func (w *Workbook) validateSheetName(name string, allowIndex int) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("sheet edit: sheet name is empty")
	}
	if len([]rune(name)) > 31 {
		return fmt.Errorf("sheet edit: sheet name %q exceeds 31 characters", name)
	}
	if strings.ContainsAny(name, `[]:*?/\`) {
		return fmt.Errorf("sheet edit: sheet name %q contains invalid characters", name)
	}
	for i, sheet := range w.ir.Sheets {
		if i != allowIndex && strings.EqualFold(sheet.Name, name) {
			return fmt.Errorf("sheet edit: sheet name %q already exists", name)
		}
	}
	return nil
}

func (w *Workbook) nextSheetName() string {
	for i := 1; ; i++ {
		name := "Sheet" + strconv.Itoa(i)
		if _, ok := w.SheetByName(name); !ok {
			return name
		}
	}
}

func (w *Workbook) nextSheetID() int {
	maxID := 0
	for _, sheet := range w.ir.Sheets {
		if sheet.ID > maxID {
			maxID = sheet.ID
		}
	}
	return maxID + 1
}

func (w *Workbook) normalizeForSave() {
	w.syncNamedCellStyles()
	for _, sheet := range w.ir.Sheets {
		s := &Sheet{wb: w, sheet: sheet}
		s.recalculateDimensions()
		sortRows(sheet.Rows)
		for _, row := range sheet.Rows {
			sortCells(row.Cells)
			for _, cell := range row.Cells {
				cell.Row = row.Index
				if cell.Ref == "" || cell.Ref != cellRef(cell.Row, cell.Col) {
					cell.Ref = cellRef(cell.Row, cell.Col)
				}
			}
		}
	}
	if w.ir.ActiveSheetIndex == nil && len(w.ir.Sheets) > 0 {
		v := 0
		w.ir.ActiveSheetIndex = &v
	}
	if w.ir.ActiveSheetIndex != nil {
		active := *w.ir.ActiveSheetIndex
		if active < 0 || active >= len(w.ir.Sheets) {
			active = 0
			w.ir.ActiveSheetIndex = &active
		}
	}
}

func (w *Workbook) seedLegacyDrawingVML(pkg *sheetparts.Package) {
	if w == nil || pkg == nil {
		return
	}
	for _, entry := range pkg.Worksheets {
		if entry == nil || entry.SheetID == 0 || len(entry.LegacyDrawingVML) == 0 {
			continue
		}
		w.legacyDrawingVML[entry.SheetID] = append([]byte(nil), entry.LegacyDrawingVML...)
	}
}

func (w *Workbook) markVMLDirty(sheetID int) {
	if w == nil || sheetID == 0 {
		return
	}
	w.vmlDirty[sheetID] = true
}

func (w *Workbook) seedLegacyPivotTables(pkg *sheetparts.Package) {
	if w == nil || pkg == nil {
		return
	}
	for _, entry := range pkg.Worksheets {
		if entry == nil || entry.SheetID == 0 || len(entry.PivotTables) == 0 {
			continue
		}
		w.legacyPivotParts[entry.SheetID] = clonePivotTableParts(entry.PivotTables)
	}
}

func (w *Workbook) applyLegacyPivotTables(pkg *sheetparts.Package) {
	if w == nil || pkg == nil {
		return
	}
	for _, entry := range pkg.Worksheets {
		if entry == nil || entry.SheetID == 0 || w.pivotDirty[entry.SheetID] {
			continue
		}
		legacy := w.legacyPivotParts[entry.SheetID]
		if len(legacy) == 0 {
			continue
		}
		entry.PivotTables = clonePivotTableParts(legacy)
	}
}

func (w *Workbook) markPivotDirty(sheetID int) {
	if w == nil || sheetID == 0 {
		return
	}
	w.pivotDirty[sheetID] = true
}

func (w *Workbook) markAllPivotDirty() {
	if w == nil || w.ir == nil {
		return
	}
	for _, sheet := range w.ir.Sheets {
		if sheet != nil && sheet.ID != 0 && len(sheet.PivotTables) > 0 {
			w.pivotDirty[sheet.ID] = true
		}
	}
}

func (s *Sheet) recalculateDimensions() {
	firstRow, lastRow, firstCol, lastCol := 0, 0, 0, 0
	for _, row := range s.sheet.Rows {
		if len(row.Cells) == 0 {
			continue
		}
		if firstRow == 0 || row.Index < firstRow {
			firstRow = row.Index
		}
		if row.Index > lastRow {
			lastRow = row.Index
		}
		for _, cell := range row.Cells {
			if cell.Col < 1 {
				continue
			}
			if firstCol == 0 || cell.Col < firstCol {
				firstCol = cell.Col
			}
			if cell.Col > lastCol {
				lastCol = cell.Col
			}
		}
	}
	s.sheet.DimFirstRow = firstRow
	s.sheet.DimLastRow = lastRow
	s.sheet.DimFirstCol = firstCol
	s.sheet.DimLastCol = lastCol
}

func (w *Workbook) normalizeActiveAfterInsert(insertAt int) {
	if w.ir.ActiveSheetIndex == nil {
		return
	}
	active := *w.ir.ActiveSheetIndex
	if active >= insertAt {
		active++
		w.ir.ActiveSheetIndex = &active
	}
}

func (w *Workbook) shiftSelectedForInsert(insertAt int) {
	remapped := make(map[int]bool, len(w.selected))
	for idx := range w.selected {
		if idx >= insertAt {
			remapped[idx+1] = true
		} else {
			remapped[idx] = true
		}
	}
	w.selected = remapped
}

func (w *Workbook) shiftSelectedForRemove(removeAt int) {
	remapped := make(map[int]bool, len(w.selected))
	for idx := range w.selected {
		switch {
		case idx == removeAt:
			continue
		case idx > removeAt:
			remapped[idx-1] = true
		default:
			remapped[idx] = true
		}
	}
	w.selected = remapped
}

func (w *Workbook) remapSelectionAfterMove(from, to int) {
	remapped := make(map[int]bool, len(w.selected))
	for idx := range w.selected {
		remapped[remapMovedIndex(idx, from, to)] = true
	}
	w.selected = remapped
}

func remapMovedIndex(index, from, to int) int {
	switch {
	case index == from:
		return to
	case from < to && index > from && index <= to:
		return index - 1
	case from > to && index >= to && index < from:
		return index + 1
	default:
		return index
	}
}

func cloneSheet(src *sheetir.Sheet) *sheetir.Sheet {
	if src == nil {
		return &sheetir.Sheet{}
	}
	clone := *src
	clone.Rows = nil
	for _, row := range src.Rows {
		rc := *row
		rc.Cells = nil
		for _, cell := range row.Cells {
			cc := *cell
			cc.RichTextRuns = cloneRichTextRuns(cell.RichTextRuns)
			rc.Cells = append(rc.Cells, &cc)
		}
		clone.Rows = append(clone.Rows, &rc)
	}
	clone.ColumnWidths = cloneFloatMap(src.ColumnWidths)
	clone.HiddenCols = cloneBoolMap(src.HiddenCols)
	clone.Comments = cloneStringMap(src.Comments)
	clone.OutlineRows = cloneIntMap(src.OutlineRows)
	clone.CollapsedRows = cloneBoolMap(src.CollapsedRows)
	clone.OutlineCols = cloneIntMap(src.OutlineCols)
	clone.CollapsedCols = cloneBoolMap(src.CollapsedCols)
	clone.RowPageBreaks = append([]int(nil), src.RowPageBreaks...)
	clone.ColPageBreaks = append([]int(nil), src.ColPageBreaks...)
	clone.PageSetup = clonePageSetup(src.PageSetup)
	clone.HeaderFooter = cloneHeaderFooter(src.HeaderFooter)
	clone.FormControls = cloneFormControls(src.FormControls)
	clone.VMLObjects = cloneVMLObjects(src.VMLObjects)
	clone.Drawings = cloneDrawings(src.Drawings)
	clone.MergedCells = nil
	for _, merged := range src.MergedCells {
		mc := *merged
		clone.MergedCells = append(clone.MergedCells, &mc)
	}
	clone.PivotTables = clonePivotTables(src.PivotTables)
	return &clone
}

func clonePivotTables(src []*sheetir.PivotTable) []*sheetir.PivotTable {
	out := make([]*sheetir.PivotTable, 0, len(src))
	for _, pivot := range src {
		if pivot == nil {
			continue
		}
		pc := *pivot
		pc.RowFields = append([]string(nil), pivot.RowFields...)
		pc.ColFields = append([]string(nil), pivot.ColFields...)
		pc.PageFields = append([]string(nil), pivot.PageFields...)
		pc.Fields = clonePivotFields(pivot.Fields)
		pc.PageFieldSelections = clonePivotPageFieldSelections(pivot.PageFieldSelections)
		pc.DataFields = nil
		for _, field := range pivot.DataFields {
			if field == nil {
				continue
			}
			fc := *field
			pc.DataFields = append(pc.DataFields, &fc)
		}
		out = append(out, &pc)
	}
	return out
}

func clonePivotFields(src []*sheetir.PivotField) []*sheetir.PivotField {
	out := make([]*sheetir.PivotField, 0, len(src))
	for _, field := range src {
		if field == nil {
			continue
		}
		fc := *field
		fc.Items = nil
		for _, item := range field.Items {
			if item != nil {
				ic := *item
				fc.Items = append(fc.Items, &ic)
			}
		}
		out = append(out, &fc)
	}
	return out
}

func clonePivotPageFieldSelections(src []*sheetir.PivotPageFieldSelection) []*sheetir.PivotPageFieldSelection {
	out := make([]*sheetir.PivotPageFieldSelection, 0, len(src))
	for _, selection := range src {
		if selection != nil {
			sc := *selection
			out = append(out, &sc)
		}
	}
	return out
}

func clonePivotTableParts(src []*sheetparts.PivotTableWithCache) []*sheetparts.PivotTableWithCache {
	out := make([]*sheetparts.PivotTableWithCache, 0, len(src))
	for _, ptc := range src {
		if ptc == nil {
			continue
		}
		clone := &sheetparts.PivotTableWithCache{}
		if ptc.Table != nil {
			table := *ptc.Table
			table.Raw = append([]byte(nil), ptc.Table.Raw...)
			clone.Table = &table
		}
		if ptc.Cache != nil {
			cache := *ptc.Cache
			cache.Raw = append([]byte(nil), ptc.Cache.Raw...)
			clone.Cache = &cache
		}
		if ptc.Records != nil {
			records := *ptc.Records
			records.Raw = append([]byte(nil), ptc.Records.Raw...)
			clone.Records = &records
		}
		out = append(out, clone)
	}
	return out
}

func clonePageSetup(src *sheetir.PageSetup) *sheetir.PageSetup {
	if src == nil {
		return nil
	}
	clone := *src
	return &clone
}

func cloneHeaderFooter(src *sheetir.HeaderFooter) *sheetir.HeaderFooter {
	if src == nil {
		return nil
	}
	clone := *src
	if len(src.Images) > 0 {
		clone.Images = make([]sheetir.HFImage, len(src.Images))
		for i, img := range src.Images {
			clone.Images[i] = sheetir.HFImage{
				ContentType: img.ContentType,
				Data:        append([]byte(nil), img.Data...),
			}
		}
	}
	return &clone
}

func cloneRichTextRuns(runs []*sheetir.RichTextRun) []*sheetir.RichTextRun {
	out := make([]*sheetir.RichTextRun, 0, len(runs))
	for _, run := range runs {
		rc := *run
		out = append(out, &rc)
	}
	return out
}

func cloneDrawings(drawings []*sheetir.Drawing) []*sheetir.Drawing {
	out := make([]*sheetir.Drawing, 0, len(drawings))
	for _, drawing := range drawings {
		if drawing == nil {
			continue
		}
		clone := *drawing
		if drawing.Image != nil {
			image := *drawing.Image
			image.Data = append([]byte(nil), drawing.Image.Data...)
			clone.Image = &image
		}
		if drawing.Shape != nil {
			shape := *drawing.Shape
			shape.Adjustments = append([]float64(nil), drawing.Shape.Adjustments...)
			shape.LineStyle = cloneDrawingLineStyle(drawing.Shape.LineStyle)
			shape.RichTextRuns = cloneRichTextRuns(drawing.Shape.RichTextRuns)
			clone.Shape = &shape
		}
		if drawing.Chart != nil {
			chart := *drawing.Chart
			clone.Chart = &chart
		}
		if drawing.Group != nil {
			clone.Group = &sheetir.DrawingGroup{Children: cloneDrawings(drawing.Group.Children)}
		}
		out = append(out, &clone)
	}
	return out
}

func cloneVMLObjects(objects []*sheetir.VMLObject) []*sheetir.VMLObject {
	out := make([]*sheetir.VMLObject, 0, len(objects))
	for _, obj := range objects {
		if obj == nil {
			continue
		}
		clone := *obj
		out = append(out, &clone)
	}
	return out
}

func cloneFormControls(controls []*sheetir.FormControl) []*sheetir.FormControl {
	out := make([]*sheetir.FormControl, 0, len(controls))
	for _, control := range controls {
		if control == nil {
			continue
		}
		clone := *control
		clone.FirstButton = control.FirstButton
		clone.Selected = append([]int(nil), control.Selected...)
		out = append(out, &clone)
	}
	return out
}

func plainTextFromRuns(runs []*sheetir.RichTextRun) string {
	var b strings.Builder
	for _, run := range runs {
		b.WriteString(run.Text)
	}
	return b.String()
}

func cloneFloatMap(in map[int]float64) map[int]float64 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneBoolMap(in map[int]bool) map[int]bool {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneIntMap(in map[int]int) map[int]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sortRows(rows []*sheetir.Row) {
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Index < rows[j].Index
	})
}

func sortCells(cells []*sheetir.Cell) {
	sort.Slice(cells, func(i, j int) bool {
		return cells[i].Col < cells[j].Col
	})
}

func appendUniqueInt(values []int, value int) []int {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	values = append(values, value)
	sort.Ints(values)
	return values
}

func removeIntValue(values []int, value int) ([]int, bool) {
	for i, existing := range values {
		if existing != value {
			continue
		}
		return append(values[:i], values[i+1:]...), true
	}
	return values, false
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
