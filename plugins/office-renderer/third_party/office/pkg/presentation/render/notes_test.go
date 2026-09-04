package render_test

import (
	"strings"
	"testing"

	resolve "office/pkg/presentation/parsers/ooxml/resolve"
	"office/pkg/presentation/render"
)

func TestHTMLRendersNotesBackgroundColor(t *testing.T) {
	pres := &resolve.Presentation{
		SlideWidthEMU:  9144000,
		SlideHeightEMU: 6858000,
		Slides: []*resolve.Slide{{
			Index: 0,
			Notes: &resolve.NotesSlide{
				BgColor: "FFF2CC",
				Shapes:  []*resolve.Shape{{Kind: resolve.ShapeText, Bounds: resolve.Rect{W: 1000, H: 1000}, Paragraphs: []*resolve.Paragraph{{Runs: []*resolve.Run{{Text: "Speaker note"}}}}}},
			},
		}},
	}

	html, err := render.HTML(pres)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, `<div class="notes" style="background-color:#FFF2CC">`) {
		t.Fatalf("expected notes background color in output: %s", html)
	}
	if !strings.Contains(html, "Speaker note") {
		t.Fatalf("expected notes content in output: %s", html)
	}
}