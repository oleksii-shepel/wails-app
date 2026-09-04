package edit

import (
	"strings"

	wordir "office/pkg/word/ir"
)

// TextRange represents a contiguous span of text within a single paragraph.
type TextRange struct {
	paragraph *Paragraph
	startRun  int // inclusive run index
	startPos  int // byte offset within start run's text
	endRun    int // inclusive run index
	endPos    int // byte offset within end run's text (exclusive)
}

// Text returns the text content covered by the range.
func (tr *TextRange) Text() string {
	if tr == nil || tr.paragraph == nil || tr.paragraph.ir == nil {
		return ""
	}
	runs := tr.paragraph.ir.Runs
	if tr.startRun < 0 || tr.startRun >= len(runs) {
		return ""
	}
	var b strings.Builder
	for i := tr.startRun; i <= tr.endRun && i < len(runs); i++ {
		text := runs[i].Text
		if i == tr.startRun && i == tr.endRun {
			b.WriteString(sliceText(text, tr.startPos, tr.endPos))
		} else if i == tr.startRun {
			b.WriteString(sliceText(text, tr.startPos, len(text)))
		} else if i == tr.endRun {
			b.WriteString(sliceText(text, 0, tr.endPos))
		} else {
			b.WriteString(text)
		}
	}
	return b.String()
}

// Delete removes all text in the range and deletes any runs that become empty.
func (tr *TextRange) Delete() {
	if tr == nil || tr.paragraph == nil || tr.paragraph.ir == nil {
		return
	}
	runs := &tr.paragraph.ir.Runs
	if tr.startRun < 0 || tr.startRun >= len(*runs) {
		return
	}

	// Clamp endRun
	endRun := tr.endRun
	if endRun >= len(*runs) {
		endRun = len(*runs) - 1
	}

	// Collect surviving runs
	var newRuns []wordir.Run
	for i := 0; i < tr.startRun; i++ {
		newRuns = append(newRuns, (*runs)[i])
	}

	for i := tr.startRun; i <= endRun; i++ {
		text := (*runs)[i].Text
		var before, after string
		if i == tr.startRun && i == endRun {
			before = sliceText(text, 0, tr.startPos)
			after = sliceText(text, tr.endPos, len(text))
		} else if i == tr.startRun {
			before = sliceText(text, 0, tr.startPos)
		} else if i == endRun {
			after = sliceText(text, tr.endPos, len(text))
		}
		if before != "" {
			r := cloneRun((*runs)[i])
			r.Text = before
			newRuns = append(newRuns, r)
		}
		if after != "" {
			r := cloneRun((*runs)[i])
			r.Text = after
			newRuns = append(newRuns, r)
		}
	}

	for i := endRun + 1; i < len(*runs); i++ {
		newRuns = append(newRuns, (*runs)[i])
	}

	*runs = newRuns
}

// ReplaceText replaces the content of the range with newText.
// If the range spans a single run, the run text is spliced.
// If it spans multiple runs, all fully-covered runs are removed and
// the boundary runs are trimmed, with a single new run inserted
// carrying the formatting of the start run.
func (tr *TextRange) ReplaceText(newText string) {
	if tr == nil || tr.paragraph == nil || tr.paragraph.ir == nil {
		return
	}
	runs := &tr.paragraph.ir.Runs
	if tr.startRun < 0 || tr.startRun >= len(*runs) {
		return
	}
	endRun := tr.endRun
	if endRun >= len(*runs) {
		endRun = len(*runs) - 1
	}

	var newRuns []wordir.Run
	for i := 0; i < tr.startRun; i++ {
		newRuns = append(newRuns, (*runs)[i])
	}

	// Start-run prefix (text before the range)
	if tr.startRun <= endRun {
		startText := (*runs)[tr.startRun].Text
		prefix := sliceText(startText, 0, tr.startPos)
		if prefix != "" {
			r := cloneRun((*runs)[tr.startRun])
			r.Text = prefix
			newRuns = append(newRuns, r)
		}
	}

	// Replacement text as a new run with the start run's formatting
	if newText != "" {
		if tr.startRun <= endRun && tr.startRun < len(*runs) {
			r := cloneRun((*runs)[tr.startRun])
			r.Text = newText
			newRuns = append(newRuns, r)
		} else {
			newRuns = append(newRuns, wordir.Run{Kind: "run", Text: newText})
		}
	}

	// End-run suffix (text after the range)
	if endRun < len(*runs) {
		endText := (*runs)[endRun].Text
		suffix := sliceText(endText, tr.endPos, len(endText))
		if suffix != "" {
			r := cloneRun((*runs)[endRun])
			r.Text = suffix
			newRuns = append(newRuns, r)
		}
	}

	for i := endRun + 1; i < len(*runs); i++ {
		newRuns = append(newRuns, (*runs)[i])
	}

	*runs = newRuns
}

