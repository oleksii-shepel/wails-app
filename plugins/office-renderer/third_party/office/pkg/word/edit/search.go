package edit

import (
	"regexp"
	"strings"
)

// FindText returns all body paragraphs that contain the given substring.
// It searches the main document body only; header/footer/footnote stories
// are not included.
func (d *Document) FindText(s string) []*Paragraph {
	if s == "" {
		return nil
	}
	var out []*Paragraph
	for _, p := range d.Paragraphs() {
		if strings.Contains(p.Text(), s) {
			out = append(out, p)
		}
	}
	return out
}

// FindTextRegex returns all body paragraphs whose text matches the given regex.
// It searches the main document body only; header/footer/footnote stories
// are not included.
func (d *Document) FindTextRegex(pattern string) ([]*Paragraph, error) {
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	var out []*Paragraph
	for _, p := range d.Paragraphs() {
		if re.MatchString(p.Text()) {
			out = append(out, p)
		}
	}
	return out, nil
}

// RunSearchOptions defines a small formatting-aware body-run search filter.
type RunSearchOptions struct {
	TextContains     string
	TextPattern      string
	Kind             string
	ParagraphStyleID string
	Bold             *bool
	Italic           *bool
	Underline        string
	Color            string
	Highlight        string
}

// FindRuns returns body runs matching the given text/format filters.
func (d *Document) FindRuns(opts RunSearchOptions) ([]*Run, error) {
	var re *regexp.Regexp
	var err error
	if opts.TextPattern != "" {
		re, err = regexp.Compile(opts.TextPattern)
		if err != nil {
			return nil, err
		}
	}

	var out []*Run
	for _, p := range d.Paragraphs() {
		if opts.ParagraphStyleID != "" && p.ir.StyleID != opts.ParagraphStyleID {
			continue
		}
		runs := p.Runs()
		for _, run := range runs {
			if !runMatches(run, re, opts) {
				continue
			}
			out = append(out, run)
		}
	}
	return out, nil
}

func runMatches(run *Run, re *regexp.Regexp, opts RunSearchOptions) bool {
	if run == nil {
		return false
	}
	ir := run.ir()
	if ir == nil {
		return false
	}
	if opts.Kind != "" && ir.Kind != opts.Kind {
		return false
	}
	if opts.TextContains != "" && !strings.Contains(ir.Text, opts.TextContains) {
		return false
	}
	if re != nil && !re.MatchString(ir.Text) {
		return false
	}
	if opts.Bold != nil {
		if ir.Format.Bold == nil || *ir.Format.Bold != *opts.Bold {
			return false
		}
	}
	if opts.Italic != nil {
		if ir.Format.Italic == nil || *ir.Format.Italic != *opts.Italic {
			return false
		}
	}
	if opts.Underline != "" && ir.Format.Underline != opts.Underline {
		return false
	}
	if opts.Color != "" && ir.Format.Color != opts.Color {
		return false
	}
	if opts.Highlight != "" && ir.Format.Highlight != opts.Highlight {
		return false
	}
	return true
}

// ReplaceAll replaces every occurrence of old with new across all runs in
// the document body and returns the number of replacements made.
// It does not cross run boundaries — each replacement is confined to a
// single run's text.
func (d *Document) ReplaceAll(old, new string) int {
	if old == "" || old == new {
		return 0
	}
	count := 0
	for _, p := range d.Paragraphs() {
		count += p.ReplaceAll(old, new)
	}
	return count
}

// ReplaceAll replaces every occurrence of old with new within the paragraph.
// It works across run boundaries and returns the number of replacements made.
// Replacements are performed from the end of the paragraph backwards so that
// earlier ranges remain valid.
func (p *Paragraph) ReplaceAll(old, new string) int {
	if p.ir == nil || old == "" || old == new {
		return 0
	}
	ranges := p.FindTextRangeAll(old)
	if len(ranges) == 0 {
		return 0
	}
	count := 0
	// Replace from last range to first to avoid invalidating earlier indices.
	for i := len(ranges) - 1; i >= 0; i-- {
		ranges[i].ReplaceText(new)
		count++
	}
	return count
}