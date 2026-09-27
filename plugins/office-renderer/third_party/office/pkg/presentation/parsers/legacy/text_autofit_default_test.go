package legacy

import (
	"office/pkg/presentation/ir"
	"testing"
)

func TestCanonicalLegacyTextDefaultsToFixedNoAutofitOverflow(t *testing.T) {
	text := &ir.Shape{Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "overflow"}}}}}
	explicitGrow := &ir.Shape{Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "grow"}}}}, TextAutoGrow: true}
	explicitNorm := &ir.Shape{Paragraphs: []*ir.Paragraph{{Runs: []*ir.Run{{Text: "fit"}}}}}
	explicitNorm.TextBodyProps.AutofitType = "normAutofit"
	pres := &ir.Presentation{Slides: []*ir.Slide{{Shapes: []*ir.Shape{text, explicitGrow, explicitNorm}}}}
	normalizeLegacyPptEffectiveTextAutofitDefaults(pres)
	if text.TextBodyProps.AutofitType != "noAutofit" || !text.TextAllowOverflow || text.TextAutoGrow {
		t.Fatalf("default text fit = %+v allow=%v grow=%v", text.TextBodyProps, text.TextAllowOverflow, text.TextAutoGrow)
	}
	if explicitGrow.TextAllowOverflow || !explicitGrow.TextAutoGrow || explicitGrow.TextBodyProps.AutofitType != "" {
		t.Fatalf("explicit grow changed: %+v", explicitGrow)
	}
	if explicitNorm.TextAllowOverflow || explicitNorm.TextBodyProps.AutofitType != "normAutofit" {
		t.Fatalf("explicit normAutofit changed: %+v", explicitNorm)
	}
}
