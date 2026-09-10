package layout

import (
	"math"
	"strings"

	resolve "office/pkg/word/ir"
)

// lineSlot is one horizontal text-flow region inside a physical line band.
// Multiple slots may belong to the same physical line when a drawing occupies
// the middle of that line.
type lineSlot struct {
	BandIndex   int
	SlotIndex   int
	Top         float64
	Bottom      float64
	Span        lineSpan
	Constrained bool
}

func (s lineSlot) Width() float64 { return s.Span.Width() }

type paragraphSlotPlan struct {
	Slots             []lineSlot
	PhysicalLineCount int
}

func (p paragraphLinePlan) SlotPlan() paragraphSlotPlan {
	plan := paragraphSlotPlan{PhysicalLineCount: len(p.Bands)}
	for bandIndex, band := range p.Bands {
		for slotIndex, span := range band.Spans {
			if span.Width() <= 0 {
				continue
			}
			plan.Slots = append(plan.Slots, lineSlot{
				BandIndex:   bandIndex,
				SlotIndex:   slotIndex,
				Top:         band.Top,
				Bottom:      band.Bottom,
				Span:        span,
				Constrained: band.Constrained,
			})
		}
	}
	return plan
}

func (p paragraphSlotPlan) PhysicalLineRange(startSlot, endSlot int) (startLine, endLine int, ok bool) {
	if startSlot < 0 {
		startSlot = 0
	}
	if endSlot > len(p.Slots) {
		endSlot = len(p.Slots)
	}
	if startSlot >= endSlot || startSlot >= len(p.Slots) {
		return 0, 0, false
	}
	return p.Slots[startSlot].BandIndex, p.Slots[endSlot-1].BandIndex + 1, true
}

func (p paragraphLinePlan) PhysicalHeight(startSlot, endSlot int) float64 {
	slots := p.SlotPlan()
	startLine, endLine, ok := slots.PhysicalLineRange(startSlot, endSlot)
	if !ok || startLine < 0 || endLine > len(p.Bands) || startLine >= endLine {
		return 0
	}
	return p.Bands[endLine-1].Bottom - p.Bands[startLine].Top
}

func (p paragraphSlotPlan) SlotRangeForPhysicalLines(startLine, endLine int) (startSlot, endSlot int, ok bool) {
	if startLine < 0 {
		startLine = 0
	}
	if endLine > p.PhysicalLineCount {
		endLine = p.PhysicalLineCount
	}
	if startLine >= endLine {
		return 0, 0, false
	}
	startSlot = -1
	for i, slot := range p.Slots {
		if slot.BandIndex < startLine {
			continue
		}
		if slot.BandIndex >= endLine {
			break
		}
		if startSlot < 0 {
			startSlot = i
		}
		endSlot = i + 1
	}
	if startSlot < 0 {
		return 0, 0, false
	}
	return startSlot, endSlot, true
}

type paragraphSlotRun struct {
	Slot lineSlot
	Runs []resolve.Run
}

// paragraphPhysicalLine is the authoritative laid-out line. Spatial run
// ownership and metrics live together so sibling slots can never become
// independent vertical lines by accident.
type paragraphPhysicalLine struct {
	Slots  []paragraphSlotRun
	Metric paragraphLineMetric
}

func (l paragraphPhysicalLine) Runs() []resolve.Run {
	var runs []resolve.Run
	for _, slot := range l.Slots {
		runs = append(runs, slot.Runs...)
	}
	return runs
}

// paragraphSlotFlow owns physical lines only. Any flattened run view is a
// boundary projection and must never become layout state again.
type paragraphSlotFlow struct {
	Lines []paragraphPhysicalLine
}

type paragraphFlowContext struct {
	DefaultTabStopTwips int
}

func paragraphFlowContextForDocument(doc *resolve.Document) paragraphFlowContext {
	ctx := paragraphFlowContext{DefaultTabStopTwips: defaultWordTabStopTwips}
	if doc != nil && doc.DefaultTabStop > 0 {
		ctx.DefaultTabStopTwips = doc.DefaultTabStop
	}
	return ctx
}

