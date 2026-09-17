package layout

import (
	"math"
	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
)

// TextFrameMetrics is the format-independent layout result for a presentation
// text frame. Rendering consumes this instead of reinterpreting DrawingML
// autofit/inset semantics itself.
type TextFrameMetrics struct {
	InsetTopPt, InsetRightPt, InsetBottomPt, InsetLeftPt float64
	FontScale, LineScale                                 float64
	VerticalJustify                                      string
	AutoGrow, AllowOverflow                              bool
}

// ResolveTextFrameMetrics converts canonical text-body state into concrete
// layout metrics. The values are deliberately CSS-independent so other
// presentation renderers can share the same decisions.
func ResolveTextFrameMetrics(shape *ir.Shape) TextFrameMetrics {
	m := TextFrameMetrics{
		InsetTopPt: 3.6, InsetRightPt: 7.2, InsetBottomPt: 3.6, InsetLeftPt: 7.2,
		FontScale: 1, LineScale: 1, VerticalJustify: "flex-start",
	}
	if shape == nil {
		return m
	}
	p := shape.TextBodyProps
	// Canonical Shape insets carry provenance. This is especially important for
	// binary PPT where zero is a valid explicit OfficeArt text inset. Treating
	// zero as "unset" silently substituted DrawingML defaults and narrowed text
	// boxes by as much as 14.4pt.
	if shape.HasTextInsets {
		m.InsetTopPt = drawing.EMUToPoints(shape.TextInsetTopEMU)
		m.InsetRightPt = drawing.EMUToPoints(shape.TextInsetRightEMU)
		m.InsetBottomPt = drawing.EMUToPoints(shape.TextInsetBottomEMU)
		m.InsetLeftPt = drawing.EMUToPoints(shape.TextInsetLeftEMU)
	} else {
		// Compatibility for canonical shapes produced by older callers. Non-zero
		// bodyPr values are still meaningful even when provenance was unavailable.
		if p.InsetT != 0 {
			m.InsetTopPt = drawing.EMUToPoints(p.InsetT)
		} else if shape.TextInsetTopEMU != 0 {
			m.InsetTopPt = drawing.EMUToPoints(shape.TextInsetTopEMU)
		}
		if p.InsetR != 0 {
			m.InsetRightPt = drawing.EMUToPoints(p.InsetR)
		} else if shape.TextInsetRightEMU != 0 {
			m.InsetRightPt = drawing.EMUToPoints(shape.TextInsetRightEMU)
		}
		if p.InsetB != 0 {
			m.InsetBottomPt = drawing.EMUToPoints(p.InsetB)
		} else if shape.TextInsetBottomEMU != 0 {
			m.InsetBottomPt = drawing.EMUToPoints(shape.TextInsetBottomEMU)
		}
		if p.InsetL != 0 {
			m.InsetLeftPt = drawing.EMUToPoints(p.InsetL)
		} else if shape.TextInsetLeftEMU != 0 {
			m.InsetLeftPt = drawing.EMUToPoints(shape.TextInsetLeftEMU)
		}
	}

	anchor := p.Anchor
	if anchor == "" {
		anchor = shape.TextAnchor
	}
	switch anchor {
	case "ctr":
		m.VerticalJustify = "center"
	case "b":
		m.VerticalJustify = "flex-end"
	case "just":
		// PowerPoint's justified vertical anchor distributes the text block
		// across the full progression extent. Keep the resolved distribution in
		// the physical frame contract instead of degrading it to top anchoring.
		m.VerticalJustify = "space-between"
	case "dist":
		m.VerticalJustify = "space-around"
	}

	m.AllowOverflow = shape.TextAllowOverflow || p.AutofitType == "noAutofit"
	m.AutoGrow = shape.TextAutoGrow || p.AutofitType == "spAutoFit"
	if p.AutofitType == "normAutofit" {
		if p.AutofitFontScale > 0 {
			// DrawingML normAutofit is a shrink-to-fit contract. A malformed or
			// extension-produced fontScale above 100% must never enlarge text: that
			// would turn autofit into a second, format-specific font-sizing model.
			s := float64(p.AutofitFontScale) / 100000.0
			if s > 1 {
				s = 1
			}
			if s > 0 {
				m.FontScale = s
			}
		}
		if p.AutofitLineSpaceReduction > 0 {
			// Reduction cannot invert/collapse the physical line box. Ignore
			// malformed values at or above 100% and retain the authored line scale.
			s := 1 - float64(p.AutofitLineSpaceReduction)/100000.0
			if s > 0 && s <= 1 {
				m.LineScale = s
			}
		}
	}
	return m
}

// ResolveEffectiveTextFrameMetrics applies shape-geometry content insets before
// text flow is measured. Geometry insets are part of the physical text frame,
// not a serializer decoration: they affect wrapping, vertical anchoring and
// autofit using the same content rectangle.
func ResolveEffectiveTextFrameMetrics(shape *ir.Shape, visual ShapeVisualPlan) TextFrameMetrics {
	frame := ResolveTextFrameMetrics(shape)
	if shape == nil {
		return frame
	}
	w := drawing.EMUToPoints(shape.Bounds.W)
	h := drawing.EMUToPoints(shape.Bounds.H)
	if w > 0 && h > 0 {
		frame.InsetTopPt = math.Max(frame.InsetTopPt, h*visual.Geometry.ContentInsetTop)
		frame.InsetRightPt = math.Max(frame.InsetRightPt, w*visual.Geometry.ContentInsetRight)
		frame.InsetBottomPt = math.Max(frame.InsetBottomPt, h*visual.Geometry.ContentInsetBottom)
		frame.InsetLeftPt = math.Max(frame.InsetLeftPt, w*visual.Geometry.ContentInsetLeft)
	}
	return frame
}
