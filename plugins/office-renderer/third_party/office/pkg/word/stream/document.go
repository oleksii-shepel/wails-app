package stream

import (
	"bytes"
	"io"
	"os"

	wordir "office/pkg/word/ir"
)

// NewDocumentSession creates a streaming semantic document sink without
// incremental pagination. This is intended for full-document renderers that
// still consume word/ir.Document but should not materialize the format-specific
// parser model first.
func NewDocumentSession() *LayoutSession {
	return &LayoutSession{}
}

// ParsePath parses a DOCX-family file into shared semantic Word IR.
func ParsePath(path string, opts DOCXOptions) (*wordir.Document, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	return ParseReaderAt(file, info.Size(), opts)
}

// ParseBytes parses an in-memory DOCX-family payload into shared semantic Word IR.
func ParseBytes(data []byte, opts DOCXOptions) (*wordir.Document, error) {
	return ParseReaderAt(bytes.NewReader(data), int64(len(data)), opts)
}

// ParseReaderAt streams a DOCX into the shared semantic Word IR without
// using the materialized OOXML parts/resolver pipeline. The returned document
// is suitable for the existing renderer/edit-neutral consumers.
func ParseReaderAt(r io.ReaderAt, size int64, opts DOCXOptions) (*wordir.Document, error) {
	session := NewDocumentSession()
	source := NewDOCXSource(r, size, opts)
	if err := source.Walk(session); err != nil {
		return nil, err
	}
	doc := session.Document()
	finalizeStreamedDocument(doc)
	return doc, nil
}

// ReadDOCDocument streams an in-memory legacy binary DOC through the shared
// semantic event sink.
func ReadDOCDocument(data []byte) (*wordir.Document, error) {
	session := NewDocumentSession()
	source := NewDOCSource(data)
	if err := source.Walk(session); err != nil {
		return nil, err
	}
	doc := session.Document()
	finalizeStreamedDocument(doc)
	return doc, nil
}

// finalizeStreamedDocument closes the gaps between the streaming pipeline and
// the materialized resolver: it assigns document-global paragraph ordinals to
// every story (the body is already ordinalized by the layout session; note,
// header and textbox stories are not) and applies Word's default note number
// formats when settings.xml carried no explicit override.
func finalizeStreamedDocument(doc *wordir.Document) {
	if doc == nil {
		return
	}
	next := len(doc.Paragraphs)
	assign := func(p *wordir.Paragraph) {
		if p == nil {
			return
		}
		p.Index = next
		next++
	}
	var walkBlocks func(blocks []wordir.Block)
	walkBlocks = func(blocks []wordir.Block) {
		for i := range blocks {
			block := &blocks[i]
			switch {
			case block.Paragraph != nil:
				assign(block.Paragraph)
			case block.Table != nil:
				for ri := range block.Table.Rows {
					for ci := range block.Table.Rows[ri].Cells {
						walkBlocks(block.Table.Rows[ri].Cells[ci].Blocks)
					}
				}
			}
		}
	}
	var walkStory func(story *wordir.Story)
	walkStory = func(story *wordir.Story) {
		if story == nil {
			return
		}
		for i := range story.Blocks {
			block := &story.Blocks[i]
			switch {
			case block.Paragraph != nil:
				assign(block.Paragraph)
			case block.Table != nil:
				for ri := range block.Table.Rows {
					for ci := range block.Table.Rows[ri].Cells {
						walkBlocks(block.Table.Rows[ri].Cells[ci].Blocks)
					}
				}
			}
		}
		// Stories without a Blocks projection still carry Paragraphs.
		if len(story.Blocks) == 0 {
			for i := range story.Paragraphs {
				assign(&story.Paragraphs[i])
			}
		}
	}
	// Textbox stories live inside body runs; walk them as part of the body pass.
	var walkRuns func(runs []wordir.Run)
	walkRuns = func(runs []wordir.Run) {
		for i := range runs {
			if runs[i].TextBox != nil {
				walkBlocks(runs[i].TextBox.Blocks)
			}
		}
	}
	var walkBodyTextboxes func(blocks []wordir.Block)
	walkBodyTextboxes = func(blocks []wordir.Block) {
		for i := range blocks {
			block := &blocks[i]
			switch {
			case block.Paragraph != nil:
				walkRuns(block.Paragraph.Runs)
			case block.Table != nil:
				for ri := range block.Table.Rows {
					for ci := range block.Table.Rows[ri].Cells {
						walkBodyTextboxes(block.Table.Rows[ri].Cells[ci].Blocks)
					}
				}
			}
		}
	}
	walkBodyTextboxes(doc.Blocks)
	// Body paragraphs already carry ordinals from the layout session; keep the
	// doc.Paragraphs copies consistent with the block pointers by position.
	for i := range doc.Paragraphs {
		doc.Paragraphs[i].Index = i
	}
	for name := range doc.Headers {
		story := doc.Headers[name]
		walkStory(&story)
		doc.Headers[name] = story
	}
	for name := range doc.Footers {
		story := doc.Footers[name]
		walkStory(&story)
		doc.Footers[name] = story
	}
	for id := range doc.Footnotes {
		story := doc.Footnotes[id]
		walkStory(&story)
		doc.Footnotes[id] = story
	}
	for id := range doc.Endnotes {
		story := doc.Endnotes[id]
		walkStory(&story)
		doc.Endnotes[id] = story
	}
	for id := range doc.Comments {
		story := doc.Comments[id]
		walkStory(&story)
		doc.Comments[id] = story
	}
	// Word defaults: footnotes count in decimal, endnotes in lower-roman.
	// settings.xml overrides already land in these fields when present.
	if doc.FootnoteNumberFormat == "" {
		doc.FootnoteNumberFormat = "decimal"
	}
	if doc.EndnoteNumberFormat == "" {
		doc.EndnoteNumberFormat = "lowerRoman"
	}
}

// ReadDOCFileDocument streams a legacy DOC directly from a file-backed CFB
// container, avoiding whole-file materialization before semantic parsing.
func ReadDOCFileDocument(path string) (*wordir.Document, error) {
	session := NewDocumentSession()
	source := NewDOCFileSource(path)
	if err := source.Walk(session); err != nil {
		return nil, err
	}
	return session.Document(), nil
}
