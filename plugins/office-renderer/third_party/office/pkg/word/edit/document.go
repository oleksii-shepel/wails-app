package edit

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"office/pkg/word/convert/legacytoooxml"
	wordir "office/pkg/word/ir"
	"office/pkg/word/parsers/legacy"
	"office/pkg/word/parsers/ooxml/parts"
)

// BodyElement represents a block-level element in a document body.
type BodyElement interface {
	ElementType() string
}

// Document is a format-neutral editor for a Word IR document.
type Document struct {
	IR *wordir.Document
	// nextParaIndex is used to assign unique paragraph indices when adding
	// new paragraphs. It is initialized from the highest existing index.
	nextParaIndex int
	// storyPtrs are pointer mirrors of the IR story maps so that
	// StoryEditor can mutate stories in-place without changing the IR type.
	headerPtrs   map[string]*wordir.Story
	footerPtrs   map[string]*wordir.Story
	footnotePtrs map[int]*wordir.Story
	endnotePtrs  map[int]*wordir.Story
	commentPtrs  map[int]*wordir.Story
	// extraRels holds user-added relationships that are merged into the
	// generated OOXML package during ToOOXML / SaveAsDOCX.
	extraRels []Relationship
}

// HeaderFooterStory identifies a header or footer story by Word story type.
type HeaderFooterStory struct {
	Type  string
	Story *StoryEditor
}

// NumberedStory identifies a footnote, endnote, or comment story by ID.
type NumberedStory struct {
	ID    int
	Story *StoryEditor
}

// NewDocument creates a new empty editable document.
func NewDocument() *Document {
	return &Document{
		IR: &wordir.Document{
			Paragraphs:      make([]wordir.Paragraph, 0),
			Tables:          make([]wordir.Table, 0),
			Blocks:          make([]wordir.Block, 0),
			Headers:         make(map[string]wordir.Story),
			Footers:         make(map[string]wordir.Story),
			Styles:          make(map[string]wordir.Style),
			CharacterStyles: make(map[string]wordir.CharacterStyle),
			Footnotes:       make(map[int]wordir.Story),
			Endnotes:        make(map[int]wordir.Story),
			Comments:        make(map[int]wordir.Story),
		},
		headerPtrs:   make(map[string]*wordir.Story),
		footerPtrs:   make(map[string]*wordir.Story),
		footnotePtrs: make(map[int]*wordir.Story),
		endnotePtrs:  make(map[int]*wordir.Story),
		commentPtrs:  make(map[int]*wordir.Story),
	}
}

// OpenIR wraps an existing IR document for editing.
func OpenIR(irDoc *wordir.Document) *Document {
	if irDoc == nil {
		return NewDocument()
	}
	d := &Document{
		IR:           irDoc,
		headerPtrs:   make(map[string]*wordir.Story),
		footerPtrs:   make(map[string]*wordir.Story),
		footnotePtrs: make(map[int]*wordir.Story),
		endnotePtrs:  make(map[int]*wordir.Story),
		commentPtrs:  make(map[int]*wordir.Story),
	}
	// Compute nextParaIndex from existing paragraphs/blocks.
	for _, p := range irDoc.Paragraphs {
		if p.Index >= d.nextParaIndex {
			d.nextParaIndex = p.Index + 1
		}
	}
	for _, b := range irDoc.Blocks {
		if b.Paragraph != nil && b.Paragraph.Index >= d.nextParaIndex {
			d.nextParaIndex = b.Paragraph.Index + 1
		}
	}
	d.syncStoryPtrs()
	return d
}

// syncStoryPtrs populates the pointer caches from the IR value maps.
func (d *Document) syncStoryPtrs() {
	for k, v := range d.IR.Headers {
		cp := v
		d.headerPtrs[k] = &cp
	}
	for k, v := range d.IR.Footers {
		cp := v
		d.footerPtrs[k] = &cp
	}
	for k, v := range d.IR.Footnotes {
		cp := v
		d.footnotePtrs[k] = &cp
	}
	for k, v := range d.IR.Endnotes {
		cp := v
		d.endnotePtrs[k] = &cp
	}
	for k, v := range d.IR.Comments {
		cp := v
		d.commentPtrs[k] = &cp
	}
}

// flushStoryPtrs writes the pointer caches back to the IR value maps.
func (d *Document) flushStoryPtrs() {
	for k, p := range d.headerPtrs {
		d.IR.Headers[k] = *p
	}
	for k, p := range d.footerPtrs {
		d.IR.Footers[k] = *p
	}
	for k, p := range d.footnotePtrs {
		d.IR.Footnotes[k] = *p
	}
	for k, p := range d.endnotePtrs {
		d.IR.Endnotes[k] = *p
	}
	for k, p := range d.commentPtrs {
		d.IR.Comments[k] = *p
	}
}

// AddParagraph appends a new paragraph to the document body and returns it.
func (d *Document) AddParagraph() *Paragraph {
	p := wordir.Paragraph{Index: d.nextParaIndex}
	d.nextParaIndex++
	d.IR.Blocks = append(d.IR.Blocks, wordir.Block{
		Index:     len(d.IR.Blocks),
		Kind:      "paragraph",
		Paragraph: &p,
	})
	d.rebuildFlatSlices()
	return &Paragraph{ir: d.IR.Blocks[len(d.IR.Blocks)-1].Paragraph, doc: d}
}

// InsertParagraphAt inserts a new paragraph at the given block index.
func (d *Document) InsertParagraphAt(blockIdx int) *Paragraph {
	if blockIdx < 0 {
		blockIdx = 0
	}
	if blockIdx > len(d.IR.Blocks) {
		blockIdx = len(d.IR.Blocks)
	}
	p := wordir.Paragraph{Index: d.nextParaIndex}
	d.nextParaIndex++
	b := wordir.Block{
		Index:     blockIdx,
		Kind:      "paragraph",
		Paragraph: &p,
	}
	// Insert block at position.
	d.IR.Blocks = append(d.IR.Blocks, wordir.Block{})
	copy(d.IR.Blocks[blockIdx+1:], d.IR.Blocks[blockIdx:])
	d.IR.Blocks[blockIdx] = b
	// Re-index subsequent blocks.
	for i := blockIdx + 1; i < len(d.IR.Blocks); i++ {
		d.IR.Blocks[i].Index = i
	}
	d.rebuildFlatSlices()
	return &Paragraph{ir: d.IR.Blocks[blockIdx].Paragraph, doc: d}
}

// RemoveParagraph removes a paragraph from the document body.
// It compares paragraph identity by pointer.
func (d *Document) RemoveParagraph(p *Paragraph) {
	if p == nil || p.ir == nil {
		return
	}
	for i := range d.IR.Blocks {
		if d.IR.Blocks[i].Paragraph == p.ir {
			d.IR.Blocks = append(d.IR.Blocks[:i], d.IR.Blocks[i+1:]...)
			for j := i; j < len(d.IR.Blocks); j++ {
				d.IR.Blocks[j].Index = j
			}
			d.rebuildFlatSlices()
			return
		}
	}
}

// Paragraphs returns all paragraphs in document order.
func (d *Document) Paragraphs() []*Paragraph {
	var out []*Paragraph
	for i := range d.IR.Blocks {
		if b := &d.IR.Blocks[i]; b.Kind == "paragraph" && b.Paragraph != nil {
			out = append(out, &Paragraph{ir: b.Paragraph, doc: d})
		}
	}
	return out
}

// ParagraphIterator provides an iterator over the document's paragraphs,
// matching the POI getParagraphsIterator() pattern.
type ParagraphIterator struct {
	paragraphs []*Paragraph
	index      int
}

// HasNext reports whether there are more paragraphs to iterate.
func (it *ParagraphIterator) HasNext() bool {
	return it != nil && it.index < len(it.paragraphs)
}

// Next returns the next paragraph and advances the iterator.
// Returns nil if there are no more paragraphs.
func (it *ParagraphIterator) Next() *Paragraph {
	if it == nil || it.index >= len(it.paragraphs) {
		return nil
	}
	p := it.paragraphs[it.index]
	it.index++
	return p
}

// ParagraphIterator returns an iterator over all paragraphs in document order.
func (d *Document) ParagraphIterator() *ParagraphIterator {
	return &ParagraphIterator{paragraphs: d.Paragraphs()}
}

// Bookmark represents a bookmark found in the document.
type Bookmark struct {
	Name     string // bookmark name
	Location string // "body", "header", "footer", "footnote", "endnote", "comment"
}

// ImageRun pairs a run containing an image with its location context.
type ImageRun struct {
	Run      *Run
	Location string // "body", "header", "footer", "footnote", "endnote", "comment"
}

// ImageRuns returns all runs that contain an image anywhere in the document
// body and all stories (headers, footers, footnotes, endnotes, comments).
func (d *Document) ImageRuns() []ImageRun {
	var out []ImageRun

	visit := func(_ *wordir.Run, para *wordir.Paragraph, runIndex int, loc string) {
		out = append(out, ImageRun{
			Run:      &Run{para: &Paragraph{ir: para, doc: d}, index: runIndex},
			Location: loc,
		})
	}

	walkDocumentRunImages(d.IR, "body", visit)

	for _, s := range d.headerPtrs {
		walkStoryRunImages(s, "header", visit)
	}
	for _, s := range d.footerPtrs {
		walkStoryRunImages(s, "footer", visit)
	}
	for _, s := range d.footnotePtrs {
		walkStoryRunImages(s, "footnote", visit)
	}
	for _, s := range d.endnotePtrs {
		walkStoryRunImages(s, "endnote", visit)
	}
	for _, s := range d.commentPtrs {
		walkStoryRunImages(s, "comment", visit)
	}

	return out
}

// InlineImage holds a reference to an image found in the document.
type InlineImage struct {
	Index    int    // zero-based position in the returned slice
	MIMEType string // e.g. "image/png"
	Width    int64  // EMU
	Height   int64  // EMU
	Location string // "body", "header", "footer", "footnote", "endnote", "comment"
	Alt      string // alternative text
	Title    string // image title
}

