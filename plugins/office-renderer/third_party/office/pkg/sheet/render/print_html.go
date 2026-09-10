package render

import (
	"encoding/base64"
	"fmt"
	"html"
	"strconv"
	"strings"

	"office/pkg/sheet/parsers/ooxml/resolve"
)

// PrintHTML renders the workbook as canonical physical worksheet pages.
// Pagination decisions come exclusively from Sheet.PrintPages().
func PrintHTML(wb *resolve.Workbook) (string, error) {
	html, _, err := PrintHTMLWithOptions(wb, HTMLOptions{})
	return html, err
}

// PrintHTMLWithOptions is the print-oriented counterpart of HTMLWithOptions.
// It preserves the normal renderer for each page while constraining each sheet
// view to the canonical page slice and its repeated title rows/columns.
func PrintHTMLWithOptions(wb *resolve.Workbook, opts HTMLOptions) (string, []HTMLAsset, error) {
	if wb == nil {
		return "", nil, fmt.Errorf("sheet render: workbook is nil")
	}
	if err := opts.validate(); err != nil {
		return "", nil, err
	}
	if opts.ExternalWorkbookResolver != nil {
		if err := resolve.RefreshExternalWorkbooks(wb, opts.ExternalWorkbookResolver); err != nil {
			return "", nil, err
		}
	}
	r := &htmlRenderer{wb: wb, options: opts.normalized(), embeddedFamilies: buildEmbeddedFamilyIndex(wb)}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n<style>\n")
	r.writeEmbeddedFontCSS(&b)
	b.WriteString(sheetCSS)
	b.WriteString(printPageCSS)
	b.WriteString("</style>\n<script>\n")
	b.WriteString(sheetJS)
	b.WriteString("</script>\n</head>\n<body class=\"print-document\">\n")

	physical := 0
	for sheetIndex, sheet := range wb.Sheets {
		if sheet == nil || sheet.Hidden {
			continue
		}
		pages := sheet.PrintPages()
		if len(pages) == 0 {
			continue
		}
		for pageIndex, page := range pages {
			physical++
			logicalPage := pageIndex + 1
			if sheet.PageSetup != nil && sheet.PageSetup.UseFirstPageNumber && sheet.PageSetup.FirstPageNumber > 0 {
				logicalPage = sheet.PageSetup.FirstPageNumber + pageIndex
			}
			fmt.Fprintf(&b, "<div class=\"print-page\" data-sheet=%q data-sheet-page=%q data-physical-page=%q data-print-scale=%q style=\"width:%spt;height:%spt;padding:%spt %spt %spt %spt;--hf-header:%spt;--hf-footer:%spt;--page-left:%spt;--page-right:%spt\">\n",
				sheet.Name, strconv.Itoa(logicalPage), strconv.Itoa(physical), strconv.FormatFloat(page.Scale, 'f', -1, 64),
				pt(page.PageWidthPt), pt(page.PageHeightPt), pt(page.MarginTopPt), pt(page.MarginRightPt), pt(page.MarginBottomPt), pt(page.MarginLeftPt),
				pt(page.HeaderMarginPt), pt(page.FooterMarginPt), pt(page.MarginLeftPt), pt(page.MarginRightPt))
			header, footer := printHeaderFooterForPage(sheet, pageIndex, logicalPage, len(pages))
			imageIndex := 0
			renderPrintHeaderFooter(&b, "header", header, sheet.HeaderFooter, &imageIndex)
			pageSheet := sheetForPrintPage(sheet, page.FirstRow, page.LastRow, page.FirstCol, page.LastCol,
				page.RepeatRowsFrom, page.RepeatRowsTo, page.RepeatColsFrom, page.RepeatColsTo)
			r.renderSheet(&b, pageSheet, sheetIndex)
			renderPrintHeaderFooter(&b, "footer", footer, sheet.HeaderFooter, &imageIndex)
			b.WriteString("</div>\n")
		}
	}
	b.WriteString("</body>\n</html>")
	return b.String(), r.assetsCopy(), nil
}

type printHFSections struct{ Left, Center, Right string }

func printHeaderFooterForPage(sheet *resolve.Sheet, pageIndex, pageNumber, pageCount int) (printHFSections, printHFSections) {
	if sheet == nil || sheet.HeaderFooter == nil {
		return printHFSections{}, printHFSections{}
	}
	hf := sheet.HeaderFooter
	header, footer := hf.OddHeader, hf.OddFooter
	if pageIndex == 0 && hf.DifferentFirst {
		header, footer = hf.FirstHeader, hf.FirstFooter
	} else if hf.DifferentOddEven && pageNumber%2 == 0 {
		header, footer = hf.EvenHeader, hf.EvenFooter
	}
	return parsePrintHFSections(expandPrintHFFields(header, sheet.Name, pageNumber, pageCount)),
		parsePrintHFSections(expandPrintHFFields(footer, sheet.Name, pageNumber, pageCount))
}

func parsePrintHFSections(value string) printHFSections {
	var out printHFSections
	current := &out.Center
	for i := 0; i < len(value); {
		if i+1 < len(value) && value[i] == '&' {
			switch value[i+1] {
			case 'L':
				current = &out.Left
				i += 2
				continue
			case 'C':
				current = &out.Center
				i += 2
				continue
			case 'R':
				current = &out.Right
				i += 2
				continue
			}
		}
		*current += string(value[i])
		i++
	}
	return out
}

