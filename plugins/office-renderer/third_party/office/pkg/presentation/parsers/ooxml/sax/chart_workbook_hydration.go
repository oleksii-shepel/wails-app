package sax

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	chartpkg "office/pkg/ooxml/chart"
	sheetir "office/pkg/sheet/ir"
	sheetooxml "office/pkg/sheet/parsers/ooxml"
)

// resolveOOXMLChartData keeps the authored chart XML when it already contains
// usable cached points. When caches are absent, it resolves the chart's own
// embedded workbook and hydrates formula-backed series before serialising the
// canonical chart back to ChartData. The preview image remains a fallback only.
func (b *irBuilder) resolveOOXMLChartData(ev Event, chartData []byte) []byte {
	authored := append([]byte(nil), chartData...)
	if len(authored) == 0 {
		return authored
	}
	c, err := chartpkg.ParseChart(authored)
	if err != nil || c == nil || chartHasResolvedPoints(c) {
		return authored
	}
	rel := b.relationshipForEvent(ev)
	chartPart := rel.Target
	if chartPart == "" {
		return authored
	}
	workbook := b.embeddedChartWorkbook(chartPart)
	if len(workbook) == 0 {
		return authored
	}
	wb, err := sheetooxml.ParseBytes(workbook)
	if err != nil || wb == nil {
		return authored
	}
	if !hydrateChartFromWorkbook(c, wb) {
		return authored
	}
	resolved, err := chartpkg.MarshalChart(c)
	if err != nil || len(resolved) == 0 {
		return authored
	}
	return resolved
}

func (b *irBuilder) embeddedChartWorkbook(chartPart string) []byte {
	for relID, rel := range b.partRelationships[chartPart] {
		if rel.External {
			continue
		}
		relType := strings.ToLower(rel.Target)
		payload := b.partPayloads[chartPart][relID]
		canonical := strings.ToLower(payload.RelType)
		if strings.HasSuffix(canonical, "/package") || strings.HasSuffix(relType, ".xlsx") || strings.Contains(strings.ToLower(payload.ContentType), "spreadsheetml.sheet") {
			if len(payload.Data) > 0 {
				return append([]byte(nil), payload.Data...)
			}
		}
	}
	return nil
}

func chartHasResolvedPoints(c *chartpkg.Chart) bool {
	for _, s := range c.Series {
		if len(s.Values) > 0 || len(s.XValues) > 0 || len(s.BubbleSizes) > 0 {
			return true
		}
	}
	return false
}

func hydrateChartFromWorkbook(c *chartpkg.Chart, wb *sheetir.Workbook) bool {
	if c == nil || wb == nil {
		return false
	}
	changed := false
	for i := range c.Series {
		s := &c.Series[i]
		if s.Name == "" && s.NameFormula != "" {
			if cells := workbookCellsForFormula(wb, s.NameFormula); len(cells) > 0 {
				s.Name = chartCellText(cells[0])
				changed = changed || s.Name != ""
			}
		}
		if len(s.Values) == 0 && s.ValueFormula != "" {
			cells := workbookCellsForFormula(wb, s.ValueFormula)
			vals := numericChartCells(cells)
			if len(vals) > 0 {
				s.Values = vals
				changed = true
			}
		}
		if len(s.XValues) == 0 && s.XValueFormula != "" {
			cells := workbookCellsForFormula(wb, s.XValueFormula)
			vals := numericChartCells(cells)
			if len(vals) > 0 {
				s.XValues = vals
				changed = true
			}
		}
		if len(s.Labels) == 0 && s.LabelFormula != "" {
			cells := workbookCellsForFormula(wb, s.LabelFormula)
			if len(cells) > 0 {
				labels := make([]string, 0, len(cells))
				for _, cell := range cells {
					labels = append(labels, chartCellText(cell))
				}
				s.Labels = labels
				if len(c.Categories) == 0 {
					c.Categories = append([]string(nil), labels...)
				}
				changed = true
			}
		}
	}
	return changed
}