// Images returns all inline images found anywhere in the document body
// and all stories (headers, footers, footnotes, endnotes, comments).
func (d *Document) Images() []InlineImage {
	var out []InlineImage

	add := func(img *wordir.Image, loc string) {
		if img == nil {
			return
		}
		idx := len(out)
		out = append(out, InlineImage{
			Index:    idx,
			MIMEType: img.MIMEType,
			Width:    img.Width,
			Height:   img.Height,
			Location: loc,
			Alt:      img.Alt,
			Title:    img.Title,
		})
	}

	walkDocumentRunImages(d.IR, "body", func(r *wordir.Run, _ *wordir.Paragraph, _ int, loc string) {
		add(r.Image, loc)
	})
	for _, s := range d.headerPtrs {
		walkStoryRunImages(s, "header", func(r *wordir.Run, _ *wordir.Paragraph, _ int, loc string) {
			add(r.Image, loc)
		})
	}
	for _, s := range d.footerPtrs {
		walkStoryRunImages(s, "footer", func(r *wordir.Run, _ *wordir.Paragraph, _ int, loc string) {
			add(r.Image, loc)
		})
	}
	for _, s := range d.footnotePtrs {
		walkStoryRunImages(s, "footnote", func(r *wordir.Run, _ *wordir.Paragraph, _ int, loc string) {
			add(r.Image, loc)
		})
	}
	for _, s := range d.endnotePtrs {
		walkStoryRunImages(s, "endnote", func(r *wordir.Run, _ *wordir.Paragraph, _ int, loc string) {
			add(r.Image, loc)
		})
	}
	for _, s := range d.commentPtrs {
		walkStoryRunImages(s, "comment", func(r *wordir.Run, _ *wordir.Paragraph, _ int, loc string) {
			add(r.Image, loc)
		})
	}

	return out
}

func walkDocumentRunImages(doc *wordir.Document, loc string, visit func(*wordir.Run, *wordir.Paragraph, int, string)) {
	if doc == nil {
		return
	}
	if len(doc.Blocks) > 0 {
		walkBlockRunImages(doc.Blocks, loc, visit)
		return
	}
	for i := range doc.Paragraphs {
		walkParagraphRunImages(&doc.Paragraphs[i], loc, visit)
	}
	for i := range doc.Tables {
		walkTableRunImages(&doc.Tables[i], loc, visit)
	}
}

func walkStoryRunImages(story *wordir.Story, loc string, visit func(*wordir.Run, *wordir.Paragraph, int, string)) {
	if story == nil {
		return
	}
	if len(story.Blocks) > 0 {
		walkBlockRunImages(story.Blocks, loc, visit)
		return
	}
	for i := range story.Paragraphs {
		walkParagraphRunImages(&story.Paragraphs[i], loc, visit)
	}
	for i := range story.Tables {
		walkTableRunImages(&story.Tables[i], loc, visit)
	}
}

func walkBlockRunImages(blocks []wordir.Block, loc string, visit func(*wordir.Run, *wordir.Paragraph, int, string)) {
	for i := range blocks {
		if blocks[i].Paragraph != nil {
			walkParagraphRunImages(blocks[i].Paragraph, loc, visit)
		}
		if blocks[i].Table != nil {
			walkTableRunImages(blocks[i].Table, loc, visit)
		}
	}
}

func walkTableRunImages(table *wordir.Table, loc string, visit func(*wordir.Run, *wordir.Paragraph, int, string)) {
	if table == nil {
		return
	}
	for i := range table.Rows {
		for j := range table.Rows[i].Cells {
			walkCellRunImages(&table.Rows[i].Cells[j], loc, visit)
		}
	}
}

func walkCellRunImages(cell *wordir.TableCell, loc string, visit func(*wordir.Run, *wordir.Paragraph, int, string)) {
	if cell == nil {
		return
	}
	if len(cell.Blocks) > 0 {
		walkBlockRunImages(cell.Blocks, loc, visit)
		return
	}
	for i := range cell.Paragraphs {
		walkParagraphRunImages(&cell.Paragraphs[i], loc, visit)
	}
	for i := range cell.Tables {
		walkTableRunImages(&cell.Tables[i], loc, visit)
	}
}

func walkParagraphRunImages(para *wordir.Paragraph, loc string, visit func(*wordir.Run, *wordir.Paragraph, int, string)) {
	if para == nil {
		return
	}
	for i := range para.Runs {
		run := &para.Runs[i]
		if run.Image != nil {
			visit(run, para, i, loc)
		}
		walkTextBoxRunImages(run.TextBox, loc, visit)
		walkShapeRunImages(run.Shape, loc, visit)
	}
}

func walkShapeRunImages(shape *wordir.Shape, loc string, visit func(*wordir.Run, *wordir.Paragraph, int, string)) {
	if shape == nil {
		return
	}
	walkTextBoxRunImages(shape.TextBox, loc, visit)
	for i := range shape.Children {
		walkShapeRunImages(&shape.Children[i], loc, visit)
	}
}

func walkTextBoxRunImages(textBox *wordir.TextBox, loc string, visit func(*wordir.Run, *wordir.Paragraph, int, string)) {
	if textBox == nil {
		return
	}
	walkBlockRunImages(textBox.Blocks, loc, visit)
}

// AddTable appends a new table to the document body and returns it.
func (d *Document) AddTable() *Table {
	t := wordir.Table{Index: len(d.IR.Tables)}
	d.IR.Blocks = append(d.IR.Blocks, wordir.Block{
		Index: len(d.IR.Blocks),
		Kind:  "table",
		Table: &t,
	})
	d.rebuildFlatSlices()
	return &Table{ir: d.IR.Blocks[len(d.IR.Blocks)-1].Table, doc: d}
}

// InsertTableAt inserts a new table at the given block index.
func (d *Document) InsertTableAt(blockIdx int) *Table {
	if blockIdx < 0 {
		blockIdx = 0
	}
	if blockIdx > len(d.IR.Blocks) {
		blockIdx = len(d.IR.Blocks)
	}
	t := wordir.Table{Index: len(d.IR.Tables)}
	b := wordir.Block{
		Index: blockIdx,
		Kind:  "table",
		Table: &t,
	}
	d.IR.Blocks = append(d.IR.Blocks, wordir.Block{})
	copy(d.IR.Blocks[blockIdx+1:], d.IR.Blocks[blockIdx:])
	d.IR.Blocks[blockIdx] = b
	for i := blockIdx + 1; i < len(d.IR.Blocks); i++ {
		d.IR.Blocks[i].Index = i
	}
	d.rebuildFlatSlices()
	return &Table{ir: d.IR.Blocks[blockIdx].Table, doc: d}
}

// RemoveTable removes a table from the document body.
func (d *Document) RemoveTable(t *Table) {
	if t == nil || t.ir == nil {
		return
	}
	for i := range d.IR.Blocks {
		if d.IR.Blocks[i].Table == t.ir {
			d.IR.Blocks = append(d.IR.Blocks[:i], d.IR.Blocks[i+1:]...)
			for j := i; j < len(d.IR.Blocks); j++ {
				d.IR.Blocks[j].Index = j
			}
			d.rebuildFlatSlices()
			return
		}
	}
}

// Tables returns all tables in document order.
func (d *Document) Tables() []*Table {
	var out []*Table
	for i := range d.IR.Blocks {
		if b := &d.IR.Blocks[i]; b.Kind == "table" && b.Table != nil {
			out = append(out, &Table{ir: b.Table, doc: d})
		}
	}
	return out
}

// TableIterator provides an iterator over the document's tables.
type TableIterator struct {
	tables []*Table
	index  int
}

// HasNext reports whether there are more tables to iterate.
func (it *TableIterator) HasNext() bool {
	return it != nil && it.index < len(it.tables)
}

// Next returns the next table and advances the iterator.
// Returns nil if there are no more tables.
func (it *TableIterator) Next() *Table {
	if it == nil || it.index >= len(it.tables) {
		return nil
	}
	t := it.tables[it.index]
	it.index++
	return t
}

// TableIterator returns an iterator over all tables in document order.
func (d *Document) TableIterator() *TableIterator {
	return &TableIterator{tables: d.Tables()}
}

// BodyElements returns all top-level body elements (paragraphs and tables)
// in document order.
func (d *Document) BodyElements() []BodyElement {
	var out []BodyElement
	for i := range d.IR.Blocks {
		b := &d.IR.Blocks[i]
		switch b.Kind {
		case "paragraph":
			if b.Paragraph != nil {
				out = append(out, &Paragraph{ir: b.Paragraph, doc: d})
			}
		case "table":
			if b.Table != nil {
				out = append(out, &Table{ir: b.Table, doc: d})
			}
		}
	}
	return out
}

// SetTitle sets the document title.
func (d *Document) SetTitle(v string) { d.IR.Title = v }

// SetAuthor sets the document author.
func (d *Document) SetAuthor(v string) { d.IR.Author = v }

// SetSubject sets the document subject.
func (d *Document) SetSubject(v string) { d.IR.Subject = v }

// SetDescription sets the document description.
func (d *Document) SetDescription(v string) { d.IR.Description = v }

// SetKeywords sets the document keywords.
func (d *Document) SetKeywords(v string) { d.IR.Keywords = v }

// SetCompany sets the document company.
func (d *Document) SetCompany(v string) { d.IR.Company = v }

// SetManager sets the document manager.
func (d *Document) SetManager(v string) { d.IR.Manager = v }

// SetTemplate sets the document template name.
func (d *Document) SetTemplate(v string) { d.IR.Template = v }

// SetApplication sets the document application name.
func (d *Document) SetApplication(v string) { d.IR.Application = v }

// AddHeader creates a new header story of the given type (e.g. "default",
// "first", "even") and returns a StoryEditor for it.
func (d *Document) AddHeader(typ string) *StoryEditor {
	story := &wordir.Story{}
	d.headerPtrs[typ] = story
	return &StoryEditor{story: story, doc: d}
}

// AddFooter creates a new footer story of the given type and returns a
// StoryEditor for it.
func (d *Document) AddFooter(typ string) *StoryEditor {
	story := &wordir.Story{}
	d.footerPtrs[typ] = story
	return &StoryEditor{story: story, doc: d}
}

// AddFootnote creates a new footnote with the given ID and returns a
// StoryEditor for it.
func (d *Document) AddFootnote(id int) *StoryEditor {
	story := &wordir.Story{}
	d.footnotePtrs[id] = story
	return &StoryEditor{story: story, doc: d}
}

// AddEndnote creates a new endnote with the given ID and returns a
// StoryEditor for it.
func (d *Document) AddEndnote(id int) *StoryEditor {
	story := &wordir.Story{}
	d.endnotePtrs[id] = story
	return &StoryEditor{story: story, doc: d}
}

