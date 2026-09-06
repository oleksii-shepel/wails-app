package docbinary

import (
	"testing"

	"office/pkg/ooxml/units"
	wordir "office/pkg/word/ir"
)

func TestPlacementFromAnchorNormalizesRectangle(t *testing.T) {
	a := FloatingAnchor{Left: 300, Top: 400, Right: 100, Bottom: 200, BX: 1, BY: 1}
	p, ok := PlacementFromAnchor(a)
	if !ok {
		t.Fatal("placement rejected")
	}
	if p.X != units.TwipsToEMU(100) || p.Y != units.TwipsToEMU(200) || p.Width != units.TwipsToEMU(200) || p.Height != units.TwipsToEMU(200) {
		t.Fatalf("placement=%+v", p)
	}
	if p.XRelative != "page" || p.YRelative != "page" {
		t.Fatalf("relative=%s/%s", p.XRelative, p.YRelative)
	}
}

func TestPlacementFromAnchorHonorsRcaSimple(t *testing.T) {
	a := FloatingAnchor{Left: 1, Top: 2, Right: 11, Bottom: 22, BX: 2, BY: 2, RcaSimple: true, Flags: 0x2000}
	p, ok := PlacementFromAnchor(a)
	if !ok {
		t.Fatal("placement rejected")
	}
	if p.XRelative != "page" || p.YRelative != "page" {
		t.Fatalf("relative=%s/%s", p.XRelative, p.YRelative)
	}
}

func TestSpaWrapProjection(t *testing.T) {
	if got := WrapTypeFromSpaFlags(3 << 5); got != "none" {
		t.Fatalf("wrap=%q", got)
	}
	if got := WrapSideFromSpaFlags(2 << 9); got != "right" {
		t.Fatalf("side=%q", got)
	}
}

func TestApplyAnchorToShapeCarriesFlowMetadata(t *testing.T) {
	a := FloatingAnchor{CP: 166, Shape: 42, Left: 10, Top: 20, Right: 110, Bottom: 220, BX: 2, BY: 2, Wrap: 3, Flags: uint16(2<<1) | uint16(2<<3) | uint16(3<<5)}
	shape := &wordir.Shape{Kind: "rect"}
	if !ApplyAnchorToShape(shape, a) {
		t.Fatal("anchor not applied")
	}
	if shape.LegacyShapeID != 42 || !shape.LegacyFlowAnchor || shape.LegacyFlowOrder != 166 {
		t.Fatalf("shape=%+v", shape)
	}
}

func TestApplyAnchorToShapeDoesNotMarkOrdinaryFloatAsFlow(t *testing.T) {
	a := FloatingAnchor{CP: 42, Shape: 900, Left: 100, Top: 200, Right: 600, Bottom: 800, BX: 1, BY: 1, Wrap: 0}
	shape := &wordir.Shape{Kind: "rect"}
	if !ApplyAnchorToShape(shape, a) {
		t.Fatal("anchor not applied")
	}
	if shape.LegacyFlowAnchor || shape.LegacyFlowOrder != 0 {
		t.Fatalf("ordinary float incorrectly marked as SHAPE-field flow: %+v", shape)
	}
}

func TestApplyAnchorToTextBox(t *testing.T) {
	a := FloatingAnchor{Shape: 7, Left: 10, Top: 20, Right: 110, Bottom: 220, BX: 1, BY: 1}
	tb := &wordir.TextBox{}
	if !ApplyAnchorToTextBox(tb, a) {
		t.Fatal("anchor not applied")
	}
	if tb.LegacyShapeID != 7 || !tb.Floating || tb.Width <= 0 || tb.Height <= 0 {
		t.Fatalf("textbox=%+v", tb)
	}
}
