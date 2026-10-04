package layout

import (
	"math"
	"sort"
	"strings"

	sharedfontmetrics "office/pkg/fontmetrics"
	wordfontmetrics "office/pkg/word/fontmetrics"
	"office/pkg/word/inlineposition"
	resolve "office/pkg/word/ir"
	wordtextmetrics "office/pkg/word/textmetrics"
)

const (
	emuPerPoint = 12700 // 914400 EMU per inch / 72 points per inch

	// wordSingleLineRatio is the canonical fallback advance for Word single spacing
	// when exact font metrics are unavailable. Keep the fallback at 1.20 so layout,
	// pagination, and paragraph-relative drawing anchors share the same physical line
	// advance used by the renderer. Reducing this to 1.15 accumulates vertical drift
	// across preceding lines and moves paragraph-relative floats upward.
	wordSingleLineRatio = 1.15
	// cssNormalLineRatio is retained as an internal compatibility alias for older
	// layout tests; its value now follows Word single-line geometry.
	cssNormalLineRatio = wordSingleLineRatio

	// twipsPerPoint is the number of twentieths-of-a-point (twips) in one point.
	twipsPerPoint = 20
)

// PageGeometry holds resolved page geometry in points.
type PageGeometry struct {
	Width        float64 // page width in points
	Height       float64 // page height in points
	MarginTop    float64 // top margin in points
	MarginBottom float64 // bottom margin in points
	MarginLeft   float64 // left margin in points (= column left edge for single-column)
	MarginRight  float64 // right margin in points
	// Column geometry (Fix 3: column-relative X positioning)
	ColumnCount     int       // number of text columns (≥ 1; 1 for single-column docs)
	ColumnWidthPt   float64   // default/equal column width in points
	ColumnGapPt     float64   // default/equal adjacent-column gap in points
	ColumnWidthsPt  []float64 // explicit per-column widths when the source supplies them
	ColumnGapsPt    []float64 // explicit per-gap spacing when the source supplies it
	ExplicitColumns bool      // source supplied a physical per-column grid, even when widths happen to be equal
	ColumnSep       bool      // draw a separator rule between text columns
	DocGridType     string    // section docGrid type: "lines", "linesAndChars", or ""
	DocGridPitchPt  float64   // snapped line pitch in points when docGrid is active
}

// BlockBox is a flow-positioned block (paragraph or table row) with an
// explicit Y coordinate and height computed from Word's formatting rules.
type BlockBox struct {
	Block         *resolve.Block
	Geometry      PageGeometry
	Page          int
	Column        int
	Y             float64 // top edge, measured from the content-area top (after MarginTop), in points
	Height        float64 // computed height in points
	Fragment      *ParagraphFragment
	TableFragment *TableFragment
}

// ParagraphFragment describes a line-range fragment of a paragraph that was
// split across pages or columns during pagination.
type ParagraphFragment struct {
	StartLine int
	EndLine   int
	Lines     []ParagraphLineLayout
	// SpatialRendering is true when horizontal slot geometry is semantically
	// significant and the renderer must not flatten Lines back into browser flow.
	SpatialRendering bool
	// FallbackRuns exists only for structural fragments with no physical-line
	// projection. Measured text is represented exclusively by Lines.
	FallbackRuns    []resolve.Run
	IsContinuation  bool
	IsLast          bool
	StartsNewColumn bool
	// OriginY is the flow Y of the fragment's first physical line in the same
	// page-absolute coordinate space used by BlockBox.Y + Geometry.MarginTop.
	// Line slots are rebased to their first line, so this is the only place
	// that preserves the measured spacing-before for renderers.
	OriginY float64
}

// TableFragment describes a row-range fragment of a table that was split at
// legal row boundaries during pagination. Header rows can be repeated on
// continuation fragments without duplicating them in the source IR.
type TableFragment struct {
	StartRow       int
	EndRow         int // exclusive
	RepeatHeaders  bool
	IsContinuation bool
	IsLast         bool
}

// FloatBox is a floating object resolved to absolute page coordinates.
// X and Y are measured from the page top-left corner (not from the margin).
type FloatBox struct {
	// Exactly one of Image, TextBox, Shape, FrameParagraph, FloatTable will be set.
	Image          *resolve.Image
	TextBox        *resolve.TextBox
	Shape          *resolve.Shape
	FrameParagraph *resolve.Paragraph // set for w:framePr frame paragraphs
	FloatTable     *resolve.Table     // set for w:tblpPr floating tables
	Page           int
	X              float64 // from page left, in points
	Y              float64 // from page top, in points
	Width          float64 // object width in points
	Height         float64 // object height in points
	ZIndex         int64
	BehindDoc      bool
}

// Page is a single paginated page with geometry-relative block and float boxes.
type Page struct {
	Number            int
	SectionPageNumber int // 1-based physical page ordinal within the owning section
	Geometry          PageGeometry
	Section           resolve.Section
	SectionIndex      int // index into doc.Sections for the section that owns this page
	Blocks            []BlockBox
	Footnotes         []BlockBox
	Floats            []FloatBox
}

// Layout is the legacy single-flow layout entry point.
//
// It keeps the original contract for existing renderers by laying out all
// content blocks against the first resolved section geometry and ignoring
// explicit section marker blocks. Use Paginate for section-aware page layout.
func Layout(doc *resolve.Document) (geom PageGeometry, blocks []BlockBox, floats []FloatBox) {
	geom = resolveGeometry(doc)
	blocks, floats = layoutBlocks(contentBlocks(doc.Blocks), doc, geom, 1)
	return
}

// Paginate computes page-aware layout for the resolved document.
//
// Content is split by explicit section marker blocks and then flowed into pages
// using the geometry of each resolved section. Paragraph page-break-before,
// page break runs, and page overflow all start new pages.
func Paginate(doc *resolve.Document) []Page {
	spans := splitSections(doc)
	if len(spans) == 0 {
		spans = []sectionSpan{{
			Section: resolve.Section{},
			Blocks:  contentBlocks(doc.Blocks),
		}}
	}

	pages := make([]Page, 0, len(spans))
	// Fix 5: track accumulated footnote height per page so the usable body
	// height is reduced when footnotes are present on a page.
	pageFootnoteH := make([]float64, 0, len(spans)) // parallel to pages
	pageFootnoteIDs := make([][]int, 0, len(spans))
	seenFootnotes := make(map[int]bool) // footnote IDs already counted on current page
	seenDocEndnotes := make(map[int]bool)
	docEndnoteIDs := make([]int, 0)
	nextSyntheticIndex := -1
	endnotePosition := "docEnd"
	if doc != nil && strings.EqualFold(doc.EndnotePosition, "sectEnd") {
		endnotePosition = "sectEnd"
	}

	// Declare before closures so the closures can capture them by reference.
	currentPage := -1
	currentGeom := PageGeometry{}
	currentColumn := 0
	currentY := 0.0
	currentColumnStartY := 0.0
	columnHasContent := false

	currentSectionIndex := 0
	currentSectionPageNumber := 0
	startPage := func(section resolve.Section) int {
		number := len(pages) + 1
		currentSectionPageNumber++
		geom := resolveGeometryForSectionPageInSection(doc, &section, number, currentSectionPageNumber)
		currentGeom = geom
		currentColumnStartY = 0
		pages = append(pages, Page{
			Number:            number,
			SectionPageNumber: currentSectionPageNumber,
			Geometry:          geom,
			Section:           section,
			SectionIndex:      currentSectionIndex,
		})
		pageFootnoteH = append(pageFootnoteH, 0)
		pageFootnoteIDs = append(pageFootnoteIDs, nil)
		seenFootnotes = make(map[int]bool) // reset per page
		return len(pages) - 1
	}

	advanceFlow := func(section resolve.Section) {
		geom := currentGeom
		if geom.ColumnCount > 1 && currentColumn < geom.ColumnCount-1 {
			currentColumn++
			currentY = currentColumnStartY
			columnHasContent = false
			return
		}
		currentPage = startPage(section)
		currentColumn = 0
		currentY = 0
		currentColumnStartY = 0
		columnHasContent = false
	}

	// effectiveContentHeight returns the page-specific body region after all
	// dynamic bottom reservations already known for this page. Header/footer
	// reservations are part of PageGeometry; footnotes are dynamic and shrink
	// the body as references are committed to the page.
	effectiveContentHeight := func(geom PageGeometry) float64 {
		footnoteReserve := 0.0
		if currentPage >= 0 && currentPage < len(pageFootnoteH) {
			footnoteReserve = pageFootnoteH[currentPage]
		}
		return reservedBodyHeight(geom, footnoteReserve, 0)
	}

	collectEndnotesForBlock := func(block *resolve.Block, seen map[int]bool, ids *[]int) {
		if block == nil || block.Paragraph == nil {
			return
		}
		for _, run := range block.Paragraph.Runs {
			if run.Kind != "endnoteRef" || seen[run.NoteID] {
				continue
			}
			seen[run.NoteID] = true
			*ids = append(*ids, run.NoteID)
		}
	}

	flowBlocks := func(section resolve.Section, blockList []resolve.Block, endnoteCollector func(*resolve.Block), pagePrelude func() []resolve.Block, pageEpilogue func() []resolve.Block) {
		estimateExtraBlocksHeight := func(extraBlocks []resolve.Block, geom PageGeometry) float64 {
			if len(extraBlocks) == 0 {
				return 0
			}
			total := 0.0
			for i := range extraBlocks {
				height, _ := measureBlock(doc, extraBlocks, i, geom, 0, total, nil)
				total += height
			}
			return total
		}
		shouldKeepWhole := func(block *resolve.Block) bool {
			if block == nil || block.Paragraph == nil {
				return false
			}
			format := block.Paragraph.Format
			return format.KeepLines
		}
		addFootnotesForRuns := func(runs []resolve.Run, geom PageGeometry) {
			if doc == nil || currentPage < 0 || currentPage >= len(pageFootnoteIDs) {
				return
			}
			footnoteGeom := footnoteLayoutGeometry(geom)
			footnoteW := footnoteGeom.ColumnWidthPt
			for _, run := range runs {
				if run.Kind != "footnoteRef" || seenFootnotes[run.NoteID] {
					continue
				}
				seenFootnotes[run.NoteID] = true
				if len(pageFootnoteIDs[currentPage]) == 0 {
					// Word prints the footnote separator above the first note on
					// the page. It is material the body must stay clear of, so
					// reserve it together with the note story that follows.
					pageFootnoteH[currentPage] += noteSeparatorHeightPt(doc, geom)
				}
				pageFootnoteIDs[currentPage] = append(pageFootnoteIDs[currentPage], run.NoteID)
				if story, ok := doc.Footnotes[run.NoteID]; ok {
					pageFootnoteH[currentPage] += estimateStoryHeightPt(doc, story, footnoteW)
				}
			}
		}
		emitBlockBox := func(block *resolve.Block, geom PageGeometry, height float64, blockFloats []FloatBox, fragment *ParagraphFragment) {
			pages[currentPage].Blocks = append(pages[currentPage].Blocks, BlockBox{
				Block:    block,
				Geometry: geom,
				Page:     pages[currentPage].Number,
				Column:   currentColumn,
				Y:        currentY,
				Height:   height,
				Fragment: fragment,
			})
			commitPageFloats(&pages[currentPage], blockFloats)
			currentY += height
			columnHasContent = true
		}
		emitExtraBlocks := func(extraBlocks []resolve.Block) {
			for i := range extraBlocks {
				block := &extraBlocks[i]
				height, blockFloats := measureBlock(doc, extraBlocks, i, currentGeom, currentColumn, currentY, wrapFloatsForFlowRegion(pages[currentPage]))
				emitBlockBox(block, currentGeom, height, blockFloats, nil)
			}
		}
		emitPrelude := func() {
			if pagePrelude == nil {
				return
			}
			emitExtraBlocks(pagePrelude())
		}
		emitEpilogue := func() {
			if pageEpilogue == nil {
				return
			}
			emitExtraBlocks(pageEpilogue())
		}
		trySplitParagraphAcrossFlow := func(index int, block *resolve.Block, geom PageGeometry, column int, y float64, activeFloats []FloatBox, availH float64) bool {
			if block == nil || block.Paragraph == nil || doc == nil {
				return false
			}
			p := block.Paragraph
			if p.Frame != nil || p.Format.KeepLines {
				return false
			}
			prevPara := adjacentBlock(blockList, index, -1)
			nextPara := adjacentBlock(blockList, index, +1)
			textMetrics := resolveParagraphSpacingMetrics(doc, p, prevPara, nextPara, geom, flowStartSuppressesSpacing(blockList, index, y))
			metrics, ok := paragraphFlowMetricsForPagination(doc, p, geom, column, y+geom.MarginTop, textMetrics, activeFloats)
			if !ok || len(metrics.lineHeights) == 0 || y+metrics.totalHeight() <= availH {
				return false
			}

			startLine := 0
			for startLine < len(metrics.lineHeights) {
				// Re-resolve physical geometry after a flow advance. Floats are
				// page/column-local; carrying the first page's slot exclusions into a
				// continuation fragment makes both pagination and rendering drift.
				if startLine > 0 {
					remainingRuns := paragraphPhysicalLinesRuns(metrics.fragmentPhysicalLines(startLine, len(metrics.lineHeights)))
					continuationParagraph := *p
					continuationParagraph.Runs = remainingRuns
					continuationParagraph.Text = paragraphTextFromRuns(remainingRuns)
					continuationTextMetrics := resolveTextLayoutMetrics(doc, &continuationParagraph, currentGeom, 0, 0)
					continuationMetrics, continuationOK := paragraphFlowMetricsForPagination(
						doc, &continuationParagraph, currentGeom, currentColumn, currentY+currentGeom.MarginTop,
						continuationTextMetrics, wrapFloatsForFlowRegion(pages[currentPage]),
					)
					if continuationOK && len(continuationMetrics.lineHeights) > 0 {
						metrics.lineHeights = append(metrics.lineHeights[:startLine], continuationMetrics.lineHeights...)
						metrics.physicalLines = append(metrics.physicalLines[:startLine], continuationMetrics.physicalLines...)
					}
				}
				remainingHeight := effectiveContentHeight(currentGeom) - currentY
				linesToTake, fragmentHeight := metrics.fragmentFit(startLine, remainingHeight, p.Format.WidowControl)
				if linesToTake == 0 && !columnHasContent {
					// An empty region cannot honour widow control or hold an oversized
					// line any better than the next one, so Word breaks here anyway.
					linesToTake, fragmentHeight = metrics.fragmentFit(startLine, remainingHeight, false)
					if linesToTake == 0 {
						linesToTake, fragmentHeight = 1, metrics.singleLineFragmentHeight(startLine)
					}
				}
				if linesToTake == 0 {
					if !columnHasContent {
						return false
					}
					previousPage := currentPage
					emitEpilogue()
					advanceFlow(section)
					if pagePrelude != nil && currentPage != previousPage {
						emitPrelude()
					}
					continue
				}

				endLine := startLine + linesToTake
				physicalLines := metrics.fragmentPhysicalLines(startLine, endLine)
				fragmentRuns := paragraphPhysicalLinesRuns(physicalLines)
				fragmentOriginY := fragmentFlowOriginY(startLine, metrics, physicalLines)
				fragment := &ParagraphFragment{
					StartLine:        startLine,
					EndLine:          endLine,
					Lines:            paragraphLineLayouts(physicalLines, fragmentOriginY),
					SpatialRendering: paragraphPhysicalLinesNeedSpatialRendering(p, currentGeom, currentColumn, physicalLines),
					IsContinuation:   startLine > 0,
					IsLast:           endLine == len(metrics.lineHeights),
					OriginY:          fragmentOriginY,
				}
				addFootnotesForRuns(fragmentRuns, currentGeom)
				// A floating anchor belongs to the first physical fragment only. Keep
				// its FloatBox on that page so following paragraphs inherit the same
				// exclusion geometry used while breaking this paragraph. Continuation
				// fragments contain only the remaining physical runs and must not
				// duplicate the anchor on later pages/columns.
				var fragmentFloats []FloatBox
				if startLine == 0 && paragraphHasFloatingContent(p) {
					fragmentFloats = appendParagraphFloats(nil, p.Runs, currentGeom, currentColumn, currentY+currentGeom.MarginTop, fragmentHeight)
				}
				emitBlockBox(block, currentGeom, fragmentHeight, fragmentFloats, fragment)
				startLine = endLine
				if startLine >= len(metrics.lineHeights) {
					return true
				}

				previousPage := currentPage
				emitEpilogue()
				advanceFlow(section)
				if pagePrelude != nil && currentPage != previousPage {
					emitPrelude()
				}
			}
			return true
		}
		trySplitParagraphOnColumnBreak := func(index int, block *resolve.Block) bool {
			if block == nil || block.Paragraph == nil || currentGeom.ColumnCount < 2 {
				return false
			}
			p := block.Paragraph
			if p.Frame != nil || paragraphHasFloatingContent(p) {
				return false
			}
			segments := paragraphColumnBreakSegments(p)
			if len(segments) <= 1 {
				return false
			}

			prevPara := adjacentBlock(blockList, index, -1)
			nextPara := adjacentBlock(blockList, index, +1)
			baseSpacingBefore := ParagraphSpacingBefore(doc, p, prevPara, currentGeom, flowStartSuppressesSpacing(blockList, index, currentY))
			baseSpacingAfter := ParagraphSpacingAfter(doc, p, nextPara, currentGeom)

			for segIndex, segRuns := range segments {
				if segIndex > 0 {
					if currentColumn < currentGeom.ColumnCount-1 {
						currentColumn++
						currentY = currentColumnStartY
						columnHasContent = false
					} else {
						previousPage := currentPage
						emitEpilogue()
						advanceFlow(section)
						if pagePrelude != nil && currentPage != previousPage {
							emitPrelude()
						}
					}
				}

				geom := currentGeom
				segmentPara := *p
				segmentPara.Runs = segRuns
				segmentPara.Text = paragraphTextFromRuns(segRuns)
				height := paragraphHeightInFlow(doc, &segmentPara, geom, currentColumn, currentY+geom.MarginTop, wrapFloatsForFlowRegion(pages[currentPage]))
				if segIndex == 0 {
					height += baseSpacingBefore
				}
				if segIndex == len(segments)-1 {
					height += baseSpacingAfter
				}
				availH := effectiveContentHeight(geom)
				if columnHasContent && currentY+height > availH {
					if currentColumn < geom.ColumnCount-1 {
						currentColumn++
						currentY = currentColumnStartY
						columnHasContent = false
					} else {
						previousPage := currentPage
						emitEpilogue()
						advanceFlow(section)
						if pagePrelude != nil && currentPage != previousPage {
							emitPrelude()
						}
					}
					geom = currentGeom
					height = paragraphHeightInFlow(doc, &segmentPara, geom, currentColumn, currentY+geom.MarginTop, wrapFloatsForFlowRegion(pages[currentPage]))
					if segIndex == 0 {
						height += baseSpacingBefore
					}
					if segIndex == len(segments)-1 {
						height += baseSpacingAfter
					}
				}

				blockFloats := appendParagraphFloats(nil, segmentPara.Runs, geom, currentColumn, currentY+geom.MarginTop, height)
				addFootnotesForRuns(segmentPara.Runs, geom)

				// Explicit column-break segments are real paragraph fragments, not a
				// renderer-only structural exception. Project the segment through the
				// same physical-line owner used by ordinary and paginated paragraphs so
				// every text-bearing fragment crosses the layout/render boundary with
				// canonical line geometry.
				fragment := fullParagraphPhysicalFragment(
					doc, &segmentPara, geom, currentColumn, currentY,
					func() float64 {
						if segIndex == 0 {
							return baseSpacingBefore
						}
						return 0
					}(),
					func() float64 {
						if segIndex == len(segments)-1 {
							return baseSpacingAfter
						}
						return 0
					}(),
					wrapFloatsForFlowRegion(pages[currentPage]),
				)
				if fragment == nil {
					// Drawing-only segments intentionally have no physical text line.
					fragment = &ParagraphFragment{FallbackRuns: segmentPara.Runs}
				}
				fragment.IsContinuation = segIndex > 0
				fragment.IsLast = segIndex == len(segments)-1
				fragment.StartsNewColumn = segIndex > 0 && currentColumn > 0
				emitBlockBox(block, geom, height, blockFloats, fragment)
			}
			return true
		}
		trySplitTableAcrossFlow := func(block *resolve.Block, reserveHeight float64) bool {
			if block == nil || block.Table == nil || block.Table.Format.FloatProps != nil {
				return false
			}
			t := block.Table
			if len(t.Rows) < 2 {
				return false
			}
			rowHeights := TableRowHeights(doc, t, flowContentWidthPt(currentGeom))
			if len(rowHeights) != len(t.Rows) {
				return false
			}
			refreshRowHeights := func() bool {
				rowHeights = TableRowHeights(doc, t, flowContentWidthPt(currentGeom))
				return len(rowHeights) == len(t.Rows)
			}
			flowLimitHeight := effectiveContentHeight(currentGeom) - reserveHeight
			if flowLimitHeight < 1 {
				flowLimitHeight = 1
			}
			totalHeight := sumFloat64s(rowHeights)
			if currentY+totalHeight <= flowLimitHeight {
				return false
			}

			headerEnd := leadingTableHeaderRows(t)
			startRow := 0
			for startRow < len(t.Rows) {
				repeatHeaders := tableFragmentRepeatsHeaders(startRow, headerEnd)
				headerHeight := 0.0
				if repeatHeaders {
					headerHeight = sumFloat64s(rowHeights[:headerEnd])
				}

				remainingHeight := flowLimitHeight - currentY
				if remainingHeight <= headerHeight && columnHasContent {
					previousPage := currentPage
					emitEpilogue()
					advanceFlow(section)
					if pagePrelude != nil && currentPage != previousPage {
						emitPrelude()
					}
					if !refreshRowHeights() {
						return false
					}
					flowLimitHeight = tableFlowLimitHeight(effectiveContentHeight(currentGeom), reserveHeight)
					continue
				}

				endRow := startRow
				fragmentHeight := headerHeight
				lastSafeEnd := startRow
				lastSafeHeight := headerHeight
				for endRow < len(t.Rows) {
					rowHeight := rowHeights[endRow]
					if endRow > startRow && fragmentHeight+rowHeight > remainingHeight {
						break
					}
					if endRow == startRow && fragmentHeight+rowHeight > remainingHeight && columnHasContent {
						break
					}
					fragmentHeight += rowHeight
					endRow++
					if tableFragmentBoundarySafe(t, endRow) {
						lastSafeEnd = endRow
						lastSafeHeight = fragmentHeight
					}
				}

				// A page boundary must not cut through a vertical merge. Prefer the
				// last complete merge group that fitted. If the first group cannot fit
				// in the remaining region, move it to a fresh region; if it is taller
				// than a whole page, keep the complete group atomic and let it overflow.
				if endRow < len(t.Rows) && !tableFragmentBoundarySafe(t, endRow) {
					if lastSafeEnd > startRow {
						endRow = lastSafeEnd
						fragmentHeight = lastSafeHeight
					} else if columnHasContent {
						previousPage := currentPage
						emitEpilogue()
						advanceFlow(section)
						if pagePrelude != nil && currentPage != previousPage {
							emitPrelude()
						}
						continue
					} else {
						endRow = startRow
						fragmentHeight = headerHeight
						for endRow < len(t.Rows) {
							fragmentHeight += rowHeights[endRow]
							endRow++
							if tableFragmentBoundarySafe(t, endRow) {
								break
							}
						}
					}
				}

				// Keep a leading header group with at least one body row when the
				// whole group can fit at the top of a fresh flow region.
				if startRow == 0 && headerEnd > 0 && endRow == headerEnd && headerEnd < len(t.Rows) && columnHasContent {
					need := sumFloat64s(rowHeights[:headerEnd+1])
					if need <= flowLimitHeight {
						previousPage := currentPage
						emitEpilogue()
						advanceFlow(section)
						if pagePrelude != nil && currentPage != previousPage {
							emitPrelude()
						}
						continue
					}
				}

				if endRow == startRow {
					if columnHasContent {
						previousPage := currentPage
						emitEpilogue()
						advanceFlow(section)
						if pagePrelude != nil && currentPage != previousPage {
							emitPrelude()
						}
						continue
					}
					// A single row can itself be taller than the page. Keep it atomic
					// (especially for cantSplit) and allow the renderer to overflow it.
					endRow++
					fragmentHeight += rowHeights[startRow]
				}

				pages[currentPage].Blocks = append(pages[currentPage].Blocks, BlockBox{
					Block:    block,
					Geometry: currentGeom,
					Page:     pages[currentPage].Number,
					Column:   currentColumn,
					Y:        currentY,
					Height:   fragmentHeight,
					TableFragment: &TableFragment{
						StartRow:       startRow,
						EndRow:         endRow,
						RepeatHeaders:  repeatHeaders,
						IsContinuation: startRow > 0,
						IsLast:         endRow == len(t.Rows),
					},
				})
				currentY += fragmentHeight
				columnHasContent = true
				startRow = endRow

				if startRow < len(t.Rows) {
					previousPage := currentPage
					emitEpilogue()
					advanceFlow(section)
					if pagePrelude != nil && currentPage != previousPage {
						emitPrelude()
					}
					flowLimitHeight = tableFlowLimitHeight(effectiveContentHeight(currentGeom), reserveHeight)
				}
			}
			return true
		}
		pendingPrelude := false
		// Consecutive legacy SHAPE-field paragraphs belong to one anchor-line
		// sequence. Some DOC producers/parser paths expose each field as a separate
		// paragraph, so resetting the packing cursor per paragraph collapses every
		// zero-offset shape onto the same page-margin origin.
		var legacyFlowSequence *legacyShapeFlowState
		legacyFlowPage := -1
		legacyFlowColumn := -1
		// Position-independent floats anchored in later blocks still wrap the
		// text that precedes their anchor, matching Word's bottom-up float
		// placement. pendingIndependents is block-ordered; pendingCursor skips
		// anchors that have already been emitted.
		pendingIndependents := pendingIndependentFloats(blockList, currentGeom)
		pendingCursor := 0
		for i := range blockList {
			block := &blockList[i]
			if pendingPrelude {
				emitPrelude()
				pendingPrelude = false
			}
			// Floats committed to this page plus position-independent floats
			// anchored in blocks that have not been emitted yet. The pending
			// floats are filtered again inside paragraphWrapExclusions by their
			// vertical overlap with the paragraph being measured.
			for pendingCursor < len(pendingIndependents) && pendingIndependents[pendingCursor].order <= i {
				pendingCursor++
			}
			blockActiveFloats := wrapFloatsForFlowRegion(pages[currentPage])
			if pendingCursor < len(pendingIndependents) {
				blockActiveFloats = appendPendingIndependentFloats(blockActiveFloats, pendingIndependents[pendingCursor:], blockList, i)
			}
			geom := currentGeom
			if blockForcesPageBreakBefore(block) && pageHasContent(pages[currentPage]) {
				emitEpilogue()
				currentPage = startPage(section)
				currentColumn = 0
				currentY = 0
				columnHasContent = false
				pendingPrelude = pagePrelude != nil
				geom = currentGeom
			}
			clusterEnd := keepNextClusterEnd(blockList, i)
			hasMoreBlocks := i < len(blockList)-1
			epilogueReserveHeight := 0.0
			if hasMoreBlocks && pageEpilogue != nil {
				epilogueReserveHeight = estimateExtraBlocksHeight(pageEpilogue(), geom)
			}
			reservedAvailH := reservedBodyHeightFromContent(effectiveContentHeight(geom), epilogueReserveHeight)
			if clusterEnd > i {
				clusterHeight := measureBlockClusterHeightAt(doc, blockList, i, clusterEnd, geom, currentColumn, currentY, blockActiveFloats)
				availH := effectiveContentHeight(geom)
				if clusterEnd < len(blockList)-1 && pageEpilogue != nil {
					availH = reservedAvailH
				}
				if keepClusterShouldAdvance(clusterHeight, currentY, availH, columnHasContent) {
					emitEpilogue()
					advanceFlow(section)
					geom = currentGeom
				}
			}

			height, blockFloats := measureBlock(doc, blockList, i, geom, currentColumn, currentY, blockActiveFloats)
			availH := effectiveContentHeight(geom)

			// Drawing-only OOXML paragraphs still own physical pagination. Preflight
			// paragraph-relative anchors against the remaining physical page before
			// committing the paragraph; otherwise a late-page textbox is painted at
			// y>pageHeight and clipped while the document incorrectly remains one page.
			if block.Kind == "paragraph" && block.Paragraph != nil && columnHasContent &&
				paragraphDrawingRequiresNextPage(block.Paragraph, geom, currentY, height) {
				previousPage := currentPage
				emitEpilogue()
				advanceFlow(section)
				if pagePrelude != nil && currentPage != previousPage {
					emitPrelude()
				}
				geom = currentGeom
				height, blockFloats = measureBlock(doc, blockList, i, geom, currentColumn, currentY, blockActiveFloats)
				availH = effectiveContentHeight(geom)
			}
			if hasMoreBlocks && pageEpilogue != nil && columnHasContent && currentY+height > reservedAvailH {
				previousPage := currentPage
				emitEpilogue()
				advanceFlow(section)
				if currentPage != previousPage && pagePrelude != nil {
					emitPrelude()
				}
				geom = currentGeom
				hasMoreBlocks = i < len(blockList)-1
				epilogueReserveHeight = 0
				if hasMoreBlocks && pageEpilogue != nil {
					epilogueReserveHeight = estimateExtraBlocksHeight(pageEpilogue(), geom)
				}
				reservedAvailH = reservedBodyHeightFromContent(effectiveContentHeight(geom), epilogueReserveHeight)
				height, blockFloats = measureBlock(doc, blockList, i, geom, currentColumn, currentY, blockActiveFloats)
				availH = effectiveContentHeight(geom)
			}
			// A paragraph-relative floating drawing is anchored to this physical
			// paragraph. If its authored rectangle would cross the physical page
			// bottom, Word moves the anchor paragraph to the next flow region rather
			// than painting the object into the clipped area below the page.
			//
			// This matters especially for drawing-only paragraphs used by Word
			// templates: their ordinary paragraph height can still fit while the
			// anchored text box itself cannot.
			if columnHasContent && paragraphRelativeFloatOverflowsPage(blockFloats, geom) {
				previousPage := currentPage
				emitEpilogue()
				advanceFlow(section)
				if pagePrelude != nil && currentPage != previousPage {
					emitPrelude()
				}
				geom = currentGeom
				height, blockFloats = measureBlock(doc, blockList, i, geom, currentColumn, currentY, blockActiveFloats)
				availH = effectiveContentHeight(geom)
			}
			if trySplitParagraphOnColumnBreak(i, block) {
				if endnoteCollector != nil {
					endnoteCollector(block)
				}
				continue
			}
			if trySplitParagraphAcrossFlow(i, block, geom, currentColumn, currentY, blockActiveFloats, availH) {
				if endnoteCollector != nil {
					endnoteCollector(block)
				}
				continue
			}
			tableReserveHeight := 0.0
			if hasMoreBlocks && pageEpilogue != nil {
				tableReserveHeight = epilogueReserveHeight
			}
			if trySplitTableAcrossFlow(block, tableReserveHeight) {
				if endnoteCollector != nil {
					endnoteCollector(block)
				}
				continue
			}
			if shouldKeepWhole(block) && columnHasContent && currentY+height > availH {
				emitEpilogue()
				advanceFlow(section)
				if pagePrelude != nil {
					emitPrelude()
				}
				geom = currentGeom
				height, blockFloats = measureBlock(doc, blockList, i, geom, currentColumn, currentY, blockActiveFloats)
				availH = effectiveContentHeight(geom)
			}
			if columnHasContent && currentY+height > availH {
				previousPage := currentPage
				emitEpilogue()
				advanceFlow(section)
				if pagePrelude != nil && currentPage != previousPage {
					emitPrelude()
				}
				geom = currentGeom
				height, blockFloats = measureBlock(doc, blockList, i, geom, currentColumn, currentY, blockActiveFloats)
			}

			if block != nil && block.Paragraph != nil {
				if paragraphHasLegacyShapeFlow(block.Paragraph) {
					if legacyFlowSequence == nil || legacyFlowPage != currentPage || legacyFlowColumn != currentColumn {
						legacyFlowSequence = &legacyShapeFlowState{
							left:  columnLeftPt(geom, currentColumn),
							width: columnWidthPt(geom, currentColumn) - geom.MarginLeft,
							y:     legacyShapeFlowInitialY(block.Paragraph.Runs, currentY+geom.MarginTop, height),
						}
						if legacyFlowSequence.width <= 0 {
							legacyFlowSequence.width = geom.Width - geom.MarginLeft - geom.MarginRight
						}
						legacyFlowPage = currentPage
						legacyFlowColumn = currentColumn
					}
					blockFloats = appendParagraphFloatsWithLegacyFlowState(
						nil, block.Paragraph.Runs, geom, currentColumn,
						currentY+geom.MarginTop, height, legacyFlowSequence,
					)
				} else {
					legacyFlowSequence = nil
					legacyFlowPage = -1
					legacyFlowColumn = -1
				}
				addFootnotesForRuns(block.Paragraph.Runs, geom)
			} else {
				legacyFlowSequence = nil
				legacyFlowPage = -1
				legacyFlowColumn = -1
			}
			if endnoteCollector != nil {
				endnoteCollector(block)
			}

			var paragraphFragment *ParagraphFragment
			if block != nil && block.Paragraph != nil && block.Paragraph.Frame == nil {
				prevPara := adjacentBlock(blockList, i, -1)
				nextPara := adjacentBlock(blockList, i, +1)
				spacingBefore := ParagraphSpacingBefore(doc, block.Paragraph, prevPara, geom, flowStartSuppressesSpacing(blockList, i, currentY))
				spacingAfter := ParagraphSpacingAfter(doc, block.Paragraph, nextPara, geom)

				// The physical fragment is the canonical paragraph topology produced by
				// layout. Once it exists, attach it to the block unconditionally so
				// rendering consumes the exact lines that pagination measured. Float
				// geometry still affects how those lines are planned, but it no longer
				// decides whether the canonical topology crosses the layout/render
				// boundary.
				paragraphFragment = fullParagraphPhysicalFragment(
					doc, block.Paragraph, geom, currentColumn, currentY,
					spacingBefore, spacingAfter, blockActiveFloats,
				)
			}
			emitBlockBox(block, geom, height, blockFloats, paragraphFragment)

			if blockForcesPageBreakAfter(block) && i < len(blockList)-1 {
				currentPage = startPage(section)
				currentColumn = 0
				currentY = 0
				columnHasContent = false
				pendingPrelude = pagePrelude != nil
			}
		}
	}

	for spanIndex, span := range spans {
		currentSectionIndex = span.SectionIndex
		currentSectionPageNumber = 0
		nextGeom := resolveGeometryForSectionPageInSection(doc, &span.Section, len(pages)+1, 1)
		if currentPage == -1 {
			currentPage = startPage(span.Section)
			currentColumn = 0
			currentY = 0
			currentColumnStartY = 0
			columnHasContent = false
		} else if sectionStartsNewPage(spans[spanIndex-1].Section, span.Section, currentGeom, nextGeom, pages[currentPage]) {
			currentPage = startPage(span.Section)
			currentColumn = 0
			currentY = 0
			currentColumnStartY = 0
			columnHasContent = false
		} else if !sameGeometry(currentGeom, nextGeom) {
			currentGeom = samePageContinuousGeometry(currentGeom, nextGeom)
			currentColumn = 0
			currentColumnStartY = currentY
		}

		spanSeenEndnotes := make(map[int]bool)
		spanEndnoteIDs := make([]int, 0)
		flowBlocks(span.Section, span.Blocks, func(block *resolve.Block) {
			if endnotePosition == "sectEnd" {
				collectEndnotesForBlock(block, spanSeenEndnotes, &spanEndnoteIDs)
				return
			}
			collectEndnotesForBlock(block, seenDocEndnotes, &docEndnoteIDs)
		}, nil, nil)
		if endnotePosition == "sectEnd" {
			flowBlocks(span.Section, materializeEndnoteBlocks(doc, spanEndnoteIDs, flowContentWidthPt(currentGeom), &nextSyntheticIndex), nil,
				func() []resolve.Block { return continuationSeparatorBlocks(doc, &nextSyntheticIndex) },
				func() []resolve.Block { return continuationNoticeBlocks(doc, &nextSyntheticIndex) },
			)
		}
	}

	if len(pages) == 0 {
		pages = append(pages, Page{Number: 1, Geometry: resolveGeometry(doc)})
		pageFootnoteH = append(pageFootnoteH, 0)
		pageFootnoteIDs = append(pageFootnoteIDs, nil)
		currentPage = 0
		currentGeom = pages[0].Geometry
	}
	if endnotePosition != "sectEnd" && len(docEndnoteIDs) > 0 {
		finalSection := resolve.Section{}
		if len(spans) > 0 {
			finalSection = spans[len(spans)-1].Section
			currentSectionIndex = spans[len(spans)-1].SectionIndex
		} else if doc != nil && len(doc.Sections) > 0 {
			finalSection = doc.Sections[len(doc.Sections)-1]
			currentSectionIndex = len(doc.Sections) - 1
		}
		flowBlocks(finalSection, materializeEndnoteBlocks(doc, docEndnoteIDs, flowContentWidthPt(currentGeom), &nextSyntheticIndex), nil,
			func() []resolve.Block { return continuationSeparatorBlocks(doc, &nextSyntheticIndex) },
			func() []resolve.Block { return continuationNoticeBlocks(doc, &nextSyntheticIndex) },
		)
	}
	materializePageFootnotes(doc, pages, pageFootnoteIDs, &nextSyntheticIndex)
	return pages
}

