package layout

import (
	"math"

	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
)

// TextBoxVisualPlan is the physical box contract for a composed text body.
// It owns effective content insets, vertical placement, growth and clipping so
// serializers do not reconstruct those decisions from frame/body properties.
type TextBoxVisualPlan struct {
	InsetTopPt, InsetRightPt, InsetBottomPt, InsetLeftPt         float64
	ContentTopPt, ContentRightPt, ContentBottomPt, ContentLeftPt float64
	OuterWidthPt, MinOuterWidthPt, OuterHeightPt, MinOuterHeightPt float64
	ContentAutoWidth, ContentAutoHeight                           bool
	OverflowVisible                                              bool
	ClipContent                                                  bool
}

func ResolveTextBoxVisualPlan(shape *ir.Shape, frame TextFrameMetrics, frameLayout TextFrameLayout, visual ShapeVisualPlan) TextBoxVisualPlan {
	p := TextBoxVisualPlan{
		InsetTopPt: frame.InsetTopPt, InsetRightPt: frame.InsetRightPt,
		InsetBottomPt: frame.InsetBottomPt, InsetLeftPt: frame.InsetLeftPt,
		ContentAutoHeight: frame.AutoGrow,
		OverflowVisible:   frame.AutoGrow || frame.AllowOverflow,
		ClipContent:       !(frame.AutoGrow || frame.AllowOverflow),
	}
	if shape == nil {
		return p
	}
	w := drawing.EMUToPoints(shape.Bounds.W)
	h := drawing.EMUToPoints(shape.Bounds.H)
	p.OuterWidthPt = w
	p.OuterHeightPt = h
	if w > 0 && h > 0 {
		p.InsetTopPt = math.Max(p.InsetTopPt, h*visual.Geometry.ContentInsetTop)
		p.InsetRightPt = math.Max(p.InsetRightPt, w*visual.Geometry.ContentInsetRight)
		p.InsetBottomPt = math.Max(p.InsetBottomPt, h*visual.Geometry.ContentInsetBottom)
		p.InsetLeftPt = math.Max(p.InsetLeftPt, w*visual.Geometry.ContentInsetLeft)
	}
	if frameLayout.Measured {
		// Vertical writing advances lines horizontally. Apply body anchoring on
		// that physical progression edge instead of incorrectly adding top padding.
		flow := ResolveTextFlowMetrics(shape, frame)
		if flow.Vertical {
			if flow.WritingMode == "vertical-rl" {
				p.ContentRightPt = frameLayout.VerticalOffsetPt
			} else {
				p.ContentLeftPt = frameLayout.VerticalOffsetPt
			}
		} else {
			p.ContentTopPt = frameLayout.VerticalOffsetPt
		}
	}
	if frame.AutoGrow && frameLayout.Measured {
		flow := ResolveTextFlowMetrics(shape, frame)
		// spAutoFit grows the shape on the physical line-progression axis. For
		// ordinary horizontal text that is height; for vertical writing it is
		// width. Keeping this decision in the physical box prevents the HTML
		// renderer from treating every auto-growing text body as height:auto.
		if flow.Vertical {
			p.ContentAutoHeight = false
			p.ContentAutoWidth = true
			p.MinOuterWidthPt = p.InsetLeftPt + frameLayout.ContentHeightPt + p.InsetRightPt
			if p.MinOuterWidthPt < w {
				p.MinOuterWidthPt = w
			}
		} else {
			// RequiredHeightPt is based on canonical text-body insets. Recompute
			// with the effective geometry-aware insets owned by this physical box.
			p.MinOuterHeightPt = p.InsetTopPt + frameLayout.ContentHeightPt + p.InsetBottomPt
			if p.MinOuterHeightPt < h {
				p.MinOuterHeightPt = h
			}
		}
	}
	return p
}
