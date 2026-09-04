package edit

import wordir "office/pkg/word/ir"

// ContentControl is a wrapper around a Word IR content control (structured
// document tag) that provides a convenient editing API.
type ContentControl struct {
	ir *wordir.ContentControl
}

// ID returns the content control identifier (w:id).
func (cc *ContentControl) ID() int {
	if cc == nil || cc.ir == nil {
		return 0
	}
	return cc.ir.ID
}

// Tag returns the programmatic tag (w:tag).
func (cc *ContentControl) Tag() string {
	if cc == nil || cc.ir == nil {
		return ""
	}
	return cc.ir.Tag
}

// Title returns the display title (w:alias).
func (cc *ContentControl) Title() string {
	if cc == nil || cc.ir == nil {
		return ""
	}
	return cc.ir.Title
}

// Type returns the content control type (e.g. "plainText", "richText",
// "checkBox", "comboBox", "dropDownList", "date", "picture").
func (cc *ContentControl) Type() string {
	if cc == nil || cc.ir == nil {
		return ""
	}
	return cc.ir.Type
}

// Checked reports whether a checkBox content control is checked.
func (cc *ContentControl) Checked() bool {
	if cc == nil || cc.ir == nil {
		return false
	}
	return cc.ir.Checked
}

// SetChecked sets the checked state of a checkBox content control.
func (cc *ContentControl) SetChecked(v bool) {
	if cc == nil || cc.ir == nil {
		return
	}
	cc.ir.Checked = v
}

// SetTag sets the programmatic tag.
func (cc *ContentControl) SetTag(v string) {
	if cc == nil || cc.ir == nil {
		return
	}
	cc.ir.Tag = v
}

// SetTitle sets the display title.
func (cc *ContentControl) SetTitle(v string) {
	if cc == nil || cc.ir == nil {
		return
	}
	cc.ir.Title = v
}

// IsShowingPlaceholder reports whether the control is currently showing
// placeholder text.
func (cc *ContentControl) IsShowingPlaceholder() bool {
	if cc == nil || cc.ir == nil {
		return false
	}
	return cc.ir.ShowingPlaceholder
}

// SetShowingPlaceholder sets whether the control shows placeholder text.
func (cc *ContentControl) SetShowingPlaceholder(v bool) {
	if cc == nil || cc.ir == nil {
		return
	}
	cc.ir.ShowingPlaceholder = v
}

// DataValue returns the resolved custom XML data value, if any.
func (cc *ContentControl) DataValue() string {
	if cc == nil || cc.ir == nil {
		return ""
	}
	return cc.ir.DataValue
}

// SetDataValue sets the custom XML data value.
func (cc *ContentControl) SetDataValue(v string) {
	if cc == nil || cc.ir == nil {
		return
	}
	cc.ir.DataValue = v
}

// ListItems returns the SDT list items for comboBox/dropDownList controls.
func (cc *ContentControl) ListItems() []wordir.SdtListItem {
	if cc == nil || cc.ir == nil || len(cc.ir.ListItems) == 0 {
		return nil
	}
	out := make([]wordir.SdtListItem, len(cc.ir.ListItems))
	copy(out, cc.ir.ListItems)
	return out
}

// DateFormat returns the configured date picture for date SDTs.
func (cc *ContentControl) DateFormat() string {
	if cc == nil || cc.ir == nil {
		return ""
	}
	return cc.ir.DateFormat
}

// DateFullDate returns the ISO-8601 fullDate value for date SDTs.
func (cc *ContentControl) DateFullDate() string {
	if cc == nil || cc.ir == nil {
		return ""
	}
	return cc.ir.DateFullDate
}

// DataBinding returns the data-binding XPath and store item ID.
func (cc *ContentControl) DataBinding() (xpath, storeItemID string) {
	if cc == nil || cc.ir == nil {
		return "", ""
	}
	return cc.ir.DataXPath, cc.ir.DataStoreID
}

// Placeholder returns the placeholder docPart name and gallery metadata.
func (cc *ContentControl) Placeholder() (docPart, gallery string) {
	if cc == nil || cc.ir == nil {
		return "", ""
	}
	return cc.ir.PlaceholderDocPart, cc.ir.PlaceholderDocPartGallery
}

