package legacy

import (
	"encoding/binary"
	"sort"
	"strings"

	wordir "office/pkg/word/ir"
)

const (
	legacyFibPlcftxbxTxt    = 56
	legacyFibPlcfHdrtxbxTxt = 58
	legacyFTXBXSSize        = 22
	legacyFTXBXSLidOffset   = 14
)

type legacyTextboxOwnedRange struct {
	StartCP int
	EndCP   int
	ShapeID uint32
	Header  bool
}

func readLegacyTextboxOwnedRanges(tableStream []byte, fibBase, cbRgFcLcb int, index int, header bool) []legacyTextboxOwnedRange {
	if index < 0 || index >= cbRgFcLcb || fibBase < 0 {
		return nil
	}
	pair := fibBase + index*8
	if pair < 0 || pair+8 > len(tableStream)+fibBase {
		// FIB fc/lcb pairs live in WordDocument, not the table stream; caller
		// passes the extracted fc/lcb separately through the wrapper below.
		return nil
	}
	return nil
}

func legacyFibRange(wordStream []byte, fibBase, cbRgFcLcb, index int) (int, int, bool) {
	if index < 0 || index >= cbRgFcLcb {
		return 0, 0, false
	}
	off := fibBase + index*8
	if off < 0 || off+8 > len(wordStream) {
		return 0, 0, false
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[off : off+4]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[off+4 : off+8]))
	if fc < 0 || lcb <= 0 {
		return 0, 0, false
	}
	return fc, lcb, true
}

func parseLegacyTextboxPLC(blob []byte, header bool) []legacyTextboxOwnedRange {
	if len(blob) < 4+legacyFTXBXSSize {
		return nil
	}
	// PLC layout: (n+1) CPs followed by n FTXBXS records:
	// lcb = 4*(n+1) + 22*n = 26*n + 4.
	if (len(blob)-4)%26 != 0 {
		return nil
	}
	n := (len(blob) - 4) / 26
	if n <= 0 {
		return nil
	}
	cpBytes := 4 * (n + 1)
	if cpBytes+n*legacyFTXBXSSize != len(blob) {
		return nil
	}
	out := make([]legacyTextboxOwnedRange, 0, n)
	for i := 0; i < n; i++ {
		cp0 := int(binary.LittleEndian.Uint32(blob[i*4 : i*4+4]))
		cp1 := int(binary.LittleEndian.Uint32(blob[(i+1)*4 : (i+1)*4+4]))
		rec := blob[cpBytes+i*legacyFTXBXSSize : cpBytes+(i+1)*legacyFTXBXSSize]
		lid := binary.LittleEndian.Uint32(rec[legacyFTXBXSLidOffset : legacyFTXBXSLidOffset+4])
		if cp1 <= cp0 || lid == 0 {
			continue
		}
		out = append(out, legacyTextboxOwnedRange{StartCP: cp0, EndCP: cp1, ShapeID: lid, Header: header})
	}
	return out
}

func loadLegacyTextboxOwnedRanges(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) []legacyTextboxOwnedRange {
	var out []legacyTextboxOwnedRange
	for _, spec := range []struct {
		index  int
		header bool
	}{
		{legacyFibPlcftxbxTxt, false},
		{legacyFibPlcfHdrtxbxTxt, true},
	} {
		fc, lcb, ok := legacyFibRange(wordStream, fibBase, cbRgFcLcb, spec.index)
		if !ok || fc+lcb > len(tableStream) {
			continue
		}
		out = append(out, parseLegacyTextboxPLC(tableStream[fc:fc+lcb], spec.header)...)
	}
	return out
}

func legacyTextboxStoryBase(counts legacyStoryCounts, header bool) int {
	base := counts.Text + counts.Ftn + counts.Hdr + counts.Mcr + counts.Atn + counts.Edn
	if header {
		base += counts.Txbx
	}
	return base
}