// AddComment creates a new comment with the given ID and returns a
// StoryEditor for it.
func (d *Document) AddComment(id int) *StoryEditor {
	story := &wordir.Story{}
	d.commentPtrs[id] = story
	return &StoryEditor{story: story, doc: d}
}

// Header returns the header story editor for the given type, or nil if none.
func (d *Document) Header(typ string) *StoryEditor {
	if s, ok := d.headerPtrs[typ]; ok {
		return &StoryEditor{story: s, doc: d}
	}
	return nil
}

// Footer returns the footer story editor for the given type, or nil if none.
func (d *Document) Footer(typ string) *StoryEditor {
	if s, ok := d.footerPtrs[typ]; ok {
		return &StoryEditor{story: s, doc: d}
	}
	return nil
}

// Headers returns all header stories sorted by type.
func (d *Document) Headers() []HeaderFooterStory {
	return headerFooterStories(d, d.headerPtrs)
}

// Footers returns all footer stories sorted by type.
func (d *Document) Footers() []HeaderFooterStory {
	return headerFooterStories(d, d.footerPtrs)
}

// Footnote returns the footnote story editor for the given ID, or nil if none.
func (d *Document) Footnote(id int) *StoryEditor {
	if s, ok := d.footnotePtrs[id]; ok {
		return &StoryEditor{story: s, doc: d}
	}
	return nil
}

// Footnotes returns all footnote stories sorted by ID.
func (d *Document) Footnotes() []NumberedStory {
	return numberedStories(d, d.footnotePtrs)
}

// Endnote returns the endnote story editor for the given ID, or nil if none.
func (d *Document) Endnote(id int) *StoryEditor {
	if s, ok := d.endnotePtrs[id]; ok {
		return &StoryEditor{story: s, doc: d}
	}
	return nil
}

// Endnotes returns all endnote stories sorted by ID.
func (d *Document) Endnotes() []NumberedStory {
	return numberedStories(d, d.endnotePtrs)
}

// Comment returns the comment story editor for the given ID, or nil if none.
func (d *Document) Comment(id int) *StoryEditor {
	if s, ok := d.commentPtrs[id]; ok {
		return &StoryEditor{story: s, doc: d}
	}
	return nil
}

// Comments returns all comment stories sorted by ID.
func (d *Document) Comments() []NumberedStory {
	return numberedStories(d, d.commentPtrs)
}

func headerFooterStories(doc *Document, stories map[string]*wordir.Story) []HeaderFooterStory {
	keys := make([]string, 0, len(stories))
	for typ := range stories {
		keys = append(keys, typ)
	}
	sort.Strings(keys)
	out := make([]HeaderFooterStory, 0, len(keys))
	for _, typ := range keys {
		out = append(out, HeaderFooterStory{
			Type:  typ,
			Story: &StoryEditor{story: stories[typ], doc: doc},
		})
	}
	return out
}