// ContentControlLocation describes where a content control was found in the
// document.
type ContentControlLocation struct {
	Control  *ContentControl
	Location string // "body", "header", "footer", "footnote", "endnote", "comment"
}

// ContentControls returns every content control found in the document body and
// all stories (headers, footers, footnotes, endnotes, comments). The result
// preserves document order within each location but locations are enumerated
// in the order: body, headers, footers, footnotes, endnotes, comments.
func (d *Document) ContentControls() []ContentControlLocation {
	if d == nil || d.IR == nil {
		return nil
	}
	var out []ContentControlLocation
	seen := make(map[*wordir.ContentControl]bool)
	add := func(cc *wordir.ContentControl, loc string) {
		if cc == nil || seen[cc] {
			return
		}
		seen[cc] = true
		out = append(out, ContentControlLocation{
			Control:  &ContentControl{ir: cc},
			Location: loc,
		})
	}

	walkContentControlBlocks(d.IR.Blocks, "body", add)
	walkContentControlStories(d.headerPtrs, "header", add)
	walkContentControlStories(d.footerPtrs, "footer", add)
	walkContentControlStories(d.footnotePtrs, "footnote", add)
	walkContentControlStories(d.endnotePtrs, "endnote", add)
	walkContentControlStories(d.commentPtrs, "comment", add)

	return out
}

// ContentControlsByTag returns content controls whose Tag equals the given
// value. The search covers the body and all stories.
func (d *Document) ContentControlsByTag(tag string) []ContentControlLocation {
	var out []ContentControlLocation
	for _, loc := range d.ContentControls() {
		if loc.Control != nil && loc.Control.Tag() == tag {
			out = append(out, loc)
		}
	}
	return out
}

// ContentControlByID returns the first content control with the given ID, or
// nil if none is found.
func (d *Document) ContentControlByID(id int) *ContentControl {
	for _, loc := range d.ContentControls() {
		if loc.Control != nil && loc.Control.ID() == id {
			return loc.Control
		}
	}
	return nil
}

// WrapParagraphInContentControl wraps the given body paragraph in a block-level
// content control. It searches the document body and all stories for the block
// that contains this paragraph. Returns true if the paragraph was found and
// wrapped.
func (d *Document) WrapParagraphInContentControl(p *Paragraph, cc *wordir.ContentControl) bool {
	if d == nil || d.IR == nil || p == nil || p.ir == nil {
		return false
	}
	return setParagraphContentControlInBlocks(d.IR.Blocks, p.ir, cc) ||
		setParagraphContentControlInStories(d.headerPtrs, p.ir, cc) ||
		setParagraphContentControlInStories(d.footerPtrs, p.ir, cc) ||
		setParagraphContentControlInStories(d.footnotePtrs, p.ir, cc) ||
		setParagraphContentControlInStories(d.endnotePtrs, p.ir, cc) ||
		setParagraphContentControlInStories(d.commentPtrs, p.ir, cc)
}

// WrapTableInContentControl wraps the given body table in a block-level
// content control. It searches the document body and all stories for the block
// that contains this table. Returns true if the table was found and wrapped.
func (d *Document) WrapTableInContentControl(t *Table, cc *wordir.ContentControl) bool {
	if d == nil || d.IR == nil || t == nil || t.ir == nil {
		return false
	}
	return setTableContentControlInBlocks(d.IR.Blocks, t.ir, cc) ||
		setTableContentControlInStories(d.headerPtrs, t.ir, cc) ||
		setTableContentControlInStories(d.footerPtrs, t.ir, cc) ||
		setTableContentControlInStories(d.footnotePtrs, t.ir, cc) ||
		setTableContentControlInStories(d.endnotePtrs, t.ir, cc) ||
		setTableContentControlInStories(d.commentPtrs, t.ir, cc)
}

// UnwrapBlockContentControlFromParagraph removes any block-level content
// control from the block containing the given paragraph. Returns true if a
// content control was removed.
func (d *Document) UnwrapBlockContentControlFromParagraph(p *Paragraph) bool {
	if d == nil || d.IR == nil || p == nil || p.ir == nil {
		return false
	}
	return clearParagraphContentControlInBlocks(d.IR.Blocks, p.ir) ||
		clearParagraphContentControlInStories(d.headerPtrs, p.ir) ||
		clearParagraphContentControlInStories(d.footerPtrs, p.ir) ||
		clearParagraphContentControlInStories(d.footnotePtrs, p.ir) ||
		clearParagraphContentControlInStories(d.endnotePtrs, p.ir) ||
		clearParagraphContentControlInStories(d.commentPtrs, p.ir)
}