// legacyTextboxAnchorByShapeID builds the ownership-position map for both
// textbox story domains. Main textbox ranges (PlcftxbxTxt) are owned by shapes
// from PlcSpaMom, while header textbox ranges (PlcfHdrtxbxTxt) are owned by
// shapes from PlcSpaHdr. Previously only the main-story anchors were supplied,
// so header textboxes found their FTXBXS.lid owner but lost its geometry.
func loadLegacyHeaderFloatingDrawingAnchors(
	wordStream, tableStream []byte,
	fibBase, cbRgFcLcb, cpBase, storyTextLen int,
) map[int]legacyFloatingDrawingAnchor {
	fc, lcb, ok := readFibFcLcbEntry(
		wordStream, fibBase, cbRgFcLcb, legacyFibPlcSpaHdr,
	)
	if !ok || lcb <= 0 || fc < 0 || fc+lcb > len(tableStream) {
		return nil
	}
	anchors, valid := parseLegacyPlcfSpa(tableStream[fc:fc+lcb], storyTextLen)
	if !valid || len(anchors) == 0 {
		return nil
	}
	out := make(map[int]legacyFloatingDrawingAnchor, len(anchors))
	for _, anchor := range anchors {
		anchor.CP += cpBase
		out[anchor.CP] = anchor
	}
	return out
}

func legacyTextboxAnchorByShapeID(
	wordStream, tableStream []byte,
	fibBase, cbRgFcLcb int,
	counts legacyStoryCounts,
	mainAnchors map[int]legacyFloatingDrawingAnchor,
) map[uint32]legacyFloatingDrawingAnchor {
	out := legacyAnchorByShapeID(mainAnchors)

	// Header Document CPs begin after Main + Footnote stories in the global
	// character stream. The CP base does not affect lid/spid ownership itself,
	// but preserving the correct global CP keeps this map useful for diagnostics
	// and avoids silently mixing two coordinate domains.
	hdrBase := counts.Text + counts.Ftn
	headerAnchors := loadLegacyHeaderFloatingDrawingAnchors(
		wordStream, tableStream, fibBase, cbRgFcLcb,
		hdrBase, counts.Hdr,
	)
	for shapeID, anchor := range legacyAnchorByShapeID(headerAnchors) {
		out[shapeID] = anchor
	}
	return out
}

func legacyAnchorByShapeID(anchors map[int]legacyFloatingDrawingAnchor) map[uint32]legacyFloatingDrawingAnchor {
	out := make(map[uint32]legacyFloatingDrawingAnchor, len(anchors))
	for _, anchor := range anchors {
		if anchor.Shape != 0 {
			out[anchor.Shape] = anchor
		}
	}
	return out
}

func extractLegacyDrawingTextBoxesOwned(
	wordStream, tableStream, clxData []byte,
	fibBase, cbRgFcLcb int,
	counts legacyStoryCounts,
	chpxEntries []chpxEntry,
	papxEntries []papxEntry,
	anchors map[int]legacyFloatingDrawingAnchor,
	additionalOfficeArtStreams ...[]byte,
) []*wordir.TextBox {
	ranges := loadLegacyTextboxOwnedRanges(wordStream, tableStream, fibBase, cbRgFcLcb)
	if len(ranges) == 0 {
		return extractLegacyDrawingTextBoxes(wordStream, clxData, counts, chpxEntries, papxEntries, anchors, additionalOfficeArtStreams...)
	}

	officeArtStreams := make([][]byte, 0, 2+len(additionalOfficeArtStreams))
	officeArtStreams = append(officeArtStreams, wordStream, tableStream)
	officeArtStreams = append(officeArtStreams, additionalOfficeArtStreams...)
	textFlows := extractLegacyDrawingTextFlowsFromStreams(officeArtStreams...)
	anchorByShape := legacyTextboxAnchorByShapeID(
		wordStream, tableStream, fibBase, cbRgFcLcb, counts, anchors,
	)

	out := make([]*wordir.TextBox, 0, len(ranges))
	for _, owned := range ranges {
		base := legacyTextboxStoryBase(counts, owned.Header)
		start := base + owned.StartCP
		end := base + owned.EndCP
		runs := assembleDocRunsFromClxRange(wordStream, clxData, start, end, chpxEntries, papxEntries)
		if len(runs) == 0 {
			continue
		}
		story := buildLegacyStoryFromRuns(runs)
		if len(story.Blocks) == 0 {
			continue
		}
		tb := &wordir.TextBox{
			LegacyShapeID: owned.ShapeID,
			Blocks:        story.Blocks,
			Floating:      true,
			WrapType:      "square",
			XRelative:     "page",
			YRelative:     "page",
		}
		if anchor, ok := anchorByShape[owned.ShapeID]; ok {
			applyLegacyAnchorToTextBox(tb, anchor, textFlows)
		} else if vert, ok := textFlows[owned.ShapeID]; ok {
			tb.Vert = vert
		}
		out = append(out, tb)
	}
	if len(out) == 0 {
		return extractLegacyDrawingTextBoxes(wordStream, clxData, counts, chpxEntries, papxEntries, anchors, additionalOfficeArtStreams...)
	}
	return out
}

