package edit

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	sheetir "office/pkg/sheet/ir"
)

// SetArrayFormula creates or replaces an array formula over an A1-style range.
func (s *Sheet) SetArrayFormula(rangeRef, formula string) (*sheetir.ArrayFormula, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	formula = strings.TrimSpace(formula)
	if formula == "" {
		return nil, fmt.Errorf("sheet edit: array formula is empty")
	}
	bounds, err := parseRangeRef(rangeRef)
	if err != nil {
		return nil, err
	}
	anchorRef := cellRef(bounds.topRow, bounds.leftCol)
	group := &sheetir.ArrayFormula{
		AnchorRef: anchorRef,
		RangeRef:  normalizedRangeRef(bounds),
		Formula:   formula,
		TopRow:    bounds.topRow,
		LeftCol:   bounds.leftCol,
		BottomRow: bounds.bottomRow,
		RightCol:  bounds.rightCol,
	}
	s.removeArrayFormulaGroup(anchorRef)
	s.sheet.ArrayFormulas = append(s.sheet.ArrayFormulas, group)
	for row := bounds.topRow; row <= bounds.bottomRow; row++ {
		for col := bounds.leftCol; col <= bounds.rightCol; col++ {
			cell, err := s.CreateCell(row, col)
			if err != nil {
				return nil, err
			}
			cell.cell.Kind = sheetir.CellFormula
			cell.cell.Formula = formula
			cell.cell.ArrayFormulaAnchor = anchorRef
			cell.cell.ArrayFormulaRange = group.RangeRef
		}
	}
	return group, nil
}

// ClearArrayFormula removes array formula metadata for a group anchor.
func (s *Sheet) ClearArrayFormula(anchorRef string) bool {
	if s == nil || s.sheet == nil {
		return false
	}
	anchorRef = strings.ToUpper(strings.TrimSpace(anchorRef))
	group, ok := s.arrayFormulaGroup(anchorRef)
	if !ok {
		return false
	}
	s.removeArrayFormulaGroup(anchorRef)
	for _, row := range s.sheet.Rows {
		for _, cell := range row.Cells {
			if strings.EqualFold(cell.ArrayFormulaAnchor, anchorRef) {
				cell.ArrayFormulaAnchor = ""
				cell.ArrayFormulaRange = ""
				if strings.EqualFold(cell.Ref, group.AnchorRef) {
					cell.Formula = ""
				}
			}
		}
	}
	return true
}

// SetSharedFormula creates or replaces a shared formula over an A1-style range.
func (s *Sheet) SetSharedFormula(rangeRef, formula string) (*sheetir.SharedFormula, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	formula = strings.TrimSpace(formula)
	if formula == "" {
		return nil, fmt.Errorf("sheet edit: shared formula is empty")
	}
	bounds, err := parseRangeRef(rangeRef)
	if err != nil {
		return nil, err
	}
	anchorRef := cellRef(bounds.topRow, bounds.leftCol)
	group := &sheetir.SharedFormula{
		Index:     s.nextSharedFormulaIndex(),
		AnchorRef: anchorRef,
		RangeRef:  normalizedRangeRef(bounds),
		Formula:   formula,
		TopRow:    bounds.topRow,
		LeftCol:   bounds.leftCol,
		BottomRow: bounds.bottomRow,
		RightCol:  bounds.rightCol,
	}
	s.removeSharedFormulaGroup(anchorRef)
	s.sheet.SharedFormulas = append(s.sheet.SharedFormulas, group)
	for row := bounds.topRow; row <= bounds.bottomRow; row++ {
		for col := bounds.leftCol; col <= bounds.rightCol; col++ {
			cell, err := s.CreateCell(row, col)
			if err != nil {
				return nil, err
			}
			cell.cell.Kind = sheetir.CellFormula
			if row == bounds.topRow && col == bounds.leftCol {
				cell.cell.Formula = formula
			}
		}
	}
	return group, nil
}

// ClearSharedFormula removes shared formula metadata for a group anchor.
func (s *Sheet) ClearSharedFormula(anchorRef string) bool {
	if s == nil || s.sheet == nil {
		return false
	}
	anchorRef = strings.ToUpper(strings.TrimSpace(anchorRef))
	group, ok := s.sharedFormulaGroup(anchorRef)
	if !ok {
		return false
	}
	s.removeSharedFormulaGroup(anchorRef)
	for _, row := range s.sheet.Rows {
		for _, cell := range row.Cells {
			if inFormulaBounds(cell.Row, cell.Col, group.TopRow, group.LeftCol, group.BottomRow, group.RightCol) {
				cell.Formula = ""
			}
		}
	}
	return true
}

