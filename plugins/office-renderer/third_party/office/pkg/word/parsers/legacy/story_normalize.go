package legacy

import (
	"unicode"
	"unicode/utf8"

	wordir "office/pkg/word/ir"
)

// stripLeadingStoryControlMarker removes the non-printing reference marker
// that can prefix binary side stories. It operates on canonical word IR so
// streaming and lossless story materialization share the same normalization.
func stripLeadingStoryControlMarker(story *wordir.Story) {
	if story == nil {
		return
	}
	for pi := range story.Paragraphs {
		p := &story.Paragraphs[pi]
		for ri := range p.Runs {
			run := &p.Runs[ri]
			if run.Text == "" {
				continue
			}
			first, width := utf8.DecodeRuneInString(run.Text)
			if first == utf8.RuneError && width == 0 {
				return
			}
			if first == '\n' || first == '\t' || unicode.IsPrint(first) {
				return
			}
			run.Text = run.Text[width:]
			if len(p.Text) >= width {
				p.Text = p.Text[width:]
			}
			if len(story.Text) >= width {
				story.Text = story.Text[width:]
			}
			return
		}
		if p.Text != "" {
			return
		}
	}
}
