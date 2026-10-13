// Package ooxml owns XLSX-family parsing into the canonical Sheet IR.
//
// Callers outside parsers/ooxml should use this package rather than composing
// OPC loading, typed parts parsing, and IR resolution themselves. Keeping that
// composition behind one boundary lets the streaming parser progressively take
// ownership without leaving multiple authoritative XLSX interpretation paths.
package ooxml

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"office/pkg/ooxml/opc"
	sheetir "office/pkg/sheet/ir"
	"office/pkg/sheet/parsers/ooxml/parts"
	"office/pkg/sheet/parsers/ooxml/resolve"
	"office/pkg/sheet/parsers/ooxml/sax"
)

// ParsePath parses an XLSX-family file into canonical Sheet IR.
func ParsePath(path string) (*sheetir.Workbook, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open workbook: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat workbook: %w", err)
	}
	return ParseReaderAt(file, info.Size())
}

// ParseBytes parses an XLSX-family payload into the canonical Sheet IR.
func ParseBytes(data []byte) (*sheetir.Workbook, error) {
	return ParseReaderAt(bytes.NewReader(data), int64(len(data)))
}

// ParseReaderAt parses an XLSX-family package from a seekable reader into the
// canonical Sheet IR.
//
// Package-level metadata, worksheet streaming, and relationship-heavy objects
// have separate ownership boundaries. High-level callers must not compose those
// layers directly.
func ParseReaderAt(r io.ReaderAt, size int64) (*sheetir.Workbook, error) {
	if r == nil {
		return nil, fmt.Errorf("sheet ooxml: reader cannot be nil")
	}
	if size < 0 {
		return nil, fmt.Errorf("sheet ooxml: invalid package size %d", size)
	}

	pkg, err := opc.OpenReaderAt(r, size)
	if err != nil {
		return nil, fmt.Errorf("open package: %w", err)
	}
	parsed, err := parts.LoadMetadataPackage(pkg)
	if err != nil {
		return nil, fmt.Errorf("load package metadata: %w", err)
	}
	if err := parts.LoadRelationshipObjects(pkg, parsed); err != nil {
		return nil, fmt.Errorf("load worksheet relationship objects: %w", err)
	}
	wb, err := resolve.ResolveMetadataPackage(parsed)
	if err != nil {
		return nil, fmt.Errorf("resolve package metadata: %w", err)
	}
	styles := resolve.ResolveCellStyleTable(parsed.Styles, parsed.Theme)
	if err := applyStreamingWorksheetCore(r, size, wb, styles, parsed.Styles, parsed.Theme); err != nil {
		return nil, fmt.Errorf("stream worksheet core: %w", err)
	}
	return wb, nil
}

