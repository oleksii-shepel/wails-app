package drawing_test

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	drawingcss "office/pkg/render/drawingcss"
)

func TestInnerShadowCSSInsetBoxShadow(t *testing.T) {
	// dir=5400000 → 90° → pointing down: offsetX=0, offsetY≈dist
	effect := drawing.InnerShadow{
		Color:    "FF0000",
		Alpha:    0.5,
		Blur:     4.0,
		Distance: 6.0,
		Angle:    90.0, // pointing down (clockwise from right)
	}
	css := drawingcss.InnerShadow(effect)
	shadow, ok := css["box-shadow"]
	if !ok {
		t.Fatal("InnerShadow.CSS() missing box-shadow key")
	}
	if !strings.HasPrefix(shadow, "inset ") {
		t.Errorf("box-shadow does not start with 'inset': %q", shadow)
	}
	// Expect rgba color since alpha < 1
	if !strings.Contains(shadow, "rgba(") {
		t.Errorf("expected rgba color in box-shadow, got: %q", shadow)
	}
	// There should be no filter property
	if _, hasFilter := css["filter"]; hasFilter {
		t.Error("InnerShadow.CSS() should not emit a filter property")
	}
}

func TestInnerShadowCSSOpaqueSolid(t *testing.T) {
	effect := drawing.InnerShadow{
		Color:    "003366",
		Alpha:    1.0,
		Blur:     2.0,
		Distance: 3.0,
		Angle:    0.0, // pointing right
	}
	css := drawingcss.InnerShadow(effect)
	shadow := css["box-shadow"]
	if !strings.HasPrefix(shadow, "inset ") {
		t.Errorf("expected inset prefix: %q", shadow)
	}
	if !strings.Contains(shadow, "#003366") {
		t.Errorf("expected hex color in opaque shadow: %q", shadow)
	}
}

func TestInnerShadowTypeConstant(t *testing.T) {
	e := drawing.InnerShadow{}
	if e.Type() != drawing.EffectTypeInnerShadow {
		t.Errorf("Type() = %q, want %q", e.Type(), drawing.EffectTypeInnerShadow)
	}
}

func TestEffectListCombinesInnerAndOuterShadow(t *testing.T) {
	list := drawing.EffectList{
		Effects: []drawing.Effect{
			drawing.Shadow{Color: "000000", Alpha: 0.5, Blur: 3.0, Distance: 2.0, Angle: 45.0},
			drawing.InnerShadow{Color: "000000", Alpha: 0.3, Blur: 2.0, Distance: 1.0, Angle: 90.0},
		},
	}
	css := drawingcss.EffectList(list)
	// Outer shadow → filter: drop-shadow(...)
	if !strings.Contains(css["filter"], "drop-shadow(") {
		t.Errorf("expected outer drop-shadow in filter: %q", css["filter"])
	}
	// Inner shadow → box-shadow: inset ...
	if !strings.HasPrefix(css["box-shadow"], "inset ") {
		t.Errorf("expected inset box-shadow: %q", css["box-shadow"])
	}
}

func TestEffectListMultipliesOpacityEffects(t *testing.T) {
	list := drawing.EffectList{
		Effects: []drawing.Effect{
			drawing.AlphaMod{Amount: 50000},
			drawing.AlphaModFix{Amount: 80000},
		},
	}
	css := drawingcss.EffectList(list)
	if got := css["opacity"]; got != "0.4000" {
		t.Fatalf("opacity = %q, want 0.4000", got)
	}
}