// UnwrapBlockContentControlFromTable removes any block-level content control
// from the block containing the given table. Returns true if a content control
// was removed.
func (d *Document) UnwrapBlockContentControlFromTable(t *Table) bool {
	if d == nil || d.IR == nil || t == nil || t.ir == nil {
		return false
	}
	return clearTableContentControlInBlocks(d.IR.Blocks, t.ir) ||
		clearTableContentControlInStories(d.headerPtrs, t.ir) ||
		clearTableContentControlInStories(d.footerPtrs, t.ir) ||
		clearTableContentControlInStories(d.footnotePtrs, t.ir) ||
		clearTableContentControlInStories(d.endnotePtrs, t.ir) ||
		clearTableContentControlInStories(d.commentPtrs, t.ir)
}

func walkContentControlBlocks(blocks []wordir.Block, loc string, visit func(*wordir.ContentControl, string)) {
	for i := range blocks {
		block := &blocks[i]
		visit(block.ContentControl, loc)
		if block.Paragraph != nil {
			walkContentControlParagraph(block.Paragraph, loc, visit)
		}
		if block.Table != nil {
			walkContentControlTable(block.Table, loc, visit)
		}
	}
}

func walkContentControlParagraph(para *wordir.Paragraph, loc string, visit func(*wordir.ContentControl, string)) {
	if para == nil {
		return
	}
	for i := range para.Runs {
		run := &para.Runs[i]
		visit(run.ContentControl, loc)
		walkContentControlTextBox(run.TextBox, loc, visit)
		walkContentControlShape(run.Shape, loc, visit)
	}
}

func walkContentControlTable(table *wordir.Table, loc string, visit func(*wordir.ContentControl, string)) {
	if table == nil {
		return
	}
	for i := range table.Rows {
		for j := range table.Rows[i].Cells {
			cell := &table.Rows[i].Cells[j]
			visit(cell.ContentControl, loc)
			walkContentControlBlocks(cell.Blocks, loc, visit)
		}
	}
}

func walkContentControlTextBox(textBox *wordir.TextBox, loc string, visit func(*wordir.ContentControl, string)) {
	if textBox == nil {
		return
	}
	walkContentControlBlocks(textBox.Blocks, loc, visit)
}

func walkContentControlShape(shape *wordir.Shape, loc string, visit func(*wordir.ContentControl, string)) {
	if shape == nil {
		return
	}
	walkContentControlTextBox(shape.TextBox, loc, visit)
	for i := range shape.Children {
		walkContentControlShape(&shape.Children[i], loc, visit)
	}
}

func walkContentControlStories[K comparable](stories map[K]*wordir.Story, loc string, visit func(*wordir.ContentControl, string)) {
	for _, story := range stories {
		if story == nil {
			continue
		}
		walkContentControlBlocks(story.Blocks, loc, visit)
	}
}

func setParagraphContentControlInBlocks(blocks []wordir.Block, para *wordir.Paragraph, cc *wordir.ContentControl) bool {
	for i := range blocks {
		block := &blocks[i]
		if block.Paragraph == para {
			block.ContentControl = cc
			return true
		}
		if block.Paragraph != nil && setParagraphContentControlInParagraph(block.Paragraph, para, cc) {
			return true
		}
		if block.Table != nil && setParagraphContentControlInTable(block.Table, para, cc) {
			return true
		}
	}
	return false
}

func setParagraphContentControlInParagraph(container, target *wordir.Paragraph, cc *wordir.ContentControl) bool {
	if container == nil {
		return false
	}
	for i := range container.Runs {
		run := &container.Runs[i]
		if setParagraphContentControlInTextBox(run.TextBox, target, cc) {
			return true
		}
		if setParagraphContentControlInShape(run.Shape, target, cc) {
			return true
		}
	}
	return false
}