// applyStreamingWorksheetCore transfers ownership of worksheet identity, row
// geometry, cell values/formulas, and hyperlinks to the SAX path. The
// package metadata resolver supplies workbook/style context; worksheet core and
// formula groups are owned by the streaming path.
func applyStreamingWorksheetCore(r io.ReaderAt, size int64, wb *sheetir.Workbook, styles []sheetir.CellStyle, styleSheet *parts.StyleSheet, theme *parts.Theme) error {
	if wb == nil {
		return fmt.Errorf("nil workbook")
	}

	sheetByIndex := make(map[int]*sheetir.Sheet, len(wb.Sheets))
	rowBySheet := make(map[int]map[int]*sheetir.Row, len(wb.Sheets))

	for sheetIndex, sheet := range wb.Sheets {
		if sheet == nil {
			continue
		}
		// Ordinary worksheet structures are SAX-owned. The metadata package
		// deliberately does not parse sheetData/cols/merges/views.
		sheet.Rows = nil
		sheet.ColumnWidths = make(map[int]float64)
		sheet.HiddenCols = nil
		sheet.PaneSelections = nil
		sheet.MergedCells = nil
		sheetByIndex[sheetIndex] = sheet
		rowBySheet[sheetIndex] = make(map[int]*sheetir.Row)
	}

	sharedBySheet := make(map[int]map[int]*sheetir.SharedFormula)
	sharedCellBySheet := make(map[int]map[string]int)
	arrayBySheet := make(map[int]map[string]*sheetir.ArrayFormula)
	printBySheet := make(map[int]*streamingPrintMetadata)
	validationBySheet := make(map[int][]parts.DataValidation)
	conditionalBySheet := make(map[int][]parts.ConditionalFmt)
	autoFilterBySheet := make(map[int]*parts.AutoFilter)
	for i := range wb.Sheets {
		sharedBySheet[i] = make(map[int]*sheetir.SharedFormula)
		sharedCellBySheet[i] = make(map[string]int)
		arrayBySheet[i] = make(map[string]*sheetir.ArrayFormula)
	}

	err := sax.WalkReaderAt(r, size, sax.HandlerFunc(func(event sax.Event) error {
		sheet := sheetByIndex[event.SheetIndex]
		if sheet == nil {
			return fmt.Errorf("streamed sheet %d has no metadata projection", event.SheetIndex)
		}

		switch event.Type {
		case sax.EventStartSheet:
			sheet.Name = event.SheetName
			sheet.ID = event.SheetID
			sheet.Hidden = event.Hidden

		case sax.EventSheetFormat:
			sheet.DefaultColWidthChars = event.DefaultColWidth
			sheet.DefaultRowHeightPt = event.DefaultRowHeight
			sheet.DefaultRowHeightUnsynced = event.DefaultRowCustom
			sheet.DefaultRowHeightZeroHeight = event.DefaultRowZero
			sheet.RowOutlineLevelMax = event.RowOutlineLevelMax
			sheet.ColOutlineLevelMax = event.ColOutlineLevelMax

		case sax.EventColumn:
			if sheet.ColumnWidths == nil {
				sheet.ColumnWidths = make(map[int]float64)
			}
			if event.ColumnWidth > 0 {
				for col := event.ColumnMin; col <= event.ColumnMax; col++ {
					sheet.ColumnWidths[col] = event.ColumnWidth
				}
			}
			if event.ColumnHidden {
				if sheet.HiddenCols == nil {
					sheet.HiddenCols = make(map[int]bool)
				}
				for col := event.ColumnMin; col <= event.ColumnMax; col++ {
					sheet.HiddenCols[col] = true
				}
			}

		case sax.EventSheetView:
			if event.ZoomScale != 0 || event.ShowGridLines != nil {
				sheet.ZoomScale = event.ZoomScale
				sheet.ShowGridLines = event.ShowGridLines
			}
			if event.PaneState != "" || event.SplitX != 0 || event.SplitY != 0 || event.TopLeftCell != "" || event.ActivePane != "" {
				sheet.PaneState, sheet.SplitX, sheet.SplitY = event.PaneState, event.SplitX, event.SplitY
				sheet.TopLeftCell, sheet.ActivePane = event.TopLeftCell, event.ActivePane
				sheet.FrozenRows, sheet.FrozenCols, sheet.SplitRows, sheet.SplitCols = 0, 0, 0, 0
				switch event.PaneState {
				case "frozen", "frozenSplit":
					sheet.FrozenRows, sheet.FrozenCols = int(event.SplitY), int(event.SplitX)
				case "split":
					sheet.SplitRows, sheet.SplitCols = int(event.SplitY), int(event.SplitX)
				}
			}

		case sax.EventPaneSelection:
			sheet.PaneSelections = append(sheet.PaneSelections, sheetir.PaneSelection{
				Pane: event.SelectionPane, ActiveCell: event.SelectionActiveCell,
				ActiveCellID: event.SelectionActiveCellID, SqRef: event.SelectionSqRef,
			})

		case sax.EventMergedCell:
			if merged, ok := parseStreamingMergeRef(event.MergeRef); ok {
				sheet.MergedCells = append(sheet.MergedCells, merged)
			}

		case sax.EventStartRow:
			row := &sheetir.Row{
				Index:        event.RowIndex,
				HeightPt:     event.RowHeight,
				Hidden:       event.RowHidden,
				OutlineLevel: event.RowOutlineLevel,
				Collapsed:    event.RowCollapsed,
			}
			if event.RowStyleIndex > 0 && event.RowStyleIndex < len(styles) {
				style := styles[event.RowStyleIndex]
				row.Style = &style
			}
			rowBySheet[event.SheetIndex][event.RowIndex] = row
			sheet.Rows = append(sheet.Rows, row)

		case sax.EventPrintOptions:
			meta := ensureStreamingPrintMetadata(printBySheet, event.SheetIndex)
			meta.printOptions = event.PrintOptions
		case sax.EventPageMargins:
			meta := ensureStreamingPrintMetadata(printBySheet, event.SheetIndex)
			meta.pageMargins = event.PageMargins
		case sax.EventPageSetup:
			meta := ensureStreamingPrintMetadata(printBySheet, event.SheetIndex)
			meta.pageSetup = event.PageSetup
		case sax.EventRowBreaks:
			meta := ensureStreamingPrintMetadata(printBySheet, event.SheetIndex)
			meta.rowBreaks = event.PageBreaks
		case sax.EventColBreaks:
			meta := ensureStreamingPrintMetadata(printBySheet, event.SheetIndex)
			meta.colBreaks = event.PageBreaks
		case sax.EventSheetProtection:
			meta := ensureStreamingPrintMetadata(printBySheet, event.SheetIndex)
			meta.protection = event.SheetProtection
		case sax.EventDataValidation:
			if event.DataValidation != nil {
				validationBySheet[event.SheetIndex] = append(validationBySheet[event.SheetIndex], *event.DataValidation)
			}
		case sax.EventConditionalFmt:
			if event.ConditionalFmt != nil {
				conditionalBySheet[event.SheetIndex] = append(conditionalBySheet[event.SheetIndex], *event.ConditionalFmt)
			}
		case sax.EventAutoFilter:
			autoFilterBySheet[event.SheetIndex] = event.AutoFilter
		case sax.EventComment:
			applyStreamingComment(sheet, event, styleSheet, theme)
		case sax.EventCell:
			row := rowBySheet[event.SheetIndex][event.RowIndex]
			if row == nil {
				row = &sheetir.Row{Index: event.RowIndex}
				rowBySheet[event.SheetIndex][event.RowIndex] = row
				sheet.Rows = append(sheet.Rows, row)
			}
			cell := &sheetir.Cell{}
			applyStreamingCellCore(cell, event, styles)
			applyStreamingFormulaMetadata(sheet, cell, event, sharedBySheet[event.SheetIndex], sharedCellBySheet[event.SheetIndex], arrayBySheet[event.SheetIndex])
			if event.CommentText != "" {
				applyStreamingComment(sheet, event, styleSheet, theme)
			}
			row.Cells = append(row.Cells, cell)
		}
		return nil
	}), sax.Options{IncludeHiddenSheets: true, IncludeComments: true})
	if err != nil {
		return err
	}
	for sheetIndex, sheet := range wb.Sheets {
		if sheet == nil {
			continue
		}
		sheet.SharedFormulas = sheet.SharedFormulas[:0]
		for _, group := range sharedBySheet[sheetIndex] {
			sheet.SharedFormulas = append(sheet.SharedFormulas, group)
		}
		sheet.ArrayFormulas = sheet.ArrayFormulas[:0]
		for _, group := range arrayBySheet[sheetIndex] {
			sheet.ArrayFormulas = append(sheet.ArrayFormulas, group)
		}
		applyStreamingFormulaGroups(sheet, sharedBySheet[sheetIndex], sharedCellBySheet[sheetIndex], arrayBySheet[sheetIndex])
		meta := printBySheet[sheetIndex]
		if meta == nil {
			meta = &streamingPrintMetadata{}
		}
		resolve.ApplyStreamingPrintMetadata(sheet, meta.printOptions, meta.pageMargins, meta.pageSetup, meta.rowBreaks, meta.colBreaks, meta.protection)
		resolve.ApplyStreamingWorksheetSemantics(sheet, autoFilterBySheet[sheetIndex], validationBySheet[sheetIndex], conditionalBySheet[sheetIndex], styleSheet, theme)
	}
	resolve.ResolveStreamingValidationLists(wb)
	return nil
}

