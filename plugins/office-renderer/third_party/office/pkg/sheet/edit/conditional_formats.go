package edit

import (
	"fmt"
	"strings"

	sheetir "office/pkg/sheet/ir"
)

// ConditionalFormattingRules returns worksheet conditional-formatting rules in storage order.
func (s *Sheet) ConditionalFormattingRules() []*sheetir.CFRule {
	if s == nil || s.sheet == nil {
		return nil
	}
	return append([]*sheetir.CFRule(nil), s.sheet.CFRules...)
}

// ConditionalFormattingRule returns the first rule matching sqRef, type, and priority.
func (s *Sheet) ConditionalFormattingRule(sqRef, ruleType string, priority int) (*sheetir.CFRule, bool) {
	if s == nil || s.sheet == nil {
		return nil, false
	}
	sqRef = normalizeValidationKey(sqRef)
	ruleType = strings.TrimSpace(ruleType)
	for _, rule := range s.sheet.CFRules {
		if rule == nil {
			continue
		}
		if normalizeValidationKey(rule.SqRef) == sqRef && strings.EqualFold(rule.Type, ruleType) && rule.Priority == priority {
			return rule, true
		}
	}
	return nil, false
}

// AddConditionalFormattingRule appends a conditional-formatting rule.
func (s *Sheet) AddConditionalFormattingRule(rule sheetir.CFRule) (*sheetir.CFRule, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	rule.SqRef = strings.TrimSpace(rule.SqRef)
	rule.Type = strings.TrimSpace(rule.Type)
	rule.Operator = strings.TrimSpace(rule.Operator)
	if rule.SqRef == "" {
		return nil, fmt.Errorf("sheet edit: conditional-formatting range is empty")
	}
	if rule.Type == "" {
		return nil, fmt.Errorf("sheet edit: conditional-formatting type is empty")
	}
	if rule.Priority == 0 {
		rule.Priority = s.nextConditionalFormattingPriority()
	}
	clone := cloneCFRule(&rule)
	s.sheet.CFRules = append(s.sheet.CFRules, clone)
	return clone, nil
}

// SetConditionalFormattingRule replaces the first matching sqRef/type/priority rule or appends it.
func (s *Sheet) SetConditionalFormattingRule(rule sheetir.CFRule) (*sheetir.CFRule, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	rule.SqRef = strings.TrimSpace(rule.SqRef)
	rule.Type = strings.TrimSpace(rule.Type)
	if rule.SqRef == "" {
		return nil, fmt.Errorf("sheet edit: conditional-formatting range is empty")
	}
	if rule.Type == "" {
		return nil, fmt.Errorf("sheet edit: conditional-formatting type is empty")
	}
	if rule.Priority == 0 {
		rule.Priority = s.nextConditionalFormattingPriority()
	}
	clone := cloneCFRule(&rule)
	key := normalizeValidationKey(rule.SqRef)
	for i, existing := range s.sheet.CFRules {
		if existing == nil {
			continue
		}
		if normalizeValidationKey(existing.SqRef) == key && strings.EqualFold(existing.Type, rule.Type) && existing.Priority == rule.Priority {
			s.sheet.CFRules[i] = clone
			return clone, nil
		}
	}
	s.sheet.CFRules = append(s.sheet.CFRules, clone)
	return clone, nil
}

// AddFormulaConditionalFormat adds an expression-based conditional-formatting rule.
func (s *Sheet) AddFormulaConditionalFormat(sqRef, formula string, style sheetir.DxfStyle) (*sheetir.CFRule, error) {
	formula = normalizeValidationFormula(formula)
	if formula == "" {
		return nil, fmt.Errorf("sheet edit: conditional-formatting formula is empty")
	}
	return s.AddConditionalFormattingRule(sheetir.CFRule{
		SqRef:    sqRef,
		Type:     "expression",
		Formula:  []string{formula},
		DxfStyle: style,
	})
}

