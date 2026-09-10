package ir

import "strings"

// CanonicalVisibleRuns returns the paragraph run stream shared by layout and
// rendering. Paragraph.Text owns authored whitespace; Runs own formatting and
// inline semantics. Legacy binary documents can leave whitespace attached to
// hidden CHPX fragments, so reconcile visible text without moving formatting
// ownership of non-whitespace characters.
func CanonicalVisibleRuns(p *Paragraph) []Run {
	if p == nil || len(p.Runs) == 0 { return nil }
	out := append([]Run(nil), p.Runs...)
	if p.Text == "" { return out }
	canonicalParts := strings.Split(p.Text, "\t")
	part, start := 0, 0
	for i := 0; i <= len(out); i++ {
		atTab := i < len(out) && out[i].Kind == "tab" && !runHidden(out[i])
		if i != len(out) && !atTab { continue }
		if part < len(canonicalParts) { reconcileVisibleTextInterval(out[start:i], canonicalParts[part]) }
		if atTab { out[i].Text = ""; part++; start = i + 1 }
	}
	return out
}

func reconcileVisibleTextInterval(runs []Run, canonical string) {
	var visible strings.Builder
	var owners []int
	var ownerCounts []int
	for i := range runs {
		if runHidden(runs[i]) || !runOwnsVisibleText(runs[i]) { continue }
		visible.WriteString(runs[i].Text)
		if count := nonWhitespaceCount(runs[i].Text); count > 0 {
			owners = append(owners, i); ownerCounts = append(ownerCounts, count)
		}
	}
	if stripRunWhitespace(visible.String()) != stripRunWhitespace(canonical) || len(owners) == 0 { return }
	for i := range runs {
		if !runHidden(runs[i]) && runOwnsVisibleText(runs[i]) { runs[i].Text = "" }
	}
	ownerPos, remaining := 0, ownerCounts[0]
	var pending strings.Builder
	for _, ch := range canonical {
		if isRunWhitespace(ch) { pending.WriteRune(ch); continue }
		for remaining == 0 && ownerPos+1 < len(owners) { ownerPos++; remaining = ownerCounts[ownerPos] }
		idx := owners[ownerPos]
		runs[idx].Text += pending.String(); pending.Reset(); runs[idx].Text += string(ch); remaining--
	}
	if pending.Len() > 0 { runs[owners[len(owners)-1]].Text += pending.String() }
}

func runHidden(run Run) bool {
	return (run.Format.Vanish != nil && *run.Format.Vanish) || (run.Format.SpecVanish != nil && *run.Format.SpecVanish)
}
func runOwnsVisibleText(run Run) bool {
	switch run.Kind { case "", "run", "hyperlink": return true; default: return false }
}
func stripRunWhitespace(s string) string {
	return strings.Map(func(r rune) rune { if isRunWhitespace(r) { return -1 }; return r }, s)
}
func isRunWhitespace(r rune) bool {
	switch r { case ' ', '\t', '\r', '\n', '\f', '\v': return true; default: return false }
}
func nonWhitespaceCount(s string) int {
	n := 0; for _, r := range s { if !isRunWhitespace(r) { n++ } }; return n
}
