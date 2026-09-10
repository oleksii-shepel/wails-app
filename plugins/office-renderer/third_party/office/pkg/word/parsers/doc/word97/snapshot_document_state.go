package word97

import (
	"office/pkg/diag"
	wordir "office/pkg/word/ir"
)

// ApplySnapshotDocumentState projects canonical snapshot-owned document state
// onto the public Word IR after body paragraphs/tables have been assembled.
//
// This is intentionally the one Word97 document-state projection boundary:
// settings, fonts, bookmarks, note stories, section/header/footer structure,
// note separators, source provenance and compatibility all come from Snapshot.
// The legacy package no longer re-projects those fields piecemeal.
func ApplySnapshotDocumentState(doc *wordir.Document, snapshot *Snapshot) {
	if doc == nil || snapshot == nil || snapshot.Core == nil {
		return
	}
	core := snapshot.Core

	applyDocumentSettings(doc, core.Settings)

	if len(core.FontFamilies) > 0 {
		doc.FontFamilies = make(map[string]wordir.FontFamilyInfo, len(core.FontFamilies))
		for name, info := range core.FontFamilies {
			doc.FontFamilies[name] = info
		}
	}
	if len(core.EmbeddedFonts) > 0 {
		doc.EmbeddedFonts = append([]*wordir.EmbeddedFont(nil), core.EmbeddedFonts...)
	}
	doc.BookmarkEnds = bookmarkEndCPByName(core.Metadata.BookmarkEnds)

	ctx := snapshot.Context
	if ctx != nil {
		if len(ctx.Footnotes) > 0 {
			doc.Footnotes = cloneIntStoryMap(ctx.Footnotes)
		}
		if len(ctx.Endnotes) > 0 {
			doc.Endnotes = cloneIntStoryMap(ctx.Endnotes)
		}
		if len(ctx.Comments) > 0 {
			doc.Comments = cloneIntStoryMap(ctx.Comments)
		}
		applyDocumentContext(doc, ctx)
		ProjectDetachedDrawingTextBoxes(doc, ctx.Drawings.TextBoxes)
	}

	doc.Source = &wordir.DocumentSource{
		DOP:              append([]byte(nil), core.RawDOP...),
		Version:          core.Layout.Version,
		ComplexFileTable: cloneComplexFileTable(core.Source),
	}
	doc.Compatibility = core.Settings.Compatibility
}

func applyDocumentSettings(doc *wordir.Document, s DocumentSettings) {
	if s.FootnoteNumberFormat != "" {
		doc.FootnoteNumberFormat = s.FootnoteNumberFormat
	}
	if s.FootnoteNumberStart > 0 {
		doc.FootnoteNumberStart = s.FootnoteNumberStart
	}
	if s.FootnoteRestart != "" {
		doc.FootnoteRestart = s.FootnoteRestart
	}
	if s.EndnotePosition != "" {
		doc.EndnotePosition = s.EndnotePosition
	}
	if s.EndnoteNumberFormat != "" {
		doc.EndnoteNumberFormat = s.EndnoteNumberFormat
	}
	if s.EndnoteNumberStart > 0 {
		doc.EndnoteNumberStart = s.EndnoteNumberStart
	}
	if s.EndnoteRestart != "" {
		doc.EndnoteRestart = s.EndnoteRestart
	}
	if s.DefaultTabStop > 0 {
		doc.DefaultTabStop = s.DefaultTabStop
	}
	if s.ZoomPercent > 0 {
		doc.ZoomPercent = s.ZoomPercent
	}
	if s.ViewMode != "" {
		doc.ViewMode = s.ViewMode
	}
	if s.ProtectionEnabled {
		doc.ProtectionEnabled = true
	}
	if s.DocumentProtection != nil {
		doc.DocumentProtection = s.DocumentProtection
	}
	if s.CreatedAt != "" {
		doc.CreatedAt = s.CreatedAt
	}
	if s.ModifiedAt != "" {
		doc.ModifiedAt = s.ModifiedAt
	}
	if s.LastPrintedAt != "" {
		doc.LastPrintedAt = s.LastPrintedAt
	}
	if s.WordCount > 0 {
		doc.WordCount = s.WordCount
	}
	if s.CharCount > 0 {
		doc.CharCount = s.CharCount
	}
	if s.PageCount > 0 {
		doc.PageCount = s.PageCount
	}
	if s.ParagraphCount > 0 {
		doc.ParagraphCount = s.ParagraphCount
	}
	if s.MirrorMargins {
		doc.MirrorMargins = true
	}
	if s.TrackChangesEnabled {
		doc.TrackChangesEnabled = true
	}
}

func applyDocumentContext(doc *wordir.Document, ctx *Context) {
	if len(ctx.Sections) > 0 {
		doc.Sections = cloneSections(ctx.Sections)
		insertSectionBoundaryBlocks(doc)
	}
	if len(ctx.Headers) > 0 {
		doc.Headers = cloneStringStoryMap(ctx.Headers)
	}
	if len(ctx.Footers) > 0 {
		doc.Footers = cloneStringStoryMap(ctx.Footers)
	}
	if ctx.FootnoteSeparator != nil && ctx.FootnoteSeparator.Text != "" {
		doc.FootnoteSeparator = cloneStory(*ctx.FootnoteSeparator)
	}
	if ctx.FootnoteContinuationSep != nil && ctx.FootnoteContinuationSep.Text != "" {
		doc.FootnoteContinuationSep = cloneStory(*ctx.FootnoteContinuationSep)
	}
	if ctx.FootnoteContinuationNotice != nil && ctx.FootnoteContinuationNotice.Text != "" {
		doc.FootnoteContinuationNotice = cloneStory(*ctx.FootnoteContinuationNotice)
	}
	if ctx.EndnoteSeparator != nil && ctx.EndnoteSeparator.Text != "" {
		doc.EndnoteSeparator = cloneStory(*ctx.EndnoteSeparator)
	}
	if ctx.EndnoteContinuationSep != nil && ctx.EndnoteContinuationSep.Text != "" {
		doc.EndnoteContinuationSep = cloneStory(*ctx.EndnoteContinuationSep)
	}
	if ctx.EndnoteContinuationNotice != nil && ctx.EndnoteContinuationNotice.Text != "" {
		doc.EndnoteContinuationNotice = cloneStory(*ctx.EndnoteContinuationNotice)
	}
}

