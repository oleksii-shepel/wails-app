package render

import (
	"fmt"
	"html"
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
	if r.doc != nil && r.doc.Source != nil && box.Geometry.ColumnCount > 1 {
		// Legacy Word's column text reflows within each assigned column. Rendering
		// physical slots as absolute rows prevents that wrapping and duplicates
		// rows when CSS advances a fragment to its next column.
		r.renderParagraphSegmentContent(
			p,
			runs,
			!box.Fragment.IsContinuation,
			box.Fragment.IsContinuation,
			!box.Fragment.IsLast,
			box.Fragment.IsContinuation,
			box.Fragment.IsContinuation,
			false,
			"",
			func() { r.renderRuns(runs) },
		)
		return
	}
	extraStyle := fmt.Sprintf("position:relative;height:%.2fpt;min-height:%.2fpt;margin-top:0;margin-bottom:0;margin-left:0;margin-right:0;text-indent:0", box.Height, box.Height)
	savedInlineMarker := r.physicalInlineListMarker
	r.physicalInlineListMarker = p.List != nil && !box.Fragment.IsContinuation
	defer func() { r.physicalInlineListMarker = savedInlineMarker }()
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
	for lineIndex, line := range fragment.Lines {
		if lineIndex > 0 && len(runs) > 0 && physicalLineNeedsSpace(runs, line) {
			runs = append(runs, resolve.Run{Kind: "run", Text: " ", Format: runs[len(runs)-1].Format})
		}
		for _, slot := range line.Slots {
			runs = append(runs, slot.Runs...)
		}
	}
	return runs
}

func physicalLineNeedsSpace(previous []resolve.Run, next layout.ParagraphLineLayout) bool {
	last := previous[len(previous)-1].Text
	if last == "" || last[len(last)-1] == ' ' || last[len(last)-1] == '-' {
		return false
	}
	for _, slot := range next.Slots {
		for _, run := range slot.Runs {
			if run.Text != "" {
				return run.Text[0] != ' '
			}
		}
	}
	return false
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
			if lineIndex == 0 && slotIndex == 0 && r.physicalInlineListMarker && !fragment.IsContinuation {
				r.renderPhysicalInlineListMarker(box.Block.Paragraph, slot.Runs)
			}
			if len(slot.PositionedRuns) > 0 {
				if canRenderPhysicalRunsSequentially(slot.PositionedRuns) {
					r.renderPhysicalSequentialRuns(slot.PositionedRuns)
				} else {
					r.renderPhysicalPositionedRuns(slot.PositionedRuns, height)
				}
			} else {
				r.renderRuns(slot.Runs)
			}
			r.buf.WriteString(`</span>`)
		}
	}
}

// canRenderPhysicalRunsSequentially identifies the common tabbed-line case:
// the layout engine fixed the line and supplies one horizontal tab gap, but
// the text itself has no explicit offsets. Rendering each word at estimated
// x-coordinates lets browser glyph advances eat the intervening spaces.
func (r *htmlRenderer) renderPhysicalInlineListMarker(p *resolve.Paragraph, runs []resolve.Run) {
	marker, bullet := r.resolvedListMarker(p)
	if marker == "" {
		return
	}
	className := "word-list-marker word-physical-inline-list-marker"
	if bullet {
		className += " word-list-bullet"
	}
	ariaHidden := ""
	if bullet {
		ariaHidden = ` aria-hidden="true"`
	}
	style := r.listMarkerStyle(p, runs)
	if style != "" {
		fmt.Fprintf(&r.buf, `<span class=%q style=%q%s>%s</span>`, className, style, ariaHidden, html.EscapeString(marker))
		return
	}
	fmt.Fprintf(&r.buf, `<span class=%q%s>%s</span>`, className, ariaHidden, html.EscapeString(marker))
}

func canRenderPhysicalRunsSequentially(runs []layout.ParagraphRunLayout) bool {
	if len(runs) == 0 || math.Abs(runs[0].Left) > 0.01 {
		return false
	}
	for _, placed := range runs {
		if placed.Run.FieldKind == "ADVANCE" || math.Abs(placed.Top) > 0.01 || physicalInlineObject(placed.Run) {
			return false
		}
	}
	return true
}

