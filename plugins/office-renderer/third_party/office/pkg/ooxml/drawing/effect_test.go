package drawing

import "testing"

func TestEffectTypes(t *testing.T) {
	tests := []struct {
		effect Effect
		want   EffectType
	}{
		{Shadow{}, EffectTypeShadow},
		{InnerShadow{}, EffectTypeInnerShadow},
		{Glow{}, EffectTypeGlow},
		{SoftEdge{}, EffectTypeSoftEdge},
		{Reflection{}, EffectTypeReflection},
		{Blur{}, EffectTypeBlur},
		{ThreeD{}, EffectTypeThreeD},
		{Grayscale{}, EffectTypeGrayscale},
		{BiLevel{}, EffectTypeBiLevel},
		{AlphaBiLevel{}, EffectTypeAlphaBiLevel},
		{Duotone{}, EffectTypeDuotone},
		{AlphaMod{}, EffectTypeAlphaMod},
		{AlphaOff{}, EffectTypeAlphaOff},
		{AlphaModFix{}, EffectTypeAlphaModFix},
		{AlphaRepl{}, EffectTypeAlphaRepl},
		{AlphaOutset{}, EffectTypeAlphaOutset},
		{AlphaInv{}, EffectTypeAlphaInv},
		{AlphaCeiling{}, EffectTypeAlphaCeiling},
		{AlphaFloor{}, EffectTypeAlphaFloor},
		{DeSat{}, EffectTypeDeSat},
		{DeSatLow{}, EffectTypeDeSatLow},
		{DeSatMod{}, EffectTypeDeSatMod},
		{HSLEffect{}, EffectTypeHSLEffect},
		{Lum{}, EffectTypeLum},
		{ClrChange{}, EffectTypeClrChange},
		{ClrRepl{}, EffectTypeClrRepl},
		{FillOverlay{}, EffectTypeFillOverlay},
		{BlendMode{}, EffectTypeBlendMode},
		{RelativeOffset{}, EffectTypeRelOff},
		{Tint{}, EffectTypeTint},
		{TransformEffect{}, EffectTypeTransform},
	}
	for _, tt := range tests {
		if got := tt.effect.Type(); got != tt.want {
			t.Errorf("%T.Type() = %q, want %q", tt.effect, got, tt.want)
		}
	}
}

func TestParseEffectOuterShadow(t *testing.T) {
	effect, err := ParseEffect(map[string]interface{}{
		"outerShdw": map[string]interface{}{
			"srgbClr": map[string]interface{}{
				"val":   "FF0000",
				"alpha": 50000.0,
			},
			"blurRad": 228600.0,
			"dist":    127000.0,
			"dir":     10800000.0,
		},
	})
	if err != nil {
		t.Fatalf("ParseEffect error: %v", err)
	}
	if effect == nil {
		t.Fatal("ParseEffect returned nil")
	}
	shadow, ok := effect.(Shadow)
	if !ok {
		t.Fatalf("expected Shadow, got %T", effect)
	}
	if shadow.Color != "FF0000" {
		t.Errorf("Color = %q, want FF0000", shadow.Color)
	}
	if shadow.Alpha != 0.5 {
		t.Errorf("Alpha = %v, want 0.5", shadow.Alpha)
	}
	if shadow.Blur != 18.0 {
		t.Errorf("Blur = %v, want 18.0", shadow.Blur)
	}
	if shadow.Distance != 10.0 {
		t.Errorf("Distance = %v, want 10.0", shadow.Distance)
	}
	if shadow.Angle != 180.0 {
		t.Errorf("Angle = %v, want 180.0", shadow.Angle)
	}
}

func TestParseEffectGlow(t *testing.T) {
	effect, err := ParseEffect(map[string]interface{}{
		"glow": map[string]interface{}{
			"srgbClr": map[string]interface{}{
				"val":   "00FF00",
				"alpha": 75000.0,
			},
			"rad": 127000.0,
		},
	})
	if err != nil {
		t.Fatalf("ParseEffect error: %v", err)
	}
	glow, ok := effect.(Glow)
	if !ok {
		t.Fatalf("expected Glow, got %T", effect)
	}
	if glow.Color != "00FF00" || glow.Alpha != 0.75 || glow.Radius != 10.0 {
		t.Errorf("glow = %+v", glow)
	}
}

func TestParseEffectSoftEdge(t *testing.T) {
	effect, err := ParseEffect(map[string]interface{}{
		"softEdge": map[string]interface{}{
			"rad": 127000.0,
		},
	})
	if err != nil {
		t.Fatalf("ParseEffect error: %v", err)
	}
	se, ok := effect.(SoftEdge)
	if !ok {
		t.Fatalf("expected SoftEdge, got %T", effect)
	}
	if se.Radius != 10.0 {
		t.Errorf("Radius = %v, want 10.0", se.Radius)
	}
}

func TestParseEffectUnknown(t *testing.T) {
	effect, err := ParseEffect(map[string]interface{}{})
	if err != nil {
		t.Fatalf("ParseEffect error: %v", err)
	}
	if effect != nil {
		t.Fatalf("expected nil for empty data, got %T", effect)
	}
}

func TestRichPresentationEffectMetadataTypes(t *testing.T) {
	shadow := Shadow{Alignment: "br", RotateWithShape: true, ScaleX: 1.25, ScaleY: 0.75, SkewX: 10, SkewY: -5}
	if shadow.Alignment != "br" || !shadow.RotateWithShape || shadow.ScaleX != 1.25 || shadow.SkewY != -5 {
		t.Fatalf("unexpected shadow metadata: %#v", shadow)
	}
	reflection := Reflection{StartAlpha: 60000, StartPosition: 10000, EndAlpha: 5000, EndPosition: 90000, FadeDirection: 90, ScaleX: .9, ScaleY: .6, Alignment: "b", RotateWithShape: true, ContainerType: "tree"}
	if reflection.EndPosition != 90000 || reflection.ScaleY != .6 || reflection.Alignment != "b" || !reflection.RotateWithShape || reflection.ContainerType != "tree" {
		t.Fatalf("unexpected reflection metadata: %#v", reflection)
	}
}
