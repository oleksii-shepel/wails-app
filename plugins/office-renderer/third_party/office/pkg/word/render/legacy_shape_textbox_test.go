package render

import (
	"strings"
	"testing"

	"office/pkg/word/parsers/ooxml/resolve"
)

func TestShapeTextBoxStylesUsesBorderBoxForInsets(t *testing.T) {
	tb := &resolve.TextBox{
		PaddingLeft:   12,
		PaddingTop:    6,
		PaddingRight:  4,
		PaddingBottom: 2,
	}
	css := strings.Join(shapeTextBoxStyles(tb), ";")
	if !strings.Contains(css, "box-sizing:border-box") {
		t.Fatalf("css=%q", css)
	}
	if !strings.Contains(css, "padding:6.00pt 4.00pt 2.00pt 12.00pt") {
		t.Fatalf("css=%q", css)
	}
}

func TestShapeTextBoxStylesCarriesNoWrapAndVerticalFlow(t *testing.T) {
	tb := &resolve.TextBox{
		NoWrap: true,
		Vert:   "vert",
	}
	css := strings.Join(shapeTextBoxStyles(tb), ";")
	if !strings.Contains(css, "white-space:nowrap") {
		t.Fatalf("css=%q", css)
	}
	if !strings.Contains(css, "writing-mode:vertical-rl") {
		t.Fatalf("css=%q", css)
	}
}

func TestTextBoxInnerStylesCentersVertically(t *testing.T) {
	tb := &resolve.TextBox{
		Anchor:       "center",
		AnchorCenter: true,
	}
	outer := strings.Join(shapeTextBoxStyles(tb), ";")
	if strings.Contains(outer, "align-items:center") || strings.Contains(outer, "text-align:center") {
		t.Fatalf("outer shape box duplicated inner alignment: %q", outer)
	}
	inner := strings.Join(textBoxInnerStyles(tb), ";")
	if !strings.Contains(inner, "align-items:center") || !strings.Contains(inner, "text-align:center") {
		t.Fatalf("inner css=%q", inner)
	}
}

func TestShapeTextBoxStylesDoesNotApplyShapeRotationAgain(t *testing.T) {
	tb := &resolve.TextBox{
		RotationDeg: 45,
	}
	css := strings.Join(shapeTextBoxStyles(tb), ";")
	if strings.Contains(css, "45.00deg") {
		t.Fatalf("shape rotation leaked into inner textbox css=%q", css)
	}
}
