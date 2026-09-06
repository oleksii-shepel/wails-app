package word97

import (
	"fmt"
	"io"
	"strings"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"
)

// LoadContextReaderAt materializes the bounded document-wide semantic context
// required by the event walker: side stories, sections, headers/footers and
// preserved companion metadata. Main-story text remains streaming.
func LoadContextReaderAt(word io.ReaderAt, wordSize int64, f *cfb.File, opened *OpenCore) *Context {
	ctx := &Context{}
	if word == nil || opened == nil {
		return ctx
	}
	ctx.Sections = loadCanonicalSections(opened)
	loadCanonicalSideStories(word, wordSize, opened, ctx)
	loadCanonicalHeadersFooters(word, wordSize, opened, ctx)
	applyPreservationToContext(LoadPreservation(f), ctx)
	return ctx
}

func loadCanonicalSections(opened *OpenCore) []wordir.Section {
	if opened == nil || len(opened.Sections) == 0 {
		return nil
	}
	sections := make([]wordir.Section, 0, len(opened.Sections))
	for _, record := range opened.Sections {
		d := record.Descriptor
		sec := wordir.Section{}
		if len(record.Sepx) > 0 {
			sec, _ = docformat.DecodeSEPX(record.Sepx)
			sec.Source = &wordir.SectionSource{SepxSprms: append([]byte(nil), record.Sepx...), StartCP: d.StartCP, EndCP: d.EndCP}
		} else {
			sec.Source = &wordir.SectionSource{StartCP: d.StartCP, EndCP: d.EndCP}
		}
		sections = append(sections, sec)
	}
	return sections
}

func loadCanonicalSideStories(word io.ReaderAt, wordSize int64, opened *OpenCore, ctx *Context) {
	plans := opened.SideStories
	ctx.FootnoteRefs = referenceMapFromRanges(plans.Footnotes)
	ctx.EndnoteRefs = referenceMapFromRanges(plans.Endnotes)
	ctx.CommentRefs = referenceMapFromRanges(plans.Comments)
	ctx.Footnotes = materializeReferencedStories(word, wordSize, opened, plans.Footnotes)
	ctx.Endnotes = materializeReferencedStories(word, wordSize, opened, plans.Endnotes)
	ctx.Comments = materializeReferencedStories(word, wordSize, opened, plans.Comments)
}

func referenceMapFromRanges(ranges []ReferencedStoryRange) map[int]int {
	if len(ranges) == 0 {
		return nil
	}
	out := make(map[int]int, len(ranges))
	for _, item := range ranges {
		out[item.ReferenceCP] = item.ID
	}
	return out
}

