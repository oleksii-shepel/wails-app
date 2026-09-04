package layout

import (
	"math"
	"sort"
	"strings"

	resolve "office/pkg/word/ir"
)

const (
	emuPerPoint = 12700 // 914400 EMU per inch / 72 points per inch

	// cssNormalLineRatio is the approximation of CSS line-height:normal relative
	// to the font point size (≈1.2 for most fonts). Word's "single" line spacing
	// uses the font's actual advance height — the same quantity browsers express
	// as line-height:normal. In OOXML, w:line/240 is a multiplier on top of
	// "single", so the CSS equivalent is (w:line/240) × cssNormalLineRatio.
	cssNormalLineRatio = 1.2

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
	StartLine       int
	EndLine         int
	Runs            []resolve.Run
	Text            string
	IsContinuation  bool
	IsLast          bool
	StartsNewColumn bool
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

	// effectiveContentHeight returns the usable body height on the current page
	// after subtracting space already reserved for footnotes.
	effectiveContentHeight := func(geom PageGeometry) float64 {
		base := pageContentHeight(geom)
		if currentPage >= 0 && currentPage < len(pageFootnoteH) {
			base -= pageFootnoteH[currentPage]
			if base < 1 {
				base = 1
			}
		}
		return base
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
				pageFootnoteIDs[currentPage] = append(pageFootnoteIDs[currentPage], run.NoteID)
				if story, ok := doc.Footnotes[run.NoteID]; ok {
					h := estimateStoryHeightPt(doc, story, footnoteW)
					pageFootnoteH[currentPage] += h
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
			for j := range blockFloats {
				blockFloats[j].Page = pages[currentPage].Number
			}
			pages[currentPage].Floats = append(pages[currentPage].Floats, blockFloats...)
			currentY += height
			columnHasContent = true
		}
		emitExtraBlocks := func(extraBlocks []resolve.Block) {
			for i := range extraBlocks {
				block := &extraBlocks[i]
				height, blockFloats := measureBlock(doc, extraBlocks, i, currentGeom, currentColumn, currentY, pages[currentPage].Floats)
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
			if p.Frame != nil || p.Format.KeepLines || paragraphHasFloatingContent(p) {
				return false
			}
			prevPara := adjacentBlock(blockList, index, -1)
			nextPara := adjacentBlock(blockList, index, +1)
			spacingBefore := ParagraphSpacingBefore(doc, p, prevPara, geom, y == 0)
			metrics, ok := paragraphFlowMetricsForPagination(doc, p, geom, column, y+geom.MarginTop, spacingBefore, activeFloats)
			metrics.spacingBefore = spacingBefore
			metrics.spacingAfter = ParagraphSpacingAfter(doc, p, nextPara, geom)
			if !ok || len(metrics.lineHeights) == 0 || y+metrics.totalHeight() <= availH {
				return false
			}

			startLine := 0
			for startLine < len(metrics.lineHeights) {
				remainingHeight := effectiveContentHeight(currentGeom) - currentY
				linesToTake, fragmentHeight := metrics.fragmentFit(startLine, remainingHeight, p.Format.WidowControl)
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
				fragmentRuns, fragmentText := metrics.fragmentContent(startLine, endLine)
				fragment := &ParagraphFragment{
					StartLine:      startLine,
					EndLine:        endLine,
					Runs:           fragmentRuns,
					Text:           fragmentText,
					IsContinuation: startLine > 0,
					IsLast:         endLine == len(metrics.lineHeights),
				}
				addFootnotesForRuns(fragmentRuns, currentGeom)
				emitBlockBox(block, currentGeom, fragmentHeight, nil, fragment)
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
			baseSpacingBefore := ParagraphSpacingBefore(doc, p, prevPara, currentGeom, currentY == 0)
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
				height := paragraphHeightInFlow(doc, &segmentPara, geom, currentColumn, currentY+geom.MarginTop, pages[currentPage].Floats)
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
					height = paragraphHeightInFlow(doc, &segmentPara, geom, currentColumn, currentY+geom.MarginTop, pages[currentPage].Floats)
					if segIndex == 0 {
						height += baseSpacingBefore
					}
					if segIndex == len(segments)-1 {
						height += baseSpacingAfter
					}
				}

				blockFloats := appendParagraphFloats(nil, segmentPara.Runs, geom, currentColumn, currentY+geom.MarginTop, height)
				addFootnotesForRuns(segmentPara.Runs, geom)
				emitBlockBox(block, geom, height, blockFloats, &ParagraphFragment{
					Runs:            segmentPara.Runs,
					Text:            segmentPara.Text,
					IsContinuation:  segIndex > 0,
					IsLast:          segIndex == len(segments)-1,
					StartsNewColumn: segIndex > 0 && currentColumn > 0,
				})
			}
			return true
		}
		trySplitTableAcrossFlow := func(block *resolve.Block) bool {
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
			totalHeight := sumFloat64s(rowHeights)
			if currentY+totalHeight <= effectiveContentHeight(currentGeom) {
				return false
			}

			headerEnd := leadingTableHeaderRows(t)
			startRow := 0
			for startRow < len(t.Rows) {
				repeatHeaders := startRow >= headerEnd && startRow > 0 && headerEnd > 0
				headerHeight := 0.0
				if repeatHeaders {
					headerHeight = sumFloat64s(rowHeights[:headerEnd])
				}

				remainingHeight := effectiveContentHeight(currentGeom) - currentY
				if remainingHeight <= headerHeight && columnHasContent {
					previousPage := currentPage
					emitEpilogue()
					advanceFlow(section)
					if pagePrelude != nil && currentPage != previousPage {
						emitPrelude()
					}
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
					if tableSplitBoundarySafe(t, endRow) {
						lastSafeEnd = endRow
						lastSafeHeight = fragmentHeight
					}
				}

				// A page boundary must not cut through a vertical merge. Prefer the
				// last complete merge group that fitted. If the first group cannot fit
				// in the remaining region, move it to a fresh region; if it is taller
				// than a whole page, keep the complete group atomic and let it overflow.
				if endRow < len(t.Rows) && !tableSplitBoundarySafe(t, endRow) {
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
							if tableSplitBoundarySafe(t, endRow) {
								break
							}
						}
					}
				}

				// Keep a leading header group with at least one body row when the
				// whole group can fit at the top of a fresh flow region.
				if startRow == 0 && headerEnd > 0 && endRow == headerEnd && headerEnd < len(t.Rows) && columnHasContent {
					need := sumFloat64s(rowHeights[:headerEnd+1])
					if need <= effectiveContentHeight(currentGeom) {
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
				}
			}
			return true
		}
		pendingPrelude := false
		for i := range blockList {
			block := &blockList[i]
			if pendingPrelude {
				emitPrelude()
				pendingPrelude = false
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
			reservedAvailH := effectiveContentHeight(geom) - epilogueReserveHeight
			if reservedAvailH < 1 {
				reservedAvailH = 1
			}
			if clusterEnd > i {
				clusterHeight := measureBlockClusterHeight(doc, blockList, i, clusterEnd, geom)
				availH := effectiveContentHeight(geom)
				if clusterEnd < len(blockList)-1 && pageEpilogue != nil {
					availH = reservedAvailH
				}
				if clusterHeight <= availH && clusterHeight > availH-currentY && columnHasContent {
					emitEpilogue()
					advanceFlow(section)
					geom = currentGeom
				}
			}

			height, blockFloats := measureBlock(doc, blockList, i, geom, currentColumn, currentY, pages[currentPage].Floats)
			availH := effectiveContentHeight(geom)
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
				reservedAvailH = effectiveContentHeight(geom) - epilogueReserveHeight
				if reservedAvailH < 1 {
					reservedAvailH = 1
				}
				height, blockFloats = measureBlock(doc, blockList, i, geom, currentColumn, currentY, pages[currentPage].Floats)
				availH = effectiveContentHeight(geom)
			}
			if trySplitParagraphOnColumnBreak(i, block) {
				if endnoteCollector != nil {
					endnoteCollector(block)
				}
				continue
			}
			if trySplitParagraphAcrossFlow(i, block, geom, currentColumn, currentY, pages[currentPage].Floats, availH) {
				if endnoteCollector != nil {
					endnoteCollector(block)
				}
				continue
			}
			if trySplitTableAcrossFlow(block) {
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
				height, blockFloats = measureBlock(doc, blockList, i, geom, currentColumn, currentY, pages[currentPage].Floats)
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
				height, blockFloats = measureBlock(doc, blockList, i, geom, currentColumn, currentY, pages[currentPage].Floats)
			}

			if block != nil && block.Paragraph != nil {
				addFootnotesForRuns(block.Paragraph.Runs, geom)
			}
			if endnoteCollector != nil {
				endnoteCollector(block)
			}

			emitBlockBox(block, geom, height, blockFloats, nil)

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
			flowBlocks(span.Section, materializeEndnoteBlocks(doc, spanEndnoteIDs, &nextSyntheticIndex), nil,
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
		flowBlocks(finalSection, materializeEndnoteBlocks(doc, docEndnoteIDs, &nextSyntheticIndex), nil,
			func() []resolve.Block { return continuationSeparatorBlocks(doc, &nextSyntheticIndex) },
			func() []resolve.Block { return continuationNoticeBlocks(doc, &nextSyntheticIndex) },
		)
	}
	materializePageFootnotes(doc, pages, pageFootnoteIDs, &nextSyntheticIndex)
	return pages
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
		blocks, nextNumber := materializeFootnoteBlocks(doc, footnoteIDs[pageIndex], number, format, nextSyntheticIndex)
		if len(blocks) == 0 {
			continue
		}
		number = nextNumber
		geom := page.Geometry
		footnoteGeom := footnoteLayoutGeometry(geom)
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

func materializeFootnoteBlocks(doc *resolve.Document, noteIDs []int, startNumber int, noteFormat string, nextSyntheticIndex *int) ([]resolve.Block, int) {
	if doc == nil || len(noteIDs) == 0 {
		return nil, startNumber
	}
	blocks := make([]resolve.Block, 0, len(noteIDs)+1)
	blocks = append(blocks, footnoteSeparatorBlocks(doc, nextSyntheticIndex)...)
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

func materializeEndnoteBlocks(doc *resolve.Document, noteIDs []int, nextSyntheticIndex *int) []resolve.Block {
	if doc == nil || len(noteIDs) == 0 {
		return nil
	}
	blocks := make([]resolve.Block, 0, len(noteIDs)+1)
	blocks = append(blocks, startSeparatorBlocks(doc, nextSyntheticIndex)...)
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

func startSeparatorBlocks(doc *resolve.Document, nextSyntheticIndex *int) []resolve.Block {
	if doc != nil && storyHasContent(doc.EndnoteSeparator) {
		return clonePlainStoryBlocksForLayout(doc.EndnoteSeparator, nextSyntheticIndex)
	}
	return []resolve.Block{makeNoteSeparatorBlock(nextSyntheticIndex)}
}

func footnoteSeparatorBlocks(doc *resolve.Document, nextSyntheticIndex *int) []resolve.Block {
	if doc != nil && storyHasContent(doc.FootnoteSeparator) {
		return clonePlainStoryBlocksForLayout(doc.FootnoteSeparator, nextSyntheticIndex)
	}
	return []resolve.Block{makeNoteSeparatorBlock(nextSyntheticIndex)}
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

func makeNoteSeparatorBlock(nextSyntheticIndex *int) resolve.Block {
	separator := resolve.Paragraph{
		Index: nextLayoutSyntheticIndex(nextSyntheticIndex),
		Format: resolve.ParagraphFormat{
			SpacingAfter: layoutIntPtr(120),
			KeepNext:     true,
			Borders: &resolve.ParagraphBorders{
				Top: &resolve.BorderSide{Val: "single", SizePt: 0.75, Color: "444444"},
			},
		},
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
			clearancePt := defaultFontSizePt(doc) * (cssNormalLineRatio - 1.0)
			if clearancePt < 2.0 {
				clearancePt = 2.0
			}
			requiredFooterPt := storyH + clearancePt
			if requiredFooterPt > allocatedFooterPt {
				g.MarginBottom += requiredFooterPt - allocatedFooterPt
			}
		}
	}

	return g
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
	return layoutBlocks(blockList, doc, geom, 1)
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
		height = paragraphHeightInFlow(doc, p, geom, column, y+geom.MarginTop, activeFloats)
		height += ParagraphSpacingBefore(doc, p, prevPara, geom, y == 0)
		height += ParagraphSpacingAfter(doc, p, nextPara, geom)
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
		// For drop caps, height is lines × document line height — not the drop cap font size.
		if fp.DropCap != "" && fp.DropCap != "none" && fp.Lines > 0 {
			lineH := defaultFontSizePt(doc) * cssNormalLineRatio
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
	if p.Format.IndentLeft != nil && *p.Format.IndentLeft > 0 {
		widthPt += twipsToPoints(*p.Format.IndentLeft)
	}
	if p.Format.IndentFirstLine != nil && *p.Format.IndentFirstLine > 0 {
		widthPt += twipsToPoints(*p.Format.IndentFirstLine)
	}

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
	tokens := paragraphRenderTokens(p, defaultSizePt)
	if len(tokens) == 0 {
		return 0
	}
	spaceWidthPt := defaultSizePt * 0.28
	widthPt := 0.0
	for i, token := range tokens {
		if i > 0 {
			widthPt += spaceWidthPt
		}
		widthPt += token.widthPt
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
		end++
	}
	return end
}

func blockKeepsNext(block *resolve.Block) bool {
	return block != nil && block.Kind == "paragraph" && block.Paragraph != nil && block.Paragraph.Format.KeepNext
}

func measureBlockClusterHeight(doc *resolve.Document, blocks []resolve.Block, start, end int, geom PageGeometry) float64 {
	if start < 0 {
		start = 0
	}
	if end >= len(blocks) {
		end = len(blocks) - 1
	}
	total := 0.0
	for i := start; i <= end; i++ {
		height, _ := measureBlock(doc, blocks, i, geom, 0, 0, nil)
		total += height
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

func blockForcesPageBreakBefore(block *resolve.Block) bool {
	return block != nil && block.Kind == "paragraph" && block.Paragraph != nil && block.Paragraph.Format.PageBreakBefore
}

func blockForcesPageBreakAfter(block *resolve.Block) bool {
	if block == nil || block.Kind != "paragraph" || block.Paragraph == nil {
		return false
	}
	for _, run := range block.Paragraph.Runs {
		if run.Kind == "break" && run.BreakType == "page" {
			return true
		}
	}
	return false
}

func paragraphHeightInFlow(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry, column int, paraPageY float64, activeFloats []FloatBox) float64 {
	baseWidthPt := paragraphContentWidthPt(p, geom)
	firstWidthPt, continuationWidthPt := paragraphLineWidthsPt(p, baseWidthPt)
	defaultSizePt := defaultFontSizePt(doc)
	fontSizePt := paragraphFontSizePt(doc, p)
	lineHeight := paragraphLineHeightPt(p, geom, fontSizePt)
	topBorderOutset, bottomBorderOutset := paragraphVerticalBorderOutsetPt(p)
	leadingBlockFloatHeight := paragraphLeadingBlockFloatHeightPt(p)
	activeFloatClearance := paragraphTopAndBottomClearancePt(paraPageY+leadingBlockFloatHeight, lineHeight, activeFloats)

	var spacingBefore, spacingAfter float64
	if paragraphContainsOnlyLeadingBlockFloatImageForLayout(p) || paragraphContainsOnlyLeadingBlockFloatTextBoxForLayout(p) || paragraphContainsOnlyLeadingBlockFloatShapeForLayout(p) {
		return spacingBefore + topBorderOutset + leadingBlockFloatHeight + bottomBorderOutset + spacingAfter
	}

	contentLeftPt := paragraphContentLeftPt(p, columnLeftPt(geom, column))
	firstLeftPt := paragraphFirstLineLeftPt(p, contentLeftPt)
	contentRightPt := contentLeftPt + continuationWidthPt

	lineMetrics := estimateParagraphLineMetricsWithWidths(p, firstWidthPt, continuationWidthPt, defaultSizePt, nil)
	lineHeights := paragraphLineHeightsPt(p, geom, lineMetrics, defaultSizePt)
	contentHeight := sumFloat64s(lineHeights)
	for range 2 {
		exclusions := paragraphWrapExclusions(p, geom, column, paraPageY, spacingBefore+contentHeight+spacingAfter, activeFloats)
		lineMetrics = estimateParagraphLineMetricsWithWidths(p, firstWidthPt, continuationWidthPt, defaultSizePt, func(lineIndex int) float64 {
			lineTop := paraPageY + spacingBefore + leadingBlockFloatHeight + activeFloatClearance + cumulativeLineHeight(lineHeights, lineIndex)
			lineBottom := lineTop + lineHeightAt(lineHeights, lineIndex, lineHeight)
			lineLeft := contentLeftPt
			lineRight := contentRightPt
			if lineIndex == 0 {
				lineLeft = firstLeftPt
				lineRight = firstLeftPt + firstWidthPt
			}
			return lineAvailableWidthPt(lineLeft, lineRight, lineTop, lineBottom, exclusions)
		})
		lineHeights = paragraphLineHeightsPt(p, geom, lineMetrics, defaultSizePt)
		newContentHeight := sumFloat64s(lineHeights)
		if almostSame(newContentHeight, contentHeight) {
			contentHeight = newContentHeight
			break
		}
		contentHeight = newContentHeight
	}

	return spacingBefore + topBorderOutset + leadingBlockFloatHeight + activeFloatClearance + contentHeight + bottomBorderOutset + spacingAfter
}

type paragraphFlowMetrics struct {
	lineHeights        []float64
	lineWidths         []float64
	lineRuns           [][]resolve.Run
	spacingBefore      float64
	spacingAfter       float64
	topBorderOutset    float64
	bottomBorderOutset float64
	topClearance       float64
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
		if endLine == len(m.lineHeights) {
			candidateHeight += m.bottomBorderOutset + m.spacingAfter
		}
		if candidateHeight > availableHeight {
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
		if startLine > 0 && bestLines < 2 {
			return 0, 0
		}
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

func (m paragraphFlowMetrics) fragmentContent(startLine, endLine int) ([]resolve.Run, string) {
	if startLine < 0 {
		startLine = 0
	}
	if endLine > len(m.lineRuns) {
		endLine = len(m.lineRuns)
	}
	if startLine >= endLine {
		return nil, ""
	}
	runs := make([]resolve.Run, 0)
	for i := startLine; i < endLine; i++ {
		runs = append(runs, m.lineRuns[i]...)
	}
	return runs, paragraphTextFromRuns(runs)
}

func paragraphFlowMetricsForPagination(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry, column int, paraPageY, spacingBefore float64, activeFloats []FloatBox) (paragraphFlowMetrics, bool) {
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
	contentRightPt := contentLeftPt + continuationWidthPt
	lineStartY := paraPageY + spacingBefore + topBorderOutset
	topClearance := paragraphTopAndBottomClearancePt(lineStartY, lineHeight, activeFloats)

	lineMetrics := estimateParagraphLineMetricsWithWidths(p, firstWidthPt, continuationWidthPt, defaultSizePt, nil)
	lineHeights := paragraphLineHeightsPt(p, geom, lineMetrics, defaultSizePt)
	lineWidths := make([]float64, len(lineHeights))
	for range 3 {
		exclusions := paragraphWrapExclusions(p, geom, column, paraPageY, topClearance+sumFloat64s(lineHeights), activeFloats)
		lineWidths = make([]float64, len(lineHeights))
		lineMetrics = estimateParagraphLineMetricsWithWidths(p, firstWidthPt, continuationWidthPt, defaultSizePt, func(lineIndex int) float64 {
			lineTop := lineStartY + topClearance + cumulativeLineHeight(lineHeights, lineIndex)
			lineBottom := lineTop + lineHeightAt(lineHeights, lineIndex, lineHeight)
			lineLeft := contentLeftPt
			lineRight := contentRightPt
			if lineIndex == 0 {
				lineLeft = firstLeftPt
				lineRight = firstLeftPt + firstWidthPt
			}
			available := lineAvailableWidthPt(lineLeft, lineRight, lineTop, lineBottom, exclusions)
			if lineIndex < len(lineWidths) {
				lineWidths[lineIndex] = available
			}
			return available
		})
		newLineHeights := paragraphLineHeightsPt(p, geom, lineMetrics, defaultSizePt)
		if len(newLineHeights) == len(lineHeights) && almostSame(sumFloat64s(newLineHeights), sumFloat64s(lineHeights)) {
			lineHeights = newLineHeights
			break
		}
		lineHeights = newLineHeights
	}

	// The estimator may have changed the number of lines on the last iteration.
	// Recompute the actual per-line widths against the converged vertical positions
	// before slicing runs into page/column fragments.
	exclusions := paragraphWrapExclusions(p, geom, column, paraPageY, topClearance+sumFloat64s(lineHeights), activeFloats)
	lineWidths = make([]float64, len(lineHeights))
	for i := range lineWidths {
		lineTop := lineStartY + topClearance + cumulativeLineHeight(lineHeights, i)
		lineBottom := lineTop + lineHeightAt(lineHeights, i, lineHeight)
		lineLeft := contentLeftPt
		lineRight := contentRightPt
		if i == 0 {
			lineLeft = firstLeftPt
			lineRight = firstLeftPt + firstWidthPt
		}
		lineWidths[i] = lineAvailableWidthPt(lineLeft, lineRight, lineTop, lineBottom, exclusions)
	}
	lineRuns := paragraphFragmentRunLines(p, lineWidths, defaultSizePt)
	if len(lineRuns) != len(lineHeights) {
		return paragraphFlowMetrics{}, false
	}
	return paragraphFlowMetrics{
		lineHeights:        lineHeights,
		lineWidths:         lineWidths,
		lineRuns:           lineRuns,
		topBorderOutset:    topBorderOutset,
		bottomBorderOutset: bottomBorderOutset,
		topClearance:       topClearance,
	}, true
}

func paragraphHasFloatingContent(p *resolve.Paragraph) bool {
	if p == nil {
		return false
	}
	for _, run := range p.Runs {
		if run.Image != nil && run.Image.Floating {
			return true
		}
		if run.TextBox != nil && run.TextBox.Floating {
			return true
		}
		if run.Shape != nil && run.Shape.Floating {
			return true
		}
	}
	return false
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

// ParagraphSpacingBefore returns the space-before contribution of p (not including
// the previous paragraph's space-after). Callers sum Before + After across adjacent
// paragraphs to get the correct Word-model additive gap.
func ParagraphSpacingBefore(doc *resolve.Document, p, prev *resolve.Paragraph, geom PageGeometry, atFlowStart bool) float64 {
	if p == nil || atFlowStart {
		return 0
	}
	if ContextualSpacingSuppresses(p, prev) {
		return 0
	}
	return resolvedParagraphSpacingBeforePt(doc, p, geom)
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
	defaultSizePt := defaultFontSizePt(doc)
	fontSizePt := paragraphFontSizePt(doc, p)
	firstWidthPt, continuationWidthPt := paragraphLineWidthsPt(p, widthPt)
	lineHeight := paragraphLineHeightPt(p, geom, fontSizePt)
	topBorderOutset, bottomBorderOutset := paragraphVerticalBorderOutsetPt(p)

	lineMetrics := estimateParagraphLineMetricsWithWidths(p, firstWidthPt, continuationWidthPt, defaultSizePt, nil)
	lineHeights := paragraphLineHeightsPt(p, geom, lineMetrics, defaultSizePt)
	contentHeight := sumFloat64s(lineHeights)
	if contentHeight == 0 {
		contentHeight = lineHeight
	}
	return topBorderOutset + contentHeight + bottomBorderOutset
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

// tableSplitBoundarySafe reports whether a table fragment may end immediately
// before row endRow. A w:vMerge continuation in the next row means the merge
// started in an earlier row and therefore crosses this boundary. Keeping such
// groups together preserves valid rowspan geometry in every rendered fragment.
func tableSplitBoundarySafe(t *resolve.Table, endRow int) bool {
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
	width := TableWidthPt(t, availableWidthPt)
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
	return heights
}

// appendParagraphFloats collects every floating object from runs and appends
// a FloatBox with its absolute page position to floats.
//
// paraPageY is the paragraph's top edge in page-absolute coordinates
// (i.e., already includes the page margin-top).
func appendParagraphFloats(floats []FloatBox, runs []resolve.Run, geom PageGeometry, column int, paraPageY, paraHeight float64) []FloatBox {
	legacyFlow := legacyShapeFlowState{
		left:  columnLeftPt(geom, column),
		width: columnWidthPt(geom, column),
		y:     paraPageY + paraHeight,
	}
	if legacyFlow.width <= 0 {
		legacyFlow.width = geom.Width - geom.MarginLeft - geom.MarginRight
	}
	for i := range runs {
		run := &runs[i]
		switch run.Kind {
		case "image":
			img := run.Image
			if img == nil || !img.Floating {
				continue
			}
			widthPt := emuToPoints(img.Width)
			heightPt := emuToPoints(img.Height)
			if img.SizeRelHPct > 0 {
				widthPt = relativeObjectSizePt(img.SizeRelHPct, img.SizeRelHFrom, true, geom, column)
			}
			if img.SizeRelVPct > 0 {
				heightPt = relativeObjectSizePt(img.SizeRelVPct, img.SizeRelVFrom, false, geom, column)
			}
			x := resolveX(img.X, img.XRelative, img.XAlign, geom, column, widthPt)
			y := resolveY(img.Y, img.YRelative, img.YAlign, geom, paraPageY, paraHeight, heightPt)
			floats = append(floats, FloatBox{
				Image:     img,
				X:         x,
				Y:         y,
				Width:     widthPt,
				Height:    heightPt,
				ZIndex:    img.ZIndex,
				BehindDoc: img.BehindDoc,
			})
		case "textbox":
			tb := run.TextBox
			if tb == nil || !tb.Floating {
				continue
			}
			widthPt := emuToPoints(tb.Width)
			heightPt := emuToPoints(tb.Height)
			if tb.SizeRelHPct > 0 {
				widthPt = relativeObjectSizePt(tb.SizeRelHPct, tb.SizeRelHFrom, true, geom, column)
			}
			if tb.SizeRelVPct > 0 {
				heightPt = relativeObjectSizePt(tb.SizeRelVPct, tb.SizeRelVFrom, false, geom, column)
			}
			x := resolveX(tb.X, tb.XRelative, tb.XAlign, geom, column, widthPt)
			y := resolveY(tb.Y, tb.YRelative, tb.YAlign, geom, paraPageY, paraHeight, heightPt)
			floats = append(floats, FloatBox{
				TextBox:   tb,
				X:         x,
				Y:         y,
				Width:     widthPt,
				Height:    heightPt,
				ZIndex:    tb.ZIndex,
				BehindDoc: tb.BehindDoc,
			})
		case "shape":
			sh := run.Shape
			if sh == nil || !sh.Floating {
				continue
			}
			widthPt := emuToPoints(sh.Width)
			heightPt := emuToPoints(sh.Height)
			if sh.SizeRelHPct > 0 {
				widthPt = relativeObjectSizePt(sh.SizeRelHPct, sh.SizeRelHFrom, true, geom, column)
			}
			if sh.SizeRelVPct > 0 {
				heightPt = relativeObjectSizePt(sh.SizeRelVPct, sh.SizeRelVFrom, false, geom, column)
			}
			var x, y float64
			if sh.LegacyFlowAnchor {
				x, y = legacyFlow.place(sh, widthPt, heightPt)
			} else {
				x = resolveX(sh.X, sh.XRelative, sh.XAlign, geom, column, widthPt)
				y = resolveY(sh.Y, sh.YRelative, sh.YAlign, geom, paraPageY, paraHeight, heightPt)
			}
			floats = append(floats, FloatBox{
				Shape:     sh,
				X:         x,
				Y:         y,
				Width:     widthPt,
				Height:    heightPt,
				ZIndex:    sh.ZIndex,
				BehindDoc: sh.BehindDoc,
			})
		}
	}
	return floats
}

type legacyShapeFlowState struct {
	left   float64
	width  float64
	cursor float64
	y      float64
	rowH   float64
}

func (s *legacyShapeFlowState) place(shape *resolve.Shape, widthPt, heightPt float64) (float64, float64) {
	if s == nil || shape == nil {
		return 0, 0
	}
	localX := emuToPoints(shape.X)
	localY := emuToPoints(shape.Y)
	advanceX := widthPt
	if localX > 0 {
		advanceX += localX
	}

	if s.cursor > 0 && s.width > 0 && s.cursor+advanceX > s.width {
		s.cursor = 0
		s.y += s.rowH
		s.rowH = 0
	}

	x := s.left + s.cursor + localX
	y := s.y + localY

	// Rotation can extend beyond the unrotated Spa rectangle. Reserve the
	// actual lower bound in the anchor-flow row so the next row does not
	// collide with a rotated shape.
	rowBottom := localY + heightPt
	if poly := rotatedRectPolygon(s.cursor+localX, localY, widthPt, heightPt, shape.RotationDeg); len(poly) > 0 {
		if _, _, _, bottom, ok := polygonBounds(poly); ok && bottom > rowBottom {
			rowBottom = bottom
		}
	}
	if rowBottom > s.rowH {
		s.rowH = rowBottom
	}
	s.cursor += advanceX
	return x, y
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
	if doc.DefaultRunFormat.FontSize != nil {
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

func paragraphLineHeightPt(p *resolve.Paragraph, geom PageGeometry, fontSizePt float64) float64 {
	inlineHeightPt := paragraphInlineImageHeightPt(p)
	lineHeight := math.Max(fontSizePt*cssNormalLineRatio, inlineHeightPt)
	if p == nil || p.Format.LineSpacing == nil {
		return snapLineHeightToDocGrid(p, geom, lineHeight)
	}
	switch p.Format.LineRule {
	case "auto", "":
		lineHeight = math.Max(autoParagraphLineHeightPt(p, fontSizePt), inlineHeightPt)
	case "exact":
		lineHeight = twipsToPoints(*p.Format.LineSpacing)
	case "atLeast":
		lineHeight = math.Max(lineHeight, twipsToPoints(*p.Format.LineSpacing))
	}
	return snapLineHeightToDocGrid(p, geom, lineHeight)
}

func autoParagraphLineHeightPt(p *resolve.Paragraph, fontSizePt float64) float64 {
	if p == nil || p.Format.LineSpacing == nil || *p.Format.LineSpacing <= 0 {
		return fontSizePt * cssNormalLineRatio
	}
	multiple := float64(*p.Format.LineSpacing) / 240.0
	// Word stores auto line spacing as a line multiple for both OOXML and
	// binary DOC. 240=single, 360=1.5 lines, 480=double. CSS `normal` is
	// approximately 1.2, so preserve that floor for single spacing without
	// multiplying authored 1.5/double spacing by 1.2 a second time.
	return math.Max(cssNormalLineRatio, multiple) * fontSizePt
}

func paragraphInlineImageHeightPt(p *resolve.Paragraph) float64 {
	maxHeight := 0.0
	if p == nil {
		return maxHeight
	}
	for _, run := range p.Runs {
		if run.Kind != "image" || run.Image == nil || run.Image.Floating {
			continue
		}
		if heightPt := emuToPoints(run.Image.Height); heightPt > maxHeight {
			maxHeight = heightPt
		}
	}
	return maxHeight
}

// estimateParagraphLineCount estimates how many lines a paragraph occupies.
//
// Fix 1: instead of a single 0.5× character width factor, this function
// iterates over runs and uses per-character Unicode width factors and per-run
// font sizes, producing a much better estimate for mixed-script and mixed-size
// paragraphs.
func estimateParagraphLineCount(p *resolve.Paragraph, widthPt, defaultSizePt float64) int {
	return estimateParagraphLineCountWithWidths(p, widthPt, widthPt, defaultSizePt, nil)
}

func estimateParagraphLineCountWithWidths(p *resolve.Paragraph, firstWidthPt, continuationWidthPt, defaultSizePt float64, widthForLine func(int) float64) int {
	if p == nil {
		return 1
	}
	if firstWidthPt <= 1 && continuationWidthPt <= 1 {
		return 1
	}
	tokens := paragraphLineTokens(p, defaultSizePt)

	if len(tokens) == 0 {
		// Fallback: use raw paragraph text with average proportional width.
		text := strings.TrimSpace(p.Text)
		if text == "" {
			return 1
		}
		avgCharWidthPt := defaultSizePt * 0.45
		if avgCharWidthPt <= 0 {
			avgCharWidthPt = 6
		}
		widthPt := firstWidthPt
		if continuationWidthPt > widthPt {
			widthPt = continuationWidthPt
		}
		charsPerLine := int(widthPt / avgCharWidthPt)
		if charsPerLine < 1 {
			charsPerLine = 1
		}
		lineCount := 0
		for _, segment := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
			segment = strings.TrimSpace(segment)
			if segment == "" {
				lineCount++
				continue
			}
			lineCount += estimateWrappedLineCount(segment, charsPerLine)
		}
		if lineCount < 1 {
			return 1
		}
		return lineCount
	}

	currentLineWidth := func(lineIndex int) float64 {
		if widthForLine != nil {
			return widthForLine(lineIndex)
		}
		if lineIndex == 0 {
			if firstWidthPt > 1 {
				return firstWidthPt
			}
			return 1
		}
		if continuationWidthPt > 1 {
			return continuationWidthPt
		}
		return 1
	}

	const maxLineSearch = 1000

	lineCount := 1
	lineWidthPt := currentLineWidth(0)
	for lineWidthPt <= 1 && lineCount <= maxLineSearch {
		lineCount++
		lineWidthPt = currentLineWidth(lineCount - 1)
	}
	currentWidthPt := 0.0
	previousFontSizePt := 0.0
	for _, token := range tokens {
		spaceWidthPt := interWordSpaceWidthPt(previousFontSizePt, token.fontSizePt, defaultSizePt)
		needed := token.widthPt
		if currentWidthPt > 0 {
			needed += spaceWidthPt
		}
		if currentWidthPt == 0 || currentWidthPt+needed <= lineWidthPt {
			if currentWidthPt > 0 {
				currentWidthPt += spaceWidthPt
			}
			currentWidthPt += token.widthPt
			previousFontSizePt = token.fontSizePt
			continue
		}

		lineCount++
		lineWidthPt = currentLineWidth(lineCount - 1)
		for lineWidthPt <= 1 && lineCount <= maxLineSearch {
			lineCount++
			lineWidthPt = currentLineWidth(lineCount - 1)
		}
		currentWidthPt = token.widthPt
		previousFontSizePt = token.fontSizePt
	}
	if lineCount < 1 {
		return 1
	}
	return lineCount
}

type paragraphToken struct {
	widthPt        float64
	fontSizePt     float64
	inlineHeightPt float64
}

type paragraphRenderToken struct {
	run        resolve.Run
	widthPt    float64
	fontSizePt float64
}

func runWordWidthPt(format resolve.RunFormat, rawWord string, defaultSizePt float64) (float64, float64) {
	fontSizePt := defaultSizePt
	if format.FontSize != nil && *format.FontSize > 0 {
		fontSizePt = float64(*format.FontSize) / 2.0
	}
	fontName := preferredRunFontName(format)
	monospace := isFontMonospace(fontName)
	widthScale := fontWidthScale(fontName)
	text := rawWord
	if format.Caps != nil && *format.Caps {
		text = strings.ToUpper(text)
	}
	widthPt := 0.0
	runeCount := 0
	for _, ch := range text {
		widthPt += charWidthFactor(ch, monospace) * fontSizePt * widthScale
		runeCount++
	}
	// CSS letter-spacing is part of the rendered advance but historically was
	// omitted from pagination. Preserve the authored twentieths-of-a-point value
	// between adjacent glyphs so layout and HTML wrap the same words.
	if format.CharacterSpacing != nil && *format.CharacterSpacing != 0 && runeCount > 1 {
		widthPt += float64(*format.CharacterSpacing) / 20.0 * float64(runeCount-1)
	}
	return widthPt, fontSizePt
}

func interWordSpaceWidthPt(previousFontSizePt, currentFontSizePt, defaultSizePt float64) float64 {
	fontSizePt := math.Max(previousFontSizePt, currentFontSizePt)
	if fontSizePt <= 0 {
		fontSizePt = defaultSizePt
	}
	return fontSizePt * 0.28
}

func paragraphLineTokens(p *resolve.Paragraph, defaultSizePt float64) []paragraphToken {
	if p == nil {
		return nil
	}
	tokens := make([]paragraphToken, 0, len(p.Runs))
	for _, run := range p.Runs {
		switch run.Kind {
		case "", "run":
			if run.Text == "" {
				continue
			}
			for _, rawWord := range strings.Fields(run.Text) {
				wpt, fontSizePt := runWordWidthPt(run.Format, rawWord, defaultSizePt)
				tokens = append(tokens, paragraphToken{widthPt: wpt, fontSizePt: fontSizePt})
			}
		case "image":
			if run.Image == nil || run.Image.Floating {
				continue
			}
			wpt := emuToPoints(run.Image.Width)
			if wpt <= 0 {
				wpt = 1
			}
			tokens = append(tokens, paragraphToken{widthPt: wpt, fontSizePt: defaultSizePt, inlineHeightPt: emuToPoints(run.Image.Height)})
		}
	}
	return tokens
}

func paragraphRenderTokens(p *resolve.Paragraph, defaultSizePt float64) []paragraphRenderToken {
	if p == nil {
		return nil
	}
	tokens := make([]paragraphRenderToken, 0, len(p.Runs))
	for _, run := range p.Runs {
		switch run.Kind {
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
			tokens = append(tokens, paragraphRenderToken{run: run, widthPt: defaultSizePt * 0.8, fontSizePt: defaultSizePt})
			continue
		case "textbox":
			if run.TextBox == nil || run.TextBox.Floating {
				continue
			}
		case "shape":
			if run.Shape == nil || run.Shape.Floating {
				continue
			}
		}
		if run.Text == "" {
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

func paragraphFragmentRunLines(p *resolve.Paragraph, lineWidths []float64, defaultSizePt float64) [][]resolve.Run {
	if len(lineWidths) > 0 {
		narrow := true
		for _, width := range lineWidths {
			if width > 1 {
				narrow = false
				break
			}
		}
		if narrow {
			return paragraphNarrowColumnRunLines(p)
		}
	}
	tokens := paragraphRenderTokens(p, defaultSizePt)
	if len(tokens) == 0 {
		return nil
	}
	widthForLine := func(lineIndex int) float64 {
		if lineIndex >= 0 && lineIndex < len(lineWidths) && lineWidths[lineIndex] > 1 {
			return lineWidths[lineIndex]
		}
		if len(lineWidths) > 0 && lineWidths[len(lineWidths)-1] > 1 {
			return lineWidths[len(lineWidths)-1]
		}
		return 1
	}

	lineIndex := 0
	lineWidthPt := widthForLine(lineIndex)
	currentWidthPt := 0.0
	currentLine := make([]resolve.Run, 0, 8)
	lines := make([][]resolve.Run, 0, len(lineWidths))
	flushLine := func() {
		if len(currentLine) == 0 {
			return
		}
		lines = append(lines, append([]resolve.Run(nil), currentLine...))
		currentLine = currentLine[:0]
		currentWidthPt = 0
		lineIndex++
		lineWidthPt = widthForLine(lineIndex)
	}

	previousFontSizePt := 0.0
	for _, token := range tokens {
		spaceWidthPt := interWordSpaceWidthPt(previousFontSizePt, token.fontSizePt, defaultSizePt)
		needed := token.widthPt
		if currentWidthPt > 0 {
			needed += spaceWidthPt
		}
		if currentWidthPt > 0 && currentWidthPt+needed > lineWidthPt {
			flushLine()
			previousFontSizePt = 0
			spaceWidthPt = interWordSpaceWidthPt(0, token.fontSizePt, defaultSizePt)
		}
		if currentWidthPt > 0 {
			currentLine = append(currentLine, resolve.Run{Kind: "run", Text: " "})
			currentWidthPt += spaceWidthPt
		}
		currentLine = append(currentLine, token.run)
		currentWidthPt += token.widthPt
		previousFontSizePt = token.fontSizePt
	}
	flushLine()
	return lines
}

func paragraphNarrowColumnRunLines(p *resolve.Paragraph) [][]resolve.Run {
	if p == nil {
		return nil
	}
	lines := make([][]resolve.Run, 0, len([]rune(p.Text)))
	for _, run := range p.Runs {
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

func estimateParagraphLineMetricsWithWidths(p *resolve.Paragraph, firstWidthPt, continuationWidthPt, defaultSizePt float64, widthForLine func(int) float64) []paragraphLineMetric {
	if p == nil {
		return []paragraphLineMetric{{maxFontSizePt: defaultSizePt}}
	}
	if firstWidthPt <= 1 && continuationWidthPt <= 1 {
		// Extremely narrow Word columns are legal in real DOCX files (for
		// example when an authored inter-column gap consumes nearly all body
		// width). Word then wraps words down to individual glyphs. The old fast
		// path treated such a paragraph as one line, causing dozens of reference
		// pages to collapse into one or two renderer pages.
		return paragraphNarrowColumnLineMetrics(p, defaultSizePt)
	}
	tokens := paragraphLineTokens(p, defaultSizePt)
	if len(tokens) == 0 {
		return []paragraphLineMetric{{maxFontSizePt: defaultSizePt}}
	}

	currentLineWidth := func(lineIndex int) float64 {
		if widthForLine != nil {
			return widthForLine(lineIndex)
		}
		if lineIndex == 0 {
			if firstWidthPt > 1 {
				return firstWidthPt
			}
			return 1
		}
		if continuationWidthPt > 1 {
			return continuationWidthPt
		}
		return 1
	}

	const maxLineSearch = 1000

	lineWidthPt := currentLineWidth(0)
	metrics := make([]paragraphLineMetric, 0, 4)
	for lineWidthPt <= 1 && len(metrics) <= maxLineSearch {
		metrics = append(metrics, paragraphLineMetric{maxFontSizePt: defaultSizePt})
		lineWidthPt = currentLineWidth(len(metrics))
	}
	currentWidthPt := 0.0
	previousFontSizePt := 0.0
	currentMetric := paragraphLineMetric{maxFontSizePt: defaultSizePt}
	for _, token := range tokens {
		spaceWidthPt := interWordSpaceWidthPt(previousFontSizePt, token.fontSizePt, defaultSizePt)
		needed := token.widthPt
		if currentWidthPt > 0 {
			needed += spaceWidthPt
		}
		if currentWidthPt == 0 || currentWidthPt+needed <= lineWidthPt {
			if currentWidthPt > 0 {
				currentWidthPt += spaceWidthPt
			}
			currentWidthPt += token.widthPt
			previousFontSizePt = token.fontSizePt
			currentMetric.maxFontSizePt = math.Max(currentMetric.maxFontSizePt, token.fontSizePt)
			currentMetric.maxInlineHeightPt = math.Max(currentMetric.maxInlineHeightPt, token.inlineHeightPt)
			continue
		}

		metrics = append(metrics, currentMetric)
		lineWidthPt = currentLineWidth(len(metrics))
		for lineWidthPt <= 1 && len(metrics) <= maxLineSearch {
			metrics = append(metrics, paragraphLineMetric{maxFontSizePt: defaultSizePt})
			lineWidthPt = currentLineWidth(len(metrics))
		}
		currentWidthPt = token.widthPt
		previousFontSizePt = token.fontSizePt
		currentMetric = paragraphLineMetric{
			maxFontSizePt:     math.Max(defaultSizePt, token.fontSizePt),
			maxInlineHeightPt: token.inlineHeightPt,
		}
	}
	metrics = append(metrics, currentMetric)
	if len(metrics) == 0 {
		return []paragraphLineMetric{{maxFontSizePt: defaultSizePt}}
	}
	return metrics
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
				metrics = append(metrics, paragraphLineMetric{maxFontSizePt: defaultSizePt, maxInlineHeightPt: emuToPoints(run.Image.Height)})
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
	lineHeight := math.Max(fontSizePt*cssNormalLineRatio, metric.maxInlineHeightPt)
	if p == nil || p.Format.LineSpacing == nil {
		return snapLineHeightToDocGrid(p, geom, lineHeight)
	}
	switch p.Format.LineRule {
	case "auto", "":
		lineHeight = math.Max(autoParagraphLineHeightPt(p, fontSizePt), metric.maxInlineHeightPt)
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
	// When non-nil, lineAvailableWidthPt clips against the polygon per-line instead of the bbox.
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

func paragraphWrapExclusions(p *resolve.Paragraph, geom PageGeometry, column int, paraPageY, paraHeight float64, activeFloats []FloatBox) []lineExclusion {
	if p == nil && len(activeFloats) == 0 {
		return nil
	}
	localFloats := activeParagraphFloats(p, geom, column, paraPageY, paraHeight)
	exclusions := make([]lineExclusion, 0, len(activeFloats)+len(localFloats))
	for _, fl := range activeFloats {
		if exclusion, ok := lineExclusionForFloat(fl); ok {
			exclusions = append(exclusions, exclusion)
		}
	}
	for _, fl := range localFloats {
		if fl.Image != nil && paragraphBlockFloatImageEligibleForLayout(p, fl.Image) && strings.EqualFold(fl.Image.WrapType, "topAndBottom") {
			continue
		}
		if fl.TextBox != nil && paragraphBlockFloatTextBoxEligibleForLayout(p, fl.TextBox) && strings.EqualFold(fl.TextBox.WrapType, "topAndBottom") {
			continue
		}
		if fl.Shape != nil && paragraphBlockFloatShapeEligibleForLayout(p, fl.Shape) && strings.EqualFold(fl.Shape.WrapType, "topAndBottom") {
			continue
		}
		if exclusion, ok := lineExclusionForFloat(fl); ok {
			exclusions = append(exclusions, exclusion)
		}
	}
	return exclusions
}

func paragraphLeadingBlockFloatHeightPt(p *resolve.Paragraph) float64 {
	if p == nil {
		return 0
	}
	height := 0.0
	for _, run := range p.Runs {
		if run.Kind == "image" && run.Image != nil && run.Image.Floating && strings.EqualFold(run.Image.WrapType, "topAndBottom") && paragraphBlockFloatImageEligibleForLayout(p, run.Image) {
			height += emuToPoints(run.Image.Height) + run.Image.DistTop + run.Image.DistBottom
			continue
		}
		if run.Kind == "textbox" && run.TextBox != nil && run.TextBox.Floating && strings.EqualFold(run.TextBox.WrapType, "topAndBottom") && paragraphBlockFloatTextBoxEligibleForLayout(p, run.TextBox) {
			height += emuToPoints(run.TextBox.Height) + run.TextBox.DistTop + run.TextBox.DistBottom
			continue
		}
		if run.Kind == "shape" && run.Shape != nil && run.Shape.Floating && strings.EqualFold(run.Shape.WrapType, "topAndBottom") && paragraphBlockFloatShapeEligibleForLayout(p, run.Shape) {
			height += emuToPoints(run.Shape.Height) + run.Shape.DistTop + run.Shape.DistBottom
			continue
		}
		if strings.TrimSpace(run.Text) != "" {
			break
		}
	}
	return height
}

func paragraphContainsOnlyLeadingBlockFloatImageForLayout(p *resolve.Paragraph) bool {
	if p == nil {
		return false
	}
	for _, run := range p.Runs {
		if run.Kind == "image" && run.Image != nil {
			return run.Image.Floating && strings.EqualFold(run.Image.WrapType, "topAndBottom") && paragraphContainsOnlyFloatingImageForLayout(p, run.Image)
		}
		if strings.TrimSpace(run.Text) != "" {
			return false
		}
		if run.Kind == "textbox" || run.Kind == "shape" || run.Kind == "hyperlink" {
			return false
		}
	}
	return false
}

func paragraphContainsOnlyLeadingBlockFloatTextBoxForLayout(p *resolve.Paragraph) bool {
	if p == nil {
		return false
	}
	for _, run := range p.Runs {
		if run.Kind == "textbox" && run.TextBox != nil {
			return run.TextBox.Floating && strings.EqualFold(run.TextBox.WrapType, "topAndBottom") && paragraphContainsOnlyFloatingTextBoxForLayout(p, run.TextBox)
		}
		if strings.TrimSpace(run.Text) != "" {
			return false
		}
		if run.Kind == "shape" || run.Kind == "hyperlink" {
			return false
		}
	}
	return false
}

func paragraphContainsOnlyLeadingBlockFloatShapeForLayout(p *resolve.Paragraph) bool {
	if p == nil {
		return false
	}
	for _, run := range p.Runs {
		if run.Kind == "shape" && run.Shape != nil {
			return run.Shape.Floating && strings.EqualFold(run.Shape.WrapType, "topAndBottom") && paragraphContainsOnlyFloatingShapeForLayout(p, run.Shape)
		}
		if strings.TrimSpace(run.Text) != "" {
			return false
		}
		if run.Kind == "textbox" || run.Kind == "image" || run.Kind == "hyperlink" {
			return false
		}
	}
	return false
}

func paragraphContainsOnlyFloatingImageForLayout(p *resolve.Paragraph, target *resolve.Image) bool {
	if p == nil || target == nil {
		return false
	}
	found := false
	for _, run := range p.Runs {
		switch {
		case run.Kind == "image" && run.Image == target:
			found = true
		case run.Kind == "image" && run.Image != nil:
			return false
		case strings.TrimSpace(run.Text) != "":
			return false
		case run.Kind == "textbox" || run.Kind == "shape" || run.Kind == "hyperlink":
			return false
		}
	}
	return found
}

func paragraphContainsOnlyFloatingTextBoxForLayout(p *resolve.Paragraph, target *resolve.TextBox) bool {
	if p == nil || target == nil {
		return false
	}
	found := false
	for _, run := range p.Runs {
		switch {
		case run.Kind == "textbox" && run.TextBox == target:
			found = true
		case run.Kind == "textbox" && run.TextBox != nil:
			return false
		case strings.TrimSpace(run.Text) != "":
			return false
		case run.Kind == "shape" || run.Kind == "hyperlink":
			return false
		}
	}
	return found
}

func paragraphContainsOnlyFloatingShapeForLayout(p *resolve.Paragraph, target *resolve.Shape) bool {
	if p == nil || target == nil {
		return false
	}
	found := false
	for _, run := range p.Runs {
		switch {
		case run.Kind == "shape" && run.Shape == target:
			found = true
		case run.Kind == "shape" && run.Shape != nil:
			return false
		case strings.TrimSpace(run.Text) != "":
			return false
		case run.Kind == "textbox" || run.Kind == "image" || run.Kind == "hyperlink":
			return false
		}
	}
	return found
}

func paragraphBlockFloatImageEligibleForLayout(p *resolve.Paragraph, target *resolve.Image) bool {
	if paragraphContainsOnlyFloatingImageForLayout(p, target) {
		return true
	}
	if p == nil || target == nil {
		return false
	}
	targetIndex := -1
	for i, run := range p.Runs {
		if run.Kind == "image" && run.Image == target {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return false
	}
	for i := 0; i < targetIndex; i++ {
		run := p.Runs[i]
		switch {
		case strings.TrimSpace(run.Text) == "":
			continue
		case run.Kind == "image" && run.Image != nil && run.Image.Floating:
			continue
		default:
			return false
		}
	}
	return true
}

func paragraphBlockFloatShapeEligibleForLayout(p *resolve.Paragraph, target *resolve.Shape) bool {
	if paragraphContainsOnlyFloatingShapeForLayout(p, target) {
		return true
	}
	if p == nil || target == nil {
		return false
	}
	targetIndex := -1
	for i, run := range p.Runs {
		if run.Kind == "shape" && run.Shape == target {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return false
	}
	for i := 0; i < targetIndex; i++ {
		run := p.Runs[i]
		switch {
		case strings.TrimSpace(run.Text) == "":
			continue
		case run.Kind == "shape" && run.Shape != nil && run.Shape.Floating:
			continue
		default:
			return false
		}
	}
	return true
}

func paragraphBlockFloatTextBoxEligibleForLayout(p *resolve.Paragraph, target *resolve.TextBox) bool {
	if paragraphContainsOnlyFloatingTextBoxForLayout(p, target) {
		return true
	}
	if p == nil || target == nil {
		return false
	}
	targetIndex := -1
	for i, run := range p.Runs {
		if run.Kind == "textbox" && run.TextBox == target {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return false
	}
	for i := 0; i < targetIndex; i++ {
		run := p.Runs[i]
		switch {
		case strings.TrimSpace(run.Text) == "":
			continue
		case run.Kind == "image" && run.Image != nil && run.Image.Floating:
			continue
		case run.Kind == "textbox" && run.TextBox != nil && run.TextBox.Floating:
			continue
		case run.Kind == "shape" && run.Shape != nil && run.Shape.Floating:
			continue
		default:
			return false
		}
	}
	return true
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
	switch {
	case fl.Image != nil:
		img := fl.Image
		if img == nil || fl.BehindDoc || strings.EqualFold(img.WrapType, "none") {
			return lineExclusion{}, false
		}
		exc := lineExclusion{
			top:       fl.Y - img.DistTop,
			bottom:    fl.Y + fl.Height + img.DistBottom,
			left:      fl.X - img.DistLeft,
			right:     fl.X + fl.Width + img.DistRight,
			fullWidth: strings.EqualFold(img.WrapType, "topAndBottom"),
			wrapSide:  img.WrapSide,
			padLeft:   img.DistLeft,
			padRight:  img.DistRight,
		}
		if len(img.WrapPolygon) > 0 {
			exc.polygon = absolutePolygon(img.WrapPolygon, fl.X, fl.Y)
		} else if img.RotationDeg != 0 {
			rotated := rotatedRectPolygon(fl.X, fl.Y, fl.Width, fl.Height, img.RotationDeg)
			if left, top, right, bottom, ok := polygonBounds(rotated); ok {
				exc.left = left - img.DistLeft
				exc.right = right + img.DistRight
				exc.top = top - img.DistTop
				exc.bottom = bottom + img.DistBottom
			}
			if strings.EqualFold(img.WrapType, "tight") || strings.EqualFold(img.WrapType, "through") {
				exc.polygon = rotated
			}
		}
		return exc, true
	case fl.TextBox != nil:
		tb := fl.TextBox
		if tb == nil || fl.BehindDoc || strings.EqualFold(tb.WrapType, "none") {
			return lineExclusion{}, false
		}
		exc := lineExclusion{
			top:       fl.Y - tb.DistTop,
			bottom:    fl.Y + fl.Height + tb.DistBottom,
			left:      fl.X - tb.DistLeft,
			right:     fl.X + fl.Width + tb.DistRight,
			fullWidth: strings.EqualFold(tb.WrapType, "topAndBottom"),
			wrapSide:  tb.WrapSide,
			padLeft:   tb.DistLeft,
			padRight:  tb.DistRight,
		}
		if len(tb.WrapPolygon) > 0 {
			exc.polygon = absolutePolygon(tb.WrapPolygon, fl.X, fl.Y)
		}
		return exc, true
	case fl.Shape != nil:
		if fl.BehindDoc {
			return lineExclusion{}, false
		}
		sh := fl.Shape
		exc := lineExclusion{
			top:       fl.Y - sh.DistTop,
			bottom:    fl.Y + fl.Height + sh.DistBottom,
			left:      fl.X - sh.DistLeft,
			right:     fl.X + fl.Width + sh.DistRight,
			fullWidth: false,
			wrapSide:  sh.WrapSide,
			padLeft:   sh.DistLeft,
			padRight:  sh.DistRight,
		}
		if strings.EqualFold(sh.WrapType, "none") || fl.BehindDoc {
			return lineExclusion{}, false
		}
		if strings.EqualFold(sh.WrapType, "topAndBottom") {
			exc.fullWidth = true
		}
		if len(sh.WrapPolygon) > 0 {
			exc.polygon = absolutePolygon(sh.WrapPolygon, fl.X, fl.Y)
		}
		return exc, true
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
	return mergeLineExclusionIntervals(intervals)
}

// polygonIntervalsAtBand approximates the filled X-intervals occupied by a
// polygon across the horizontal band [lineTop, lineBottom] by sampling multiple
// points within the band and merging the resulting scanline intervals.
// For better accuracy with complex concave polygons, we sample at:
// - Top, 1/4, middle, 3/4, bottom of the band
// - All polygon vertices within the band
// - Additional points where polygon edges change direction significantly
func polygonIntervalsAtBand(polygon [][2]float64, lineTop, lineBottom float64) [][2]float64 {
	if len(polygon) < 3 {
		return nil
	}
	// Sample at 5 evenly spaced points for better accuracy
	bandHeight := lineBottom - lineTop
	samples := []float64{
		lineTop,
		lineTop + bandHeight*0.25,
		lineTop + bandHeight*0.5,
		lineTop + bandHeight*0.75,
		lineBottom,
	}

	// Add all polygon vertices within the band
	for _, pt := range polygon {
		y := pt[1]
		if y <= lineTop || y >= lineBottom {
			continue
		}
		duplicate := false
		for _, sample := range samples {
			if math.Abs(sample-y) < 1e-9 {
				duplicate = true
				break
			}
		}
		if !duplicate {
			samples = append(samples, y)
		}
	}

	// For complex polygons, also sample at edge intersection points
	// where the slope changes significantly
	n := len(polygon)
	for i := 0; i < n; i++ {
		p1 := polygon[i]
		p2 := polygon[(i+1)%n]
		p3 := polygon[(i+2)%n]

		// Check if this vertex represents a significant direction change
		dx1 := p2[0] - p1[0]
		dy1 := p2[1] - p1[1]
		dx2 := p3[0] - p2[0]
		dy2 := p3[1] - p2[1]

		// Normalize direction vectors
		len1 := math.Sqrt(dx1*dx1 + dy1*dy1)
		len2 := math.Sqrt(dx2*dx2 + dy2*dy2)

		if len1 > 1e-9 && len2 > 1e-9 {
			dot := (dx1*dx2 + dy1*dy2) / (len1 * len2)
			// If direction change is significant (angle > 45 degrees)
			if dot < 0.707 && p2[1] > lineTop && p2[1] < lineBottom {
				duplicate := false
				for _, sample := range samples {
					if math.Abs(sample-p2[1]) < 1e-9 {
						duplicate = true
						break
					}
				}
				if !duplicate {
					samples = append(samples, p2[1])
				}
			}
		}
	}

	intervals := make([][2]float64, 0, len(samples)*2)
	for _, y := range samples {
		intervals = append(intervals, polygonIntervalsAtY(polygon, y)...)
	}
	return mergeLineExclusionIntervals(intervals)
}

// polygonXRangeAtBand preserves the legacy min/max query over the merged scanline
// intervals used for tight/through wrapping.
func polygonXRangeAtBand(polygon [][2]float64, lineTop, lineBottom float64) (minX, maxX float64, ok bool) {
	intervals := polygonIntervalsAtBand(polygon, lineTop, lineBottom)
	if len(intervals) == 0 {
		return 0, 0, false
	}
	return intervals[0][0], intervals[len(intervals)-1][1], true
}

func lineAvailableWidthPt(lineLeft, lineRight, lineTop, lineBottom float64, exclusions []lineExclusion) float64 {
	width := lineRight - lineLeft
	if width < 1 {
		return 1
	}
	allowed := [][2]float64{{lineLeft, lineRight}}
	activeCount := 0
	for _, exclusion := range exclusions {
		if exclusion.bottom <= lineTop || exclusion.top >= lineBottom {
			continue
		}
		if exclusion.fullWidth {
			return 1
		}
		// For polygon exclusions (tight/through), clip against the per-line polygon intervals.
		polygonIntervals := [][2]float64{{exclusion.left, exclusion.right}}
		if len(exclusion.polygon) >= 2 {
			polygonIntervals = polygonIntervalsAtBand(exclusion.polygon, lineTop, lineBottom)
			if len(polygonIntervals) == 0 {
				continue // polygon doesn't intersect this line band
			}
			for i := range polygonIntervals {
				polygonIntervals[i][0] -= exclusion.padLeft
				polygonIntervals[i][1] += exclusion.padRight
			}
		}
		clipped := make([][2]float64, 0, len(polygonIntervals))
		for _, polygonInterval := range polygonIntervals {
			left := math.Max(lineLeft, polygonInterval[0])
			right := math.Min(lineRight, polygonInterval[1])
			if right > left {
				clipped = append(clipped, [2]float64{left, right})
			}
		}
		if len(clipped) == 0 {
			continue
		}

		activeCount++
		blocked := mergeLineExclusionIntervals(clipped)
		exclusionAllowed := allowedLineIntervalsForExclusion(lineLeft, lineRight, blocked, exclusion.wrapSide)
		if len(exclusionAllowed) == 0 {
			return 1
		}
		allowed = intersectLineIntervals(allowed, exclusionAllowed)
		if len(allowed) == 0 {
			return 1
		}
	}
	if activeCount == 0 {
		return width
	}
	available := maxLineIntervalWidth(allowed)
	if available < 1 {
		return 1
	}
	return available
}

func allowedLineIntervalsForExclusion(lineLeft, lineRight float64, blocked [][2]float64, wrapSide string) [][2]float64 {
	if len(blocked) == 0 {
		return [][2]float64{{lineLeft, lineRight}}
	}
	leftEdge := blocked[0][0]
	rightEdge := blocked[len(blocked)-1][1]
	switch strings.ToLower(strings.TrimSpace(wrapSide)) {
	case "left":
		if leftEdge <= lineLeft {
			return nil
		}
		return [][2]float64{{lineLeft, leftEdge}}
	case "right":
		if rightEdge >= lineRight {
			return nil
		}
		return [][2]float64{{rightEdge, lineRight}}
	case "largest":
		leftWidth := leftEdge - lineLeft
		rightWidth := lineRight - rightEdge
		if rightWidth > leftWidth {
			if rightWidth <= 0 {
				return nil
			}
			return [][2]float64{{rightEdge, lineRight}}
		}
		if leftWidth <= 0 {
			return nil
		}
		return [][2]float64{{lineLeft, leftEdge}}
	default:
		return complementLineIntervals(lineLeft, lineRight, blocked)
	}
}

func complementLineIntervals(lineLeft, lineRight float64, blocked [][2]float64) [][2]float64 {
	allowed := make([][2]float64, 0, len(blocked)+1)
	current := lineLeft
	for _, interval := range blocked {
		if interval[0] > current {
			allowed = append(allowed, [2]float64{current, interval[0]})
		}
		if interval[1] > current {
			current = interval[1]
		}
	}
	if current < lineRight {
		allowed = append(allowed, [2]float64{current, lineRight})
	}
	return allowed
}

func intersectLineIntervals(left, right [][2]float64) [][2]float64 {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	intersections := make([][2]float64, 0, min(len(left), len(right)))
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		start := math.Max(left[i][0], right[j][0])
		end := math.Min(left[i][1], right[j][1])
		if end > start {
			intersections = append(intersections, [2]float64{start, end})
		}
		if left[i][1] < right[j][1] {
			i++
		} else {
			j++
		}
	}
	return intersections
}

func maxLineIntervalWidth(intervals [][2]float64) float64 {
	maxWidth := 0.0
	for _, interval := range intervals {
		if width := interval[1] - interval[0]; width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}

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

func mergeLineExclusionIntervals(intervals [][2]float64) [][2]float64 {
	if len(intervals) == 0 {
		return nil
	}
	sorted := append(make([][2]float64, 0, len(intervals)), intervals...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i][0] < sorted[j][0]
	})
	merged := make([][2]float64, 0, len(sorted))
	current := sorted[0]
	for _, interval := range sorted[1:] {
		if interval[0] <= current[1] {
			if interval[1] > current[1] {
				current[1] = interval[1]
			}
			continue
		}
		merged = append(merged, current)
		current = interval
	}
	merged = append(merged, current)
	return merged
}

func largestContiguousAvailableWidthPt(lineLeft, lineRight float64, intervals [][2]float64) float64 {
	currentLeft := lineLeft
	best := 0.0
	for _, interval := range intervals {
		if interval[0] > currentLeft {
			best = math.Max(best, interval[0]-currentLeft)
		}
		if interval[1] > currentLeft {
			currentLeft = interval[1]
		}
	}
	if lineRight > currentLeft {
		best = math.Max(best, lineRight-currentLeft)
	}
	return best
}

// charWidthFactor returns the approximate advance width of a Unicode character
// as a fraction of the nominal font point size.
//
// These constants are calibrated for common proportional Latin fonts.
// CJK ideographs are full-width (≈1.0×); narrow Latin glyphs (i, l, 1) are
// narrower (≈0.28×); uppercase letters average ≈0.62× and lowercase ≈0.45×.
func charWidthFactor(ch rune, monospace bool) float64 {
	if monospace {
		return 0.6
	}
	switch {
	// CJK unified ideographs and fullwidth forms
	case ch >= 0x4E00 && ch <= 0x9FFF,
		ch >= 0x3000 && ch <= 0x303F,
		ch >= 0xF900 && ch <= 0xFAFF,
		ch >= 0xFF00 && ch <= 0xFFEF:
		return 1.0
	// Narrow glyphs: i, l, 1, !, |, punctuation
	case ch == 'i', ch == 'l', ch == '1', ch == '!', ch == '|',
		ch == '.', ch == ',', ch == ';', ch == ':', ch == '\'', ch == '`':
		return 0.28
	// Uppercase Latin and common extended uppercase
	case ch >= 'A' && ch <= 'Z',
		ch >= 0xC0 && ch <= 0xD6,
		ch >= 0xD8 && ch <= 0xDE:
		return 0.62
	// Digits
	case ch >= '0' && ch <= '9':
		return 0.55
	// Default: lowercase and other Latin-range characters
	default:
		return 0.45
	}
}

// isFontMonospace returns true when the font name is a known monospace typeface.
func isFontMonospace(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	for _, m := range []string{
		"courier", "consolas", "lucida console", "monaco", "menlo",
		"source code pro", "fira mono", "fira code", "inconsolata",
		"roboto mono", "dejavu sans mono", "liberation mono", "anonymous pro",
		"cascadia code", "cascadia mono", "jetbrains mono", "hack",
	} {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func preferredRunFontName(format resolve.RunFormat) string {
	if format.FontASCII != "" {
		return format.FontASCII
	}
	if format.FontHAnsi != "" {
		return format.FontHAnsi
	}
	if format.FontEastAsia != "" {
		return format.FontEastAsia
	}
	return format.FontCS
}

func fontWidthScale(name string) float64 {
	if name == "" {
		return 1.0
	}
	lower := strings.ToLower(name)
	for _, family := range []string{"calibri", "arial", "aptos", "segoe", "helvetica", "verdana", "tahoma"} {
		if strings.Contains(lower, family) {
			return 0.92
		}
	}
	return 1.0
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
	return 14.4
}

func estimateTableRowContentHeight(doc *resolve.Document, row resolve.TableRow, tf resolve.TableFormat, tableWidthPt float64, includeTableTop, includeTableBottom bool) float64 {
	cellWidths := TableCellWidths(row, tf, tableWidthPt)
	maxHeight := 0.0
	for i, cell := range row.Cells {
		if cell.Format.VMerge == "continue" {
			continue
		}
		cellWidthPt := tableWidthPt
		if i < len(cellWidths) && cellWidths[i] > 0 {
			cellWidthPt = cellWidths[i]
		}
		height := tableCellHeight(doc, cell, tf, cellWidthPt, includeTableTop, includeTableBottom)
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

func tableCellHeight(doc *resolve.Document, cell resolve.TableCell, tf resolve.TableFormat, cellWidthPt float64, includeTableTop, includeTableBottom bool) float64 {
	paddingTop := twipsToPoints(tf.CellPadTop)
	paddingBottom := twipsToPoints(tf.CellPadBottom)
	paddingLeft := twipsToPoints(tf.CellPadLeft)
	paddingRight := twipsToPoints(tf.CellPadRight)
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
	for i, block := range blocks {
		switch block.Kind {
		case "paragraph":
			p := block.Paragraph
			prev := adjacentBlock(blocks, i, -1)
			next := adjacentBlock(blocks, i, +1)
			contentHeight += ParagraphSpacingBefore(doc, p, prev, PageGeometry{}, i == 0)
			contentHeight += paragraphHeightInWidth(doc, p, PageGeometry{}, paragraphWidthWithinCell(p, contentWidthPt))
			contentHeight += ParagraphSpacingAfter(doc, p, next, PageGeometry{})
		case "table":
			contentHeight += tableHeight(doc, block.Table, contentWidthPt)
		}
	}
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
	total := 0.0
	for i, block := range blocks {
		switch block.Kind {
		case "paragraph":
			p := block.Paragraph
			prev := adjacentBlock(blocks, i, -1)
			next := adjacentBlock(blocks, i, +1)
			total += ParagraphSpacingBefore(doc, p, prev, PageGeometry{}, i == 0)
			total += paragraphHeightInWidth(doc, p, PageGeometry{}, paragraphWidthWithinCell(p, widthPt))
			total += ParagraphSpacingAfter(doc, p, next, PageGeometry{})
		case "table":
			total += tableHeight(doc, block.Table, widthPt)
		}
	}
	return total
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
