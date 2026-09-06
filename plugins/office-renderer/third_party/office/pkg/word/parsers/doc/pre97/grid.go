package pre97

import "strings"

const GridBoundaryKind = "legacy-list-boundary"

// NewGridCursor reconstructs dense Word 6/95 cell-grid runs into bounded
// logical table-row runs. This is a recovery heuristic for producer variants
// whose old table model arrives as repeated bell markers inside one paragraph.
func NewGridCursor(inner Cursor) Cursor {
	if inner == nil { return nil }
	return &gridCursor{inner:inner}
}

type gridCursor struct { inner Cursor; queue []Run }
func (c *gridCursor) Next() (Run, bool, error) {
	for {
		if len(c.queue) > 0 { r := c.queue[0]; c.queue = c.queue[1:]; return r, true, nil }
		r, ok, err := c.inner.Next(); if err != nil || !ok { return r, ok, err }
		c.queue = append(c.queue, NormalizeGridRun(r)...)
	}
}

// NormalizeGridRun mirrors the mature Word 6/95 dense-grid rescue while using
// only the canonical Run contract.
func NormalizeGridRun(run Run) []Run {
	bellCount := strings.Count(run.Text, "\a")
	if bellCount < 12 { return []Run{run} }
	grid := inferGridWidth(run.Text, bellCount)
	if grid <= 0 { return []Run{run} }
	rows := splitGridRows(run.Text, grid)
	if len(rows) < 2 { return []Run{run} }
	out := make([]Run, 0, len(rows)+1)
	cp := run.StartCP
	for _, text := range rows {
		cloned := run
		cloned.StartCP = cp
		cloned.Text = text
		cloned.LegacyInTableHint = false
		cloned.ParagraphFormat.InTable = false
		cloned.ParagraphFormat.IsTableCellTrailingParagraph = false
		cloned.ParagraphFormat.IsTableTrailingParagraph = false
		out = append(out, cloned)
		cp += len([]rune(text))
	}
	out = append(out, Run{Kind:GridBoundaryKind, StartCP:cp})
	return out
}

func inferGridWidth(text string, bellCount int) int {
	if bellCount < 12 { return 0 }
	bestGrid, bestScore := 0, -1
	for grid := 4; grid <= 24; grid++ {
		if bellCount%grid != 0 { continue }
		rows := splitGridRows(text, grid); if len(rows) < 3 { continue }
		counts := make([]int, 0, len(rows))
		for _, row := range rows {
			clean := strings.Trim(row, "\r\n\f"); if clean == "" { continue }
			cells := 1; if strings.Contains(clean, "\t") { cells = len(strings.Split(clean, "\t")) }
			counts = append(counts, cells)
		}
		if len(counts) < 3 { continue }
		freq := map[int]int{}; mode, modeN := 0, 0
		for _, n := range counts {
			if n < 2 || n > 8 { continue }
			freq[n]++; if freq[n] > modeN { mode, modeN = n, freq[n] }
		}
		if mode < 2 || mode > 6 || modeN < 2 { continue }
		score := modeN*(mode-1)*(mode-1) - (len(counts)-modeN)
		if score > bestScore || score == bestScore && (bestGrid == 0 || grid < bestGrid) { bestGrid, bestScore = grid, score }
	}
	return bestGrid
}

func splitGridRows(text string, grid int) []string {
	if grid <= 0 { return nil }
	leadingBreak := false
	if strings.HasPrefix(text, "\x0e") { leadingBreak = true; text = strings.TrimPrefix(text, "\x0e") }
	var rows, cells []string
	var cell strings.Builder
	bells := 0
	flushCell := func() {
		value := strings.TrimSpace(strings.ReplaceAll(cell.String(), "\v", " "))
		if value != "" { cells = append(cells, value) }
		cell.Reset()
	}
	flushRow := func() {
		flushCell(); if len(cells) == 0 { return }
		row := strings.Join(cells, "\t"); if len(cells) == 1 { row += "\t" }; row += "\r"
		if leadingBreak && len(rows) == 0 { row = "\f"+row; leadingBreak = false }
		rows = append(rows, row); cells = cells[:0]
	}
	for _, ch := range text {
		if ch != '\a' { if ch != '\r' && ch != '\n' { cell.WriteRune(ch) }; continue }
		flushCell(); bells++; if bells%grid == 0 { flushRow() }
	}
	if cell.Len() > 0 || len(cells) > 0 { flushRow() }
	return rows
}