func workbookCellsForFormula(wb *sheetir.Workbook, formula string) []*sheetir.Cell {
	sheetName, startRow, startCol, endRow, endCol, ok := parseChartA1Range(formula)
	if !ok {
		return nil
	}
	var sh *sheetir.Sheet
	for _, candidate := range wb.Sheets {
		if candidate != nil && strings.EqualFold(candidate.Name, sheetName) {
			sh = candidate
			break
		}
	}
	if sh == nil {
		return nil
	}
	byPos := make(map[[2]int]*sheetir.Cell)
	for _, row := range sh.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell != nil {
				byPos[[2]int{cell.Row, cell.Col}] = cell
			}
		}
	}
	out := make([]*sheetir.Cell, 0, (endRow-startRow+1)*(endCol-startCol+1))
	// Preserve Excel range traversal semantics: row-major for horizontal or
	// rectangular ranges, naturally column-major for a one-column range.
	for r := startRow; r <= endRow; r++ {
		for col := startCol; col <= endCol; col++ {
			if cell := byPos[[2]int{r, col}]; cell != nil {
				out = append(out, cell)
			}
		}
	}
	return out
}

func parseChartA1Range(formula string) (sheet string, sr, sc, er, ec int, ok bool) {
	f := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(formula), "="))
	bang := strings.LastIndex(f, "!")
	if bang <= 0 || bang >= len(f)-1 {
		return "", 0, 0, 0, 0, false
	}
	sheet = strings.TrimSpace(f[:bang])
	// Strip external workbook prefix such as [1] or [Book.xlsx].
	if close := strings.LastIndex(sheet, "]"); close >= 0 {
		sheet = sheet[close+1:]
	}
	if len(sheet) >= 2 && sheet[0] == '\'' && sheet[len(sheet)-1] == '\'' {
		sheet = strings.ReplaceAll(sheet[1:len(sheet)-1], "''", "'")
	}
	refs := strings.SplitN(strings.TrimSpace(f[bang+1:]), ":", 2)
	sr, sc, ok = parseChartA1Cell(refs[0])
	if !ok {
		return "", 0, 0, 0, 0, false
	}
	er, ec = sr, sc
	if len(refs) == 2 {
		er, ec, ok = parseChartA1Cell(refs[1])
		if !ok {
			return "", 0, 0, 0, 0, false
		}
	}
	if er < sr {
		sr, er = er, sr
	}
	if ec < sc {
		sc, ec = ec, sc
	}
	return sheet, sr, sc, er, ec, true
}

func parseChartA1Cell(ref string) (row, col int, ok bool) {
	r := strings.ReplaceAll(strings.TrimSpace(ref), "$", "")
	i := 0
	for i < len(r) && unicode.IsLetter(rune(r[i])) {
		c := unicode.ToUpper(rune(r[i]))
		col = col*26 + int(c-'A'+1)
		i++
	}
	if i == 0 || i == len(r) {
		return 0, 0, false
	}
	n, err := strconv.Atoi(r[i:])
	if err != nil || n <= 0 || col <= 0 {
		return 0, 0, false
	}
	return n, col, true
}

func numericChartCells(cells []*sheetir.Cell) []float64 {
	out := make([]float64, 0, len(cells))
	for _, cell := range cells {
		if cell == nil {
			continue
		}
		switch cell.Kind {
		case sheetir.CellNumber, sheetir.CellBool:
			out = append(out, cell.NumValue)
		default:
			if v, err := strconv.ParseFloat(strings.TrimSpace(cell.StrValue), 64); err == nil {
				out = append(out, v)
			}
		}
	}
	return out
}

func chartCellText(cell *sheetir.Cell) string {
	if cell == nil {
		return ""
	}
	switch cell.Kind {
	case sheetir.CellNumber:
		return strconv.FormatFloat(cell.NumValue, 'g', -1, 64)
	case sheetir.CellBool:
		if cell.NumValue != 0 {
			return "TRUE"
		}
		return "FALSE"
	default:
		if cell.StrValue != "" {
			return cell.StrValue
		}
		if cell.Formula != "" && cell.HasCachedValue {
			return fmt.Sprint(cell.NumValue)
		}
		return ""
	}
}
