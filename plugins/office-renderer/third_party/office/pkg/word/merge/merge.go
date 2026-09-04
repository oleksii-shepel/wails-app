package merge

import (
	"fmt"
	"regexp"
	"strings"

	"office/pkg/word/ir"
)

// ---------------------------------------------------------------------------
// Data source
// ---------------------------------------------------------------------------

// Data is a single record of field name → value mappings.
type Data map[string]string

// Record is an alias for Data, used in multi-record contexts.
type Record = Data

// ---------------------------------------------------------------------------
// Processor
// ---------------------------------------------------------------------------

// Processor finds and replaces merge fields in a resolved document.
type Processor struct {
	data        Data
	records     []Record
	recordIndex int
	// PlaceholderPattern matches text-based placeholders like {{Name}}.
	// Defaults to matching {{fieldName}} and «fieldName».
	PlaceholderPattern *regexp.Regexp
	// OnMissingField is called when a merge field has no matching data.
	// If nil, the field's cached text or placeholder is preserved.
	OnMissingField func(fieldName string) string
	// PreserveFields, when true, keeps the original MERGEFIELD runs
	// instead of replacing them with plain text. The field text is
	// updated but the field structure is preserved.
	PreserveFields bool
}

// NewProcessor creates a mail-merge processor with the given data.
func NewProcessor(data Data) *Processor {
	return &Processor{
		data:               data,
		PlaceholderPattern: regexp.MustCompile(`(?:\{\{|\xab)([^}\xbb]+)(?:\}\}|\xbb)`),
	}
}

// NewProcessorWithRecords creates a processor for multi-record mail merge.
func NewProcessorWithRecords(records []Record) *Processor {
	p := NewProcessor(nil)
	p.records = records
	return p
}

// SetRecordIndex sets the current record index for multi-record processing.
func (p *Processor) SetRecordIndex(index int) {
	p.recordIndex = index
}

// CurrentRecord returns the active data record.
func (p *Processor) CurrentRecord() Data {
	if p.recordIndex >= 0 && p.recordIndex < len(p.records) {
		return p.records[p.recordIndex]
	}
	return p.data
}

// lookup returns the value for a field name from the current record.
func (p *Processor) lookup(name string) (string, bool) {
	record := p.CurrentRecord()
	if record == nil {
		return "", false
	}
	value, ok := record[name]
	return value, ok
}

// ---------------------------------------------------------------------------
// Process document
// ---------------------------------------------------------------------------

// Process replaces all merge fields in doc with values from the data source.
// Returns a new document; the original is not modified.
func (p *Processor) Process(doc *ir.Document) (*ir.Document, error) {
	if doc == nil {
		return nil, fmt.Errorf("merge: document is nil")
	}

	// Deep copy the document so we don't modify the original.
	merged := deepCopyDocument(doc)

	// Process all blocks in the document body.
	merged.Blocks = p.processBlocks(merged.Blocks)

	// Process headers.
	for key, story := range merged.Headers {
		merged.Headers[key] = p.processStory(story)
	}

	// Process footers.
	for key, story := range merged.Footers {
		merged.Footers[key] = p.processStory(story)
	}

	// Process footnotes.
	for key, story := range merged.Footnotes {
		merged.Footnotes[key] = p.processStory(story)
	}

	// Process endnotes.
	for key, story := range merged.Endnotes {
		merged.Endnotes[key] = p.processStory(story)
	}

	// Process comments.
	for key, story := range merged.Comments {
		merged.Comments[key] = p.processStory(story)
	}

	return merged, nil
}

// processStory processes all blocks in a story (header, footer, etc.).
func (p *Processor) processStory(story ir.Story) ir.Story {
	story.Blocks = p.processBlocks(story.Blocks)
	for i := range story.Paragraphs {
		story.Paragraphs[i] = p.processParagraph(story.Paragraphs[i])
	}
	return story
}

// processBlocks processes a slice of blocks.
func (p *Processor) processBlocks(blocks []ir.Block) []ir.Block {
	for i := range blocks {
		switch blocks[i].Kind {
		case "paragraph":
			if blocks[i].Paragraph != nil {
				*blocks[i].Paragraph = p.processParagraph(*blocks[i].Paragraph)
			}
		case "table":
			if blocks[i].Table != nil {
				*blocks[i].Table = p.processTable(*blocks[i].Table)
			}
		}
	}
	return blocks
}