func numberedStories(doc *Document, stories map[int]*wordir.Story) []NumberedStory {
	ids := make([]int, 0, len(stories))
	for id := range stories {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	out := make([]NumberedStory, 0, len(ids))
	for _, id := range ids {
		out = append(out, NumberedStory{
			ID:    id,
			Story: &StoryEditor{story: stories[id], doc: doc},
		})
	}
	return out
}

// RemoveHeader removes the header of the given type.
func (d *Document) RemoveHeader(typ string) {
	delete(d.headerPtrs, typ)
	delete(d.IR.Headers, typ)
}

// RemoveFooter removes the footer of the given type.
func (d *Document) RemoveFooter(typ string) {
	delete(d.footerPtrs, typ)
	delete(d.IR.Footers, typ)
}

// RemoveFootnote removes the footnote with the given ID.
func (d *Document) RemoveFootnote(id int) {
	delete(d.footnotePtrs, id)
	delete(d.IR.Footnotes, id)
}

// RemoveEndnote removes the endnote with the given ID.
func (d *Document) RemoveEndnote(id int) {
	delete(d.endnotePtrs, id)
	delete(d.IR.Endnotes, id)
}

// RemoveComment removes the comment with the given ID.
func (d *Document) RemoveComment(id int) {
	delete(d.commentPtrs, id)
	delete(d.IR.Comments, id)
}

// NextFootnoteID returns the next unused footnote ID.
func (d *Document) NextFootnoteID() int {
	max := 0
	for id := range d.IR.Footnotes {
		if id > max {
			max = id
		}
	}
	for id := range d.footnotePtrs {
		if id > max {
			max = id
		}
	}
	return max + 1
}

// NextEndnoteID returns the next unused endnote ID.
func (d *Document) NextEndnoteID() int {
	max := 0
	for id := range d.IR.Endnotes {
		if id > max {
			max = id
		}
	}
	for id := range d.endnotePtrs {
		if id > max {
			max = id
		}
	}
	return max + 1
}

// NextCommentID returns the next unused comment ID.
func (d *Document) NextCommentID() int {
	max := 0
	for id := range d.IR.Comments {
		if id > max {
			max = id
		}
	}
	for id := range d.commentPtrs {
		if id > max {
			max = id
		}
	}
	return max + 1
}

// AddStoryField appends a new block-spanning field to the document body and
// returns a StoryField editor for it.
func (d *Document) AddStoryField(kind, instr string) *StoryField {
	sf := wordir.StoryField{Kind: kind, Instr: instr}
	d.IR.Fields = append(d.IR.Fields, sf)
	return &StoryField{ir: &d.IR.Fields[len(d.IR.Fields)-1], doc: d}
}

// StoryFields returns all story-spanning fields in the document body.
func (d *Document) StoryFields() []*StoryField {
	out := make([]*StoryField, len(d.IR.Fields))
	for i := range d.IR.Fields {
		out[i] = &StoryField{ir: &d.IR.Fields[i], doc: d}
	}
	return out
}

// AddSection appends a new section to the document.
func (d *Document) AddSection() *Section {
	d.IR.Sections = append(d.IR.Sections, wordir.Section{})
	return &Section{doc: d, index: len(d.IR.Sections) - 1}
}

// Sections returns all sections in the document.
func (d *Document) Sections() []*Section {
	out := make([]*Section, len(d.IR.Sections))
	for i := range d.IR.Sections {
		out[i] = &Section{doc: d, index: i}
	}
	return out
}

// InsertSectionAt inserts a new section at the given index.
func (d *Document) InsertSectionAt(idx int) *Section {
	if idx < 0 {
		idx = 0
	}
	if idx > len(d.IR.Sections) {
		idx = len(d.IR.Sections)
	}
	d.IR.Sections = append(d.IR.Sections, wordir.Section{})
	copy(d.IR.Sections[idx+1:], d.IR.Sections[idx:])
	d.IR.Sections[idx] = wordir.Section{}
	return &Section{doc: d, index: idx}
}

// RemoveSection removes a section from the document.
func (d *Document) RemoveSection(s *Section) {
	if s == nil || s.doc != d {
		return
	}
	idx := s.index
	if idx < 0 || idx >= len(d.IR.Sections) {
		return
	}
	d.IR.Sections = append(d.IR.Sections[:idx], d.IR.Sections[idx+1:]...)
}

// EnsureNumberingDefinition creates or returns an existing numbering
// definition with the given numID and abstractNumID. The definition is used
// by the OOXML converter to emit a valid numbering.xml part.
func (d *Document) EnsureNumberingDefinition(numID, abstractNumID int) *NumberingDef {
	for i := range d.IR.NumberingDefinitions {
		if d.IR.NumberingDefinitions[i].NumID == numID {
			return &NumberingDef{doc: d, index: i}
		}
	}
	d.IR.NumberingDefinitions = append(d.IR.NumberingDefinitions, wordir.NumberingDefinition{
		NumID:         numID,
		AbstractNumID: abstractNumID,
	})
	return &NumberingDef{doc: d, index: len(d.IR.NumberingDefinitions) - 1}
}

// EnsureParagraphStyle creates or updates a paragraph style with the given ID
// and name. If the style already exists, its name is updated.
func (d *Document) EnsureParagraphStyle(id, name string) *ParagraphStyle {
	if d.IR.Styles == nil {
		d.IR.Styles = make(map[string]wordir.Style)
	}
	s := d.IR.Styles[id]
	s.ID = id
	s.Name = name
	d.IR.Styles[id] = s
	return &ParagraphStyle{doc: d, id: id}
}

// EnsureCharacterStyle creates or updates a character style with the given ID
// and name.
func (d *Document) EnsureCharacterStyle(id, name string) *CharacterStyle {
	if d.IR.CharacterStyles == nil {
		d.IR.CharacterStyles = make(map[string]wordir.CharacterStyle)
	}
	s := d.IR.CharacterStyles[id]
	s.ID = id
	s.Name = name
	d.IR.CharacterStyles[id] = s
	return &CharacterStyle{doc: d, id: id}
}

// ParagraphStyles returns all paragraph styles in the document.
func (d *Document) ParagraphStyles() []*ParagraphStyle {
	var out []*ParagraphStyle
	for id, s := range d.IR.Styles {
		if s.TableFormat == nil {
			out = append(out, &ParagraphStyle{doc: d, id: id})
		}
	}
	return out
}

// CharacterStyles returns all character styles in the document.
func (d *Document) CharacterStyles() []*CharacterStyle {
	var out []*CharacterStyle
	for id := range d.IR.CharacterStyles {
		out = append(out, &CharacterStyle{doc: d, id: id})
	}
	return out
}

// TableStyles returns all table styles in the document.
func (d *Document) TableStyles() []*TableStyle {
	var out []*TableStyle
	for id, s := range d.IR.Styles {
		if s.TableFormat != nil {
			out = append(out, &TableStyle{doc: d, id: id})
		}
	}
	return out
}

// ParagraphStyle returns the paragraph style with the given ID, or nil if not found.
func (d *Document) ParagraphStyle(id string) *ParagraphStyle {
	if d.IR.Styles == nil {
		return nil
	}
	s, ok := d.IR.Styles[id]
	if !ok || s.TableFormat != nil {
		return nil
	}
	return &ParagraphStyle{doc: d, id: id}
}

// CharacterStyle returns the character style with the given ID, or nil if not found.
func (d *Document) CharacterStyle(id string) *CharacterStyle {
	if d.IR.CharacterStyles == nil {
		return nil
	}
	if _, ok := d.IR.CharacterStyles[id]; !ok {
		return nil
	}
	return &CharacterStyle{doc: d, id: id}
}

// TableStyle returns the table style with the given ID, or nil if not found.
func (d *Document) TableStyle(id string) *TableStyle {
	if d.IR.Styles == nil {
		return nil
	}
	s, ok := d.IR.Styles[id]
	if !ok || s.TableFormat == nil {
		return nil
	}
	return &TableStyle{doc: d, id: id}
}

// RemoveParagraphStyle removes a paragraph style by ID.
func (d *Document) RemoveParagraphStyle(id string) {
	if d.IR.Styles == nil {
		return
	}
	s, ok := d.IR.Styles[id]
	if !ok || s.TableFormat != nil {
		return
	}
	delete(d.IR.Styles, id)
}

// RemoveCharacterStyle removes a character style by ID.
func (d *Document) RemoveCharacterStyle(id string) {
	if d.IR.CharacterStyles == nil {
		return
	}
	delete(d.IR.CharacterStyles, id)
}

// RemoveTableStyle removes a table style by ID.
func (d *Document) RemoveTableStyle(id string) {
	if d.IR.Styles == nil {
		return
	}
	s, ok := d.IR.Styles[id]
	if !ok || s.TableFormat == nil {
		return
	}
	delete(d.IR.Styles, id)
}

// EnsureTableStyle creates or returns an existing table style with the given ID
// and name.
func (d *Document) EnsureTableStyle(id, name string) *TableStyle {
	if d.IR.Styles == nil {
		d.IR.Styles = make(map[string]wordir.Style)
	}
	s := d.IR.Styles[id]
	s.ID = id
	s.Name = name
	if s.TableFormat == nil {
		s.TableFormat = &wordir.TableFormat{}
	}
	d.IR.Styles[id] = s
	return &TableStyle{doc: d, id: id}
}

// ToOOXML serializes the edited IR document to an OOXML package.
func (d *Document) ToOOXML() (*parts.Package, error) {
	if d.IR == nil {
		return nil, fmt.Errorf("edit: nil IR document")
	}
	// Ensure flat slices and story caches are in sync before export.
	d.rebuildFlatSlices()
	d.flushStoryPtrs()
	pkg, err := legacytoooxml.Convert(d.IR)
	if err != nil {
		return nil, err
	}
	// Merge user-added extra relationships into the package.
	if len(d.extraRels) > 0 {
		for _, er := range d.extraRels {
			key := er.SourcePart
			if key == "" {
				key = pkg.MainPartName
			}
			if key == "" {
				key = "word/document.xml"
			}
			if pkg.PartRels == nil {
				pkg.PartRels = make(map[string]*parts.Relationships)
			}
			rels, ok := pkg.PartRels[key]
			if !ok || rels == nil {
				rels = &parts.Relationships{}
				pkg.PartRels[key] = rels
			}
			// Avoid duplicate IDs.
			exists := false
			for _, existing := range rels.Relationships {
				if existing.ID == er.ID {
					exists = true
					break
				}
			}
			if !exists {
				rels.Relationships = append(rels.Relationships, &parts.Relationship{
					ID:         er.ID,
					Type:       er.Type,
					Target:     er.Target,
					TargetMode: er.TargetMode,
				})
			}
		}
	}
	return pkg, nil
}

// SaveAsDOCX writes the document as a .docx file to w.
func (d *Document) SaveAsDOCX(w io.Writer) error {
	pkg, err := d.ToOOXML()
	if err != nil {
		return err
	}
	return parts.WritePackage(pkg, w)
}

// SaveAsDOC writes the document as a legacy Word 97-2003 .doc file to w.
// The writer covers the semantic legacy feature set represented by the IR,
// including styles, numbering, tables, images, OfficeArt, fields, secondary
// stories, and preservation metadata for embedded-object fidelity.
func (d *Document) SaveAsDOC(w io.Writer) error {
	if d == nil || d.IR == nil {
		return fmt.Errorf("edit: nil document")
	}
	return legacy.WriteDocument(d.IR, w)
}

// rebuildFlatSlices rebuilds the Document.Paragraphs and Document.Tables
// slices from Blocks so that downstream consumers (renderer, converter)
// can use whichever view they prefer. After rebuilding, Block paragraph
// and table pointers are updated to point into the new slices so that
// Block-level and slice-level views remain consistent.
func (d *Document) rebuildFlatSlices() {
	d.IR.Paragraphs = d.IR.Paragraphs[:0]
	d.IR.Tables = d.IR.Tables[:0]
	for i := range d.IR.Blocks {
		b := &d.IR.Blocks[i]
		switch b.Kind {
		case "paragraph":
			if b.Paragraph != nil {
				d.IR.Paragraphs = append(d.IR.Paragraphs, *b.Paragraph)
			}
		case "table":
			if b.Table != nil {
				d.IR.Tables = append(d.IR.Tables, *b.Table)
			}
		}
	}
}

// SetDefaultFont sets the document default font family and size.
// ascii, eastAsia, and cs are font family names; size is in half-points.
func (d *Document) SetDefaultFont(ascii, eastAsia, cs string, size int) {
	if d.IR.DefaultRunFormat.FontSize == nil {
		d.IR.DefaultRunFormat.FontSize = &size
	} else {
		*d.IR.DefaultRunFormat.FontSize = size
	}
	d.IR.DefaultRunFormat.FontASCII = ascii
	d.IR.DefaultRunFormat.FontEastAsia = eastAsia
	d.IR.DefaultRunFormat.FontCS = cs
}

// DefaultParagraphStyleID returns the default paragraph style ID.
func (d *Document) DefaultParagraphStyleID() string {
	if d == nil || d.IR == nil {
		return ""
	}
	return d.IR.DefaultParagraphStyleID
}

// SetDefaultParagraphStyle sets the default paragraph style ID.
func (d *Document) SetDefaultParagraphStyle(styleID string) {
	if d == nil || d.IR == nil {
		return
	}
	d.IR.DefaultParagraphStyleID = styleID
}

// DefaultCharacterStyleID returns the default character style ID.
func (d *Document) DefaultCharacterStyleID() string {
	if d == nil || d.IR == nil {
		return ""
	}
	return d.IR.DefaultCharacterStyleID
}

// SetDefaultCharacterStyle sets the default character style ID.
func (d *Document) SetDefaultCharacterStyle(styleID string) {
	if d == nil || d.IR == nil {
		return
	}
	d.IR.DefaultCharacterStyleID = styleID
}

// DefaultRunFormat returns the document-wide default run format (from
// w:docDefaults/w:rPrDefault). Changes to the returned value do not affect the
// document; use SetDefaultRunFormat to apply modifications.
func (d *Document) DefaultRunFormat() wordir.RunFormat {
	if d == nil || d.IR == nil {
		return wordir.RunFormat{}
	}
	return d.IR.DefaultRunFormat
}

// SetDefaultRunFormat sets the document-wide default run format.
func (d *Document) SetDefaultRunFormat(fmt wordir.RunFormat) {
	if d == nil || d.IR == nil {
		return
	}
	d.IR.DefaultRunFormat = fmt
}

// DefaultParagraphFormat returns the document-wide default paragraph format
// (from w:docDefaults/w:pPrDefault). Changes to the returned value do not
// affect the document; use SetDefaultParagraphFormat to apply modifications.
func (d *Document) DefaultParagraphFormat() wordir.ParagraphFormat {
	if d == nil || d.IR == nil {
		return wordir.ParagraphFormat{}
	}
	return d.IR.DefaultParagraphFormat
}

// SetDefaultParagraphFormat sets the document-wide default paragraph format.
func (d *Document) SetDefaultParagraphFormat(fmt wordir.ParagraphFormat) {
	if d == nil || d.IR == nil {
		return
	}
	d.IR.DefaultParagraphFormat = fmt
}

// SetDocumentProtection enables document editing restrictions.
// edit is one of: "forms", "comments", "trackedChanges", "readOnly".
func (d *Document) SetDocumentProtection(edit string, enforce bool) {
	d.IR.DocumentProtection = &wordir.DocumentProtection{
		Edit:        edit,
		Enforcement: enforce,
	}
}

// SetZoom sets the document zoom percentage.
func (d *Document) SetZoom(percent int) {
	d.IR.ZoomPercent = percent
}

// SetDefaultTabStop sets the default tab stop interval in twips.
func (d *Document) SetDefaultTabStop(twips int) {
	d.IR.DefaultTabStop = twips
}

// SetTrackChanges toggles track-changes mode.
func (d *Document) SetTrackChanges(v bool) {
	d.IR.TrackChangesEnabled = v
}

// SetEvenAndOddHeaders toggles different headers/footers for even and odd pages.
func (d *Document) SetEvenAndOddHeaders(v bool) {
	d.IR.EvenAndOddHeaders = v
}

// SetUpdateFields toggles automatic field updating on open.
func (d *Document) SetUpdateFields(v bool) {
	d.IR.UpdateFields = v
}

// SetMirrorMargins toggles mirrored page margins for facing pages.
func (d *Document) SetMirrorMargins(v bool) {
	d.IR.MirrorMargins = v
}

// SetAutoHyphenation toggles automatic hyphenation.
func (d *Document) SetAutoHyphenation(v bool) {
	d.IR.AutoHyphenation = v
}

// SetCompatibilityMode sets the Word compatibility mode version (e.g., "15").
func (d *Document) SetCompatibilityMode(mode string) {
	d.IR.CompatibilityMode = mode
}

// SetProofState sets the spelling and grammar proofing state.
// Values are "clean", "dirty", or "".
func (d *Document) SetProofState(spelling, grammar string) {
	d.IR.ProofState = &wordir.ProofState{Spelling: spelling, Grammar: grammar}
}

// SetThemeFontLang sets the theme font language values.
func (d *Document) SetThemeFontLang(val, eastAsia, bidi string) {
	d.IR.ThemeFontLang = &wordir.ThemeFontLang{Val: val, EastAsia: eastAsia, Bidi: bidi}
}

// SetFootnoteProperties sets document-wide footnote settings.
// position is "pageBottom", "belowText", or "sectEnd".
// format is the numbering format (e.g., "decimal", "lowerRoman").
// start is the starting number.
// restart is "continuous", "eachSect", or "eachPage".
func (d *Document) SetFootnoteProperties(position, format string, start int, restart string) {
	d.IR.FootnoteNumberFormat = format
	d.IR.FootnoteNumberStart = start
	d.IR.FootnoteRestart = restart
}

// SetEndnoteProperties sets document-wide endnote settings.
// format is the numbering format (e.g., "decimal", "lowerRoman").
// start is the starting number.
// restart is "continuous" or "eachSect".
func (d *Document) SetEndnoteProperties(format string, start int, restart string) {
	d.IR.EndnoteNumberFormat = format
	d.IR.EndnoteNumberStart = start
	d.IR.EndnoteRestart = restart
}

// misc returns the document's MiscSettings, lazily creating it if needed.
func (d *Document) misc() *wordir.MiscSettings {
	if d == nil || d.IR == nil {
		return nil
	}
	if d.IR.MiscSettings == nil {
		d.IR.MiscSettings = &wordir.MiscSettings{}
	}
	return d.IR.MiscSettings
}

// SetViewMode sets the document's saved view mode (e.g. "print", "web", "outline").
func (d *Document) SetViewMode(mode string) {
	d.IR.ViewMode = mode
}

// SetDefaultTableStyle sets the document-wide default table style ID.
func (d *Document) SetDefaultTableStyle(styleID string) {
	d.IR.DefaultTableStyle = styleID
}

// SetBackgroundColor sets the document page background color as a 6-digit hex string.
func (d *Document) SetBackgroundColor(hex string) {
	d.IR.BackgroundColor = hex
}

// SetDisplayBackgroundShape toggles whether the document background shape is displayed.
func (d *Document) SetDisplayBackgroundShape(v bool) {
	d.IR.DisplayBackgroundShape = &v
}

// SetDoNotDisplayPageBoundaries toggles suppression of page boundary display.
func (d *Document) SetDoNotDisplayPageBoundaries(v bool) {
	d.IR.DoNotDisplayPageBoundaries = v
}

// SetDoNotTrackFormatting toggles suppression of formatting change tracking.
func (d *Document) SetDoNotTrackFormatting(v bool) {
	d.IR.DoNotTrackFormatting = v
}

// SetDoNotTrackMoves toggles suppression of move tracking.
func (d *Document) SetDoNotTrackMoves(v bool) {
	d.IR.DoNotTrackMoves = v
}

// SetConsecutiveHyphenLimit sets the maximum number of consecutive hyphenated lines.
func (d *Document) SetConsecutiveHyphenLimit(limit int) {
	d.IR.ConsecutiveHyphenLimit = limit
}

// SetHyphenationZone sets the hyphenation zone width in twips.
func (d *Document) SetHyphenationZone(twips int) {
	d.IR.HyphenationZone = twips
}

// SetDoNotHyphenateCaps toggles suppression of hyphenation for words in ALL CAPS.
func (d *Document) SetDoNotHyphenateCaps(v bool) {
	d.IR.DoNotHyphenateCaps = v
}

// SetDocumentType sets the document classification (e.g. "letter", "email").
func (d *Document) SetDocumentType(docType string) {
	d.IR.DocumentType = docType
}

// AddCustomProperty adds a custom document property.
func (d *Document) AddCustomProperty(name, value string) {
	if d.IR.CustomProperties == nil {
		d.IR.CustomProperties = make(map[string]string)
	}
	d.IR.CustomProperties[name] = value
}

// SetVariable sets a document variable stored in settings.xml (w:docVars).
func (d *Document) SetVariable(name, value string) {
	if d.IR.DocumentVariables == nil {
		d.IR.DocumentVariables = make(map[string]string)
	}
	d.IR.DocumentVariables[name] = value
}

// AddCustomXML adds a raw custom XML datastore payload keyed by store item ID.
// The key is normalized to uppercase trimmed form to match the OOXML loader.
func (d *Document) AddCustomXML(uri string, data []byte) {
	if d.IR.CustomXMLParts == nil {
		d.IR.CustomXMLParts = make(map[string][]byte)
	}
	key := strings.ToUpper(strings.TrimSpace(uri))
	if key == "" {
		return
	}
	d.IR.CustomXMLParts[key] = append([]byte(nil), data...)
}

// CreateTableStyle creates a new table style with the given ID and name.
func (d *Document) CreateTableStyle(id, name string) *TableStyle {
	if d.IR.Styles == nil {
		d.IR.Styles = make(map[string]wordir.Style)
	}
	s := wordir.Style{
		ID:          id,
		Name:        name,
		TableFormat: &wordir.TableFormat{},
	}
	d.IR.Styles[id] = s
	return &TableStyle{doc: d, id: id}
}

// InlineField represents a single inline field found inside a run.
type InlineField struct {
	Kind     string // e.g. "PAGE", "DATE", "MERGEFIELD"
	Instr    string // raw field instruction
	Arg      string // field argument
	Anchor   string // bookmark anchor
	Target   string // hyperlink target
	Location string // "body", "header", "footer", "footnote", "endnote", "comment"
}

// Bookmarks returns every bookmark start found in the document body and all
// stories (headers, footers, footnotes, endnotes, comments).
func (d *Document) Bookmarks() []Bookmark {
	if d == nil || d.IR == nil {
		return nil
	}
	var out []Bookmark
	seen := make(map[string]bool)

	add := func(para *wordir.Paragraph, loc string) {
		if para == nil {
			return
		}
		for _, name := range para.Bookmarks {
			if !seen[name] {
				seen[name] = true
				out = append(out, Bookmark{Name: name, Location: loc})
			}
		}
	}

	// Body
	for _, b := range d.IR.Blocks {
		add(b.Paragraph, "body")
	}
	// Stories
	for _, s := range d.headerPtrs {
		for i := range s.Paragraphs {
			add(&s.Paragraphs[i], "header")
		}
	}
	for _, s := range d.footerPtrs {
		for i := range s.Paragraphs {
			add(&s.Paragraphs[i], "footer")
		}
	}
	for _, s := range d.footnotePtrs {
		for i := range s.Paragraphs {
			add(&s.Paragraphs[i], "footnote")
		}
	}
	for _, s := range d.endnotePtrs {
		for i := range s.Paragraphs {
			add(&s.Paragraphs[i], "endnote")
		}
	}
	for _, s := range d.commentPtrs {
		for i := range s.Paragraphs {
			add(&s.Paragraphs[i], "comment")
		}
	}

	return out
}

// BookmarkCount returns the total number of unique bookmarks in the document.
func (d *Document) BookmarkCount() int {
	return len(d.Bookmarks())
}

// Fields is an alias for EnumerateFields, provided for POI parity.
func (d *Document) Fields() []InlineField {
	return d.EnumerateFields()
}

// EnumerateFields scans the entire document (body, headers, footers, footnotes,
// endnotes, comments) and returns all inline fields found in runs.
func (d *Document) EnumerateFields() []InlineField {
	var out []InlineField
	add := func(r wordir.Run, loc string) {
		if r.FieldKind == "" {
			return
		}
		out = append(out, InlineField{
			Kind:     r.FieldKind,
			Instr:    r.FieldInstr,
			Arg:      r.FieldArg,
			Anchor:   r.Anchor,
			Target:   r.Target,
			Location: loc,
		})
	}

	// Body paragraphs (use Blocks to avoid stale flat-slice copies)
	for _, b := range d.IR.Blocks {
		if b.Paragraph == nil {
			continue
		}
		for _, r := range b.Paragraph.Runs {
			add(r, "body")
		}
	}

	// Headers (use pointer caches to see unflushed edits)
	for _, s := range d.headerPtrs {
		for _, p := range s.Paragraphs {
			for _, r := range p.Runs {
				add(r, "header")
			}
		}
	}

	// Footers
	for _, s := range d.footerPtrs {
		for _, p := range s.Paragraphs {
			for _, r := range p.Runs {
				add(r, "footer")
			}
		}
	}

	// Footnotes
	for _, s := range d.footnotePtrs {
		for _, p := range s.Paragraphs {
			for _, r := range p.Runs {
				add(r, "footnote")
			}
		}
	}

	// Endnotes
	for _, s := range d.endnotePtrs {
		for _, p := range s.Paragraphs {
			for _, r := range p.Runs {
				add(r, "endnote")
			}
		}
	}

	// Comments
	for _, s := range d.commentPtrs {
		for _, p := range s.Paragraphs {
			for _, r := range p.Runs {
				add(r, "comment")
			}
		}
	}

	return out
}

// SetDisplayHorizontalDrawingGrid sets the display horizontal drawing grid value.
func (d *Document) SetDisplayHorizontalDrawingGrid(v int) {
	if m := d.misc(); m != nil {
		m.DisplayHorizontalDrawingGrid = v
	}
}

// DisplayHorizontalDrawingGrid returns the display horizontal drawing grid value.
func (d *Document) DisplayHorizontalDrawingGrid() int {
	if m := d.misc(); m != nil {
		return m.DisplayHorizontalDrawingGrid
	}
	return 0
}

// SetDisplayVerticalDrawingGrid sets the display vertical drawing grid value.
func (d *Document) SetDisplayVerticalDrawingGrid(v int) {
	if m := d.misc(); m != nil {
		m.DisplayVerticalDrawingGrid = v
	}
}

// DisplayVerticalDrawingGrid returns the display vertical drawing grid value.
func (d *Document) DisplayVerticalDrawingGrid() int {
	if m := d.misc(); m != nil {
		return m.DisplayVerticalDrawingGrid
	}
	return 0
}

// SetDoNotUseMarginsForDrawingGridOrigin sets the do not use margins for drawing grid origin flag.
func (d *Document) SetDoNotUseMarginsForDrawingGridOrigin(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotUseMarginsForDrawingGridOrigin = v
	}
}

