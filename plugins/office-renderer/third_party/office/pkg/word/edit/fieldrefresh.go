package edit

import (
	"strings"
	"time"

	wordir "office/pkg/word/ir"
)

// RefreshFields performs a best-effort in-place update of field results in the
// document IR. It evaluates fields that do not depend on layout (DATE, TIME,
// CREATEDATE, SAVEDATE, PRINTDATE, DOCPROPERTY, DOCVARIABLE) and leaves
// layout-dependent fields (PAGE, NUMPAGES, REF, PAGEREF, etc.) unchanged.
//
// For DATE/TIME fields, existing cached text is overwritten with a freshly
// computed value. For DOCPROPERTY/DOCVARIABLE, the run text is set to the
// current property/variable value if known.
func (d *Document) RefreshFields() {
	if d == nil || d.IR == nil {
		return
	}

	refreshRun := func(r *wordir.Run) {
		switch r.FieldKind {
		case "DATE":
			r.Text = formatFieldDate(r.FieldInstr, time.Now())
		case "TIME":
			r.Text = formatFieldDate(r.FieldInstr, time.Now())
		case "CREATEDATE":
			r.Text = formatFieldDate(r.FieldInstr, parseIRTime(d.IR.CreatedAt))
		case "SAVEDATE":
			r.Text = formatFieldDate(r.FieldInstr, parseIRTime(d.IR.ModifiedAt))
		case "PRINTDATE":
			r.Text = formatFieldDate(r.FieldInstr, parseIRTime(d.IR.LastPrintedAt))
		case "DOCPROPERTY":
			if v, ok := docPropertyValue(d.IR, r.FieldArg); ok {
				r.Text = v
			}
		case "DOCVARIABLE":
			if v, ok := d.IR.DocumentVariables[r.FieldArg]; ok {
				r.Text = v
			}
		}
	}

	// Body paragraphs
	for _, b := range d.IR.Blocks {
		if b.Paragraph == nil {
			continue
		}
		for i := range b.Paragraph.Runs {
			refreshRun(&b.Paragraph.Runs[i])
		}
	}

	// Stories (use pointer caches to see unflushed edits)
	refreshStory := func(s *wordir.Story) {
		if s == nil {
			return
		}
		for _, b := range s.Blocks {
			if b.Paragraph == nil {
				continue
			}
			for i := range b.Paragraph.Runs {
				refreshRun(&b.Paragraph.Runs[i])
			}
		}
	}
	for _, s := range d.headerPtrs {
		refreshStory(s)
	}
	for _, s := range d.footerPtrs {
		refreshStory(s)
	}
	for _, s := range d.footnotePtrs {
		refreshStory(s)
	}
	for _, s := range d.endnotePtrs {
		refreshStory(s)
	}
	for _, s := range d.commentPtrs {
		refreshStory(s)
	}
}

// docPropertyValue returns the value of a built-in or custom document property.
func docPropertyValue(doc *wordir.Document, name string) (string, bool) {
	name = strings.TrimSpace(name)
	switch strings.ToLower(name) {
	case "title":
		return doc.Title, true
	case "author", "creator":
		return doc.Author, true
	case "subject":
		return doc.Subject, true
	case "description", "comments":
		return doc.Description, true
	case "keywords":
		return doc.Keywords, true
	case "company":
		return doc.Company, true
	case "manager":
		return doc.Manager, true
	case "category":
		return "", true
	}
	if doc.CustomProperties != nil {
		if v, ok := doc.CustomProperties[name]; ok {
			return v, true
		}
	}
	return "", false
}

// parseIRTime parses ISO-8601-ish strings from the IR; falls back to zero time.
func parseIRTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// formatFieldDate converts a field instruction and a time value into a display
// string. It recognises common Word date/time picture switches (e.g. \@ "dd/MM/yyyy")
// and falls back to a sensible default.
func formatFieldDate(instr string, t time.Time) string {
	// Extract \@ "..." picture switch.
	picture := ""
	if i := strings.Index(instr, `\@`); i >= 0 {
		rest := strings.TrimSpace(instr[i+2:])
		if strings.HasPrefix(rest, `"`) {
			rest = rest[1:]
			if j := strings.Index(rest, `"`); j >= 0 {
				picture = rest[:j]
			}
		}
	}

	if picture == "" {
		// Default formats when no picture switch is present.
		if strings.Contains(strings.ToUpper(instr), "TIME") {
			picture = "h:mm:ss AM/PM"
		} else {
			picture = "dd/MM/yyyy"
		}
	}

	// Map Word date picture switches to Go layout.
	layout := wordPictureToGoLayout(picture)
	if layout == "" {
		layout = picture // try raw Go layout
	}

	result := t.Format(layout)
	// Post-process AM/PM to upper-case if the picture uses capital letters.
	if strings.Contains(picture, "AM/PM") || strings.Contains(picture, "A/P") {
		result = strings.ToUpper(result)
	}
	return result
}

// wordPictureToGoLayout converts common Word date picture strings to Go time
// layouts. Returns empty string when no known mapping exists.
func wordPictureToGoLayout(pic string) string {
	// Normalise common Word patterns.
	pic = strings.TrimSpace(pic)
	switch strings.ToLower(pic) {
	case "dd/mm/yyyy":
		return "02/01/2006"
	case "dd/mm/yy":
		return "02/01/06"
	case "mm/dd/yyyy":
		return "01/02/2006"
	case "mm/dd/yy":
		return "01/02/06"
	case "yyyy-mm-dd":
		return "2006-01-02"
	case "dd mmmm yyyy":
		return "02 January 2006"
	case "dd mmm yyyy":
		return "02 Jan 2006"
	case "mmmm dd, yyyy":
		return "January 02, 2006"
	case "ddd dd mmm yyyy":
		return "Mon 02 Jan 2006"
	case "dddd, dd mmmm yyyy":
		return "Monday, 02 January 2006"
	case "hh:mm:ss am/pm":
		return "03:04:05 PM"
	case "h:mm:ss am/pm":
		return "3:04:05 PM"
	case "hh:mm am/pm":
		return "03:04 PM"
	case "h:mm am/pm":
		return "3:04 PM"
	case "hh:mm:ss":
		return "15:04:05"
	case "hh:mm":
		return "15:04"
	case "h:mm:ss":
		return "15:04:05"
	case "h:mm":
		return "15:04"
	}

	// Heuristic substitution of Word tokens to Go layout tokens.
	// Order matters: longest tokens first so "yyyy" beats "yy", etc.
	replacements := []struct{ from, to string }{
		{"yyyy", "2006"},
		{"yy", "06"},
		{"mmmm", "January"},
		{"mmm", "Jan"},
		{"mm", "01"},
		{"dd", "02"},
		{"dddd", "Monday"},
		{"ddd", "Mon"},
		{"hh", "15"},
		{"HH", "15"},
		{"h", "3"},
		{"H", "15"},
		{"MM", "04"},
		{"ss", "05"},
		{"AM/PM", "PM"},
		{"am/pm", "pm"},
		{"A/P", "P"},
		{"a/p", "p"},
	}
	layout := pic
	replaced := make(map[string]bool)
	for _, r := range replacements {
		if strings.Contains(layout, r.from) {
			layout = strings.ReplaceAll(layout, r.from, r.to)
			replaced[r.from] = true
		}
	}
	// If the result still contains unmapped Word tokens, give up.
	for _, r := range replacements {
		if !replaced[r.from] && strings.Contains(layout, r.from) {
			return ""
		}
	}
	return layout
}