// reservedBodyHeight is the canonical page body boundary. Static header/footer
// ownership is already encoded in PageGeometry margins; dynamic reservations
// (footnotes and continuation epilogues) are deducted exactly once here.
func reservedBodyHeight(geom PageGeometry, footnoteReserve, epilogueReserve float64) float64 {
	return reservedBodyHeightFromContent(pageContentHeight(geom)-footnoteReserve, epilogueReserve)
}

func reservedBodyHeightFromContent(contentHeight, epilogueReserve float64) float64 {
	limit := contentHeight - epilogueReserve
	if limit < 1 {
		return 1
	}
	return limit
}

// tableFlowLimitHeight is the canonical vertical boundary for table fragments.
// It deliberately uses the same reserved body region on the first and every
// continuation flow region, so repeated headers and body rows cannot consume
// space owned by page epilogues (footnotes/endnote continuation material).
func tableFlowLimitHeight(contentHeight, reserveHeight float64) float64 {
	return reservedBodyHeightFromContent(contentHeight, reserveHeight)
}

func materializePageFootnotes(doc *resolve.Document, pages []Page, footnoteIDs [][]int, nextSyntheticIndex *int) {
	if doc == nil {
		return
	}
	number := footnoteStartNumberForSection(doc, resolve.Section{})
	previousSectionIndex := -1
	for pageIndex := range pages {
		if pageIndex >= len(footnoteIDs) || len(footnoteIDs[pageIndex]) == 0 {
			continue
		}
		page := &pages[pageIndex]
		format, restart := footnoteFormatAndRestartForSection(doc, page.Section)
		sectionChanged := previousSectionIndex >= 0 && page.SectionIndex != previousSectionIndex
		if restart == "eachPage" || (restart == "eachSect" && sectionChanged) || previousSectionIndex < 0 {
			number = footnoteStartNumberForSection(doc, page.Section)
		}
		previousSectionIndex = page.SectionIndex
		geom := page.Geometry
		footnoteGeom := footnoteLayoutGeometry(geom)
		blocks, nextNumber := materializeFootnoteBlocks(doc, footnoteIDs[pageIndex], number, format, footnoteGeom.ColumnWidthPt, nextSyntheticIndex)
		if len(blocks) == 0 {
			continue
		}
		number = nextNumber
		heights := make([]float64, len(blocks))
		totalHeight := 0.0
		for i := range blocks {
			height, _ := measureBlock(doc, blocks, i, footnoteGeom, 0, totalHeight, nil)
			heights[i] = height
			totalHeight += height
		}
		startY := pageContentHeight(geom) - totalHeight
		if startY < 0 {
			startY = 0
		}
		y := startY
		for i := range blocks {
			block := &blocks[i]
			page.Footnotes = append(page.Footnotes, BlockBox{
				Block:    block,
				Geometry: footnoteGeom,
				Page:     page.Number,
				Column:   0,
				Y:        y,
				Height:   heights[i],
			})
			y += heights[i]
		}
	}
}

func footnoteFormatAndRestartForSection(doc *resolve.Document, section resolve.Section) (string, string) {
	format := section.FootnoteNumFmt
	if format == "" && doc != nil {
		format = doc.FootnoteNumberFormat
	}
	if format == "" {
		format = "decimal"
	}
	restart := section.FootnoteRestart
	if restart == "" && doc != nil {
		restart = doc.FootnoteRestart
	}
	if restart == "" {
		restart = "continuous"
	}
	return format, restart
}

func footnoteStartNumberForSection(doc *resolve.Document, section resolve.Section) int {
	if section.FootnoteNumStart > 0 {
		return section.FootnoteNumStart
	}
	return footnoteStartNumber(doc)
}

func materializeFootnoteBlocks(doc *resolve.Document, noteIDs []int, startNumber int, noteFormat string, widthPt float64, nextSyntheticIndex *int) ([]resolve.Block, int) {
	if doc == nil || len(noteIDs) == 0 {
		return nil, startNumber
	}
	blocks := make([]resolve.Block, 0, len(noteIDs)+1)
	blocks = append(blocks, footnoteSeparatorBlocks(doc, widthPt, nextSyntheticIndex)...)
	number := startNumber
	for _, id := range noteIDs {
		story, ok := doc.Footnotes[id]
		if !ok {
			continue
		}
		blocks = append(blocks, cloneFootnoteStoryBlocksForLayout(story, id, number, noteFormat, nextSyntheticIndex)...)
		number++
	}
	return blocks, number
}

func materializeEndnoteBlocks(doc *resolve.Document, noteIDs []int, widthPt float64, nextSyntheticIndex *int) []resolve.Block {
	if doc == nil || len(noteIDs) == 0 {
		return nil
	}
	blocks := make([]resolve.Block, 0, len(noteIDs)+1)
	blocks = append(blocks, startSeparatorBlocks(doc, widthPt, nextSyntheticIndex)...)
	number := 1
	if doc.EndnoteNumberStart > 0 {
		number = doc.EndnoteNumberStart
	}
	format := doc.EndnoteNumberFormat
	if format == "" {
		format = "decimal"
	}
	for _, id := range noteIDs {
		story, ok := doc.Endnotes[id]
		if !ok {
			continue
		}
		blocks = append(blocks, cloneEndnoteStoryBlocksForLayout(story, id, number, format, nextSyntheticIndex)...)
		number++
	}
	return blocks
}

func startSeparatorBlocks(doc *resolve.Document, widthPt float64, nextSyntheticIndex *int) []resolve.Block {
	if doc != nil && storyHasContent(doc.EndnoteSeparator) {
		return clonePlainStoryBlocksForLayout(doc.EndnoteSeparator, nextSyntheticIndex)
	}
	return []resolve.Block{makeNoteSeparatorBlock(doc, widthPt, nextSyntheticIndex)}
}

// noteSeparatorHeightPt measures the separator material Word prints above the
// first footnote on a page. materializePageFootnotes lays the same blocks out
// at the bottom of the page, so the two measurements must stay in step: the
// body region shrinks by exactly what the separator occupies.
func noteSeparatorHeightPt(doc *resolve.Document, geom PageGeometry) float64 {
	var scratch int
	blocks := footnoteSeparatorBlocks(doc, footnoteLayoutGeometry(geom).ColumnWidthPt, &scratch)
	if len(blocks) == 0 {
		return 0
	}
	footnoteGeom := footnoteLayoutGeometry(geom)
	total := 0.0
	for i := range blocks {
		height, _ := measureBlock(doc, blocks, i, footnoteGeom, 0, total, nil)
		total += height
	}
	return total
}

func footnoteSeparatorBlocks(doc *resolve.Document, widthPt float64, nextSyntheticIndex *int) []resolve.Block {
	if doc != nil && storyHasContent(doc.FootnoteSeparator) {
		return clonePlainStoryBlocksForLayout(doc.FootnoteSeparator, nextSyntheticIndex)
	}
	return []resolve.Block{makeNoteSeparatorBlock(doc, widthPt, nextSyntheticIndex)}
}

func continuationSeparatorBlocks(doc *resolve.Document, nextSyntheticIndex *int) []resolve.Block {
	if doc == nil || !storyHasContent(doc.EndnoteContinuationSep) {
		return nil
	}
	return clonePlainStoryBlocksForLayout(doc.EndnoteContinuationSep, nextSyntheticIndex)
}

