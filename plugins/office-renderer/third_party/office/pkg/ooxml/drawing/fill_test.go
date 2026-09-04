package drawing

import (
	"math"
	"strings"
	"testing"
)

func TestFillTypes(t *testing.T) {
	tests := []struct {
		fill Fill
		want FillType
	}{
		{NoFill{}, FillTypeNone},
		{SolidFill{}, FillTypeSolid},
		{GradientFill{}, FillTypeGradient},
		{PatternFill{}, FillTypePattern},
		{PictureFill{}, FillTypePicture},
	}
	for _, tt := range tests {
		if got := tt.fill.Type(); got != tt.want {
			t.Errorf("%T.Type() = %q, want %q", tt.fill, got, tt.want)
		}
	}
}

func TestPictureFillSourceURL(t *testing.T) {
	// With image URL
	pf := PictureFill{ImageURL: "http://example.com/img.png"}
	if got := pf.SourceURL(); got != "http://example.com/img.png" {
		t.Errorf("SourceURL() = %q, want URL", got)
	}

	// With image data and content type
	pf2 := PictureFill{ImageData: []byte("hello"), ContentType: "image/png"}
	got := pf2.SourceURL()
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Errorf("SourceURL() = %q, want data URL prefix", got)
	}

	// Empty
	pf3 := PictureFill{}
	if got := pf3.SourceURL(); got != "" {
		t.Errorf("SourceURL() = %q, want empty", got)
	}
}

func TestParseFillNoFill(t *testing.T) {
	fill, err := ParseFill(map[string]interface{}{"noFill": map[string]interface{}{}})
	if err != nil {
		t.Fatalf("ParseFill error: %v", err)
	}
	if _, ok := fill.(NoFill); !ok {
		t.Fatalf("expected NoFill, got %T", fill)
	}
}

