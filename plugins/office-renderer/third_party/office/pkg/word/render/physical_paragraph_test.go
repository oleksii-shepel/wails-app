package render

import (
	"testing"

	resolve "office/pkg/word/ir"
	"office/pkg/word/layout"
)

func TestCanRenderPhysicalRunsSequentially(t *testing.T) {
	tests := []struct {
		name string
		runs []layout.ParagraphRunLayout
		want bool
	}{
		{
			name: "tabbed line with ordinary text",
			runs: []layout.ParagraphRunLayout{
				{Run: resolve.Run{Text: "1."}},
				{Run: resolve.Run{Kind: "tab"}, Left: 12, Width: 24},
				{Run: resolve.Run{Text: "Text"}, Left: 36},
			},
			want: true,
		},
		{
			name: "explicit advance remains positioned",
			runs: []layout.ParagraphRunLayout{{Run: resolve.Run{FieldKind: "ADVANCE"}}},
		},
		{
			name: "indented run remains positioned",
			runs: []layout.ParagraphRunLayout{{Run: resolve.Run{Text: "Text"}, Left: 10}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canRenderPhysicalRunsSequentially(tt.runs); got != tt.want {
				t.Fatalf("canRenderPhysicalRunsSequentially() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestListMarkerFallbackFontSizeUsesFirstVisibleRun(t *testing.T) {
	size := 32 // half-points = 16pt
	p := &resolve.Paragraph{}
	runs := []resolve.Run{
		{Text: " "},
		{Text: "Item", Format: resolve.RunFormat{FontSize: &size}},
	}
	if got := listMarkerFallbackFontSizePt(p, runs); got != 16 {
		t.Fatalf("fallback marker size = %.1fpt, want 16pt", got)
	}
}
