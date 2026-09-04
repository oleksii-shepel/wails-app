package edit

import wordir "office/pkg/word/ir"

// FormField wraps a Word IR legacy form field and exposes a small editing API.
type FormField struct {
	ir *wordir.FormField
}

// Type returns the field type: "text", "checkbox", or "dropdown".
func (ff *FormField) Type() string {
	if ff == nil || ff.ir == nil {
		return ""
	}
	return ff.ir.Type
}

// Name returns the field name.
func (ff *FormField) Name() string {
	if ff == nil || ff.ir == nil {
		return ""
	}
	return ff.ir.Name
}

// SetName sets the field name.
func (ff *FormField) SetName(v string) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.Name = v
}

// Enabled reports whether the field is enabled.
func (ff *FormField) Enabled() bool {
	if ff == nil || ff.ir == nil {
		return false
	}
	return ff.ir.Enabled
}

// SetEnabled toggles the field enabled state.
func (ff *FormField) SetEnabled(v bool) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.Enabled = v
}

// CalcOnExit reports whether the field recalculates on exit.
func (ff *FormField) CalcOnExit() bool {
	if ff == nil || ff.ir == nil {
		return false
	}
	return ff.ir.CalcOnExit
}

// SetCalcOnExit toggles calculate-on-exit behavior.
func (ff *FormField) SetCalcOnExit(v bool) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.CalcOnExit = v
}

// DefaultText returns the current/default text value for text form fields.
func (ff *FormField) DefaultText() string {
	if ff == nil || ff.ir == nil {
		return ""
	}
	return ff.ir.DefaultText
}

// SetDefaultText sets the current/default text value.
func (ff *FormField) SetDefaultText(v string) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.DefaultText = v
}

// MaxLength returns the text field maximum length.
func (ff *FormField) MaxLength() int {
	if ff == nil || ff.ir == nil {
		return 0
	}
	return ff.ir.MaxLength
}

// SetMaxLength sets the text field maximum length.
func (ff *FormField) SetMaxLength(v int) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.MaxLength = v
}

// TextType returns the text field input type.
func (ff *FormField) TextType() string {
	if ff == nil || ff.ir == nil {
		return ""
	}
	return ff.ir.TextType
}

// SetTextType sets the text field input type.
func (ff *FormField) SetTextType(v string) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.TextType = v
}

// TextFormat returns the text field format string.
func (ff *FormField) TextFormat() string {
	if ff == nil || ff.ir == nil {
		return ""
	}
	return ff.ir.TextFormat
}

// SetTextFormat sets the text field format string.
func (ff *FormField) SetTextFormat(v string) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.TextFormat = v
}

// Checked reports whether a checkbox field is checked.
func (ff *FormField) Checked() bool {
	if ff == nil || ff.ir == nil {
		return false
	}
	return ff.ir.Checked
}

// SetChecked sets the checkbox checked state.
func (ff *FormField) SetChecked(v bool) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.Checked = v
}

// CheckboxSize returns the checkbox size in half-points.
func (ff *FormField) CheckboxSize() int {
	if ff == nil || ff.ir == nil {
		return 0
	}
	return ff.ir.CheckboxSize
}

// SetCheckboxSize sets the checkbox size in half-points.
func (ff *FormField) SetCheckboxSize(v int) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.CheckboxSize = v
}

// CheckboxSizeAuto reports whether the checkbox uses auto-sizing.
func (ff *FormField) CheckboxSizeAuto() bool {
	if ff == nil || ff.ir == nil {
		return false
	}
	return ff.ir.CheckboxSizeAuto
}

// SetCheckboxSizeAuto toggles checkbox auto-sizing.
func (ff *FormField) SetCheckboxSizeAuto(v bool) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.CheckboxSizeAuto = v
}

// Items returns the list of dropdown items.
func (ff *FormField) Items() []string {
	if ff == nil || ff.ir == nil || len(ff.ir.Items) == 0 {
		return nil
	}
	out := make([]string, len(ff.ir.Items))
	copy(out, ff.ir.Items)
	return out
}

// SetItems replaces the dropdown items.
func (ff *FormField) SetItems(items []string) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.Items = append([]string(nil), items...)
}

// DefaultIndex returns the selected dropdown item index.
func (ff *FormField) DefaultIndex() int {
	if ff == nil || ff.ir == nil {
		return 0
	}
	return ff.ir.DefaultIdx
}

// SetDefaultIndex sets the selected dropdown item index.
func (ff *FormField) SetDefaultIndex(v int) {
	if ff == nil || ff.ir == nil {
		return
	}
	ff.ir.DefaultIdx = v
}

// FormFieldLocation describes where a form field was found.
type FormFieldLocation struct {
	Field    *FormField
	Location string // "body", "header", "footer", "footnote", "endnote", "comment"
}

// FormFields returns every legacy form field found in the document body and
// all stories.
func (d *Document) FormFields() []FormFieldLocation {
	if d == nil || d.IR == nil {
		return nil
	}
	var out []FormFieldLocation
	addBlocks := func(blocks []wordir.Block, loc string) {
		var walkBlocks func([]wordir.Block)
		walkBlocks = func(blocks []wordir.Block) {
			for _, b := range blocks {
				if b.Paragraph != nil {
					for i := range b.Paragraph.Runs {
						run := &b.Paragraph.Runs[i]
						if run.FormField != nil {
							out = append(out, FormFieldLocation{
								Field:    &FormField{ir: run.FormField},
								Location: loc,
							})
						}
						if run.TextBox != nil {
							walkBlocks(run.TextBox.Blocks)
						}
					}
				}
				if b.Table != nil {
					for _, row := range b.Table.Rows {
						for _, cell := range row.Cells {
							walkBlocks(cell.Blocks)
						}
					}
				}
			}
		}
		walkBlocks(blocks)
	}

	addBlocks(d.IR.Blocks, "body")
	for _, s := range d.headerPtrs {
		if s != nil {
			addBlocks(s.Blocks, "header")
		}
	}
	for _, s := range d.footerPtrs {
		if s != nil {
			addBlocks(s.Blocks, "footer")
		}
	}
	for _, s := range d.footnotePtrs {
		if s != nil {
			addBlocks(s.Blocks, "footnote")
		}
	}
	for _, s := range d.endnotePtrs {
		if s != nil {
			addBlocks(s.Blocks, "endnote")
		}
	}
	for _, s := range d.commentPtrs {
		if s != nil {
			addBlocks(s.Blocks, "comment")
		}
	}
	return out
}

// FormFieldByName returns the first form field whose name matches the given value.
func (d *Document) FormFieldByName(name string) *FormField {
	for _, loc := range d.FormFields() {
		if loc.Field != nil && loc.Field.Name() == name {
			return loc.Field
		}
	}
	return nil
}