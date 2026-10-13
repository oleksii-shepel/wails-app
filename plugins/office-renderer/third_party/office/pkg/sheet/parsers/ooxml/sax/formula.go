package sax

import (
	"strconv"
	"strings"
)

// TranslateFormula applies a row/column offset to relative A1 references.
// It is used for the implicit members of OOXML shared-formula groups.
func TranslateFormula(formula string, rowDelta, colDelta int) string {
	if formula == "" || (rowDelta == 0 && colDelta == 0) {
		return formula
	}
	var out strings.Builder
	for i := 0; i < len(formula); {
		if formula[i] == '"' {
			start := i
			i++
			for i < len(formula) {
				if formula[i] == '"' {
					i++
					if i < len(formula) && formula[i] == '"' {
						i++
						continue
					}
					break
				}
				i++
			}
			out.WriteString(formula[start:i])
			continue
		}
		if isFormulaCellRefLead(formula, i) {
			if token, n, ok := readFormulaCellRef(formula[i:]); ok {
				out.WriteString(shiftFormulaCellRef(token, rowDelta, colDelta))
				i += n
				continue
			}
		}
		out.WriteByte(formula[i])
		i++
	}
	return out.String()
}

func isFormulaCellRefLead(formula string, index int) bool {
	if index < 0 || index >= len(formula) {
		return false
	}
	ch := formula[index]
	if ch != '$' && !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')) {
		return false
	}
	if index == 0 {
		return true
	}
	previous := formula[index-1]
	return !((previous >= 'A' && previous <= 'Z') || (previous >= 'a' && previous <= 'z') || (previous >= '0' && previous <= '9') || previous == '_' || previous == '.')
}

func readFormulaCellRef(value string) (string, int, bool) {
	i := 0
	if i < len(value) && value[i] == '$' {
		i++
	}
	columnStart := i
	for i < len(value) && ((value[i] >= 'A' && value[i] <= 'Z') || (value[i] >= 'a' && value[i] <= 'z')) {
		i++
	}
	if i == columnStart {
		return "", 0, false
	}
	if i < len(value) && value[i] == '$' {
		i++
	}
	rowStart := i
	for i < len(value) && value[i] >= '0' && value[i] <= '9' {
		i++
	}
	if i == rowStart {
		return "", 0, false
	}
	return value[:i], i, true
}

func shiftFormulaCellRef(token string, rowDelta, colDelta int) string {
	index, columnAbsolute, rowAbsolute := 0, false, false
	if token[index] == '$' {
		columnAbsolute = true
		index++
	}
	columnStart := index
	for index < len(token) && ((token[index] >= 'A' && token[index] <= 'Z') || (token[index] >= 'a' && token[index] <= 'z')) {
		index++
	}
	column := formulaColumnIndex(token[columnStart:index])
	if index < len(token) && token[index] == '$' {
		rowAbsolute = true
		index++
	}
	row, err := strconv.Atoi(token[index:])
	if err != nil || column < 1 {
		return token
	}
	if !columnAbsolute {
		column += colDelta
	}
	if !rowAbsolute {
		row += rowDelta
	}
	if column < 1 || row < 1 {
		return "#REF!"
	}
	var out strings.Builder
	if columnAbsolute {
		out.WriteByte('$')
	}
	out.WriteString(formulaColumnLetters(column))
	if rowAbsolute {
		out.WriteByte('$')
	}
	out.WriteString(strconv.Itoa(row))
	return out.String()
}

func formulaColumnIndex(value string) int {
	column := 0
	for _, ch := range strings.ToUpper(value) {
		if ch < 'A' || ch > 'Z' {
			return 0
		}
		column = column*26 + int(ch-'A'+1)
	}
	return column
}

func formulaColumnLetters(column int) string {
	var out [8]byte
	i := len(out)
	for column > 0 {
		column--
		i--
		out[i] = byte('A' + column%26)
		column /= 26
	}
	return string(out[i:])
}
