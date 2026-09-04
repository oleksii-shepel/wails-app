package edit

import sheetir "office/pkg/sheet/ir"

// WorkbookProtection returns the workbook-level protection metadata, if any.
func (w *Workbook) WorkbookProtection() *sheetir.WorkbookProtection {
	if w == nil || w.ir == nil {
		return nil
	}
	return w.ir.Protection
}

// SetWorkbookProtection replaces the workbook-level protection metadata.
func (w *Workbook) SetWorkbookProtection(protection sheetir.WorkbookProtection) *sheetir.WorkbookProtection {
	if w == nil || w.ir == nil {
		return nil
	}
	copyProtection := protection
	w.ir.Protection = &copyProtection
	return w.ir.Protection
}

// EnsureWorkbookProtection returns workbook-level protection metadata, creating it when needed.
func (w *Workbook) EnsureWorkbookProtection() *sheetir.WorkbookProtection {
	if w == nil || w.ir == nil {
		return nil
	}
	if w.ir.Protection == nil {
		w.ir.Protection = &sheetir.WorkbookProtection{}
	}
	return w.ir.Protection
}

// ClearWorkbookProtection removes workbook-level protection metadata.
func (w *Workbook) ClearWorkbookProtection() {
	if w == nil || w.ir == nil {
		return
	}
	w.ir.Protection = nil
}

// SheetProtection returns the worksheet-level protection metadata, if any.
func (s *Sheet) SheetProtection() *sheetir.SheetProtection {
	if s == nil || s.sheet == nil {
		return nil
	}
	return s.sheet.Protection
}

// SetSheetProtection replaces the worksheet-level protection metadata.
func (s *Sheet) SetSheetProtection(protection sheetir.SheetProtection) *sheetir.SheetProtection {
	if s == nil || s.sheet == nil {
		return nil
	}
	copyProtection := protection
	s.sheet.Protection = &copyProtection
	return s.sheet.Protection
}

// EnsureSheetProtection returns worksheet-level protection metadata, creating it when needed.
func (s *Sheet) EnsureSheetProtection() *sheetir.SheetProtection {
	if s == nil || s.sheet == nil {
		return nil
	}
	if s.sheet.Protection == nil {
		s.sheet.Protection = &sheetir.SheetProtection{}
	}
	return s.sheet.Protection
}

// ClearSheetProtection removes worksheet-level protection metadata.
func (s *Sheet) ClearSheetProtection() {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.Protection = nil
}