// DoNotUseMarginsForDrawingGridOrigin returns the do not use margins for drawing grid origin flag.
func (d *Document) DoNotUseMarginsForDrawingGridOrigin() bool {
	if m := d.misc(); m != nil {
		return m.DoNotUseMarginsForDrawingGridOrigin
	}
	return false
}

// SetDrawingGridHorizontalSpacing sets the drawing grid horizontal spacing value.
func (d *Document) SetDrawingGridHorizontalSpacing(v int) {
	if m := d.misc(); m != nil {
		m.DrawingGridHorizontalSpacing = v
	}
}

// DrawingGridHorizontalSpacing returns the drawing grid horizontal spacing value.
func (d *Document) DrawingGridHorizontalSpacing() int {
	if m := d.misc(); m != nil {
		return m.DrawingGridHorizontalSpacing
	}
	return 0
}

// SetDrawingGridVerticalSpacing sets the drawing grid vertical spacing value.
func (d *Document) SetDrawingGridVerticalSpacing(v int) {
	if m := d.misc(); m != nil {
		m.DrawingGridVerticalSpacing = v
	}
}

// DrawingGridVerticalSpacing returns the drawing grid vertical spacing value.
func (d *Document) DrawingGridVerticalSpacing() int {
	if m := d.misc(); m != nil {
		return m.DrawingGridVerticalSpacing
	}
	return 0
}

