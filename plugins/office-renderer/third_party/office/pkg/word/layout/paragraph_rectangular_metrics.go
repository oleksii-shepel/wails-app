package layout

import resolve "office/pkg/word/ir"

// estimateParagraphLineMetricsWithWidths projects ordinary rectangular
// paragraph flow onto the canonical span-native breaker. It remains as the
// bootstrap boundary for callers that do not have drawing exclusions yet;
// token placement itself is owned exclusively by breakParagraphAcrossSlots.
func estimateParagraphLineMetricsWithWidths(
	p *resolve.Paragraph,
	firstWidthPt, continuationWidthPt, defaultSizePt float64,
	_ func(int) float64,
) []paragraphLineMetric {
	// The slot breaker can extend provisional geometry itself. Supplying a
	// thousand empty bands here made empty/short paragraphs appear to contain a
	// thousand physical lines and also poisoned the first wrap-convergence pass.
	// Two bands are sufficient to encode the only distinction rectangular flow
	// needs: first-line width versus continuation width. Further lines inherit
	// the final band's slots inside breakParagraphAcrossSlots.
	if firstWidthPt < 1 {
		firstWidthPt = 1
	}
	if continuationWidthPt < 1 {
		continuationWidthPt = 1
	}

	bands := []lineBand{
		{Spans: []lineSpan{{Left: 0, Right: firstWidthPt}}},
		{Spans: []lineSpan{{Left: 0, Right: continuationWidthPt}}},
	}

	flow := breakParagraphAcrossSlots(
		p,
		paragraphLinePlan{Bands: bands}.SlotPlan(),
		defaultSizePt,
	)

	// An empty paragraph still owns exactly one physical line. The generic slot
	// breaker intentionally preserves provisional empty bands, so normalize the
	// rectangular bootstrap boundary here rather than teaching drawing-aware
	// flow a special case.
	if p == nil || len(paragraphRenderTokens(p, defaultSizePt)) == 0 {
		return []paragraphLineMetric{{maxFontSizePt: defaultSizePt}}
	}
	if len(flow.LineMetrics()) == 0 {
		return []paragraphLineMetric{{maxFontSizePt: defaultSizePt}}
	}
	return flow.LineMetrics()
}
