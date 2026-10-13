package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestPass12SynthesizedHeaderFooterUsesMasterPlaceholderTemplate(t *testing.T) {
	templateParagraph := &presentationir.Paragraph{
		Align: "ctr",
		Runs:  []*presentationir.Run{{FontSize: 1400, FontFamily: "Tahoma", Color: "40458C"}},
	}
	template := legacyPptTextShape{
		bounds:      presentationir.Rect{X: 2895600, Y: 6477000, W: 2895600, H: 457200},
		placeholder: presentationir.PlaceholderFooter,
		paragraphs:  []*presentationir.Paragraph{templateParagraph},
		transform: legacyPptShapeTransform{
			HasTextAnchor: true,
			TextAnchor:    "b",
		},
	}

	_, footer, _, _ := synthesizeLegacyHeaderFooterShapes(
		1,
		legacyPptHeaderFooterInfo{flags: legacyPptHFFlagFooter, footerText: "Transport CDM Workshop"},
		map[presentationir.PlaceholderKind]legacyPptTextShape{presentationir.PlaceholderFooter: template},
		nil, nil, nil, nil,
		legacyPptDefaultCanvas())
	if footer == nil {
		t.Fatal("footer = nil")
	}
	if footer.Bounds != template.bounds {
		t.Fatalf("footer bounds = %+v, want master template %+v", footer.Bounds, template.bounds)
	}
	if footer.TextAnchor != "b" {
		t.Fatalf("footer anchor = %q, want bottom", footer.TextAnchor)
	}
	if len(footer.Paragraphs) != 1 || footer.Paragraphs[0].Align != "ctr" {
		t.Fatalf("footer paragraph = %#v, want centered template formatting", footer.Paragraphs)
	}
	if len(footer.Paragraphs[0].Runs) != 1 {
		t.Fatalf("footer runs = %#v", footer.Paragraphs[0].Runs)
	}
	run := footer.Paragraphs[0].Runs[0]
	if run.Text != "Transport CDM Workshop" || run.FontSize != 1400 || run.FontFamily != "Tahoma" || run.Color != "40458C" {
		t.Fatalf("footer run = %#v, want template style with synthesized text", run)
	}
}