func (f paragraphSlotFlow) LineRuns() [][]resolve.Run {
	return paragraphPhysicalLinesRunProjection(f.Lines)
}

func (f paragraphSlotFlow) LineMetrics() []paragraphLineMetric {
	out := make([]paragraphLineMetric, len(f.Lines))
	for i := range f.Lines {
		out[i] = f.Lines[i].Metric
	}
	return out
}

func paragraphPhysicalLinesRunProjection(lines []paragraphPhysicalLine) [][]resolve.Run {
	out := make([][]resolve.Run, len(lines))
	for i := range lines {
		out[i] = lines[i].Runs()
	}
	return out
}

// breakParagraphAcrossSlots is the single token-placement owner for span-first
// paragraph layout. Tokens move through slots horizontally. Moving to another
// slot in the same band does not create another physical line.

const defaultWordTabStopTwips = 720

type resolvedParagraphTabStop struct {
	PositionPt float64
	Alignment  string
	Leader     string
}

func nextParagraphTabStop(p *resolve.Paragraph, currentXPt float64, ctx paragraphFlowContext) resolvedParagraphTabStop {
	if ctx.DefaultTabStopTwips <= 0 {
		ctx.DefaultTabStopTwips = defaultWordTabStopTwips
	}
	if p != nil {
		best := resolvedParagraphTabStop{PositionPt: math.Inf(1)}
		for _, stop := range p.Format.TabStops {
			if stop.Pos <= 0 || stop.Algn == "bar" {
				continue
			}
			pos := float64(stop.Pos) / twipsPerPoint
			if pos > currentXPt+0.01 && pos < best.PositionPt {
				best = resolvedParagraphTabStop{
					PositionPt: pos,
					Alignment:  stop.Algn,
					Leader:     stop.Leader,
				}
			}
		}
		if !math.IsInf(best.PositionPt, 1) {
			if best.Alignment == "" {
				best.Alignment = "left"
			}
			return best
		}
	}
	step := float64(ctx.DefaultTabStopTwips) / twipsPerPoint
	return resolvedParagraphTabStop{
		PositionPt: (math.Floor(currentXPt/step) + 1) * step,
		Alignment:  "left",
	}
}

func paragraphTabAdvancePt(p *resolve.Paragraph, currentXPt float64, ctx paragraphFlowContext) float64 {
	return nextParagraphTabStop(p, currentXPt, ctx).PositionPt - currentXPt
}

func followingTabFieldWidthPt(tokens []paragraphRenderToken, start int, defaultSizePt float64) float64 {
	width := 0.0
	previousFontSize := 0.0
	for i := start; i < len(tokens); i++ {
		token := tokens[i]
		if token.tab || token.forceLineBreak {
			break
		}
		if token.separated && width > 0 {
			width += interWordSpaceWidthPt(previousFontSize, token.fontSizePt, defaultSizePt)
		}
		width += token.widthPt
		previousFontSize = token.fontSizePt
	}
	return width
}

func decimalPrefixWidthPt(tokens []paragraphRenderToken, start int, defaultSizePt float64) float64 {
	width := 0.0
	previousFontSize := 0.0
	for i := start; i < len(tokens); i++ {
		token := tokens[i]
		if token.tab || token.forceLineBreak {
			break
		}
		if token.separated && width > 0 {
			width += interWordSpaceWidthPt(previousFontSize, token.fontSizePt, defaultSizePt)
		}
		if token.run.Text != "" {
			if cut := strings.IndexAny(token.run.Text, ".,"); cut >= 0 {
				prefix := []rune(token.run.Text)[:cut]
				if len(prefix) > 0 {
					w, _ := runWordWidthPt(token.run.Format, string(prefix), defaultSizePt)
					width += w
				}
				return width
			}
		}
		width += token.widthPt
		previousFontSize = token.fontSizePt
	}
	return width
}

