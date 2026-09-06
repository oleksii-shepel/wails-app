package legacy

import (
	"bytes"
	"strings"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
	word97 "office/pkg/word/parsers/doc/word97"
)

// canonicalLosslessStoryCore adapts the remaining lossless legacy CLX/CHPX/PAPX
// storage into the canonical Word97 story materializer. Story semantics live in
// word97; this helper is only the compatibility boundary for old callers.
func canonicalLosslessStoryCore(clxData []byte, chpxEntries []chpxEntry, papxEntries []papxEntry) *word97.OpenCore {
	complex := docbinary.ParseComplexFileTable(clxData)
	if complex == nil || len(complex.PlcPcd) == 0 {
		return nil
	}
	return &word97.OpenCore{
		Complex: complex,
		CHPX:    toSequentialCHPXEntries(chpxEntries),
		PAPX:    toSequentialPAPXEntries(papxEntries),
	}
}

func materializeCanonicalLosslessStory(wordStream []byte, opened *word97.OpenCore, startCP, endCP int) wordir.Story {
	if len(wordStream) == 0 || opened == nil || startCP >= endCP {
		return wordir.Story{}
	}
	story := word97.MaterializeStoryRange(
		bytes.NewReader(wordStream),
		int64(len(wordStream)),
		opened,
		word97.StoryRange{StartCP: startCP, EndCP: endCP},
	)
	stripLeadingStoryControlMarker(&story)
	return story
}

func materializeCanonicalLosslessStoryRange(wordStream, clxData []byte, startCP, endCP int, chpxEntries []chpxEntry, papxEntries []papxEntry) wordir.Story {
	return materializeCanonicalLosslessStory(
		wordStream,
		canonicalLosslessStoryCore(clxData, chpxEntries, papxEntries),
		startCP,
		endCP,
	)
}

// splitCanonicalStoryOnBreak divides a canonical story at explicit break runs
// without falling back to the old DOC run/story builder. It is primarily used
// for legacy textbox stories where form-feed characters delimit independent
// textbox payloads, but it intentionally operates on shared word IR.
func splitCanonicalStoryOnBreak(story wordir.Story, breakType string) []wordir.Story {
	if len(story.Blocks) == 0 {
		if story.Text == "" && len(story.Paragraphs) == 0 && len(story.Tables) == 0 {
			return nil
		}
		return []wordir.Story{story}
	}

	var out []wordir.Story
	current := wordir.Story{}
	flush := func() {
		if len(current.Blocks) == 0 && len(current.Paragraphs) == 0 && len(current.Tables) == 0 && strings.TrimSpace(current.Text) == "" {
			current = wordir.Story{}
			return
		}
		out = append(out, current)
		current = wordir.Story{}
	}
	appendParagraph := func(p wordir.Paragraph) {
		if len(p.Runs) == 0 && p.Text == "" {
			return
		}
		p.Index = len(current.Paragraphs)
		current.Paragraphs = append(current.Paragraphs, p)
		paragraph := &current.Paragraphs[len(current.Paragraphs)-1]
		current.Blocks = append(current.Blocks, wordir.Block{Index: len(current.Blocks), Kind: "paragraph", Paragraph: paragraph})
		current.Text += paragraph.Text
	}
	appendTable := func(t wordir.Table) {
		t.Index = len(current.Tables)
		current.Tables = append(current.Tables, t)
		table := &current.Tables[len(current.Tables)-1]
		current.Blocks = append(current.Blocks, wordir.Block{Index: len(current.Blocks), Kind: "table", Table: table})
		current.Text += table.Text
	}

	for _, block := range story.Blocks {
		if block.Paragraph == nil {
			if block.Table != nil {
				appendTable(*block.Table)
			}
			continue
		}

		base := *block.Paragraph
		segment := base
		segment.Runs = nil
		segment.Text = ""
		for _, run := range base.Runs {
			if run.Kind == "break" && (breakType == "" || run.BreakType == breakType) {
				appendParagraph(segment)
				segment = base
				segment.Runs = nil
				segment.Text = ""
				flush()
				continue
			}
			segment.Runs = append(segment.Runs, run)
			segment.Text += run.Text
		}
		appendParagraph(segment)
	}
	flush()
	return out
}