func (r *htmlRenderer) renderPhysicalSequentialRuns(runs []layout.ParagraphRunLayout) {
	for _, placed := range runs {
		if placed.Run.Kind == "tab" {
			if placed.Width > 0 {
				leaderStyle := tabLeaderCSS(placed.Leader)
				if leaderStyle != "" {
					fmt.Fprintf(&r.buf, `<span class="word-physical-tab" aria-hidden="true" data-leader=%q style="display:inline-block;width:%.2fpt;height:%.2fpt;%s"></span>`, placed.Leader, placed.Width, placed.Height, leaderStyle)
				} else {
					fmt.Fprintf(&r.buf, `<span class="word-physical-tab" aria-hidden="true" style="display:inline-block;width:%.2fpt;height:%.2fpt"></span>`, placed.Width, placed.Height)
				}
			}
			continue
		}
		r.renderRun(layoutOwnedVerticalRun(placed.Run, placed.Height))
	}
}

func (r *htmlRenderer) renderPhysicalPositionedRuns(runs []layout.ParagraphRunLayout, lineHeight float64) {
	for _, placed := range runs {
		run := placed.Run
		if run.FieldKind == "ADVANCE" {
			fmt.Fprintf(&r.buf, `<span class="word-field word-field-advance" aria-hidden="true" data-layout-owned="true" style="position:absolute;left:%.2fpt;top:%.2fpt;width:0;height:0"></span>`, placed.Left, placed.Top)
			continue
		}
		if run.Kind == "tab" {
			if placed.Width > 0 {
				leaderStyle := tabLeaderCSS(placed.Leader)
				if leaderStyle != "" {
					fmt.Fprintf(&r.buf, `<span class="word-physical-tab" aria-hidden="true" data-leader=%q style="position:absolute;left:%.2fpt;top:%.2fpt;width:%.2fpt;height:%.2fpt;%s"></span>`, placed.Leader, placed.Left, placed.Top, placed.Width, lineHeight, leaderStyle)
				} else {
					fmt.Fprintf(&r.buf, `<span class="word-physical-tab" aria-hidden="true" style="position:absolute;left:%.2fpt;top:%.2fpt;width:%.2fpt;height:%.2fpt"></span>`, placed.Left, placed.Top, placed.Width, lineHeight)
				}
			}
			continue
		}
		style := fmt.Sprintf("position:absolute;left:%.2fpt;top:%.2fpt;white-space:nowrap", placed.Left, placed.Top)
		if placed.Width > 0 && placed.Height > 0 && physicalInlineObject(run) {
			style += fmt.Sprintf(";width:%.2fpt;height:%.2fpt", placed.Width, placed.Height)
		}
		fmt.Fprintf(&r.buf, `<span class="word-physical-run" style=%q>`, style)
		r.renderRun(layoutOwnedVerticalRun(run, placed.Height))
		r.buf.WriteString(`</span>`)
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

func physicalInlineObject(run resolve.Run) bool {
	switch run.Kind {
	case "image", "chart", "textbox", "shape", "olePlaceholder":
		return true
	default:
		return false
	}
}

func layoutOwnedVerticalRun(run resolve.Run, physicalHeightPt float64) resolve.Run {
	f := run.Format
	if f.VertAlignVal == "superscript" || f.VertAlignVal == "subscript" {
		if physicalHeightPt > 0 {
			v := int(math.Round(physicalHeightPt * 2))
			f.FontSize = &v
			if f.FontSizeCS != nil {
				vc := v
				f.FontSizeCS = &vc
			}
		} else if f.FontSize != nil && *f.FontSize > 0 {
			v := int(math.Round(float64(*f.FontSize) * 0.65))
			f.FontSize = &v
		}
		f.VertAlignVal = ""
	}
	// Physical layout already owns w:position. Clearing it prevents the generic
	// run renderer from applying a second CSS vertical-align offset.
	f.Position = nil
	run.Format = f
	return run
}
