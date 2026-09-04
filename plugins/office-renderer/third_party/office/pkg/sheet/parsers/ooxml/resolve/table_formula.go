package resolve

import (
	"strconv"
	"strings"
)

// ExpandStructuredTableRefs rewrites common Excel structured-table references
// such as Table1[Sales] into ordinary A1 ranges for workbook-scoped formula
// evaluation. It currently handles full-table, data/header/totals sections,
// and single- or multi-column spans, but not row-relative [@Column] forms.
func ExpandStructuredTableRefs(expr string, wb *Workbook, sheetIndex int) string {
	if expr == "" || wb == nil {
		return expr
	}
	tables := buildStructuredTableFormulaLookup(wb)
	if len(tables) == 0 {
		return expr
	}
	var out strings.Builder
	for i := 0; i < len(expr); {
		if expr[i] == '"' {
			start := i
			i++
			for i < len(expr) {
				if expr[i] == '"' {
					i++
					if i < len(expr) && expr[i] == '"' {
						i++
						continue
					}
					break
				}
				i++
			}
			out.WriteString(expr[start:i])
			continue
		}
		if isWorkbookDefinedNameLead(expr, i) {
			name, n := readWorkbookDefinedNameIdentifier(expr[i:])
			if n > 0 {
				end := i + n
				if tableRef, ok := tables[strings.ToUpper(name)]; ok && end < len(expr) && expr[end] == '[' {
					spec, consumed, ok := readStructuredTableSpec(expr[end:])
					if ok {
						if replacement, ok := structuredTableReplacement(tableRef, spec); ok {
							out.WriteString(replacement)
							i = end + consumed
							continue
						}
					}
				}
				out.WriteString(expr[i:end])
				i = end
				continue
			}
		}
		out.WriteByte(expr[i])
		i++
	}
	return out.String()
}

type structuredTableFormulaRef struct {
	sheetName       string
	table           *Table
	firstRow        int
	firstCol        int
	lastRow         int
	lastCol         int
	columnIndexByID map[string]int
}

func buildStructuredTableFormulaLookup(wb *Workbook) map[string]structuredTableFormulaRef {
	if wb == nil {
		return nil
	}
	lookup := make(map[string]structuredTableFormulaRef)
	for _, sheet := range wb.Sheets {
		if sheet == nil {
			continue
		}
		for _, table := range sheet.Tables {
			if table == nil || strings.TrimSpace(table.Ref) == "" {
				continue
			}
			firstRow, firstCol, lastRow, lastCol, ok := parsePivotFormulaRange(table.Ref)
			if !ok {
				continue
			}
			ref := structuredTableFormulaRef{
				sheetName:       sheet.Name,
				table:           table,
				firstRow:        firstRow,
				firstCol:        firstCol,
				lastRow:         lastRow,
				lastCol:         lastCol,
				columnIndexByID: make(map[string]int),
			}
			for idx, col := range table.Columns {
				if col == nil || strings.TrimSpace(col.Name) == "" {
					continue
				}
				ref.columnIndexByID[strings.ToUpper(strings.TrimSpace(col.Name))] = firstCol + idx
			}
			if strings.TrimSpace(table.Name) != "" {
				lookup[strings.ToUpper(strings.TrimSpace(table.Name))] = ref
			}
			if strings.TrimSpace(table.DisplayName) != "" {
				lookup[strings.ToUpper(strings.TrimSpace(table.DisplayName))] = ref
			}
		}
	}
	return lookup
}

func readStructuredTableSpec(s string) (string, int, bool) {
	if s == "" || s[0] != '[' {
		return "", 0, false
	}
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return s[:i+1], i + 1, true
			}
		}
	}
	return "", 0, false
}