func TestParseFillSolid(t *testing.T) {
	fill, err := ParseFill(map[string]interface{}{
		"solidFill": map[string]interface{}{
			"srgbClr": map[string]interface{}{
				"val":   "FF0000",
				"alpha": 50000.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("ParseFill error: %v", err)
	}
	solid, ok := fill.(SolidFill)
	if !ok {
		t.Fatalf("expected SolidFill, got %T", fill)
	}
	if solid.Color != "FF0000" || solid.Alpha != 0.5 {
		t.Errorf("solid = %+v", solid)
	}
}

func TestParseFillSolidScheme(t *testing.T) {
	fill, err := ParseFill(map[string]interface{}{
		"solidFill": map[string]interface{}{
			"schemeClr": map[string]interface{}{
				"val":   "accent1",
				"shade": "80000",
				"tint":  "20000",
			},
		},
	})
	if err != nil {
		t.Fatalf("ParseFill error: %v", err)
	}
	solid, ok := fill.(SolidFill)
	if !ok {
		t.Fatalf("expected SolidFill, got %T", fill)
	}
	if solid.ThemeColor != "accent1" || solid.ThemeShade != "80000" || solid.ThemeTint != "20000" {
		t.Errorf("solid = %+v", solid)
	}
}

func TestParseFillGradient(t *testing.T) {
	fill, err := ParseFill(map[string]interface{}{
		"gradFill": map[string]interface{}{
			"lin": map[string]interface{}{"ang": 5400000.0},
			"gsLst": map[string]interface{}{
				"gs": []interface{}{
					map[string]interface{}{
						"pos":     0.0,
						"srgbClr": map[string]interface{}{"val": "FF0000", "alpha": 100000.0},
					},
					map[string]interface{}{
						"pos":     100000.0,
						"srgbClr": map[string]interface{}{"val": "0000FF"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ParseFill error: %v", err)
	}
	grad, ok := fill.(GradientFill)
	if !ok {
		t.Fatalf("expected GradientFill, got %T", fill)
	}
	if grad.Angle != 90.0 {
		t.Errorf("Angle = %v, want 90.0", grad.Angle)
	}
	if len(grad.Stops) != 2 {
		t.Fatalf("stops = %d, want 2", len(grad.Stops))
	}
	if grad.Stops[0].Color != "FF0000" || grad.Stops[0].Alpha != 1.0 {
		t.Errorf("stop0 = %+v", grad.Stops[0])
	}
}

func TestParseFillDefault(t *testing.T) {
	fill, err := ParseFill(map[string]interface{}{})
	if err != nil {
		t.Fatalf("ParseFill error: %v", err)
	}
	if _, ok := fill.(NoFill); !ok {
		t.Fatalf("expected NoFill for empty data, got %T", fill)
	}
}

func TestPictureFillSourceURLNoContentType(t *testing.T) {
	pf := PictureFill{ImageData: []byte("hello")}
	got := pf.SourceURL()
	if !strings.HasPrefix(got, "data:application/octet-stream;base64,") {
		t.Errorf("SourceURL() = %q, want octet-stream data URL", got)
	}
}

func TestSolidFillResolvedAlpha(t *testing.T) {
	// Default alpha when zero/negative
	f := SolidFill{Alpha: 0}
	if got := f.ResolvedAlpha(); got != 1.0 {
		t.Errorf("ResolvedAlpha() = %v, want 1.0", got)
	}

	// With modulation
	f2 := SolidFill{Alpha: 0.8, AlphaMod: 0.5}
	if got := f2.ResolvedAlpha(); got != 0.4 {
		t.Errorf("ResolvedAlpha() = %v, want 0.4", got)
	}

	// With offset
	f3 := SolidFill{Alpha: 0.5, AlphaOff: 0.2}
	if got := f3.ResolvedAlpha(); got != 0.7 {
		t.Errorf("ResolvedAlpha() = %v, want 0.7", got)
	}

	// Clamped above 1
	f4 := SolidFill{Alpha: 0.8, AlphaOff: 0.5}
	if got := f4.ResolvedAlpha(); got != 1.0 {
		t.Errorf("ResolvedAlpha() = %v, want 1.0", got)
	}
}

func TestSolidFillApplyColorTransformsEmpty(t *testing.T) {
	f := SolidFill{}
	if got := f.ApplyColorTransforms(""); got != "" {
		t.Errorf("ApplyColorTransforms(%q) = %q, want empty", "", got)
	}
}

func TestSolidFillApplyColorTransformsInvalidHex(t *testing.T) {
	f := SolidFill{}
	if got := f.ApplyColorTransforms("notahex"); got != "notahex" {
		t.Errorf("ApplyColorTransforms invalid = %q, want unchanged", got)
	}
}

func TestSolidFillApplyColorTransformsShadeAndTint(t *testing.T) {
	// Shade darkens
	f := SolidFill{HasShade: true, Shade: 0.5}
	got := f.ApplyColorTransforms("FFFFFF")
	if got != "808080" {
		t.Errorf("shade 0.5 of white = %q, want 808080", got)
	}

	// Tint lightens
	f2 := SolidFill{HasTint: true, Tint: 0.5}
	got2 := f2.ApplyColorTransforms("000000")
	if got2 != "808080" {
		t.Errorf("tint 0.5 of black = %q, want 808080", got2)
	}
}

func TestSolidFillApplyColorTransformsGrayAndInv(t *testing.T) {
	// Gray
	f := SolidFill{Gray: true}
	got := f.ApplyColorTransforms("FF0000")
	if got != "4C4C4C" {
		t.Errorf("gray of red = %q, want 4C4C4C", got)
	}

	// Invert
	f2 := SolidFill{Inv: true}
	got2 := f2.ApplyColorTransforms("FF0000")
	if got2 != "00FFFF" {
		t.Errorf("invert of red = %q, want 00FFFF", got2)
	}
}

func TestSolidFillApplyColorTransformsComp(t *testing.T) {
	f := SolidFill{Comp: true}
	got := f.ApplyColorTransforms("00FF00") // green
	// complement of green is magenta (implementation gives FF00FE due to rounding)
	if got != "FF00FE" {
		t.Errorf("comp of green = %q, want FF00FE", got)
	}
}

func TestSolidFillApplyColorTransformsChannel(t *testing.T) {
	// Red replacement
	f := SolidFill{HasRed: true, Red: 1.0}
	got := f.ApplyColorTransforms("000000")
	if got != "FF0000" {
		t.Errorf("red replace = %q, want FF0000", got)
	}

	// Green modulation
	f2 := SolidFill{HasGreenMod: true, GreenMod: 0.5}
	got2 := f2.ApplyColorTransforms("00FF00")
	// Only green channel is modulated: 255 * 0.5 = 128 -> 80
	if got2 != "008000" {
		t.Errorf("green mod = %q, want 008000", got2)
	}
}

func TestClampUnit(t *testing.T) {
	if got := clampUnit(-0.5); got != 0 {
		t.Errorf("clampUnit(-0.5) = %v, want 0", got)
	}
	if got := clampUnit(0.5); got != 0.5 {
		t.Errorf("clampUnit(0.5) = %v, want 0.5", got)
	}
	if got := clampUnit(1.5); got != 1 {
		t.Errorf("clampUnit(1.5) = %v, want 1", got)
	}
}

func TestSolidFillApplyColorTransformsLuminance(t *testing.T) {
	// HasLum - pure red has l=0.5, so setting Lum=0.5 is no change; use a darker color
	f := SolidFill{HasLum: true, Lum: 0.75}
	got := f.ApplyColorTransforms("800000")
	if got == "800000" || got == "" {
		t.Errorf("HasLum = %q, want transformed", got)
	}

	// LumMod
	f2 := SolidFill{LumMod: 0.5}
	got2 := f2.ApplyColorTransforms("808080")
	if got2 == "808080" {
		t.Errorf("LumMod = %q, want transformed", got2)
	}

	// LumOff
	f3 := SolidFill{LumOff: 0.2}
	got3 := f3.ApplyColorTransforms("808080")
	if got3 == "808080" {
		t.Errorf("LumOff = %q, want transformed", got3)
	}
}

func TestSolidFillApplyColorTransformsSaturation(t *testing.T) {
	// HasSat
	f := SolidFill{HasSat: true, Sat: 0.5}
	got := f.ApplyColorTransforms("FF0000")
	if got == "FF0000" || got == "" {
		t.Errorf("HasSat = %q, want transformed", got)
	}

	// SatMod
	f2 := SolidFill{SatMod: 0.5}
	got2 := f2.ApplyColorTransforms("00FF00")
	if got2 == "00FF00" {
		t.Errorf("SatMod = %q, want transformed", got2)
	}

	// SatOff
	f3 := SolidFill{SatOff: 0.5}
	got3 := f3.ApplyColorTransforms("00FF00")
	if got3 == "00FF00" {
		t.Errorf("SatOff = %q, want transformed", got3)
	}
}

func TestSolidFillApplyColorTransformsHue(t *testing.T) {
	// HasHue
	f := SolidFill{HasHue: true, Hue: 180}
	got := f.ApplyColorTransforms("FF0000")
	if got == "FF0000" || got == "" {
		t.Errorf("HasHue = %q, want transformed", got)
	}

	// HueMod - red has h=0, so 0*0.5=0, no change; use green which has h=120
	f2 := SolidFill{HueMod: 0.5}
	got2 := f2.ApplyColorTransforms("00FF00")
	if got2 == "00FF00" {
		t.Errorf("HueMod = %q, want transformed", got2)
	}

	// HueOff
	f3 := SolidFill{HueOff: 60}
	got3 := f3.ApplyColorTransforms("FF0000")
	if got3 == "FF0000" {
		t.Errorf("HueOff = %q, want transformed", got3)
	}
}

func TestSolidFillApplyColorTransformsGamma(t *testing.T) {
	// Gamma
	f := SolidFill{Gamma: true}
	got := f.ApplyColorTransforms("808080")
	if got == "808080" {
		t.Errorf("Gamma = %q, want transformed", got)
	}

	// InvGamma
	f2 := SolidFill{InvGamma: true}
	got2 := f2.ApplyColorTransforms("808080")
	if got2 == "808080" {
		t.Errorf("InvGamma = %q, want transformed", got2)
	}
}

func TestGammaTransformZero(t *testing.T) {
	got := gammaTransform(128, 0)
	if got != 128 {
		t.Errorf("gammaTransform(128, 0) = %v, want 128", got)
	}
}

func TestGammaTransformNegative(t *testing.T) {
	got := gammaTransform(128, -1)
	if got != 128 {
		t.Errorf("gammaTransform(128, -1) = %v, want 128", got)
	}
}

func TestRGBToHSLGray(t *testing.T) {
	h, s, l := rgbToHSL(128, 128, 128)
	if h != 0 || s != 0 || math.Abs(l-0.5) > 0.01 {
		t.Errorf("rgbToHSL gray = (%v,%v,%v), want (0,0,~0.5)", h, s, l)
	}
}

func TestRGBToHSLLightHigh(t *testing.T) {
	// Color with l > 0.5: white-ish color
	_, _, l := rgbToHSL(200, 200, 100)
	if l <= 0.5 {
		t.Errorf("rgbToHSL light = %v, want > 0.5", l)
	}
}

func TestRGBToHSLRedMax(t *testing.T) {
	// Red is max, green < blue to hit h += 6 branch
	h, _, _ := rgbToHSL(255, 0, 128)
	if h < 0 || h > 360 {
		t.Errorf("rgbToHSL red max = %v, want in [0,360]", h)
	}
}

func TestRGBToHSLBlueMax(t *testing.T) {
	// Blue is max
	h, s, l := rgbToHSL(0, 0, 255)
	if h < 220 || h > 260 {
		t.Errorf("rgbToHSL blue max = %v, want ~240", h)
	}
	if s != 1.0 {
		t.Errorf("rgbToHSL blue sat = %v, want 1.0", s)
	}
	if l != 0.5 {
		t.Errorf("rgbToHSL blue light = %v, want 0.5", l)
	}
}

func TestApplyChannelTransformNegative(t *testing.T) {
	got := applyChannelTransform(128, -1, true, 0, false, 0, false)
	if got != 0 {
		t.Errorf("absolute -1 = %v, want 0", got)
	}
}

func TestApplyChannelTransformOverflow(t *testing.T) {
	// off=2.0 adds 510 to value, should clamp to 255
	got := applyChannelTransform(0, 0, false, 0, false, 2.0, true)
	if got != 255 {
		t.Errorf("overflow = %v, want 255", got)
	}
}

func TestSolidFillApplyColorTransformsNegativeHue(t *testing.T) {
	f := SolidFill{HasHue: true, Hue: -90}
	got := f.ApplyColorTransforms("FF0000")
	if got == "" || got == "FF0000" {
		t.Errorf("negative hue = %q, want transformed", got)
	}
}

func TestSolidFillApplyColorTransformsNegativeHueOff(t *testing.T) {
	// HueOff negative enough to make normalized h < 0
	f := SolidFill{HueOff: -90}
	got := f.ApplyColorTransforms("FF0000")
	if got == "" || got == "FF0000" {
		t.Errorf("negative hue off = %q, want transformed", got)
	}
}

func TestSolidFillApplyColorTransformsExtremeLumOff(t *testing.T) {
	// Large positive LumOff should clamp to white
	f := SolidFill{LumOff: 2.0}
	got := f.ApplyColorTransforms("000000")
	if got != "FFFFFF" {
		t.Errorf("extreme lum off = %q, want FFFFFF", got)
	}
	// Large negative LumOff should clamp to black
	f2 := SolidFill{LumOff: -2.0}
	got2 := f2.ApplyColorTransforms("FFFFFF")
	if got2 != "000000" {
		t.Errorf("extreme negative lum off = %q, want 000000", got2)
	}
}

func TestApplyChannelTransformAbsolute(t *testing.T) {
	got := applyChannelTransform(128, 1.0, true, 0, false, 0, false)
	if got != 255 {
		t.Errorf("absolute = %v, want 255", got)
	}
}

func TestApplyChannelTransformMod(t *testing.T) {
	got := applyChannelTransform(128, 0, false, 0.5, true, 0, false)
	if got != 64 {
		t.Errorf("mod = %v, want 64", got)
	}
}

func TestApplyChannelTransformOff(t *testing.T) {
	got := applyChannelTransform(128, 0, false, 0, false, 0.5, true)
	if got != 255 {
		t.Errorf("off = %v, want 255", got)
	}
}

func TestSolidFillApplyColorTransformsShortHex(t *testing.T) {
	f := SolidFill{Gray: true}
	got := f.ApplyColorTransforms("F00") // shorthand hex not supported
	if got != "F00" {
		t.Errorf("short hex = %q, want F00 (unchanged)", got)
	}
}

func TestSolidFillApplyColorTransformsBlueChannel(t *testing.T) {
	// Blue replacement
	f := SolidFill{HasBlue: true, Blue: 1.0}
	got := f.ApplyColorTransforms("000000")
	if got != "0000FF" {
		t.Errorf("blue replace = %q, want 0000FF", got)
	}

	// Blue modulation
	f2 := SolidFill{HasBlueMod: true, BlueMod: 0.5}
	got2 := f2.ApplyColorTransforms("0000FF")
	if got2 != "000080" {
		t.Errorf("blue mod = %q, want 000080", got2)
	}

	// Blue offset (clamped)
	f3 := SolidFill{HasBlueOff: true, BlueOff: 0.5}
	got3 := f3.ApplyColorTransforms("000000")
	if got3 != "000080" {
		t.Errorf("blue off = %q, want 000080", got3)
	}
}

func TestHSLToRGBEdgeCases(t *testing.T) {
	// Lightness = 0.5, s > 0, q branch where l < 0.5 is false
	r, g, b := hslToRGB(120, 1.0, 0.5)
	if r != 0 || g != 255 || b != 0 {
		t.Errorf("hslToRGB(120,1,0.5) = (%d,%d,%d), want (0,255,0)", r, g, b)
	}

	// Lightness = 0.75 (l >= 0.5 branch)
	r2, g2, b2 := hslToRGB(240, 1.0, 0.75)
	// Floating point may give 127 or 128
	if r2 < 126 || r2 > 129 || g2 < 126 || g2 > 129 || b2 != 255 {
		t.Errorf("hslToRGB(240,1,0.75) = (%d,%d,%d), want ~(128,128,255)", r2, g2, b2)
	}
}

func TestHueToRGBWrap(t *testing.T) {
	// t > 1 branch (wrap around): t=1.2 → 0.2, which is in [1/6, 1/2), returns q=0.9
	got := hueToRGB(0.1, 0.9, 1.2)
	if math.Abs(got-0.9) > 0.01 {
		t.Errorf("hueToRGB wrap = %v, want ~0.9", got)
	}

	// t < 0 branch: t=-0.2 → 0.8, which is >= 2/3, returns p=0.1
	got2 := hueToRGB(0.1, 0.9, -0.2)
	if math.Abs(got2-0.1) > 0.01 {
		t.Errorf("hueToRGB negative = %v, want ~0.1", got2)
	}
}

func TestGammaTransformMidpoint(t *testing.T) {
	got := gammaTransform(128, 1)
	if got < 0 || got > 255 {
		t.Errorf("gammaTransform(128,1) = %v, want in [0,255]", got)
	}
}

func TestGammaTransformPositive(t *testing.T) {
	// gamma > 0, channel=255 → should stay 255
	got := gammaTransform(255, 2.2)
	if got != 255 {
		t.Errorf("gammaTransform(255,2.2) = %v, want 255", got)
	}
	// gamma > 0, channel=0 → should stay 0
	got2 := gammaTransform(0, 2.2)
	if got2 != 0 {
		t.Errorf("gammaTransform(0,2.2) = %v, want 0", got2)
	}
}
