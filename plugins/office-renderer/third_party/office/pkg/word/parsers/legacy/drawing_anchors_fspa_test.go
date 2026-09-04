package legacy

import "testing"

func TestLegacySpaRcaSimpleForcesPageRelative(t *testing.T) {
	flags := uint16((2 << 1) | (2 << 3) | 0x2000) // bx=column, by=paragraph, fRcaSimple
	if got := legacyAnchorRelativeFromSpaFlags(flags, false); got != "page" {
		t.Fatalf("horizontal relative = %q, want page", got)
	}
	if got := legacyAnchorRelativeFromSpaFlags(flags, true); got != "page" {
		t.Fatalf("vertical relative = %q, want page", got)
	}
}

func TestLegacyPlacementFromAnchorNormalizesRectangle(t *testing.T) {
	anchor := legacyFloatingDrawingAnchor{
		Left:   300,
		Top:    500,
		Right:  100,
		Bottom: 200,
		BX:     2,
		BY:     2,
	}
	got, ok := legacyPlacementFromAnchor(anchor)
	if !ok {
		t.Fatal("placement failed")
	}
	if got.XRelative != "column" || got.YRelative != "paragraph" {
		t.Fatalf("relative = %q/%q", got.XRelative, got.YRelative)
	}
	if got.X != 100*635 || got.Y != 200*635 || got.Width != 200*635 || got.Height != 300*635 {
		t.Fatalf("placement = %+v", got)
	}
}

func TestLegacyShapeUsesAnchorLineFlowOnlyForColumnParagraphNoWrapSpa(t *testing.T) {
	flow := legacyFloatingDrawingAnchor{BX: 2, BY: 2, Wrap: 3}
	if !legacyShapeUsesAnchorLineFlow(flow) {
		t.Fatal("expected column/paragraph wr=3 Spa to use anchor-line flow")
	}

	for _, tc := range []legacyFloatingDrawingAnchor{
		{BX: 1, BY: 2, Wrap: 3},
		{BX: 2, BY: 1, Wrap: 3},
		{BX: 2, BY: 2, Wrap: 0},
		{BX: 2, BY: 2, Wrap: 3, RcaSimple: true},
	} {
		if legacyShapeUsesAnchorLineFlow(tc) {
			t.Fatalf("unexpected anchor-line flow for %+v", tc)
		}
	}
}
