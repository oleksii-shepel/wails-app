package legacy

import (
	"fmt"
	"strings"
	"unicode"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
)

type legacyHeaderFooterData struct {
	Headers                    map[string]wordir.Story
	Footers                    map[string]wordir.Story
	SectionHeaders             [][]wordir.Reference
	SectionFooters             [][]wordir.Reference
	FootnoteSeparator          wordir.Story
	FootnoteContinuationSep    wordir.Story
	FootnoteContinuationNotice wordir.Story
	EndnoteSeparator           wordir.Story
	EndnoteContinuationSep     wordir.Story
	EndnoteContinuationNotice  wordir.Story
}

// loadDocHeadersFooters is the pre-Word97 compatibility story adapter.
// Normal Word97 headers, footers and note separators are projected directly
// from the canonical Word97 snapshot; this adapter is pre-Word97 only.
func loadDocHeadersFooters(wordStream []byte, _ *cfb.File, _ int) *legacyHeaderFooterData {
	return loadPreWord97HeaderFooterStories(wordStream)
}

func cloneLegacyStoryMap(src map[string]wordir.Story) map[string]wordir.Story {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]wordir.Story, len(src))
	for key, story := range src {
		copyStory := story
		copyStory.Paragraphs = append([]wordir.Paragraph(nil), story.Paragraphs...)
		copyStory.Blocks = append([]wordir.Block(nil), story.Blocks...)
		copyStory.Tables = append([]wordir.Table(nil), story.Tables...)
		out[key] = copyStory
	}
	return out
}

// buildLegacyHeaderFooterDataFromSlots remains for pre-Word97 compatibility.
// Word97 no longer calls this path; its section/story references arrive already
// resolved from the canonical context.
func buildLegacyHeaderFooterDataFromSlots(noteStories [6]wordir.Story, slots [][6]wordir.Story, sectionCount int) *legacyHeaderFooterData {
	if sectionCount <= 0 {
		sectionCount = len(slots)
	}
	data := &legacyHeaderFooterData{
		Headers:        map[string]wordir.Story{},
		Footers:        map[string]wordir.Story{},
		SectionHeaders: make([][]wordir.Reference, sectionCount),
		SectionFooters: make([][]wordir.Reference, sectionCount),
	}
	prevHeaders := map[string]wordir.Reference{}
	prevFooters := map[string]wordir.Reference{}
	for sectionIndex := 0; sectionIndex < sectionCount; sectionIndex++ {
		var group [6]wordir.Story
		if sectionIndex < len(slots) {
			group = slots[sectionIndex]
		}
		data.SectionHeaders[sectionIndex] = buildLegacyHeaderReferences(data.Headers, group, sectionIndex, prevHeaders)
		data.SectionFooters[sectionIndex] = buildLegacyFooterReferences(data.Footers, group, sectionIndex, prevFooters)
	}
	if noteStories[3].Text != "" {
		data.EndnoteSeparator = noteStories[3]
	}
	if noteStories[4].Text != "" {
		data.EndnoteContinuationSep = noteStories[4]
	}
	if noteStories[5].Text != "" {
		data.EndnoteContinuationNotice = noteStories[5]
	}
	if noteStories[0].Text != "" {
		data.FootnoteSeparator = noteStories[0]
	}
	if noteStories[1].Text != "" {
		data.FootnoteContinuationSep = noteStories[1]
	}
	if noteStories[2].Text != "" {
		data.FootnoteContinuationNotice = noteStories[2]
	}
	if len(data.Headers) == 0 {
		data.Headers = nil
	}
	if len(data.Footers) == 0 {
		data.Footers = nil
	}
	if data.Headers == nil && data.Footers == nil &&
		data.FootnoteSeparator.Text == "" && data.FootnoteContinuationSep.Text == "" && data.FootnoteContinuationNotice.Text == "" &&
		data.EndnoteSeparator.Text == "" && data.EndnoteContinuationSep.Text == "" && data.EndnoteContinuationNotice.Text == "" {
		return nil
	}
	return data
}

func buildLegacyHeaderReferences(stories map[string]wordir.Story, group [6]wordir.Story, sectionIndex int, prev map[string]wordir.Reference) []wordir.Reference {
	specs := []struct {
		slot           int
		refType, label string
	}{
		{slot: 0, refType: "even", label: "even"},
		{slot: 1, refType: "default", label: "default"},
		{slot: 4, refType: "first", label: "first"},
	}
	return buildLegacyHeaderFooterRefs(stories, group, sectionIndex, prev, "header", specs)
}

func buildLegacyFooterReferences(stories map[string]wordir.Story, group [6]wordir.Story, sectionIndex int, prev map[string]wordir.Reference) []wordir.Reference {
	specs := []struct {
		slot           int
		refType, label string
	}{
		{slot: 2, refType: "even", label: "even"},
		{slot: 3, refType: "default", label: "default"},
		{slot: 5, refType: "first", label: "first"},
	}
	return buildLegacyHeaderFooterRefs(stories, group, sectionIndex, prev, "footer", specs)
}

