package cascade

import (
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/drawingxml"
	"office/pkg/presentation/parsers/ooxml/parts"
)

func TestBackgroundColorOwnsDirectAndThemeColorProjection(t *testing.T) {
	if got := BackgroundColor(&parts.SlideBg{BgPrSrgb: &drawingxml.SRGBClr{Val: "AABBCC"}}, nil); got != "AABBCC" {
		t.Fatalf("direct background color = %q", got)
	}
	got := BackgroundColor(&parts.SlideBg{BgPrScheme: &drawingxml.SchemeClr{Val: "accent1"}}, map[string]string{"accent1": "112233"})
	if got != "112233" {
		t.Fatalf("theme background color = %q", got)
	}
}

func TestBackgroundFillOwnsExplicitFillSemantics(t *testing.T) {
	fill := BackgroundFill(&parts.SlideBg{BgPrSrgb: &drawingxml.SRGBClr{Val: "ABCDEF"}}, nil, nil, nil)
	solid, ok := fill.(drawing.SolidFill)
	if !ok || solid.Color != "ABCDEF" {
		t.Fatalf("background fill = %#v", fill)
	}
	if _, ok := BackgroundFill(&parts.SlideBg{BgPrNoFill: &struct{}{}}, nil, nil, nil).(drawing.NoFill); !ok {
		t.Fatal("explicit no-fill was not preserved")
	}
}

func TestThemeFormatFillOwnsThemeSchemeResolution(t *testing.T) {
	fill := ThemeFormatFill(&parts.ThemeFormatFillStyle{FillScheme: &drawingxml.SchemeClr{Val: "accent2"}}, map[string]string{"accent2": "445566"})
	solid, ok := fill.(drawing.SolidFill)
	if !ok || solid.Color != "445566" || solid.ThemeColor != "accent2" {
		t.Fatalf("theme format fill = %#v", fill)
	}
}