// AddCellIsConditionalFormat adds a cellIs comparison conditional-formatting rule.
func (s *Sheet) AddCellIsConditionalFormat(sqRef, operator, formula1, formula2 string, style sheetir.DxfStyle) (*sheetir.CFRule, error) {
	formula1 = normalizeValidationFormula(formula1)
	formula2 = normalizeValidationFormula(formula2)
	if formula1 == "" {
		return nil, fmt.Errorf("sheet edit: conditional-formatting formula is empty")
	}
	formulas := []string{formula1}
	if formula2 != "" {
		formulas = append(formulas, formula2)
	}
	return s.AddConditionalFormattingRule(sheetir.CFRule{
		SqRef:    sqRef,
		Type:     "cellIs",
		Operator: strings.TrimSpace(operator),
		Formula:  formulas,
		DxfStyle: style,
	})
}

// AddTop10ConditionalFormat adds a top/bottom-N conditional-formatting rule.
func (s *Sheet) AddTop10ConditionalFormat(sqRef string, rank int, percent, bottom bool, style sheetir.DxfStyle) (*sheetir.CFRule, error) {
	if rank <= 0 {
		return nil, fmt.Errorf("sheet edit: top10 rank must be > 0")
	}
	return s.AddConditionalFormattingRule(sheetir.CFRule{
		SqRef:    sqRef,
		Type:     "top10",
		Rank:     rank,
		Percent:  percent,
		Bottom:   bottom,
		DxfStyle: style,
	})
}

// AddTextConditionalFormat adds a containsText/notContainsText/beginsWith/endsWith rule.
func (s *Sheet) AddTextConditionalFormat(sqRef, ruleType, text string, style sheetir.DxfStyle) (*sheetir.CFRule, error) {
	ruleType = strings.TrimSpace(ruleType)
	text = strings.TrimSpace(text)
	if ruleType == "" {
		return nil, fmt.Errorf("sheet edit: text conditional-formatting type is empty")
	}
	if text == "" {
		return nil, fmt.Errorf("sheet edit: text conditional-formatting value is empty")
	}
	return s.AddConditionalFormattingRule(sheetir.CFRule{
		SqRef:    sqRef,
		Type:     ruleType,
		Text:     text,
		Formula:  []string{textConditionalFormula(ruleType, text, firstCellRefFromRange(sqRef))},
		DxfStyle: style,
	})
}

// AddTimePeriodConditionalFormat adds a date/time-period conditional-formatting rule.
func (s *Sheet) AddTimePeriodConditionalFormat(sqRef, period string, style sheetir.DxfStyle) (*sheetir.CFRule, error) {
	period = strings.TrimSpace(period)
	if period == "" {
		return nil, fmt.Errorf("sheet edit: time-period conditional-formatting period is empty")
	}
	return s.AddConditionalFormattingRule(sheetir.CFRule{
		SqRef:      sqRef,
		Type:       "timePeriod",
		TimePeriod: period,
		DxfStyle:   style,
	})
}

// AddDuplicateValuesConditionalFormat adds a duplicateValues conditional-formatting rule.
func (s *Sheet) AddDuplicateValuesConditionalFormat(sqRef string, style sheetir.DxfStyle) (*sheetir.CFRule, error) {
	return s.AddConditionalFormattingRule(sheetir.CFRule{
		SqRef:    sqRef,
		Type:     "duplicateValues",
		DxfStyle: style,
	})
}