func buildLegacyHeaderFooterRefs(stories map[string]wordir.Story, group [6]wordir.Story, sectionIndex int, prev map[string]wordir.Reference, kind string, specs []struct {
	slot           int
	refType, label string
}) []wordir.Reference {
	refs := make([]wordir.Reference, 0, len(specs))
	for _, spec := range specs {
		story := group[spec.slot]
		if story.Text == "" {
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

func buildHdrFtrStoryFromRaw(raw string) wordir.Story {
	runs := applyPreWord97FieldsToRuns([]binaryWordRun{{Text: raw}})
	return buildHdrFtrStoryFromRuns(runs)
}

func buildHdrFtrStoryFromRuns(runs []binaryWordRun) wordir.Story {
	runs = applyPreWord97FieldsToRuns(runs)
	normalized := make([]binaryWordRun, 0, len(runs))
	for _, run := range runs {
		if run.Format.Vanish != nil && *run.Format.Vanish {
			continue
		}
		run.Text = normalizeLegacyStoryText(run.Text)
		if run.Text == "" && run.Kind == "" && run.Image == nil {
			continue
		}
		normalized = append(normalized, run)
	}
	normalized = applyLegacyHeaderFooterFieldHeuristics(normalized)
	if len(normalized) == 0 {
		return wordir.Story{}
	}
	lineSlices := splitDocRunsIntoLines(normalized)
	if len(lineSlices) == 0 {
		return wordir.Story{}
	}
	paragraphs := make([]wordir.Paragraph, 0, len(lineSlices))
	blocks := make([]wordir.Block, 0, len(lineSlices))
	textParts := make([]string, 0, len(lineSlices))
	for _, lineRuns := range lineSlices {
		if lineSliceIsLegacyListBoundary(lineRuns) {
			continue
		}
		paragraph, _ := buildLegacyParagraphFromRuns(len(paragraphs), lineRuns, nil, nil, nil, nil)
		if paragraph.Text == "" && len(paragraph.Runs) == 0 {
			continue
		}
		paragraphs = append(paragraphs, paragraph)
		blocks = append(blocks, wordir.Block{Index: len(blocks), Kind: "paragraph", Paragraph: &paragraphs[len(paragraphs)-1]})
		textParts = append(textParts, paragraphs[len(paragraphs)-1].Text)
	}
	text := strings.TrimSpace(strings.Join(textParts, "\n"))
	if len(paragraphs) == 0 || text == "" {
		return wordir.Story{}
	}
	return wordir.Story{Paragraphs: paragraphs, Blocks: blocks, Text: text}
}

func applyLegacyHeaderFooterFieldHeuristics(runs []binaryWordRun) []binaryWordRun {
	out := make([]binaryWordRun, 0, len(runs))
	for _, run := range runs {
		if run.FieldKind != "" || run.Target != "" || run.Kind != "" {
			out = append(out, run)
			continue
		}
		out = append(out, splitLegacyHeaderFooterTextRun(run)...)
	}
	return out
}

func splitLegacyHeaderFooterTextRun(run binaryWordRun) []binaryWordRun {
	text := run.Text
	if !strings.Contains(text, "PAGE ") && !strings.Contains(text, "FILENAME ") && !strings.Contains(text, "AUTHOR ") && !strings.Contains(text, "DATE ") {
		return []binaryWordRun{run}
	}
	out := make([]binaryWordRun, 0, 3)
	for {
		idx, kind, prefixLen := findLegacyHeaderFooterField(text)
		if idx < 0 {
			if text != "" {
				run.Text = text
				out = append(out, run)
			}
			return out
		}
		if idx > 0 {
			prefix := run
			prefix.Text = text[:idx]
			out = append(out, prefix)
		}
		rest := text[idx+prefixLen:]
		valueLen := 0
		switch kind {
		case "PAGE", "FILENAME", "AUTHOR":
			for valueLen < len(rest) {
				ch := rest[valueLen]
				if ch == '\t' || ch == '\n' {
					break
				}
				valueLen++
			}
		case "DATE":
			if tab := strings.IndexAny(rest, "\t\n"); tab >= 0 {
				valueLen = tab
			} else {
				valueLen = len(rest)
			}
			if quote := strings.LastIndexByte(rest[:valueLen], '"'); quote >= 0 && quote+1 < valueLen {
				for quote+1 < valueLen && rest[quote+1] == ' ' {
					quote++
				}
				rest = rest[quote+1:]
				valueLen = len(rest)
			}
		}
		if valueLen == 0 {
			run.Text = text
			out = append(out, run)
			return out
		}
		field := run
		field.Text = strings.TrimSpace(rest[:valueLen])
		if kind == "PAGE" {
			field.FieldKind = "PAGE"
			field.FieldArg = ""
			field.Anchor = ""
		}
		out = append(out, field)
		text = rest[valueLen:]
	}
}

func findLegacyHeaderFooterField(text string) (idx int, kind string, prefixLen int) {
	best := -1
	for _, candidate := range []struct{ kind, prefix string }{
		{kind: "PAGE", prefix: "PAGE "},
		{kind: "FILENAME", prefix: "FILENAME "},
		{kind: "AUTHOR", prefix: "AUTHOR "},
		{kind: "DATE", prefix: "DATE "},
	} {
		i := strings.Index(text, candidate.prefix)
		if i < 0 {
			continue
		}
		if best < 0 || i < best {
			best = i
			kind = candidate.kind
			prefixLen = len(candidate.prefix)
		}
	}
	return best, kind, prefixLen
}

func normalizeLegacyStoryText(text string) string {
	text = strings.ReplaceAll(text, "\u0007", "\t")
	text = strings.ReplaceAll(text, "\u000B", "\n")
	text = strings.ReplaceAll(text, "\u000C", "\n")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	var b strings.Builder
	for _, r := range text {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r == '\u0000':
			continue
		case unicode.IsPrint(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
