package render

import (
	"strings"
	"testing"

	"office/pkg/word/ir"
)

func TestTextBoxViewportUsesShapeRectangleNotPage(t *testing.T) {
	w, h := textBoxViewportSize(144, 72, 612, 792)
	if w != 144 || h != 72 {
		t.Fatalf("viewport=(%v,%v), want shape rectangle (144,72)", w, h)
	}
}

func TestTextBoxViewportFallsBackOnlyWhenShapeDimensionMissing(t *testing.T) {
	w, h := textBoxViewportSize(0, 0, 612, 792)
	if w != 612 || h != 792 {
		t.Fatalf("viewport=(%v,%v), want page fallback (612,792)", w, h)
	}
}

func TestShapeTextBoxOuterStylesDoNotDuplicateInnerAlignment(t *testing.T) {
	tb := &ir.TextBox{
		Anchor:       "center",
		AnchorCenter: true,
		NumCol:       2,
		TextRotation: 15,
	}
	css := strings.Join(shapeTextBoxStyles(tb), ";")
	for _, forbidden := range []string{"justify-content:center", "text-align:center", "column-count:2", "rotate(15"} {
		if strings.Contains(css, forbidden) {
			t.Fatalf("outer shape textbox duplicated inner semantic %q in css=%q", forbidden, css)
		}
	}
	inner := strings.Join(textBoxInnerStyles(tb), ";")
	if !strings.Contains(inner, "align-items:center") || !strings.Contains(inner, "text-align:center") {
		t.Fatalf("inner textbox semantics missing: %q", inner)
	}
}
