package layout

import (
	"math"

	"office/pkg/fontmetrics"
	ir "office/pkg/presentation/ir"
)

// TextVisualPlan is the renderer-neutral physical composition contract for a
// presentation text body. Paragraph line boxes and frame placement are composed
// together so wrapping is measured exactly once against the same flow geometry.
type TextVisualPlan struct {
	// FrameBounds, Transform, Effects and Paint close the outer text-shape
	// composition boundary. TextFrame/Flow below own the inner text body.
	PhysicalCompositionPlan

	Frame      TextFrameMetrics
	Flow       TextFlowMetrics
	Layout     TextFrameLayout
	Geometry   TextGeometryVisualPlan
	Box        TextBoxVisualPlan
	Paragraphs []ParagraphVisualPlan
}

func ResolveTextVisualPlan(shape *ir.Shape) TextVisualPlan {
	return resolveTextVisualPlanWithShapeVisual(shape, ResolveShapeVisualPlan(shape))
}

// resolveTextVisualPlanWithShapeVisual owns the complete text composition pass:
// frame -> flow -> paragraph line boxes -> vertical frame measurement -> box.
// Serializers therefore never need a second wrapping or anchoring pass.
func resolveTextVisualPlanWithShapeVisual(shape *ir.Shape, visual ShapeVisualPlan) TextVisualPlan {
	return resolveTextVisualPlanWithShapeVisualAndComposition(shape, visual, resolvePhysicalCompositionPlan(shape))
}

func resolveTextVisualPlanWithShapeVisualAndComposition(shape *ir.Shape, visual ShapeVisualPlan, composition PhysicalCompositionPlan) TextVisualPlan {
	return resolveTextVisualPlanWithShapeVisualAndCompositionAndOptions(shape, visual, composition, PhysicalMaterializeOptions{})
}

func resolveTextVisualPlanWithShapeVisualAndCompositionAndOptions(shape *ir.Shape, visual ShapeVisualPlan, composition PhysicalCompositionPlan, opts PhysicalMaterializeOptions) TextVisualPlan {
	frame := ResolveEffectiveTextFrameMetrics(shape, visual)
	flow := ResolveTextFlowMetrics(shape, frame)
	flow = constrainTextFlowToSlide(shape, frame, flow, opts)
	plan := TextVisualPlan{PhysicalCompositionPlan: composition, Frame: frame, Flow: flow, Geometry: ResolveTextGeometryVisualPlan(shape)}
	plan.Paragraphs = resolveTextParagraphVisualPlansWithCatalog(shape, flow, frame, opts.TextFonts)
	plan.Layout = LayoutTextFrameFromParagraphPlansWithFlow(shape, frame, flow, plan.Paragraphs)

	// An omitted normAutofit fontScale is not permission to invent a shrink
	// factor from our own measurements. Keep the DrawingML default (100%) here.
	// When PowerPoint persisted an actual autofit scale, ResolveTextFrameMetrics
	// already applies it and both layout and rendering consume the same value.

	plan.Box = ResolveTextBoxVisualPlan(shape, plan.Frame, plan.Layout, visual)
	return plan
}

func resolveTextParagraphVisualPlans(shape *ir.Shape, flow TextFlowMetrics, frame TextFrameMetrics) []ParagraphVisualPlan {
	return resolveTextParagraphVisualPlansWithCatalog(shape, flow, frame, nil)
}

func resolveTextParagraphVisualPlansWithCatalog(shape *ir.Shape, flow TextFlowMetrics, frame TextFrameMetrics, catalog *fontmetrics.Catalog) []ParagraphVisualPlan {
	if shape == nil {
		return nil
	}
	extent := flow.LineExtentPt
	paragraphs := make([]ParagraphVisualPlan, 0, len(shape.Paragraphs))
	autoCounters := map[string]int{}
	for i, p := range shape.Paragraphs {
		pp := ResolveParagraphVisualPlan(p, extent, frame.FontScale, frame.LineScale)
		if pp.Marker.Kind == "auto" {
			if autoCounters[pp.Marker.CounterKey] == 0 {
				autoCounters[pp.Marker.CounterKey] = pp.Marker.Start
			}
			pp.Marker.Text = formatAutoNumForLayout(pp.Marker.NumberingType, autoCounters[pp.Marker.CounterKey])
			autoCounters[pp.Marker.CounterKey]++
			if !flow.NoWrap && CanLayoutParagraphLines(p, pp.Metrics, frame.FontScale) {
				lines := layoutParagraphLinesWithMarkerAndCatalog(p, pp.Metrics, frame.FontScale, pp.Marker.Text, pp.Marker.FontSizePt, pp.Marker.FontFamily, pp.Marker.GapPt, catalog)
				pp.Lines = append([]PhysicalTextLine(nil), lines.Lines...)
				pp.LinesMeasured = true
			}
		} else {
			clear(autoCounters)
		}
		if flow.NoWrap {
			pp = ResolveNoWrapParagraphVisualPlan(p, pp, frame.FontScale)
		}
		if i == 0 {
			SuppressLeadingSpaceBefore(&pp.Metrics)
		}
		paragraphs = append(paragraphs, pp)
	}
	return paragraphs
}

func constrainTextFlowToSlide(shape *ir.Shape, frame TextFrameMetrics, flow TextFlowMetrics, opts PhysicalMaterializeOptions) TextFlowMetrics {
	if shape == nil || opts.SlideWidthPt <= 0 || opts.SlideHeightPt <= 0 || flow.NoWrap {
		return flow
	}
	x, y := float64(shape.Bounds.X)/12700.0, float64(shape.Bounds.Y)/12700.0
	w, h := float64(shape.Bounds.W)/12700.0, float64(shape.Bounds.H)/12700.0
	if w <= 0 || h <= 0 {
		return flow
	}
	left, right := x+frame.InsetLeftPt, x+w-frame.InsetRightPt
	top, bottom := y+frame.InsetTopPt, y+h-frame.InsetBottomPt
	visibleW := math.Max(0, math.Min(right, opts.SlideWidthPt)-math.Max(left, 0))
	visibleH := math.Max(0, math.Min(bottom, opts.SlideHeightPt)-math.Max(top, 0))
	rot := math.Mod(math.Abs(flow.RotationDeg), 180)
	lineExtent := visibleW
	if flow.Vertical || math.Abs(rot-90) < 0.001 {
		lineExtent = visibleH
	}
	if flow.Columns > 1 {
		lineExtent = math.Max(0, lineExtent-float64(flow.Columns-1)*flow.ColumnGapPt) / float64(flow.Columns)
	}
	if lineExtent > 0 && (flow.LineExtentPt <= 0 || lineExtent < flow.LineExtentPt) {
		flow.LineExtentPt, flow.ColumnWidthPt = lineExtent, lineExtent
	}
	return flow
}