func extractLegacyDrawingTextBoxes(wordStream, clxData []byte, counts legacyStoryCounts, chpxEntries []chpxEntry, papxEntries []papxEntry, anchors map[int]legacyFloatingDrawingAnchor, additionalOfficeArtStreams ...[]byte) []*wordir.TextBox {
	if counts.Txbx <= 0 && counts.HdrTxbx <= 0 {
		return nil
	}

	storyStart := counts.Text + counts.Ftn + counts.Hdr + counts.Mcr + counts.Atn + counts.Edn
	storyLengths := []int{counts.Txbx, counts.HdrTxbx}
	result := make([]*wordir.TextBox, 0, 2)
	for _, storyLen := range storyLengths {
		if storyLen <= 0 {
			continue
		}
		storyEnd := storyStart + storyLen
		runs := assembleDocRunsFromClxRange(wordStream, clxData, storyStart, storyEnd, chpxEntries, papxEntries)
		storyStart = storyEnd
		if len(runs) == 0 {
			continue
		}
		for _, group := range splitLegacyTextBoxStoryRuns(runs) {
			story := buildLegacyStoryFromRuns(group)
			if len(story.Blocks) == 0 {
				continue
			}
			result = append(result, &wordir.TextBox{
				Blocks:    story.Blocks,
				Floating:  true,
				WrapType:  "square",
				XRelative: "page",
				YRelative: "page",
			})
		}
	}
	if len(result) == 0 {
		return nil
	}

	return result
}

func legacySortedFloatingDrawingAnchors(anchors map[int]legacyFloatingDrawingAnchor) []legacyFloatingDrawingAnchor {
	if len(anchors) == 0 {
		return nil
	}
	cps := make([]int, 0, len(anchors))
	for cp := range anchors {
		cps = append(cps, cp)
	}
	sort.Ints(cps)
	out := make([]legacyFloatingDrawingAnchor, 0, len(cps))
	for _, cp := range cps {
		out = append(out, anchors[cp])
	}
	return out
}

func applyLegacyAnchorToTextBox(tb *wordir.TextBox, anchor legacyFloatingDrawingAnchor, textFlows map[uint32]string) {
	if tb == nil {
		return
	}
	placement, ok := legacyPlacementFromAnchor(anchor)
	if !ok {
		return
	}
	tb.LegacyShapeID = anchor.Shape
	tb.Floating = true
	tb.XRelative = placement.XRelative
	tb.YRelative = placement.YRelative
	tb.X = placement.X
	tb.Y = placement.Y
	tb.Width = placement.Width
	tb.Height = placement.Height
	tb.WrapType = placement.WrapType
	tb.WrapSide = placement.WrapSide
	tb.BehindDoc = placement.BehindDoc
	if vert, ok := textFlows[anchor.Shape]; ok {
		tb.Vert = vert
		tb.HasVertSetting = true
	}
}

