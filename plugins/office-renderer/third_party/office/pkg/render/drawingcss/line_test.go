package drawingcss_test

import (
	"testing"

	"office/pkg/ooxml/drawing"
	drawingcss "office/pkg/render/drawingcss"
)

func TestLineStyleCSS(t *testing.T) {
	tests := []struct {
		name      string
		lineStyle drawing.LineStyle
		wantCSS   map[string]string
	}{
		{
			name:      "default line style",
			lineStyle: drawing.DefaultLineStyle(),
			wantCSS: map[string]string{
				"stroke":            "#000000",
				"stroke-width":      "1.0pt",
				"stroke-linecap":    "butt",
				"stroke-linejoin":   "miter",
				"stroke-miterlimit": "10.0",
				"stroke-dasharray":  "none",
			},
		},
		{
			name: "round cap and join",
			lineStyle: drawing.LineStyle{
				Width:      2.0,
				Color:      "FF0000",
				Alpha:      1.0,
				Cap:        drawing.LineCapRound,
				Join:       drawing.LineJoinRound,
				MiterLimit: 10.0,
				Dash:       drawing.DashPatternDash,
			},
			wantCSS: map[string]string{
				"stroke":           "#FF0000",
				"stroke-width":     "2.0pt",
				"stroke-linecap":   "round",
				"stroke-linejoin":  "round",
				"stroke-dasharray": "3, 3",
			},
		},
		{
			name: "square cap and bevel join with miter limit",
			lineStyle: drawing.LineStyle{
				Width:      1.5,
				Color:      "00FF00",
				Alpha:      1.0,
				Cap:        drawing.LineCapSquare,
				Join:       drawing.LineJoinBevel,
				MiterLimit: 8.0,
				Dash:       drawing.DashPatternDashDot,
			},
			wantCSS: map[string]string{
				"stroke":           "#00FF00",
				"stroke-width":     "1.5pt",
				"stroke-linecap":   "square",
				"stroke-linejoin":  "bevel",
				"stroke-dasharray": "3, 3, 1, 3",
			},
		},
		{
			name: "miter join with custom limit",
			lineStyle: drawing.LineStyle{
				Width:      3.0,
				Color:      "0000FF",
				Alpha:      1.0,
				Cap:        drawing.LineCapFlat,
				Join:       drawing.LineJoinMiter,
				MiterLimit: 15.5,
				Dash:       drawing.DashPatternLongDashDotDot,
			},
			wantCSS: map[string]string{
				"stroke":            "#0000FF",
				"stroke-width":      "3.0pt",
				"stroke-linecap":    "butt",
				"stroke-linejoin":   "miter",
				"stroke-miterlimit": "15.5",
				"stroke-dasharray":  "5, 3, 1, 3, 1, 3",
			},
		},
		{
			name: "solid line (no dash)",
			lineStyle: drawing.LineStyle{
				Width:      1.0,
				Color:      "FFFFFF",
				Alpha:      1.0,
				Cap:        drawing.LineCapFlat,
				Join:       drawing.LineJoinMiter,
				MiterLimit: 4.0,
				Dash:       drawing.DashPatternSolid,
			},
			wantCSS: map[string]string{
				"stroke":            "#FFFFFF",
				"stroke-width":      "1.0pt",
				"stroke-linecap":    "butt",
				"stroke-linejoin":   "miter",
				"stroke-miterlimit": "4.0",
				"stroke-dasharray":  "none",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := drawingcss.LineStyle(tt.lineStyle)

			for key, wantValue := range tt.wantCSS {
				gotValue, ok := got[key]
				if !ok {
					t.Errorf("missing CSS property %q", key)
					continue
				}
				if gotValue != wantValue {
					t.Errorf("CSS[%q] = %q, want %q", key, gotValue, wantValue)
				}
			}

			for key := range got {
				if _, ok := tt.wantCSS[key]; !ok {
					t.Errorf("unexpected CSS property %q = %q", key, got[key])
				}
			}
		})
	}
}