// SetDrawingGridHorizontalOrigin sets the drawing grid horizontal origin value.
func (d *Document) SetDrawingGridHorizontalOrigin(v int) {
	if m := d.misc(); m != nil {
		m.DrawingGridHorizontalOrigin = v
	}
}

// DrawingGridHorizontalOrigin returns the drawing grid horizontal origin value.
func (d *Document) DrawingGridHorizontalOrigin() int {
	if m := d.misc(); m != nil {
		return m.DrawingGridHorizontalOrigin
	}
	return 0
}

// SetDrawingGridVerticalOrigin sets the drawing grid vertical origin value.
func (d *Document) SetDrawingGridVerticalOrigin(v int) {
	if m := d.misc(); m != nil {
		m.DrawingGridVerticalOrigin = v
	}
}

// DrawingGridVerticalOrigin returns the drawing grid vertical origin value.
func (d *Document) DrawingGridVerticalOrigin() int {
	if m := d.misc(); m != nil {
		return m.DrawingGridVerticalOrigin
	}
	return 0
}

// SetBookFoldRevPrinting sets the book fold rev printing flag.
func (d *Document) SetBookFoldRevPrinting(v bool) {
	if m := d.misc(); m != nil {
		m.BookFoldRevPrinting = v
	}
}

// BookFoldRevPrinting returns the book fold rev printing flag.
func (d *Document) BookFoldRevPrinting() bool {
	if m := d.misc(); m != nil {
		return m.BookFoldRevPrinting
	}
	return false
}

// SetBookFoldPrintingSheets sets the book fold printing sheets value.
func (d *Document) SetBookFoldPrintingSheets(v int) {
	if m := d.misc(); m != nil {
		m.BookFoldPrintingSheets = v
	}
}

// BookFoldPrintingSheets returns the book fold printing sheets value.
func (d *Document) BookFoldPrintingSheets() int {
	if m := d.misc(); m != nil {
		return m.BookFoldPrintingSheets
	}
	return 0
}

// SetBookFoldPrinting sets the book fold printing flag.
func (d *Document) SetBookFoldPrinting(v bool) {
	if m := d.misc(); m != nil {
		m.BookFoldPrinting = v
	}
}

// BookFoldPrinting returns the book fold printing flag.
func (d *Document) BookFoldPrinting() bool {
	if m := d.misc(); m != nil {
		return m.BookFoldPrinting
	}
	return false
}

// SetPrintTwoOnOne sets the print two on one flag.
func (d *Document) SetPrintTwoOnOne(v bool) {
	if m := d.misc(); m != nil {
		m.PrintTwoOnOne = v
	}
}

// PrintTwoOnOne returns the print two on one flag.
func (d *Document) PrintTwoOnOne() bool {
	if m := d.misc(); m != nil {
		return m.PrintTwoOnOne
	}
	return false
}

// SetPrintFormsData sets the print forms data flag.
func (d *Document) SetPrintFormsData(v bool) {
	if m := d.misc(); m != nil {
		m.PrintFormsData = v
	}
}

// PrintFormsData returns the print forms data flag.
func (d *Document) PrintFormsData() bool {
	if m := d.misc(); m != nil {
		return m.PrintFormsData
	}
	return false
}

// SetPrintPostScriptOverText sets the print post script over text flag.
func (d *Document) SetPrintPostScriptOverText(v bool) {
	if m := d.misc(); m != nil {
		m.PrintPostScriptOverText = v
	}
}

// PrintPostScriptOverText returns the print post script over text flag.
func (d *Document) PrintPostScriptOverText() bool {
	if m := d.misc(); m != nil {
		return m.PrintPostScriptOverText
	}
	return false
}

// SetPrintFractionalCharacterWidth sets the print fractional character width flag.
func (d *Document) SetPrintFractionalCharacterWidth(v bool) {
	if m := d.misc(); m != nil {
		m.PrintFractionalCharacterWidth = v
	}
}

// PrintFractionalCharacterWidth returns the print fractional character width flag.
func (d *Document) PrintFractionalCharacterWidth() bool {
	if m := d.misc(); m != nil {
		return m.PrintFractionalCharacterWidth
	}
	return false
}

// SetDecimalSymbol sets the decimal symbol value.
func (d *Document) SetDecimalSymbol(v string) {
	if m := d.misc(); m != nil {
		m.DecimalSymbol = v
	}
}

// DecimalSymbol returns the decimal symbol value.
func (d *Document) DecimalSymbol() string {
	if m := d.misc(); m != nil {
		return m.DecimalSymbol
	}
	return ""
}

// SetListSeparator sets the list separator value.
func (d *Document) SetListSeparator(v string) {
	if m := d.misc(); m != nil {
		m.ListSeparator = v
	}
}

// ListSeparator returns the list separator value.
func (d *Document) ListSeparator() string {
	if m := d.misc(); m != nil {
		return m.ListSeparator
	}
	return ""
}

// SetEmbedTrueTypeFonts sets the embed true type fonts flag.
func (d *Document) SetEmbedTrueTypeFonts(v bool) {
	if m := d.misc(); m != nil {
		m.EmbedTrueTypeFonts = v
	}
}

// EmbedTrueTypeFonts returns the embed true type fonts flag.
func (d *Document) EmbedTrueTypeFonts() bool {
	if m := d.misc(); m != nil {
		return m.EmbedTrueTypeFonts
	}
	return false
}

// SetEmbedSystemFonts sets the embed system fonts flag.
func (d *Document) SetEmbedSystemFonts(v bool) {
	if m := d.misc(); m != nil {
		m.EmbedSystemFonts = v
	}
}

// EmbedSystemFonts returns the embed system fonts flag.
func (d *Document) EmbedSystemFonts() bool {
	if m := d.misc(); m != nil {
		return m.EmbedSystemFonts
	}
	return false
}

// SetSaveSubsetFonts sets the save subset fonts flag.
func (d *Document) SetSaveSubsetFonts(v bool) {
	if m := d.misc(); m != nil {
		m.SaveSubsetFonts = v
	}
}

// SaveSubsetFonts returns the save subset fonts flag.
func (d *Document) SaveSubsetFonts() bool {
	if m := d.misc(); m != nil {
		return m.SaveSubsetFonts
	}
	return false
}

// SetSaveFormsData sets the save forms data flag.
func (d *Document) SetSaveFormsData(v bool) {
	if m := d.misc(); m != nil {
		m.SaveFormsData = v
	}
}

// SaveFormsData returns the save forms data flag.
func (d *Document) SaveFormsData() bool {
	if m := d.misc(); m != nil {
		return m.SaveFormsData
	}
	return false
}

// SetShowXMLTags sets the show x m l tags flag.
func (d *Document) SetShowXMLTags(v bool) {
	if m := d.misc(); m != nil {
		m.ShowXMLTags = v
	}
}

// ShowXMLTags returns the show x m l tags flag.
func (d *Document) ShowXMLTags() bool {
	if m := d.misc(); m != nil {
		return m.ShowXMLTags
	}
	return false
}

// SetHideSpellingErrors sets the hide spelling errors flag.
func (d *Document) SetHideSpellingErrors(v bool) {
	if m := d.misc(); m != nil {
		m.HideSpellingErrors = v
	}
}

// HideSpellingErrors returns the hide spelling errors flag.
func (d *Document) HideSpellingErrors() bool {
	if m := d.misc(); m != nil {
		return m.HideSpellingErrors
	}
	return false
}

// SetHideGrammaticalErrors sets the hide grammatical errors flag.
func (d *Document) SetHideGrammaticalErrors(v bool) {
	if m := d.misc(); m != nil {
		m.HideGrammaticalErrors = v
	}
}

// HideGrammaticalErrors returns the hide grammatical errors flag.
func (d *Document) HideGrammaticalErrors() bool {
	if m := d.misc(); m != nil {
		return m.HideGrammaticalErrors
	}
	return false
}

// SetFormsDesign sets the forms design flag.
func (d *Document) SetFormsDesign(v bool) {
	if m := d.misc(); m != nil {
		m.FormsDesign = v
	}
}

// FormsDesign returns the forms design flag.
func (d *Document) FormsDesign() bool {
	if m := d.misc(); m != nil {
		return m.FormsDesign
	}
	return false
}

// SetLinkStyles sets the link styles flag.
func (d *Document) SetLinkStyles(v bool) {
	if m := d.misc(); m != nil {
		m.LinkStyles = v
	}
}

// LinkStyles returns the link styles flag.
func (d *Document) LinkStyles() bool {
	if m := d.misc(); m != nil {
		return m.LinkStyles
	}
	return false
}

// SetRevisionView sets the revision view flag.
func (d *Document) SetRevisionView(v bool) {
	if m := d.misc(); m != nil {
		m.RevisionView = v
	}
}

// RevisionView returns the revision view flag.
func (d *Document) RevisionView() bool {
	if m := d.misc(); m != nil {
		return m.RevisionView
	}
	return false
}

// SetStyleLockTheme sets the style lock theme flag.
func (d *Document) SetStyleLockTheme(v bool) {
	if m := d.misc(); m != nil {
		m.StyleLockTheme = v
	}
}

// StyleLockTheme returns the style lock theme flag.
func (d *Document) StyleLockTheme() bool {
	if m := d.misc(); m != nil {
		return m.StyleLockTheme
	}
	return false
}

// SetStyleLockQFSet sets the style lock q f set flag.
func (d *Document) SetStyleLockQFSet(v bool) {
	if m := d.misc(); m != nil {
		m.StyleLockQFSet = v
	}
}

// StyleLockQFSet returns the style lock q f set flag.
func (d *Document) StyleLockQFSet() bool {
	if m := d.misc(); m != nil {
		return m.StyleLockQFSet
	}
	return false
}