func materializeReferencedStories(word io.ReaderAt, wordSize int64, opened *OpenCore, ranges []ReferencedStoryRange) map[int]wordir.Story {
	if len(ranges) == 0 {
		return nil
	}
	out := make(map[int]wordir.Story, len(ranges))
	for _, item := range ranges {
		story := MaterializeStoryRange(word, wordSize, opened, item.Range)
		normalizeReferencedStory(&story)
		if storyHasContent(story) {
			out[item.ID] = story
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Note/comment side stories can begin with structural reference markers.
// 0x02 is used for note references and 0x05 for annotation references. They
// are not visible story text and must not escape the binary decoding layer.
func normalizeReferencedStory(story *wordir.Story) {
	if story == nil {
		return
	}
	trim := func(s string) string { return strings.TrimLeft(s, "\x02\x05") }
	story.Text = trim(story.Text)
	for i := range story.Paragraphs {
		p := &story.Paragraphs[i]
		p.Text = trim(p.Text)
		for j := range p.Runs {
			if p.Runs[j].Text == "" {
				continue
			}
			p.Runs[j].Text = trim(p.Runs[j].Text)
			break
		}
	}
	for i := range story.Blocks {
		if story.Blocks[i].Paragraph != nil {
			p := story.Blocks[i].Paragraph
			p.Text = trim(p.Text)
			for j := range p.Runs {
				if p.Runs[j].Text == "" {
					continue
				}
				p.Runs[j].Text = trim(p.Runs[j].Text)
				break
			}
		}
	}
}

func loadCanonicalHeadersFooters(word io.ReaderAt, wordSize int64, opened *OpenCore, ctx *Context) {
	plan := opened.HeaderFooters
	if plan == nil {
		return
	}
	sectionCount := plan.SectionCount
	if len(ctx.Sections) > sectionCount {
		sectionCount = len(ctx.Sections)
	}
	var noteStories [6]wordir.Story
	for i, r := range plan.NoteStories {
		noteStories[i] = MaterializeStoryRange(word, wordSize, opened, r)
	}
	slots := make([][6]wordir.Story, len(plan.Sections))
	for i, group := range plan.Sections {
		for j, r := range group {
			slots[i][j] = MaterializeStoryRange(word, wordSize, opened, r)
		}
	}
	applyHeaderFooterSlots(ctx, noteStories, slots, sectionCount)
}

func applyHeaderFooterSlots(ctx *Context, noteStories [6]wordir.Story, slots [][6]wordir.Story, sectionCount int) {
	if ctx == nil {
		return
	}
	if sectionCount <= 0 {
		sectionCount = len(slots)
	}
	if sectionCount > len(ctx.Sections) {
		ctx.Sections = append(ctx.Sections, make([]wordir.Section, sectionCount-len(ctx.Sections))...)
	}
	headers := map[string]wordir.Story{}
	footers := map[string]wordir.Story{}
	prevHeaders := map[string]wordir.Reference{}
	prevFooters := map[string]wordir.Reference{}
	for sectionIndex := 0; sectionIndex < sectionCount; sectionIndex++ {
		var group [6]wordir.Story
		if sectionIndex < len(slots) {
			group = slots[sectionIndex]
		}
		ctx.Sections[sectionIndex].Headers = buildHeaderFooterRefs(headers, group, sectionIndex, prevHeaders, "header", []storySlotSpec{{0, "even", "even"}, {1, "default", "default"}, {4, "first", "first"}})
		ctx.Sections[sectionIndex].Footers = buildHeaderFooterRefs(footers, group, sectionIndex, prevFooters, "footer", []storySlotSpec{{2, "even", "even"}, {3, "default", "default"}, {5, "first", "first"}})
	}
	if len(headers) > 0 {
		ctx.Headers = headers
	}
	if len(footers) > 0 {
		ctx.Footers = footers
	}
	if storyHasContent(noteStories[0]) {
		s := noteStories[0]
		ctx.FootnoteSeparator = &s
	}
	if storyHasContent(noteStories[1]) {
		s := noteStories[1]
		ctx.FootnoteContinuationSep = &s
	}
	if storyHasContent(noteStories[2]) {
		s := noteStories[2]
		ctx.FootnoteContinuationNotice = &s
	}
	if storyHasContent(noteStories[3]) {
		s := noteStories[3]
		ctx.EndnoteSeparator = &s
	}
	if storyHasContent(noteStories[4]) {
		s := noteStories[4]
		ctx.EndnoteContinuationSep = &s
	}
	if storyHasContent(noteStories[5]) {
		s := noteStories[5]
		ctx.EndnoteContinuationNotice = &s
	}
}

type storySlotSpec struct {
	slot    int
	refType string
	label   string
}

func buildHeaderFooterRefs(stories map[string]wordir.Story, group [6]wordir.Story, sectionIndex int, prev map[string]wordir.Reference, kind string, specs []storySlotSpec) []wordir.Reference {
	refs := make([]wordir.Reference, 0, len(specs))
	for _, spec := range specs {
		story := group[spec.slot]
		if !storyHasContent(story) {
			if ref, ok := prev[spec.refType]; ok {
				refs = append(refs, ref)
			}
			continue
		}
		partName := fmt.Sprintf("legacy-%s-s%d-%s", kind, sectionIndex+1, spec.label)
		stories[partName] = story
		ref := wordir.Reference{Type: spec.refType, PartName: partName}
		refs = append(refs, ref)
		prev[spec.refType] = ref
	}
	return refs
}

func applyPreservationToContext(meta *PreservationMeta, ctx *Context) {
	if meta == nil || ctx == nil {
		return
	}
	mergeContextStringStories(&ctx.Headers, meta.Headers)
	mergeContextStringStories(&ctx.Footers, meta.Footers)
	mergeContextIntStories(&ctx.Footnotes, meta.Footnotes)
	mergeContextIntStories(&ctx.Endnotes, meta.Endnotes)
	mergeContextIntStories(&ctx.Comments, meta.Comments)
	if meta.FootnoteSeparator != nil {
		s := *meta.FootnoteSeparator
		ctx.FootnoteSeparator = &s
	}
	if meta.FootnoteContinuationSep != nil {
		s := *meta.FootnoteContinuationSep
		ctx.FootnoteContinuationSep = &s
	}
	if meta.FootnoteContinuationNotice != nil {
		s := *meta.FootnoteContinuationNotice
		ctx.FootnoteContinuationNotice = &s
	}
	if meta.EndnoteSeparator != nil {
		s := *meta.EndnoteSeparator
		ctx.EndnoteSeparator = &s
	}
	if meta.EndnoteContinuationSep != nil {
		s := *meta.EndnoteContinuationSep
		ctx.EndnoteContinuationSep = &s
	}
	if meta.EndnoteContinuationNotice != nil {
		s := *meta.EndnoteContinuationNotice
		ctx.EndnoteContinuationNotice = &s
	}
	n := len(meta.SectionHeaders)
	if len(meta.SectionFooters) > n {
		n = len(meta.SectionFooters)
	}
	if n > len(ctx.Sections) {
		ctx.Sections = append(ctx.Sections, make([]wordir.Section, n-len(ctx.Sections))...)
	}
	for i := 0; i < n; i++ {
		if i < len(meta.SectionHeaders) && meta.SectionHeaders[i] != nil {
			ctx.Sections[i].Headers = append([]wordir.Reference(nil), meta.SectionHeaders[i]...)
		}
		if i < len(meta.SectionFooters) && meta.SectionFooters[i] != nil {
			ctx.Sections[i].Footers = append([]wordir.Reference(nil), meta.SectionFooters[i]...)
		}
	}
}

func mergeContextStringStories(dst *map[string]wordir.Story, src map[string]wordir.Story) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[string]wordir.Story)
	}
	for k, v := range src {
		(*dst)[k] = v
	}
}

func mergeContextIntStories(dst *map[int]wordir.Story, src map[int]wordir.Story) {
	if len(src) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[int]wordir.Story)
	}
	for k, v := range src {
		(*dst)[k] = v
	}
}

func storyHasContent(story wordir.Story) bool {
	return story.Text != "" || len(story.Blocks) > 0 || len(story.Paragraphs) > 0 || len(story.Tables) > 0
}
