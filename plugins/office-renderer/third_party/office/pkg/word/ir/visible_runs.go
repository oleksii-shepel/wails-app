package ir

import (
	"strings"
	"unicode/utf8"
)

// CanonicalVisibleRuns returns the paragraph run stream shared by layout and
// rendering. Paragraph.Text owns authored whitespace; Runs own formatting and
// inline semantics. Legacy binary documents can leave whitespace attached to
// hidden CHPX fragments, so reconcile visible text without moving formatting
// ownership of non-whitespace characters.
func CanonicalVisibleRuns(p *Paragraph) []Run {
	if p == nil || len(p.Runs) == 0 {
		return nil
	}
	out := append([]Run(nil), p.Runs...)
	if p.Text == "" {
		return out
	}
	canonicalParts := strings.Split(p.Text, "\t")
	part, start := 0, 0
	for i := 0; i <= len(out); i++ {
		atTab := i < len(out) && out[i].Kind == "tab" && !runHidden(out[i])
		if i != len(out) && !atTab {
			continue
		}
		if part < len(canonicalParts) {
			reconcileVisibleTextInterval(out[start:i], canonicalParts[part])
		}
		if atTab {
			out[i].Text = ""
			part++
			start = i + 1
		}
	}
	return out
}

func reconcileVisibleTextInterval(runs []Run, canonical string) {
	var visible strings.Builder
	var owners []int
	var ownerCounts []int
	for i := range runs {
		if runHidden(runs[i]) || !runOwnsVisibleText(runs[i]) {
			continue
		}
		visible.WriteString(runs[i].Text)
		if count := nonWhitespaceCount(runs[i].Text); count > 0 {
			owners = append(owners, i)
			ownerCounts = append(ownerCounts, count)
		}
	}
	visibleText := visible.String()
	if len(owners) == 0 {
		return
	}
	if !sameVisibleText(visibleText, canonical) {
		// Paragraph.Text is the canonical authored text. Legacy CHPX projection can
		// occasionally lose a short edge fragment when a formatting boundary and a
		// paragraph/story boundary share the same CP. Repair only exact raw-text
		// prefix/suffix loss; never synthesize text for an arbitrary mismatch.
		if strings.HasSuffix(canonical, visibleText) {
			runs[owners[0]].Text = strings.TrimSuffix(canonical, visibleText) + runs[owners[0]].Text
		} else if strings.HasPrefix(canonical, visibleText) {
			runs[owners[len(owners)-1]].Text += strings.TrimPrefix(canonical, visibleText)
		}
		return
	}
	// An owner that authored trailing whitespace keeps it: that whitespace is
	// real advance in Word even when an inline object follows, and moving it
	// past the object shifts the object's own in-flow position.
	ownerTrailingWS := make([]int, len(owners))
	for k, idx := range owners {
		ownerTrailingWS[k] = trailingWhitespaceLen(runs[idx].Text)
	}
	for i := range runs {
		if !runHidden(runs[i]) && runOwnsVisibleText(runs[i]) {
			runs[i].Text = ""
		}
	}
	ownerPos, remaining := 0, ownerCounts[0]
	atOwnerStart := false
	// Accumulate into one builder per owner. Appending per character straight
	// onto Run.Text reallocates the whole string each time, which makes the
	// rebuild quadratic in the paragraph's length.
	builders := make([]strings.Builder, len(owners))
	var pending []byte
	for _, ch := range canonical {
		if isRunWhitespace(ch) {
			pending = utf8.AppendRune(pending, ch)
			continue
		}
		if remaining == 0 && ownerPos+1 < len(owners) {
			ownerPos++
			remaining = ownerCounts[ownerPos]
			atOwnerStart = true
		}
		ws := pending
		pending = pending[:0]
		if atOwnerStart {
			atOwnerStart = false
			keep := ownerTrailingWS[ownerPos-1]
			if keep > len(ws) {
				keep = len(ws)
			}
			if keep > 0 {
				builders[ownerPos-1].Write(ws[:keep])
				ws = ws[keep:]
			}
		}
		builders[ownerPos].Write(ws)
		builders[ownerPos].WriteRune(ch)
		remaining--
	}
	if len(pending) > 0 {
		builders[len(owners)-1].Write(pending)
	}
	for k, idx := range owners {
		runs[idx].Text = builders[k].String()
	}
}

func trailingWhitespaceLen(s string) int {
	n := 0
	for i := len(s) - 1; i >= 0; i-- {
		r := rune(s[i])
		if !isRunWhitespace(r) {
			break
		}
		n++
	}
	return n
}

// Hidden reports whether the run's content is not displayed. Word hides
// w:vanish runs unless "show hidden text" is on, and hides w:specVanish runs
// (list paragraph marks, style separators) unconditionally. Hidden content
// takes no layout space, so both the measuring and painting paths must skip it.
func (f RunFormat) Hidden() bool {
	return (f.Vanish != nil && *f.Vanish) || (f.SpecVanish != nil && *f.SpecVanish)
}

func runHidden(run Run) bool {
	return run.Format.Hidden()
}
func runOwnsVisibleText(run Run) bool {
	switch run.Kind {
	case "", "run", "hyperlink":
		return true
	default:
		return false
	}
}

// sameVisibleText reports whether two strings carry the same non-whitespace
// text. It compares in place: stripping both sides with strings.Map allocates a
// copy of the whole paragraph on every call.
func sameVisibleText(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if isRunWhitespace(rune(a[i])) {
			i++
			continue
		}
		if isRunWhitespace(rune(b[j])) {
			j++
			continue
		}
		if a[i] != b[j] {
			return false
		}
		i++
		j++
	}
	for ; i < len(a); i++ {
		if !isRunWhitespace(rune(a[i])) {
			return false
		}
	}
	for ; j < len(b); j++ {
		if !isRunWhitespace(rune(b[j])) {
			return false
		}
	}
	return true
}

func isRunWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n', '\f', '\v':
		return true
	default:
		return false
	}
}
func nonWhitespaceCount(s string) int {
	n := 0
	for _, r := range s {
		if !isRunWhitespace(r) {
			n++
		}
	}
	return n
}
