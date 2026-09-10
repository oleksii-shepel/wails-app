package word97

import (
	wordir "office/pkg/word/ir"
	docofficeart "office/pkg/word/parsers/doc/officeart"
)

// ProjectDetachedDrawingTextBoxes completes canonical Word97 textbox projection.
//
// Textbox stories, shape ownership, anchors, text flow, and OfficeArt visuals
// have already been decoded before this boundary. This helper only attaches
// decoded textboxes to their owning shapes and preserves the historical
// fallback for genuinely unowned textbox stories.
func ProjectDetachedDrawingTextBoxes(doc *wordir.Document, textBoxes []*wordir.TextBox) {
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
				remaining = docofficeart.AttachTextBoxes(run.Shape, remaining)
			}
		}
	}

	for _, textBox := range remaining {
		if textBox == nil || len(textBox.Blocks) == 0 || textBox.LegacyShapeID != 0 {
			continue
		}
		paragraph := wordir.Paragraph{
			Index: len(doc.Paragraphs),
			Runs:  []wordir.Run{{Kind: "textbox", TextBox: textBox}},
		}
		doc.Paragraphs = append(doc.Paragraphs, paragraph)
		doc.Blocks = append(doc.Blocks, wordir.Block{
			Kind:      "paragraph",
			Paragraph: &doc.Paragraphs[len(doc.Paragraphs)-1],
		})
	}
}
