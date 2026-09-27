package layout

import (
	"math"

	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
)

// TextFrameLayout is the format-independent vertical measurement of a text
// frame after paragraph wrapping has been resolved.
type TextFrameLayout struct {
	ContentHeightPt   float64
	AvailableHeightPt float64
	RequiredHeightPt  float64
	VerticalOffsetPt  float64
	OverflowPt        float64
	Measured          bool
}

// LayoutTextFrame measures the vertical extent of canonical paragraphs using
// the same physical-line boundary used by horizontal wrapping. It deliberately
// declines ownership when any paragraph cannot be measured safely; renderers
// can then retain their semantic fallback instead of inventing geometry.
func LayoutTextFrame(shape *ir.Shape, frame TextFrameMetrics) TextFrameLayout {
	if shape == nil {
		return TextFrameLayout{}
	}
	frameWidthPt := TextFrameContentWidthPt(shape, frame)
	frameHeightPt := drawing.EMUToPoints(shape.Bounds.H)
	availableHeight := math.Max(0, frameHeightPt-frame.InsetTopPt-frame.InsetBottomPt)
	out := TextFrameLayout{AvailableHeightPt: availableHeight}
	measuredAny := false

	for _, p := range shape.Paragraphs {
		if p == nil {
			continue
		}
		baseFontPt := paragraphBaseFontPt(p, frame.FontScale)
		pm := ResolveParagraphMetrics(p, frameWidthPt, baseFontPt, frame.LineScale)
		if !measuredAny {
			SuppressLeadingSpaceBefore(&pm)
		}
		measuredAny = true
		if !CanLayoutParagraphLines(p, pm, frame.FontScale) {
			return TextFrameLayout{AvailableHeightPt: availableHeight}
		}
		marker := ""
		if p.BulletChar != "" {
			marker = p.BulletChar
		} else if p.AutoNum {
			// Marker width only needs a stable representative here; numbering
			// text remains renderer-owned until list formatting moves to layout.
			marker = "1."
		}
		lines := LayoutParagraphLines(p, pm, frame.FontScale, marker, baseFontPt)
		out.ContentHeightPt += pm.SpaceBeforePt + lines.ContentHeightPt + pm.SpaceAfterPt
	}

	out.Measured = true
	out.RequiredHeightPt = frame.InsetTopPt + out.ContentHeightPt + frame.InsetBottomPt
	if out.ContentHeightPt > availableHeight {
		out.OverflowPt = out.ContentHeightPt - availableHeight
	}
	if out.ContentHeightPt < availableHeight {
		spare := availableHeight - out.ContentHeightPt
		switch frame.VerticalJustify {
		case "center":
			out.VerticalOffsetPt = spare / 2
		case "flex-end":
			out.VerticalOffsetPt = spare
		}
	}
	return out
}

// LayoutTextFrameFromParagraphPlans measures a frame from already-materialized
// physical paragraph lines. This is the canonical composition path used by
// TextVisualPlan and prevents frame measurement from wrapping text a second time.
func LayoutTextFrameFromParagraphPlans(shape *ir.Shape, frame TextFrameMetrics, paragraphs []ParagraphVisualPlan) TextFrameLayout {
	if shape == nil {
		return TextFrameLayout{}
	}
	frameHeightPt := drawing.EMUToPoints(shape.Bounds.H)
	availableHeight := math.Max(0, frameHeightPt-frame.InsetTopPt-frame.InsetBottomPt)
	out := TextFrameLayout{AvailableHeightPt: availableHeight}
	for _, pp := range paragraphs {
		if !pp.LinesMeasured {
			return TextFrameLayout{AvailableHeightPt: availableHeight}
		}
		out.ContentHeightPt += pp.Metrics.SpaceBeforePt + physicalParagraphLinesHeight(pp.Lines) + pp.Metrics.SpaceAfterPt
	}
	out.Measured = true
	out.RequiredHeightPt = frame.InsetTopPt + out.ContentHeightPt + frame.InsetBottomPt
	if out.ContentHeightPt > availableHeight {
		out.OverflowPt = out.ContentHeightPt - availableHeight
	} else {
		spare := availableHeight - out.ContentHeightPt
		switch frame.VerticalJustify {
		case "center":
			out.VerticalOffsetPt = spare / 2
		case "flex-end":
			out.VerticalOffsetPt = spare
		}
	}
	return out
}

func physicalParagraphLinesHeight(lines []PhysicalTextLine) float64 {
	h := 0.0
	for _, line := range lines {
		h += line.HeightPt
	}
	return h
}

// LayoutTextFrameFromParagraphPlansWithFlow measures the line-progression axis
// from the already materialized paragraph plans. Horizontal text progresses
// downward; vertical text progresses across the frame. This keeps anchoring on
// the same physical axis as the composed lines.
func LayoutTextFrameFromParagraphPlansWithFlow(shape *ir.Shape, frame TextFrameMetrics, flow TextFlowMetrics, paragraphs []ParagraphVisualPlan) TextFrameLayout {
	if shape == nil {
		return TextFrameLayout{}
	}
	available := flow.ProgressionExtentPt
	if available <= 0 {
		available = TextFrameContentHeightPt(shape, frame)
	}
	out := TextFrameLayout{AvailableHeightPt: available}
	for _, pp := range paragraphs {
		if !pp.LinesMeasured {
			return TextFrameLayout{AvailableHeightPt: available}
		}
		out.ContentHeightPt += pp.Metrics.SpaceBeforePt + physicalParagraphLinesHeight(pp.Lines) + pp.Metrics.SpaceAfterPt
	}
	out.Measured = true
	out.RequiredHeightPt = out.ContentHeightPt
	if !flow.Vertical {
		out.RequiredHeightPt += frame.InsetTopPt + frame.InsetBottomPt
	}
	delta := available - out.ContentHeightPt
	if delta < 0 {
		out.OverflowPt = -delta
	}
	// Anchoring is independent of clipping. When content is taller than the
	// frame, centered text overflows equally on both progression edges and
	// bottom-anchored text overflows toward the start edge. Keeping the signed
	// offset here prevents serializers from silently degrading overflow to top.
	switch frame.VerticalJustify {
	case "center":
		out.VerticalOffsetPt = delta / 2
	case "flex-end":
		out.VerticalOffsetPt = delta
	}
	return out
}