func continuationNoticeBlocks(doc *resolve.Document, nextSyntheticIndex *int) []resolve.Block {
	if doc == nil || !storyHasContent(doc.EndnoteContinuationNotice) {
		return nil
	}
	return clonePlainStoryBlocksForLayout(doc.EndnoteContinuationNotice, nextSyntheticIndex)
}

// noteSeparatorBandTwips sizes the strip between the separator rule and the
// first note. Word draws the rule on the separator line's baseline, so only
// that line's descender band sits under it; reserving the whole line box
// instead would lift the rule half a line away from the notes and let the body
// stop short of the page's real text boundary. The rule is part of the
// paragraph's border box, so the exact line height carries the band less the
// 0.75pt the border itself occupies.
const noteSeparatorBandTwips = 87 // 4.37pt + 0.75pt rule = a 5.12pt band

// noteSeparatorRuleTwips is the length Word gives the footnote and endnote
// separator rule: two inches, measured from the notes' text indent.
const noteSeparatorRuleTwips = 2880

// makeNoteSeparatorBlock synthesizes the separator Word prints before the
// first note of a page or section. Word gives it the default paragraph's
// first-line indent (the separator style inherits Normal) and draws a two-inch
// rule there, so the rule starts where the note text does and stops well short
// of the column edge.
func makeNoteSeparatorBlock(doc *resolve.Document, widthPt float64, nextSyntheticIndex *int) resolve.Block {
	format := resolve.ParagraphFormat{
		KeepNext: true,
		// The rule is the paragraph's top border; the paragraph itself only
		// occupies the band below it.
		LineRule:      "exact",
		LineSpacing:   layoutIntPtr(noteSeparatorBandTwips),
		NoteSeparator: true,
		Borders: &resolve.ParagraphBorders{
			Top: &resolve.BorderSide{Val: "single", SizePt: 0.75, Color: "444444"},
		},
	}
	// The separator paragraph carries no style of its own, so it inherits the
	// document's default paragraph style: that is where Word's rule indent comes
	// from.
	indentLeft := 0
	if doc != nil {
		if style, ok := doc.Styles[doc.DefaultParagraphStyleID]; ok && style.Format.IndentFirstLine != nil {
			indentLeft = *style.Format.IndentFirstLine
		} else if doc.DefaultParagraphFormat.IndentFirstLine != nil {
			indentLeft = *doc.DefaultParagraphFormat.IndentFirstLine
		}
	}
	format.IndentLeft = layoutIntPtr(indentLeft)
	contentTwips := int(math.Round(widthPt * twipsPerPoint))
	if right := contentTwips - indentLeft - noteSeparatorRuleTwips; right > 0 {
		format.IndentRight = layoutIntPtr(right)
	}
	separator := resolve.Paragraph{
		Index:  nextLayoutSyntheticIndex(nextSyntheticIndex),
		Format: format,
	}
	return resolve.Block{
		Index:     nextLayoutSyntheticIndex(nextSyntheticIndex),
		Kind:      "paragraph",
		Paragraph: &separator,
	}
}

func clonePlainStoryBlocksForLayout(story resolve.Story, nextSyntheticIndex *int) []resolve.Block {
	blocks := contentBlocks(story.Blocks)
	if len(blocks) == 0 {
		return nil
	}
	cloned := make([]resolve.Block, 0, len(blocks))
	for _, block := range blocks {
		copyBlock := block
		copyBlock.Index = nextLayoutSyntheticIndex(nextSyntheticIndex)
		if block.Paragraph != nil {
			paragraphCopy := *block.Paragraph
			paragraphCopy.Index = nextLayoutSyntheticIndex(nextSyntheticIndex)
			copyBlock.Paragraph = &paragraphCopy
		}
		if block.Table != nil {
			tableCopy := *block.Table
			tableCopy.Index = nextLayoutSyntheticIndex(nextSyntheticIndex)
			copyBlock.Table = &tableCopy
		}
		cloned = append(cloned, copyBlock)
	}
	return cloned
}

func storyHasContent(story resolve.Story) bool {
	return len(contentBlocks(story.Blocks)) > 0 || len(story.Paragraphs) > 0 || len(story.Tables) > 0
}

func cloneEndnoteStoryBlocksForLayout(story resolve.Story, noteID, noteNumber int, noteFormat string, nextSyntheticIndex *int) []resolve.Block {
	blocks := contentBlocks(story.Blocks)
	if len(blocks) == 0 {
		return []resolve.Block{makeNoteLabelBlock("endnote", noteID, noteNumber, noteFormat, nextSyntheticIndex)}
	}
	cloned := make([]resolve.Block, 0, len(blocks))
	decoratedFirstBlock := false
	for _, block := range blocks {
		copyBlock := block
		copyBlock.Index = nextLayoutSyntheticIndex(nextSyntheticIndex)
		if !decoratedFirstBlock {
			copyBlock.NoteKind = "endnote"
			copyBlock.NoteID = noteID
		}
		if block.Paragraph != nil {
			paragraphCopy := *block.Paragraph
			paragraphCopy.Index = nextLayoutSyntheticIndex(nextSyntheticIndex)
			if !decoratedFirstBlock {
				paragraphCopy.List = &resolve.ListMarker{Kind: "ordered", Level: 0, Start: noteNumber, Format: noteFormat}
				decoratedFirstBlock = true
			}
			copyBlock.Paragraph = &paragraphCopy
		}
		if block.Table != nil {
			if !decoratedFirstBlock {
				cloned = append(cloned, makeNoteLabelBlock("endnote", noteID, noteNumber, noteFormat, nextSyntheticIndex))
				decoratedFirstBlock = true
			}
			tableCopy := *block.Table
			tableCopy.Index = nextLayoutSyntheticIndex(nextSyntheticIndex)
			copyBlock.Table = &tableCopy
		}
		cloned = append(cloned, copyBlock)
	}
	return cloned
}

func cloneFootnoteStoryBlocksForLayout(story resolve.Story, noteID, noteNumber int, noteFormat string, nextSyntheticIndex *int) []resolve.Block {
	blocks := contentBlocks(story.Blocks)
	if len(blocks) == 0 {
		return []resolve.Block{makeNoteLabelBlock("footnote", noteID, noteNumber, noteFormat, nextSyntheticIndex)}
	}
	cloned := make([]resolve.Block, 0, len(blocks))
	decoratedFirstBlock := false
	for _, block := range blocks {
		copyBlock := block
		copyBlock.Index = nextLayoutSyntheticIndex(nextSyntheticIndex)
		if !decoratedFirstBlock {
			copyBlock.NoteKind = "footnote"
			copyBlock.NoteID = noteID
		}
		if block.Paragraph != nil {
			paragraphCopy := *block.Paragraph
			paragraphCopy.Index = nextLayoutSyntheticIndex(nextSyntheticIndex)
			if !decoratedFirstBlock {
				paragraphCopy.List = &resolve.ListMarker{Kind: "ordered", Level: 0, Start: noteNumber, Format: noteFormat}
				decoratedFirstBlock = true
			}
			copyBlock.Paragraph = &paragraphCopy
		}
		if block.Table != nil {
			if !decoratedFirstBlock {
				cloned = append(cloned, makeNoteLabelBlock("footnote", noteID, noteNumber, noteFormat, nextSyntheticIndex))
				decoratedFirstBlock = true
			}
			tableCopy := *block.Table
			tableCopy.Index = nextLayoutSyntheticIndex(nextSyntheticIndex)
			copyBlock.Table = &tableCopy
		}
		cloned = append(cloned, copyBlock)
	}
	return cloned
}

func footnoteStartNumber(doc *resolve.Document) int {
	if doc != nil && doc.FootnoteNumberStart > 0 {
		return doc.FootnoteNumberStart
	}
	return 1
}

func makeNoteLabelBlock(noteKind string, noteID, noteNumber int, noteFormat string, nextSyntheticIndex *int) resolve.Block {
	paragraph := resolve.Paragraph{
		Index: nextLayoutSyntheticIndex(nextSyntheticIndex),
		Text:  "",
		List:  &resolve.ListMarker{Kind: "ordered", Level: 0, Start: noteNumber, Format: noteFormat},
		Format: resolve.ParagraphFormat{
			KeepNext: true,
		},
	}
	return resolve.Block{
		Index:     nextLayoutSyntheticIndex(nextSyntheticIndex),
		Kind:      "paragraph",
		Paragraph: &paragraph,
		NoteKind:  noteKind,
		NoteID:    noteID,
	}
}

func layoutIntPtr(v int) *int {
	return &v
}

func nextLayoutSyntheticIndex(nextSyntheticIndex *int) int {
	if nextSyntheticIndex == nil {
		return -1
	}
	index := *nextSyntheticIndex
	*nextSyntheticIndex = index - 1
	return index
}

// resolveGeometry extracts page geometry from the first section, falling back
// to US-Letter defaults when section properties are absent.
func resolveGeometry(doc *resolve.Document) PageGeometry {
	if len(doc.Sections) > 0 {
		return resolveGeometryForSectionPage(doc, &doc.Sections[0], 1)
	}
	return resolveGeometryForSectionPage(doc, nil, 1)
}

func resolveGeometryForSection(doc *resolve.Document, section *resolve.Section) PageGeometry {
	return resolveGeometryForSectionPage(doc, section, 1)
}

// effectiveSectionColumnGapPt converts Word column spacing to points and guards
// against malformed OOXML produced by applications that write EMU values directly
// into w:cols/@w:space. A gap that consumes the complete printable width cannot
// describe a usable Word column grid; Word repairs such geometry rather than
// producing zero-width columns. OOXML also defaults omitted column spacing to
// 720 twips (0.5in). Legacy sections retain an explicit zero gap.
func effectiveSectionColumnGapPt(sec *resolve.Section, contentW float64) float64 {
	if sec == nil || sec.Columns == nil || sec.Columns.Num <= 1 {
		return 0
	}
	space := sec.Columns.Space
	if space <= 0 && sec.Source == nil {
		space = 720
	}
	return saneColumnGapPt(twipsToPoints(space), contentW, sec.Columns.Num)
}

func saneExplicitColumnGapPt(spaceTwips int, contentW float64, columnCount int) float64 {
	return saneColumnGapPt(twipsToPoints(spaceTwips), contentW, columnCount)
}

func saneColumnGapPt(gapPt, contentW float64, columnCount int) float64 {
	if gapPt < 0 {
		gapPt = 0
	}
	if columnCount <= 1 || contentW <= 0 {
		return gapPt
	}
	if gapPt*float64(columnCount-1) < contentW-1 {
		return gapPt
	}

	// 720 twips is Word's normal/default inter-column spacing. Constrain the
	// fallback for exceptionally narrow pages so at least one point remains in
	// every column.
	fallback := 36.0
	maxGap := (contentW - float64(columnCount)) / float64(columnCount-1)
	if maxGap < 0 {
		maxGap = 0
	}
	if fallback > maxGap {
		fallback = maxGap
	}
	return fallback
}

func resolveGeometryForSectionPage(doc *resolve.Document, section *resolve.Section, pageNumber int) PageGeometry {
	return resolveGeometryForSectionPageInSection(doc, section, pageNumber, pageNumber)
}

func resolveGeometryForSectionPageInSection(doc *resolve.Document, section *resolve.Section, pageNumber, sectionPageNumber int) PageGeometry {
	g := PageGeometry{
		Width:        612, // US Letter 8.5in
		Height:       792, // US Letter 11in
		MarginTop:    72,  // 1in
		MarginBottom: 72,  // 1in
		MarginLeft:   72,  // 1in
		MarginRight:  72,  // 1in
	}
	if section == nil {
		contentW := g.Width - g.MarginLeft - g.MarginRight
		g.ColumnCount = 1
		g.ColumnWidthPt = contentW
		g.ColumnGapPt = 0
		return g
	}
	sec := *section
	if sec.PageSize != nil && sec.PageSize.Width > 0 {
		g.Width = twipsToPoints(sec.PageSize.Width)
		if sec.PageSize.Height > 0 {
			g.Height = twipsToPoints(sec.PageSize.Height)
		}
	}
	if sec.PageMargin != nil {
		g.MarginTop = twipsToPoints(sec.PageMargin.Top)
		g.MarginBottom = twipsToPoints(sec.PageMargin.Bottom)
		g.MarginLeft = twipsToPoints(sec.PageMargin.Left)
		g.MarginRight = twipsToPoints(sec.PageMargin.Right)
	}
	if doc.MirrorMargins && pageNumber%2 == 0 {
		g.MarginLeft, g.MarginRight = g.MarginRight, g.MarginLeft
	}

	// Fix 3: column geometry — compute column width for column-relative float positioning.
	contentW := g.Width - g.MarginLeft - g.MarginRight
	g.ColumnCount = 1
	g.ColumnWidthPt = contentW
	g.ColumnGapPt = 0
	g.ColumnSep = false
	if sec.Columns != nil && sec.Columns.Num > 1 {
		g.ColumnCount = sec.Columns.Num
		gapPt := effectiveSectionColumnGapPt(&sec, contentW)
		totalGap := gapPt * float64(sec.Columns.Num-1)
		colW := (contentW - totalGap) / float64(sec.Columns.Num)
		if colW < 1 {
			colW = 1
		}
		g.ColumnWidthPt = colW
		g.ColumnGapPt = gapPt
		g.ColumnSep = sec.Columns.Sep

		// Preserve explicit legacy/OOXML unequal column geometry for layout
		// consumers such as column-relative floating objects. CSS multi-column
		// rendering still uses the equal-width fallback where necessary, but
		// geometric anchors must use the physical source column widths/gaps.
		if len(sec.Columns.Widths) >= sec.Columns.Num {
			g.ExplicitColumns = true
			g.ColumnWidthsPt = make([]float64, sec.Columns.Num)
			valid := true
			for i := 0; i < sec.Columns.Num; i++ {
				w := twipsToPoints(sec.Columns.Widths[i])
				if w <= 0 {
					valid = false
					break
				}
				g.ColumnWidthsPt[i] = w
			}
			if !valid {
				g.ColumnWidthsPt = nil
				g.ExplicitColumns = false
			}
		}
		if len(sec.Columns.Spaces) >= sec.Columns.Num-1 {
			g.ColumnGapsPt = make([]float64, sec.Columns.Num-1)
			for i := range g.ColumnGapsPt {
				g.ColumnGapsPt[i] = saneExplicitColumnGapPt(sec.Columns.Spaces[i], contentW, sec.Columns.Num)
			}
		}
		if len(g.ColumnWidthsPt) > 0 {
			g.ColumnWidthPt = g.ColumnWidthsPt[0]
		}
		if len(g.ColumnGapsPt) > 0 {
			g.ColumnGapPt = g.ColumnGapsPt[0]
		}
	}
	if sec.DocGrid != nil {
		g.DocGridType = sec.DocGrid.Type
		if sec.DocGrid.LinePitch > 0 {
			g.DocGridPitchPt = twipsToPoints(sec.DocGrid.LinePitch)
		}
	}

	// Fix 4: header height — if the default header story is taller than the
	// space allocated between w:pgMar[@w:header] and w:pgMar[@w:top], expand
	// MarginTop to accommodate the overflow so floating objects anchored to the
	// page don't collide with header content.
	if sec.PageMargin != nil && doc != nil {
		headerDistPt := twipsToPoints(sec.PageMargin.Header)
		allocatedPt := g.MarginTop - headerDistPt // space between header edge and body
		if allocatedPt < 0 {
			allocatedPt = 0
		}
		if story, ok := sectionStoryForPage(doc.Headers, sec.Headers, pageNumber, sectionPageNumber, doc.EvenAndOddHeaders, sec.TitlePage); ok {
			storyH := estimateStoryHeightPt(doc, story, contentW)
			if storyH > allocatedPt {
				g.MarginTop += storyH - allocatedPt
			}
		}

		footerDistPt := twipsToPoints(sec.PageMargin.Footer)
		allocatedFooterPt := g.MarginBottom - footerDistPt // space between body and footer edge
		if allocatedFooterPt < 0 {
			allocatedFooterPt = 0
		}
		if story, ok := sectionStoryForPage(doc.Footers, sec.Footers, pageNumber, sectionPageNumber, doc.EvenAndOddHeaders, sec.TitlePage); ok {
			storyH := estimateStoryHeightPt(doc, story, contentW)
			// Keep a small body/footer clearance in the layout frame instead of
			// clipping the rendered page body at the footer top. Browser glyph
			// descenders and Word's natural leading can extend beyond the nominal
			// text line box by a couple of points; reserving that leading here makes
			// pagination move the final line naturally to the next page while leaving
			// the body itself uncropped.
			// Reserve the footer's physical story extent plus the actual
			// bottom overhang of a body line. Do not reserve a complete extra
			// body line: Word can keep the final body line on the same page as a
			// two-line footer as long as their physical boxes do not intersect.
			clearancePt := physicalBodyFooterClearancePt(doc)
			requiredFooterPt := storyH + clearancePt
			if requiredFooterPt > allocatedFooterPt {
				g.MarginBottom += requiredFooterPt - allocatedFooterPt
			}
		}
	}

	return g
}

// physicalBodyFooterClearancePt returns only the part of a normal body line
// that extends beyond the nominal font box. Footer pagination needs this
// physical overhang, not an entire speculative extra line.
func physicalBodyFooterClearancePt(doc *resolve.Document) float64 {
	fontPt := defaultFontSizePt(doc)
	linePt := fontPt * wordSingleLineRatio
	clearancePt := linePt - fontPt
	if clearancePt < 2.0 {
		clearancePt = 2.0
	}
	return clearancePt
}

// layoutBlocks walks the document block list in order, accumulating Y positions
// and collecting floating objects with their absolute page coordinates.
func layoutBlocks(blockList []resolve.Block, doc *resolve.Document, geom PageGeometry, pageNumber int) ([]BlockBox, []FloatBox) {
	blocks := make([]BlockBox, 0, len(blockList))
	var floats []FloatBox

	column := 0
	y := 0.0 // current flow Y, relative to content-area top (after MarginTop)

	for i := range blockList {
		block := &blockList[i]
		height, blockFloats := measureBlock(doc, blockList, i, geom, column, y, floats)
		if block.Kind != "paragraph" && block.Kind != "table" {
			continue
		}
		if len(blocks) > 0 && geom.ColumnCount > 1 && y+height > pageContentHeight(geom) && column < geom.ColumnCount-1 {
			column++
			y = 0
			height, blockFloats = measureBlock(doc, blockList, i, geom, column, y, floats)
		}
		for j := range blockFloats {
			blockFloats[j].Page = pageNumber
		}
		floats = append(floats, blockFloats...)

		blocks = append(blocks, BlockBox{
			Block:    block,
			Geometry: geom,
			Page:     pageNumber,
			Column:   column,
			Y:        y,
			Height:   height,
		})
		y += height
	}

	return blocks, floats
}

// LayoutBlocksForRender lays out a standalone block list inside the supplied
// geometry and returns flow block boxes plus floating objects using the same
// single-frame logic as pagination. This is used for nested story containers
// such as floating text boxes, headers/footers, and note frames.
func LayoutBlocksForRender(doc *resolve.Document, blockList []resolve.Block, geom PageGeometry) ([]BlockBox, []FloatBox) {
	blocks, floats := layoutBlocks(blockList, doc, geom, 1)

	// Standalone stories (headers, footers, note frames, text boxes) must cross
	// the layout/render boundary with the same canonical physical paragraph
	// topology as paginated body content. layoutBlocks owns measurement and float
	// collection; attach the measured paragraph fragment here before the renderer
	// consumes these boxes.
	for i := range blocks {
		box := &blocks[i]
		if box.Block == nil || box.Block.Paragraph == nil || box.Block.Paragraph.Frame != nil {
			continue
		}
		p := box.Block.Paragraph
		blockIndex := -1
		for j := range blockList {
			if &blockList[j] == box.Block {
				blockIndex = j
				break
			}
		}
		var prevPara, nextPara *resolve.Paragraph
		if blockIndex >= 0 {
			prevPara = adjacentBlock(blockList, blockIndex, -1)
			nextPara = adjacentBlock(blockList, blockIndex, +1)
		}
		spacingBefore := ParagraphSpacingBefore(doc, p, prevPara, geom, flowStartSuppressesSpacing(blockList, blockIndex, box.Y))
		spacingAfter := ParagraphSpacingAfter(doc, p, nextPara, geom)
		box.Fragment = fullParagraphPhysicalFragment(
			doc, p, geom, box.Column, box.Y,
			spacingBefore, spacingAfter, floats,
		)
	}

	return blocks, floats
}

type sectionSpan struct {
	Section      resolve.Section
	SectionIndex int // index into doc.Sections
	Blocks       []resolve.Block
}

func splitSections(doc *resolve.Document) []sectionSpan {
	if doc == nil {
		return nil
	}
	blocks := contentBlocks(doc.Blocks)
	if len(doc.Blocks) == len(blocks) {
		if len(blocks) == 0 && len(doc.Sections) == 0 {
			return nil
		}
		section := resolve.Section{}
		if len(doc.Sections) > 0 {
			section = doc.Sections[0]
		}
		return []sectionSpan{{Section: section, SectionIndex: 0, Blocks: blocks}}
	}

	spans := make([]sectionSpan, 0, len(doc.Sections))
	current := make([]resolve.Block, 0, len(doc.Blocks))
	sectionIndex := 0

	flush := func(section *resolve.Section) {
		hadContent := len(current) > 0
		resolvedSection := resolve.Section{}
		if section != nil {
			resolvedSection = *section
		} else if sectionIndex < len(doc.Sections) {
			resolvedSection = doc.Sections[sectionIndex]
		} else if len(doc.Sections) > 0 {
			resolvedSection = doc.Sections[len(doc.Sections)-1]
		}
		if hadContent {
			spans = append(spans, sectionSpan{
				Section:      resolvedSection,
				SectionIndex: sectionIndex,
				Blocks:       append([]resolve.Block(nil), current...),
			})
		}
		current = current[:0]
		sectionIndex++
	}

	for i := range doc.Blocks {
		block := doc.Blocks[i]
		if block.Kind == "section" {
			flush(nil)
			continue
		}
		current = append(current, block)
	}

	if len(current) > 0 || len(spans) == 0 {
		resolvedSection := resolve.Section{}
		if sectionIndex < len(doc.Sections) {
			resolvedSection = doc.Sections[sectionIndex]
		} else if len(doc.Sections) > 0 {
			resolvedSection = doc.Sections[len(doc.Sections)-1]
		}
		spans = append(spans, sectionSpan{
			Section:      resolvedSection,
			SectionIndex: sectionIndex,
			Blocks:       append([]resolve.Block(nil), current...),
		})
	}

	return spans
}

