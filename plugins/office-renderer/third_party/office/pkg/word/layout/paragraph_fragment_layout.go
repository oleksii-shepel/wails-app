package layout

import resolve "office/pkg/word/ir"

// ParagraphLineLayout is the renderer-facing geometry of one physical text
// line. A line can contain multiple horizontal slots when text wraps around a
// floating drawing. Slots are spatial siblings, not independent vertical lines.
type ParagraphLineLayout struct {
	Slots []ParagraphSlotLayout
}

// ParagraphSlotLayout is one horizontal text region within a physical line.
// Coordinates are in layout points in the same flow coordinate space used by
// the paragraph line planner.
type ParagraphRunLayout struct {
	Run    resolve.Run
	Left   float64
	Top    float64
	Width  float64
	Height float64
	Leader string
}

type ParagraphSlotLayout struct {
	Left           float64
	Right          float64
	Top            float64
	Bottom         float64
	Runs           []resolve.Run
	PositionedRuns []ParagraphRunLayout
}

func paragraphLineLayouts(lines []paragraphPhysicalLine) []ParagraphLineLayout {
	if len(lines) == 0 {
		return nil
	}
	// Physical line planning uses page/column Y coordinates because float
	// exclusions live in that coordinate space. ParagraphFragment geometry is
	// renderer-facing, however, so its vertical coordinates must be local to
	// the fragment. Rebase every fragment to its first physical line while
	// preserving the measured distances between lines and slots.
	originY := 0.0
	haveOrigin := false
	for _, line := range lines {
		for _, slot := range line.Slots {
			if !haveOrigin || slot.Slot.Top < originY {
				originY = slot.Slot.Top
				haveOrigin = true
			}
		}
	}
	out := make([]ParagraphLineLayout, len(lines))
	for i, line := range lines {
		out[i].Slots = make([]ParagraphSlotLayout, len(line.Slots))
		for j, slot := range line.Slots {
			layoutSlot := ParagraphSlotLayout{
				Left:   slot.Slot.Span.Left,
				Right:  slot.Slot.Span.Right,
				Top:    slot.Slot.Top - originY,
				Bottom: slot.Slot.Bottom - originY,
				Runs:   slot.Runs,
			}
			if slot.HasAdvance || slot.HasPositioned {
				layoutSlot.PositionedRuns = make([]ParagraphRunLayout, 0, len(slot.Positioned))
				lineHeight := slot.Slot.Bottom - slot.Slot.Top
				for _, positioned := range slot.Positioned {
					height := positioned.HeightPt
					top := positionedRunFinalTop(positioned.Run, positioned.TopPt, height, lineHeight)
					layoutSlot.PositionedRuns = append(layoutSlot.PositionedRuns, ParagraphRunLayout{
						Run: positioned.Run, Left: positioned.LeftPt, Top: top, Width: positioned.WidthPt, Height: height, Leader: positioned.Leader,
					})
				}
			}
			out[i].Slots[j] = layoutSlot
		}
	}
	return out
}

// paragraphPhysicalLinesRuns is a compatibility projection for consumers that
// still render ParagraphFragment.Runs. Geometry-aware renderers must use Lines.
func paragraphPhysicalLinesRuns(lines []paragraphPhysicalLine) []resolve.Run {
	var runs []resolve.Run
	for _, line := range lines {
		runs = append(runs, line.Runs()...)
	}
	return runs
}