func parseStreamingMergeRef(ref string) (*sheetir.MergedCell, bool) {
	parts := strings.SplitN(strings.ReplaceAll(strings.TrimSpace(ref), "$", ""), ":", 2)
	if len(parts) == 1 {
		parts = append(parts, parts[0])
	}
	r1, c1, ok1 := parseStreamingCellRef(parts[0])
	r2, c2, ok2 := parseStreamingCellRef(parts[1])
	if !ok1 || !ok2 {
		return nil, false
	}
	if r2 < r1 {
		r1, r2 = r2, r1
	}
	if c2 < c1 {
		c1, c2 = c2, c1
	}
	return &sheetir.MergedCell{TopRow: r1, LeftCol: c1, BottomRow: r2, RightCol: c2}, true
}
func parseStreamingCellRef(ref string) (int, int, bool) {
	ref = strings.ToUpper(strings.TrimSpace(ref))
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		i++
	}
	if i == 0 || i == len(ref) {
		return 0, 0, false
	}
	col := 0
	for _, ch := range ref[:i] {
		col = col*26 + int(ch-'A'+1)
	}
	row, err := strconv.Atoi(ref[i:])
	return row, col, err == nil && row > 0
}

type streamingPrintMetadata struct {
	printOptions *parts.PrintOptions
	pageMargins  *parts.PageMargins
	pageSetup    *parts.PageSetup
	rowBreaks    *parts.PageBreaks
	colBreaks    *parts.PageBreaks
	protection   *parts.SheetProtection
}