// SetShowEnvelope sets the show envelope flag.
func (d *Document) SetShowEnvelope(v bool) {
	if m := d.misc(); m != nil {
		m.ShowEnvelope = v
	}
}

// ShowEnvelope returns the show envelope flag.
func (d *Document) ShowEnvelope() bool {
	if m := d.misc(); m != nil {
		return m.ShowEnvelope
	}
	return false
}

// SetStrictFirstAndLastChars sets the strict first and last chars flag.
func (d *Document) SetStrictFirstAndLastChars(v bool) {
	if m := d.misc(); m != nil {
		m.StrictFirstAndLastChars = v
	}
}

// StrictFirstAndLastChars returns the strict first and last chars flag.
func (d *Document) StrictFirstAndLastChars() bool {
	if m := d.misc(); m != nil {
		return m.StrictFirstAndLastChars
	}
	return false
}

// SetDoNotShadeFormData sets the do not shade form data flag.
func (d *Document) SetDoNotShadeFormData(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotShadeFormData = v
	}
}

// DoNotShadeFormData returns the do not shade form data flag.
func (d *Document) DoNotShadeFormData() bool {
	if m := d.misc(); m != nil {
		return m.DoNotShadeFormData
	}
	return false
}

// SetNoPunctuationKerning sets the no punctuation kerning flag.
func (d *Document) SetNoPunctuationKerning(v bool) {
	if m := d.misc(); m != nil {
		m.NoPunctuationKerning = v
	}
}

// NoPunctuationKerning returns the no punctuation kerning flag.
func (d *Document) NoPunctuationKerning() bool {
	if m := d.misc(); m != nil {
		return m.NoPunctuationKerning
	}
	return false
}

// SetAlignBordersAndEdges sets the align borders and edges flag.
func (d *Document) SetAlignBordersAndEdges(v bool) {
	if m := d.misc(); m != nil {
		m.AlignBordersAndEdges = v
	}
}

// AlignBordersAndEdges returns the align borders and edges flag.
func (d *Document) AlignBordersAndEdges() bool {
	if m := d.misc(); m != nil {
		return m.AlignBordersAndEdges
	}
	return false
}

// SetGutterAtTop sets the gutter at top flag.
func (d *Document) SetGutterAtTop(v bool) {
	if m := d.misc(); m != nil {
		m.GutterAtTop = v
	}
}

// GutterAtTop returns the gutter at top flag.
func (d *Document) GutterAtTop() bool {
	if m := d.misc(); m != nil {
		return m.GutterAtTop
	}
	return false
}

// SetDoNotValidateAgainstSchema sets the do not validate against schema flag.
func (d *Document) SetDoNotValidateAgainstSchema(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotValidateAgainstSchema = v
	}
}

// DoNotValidateAgainstSchema returns the do not validate against schema flag.
func (d *Document) DoNotValidateAgainstSchema() bool {
	if m := d.misc(); m != nil {
		return m.DoNotValidateAgainstSchema
	}
	return false
}

// SetSaveInvalidXml sets the save invalid xml flag.
func (d *Document) SetSaveInvalidXml(v bool) {
	if m := d.misc(); m != nil {
		m.SaveInvalidXml = v
	}
}

// SaveInvalidXml returns the save invalid xml flag.
func (d *Document) SaveInvalidXml() bool {
	if m := d.misc(); m != nil {
		return m.SaveInvalidXml
	}
	return false
}

// SetIgnoreMixedContent sets the ignore mixed content flag.
func (d *Document) SetIgnoreMixedContent(v bool) {
	if m := d.misc(); m != nil {
		m.IgnoreMixedContent = v
	}
}

// IgnoreMixedContent returns the ignore mixed content flag.
func (d *Document) IgnoreMixedContent() bool {
	if m := d.misc(); m != nil {
		return m.IgnoreMixedContent
	}
	return false
}

// SetAlwaysShowPlaceholderText sets the always show placeholder text flag.
func (d *Document) SetAlwaysShowPlaceholderText(v bool) {
	if m := d.misc(); m != nil {
		m.AlwaysShowPlaceholderText = v
	}
}

// AlwaysShowPlaceholderText returns the always show placeholder text flag.
func (d *Document) AlwaysShowPlaceholderText() bool {
	if m := d.misc(); m != nil {
		return m.AlwaysShowPlaceholderText
	}
	return false
}

// SetDoNotDemarcateInvalidXml sets the do not demarcate invalid xml flag.
func (d *Document) SetDoNotDemarcateInvalidXml(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotDemarcateInvalidXml = v
	}
}

// DoNotDemarcateInvalidXml returns the do not demarcate invalid xml flag.
func (d *Document) DoNotDemarcateInvalidXml() bool {
	if m := d.misc(); m != nil {
		return m.DoNotDemarcateInvalidXml
	}
	return false
}

// SetSaveXmlDataOnly sets the save xml data only flag.
func (d *Document) SetSaveXmlDataOnly(v bool) {
	if m := d.misc(); m != nil {
		m.SaveXmlDataOnly = v
	}
}

// SaveXmlDataOnly returns the save xml data only flag.
func (d *Document) SaveXmlDataOnly() bool {
	if m := d.misc(); m != nil {
		return m.SaveXmlDataOnly
	}
	return false
}

// SetUseXSLTWhenSaving sets the use x s l t when saving flag.
func (d *Document) SetUseXSLTWhenSaving(v bool) {
	if m := d.misc(); m != nil {
		m.UseXSLTWhenSaving = v
	}
}

// UseXSLTWhenSaving returns the use x s l t when saving flag.
func (d *Document) UseXSLTWhenSaving() bool {
	if m := d.misc(); m != nil {
		return m.UseXSLTWhenSaving
	}
	return false
}

// SetAlwaysMergeEmptyNamespace sets the always merge empty namespace flag.
func (d *Document) SetAlwaysMergeEmptyNamespace(v bool) {
	if m := d.misc(); m != nil {
		m.AlwaysMergeEmptyNamespace = v
	}
}

// AlwaysMergeEmptyNamespace returns the always merge empty namespace flag.
func (d *Document) AlwaysMergeEmptyNamespace() bool {
	if m := d.misc(); m != nil {
		return m.AlwaysMergeEmptyNamespace
	}
	return false
}

// SetDoNotIncludeSubdocsInStats sets the do not include subdocs in stats flag.
func (d *Document) SetDoNotIncludeSubdocsInStats(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotIncludeSubdocsInStats = v
	}
}

// DoNotIncludeSubdocsInStats returns the do not include subdocs in stats flag.
func (d *Document) DoNotIncludeSubdocsInStats() bool {
	if m := d.misc(); m != nil {
		return m.DoNotIncludeSubdocsInStats
	}
	return false
}

// SetDoNotAutoCompressPictures sets the do not auto compress pictures flag.
func (d *Document) SetDoNotAutoCompressPictures(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotAutoCompressPictures = v
	}
}

// DoNotAutoCompressPictures returns the do not auto compress pictures flag.
func (d *Document) DoNotAutoCompressPictures() bool {
	if m := d.misc(); m != nil {
		return m.DoNotAutoCompressPictures
	}
	return false
}

// SetDoNotEmbedSmartTags sets the do not embed smart tags flag.
func (d *Document) SetDoNotEmbedSmartTags(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotEmbedSmartTags = v
	}
}

// DoNotEmbedSmartTags returns the do not embed smart tags flag.
func (d *Document) DoNotEmbedSmartTags() bool {
	if m := d.misc(); m != nil {
		return m.DoNotEmbedSmartTags
	}
	return false
}

// SetForceUpgrade sets the force upgrade flag.
func (d *Document) SetForceUpgrade(v bool) {
	if m := d.misc(); m != nil {
		m.ForceUpgrade = v
	}
}

// ForceUpgrade returns the force upgrade flag.
func (d *Document) ForceUpgrade() bool {
	if m := d.misc(); m != nil {
		return m.ForceUpgrade
	}
	return false
}

// SetActive sets the active flag.
func (d *Document) SetActive(v bool) {
	if m := d.misc(); m != nil {
		m.Active = v
	}
}

// Active returns the active flag.
func (d *Document) Active() bool {
	if m := d.misc(); m != nil {
		return m.Active
	}
	return false
}

// SetBodyDiv sets the body div flag.
func (d *Document) SetBodyDiv(v bool) {
	if m := d.misc(); m != nil {
		m.BodyDiv = v
	}
}

// BodyDiv returns the body div flag.
func (d *Document) BodyDiv() bool {
	if m := d.misc(); m != nil {
		return m.BodyDiv
	}
	return false
}

// SetCachedColBalance sets the cached col balance flag.
func (d *Document) SetCachedColBalance(v bool) {
	if m := d.misc(); m != nil {
		m.CachedColBalance = v
	}
}

// CachedColBalance returns the cached col balance flag.
func (d *Document) CachedColBalance() bool {
	if m := d.misc(); m != nil {
		return m.CachedColBalance
	}
	return false
}

// SetDoNotSnapToGridInCell sets the do not snap to grid in cell flag.
func (d *Document) SetDoNotSnapToGridInCell(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotSnapToGridInCell = v
	}
}

// DoNotSnapToGridInCell returns the do not snap to grid in cell flag.
func (d *Document) DoNotSnapToGridInCell() bool {
	if m := d.misc(); m != nil {
		return m.DoNotSnapToGridInCell
	}
	return false
}

// SetDoNotSuppressIndentation sets the do not suppress indentation flag.
func (d *Document) SetDoNotSuppressIndentation(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotSuppressIndentation = v
	}
}

// DoNotSuppressIndentation returns the do not suppress indentation flag.
func (d *Document) DoNotSuppressIndentation() bool {
	if m := d.misc(); m != nil {
		return m.DoNotSuppressIndentation
	}
	return false
}

// SetDoNotSuppressParagraphBorders sets the do not suppress paragraph borders flag.
func (d *Document) SetDoNotSuppressParagraphBorders(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotSuppressParagraphBorders = v
	}
}

// DoNotSuppressParagraphBorders returns the do not suppress paragraph borders flag.
func (d *Document) DoNotSuppressParagraphBorders() bool {
	if m := d.misc(); m != nil {
		return m.DoNotSuppressParagraphBorders
	}
	return false
}

// SetDoNotUseEastAsianBreakRules sets the do not use east asian break rules flag.
func (d *Document) SetDoNotUseEastAsianBreakRules(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotUseEastAsianBreakRules = v
	}
}

