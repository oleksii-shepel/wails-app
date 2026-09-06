package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestSplitCanonicalStoryOnPageBreak(t *testing.T) {
	story := wordir.Story{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{
				{Kind: "run", Text: "left"},
				{Kind: "break", BreakType: "page"},
				{Kind: "run", Text: "right"},
			},
			Text: "leftright",
		}},
	}
	story.Blocks = []wordir.Block{{Kind: "paragraph", Paragraph: &story.Paragraphs[0]}}

	parts := splitCanonicalStoryOnBreak(story, "page")
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(parts))
	}
	if parts[0].Text != "left" || parts[1].Text != "right" {
		t.Fatalf("texts = %q / %q, want left / right", parts[0].Text, parts[1].Text)
	}
	if len(parts[0].Blocks) != 1 || len(parts[1].Blocks) != 1 {
		t.Fatalf("block counts = %d / %d, want 1 / 1", len(parts[0].Blocks), len(parts[1].Blocks))
	}
}

func TestSplitCanonicalStoryKeepsTables(t *testing.T) {
	table := wordir.Table{Text: "table"}
	story := wordir.Story{
		Tables: []wordir.Table{table},
	}
	story.Blocks = []wordir.Block{{Kind: "table", Table: &story.Tables[0]}}

	parts := splitCanonicalStoryOnBreak(story, "page")
	if len(parts) != 1 || len(parts[0].Tables) != 1 || parts[0].Text != "table" {
		t.Fatalf("split table story = %+v", parts)
	}
}