func expandPrintHFFields(value, sheetName string, pageNumber, pageCount int) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '&' || i+1 >= len(value) {
			b.WriteByte(value[i])
			continue
		}
		i++
		switch value[i] {
		case '&':
			b.WriteByte('&')
		case 'P':
			b.WriteString(strconv.Itoa(pageNumber))
		case 'N':
			b.WriteString(strconv.Itoa(pageCount))
		case 'A':
			b.WriteString(sheetName)
		default:
			b.WriteByte('&')
			b.WriteByte(value[i])
		}
	}
	return b.String()
}

func renderPrintHeaderFooter(b *strings.Builder, kind string, s printHFSections, hf *resolve.HeaderFooter, imageIndex *int) {
	if s.Left == "" && s.Center == "" && s.Right == "" {
		return
	}
	fmt.Fprintf(b, "<div class=\"print-%s\"><span class=\"hf-left\">%s</span><span class=\"hf-center\">%s</span><span class=\"hf-right\">%s</span></div>\n",
		kind, renderHFSection(s.Left, hf, imageIndex), renderHFSection(s.Center, hf, imageIndex), renderHFSection(s.Right, hf, imageIndex))
}

func renderHFSection(value string, hf *resolve.HeaderFooter, imageIndex *int) string {
	escaped := html.EscapeString(value)
	for strings.Contains(escaped, "&amp;G") && hf != nil && imageIndex != nil && *imageIndex < len(hf.Images) {
		img := hf.Images[*imageIndex]
		*imageIndex = *imageIndex + 1
		mime := strings.TrimSpace(img.ContentType)
		if mime == "" {
			mime = "image/png"
		}
		tag := "<img class=\"hf-image\" alt=\"\" src=\"data:" + html.EscapeString(mime) + ";base64," + base64.StdEncoding.EncodeToString(img.Data) + "\">"
		escaped = strings.Replace(escaped, "&amp;G", tag, 1)
	}
	return escaped
}

func pt(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }

func sheetForPrintPage(src *resolve.Sheet, firstRow, lastRow, firstCol, lastCol, titleRowFrom, titleRowTo, titleColFrom, titleColTo int) *resolve.Sheet {
	cp := *src
	cp.PrintArea = ""
	cp.PrintTitleRows = ""
	cp.PrintTitleCols = ""
	cp.Rows = nil
	rowSelected := func(row int) bool {
		return row >= firstRow && row <= lastRow || titleRowFrom > 0 && row >= titleRowFrom && row <= titleRowTo
	}
	colSelected := func(col int) bool {
		return col >= firstCol && col <= lastCol || titleColFrom > 0 && col >= titleColFrom && col <= titleColTo
	}
	for _, row := range src.Rows {
		if row == nil || !rowSelected(row.Index) {
			continue
		}
		rc := *row
		rc.Cells = nil
		for _, cell := range row.Cells {
			if cell != nil && colSelected(cell.Col) {
				rc.Cells = append(rc.Cells, cell)
			}
		}
		cp.Rows = append(cp.Rows, &rc)
	}
	cp.HiddenCols = make(map[int]bool, len(src.HiddenCols)+lastCol)
	for col, hidden := range src.HiddenCols {
		cp.HiddenCols[col] = hidden
	}
	maxCol := lastCol
	if titleColTo > maxCol {
		maxCol = titleColTo
	}
	for col := 1; col <= maxCol; col++ {
		if !colSelected(col) {
			cp.HiddenCols[col] = true
		}
	}
	cp.MergedCells = nil
	for _, m := range src.MergedCells {
		if m == nil {
			continue
		}
		if rowSelected(m.TopRow) && rowSelected(m.BottomRow) && colSelected(m.LeftCol) && colSelected(m.RightCol) {
			cp.MergedCells = append(cp.MergedCells, m)
		}
	}
	cp.Drawings = nil
	for _, d := range src.Drawings {
		if d == nil {
			continue
		}
		a := d.Position
		if rowSelected(a.FromCell.Row) && colSelected(a.FromCell.Col) {
			cp.Drawings = append(cp.Drawings, d)
		}
	}
	cp.FormControls = nil
	for _, c := range src.FormControls {
		if c == nil {
			continue
		}
		parts := strings.Split(c.Anchor, ",")
		if len(parts) < 3 {
			continue
		}
		col, errCol := strconv.Atoi(strings.TrimSpace(parts[0]))
		row, errRow := strconv.Atoi(strings.TrimSpace(parts[2]))
		if errCol == nil && errRow == nil && rowSelected(row+1) && colSelected(col+1) {
			cp.FormControls = append(cp.FormControls, c)
		}
	}
	return &cp
}

const printPageCSS = `
.print-document { margin: 0; background: white; }
.print-page { break-after: page; page-break-after: always; overflow: hidden; box-sizing: border-box; position: relative; }
.print-page:last-child { break-after: auto; page-break-after: auto; }
.print-page > .sheet { margin: 0; }
.print-header, .print-footer { display: grid; grid-template-columns: 1fr 1fr 1fr; box-sizing: border-box; font-size: 10pt; line-height: 1.2; position: absolute; left: var(--page-left); right: var(--page-right); }
.print-header { top: var(--hf-header); }
.print-footer { bottom: var(--hf-footer); }
.hf-image { max-height: 36pt; max-width: 100%; vertical-align: middle; object-fit: contain; }
.print-header .hf-center, .print-footer .hf-center { text-align: center; }
.print-header .hf-right, .print-footer .hf-right { text-align: right; }
.print-page .sheet-name, .print-page .outline-controls, .print-page .pane-meta { display: none !important; }
@media print {
  .print-page { box-shadow: none !important; }
  .grid-wrap { overflow: visible !important; }
}
`
