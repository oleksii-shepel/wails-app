package legacy

import (
	"testing"

	"office/pkg/officeart/ir"
	"office/pkg/officeart/legacy/escher"
)

func TestShapeFromEscherBuildsTextBox(t *testing.T) {
	node := &escher.ShapeNode{
		ShapeID:          77,
		ShapeType:        1,
		HasClientTextbox: true,
		Properties: []escher.Property{
			{ID: escher.PropRotation, Value: 30 * 65536},
			{ID: escher.PropTextLeft, Value: 25400},
			{ID: escher.PropWrapText, Value: 0},
		},
	}
	got := ShapeFromEscher(node)
	if got.Kind != ir.ShapeTextBox || got.Text == nil {
		t.Fatalf("shape = %#v", got)
	}
	if got.Transform.RotationDeg != 30 {
		t.Fatalf("rotation = %g", got.Transform.RotationDeg)
	}
	if got.Text.Insets.Left != 25400 || got.Text.Wrap != ir.TextWrapSquare {
		t.Fatalf("text = %#v", got.Text)
	}
}

func TestShapeFromEscherPromotesGeometryTextIntoSharedIR(t *testing.T) {
	node := &escher.ShapeNode{
		ShapeID:   90,
		ShapeType: 1,
		Properties: []escher.Property{
			{ID: escher.PropGTextUnicode, IsComplex: true, Complex: geometryTextUTF16("Shared WordArt")},
			{ID: escher.PropGTextFont, IsComplex: true, Complex: geometryTextUTF16("Arial Black")},
			{ID: escher.PropGTextSize, Value: 30 << 16},
			{ID: escher.PropGTextAlign, Value: 1},
		},
	}
	got := ShapeFromEscher(node)
	if got == nil || got.Text == nil || len(got.Text.Paragraphs) != 1 {
		t.Fatalf("shape text = %#v", got)
	}
	p := got.Text.Paragraphs[0]
	if p.Align != "center" || len(p.Runs) != 1 {
		t.Fatalf("paragraph = %+v", p)
	}
	run := p.Runs[0]
	if run.Text != "Shared WordArt" || run.FontFamily != "Arial Black" || run.FontSizePt != 30 {
		t.Fatalf("run = %+v", run)
	}
}

func TestShapeFromEscherKeepsGeometryTextAsAutoShape(t *testing.T) {
	node := &escher.ShapeNode{
		ShapeID:   91,
		ShapeType: 1,
		Properties: []escher.Property{
			{ID: escher.PropGTextUnicode, IsComplex: true, Complex: geometryTextUTF16("WordArt")},
		},
	}
	got := ShapeFromEscher(node)
	if got.Kind == ir.ShapeTextBox {
		t.Fatalf("GeoText auto-shape incorrectly became textbox: %+v", got)
	}
	if got.Text == nil || got.Text.Paragraphs[0].Runs[0].Text != "WordArt" {
		t.Fatalf("GeoText missing: %+v", got)
	}
}
