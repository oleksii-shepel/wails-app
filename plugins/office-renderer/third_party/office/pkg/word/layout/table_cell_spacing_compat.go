package layout

import resolve "office/pkg/word/ir"

// tableCellParagraphSpacingBefore is retained as a focused compatibility
// projection for table-cell pagination tests. Production table/story
// measurement resolves the same value through resolveStoryParagraphMetrics.
func tableCellParagraphSpacingBefore(doc *resolve.Document, p, prev *resolve.Paragraph) float64 {
	return resolveStoryParagraphMetrics(doc, p, prev, nil, PageGeometry{}).SpacingBefore
}