func contentBlocks(blocks []resolve.Block) []resolve.Block {
	if len(blocks) == 0 {
		return nil
	}
	content := make([]resolve.Block, 0, len(blocks))
	for _, block := range blocks {
		if block.Kind == "section" {
			continue
		}
		content = append(content, block)
	}
	return content
}

func measureBlock(doc *resolve.Document, blocks []resolve.Block, index int, geom PageGeometry, column int, y float64, activeFloats []FloatBox) (float64, []FloatBox) {
	if index < 0 || index >= len(blocks) {
		return 0, nil
	}
	block := &blocks[index]
	var height float64
	var floats []FloatBox

	switch block.Kind {
	case "paragraph":
		p := block.Paragraph
		if p != nil && p.Frame != nil {
			// Frame paragraphs are out-of-flow; emit a FloatBox and contribute no height.
			fb := frameParaFloat(doc, p, geom, column, y+geom.MarginTop)
			return 0, []FloatBox{fb}
		}
		prevPara := adjacentBlock(blocks, index, -1)
		nextPara := adjacentBlock(blocks, index, +1)
		metrics := resolveParagraphSpacingMetrics(doc, p, prevPara, nextPara, geom, flowStartSuppressesSpacing(blocks, index, y))
		height = paragraphHeightInFlow(doc, p, geom, column, y+geom.MarginTop, activeFloats)
		height += metrics.SpacingBefore + metrics.SpacingAfter
		floats = appendParagraphFloats(nil, p.Runs, geom, column, y+geom.MarginTop, height)
	case "table":
		t := block.Table
		if t != nil && t.Format.FloatProps != nil {
			// Floating tables are out-of-flow; emit a FloatBox and contribute no height.
			fb := tableFloat(doc, t, geom, column, y+geom.MarginTop)
			return 0, []FloatBox{fb}
		}
		height = tableHeight(doc, block.Table, flowContentWidthPt(geom))
	}

	return height, floats
}

// frameParaFloat builds a FloatBox for a w:framePr frame paragraph.
// Frame paragraphs are out-of-flow; their content is rendered at the resolved
// absolute position rather than advancing the block cursor.
func frameParaFloat(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry, column int, paraPageY float64) FloatBox {
	fp := p.Frame
	widthPt := float64(fp.W) / twipsPerPoint
	if widthPt <= 0 {
		widthPt = autoFrameParagraphWidthPt(doc, p, geom)
	}
	heightPt := float64(fp.H) / twipsPerPoint
	if heightPt <= 0 {
		// For drop caps, height is lines × the body text line height — not
		// the drop cap font size. Drop-cap anchors usually carry an oversized
		// "exact" line spacing to fit the cap glyph; the frame spans the
		// body-text lines that flow beside it instead.
		if fp.DropCap != "" && fp.DropCap != "none" && fp.Lines > 0 {
			lineH := paragraphLineHeightPt(p, geom, paragraphFontSizePt(doc, p))
			bodyLineH := defaultFontSizePt(doc) * wordSingleLineRatio
			if doc != nil && doc.DefaultParagraphFormat.LineSpacing != nil {
				dummy := resolve.Paragraph{Format: doc.DefaultParagraphFormat}
				if body := autoParagraphLineHeightPt(&dummy, defaultFontSizePt(doc)); body > 0 {
					bodyLineH = body
				}
			}
			if lineH > bodyLineH*1.5 {
				lineH = bodyLineH
			}
			heightPt = float64(fp.Lines) * snapLineHeightToDocGrid(p, geom, lineH)
		} else {
			heightPt = paragraphHeightInWidth(doc, p, geom, widthPt)
		}
	}

	var x float64
	if fp.XAlign != "" {
		x = resolveXAlign(fp.XAlign, frameAnchorRelativeX(fp.HAnchor), geom, column, widthPt)
	} else {
		offsetPt := float64(fp.X) / twipsPerPoint
		if fp.HAnchor == "page" {
			x = offsetPt
		} else if fp.DropCap != "" && fp.DropCap != "none" {
			// Word places a drop cap at the anchor paragraph's first-line
			// indent rather than the bare margin: a Normal-styled anchor with
			// a 21.6pt first-line indent drops the cap at margin+21.6pt.
			x = paragraphFirstLineLeftPt(p, paragraphContentLeftPt(p, columnLeftPt(geom, column)))
		} else {
			x = geom.MarginLeft + offsetPt
		}
	}

	var y float64
	if fp.YAlign != "" && fp.YAlign != "inline" {
		y = resolveYAlign(fp.YAlign, frameAnchorRelativeY(fp.VAnchor), geom, paraPageY, 0, heightPt)
	} else {
		offsetPt := float64(fp.Y) / twipsPerPoint
		if fp.VAnchor == "page" {
			y = offsetPt
		} else if fp.VAnchor == "text" {
			y = paraPageY + offsetPt
		} else { // "margin" or ""
			y = geom.MarginTop + offsetPt
		}
	}

	return FloatBox{
		FrameParagraph: p,
		X:              x,
		Y:              y,
		Width:          widthPt,
		Height:         heightPt,
	}
}

func autoFrameParagraphWidthPt(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry) float64 {
	if p == nil || p.Frame == nil {
		return geom.ColumnWidthPt
	}
	fp := p.Frame
	if fp.DropCap == "" || fp.DropCap == "none" {
		return geom.ColumnWidthPt
	}

	widthPt := dropCapContentWidthPt(doc, p)
	leftBorderOutset, rightBorderOutset := paragraphHorizontalBorderOutsetPt(p)
	widthPt += leftBorderOutset + rightBorderOutset
	// The frame X already carries the anchor paragraph's first-line indent, so
	// the width must be the cap content alone or the wrap region would double
	// the indent.

	if widthPt <= 0 {
		widthPt = defaultFontSizePt(doc)
	}
	if geom.ColumnWidthPt > 0 && widthPt > geom.ColumnWidthPt {
		return geom.ColumnWidthPt
	}
	return widthPt
}

func dropCapContentWidthPt(doc *resolve.Document, p *resolve.Paragraph) float64 {
	defaultSizePt := defaultFontSizePt(doc)
	catalog := wordfontmetrics.BuildCatalog(doc)
	spaceWidthPt := defaultSizePt * 0.28
	widthPt := 0.0
	emitted := 0
	for _, run := range resolve.CanonicalVisibleRuns(p) {
		if run.Text == "" || run.Format.Hidden() {
			continue
		}
		// The frame has to hold the cap's own advance. A generic per-character
		// estimate is narrower than most letters at display sizes, which clips
		// the glyph, so measure against the family's real metrics when the
		// document provides them.
		width, _ := runWordWidthPtWithCatalog(catalog, run.Format, run.Text, defaultSizePt)
		if width <= 0 {
			width, _ = runWordWidthPt(run.Format, run.Text, defaultSizePt)
		}
		if emitted > 0 {
			widthPt += spaceWidthPt
		}
		widthPt += width
		emitted++
	}
	return widthPt
}

// frameAnchorRelativeX maps w:framePr hAnchor values to the relative
// reference strings used by resolveXAlign.
func frameAnchorRelativeX(anchor string) string {
	switch anchor {
	case "page":
		return "page"
	default: // "text", "margin", ""
		return "margin"
	}
}

// frameAnchorRelativeY maps w:framePr vAnchor values to the relative
// reference strings used by resolveYAlign.
// "text" means relative to the current paragraph line, which maps to "paragraph".
func frameAnchorRelativeY(anchor string) string {
	switch anchor {
	case "page":
		return "page"
	case "text":
		return "paragraph"
	default: // "margin", ""
		return "margin"
	}
}

// tableFloat builds a FloatBox for a w:tblpPr floating table.
// Floating tables are out-of-flow; they contribute no height to the block cursor.
func tableFloat(doc *resolve.Document, t *resolve.Table, geom PageGeometry, column int, tablePageY float64) FloatBox {
	fp := t.Format.FloatProps

	// Resolve the authored table width through the same path used by normal
	// tables. This preserves dxa/pct widths when tblGrid is absent or incomplete
	// instead of expanding every floating table to the full text width.
	widthPt := TableWidthPt(t, flowContentWidthPt(geom))
	if widthPt <= 0 {
		widthPt = flowContentWidthPt(geom)
	}

	heightPt := tableHeight(doc, t, widthPt)

	horzAnchor := fp.HorzAnchor
	if horzAnchor == "" {
		horzAnchor = "text" // OOXML default
	}
	vertAnchor := fp.VertAnchor
	if vertAnchor == "" {
		vertAnchor = "text" // OOXML default
	}

	var x float64
	if fp.XAlign != "" {
		x = resolveXAlign(fp.XAlign, frameAnchorRelativeX(horzAnchor), geom, column, widthPt)
	} else {
		offsetPt := float64(fp.X) / twipsPerPoint
		if horzAnchor == "page" {
			x = offsetPt
		} else {
			x = geom.MarginLeft + offsetPt
		}
	}

	var y float64
	if fp.YAlign != "" && fp.YAlign != "inline" {
		y = resolveYAlign(fp.YAlign, frameAnchorRelativeY(vertAnchor), geom, tablePageY, 0, heightPt)
	} else {
		offsetPt := float64(fp.Y) / twipsPerPoint
		if vertAnchor == "page" {
			y = offsetPt
		} else if vertAnchor == "text" {
			y = tablePageY + offsetPt
		} else { // "margin" or ""
			y = geom.MarginTop + offsetPt
		}
	}

	return FloatBox{
		FloatTable: t,
		X:          x,
		Y:          y,
		Width:      widthPt,
		Height:     heightPt,
	}
}

func keepNextClusterEnd(blocks []resolve.Block, start int) int {
	if start < 0 || start >= len(blocks) {
		return start
	}
	end := start
	for end < len(blocks)-1 && blockKeepsNext(&blocks[end]) {
		// Explicit page boundaries are stronger than keep-with-next. A paragraph
		// before a manual page break, or a following paragraph with
		// PageBreakBefore, must not be preflighted as one movable keep group.
		if blockForcesPageBreakAfter(&blocks[end]) || blockForcesPageBreakBefore(&blocks[end+1]) {
			break
		}
		end++
	}
	return end
}

// keepClusterShouldAdvance decides whether an intact keep-with-next group
// should move to the next flow region. Groups taller than a complete region
// deliberately stay put and fall back to normal block/paragraph fragmentation;
// otherwise repeatedly moving the same oversized group can never make progress.
func keepClusterShouldAdvance(clusterHeight, currentY, availableHeight float64, regionHasContent bool) bool {
	if !regionHasContent || clusterHeight <= 0 || availableHeight <= 0 {
		return false
	}
	if clusterHeight > availableHeight {
		return false
	}
	return clusterHeight > availableHeight-currentY
}

func blockKeepsNext(block *resolve.Block) bool {
	return block != nil && block.Kind == "paragraph" && block.Paragraph != nil && block.Paragraph.Format.KeepNext
}

func measureBlockClusterHeight(doc *resolve.Document, blocks []resolve.Block, start, end int, geom PageGeometry) float64 {
	return measureBlockClusterHeightAt(doc, blocks, start, end, geom, 0, 0, nil)
}

// measureBlockClusterHeightAt measures a keep-with-next chain in the actual
// flow context. This matters for paragraph spacing, column width, and active
// floating exclusions: measuring every member at y=0 made the preflight
// decision disagree with the blocks' real pagination geometry.
func measureBlockClusterHeightAt(doc *resolve.Document, blocks []resolve.Block, start, end int, geom PageGeometry, column int, startY float64, activeFloats []FloatBox) float64 {
	if start < 0 {
		start = 0
	}
	if end >= len(blocks) {
		end = len(blocks) - 1
	}
	total := 0.0
	y := startY
	for i := start; i <= end; i++ {
		height, _ := measureBlock(doc, blocks, i, geom, column, y, activeFloats)
		total += height
		y += height
	}
	return total
}

func pageHasContent(page Page) bool {
	return len(page.Blocks) > 0 || len(page.Floats) > 0
}

func blockShouldMoveToNextPage(currentY, height float64, geom PageGeometry, page Page) bool {
	if !pageHasContent(page) {
		return false
	}
	return currentY+height > pageContentHeight(geom)
}

func pageContentHeight(geom PageGeometry) float64 {
	height := geom.Height - geom.MarginTop - geom.MarginBottom
	if height < 1 {
		return 1
	}
	return height
}

func sectionStartsNewPage(previous, next resolve.Section, previousGeom, nextGeom PageGeometry, current Page) bool {
	if !pageHasContent(current) {
		return false
	}
	breakKind := sectionBreakKind(next)
	if strings.EqualFold(breakKind, "continuous") && samePageFrameGeometry(previousGeom, nextGeom) {
		return false
	}
	if !sameGeometry(previousGeom, nextGeom) {
		return true
	}
	return !strings.EqualFold(breakKind, "continuous")
}

// sectionBreakKind returns the break that terminates a section. OOXML historically
// populated Section.Type while the legacy DOC parser uses BreakKind; layout must
// honor both representations. An omitted break is Word's default next-page break.
func sectionBreakKind(section resolve.Section) string {
	if section.BreakKind != "" {
		return section.BreakKind
	}
	if section.Type != "" {
		return section.Type
	}
	return "nextPage"
}

func samePageFrameGeometry(left, right PageGeometry) bool {
	return almostSame(left.Width, right.Width) &&
		almostSame(left.Height, right.Height)
}

func samePageContinuousGeometry(current, next PageGeometry) PageGeometry {
	merged := current
	merged.ColumnCount = next.ColumnCount
	merged.ColumnWidthPt = next.ColumnWidthPt
	merged.ColumnGapPt = next.ColumnGapPt
	merged.ColumnWidthsPt = append([]float64(nil), next.ColumnWidthsPt...)
	merged.ExplicitColumns = next.ExplicitColumns
	merged.ColumnGapsPt = append([]float64(nil), next.ColumnGapsPt...)
	merged.ColumnSep = next.ColumnSep
	merged.DocGridType = next.DocGridType
	merged.DocGridPitchPt = next.DocGridPitchPt
	return merged
}

func sameGeometry(left, right PageGeometry) bool {
	return almostSame(left.Width, right.Width) &&
		almostSame(left.Height, right.Height) &&
		almostSame(left.MarginTop, right.MarginTop) &&
		almostSame(left.MarginRight, right.MarginRight) &&
		almostSame(left.MarginBottom, right.MarginBottom) &&
		almostSame(left.MarginLeft, right.MarginLeft) &&
		left.ColumnCount == right.ColumnCount &&
		almostSame(left.ColumnWidthPt, right.ColumnWidthPt) &&
		almostSame(left.ColumnGapPt, right.ColumnGapPt) &&
		left.ExplicitColumns == right.ExplicitColumns &&
		floatSliceAlmostSame(left.ColumnWidthsPt, right.ColumnWidthsPt) &&
		floatSliceAlmostSame(left.ColumnGapsPt, right.ColumnGapsPt) &&
		left.ColumnSep == right.ColumnSep &&
		strings.EqualFold(left.DocGridType, right.DocGridType) &&
		almostSame(left.DocGridPitchPt, right.DocGridPitchPt)
}

func almostSame(left, right float64) bool {
	return math.Abs(left-right) < 0.001
}

func floatSliceAlmostSame(left, right []float64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !almostSame(left[i], right[i]) {
			return false
		}
	}
	return true
}

type explicitFlowBoundary uint8

const (
	explicitFlowBoundaryNone explicitFlowBoundary = iota
	explicitFlowBoundaryPageBefore
	explicitFlowBoundaryPageAfter
)

func blockExplicitFlowBoundary(block *resolve.Block) explicitFlowBoundary {
	if block == nil || block.Kind != "paragraph" || block.Paragraph == nil {
		return explicitFlowBoundaryNone
	}
	if block.Paragraph.Format.PageBreakBefore {
		return explicitFlowBoundaryPageBefore
	}
	for _, run := range block.Paragraph.Runs {
		if run.Kind == "break" && strings.EqualFold(run.BreakType, "page") {
			return explicitFlowBoundaryPageAfter
		}
	}
	return explicitFlowBoundaryNone
}

func blockForcesPageBreakBefore(block *resolve.Block) bool {
	return blockExplicitFlowBoundary(block) == explicitFlowBoundaryPageBefore
}

func blockForcesPageBreakAfter(block *resolve.Block) bool {
	return blockExplicitFlowBoundary(block) == explicitFlowBoundaryPageAfter
}

func paragraphHeightInFlow(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry, column int, paraPageY float64, activeFloats []FloatBox) float64 {
	baseWidthPt := paragraphContentWidthPt(p, geom)
	firstWidthPt, continuationWidthPt := paragraphLineWidthsPt(p, baseWidthPt)
	defaultSizePt := defaultFontSizePt(doc)
	textMetrics := resolveTextLayoutMetrics(doc, p, geom, 0, 0)
	lineHeight := textMetrics.LineHeightPt
	topBorderOutset, bottomBorderOutset := paragraphVerticalBorderOutsetPt(p)
	leadingBlockFloatHeight := paragraphLeadingBlockDrawingHeightPt(p, geom, column, paraPageY, lineHeight)
	activeFloatClearance := paragraphTopAndBottomClearancePt(paraPageY+leadingBlockFloatHeight, lineHeight, activeFloats)

	var spacingBefore, spacingAfter float64
	if paragraphContainsOnlyLeadingBlockDrawingForLayout(p) {
		return spacingBefore + topBorderOutset + leadingBlockFloatHeight + bottomBorderOutset + spacingAfter
	}

	contentLeftPt := paragraphContentLeftPt(p, columnLeftPt(geom, column))
	firstLeftPt := paragraphFirstLineLeftPt(p, contentLeftPt)

	initialMetrics := estimateParagraphLineMetricsWithWidths(p, firstWidthPt, continuationWidthPt, defaultSizePt, nil)
	initialHeights := paragraphLineHeightsPt(p, geom, initialMetrics, defaultSizePt)
	startY := paraPageY + spacingBefore + leadingBlockFloatHeight + activeFloatClearance

	_, _, lineHeights := convergedParagraphFlowPlan(
		p, geom,
		firstLeftPt, firstWidthPt,
		contentLeftPt, continuationWidthPt,
		startY, lineHeight, defaultSizePt,
		initialHeights,
		func(contentHeight float64) []lineExclusion {
			return paragraphWrapExclusions(
				p, geom, column, paraPageY,
				spacingBefore+contentHeight+spacingAfter,
				activeFloats,
			)
		},
		paragraphFlowContextForDocument(doc, columnLeftPt(geom, column)),
	)

	return spacingBefore + topBorderOutset + leadingBlockFloatHeight +
		activeFloatClearance + sumFloat64s(lineHeights) +
		bottomBorderOutset + spacingAfter
}

type paragraphFlowMetrics struct {
	lineHeights        []float64
	physicalLines      []paragraphPhysicalLine
	spacingBefore      float64
	spacingAfter       float64
	topBorderOutset    float64
	bottomBorderOutset float64
	topClearance       float64
	// flowOriginY is the page-absolute Y where the paragraph's own content
	// starts (after spacing and border, before any clearance) and
	// leadingBlockFloatHeight is the strip a leading top-and-bottom drawing
	// claims above the first text line. Renderers lay out that drawing as
	// in-flow block content, so such a fragment anchors to flowOriginY instead
	// of to the first text line, which would sit on top of the drawing.
	flowOriginY             float64
	leadingBlockFloatHeight float64
	// trailingLeading is the auto line-spacing extra Word places below each
	// line's text; the last line in a region may hang it past the bottom.
	trailingLeading float64
}

func (m paragraphFlowMetrics) totalHeight() float64 {
	return m.spacingBefore + m.topBorderOutset + m.topClearance + sumFloat64s(m.lineHeights) + m.bottomBorderOutset + m.spacingAfter
}

func (m paragraphFlowMetrics) fragmentFit(startLine int, availableHeight float64, widowControl bool) (int, float64) {
	if startLine < 0 || startLine >= len(m.lineHeights) || availableHeight <= 0 {
		return 0, 0
	}
	prefix := 0.0
	if startLine == 0 {
		prefix += m.spacingBefore + m.topBorderOutset + m.topClearance
	}
	bestLines := 0
	bestHeight := 0.0
	contentHeight := 0.0
	for endLine := startLine + 1; endLine <= len(m.lineHeights); endLine++ {
		contentHeight += m.lineHeights[endLine-1]
		candidateHeight := prefix + contentHeight
		textBottom := candidateHeight - math.Min(m.trailingLeading, m.lineHeights[endLine-1])
		if endLine == len(m.lineHeights) {
			// Word lets the final line's space-after run past the region bottom.
			if textBottom+m.bottomBorderOutset > availableHeight {
				break
			}
			candidateHeight += m.bottomBorderOutset + m.spacingAfter
		} else if textBottom > availableHeight {
			break
		}
		bestLines = endLine - startLine
		bestHeight = candidateHeight
	}
	if bestLines == 0 {
		return 0, 0
	}
	if !widowControl {
		return bestLines, bestHeight
	}
	remainingLines := len(m.lineHeights) - (startLine + bestLines)
	if remainingLines == 0 {
		// Widow control constrains a split boundary, not an already-started
		// continuation's final fragment. If earlier fragments already left one
		// physical line for the final region, refusing it here can advance
		// forever because the same one-line continuation is reconsidered on
		// every empty page/column.
		return bestLines, bestHeight
	}
	if bestLines < 2 {
		return 0, 0
	}
	if remainingLines < 2 {
		bestLines -= 2 - remainingLines
		if bestLines < 2 {
			return 0, 0
		}
		bestHeight = prefix + sumFloat64s(m.lineHeights[startLine:startLine+bestLines])
	}
	return bestLines, bestHeight
}

// singleLineFragmentHeight measures a fragment holding only line startLine,
// used when even that line exceeds an empty region.
func (m paragraphFlowMetrics) singleLineFragmentHeight(startLine int) float64 {
	height := m.lineHeights[startLine]
	if startLine == 0 {
		height += m.spacingBefore + m.topBorderOutset + m.topClearance
	}
	if startLine+1 == len(m.lineHeights) {
		height += m.bottomBorderOutset + m.spacingAfter
	}
	return height
}

