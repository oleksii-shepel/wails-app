package legacy

import (
	"reflect"

	wordir "office/pkg/word/ir"
)

type legacyTableRowProps struct {
	// cellBoundaries preserves the row-local rgdxaCenter values from sprmTDefTable.
	// Unlike cell widths, these absolute boundaries let us reconstruct the union
	// table grid across rows whose merged cells expose different physical cuts.
	cellBoundaries []int
	hMerge         map[int]byte
	vMerge         map[int]byte
	cellFormats    map[int]wordir.TableCellFormat

	// Row/table-level properties from TAPx sprms.
	tableJc       string // "left"|"center"|"right"; empty = unset (sprmTJc90 / sprmTJc)
	tableDxaLeft  *int   // signed twips; nil = unset (sprmTDxaLeft)
	tableDxaGap   *int   // half-gap between cells in twips; nil = unset (sprmTDxaGapHalf)
	rowHeight     *int   // signed twips; nil = unset (sprmTDyaRowHeight)
	cantSplit     *bool  // sprmTFCantSplit
	tableHeader   *bool  // sprmTFTableHeader (repeat row as header)
	tableBorders  *wordir.TableBorders
	cellPadTop    *int
	cellPadBottom *int
	cellPadLeft   *int
	cellPadRight  *int
	// Table width from sprmTTableWidth (FtsWWidth_Table: 1 byte fts + 2 bytes width).
	tableWidth     *int
	tableWidthType string // "nil" | "auto" | "pct" | "dxa"
	// AutoFit from sprmTFAutofit (Bool8).
	autoFit *bool
	// BidiVisual from sprmTFBiDi / sprmTFBiDi90 (Bool16).
	bidiVisual *bool

	// Legacy floating-table positioning. MS-DOC stores these in TAPx on each
	// row (sprmTPc / sprmTDxaAbs / sprmTDyaAbs / sprmTDx[a|y]FromText*).
	// They are table-level semantics even though physically repeated per row.
	floatProps *wordir.TableFloatProps
	noOverlap  *bool

	// sprmTPropRMark — row property revision metadata from TAPx.
	propertyRevision *legacyPendingRevisionMark
	// resolvedPropertyRevision is the sttbfRMark-resolved form populated in
	// extractDocRunsViaFIB after the author table has been loaded.
	resolvedPropertyRevision *wordir.RevisionMark
}

func (p *legacyTableRowProps) clone() *legacyTableRowProps {
	if p == nil {
		return nil
	}
	out := &legacyTableRowProps{tableJc: p.tableJc}
	if len(p.cellBoundaries) > 0 {
		out.cellBoundaries = append([]int(nil), p.cellBoundaries...)
	}
	if len(p.hMerge) > 0 {
		out.hMerge = make(map[int]byte, len(p.hMerge))
		for index, state := range p.hMerge {
			out.hMerge[index] = state
		}
	}
	if len(p.vMerge) > 0 {
		out.vMerge = make(map[int]byte, len(p.vMerge))
		for index, state := range p.vMerge {
			out.vMerge[index] = state
		}
	}
	if len(p.cellFormats) > 0 {
		out.cellFormats = make(map[int]wordir.TableCellFormat, len(p.cellFormats))
		for index, format := range p.cellFormats {
			out.cellFormats[index] = cloneLegacyTableCellFormat(format)
		}
	}
	if p.tableDxaLeft != nil {
		v := *p.tableDxaLeft
		out.tableDxaLeft = &v
	}
	if p.tableDxaGap != nil {
		v := *p.tableDxaGap
		out.tableDxaGap = &v
	}
	if p.rowHeight != nil {
		v := *p.rowHeight
		out.rowHeight = &v
	}
	if p.cantSplit != nil {
		v := *p.cantSplit
		out.cantSplit = &v
	}
	if p.tableHeader != nil {
		v := *p.tableHeader
		out.tableHeader = &v
	}
	if p.tableBorders != nil {
		out.tableBorders = cloneLegacyTableBorders(p.tableBorders)
	}
	if p.propertyRevision != nil {
		v := *p.propertyRevision
		out.propertyRevision = &v
	}
	if p.resolvedPropertyRevision != nil {
		v := *p.resolvedPropertyRevision
		out.resolvedPropertyRevision = &v
	}
	if p.cellPadTop != nil {
		v := *p.cellPadTop
		out.cellPadTop = &v
	}
	if p.cellPadBottom != nil {
		v := *p.cellPadBottom
		out.cellPadBottom = &v
	}
	if p.cellPadLeft != nil {
		v := *p.cellPadLeft
		out.cellPadLeft = &v
	}
	if p.cellPadRight != nil {
		v := *p.cellPadRight
		out.cellPadRight = &v
	}
	if p.tableWidth != nil {
		v := *p.tableWidth
		out.tableWidth = &v
	}
	out.tableWidthType = p.tableWidthType
	if p.autoFit != nil {
		v := *p.autoFit
		out.autoFit = &v
	}
	if p.bidiVisual != nil {
		v := *p.bidiVisual
		out.bidiVisual = &v
	}
	if p.floatProps != nil {
		v := *p.floatProps
		out.floatProps = &v
	}
	if p.noOverlap != nil {
		v := *p.noOverlap
		out.noOverlap = &v
	}
	return out
}

