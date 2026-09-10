package stream

import (
	"testing"

	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func TestParagraphFormatFromSAXUsesWordSnapToGridDefault(t *testing.T) {
	got := paragraphFormatFromSAX(wordsax.ParagraphProperties{})
	if !got.SnapToGrid {
		t.Fatal("omitted w:snapToGrid must use Word's enabled default; layout applies it only when a document grid exists")
	}
	off := false
	got = paragraphFormatFromSAX(wordsax.ParagraphProperties{SnapToGrid: &off})
	if got.SnapToGrid {
		t.Fatal("explicit w:snapToGrid=0 was ignored")
	}
}

func TestRunFormatFromSAXCarriesCharacterSpacingAndComplexScriptMetrics(t *testing.T) {
	spacing, kern, csSize := 50, 24, 30
	boldCS := true
	got := runFormatFromSAX(wordsax.RunProperties{
		CharacterSpacing: &spacing,
		KerningHalfPoints: &kern,
		FontSizeCSHalfPoints: &csSize,
		BoldCS: &boldCS,
	})
	if got.CharacterSpacing == nil || *got.CharacterSpacing != 50 {
		t.Fatalf("CharacterSpacing = %v", got.CharacterSpacing)
	}
	if got.Kerning == nil || *got.Kerning != 24 {
		t.Fatalf("Kerning = %v", got.Kerning)
	}
	if got.FontSizeCS == nil || *got.FontSizeCS != 30 || got.BoldCS == nil || !*got.BoldCS {
		t.Fatalf("complex-script metrics lost: %#v", got)
	}
}
