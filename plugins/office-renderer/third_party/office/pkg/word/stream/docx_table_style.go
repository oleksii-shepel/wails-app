package stream

import (
	"encoding/xml"

	wordir "office/pkg/word/ir"
	"office/pkg/word/parsers/ooxml/parts"
	wordresolve "office/pkg/word/parsers/ooxml/resolve"
)

// Table styling is resolved after the table closes. The SAX walker forwards the
// raw w:tblPr / w:trPr / w:tcPr blocks because conditional table-style
// formatting (first row, banded rows, …) depends on a cell's row and column
// position, which is only known once the whole table has been read. Direct
// properties merge over the table style; conditional formats merge over the
// style's base formatting and under direct cell properties, matching Word.

// streamTableStyles resolves style inheritance for one streamed table.
type streamTableStyles struct {
	styleID     string
	themeColors map[string]string
	styles      map[string]wordir.Style
}

func newStreamTableStyles(doc *wordir.Document, metadata *wordresolve.RenderMetadata) streamTableStyles {
	resolved := streamTableStyles{}
	if doc != nil {
		resolved.styles = doc.Styles
		if resolved.styles == nil && metadata != nil {
			resolved.styles = metadata.Document.Styles
		}
	}
	if metadata != nil {
		resolved.themeColors = metadata.ThemeColorIndex()
	}
	return resolved
}

// apply resolves base table formatting, direct properties, and conditional
// formatting onto the table the stream has already assembled.
func (s streamTableStyles) apply(table *wordir.Table, rawTable, rawGrid string, rawRows []string, rawCells [][]string) {
	if table == nil {
		return
	}
	var columnWidths []int
	var properties parts.TableProperties
	if rawTable != "" {
		if err := xml.Unmarshal([]byte(rawTable), &properties); err != nil {
			properties = parts.TableProperties{}
		}
	}
	if len(rawGrid) > 0 {
		var grid streamTableGridXML
		if err := xml.Unmarshal([]byte(rawGrid), &grid); err == nil {
			for _, column := range grid.Columns {
				columnWidths = append(columnWidths, column.Width)
			}
		}
	}

	styleID := ""
	if properties.Style != nil {
		styleID = properties.Style.Val
	}
	if styleID == "" {
		styleID = s.styleID
	}
	var (
		base          wordir.TableFormat
		baseRunFormat wordir.RunFormat
		baseParaFmt   wordir.ParagraphFormat
		conditional   []wordir.ConditionalTableStyle
	)
	if style, ok := s.styles[styleID]; ok {
		if style.TableFormat != nil {
			base = *style.TableFormat
		}
		// Only what the table style itself declares. The style index layers the
		// document defaults under every style for effective lookup, but a table
		// style must not push the document's paragraph spacing into every cell:
		// Word's TableGrid, for example, declares zero space-after while the
		// document default is 10pt, and applying the default makes every row
		// taller than the source document.
		baseRunFormat = tableStyleDeclaredRunFormat(style, s.styles)
		baseParaFmt = tableStyleDeclaredParagraphFormat(style, s.styles)
		conditional = style.TableStyles
	}

	direct := wordresolve.TableFormatFromProperties(&properties, s.themeColors)
	table.Format = wordresolve.MergeTableFormat(base, direct)
	// Column widths come from w:tblGrid, which is not part of w:tblPr, so they
	// are applied after the property merge rather than being overwritten by it.
	table.Format.ColumnWidths = columnWidths

	totalRows := len(table.Rows)
	for rowIdx := range table.Rows {
		row := &table.Rows[rowIdx]
		if rowIdx < len(rawRows) && rawRows[rowIdx] != "" {
			var rowProperties parts.TableRowProperties
			if err := xml.Unmarshal([]byte(rawRows[rowIdx]), &rowProperties); err == nil {
				applyStreamRowProperties(&row.Format, &rowProperties)
			}
		}
		totalCols := len(row.Cells)
		for cellIdx := range row.Cells {
			cell := &row.Cells[cellIdx]
			runOverlay := wordresolve.MergeRunFormat(baseRunFormat, wordir.RunFormat{})
			paraOverlay := wordresolve.MergeParagraphFormat(baseParaFmt, wordir.ParagraphFormat{})
			types := wordresolve.TableConditionalTypes(rowIdx, totalRows, cellIdx, totalCols, properties.Look)
			conditionalCell, conditionalRun, conditionalPara := wordresolve.ConditionalTableOverlay(conditional, types)
			cell.Format = wordresolve.MergeTableCellFormat(cell.Format, conditionalCell)
			runOverlay = wordresolve.MergeRunFormat(runOverlay, conditionalRun)
			paraOverlay = wordresolve.MergeParagraphFormat(paraOverlay, conditionalPara)
			if rowIdx < len(rawCells) && cellIdx < len(rawCells[rowIdx]) && rawCells[rowIdx][cellIdx] != "" {
				var cellProperties parts.TableCellProperties
				if err := xml.Unmarshal([]byte(rawCells[rowIdx][cellIdx]), &cellProperties); err == nil {
					cell.Format = wordresolve.MergeTableCellFormat(cell.Format, wordresolve.TableCellFormatFromProperties(&cellProperties, s.themeColors))
				}
			}
			applyCellFormatOverlay(cell, runOverlay, paraOverlay)
		}
	}
}

