package ir

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// PrintPage describes one physical worksheet print page in sheet coordinates.
type PrintPage struct {
	FirstRow, LastRow            int
	FirstCol, LastCol            int
	RepeatRowsFrom, RepeatRowsTo int
	RepeatColsFrom, RepeatColsTo int
	Scale                        float64
	// PageWidthPt/PageHeightPt are the physical paper dimensions after orientation.
	PageWidthPt, PageHeightPt float64
	// Margins are physical page margins in points. Header/Footer are the authored
	// distances from the corresponding paper edge.
	MarginLeftPt, MarginRightPt, MarginTopPt, MarginBottomPt float64
	HeaderMarginPt, FooterMarginPt                           float64
}

// PrintPages computes canonical physical page slices from worksheet geometry and
// print settings. It intentionally lives in IR so HTML/PDF renderers consume the
// same pagination decision instead of reinterpreting pageSetup independently.
func (s *Sheet) PrintPages() []PrintPage {
	if s == nil {
		return nil
	}
	fr, lr, fc, lc := s.printBounds()
	if fr == 0 || lr < fr || fc == 0 || lc < fc {
		return nil
	}
	rr1, rr2 := parseRowTitleRange(s.PrintTitleRows)
	rc1, rc2 := parseColTitleRange(s.PrintTitleCols)
	ps := s.PageSetup
	if ps == nil {
		ps = &PageSetup{}
	}
	pageW, pageH := paperSizePoints(ps.PaperSize)
	if strings.EqualFold(ps.Orientation, "landscape") {
		pageW, pageH = pageH, pageW
	}
	ml, mr, mt, mb := ps.MarginLeft*72, ps.MarginRight*72, ps.MarginTop*72, ps.MarginBottom*72
	if ml == 0 {
		ml = 0.7 * 72
	}
	if mr == 0 {
		mr = 0.7 * 72
	}
	if mt == 0 {
		mt = 0.75 * 72
	}
	if mb == 0 {
		mb = 0.75 * 72
	}
	availW, availH := math.Max(1, pageW-ml-mr), math.Max(1, pageH-mt-mb)
	titleW, titleH := s.colsWidthPoints(rc1, rc2), s.rowsHeightPoints(rr1, rr2)
	scale := 1.0
	if ps.Scale > 0 {
		scale = float64(ps.Scale) / 100
	}
	totalW, totalH := s.colsWidthPoints(fc, lc), s.rowsHeightPoints(fr, lr)
	if ps.FitToPage {
		sx, sy := math.Inf(1), math.Inf(1)
		if ps.FitToWidth > 0 {
			sx = availW * float64(ps.FitToWidth) / math.Max(totalW, 1)
		}
		if ps.FitToHeight > 0 {
			sy = availH * float64(ps.FitToHeight) / math.Max(totalH, 1)
		}
		if v := math.Min(sx, sy); !math.IsInf(v, 1) && v > 0 {
			scale = math.Min(1, v)
		}
	}
	colSlices := s.paginateCols(fc, lc, math.Max(1, availW/scale-titleW), s.ColPageBreaks)
	rowSlices := s.paginateRows(fr, lr, math.Max(1, availH/scale-titleH), s.RowPageBreaks)
	headerMargin, footerMargin := ps.MarginHeader*72, ps.MarginFooter*72
	if headerMargin == 0 {
		headerMargin = 0.3 * 72
	}
	if footerMargin == 0 {
		footerMargin = 0.3 * 72
	}
	pages := make([]PrintPage, 0, len(colSlices)*len(rowSlices))
	appendPage := func(r, c [2]int) {
		pages = append(pages, PrintPage{
			FirstRow: r[0], LastRow: r[1], FirstCol: c[0], LastCol: c[1],
			RepeatRowsFrom: rr1, RepeatRowsTo: rr2, RepeatColsFrom: rc1, RepeatColsTo: rc2, Scale: scale,
			PageWidthPt: pageW, PageHeightPt: pageH,
			MarginLeftPt: ml, MarginRightPt: mr, MarginTopPt: mt, MarginBottomPt: mb,
			HeaderMarginPt: headerMargin, FooterMarginPt: footerMargin,
		})
	}
	if ps.PageOrder == "overThenDown" {
		for _, r := range rowSlices {
			for _, c := range colSlices {
				appendPage(r, c)
			}
		}
	} else {
		for _, c := range colSlices {
			for _, r := range rowSlices {
				appendPage(r, c)
			}
		}
	}
	return pages
}

func (s *Sheet) printBounds() (fr, lr, fc, lc int) {
	if a, b, c, d, ok := parseAreaBounds(s.PrintArea); ok {
		return a, b, c, d
	}
	for _, r := range s.Rows {
		if r == nil {
			continue
		}
		if fr == 0 || r.Index < fr {
			fr = r.Index
		}
		if r.Index > lr {
			lr = r.Index
		}
		for _, cell := range r.Cells {
			if cell == nil {
				continue
			}
			if fc == 0 || cell.Col < fc {
				fc = cell.Col
			}
			if cell.Col > lc {
				lc = cell.Col
			}
		}
	}
	if fr == 0 && s.DimFirstRow > 0 {
		fr, lr, fc, lc = s.DimFirstRow, s.DimLastRow, s.DimFirstCol, s.DimLastCol
	}
	return
}

