package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func Test37625MasterBulletFontSurvivesIntoCanonicalParagraph(t *testing.T) {
	paragraph := &presentationir.Paragraph{BulletChar: "w", Runs: []*presentationir.Run{{Text: "The policy context", FontFamily: "Palatino"}}}
	master := legacyPptMasterTextLevel{paragraph: stpaPFEntry{HasBullet: true, HasBulletSpecified: true, BulletChar: 'w', BulletFont: "Wingdings"}}
	got := mergeLegacyPptMasterParagraph(paragraph, master, false)
	if got.BulletFontFamily != "Wingdings" {
		t.Fatalf("bullet font = %q, want inherited Wingdings", got.BulletFontFamily)
	}
	pres := &presentationir.Presentation{Slides: []*presentationir.Slide{{Shapes: []*presentationir.Shape{{Paragraphs: []*presentationir.Paragraph{got}}}}}}
	presentationir.NormalizeLegacyListMarkerGlyphs(pres)
	if got.BulletChar != "◆" || got.BulletFontFamily != "" || !got.BulletFontTx {
		t.Fatalf("canonical marker = %q font=%q tx=%v, want Unicode diamond following text font", got.BulletChar, got.BulletFontFamily, got.BulletFontTx)
	}
}


func Test37625LegacyStorageBulletCodesDecodeEvenAfterFontProvenanceCollapsed(t *testing.T) {
	pres := &presentationir.Presentation{Slides: []*presentationir.Slide{{Shapes: []*presentationir.Shape{{
		Paragraphs: []*presentationir.Paragraph{
			{BulletSpecified: true, BulletChar: "w", BulletFontTx: true, Runs: []*presentationir.Run{{Text: "The policy context", FontFamily: "Palatino"}}},
			{BulletSpecified: true, BulletChar: "n", Runs: []*presentationir.Run{{Text: "Need for an effective maintenance", FontFamily: "Palatino"}}},
		},
	}}}}}
	normalizeLegacyPptSymbolBulletStorageCodes(pres)
	got := pres.Slides[0].Shapes[0].Paragraphs
	if got[0].BulletChar != "◆" || got[1].BulletChar != "▪" {
		t.Fatalf("decoded markers = %q/%q, want diamond/square", got[0].BulletChar, got[1].BulletChar)
	}
	for i, p := range got {
		if p.BulletFontFamily != "" || !p.BulletFontTx {
			t.Fatalf("paragraph %d marker font = %q tx=%v, want follow-text Unicode marker", i, p.BulletFontFamily, p.BulletFontTx)
		}
	}
}
