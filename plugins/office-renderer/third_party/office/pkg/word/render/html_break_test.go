package render_test

import (
	"strings"
	"testing"

	resolve "office/pkg/word/ir"
	"office/pkg/word/render"
)

func TestHTMLRendersEmptyLineBreakRun(t *testing.T) {
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
	if !strings.Contains(html, "Title<br>Caption") {
		t.Fatalf("HTML = %q, want rendered line break", html)
	}
}