// processTable processes all cells in a table.
func (p *Processor) processTable(table ir.Table) ir.Table {
	for i := range table.Rows {
		for j := range table.Rows[i].Cells {
			table.Rows[i].Cells[j].Blocks = p.processBlocks(table.Rows[i].Cells[j].Blocks)
		}
	}
	return table
}

// processParagraph processes all runs in a paragraph.
func (p *Processor) processParagraph(para ir.Paragraph) ir.Paragraph {
	para.Runs = p.processRuns(para.Runs)
	// Also process text in the paragraph-level Text field.
	para.Text = p.replacePlaceholders(para.Text)
	return para
}

// processRuns processes a slice of runs, replacing MERGEFIELD runs.
func (p *Processor) processRuns(runs []ir.Run) []ir.Run {
	var result []ir.Run
	for _, run := range runs {
		switch run.FieldKind {
		case "MERGEFIELD":
			result = append(result, p.processMergeField(run)...)
		case "ASK", "FILLIN":
			// These are interactive fields; skip them in batch merge.
			result = append(result, run)
		default:
			// Process nested fields and text placeholders.
			run.Text = p.replacePlaceholders(run.Text)
			if len(run.Fields) > 0 {
				run.Fields = p.processFields(run.Fields)
			}
			result = append(result, run)
		}
	}
	return result
}

// processMergeField replaces a single MERGEFIELD run with the merged value.
func (p *Processor) processMergeField(run ir.Run) []ir.Run {
	fieldName := strings.TrimSpace(run.FieldArg)
	if fieldName == "" {
		// No field name; preserve as-is.
		return []ir.Run{run}
	}

	value, ok := p.lookup(fieldName)
	if !ok {
		if p.OnMissingField != nil {
			value = p.OnMissingField(fieldName)
		} else if run.Text != "" {
			// Fall back to cached text.
			value = run.Text
		} else {
			// Use placeholder.
			value = fmt.Sprintf("«%s»", fieldName)
		}
	}

	if p.PreserveFields {
		// Update the field's cached text but keep the field structure.
		run.Text = value
		return []ir.Run{run}
	}

	// Replace with a plain text run.
	return []ir.Run{{
		Text:   value,
		Kind:   "run",
		Format: run.Format,
	}}
}

// processFields processes nested fields (for IF, COMPARE, etc.).
func (p *Processor) processFields(fields []ir.Field) []ir.Field {
	for i := range fields {
		for j := range fields[i].Children {
			fields[i].Children[j].Text = p.replacePlaceholders(fields[i].Children[j].Text)
			if len(fields[i].Children[j].Fields) > 0 {
				fields[i].Children[j].Fields = p.processFields(fields[i].Children[j].Fields)
			}
		}
		for j := range fields[i].InstrRuns {
			fields[i].InstrRuns[j].Text = p.replacePlaceholders(fields[i].InstrRuns[j].Text)
		}
	}
	return fields
}

