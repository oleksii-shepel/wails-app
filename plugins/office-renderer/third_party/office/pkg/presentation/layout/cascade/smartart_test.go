package cascade

import (
	"testing"

	"office/pkg/ooxml/drawingxml"

	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestSmartArtFormatSchemeOwnsThemeMatrixProjection(t *testing.T) {
	raw := &parts.Theme{}
	raw.Elements.FmtScheme.FillStyleList = []parts.ThemeFormatFillStyle{{
		FillScheme: &drawingxml.SchemeClr{Val: "phClr"},
	}}
	raw.Elements.FmtScheme.BGFillStyleList = []parts.ThemeFormatFillStyle{{
		FillScheme: &drawingxml.SchemeClr{Val: "phClr"},
	}}
	theme := &ir.Theme{MajorFont: "Major", MinorFont: "Minor"}

	got := SmartArtFormatScheme(raw, map[string]string{"accent1": "336699"}, theme)
	if got == nil {
		t.Fatal("SmartArt format scheme = nil")
	}
	if len(got.FillStyles) != 1 || !got.FillStyles[0].UsesPlaceholder {
		t.Fatalf("fill styles = %#v, want one placeholder-aware entry", got.FillStyles)
	}
	if len(got.BgFillStyles) != 1 || !got.BgFillStyles[0].UsesPlaceholder {
		t.Fatalf("background fill styles = %#v, want one placeholder-aware entry", got.BgFillStyles)
	}
	if got.MajorFont != "Major" || got.MinorFont != "Minor" {
		t.Fatalf("fonts = (%q,%q), want (Major,Minor)", got.MajorFont, got.MinorFont)
	}
}
