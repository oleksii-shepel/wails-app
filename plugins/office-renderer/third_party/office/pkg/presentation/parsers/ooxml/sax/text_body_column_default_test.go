package sax

import (
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestAuthoredBodyPrDefaultsNumColToOneBeforePlaceholderInheritance(t *testing.T) {
	slide := &ir.Shape{}
	applyTextBodyProps(slide, TextBodyProperties{}) // authored <a:bodyPr/>
	if slide.TextBodyProps.Columns != 1 {
		t.Fatalf("authored bodyPr numCol default = %d, want 1", slide.TextBodyProps.Columns)
	}

	layout := &ir.Shape{}
	applyTextBodyProps(layout, TextBodyProperties{Columns: 2, HasColumns: true})
	inheritTextBodyFacts(slide, layout)
	if slide.TextBodyProps.Columns != 1 {
		t.Fatalf("layout numCol leaked through authored bodyPr default: got %d, want 1", slide.TextBodyProps.Columns)
	}
}
