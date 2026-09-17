package layout

import (
	"math"

	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
)

// TextFlowMetrics is the format-independent composition result for text flow
// inside a presentation text frame. It owns writing direction, body rotation,
// wrapping and multi-column geometry so renderers only materialize the result.
type TextFlowMetrics struct {
	RotationDeg     float64
	WritingMode     string
	AnchorCenter    bool
	TransformOrigin string
	NoWrap          bool
	Columns         int
	ColumnGapPt     float64
	ColumnWidthPt   float64
	// LineExtentPt is the physical extent available along the glyph-flow axis.
	// ProgressionExtentPt is the perpendicular extent available for line stacking.
	LineExtentPt, ProgressionExtentPt float64
	Vertical                          bool
}

// ResolveTextFlowMetrics converts canonical text-body state and resolved frame
// geometry into physical text-flow metrics.
func ResolveTextFlowMetrics(shape *ir.Shape, frame TextFrameMetrics) TextFlowMetrics {
	m := TextFlowMetrics{Columns: 1}
	if shape == nil {
		return m
	}
	p := shape.TextBodyProps
	m.RotationDeg = float64(p.Rot) / 60000.0
	m.AnchorCenter = p.AnchorCtr
	if m.AnchorCenter {
		// DrawingML anchorCtr centers the text anchor used by body rotation; it
		// does not change paragraph alignment. Keep that distinction physical so
		// serializers never translate anchorCtr into text-align:center.
		m.TransformOrigin = "center center"
	}
	m.NoWrap = p.Wrap == "none"
	m.WritingMode = writingModeForVert(p.Vert)
	m.Vertical = m.WritingMode != ""
	if p.Vert == "vert270" {
		m.RotationDeg += 180
	}
	if p.Columns > 1 {
		m.Columns = p.Columns
		m.ColumnGapPt = drawing.EMUToPoints(p.ColumnSpacingEMU)
	}

	contentWidth := TextFrameContentWidthPt(shape, frame)
	contentHeight := TextFrameContentHeightPt(shape, frame)
	// A quarter-turn body rotation swaps the physical glyph-flow and line-
	// progression extents even when the source uses horizontal writing. Without
	// this, 90/270-degree text wraps against the unrotated width and then gets
	// rotated into a different box by the serializer.
	rot := math.Mod(math.Abs(m.RotationDeg), 180)
	quarterTurn := math.Abs(rot-90) < 0.001
	if m.Vertical {
		// In vertical writing the glyph-flow axis is the frame height and lines
		// progress across the frame width. Columns therefore partition the
		// progression axis rather than the glyph-flow axis.
		m.LineExtentPt = contentHeight
		gapTotal := float64(m.Columns-1) * m.ColumnGapPt
		if gapTotal < contentWidth {
			m.ProgressionExtentPt = (contentWidth - gapTotal) / float64(m.Columns)
		}
		m.ColumnWidthPt = m.ProgressionExtentPt
	} else {
		lineExtent, progressionExtent := contentWidth, contentHeight
		if quarterTurn {
			lineExtent, progressionExtent = contentHeight, contentWidth
		}
		gapTotal := float64(m.Columns-1) * m.ColumnGapPt
		if gapTotal < lineExtent {
			m.ColumnWidthPt = (lineExtent - gapTotal) / float64(m.Columns)
		}
		m.LineExtentPt = m.ColumnWidthPt
		m.ProgressionExtentPt = progressionExtent
	}
	return m
}

func writingModeForVert(vert string) string {
	switch vert {
	case "vert", "wordArtVert", "wordArtVertRtl":
		return "vertical-rl"
	case "vert270":
		return "vertical-lr"
	default:
		return ""
	}
}
