package render

import (
	"strings"
	"testing"

	"office/pkg/ooxml/drawing"
	playout "office/pkg/presentation/layout"
)

func TestCSSBorderStyleForLine(t *testing.T) {
	tests := []struct {
		name  string
		style *drawing.LineStyle
		want  string
	}{
		{"nil", nil, "solid"},
		{"solid", &drawing.LineStyle{Dash: drawing.DashPatternSolid}, "solid"},
		{"dashed", &drawing.LineStyle{Dash: drawing.DashPatternDash}, "dashed"},
		{"double", &drawing.LineStyle{CompoundType: "dbl"}, "double"},
		{"thickThin fallback", &drawing.LineStyle{CompoundType: "thickThin"}, "double"},
		{"triple fallback", &drawing.LineStyle{CompoundType: "tri"}, "double"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cssBorderStyleForLine(tt.style); got != tt.want {
				t.Errorf("cssBorderStyleForLine() = %q, want %q", got, tt.want)
			}
		})
	}
}
func TestWriteSVGStrokePlanMaterializesPhysicalAlphaAndScaledDash(t *testing.T) {
	var b strings.Builder
	writeSVGStrokePlan(&b, playout.StrokeVisualPlan{
		Color: "123456", WidthPt: 2, SVGLineCap: "round", SVGLineJoin: "miter",
		MiterLimit: 8, DashArray: "2, 6", Alpha: 0.4, AlphaSet: true,
	}, "", "")
	got := b.String()
	for _, want := range []string{
		`stroke="#123456"`, `stroke-width="2.00"`, `stroke-linecap="round"`,
		`stroke-linejoin="miter"`, `stroke-miterlimit="8.00"`, `stroke-dasharray="2, 6"`,
		`stroke-opacity="0.4000"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stroke output %q missing %q", got, want)
		}
	}
}