func splitLegacyTextBoxStoryRuns(runs []docRun) [][]docRun {
	if len(runs) == 0 {
		return nil
	}
	groups := [][]docRun{{}}
	for _, run := range runs {
		if !strings.ContainsRune(run.Text, '\f') {
			groups[len(groups)-1] = append(groups[len(groups)-1], run)
			continue
		}
		parts := strings.Split(run.Text, "\f")
		cp := run.StartCP
		for i, part := range parts {
			if part != "" {
				cloned := run
				cloned.Text = part
				cloned.StartCP = cp
				groups[len(groups)-1] = append(groups[len(groups)-1], cloned)
				cp += len([]rune(part))
			}
			if i+1 < len(parts) {
				cp++
				groups = append(groups, nil)
			}
		}
	}
	out := groups[:0]
	for _, group := range groups {
		if len(group) > 0 {
			out = append(out, group)
		}
	}
	return out
}

func mergeLegacyTextBoxProperties(incoming, placeholder *wordir.TextBox) {
	if incoming == nil || placeholder == nil {
		return
	}

	if placeholder.HasVertSetting {
		incoming.Vert = placeholder.Vert
		incoming.HasVertSetting = true
	} else {
		incoming.Vert = firstNonEmpty(incoming.Vert, placeholder.Vert)
	}

	if placeholder.HasWrapSetting {
		incoming.NoWrap = placeholder.NoWrap
		incoming.HasWrapSetting = true
	} else {
		incoming.NoWrap = incoming.NoWrap || placeholder.NoWrap
	}

	if placeholder.HasAnchorSetting {
		incoming.Anchor = placeholder.Anchor
		incoming.AnchorCenter = placeholder.AnchorCenter
		incoming.HasAnchorSetting = true
	} else {
		incoming.Anchor = firstNonEmpty(incoming.Anchor, placeholder.Anchor)
		incoming.AnchorCenter = incoming.AnchorCenter || placeholder.AnchorCenter
	}

	if placeholder.HasPaddingLeft {
		incoming.PaddingLeft = placeholder.PaddingLeft
		incoming.HasPaddingLeft = true
	} else if incoming.PaddingLeft == 0 {
		incoming.PaddingLeft = placeholder.PaddingLeft
	}
	if placeholder.HasPaddingTop {
		incoming.PaddingTop = placeholder.PaddingTop
		incoming.HasPaddingTop = true
	} else if incoming.PaddingTop == 0 {
		incoming.PaddingTop = placeholder.PaddingTop
	}
	if placeholder.HasPaddingRight {
		incoming.PaddingRight = placeholder.PaddingRight
		incoming.HasPaddingRight = true
	} else if incoming.PaddingRight == 0 {
		incoming.PaddingRight = placeholder.PaddingRight
	}
	if placeholder.HasPaddingBottom {
		incoming.PaddingBottom = placeholder.PaddingBottom
		incoming.HasPaddingBottom = true
	} else if incoming.PaddingBottom == 0 {
		incoming.PaddingBottom = placeholder.PaddingBottom
	}

	incoming.Autofit = firstNonEmpty(incoming.Autofit, placeholder.Autofit)
	if incoming.FontScale == 0 {
		incoming.FontScale = placeholder.FontScale
	}
	if incoming.LnSpcReduction == 0 {
		incoming.LnSpcReduction = placeholder.LnSpcReduction
	}
}

