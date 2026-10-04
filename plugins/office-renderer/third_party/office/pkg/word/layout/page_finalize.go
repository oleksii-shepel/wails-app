package layout

import resolve "office/pkg/word/ir"

// pageFootnoteFinalizer carries only the numbering state needed to finalize
// footnotes page-by-page. This removes the previous requirement to keep every
// page alive until document pagination completes.
type pageFootnoteFinalizer struct {
	number               int
	previousSectionIndex int
	initialized          bool
	nextSyntheticIndex   int
}

func newPageFootnoteFinalizer(doc *resolve.Document) *pageFootnoteFinalizer {
	return &pageFootnoteFinalizer{
		number:               footnoteStartNumberForSection(doc, resolve.Section{}),
		previousSectionIndex: -1,
		nextSyntheticIndex:   -1,
	}
}

// Finalize resolves page-local footnote boxes in-place and advances numbering
// state for the next page. Pages without footnotes still update section state
// so eachSect restart semantics stay correct when the next note appears.
func (f *pageFootnoteFinalizer) Finalize(doc *resolve.Document, page *Page, footnoteIDs []int) {
	if f == nil || page == nil || doc == nil {
		return
	}

	format, restart := footnoteFormatAndRestartForSection(doc, page.Section)
	sectionChanged := f.initialized && page.SectionIndex != f.previousSectionIndex
	if !f.initialized || restart == "eachPage" || (restart == "eachSect" && sectionChanged) {
		f.number = footnoteStartNumberForSection(doc, page.Section)
	}
	f.previousSectionIndex = page.SectionIndex
	f.initialized = true

	if len(footnoteIDs) == 0 {
		return
	}

	blocks, nextNumber := materializeFootnoteBlocks(doc, footnoteIDs, f.number, format, footnoteLayoutGeometry(page.Geometry).ColumnWidthPt, &f.nextSyntheticIndex)
	if len(blocks) == 0 {
		return
	}
	f.number = nextNumber

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