// InsertAfter inserts newText immediately after the range end. The inserted
// text inherits the formatting of the end run. If the range is nil or invalid,
// this is a no-op.
func (tr *TextRange) InsertAfter(newText string) {
	if tr == nil || tr.paragraph == nil || tr.paragraph.ir == nil || newText == "" {
		return
	}
	runs := &tr.paragraph.ir.Runs
	if tr.endRun < 0 || tr.endRun >= len(*runs) {
		return
	}

	endRun := tr.endRun
	endText := (*runs)[endRun].Text
	if tr.endPos >= len(endText) {
		// Insert at end of end run: append a new run after it.
		r := cloneRun((*runs)[endRun])
		r.Text = newText
		*runs = append((*runs)[:endRun+1], append([]wordir.Run{r}, (*runs)[endRun+1:]...)...)
		return
	}

	// Split endRun into prefix [0:endPos] and suffix [endPos:], then insert
	// newText as a new run between them.
	prefix := sliceText(endText, 0, tr.endPos)
	suffix := sliceText(endText, tr.endPos, len(endText))
	newRuns := make([]wordir.Run, 0, len(*runs)+2)
	newRuns = append(newRuns, (*runs)[:endRun]...)
	if prefix != "" {
		r := cloneRun((*runs)[endRun])
		r.Text = prefix
		newRuns = append(newRuns, r)
	}
	inserted := cloneRun((*runs)[endRun])
	inserted.Text = newText
	newRuns = append(newRuns, inserted)
	if suffix != "" {
		r := cloneRun((*runs)[endRun])
		r.Text = suffix
		newRuns = append(newRuns, r)
	}
	newRuns = append(newRuns, (*runs)[endRun+1:]...)
	*runs = newRuns
}

// InsertBefore inserts newText immediately before the range start. The inserted
// text inherits the formatting of the start run. If the range is nil or invalid,
// this is a no-op.
func (tr *TextRange) InsertBefore(newText string) {
	if tr == nil || tr.paragraph == nil || tr.paragraph.ir == nil || newText == "" {
		return
	}
	runs := &tr.paragraph.ir.Runs
	if tr.startRun < 0 || tr.startRun >= len(*runs) {
		return
	}

	startText := (*runs)[tr.startRun].Text
	if tr.startPos <= 0 {
		// Insert at beginning of start run: prepend a new run before it.
		r := cloneRun((*runs)[tr.startRun])
		r.Text = newText
		*runs = append((*runs)[:tr.startRun], append([]wordir.Run{r}, (*runs)[tr.startRun:]...)...)
		return
	}

	// Split startRun into prefix [0:startPos] and suffix [startPos:], then insert
	// newText as a new run between them.
	prefix := sliceText(startText, 0, tr.startPos)
	suffix := sliceText(startText, tr.startPos, len(startText))
	newRuns := make([]wordir.Run, 0, len(*runs)+2)
	newRuns = append(newRuns, (*runs)[:tr.startRun]...)
	if prefix != "" {
		r := cloneRun((*runs)[tr.startRun])
		r.Text = prefix
		newRuns = append(newRuns, r)
	}
	inserted := cloneRun((*runs)[tr.startRun])
	inserted.Text = newText
	newRuns = append(newRuns, inserted)
	if suffix != "" {
		r := cloneRun((*runs)[tr.startRun])
		r.Text = suffix
		newRuns = append(newRuns, r)
	}
	newRuns = append(newRuns, (*runs)[tr.startRun+1:]...)
	*runs = newRuns
}

// FindTextRange locates the first occurrence of s in the paragraph and returns
// a TextRange describing it. Returns nil if s is empty or not found.
func (p *Paragraph) FindTextRange(s string) *TextRange {
	if p.ir == nil || s == "" {
		return nil
	}
	fullText := p.Text()
	offset := strings.Index(fullText, s)
	if offset < 0 {
		return nil
	}
	startRun, startPos := runIndexForCharPos(p.ir.Runs, offset)
	endRun, endPos := runIndexForCharPos(p.ir.Runs, offset+len(s))
	if startRun < 0 || endRun < 0 {
		return nil
	}
	return &TextRange{
		paragraph: p,
		startRun:  startRun,
		startPos:  startPos,
		endRun:    endRun,
		endPos:    endPos,
	}
}

// FindTextRangeAll returns all non-overlapping occurrences of s in the paragraph.
func (p *Paragraph) FindTextRangeAll(s string) []*TextRange {
	if p.ir == nil || s == "" {
		return nil
	}
	var out []*TextRange
	fullText := p.Text()
	searchFrom := 0
	for {
		offset := strings.Index(fullText[searchFrom:], s)
		if offset < 0 {
			break
		}
		absOffset := searchFrom + offset
		startRun, startPos := runIndexForCharPos(p.ir.Runs, absOffset)
		endRun, endPos := runIndexForCharPos(p.ir.Runs, absOffset+len(s))
		if startRun < 0 || endRun < 0 {
			break
		}
		out = append(out, &TextRange{
			paragraph: p,
			startRun:  startRun,
			startPos:  startPos,
			endRun:    endRun,
			endPos:    endPos,
		})
		searchFrom = absOffset + len(s)
	}
	return out
}

// runIndexForCharPos maps a character offset in the concatenated paragraph text
// to the run index and byte position within that run. Returns (-1, -1) if offset
// is out of range.
func runIndexForCharPos(runs []wordir.Run, offset int) (runIdx, pos int) {
	if offset < 0 {
		return -1, -1
	}
	current := 0
	for i, r := range runs {
		textLen := len(r.Text)
		if offset >= current && offset < current+textLen {
			return i, offset - current
		}
		current += textLen
	}
	if offset == current {
		// Position at the very end — map to the last run or an empty append point
		if len(runs) > 0 {
			return len(runs) - 1, len(runs[len(runs)-1].Text)
		}
		return -1, -1
	}
	return -1, -1
}

// sliceText returns text[start:end] safely clamping to the string bounds.
func sliceText(text string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	if start >= end {
		return ""
	}
	return text[start:end]
}