func structuredTableReplacement(ref structuredTableFormulaRef, spec string) (string, bool) {
	section, startCol, endCol, ok := parseStructuredTableTokens(spec)
	if !ok {
		return "", false
	}
	firstRow, lastRow, ok := structuredTableSectionRows(ref, section)
	if !ok {
		return "", false
	}
	firstCol := ref.firstCol
	lastCol := ref.lastCol
	if startCol != "" {
		var colOK bool
		firstCol, lastCol, colOK = structuredTableColumns(ref, startCol, endCol)
		if !colOK {
			return "", false
		}
	}
	if firstRow < 1 || lastRow < firstRow || firstCol < 1 || lastCol < firstCol {
		return "#REF!", true
	}
	startRef := colLettersFromIndex(firstCol) + strconv.Itoa(firstRow)
	endRef := colLettersFromIndex(lastCol) + strconv.Itoa(lastRow)
	prefix := quoteFormulaSheetName(ref.sheetName) + "!"
	if startRef == endRef {
		return prefix + startRef, true
	}
	return prefix + startRef + ":" + endRef, true
}

func parseStructuredTableTokens(spec string) (section, startCol, endCol string, ok bool) {
	spec = strings.TrimSpace(spec)
	if len(spec) < 2 || spec[0] != '[' || spec[len(spec)-1] != ']' {
		return "", "", "", false
	}
	inner := spec[1 : len(spec)-1]
	if !strings.Contains(inner, "[") {
		token := strings.TrimSpace(inner)
		if strings.HasPrefix(token, "@") || strings.EqualFold(token, "#THIS ROW") {
			return "", "", "", false
		}
		if strings.HasPrefix(token, "#") {
			return token, "", "", true
		}
		return "#Data", token, token, true
	}
	var tokens []string
	for i := 0; i < len(inner); {
		if inner[i] != '[' {
			i++
			continue
		}
		end := i + 1
		for end < len(inner) && inner[end] != ']' {
			end++
		}
		if end >= len(inner) {
			return "", "", "", false
		}
		tokens = append(tokens, strings.TrimSpace(inner[i+1:end]))
		i = end + 1
	}
	switch len(tokens) {
	case 1:
		if strings.HasPrefix(tokens[0], "#") {
			return tokens[0], "", "", true
		}
		return "#Data", tokens[0], tokens[0], true
	case 2:
		if strings.HasPrefix(tokens[0], "#") {
			return tokens[0], tokens[1], tokens[1], true
		}
		return "#Data", tokens[0], tokens[1], true
	case 3:
		if strings.HasPrefix(tokens[0], "#") {
			return tokens[0], tokens[1], tokens[2], true
		}
	}
	return "", "", "", false
}

func structuredTableSectionRows(ref structuredTableFormulaRef, section string) (int, int, bool) {
	headerRows := ref.table.HeaderRowCount
	if headerRows <= 0 {
		headerRows = 1
	}
	totalsRows := ref.table.TotalsRowCount
	if totalsRows < 0 {
		totalsRows = 0
	}
	dataFirst := ref.firstRow + headerRows
	dataLast := ref.lastRow - totalsRows
	switch strings.ToUpper(strings.TrimSpace(section)) {
	case "", "#DATA":
		return dataFirst, dataLast, true
	case "#ALL":
		return ref.firstRow, ref.lastRow, true
	case "#HEADERS":
		return ref.firstRow, ref.firstRow + headerRows - 1, true
	case "#TOTALS":
		if totalsRows == 0 {
			return 0, 0, false
		}
		return ref.lastRow - totalsRows + 1, ref.lastRow, true
	}
	return 0, 0, false
}

func structuredTableColumns(ref structuredTableFormulaRef, startCol, endCol string) (int, int, bool) {
	firstCol, ok := ref.columnIndexByID[strings.ToUpper(strings.TrimSpace(startCol))]
	if !ok {
		return 0, 0, false
	}
	lastCol, ok := ref.columnIndexByID[strings.ToUpper(strings.TrimSpace(endCol))]
	if !ok {
		return 0, 0, false
	}
	if lastCol < firstCol {
		firstCol, lastCol = lastCol, firstCol
	}
	return firstCol, lastCol, true
}

func quoteFormulaSheetName(name string) string {
	if name == "" {
		return name
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.') {
			return "'" + strings.ReplaceAll(name, "'", "''") + "'"
		}
	}
	return name
}