// AddColorScaleConditionalFormat adds a color-scale conditional-formatting rule.
func (s *Sheet) AddColorScaleConditionalFormat(sqRef string, cfvo []sheetir.CFCfvo, colors []string) (*sheetir.CFRule, error) {
	if len(colors) < 2 {
		return nil, fmt.Errorf("sheet edit: color scale requires at least two colors")
	}
	if len(cfvo) > 0 && len(cfvo) != len(colors) {
		return nil, fmt.Errorf("sheet edit: color scale thresholds must match colors")
	}
	if len(cfvo) == 0 {
		cfvo = []sheetir.CFCfvo{{Type: "min"}, {Type: "max"}}
		if len(colors) == 3 {
			cfvo = []sheetir.CFCfvo{{Type: "min"}, {Type: "percentile", Val: "50"}, {Type: "max"}}
		}
	}
	return s.AddConditionalFormattingRule(sheetir.CFRule{
		SqRef: sqRef,
		Type:  "colorScale",
		ColorScale: &sheetir.CFColorScale{
			Cfvo:   cloneCfvo(cfvo),
			Colors: append([]string(nil), colors...),
		},
	})
}

// AddDataBarConditionalFormat adds a data-bar conditional-formatting rule.
func (s *Sheet) AddDataBarConditionalFormat(sqRef, color string, cfvo []sheetir.CFCfvo, showValue bool) (*sheetir.CFRule, error) {
	color = strings.TrimSpace(color)
	if color == "" {
		return nil, fmt.Errorf("sheet edit: data bar color is empty")
	}
	if len(cfvo) == 0 {
		cfvo = []sheetir.CFCfvo{{Type: "min"}, {Type: "max"}}
	}
	return s.AddConditionalFormattingRule(sheetir.CFRule{
		SqRef: sqRef,
		Type:  "dataBar",
		DataBar: &sheetir.CFDataBar{
			ShowValue: showValue,
			Color:     color,
			Cfvo:      cloneCfvo(cfvo),
		},
	})
}

// AddIconSetConditionalFormat adds an icon-set conditional-formatting rule.
func (s *Sheet) AddIconSetConditionalFormat(sqRef, set string, cfvo []sheetir.CFCfvo, showValue, reverse bool) (*sheetir.CFRule, error) {
	set = strings.TrimSpace(set)
	if set == "" {
		set = "3TrafficLights1"
	}
	if len(cfvo) == 0 {
		cfvo = []sheetir.CFCfvo{{Type: "percent", Val: "0"}, {Type: "percent", Val: "33"}, {Type: "percent", Val: "67"}}
	}
	return s.AddConditionalFormattingRule(sheetir.CFRule{
		SqRef: sqRef,
		Type:  "iconSet",
		IconSet: &sheetir.CFIconSet{
			Set:       set,
			ShowValue: showValue,
			Reverse:   reverse,
			Cfvo:      cloneCfvo(cfvo),
		},
	})
}

// RemoveConditionalFormattingRules removes all conditional-formatting rules matching sqRef.
func (s *Sheet) RemoveConditionalFormattingRules(sqRef string) int {
	if s == nil || s.sheet == nil {
		return 0
	}
	key := normalizeValidationKey(sqRef)
	kept := s.sheet.CFRules[:0]
	removed := 0
	for _, rule := range s.sheet.CFRules {
		if rule != nil && normalizeValidationKey(rule.SqRef) == key {
			removed++
			continue
		}
		kept = append(kept, rule)
	}
	s.sheet.CFRules = kept
	return removed
}

// ClearConditionalFormattingRules removes every conditional-formatting rule from the worksheet.
func (s *Sheet) ClearConditionalFormattingRules() {
	if s == nil || s.sheet == nil {
		return
	}
	s.sheet.CFRules = nil
}

func (s *Sheet) nextConditionalFormattingPriority() int {
	maxPriority := 0
	for _, rule := range s.sheet.CFRules {
		if rule != nil && rule.Priority > maxPriority {
			maxPriority = rule.Priority
		}
	}
	return maxPriority + 1
}