// replacePlaceholders replaces text-based placeholders like {{Name}} with values.
func (p *Processor) replacePlaceholders(text string) string {
	if p.PlaceholderPattern == nil {
		return text
	}
	return p.PlaceholderPattern.ReplaceAllStringFunc(text, func(match string) string {
		// Extract field name from {{Name}} or «Name»
		name := strings.Trim(match, "{}«»")
		name = strings.TrimSpace(name)
		if value, ok := p.lookup(name); ok {
			return value
		}
		if p.OnMissingField != nil {
			return p.OnMissingField(name)
		}
		return match
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// deepCopyDocument creates a deep copy of a document.
// Since Process() creates new slices for runs and blocks, we need to ensure
// each Process() call works on independent data. We do this by cloning
// the document structure before processing.
func deepCopyDocument(doc *ir.Document) *ir.Document {
	if doc == nil {
		return nil
	}
	// Create a new document with copied top-level fields.
	// The Process method will create new slices for blocks, runs, etc.
	// We copy the slices so that modifications don't affect the original.
	merged := &ir.Document{
		Title:                      doc.Title,
		Subject:                    doc.Subject,
		Author:                     doc.Author,
		UserName:                   doc.UserName,
		Description:                doc.Description,
		Keywords:                   doc.Keywords,
		LastModifiedBy:             doc.LastModifiedBy,
		Company:                    doc.Company,
		Manager:                    doc.Manager,
		Application:                doc.Application,
		AppVersion:                 doc.AppVersion,
		RevNum:                     doc.RevNum,
		Template:                   doc.Template,
		TemplatePath:               doc.TemplatePath,
		TotalEditMinutes:           doc.TotalEditMinutes,
		Filename:                   doc.Filename,
		FootnoteNumberFormat:       doc.FootnoteNumberFormat,
		FootnoteNumberStart:        doc.FootnoteNumberStart,
		FootnoteRestart:            doc.FootnoteRestart,
		EndnotePosition:            doc.EndnotePosition,
		EndnoteNumberFormat:        doc.EndnoteNumberFormat,
		EndnoteNumberStart:         doc.EndnoteNumberStart,
		EndnoteRestart:             doc.EndnoteRestart,
		DefaultParagraphStyleID:    doc.DefaultParagraphStyleID,
		DefaultTabStop:             doc.DefaultTabStop,
		CompatibilityMode:          doc.CompatibilityMode,
		ZoomPercent:                doc.ZoomPercent,
		ViewMode:                   doc.ViewMode,
		ProtectionEnabled:          doc.ProtectionEnabled,
		CreatedAt:                  doc.CreatedAt,
		ModifiedAt:                 doc.ModifiedAt,
		LastPrintedAt:              doc.LastPrintedAt,
		WordCount:                  doc.WordCount,
		CharCount:                  doc.CharCount,
		PageCount:                  doc.PageCount,
		ParagraphCount:             doc.ParagraphCount,
		MirrorMargins:              doc.MirrorMargins,
		TrackChangesEnabled:        doc.TrackChangesEnabled,
		CharacterSpacingControl:    doc.CharacterSpacingControl,
		AutoHyphenation:            doc.AutoHyphenation,
		ConsecutiveHyphenLimit:     doc.ConsecutiveHyphenLimit,
		HyphenationZone:            doc.HyphenationZone,
		DoNotHyphenateCaps:         doc.DoNotHyphenateCaps,
		EvenAndOddHeaders:          doc.EvenAndOddHeaders,
		UpdateFields:               doc.UpdateFields,
		DocumentType:               doc.DocumentType,
		DefaultTableStyle:          doc.DefaultTableStyle,
		BackgroundColor:            doc.BackgroundColor,
		DoNotDisplayPageBoundaries: doc.DoNotDisplayPageBoundaries,
		BordersDoNotSurroundFooter: doc.BordersDoNotSurroundFooter,
		BordersDoNotSurroundHeader: doc.BordersDoNotSurroundHeader,
		DoNotTrackFormatting:       doc.DoNotTrackFormatting,
		DoNotTrackMoves:            doc.DoNotTrackMoves,
	}

	// Deep copy blocks
	merged.Blocks = cloneBlocks(doc.Blocks)

	// Deep copy headers
	if doc.Headers != nil {
		merged.Headers = make(map[string]ir.Story, len(doc.Headers))
		for k, v := range doc.Headers {
			merged.Headers[k] = cloneStory(v)
		}
	}

	// Deep copy footers
	if doc.Footers != nil {
		merged.Footers = make(map[string]ir.Story, len(doc.Footers))
		for k, v := range doc.Footers {
			merged.Footers[k] = cloneStory(v)
		}
	}

	// Deep copy footnotes
	if doc.Footnotes != nil {
		merged.Footnotes = make(map[int]ir.Story, len(doc.Footnotes))
		for k, v := range doc.Footnotes {
			merged.Footnotes[k] = cloneStory(v)
		}
	}

	// Deep copy endnotes
	if doc.Endnotes != nil {
		merged.Endnotes = make(map[int]ir.Story, len(doc.Endnotes))
		for k, v := range doc.Endnotes {
			merged.Endnotes[k] = cloneStory(v)
		}
	}

	// Deep copy comments
	if doc.Comments != nil {
		merged.Comments = make(map[int]ir.Story, len(doc.Comments))
		for k, v := range doc.Comments {
			merged.Comments[k] = cloneStory(v)
		}
	}

	// Deep copy styles
	if doc.Styles != nil {
		merged.Styles = make(map[string]ir.Style, len(doc.Styles))
		for k, v := range doc.Styles {
			merged.Styles[k] = v
		}
	}

	// Deep copy character styles
	if doc.CharacterStyles != nil {
		merged.CharacterStyles = make(map[string]ir.CharacterStyle, len(doc.CharacterStyles))
		for k, v := range doc.CharacterStyles {
			merged.CharacterStyles[k] = v
		}
	}

	// Deep copy custom properties
	if doc.CustomProperties != nil {
		merged.CustomProperties = make(map[string]string, len(doc.CustomProperties))
		for k, v := range doc.CustomProperties {
			merged.CustomProperties[k] = v
		}
	}

	// Deep copy document variables
	if doc.DocumentVariables != nil {
		merged.DocumentVariables = make(map[string]string, len(doc.DocumentVariables))
		for k, v := range doc.DocumentVariables {
			merged.DocumentVariables[k] = v
		}
	}

	// Deep copy font families
	if doc.FontFamilies != nil {
		merged.FontFamilies = make(map[string]ir.FontFamilyInfo, len(doc.FontFamilies))
		for k, v := range doc.FontFamilies {
			merged.FontFamilies[k] = v
		}
	}

	// Deep copy sections
	merged.Sections = make([]ir.Section, len(doc.Sections))
	copy(merged.Sections, doc.Sections)

	return merged
}

// cloneStory creates a deep copy of a Story.
func cloneStory(story ir.Story) ir.Story {
	cloned := ir.Story{
		Text:   story.Text,
		Author: story.Author,
		Date:   story.Date,
	}
	cloned.Blocks = cloneBlocks(story.Blocks)
	cloned.Paragraphs = make([]ir.Paragraph, len(story.Paragraphs))
	for i, p := range story.Paragraphs {
		cloned.Paragraphs[i] = p
	}
	cloned.Tables = make([]ir.Table, len(story.Tables))
	for i, t := range story.Tables {
		cloned.Tables[i] = t
	}
	return cloned
}

// cloneBlocks creates a deep copy of a slice of blocks.
func cloneBlocks(blocks []ir.Block) []ir.Block {
	if blocks == nil {
		return nil
	}
	cloned := make([]ir.Block, len(blocks))
	for i, block := range blocks {
		cloned[i] = cloneBlock(block)
	}
	return cloned
}

// cloneBlock creates a deep copy of a single block.
func cloneBlock(block ir.Block) ir.Block {
	cloned := ir.Block{
		Index:      block.Index,
		Kind:       block.Kind,
		FieldKind:  block.FieldKind,
		FieldIndex: block.FieldIndex,
	}
	if block.Paragraph != nil {
		cloned.Paragraph = cloneParagraph(*block.Paragraph)
	}
	if block.Table != nil {
		cloned.Table = cloneTable(*block.Table)
	}
	if block.Section != nil {
		sec := *block.Section
		cloned.Section = &sec
	}
	if block.ContentControl != nil {
		cc := *block.ContentControl
		cloned.ContentControl = &cc
	}
	if block.CustomXml != nil {
		cx := *block.CustomXml
		cloned.CustomXml = &cx
	}
	if block.AltChunk != nil {
		ac := *block.AltChunk
		cloned.AltChunk = &ac
	}
	return cloned
}

// cloneParagraph creates a deep copy of a paragraph.
func cloneParagraph(para ir.Paragraph) *ir.Paragraph {
	cloned := &ir.Paragraph{
		Index:                        para.Index,
		StyleID:                      para.StyleID,
		StyleName:                    para.StyleName,
		StylePath:                    append([]string(nil), para.StylePath...),
		Text:                         para.Text,
		Bookmarks:                    append([]string(nil), para.Bookmarks...),
		CommentRangeStarts:           append([]int(nil), para.CommentRangeStarts...),
		CommentRangeEnds:             append([]int(nil), para.CommentRangeEnds...),
		MoveFromRangeStarts:          para.MoveFromRangeStarts,
		MoveFromRangeEnds:            para.MoveFromRangeEnds,
		MoveToRangeStarts:            para.MoveToRangeStarts,
		MoveToRangeEnds:              para.MoveToRangeEnds,
		PermRangeStarts:              para.PermRangeStarts,
		PermRangeEnds:                para.PermRangeEnds,
		CustomXmlInsRangeStarts:      para.CustomXmlInsRangeStarts,
		CustomXmlInsRangeEnds:        para.CustomXmlInsRangeEnds,
		CustomXmlDelRangeStarts:      para.CustomXmlDelRangeStarts,
		CustomXmlDelRangeEnds:        para.CustomXmlDelRangeEnds,
		CustomXmlMoveFromRangeStarts: para.CustomXmlMoveFromRangeStarts,
		CustomXmlMoveFromRangeEnds:   para.CustomXmlMoveFromRangeEnds,
		CustomXmlMoveToRangeStarts:   para.CustomXmlMoveToRangeStarts,
		CustomXmlMoveToRangeEnds:     para.CustomXmlMoveToRangeEnds,
		Frame:                        para.Frame,
	}
	if para.Numbering != nil {
		num := *para.Numbering
		cloned.Numbering = &num
	}
	if para.PropertyRevision != nil {
		pr := *para.PropertyRevision
		cloned.PropertyRevision = &pr
	}
	if para.List != nil {
		list := *para.List
		cloned.List = &list
	}
	if para.DefaultRunFormat != nil {
		drf := *para.DefaultRunFormat
		cloned.DefaultRunFormat = &drf
	}
	cloned.Runs = make([]ir.Run, len(para.Runs))
	for i, run := range para.Runs {
		cloned.Runs[i] = cloneRun(run)
	}
	return cloned
}

// cloneRun creates a deep copy of a run.
func cloneRun(run ir.Run) ir.Run {
	cloned := ir.Run{
		Text:           run.Text,
		Kind:           run.Kind,
		Revision:       run.Revision,
		RevisionAuthor: run.RevisionAuthor,
		RevisionDate:   run.RevisionDate,
		MoveID:         run.MoveID,
		FieldKind:      run.FieldKind,
		FieldInstr:     run.FieldInstr,
		FieldArg:       run.FieldArg,
		Target:         run.Target,
		Anchor:         run.Anchor,
		BreakType:      run.BreakType,
		BreakClear:     run.BreakClear,
		NoteID:         run.NoteID,
		CommentID:      run.CommentID,
		MathML:         run.MathML,
		Format:         run.Format,
	}
	if run.PropertyRevision != nil {
		pr := *run.PropertyRevision
		cloned.PropertyRevision = &pr
	}
	if run.DisplayFieldRevision != nil {
		dfr := *run.DisplayFieldRevision
		cloned.DisplayFieldRevision = &dfr
	}
	if run.Image != nil {
		img := *run.Image
		cloned.Image = &img
	}
	if run.TextBox != nil {
		tb := *run.TextBox
		cloned.TextBox = &tb
	}
	if run.Shape != nil {
		s := *run.Shape
		cloned.Shape = &s
	}
	if run.Chart != nil {
		// Chart is a pointer; keep reference (charts are typically shared)
		cloned.Chart = run.Chart
	}
	if run.FormField != nil {
		ff := *run.FormField
		cloned.FormField = &ff
	}
	if run.Ruby != nil {
		ruby := *run.Ruby
		cloned.Ruby = &ruby
	}
	if run.ContentControl != nil {
		cc := *run.ContentControl
		cloned.ContentControl = &cc
	}
	if run.CustomXml != nil {
		cx := *run.CustomXml
		cloned.CustomXml = &cx
	}
	if run.SmartTag != nil {
		st := *run.SmartTag
		cloned.SmartTag = &st
	}
	if len(run.Fields) > 0 {
		cloned.Fields = make([]ir.Field, len(run.Fields))
		for i, field := range run.Fields {
			cloned.Fields[i] = cloneField(field)
		}
	}
	if len(run.FieldInstrRuns) > 0 {
		cloned.FieldInstrRuns = make([]ir.Run, len(run.FieldInstrRuns))
		for i, r := range run.FieldInstrRuns {
			cloned.FieldInstrRuns[i] = cloneRun(r)
		}
	}
	return cloned
}

// cloneField creates a deep copy of a field.
func cloneField(field ir.Field) ir.Field {
	cloned := ir.Field{
		Kind:   field.Kind,
		Instr:  field.Instr,
		Arg:    field.Arg,
		Anchor: field.Anchor,
		Target: field.Target,
	}
	if len(field.Children) > 0 {
		cloned.Children = make([]ir.Run, len(field.Children))
		for i, child := range field.Children {
			cloned.Children[i] = cloneRun(child)
		}
	}
	if len(field.InstrRuns) > 0 {
		cloned.InstrRuns = make([]ir.Run, len(field.InstrRuns))
		for i, r := range field.InstrRuns {
			cloned.InstrRuns[i] = cloneRun(r)
		}
	}
	return cloned
}

// cloneTable creates a deep copy of a table.
func cloneTable(table ir.Table) *ir.Table {
	cloned := &ir.Table{
		Index:   table.Index,
		Text:    table.Text,
		StyleID: table.StyleID,
		Format:  table.Format,
	}
	cloned.Rows = make([]ir.TableRow, len(table.Rows))
	for i, row := range table.Rows {
		cloned.Rows[i] = cloneTableRow(row)
	}
	return cloned
}

// cloneTableRow creates a deep copy of a table row.
func cloneTableRow(row ir.TableRow) ir.TableRow {
	cloned := ir.TableRow{
		Format: row.Format,
	}
	if row.Override != nil {
		ov := *row.Override
		cloned.Override = &ov
	}
	cloned.Cells = make([]ir.TableCell, len(row.Cells))
	for i, cell := range row.Cells {
		cloned.Cells[i] = cloneTableCell(cell)
	}
	return cloned
}

// cloneTableCell creates a deep copy of a table cell.
func cloneTableCell(cell ir.TableCell) ir.TableCell {
	cloned := ir.TableCell{
		Text:   cell.Text,
		Format: cell.Format,
	}
	if cell.PropertyRevision != nil {
		pr := *cell.PropertyRevision
		cloned.PropertyRevision = &pr
	}
	if cell.CellIns != nil {
		ci := *cell.CellIns
		cloned.CellIns = &ci
	}
	if cell.CellDel != nil {
		cd := *cell.CellDel
		cloned.CellDel = &cd
	}
	if cell.CellMerge != nil {
		cm := *cell.CellMerge
		cloned.CellMerge = &cm
	}
	cloned.Paragraphs = make([]ir.Paragraph, len(cell.Paragraphs))
	for i, p := range cell.Paragraphs {
		cloned.Paragraphs[i] = p
	}
	cloned.Tables = make([]ir.Table, len(cell.Tables))
	for i, t := range cell.Tables {
		cloned.Tables[i] = t
	}
	cloned.Blocks = cloneBlocks(cell.Blocks)
	return cloned
}

// ---------------------------------------------------------------------------
// Convenience functions
// ---------------------------------------------------------------------------

// MergeDocument is a convenience function that creates a processor and processes a document.
func MergeDocument(doc *ir.Document, data Data) (*ir.Document, error) {
	return NewProcessor(data).Process(doc)
}

// MergeDocumentWithRecords processes a document with multi-record data.
func MergeDocumentWithRecords(doc *ir.Document, records []Record) ([]*ir.Document, error) {
	if doc == nil {
		return nil, fmt.Errorf("merge: document is nil")
	}
	proc := NewProcessorWithRecords(records)
	var results []*ir.Document
	for i := range records {
		proc.SetRecordIndex(i)
		merged, err := proc.Process(doc)
		if err != nil {
			return nil, fmt.Errorf("merge record %d: %w", i, err)
		}
		results = append(results, merged)
	}
	return results, nil
}

// ExtractMergeFields returns all unique MERGEFIELD names found in a document.
func ExtractMergeFields(doc *ir.Document) []string {
	if doc == nil {
		return nil
	}
	seen := make(map[string]bool)
	var fields []string

	var extractFromRuns func(runs []ir.Run)
	extractFromRuns = func(runs []ir.Run) {
		for _, run := range runs {
			if run.FieldKind == "MERGEFIELD" && run.FieldArg != "" {
				name := strings.TrimSpace(run.FieldArg)
				if !seen[name] {
					seen[name] = true
					fields = append(fields, name)
				}
			}
			for _, field := range run.Fields {
				for _, child := range field.Children {
					extractFromRuns([]ir.Run{child})
				}
			}
		}
	}

	var extractFromBlocks func(blocks []ir.Block)
	extractFromBlocks = func(blocks []ir.Block) {
		for _, block := range blocks {
			switch block.Kind {
			case "paragraph":
				if block.Paragraph != nil {
					extractFromRuns(block.Paragraph.Runs)
				}
			case "table":
				if block.Table != nil {
					for _, row := range block.Table.Rows {
						for _, cell := range row.Cells {
							extractFromBlocks(cell.Blocks)
						}
					}
				}
			}
		}
	}

	extractFromBlocks(doc.Blocks)
	for _, story := range doc.Headers {
		extractFromBlocks(story.Blocks)
		for _, para := range story.Paragraphs {
			extractFromRuns(para.Runs)
		}
	}
	for _, story := range doc.Footers {
		extractFromBlocks(story.Blocks)
		for _, para := range story.Paragraphs {
			extractFromRuns(para.Runs)
		}
	}
	for _, story := range doc.Footnotes {
		extractFromBlocks(story.Blocks)
		for _, para := range story.Paragraphs {
			extractFromRuns(para.Runs)
		}
	}
	for _, story := range doc.Endnotes {
		extractFromBlocks(story.Blocks)
		for _, para := range story.Paragraphs {
			extractFromRuns(para.Runs)
		}
	}
	for _, story := range doc.Comments {
		extractFromBlocks(story.Blocks)
		for _, para := range story.Paragraphs {
			extractFromRuns(para.Runs)
		}
	}

	return fields
}