func setParagraphContentControlInTable(table *wordir.Table, para *wordir.Paragraph, cc *wordir.ContentControl) bool {
	if table == nil {
		return false
	}
	for i := range table.Rows {
		for j := range table.Rows[i].Cells {
			if setParagraphContentControlInBlocks(table.Rows[i].Cells[j].Blocks, para, cc) {
				return true
			}
		}
	}
	return false
}

func setParagraphContentControlInTextBox(textBox *wordir.TextBox, para *wordir.Paragraph, cc *wordir.ContentControl) bool {
	if textBox == nil {
		return false
	}
	return setParagraphContentControlInBlocks(textBox.Blocks, para, cc)
}

func setParagraphContentControlInShape(shape *wordir.Shape, para *wordir.Paragraph, cc *wordir.ContentControl) bool {
	if shape == nil {
		return false
	}
	if setParagraphContentControlInTextBox(shape.TextBox, para, cc) {
		return true
	}
	for i := range shape.Children {
		if setParagraphContentControlInShape(&shape.Children[i], para, cc) {
			return true
		}
	}
	return false
}

func setParagraphContentControlInStories[K comparable](stories map[K]*wordir.Story, para *wordir.Paragraph, cc *wordir.ContentControl) bool {
	for _, story := range stories {
		if story != nil && setParagraphContentControlInBlocks(story.Blocks, para, cc) {
			return true
		}
	}
	return false
}

func setTableContentControlInBlocks(blocks []wordir.Block, table *wordir.Table, cc *wordir.ContentControl) bool {
	for i := range blocks {
		block := &blocks[i]
		if block.Table == table {
			block.ContentControl = cc
			return true
		}
		if block.Paragraph != nil && setTableContentControlInParagraph(block.Paragraph, table, cc) {
			return true
		}
		if block.Table != nil && setTableContentControlInTable(block.Table, table, cc) {
			return true
		}
	}
	return false
}

func setTableContentControlInParagraph(para *wordir.Paragraph, table *wordir.Table, cc *wordir.ContentControl) bool {
	if para == nil {
		return false
	}
	for i := range para.Runs {
		run := &para.Runs[i]
		if setTableContentControlInTextBox(run.TextBox, table, cc) {
			return true
		}
		if setTableContentControlInShape(run.Shape, table, cc) {
			return true
		}
	}
	return false
}

func setTableContentControlInTable(container, target *wordir.Table, cc *wordir.ContentControl) bool {
	if container == nil {
		return false
	}
	for i := range container.Rows {
		for j := range container.Rows[i].Cells {
			if setTableContentControlInBlocks(container.Rows[i].Cells[j].Blocks, target, cc) {
				return true
			}
		}
	}
	return false
}

func setTableContentControlInTextBox(textBox *wordir.TextBox, table *wordir.Table, cc *wordir.ContentControl) bool {
	if textBox == nil {
		return false
	}
	return setTableContentControlInBlocks(textBox.Blocks, table, cc)
}

func setTableContentControlInShape(shape *wordir.Shape, table *wordir.Table, cc *wordir.ContentControl) bool {
	if shape == nil {
		return false
	}
	if setTableContentControlInTextBox(shape.TextBox, table, cc) {
		return true
	}
	for i := range shape.Children {
		if setTableContentControlInShape(&shape.Children[i], table, cc) {
			return true
		}
	}
	return false
}

func setTableContentControlInStories[K comparable](stories map[K]*wordir.Story, table *wordir.Table, cc *wordir.ContentControl) bool {
	for _, story := range stories {
		if story != nil && setTableContentControlInBlocks(story.Blocks, table, cc) {
			return true
		}
	}
	return false
}

func clearParagraphContentControlInBlocks(blocks []wordir.Block, para *wordir.Paragraph) bool {
	for i := range blocks {
		block := &blocks[i]
		if block.Paragraph == para {
			if block.ContentControl == nil {
				return false
			}
			block.ContentControl = nil
			return true
		}
		if block.Paragraph != nil && clearParagraphContentControlInParagraph(block.Paragraph, para) {
			return true
		}
		if block.Table != nil && clearParagraphContentControlInTable(block.Table, para) {
			return true
		}
	}
	return false
}

func clearParagraphContentControlInStories[K comparable](stories map[K]*wordir.Story, para *wordir.Paragraph) bool {
	for _, story := range stories {
		if story != nil && clearParagraphContentControlInBlocks(story.Blocks, para) {
			return true
		}
	}
	return false
}

