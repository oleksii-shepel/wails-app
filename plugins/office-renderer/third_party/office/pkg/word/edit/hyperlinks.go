package edit

import wordir "office/pkg/word/ir"

// Hyperlink represents a hyperlink found in a run.
type Hyperlink struct {
	Target   string // URL or empty for internal anchor
	Anchor   string // bookmark anchor
	RelID    string // relationship ID (empty when not available)
	Text     string // visible hyperlink text
	Location string // "body", "header", "footer", "footnote", "endnote", "comment"
}

// Hyperlinks returns all hyperlinks found anywhere in the document body and
// all stories (headers, footers, footnotes, endnotes, comments).
func (d *Document) Hyperlinks() []Hyperlink {
	if d == nil || d.IR == nil {
		return nil
	}
	var out []Hyperlink

	add := func(runs []wordir.Run, loc string) {
		for _, r := range runs {
			if r.Kind != "hyperlink" && r.Target == "" && r.Anchor == "" {
				continue
			}
			if r.Kind == "hyperlink" || r.Target != "" || r.Anchor != "" {
				out = append(out, Hyperlink{
					Target:   r.Target,
					Anchor:   r.Anchor,
					RelID:    r.RelID,
					Text:     r.Text,
					Location: loc,
				})
			}
		}
	}

	// Body paragraphs
	for _, b := range d.IR.Blocks {
		if b.Paragraph == nil {
			continue
		}
		add(b.Paragraph.Runs, "body")
	}

	// Headers
	for _, s := range d.headerPtrs {
		for _, p := range s.Paragraphs {
			add(p.Runs, "header")
		}
	}

	// Footers
	for _, s := range d.footerPtrs {
		for _, p := range s.Paragraphs {
			add(p.Runs, "footer")
		}
	}

	// Footnotes
	for _, s := range d.footnotePtrs {
		for _, p := range s.Paragraphs {
			add(p.Runs, "footnote")
		}
	}

	// Endnotes
	for _, s := range d.endnotePtrs {
		for _, p := range s.Paragraphs {
			add(p.Runs, "endnote")
		}
	}

	// Comments
	for _, s := range d.commentPtrs {
		for _, p := range s.Paragraphs {
			add(p.Runs, "comment")
		}
	}

	return out
}

// HyperlinkCount returns the total number of hyperlinks in the document.
func (d *Document) HyperlinkCount() int {
	return len(d.Hyperlinks())
}