func ensureStreamingPrintMetadata(all map[int]*streamingPrintMetadata, sheetIndex int) *streamingPrintMetadata {
	meta := all[sheetIndex]
	if meta == nil {
		meta = &streamingPrintMetadata{}
		all[sheetIndex] = meta
	}
	return meta
}

func applyStreamingComment(sheet *sheetir.Sheet, event sax.Event, styles *parts.StyleSheet, theme *parts.Theme) {
	if sheet == nil {
		return
	}
	ref := strings.ToUpper(strings.TrimSpace(event.CellRef))
	text := event.CommentText
	if ref == "" || text == "" {
		return
	}
	if sheet.Comments == nil {
		sheet.Comments = make(map[string]string)
	}
	if sheet.Notes == nil {
		sheet.Notes = make(map[string]*sheetir.CommentNote)
	}
	sheet.Comments[ref] = text
	sheet.Notes[ref] = &sheetir.CommentNote{Ref: ref, Author: event.CommentAuthor, Text: text, RichText: resolve.ResolveStreamingCommentRichText(event.CommentRichText, styles, theme)}
}

func applyStreamingFormulaMetadata(sheet *sheetir.Sheet, cell *sheetir.Cell, event sax.Event, shared map[int]*sheetir.SharedFormula, sharedCells map[string]int, arrays map[string]*sheetir.ArrayFormula) {
	switch event.FormulaType {
	case "shared":
		if event.FormulaSharedIndex == nil {
			return
		}
		index := *event.FormulaSharedIndex
		sharedCells[cell.Ref] = index
		group := shared[index]
		if group == nil {
			group = &sheetir.SharedFormula{Index: index}
			shared[index] = group
		}
		if event.Formula != "" {
			group.AnchorRef = event.CellRef
			group.RangeRef = event.FormulaRef
			group.Formula = event.Formula
			if merged, ok := parseStreamingMergeRef(event.FormulaRef); ok {
				group.TopRow, group.LeftCol = merged.TopRow, merged.LeftCol
				group.BottomRow, group.RightCol = merged.BottomRow, merged.RightCol
			}
		}
	case "array":
		ref := event.FormulaRef
		if ref == "" {
			ref = event.CellRef
		}
		group := arrays[ref]
		if group == nil {
			group = &sheetir.ArrayFormula{AnchorRef: event.CellRef, RangeRef: ref, Formula: event.Formula}
			if merged, ok := parseStreamingMergeRef(ref); ok {
				group.TopRow, group.LeftCol = merged.TopRow, merged.LeftCol
				group.BottomRow, group.RightCol = merged.BottomRow, merged.RightCol
			}
			arrays[ref] = group
		}
		cell.ArrayFormulaAnchor = group.AnchorRef
		cell.ArrayFormulaRange = group.RangeRef
	}
}