func (p *legacyTableRowProps) hasData() bool {
	if p == nil {
		return false
	}
	return len(p.cellBoundaries) >= 2 || len(p.hMerge) > 0 || len(p.vMerge) > 0 || len(p.cellFormats) > 0 ||
		p.tableJc != "" || p.tableDxaLeft != nil || p.tableDxaGap != nil || p.rowHeight != nil || p.cantSplit != nil || p.tableHeader != nil || p.tableBorders != nil ||
		p.propertyRevision != nil ||
		p.cellPadTop != nil || p.cellPadBottom != nil || p.cellPadLeft != nil || p.cellPadRight != nil ||
		p.tableWidth != nil || p.tableWidthType != "" || p.autoFit != nil || p.bidiVisual != nil ||
		p.floatProps != nil || p.noOverlap != nil
}

func (p *legacyTableRowProps) maxCellIndex() int {
	if p == nil {
		return 0
	}
	max := 0
	if len(p.cellBoundaries) >= 2 {
		max = len(p.cellBoundaries) - 1
	}
	for index := range p.hMerge {
		if index+1 > max {
			max = index + 1
		}
	}
	for index := range p.vMerge {
		if index+1 > max {
			max = index + 1
		}
	}
	for index := range p.cellFormats {
		if index+1 > max {
			max = index + 1
		}
	}
	return max
}

func (p *legacyTableRowProps) physicalCellCount() int {
	if p == nil || len(p.cellBoundaries) < 2 {
		return 0
	}
	return len(p.cellBoundaries) - 1
}

func (p *legacyTableRowProps) validCellBoundaries() []int {
	if p == nil || len(p.cellBoundaries) < 2 {
		return nil
	}
	for i := 1; i < len(p.cellBoundaries); i++ {
		if p.cellBoundaries[i] <= p.cellBoundaries[i-1] {
			return nil
		}
	}
	return p.cellBoundaries
}

func (p *legacyTableRowProps) hMergeState(index int) byte {
	if p == nil || p.hMerge == nil {
		return 0
	}
	return p.hMerge[index]
}

func (p *legacyTableRowProps) vMergeState(index int) byte {
	if p == nil || p.vMerge == nil {
		return 0
	}
	return p.vMerge[index]
}

func (p *legacyTableRowProps) setHMerge(first, lim int) {
	if lim <= first || first < 0 {
		return
	}
	if p.hMerge == nil {
		p.hMerge = make(map[int]byte, lim-first)
	}
	for index := first; index < lim; index++ {
		if index == first {
			p.hMerge[index] = 2
			continue
		}
		p.hMerge[index] = 1
	}
}

func (p *legacyTableRowProps) clearHMerge(first, lim int) {
	if p == nil || p.hMerge == nil || lim <= first {
		return
	}
	for index := first; index < lim; index++ {
		delete(p.hMerge, index)
	}
}

func (p *legacyTableRowProps) setVMerge(index int, state byte) {
	if index < 0 {
		return
	}
	if state == 0 {
		if p.vMerge != nil {
			delete(p.vMerge, index)
		}
		return
	}
	if p.vMerge == nil {
		p.vMerge = make(map[int]byte, 1)
	}
	p.vMerge[index] = state
}

func (p *legacyTableRowProps) cellFormat(index int) wordir.TableCellFormat {
	if p == nil || p.cellFormats == nil {
		return wordir.TableCellFormat{}
	}
	return cloneLegacyTableCellFormat(p.cellFormats[index])
}

func (p *legacyTableRowProps) setCellFormat(index int, format wordir.TableCellFormat) {
	if index < 0 || !legacyTableCellFormatMeaningful(format) {
		return
	}
	if p.cellFormats == nil {
		p.cellFormats = make(map[int]wordir.TableCellFormat, 1)
	}
	p.cellFormats[index] = mergeLegacyTableCellFormat(p.cellFormats[index], format)
}

func (p *legacyTableRowProps) applyCellFormatRange(first, lim int, format wordir.TableCellFormat, oddOnly bool) {
	if lim <= first || !legacyTableCellFormatMeaningful(format) {
		return
	}
	step := 1
	if oddOnly {
		step = 2
	}
	for index := first; index < lim; index += step {
		p.setCellFormat(index, format)
	}
}

