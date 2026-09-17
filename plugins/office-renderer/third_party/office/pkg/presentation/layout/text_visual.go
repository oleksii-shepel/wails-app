package layout

import ir "office/pkg/presentation/ir"

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
	frame := ResolveEffectiveTextFrameMetrics(shape, visual)
	flow := ResolveTextFlowMetrics(shape, frame)
	plan := TextVisualPlan{PhysicalCompositionPlan: composition, Frame: frame, Flow: flow, Geometry: ResolveTextGeometryVisualPlan(shape)}
	plan.Paragraphs = resolveTextParagraphVisualPlans(shape, flow, frame)
	plan.Layout = LayoutTextFrameFromParagraphPlansWithFlow(shape, frame, flow, plan.Paragraphs)

	// An omitted normAutofit fontScale is not permission to invent a shrink
	// factor from our own measurements. Keep the DrawingML default (100%) here.
	// When PowerPoint persisted an actual autofit scale, ResolveTextFrameMetrics
	// already applies it and both layout and rendering consume the same value.

	plan.Box = ResolveTextBoxVisualPlan(shape, plan.Frame, plan.Layout, visual)
	return plan
}

func resolveTextParagraphVisualPlans(shape *ir.Shape, flow TextFlowMetrics, frame TextFrameMetrics) []ParagraphVisualPlan {
	if shape == nil {
		return nil
	}
	extent := flow.LineExtentPt
	paragraphs := make([]ParagraphVisualPlan, 0, len(shape.Paragraphs))
	autoCounters := map[string]int{}
	for _, p := range shape.Paragraphs {
		pp := ResolveParagraphVisualPlan(p, extent, frame.FontScale, frame.LineScale)
		if pp.Marker.Kind == "auto" {
			if autoCounters[pp.Marker.CounterKey] == 0 {
				autoCounters[pp.Marker.CounterKey] = pp.Marker.Start
			}
			pp.Marker.Text = formatAutoNumForLayout(pp.Marker.NumberingType, autoCounters[pp.Marker.CounterKey])
			autoCounters[pp.Marker.CounterKey]++
			if !flow.NoWrap && CanLayoutParagraphLines(p, pp.Metrics, frame.FontScale) {
				lines := layoutParagraphLinesWithMarker(p, pp.Metrics, frame.FontScale, pp.Marker.Text, pp.Marker.FontSizePt, pp.Marker.FontFamily, pp.Marker.GapPt)
				pp.Lines = append([]PhysicalTextLine(nil), lines.Lines...)
				pp.LinesMeasured = true
			}
		} else {
			clear(autoCounters)
		}
		if flow.NoWrap {
			pp = ResolveNoWrapParagraphVisualPlan(p, pp, frame.FontScale)
		}
		paragraphs = append(paragraphs, pp)
	}
	return paragraphs
}