// applyStreamingFormulaGroups expands OOXML formula-group metadata after SAX
// has observed every cell. Shared-formula dependants do not carry formula text,
// and array members need the anchor's metadata even when they are not anchors.
func applyStreamingFormulaGroups(sheet *sheetir.Sheet, shared map[int]*sheetir.SharedFormula, sharedCells map[string]int, arrays map[string]*sheetir.ArrayFormula) {
	if sheet == nil {
		return
	}
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell == nil {
				continue
			}
			if index, ok := sharedCells[cell.Ref]; ok {
				if group := shared[index]; group != nil && group.Formula != "" {
					cell.Formula = sax.TranslateFormula(group.Formula, cell.Row-group.TopRow, cell.Col-group.LeftCol)
				}
			}
			for _, group := range arrays {
				if group != nil && cell.Row >= group.TopRow && cell.Row <= group.BottomRow && cell.Col >= group.LeftCol && cell.Col <= group.RightCol {
					cell.Formula = group.Formula
					cell.ArrayFormulaAnchor = group.AnchorRef
					cell.ArrayFormulaRange = group.RangeRef
					break
				}
			}
		}
	}
}

func applyStreamingCellCore(cell *sheetir.Cell, event sax.Event, styles []sheetir.CellStyle) {
	cell.Row = event.RowIndex
	cell.Col = event.ColIndex
	cell.Ref = event.CellRef
	cell.Formula = event.Formula
	cell.Style = sheetir.CellStyle{}
	if event.StyleIndex >= 0 && event.StyleIndex < len(styles) {
		cell.Style = styles[event.StyleIndex]
	}

	if event.HyperlinkTarget != "" {
		cell.HyperlinkURL = event.HyperlinkTarget
	} else if event.HyperlinkLocation != "" {
		cell.HyperlinkURL = "#" + event.HyperlinkLocation
	} else {
		cell.HyperlinkURL = ""
	}

	cell.StrValue = ""
	cell.NumValue = 0
	switch strings.TrimSpace(event.CellType) {
	case "s", "inlineStr":
		cell.Kind = sheetir.CellString
		cell.StrValue = event.Text
	case "b":
		cell.Kind = sheetir.CellBool
		if strings.TrimSpace(event.RawValue) == "1" {
			cell.NumValue = 1
		}
	case "e":
		cell.Kind = sheetir.CellError
		cell.StrValue = event.Text
	case "str":
		cell.Kind = sheetir.CellFormula
		cell.StrValue = event.Text
	default:
		raw := strings.TrimSpace(event.RawValue)
		if raw == "" {
			if event.Formula != "" {
				cell.Kind = sheetir.CellFormula
			} else {
				cell.Kind = sheetir.CellEmpty
			}
			return
		}
		if value, err := strconv.ParseFloat(raw, 64); err == nil {
			cell.Kind = sheetir.CellNumber
			cell.NumValue = value
		} else if event.Formula != "" {
			cell.Kind = sheetir.CellFormula
			cell.StrValue = event.Text
		}
	}
}
