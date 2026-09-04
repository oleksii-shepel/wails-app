package edit

import wordir "office/pkg/word/ir"

// StoryEditor edits a sub-story (header, footer, footnote, endnote, comment).
type StoryEditor struct {
	story *wordir.Story
	doc   *Document
}

// AddParagraph appends a paragraph to the story.
func (se *StoryEditor) AddParagraph() *Paragraph {
	if se.story == nil {
		return nil
	}
	p := wordir.Paragraph{}
	se.story.Paragraphs = append(se.story.Paragraphs, p)
	se.story.Blocks = append(se.story.Blocks, wordir.Block{
		Index:     len(se.story.Blocks),
		Kind:      "paragraph",
		Paragraph: &se.story.Paragraphs[len(se.story.Paragraphs)-1],
	})
	return &Paragraph{ir: &se.story.Paragraphs[len(se.story.Paragraphs)-1], doc: se.doc}
}

// Paragraphs returns all paragraphs in the story.
func (se *StoryEditor) Paragraphs() []*Paragraph {
	if se.story == nil {
		return nil
	}
	out := make([]*Paragraph, len(se.story.Paragraphs))
	for i := range se.story.Paragraphs {
		out[i] = &Paragraph{ir: &se.story.Paragraphs[i], doc: se.doc}
	}
	return out
}

// AddTable appends a table to the story.
func (se *StoryEditor) AddTable() *Table {
	if se.story == nil {
		return nil
	}
	t := wordir.Table{}
	se.story.Tables = append(se.story.Tables, t)
	se.story.Blocks = append(se.story.Blocks, wordir.Block{
		Index: len(se.story.Blocks),
		Kind:  "table",
		Table: &se.story.Tables[len(se.story.Tables)-1],
	})
	return &Table{ir: &se.story.Tables[len(se.story.Tables)-1], doc: se.doc}
}

// SetAuthor sets the story author (used for comments).
func (se *StoryEditor) SetAuthor(v string) {
	if se.story != nil {
		se.story.Author = v
	}
}

// SetDate sets the story date (used for comments, YYYY-MM-DD).
func (se *StoryEditor) SetDate(v string) {
	if se.story != nil {
		se.story.Date = v
	}
}

// AddStoryField appends a new block-spanning field to the story and returns a
// StoryField editor for it.
func (se *StoryEditor) AddStoryField(kind, instr string) *StoryField {
	if se.story == nil {
		return nil
	}
	sf := wordir.StoryField{Kind: kind, Instr: instr}
	se.story.Fields = append(se.story.Fields, sf)
	return &StoryField{ir: &se.story.Fields[len(se.story.Fields)-1], doc: se.doc, story: se.story}
}

// StoryFields returns all story-spanning fields in the story.
func (se *StoryEditor) StoryFields() []*StoryField {
	if se.story == nil {
		return nil
	}
	out := make([]*StoryField, len(se.story.Fields))
	for i := range se.story.Fields {
		out[i] = &StoryField{ir: &se.story.Fields[i], doc: se.doc, story: se.story}
	}
	return out
}