func clearTableContentControlInBlocks(blocks []wordir.Block, table *wordir.Table) bool {
	for i := range blocks {
		block := &blocks[i]
		if block.Table == table {
			if block.ContentControl == nil {
				return false
			}
			block.ContentControl = nil
			return true
		}
		if block.Paragraph != nil && clearTableContentControlInParagraph(block.Paragraph, table) {
			return true
		}
		if block.Table != nil && clearTableContentControlInTable(block.Table, table) {
			return true
		}
	}
	return false
}

func clearTableContentControlInStories[K comparable](stories map[K]*wordir.Story, table *wordir.Table) bool {
	for _, story := range stories {
		if story != nil && clearTableContentControlInBlocks(story.Blocks, table) {
			return true
		}
	}
	return false
}

func clearParagraphContentControlInParagraph(container, target *wordir.Paragraph) bool {
	if container == nil {
		return false
	}
	for i := range container.Runs {
		run := &container.Runs[i]
		if clearParagraphContentControlInTextBox(run.TextBox, target) {
			return true
		}
		if clearParagraphContentControlInShape(run.Shape, target) {
			return true
		}
	}
	return false
}

func clearParagraphContentControlInTable(table *wordir.Table, para *wordir.Paragraph) bool {
	if table == nil {
		return false
	}
	for i := range table.Rows {
		for j := range table.Rows[i].Cells {
			if clearParagraphContentControlInBlocks(table.Rows[i].Cells[j].Blocks, para) {
				return true
			}
		}
	}
	return false
}

func clearParagraphContentControlInTextBox(textBox *wordir.TextBox, para *wordir.Paragraph) bool {
	if textBox == nil {
		return false
	}
	return clearParagraphContentControlInBlocks(textBox.Blocks, para)
}

func clearParagraphContentControlInShape(shape *wordir.Shape, para *wordir.Paragraph) bool {
	if shape == nil {
		return false
	}
	if clearParagraphContentControlInTextBox(shape.TextBox, para) {
		return true
	}
	for i := range shape.Children {
		if clearParagraphContentControlInShape(&shape.Children[i], para) {
			return true
		}
	}
	return false
}

func clearTableContentControlInParagraph(para *wordir.Paragraph, table *wordir.Table) bool {
	if para == nil {
		return false
	}
	for i := range para.Runs {
		run := &para.Runs[i]
		if clearTableContentControlInTextBox(run.TextBox, table) {
			return true
		}
		if clearTableContentControlInShape(run.Shape, table) {
			return true
		}
	}
	return false
}

func clearTableContentControlInTable(container, target *wordir.Table) bool {
	if container == nil {
		return false
	}
	for i := range container.Rows {
		for j := range container.Rows[i].Cells {
			if clearTableContentControlInBlocks(container.Rows[i].Cells[j].Blocks, target) {
				return true
			}
		}
	}
	return false
}

func clearTableContentControlInTextBox(textBox *wordir.TextBox, table *wordir.Table) bool {
	if textBox == nil {
		return false
	}
	return clearTableContentControlInBlocks(textBox.Blocks, table)
}

func clearTableContentControlInShape(shape *wordir.Shape, table *wordir.Table) bool {
	if shape == nil {
		return false
	}
	if clearTableContentControlInTextBox(shape.TextBox, table) {
		return true
	}
	for i := range shape.Children {
		if clearTableContentControlInShape(&shape.Children[i], table) {
			return true
		}
	}
	return false
}

func blockContentControlForParagraphInBlocks(blocks []wordir.Block, para *wordir.Paragraph) *wordir.ContentControl {
	for i := range blocks {
		block := &blocks[i]
		if block.Paragraph == para {
			return block.ContentControl
		}
		if block.Paragraph != nil {
			if cc := blockContentControlForParagraphInParagraph(block.Paragraph, para); cc != nil {
				return cc
			}
		}
		if block.Table != nil {
			if cc := blockContentControlForParagraphInTable(block.Table, para); cc != nil {
				return cc
			}
		}
	}
	return nil
}

