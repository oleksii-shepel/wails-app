package drawing

import (
	"math"
	"testing"
)

func TestApproximateOpacity(t *testing.T) {
	tests := []struct {
		name    string
		el      EffectList
		want    float64
		wantOk  bool
	}{
		{
			name:   "empty list",
			el:     EffectList{},
			want:   1.0,
			wantOk: false,
		},
		{
			name:   "single shadow",
			el:     EffectList{Effects: []Effect{Shadow{Alpha: 0.5}}},
			want:   0.5,
			wantOk: true,
		},
		{
			name:   "shadow with alpha mod",
			el:     EffectList{Effects: []Effect{Shadow{Alpha: 0.5}, AlphaMod{Amount: 50000}}},
			want:   0.25,
			wantOk: true,
		},
		{
			name:   "alpha replace",
			el:     EffectList{Effects: []Effect{AlphaRepl{Alpha: 80000}}},
			want:   0.8,
			wantOk: true,
		},
		{
			name:   "alpha offset",
			el:     EffectList{Effects: []Effect{AlphaOff{Amount: 20000}}},
			want:   1.0, // clamped to [0,1]
			wantOk: true,
		},
		{
			name:   "alpha mod fix",
			el:     EffectList{Effects: []Effect{AlphaModFix{Amount: 30000}}},
			want:   0.3,
			wantOk: true,
		},
		{
			name:   "alpha bilevel",
			el:     EffectList{Effects: []Effect{AlphaBiLevel{Threshold: 50000}}},
			want:   0.5,
			wantOk: true,
		},
		{
			name:   "glow",
			el:     EffectList{Effects: []Effect{Glow{Alpha: 0.75}}},
			want:   0.75,
			wantOk: true,
		},
		{
			name:   "inner shadow",
			el:     EffectList{Effects: []Effect{InnerShadow{Alpha: 0.4}}},
			want:   0.4,
			wantOk: true,
		},
		{
			name:   "reflection",
			el:     EffectList{Effects: []Effect{Reflection{Alpha: 0.6}}},
			want:   0.6,
			wantOk: true,
		},
		{
			name:   "solid fill overlay",
			el:     EffectList{Effects: []Effect{FillOverlay{Fill: SolidFill{Color: "FF0000", Alpha: 0.5}}}},
			want:   0.5,
			wantOk: true,
		},
		{
			name:   "gradient fill overlay",
			el:     EffectList{Effects: []Effect{FillOverlay{Fill: GradientFill{Stops: []GradientStop{{Alpha: 0.2}, {Alpha: 0.8}}}}}},
			want:   0.5,
			wantOk: true,
		},
		{
			name:   "pattern fill overlay",
			el:     EffectList{Effects: []Effect{FillOverlay{Fill: PatternFill{ForeAlpha: 0.3, BackAlpha: 0.1}}}},
			want:   0.3,
			wantOk: true,
		},
		{
			name:   "picture fill overlay",
			el:     EffectList{Effects: []Effect{FillOverlay{Fill: PictureFill{Alpha: 0.7}}}},
			want:   0.7,
			wantOk: true,
		},
		{
			name:   "shadow then glow",
			el:     EffectList{Effects: []Effect{Shadow{Alpha: 0.5}, Glow{Alpha: 0.5}}},
			want:   0.25,
			wantOk: true,
		},
		{
			name:   "zero alpha shadow",
			el:     EffectList{Effects: []Effect{Shadow{Alpha: 0}}},
			want:   1.0,
			wantOk: false,
		},
		{
			name:   "pointer alpha mod",
			el:     EffectList{Effects: []Effect{&AlphaMod{Amount: 50000}}},
			want:   0.5,
			wantOk: true,
		},
		{
			name:   "nil pointer alpha mod",
			el:     EffectList{Effects: []Effect{(*AlphaMod)(nil)}},
			want:   1.0,
			wantOk: false,
		},
		{
			name:   "nil pointer shadow",
			el:     EffectList{Effects: []Effect{(*Shadow)(nil)}},
			want:   1.0,
			wantOk: false,
		},
		{
			name:   "non-opacity effect type",
			el:     EffectList{Effects: []Effect{Blur{Radius: 5}}},
			want:   1.0,
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ApproximateOpacity(tt.el)
			if ok != tt.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOk)
			}
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("opacity = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApproximateFillOpacityNilFill(t *testing.T) {
	el := EffectList{Effects: []Effect{FillOverlay{Fill: nil}}}
	got, ok := ApproximateOpacity(el)
	if ok {
		t.Fatalf("expected false for nil fill, got %v", got)
	}
}

func TestApproximateFillOpacityEmptyGradient(t *testing.T) {
	el := EffectList{Effects: []Effect{FillOverlay{Fill: GradientFill{Stops: []GradientStop{}}}}}
	got, ok := ApproximateOpacity(el)
	if ok {
		t.Fatalf("expected false for empty gradient, got %v", got)
	}
}

func TestApproximateFillOpacityNilGradientPointer(t *testing.T) {
	el := EffectList{Effects: []Effect{FillOverlay{Fill: (*GradientFill)(nil)}}}
	got, ok := ApproximateOpacity(el)
	if ok {
		t.Fatalf("expected false for nil gradient pointer, got %v", got)
	}
}

func TestApproximateFillOpacityNilPatternPointer(t *testing.T) {
	el := EffectList{Effects: []Effect{FillOverlay{Fill: (*PatternFill)(nil)}}}
	got, ok := ApproximateOpacity(el)
	if ok {
		t.Fatalf("expected false for nil pattern pointer, got %v", got)
	}
}

func TestApproximateFillOpacityNilPicturePointer(t *testing.T) {
	el := EffectList{Effects: []Effect{FillOverlay{Fill: (*PictureFill)(nil)}}}
	got, ok := ApproximateOpacity(el)
	if ok {
		t.Fatalf("expected false for nil picture pointer, got %v", got)
	}
}

func TestApproximateFillOpacityNilSolidPointer(t *testing.T) {
	el := EffectList{Effects: []Effect{FillOverlay{Fill: (*SolidFill)(nil)}}}
	got, ok := ApproximateOpacity(el)
	if ok {
		t.Fatalf("expected false for nil solid pointer, got %v", got)
	}
}

func TestApproximateFillOpacityUnknownFill(t *testing.T) {
	el := EffectList{Effects: []Effect{FillOverlay{Fill: NoFill{}}}}
	got, ok := ApproximateOpacity(el)
	if ok {
		t.Fatalf("expected false for unknown fill, got %v", got)
	}
}

func TestApproximateFillOpacityNilInterface(t *testing.T) {
	got, ok := approximateFillOpacity(nil)
	if ok || got != 0 {
		t.Errorf("nil interface = (%v,%v), want (0,false)", got, ok)
	}
}

func TestApproximateFillOpacityNoFill(t *testing.T) {
	got, ok := approximateFillOpacity(NoFill{})
	if ok || got != 0 {
		t.Errorf("NoFill = (%v,%v), want (0,false)", got, ok)
	}
}

func TestApproximateFillOpacitySolidFillPointer(t *testing.T) {
	f := SolidFill{Alpha: 0.5}
	got, ok := approximateFillOpacity(&f)
	if !ok || math.Abs(got-0.5) > 1e-9 {
		t.Errorf("SolidFill ptr = %v, want 0.5", got)
	}
}

func TestApproximateFillOpacityGradientFillPointer(t *testing.T) {
	gf := GradientFill{Stops: []GradientStop{{Alpha: 0.2}, {Alpha: 0.8}}}
	got, ok := approximateFillOpacity(&gf)
	if !ok || math.Abs(got-0.5) > 1e-9 {
		t.Errorf("GradientFill ptr = %v, want 0.5", got)
	}
}

func TestApproximateFillOpacityPatternFillPointer(t *testing.T) {
	pf := PatternFill{ForeAlpha: 0.3, BackAlpha: 0.1}
	got, ok := approximateFillOpacity(&pf)
	if !ok || math.Abs(got-0.3) > 1e-9 {
		t.Errorf("PatternFill ptr = %v, want 0.3", got)
	}
}

func TestApproximateFillOpacityPictureFillPointer(t *testing.T) {
	pf := PictureFill{Alpha: 0.7}
	got, ok := approximateFillOpacity(&pf)
	if !ok || math.Abs(got-0.7) > 1e-9 {
		t.Errorf("PictureFill ptr = %v, want 0.7", got)
	}
}

func TestApproximateFillOpacityNilFillOverlayPointer(t *testing.T) {
	el := EffectList{Effects: []Effect{(*FillOverlay)(nil)}}
	got, ok := ApproximateOpacity(el)
	if ok {
		t.Fatalf("expected false for nil FillOverlay pointer, got %v", got)
	}
}

func TestApproximateEffectOpacityNilInnerShadow(t *testing.T) {
	el := EffectList{Effects: []Effect{(*InnerShadow)(nil)}}
	got, ok := ApproximateOpacity(el)
	if ok {
		t.Fatalf("expected false for nil InnerShadow, got %v", got)
	}
}

func TestApproximateEffectOpacityNilReflection(t *testing.T) {
	el := EffectList{Effects: []Effect{(*Reflection)(nil)}}
	got, ok := ApproximateOpacity(el)
	if ok {
		t.Fatalf("expected false for nil Reflection, got %v", got)
	}
}

func TestApproximateEffectOpacityNilGlow(t *testing.T) {
	el := EffectList{Effects: []Effect{(*Glow)(nil)}}
	got, ok := ApproximateOpacity(el)
	if ok {
		t.Fatalf("expected false for nil Glow, got %v", got)
	}
}

func TestApproximateEffectOpacityPointerShadow(t *testing.T) {
	el := EffectList{Effects: []Effect{&Shadow{Alpha: 0.5}}}
	got, ok := ApproximateOpacity(el)
	if !ok || math.Abs(got-0.5) > 1e-9 {
		t.Errorf("Shadow ptr = %v, want 0.5", got)
	}
}

func TestApproximateAlphaValueZero(t *testing.T) {
	got, ok := approximateAlphaValue(0)
	if ok {
		t.Fatalf("expected false for alpha=0, got %v", got)
	}
	if got != 0 {
		t.Errorf("alpha=0 = %v, want 0", got)
	}
}

func TestApproximateAlphaValueNegative(t *testing.T) {
	got, ok := approximateAlphaValue(-0.5)
	if ok {
		t.Fatalf("expected false for negative alpha, got %v", got)
	}
	if got != 0 {
		t.Errorf("alpha=-0.5 = %v, want 0", got)
	}
}

func TestApproximateFillOpacityGradientWithZeroAlphaStop(t *testing.T) {
	gf := GradientFill{Stops: []GradientStop{{Alpha: 0}, {Alpha: 1.0}}}
	got, ok := approximateFillOpacity(gf)
	if !ok {
		t.Fatal("expected true")
	}
	// (1 + 1.0) / 2 = 1.0 because zero alpha gets replaced with 1
	if got != 1.0 {
		t.Errorf("gradient with zero stop = %v, want 1.0", got)
	}
}

func TestApplyOpacityEffectPointerVariants(t *testing.T) {
	// AlphaModFix pointer: current * (amount/100000)
	got, ok := applyOpacityEffect(&AlphaModFix{Amount: 60000}, 0.5)
	if !ok || math.Abs(got-0.3) > 1e-9 {
		t.Errorf("AlphaModFix ptr = %v, want 0.3", got)
	}

	// AlphaRepl pointer
	got2, ok2 := applyOpacityEffect(&AlphaRepl{Alpha: 80000}, 0.5)
	if !ok2 || math.Abs(got2-0.8) > 1e-9 {
		t.Errorf("AlphaRepl ptr = %v, want 0.8", got2)
	}

	// AlphaOff pointer: current + amount/100000, clamped
	got3, ok3 := applyOpacityEffect(&AlphaOff{Amount: 10000}, 0.5)
	if !ok3 || math.Abs(got3-0.6) > 1e-9 {
		t.Errorf("AlphaOff ptr = %v, want 0.6", got3)
	}

	// AlphaBiLevel pointer: current * (threshold/100000)
	got4, ok4 := applyOpacityEffect(&AlphaBiLevel{Threshold: 100000}, 0.5)
	if !ok4 || math.Abs(got4-0.5) > 1e-9 {
		t.Errorf("AlphaBiLevel ptr = %v, want 0.5", got4)
	}
}

func TestApplyOpacityEffectNilPointers(t *testing.T) {
	for _, eff := range []Effect{(*AlphaModFix)(nil), (*AlphaRepl)(nil), (*AlphaOff)(nil), (*AlphaBiLevel)(nil)} {
		got, ok := applyOpacityEffect(eff, 0.5)
		if ok || got != 0 {
			t.Errorf("nil pointer = (%v,%v), want (0,false)", got, ok)
		}
	}
}

func TestApproximateEffectOpacityPointerVariants(t *testing.T) {
	// InnerShadow pointer
	got, ok := approximateEffectOpacity(&InnerShadow{Alpha: 0.4})
	if !ok || math.Abs(got-0.4) > 1e-9 {
		t.Errorf("InnerShadow ptr = %v, want 0.4", got)
	}

	// Glow pointer
	got2, ok2 := approximateEffectOpacity(&Glow{Alpha: 0.75})
	if !ok2 || math.Abs(got2-0.75) > 1e-9 {
		t.Errorf("Glow ptr = %v, want 0.75", got2)
	}

	// Reflection pointer
	got3, ok3 := approximateEffectOpacity(&Reflection{Alpha: 0.6})
	if !ok3 || math.Abs(got3-0.6) > 1e-9 {
		t.Errorf("Reflection ptr = %v, want 0.6", got3)
	}

	// FillOverlay pointer with solid fill
	got4, ok4 := approximateEffectOpacity(&FillOverlay{Fill: SolidFill{Alpha: 0.5}})
	if !ok4 || math.Abs(got4-0.5) > 1e-9 {
		t.Errorf("FillOverlay ptr = %v, want 0.5", got4)
	}
}

func TestApproximateEffectOpacityNilFillOverlayFill(t *testing.T) {
	got, ok := approximateEffectOpacity(&FillOverlay{Fill: nil})
	if ok {
		t.Fatalf("expected false, got %v", got)
	}
}

func TestClampFloat(t *testing.T) {
	if got := clampFloat(-0.5, 0, 1); got != 0 {
		t.Errorf("clampFloat(-0.5) = %v, want 0", got)
	}
	if got := clampFloat(1.5, 0, 1); got != 1 {
		t.Errorf("clampFloat(1.5) = %v, want 1", got)
	}
	if got := clampFloat(0.5, 0, 1); got != 0.5 {
		t.Errorf("clampFloat(0.5) = %v, want 0.5", got)
	}
}