func mergeLegacyTableRowProps(base, overlay *legacyTableRowProps) *legacyTableRowProps {
	if base == nil {
		if overlay == nil {
			return nil
		}
		return overlay.clone()
	}
	if overlay == nil {
		return base.clone()
	}
	out := base.clone()
	if out == nil {
		out = &legacyTableRowProps{}
	}
	for index, state := range overlay.hMerge {
		if out.hMerge == nil {
			out.hMerge = make(map[int]byte, len(overlay.hMerge))
		}
		out.hMerge[index] = state
	}
	for index, state := range overlay.vMerge {
		if out.vMerge == nil {
			out.vMerge = make(map[int]byte, len(overlay.vMerge))
		}
		out.vMerge[index] = state
	}
	for index, format := range overlay.cellFormats {
		if out.cellFormats == nil {
			out.cellFormats = make(map[int]wordir.TableCellFormat, len(overlay.cellFormats))
		}
		out.cellFormats[index] = mergeLegacyTableCellFormat(out.cellFormats[index], format)
	}
	if out.tableJc == "" && overlay.tableJc != "" {
		out.tableJc = overlay.tableJc
	}
	if out.tableDxaLeft == nil && overlay.tableDxaLeft != nil {
		v := *overlay.tableDxaLeft
		out.tableDxaLeft = &v
	}
	if out.tableDxaGap == nil && overlay.tableDxaGap != nil {
		v := *overlay.tableDxaGap
		out.tableDxaGap = &v
	}
	if out.rowHeight == nil && overlay.rowHeight != nil {
		v := *overlay.rowHeight
		out.rowHeight = &v
	}
	if out.cantSplit == nil && overlay.cantSplit != nil {
		v := *overlay.cantSplit
		out.cantSplit = &v
	}
	if out.tableHeader == nil && overlay.tableHeader != nil {
		v := *overlay.tableHeader
		out.tableHeader = &v
	}
	if out.tableBorders == nil && overlay.tableBorders != nil {
		out.tableBorders = cloneLegacyTableBorders(overlay.tableBorders)
	}
	if out.cellPadTop == nil && overlay.cellPadTop != nil {
		v := *overlay.cellPadTop
		out.cellPadTop = &v
	}
	if out.cellPadBottom == nil && overlay.cellPadBottom != nil {
		v := *overlay.cellPadBottom
		out.cellPadBottom = &v
	}
	if out.cellPadLeft == nil && overlay.cellPadLeft != nil {
		v := *overlay.cellPadLeft
		out.cellPadLeft = &v
	}
	if out.cellPadRight == nil && overlay.cellPadRight != nil {
		v := *overlay.cellPadRight
		out.cellPadRight = &v
	}
	if out.floatProps == nil && overlay.floatProps != nil {
		v := *overlay.floatProps
		out.floatProps = &v
	}
	if out.noOverlap == nil && overlay.noOverlap != nil {
		v := *overlay.noOverlap
		out.noOverlap = &v
	}
	return out
}

func legacyTableRowPropsConflict(current, next *legacyTableRowProps) bool {
	if current == nil || next == nil {
		return false
	}
	if current.tableJc != "" && next.tableJc != "" && current.tableJc != next.tableJc {
		return true
	}
	if current.tableDxaLeft != nil && next.tableDxaLeft != nil && *current.tableDxaLeft != *next.tableDxaLeft {
		return true
	}
	if current.tableDxaGap != nil && next.tableDxaGap != nil && *current.tableDxaGap != *next.tableDxaGap {
		return true
	}
	if current.rowHeight != nil && next.rowHeight != nil && *current.rowHeight != *next.rowHeight {
		return true
	}
	if (current.rowHeight == nil) != (next.rowHeight == nil) {
		return true
	}
	if current.cantSplit != nil && next.cantSplit != nil && *current.cantSplit != *next.cantSplit {
		return true
	}
	if (current.cantSplit == nil) != (next.cantSplit == nil) {
		return true
	}
	if current.tableHeader != nil && next.tableHeader != nil && *current.tableHeader != *next.tableHeader {
		return true
	}
	if (current.tableHeader == nil) != (next.tableHeader == nil) {
		return true
	}
	if current.tableBorders != nil && next.tableBorders != nil && !reflect.DeepEqual(current.tableBorders, next.tableBorders) {
		return true
	}
	if current.cellPadTop != nil && next.cellPadTop != nil && *current.cellPadTop != *next.cellPadTop {
		return true
	}
	if current.cellPadBottom != nil && next.cellPadBottom != nil && *current.cellPadBottom != *next.cellPadBottom {
		return true
	}
	if current.cellPadLeft != nil && next.cellPadLeft != nil && *current.cellPadLeft != *next.cellPadLeft {
		return true
	}
	if current.cellPadRight != nil && next.cellPadRight != nil && *current.cellPadRight != *next.cellPadRight {
		return true
	}
	if current.floatProps != nil && next.floatProps != nil && !reflect.DeepEqual(current.floatProps, next.floatProps) {
		return true
	}
	if current.noOverlap != nil && next.noOverlap != nil && *current.noOverlap != *next.noOverlap {
		return true
	}
	for index, state := range next.hMerge {
		if cur, ok := current.hMerge[index]; ok && cur != state {
			return true
		}
	}
	for index, state := range next.vMerge {
		if cur, ok := current.vMerge[index]; ok && cur != state {
			return true
		}
	}
	for index, format := range next.cellFormats {
		if cur, ok := current.cellFormats[index]; ok && !reflect.DeepEqual(cur, format) {
			return true
		}
	}
	return false
}

