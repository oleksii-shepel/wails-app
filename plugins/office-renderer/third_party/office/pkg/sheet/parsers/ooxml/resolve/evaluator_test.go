package resolve

import (
	"fmt"
	"testing"

	"office/pkg/sheet/ir"
	"office/pkg/sheet/parsers/ooxml/formula"
)

func TestFormulaEvaluatorUsesCachedExternalWorkbookValues(t *testing.T) {
	wb := &Workbook{
		Sheets: []*Sheet{{Name: "Main"}},
		ExternalLinks: []*WorkbookExternalLink{{
			Target:     "external.xlsx",
			SheetNames: []string{"Prices"},
			CachedSheets: []*WorkbookExternalSheet{{
				ID: 0, Name: "Prices",
				Cells: []*WorkbookExternalCell{{Ref: "A1", Type: "n", Value: "7"}},
			}},
		}},
	}
	evaluator := NewFormulaEvaluator(wb, FormulaEvaluatorOptions{})
	got, err := evaluator.Evaluate("Main", "'[1]Prices'!A1*3")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.Type != formula.TypeNumber || got.Num != 21 {
		t.Fatalf("value = %#v, want 21", got)
	}
}

func TestFormulaEvaluatorCanLoadLiveExternalWorkbook(t *testing.T) {
	wb := &Workbook{
		Sheets: []*Sheet{{Name: "Main"}},
		ExternalLinks: []*WorkbookExternalLink{{
			Target:     "books/source.xlsx",
			SheetNames: []string{"Prices"},
			CachedSheets: []*WorkbookExternalSheet{{
				ID: 0, Name: "Prices",
				Cells: []*WorkbookExternalCell{{Ref: "A1", Type: "n", Value: "7"}},
			}},
		}},
	}
	external := &ir.Workbook{Sheets: []*ir.Sheet{{
		Name: "Prices",
		Rows: []*ir.Row{{Index: 1, Cells: []*ir.Cell{{Ref: "A1", Row: 1, Col: 1, Kind: ir.CellNumber, NumValue: 11}}}},
	}}}
	calls := 0
	evaluator := NewFormulaEvaluator(wb, FormulaEvaluatorOptions{
		ExternalWorkbookResolver: func(target string) (*Workbook, error) {
			calls++
			if target != "books/source.xlsx" {
				return nil, fmt.Errorf("unexpected target %q", target)
			}
			return external, nil
		},
	})
	got, err := evaluator.Evaluate("Main", "'[1]Prices'!A1+1")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got.Type != formula.TypeNumber || got.Num != 12 {
		t.Fatalf("value = %#v, want 12", got)
	}
	if calls != 1 {
		t.Fatalf("resolver calls = %d, want 1", calls)
	}
	// Cached resolved workbook should be reused.
	_, _ = evaluator.Evaluate("Main", "'[1]Prices'!A1+2")
	if calls != 1 {
		t.Fatalf("resolver calls after reuse = %d, want 1", calls)
	}
}

func TestRefreshExternalWorkbooksReplacesCachedValues(t *testing.T) {
	wb := &Workbook{
		Sheets: []*Sheet{{Name: "Main"}},
		ExternalLinks: []*WorkbookExternalLink{{
			Target:       "external.xlsx",
			SheetNames:   []string{"Prices"},
			CachedSheets: []*WorkbookExternalSheet{{ID: 0, Name: "Prices", Cells: []*WorkbookExternalCell{{Ref: "A1", Type: "n", Value: "1"}}}},
		}},
	}
	external := &Workbook{Sheets: []*Sheet{{
		Name: "Prices",
		Rows: []*Row{{Index: 1, Cells: []*Cell{{Ref: "A1", Row: 1, Col: 1, Kind: CellNumber, NumValue: 9}}}},
	}}}
	if err := RefreshExternalWorkbooks(wb, func(target string) (*Workbook, error) {
		return external, nil
	}); err != nil {
		t.Fatalf("RefreshExternalWorkbooks: %v", err)
	}
	if got := wb.ExternalLinks[0].CachedSheets[0].Cells[0].Value; got != "9" {
		t.Fatalf("cached value = %q, want 9", got)
	}
	evaluator := NewFormulaEvaluator(wb, FormulaEvaluatorOptions{})
	got, err := evaluator.Evaluate("Main", "'[1]Prices'!A1")
	if err != nil || got.Type != formula.TypeNumber || got.Num != 9 {
		t.Fatalf("evaluation after refresh = %#v, err=%v", got, err)
	}
}