func attachLegacyShapeTextBoxes(shape *wordir.Shape, remaining []*wordir.TextBox) []*wordir.TextBox {
	if shape == nil || len(remaining) == 0 {
		return remaining
	}

	// Authoritative DOC ownership is FTXBXS.lid == FSP.spid. Once both sides
	// carry a non-zero ID, attach the story directly to that shape even if the
	// shape parser did not create a textbox placeholder. This makes ownership
	// independent from optional FOPT textbox properties such as txflTextFlow.
	match := -1
	if shape.LegacyShapeID != 0 {
		for i, candidate := range remaining {
			if candidate != nil && candidate.LegacyShapeID == shape.LegacyShapeID {
				match = i
				break
			}
		}
	}
	if match >= 0 {
		incoming := remaining[match]
		if shape.TextBox != nil {
			mergeLegacyTextBoxProperties(incoming, shape.TextBox)
		}
		shape.TextBox = incoming
		remaining = append(remaining[:match], remaining[match+1:]...)
	} else if shape.TextBox != nil && len(shape.TextBox.Blocks) == 0 {
		// Compatibility fallback exists only for old/malformed documents where
		// PlcftxbxTxt did not provide an authoritative shape ID.
		for i, candidate := range remaining {
			if candidate != nil && candidate.LegacyShapeID == 0 {
				match = i
				break
			}
		}
		if match >= 0 {
			incoming := remaining[match]
			mergeLegacyTextBoxProperties(incoming, shape.TextBox)
			shape.TextBox = incoming
			remaining = append(remaining[:match], remaining[match+1:]...)
		}
	}

	for i := range shape.Children {
		if len(remaining) == 0 {
			break
		}
		remaining = attachLegacyShapeTextBoxes(&shape.Children[i], remaining)
	}
	return remaining
}

func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func appendLegacyDetachedDrawingTextBoxes(doc *wordir.Document, textBoxes []*wordir.TextBox) {
	if doc == nil || len(textBoxes) == 0 {
		return
	}

	remaining := textBoxes
	for pi := range doc.Paragraphs {
		for ri := range doc.Paragraphs[pi].Runs {
			run := &doc.Paragraphs[pi].Runs[ri]
			if len(remaining) == 0 {
				return
			}
			if run.Kind == "shape" && run.Shape != nil {
				remaining = attachLegacyShapeTextBoxes(run.Shape, remaining)
			}
		}
	}

	// A textbox with a non-zero LegacyShapeID has authoritative OfficeArt
	// ownership. Rendering it as an independent paragraph is structurally
	// wrong: its position, size and rotation belong to the owning FSP shape.
	//
	// Keep the old detached compatibility path only for stories for which the
	// document provided no usable FTXBXS.lid/FSP.spid relationship.
	for _, textBox := range remaining {
		if textBox == nil || len(textBox.Blocks) == 0 || textBox.LegacyShapeID != 0 {
			continue
		}
		paragraph := wordir.Paragraph{
			Index: len(doc.Paragraphs),
			Runs: []wordir.Run{{
				Kind:    "textbox",
				TextBox: textBox,
			}},
		}
		doc.Paragraphs = append(doc.Paragraphs, paragraph)
		doc.Blocks = append(doc.Blocks, wordir.Block{
			Kind:      "paragraph",
			Paragraph: &doc.Paragraphs[len(doc.Paragraphs)-1],
		})
	}
}

// extractLegacyDrawingTextBoxesWithRanges is a compatibility entry point for
// callers that also pass FIB/range metadata. Range integers are intentionally
// ignored here; byte slices are forwarded as additional OfficeArt streams.
func extractLegacyDrawingTextBoxesWithRanges(wordStream, clxData []byte, counts legacyStoryCounts, chpxEntries []chpxEntry, papxEntries []papxEntry, anchors map[int]legacyFloatingDrawingAnchor, args ...any) []*wordir.TextBox {
	streams := make([][]byte, 0, len(args))
	for _, arg := range args {
		if b, ok := arg.([]byte); ok && len(b) != 0 {
			streams = append(streams, b)
		}
	}
	return extractLegacyDrawingTextBoxes(wordStream, clxData, counts, chpxEntries, papxEntries, anchors, streams...)
}
