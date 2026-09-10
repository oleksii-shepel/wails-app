package render_test

import (
	"strings"
	"testing"

	resolve "office/pkg/word/ir"
	"office/pkg/word/render"
)

func TestHTMLPhysicalParagraphPositionsLineBreakRun(t *testing.T) {
	p := &resolve.Paragraph{Runs: []resolve.Run{
		{Kind: "run", Text: "Title"},
		{Kind: "break", BreakType: "line"},
		{Kind: "run", Text: "Caption"},
	}}
	doc := &resolve.Document{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: p}}}
	html, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	for _, want := range []string{
		`class="word-physical-slot" data-line="0"`,
		`class="word-physical-slot" data-line="1"`,
		">Title</span>",
		">Caption</span>",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("HTML = %q, want %q", html, want)
		}
	}
}
