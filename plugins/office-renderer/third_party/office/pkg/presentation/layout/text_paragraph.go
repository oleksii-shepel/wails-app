package layout

import (
	"math"
	"strings"

	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
)

// ParagraphMetrics is the format-independent layout result for one resolved
// presentation paragraph. Values are concrete points/scales so renderers do
// not need to reinterpret DrawingML units.
type ParagraphMetrics struct {
	MarginLeftPt, MarginRightPt, TextIndentPt float64
	SpaceBeforePt, SpaceAfterPt               float64
	SpaceBeforeScale, SpaceAfterScale         float64
	LineHeightScale                           float64
	LineHeightPt                              float64
	// LineSpacingCompressed is true when the paragraph authors a percentage
	// line spacing below 100%, which tightens the line box. PowerPoint
	// compresses the leading, never the text itself, so the box never falls
	// below the run's own font size (see paragraphPhysicalLineHeightPt).
	LineSpacingCompressed                     bool
	AvailableWidthPt                          float64
	FirstLineAvailableWidthPt                 float64
	HasMarginLeft, HasMarginRight             bool
	HasTextIndent                             bool
	HasSpaceBefore, HasSpaceAfter             bool
}

// ResolveParagraphMetrics projects canonical paragraph state into physical
// text-layout metrics. frameWidthPt is the usable text-frame width after body
// insets. baseFontPt is used only for percentage paragraph spacing.
func ResolveParagraphMetrics(p *ir.Paragraph, frameWidthPt, baseFontPt, lineScale float64) ParagraphMetrics {
	m := ParagraphMetrics{LineHeightScale: lineScale, AvailableWidthPt: math.Max(0, frameWidthPt), FirstLineAvailableWidthPt: math.Max(0, frameWidthPt)}
	if lineScale <= 0 {
		m.LineHeightScale = 1
	}
	if p == nil {
		return m
	}

	m.HasMarginLeft = p.HasMarginLeft
	m.HasMarginRight = p.HasMarginRight
	m.HasTextIndent = p.HasTextIndent
	if p.HasMarginLeft {
		m.MarginLeftPt = drawing.EMUToPoints(int64(p.MarginLeftEMU))
	}
	if p.HasMarginRight {
		m.MarginRightPt = drawing.EMUToPoints(int64(p.MarginRightEMU))
	}
	if p.HasTextIndent {
		m.TextIndentPt = drawing.EMUToPoints(int64(p.TextIndentEMU))
	}

	m.AvailableWidthPt = math.Max(0, frameWidthPt-m.MarginLeftPt-m.MarginRightPt)
	m.FirstLineAvailableWidthPt = m.AvailableWidthPt
	if p.HasTextIndent {
		m.FirstLineAvailableWidthPt = math.Max(0, m.AvailableWidthPt-m.TextIndentPt)
	}

	if p.HasSpaceBefore {
		m.HasSpaceBefore = true
		m.SpaceBeforePt = float64(p.SpaceBefore) / 100.0
	}
	if p.SpaceBeforePct > 0 && baseFontPt > 0 && m.SpaceBeforePt <= 0 {
		// Legacy PPT states the authored spacing as a percentage of the line
		// height. A paragraph can carry HasSpaceBefore (the author touched the
		// field) with an absolute value of zero while the percentage is the
		// real authored distance; ignoring it flattened the 0.2-line paragraph
		// gap PowerPoint keeps between every body paragraph of 41246-2.
		m.HasSpaceBefore = true
		m.SpaceBeforeScale = float64(p.SpaceBeforePct) / 100000.0
		m.SpaceBeforePt = paragraphReferenceLineHeightPt(p, baseFontPt, m.LineHeightScale) * m.SpaceBeforeScale
	}
	if p.HasSpaceAfter {
		m.HasSpaceAfter = true
		m.SpaceAfterPt = float64(p.SpaceAfter) / 100.0
	}
	if !paragraphHasVisibleRuns(p) && baseFontPt > 0 {
		// A blank paragraph's line box is its paragraph mark's size, not the
		// face's 1.2-em line box scaled by the authored percentage: 41246-2
		// slide 11's five spacer paragraphs (8/9/16/20/24pt marks) each ran
		// 1.7pt tall and pushed the last body line ten points down its frame.
		m.LineHeightPt = baseFontPt
	}
	if p.SpaceAfterPct > 0 && baseFontPt > 0 && m.SpaceAfterPt <= 0 {
		m.HasSpaceAfter = true
		m.SpaceAfterScale = float64(p.SpaceAfterPct) / 100000.0
		m.SpaceAfterPt = paragraphReferenceLineHeightPt(p, baseFontPt, m.LineHeightScale) * m.SpaceAfterScale
	}
	if p.HasLineSpacing && p.LineSpacingPt > 0 {
		// Absolute DrawingML spacing is already a physical line-box size. Autofit
		// line-space reduction still scales that authored distance.
		m.LineHeightPt = float64(p.LineSpacingPt) / 100.0 * m.LineHeightScale
	} else if p.HasLineSpacing && p.LineSpacingPct > 0 {
		pct := float64(p.LineSpacingPct) / 100000.0
		m.LineHeightScale = pct * m.LineHeightScale
		// 41246-2 slide 14 authors its "(OP et opérateurs de formation)"
		// paragraph at 40% line spacing; the reference draws that 20pt line at a
		// 20.04pt pitch rather than the 9.6pt the bare percentage implies, so a
		// compressed line still keeps the text's own height.
		m.LineSpacingCompressed = pct < 1
	}
	return m
}

func paragraphReferenceLineHeightPt(p *ir.Paragraph, baseFontPt, lineScale float64) float64 {
	if baseFontPt <= 0 {
		return 0
	}
	if lineScale <= 0 {
		lineScale = 1
	}
	metrics := ResolveFontAdvanceMetrics(paragraphBaseFontFamily(p))
	return baseFontPt * metrics.LineBoxScale * lineScale
}

// SuppressLeadingSpaceBefore clears paragraph space-before. PowerPoint ignores
// space-before on the first paragraph of a text body, so the authored value must
// not push the first baseline down.
func SuppressLeadingSpaceBefore(m *ParagraphMetrics) {
	if m == nil {
		return
	}
	m.HasSpaceBefore = false
	m.SpaceBeforePt = 0
	m.SpaceBeforeScale = 0
}

// TextFrameContentWidthPt returns the horizontal width available to paragraph
// layout after canonical text-body insets have been applied.
func TextFrameContentWidthPt(shape *ir.Shape, frame TextFrameMetrics) float64 {
	if shape == nil {
		return 0
	}
	width := drawing.EMUToPoints(shape.Bounds.W) - frame.InsetLeftPt - frame.InsetRightPt
	return math.Max(0, width)
}

// TextFrameContentHeightPt returns the vertical extent available to text after
// canonical text-body insets. Vertical writing uses this as its glyph-flow axis.
func TextFrameContentHeightPt(shape *ir.Shape, frame TextFrameMetrics) float64 {
	if shape == nil {
		return 0
	}
	height := drawing.EMUToPoints(shape.Bounds.H) - frame.InsetTopPt - frame.InsetBottomPt
	return math.Max(0, height)
}

// paragraphHasVisibleRuns reports whether a paragraph has any run text beyond
// whitespace. Empty spacer paragraphs carry only a paragraph mark.
func paragraphHasVisibleRuns(p *ir.Paragraph) bool {
	if p == nil {
		return false
	}
	for _, r := range p.Runs {
		if r == nil {
			continue
		}
		if strings.TrimSpace(r.Text) != "" || r.MathML != "" {
			return true
		}
	}
	return false
}
