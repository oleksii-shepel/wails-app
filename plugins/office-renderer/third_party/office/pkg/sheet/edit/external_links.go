package edit

import (
	"fmt"
	"strings"

	sheetir "office/pkg/sheet/ir"
	"office/pkg/sheet/parsers/ooxml/formula"
	sheetresolve "office/pkg/sheet/parsers/ooxml/resolve"
)

// ExternalLinks returns workbook external-link metadata in workbook order.
func (w *Workbook) ExternalLinks() []*sheetir.WorkbookExternalLink {
	if w == nil || w.ir == nil {
		return nil
	}
	return append([]*sheetir.WorkbookExternalLink(nil), w.ir.ExternalLinks...)
}

// ExternalLink finds an external workbook link by target or target basename.
func (w *Workbook) ExternalLink(target string) (*sheetir.WorkbookExternalLink, bool) {
	if w == nil || w.ir == nil {
		return nil, false
	}
	key := normalizeExternalTarget(target)
	for _, link := range w.ir.ExternalLinks {
		if link != nil && normalizeExternalTarget(link.Target) == key {
			return link, true
		}
	}
	return nil, false
}

// AddExternalLink creates an external workbook link if it does not already exist.
func (w *Workbook) AddExternalLink(target string) (*sheetir.WorkbookExternalLink, error) {
	if w == nil || w.ir == nil {
		return nil, fmt.Errorf("sheet edit: nil workbook")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("sheet edit: external link target is empty")
	}
	if existing, ok := w.ExternalLink(target); ok {
		return existing, nil
	}
	link := &sheetir.WorkbookExternalLink{Target: target}
	w.ir.ExternalLinks = append(w.ir.ExternalLinks, link)
	return link, nil
}

// RemoveExternalLink removes a link by target.
func (w *Workbook) RemoveExternalLink(target string) bool {
	if w == nil || w.ir == nil {
		return false
	}
	key := normalizeExternalTarget(target)
	for i, link := range w.ir.ExternalLinks {
		if link != nil && normalizeExternalTarget(link.Target) == key {
			w.ir.ExternalLinks = append(w.ir.ExternalLinks[:i], w.ir.ExternalLinks[i+1:]...)
			return true
		}
	}
	return false
}

// SetExternalDefinedName creates or replaces a defined name on an external link.
func (w *Workbook) SetExternalDefinedName(target, name, refersTo string, sheetID *int) (*sheetir.WorkbookExternalDefinedName, error) {
	link, err := w.AddExternalLink(target)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("sheet edit: external defined name is empty")
	}
	for _, dn := range link.DefinedNames {
		if dn != nil && strings.EqualFold(strings.TrimSpace(dn.Name), name) {
			dn.RefersTo = strings.TrimSpace(refersTo)
			dn.SheetID = cloneEditIntPtr(sheetID)
			return dn, nil
		}
	}
	dn := &sheetir.WorkbookExternalDefinedName{Name: name, RefersTo: strings.TrimSpace(refersTo), SheetID: cloneEditIntPtr(sheetID)}
	link.DefinedNames = append(link.DefinedNames, dn)
	return dn, nil
}

// SetExternalCachedCell creates or replaces one cached external-workbook cell.
// valueType uses SpreadsheetML external-cell types such as "n", "str", "b", or "e".
func (w *Workbook) SetExternalCachedCell(target, sheetName, ref, valueType, value string) (*sheetir.WorkbookExternalCell, error) {
	link, err := w.AddExternalLink(target)
	if err != nil {
		return nil, err
	}
	sheetName = strings.TrimSpace(sheetName)
	if sheetName == "" {
		return nil, fmt.Errorf("sheet edit: external sheet name is empty")
	}
	row, col, err := parseCellRef(ref)
	if err != nil {
		return nil, fmt.Errorf("sheet edit: invalid external cell reference %q: %w", ref, err)
	}
	ref = cellRef(row, col)
	sheet := ensureExternalCachedSheet(link, sheetName)
	for _, cell := range sheet.Cells {
		if cell != nil && strings.EqualFold(strings.ReplaceAll(cell.Ref, "$", ""), ref) {
			cell.Ref = ref
			cell.Type = strings.TrimSpace(valueType)
			cell.Value = value
			return cell, nil
		}
	}
	cell := &sheetir.WorkbookExternalCell{Ref: ref, Type: strings.TrimSpace(valueType), Value: value}
	sheet.Cells = append(sheet.Cells, cell)
	return cell, nil
}

// ClearExternalCachedSheet removes cached cells for one external sheet while preserving link metadata.
func (w *Workbook) ClearExternalCachedSheet(target, sheetName string) bool {
	link, ok := w.ExternalLink(target)
	if !ok {
		return false
	}
	for _, sheet := range link.CachedSheets {
		if sheet != nil && strings.EqualFold(strings.TrimSpace(sheet.Name), strings.TrimSpace(sheetName)) {
			changed := len(sheet.Cells) > 0 || sheet.RefreshError
			sheet.Cells = nil
			sheet.RefreshError = false
			return changed
		}
	}
	return false
}

// NewFormulaEvaluator creates a workbook-aware evaluator over the current edited IR.
func (w *Workbook) NewFormulaEvaluator(resolver sheetresolve.ExternalWorkbookResolver) *sheetresolve.FormulaEvaluator {
	if w == nil {
		return sheetresolve.NewFormulaEvaluator(nil, sheetresolve.FormulaEvaluatorOptions{ExternalWorkbookResolver: resolver})
	}
	return sheetresolve.NewFormulaEvaluator(w.ir, sheetresolve.FormulaEvaluatorOptions{ExternalWorkbookResolver: resolver})
}

// EvaluateFormula evaluates an expression in a worksheet context.
func (w *Workbook) EvaluateFormula(sheetName, expression string, resolver sheetresolve.ExternalWorkbookResolver) (formula.Value, error) {
	return w.NewFormulaEvaluator(resolver).Evaluate(sheetName, expression)
}

// EvaluateCell returns the evaluated value for a workbook cell.
func (w *Workbook) EvaluateCell(sheetName, ref string, resolver sheetresolve.ExternalWorkbookResolver) (formula.Value, error) {
	return w.NewFormulaEvaluator(resolver).Cell(sheetName, ref)
}

func ensureExternalCachedSheet(link *sheetir.WorkbookExternalLink, name string) *sheetir.WorkbookExternalSheet {
	for _, sheet := range link.CachedSheets {
		if sheet != nil && strings.EqualFold(strings.TrimSpace(sheet.Name), name) {
			return sheet
		}
	}
	id := len(link.SheetNames)
	for i, existing := range link.SheetNames {
		if strings.EqualFold(strings.TrimSpace(existing), name) {
			id = i
			break
		}
	}
	foundName := false
	for _, existing := range link.SheetNames {
		if strings.EqualFold(strings.TrimSpace(existing), name) {
			foundName = true
			break
		}
	}
	if !foundName {
		link.SheetNames = append(link.SheetNames, name)
		id = len(link.SheetNames) - 1
	}
	sheet := &sheetir.WorkbookExternalSheet{ID: id, Name: name}
	link.CachedSheets = append(link.CachedSheets, sheet)
	return sheet
}

func normalizeExternalTarget(target string) string {
	target = strings.TrimSpace(strings.ReplaceAll(target, "\\", "/"))
	return strings.ToUpper(target)
}

func cloneEditIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}
