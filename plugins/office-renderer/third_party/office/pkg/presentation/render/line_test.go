package render

import (
	"testing"

	"office/pkg/ooxml/drawing"
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