func (m paragraphFlowMetrics) fragmentPhysicalLines(startLine, endLine int) []paragraphPhysicalLine {
	if startLine < 0 {
		startLine = 0
	}
	if endLine > len(m.physicalLines) {
		endLine = len(m.physicalLines)
	}
	if startLine >= endLine {
		return nil
	}
	return m.physicalLines[startLine:endLine]
}

func paragraphFlowMetricsForPagination(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry, column int, paraPageY float64, textMetrics TextLayoutMetrics, activeFloats []FloatBox) (paragraphFlowMetrics, bool) {
	if p == nil {
		return paragraphFlowMetrics{}, false
	}

	baseWidthPt := paragraphContentWidthPt(p, geom)
	firstWidthPt, continuationWidthPt := paragraphLineWidthsPt(p, baseWidthPt)
	defaultSizePt := defaultFontSizePt(doc)
	fontSizePt := paragraphFontSizePt(doc, p)
	lineHeight := paragraphLineHeightPt(p, geom, fontSizePt)
	topBorderOutset, bottomBorderOutset := paragraphVerticalBorderOutsetPt(p)

	contentLeftPt := paragraphContentLeftPt(p, columnLeftPt(geom, column))
	firstLeftPt := paragraphFirstLineLeftPt(p, contentLeftPt)
	// A top-and-bottom drawing that leads the paragraph is block material: its
	// height sits above the first line, exactly as paragraphHeightInFlow
	// reserves it. Without it here the fragment's lines would be planned over
	// the drawing while the block box still measured room for it.
	flowOriginY := paraPageY + textMetrics.SpacingBefore + topBorderOutset
	leadingBlockFloatHeight := paragraphLeadingBlockDrawingHeightPt(p, geom, column, paraPageY, lineHeight)
	lineStartY := flowOriginY + leadingBlockFloatHeight
	activeFloatClearance := paragraphTopAndBottomClearancePt(lineStartY, lineHeight, activeFloats)
	topClearance := leadingBlockFloatHeight + activeFloatClearance
	textStartY := lineStartY + activeFloatClearance

	initialMetrics := estimateParagraphLineMetricsWithWidths(p, firstWidthPt, continuationWidthPt, defaultSizePt, nil)
	initialHeights := paragraphLineHeightsPt(p, geom, initialMetrics, defaultSizePt)

	flow, _, lineHeights := convergedParagraphFlowPlan(
		p, geom,
		firstLeftPt, firstWidthPt,
		contentLeftPt, continuationWidthPt,
		textStartY, lineHeight, defaultSizePt,
		initialHeights,
		func(contentHeight float64) []lineExclusion {
			return paragraphWrapExclusions(
				p, geom, column, paraPageY,
				topClearance+contentHeight,
				activeFloats,
			)
		},
		paragraphFlowContextForDocument(doc, columnLeftPt(geom, column)),
	)

	if len(flow.PhysicalLines) != len(lineHeights) {
		return paragraphFlowMetrics{}, false
	}

	return paragraphFlowMetrics{
		lineHeights:        lineHeights,
		physicalLines:      flow.PhysicalLines,
		spacingBefore:      textMetrics.SpacingBefore,
		spacingAfter:       textMetrics.SpacingAfter,
		topBorderOutset:    topBorderOutset,
		bottomBorderOutset: bottomBorderOutset,
		topClearance:            topClearance,
		flowOriginY:             flowOriginY,
		leadingBlockFloatHeight: leadingBlockFloatHeight,
		trailingLeading:         autoParagraphTrailingLeadingPt(p, geom, fontSizePt),
	}, true
}

func paragraphHasFloatingContent(p *resolve.Paragraph) bool {
	return paragraphHasFloatingDrawing(p)
}

func resolvedParagraphSpacingBeforePt(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry) float64 {
	if p == nil || p.Format.SpacingBeforeAuto {
		return 0
	}
	lineHeight := paragraphLineHeightPt(p, geom, paragraphFontSizePt(doc, p))
	if p.Format.SpacingBeforeLines != nil && *p.Format.SpacingBeforeLines > 0 {
		return lineHeight * float64(*p.Format.SpacingBeforeLines) / 100.0
	}
	if p.Format.SpacingBefore != nil && *p.Format.SpacingBefore > 0 {
		return twipsToPoints(*p.Format.SpacingBefore)
	}
	return 0
}

func resolvedParagraphSpacingAfterPt(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry) float64 {
	if p == nil || p.Format.SpacingAfterAuto {
		return 0
	}
	lineHeight := paragraphLineHeightPt(p, geom, paragraphFontSizePt(doc, p))
	if p.Format.SpacingAfterLines != nil && *p.Format.SpacingAfterLines > 0 {
		return lineHeight * float64(*p.Format.SpacingAfterLines) / 100.0
	}
	if p.Format.SpacingAfter != nil && *p.Format.SpacingAfter > 0 {
		return twipsToPoints(*p.Format.SpacingAfter)
	}
	return 0
}

// ParagraphSpacingBefore returns only the additional space-before contribution
// that is not already owned by the preceding paragraph's space-after. Word does
// not add adjacent paragraph spacing values: the effective inter-paragraph gap is
// the larger of previous space-after and current space-before. Because the previous
// block already owns its space-after, this block contributes only the remainder.
// flowStartSuppressesSpacing reports whether a block measured at flow position
// y sits at the start of a flow region, where Word discards space-before.
// The space survives only for a paragraph that carries w:pageBreakBefore
// itself; a page that starts because the previous paragraph ended with a
// page-break run begins flush with the top margin, exactly like a page that
// starts because the previous content ran out.
func flowStartSuppressesSpacing(blocks []resolve.Block, index int, y float64) bool {
	if y != 0 {
		return false
	}
	if index < 0 || index >= len(blocks) {
		return true
	}
	return !blockForcesPageBreakBefore(&blocks[index])
}

func ParagraphSpacingBefore(doc *resolve.Document, p, prev *resolve.Paragraph, geom PageGeometry, atFlowStart bool) float64 {
	if p == nil || atFlowStart {
		return 0
	}
	if ContextualSpacingSuppresses(p, prev) {
		return 0
	}
	before := resolvedParagraphSpacingBeforePt(doc, p, geom)
	if prev == nil {
		return before
	}
	previousAfter := resolvedParagraphSpacingAfterPt(doc, prev, geom)
	if previousAfter >= before {
		return 0
	}
	return before - previousAfter
}

// ParagraphSpacingAfter returns the space-after contribution of p.
func ParagraphSpacingAfter(doc *resolve.Document, p, next *resolve.Paragraph, geom PageGeometry) float64 {
	if p == nil {
		return 0
	}
	if ContextualSpacingSuppresses(p, next) {
		return 0
	}
	return resolvedParagraphSpacingAfterPt(doc, p, geom)
}

func paragraphHeightInWidth(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry, widthPt float64) float64 {
	metrics, lineHeights, topBorderOutset, bottomBorderOutset := paragraphIntrinsicLineHeights(doc, p, geom, widthPt)
	return paragraphIntrinsicHeightFromMetrics(metrics, lineHeights, topBorderOutset, bottomBorderOutset)
}

// ParagraphHeightInWidthForRender exposes the paragraph content-height estimator
// to renderers that need local story-frame sizing. Spacing is not included;
// callers add ParagraphSpacingBefore/After separately.
func ParagraphHeightInWidthForRender(doc *resolve.Document, p *resolve.Paragraph, widthPt float64) float64 {
	return paragraphHeightInWidth(doc, p, PageGeometry{}, widthPt)
}

func paragraphVerticalBorderOutsetPt(p *resolve.Paragraph) (float64, float64) {
	if p == nil || p.Format.Borders == nil {
		return 0, 0
	}
	return paragraphBorderOutsetPt(p.Format.Borders.Top), paragraphBorderOutsetPt(p.Format.Borders.Bottom)
}

func paragraphHorizontalBorderOutsetPt(p *resolve.Paragraph) (float64, float64) {
	if p == nil || p.Format.Borders == nil {
		return 0, 0
	}
	return paragraphBorderOutsetPt(p.Format.Borders.Left), paragraphBorderOutsetPt(p.Format.Borders.Right)
}

func paragraphBorderOutsetPt(side *resolve.BorderSide) float64 {
	if side == nil {
		return 0
	}
	outset := 0.0
	if side.SpacePt > 0 {
		outset += side.SpacePt
	}
	if side.SizePt > 0 {
		outset += side.SizePt
	}
	return outset
}

// tableHeight returns a conservative height estimate for a table.
// Rows with explicit exact height use that value; other rows expand to fit the
// tallest estimated cell content.
func leadingTableHeaderRows(t *resolve.Table) int {
	if t == nil {
		return 0
	}
	end := 0
	for end < len(t.Rows) && t.Rows[end].Format.IsHeader {
		end++
	}
	return end
}

func tableFragmentRepeatsHeaders(startRow, headerEnd int) bool {
	return headerEnd > 0 && startRow >= headerEnd && startRow > 0
}

// tableSplitBoundarySafe reports whether a table fragment may end immediately
// before row endRow. A w:vMerge continuation in the next row means the merge
// started in an earlier row and therefore crosses this boundary. Keeping such
// groups together preserves valid rowspan geometry in every rendered fragment.
func tableFragmentBoundarySafe(t *resolve.Table, endRow int) bool {
	if t == nil || endRow <= 0 || endRow >= len(t.Rows) {
		return true
	}
	for _, cell := range t.Rows[endRow].Cells {
		if strings.EqualFold(strings.TrimSpace(cell.Format.VMerge), "continue") {
			return false
		}
	}
	return true
}

// tableSplitBoundarySafe retains the established compatibility surface.
func tableSplitBoundarySafe(t *resolve.Table, endRow int) bool {
	return tableFragmentBoundarySafe(t, endRow)
}

func tableHeight(doc *resolve.Document, t *resolve.Table, availableWidthPt float64) float64 {
	if t == nil {
		return 0
	}
	total := 0.0
	for _, rowHeight := range TableRowHeights(doc, t, availableWidthPt) {
		total += rowHeight
	}
	return total
}

// TableHeightForRender exposes table height estimation to renderers that need
// local story-frame sizing.
func TableHeightForRender(doc *resolve.Document, t *resolve.Table, availableWidthPt float64) float64 {
	return tableHeight(doc, t, availableWidthPt)
}

// TableWidthPt resolves the rendered width of a table in points.
func TableWidthPt(t *resolve.Table, availableWidthPt float64) float64 {
	if t == nil {
		return 0
	}
	return resolveTableWidthPt(t.Format, availableWidthPt)
}

// TableWidthPtForDocument resolves table width with source-format compatibility.
// Word 6/95 TAP records commonly expose absolute cell boundaries wider than the
// section text frame. Word clips/scales those old fixed tables to the printable
// text frame; allowing the raw width to escape the frame makes HTML both overflow
// horizontally and paginate too late because rows are measured too wide.
func TableWidthPtForDocument(doc *resolve.Document, t *resolve.Table, availableWidthPt float64) float64 {
	if t == nil {
		return 0
	}
	// An auto-width table is laid out inside the remaining containing block.
	// A positive table indent consumes horizontal room; treating the full cell
	// width as available makes nested tables too wide and therefore their rows
	// too short. Explicit dxa/pct widths and grid-derived widths remain
	// authoritative and are not silently rewritten.
	widthAvailable := availableWidthPt
	if t.Format.Width == nil && len(t.Format.ColumnWidths) == 0 && t.Format.Indent > 0 {
		widthAvailable -= twipsToPoints(t.Format.Indent)
		if widthAvailable < 1 {
			widthAvailable = 1
		}
	}
	width := TableWidthPt(t, widthAvailable)
	if isPreWord97Document(doc) && availableWidthPt > 0 && width > availableWidthPt {
		return availableWidthPt
	}
	return width
}

func isPreWord97Document(doc *resolve.Document) bool {
	return doc != nil && doc.Source != nil && doc.Source.Version != 0 && doc.Source.Version < 0x00C1
}

// TableRowHeights resolves the rendered height of each table row in points.
func TableRowHeights(doc *resolve.Document, t *resolve.Table, availableWidthPt float64) []float64 {
	if t == nil {
		return nil
	}
	tableWidthPt := TableWidthPtForDocument(doc, t, availableWidthPt)
	heights := make([]float64, 0, len(t.Rows))
	for i, row := range t.Rows {
		heights = append(heights, tableRowHeight(doc, row, t.Format, tableWidthPt, i == 0, i == len(t.Rows)-1))
	}
	applyVerticalMergeGroupMinimums(doc, t, tableWidthPt, heights)
	return heights
}

// applyVerticalMergeGroupMinimums treats a vertical merge as a row-group
// constraint: the merged cell's intrinsic height belongs to the complete
// restart..continue group, not solely to the restart row. This keeps ordinary
// sibling cells authoritative for individual row heights while guaranteeing
// enough aggregate height for the rowspan content.
func applyVerticalMergeGroupMinimums(doc *resolve.Document, t *resolve.Table, tableWidthPt float64, heights []float64) {
	if t == nil || len(heights) != len(t.Rows) {
		return
	}
	for rowIndex, row := range t.Rows {
		gridColumn := row.Format.GridBefore
		cellWidths := TableCellWidths(row, t.Format, tableWidthPt)
		for cellIndex, cell := range row.Cells {
			span := cell.Format.GridSpan
			if span <= 0 {
				span = 1
			}
			if !strings.EqualFold(strings.TrimSpace(cell.Format.VMerge), "restart") {
				gridColumn += span
				continue
			}

			endRow := verticalMergeGroupEnd(t, rowIndex, gridColumn)
			if endRow <= rowIndex+1 {
				gridColumn += span
				continue
			}
			cellWidth := tableWidthPt
			if cellIndex < len(cellWidths) && cellWidths[cellIndex] > 0 {
				cellWidth = cellWidths[cellIndex]
			}
			required := tableCellHeightWithRowContext(doc, cell, t.Format, row.Override, row.Format.CellSpacing, cellWidth, rowIndex == 0, endRow == len(t.Rows))
			current := sumFloat64s(heights[rowIndex:endRow])
			if deficit := required - current; deficit > 0 {
				// Word lets the merge group grow as a unit. Put the residual on
				// the final row so preceding explicit/sibling-driven row heights
				// remain stable and the split boundary stays deterministic.
				heights[endRow-1] += deficit
			}
			gridColumn += span
		}
	}
}

func verticalMergeGroupEnd(t *resolve.Table, startRow, gridColumn int) int {
	if t == nil || startRow < 0 || startRow >= len(t.Rows) {
		return startRow
	}
	end := startRow + 1
	for end < len(t.Rows) {
		cell, ok := tableCellCoveringGridColumn(t.Rows[end], gridColumn)
		if !ok || !strings.EqualFold(strings.TrimSpace(cell.Format.VMerge), "continue") {
			break
		}
		end++
	}
	return end
}

func tableCellCoveringGridColumn(row resolve.TableRow, gridColumn int) (resolve.TableCell, bool) {
	column := row.Format.GridBefore
	for _, cell := range row.Cells {
		span := cell.Format.GridSpan
		if span <= 0 {
			span = 1
		}
		if gridColumn >= column && gridColumn < column+span {
			return cell, true
		}
		column += span
	}
	return resolve.TableCell{}, false
}

// appendParagraphFloats collects every floating object from runs and appends
// a FloatBox with its absolute page position to floats.
//
// paraPageY is the paragraph's top edge in page-absolute coordinates
// (i.e., already includes the page margin-top).
func appendParagraphFloats(floats []FloatBox, runs []resolve.Run, geom PageGeometry, column int, paraPageY, paraHeight float64) []FloatBox {
	return appendParagraphFloatsWithLegacyFlowState(floats, runs, geom, column, paraPageY, paraHeight, nil)
}

func appendParagraphFloatsWithLegacyFlowState(floats []FloatBox, runs []resolve.Run, geom PageGeometry, column int, paraPageY, paraHeight float64, sharedLegacyFlow *legacyShapeFlowState) []FloatBox {
	legacyFlow := sharedLegacyFlow
	if legacyFlow == nil {
		legacyFlow = &legacyShapeFlowState{
			left: columnLeftPt(geom, column),
			// Word's legacy SHAPE-field packer measures its usable row from the
			// page's left margin, rather than using the complete column width.
			// This oddity is observable in Word 2021's compatibility layout.
			width: columnWidthPt(geom, column) - geom.MarginLeft,
			y:     legacyShapeFlowInitialY(runs, paraPageY, paraHeight),
		}
		if legacyFlow.width <= 0 {
			legacyFlow.width = geom.Width - 2*geom.MarginLeft - geom.MarginRight
		}
	}
	legacyFlowPositions := legacyShapeFlowPositions(runs, legacyFlow)
	for i := range runs {
		run := &runs[i]
		drawing := drawingFromRun(run)
		if drawing == nil {
			continue
		}

		fl, ok := resolvedDrawingFloatFromRun(run, geom, column, paraPageY, paraHeight)
		if !ok {
			continue
		}

		// Pre-97 OfficeArt SHAPE fields have a sequential anchor-line flow
		// that is not an OOXML DrawingPlacement policy. Keep that source-format
		// compatibility at this boundary, after canonical size/z-order resolution.
		if shape, ok := drawing.(*resolve.Shape); ok && shape.LegacyFlowAnchor {
			if pos, found := legacyFlowPositions[shape]; found {
				fl.X, fl.Y = pos.X, pos.Y
			} else {
				fl.X, fl.Y = legacyFlow.place(shape, fl.Width, fl.Height)
			}
		}
		floats = append(floats, fl)
	}
	return floats
}

func paragraphHasLegacyShapeFlow(p *resolve.Paragraph) bool {
	if p == nil {
		return false
	}
	for i := range p.Runs {
		run := &p.Runs[i]
		if run.Kind == "shape" && run.Shape != nil && run.Shape.Floating && run.Shape.LegacyFlowAnchor {
			return true
		}
	}
	return false
}

type legacyShapeFlowPosition struct {
	X float64
	Y float64
}

type legacyShapeFlowItem struct {
	shape    *resolve.Shape
	widthPt  float64
	heightPt float64
	order    int
	runIndex int
}

// legacyShapeFlowPositions separates story order from paint order. OfficeArt
// z-order controls which shape is painted on top; FSPA CP controls where each
// consecutive Word SHAPE field participates in anchor-line flow.
func legacyShapeFlowPositions(runs []resolve.Run, state *legacyShapeFlowState) map[*resolve.Shape]legacyShapeFlowPosition {
	items := make([]legacyShapeFlowItem, 0)
	for i := range runs {
		sh := runs[i].Shape
		if runs[i].Kind != "shape" || sh == nil || !sh.Floating || !sh.LegacyFlowAnchor {
			continue
		}
		items = append(items, legacyShapeFlowItem{
			shape:    sh,
			widthPt:  emuToPoints(sh.Width),
			heightPt: emuToPoints(sh.Height),
			order:    sh.LegacyFlowOrder,
			runIndex: i,
		})
	}
	if len(items) == 0 {
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].order <= 0 || items[j].order <= 0 || items[i].order == items[j].order {
			return items[i].runIndex < items[j].runIndex
		}
		return items[i].order < items[j].order
	})
	out := make(map[*resolve.Shape]legacyShapeFlowPosition, len(items))
	for _, item := range items {
		x, y := state.place(item.shape, item.widthPt, item.heightPt)
		out[item.shape] = legacyShapeFlowPosition{X: x, Y: y}
	}
	return out
}

type legacyShapeFlowState struct {
	left        float64
	width       float64
	cursor      float64
	y           float64
	rowH        float64
	nextGap     float64
	rowBreakGap float64
}

const legacyShapeFlowGapPt = 3

// legacyShapeFlowAnchorLineInsetPt is the Word 97 anchor-character line
// inset. A SHAPE field is anchored to the anchor character's baseline, which
// sits one descender above the anchor line's bottom — not at the paragraph
// box bottom the browser fallback would use. The value is measured from the
// Word 2021 PDFs of the legacy OfficeArt fixtures.
const legacyShapeFlowAnchorLineInsetPt = 2.5

func legacyShapeFlowAnchorY(paraPageY, paraHeight float64) float64 {
	return paraPageY + math.Max(0, paraHeight-legacyShapeFlowAnchorLineInsetPt)
}

// legacyShapeFlowInitialY chooses the anchor-line origin for Word binary SHAPE
// fields. Ordinary legacy shapes use the historical anchor-character line
// position. An OfficeArt group is different: its FSPGR members already carry a
// complete local coordinate domain and Word aligns that enclosing group box to
// the top of the anchor paragraph. Applying the ordinary line-bottom inset to a
// group shifts every member downward (and, before group-domain normalization,
// can send negative children above the page).
func legacyShapeFlowInitialY(runs []resolve.Run, paraPageY, paraHeight float64) float64 {
	for i := range runs {
		shape := runs[i].Shape
		if runs[i].Kind != "shape" || shape == nil || !shape.Floating || !shape.LegacyFlowAnchor {
			continue
		}
		if shape.Kind == "group" {
			return paraPageY
		}
		break
	}
	return legacyShapeFlowAnchorY(paraPageY, paraHeight)
}

func (s *legacyShapeFlowState) place(shape *resolve.Shape, widthPt, heightPt float64) (float64, float64) {
	if s == nil || shape == nil {
		return 0, 0
	}
	localX := emuToPoints(shape.X)
	localY := emuToPoints(shape.Y)
	shapeAdvance := legacyShapeFlowAdvance(shape, widthPt, heightPt, localX)
	gap := 0.0
	if s.cursor > 0 {
		gap = s.nextGap
		if gap <= 0 {
			gap = legacyShapeFlowGapPt
		}
	}
	advanceX := gap + shapeAdvance

	// FSPA local Y is a displacement within the current anchor row. It does
	// not start a row by itself; only horizontal exhaustion does.
	if s.cursor > 0 && s.width > 0 && s.cursor+advanceX > s.width {
		s.cursor = 0
		rowGap := s.rowBreakGap
		if rowGap <= 0 {
			rowGap = legacyShapeFlowGapPt
		}
		s.y += s.rowH + rowGap
		s.rowH = 0
		s.rowBreakGap = 0
		gap = 0
		advanceX = shapeAdvance
	}

	x := s.left + s.cursor + gap + localX
	y := s.y + localY

	// A later anchor row must start below the visible outline. This matters for
	// rotated OfficeArt rectangles: their visual bottom can extend well below
	// the unrotated FSPA rectangle and otherwise overlaps the next row.
	rowBottom := localY + heightPt
	if shape != nil && shape.RotationDeg != 0 {
		_, _, _, bottom := transformedDrawingBounds(FloatBox{Width: widthPt, Height: heightPt}, shape.RotationDeg)
		rowBottom = math.Max(rowBottom, localY+bottom)
	}
	if rowBottom > s.rowH {
		s.rowH = rowBottom
	}
	// Word keeps a displaced FSPA rectangle's local offset on both sides of
	// that shape: after it in the row, and before the following anchor row.
	s.nextGap = legacyShapeFlowGapPt + 2*math.Max(0, localX)
	s.rowBreakGap = math.Max(s.rowBreakGap, legacyShapeFlowGapPt+2*math.Max(0, localY))
	s.cursor += advanceX
	return x, y
}