func (s *Sheet) paginateCols(first, last int, capacity float64, manual []int) [][2]int {
	return paginateAxis(first, last, capacity, manual, func(i int) float64 { return s.colWidthPoints(i) })
}
func (s *Sheet) paginateRows(first, last int, capacity float64, manual []int) [][2]int {
	return paginateAxis(first, last, capacity, manual, func(i int) float64 { return s.rowHeightPoints(i) })
}
func paginateAxis(first, last int, capacity float64, manual []int, size func(int) float64) [][2]int {
	breaks := map[int]bool{}
	for _, b := range manual {
		if b > first && b <= last {
			breaks[b] = true
		}
	}
	out := [][2]int{}
	start := first
	used := 0.0
	for i := first; i <= last; i++ {
		w := size(i)
		if i > start && (breaks[i] || (w > 0 && used+w > capacity)) {
			out = append(out, [2]int{start, i - 1})
			start = i
			used = 0
		}
		used += w
	}
	if start <= last {
		out = append(out, [2]int{start, last})
	}
	return out
}
func (s *Sheet) colWidthPoints(c int) float64 {
	if s.HiddenCols[c] {
		return 0
	}
	w := s.ColumnWidths[c]
	if w <= 0 {
		w = s.DefaultColWidthChars
	}
	if w <= 0 {
		w = 8.43
	}
	px := math.Floor(((256*w + math.Floor(128.0/7.0)) / 256.0) * 7.0)
	return px * 0.75
}
func (s *Sheet) rowHeightPoints(r int) float64 {
	for _, row := range s.Rows {
		if row != nil && row.Index == r {
			if row.Hidden {
				return 0
			}
			if row.HeightPt > 0 {
				return row.HeightPt
			}
			break
		}
	}
	if s.DefaultRowHeightZeroHeight {
		return 0
	}
	if s.DefaultRowHeightPt > 0 {
		return s.DefaultRowHeightPt
	}
	return 15
}
func (s *Sheet) colsWidthPoints(a, b int) float64 {
	if a <= 0 || b < a {
		return 0
	}
	v := 0.0
	for i := a; i <= b; i++ {
		v += s.colWidthPoints(i)
	}
	return v
}
func (s *Sheet) rowsHeightPoints(a, b int) float64 {
	if a <= 0 || b < a {
		return 0
	}
	v := 0.0
	for i := a; i <= b; i++ {
		v += s.rowHeightPoints(i)
	}
	return v
}
func paperSizePoints(code int) (float64, float64) {
	switch code {
	case 1:
		return 612, 792
	case 5:
		return 612, 1008
	case 8:
		return 841.89, 1190.55
	case 9:
		return 595.28, 841.89
	case 11:
		return 419.53, 595.28
	default:
		return 612, 792
	}
}
func parseAreaBounds(v string) (int, int, int, int, bool) {
	if i := strings.Index(v, ","); i >= 0 {
		v = v[:i]
	}
	if i := strings.LastIndex(v, "!"); i >= 0 {
		v = v[i+1:]
	}
	p := strings.Split(strings.ReplaceAll(v, "$", ""), ":")
	if len(p) != 2 {
		return 0, 0, 0, 0, false
	}
	r1, c1, o1 := a1(p[0])
	r2, c2, o2 := a1(p[1])
	if !o1 || !o2 {
		return 0, 0, 0, 0, false
	}
	if r1 > r2 {
		r1, r2 = r2, r1
	}
	if c1 > c2 {
		c1, c2 = c2, c1
	}
	return r1, r2, c1, c2, true
}
func a1(v string) (int, int, bool) {
	c := 0
	i := 0
	for i < len(v) && unicode.IsLetter(rune(v[i])) {
		c = c*26 + int(unicode.ToUpper(rune(v[i]))-'A'+1)
		i++
	}
	if c == 0 || i == len(v) {
		return 0, 0, false
	}
	r, e := strconv.Atoi(v[i:])
	return r, c, e == nil && r > 0
}
func parseRowTitleRange(v string) (int, int) {
	v = strings.ReplaceAll(v, "$", "")
	if i := strings.LastIndex(v, "!"); i >= 0 {
		v = v[i+1:]
	}
	p := strings.Split(v, ":")
	if len(p) != 2 {
		return 0, 0
	}
	a, _ := strconv.Atoi(p[0])
	b, _ := strconv.Atoi(p[1])
	return a, b
}
func parseColTitleRange(v string) (int, int) {
	v = strings.ReplaceAll(v, "$", "")
	if i := strings.LastIndex(v, "!"); i >= 0 {
		v = v[i+1:]
	}
	p := strings.Split(v, ":")
	if len(p) != 2 {
		return 0, 0
	}
	return colNum(p[0]), colNum(p[1])
}
func colNum(v string) int {
	n := 0
	for _, ch := range v {
		if !unicode.IsLetter(ch) {
			return 0
		}
		n = n*26 + int(unicode.ToUpper(ch)-'A'+1)
	}
	return n
}

// SortedPageBreaks returns normalized, unique manual breaks within bounds.
func SortedPageBreaks(in []int, first, last int) []int {
	m := map[int]bool{}
	for _, v := range in {
		if v > first && v <= last {
			m[v] = true
		}
	}
	out := make([]int, 0, len(m))
	for v := range m {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}