// applyStreamRowProperties maps the w:trPr subset the renderer consumes.
func applyStreamRowProperties(format *wordir.TableRowFormat, properties *parts.TableRowProperties) {
	if format == nil || properties == nil {
		return
	}
	if properties.Height != nil {
		height := properties.Height.Val
		format.Height = &height
		format.HeightRule = properties.Height.Rule
	}
	if properties.IsHeader != nil {
		format.IsHeader = properties.IsHeader.Val == nil || bool(*properties.IsHeader.Val)
	}
	if properties.CantSplit != nil {
		format.CantSplit = properties.CantSplit.Val == nil || bool(*properties.CantSplit.Val)
	}
	if properties.Hidden != nil {
		format.Hidden = properties.Hidden.Val == nil || bool(*properties.Hidden.Val)
	}
	if properties.Justification != nil {
		format.Justification = properties.Justification.Val
	}
}

// applyCellFormatOverlay layers the table style's paragraph and run formatting
// onto the cell's content. Word applies a table style to the text inside the
// cells as well as to the cell rectangles, so header rows keep their style's
// font, weight, and color.
func applyCellFormatOverlay(cell *wordir.TableCell, runOverlay wordir.RunFormat, paraOverlay wordir.ParagraphFormat) {
	if cell == nil {
		return
	}
	applyParagraphOverlay := func(paragraph *wordir.Paragraph) {
		if paragraph == nil {
			return
		}
		paragraph.Format = wordresolve.MergeParagraphFormat(paragraph.Format, paraOverlay)
		for i := range paragraph.Runs {
			paragraph.Runs[i].Format = wordresolve.MergeRunFormat(paragraph.Runs[i].Format, runOverlay)
		}
	}
	for i := range cell.Paragraphs {
		applyParagraphOverlay(&cell.Paragraphs[i])
	}
	for i := range cell.Blocks {
		if paragraph := cell.Blocks[i].Paragraph; paragraph != nil {
			applyParagraphOverlay(paragraph)
		}
	}
}

// tableStyleDeclaredParagraphFormat returns the paragraph properties a table
// style actually declares, following its basedOn chain. Document defaults are
// deliberately excluded: cell paragraphs already inherit them, and layering
// them again would add spacing the table style does not ask for.
func tableStyleDeclaredParagraphFormat(style wordir.Style, index map[string]wordir.Style) wordir.ParagraphFormat {
	var declared wordir.ParagraphFormat
	for i := len(style.StylePath) - 1; i >= 0; i-- {
		current, ok := index[style.StylePath[i]]
		if !ok {
			continue
		}
		declared = wordresolve.MergeParagraphFormat(declared, current.RawFormat)
	}
	return declared
}

// tableStyleDeclaredRunFormat is the run-format counterpart of
// tableStyleDeclaredParagraphFormat.
func tableStyleDeclaredRunFormat(style wordir.Style, index map[string]wordir.Style) wordir.RunFormat {
	var declared wordir.RunFormat
	for i := len(style.StylePath) - 1; i >= 0; i-- {
		current, ok := index[style.StylePath[i]]
		if !ok {
			continue
		}
		declared = wordresolve.MergeRunFormat(declared, current.RawRunFormat)
	}
	return declared
}

// streamTableGridXML is the w:tblGrid subset the stream needs: column widths in
// twips drive the rendered column layout.
type streamTableGridXML struct {
	Columns []struct {
		Width int `xml:"w,attr"`
	} `xml:"gridCol"`
}