func cloneCFRule(src *sheetir.CFRule) *sheetir.CFRule {
	if src == nil {
		return nil
	}
	clone := *src
	clone.Formula = append([]string(nil), src.Formula...)
	clone.DxfStyle = cloneDxfStyle(src.DxfStyle)
	if src.DataBar != nil {
		clone.DataBar = &sheetir.CFDataBar{
			MinLength: src.DataBar.MinLength,
			MaxLength: src.DataBar.MaxLength,
			ShowValue: src.DataBar.ShowValue,
			Color:     src.DataBar.Color,
			Cfvo:      cloneCfvo(src.DataBar.Cfvo),
		}
	}
	if src.IconSet != nil {
		clone.IconSet = &sheetir.CFIconSet{
			Set:       src.IconSet.Set,
			ShowValue: src.IconSet.ShowValue,
			Reverse:   src.IconSet.Reverse,
			Cfvo:      cloneCfvo(src.IconSet.Cfvo),
		}
	}
	if src.ColorScale != nil {
		clone.ColorScale = &sheetir.CFColorScale{
			Cfvo:   cloneCfvo(src.ColorScale.Cfvo),
			Colors: append([]string(nil), src.ColorScale.Colors...),
		}
	}
	return &clone
}

func cloneCfvo(src []sheetir.CFCfvo) []sheetir.CFCfvo {
	return append([]sheetir.CFCfvo(nil), src...)
}

func cloneDxfStyle(src sheetir.DxfStyle) sheetir.DxfStyle {
	clone := src
	clone.Bold = cloneBoolPtr(src.Bold)
	clone.Italic = cloneBoolPtr(src.Italic)
	clone.Underline = cloneBoolPtr(src.Underline)
	clone.Strike = cloneBoolPtr(src.Strike)
	clone.BorderLeft = cloneIntPtr(src.BorderLeft)
	clone.BorderRight = cloneIntPtr(src.BorderRight)
	clone.BorderTop = cloneIntPtr(src.BorderTop)
	clone.BorderBottom = cloneIntPtr(src.BorderBottom)
	clone.BorderDiagonal = cloneIntPtr(src.BorderDiagonal)
	clone.ForwardDiagonalOn = cloneBoolPtr(src.ForwardDiagonalOn)
	clone.BackwardDiagonalOn = cloneBoolPtr(src.BackwardDiagonalOn)
	clone.FontHeight = cloneIntPtr(src.FontHeight)
	clone.FontWeight = cloneIntPtr(src.FontWeight)
	clone.EscapementType = cloneIntPtr(src.EscapementType)
	clone.UnderlineType = cloneIntPtr(src.UnderlineType)
	clone.FontColorIndex = cloneIntPtr(src.FontColorIndex)
	clone.Outline = cloneBoolPtr(src.Outline)
	clone.Shadow = cloneBoolPtr(src.Shadow)
	clone.FillPattern = cloneIntPtr(src.FillPattern)
	return clone
}

func cloneBoolPtr(v *bool) *bool {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func firstCellRefFromRange(sqRef string) string {
	sqRef = strings.TrimSpace(sqRef)
	if sqRef == "" {
		return "A1"
	}
	first := strings.Fields(sqRef)[0]
	if idx := strings.Index(first, ":"); idx >= 0 {
		first = first[:idx]
	}
	first = strings.ReplaceAll(first, "$", "")
	if first == "" {
		return "A1"
	}
	return first
}

func textConditionalFormula(ruleType, text, ref string) string {
	quoted := `"` + strings.ReplaceAll(text, `"`, `""`) + `"`
	switch ruleType {
	case "notContainsText":
		return fmt.Sprintf("ISERROR(SEARCH(%s,%s))", quoted, ref)
	case "beginsWith":
		return fmt.Sprintf("LEFT(%s,LEN(%s))=%s", ref, quoted, quoted)
	case "endsWith":
		return fmt.Sprintf("RIGHT(%s,LEN(%s))=%s", ref, quoted, quoted)
	default:
		return fmt.Sprintf("NOT(ISERROR(SEARCH(%s,%s)))", quoted, ref)
	}
}
