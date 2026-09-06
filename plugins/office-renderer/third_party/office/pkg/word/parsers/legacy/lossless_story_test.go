package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestStripLeadingStoryControlMarker(t *testing.T) {
	story := wordir.Story{
		Text: "\x02note",
		Paragraphs: []wordir.Paragraph{{
			Text: "\x02note",
			Runs: []wordir.Run{{Text: "\x02note"}},
		}},
	}

	stripLeadingStoryControlMarker(&story)

	if story.Text != "note" {
		t.Fatalf("story text = %q, want note", story.Text)
	}
	if got := story.Paragraphs[0].Text; got != "note" {
		t.Fatalf("paragraph text = %q, want note", got)
	}
	if got := story.Paragraphs[0].Runs[0].Text; got != "note" {
		t.Fatalf("run text = %q, want note", got)
	}
}

func TestStripLeadingStoryControlMarkerKeepsPrintableText(t *testing.T) {
	story := wordir.Story{
		Text: "note",
		Paragraphs: []wordir.Paragraph{{
			Text: "note",
			Runs: []wordir.Run{{Text: "note"}},
		}},
	}

	stripLeadingStoryControlMarker(&story)

	if story.Text != "note" || story.Paragraphs[0].Text != "note" || story.Paragraphs[0].Runs[0].Text != "note" {
		t.Fatalf("printable story changed: %+v", story)
	}
}
