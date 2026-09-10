package layout

import resolve "office/pkg/word/ir"

// TextLayoutMetrics is the canonical vertical text geometry consumed by Word
// layout. It deliberately delegates source-format semantics to the established
// layout helpers: this type is an ownership boundary, not a second formatter.
// Body paragraphs, table cells, text boxes, and paragraph-relative drawings
// should all consume geometry derived from this object.
type TextLayoutMetrics struct {
	FontSizePt    float64
	LineHeightPt  float64
	EmptyHeightPt float64
	SpacingBefore float64
	SpacingAfter  float64
}

func resolveTextLayoutMetrics(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry, spacingBefore, spacingAfter float64) TextLayoutMetrics {
	fontSizePt := paragraphFontSizePt(doc, p)
	lineHeightPt := paragraphLineHeightPt(p, geom, fontSizePt)
	emptyHeightPt := lineHeightPt

	// Empty paragraphs and drawing-only anchor paragraphs are represented in
	// Word by the paragraph mark. Its effective run formatting is independent
	// of the visible/drawing runs in the paragraph, so derive its height from
	// Paragraph.DefaultRunFormat (pPr/rPr + paragraph style) and finally the
	// document default run format.
	switch resolveParagraphFlowRole(p) {
	case paragraphFlowRoleEmpty, paragraphFlowRoleDrawingHost:
		emptyHeightPt = paragraphMarkLineHeightPt(doc, p, geom)
	}

	return TextLayoutMetrics{
		FontSizePt:    fontSizePt,
		LineHeightPt:  lineHeightPt,
		EmptyHeightPt: emptyHeightPt,
		SpacingBefore: spacingBefore,
		SpacingAfter:  spacingAfter,
	}
}

// paragraphMarkLineHeightPt resolves the physical advance owned by an empty
// paragraph mark. Paragraph.DefaultRunFormat is already the parser/resolver's
// effective paragraph-mark formatting boundary; layout must consume it rather
// than rediscovering style inheritance or borrowing formatting from a drawing.
func paragraphMarkLineHeightPt(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry) float64 {
	fontSizePt := paragraphMarkFontSizePt(doc, p)
	return paragraphLineHeightPt(p, geom, fontSizePt)
}

func paragraphMarkFontSizePt(doc *resolve.Document, p *resolve.Paragraph) float64 {
	if p != nil && p.DefaultRunFormat != nil &&
		p.DefaultRunFormat.FontSize != nil && *p.DefaultRunFormat.FontSize > 0 {
		return float64(*p.DefaultRunFormat.FontSize) / 2.0
	}
	if doc != nil && doc.DefaultRunFormat.FontSize != nil && *doc.DefaultRunFormat.FontSize > 0 {
		return float64(*doc.DefaultRunFormat.FontSize) / 2.0
	}
	return defaultFontSizePt(doc)
}

func resolveParagraphSpacingMetrics(doc *resolve.Document, p, prev, next *resolve.Paragraph, geom PageGeometry, atFlowStart bool) TextLayoutMetrics {
	before := ParagraphSpacingBefore(doc, p, prev, geom, atFlowStart)
	after := ParagraphSpacingAfter(doc, p, next, geom)
	return resolveTextLayoutMetrics(doc, p, geom, before, after)
}

func resolveStoryParagraphMetrics(doc *resolve.Document, p, prev, next *resolve.Paragraph, geom PageGeometry) TextLayoutMetrics {
	before := 0.0
	if p != nil && (prev == nil || !ContextualSpacingSuppresses(p, prev)) {
		before = resolvedParagraphSpacingBeforePt(doc, p, geom)
	}
	after := ParagraphSpacingAfter(doc, p, next, geom)
	return resolveTextLayoutMetrics(doc, p, geom, before, after)
}

func paragraphContentHeightFromMetrics(metrics TextLayoutMetrics, lineHeights []float64) float64 {
	height := sumFloat64s(lineHeights)
	if height == 0 {
		return metrics.EmptyHeightPt
	}
	return height
}

func paragraphHeightFromMetrics(metrics TextLayoutMetrics, lineHeights []float64, topBorderOutset, topClearance, bottomBorderOutset float64) float64 {
	return metrics.SpacingBefore + topBorderOutset + topClearance +
		paragraphContentHeightFromMetrics(metrics, lineHeights) +
		bottomBorderOutset + metrics.SpacingAfter
}

func paragraphIntrinsicHeightFromMetrics(metrics TextLayoutMetrics, lineHeights []float64, topBorderOutset, bottomBorderOutset float64) float64 {
	return paragraphHeightFromMetrics(metrics, lineHeights, topBorderOutset, 0, bottomBorderOutset)
}