func (s *Sheet) arrayFormulaGroup(anchorRef string) (*sheetir.ArrayFormula, bool) {
	for _, group := range s.sheet.ArrayFormulas {
		if group != nil && strings.EqualFold(group.AnchorRef, anchorRef) {
			return group, true
		}
	}
	return nil, false
}

func (s *Sheet) removeArrayFormulaGroup(anchorRef string) {
	filtered := s.sheet.ArrayFormulas[:0]
	for _, group := range s.sheet.ArrayFormulas {
		if group == nil || strings.EqualFold(group.AnchorRef, anchorRef) {
			continue
		}
		filtered = append(filtered, group)
	}
	s.sheet.ArrayFormulas = filtered
}

func (s *Sheet) sharedFormulaGroup(anchorRef string) (*sheetir.SharedFormula, bool) {
	for _, group := range s.sheet.SharedFormulas {
		if group != nil && strings.EqualFold(group.AnchorRef, anchorRef) {
			return group, true
		}
	}
	return nil, false
}

func (s *Sheet) removeSharedFormulaGroup(anchorRef string) {
	filtered := s.sheet.SharedFormulas[:0]
	for _, group := range s.sheet.SharedFormulas {
		if group == nil || strings.EqualFold(group.AnchorRef, anchorRef) {
			continue
		}
		filtered = append(filtered, group)
	}
	s.sheet.SharedFormulas = filtered
}

func (s *Sheet) nextSharedFormulaIndex() int {
	maxIndex := -1
	for _, group := range s.sheet.SharedFormulas {
		if group != nil && group.Index > maxIndex {
			maxIndex = group.Index
		}
	}
	return maxIndex + 1
}

type rangeBounds struct {
	topRow, leftCol, bottomRow, rightCol int
}

func parseRangeRef(ref string) (rangeBounds, error) {
	ref = strings.ToUpper(strings.TrimSpace(ref))
	if ref == "" {
		return rangeBounds{}, fmt.Errorf("sheet edit: formula range is empty")
	}
	parts := strings.Split(ref, ":")
	if len(parts) == 1 {
		row, col, err := parseCellRef(parts[0])
		if err != nil {
			return rangeBounds{}, err
		}
		return rangeBounds{topRow: row, leftCol: col, bottomRow: row, rightCol: col}, nil
	}
	if len(parts) != 2 {
		return rangeBounds{}, fmt.Errorf("sheet edit: invalid formula range %q", ref)
	}
	startRow, startCol, err := parseCellRef(parts[0])
	if err != nil {
		return rangeBounds{}, err
	}
	endRow, endCol, err := parseCellRef(parts[1])
	if err != nil {
		return rangeBounds{}, err
	}
	if endRow < startRow || endCol < startCol {
		return rangeBounds{}, fmt.Errorf("sheet edit: invalid formula range %q", ref)
	}
	return rangeBounds{topRow: startRow, leftCol: startCol, bottomRow: endRow, rightCol: endCol}, nil
}

func parseCellRef(ref string) (row, col int, err error) {
	ref = strings.TrimSpace(strings.TrimPrefix(ref, "$"))
	if ref == "" {
		return 0, 0, fmt.Errorf("sheet edit: empty cell reference")
	}
	i := 0
	for i < len(ref) && (unicode.IsLetter(rune(ref[i])) || ref[i] == '$') {
		if ref[i] != '$' {
			col = col*26 + int(unicode.ToUpper(rune(ref[i]))-'A') + 1
		}
		i++
	}
	if col == 0 || i >= len(ref) {
		return 0, 0, fmt.Errorf("sheet edit: invalid cell reference %q", ref)
	}
	rowText := strings.ReplaceAll(ref[i:], "$", "")
	row, err = strconv.Atoi(rowText)
	if err != nil || row < 1 {
		return 0, 0, fmt.Errorf("sheet edit: invalid cell reference %q", ref)
	}
	return row, col, nil
}

func normalizedRangeRef(bounds rangeBounds) string {
	start := cellRef(bounds.topRow, bounds.leftCol)
	end := cellRef(bounds.bottomRow, bounds.rightCol)
	if start == end {
		return start
	}
	return start + ":" + end
}

func inFormulaBounds(row, col, topRow, leftCol, bottomRow, rightCol int) bool {
	return row >= topRow && row <= bottomRow && col >= leftCol && col <= rightCol
}
