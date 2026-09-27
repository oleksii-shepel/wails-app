package legacy

import (
	"math"
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestStep4RotatedAnchorBoundsBecomeCanonicalPreRotationFrame(t *testing.T) {
	tests := []struct {
		name     string
		rotation float64
		in       presentationir.Rect
		want     presentationir.Rect
	}{
		{
			name:     "clockwise 90",
			rotation: 90,
			in:       presentationir.Rect{X: 4284, Y: 972, W: 480, H: 1920},
			want:     presentationir.Rect{X: 3564, Y: 1692, W: 1920, H: 480},
		},
		{
			name:     "clockwise 270 odd dimensions",
			rotation: 270,
			in:       presentationir.Rect{X: 4115, Y: 3646, W: 117, H: 192},
			want:     presentationir.Rect{X: 4078, Y: 3683, W: 192, H: 117},
		},
		{
			name:     "negative quarter turn",
			rotation: -90,
			in:       presentationir.Rect{X: 100, Y: 200, W: 40, H: 100},
			want:     presentationir.Rect{X: 70, Y: 230, W: 100, H: 40},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeLegacyPptRotatedAnchorBounds(tt.in, tt.rotation); got != tt.want {
				t.Fatalf("bounds = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestStep4RotatedAnchorNormalizationIsAppliedWithRotation(t *testing.T) {
	shape := &presentationir.Shape{Bounds: presentationir.Rect{X: 4284, Y: 972, W: 480, H: 1920}}
	applyLegacyPptShapeTransform(shape, legacyPptShapeTransform{HasRotation: true, Rotation: 90})
	want := presentationir.Rect{X: 3564, Y: 1692, W: 1920, H: 480}
	if shape.Bounds != want {
		t.Fatalf("shape bounds = %+v, want %+v", shape.Bounds, want)
	}
	if shape.RotationDeg != 90 {
		t.Fatalf("shape rotation = %v, want 90", shape.RotationDeg)
	}
}

func TestStep4NonTransposedRotationQuadrantsRemainAuthored(t *testing.T) {
	in := presentationir.Rect{X: 100, Y: 200, W: 300, H: 400}
	for _, rotation := range []float64{0, 15, 45, 135, 180, 225, 315, 360} {
		if got := normalizeLegacyPptRotatedAnchorBounds(in, rotation); got != in {
			t.Fatalf("rotation %v changed bounds to %+v, want %+v", rotation, got, in)
		}
	}
}

func TestStep4NonQuarterTurnTransposedQuadrantBecomesCanonicalPreRotationFrame(t *testing.T) {
	in := presentationir.Rect{X: 4284, Y: 972, W: 480, H: 1920}
	want := presentationir.Rect{X: 3564, Y: 1692, W: 1920, H: 480}
	for _, rotation := range []float64{51.19943333333333, 90, 128.80056666666667, 270, 308.8005676269531} {
		if got := normalizeLegacyPptRotatedAnchorBounds(in, rotation); got != want {
			t.Fatalf("rotation %v bounds = %+v, want %+v", rotation, got, want)
		}
	}
}

func TestStep4SingleAxisFlipConjugatesLegacyLeafRotation(t *testing.T) {
	shape := &presentationir.Shape{Bounds: presentationir.Rect{X: 100, Y: 200, W: 300, H: 400}}
	applyLegacyPptShapeTransform(shape, legacyPptShapeTransform{HasRotation: true, Rotation: 347.40965270996094, FlipH: true})
	want := 12.59
	if math.Abs(shape.RotationDeg-want) > 1e-9 {
		t.Fatalf("shape rotation = %.12f, want %.12f", shape.RotationDeg, want)
	}
	if !shape.FlipH || shape.FlipV {
		t.Fatalf("shape flips = (%t,%t), want (true,false)", shape.FlipH, shape.FlipV)
	}
}

func TestStep4TwoAxisFlipKeepsLegacyLeafRotationDirection(t *testing.T) {
	shape := &presentationir.Shape{Bounds: presentationir.Rect{X: 100, Y: 200, W: 300, H: 400}}
	applyLegacyPptShapeTransform(shape, legacyPptShapeTransform{HasRotation: true, Rotation: 31.25, FlipH: true, FlipV: true})
	if shape.RotationDeg != 31.25 {
		t.Fatalf("shape rotation = %v, want 31.25", shape.RotationDeg)
	}
}

func TestStep3NormalizePptTextPreservesPrivateUseGlyphs(t *testing.T) {
	got := normalizePptText("\uF073\r\uF044 \uF03E \uF030")
	want := "\uF073\n\uF044 \uF03E \uF030"
	if got != want {
		t.Fatalf("normalized symbol text = %q, want %q", got, want)
	}
}

func TestStep3SymbolPrivateUseRunKeepsSymbolTypeface(t *testing.T) {
	segments := legacyPptSplitCFRunByTypeface(stpaCFRun{
		FontFamily:       "Arial",
		AnsiFontFamily:   "Arial",
		SymbolFontFamily: "Symbol",
	}, "\uF044 \uF03E \uF030")
	if len(segments) != 1 {
		t.Fatalf("segments = %#v, want one Symbol segment", segments)
	}
	if segments[0].Text != "\uF044 \uF03E \uF030" || segments[0].FontFamily != "Symbol" {
		t.Fatalf("segment = %#v, want Symbol private-use glyph run", segments[0])
	}
}
