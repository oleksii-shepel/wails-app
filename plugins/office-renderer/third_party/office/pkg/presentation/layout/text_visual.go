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
	// A DrawingML shape keeps the authored scale: an omitted normAutofit
	// fontScale is not permission to invent a shrink factor from our own
	// measurements. A legacy frame is different - PowerPoint recomputes "shrink
	// text on overflow" every time it lays the shape out. Its own conversion of
	// 41246-2 slide 10 bakes the 28pt runs it derived from the authored 32pt
	// into the pptx, so a legacy frame that overflows its box takes the largest
	// step of 2.5% that fits, which is exactly the 87.5% PowerPoint found.
	plan := resolveTextPlanAtFontScale(shape, visual, composition, opts, 1)
	if shapeComputesShrinkToFit(shape, plan) {
		base := largestRunFontPt(shape)
		for _, scale := range legacyAutofitScales {
			if scale < legacyAutofitMinScale {
				break
			}
			candidate := resolveTextPlanAtFontScale(shape, visual, composition, opts, snapToWholePoints(scale, base))
			if !textPlanOverflowsBox(candidate) {
				return candidate
			}
		}
		// Nothing in range fits, so this is not a frame PowerPoint recomputes.
		// Leaving the plan untouched is the point: applying the smallest candidate
		// anyway is what collapsed 23884 slide 14's inherited 28pt body to 14pt.
	}
	return plan
}

// legacyAutofitScales walks down in PowerPoint's 2.5% shrink steps.
var legacyAutofitScales = []float64{
	0.975, 0.95, 0.925, 0.9, 0.875, 0.85, 0.825, 0.8, 0.775, 0.75,
	0.725, 0.7, 0.675, 0.65, 0.625, 0.6, 0.575, 0.55, 0.525, 0.5,
}

// legacyAutofitMinScale bounds how much PowerPoint's recompute-to-fit may take
// out of a legacy frame. The adjustment is modest by nature: 41246-2 slide 10's
// 32pt-styled intro lands on 28pt, a 12.5% cut. A frame that would need a fifth
// of its size or more is one PowerPoint does not recompute at all - 23884 slides
// 14 and 24 inherit 28pt into boxes that overflow by two to nine lines, and the
// deck's PDF draws every line at the inherited size, running past the box.
const legacyAutofitMinScale = 0.8

// shapeComputesShrinkToFit reports whether a shape with normAutofit and no
// authored font scale has to work out its own shrink: normAutofit is
// "shrink text on overflow", and the scale is what a renderer derives (see
// python-pptx: setting the autofit mode alone does not shrink anything until a
// scale is computed).
func shapeComputesShrinkToFit(shape *ir.Shape, plan TextVisualPlan) bool {
	if shape == nil || shape.TextBodyProps.AutofitFontScale >= 0 {
		return false
	}
	if shape.TextBodyProps.AutofitType != "normAutofit" {
		return false
	}
	if plan.Flow.NoWrap || plan.Frame.AutoGrow || plan.Frame.FontScale != 1 {
		return false
	}
	if !textPlanOverflowsBox(plan) {
		return false
	}
	// Only a real overflow shrinks: PowerPoint dropped a whole line on 41246-2
	// slide 10's intro (five lines into a four-line frame), while our own metrics
	// can leave a frame a point or two short of the vendor's and must not shrink
	// a shape PowerPoint drew at full size (slide 4's body keeps its 24pt).
	overflow := plan.Layout.ContentHeightPt - plan.Layout.AvailableHeightPt
	lineBox := largestRunFontPt(shape) * 1.2
	return overflow > lineBox*0.75
}

// largestRunFontPt returns the biggest effective run size in the shape, the
// base PowerPoint shrinks in whole points.
func largestRunFontPt(shape *ir.Shape) float64 {
	largest := 0.0
	for _, p := range shape.Paragraphs {
		if p == nil {
			continue
		}
		for _, r := range p.Runs {
			if r == nil {
				continue
			}
			if pt := float64(r.FontSize) / 100.0; pt > largest {
				largest = pt
			}
		}
	}
	return largest
}

func textPlanOverflowsBox(plan TextVisualPlan) bool {
	if !plan.Layout.Measured || plan.Layout.AvailableHeightPt <= 0 {
		return false
	}
	return plan.Layout.ContentHeightPt > plan.Layout.AvailableHeightPt+0.01
}

// resolveTextPlanAtFontScale is the whole composition pass for one scale.
func resolveTextPlanAtFontScale(shape *ir.Shape, visual ShapeVisualPlan, composition PhysicalCompositionPlan, opts PhysicalMaterializeOptions, scale float64) TextVisualPlan {
	frame := ResolveEffectiveTextFrameMetrics(shape, visual)
	if scale > 0 && scale < 1 {
		frame.FontScale *= scale
	}
	flow := ResolveTextFlowMetrics(shape, frame)
	flow = constrainTextFlowToSlide(shape, frame, flow, opts)
	plan := TextVisualPlan{PhysicalCompositionPlan: composition, Frame: frame, Flow: flow, Geometry: ResolveTextGeometryVisualPlan(shape)}
	plan.Paragraphs = resolveTextParagraphVisualPlansWithCatalog(shape, flow, frame, opts.TextFonts)
	plan.Layout = LayoutTextFrameFromParagraphPlansWithFlow(shape, frame, flow, plan.Paragraphs)
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

// snapToWholePoints rounds a shrink scale down so the resulting size is a whole
// number of points, which is how PowerPoint shrinks text (41246-2's 32pt intro
// becomes 28pt, its 36pt body 24pt).
func snapToWholePoints(scale, baseFontPt float64) float64 {
	if baseFontPt <= 0 {
		return scale
	}
	whole := math.Floor(baseFontPt * scale)
	if whole < 1 {
		return scale
	}
	return whole / baseFontPt
}
