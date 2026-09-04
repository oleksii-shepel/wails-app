package resolve

import (
	"fmt"
	"strconv"
	"strings"

	sheetir "office/pkg/sheet/ir"
	"office/pkg/sheet/parsers/ooxml/formula"
)

// ExternalWorkbookResolver loads a linked workbook for formula evaluation.
// The target is the external-link target stored in the workbook package. Callers
// decide whether targets may be opened from disk, a cache, a database, or another source.
type ExternalWorkbookResolver func(target string) (*Workbook, error)

// FormulaEvaluatorOptions configures workbook-aware formula evaluation.
type FormulaEvaluatorOptions struct {
	ExternalWorkbookResolver ExternalWorkbookResolver
}

// FormulaEvaluator evaluates formulas against a resolved workbook. It supports
// local/cross-sheet references, defined names, structured table references,
// GETPIVOTDATA, cached external links, and optionally live external workbooks.
type FormulaEvaluator struct {
	wb  *Workbook
	ctx *chartWorkbookFormulaContext
}

// NewFormulaEvaluator builds a reusable workbook-aware evaluator.
func NewFormulaEvaluator(wb *Workbook, opts FormulaEvaluatorOptions) *FormulaEvaluator {
	ctx := buildChartWorkbookFormulaContext(wb)
	if ctx != nil {
		ctx.externalResolver = opts.ExternalWorkbookResolver
	}
	return &FormulaEvaluator{wb: wb, ctx: ctx}
}

// Evaluate evaluates an arbitrary formula in the context of the named sheet.
func (e *FormulaEvaluator) Evaluate(sheetName, expression string) (formula.Value, error) {
	if e == nil || e.ctx == nil || e.wb == nil {
		return formula.Empty(), fmt.Errorf("sheet formula: nil workbook evaluator")
	}
	sheetIndex := e.ctx.sheetIndexByName(sheetName)
	if sheetIndex < 0 {
		return formula.Empty(), fmt.Errorf("sheet formula: sheet %q not found", sheetName)
	}
	eng := e.ctx.engines[sheetIndex]
	if eng == nil {
		return formula.Empty(), fmt.Errorf("sheet formula: sheet %q has no evaluation engine", sheetName)
	}
	expression = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(expression), "="))
	if expression == "" {
		return formula.Empty(), fmt.Errorf("sheet formula: empty expression")
	}
	expression = ExpandStructuredTableRefs(expandWorkbookDefinedNamesInFormula(expression, e.wb, sheetIndex), e.wb, sheetIndex)
	return eng.Evaluate(expression)
}

// Cell returns the evaluated value of a workbook cell, evaluating its formula when needed.
func (e *FormulaEvaluator) Cell(sheetName, ref string) (formula.Value, error) {
	if e == nil || e.ctx == nil {
		return formula.Empty(), fmt.Errorf("sheet formula: nil workbook evaluator")
	}
	sheetIndex := e.ctx.sheetIndexByName(sheetName)
	if sheetIndex < 0 {
		return formula.Empty(), fmt.Errorf("sheet formula: sheet %q not found", sheetName)
	}
	eng := e.ctx.engines[sheetIndex]
	if eng == nil {
		return formula.Empty(), fmt.Errorf("sheet formula: sheet %q has no evaluation engine", sheetName)
	}
	return eng.GetCell(strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(ref), "$", ""))), nil
}

// ExternalErrors returns external-workbook loading errors encountered so far.
func (e *FormulaEvaluator) ExternalErrors() map[string]error {
	out := map[string]error{}
	if e == nil || e.ctx == nil {
		return out
	}
	for target, err := range e.ctx.externalErrors {
		out[target] = err
	}
	return out
}

// RefreshExternalWorkbooks reloads every external workbook through resolver,
// replaces the cached external sheet data, and rehydrates chart series that
// reference those links. It does not open files or URLs on its own.
func RefreshExternalWorkbooks(wb *Workbook, resolver ExternalWorkbookResolver) error {
	if wb == nil {
		return fmt.Errorf("sheet formula: nil workbook")
	}
	if resolver == nil {
		return fmt.Errorf("sheet formula: external workbook resolver is nil")
	}
	for _, link := range wb.ExternalLinks {
		if link == nil || strings.TrimSpace(link.Target) == "" {
			continue
		}
		external, err := resolver(link.Target)
		if err != nil {
			return fmt.Errorf("sheet formula: refresh external workbook %q: %w", link.Target, err)
		}
		if external == nil {
			return fmt.Errorf("sheet formula: refresh external workbook %q returned nil", link.Target)
		}
		evaluator := NewFormulaEvaluator(external, FormulaEvaluatorOptions{ExternalWorkbookResolver: resolver})
		link.SheetNames = link.SheetNames[:0]
		link.CachedSheets = link.CachedSheets[:0]
		for sheetIndex, sheet := range external.Sheets {
			if sheet == nil {
				continue
			}
			link.SheetNames = append(link.SheetNames, sheet.Name)
			cached := &WorkbookExternalSheet{ID: sheetIndex, Name: sheet.Name}
			for _, row := range sheet.Rows {
				if row == nil {
					continue
				}
				for _, cell := range row.Cells {
					if cell == nil || strings.TrimSpace(cell.Ref) == "" {
						continue
					}
					value := formulaValueFromIRCell(cell)
					if cell.Formula != "" {
						if evaluated, evalErr := evaluator.Cell(sheet.Name, cell.Ref); evalErr == nil {
							value = evaluated
						}
					}
					cached.Cells = append(cached.Cells, externalCellFromFormulaValue(cell.Ref, value))
				}
			}
			link.CachedSheets = append(link.CachedSheets, cached)
		}
	}
	hydrateWorkbookCharts(wb)
	return nil
}

func formulaValueFromIRCell(cell *Cell) formula.Value {
	if cell == nil {
		return formula.Empty()
	}
	switch cell.Kind {
	case CellNumber, sheetir.CellDate:
		return formula.Number(cell.NumValue)
	case CellBool:
		return formula.Bool(cell.NumValue != 0)
	case CellString:
		return formula.Text(cell.StrValue)
	case CellError:
		if code := formula.ParseError(cell.StrValue); code != formula.ErrNone {
			return formula.Error(code)
		}
		return formula.Text(cell.StrValue)
	case CellFormula:
		if cell.StrValue != "" {
			return formula.Text(cell.StrValue)
		}
	}
	return formula.Empty()
}

func externalCellFromFormulaValue(ref string, value formula.Value) *WorkbookExternalCell {
	cell := &WorkbookExternalCell{Ref: strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(ref), "$", ""))}
	switch value.Type {
	case formula.TypeNumber:
		cell.Type = "n"
		cell.Value = strconv.FormatFloat(value.Num, 'g', -1, 64)
	case formula.TypeBool:
		cell.Type = "b"
		if value.Bool {
			cell.Value = "1"
		} else {
			cell.Value = "0"
		}
	case formula.TypeError:
		cell.Type = "e"
		cell.Value = value.Err.String()
	case formula.TypeText:
		cell.Type = "str"
		cell.Value = value.Text
	default:
		cell.Type = "str"
	}
	return cell
}