func cloneLegacyTableCellFormat(format wordir.TableCellFormat) wordir.TableCellFormat {
	out := format
	if format.Width != nil {
		width := *format.Width
		out.Width = &width
	}
	if format.Borders != nil {
		out.Borders = cloneLegacyCellBorders(format.Borders)
	}
	return out
}

func cloneLegacyCellBorders(borders *wordir.CellBorders) *wordir.CellBorders {
	if borders == nil {
		return nil
	}
	return &wordir.CellBorders{
		Top:    cloneLegacyBorderSide(borders.Top),
		Left:   cloneLegacyBorderSide(borders.Left),
		Bottom: cloneLegacyBorderSide(borders.Bottom),
		Right:  cloneLegacyBorderSide(borders.Right),
	}
}

func cloneLegacyTableBorders(borders *wordir.TableBorders) *wordir.TableBorders {
	if borders == nil {
		return nil
	}
	return &wordir.TableBorders{
		Top:     cloneLegacyBorderSide(borders.Top),
		Left:    cloneLegacyBorderSide(borders.Left),
		Bottom:  cloneLegacyBorderSide(borders.Bottom),
		Right:   cloneLegacyBorderSide(borders.Right),
		InsideH: cloneLegacyBorderSide(borders.InsideH),
		InsideV: cloneLegacyBorderSide(borders.InsideV),
	}
}

func cloneLegacyBorderSide(side *wordir.BorderSide) *wordir.BorderSide {
	if side == nil {
		return nil
	}
	out := *side
	return &out
}

func mergeLegacyTableCellFormat(base, overlay wordir.TableCellFormat) wordir.TableCellFormat {
	out := cloneLegacyTableCellFormat(base)
	if overlay.Width != nil {
		width := *overlay.Width
		out.Width = &width
	}
	if overlay.WidthType != "" {
		out.WidthType = overlay.WidthType
	}
	if overlay.GridSpan > 1 {
		out.GridSpan = overlay.GridSpan
	}
	if overlay.HMerge {
		out.HMerge = true
	}
	if overlay.VMerge != "" {
		out.VMerge = overlay.VMerge
	}
	if overlay.VertAlign != "" {
		out.VertAlign = overlay.VertAlign
	}
	if overlay.TextDirection != "" {
		out.TextDirection = overlay.TextDirection
	}
	if overlay.Shading != "" {
		out.Shading = overlay.Shading
	}
	if overlay.Borders != nil {
		out.Borders = mergeLegacyCellBorders(out.Borders, overlay.Borders)
	}
	return out
}

func mergeLegacyCellBorders(base, overlay *wordir.CellBorders) *wordir.CellBorders {
	if overlay == nil {
		return cloneLegacyCellBorders(base)
	}
	out := cloneLegacyCellBorders(base)
	if out == nil {
		out = &wordir.CellBorders{}
	}
	if overlay.Top != nil {
		out.Top = cloneLegacyBorderSide(overlay.Top)
	}
	if overlay.Left != nil {
		out.Left = cloneLegacyBorderSide(overlay.Left)
	}
	if overlay.Bottom != nil {
		out.Bottom = cloneLegacyBorderSide(overlay.Bottom)
	}
	if overlay.Right != nil {
		out.Right = cloneLegacyBorderSide(overlay.Right)
	}
	return out
}

func legacyTableCellFormatMeaningful(format wordir.TableCellFormat) bool {
	return format.Width != nil ||
		format.WidthType != "" ||
		format.GridSpan > 1 ||
		format.HMerge ||
		format.VMerge != "" ||
		format.VertAlign != "" ||
		format.TextDirection != "" ||
		format.Shading != "" ||
		format.Borders != nil
}
