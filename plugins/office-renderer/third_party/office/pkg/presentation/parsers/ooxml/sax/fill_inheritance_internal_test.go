package sax

import (
	"testing"

	"office/pkg/ooxml/drawing"
	ir "office/pkg/presentation/ir"
)

func TestInheritVisualFactsExplicitNoFillBlocksTemplateFill(t *testing.T) {
	dst := &ir.Shape{Fill: drawing.NoFill{}}
	src := &ir.Shape{Fill: drawing.SolidFill{Color: "FFFFFF", Alpha: 1}, FillColor: "FFFFFF"}
	inheritVisualFacts(dst, src)
	if _, ok := dst.Fill.(drawing.NoFill); !ok {
		t.Fatalf("fill = %#v, want authored drawing.NoFill", dst.Fill)
	}
	if dst.FillColor != "" {
		t.Fatalf("fill color = %q, want empty explicit no-fill", dst.FillColor)
	}
}
