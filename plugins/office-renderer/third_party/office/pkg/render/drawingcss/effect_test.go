package drawingcss_test

import (
	"testing"

	"office/pkg/ooxml/drawing"
	drawingcss "office/pkg/render/drawingcss"
)

func TestEffectListAppliesAlphaVariantsSequentially(t *testing.T) {
	tests := []struct {
		name    string
		effects []drawing.Effect
		want    string
	}{
		{
			name: "alpha mod plus offset",
			effects: []drawing.Effect{
				drawing.AlphaMod{Amount: 50000},
				drawing.AlphaOff{Amount: -10000},
			},
			want: "0.4000",
		},
		{
			name: "alpha replace then invert",
			effects: []drawing.Effect{
				drawing.AlphaRepl{Alpha: 25000},
				drawing.AlphaInv{},
			},
			want: "0.7500",
		},
		{
			name: "alpha ceiling",
			effects: []drawing.Effect{
				drawing.AlphaRepl{Alpha: 25000},
				drawing.AlphaCeiling{},
			},
			want: "1.0000",
		},
		{
			name: "alpha floor",
			effects: []drawing.Effect{
				drawing.AlphaRepl{Alpha: 25000},
				drawing.AlphaFloor{},
			},
			want: "0.0000",
		},
		{
			name: "alpha bilevel threshold",
			effects: []drawing.Effect{
				drawing.AlphaRepl{Alpha: 25000},
				drawing.AlphaBiLevel{Threshold: 20000},
			},
			want: "1.0000",
		},
		{
			name: "alpha bilevel threshold below cutoff",
			effects: []drawing.Effect{
				drawing.AlphaRepl{Alpha: 25000},
				drawing.AlphaBiLevel{Threshold: 30000},
			},
			want: "0.0000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := drawingcss.EffectList(drawing.EffectList{Effects: tt.effects})
			if got := c["opacity"]; got != tt.want {
				t.Fatalf("opacity = %q, want %q (css=%v)", got, tt.want, c)
			}
		})
	}
}

func TestAlphaOutsetApproximatesWithBlur(t *testing.T) {
	c := drawingcss.AlphaOutset(drawing.AlphaOutset{Radius: 3})
	if got := c["filter"]; got != "blur(3.0pt)" {
		t.Fatalf("filter = %q, want blur approximation", got)
	}
}

func TestEffectListCombinesDagTransformBlendAndTint(t *testing.T) {
	c := drawingcss.EffectList(drawing.EffectList{Effects: []drawing.Effect{
		drawing.RelativeOffset{X: 0.25, Y: -0.10},
		drawing.TransformEffect{ScaleX: 0.5, ScaleY: 1.25, TranslateX: 1, TranslateY: -2},
		drawing.BlendMode{Mode: "mult"},
		drawing.Tint{Hue: 1800000, Amount: 20000},
	}})

	if got := c["transform"]; got != "translate(25.00%, -10.00%) translate(1.0pt, -2.0pt) scale(0.5000, 1.2500)" {
		t.Fatalf("transform = %q", got)
	}
	if got := c["mix-blend-mode"]; got != "multiply" {
		t.Fatalf("mix-blend-mode = %q, want multiply", got)
	}
	if got := c["filter"]; got != "hue-rotate(30.00deg) brightness(120%)" {
		t.Fatalf("filter = %q", got)
	}
}

func TestEffectListApproximateOpacity(t *testing.T) {
	tests := []struct {
		name    string
		effects []drawing.Effect
		want    float64
	}{
		{
			name: "alpha pipeline",
			effects: []drawing.Effect{
				drawing.AlphaRepl{Alpha: 25000},
			},
			want: 0.25,
		},
		{
			name: "overlay fill alpha",
			effects: []drawing.Effect{
				drawing.FillOverlay{Fill: &drawing.SolidFill{Color: "00FF00", Alpha: 0.4}},
			},
			want: 0.4,
		},
		{
			name: "shadow alpha fallback",
			effects: []drawing.Effect{
				drawing.Glow{Color: "00FF00", Alpha: 0.6, Radius: 3},
			},
			want: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := drawingcss.ApproximateOpacity(drawing.EffectList{Effects: tt.effects})
			if !ok {
				t.Fatalf("ApproximateOpacity() = not found")
			}
			if got != tt.want {
				t.Fatalf("ApproximateOpacity() = %.4f, want %.4f", got, tt.want)
			}
		})
	}
}
