package layout

import resolve "office/pkg/word/ir"

func paragraphIntrinsicLineHeights(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry, widthPt float64) (TextLayoutMetrics, []float64, float64, float64) {
	metrics := resolveTextLayoutMetrics(doc, p, geom, 0, 0)
	topBorderOutset, bottomBorderOutset := paragraphVerticalBorderOutsetPt(p)
	// An authored empty paragraph still owns one Word line, but that line's
	// height is the canonical EmptyHeightPt resolved above. Do not ask the
	// wrapping estimator to manufacture a default-font line: doing so lets
	// stories/table cells/textboxes diverge from body paragraph metrics.
	if storyParagraphIsVisuallyEmpty(p) {
		return metrics, nil, topBorderOutset, bottomBorderOutset
	}
	defaultSizePt := defaultFontSizePt(doc)
	firstWidthPt, continuationWidthPt := paragraphLineWidthsPt(p, widthPt)
	lineMetrics := estimateParagraphLineMetricsWithWidths(p, firstWidthPt, continuationWidthPt, defaultSizePt, nil)
	lineHeights := paragraphLineHeightsPt(p, geom, lineMetrics, defaultSizePt)
	return metrics, lineHeights, topBorderOutset, bottomBorderOutset
}

func storyParagraphHeightInWidth(doc *resolve.Document, p, prev, next *resolve.Paragraph, geom PageGeometry, widthPt float64) float64 {
	metrics := resolveStoryParagraphMetrics(doc, p, prev, next, geom)
	_, lineHeights, topBorderOutset, bottomBorderOutset := paragraphIntrinsicLineHeights(doc, p, geom, widthPt)
	return paragraphIntrinsicHeightFromMetrics(metrics, lineHeights, topBorderOutset, bottomBorderOutset)
}

func storyBlocksHeightInWidth(doc *resolve.Document, blocks []resolve.Block, geom PageGeometry, widthPt float64) float64 {
	total := 0.0
	for i, block := range blocks {
		switch block.Kind {
		case "paragraph":
			p := block.Paragraph
			total += storyParagraphHeightInWidth(doc, p, adjacentBlock(blocks, i, -1), adjacentBlock(blocks, i, +1), geom, paragraphWidthWithinCell(p, widthPt))
		case "table":
			total += tableHeight(doc, block.Table, widthPt)
		}
	}
	return total
}