// legacyShapeFlowAdvance reserves the visual right edge of a rotated legacy
// shape, so it cannot overlap the following shape in the same row.
func legacyShapeFlowAdvance(shape *resolve.Shape, widthPt, heightPt, localX float64) float64 {
	advance := widthPt
	if shape != nil && shape.RotationDeg != 0 {
		_, _, right, _ := transformedDrawingBounds(FloatBox{Width: widthPt, Height: heightPt}, shape.RotationDeg)
		advance = math.Max(advance, localX+right)
	} else if localX > 0 {
		advance += localX
	}
	return advance
}

// relativeObjectSizePt resolves wp14:sizeRelH/wp14:sizeRelV against the
// authored reference box. pct is in thousandths of a percent (50000 = 50%).
// Relative sizing is authoritative when present; wp:extent is only the fallback.
func relativeObjectSizePt(pct int64, relativeFrom string, isWidth bool, geom PageGeometry, column int) float64 {
	if pct <= 0 {
		return 0
	}
	fraction := float64(pct) / 100000.0
	var reference float64
	if isWidth {
		switch strings.ToLower(strings.TrimSpace(relativeFrom)) {
		case "page":
			reference = geom.Width
		case "leftmargin", "insidemargin":
			reference = geom.MarginLeft
		case "rightmargin", "outsidemargin":
			reference = geom.MarginRight
		case "column":
			reference = columnWidthPt(geom, column)
		default: // margin and unknown values use the printable text width.
			reference = geom.Width - geom.MarginLeft - geom.MarginRight
		}
	} else {
		switch strings.ToLower(strings.TrimSpace(relativeFrom)) {
		case "page":
			reference = geom.Height
		case "topmargin", "insidemargin":
			reference = geom.MarginTop
		case "bottommargin", "outsidemargin":
			reference = geom.MarginBottom
		default: // margin/column use the printable text height.
			reference = geom.Height - geom.MarginTop - geom.MarginBottom
		}
	}
	if reference < 0 {
		reference = 0
	}
	return reference * fraction
}

// resolveX converts a floating object's EMU X offset and XRelative reference
// into an absolute point coordinate from the page left edge.
//
// Supported relativeFrom values (OOXML §20.4.3.4 ST_RelFromH):
//   - "page"         — offset from the physical page left edge
//   - "column"       — offset from the left edge of the current column
//   - "margin"       — offset from the left margin (text area left edge)
//   - "leftMargin"   — synonym for "margin"; offset from the left margin edge
//   - "rightMargin"  — offset from the right margin edge (page right boundary of body text)
//   - "paragraph"    — treated as "margin" (inline-container approximation)
//   - "character"    — treated as "margin" (exact inline position not tracked)
//   - "insideMargin" — inside margin: approximated as left margin
//   - "outsideMargin"— outside margin: approximated as right margin
func resolveX(xEMU int64, relative, align string, geom PageGeometry, column int, widthPt float64) float64 {
	if align != "" {
		return resolveXAlign(align, relative, geom, column, widthPt)
	}
	offsetPt := emuToPoints(xEMU)
	switch relative {
	case "page":
		return offsetPt
	case "column":
		return columnLeftPt(geom, column) + offsetPt
	case "rightMargin", "outsideMargin":
		return geom.Width - geom.MarginRight + offsetPt
	default: // "margin", "leftMargin", "paragraph", "character", "insideMargin", ""
		return geom.MarginLeft + offsetPt
	}
}

// resolveY converts a floating object's EMU Y offset and YRelative reference
// into an absolute point coordinate from the page top edge.
//
// paraPageY is the paragraph's top edge in page-absolute coordinates.
//
// Supported relativeFrom values (OOXML §20.4.3.5 ST_RelFromV):
//   - "page"          — offset from the physical page top edge
//   - "margin"        — offset from the top margin (text area top edge)
//   - "topMargin"     — synonym for "margin"; offset from the top margin edge
//   - "bottomMargin"  — offset from the bottom margin edge (body-text bottom boundary)
//   - "paragraph"     — offset from the top of the anchor paragraph
//   - "line"          — offset from the top of the current text line; approximated as "paragraph"
//   - "insideMargin"  — inside page margin; approximated as top margin
//   - "outsideMargin" — outside page margin; approximated as bottom margin
func resolveY(yEMU int64, relative, align string, geom PageGeometry, paraPageY, paraHeight, heightPt float64) float64 {
	if align != "" {
		return resolveYAlign(align, relative, geom, paraPageY, paraHeight, heightPt)
	}
	offsetPt := emuToPoints(yEMU)
	switch relative {
	case "page":
		return offsetPt
	case "margin", "topMargin", "insideMargin":
		return geom.MarginTop + offsetPt
	case "paragraph", "line":
		return paraPageY + offsetPt
	case "bottomMargin", "outsideMargin":
		return geom.Height - geom.MarginBottom + offsetPt
	default:
		return geom.MarginTop + offsetPt
	}
}

func resolveXAlign(align, relative string, geom PageGeometry, column int, widthPt float64) float64 {
	var refLeft, refWidth float64
	switch relative {
	case "page":
		refLeft = 0
		refWidth = geom.Width
	case "column":
		refLeft = columnLeftPt(geom, column)
		refWidth = columnWidthPt(geom, column)
		if refWidth <= 0 {
			refWidth = geom.Width - geom.MarginLeft - geom.MarginRight
		}
	case "rightMargin", "outsideMargin":
		refLeft = geom.Width - geom.MarginRight
		refWidth = geom.MarginRight
	default: // "margin", "leftMargin", "paragraph", "character", "insideMargin", ""
		refLeft = geom.MarginLeft
		refWidth = geom.Width - geom.MarginLeft - geom.MarginRight
	}

	switch align {
	case "left":
		return refLeft
	case "right":
		return refLeft + refWidth - widthPt
	case "center":
		return refLeft + (refWidth-widthPt)/2
	default:
		return refLeft
	}
}

func resolveYAlign(align, relative string, geom PageGeometry, paraPageY, paraHeight, heightPt float64) float64 {
	contentH := geom.Height - geom.MarginTop - geom.MarginBottom
	switch align {
	case "top":
		switch relative {
		case "page":
			return 0
		case "paragraph", "line":
			return paraPageY
		case "bottomMargin", "outsideMargin":
			return geom.Height - geom.MarginBottom
		default:
			return geom.MarginTop
		}
	case "bottom":
		switch relative {
		case "page":
			return geom.Height - heightPt
		case "paragraph", "line":
			return paraPageY + paraHeight - heightPt
		case "bottomMargin", "outsideMargin":
			return geom.Height - heightPt
		default:
			return geom.MarginTop + contentH - heightPt
		}
	case "center":
		switch relative {
		case "page":
			return (geom.Height - heightPt) / 2
		case "paragraph", "line":
			return paraPageY + (paraHeight-heightPt)/2
		case "bottomMargin", "outsideMargin":
			return geom.Height - geom.MarginBottom - heightPt/2
		default:
			return geom.MarginTop + (contentH-heightPt)/2
		}
	default:
		if relative == "page" {
			return 0
		}
		if relative == "paragraph" || relative == "line" {
			return paraPageY
		}
		if relative == "bottomMargin" || relative == "outsideMargin" {
			return geom.Height - geom.MarginBottom
		}
		return geom.MarginTop
	}
}

// ContextualSpacingSuppresses returns true when contextualSpacing is active on
// either paragraph and the neighbor has the same style — matching Word's
// behavior of suppressing spacing between same-style adjacent paragraphs.
func ContextualSpacingSuppresses(p *resolve.Paragraph, neighbor *resolve.Paragraph) bool {
	if p == nil || neighbor == nil {
		return false
	}
	if !p.Format.ContextualSpacing && !neighbor.Format.ContextualSpacing {
		return false
	}
	return p.StyleID == neighbor.StyleID
}

// adjacentBlock returns the Paragraph of the block at index i+delta, or nil.
func adjacentBlock(blocks []resolve.Block, i, delta int) *resolve.Paragraph {
	j := i + delta
	if j < 0 || j >= len(blocks) {
		return nil
	}
	return blocks[j].Paragraph
}

// defaultFontSizePt returns the document-default font size in points.
func defaultFontSizePt(doc *resolve.Document) float64 {
	if doc != nil && doc.DefaultRunFormat.FontSize != nil {
		return float64(*doc.DefaultRunFormat.FontSize) / 2.0
	}
	return 12.0 // Word default
}

// paragraphFontSizePt returns the maximum font size across all runs in the
// paragraph, falling back to the document default.
//
// Fix 2: Word uses the tallest font in the line to set the line-height, so
// the first-run approximation under-estimates height for mixed-size paragraphs.
func paragraphFontSizePt(doc *resolve.Document, p *resolve.Paragraph) float64 {
	maxSize := 0.0
	if p != nil {
		for _, run := range p.Runs {
			if run.Format.Hidden() {
				continue
			}
			if run.Format.FontSize != nil && *run.Format.FontSize > 0 {
				if size := float64(*run.Format.FontSize) / 2.0; size > maxSize {
					maxSize = size
				}
			}
		}
	}
	if maxSize > 0 {
		return maxSize
	}
	return defaultFontSizePt(doc)
}

// paragraphLineHeightRatio is the single-spacing line advance multiple for a
// paragraph's font family. Word measures a line from the font's own vertical
// metrics, so Calibri text is taller than Times New Roman at the same point
// size; the bundled metric tables carry those ratios. Families without bundled
// metrics keep the generic fallback.
func paragraphLineHeightRatio(p *resolve.Paragraph) float64 {
	if family := paragraphFontFamilyName(p); family != "" {
		if ratio := sharedfontmetrics.LineHeightRatioFor(family); ratio > 0 {
			return ratio
		}
	}
	return wordSingleLineRatio
}

// paragraphFontFamilyName returns the family the paragraph's text is set in,
// which is the first run that carries text and names a font. Without such a
// run the paragraph mark owns the line, so its family applies.
func paragraphFontFamilyName(p *resolve.Paragraph) string {
	if p == nil {
		return ""
	}
	for i := range p.Runs {
		run := &p.Runs[i]
		// Hidden text is not displayed, so it cannot own the line's font; the
		// paragraph mark's format applies instead.
		if run.Text == "" || run.Format.Hidden() {
			continue
		}
		if run.Format.FontASCII != "" {
			return run.Format.FontASCII
		}
		if run.Format.FontHAnsi != "" {
			return run.Format.FontHAnsi
		}
	}
	if mark := p.DefaultRunFormat; mark != nil {
		if mark.FontASCII != "" {
			return mark.FontASCII
		}
		return mark.FontHAnsi
	}
	return ""
}

func paragraphLineHeightPt(p *resolve.Paragraph, geom PageGeometry, fontSizePt float64) float64 {

	inlineHeightPt := paragraphInlineImageHeightPt(p)
	lineHeight := math.Max(fontSizePt*paragraphLineHeightRatio(p), inlineHeightPt)
	if p == nil || p.Format.LineSpacing == nil {
		return snapLineHeightToDocGrid(p, geom, lineHeight)
	}
	switch p.Format.LineRule {
	case "auto", "":
		lineHeight = autoParagraphLineHeightWithInlinePt(p, fontSizePt, inlineHeightPt)
	case "exact":
		lineHeight = twipsToPoints(*p.Format.LineSpacing)
	case "atLeast":
		lineHeight = math.Max(lineHeight, twipsToPoints(*p.Format.LineSpacing))
	}
	return snapLineHeightToDocGrid(p, geom, lineHeight)
}

func autoParagraphLineHeightPt(p *resolve.Paragraph, fontSizePt float64) float64 {
	baseLineHeight := fontSizePt * paragraphLineHeightRatio(p)
	if p == nil || p.Format.LineSpacing == nil || *p.Format.LineSpacing <= 0 {
		return baseLineHeight
	}
	multiple := float64(*p.Format.LineSpacing) / 240.0
	// Word stores auto line spacing as a line multiple for both OOXML and
	// binary DOC. 240=single, 360=1.5 lines, 480=double. The multiple is
	// authored relative to Word's normal line advance, represented here by the
	// CSS-normal approximation.
	return baseLineHeight * multiple
}

// autoParagraphLineHeightWithInlinePt sizes an auto-spaced line that may hold an
// inline object. Word scales only the font's single line by the multiple, so
// an object line keeps the object's height plus the multiple's extra leading
// rather than scaling the object itself.
func autoParagraphLineHeightWithInlinePt(p *resolve.Paragraph, fontSizePt, inlineHeightPt float64) float64 {
	textHeight := autoParagraphLineHeightPt(p, fontSizePt)
	if inlineHeightPt <= 0 {
		return textHeight
	}
	return math.Max(textHeight, inlineHeightPt+autoParagraphExtraLeadingPt(p, fontSizePt))
}

// autoParagraphExtraLeadingPt is what an auto line-spacing multiple adds to the
// font's single line.
func autoParagraphExtraLeadingPt(p *resolve.Paragraph, fontSizePt float64) float64 {
	return math.Max(0, autoParagraphLineHeightPt(p, fontSizePt)-fontSizePt*paragraphLineHeightRatio(p))
}

// autoParagraphTrailingLeadingPt is the extra leading Word paints below each
// auto-spaced line's text. Grid-snapped lines own their whole pitch instead.
func autoParagraphTrailingLeadingPt(p *resolve.Paragraph, geom PageGeometry, fontSizePt float64) float64 {
	if p == nil || p.Format.LineSpacing == nil || (p.Format.LineRule != "auto" && p.Format.LineRule != "") {
		return 0
	}
	if p.Format.SnapToGrid && geom.DocGridPitchPt > 0 &&
		(strings.EqualFold(geom.DocGridType, "lines") || strings.EqualFold(geom.DocGridType, "linesAndChars")) {
		return 0
	}
	return autoParagraphExtraLeadingPt(p, fontSizePt)
}

func paragraphInlineImageHeightPt(p *resolve.Paragraph) float64 {
	maxHeight := 0.0
	if p == nil {
		return maxHeight
	}
	for _, run := range p.Runs {
		if run.Format.Hidden() {
			continue
		}
		if run.Kind != "image" || run.Image == nil || run.Image.Floating {
			continue
		}
		if heightPt := inlineImageBoxHeightPt(run.Image); heightPt > maxHeight {
			maxHeight = heightPt
		}
	}
	return maxHeight
}

//
// Fix 1: instead of a single 0.5× character width factor, this function
// iterates over runs and uses per-character Unicode width factors and per-run
// font sizes, producing a much better estimate for mixed-script and mixed-size
// paragraphs.

type paragraphRenderToken struct {
	run            resolve.Run
	widthPt        float64
	heightPt       float64
	fontSizePt     float64
	forceLineBreak bool
	tab            bool
	advance        bool
	advanceOffsets inlineposition.AdvanceOffsets
	// separated records whether Word source text contains collapsible
	// whitespace before this token. The legacy rectangular estimator leaves
	// this false and keeps its historical implicit-space behavior; spatial
	// physical flow uses it to avoid inventing spaces across run boundaries.
	separated bool
}

func fieldRenderWidthText(run resolve.Run) string {
	if run.FieldKind == "" {
		if len(run.Fields) == 0 {
			return ""
		}
		var b strings.Builder
		for _, field := range run.Fields {
			text := fieldRenderWidthText(resolve.Run{
				FieldKind:      field.Kind,
				FieldInstr:     field.Instr,
				FieldArg:       field.Arg,
				FieldInstrRuns: field.InstrRuns,
				Anchor:         field.Anchor,
				Target:         field.Target,
				Text:           paragraphTextFromRuns(field.Children),
			})
			if text == "" {
				text = paragraphTextFromRuns(field.Children)
			}
			b.WriteString(text)
		}
		return b.String()
	}
	if strings.TrimSpace(run.Text) != "" {
		return run.Text
	}
	switch run.FieldKind {
	case "PAGE", "NUMPAGES", "SECTION", "SECTIONPAGES", "MERGEREC", "MERGESEQ", "AUTONUM":
		return "1"
	case "AUTONUMLGL":
		return "1."
	case "AUTONUMOUT":
		return "I."
	case "DATE", "TIME", "CREATEDATE", "SAVEDATE", "PRINTDATE":
		return "00/00/0000"
	case "ASK", "BIDIOUTLINE", "NEXT", "NEXTIF", "RD", "SET", "SKIPIF", "TA":
		return ""
	case "AUTOTEXT", "INCLUDETEXT":
		if run.FieldArg != "" {
			return "[" + strings.TrimSpace(run.FieldArg) + "]"
		}
		return "[Include Text]"
	case "INCLUDEPICTURE":
		if run.FieldArg != "" {
			return "[" + strings.TrimSpace(run.FieldArg) + "]"
		}
		return "[Include Picture]"
	case "TOA":
		return "[Table of Authorities]"
	case "TOC":
		return "[Table of Contents]"
	case "INDEX":
		return "[Index]"
	case "LISTNUM":
		if run.FieldArg != "" {
			return "[" + strings.TrimSpace(run.FieldArg) + "]"
		}
		return "[List Number]"
	case "CITATION":
		if run.FieldArg != "" {
			return "[" + strings.TrimSpace(run.FieldArg) + "]"
		}
		return "[Citation]"
	case "BARCODE":
		if run.FieldArg != "" {
			return "[Barcode: " + strings.TrimSpace(run.FieldArg) + "]"
		}
		return "[Barcode]"
	case "IMPORT":
		if run.FieldArg != "" {
			return "[Import: " + strings.TrimSpace(run.FieldArg) + "]"
		}
		return "[Import]"
	case "SAVEPICTURE":
		if run.FieldArg != "" {
			return "[Save Picture: " + strings.TrimSpace(run.FieldArg) + "]"
		}
		return "[Save Picture]"
	case "EMBED":
		return "[Embedded Object]"
	case "LINK":
		return "[Linked Object]"
	case "MERGEFIELD":
		if run.FieldArg != "" {
			return "<<" + run.FieldArg + ">>"
		}
		return ""
	default:
		return strings.TrimSpace(run.FieldArg)
	}
}

// runIsNonRenderingOLEPlaceholder reports whether a legacy object anchor is
// only an implementation marker. Word hides these anchors when no payload,
// preview, link or ProgID can be resolved, so they must not consume flow space
// either — the renderer applies the same test before writing anything.
func runIsNonRenderingOLEPlaceholder(run resolve.Run) bool {
	return run.Kind == "olePlaceholder" && run.OLEPreview == nil && len(run.OLEData) == 0 &&
		run.OLEProgID == "" && run.OLERelID == "" && run.OLELinkURL == ""
}

func runNeedsWholeToken(run resolve.Run) bool {
	return (run.Format.FitTextWidth != nil && *run.Format.FitTextWidth > 0) ||
		run.FieldKind != "" ||
		len(run.Fields) > 0 ||
		run.Kind == "hyperlink" ||
		run.Kind == "horzRule" ||
		run.Kind == "math" ||
		run.Kind == "ruby" ||
		run.Kind == "formField" ||
		run.ContentControl != nil ||
		run.CustomXml != nil ||
		run.SmartTag != nil ||
		run.SignatureLine != nil ||
		run.Chart != nil ||
		run.MathML != "" ||
		run.FormField != nil ||
		run.Ruby != nil ||
		run.Revision != "" ||
		run.PropertyRevision != nil ||
		run.DisplayFieldRevision != nil
}

func runWordWidthPt(format resolve.RunFormat, rawWord string, defaultSizePt float64) (float64, float64) {
	return wordtextmetrics.RunWidthPt(nil, format, rawWord, defaultSizePt)
}

func runWordWidthPtWithCatalog(catalog *sharedfontmetrics.Catalog, format resolve.RunFormat, rawWord string, defaultSizePt float64) (float64, float64) {
	return wordtextmetrics.RunWidthPt(catalog, format, rawWord, defaultSizePt)
}

func runInterWordSpaceWidthPt(catalog *sharedfontmetrics.Catalog, format resolve.RunFormat, defaultSizePt, previousFontSizePt float64) float64 {
	return wordtextmetrics.InterWordSpaceWidthPt(catalog, format, defaultSizePt, previousFontSizePt)
}

func interWordSpaceWidthPt(previousFontSizePt, currentFontSizePt, defaultSizePt float64) float64 {
	fontSizePt := math.Max(previousFontSizePt, currentFontSizePt)
	if fontSizePt <= 0 {
		fontSizePt = defaultSizePt
	}
	return fontSizePt * 0.28
}

// paragraphSpatialRenderTokens preserves the source run-boundary whitespace
// contract needed by authoritative physical-slot rendering. strings.Fields
// alone is insufficient here: two adjacent formatted runs may form one word,
// while whitespace may live at the end of one run and the next word in another.
//
// Word collapses ordinary whitespace for this flow just as HTML does, so a
// whitespace sequence is represented as one inter-word separation. Tabs remain
// first-class runs and are intentionally left to the tab-stop pass.
func paragraphSpatialRenderTokens(p *resolve.Paragraph, defaultSizePt float64) []paragraphRenderToken {
	return paragraphSpatialRenderTokensWithContext(p, defaultSizePt, paragraphFlowContext{})
}

