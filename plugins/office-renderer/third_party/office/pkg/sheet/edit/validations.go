package edit

import (
	"fmt"
	"strings"

	sheetir "office/pkg/sheet/ir"
)

// DataValidations returns worksheet data-validation rules in storage order.
func (s *Sheet) DataValidations() []*sheetir.DataValidation {
	if s == nil || s.sheet == nil {
		return nil
	}
	return append([]*sheetir.DataValidation(nil), s.sheet.DataValidations...)
}

// DataValidation returns the first validation rule matching sqRef and type.
func (s *Sheet) DataValidation(sqRef, validationType string) (*sheetir.DataValidation, bool) {
	if s == nil || s.sheet == nil {
		return nil, false
	}
	sqRef = normalizeValidationKey(sqRef)
	validationType = strings.TrimSpace(validationType)
	for _, dv := range s.sheet.DataValidations {
		if dv == nil {
			continue
		}
		if normalizeValidationKey(dv.SqRef) == sqRef && strings.EqualFold(dv.Type, validationType) {
			return dv, true
		}
	}
	return nil, false
}

// AddDataValidation appends a worksheet data-validation rule.
func (s *Sheet) AddDataValidation(rule sheetir.DataValidation) (*sheetir.DataValidation, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	rule.SqRef = strings.TrimSpace(rule.SqRef)
	rule.Type = strings.TrimSpace(rule.Type)
	rule.Operator = strings.TrimSpace(rule.Operator)
	if rule.SqRef == "" {
		return nil, fmt.Errorf("sheet edit: data-validation range is empty")
	}
	if rule.Type == "list" && len(rule.ListValues) > 0 && strings.TrimSpace(rule.Formula1) == "" {
		rule.Formula1 = inlineValidationListFormula(rule.ListValues)
	}
	clone := cloneDataValidation(&rule)
	s.sheet.DataValidations = append(s.sheet.DataValidations, clone)
	return clone, nil
}

// SetDataValidation replaces the first matching sqRef/type rule or appends it.
func (s *Sheet) SetDataValidation(rule sheetir.DataValidation) (*sheetir.DataValidation, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	rule.SqRef = strings.TrimSpace(rule.SqRef)
	rule.Type = strings.TrimSpace(rule.Type)
	if rule.SqRef == "" {
		return nil, fmt.Errorf("sheet edit: data-validation range is empty")
	}
	if rule.Type == "list" && len(rule.ListValues) > 0 && strings.TrimSpace(rule.Formula1) == "" {
		rule.Formula1 = inlineValidationListFormula(rule.ListValues)
	}
	clone := cloneDataValidation(&rule)
	key := normalizeValidationKey(rule.SqRef)
	for i, dv := range s.sheet.DataValidations {
		if dv == nil {
			continue
		}
		if normalizeValidationKey(dv.SqRef) == key && strings.EqualFold(dv.Type, rule.Type) {
			s.sheet.DataValidations[i] = clone
			return clone, nil
		}
	}
	s.sheet.DataValidations = append(s.sheet.DataValidations, clone)
	return clone, nil
}

// AddListValidation adds an inline-list data validation.
func (s *Sheet) AddListValidation(sqRef string, values []string, allowBlank bool) (*sheetir.DataValidation, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("sheet edit: list validation values are empty")
	}
	return s.AddDataValidation(sheetir.DataValidation{
		SqRef:      sqRef,
		Type:       "list",
		AllowBlank: allowBlank,
		Formula1:   inlineValidationListFormula(values),
		ListValues: append([]string(nil), values...),
	})
}

// AddFormulaListValidation adds a list validation backed by a range or formula.
func (s *Sheet) AddFormulaListValidation(sqRef, formula string, allowBlank bool) (*sheetir.DataValidation, error) {
	formula = normalizeValidationFormula(formula)
	if formula == "" {
		return nil, fmt.Errorf("sheet edit: list validation formula is empty")
	}
	return s.AddDataValidation(sheetir.DataValidation{
		SqRef:      sqRef,
		Type:       "list",
		AllowBlank: allowBlank,
		Formula1:   formula,
	})
}

// AddComparisonValidation adds a whole, decimal, date, time, textLength, or custom rule.
func (s *Sheet) AddComparisonValidation(sqRef, validationType, operator, formula1, formula2 string, allowBlank bool) (*sheetir.DataValidation, error) {
	validationType = strings.TrimSpace(validationType)
	formula1 = normalizeValidationFormula(formula1)
	formula2 = normalizeValidationFormula(formula2)
	if validationType == "" {
		return nil, fmt.Errorf("sheet edit: validation type is empty")
	}
	if formula1 == "" {
		return nil, fmt.Errorf("sheet edit: validation formula is empty")
	}
	return s.AddDataValidation(sheetir.DataValidation{
		SqRef:      sqRef,
		Type:       validationType,
		Operator:   strings.TrimSpace(operator),
		AllowBlank: allowBlank,
		Formula1:   formula1,
		Formula2:   formula2,
	})
}

// RemoveDataValidations removes all validation rules matching sqRef.
func (s *Sheet) RemoveDataValidations(sqRef string) int {
	if s == nil || s.sheet == nil {
		return 0
	}
	key := normalizeValidationKey(sqRef)
	kept := s.sheet.DataValidations[:0]
	removed := 0
	for _, dv := range s.sheet.DataValidations {
		if dv != nil && normalizeValidationKey(dv.SqRef) == key {
			removed++
			continue
		}
		kept = append(kept, dv)
	}
	s.sheet.DataValidations = kept
	return removed
}

// ClearDataValidations removes every data-validation rule from the worksheet.
func (s *Sheet) ClearDataValidations() {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.DataValidations = nil
}

func cloneDataValidation(src *sheetir.DataValidation) *sheetir.DataValidation {
	if src == nil {
		return nil
	}
	clone := *src
	clone.ListValues = append([]string(nil), src.ListValues...)
	return &clone
}

func normalizeValidationKey(s string) string {
	return strings.ToUpper(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

func normalizeValidationFormula(formula string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(formula), "="))
}

func inlineValidationListFormula(values []string) string {
	escaped := make([]string, 0, len(values))
	for _, value := range values {
		escaped = append(escaped, strings.ReplaceAll(value, `"`, `""`))
	}
	return `"` + strings.Join(escaped, ",") + `"`
}
