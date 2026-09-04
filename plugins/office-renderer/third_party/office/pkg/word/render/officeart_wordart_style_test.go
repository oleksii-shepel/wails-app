package render

import (
	"strings"
	"testing"

	"office/pkg/word/parsers/ooxml/resolve"
)

func TestShapeTextPathStylesRenderLegacyWordArtFormatting(t *testing.T) {
	styles := strings.Join(shapeTextPathStyles(&resolve.TextPath{
		On: "t", String: "WordArt", Font: "Arial Black", SizePt: 36,
		Bold: true, Italic: true, Vertical: true,
	}), ";")
	for _, want := range []string{
		`font-family:"Arial Black"`, "font-size:36.00pt",
		"font-weight:bold", "font-style:italic",
		"writing-mode:vertical-rl", "text-orientation:mixed",
	} {
		if !strings.Contains(styles, want) {
			t.Fatalf("styles %q missing %q", styles, want)
		}
	}
}

func TestShapeTextPathStylesKeepVMLDefaultsWhenFormattingAbsent(t *testing.T) {
	styles := strings.Join(shapeTextPathStyles(&resolve.TextPath{On: "t", String: "Curved Text"}), ";")
	for _, unwanted := range []string{"font-family:", "font-size:", "font-weight:", "font-style:", "writing-mode:"} {
		if strings.Contains(styles, unwanted) {
			t.Fatalf("unstyled textpath unexpectedly contains %q: %q", unwanted, styles)
		}
	}
}
