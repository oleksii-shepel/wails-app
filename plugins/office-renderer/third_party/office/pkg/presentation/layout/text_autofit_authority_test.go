package layout

import (
	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"testing"
)

func TestNormAutofitWithoutPersistedScaleDoesNotInventFontScale(t *testing.T) {
	shape := &ir.Shape{
		Bounds:        ir.Rect{W: 12700 * 100, H: 12700 * 20},
		TextBodyProps: drawing.TextBodyProperties{AutofitType: "normAutofit"},
		Paragraphs:    []*ir.Paragraph{{Runs: []*ir.Run{{Text: "one two three four five six seven eight nine ten", FontSize: 3200}}}},
	}
	plan := ResolveTextVisualPlan(shape)
	if plan.Frame.FontScale != 1 {
		t.Fatalf("font scale = %.4f, want DrawingML default 1.0 when fontScale is absent", plan.Frame.FontScale)
	}
}
