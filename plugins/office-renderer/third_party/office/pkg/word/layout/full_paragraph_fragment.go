package layout

import resolve "office/pkg/word/ir"

// fullParagraphPhysicalFragment captures the physical lines used to measure an
// unsplit paragraph. Once those lines exist, layout owns their geometry; the
// renderer can consume the canonical fragment runs without taking over spatial
// positioning unless float geometry actually constrained a line.
func fullParagraphPhysicalFragment(doc *resolve.Document, p *resolve.Paragraph, geom PageGeometry, column int, y, spacingBefore, spacingAfter float64, activeFloats []FloatBox) *ParagraphFragment {
	// A drawing-only paragraph owns layout/pagination and anchor progression,
	// but it has no renderable text line. Keeping it out of the fragment path
	// prevents synthetic empty browser lines without discarding its flow height.
	if paragraphIsDrawingHost(p) {
		return nil
	}
	textMetrics := resolveTextLayoutMetrics(doc, p, geom, spacingBefore, spacingAfter)
	metrics, ok := paragraphFlowMetricsForPagination(doc, p, geom, column, y+geom.MarginTop, textMetrics, activeFloats)
	if !ok || len(metrics.physicalLines) == 0 {
		return nil
	}
	physicalLines := metrics.fragmentPhysicalLines(0, len(metrics.physicalLines))
	if len(physicalLines) == 0 {
		return nil
	}
	originY := fragmentFlowOriginY(0, metrics, physicalLines)
	return &ParagraphFragment{
		StartLine:        0,
		EndLine:          len(physicalLines),
		Lines:            paragraphLineLayouts(physicalLines, originY),
		SpatialRendering: paragraphPhysicalLinesNeedSpatialRendering(p, geom, column, physicalLines),
		IsLast:           true,
		OriginY:          originY,
	}
}

// fragmentFlowOriginY is the flow Y a fragment's renderer-facing coordinates
// start from. On a first fragment that is the paragraph's own content top, so
// a leading block drawing keeps the strip the layout reserved above the first
// text line; a continuation fragment has no such strip and anchors to its own
// first line.
func fragmentFlowOriginY(startLine int, metrics paragraphFlowMetrics, lines []paragraphPhysicalLine) float64 {
	if startLine == 0 && metrics.leadingBlockFloatHeight > 0 && metrics.flowOriginY > 0 {
		return metrics.flowOriginY
	}
	return physicalLinesOriginY(lines)
}

// physicalLinesOriginY is the flow Y of the first physical line, in the
// page-absolute coordinate space shared with BlockBox.Y + Geometry.MarginTop.
func physicalLinesOriginY(lines []paragraphPhysicalLine) float64 {
	origin := 0.0
	have := false
	for _, line := range lines {
		for _, slot := range line.Slots {
			if !have || slot.Slot.Top < origin {
				origin = slot.Slot.Top
				have = true
			}
		}
	}
	return origin
}

// paragraphPhysicalLinesNeedSpatialRendering reports whether float geometry
// constrained a physical line. It is retained for diagnostics and fidelity
// assertions. Physical-line ownership itself is unconditional once layout has
// successfully measured an ordinary paragraph: the renderer must not perform a
// second, browser-specific line break.
func paragraphPhysicalLinesNeedSpatialRendering(p *resolve.Paragraph, _ PageGeometry, _ int, lines []paragraphPhysicalLine) bool {
	// Explicit line-breaking controls are part of the physical topology even when
	// every resulting line spans the full column. Letting the browser reinterpret
	// them would create a second line-breaking model after pagination.
	if p != nil {
		for _, run := range p.Runs {
			if run.Kind != "break" {
				continue
			}
			switch run.BreakType {
			case "line", "textWrapping", "carriageReturn":
				return true
			}
		}
	}
	for _, line := range lines {
		for _, slot := range line.Slots {
			if slot.Slot.Constrained || len(line.Slots) > 1 {
				return true
			}
		}
	}
	return false
}