func paragraphSpatialRenderTokensWithContext(p *resolve.Paragraph, defaultSizePt float64, ctx paragraphFlowContext) []paragraphRenderToken {
	if p == nil {
		return nil
	}
	runs := resolve.CanonicalVisibleRuns(p)
	tokens := make([]paragraphRenderToken, 0, len(runs))
	pendingSpace := false
	haveText := false
	for _, run := range runs {
		// Hidden text is not displayed and takes no space, so it neither measures
		// nor separates the visible runs around it.
		if run.Format.Hidden() {
			continue
		}
		// Binary DOC group anchors can surface as synthetic OLE placeholders.
		// They are non-rendering anchors, not inline "Embedded Object" text.
		if runIsNonRenderingOLEPlaceholder(run) {
			continue
		}
		if run.FieldKind == "ADVANCE" {
			instr := strings.TrimSpace(run.FieldInstr)
			if instr == "" {
				instr = strings.TrimSpace(run.FieldArg)
			}
			tokens = append(tokens, paragraphRenderToken{
				run: run, fontSizePt: defaultSizePt, advance: true,
				advanceOffsets: inlineposition.ParseAdvance(instr),
			})
			pendingSpace = false
			continue
		}
		switch run.Kind {
		case "break":
			switch run.BreakType {
			case "line", "textWrapping", "carriageReturn":
				tokens = append(tokens, paragraphRenderToken{
					run: run, fontSizePt: defaultSizePt, forceLineBreak: true,
				})
				pendingSpace = false
				haveText = false
			}
			continue
		case "image", "chart", "textbox", "shape", "olePlaceholder":
			if wpt, hpt, ok := inlineObjectExtentPt(run); ok {
				if wpt <= 0 {
					wpt = 1
				}
				if hpt <= 0 {
					hpt = defaultSizePt
				}
				tokens = append(tokens, paragraphRenderToken{
					run: run, widthPt: wpt, heightPt: hpt, fontSizePt: defaultSizePt,
					separated: pendingSpace && haveText,
				})
				pendingSpace = false
				haveText = true
				continue
			}
			if run.Kind == "image" || run.Kind == "chart" || run.Kind == "textbox" || run.Kind == "shape" {
				continue
			}
		case "footnoteRef", "endnoteRef", "commentRef":
			// A note mark is a character of its paragraph, so it must contribute
			// the paragraph's own size to the line metric. Using the document
			// default here measures a 10pt note body at 11pt, which raises its
			// line pitch and shifts every page break after it.
			markSizePt := runTokenFontSizePt(run, defaultSizePt)
			tokens = append(tokens, paragraphRenderToken{
				run: run, widthPt: markSizePt * 0.8, heightPt: markSizePt, fontSizePt: markSizePt,
				separated: pendingSpace && haveText,
			})
			pendingSpace = false
			haveText = true
			continue
		case "tab":
			tokens = append(tokens, paragraphRenderToken{
				run: run, fontSizePt: defaultSizePt, tab: true,
			})
			pendingSpace = false
			haveText = true
			continue
		}

		if run.Text == "" {
			if fieldText := fieldRenderWidthText(run); fieldText != "" || runNeedsWholeToken(run) {
				wpt, fontSizePt := runWordWidthPtWithCatalog(ctx.FontMetrics, run.Format, fieldText, defaultSizePt)
				tokens = append(tokens, paragraphRenderToken{
					run: run, widthPt: wpt, heightPt: fontSizePt, fontSizePt: fontSizePt,
					separated: pendingSpace && haveText,
				})
				pendingSpace = false
				haveText = true
			}
			continue
		}
		if runNeedsWholeToken(run) {
			wpt, fontSizePt := runWordWidthPtWithCatalog(ctx.FontMetrics, run.Format, run.Text, defaultSizePt)
			tokens = append(tokens, paragraphRenderToken{
				run: run, widthPt: wpt, heightPt: fontSizePt, fontSizePt: fontSizePt,
				separated: pendingSpace && haveText,
			})
			pendingSpace = false
			haveText = true
			continue
		}
		wordStart := -1
		runes := []rune(run.Text)
		flushWord := func(end int) {
			if wordStart < 0 || wordStart >= end {
				return
			}
			rawWord := string(runes[wordStart:end])
			wpt, fontSizePt := runWordWidthPtWithCatalog(ctx.FontMetrics, run.Format, rawWord, defaultSizePt)
			tokenRun := run
			tokenRun.Text = rawWord
			tokens = append(tokens, paragraphRenderToken{
				run: tokenRun, widthPt: wpt, heightPt: fontSizePt, fontSizePt: fontSizePt,
				separated: pendingSpace && haveText,
			})
			pendingSpace = false
			haveText = true
			wordStart = -1
		}
		for i := 0; i < len(runes); i++ {
			ch := runes[i]
			if ch == ' ' || ch == '\n' || ch == '\r' || ch == '\f' || ch == '\v' || ch == '\t' {
				flushWord(i)
				if haveText {
					pendingSpace = true
				}
				continue
			}
			if wordStart < 0 {
				wordStart = i
			}
			// A hard hyphen is a Word line-break opportunity. Keep it on the
			// preceding token while allowing the following syllable to start a
			// new physical line without inventing an inter-word space.
			if p.Source != nil && ch == '-' && i+1 < len(runes) && runes[i+1] != ' ' {
				flushWord(i + 1)
			}
		}
		flushWord(len(runes))
	}
	return tokens
}

func paragraphRenderTokens(p *resolve.Paragraph, defaultSizePt float64) []paragraphRenderToken {
	if p == nil {
		return nil
	}
	runs := resolve.CanonicalVisibleRuns(p)
	tokens := make([]paragraphRenderToken, 0, len(runs))
	for _, run := range runs {
		if run.Format.Hidden() {
			continue
		}
		if runIsNonRenderingOLEPlaceholder(run) {
			continue
		}
		switch run.Kind {
		case "break":
			switch run.BreakType {
			case "line", "textWrapping", "carriageReturn":
				tokens = append(tokens, paragraphRenderToken{run: run, fontSizePt: defaultSizePt, forceLineBreak: true})
			}
			continue
		case "image":
			if run.Image == nil || run.Image.Floating {
				continue
			}
			wpt := emuToPoints(run.Image.Width)
			if wpt <= 0 {
				wpt = 1
			}
			tokens = append(tokens, paragraphRenderToken{run: run, widthPt: wpt, fontSizePt: defaultSizePt})
			continue
		case "footnoteRef", "endnoteRef", "commentRef":
			// A note mark is a character of the paragraph, so it must not
			// inflate the line height with the document default when the
			// paragraph (a 10pt note body, for example) is smaller.
			markSizePt := runTokenFontSizePt(run, defaultSizePt)
			tokens = append(tokens, paragraphRenderToken{run: run, widthPt: markSizePt * 0.8, fontSizePt: markSizePt})
			continue
		case "textbox":
			if run.TextBox == nil || run.TextBox.Floating {
				continue
			}
		case "shape":
			if run.Shape == nil || run.Shape.Floating {
				continue
			}
		case "tab":
			tokens = append(tokens, paragraphRenderToken{run: run, fontSizePt: defaultSizePt, tab: true})
			continue
		}
		if run.Text == "" {
			if fieldText := fieldRenderWidthText(run); fieldText != "" || runNeedsWholeToken(run) {
				wpt, fontSizePt := runWordWidthPt(run.Format, fieldText, defaultSizePt)
				tokens = append(tokens, paragraphRenderToken{run: run, widthPt: wpt, fontSizePt: fontSizePt})
			}
			continue
		}
		if runNeedsWholeToken(run) {
			wpt, fontSizePt := runWordWidthPt(run.Format, run.Text, defaultSizePt)
			tokens = append(tokens, paragraphRenderToken{run: run, widthPt: wpt, fontSizePt: fontSizePt})
			continue
		}
		for _, rawWord := range strings.Fields(run.Text) {
			wpt, fontSizePt := runWordWidthPt(run.Format, rawWord, defaultSizePt)
			tokenRun := run
			tokenRun.Text = rawWord
			tokens = append(tokens, paragraphRenderToken{run: tokenRun, widthPt: wpt, fontSizePt: fontSizePt})
		}
	}

	return tokens
}

// runTokenFontSizePt is the point size a synthetic layout token contributes to
// its line. Runs that carry their own size (a note mark inherits the note
// style's size) use it; runs that do not fall back to the document default.
func runTokenFontSizePt(run resolve.Run, defaultSizePt float64) float64 {
	if run.Format.FontSize != nil && *run.Format.FontSize > 0 {
		return float64(*run.Format.FontSize) / 2.0
	}
	return defaultSizePt
}

func paragraphNarrowColumnRunLines(p *resolve.Paragraph) [][]resolve.Run {
	if p == nil {
		return nil
	}
	lines := make([][]resolve.Run, 0, len([]rune(p.Text)))
	for _, run := range p.Runs {
		if run.Format.Hidden() {
			continue
		}
		switch run.Kind {
		case "image", "footnoteRef", "endnoteRef", "commentRef":
			lines = append(lines, []resolve.Run{run})
			continue
		}
		if run.Text == "" {
			continue
		}
		for _, ch := range []rune(run.Text) {
			charRun := run
			charRun.Text = string(ch)
			lines = append(lines, []resolve.Run{charRun})
		}
	}
	return lines
}

func paragraphTextFromRuns(runs []resolve.Run) string {
	var builder strings.Builder
	for _, run := range runs {
		if run.Text == "" {
			continue
		}
		builder.WriteString(run.Text)
	}
	return strings.TrimSpace(builder.String())
}

func splitRunsByBreakType(runs []resolve.Run, breakType string) [][]resolve.Run {
	if len(runs) == 0 {
		return nil
	}
	segments := make([][]resolve.Run, 0, 1)
	current := make([]resolve.Run, 0, len(runs))
	for _, run := range runs {
		if run.Kind == "break" && run.BreakType == breakType {
			segments = append(segments, current)
			current = make([]resolve.Run, 0, len(runs))
			continue
		}
		current = append(current, run)
	}
	segments = append(segments, current)
	return segments
}

func paragraphColumnBreakSegments(p *resolve.Paragraph) [][]resolve.Run {
	if p == nil {
		return nil
	}
	return splitRunsByBreakType(p.Runs, "column")
}

type paragraphLineMetric struct {
	maxFontSizePt     float64
	maxInlineHeightPt float64
}

func paragraphNarrowColumnLineMetrics(p *resolve.Paragraph, defaultSizePt float64) []paragraphLineMetric {
	if p == nil {
		return []paragraphLineMetric{{maxFontSizePt: defaultSizePt}}
	}
	metrics := make([]paragraphLineMetric, 0, len([]rune(p.Text)))
	for _, run := range p.Runs {
		switch run.Kind {
		case "image":
			if run.Image != nil && !run.Image.Floating {
				metrics = append(metrics, paragraphLineMetric{maxFontSizePt: defaultSizePt, maxInlineHeightPt: inlineImageBoxHeightPt(run.Image)})
			}
			continue
		case "footnoteRef", "endnoteRef", "commentRef":
			metrics = append(metrics, paragraphLineMetric{maxFontSizePt: defaultSizePt})
			continue
		}
		if run.Text == "" {
			continue
		}
		fontSizePt := defaultSizePt
		if run.Format.FontSize != nil && *run.Format.FontSize > 0 {
			fontSizePt = float64(*run.Format.FontSize) / 2.0
		}
		for range []rune(run.Text) {
			metrics = append(metrics, paragraphLineMetric{maxFontSizePt: fontSizePt})
		}
	}
	if len(metrics) == 0 {
		return []paragraphLineMetric{{maxFontSizePt: defaultSizePt}}
	}
	return metrics
}

func paragraphLineHeightsPt(p *resolve.Paragraph, geom PageGeometry, metrics []paragraphLineMetric, defaultFontSizePt float64) []float64 {
	if len(metrics) == 0 {
		return []float64{paragraphLineHeightForMetric(p, geom, paragraphLineMetric{maxFontSizePt: defaultFontSizePt})}
	}
	heights := make([]float64, 0, len(metrics))
	for _, metric := range metrics {
		if metric.maxFontSizePt <= 0 {
			metric.maxFontSizePt = defaultFontSizePt
		}
		heights = append(heights, paragraphLineHeightForMetric(p, geom, metric))
	}
	return heights
}

func paragraphLineHeightForMetric(p *resolve.Paragraph, geom PageGeometry, metric paragraphLineMetric) float64 {
	fontSizePt := metric.maxFontSizePt
	if fontSizePt <= 0 {
		fontSizePt = 12.0
	}
	lineHeight := math.Max(fontSizePt*paragraphLineHeightRatio(p), metric.maxInlineHeightPt)
	if p == nil || p.Format.LineSpacing == nil {
		return snapLineHeightToDocGrid(p, geom, lineHeight)
	}
	switch p.Format.LineRule {
	case "auto", "":
		lineHeight = autoParagraphLineHeightWithInlinePt(p, fontSizePt, metric.maxInlineHeightPt)
	case "exact":
		lineHeight = twipsToPoints(*p.Format.LineSpacing)
	case "atLeast":
		lineHeight = math.Max(lineHeight, twipsToPoints(*p.Format.LineSpacing))
	}
	return snapLineHeightToDocGrid(p, geom, lineHeight)
}

func snapLineHeightToDocGrid(p *resolve.Paragraph, geom PageGeometry, lineHeight float64) float64 {
	if p == nil || !p.Format.SnapToGrid {
		return lineHeight
	}
	if geom.DocGridPitchPt <= 0 {
		return lineHeight
	}
	if !strings.EqualFold(geom.DocGridType, "lines") && !strings.EqualFold(geom.DocGridType, "linesAndChars") {
		return lineHeight
	}
	lines := math.Ceil(lineHeight / geom.DocGridPitchPt)
	if lines < 1 {
		lines = 1
	}
	return lines * geom.DocGridPitchPt
}

func cumulativeLineHeight(heights []float64, lineIndex int) float64 {
	total := 0.0
	if lineIndex > len(heights) {
		lineIndex = len(heights)
	}
	for i := 0; i < lineIndex; i++ {
		total += heights[i]
	}
	return total
}

func lineHeightAt(heights []float64, lineIndex int, fallback float64) float64 {
	if lineIndex >= 0 && lineIndex < len(heights) {
		return heights[lineIndex]
	}
	return fallback
}

func sumFloat64s(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total
}

type lineExclusion struct {
	top       float64
	bottom    float64
	left      float64
	right     float64
	fullWidth bool
	wrapSide  string
	padLeft   float64
	padRight  float64
	// polygon holds contour vertices in absolute page points (object-relative + floatbox origin).
	polygon [][2]float64
}

// paragraphTopAndBottomClearancePt returns the vertical clearance required at
// the start of a paragraph when an already-placed topAndBottom float overlaps the
// first line. Word moves the text cursor below such a float; treating it as a
// one-point-wide line exclusion produces many artificial lines and under/over-flow
// decisions that diverge sharply from Word pagination.
func paragraphTopAndBottomClearancePt(lineStartY, lineHeight float64, activeFloats []FloatBox) float64 {
	if len(activeFloats) == 0 {
		return 0
	}
	if lineHeight <= 0 {
		lineHeight = 1
	}
	cursor := lineStartY
	for range len(activeFloats) + 1 {
		next := cursor
		for _, fl := range activeFloats {
			exclusion, ok := lineExclusionForFloat(fl)
			if !ok || !exclusion.fullWidth {
				continue
			}
			if exclusion.bottom <= cursor || exclusion.top >= cursor+lineHeight {
				continue
			}
			if exclusion.bottom > next {
				next = exclusion.bottom
			}
		}
		if next <= cursor {
			break
		}
		cursor = next
	}
	return math.Max(0, cursor-lineStartY)
}

// paragraphAffectedByActiveWrap reports whether a previously-resolved float
// still owns text-flow geometry in this paragraph. Anchor ownership and wrap
// ownership are intentionally different: Word floats can extend through
// several following paragraphs.
func paragraphFragmentNeedsSpatialRendering(fragment *ParagraphFragment) bool {
	return fragment != nil && fragment.SpatialRendering && len(fragment.Lines) > 0
}

// activeWrapFloatsForRange narrows page-level float state to the objects that
// can actually affect a paragraph/line range in the current column. Float
// anchor lifetime and wrap lifetime are deliberately separate: a drawing may
// remain on the page after its anchor paragraph, but it stops participating in
// line layout as soon as its transformed exclusion no longer intersects the
// current flow range.
// commitPageFloats is the single ownership boundary for resolved floating
// objects. Anchors are committed to exactly one physical page.
func commitPageFloats(page *Page, floats []FloatBox) {
	if page == nil || len(floats) == 0 {
		return
	}
	for i := range floats {
		floats[i].Page = page.Number
	}
	page.Floats = append(page.Floats, floats...)
}

// wrapFloatsForFlowRegion returns the page-owned float set used when a block
// is measured in a flow region. Columns on the same page deliberately share
// floats; a new physical page owns a fresh set. This keeps anchor lifetime,
// wrap lifetime and page lifetime separate.
func wrapFloatsForFlowRegion(page Page) []FloatBox {
	if len(page.Floats) == 0 {
		return nil
	}
	return page.Floats
}

// appendPendingIndependentFloats adds position-independent floats whose anchors
// have not been emitted yet to the active wrap set. Word places such a float on
// the page that contains its anchor paragraph, so an anchor separated from the
// block being measured by a forced page break belongs to a later page and must
// not wrap this page's text. Without that guard a float anchored several pages
// ahead narrows every preceding page it is measured against.
func appendPendingIndependentFloats(active []FloatBox, pending []pendingIndependentFloat, blockList []resolve.Block, index int) []FloatBox {
	var combined []FloatBox
	for _, candidate := range pending {
		if forcedPageBreakBetween(blockList, index, candidate.order) {
			// Later anchors share the break, so every remaining candidate is on a
			// later page too.
			break
		}
		if combined == nil {
			combined = make([]FloatBox, len(active), len(active)+len(pending))
			copy(combined, active)
		}
		combined = append(combined, candidate.fl)
	}
	if combined == nil {
		return active
	}
	return combined
}

// forcedPageBreakBetween reports whether a block after start and no later than
// end forces a page break, which means end cannot be emitted on the page that
// contains start.
func forcedPageBreakBetween(blockList []resolve.Block, start, end int) bool {
	if start < 0 {
		start = 0
	}
	for i := start + 1; i <= end && i < len(blockList); i++ {
		block := &blockList[i]
		if blockForcesPageBreakBefore(block) || blockForcesPageBreakAfter(block) {
			return true
		}
	}
	return false
}

func activeWrapFloatsForRange(geom PageGeometry, column int, top, bottom float64, floats []FloatBox) []FloatBox {
	if len(floats) == 0 || bottom <= top {
		return nil
	}
	columnLeft := columnLeftPt(geom, column)
	columnRight := columnLeft + columnWidthPt(geom, column)
	if columnRight <= columnLeft {
		columnLeft = geom.MarginLeft
		columnRight = geom.Width - geom.MarginRight
	}
	active := make([]FloatBox, 0, len(floats))
	for _, fl := range floats {
		exclusion, ok := lineExclusionForFloat(fl)
		if !ok || exclusion.bottom <= top || exclusion.top >= bottom {
			continue
		}
		if !exclusion.fullWidth && (exclusion.right <= columnLeft || exclusion.left >= columnRight) {
			continue
		}
		active = append(active, fl)
	}
	return active
}

func paragraphAffectedByActiveWrap(geom PageGeometry, column int, paraPageY, paraHeight float64, activeFloats []FloatBox) bool {
	return len(activeWrapFloatsForRange(geom, column, paraPageY, paraPageY+paraHeight, activeFloats)) > 0
}

func paragraphWrapExclusions(p *resolve.Paragraph, geom PageGeometry, column int, paraPageY, paraHeight float64, activeFloats []FloatBox) []lineExclusion {
	if p == nil && len(activeFloats) == 0 {
		return nil
	}
	localFloats := activeParagraphFloats(p, geom, column, paraPageY, paraHeight)
	activeFloats = activeWrapFloatsForRange(geom, column, paraPageY, paraPageY+paraHeight, activeFloats)
	exclusions := make([]lineExclusion, 0, len(activeFloats)+len(localFloats))
	for _, fl := range activeFloats {
		if exclusion, ok := lineExclusionForFloat(fl); ok {
			exclusions = append(exclusions, exclusion)
		}
	}
	for _, fl := range localFloats {
		if drawingIsLeadingBlockFloat(p, fl) {
			continue
		}
		if exclusion, ok := lineExclusionForFloat(fl); ok {
			exclusions = append(exclusions, exclusion)
		}
	}
	return exclusions
}

func activeParagraphFloats(p *resolve.Paragraph, geom PageGeometry, column int, paraPageY, paraHeight float64) []FloatBox {
	if p == nil {
		return nil
	}
	return appendParagraphFloats(nil, p.Runs, geom, column, paraPageY, paraHeight)
}

func rotatedRectPolygon(x, y, width, height, rotationDeg float64) [][2]float64 {
	if width <= 0 || height <= 0 {
		return nil
	}
	angle := math.Mod(rotationDeg, 360)
	if angle < 0 {
		angle += 360
	}
	if math.Abs(angle) < 1e-9 || math.Abs(angle-360) < 1e-9 {
		return [][2]float64{{x, y}, {x + width, y}, {x + width, y + height}, {x, y + height}}
	}
	rad := angle * math.Pi / 180
	cosA, sinA := math.Cos(rad), math.Sin(rad)
	cx, cy := x+width/2, y+height/2
	points := [][2]float64{{-width / 2, -height / 2}, {width / 2, -height / 2}, {width / 2, height / 2}, {-width / 2, height / 2}}
	for i := range points {
		dx, dy := points[i][0], points[i][1]
		points[i][0] = cx + dx*cosA - dy*sinA
		points[i][1] = cy + dx*sinA + dy*cosA
	}
	return points
}

func polygonBounds(poly [][2]float64) (left, top, right, bottom float64, ok bool) {
	if len(poly) == 0 {
		return 0, 0, 0, 0, false
	}
	left, right = poly[0][0], poly[0][0]
	top, bottom = poly[0][1], poly[0][1]
	for _, point := range poly[1:] {
		left = math.Min(left, point[0])
		right = math.Max(right, point[0])
		top = math.Min(top, point[1])
		bottom = math.Max(bottom, point[1])
	}
	return left, top, right, bottom, true
}

func lineExclusionForFloat(fl FloatBox) (lineExclusion, bool) {
	if fl.Drawing() != nil {
		return drawingExclusionForFloat(fl)
	}
	switch {
	case fl.FloatTable != nil:
		t := fl.FloatTable
		if t == nil || t.Format.FloatProps == nil {
			return lineExclusion{}, false
		}
		fp := t.Format.FloatProps
		left := float64(fp.LeftFromText) / twipsPerPoint
		right := float64(fp.RightFromText) / twipsPerPoint
		top := float64(fp.TopFromText) / twipsPerPoint
		bottom := float64(fp.BottomFromText) / twipsPerPoint
		return lineExclusion{
			top:      fl.Y - top,
			bottom:   fl.Y + fl.Height + bottom,
			left:     fl.X - left,
			right:    fl.X + fl.Width + right,
			padLeft:  left,
			padRight: right,
		}, true
	case fl.FrameParagraph != nil:
		fp := fl.FrameParagraph.Frame
		if fp == nil || fp.DropCap == "" || fp.DropCap == "none" {
			return lineExclusion{}, false
		}
		hSpacePt := float64(fp.HSpace) / twipsPerPoint
		vSpacePt := float64(fp.VSpace) / twipsPerPoint
		return lineExclusion{
			top:      fl.Y - vSpacePt,
			bottom:   fl.Y + fl.Height + vSpacePt,
			left:     fl.X - hSpacePt,
			right:    fl.X + fl.Width + hSpacePt,
			wrapSide: "right", // text flows to the right of the drop cap
		}, true
	default:
		return lineExclusion{}, false
	}
}

