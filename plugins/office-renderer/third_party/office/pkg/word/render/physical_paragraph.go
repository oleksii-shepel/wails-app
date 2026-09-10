package render

import (
	"fmt"
	"math"

	resolve "office/pkg/word/ir"
	"office/pkg/word/layout"
)

// renderPhysicalParagraphFragment paints the layout engine's physical-line
// projection instead of asking the browser to reflow text that layout has
// already measured. This is the canonical renderer boundary for ordinary
// paragraphs, explicit line breaks, fragmented paragraphs, and Square/Tight
// wrapping. List markers remain a temporary exception until marker geometry is
// represented by physical layout.
func (r *htmlRenderer) renderPhysicalParagraphFragment(box layout.BlockBox) {
	if box.Block == nil || box.Block.Paragraph == nil || box.Fragment == nil || len(box.Fragment.Lines) == 0 {
		return
	}
	p := box.Block.Paragraph

	// List container and marker semantics are still emitted by
	// renderParagraphSegmentContent, but the item body now comes from the same
	// physical slots that pagination measured.
	runs := physicalFragmentRuns(box.Fragment)
	extraStyle := fmt.Sprintf("position:relative;height:%.2fpt;min-height:%.2fpt;margin-top:0;margin-bottom:0;margin-left:0;margin-right:0;text-indent:0", box.Height, box.Height)
	r.renderParagraphSegmentContent(
		p,
		runs,
		!box.Fragment.IsContinuation,
		box.Fragment.IsContinuation,
		!box.Fragment.IsLast,
		box.Fragment.IsContinuation,
		box.Fragment.IsContinuation,
		box.Fragment.StartsNewColumn,
		extraStyle,
		func() {
			if !box.Fragment.IsContinuation {
				r.renderPhysicalFloatingAnchors(p)
			}
			r.renderPhysicalParagraphSlots(box)
		},
	)
}

func physicalFragmentRuns(fragment *layout.ParagraphFragment) []resolve.Run {
	if fragment == nil {
		return nil
	}
	var runs []resolve.Run
	for _, line := range fragment.Lines {
		for _, slot := range line.Slots {
			runs = append(runs, slot.Runs...)
		}
	}
	return runs
}

func (r *htmlRenderer) renderPhysicalFloatingAnchors(p *resolve.Paragraph) {
	if p == nil {
		return
	}
	for _, run := range p.Runs {
		switch {
		case run.Image != nil && run.Image.Floating:
			r.renderRun(run)
		case run.TextBox != nil && run.TextBox.Floating:
			r.renderRun(run)
		case run.Shape != nil && run.Shape.Floating:
			r.renderRun(run)
		}
	}
}

func (r *htmlRenderer) renderPhysicalParagraphSlots(box layout.BlockBox) {
	fragment := box.Fragment
	if fragment == nil || len(fragment.Lines) == 0 {
		return
	}

	originTop, ok := physicalFragmentTop(fragment)
	if !ok {
		return
	}
	topInset := 0.0
	if !fragment.IsContinuation {
		topInset = originTop - (box.Y + box.Geometry.MarginTop)
		if topInset < 0 {
			topInset = 0
		}
	}
	columnLeft := rendererColumnLeftPt(box.Geometry, box.Column)

	for lineIndex, line := range fragment.Lines {
		for slotIndex, slot := range line.Slots {
			width := math.Max(0, slot.Right-slot.Left)
			height := math.Max(0, slot.Bottom-slot.Top)
			if width <= 0 || height <= 0 {
				continue
			}
			left := slot.Left - columnLeft
			top := topInset + slot.Top - originTop
			fmt.Fprintf(&r.buf, `<span class="word-physical-slot" data-line="%d" data-slot="%d" style="position:absolute;left:%.2fpt;top:%.2fpt;width:%.2fpt;height:%.2fpt;line-height:%.2fpt;white-space:nowrap;overflow:visible">`, lineIndex, slotIndex, left, top, width, height, height)
			r.renderRuns(slot.Runs)
			r.buf.WriteString(`</span>`)
		}
	}
}

func physicalFragmentTop(fragment *layout.ParagraphFragment) (float64, bool) {
	if fragment == nil {
		return 0, false
	}
	for _, line := range fragment.Lines {
		if len(line.Slots) > 0 {
			top := line.Slots[0].Top
			for _, slot := range line.Slots[1:] {
				top = math.Min(top, slot.Top)
			}
			return top, true
		}
	}
	return 0, false
}

func rendererColumnLeftPt(geom layout.PageGeometry, column int) float64 {
	if column < 0 {
		column = 0
	}
	if geom.ColumnCount <= 1 {
		return geom.MarginLeft
	}
	if column >= geom.ColumnCount {
		column = geom.ColumnCount - 1
	}
	if len(geom.ColumnWidthsPt) >= geom.ColumnCount {
		x := geom.MarginLeft
		for i := 0; i < column; i++ {
			x += geom.ColumnWidthsPt[i]
			if i < len(geom.ColumnGapsPt) {
				x += geom.ColumnGapsPt[i]
			} else {
				x += geom.ColumnGapPt
			}
		}
		return x
	}
	return geom.MarginLeft + float64(column)*(geom.ColumnWidthPt+geom.ColumnGapPt)
}
