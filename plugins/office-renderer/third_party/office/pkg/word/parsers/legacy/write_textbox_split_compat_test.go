package legacy

import wordir "office/pkg/word/ir"

// splitLegacyTextBoxStoryRuns is kept only for the historical writer test.
// Production textbox splitting is canonical; this shim adapts the old test
// shape onto splitCanonicalStoryOnBreak instead of restoring legacy parsing.
func splitLegacyTextBoxStoryRuns(runs []docRun) [][]docRun {
	if len(runs) == 0 {
		return nil
	}

	story := wordir.Story{}
	for _, legacyRun := range runs {
		run := wordir.Run{Text: legacyRun.Text}
		paragraph := wordir.Paragraph{
			Runs: []wordir.Run{run},
			Text: legacyRun.Text,
		}
		story.Paragraphs = append(story.Paragraphs, paragraph)
		story.Blocks = append(story.Blocks, wordir.Block{
			Index:     len(story.Blocks),
			Kind:      "paragraph",
			Paragraph: &story.Paragraphs[len(story.Paragraphs)-1],
		})
		story.Text += legacyRun.Text
	}

	// The canonical materializer represents form-feed as explicit page-break
	// runs. Adapt raw writer text into that representation before splitting.
	for bi := range story.Blocks {
		paragraph := story.Blocks[bi].Paragraph
		if paragraph == nil {
			continue
		}
		var canonicalRuns []wordir.Run
		var text string
		for _, run := range paragraph.Runs {
			start := 0
			for i, r := range run.Text {
				if r != '\f' {
					continue
				}
				if i > start {
					segment := run
					segment.Text = run.Text[start:i]
					canonicalRuns = append(canonicalRuns, segment)
					text += segment.Text
				}
				canonicalRuns = append(canonicalRuns, wordir.Run{Kind: "break", BreakType: "page"})
				start = i + 1
			}
			if start < len(run.Text) {
				segment := run
				segment.Text = run.Text[start:]
				canonicalRuns = append(canonicalRuns, segment)
				text += segment.Text
			}
		}
		paragraph.Runs = canonicalRuns
		paragraph.Text = text
	}

	stories := splitCanonicalStoryOnBreak(story, "page")
	out := make([][]docRun, 0, len(stories))
	for _, split := range stories {
		if split.Text == "" {
			continue
		}
		out = append(out, []docRun{{Text: split.Text, StartCP: runs[0].StartCP}})
	}
	return out
}