func blockContentControlForParagraphInParagraph(container, target *wordir.Paragraph) *wordir.ContentControl {
	if container == nil {
		return nil
	}
	for i := range container.Runs {
		run := &container.Runs[i]
		if cc := blockContentControlForParagraphInTextBox(run.TextBox, target); cc != nil {
			return cc
		}
		if cc := blockContentControlForParagraphInShape(run.Shape, target); cc != nil {
			return cc
		}
	}
	return nil
}

func blockContentControlForParagraphInTable(table *wordir.Table, para *wordir.Paragraph) *wordir.ContentControl {
	if table == nil {
		return nil
	}
	for i := range table.Rows {
		for j := range table.Rows[i].Cells {
			if cc := blockContentControlForParagraphInBlocks(table.Rows[i].Cells[j].Blocks, para); cc != nil {
				return cc
			}
		}
	}
	return nil
}

func blockContentControlForParagraphInTextBox(textBox *wordir.TextBox, para *wordir.Paragraph) *wordir.ContentControl {
	if textBox == nil {
		return nil
	}
	return blockContentControlForParagraphInBlocks(textBox.Blocks, para)
}

func blockContentControlForParagraphInShape(shape *wordir.Shape, para *wordir.Paragraph) *wordir.ContentControl {
	if shape == nil {
		return nil
	}
	if cc := blockContentControlForParagraphInTextBox(shape.TextBox, para); cc != nil {
		return cc
	}
	for i := range shape.Children {
		if cc := blockContentControlForParagraphInShape(&shape.Children[i], para); cc != nil {
			return cc
		}
	}
	return nil
}

func blockContentControlForParagraphInStories[K comparable](stories map[K]*wordir.Story, para *wordir.Paragraph) *wordir.ContentControl {
	for _, story := range stories {
		if story == nil {
			continue
		}
		if cc := blockContentControlForParagraphInBlocks(story.Blocks, para); cc != nil {
			return cc
		}
	}
	return nil
}

func blockContentControlForTableInBlocks(blocks []wordir.Block, table *wordir.Table) *wordir.ContentControl {
	for i := range blocks {
		block := &blocks[i]
		if block.Table == table {
			return block.ContentControl
		}
		if block.Paragraph != nil {
			if cc := blockContentControlForTableInParagraph(block.Paragraph, table); cc != nil {
				return cc
			}
		}
		if block.Table != nil {
			if cc := blockContentControlForTableInTable(block.Table, table); cc != nil {
				return cc
			}
		}
	}
	return nil
}

func blockContentControlForTableInParagraph(para *wordir.Paragraph, table *wordir.Table) *wordir.ContentControl {
	if para == nil {
		return nil
	}
	for i := range para.Runs {
		run := &para.Runs[i]
		if cc := blockContentControlForTableInTextBox(run.TextBox, table); cc != nil {
			return cc
		}
		if cc := blockContentControlForTableInShape(run.Shape, table); cc != nil {
			return cc
		}
	}
	return nil
}

func blockContentControlForTableInTable(container, target *wordir.Table) *wordir.ContentControl {
	if container == nil {
		return nil
	}
	for i := range container.Rows {
		for j := range container.Rows[i].Cells {
			if cc := blockContentControlForTableInBlocks(container.Rows[i].Cells[j].Blocks, target); cc != nil {
				return cc
			}
		}
	}
	return nil
}

func blockContentControlForTableInTextBox(textBox *wordir.TextBox, table *wordir.Table) *wordir.ContentControl {
	if textBox == nil {
		return nil
	}
	return blockContentControlForTableInBlocks(textBox.Blocks, table)
}

func blockContentControlForTableInShape(shape *wordir.Shape, table *wordir.Table) *wordir.ContentControl {
	if shape == nil {
		return nil
	}
	if cc := blockContentControlForTableInTextBox(shape.TextBox, table); cc != nil {
		return cc
	}
	for i := range shape.Children {
		if cc := blockContentControlForTableInShape(&shape.Children[i], table); cc != nil {
			return cc
		}
	}
	return nil
}

func blockContentControlForTableInStories[K comparable](stories map[K]*wordir.Story, table *wordir.Table) *wordir.ContentControl {
	for _, story := range stories {
		if story == nil {
			continue
		}
		if cc := blockContentControlForTableInBlocks(story.Blocks, table); cc != nil {
			return cc
		}
	}
	return nil
}