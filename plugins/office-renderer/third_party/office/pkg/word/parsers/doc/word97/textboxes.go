package word97

import (
	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

// ResolvedTextbox couples canonical textbox ownership metadata with the
// materialized semantic textbox. Shape projection/attachment remains outside
// this package so OfficeArt rendering stays format-neutral.
type ResolvedTextbox struct {
	Range   docbinary.TextboxRange
	TextBox *wordir.TextBox
}

// MaterializeTextboxes converts authoritative absolute textbox ownership ranges
// into semantic textboxes using the caller's bounded story materializer.
func MaterializeTextboxes(
	ranges []docbinary.TextboxRange,
	anchorByShape map[uint32]docbinary.FloatingAnchor,
	materialize func(startCP, endCP int) wordir.Story,
) []ResolvedTextbox {
	if len(ranges) == 0 || materialize == nil {
		return nil
	}
	out := make([]ResolvedTextbox, 0, len(ranges))
	for _, owned := range ranges {
		story := materialize(owned.StartCP, owned.EndCP)
		if len(story.Blocks) == 0 && len(story.Paragraphs) == 0 {
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
			docbinary.ApplyAnchorToTextBox(tb, anchor)
		}
		out = append(out, ResolvedTextbox{Range: owned, TextBox: tb})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