// DoNotUseEastAsianBreakRules returns the do not use east asian break rules flag.
func (d *Document) DoNotUseEastAsianBreakRules() bool {
	if m := d.misc(); m != nil {
		return m.DoNotUseEastAsianBreakRules
	}
	return false
}

// SetDoNotUseHTMLParagraphAutoSpacing sets the do not use h t m l paragraph auto spacing flag.
func (d *Document) SetDoNotUseHTMLParagraphAutoSpacing(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotUseHTMLParagraphAutoSpacing = v
	}
}

// DoNotUseHTMLParagraphAutoSpacing returns the do not use h t m l paragraph auto spacing flag.
func (d *Document) DoNotUseHTMLParagraphAutoSpacing() bool {
	if m := d.misc(); m != nil {
		return m.DoNotUseHTMLParagraphAutoSpacing
	}
	return false
}

// SetDoNotWrapTextWithPunct sets the do not wrap text with punct flag.
func (d *Document) SetDoNotWrapTextWithPunct(v bool) {
	if m := d.misc(); m != nil {
		m.DoNotWrapTextWithPunct = v
	}
}

// DoNotWrapTextWithPunct returns the do not wrap text with punct flag.
func (d *Document) DoNotWrapTextWithPunct() bool {
	if m := d.misc(); m != nil {
		return m.DoNotWrapTextWithPunct
	}
	return false
}

// SetGrowAutofit sets the grow autofit flag.
func (d *Document) SetGrowAutofit(v bool) {
	if m := d.misc(); m != nil {
		m.GrowAutofit = v
	}
}

// GrowAutofit returns the grow autofit flag.
func (d *Document) GrowAutofit() bool {
	if m := d.misc(); m != nil {
		return m.GrowAutofit
	}
	return false
}

// SetLvlRestart sets the lvl restart value.
func (d *Document) SetLvlRestart(v int) {
	if m := d.misc(); m != nil {
		m.LvlRestart = v
	}
}

// LvlRestart returns the lvl restart value.
func (d *Document) LvlRestart() int {
	if m := d.misc(); m != nil {
		return m.LvlRestart
	}
	return 0
}

// SetNumIDMacAtCleanup sets the num i d mac at cleanup value.
func (d *Document) SetNumIDMacAtCleanup(v int) {
	if m := d.misc(); m != nil {
		m.NumIDMacAtCleanup = v
	}
}

// NumIDMacAtCleanup returns the num i d mac at cleanup value.
func (d *Document) NumIDMacAtCleanup() int {
	if m := d.misc(); m != nil {
		return m.NumIDMacAtCleanup
	}
	return 0
}

// SetSaveSmartTagsAsXml sets the save smart tags as xml flag.
func (d *Document) SetSaveSmartTagsAsXml(v bool) {
	if m := d.misc(); m != nil {
		m.SaveSmartTagsAsXml = v
	}
}

// SaveSmartTagsAsXml returns the save smart tags as xml flag.
func (d *Document) SaveSmartTagsAsXml() bool {
	if m := d.misc(); m != nil {
		return m.SaveSmartTagsAsXml
	}
	return false
}

// SetSelectFldWithFirstOrLastChar sets the select fld with first or last char flag.
func (d *Document) SetSelectFldWithFirstOrLastChar(v bool) {
	if m := d.misc(); m != nil {
		m.SelectFldWithFirstOrLastChar = v
	}
}

// SelectFldWithFirstOrLastChar returns the select fld with first or last char flag.
func (d *Document) SelectFldWithFirstOrLastChar() bool {
	if m := d.misc(); m != nil {
		return m.SelectFldWithFirstOrLastChar
	}
	return false
}

// SetSplitPgBreakAndParaMark sets the split pg break and para mark flag.
func (d *Document) SetSplitPgBreakAndParaMark(v bool) {
	if m := d.misc(); m != nil {
		m.SplitPgBreakAndParaMark = v
	}
}

// SplitPgBreakAndParaMark returns the split pg break and para mark flag.
func (d *Document) SplitPgBreakAndParaMark() bool {
	if m := d.misc(); m != nil {
		return m.SplitPgBreakAndParaMark
	}
	return false
}

// SetStyleLink sets the style link value.
func (d *Document) SetStyleLink(v string) {
	if m := d.misc(); m != nil {
		m.StyleLink = v
	}
}

// StyleLink returns the style link value.
func (d *Document) StyleLink() string {
	if m := d.misc(); m != nil {
		return m.StyleLink
	}
	return ""
}

// SetSuppressBottomSpacing sets the suppress bottom spacing flag.
func (d *Document) SetSuppressBottomSpacing(v bool) {
	if m := d.misc(); m != nil {
		m.SuppressBottomSpacing = v
	}
}

// SuppressBottomSpacing returns the suppress bottom spacing flag.
func (d *Document) SuppressBottomSpacing() bool {
	if m := d.misc(); m != nil {
		return m.SuppressBottomSpacing
	}
	return false
}

// SetSuppressOverlap sets the suppress overlap flag.
func (d *Document) SetSuppressOverlap(v bool) {
	if m := d.misc(); m != nil {
		m.SuppressOverlap = v
	}
}

// SuppressOverlap returns the suppress overlap flag.
func (d *Document) SuppressOverlap() bool {
	if m := d.misc(); m != nil {
		return m.SuppressOverlap
	}
	return false
}

// SetSuppressSpacingAtTopOfPage sets the suppress spacing at top of page flag.
func (d *Document) SetSuppressSpacingAtTopOfPage(v bool) {
	if m := d.misc(); m != nil {
		m.SuppressSpacingAtTopOfPage = v
	}
}

// SuppressSpacingAtTopOfPage returns the suppress spacing at top of page flag.
func (d *Document) SuppressSpacingAtTopOfPage() bool {
	if m := d.misc(); m != nil {
		return m.SuppressSpacingAtTopOfPage
	}
	return false
}

// SetSuppressTopSpacing sets the suppress top spacing flag.
func (d *Document) SetSuppressTopSpacing(v bool) {
	if m := d.misc(); m != nil {
		m.SuppressTopSpacing = v
	}
}

// SuppressTopSpacing returns the suppress top spacing flag.
func (d *Document) SuppressTopSpacing() bool {
	if m := d.misc(); m != nil {
		return m.SuppressTopSpacing
	}
	return false
}

// SetSuppressTopSpacingWP sets the suppress top spacing w p flag.
func (d *Document) SetSuppressTopSpacingWP(v bool) {
	if m := d.misc(); m != nil {
		m.SuppressTopSpacingWP = v
	}
}

// SuppressTopSpacingWP returns the suppress top spacing w p flag.
func (d *Document) SuppressTopSpacingWP() bool {
	if m := d.misc(); m != nil {
		return m.SuppressTopSpacingWP
	}
	return false
}

// SetSwapBordersFacingPages sets the swap borders facing pages flag.
func (d *Document) SetSwapBordersFacingPages(v bool) {
	if m := d.misc(); m != nil {
		m.SwapBordersFacingPages = v
	}
}

// SwapBordersFacingPages returns the swap borders facing pages flag.
func (d *Document) SwapBordersFacingPages() bool {
	if m := d.misc(); m != nil {
		return m.SwapBordersFacingPages
	}
	return false
}

// SetTextboxTightWrap sets the textbox tight wrap flag.
func (d *Document) SetTextboxTightWrap(v bool) {
	if m := d.misc(); m != nil {
		m.TextboxTightWrap = v
	}
}

// TextboxTightWrap returns the textbox tight wrap flag.
func (d *Document) TextboxTightWrap() bool {
	if m := d.misc(); m != nil {
		return m.TextboxTightWrap
	}
	return false
}

// SetUnderlineTabInNumList sets the underline tab in num list flag.
func (d *Document) SetUnderlineTabInNumList(v bool) {
	if m := d.misc(); m != nil {
		m.UnderlineTabInNumList = v
	}
}

// UnderlineTabInNumList returns the underline tab in num list flag.
func (d *Document) UnderlineTabInNumList() bool {
	if m := d.misc(); m != nil {
		return m.UnderlineTabInNumList
	}
	return false
}

// SetUseAnsiKerningPairs sets the use ansi kerning pairs flag.
func (d *Document) SetUseAnsiKerningPairs(v bool) {
	if m := d.misc(); m != nil {
		m.UseAnsiKerningPairs = v
	}
}

// UseAnsiKerningPairs returns the use ansi kerning pairs flag.
func (d *Document) UseAnsiKerningPairs() bool {
	if m := d.misc(); m != nil {
		return m.UseAnsiKerningPairs
	}
	return false
}

// SetUsePrinterMetrics sets the use printer metrics flag.
func (d *Document) SetUsePrinterMetrics(v bool) {
	if m := d.misc(); m != nil {
		m.UsePrinterMetrics = v
	}
}

// UsePrinterMetrics returns the use printer metrics flag.
func (d *Document) UsePrinterMetrics() bool {
	if m := d.misc(); m != nil {
		return m.UsePrinterMetrics
	}
	return false
}

// SetUseSingleBorderforContiguousCells sets the use single borderfor contiguous cells flag.
func (d *Document) SetUseSingleBorderforContiguousCells(v bool) {
	if m := d.misc(); m != nil {
		m.UseSingleBorderforContiguousCells = v
	}
}

// UseSingleBorderforContiguousCells returns the use single borderfor contiguous cells flag.
func (d *Document) UseSingleBorderforContiguousCells() bool {
	if m := d.misc(); m != nil {
		return m.UseSingleBorderforContiguousCells
	}
	return false
}

// SetStylePaneFormatFilter sets the style pane format filter value.
func (d *Document) SetStylePaneFormatFilter(v string) {
	if m := d.misc(); m != nil {
		m.StylePaneFormatFilter = v
	}
}

// StylePaneFormatFilter returns the style pane format filter value.
func (d *Document) StylePaneFormatFilter() string {
	if m := d.misc(); m != nil {
		return m.StylePaneFormatFilter
	}
	return ""
}

// SetStylePaneSortMethod sets the style pane sort method value.
func (d *Document) SetStylePaneSortMethod(v string) {
	if m := d.misc(); m != nil {
		m.StylePaneSortMethod = v
	}
}

// StylePaneSortMethod returns the style pane sort method value.
func (d *Document) StylePaneSortMethod() string {
	if m := d.misc(); m != nil {
		return m.StylePaneSortMethod
	}
	return ""
}