func cloneSections(src []wordir.Section) []wordir.Section {
	if len(src) == 0 {
		return nil
	}
	out := make([]wordir.Section, len(src))
	copy(out, src)
	for i := range out {
		out[i].Headers = append([]wordir.Reference(nil), src[i].Headers...)
		out[i].Footers = append([]wordir.Reference(nil), src[i].Footers...)
		out[i].Diagnostics = append(diag.List(nil), src[i].Diagnostics...)
		if src[i].Source != nil {
			v := *src[i].Source
			v.SepxSprms = append([]byte(nil), src[i].Source.SepxSprms...)
			out[i].Source = &v
		}
	}
	return out
}

func cloneStory(story wordir.Story) wordir.Story {
	out := story
	out.Paragraphs = append([]wordir.Paragraph(nil), story.Paragraphs...)
	out.Blocks = append([]wordir.Block(nil), story.Blocks...)
	out.Tables = append([]wordir.Table(nil), story.Tables...)
	return out
}

func cloneStringStoryMap(src map[string]wordir.Story) map[string]wordir.Story {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]wordir.Story, len(src))
	for key, story := range src {
		out[key] = cloneStory(story)
	}
	return out
}

func cloneIntStoryMap(src map[int]wordir.Story) map[int]wordir.Story {
	if len(src) == 0 {
		return nil
	}
	out := make(map[int]wordir.Story, len(src))
	for key, story := range src {
		out[key] = cloneStory(story)
	}
	return out
}

func bookmarkEndCPByName(ends map[int][]string) map[string]int {
	if len(ends) == 0 {
		return nil
	}
	out := make(map[string]int)
	for cp, names := range ends {
		for _, name := range names {
			if name != "" {
				out[name] = cp
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneComplexFileTable(source *wordir.DocumentSource) []byte {
	if source == nil || len(source.ComplexFileTable) == 0 {
		return nil
	}
	return append([]byte(nil), source.ComplexFileTable...)
}

// insertSectionBoundaryBlocks projects canonical PlcfSed CP ranges into the
// flat public block stream so layout switches page geometry at the same
// physical section boundaries as Word.
func insertSectionBoundaryBlocks(doc *wordir.Document) {
	if doc == nil || len(doc.Sections) < 2 || len(doc.Blocks) == 0 {
		return
	}
	type boundary struct {
		cp      int
		section *wordir.Section
	}
	boundaries := make([]boundary, 0, len(doc.Sections)-1)
	for i := 0; i < len(doc.Sections)-1; i++ {
		sec := &doc.Sections[i]
		if sec.Source == nil || sec.Source.EndCP <= sec.Source.StartCP {
			continue
		}
		boundaries = append(boundaries, boundary{cp: sec.Source.EndCP, section: sec})
	}
	if len(boundaries) == 0 {
		return
	}

	out := make([]wordir.Block, 0, len(doc.Blocks)+len(boundaries))
	boundaryIndex := 0
	for i := range doc.Blocks {
		block := doc.Blocks[i]
		startCP, hasCP := blockStartCP(&block)
		for hasCP && boundaryIndex < len(boundaries) && startCP >= boundaries[boundaryIndex].cp {
			sectionCopy := *boundaries[boundaryIndex].section
			out = append(out, wordir.Block{Kind: "section", Section: &sectionCopy})
			boundaryIndex++
		}
		out = append(out, block)
	}
	for boundaryIndex < len(boundaries) {
		sectionCopy := *boundaries[boundaryIndex].section
		out = append(out, wordir.Block{Kind: "section", Section: &sectionCopy})
		boundaryIndex++
	}
	for i := range out {
		out[i].Index = i
	}
	doc.Blocks = out
}

func blockStartCP(block *wordir.Block) (int, bool) {
	if block == nil || block.Paragraph == nil || block.Paragraph.Source == nil {
		return 0, false
	}
	src := block.Paragraph.Source
	if src.EndCP < src.StartCP {
		return 0, false
	}
	return src.StartCP, true
}

// ApplySnapshotDiagnostics projects diagnostics already decoded by the
// canonical Word97 snapshot. The legacy projection host must not walk CHPX,
// PAPX or section diagnostics independently.
func ApplySnapshotDiagnostics(doc *wordir.Document, snapshot *Snapshot) {
	if doc == nil || snapshot == nil || snapshot.Core == nil {
		return
	}
	core := snapshot.Core
	var diagnostics diag.List
	for _, entry := range core.CHPX {
		diagnostics = append(diagnostics, entry.Diagnostics...)
	}
	for _, entry := range core.PAPX {
		diagnostics = append(diagnostics, entry.Meta.Diagnostics...)
	}
	for _, sec := range doc.Sections {
		diagnostics = append(diagnostics, sec.Diagnostics...)
	}
	doc.Diagnostics = diagnostics
}
