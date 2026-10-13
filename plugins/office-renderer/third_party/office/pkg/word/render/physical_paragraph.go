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
// wrapping. List markers occupy a measured-width gutter in the first line slot.
func (r *htmlRenderer) renderPhysicalParagraphFragment(box layout.BlockBox, boxes []layout.BlockBox, index int) {
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
	// Paragraph spacing follows Word's adjacency rule (the larger of the
	// previous paragraph's space-after and this paragraph's space-before
	// becomes the single gap). Layout already resolved that split when it
	// measured the block; the raw style values would double the gap, so derive
	// the paddings from the measured box geometry instead.
	topInset := physicalFragmentTopInset(r.doc, box, boxes, index)
	contentHeight := 0.0
	for _, line := range box.Fragment.Lines {
		for _, slot := range line.Slots {
			if slot.Bottom > contentHeight {
				contentHeight = slot.Bottom
			}
		}
	}
	bottomInset := math.Max(0, box.Height-topInset-contentHeight)
	spacingStyle := fmt.Sprintf("padding-top:%.2fpt;padding-bottom:%.2fpt", topInset, bottomInset)

	extraStyle := fmt.Sprintf("position:relative;height:%.2fpt;min-height:%.2fpt;margin-top:0;margin-bottom:0;margin-left:0;margin-right:0;text-indent:0;%s", box.Height, box.Height, spacingStyle)
	if p.Format.NoteSeparator {
		// A note separator draws its rule on the paragraph's border box, so the
		// indents that position and size the rule must survive the physical
		// path's margin reset.
		extraStyle += noteSeparatorIndentStyle(p.Format)
	}
	if p.List != nil {
		// A list item may own a deeper list in the semantic HTML tree. Its own
		// physical row is absolutely positioned, so a fixed CSS height would make
		// the child list overflow the parent and overlap the next sibling. Keep the
		// measured row as a minimum, let descendants extend the item, and expose the
		// row height so the nested list starts below the absolute physical row.
		extraStyle = fmt.Sprintf("position:relative;min-height:%.2fpt;--word-list-item-flow-height:%.2fpt;margin-top:0;margin-bottom:0;margin-left:0;margin-right:0;text-indent:0;%s", box.Height, box.Height, spacingStyle)
	}
	if box.Geometry.ColumnCount > 1 {
		// Absolute line slots cannot fragment, so CSS multicol must not split the box.
		extraStyle += ";break-inside:avoid"
	}
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
			r.renderPhysicalParagraphSlots(box, topInset)
		},
	)
}

// physicalFragmentTopInset is the measured spacing-before of a physical
// fragment: spacing plus top border outset plus float clearance. It normally
// comes from the fragment's flow origin, but field blocks can carry a fragment
// measured at a stale flow position (the field moved pages after its
// paragraphs were pre-measured), so an implausible value falls back to the
// adjacency-resolved spacing that the layout paginator itself uses.
func physicalFragmentTopInset(doc *resolve.Document, box layout.BlockBox, boxes []layout.BlockBox, index int) float64 {
	if box.Fragment == nil {
		return 0
	}
	topInset := box.Fragment.OriginY - box.Y - box.Geometry.MarginTop
	if topInset < 0 || topInset > box.Height {
		if doc == nil || box.Block == nil || box.Block.Paragraph == nil {
			return 0
		}
		// The fragment was measured at a flow position that differs from the
		// box's final position (a block-spanning field moved to another page
		// after its paragraphs were pre-measured). Those measurements used the
		// paragraph's real neighbors, so re-derive the spacing without the
		// atFlowStart suppression.
		topInset = layout.ParagraphSpacingBefore(doc, box.Block.Paragraph, adjacentPageParagraph(boxes, index, -1), box.Geometry, false)
		if topInset < 0 {
			topInset = 0
		}
	}
	return topInset
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

func (r *htmlRenderer) renderPhysicalParagraphSlots(box layout.BlockBox, topInset float64) {
	fragment := box.Fragment
	if fragment == nil || len(fragment.Lines) == 0 {
		return
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
			top := topInset + slot.Top
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
	if marker == "" && (p == nil || p.List == nil || p.List.Picture == nil) {
		return
	}
	if p.List != nil && p.List.Picture != nil {
		markerWidth := physicalListMarkerWidthPt(p)
		fmt.Fprintf(&r.buf, `<span class="word-list-marker word-physical-inline-list-marker word-list-bullet" style="--word-list-marker-width:%.2fpt">`, markerWidth)
		r.renderPictureBullet(p.List.Picture, "word-list-bullet")
		r.buf.WriteString(`</span>`)
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
	markerWidth := physicalListMarkerWidthPt(p)
	if style != "" || markerWidth != 18 {
		if style != "" {
			style += ";"
		}
		style += fmt.Sprintf("--word-list-marker-width:%.2fpt", markerWidth)
	}
	if style == "" {
		fmt.Fprintf(&r.buf, `<span class=%q%s>%s</span>`, className, ariaHidden, html.EscapeString(marker))
		return
	}
	fmt.Fprintf(&r.buf, `<span class=%q%s style=%q>%s</span>`, className, ariaHidden, style, html.EscapeString(marker))
}

func physicalListMarkerWidthPt(p *resolve.Paragraph) float64 {
	width := 18.0
	if p != nil {
		if p.Format.IndentHanging != nil && *p.Format.IndentHanging > 0 {
			width = twipsToPoints(*p.Format.IndentHanging)
		} else if p.Format.IndentFirstLine != nil && *p.Format.IndentFirstLine < 0 {
			width = twipsToPoints(-*p.Format.IndentFirstLine)
		}
		if p.List != nil && p.List.Picture != nil {
			pictureWidth := float64(p.List.Picture.Width) / 12700.0
			if pictureWidth > width {
				width = pictureWidth
			}
		}
	}
	return width
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
	// inlineObjectFlowEnd tracks how far the in-flow inline objects have
	// advanced, so the next object's margin stays relative to the flow.
	inlineObjectFlowEnd := 0.0
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
			// An inline object is a character: Word rests its bottom edge on the
			// text baseline, so keep it in flow and let the browser do the same
			// instead of pinning it to the line box with an absolute top. Its
			// exact x still comes from layout, expressed as the margin the
			// inline flow has to advance.
			style = fmt.Sprintf("display:inline-block;position:relative;margin-left:%.2fpt;white-space:nowrap;width:%.2fpt;height:%.2fpt", placed.Left-inlineObjectFlowEnd, placed.Width, placed.Height)
			inlineObjectFlowEnd = placed.Left + placed.Width
		}
		fmt.Fprintf(&r.buf, `<span class="word-physical-run" style=%q>`, style)
		r.renderRun(layoutOwnedVerticalRun(run, placed.Height))
		r.buf.WriteString(`</span>`)
	}
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

// noteSeparatorIndentStyle restores the indents that give a note separator's
// rule its two-inch length and its start at the note text indent.
func noteSeparatorIndentStyle(f resolve.ParagraphFormat) string {
	style := ""
	if f.IndentLeft != nil && *f.IndentLeft > 0 {
		style += fmt.Sprintf(";margin-left:%.2fpt", twipsToPoints(*f.IndentLeft))
	}
	if f.IndentRight != nil && *f.IndentRight > 0 {
		style += fmt.Sprintf(";margin-right:%.2fpt", twipsToPoints(*f.IndentRight))
	}
	return style
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
