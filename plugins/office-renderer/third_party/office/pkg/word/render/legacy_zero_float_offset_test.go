package render

import (
	"bytes"
	"strings"
	"testing"

	dml "office/pkg/ooxml/drawing"
	"office/pkg/word/ir"
)

func TestFloatingShapeZeroOffsetStillGetsMarginPosition(t *testing.T) {
	r := &htmlRenderer{
		pageWidthPt:    612,
		pageHeightPt:   792,
		marginLeftPt:   72,
		marginRightPt:  72,
		marginTopPt:    72,
		marginBottomPt: 72,
	}
	shape := &ir.Shape{
		Floating:  true,
		X:         0,
		Y:         0,
		XRelative: "margin",
		YRelative: "margin",
		Width:     12700 * 100,
		Height:    12700 * 50,
		Geometry:  &dml.PresetGeometry{Name: "rect"},
	}
	var out bytes.Buffer
	r.renderShapeTo(&out, shape)
	html := out.String()
	if !strings.Contains(html, "left:72.0pt") {
		t.Fatalf("zero X offset lost relative origin: %s", html)
	}
	if !strings.Contains(html, "top:72.0pt") {
		t.Fatalf("zero Y offset lost relative origin: %s", html)
	}
}

func TestResolveFloatZeroOffsetPageVsMargin(t *testing.T) {
	r := &htmlRenderer{
		pageWidthPt:    612,
		pageHeightPt:   792,
		marginLeftPt:   72,
		marginRightPt:  72,
		marginTopPt:    72,
		marginBottomPt: 72,
	}
	if got := r.resolveFloatLeft(0, "", "page", 0, false); got != 0 {
		t.Fatalf("page X=%v want 0", got)
	}
	if got := r.resolveFloatLeft(0, "", "margin", 0, false); got != 72 {
		t.Fatalf("margin X=%v want 72", got)
	}
	if got := r.resolveFloatTop(0, "", "page", 0, false); got != 0 {
		t.Fatalf("page Y=%v want 0", got)
	}
	if got := r.resolveFloatTop(0, "", "margin", 0, false); got != 72 {
		t.Fatalf("margin Y=%v want 72", got)
	}
}
