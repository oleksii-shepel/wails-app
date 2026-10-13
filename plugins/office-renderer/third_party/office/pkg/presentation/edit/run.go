package edit

import presir "office/pkg/presentation/ir"

// Run wraps an IR text run for editing.
type Run struct {
	paragraph *Paragraph
	ir        *presir.Run
}

// Text returns the run text.
func (r *Run) Text() string {
	if r == nil || r.ir == nil {
		return ""
	}
	return r.ir.Text
}

// SetText sets the run text.
func (r *Run) SetText(text string) {
	if r == nil || r.ir == nil {
		return
	}
	r.ir.Text = text
}

// SetBold toggles bold.
func (r *Run) SetBold(v bool) {
	if r == nil || r.ir == nil {
		return
	}
	r.ir.Bold = v
	r.ir.BoldSet = true
}

// Bold returns the bold flag.
func (r *Run) Bold() bool {
	if r == nil || r.ir == nil {
		return false
	}
	return r.ir.Bold
}

// SetItalic toggles italic.
func (r *Run) SetItalic(v bool) {
	if r == nil || r.ir == nil {
		return
	}
	r.ir.Italic = v
	r.ir.ItalicSet = true
}

// Italic returns the italic flag.
func (r *Run) Italic() bool {
	if r == nil || r.ir == nil {
		return false
	}
	return r.ir.Italic
}

// SetUnderline toggles underline.
func (r *Run) SetUnderline(v bool) {
	if r == nil || r.ir == nil {
		return
	}
	r.ir.Underline = v
	r.ir.UnderlineSet = true
}

// Underline returns the underline flag.
func (r *Run) Underline() bool {
	if r == nil || r.ir == nil {
		return false
	}
	return r.ir.Underline
}

// SetStrike toggles strikethrough.
func (r *Run) SetStrike(v bool) {
	if r == nil || r.ir == nil {
		return
	}
	r.ir.Strike = v
	r.ir.StrikeSet = true
}

// Strike returns the strikethrough flag.
func (r *Run) Strike() bool {
	if r == nil || r.ir == nil {
		return false
	}
	return r.ir.Strike
}

// SetColor sets the font color as a 6-digit hex string.
func (r *Run) SetColor(color string) {
	if r == nil || r.ir == nil {
		return
	}
	r.ir.Color = color
}

// Color returns the font color hex string.
func (r *Run) Color() string {
	if r == nil || r.ir == nil {
		return ""
	}
	return r.ir.Color
}

// SetFontSize sets the font size in hundredths of a point.
func (r *Run) SetFontSize(size int) {
	if r == nil || r.ir == nil {
		return
	}
	r.ir.FontSize = size
}

// FontSize returns the font size in hundredths of a point.
func (r *Run) FontSize() int {
	if r == nil || r.ir == nil {
		return 0
	}
	return r.ir.FontSize
}

// SetFontFamily sets the font family.
func (r *Run) SetFontFamily(family string) {
	if r == nil || r.ir == nil {
		return
	}
	r.ir.FontFamily = family
}

// FontFamily returns the font family.
func (r *Run) FontFamily() string {
	if r == nil || r.ir == nil {
		return ""
	}
	return r.ir.FontFamily
}

// SetHyperlinkURL sets an external URL for this text run. Pass an empty string
// to remove the hyperlink.
func (r *Run) SetHyperlinkURL(url string) {
	if r == nil || r.ir == nil {
		return
	}
	r.ir.HyperlinkURL = url
}

// HyperlinkURL returns the run's external hyperlink URL, if any.
func (r *Run) HyperlinkURL() string {
	if r == nil || r.ir == nil {
		return ""
	}
	return r.ir.HyperlinkURL
}

// RemoveHyperlink clears the hyperlink from the run.
func (r *Run) RemoveHyperlink() {
	if r == nil || r.ir == nil {
		return
	}
	r.ir.HyperlinkURL = ""
}