func alignedTabAdvancePt(p *resolve.Paragraph, tokens []paragraphRenderToken, tokenIndex int, currentXPt float64, ctx paragraphFlowContext, defaultSizePt float64) float64 {
	stop := nextParagraphTabStop(p, currentXPt, ctx)
	target := stop.PositionPt
	switch stop.Alignment {
	case "center":
		target -= followingTabFieldWidthPt(tokens, tokenIndex+1, defaultSizePt) / 2
	case "right":
		target -= followingTabFieldWidthPt(tokens, tokenIndex+1, defaultSizePt)
	case "decimal":
		target -= decimalPrefixWidthPt(tokens, tokenIndex+1, defaultSizePt)
	}
	if target <= currentXPt {
		return 0
	}
	return target - currentXPt
}

func breakParagraphAcrossSlots(p *resolve.Paragraph, plan paragraphSlotPlan, defaultSizePt float64) paragraphSlotFlow {
	return breakParagraphAcrossSlotsWithContext(p, plan, defaultSizePt, paragraphFlowContext{})
}

func breakParagraphAcrossSlotsWithContext(p *resolve.Paragraph, plan paragraphSlotPlan, defaultSizePt float64, ctx paragraphFlowContext) paragraphSlotFlow {
	physicalCount := plan.PhysicalLineCount
	if physicalCount < 1 {
		physicalCount = 1
	}
	flow := paragraphSlotFlow{Lines: make([]paragraphPhysicalLine, physicalCount)}
	for i := range flow.Lines {
		flow.Lines[i].Metric.maxFontSizePt = defaultSizePt
	}

	if p == nil {
		return flow
	}
	slotsByLine := make([][]lineSlot, physicalCount)
	for _, slot := range plan.Slots {
		if slot.BandIndex < 0 {
			continue
		}
		for slot.BandIndex >= len(slotsByLine) {
			slotsByLine = append(slotsByLine, nil)
		}
		slotsByLine[slot.BandIndex] = append(slotsByLine[slot.BandIndex], slot)
	}

	var fallbackSlots []lineSlot
	for i := len(slotsByLine) - 1; i >= 0; i-- {
		if len(slotsByLine[i]) > 0 {
			fallbackSlots = slotsByLine[i]
			break
		}
	}
	if len(fallbackSlots) == 0 {
		fallbackSlots = []lineSlot{{BandIndex: 0, SlotIndex: 0, Span: lineSpan{Left: 0, Right: 1}}}
	}

	ensureLine := func(line int) {
		for line >= len(flow.Lines) {
			flow.Lines = append(flow.Lines, paragraphPhysicalLine{
				Metric: paragraphLineMetric{maxFontSizePt: defaultSizePt},
			})
		}
		for line >= len(slotsByLine) {
			nextLine := len(slotsByLine)
			continuation := make([]lineSlot, len(fallbackSlots))
			for i, slot := range fallbackSlots {
				slot.BandIndex = nextLine
				slot.SlotIndex = i
				continuation[i] = slot
			}
			slotsByLine = append(slotsByLine, continuation)
		}
	}

	lineIndex, slotIndex := 0, 0
	currentWidth, previousFontSize := 0.0, 0.0
	advanceSlot := func() {
		slotIndex++
		currentWidth, previousFontSize = 0, 0
		for {
			ensureLine(lineIndex)
			if slotIndex < len(slotsByLine[lineIndex]) {
				return
			}
			lineIndex++
			slotIndex = 0
			ensureLine(lineIndex)
			if len(slotsByLine[lineIndex]) > 0 {
				return
			}
		}
	}

	tokens := paragraphSpatialRenderTokens(p, defaultSizePt)
	if len(tokens) == 0 {
		return flow
	}

	ensureLine(0)
	for tokenIndex, token := range tokens {
		if token.tab {
			for {
				ensureLine(lineIndex)
				if len(slotsByLine[lineIndex]) == 0 {
					lineIndex++
					slotIndex = 0
					currentWidth, previousFontSize = 0, 0
					continue
				}
				slot := slotsByLine[lineIndex][slotIndex]
				currentX := slot.Span.Left + currentWidth
				advance := alignedTabAdvancePt(p, tokens, tokenIndex, currentX-slot.Span.Left, ctx, defaultSizePt)
				if currentWidth == 0 || currentWidth+advance <= slot.Width()+0.01 {
					for len(flow.Lines[lineIndex].Slots) <= slotIndex {
						i := len(flow.Lines[lineIndex].Slots)
						flow.Lines[lineIndex].Slots = append(flow.Lines[lineIndex].Slots, paragraphSlotRun{Slot: slotsByLine[lineIndex][i]})
					}
					owner := &flow.Lines[lineIndex].Slots[slotIndex]
					owner.Runs = append(owner.Runs, token.run)
					currentWidth += advance
					previousFontSize = token.fontSizePt
					break
				}
				advanceSlot()
			}
			continue
		}
		if token.forceLineBreak {
			ensureLine(lineIndex)
			if len(slotsByLine[lineIndex]) == 0 {
				lineIndex++
				slotIndex = 0
				currentWidth, previousFontSize = 0, 0
				continue
			}
			if slotIndex >= len(slotsByLine[lineIndex]) {
				slotIndex = len(slotsByLine[lineIndex]) - 1
			}
			for len(flow.Lines[lineIndex].Slots) <= slotIndex {
				i := len(flow.Lines[lineIndex].Slots)
				flow.Lines[lineIndex].Slots = append(flow.Lines[lineIndex].Slots, paragraphSlotRun{Slot: slotsByLine[lineIndex][i]})
			}
			// The break is consumed by physical layout. Do not retain it in the
			// renderer-facing slot: the following content is already positioned on
			// the next physical line, so rendering a <br> here would apply the break
			// a second time inside the absolutely positioned slot.
			mergeRenderTokenMetric(&flow.Lines[lineIndex].Metric, token, defaultSizePt)
			lineIndex++
			slotIndex = 0
			currentWidth, previousFontSize = 0, 0
			ensureLine(lineIndex)
			continue
		}
		for {
			ensureLine(lineIndex)
			if len(slotsByLine[lineIndex]) == 0 {
				lineIndex++
				slotIndex = 0
				continue
			}
			slot := slotsByLine[lineIndex][slotIndex]
			if slot.Width() <= 0 {
				advanceSlot()
				continue
			}
			space := interWordSpaceWidthPt(previousFontSize, token.fontSizePt, defaultSizePt)
			needsSpace := currentWidth > 0 && token.separated
			needed := token.widthPt
			if needsSpace {
				needed += space
			}
			if currentWidth == 0 || currentWidth+needed <= slot.Width() {
				for len(flow.Lines[lineIndex].Slots) <= slotIndex {
					i := len(flow.Lines[lineIndex].Slots)
					flow.Lines[lineIndex].Slots = append(flow.Lines[lineIndex].Slots, paragraphSlotRun{Slot: slotsByLine[lineIndex][i]})
				}
				owner := &flow.Lines[lineIndex].Slots[slotIndex]
				if needsSpace {
					spaceRun := resolve.Run{Kind: "run", Text: " ", Format: token.run.Format}
					owner.Runs = append(owner.Runs, spaceRun)
					currentWidth += space
				}
				owner.Runs = append(owner.Runs, token.run)
				currentWidth += token.widthPt
				previousFontSize = token.fontSizePt
				mergeRenderTokenMetric(&flow.Lines[lineIndex].Metric, token, defaultSizePt)
				break
			}
			advanceSlot()
		}
	}

	lastUsed := 0
	for i := range flow.Lines {
		if len(flow.Lines[i].Runs()) > 0 {
			lastUsed = i
		}
	}
	flow.Lines = flow.Lines[:lastUsed+1]
	return flow
}

