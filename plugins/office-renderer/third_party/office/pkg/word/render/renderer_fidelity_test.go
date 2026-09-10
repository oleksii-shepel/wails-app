package render

import (
	"strings"
	"testing"
	"time"

	resolve "office/pkg/word/ir"
)

func TestHTMLVolatileDateIgnoresStaleCachedResult(t *testing.T) {
	fixed := time.Date(2026, time.September, 9, 12, 0, 0, 0, time.UTC)
	doc := &resolve.Document{Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
		Text: "4/26/2006", FieldKind: "DATE", FieldInstr: `DATE \@ "M/d/yyyy"`,
	}}}}}}
	out, _, err := HTMLWithOptions(doc, HTMLOptions{RenderTime: &fixed})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "9/9/2026") || strings.Contains(out, ">4/26/2006<") {
		t.Fatalf("DATE field did not refresh from render time: %s", out)
	}
}

func TestHTMLFilenameUsesCurrentDocumentFilenameOverCachedResult(t *testing.T) {
	doc := &resolve.Document{
		Filename: "legacy-floating-pictures.doc",
		Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{{
			Text: "Office Open XML sample.doc", FieldKind: "FILENAME", FieldInstr: "FILENAME",
		}}}}},
	}
	out, err := HTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "legacy-floating-pictures.doc") || strings.Contains(out, "Office Open XML sample.doc") {
		t.Fatalf("FILENAME field retained stale cached result: %s", out)
	}
}

func TestFooterCanonicalTabsFallbackToCenterAndRightStops(t *testing.T) {
	blocks := []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{Runs: []resolve.Run{
		{Text: "John Greer"}, {Kind: "tab"}, {Text: "Page 1"}, {Kind: "tab"}, {Text: "9/9/2026"},
	}}}}
	got := footerBlocksWithCanonicalTabs(blocks, 432)
	stops := got[0].Paragraph.Format.TabStops
	if len(stops) != 2 || stops[0].Pos != 4320 || stops[0].Algn != "center" || stops[1].Pos != 8640 || stops[1].Algn != "right" {
		t.Fatalf("footer fallback tabs = %#v, want center@216pt right@432pt", stops)
	}
	if len(blocks[0].Paragraph.Format.TabStops) != 0 {
		t.Fatal("footer tab fallback mutated source paragraph")
	}
}
