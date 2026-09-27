package sax

import (
	"office/pkg/presentation/ir"
	"testing"
)

func TestMaterializeDefaultNoAutofitAfterInheritance(t *testing.T) {
	plain := &ir.Shape{Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "plain"}}}}}
	inherited := &ir.Shape{Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "fit"}}}}}
	inherited.TextBodyProps.AutofitType = "normAutofit"
	materializeDefaultNoAutofitTree([]*ir.Shape{plain, inherited})
	if plain.TextBodyProps.AutofitType != "noAutofit" || !plain.TextAllowOverflow || plain.TextAutoGrow {
		t.Fatalf("default text fit = %+v allow=%v grow=%v", plain.TextBodyProps, plain.TextAllowOverflow, plain.TextAutoGrow)
	}
	if inherited.TextAllowOverflow || inherited.TextBodyProps.AutofitType != "normAutofit" {
		t.Fatalf("inherited normAutofit changed: %+v", inherited)
	}
}