func mergeRenderTokenMetric(dst *paragraphLineMetric, token paragraphRenderToken, defaultSizePt float64) {
	dst.maxFontSizePt = math.Max(dst.maxFontSizePt, math.Max(defaultSizePt, token.fontSizePt))
	if token.run.Kind == "image" && token.run.Image != nil && !token.run.Image.Floating {
		dst.maxInlineHeightPt = math.Max(dst.maxInlineHeightPt, emuToPoints(token.run.Image.Height))
	}
}

func mergeParagraphLineMetric(dst *paragraphLineMetric, src paragraphLineMetric) {
	dst.maxFontSizePt = math.Max(dst.maxFontSizePt, src.maxFontSizePt)
	dst.maxInlineHeightPt = math.Max(dst.maxInlineHeightPt, src.maxInlineHeightPt)
}

func paragraphPhysicalRunLines(p *resolve.Paragraph, plan paragraphLinePlan, defaultSizePt float64) [][]resolve.Run {
	return paragraphPhysicalLinesRunProjection(breakParagraphAcrossSlots(p, plan.SlotPlan(), defaultSizePt).Lines)
}

func paragraphLinePlanForFlow(
	firstLeft, firstWidth,
	continuationLeft, continuationWidth,
	startY float64,
	lineHeights []float64,
	fallbackHeight float64,
	exclusions []lineExclusion,
) paragraphLinePlan {
	return planParagraphLineBands(
		firstLeft, firstLeft+firstWidth,
		continuationLeft, continuationLeft+continuationWidth,
		startY, lineHeights, fallbackHeight, exclusions,
	)
}