// drawingLineExclusion is the canonical wrap geometry for Word drawing
// objects. Images, text boxes and shapes must use exactly the same geometry so
// pagination cannot diverge merely because the visual payload type differs.
//
// Wrap polygons are authored in the object's unscaled coordinate space. When
// sizeRelH/sizeRelV changes the rendered FloatBox size, scale the contour to the
// resolved size first, then rotate it around the same center used by CSS.

func resolvedWrapPolygon(fl FloatBox, polygon [][2]float64, sourceWidthPt, sourceHeightPt, rotationDeg float64) [][2]float64 {
	if len(polygon) == 0 {
		return nil
	}
	sx, sy := 1.0, 1.0
	if sourceWidthPt > 0 && fl.Width > 0 {
		sx = fl.Width / sourceWidthPt
	}
	if sourceHeightPt > 0 && fl.Height > 0 {
		sy = fl.Height / sourceHeightPt
	}
	abs := make([][2]float64, len(polygon))
	for i, point := range polygon {
		abs[i] = [2]float64{fl.X + point[0]*sx, fl.Y + point[1]*sy}
	}
	if rotationDeg == 0 {
		return abs
	}
	return rotatePolygonAround(abs, fl.X+fl.Width/2, fl.Y+fl.Height/2, rotationDeg)
}

func rotatePolygonAround(polygon [][2]float64, centerX, centerY, rotationDeg float64) [][2]float64 {
	if len(polygon) == 0 || rotationDeg == 0 {
		return polygon
	}
	angle := math.Mod(rotationDeg, 360)
	if angle < 0 {
		angle += 360
	}
	if math.Abs(angle) < 1e-9 || math.Abs(angle-360) < 1e-9 {
		return polygon
	}
	rad := angle * math.Pi / 180
	cosA, sinA := math.Cos(rad), math.Sin(rad)
	rotated := make([][2]float64, len(polygon))
	for i, point := range polygon {
		dx, dy := point[0]-centerX, point[1]-centerY
		rotated[i] = [2]float64{
			centerX + dx*cosA - dy*sinA,
			centerY + dx*sinA + dy*cosA,
		}
	}
	return rotated
}

// absolutePolygon translates object-relative polygon vertices to absolute page
// coordinates by adding the float box's top-left origin.
func absolutePolygon(poly [][2]float64, originX, originY float64) [][2]float64 {
	abs := make([][2]float64, len(poly))
	for i, pt := range poly {
		abs[i] = [2]float64{originX + pt[0], originY + pt[1]}
	}
	return abs
}

func polygonIntervalsAtY(polygon [][2]float64, y float64) [][2]float64 {
	n := len(polygon)
	if n < 3 {
		return nil
	}
	intersections := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		p1 := polygon[i]
		p2 := polygon[(i+1)%n]
		y1, y2 := p1[1], p2[1]
		if y1 == y2 {
			continue
		}
		if (y1 <= y && y2 > y) || (y2 <= y && y1 > y) {
			t := (y - y1) / (y2 - y1)
			intersections = append(intersections, p1[0]+t*(p2[0]-p1[0]))
		}
	}
	if len(intersections) < 2 {
		return nil
	}
	sort.Float64s(intersections)
	intervals := make([][2]float64, 0, len(intersections)/2)
	for i := 0; i+1 < len(intersections); i += 2 {
		left, right := intersections[i], intersections[i+1]
		if right > left {
			intervals = append(intervals, [2]float64{left, right})
		}
	}
	return intervals
}

// polygonIntervalsAtBand approximates the filled X-intervals occupied by a
// polygon across the horizontal band [lineTop, lineBottom] by sampling multiple
// points within the band and merging the resulting scanline intervals.
// For better accuracy with complex concave polygons, we sample at:
// - Top, 1/4, middle, 3/4, bottom of the band
// - All polygon vertices within the band
// - Additional points where polygon edges change direction significantly
// polygonXRangeAtBand preserves the legacy min/max query over the merged scanline
// intervals used for tight/through wrapping.
func commonWrapSide(exclusions []lineExclusion) string {
	if len(exclusions) == 0 {
		return ""
	}
	common := strings.ToLower(strings.TrimSpace(exclusions[0].wrapSide))
	if common == "bothsides" || common == "" {
		return ""
	}
	for _, exclusion := range exclusions[1:] {
		side := strings.ToLower(strings.TrimSpace(exclusion.wrapSide))
		if side != common {
			return ""
		}
	}
	return common
}

func paragraphContentWidthPt(p *resolve.Paragraph, geom PageGeometry) float64 {
	widthPt := flowContentWidthPt(geom)
	if p == nil {
		return widthPt
	}
	leftBorderOutset, rightBorderOutset := paragraphHorizontalBorderOutsetPt(p)
	widthPt -= leftBorderOutset + rightBorderOutset
	if p.Format.IndentLeft != nil {
		widthPt -= twipsToPoints(*p.Format.IndentLeft)
	}
	if p.Format.IndentRight != nil {
		widthPt -= twipsToPoints(*p.Format.IndentRight)
	}
	if widthPt < 1 {
		return 1
	}
	return widthPt
}

func paragraphLineWidthsPt(p *resolve.Paragraph, widthPt float64) (float64, float64) {
	continuationWidthPt := widthPt
	firstWidthPt := widthPt
	if p == nil {
		return firstWidthPt, continuationWidthPt
	}
	if p.Format.IndentFirstLine != nil {
		firstWidthPt -= twipsToPoints(*p.Format.IndentFirstLine)
	}
	if p.Format.IndentHanging != nil {
		firstWidthPt += twipsToPoints(*p.Format.IndentHanging)
	}
	if firstWidthPt < 1 {
		firstWidthPt = 1
	}
	if continuationWidthPt < 1 {
		continuationWidthPt = 1
	}
	return firstWidthPt, continuationWidthPt
}

func paragraphContentLeftPt(p *resolve.Paragraph, baseLeftPt float64) float64 {
	left := baseLeftPt
	if p != nil && p.Format.IndentLeft != nil {
		left += twipsToPoints(*p.Format.IndentLeft)
	}
	return left
}

func paragraphFirstLineLeftPt(p *resolve.Paragraph, continuationLeftPt float64) float64 {
	left := continuationLeftPt
	if p == nil {
		return left
	}
	if p.Format.IndentFirstLine != nil {
		left += twipsToPoints(*p.Format.IndentFirstLine)
	}
	if p.Format.IndentHanging != nil {
		left -= twipsToPoints(*p.Format.IndentHanging)
	}
	return left
}

func bodyContentWidthPt(geom PageGeometry) float64 {
	widthPt := geom.Width - geom.MarginLeft - geom.MarginRight
	if widthPt < 1 {
		return 1
	}
	return widthPt
}

func footnoteLayoutGeometry(geom PageGeometry) PageGeometry {
	footnoteGeom := geom
	footnoteGeom.ColumnCount = 1
	footnoteGeom.ColumnGapPt = 0
	footnoteGeom.ColumnWidthPt = bodyContentWidthPt(geom)
	footnoteGeom.ColumnWidthsPt = nil
	footnoteGeom.ColumnGapsPt = nil
	footnoteGeom.ColumnSep = false
	return footnoteGeom
}

func flowContentWidthPt(geom PageGeometry) float64 {
	if geom.ColumnCount > 1 && len(geom.ColumnWidthsPt) > 0 && geom.ColumnWidthsPt[0] > 0 {
		return geom.ColumnWidthsPt[0]
	}
	if geom.ColumnCount > 1 && geom.ColumnWidthPt > 0 {
		return geom.ColumnWidthPt
	}
	return bodyContentWidthPt(geom)
}

func columnWidthPt(geom PageGeometry, column int) float64 {
	if column >= 0 && column < len(geom.ColumnWidthsPt) && geom.ColumnWidthsPt[column] > 0 {
		return geom.ColumnWidthsPt[column]
	}
	return geom.ColumnWidthPt
}

func columnLeftPt(geom PageGeometry, column int) float64 {
	if column < 0 {
		column = 0
	}
	if geom.ColumnCount <= 1 {
		return geom.MarginLeft
	}
	maxColumn := geom.ColumnCount - 1
	if column > maxColumn {
		column = maxColumn
	}
	if len(geom.ColumnWidthsPt) >= geom.ColumnCount {
		x := geom.MarginLeft
		for i := 0; i < column; i++ {
			x += geom.ColumnWidthsPt[i]
			if i < len(geom.ColumnGapsPt) {
				x += geom.ColumnGapsPt[i]
			} else {
				x += geom.ColumnGapPt
			}
		}
		return x
	}
	return geom.MarginLeft + float64(column)*(geom.ColumnWidthPt+geom.ColumnGapPt)
}

func resolveTableWidthPt(tf resolve.TableFormat, availableWidthPt float64) float64 {
	if tf.Width != nil {
		switch tf.WidthType {
		case "dxa":
			return twipsToPoints(*tf.Width)
		case "pct":
			return availableWidthPt * float64(*tf.Width) / 5000.0
		}
	}
	if len(tf.ColumnWidths) > 0 {
		total := 0.0
		for _, width := range tf.ColumnWidths {
			total += twipsToPoints(width)
		}
		if total > 0 {
			return total
		}
	}
	if availableWidthPt > 0 {
		return availableWidthPt
	}
	return 1
}

func tableRowHeight(doc *resolve.Document, row resolve.TableRow, tf resolve.TableFormat, tableWidthPt float64, includeTableTop, includeTableBottom bool) float64 {
	contentHeight := estimateTableRowContentHeight(doc, row, tf, tableWidthPt, includeTableTop, includeTableBottom)
	if row.Format.Height != nil && *row.Format.Height > 0 {
		heightPt := twipsToPoints(*row.Format.Height)
		if row.Format.HeightRule == "exact" {
			return heightPt
		}
		if contentHeight < heightPt {
			return heightPt
		}
	}
	if contentHeight == 0 && rowHasOnlyVMergeContinuations(row) {
		return 0
	}
	if contentHeight > 0 {
		return contentHeight
	}
	return defaultFontSizePt(doc) * wordSingleLineRatio
}

func estimateTableRowContentHeight(doc *resolve.Document, row resolve.TableRow, tf resolve.TableFormat, tableWidthPt float64, includeTableTop, includeTableBottom bool) float64 {
	cellWidths := TableCellWidths(row, tf, tableWidthPt)
	maxHeight := 0.0
	for i, cell := range row.Cells {
		vMerge := strings.ToLower(strings.TrimSpace(cell.Format.VMerge))
		if vMerge == "continue" || vMerge == "restart" {
			continue
		}
		cellWidthPt := tableWidthPt
		if i < len(cellWidths) && cellWidths[i] > 0 {
			cellWidthPt = cellWidths[i]
		}
		height := tableCellHeightWithRowContext(doc, cell, tf, row.Override, row.Format.CellSpacing, cellWidthPt, includeTableTop, includeTableBottom)
		if height > maxHeight {
			maxHeight = height
		}
	}
	return maxHeight
}

// TableCellWidths resolves the rendered width of each concrete cell in a row.
func TableCellWidths(row resolve.TableRow, tf resolve.TableFormat, tableWidthPt float64) []float64 {
	if len(row.Cells) == 0 {
		return nil
	}
	effectiveCols := row.Format.GridBefore + row.Format.GridAfter
	for _, cell := range row.Cells {
		span := cell.Format.GridSpan
		if span <= 0 {
			span = 1
		}
		effectiveCols += span
	}
	if effectiveCols <= 0 {
		effectiveCols = len(row.Cells)
	}
	gridWidths := make([]float64, 0, effectiveCols)
	if len(tf.ColumnWidths) > 0 {
		totalGrid := 0.0
		for _, width := range tf.ColumnWidths {
			totalGrid += twipsToPoints(width)
		}
		if totalGrid > 0 {
			for _, width := range tf.ColumnWidths {
				gridWidths = append(gridWidths, tableWidthPt*twipsToPoints(width)/totalGrid)
			}
		}
	}
	if len(gridWidths) == 0 {
		defaultWidth := tableWidthPt / float64(effectiveCols)
		for i := 0; i < effectiveCols; i++ {
			gridWidths = append(gridWidths, defaultWidth)
		}
	}
	widths := make([]float64, 0, len(row.Cells))
	gridIndex := row.Format.GridBefore
	for _, cell := range row.Cells {
		span := cell.Format.GridSpan
		if span <= 0 {
			span = 1
		}
		width := 0.0
		for offset := 0; offset < span; offset++ {
			if gridIndex+offset < len(gridWidths) {
				width += gridWidths[gridIndex+offset]
			}
		}
		if width == 0 && cell.Format.Width != nil {
			switch cell.Format.WidthType {
			case "dxa":
				width = twipsToPoints(*cell.Format.Width)
			case "pct":
				width = tableWidthPt * float64(*cell.Format.Width) / 5000.0
			}
		}
		if width <= 0 {
			width = tableWidthPt / float64(len(row.Cells))
		}
		widths = append(widths, width)
		gridIndex += span
	}
	return widths
}

func rowHasOnlyVMergeContinuations(row resolve.TableRow) bool {
	if len(row.Cells) == 0 {
		return false
	}
	for _, cell := range row.Cells {
		if cell.Format.VMerge != "continue" {
			return false
		}
	}
	return true
}

// resolvedTableCellInsets mirrors the renderer's table-cell margin precedence
// so pagination measures the same content box that HTML ultimately paints.
// tcMar overrides table defaults side-by-side; tblPrEx row overrides then win.
// Cell spacing supplies the legacy horizontal half-gap only when no explicit
// horizontal padding was resolved.
func resolvedTableCellInsets(tf resolve.TableFormat, rowOverride *resolve.TableRowOverride, rowCellSpacing int, cf resolve.TableCellFormat) (top, bottom, left, right int) {
	top, bottom = tf.CellPadTop, tf.CellPadBottom
	left, right = tf.CellPadLeft, tf.CellPadRight

	if cf.CellPadTop != 0 {
		top = cf.CellPadTop
	}
	if cf.CellPadBottom != 0 {
		bottom = cf.CellPadBottom
	}
	if cf.CellPadLeft != 0 {
		left = cf.CellPadLeft
	}
	if cf.CellPadRight != 0 {
		right = cf.CellPadRight
	}

	if rowOverride != nil {
		if rowOverride.CellPadTop != 0 {
			top = rowOverride.CellPadTop
		}
		if rowOverride.CellPadBottom != 0 {
			bottom = rowOverride.CellPadBottom
		}
		if rowOverride.CellPadLeft != 0 {
			left = rowOverride.CellPadLeft
		}
		if rowOverride.CellPadRight != 0 {
			right = rowOverride.CellPadRight
		}
	}

	spacing := tf.CellSpacing
	if rowCellSpacing > 0 {
		spacing = rowCellSpacing
	}
	if left == 0 && spacing > 0 {
		left = spacing / 2
	}
	if right == 0 && spacing > 0 {
		right = spacing / 2
	}
	return
}

func tableCellHeight(doc *resolve.Document, cell resolve.TableCell, tf resolve.TableFormat, cellWidthPt float64, includeTableTop, includeTableBottom bool) float64 {
	return tableCellHeightWithRowContext(doc, cell, tf, nil, 0, cellWidthPt, includeTableTop, includeTableBottom)
}

func tableCellHeightWithRowContext(doc *resolve.Document, cell resolve.TableCell, tf resolve.TableFormat, rowOverride *resolve.TableRowOverride, rowCellSpacing int, cellWidthPt float64, includeTableTop, includeTableBottom bool) float64 {
	padTop, padBottom, padLeft, padRight := resolvedTableCellInsets(tf, rowOverride, rowCellSpacing, cell.Format)
	paddingTop := twipsToPoints(padTop)
	paddingBottom := twipsToPoints(padBottom)
	paddingLeft := twipsToPoints(padLeft)
	paddingRight := twipsToPoints(padRight)
	tableTop := (*resolve.BorderSide)(nil)
	tableInsideH := (*resolve.BorderSide)(nil)
	tableBottom := (*resolve.BorderSide)(nil)
	if tf.Borders != nil {
		tableTop = tf.Borders.Top
		tableInsideH = tf.Borders.InsideH
		tableBottom = tf.Borders.Bottom
	}
	// For non-first rows use the insideH border as the row separator (top of the row).
	effectiveTableTop := tableTop
	if !includeTableTop {
		effectiveTableTop = tableInsideH
	}
	borderTop := effectiveCellBorderWidthPt(nil, true, effectiveTableTop)
	borderBottom := effectiveCellBorderWidthPt(nil, includeTableBottom, tableBottom)
	if cell.Format.Borders != nil {
		borderTop = effectiveCellBorderWidthPt(cell.Format.Borders.Top, true, effectiveTableTop)
		borderBottom = effectiveCellBorderWidthPt(cell.Format.Borders.Bottom, includeTableBottom, tableBottom)
	}
	contentWidthPt := cellWidthPt - paddingLeft - paddingRight
	if contentWidthPt < 1 {
		contentWidthPt = 1
	}
	contentHeight := 0.0
	blocks := cell.Blocks
	if len(blocks) == 0 && strings.TrimSpace(cell.Text) != "" {
		blocks = []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Text: cell.Text, Runs: []resolve.Run{{Kind: "run", Text: cell.Text}}}}}
	}
	contentHeight += storyBlocksHeightInWidth(doc, blocks, PageGeometry{}, contentWidthPt)
	return paddingTop + borderTop + contentHeight + borderBottom + paddingBottom
}

func effectiveCellBorderWidthPt(cellSide *resolve.BorderSide, includeTableBorder bool, tableSide *resolve.BorderSide) float64 {
	if cellSide != nil {
		return resolvedBorderWidthPt(cellSide)
	}
	if includeTableBorder {
		return resolvedBorderWidthPt(tableSide)
	}
	return 0
}

func resolvedBorderWidthPt(side *resolve.BorderSide) float64 {
	if side == nil {
		return 0
	}
	switch strings.ToLower(side.Val) {
	case "nil", "none":
		return 0
	}
	if side.SizePt > 0 {
		return side.SizePt
	}
	return 0
}

func paragraphWidthWithinCell(p *resolve.Paragraph, widthPt float64) float64 {
	if p == nil {
		return widthPt
	}
	leftBorderOutset, rightBorderOutset := paragraphHorizontalBorderOutsetPt(p)
	widthPt -= leftBorderOutset + rightBorderOutset
	if p.Format.IndentLeft != nil {
		widthPt -= twipsToPoints(*p.Format.IndentLeft)
	}
	if p.Format.IndentRight != nil {
		widthPt -= twipsToPoints(*p.Format.IndentRight)
	}
	if widthPt < 1 {
		return 1
	}
	return widthPt
}

func estimateWrappedLineCount(text string, charsPerLine int) int {
	words := strings.Fields(text)
	if len(words) == 0 {
		return 1
	}
	count := 1
	current := runeLen(words[0])
	for _, word := range words[1:] {
		wordLen := runeLen(word)
		if current+1+wordLen <= charsPerLine {
			current += 1 + wordLen
			continue
		}
		count++
		current = wordLen
	}
	return count
}

func runeLen(s string) int {
	return len([]rune(s))
}

// sectionStoryForPage selects the header/footer story that applies to the
// given page number, falling back to the section default story when a more
// specific first/even variant is not present.
func sectionStoryForPage(stories map[string]resolve.Story, refs []resolve.Reference, pageNumber, sectionPageNumber int, evenAndOddHeaders, titlePage bool) (resolve.Story, bool) {
	if sectionPageNumber <= 0 {
		sectionPageNumber = pageNumber
	}
	if titlePage && sectionPageNumber == 1 {
		if story, ok := sectionStoryByType(stories, refs, "first"); ok {
			return story, true
		}
	}
	if evenAndOddHeaders && pageNumber > 0 && pageNumber%2 == 0 {
		if story, ok := sectionStoryByType(stories, refs, "even"); ok {
			return story, true
		}
	}
	return sectionStoryByType(stories, refs, "default")
}

func sectionStoryByType(stories map[string]resolve.Story, refs []resolve.Reference, storyType string) (resolve.Story, bool) {
	for _, ref := range refs {
		if storyType == "default" {
			if !strings.EqualFold(ref.Type, "default") && ref.Type != "" {
				continue
			}
		} else if !strings.EqualFold(ref.Type, storyType) {
			continue
		}
		if story, ok := stories[ref.PartName]; ok {
			return story, true
		}
	}
	return resolve.Story{}, false
}

// estimateStoryHeightPt computes the total estimated height in points of a
// resolved story (header, footer, or footnote) at the given content width.
func estimateStoryHeightPt(doc *resolve.Document, story resolve.Story, widthPt float64) float64 {
	blocks := trimTerminalEmptyStoryParagraphs(contentBlocks(story.Blocks))
	return storyBlocksHeightInWidth(doc, blocks, PageGeometry{}, widthPt)
}

func trimTerminalEmptyStoryParagraphs(blocks []resolve.Block) []resolve.Block {
	end := len(blocks)
	for end > 0 {
		b := blocks[end-1]
		if b.Kind != "paragraph" || !storyParagraphIsVisuallyEmpty(b.Paragraph) {
			break
		}
		end--
	}
	return blocks[:end]
}

func storyParagraphIsVisuallyEmpty(p *resolve.Paragraph) bool {
	if p == nil {
		return true
	}
	if strings.TrimSpace(p.Text) != "" {
		return false
	}
	for _, run := range p.Runs {
		if strings.TrimSpace(run.Text) != "" {
			return false
		}
		switch run.Kind {
		case "image", "drawing", "textbox", "shape", "chart", "ole", "field", "footnoteRef", "endnoteRef":
			return false
		}
	}
	return true
}

func emuToPoints(emu int64) float64 {
	return float64(emu) / emuPerPoint
}

func twipsToPoints(twips int) float64 {
	return float64(twips) / twipsPerPoint
}
