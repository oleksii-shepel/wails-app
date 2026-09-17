package sax

import "testing"

func TestResolveVisualColorAppliesThemeTransformsAfterClrMap(t *testing.T) {
	b := newIRBuilder()
	b.themeColors["accent1"] = "2DA2BF"
	b.slideColorMap = map[string]string{"accent1": "accent1"}

	base := b.resolveVisualColor(VisualColor{Kind: "schemeClr", Value: "accent1"})
	shaded := b.resolveVisualColor(VisualColor{Kind: "schemeClr", Value: "accent1", Shade: 50000, HasShade: true})
	if base != "2DA2BF" {
		t.Fatalf("base theme color = %q, want 2DA2BF", base)
	}
	if shaded == "" || shaded == base {
		t.Fatalf("shaded theme color = %q, want a transformed color distinct from %q", shaded, base)
	}
}

func TestResolveVisualColorPreservesExplicitZeroLuminanceTransform(t *testing.T) {
	b := newIRBuilder()
	got := b.resolveVisualColor(VisualColor{Kind: "srgbClr", Value: "FFFFFF", LumMod: 0, HasLumMod: true})
	if got == "" {
		t.Fatal("explicit zero luminance transform lost the color token")
	}
}