type paragraphFlowPlan struct {
	Lines         paragraphLinePlan
	PhysicalLines []paragraphPhysicalLine
}

func (p paragraphFlowPlan) PhysicalLineCount() int { return len(p.Lines.Bands) }

// LineRuns is a compatibility projection for consumers that have not yet
// become slot-aware. PhysicalLines remains the sole stored paragraph payload.
func (p paragraphFlowPlan) LineRuns() [][]resolve.Run {
	return paragraphPhysicalLinesRunProjection(p.PhysicalLines)
}

func buildParagraphFlowPlan(
	paragraph *resolve.Paragraph,
	firstLeft, firstWidth,
	continuationLeft, continuationWidth,
	startY float64,
	lineHeights []float64,
	fallbackHeight float64,
	exclusions []lineExclusion,
	defaultSizePt float64,
) paragraphFlowPlan {
	lines := paragraphLinePlanForFlow(firstLeft, firstWidth, continuationLeft, continuationWidth, startY, lineHeights, fallbackHeight, exclusions)
	slotFlow := breakParagraphAcrossSlotsWithContext(paragraph, lines.SlotPlan(), defaultSizePt, paragraphFlowContext{})
	return paragraphFlowPlan{Lines: lines, PhysicalLines: slotFlow.Lines}
}

func convergedParagraphFlowPlan(
	paragraph *resolve.Paragraph,
	geom PageGeometry,
	firstLeft, firstWidth,
	continuationLeft, continuationWidth,
	startY, fallbackHeight, defaultSizePt float64,
	initialHeights []float64,
	exclusionsForHeight func(float64) []lineExclusion,
	flowContext ...paragraphFlowContext,
) (paragraphFlowPlan, []paragraphLineMetric, []float64) {
	ctx := paragraphFlowContext{}
	if len(flowContext) > 0 {
		ctx = flowContext[0]
	}
	heights := append([]float64(nil), initialHeights...)
	var metrics []paragraphLineMetric
	var plan paragraphFlowPlan

	for range 4 {
		exclusions := exclusionsForHeight(sumFloat64s(heights))
		lines := paragraphLinePlanForFlow(firstLeft, firstWidth, continuationLeft, continuationWidth, startY, heights, fallbackHeight, exclusions)
		slotFlow := breakParagraphAcrossSlotsWithContext(paragraph, lines.SlotPlan(), defaultSizePt, ctx)
		metrics = slotFlow.LineMetrics()
		newHeights := paragraphLineHeightsPt(paragraph, geom, metrics, defaultSizePt)
		plan = paragraphFlowPlan{Lines: lines, PhysicalLines: slotFlow.Lines}
		if len(newHeights) == len(heights) && floatSliceAlmostSame(newHeights, heights) {
			heights = newHeights
			break
		}
		heights = newHeights
	}

	exclusions := exclusionsForHeight(sumFloat64s(heights))
	lines := paragraphLinePlanForFlow(firstLeft, firstWidth, continuationLeft, continuationWidth, startY, heights, fallbackHeight, exclusions)
	slotFlow := breakParagraphAcrossSlotsWithContext(paragraph, lines.SlotPlan(), defaultSizePt, ctx)
	plan = paragraphFlowPlan{Lines: lines, PhysicalLines: slotFlow.Lines}
	metrics = slotFlow.LineMetrics()
	return plan, metrics, heights
}
