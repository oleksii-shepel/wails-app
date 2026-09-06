package drawing

import (
	"testing"
	docbinary "office/pkg/word/parsers/doc/binary"
)

func TestPlacementFromAnchorNormalizesReversedBounds(t *testing.T) {
	got, ok := PlacementFromAnchor(docbinary.FloatingAnchor{Left: 1440, Top: 2880, Right: 0, Bottom: 1440, BX: 2, BY: 2})
	if !ok { t.Fatal("PlacementFromAnchor returned false") }
	if got.X != 0 || got.Y != 914400 || got.Width != 914400 || got.Height != 914400 {
		t.Fatalf("placement = %+v", got)
	}
	if got.XRelative != "column" || got.YRelative != "paragraph" { t.Fatalf("relative = %s/%s", got.XRelative, got.YRelative) }
}

func TestPlacementFromAnchorRcaSimpleForcesPage(t *testing.T) {
	got, ok := PlacementFromAnchor(docbinary.FloatingAnchor{Right: 1440, Bottom: 1440, BX: 2, BY: 2, RcaSimple: true})
	if !ok { t.Fatal("PlacementFromAnchor returned false") }
	if got.XRelative != "page" || got.YRelative != "page" { t.Fatalf("relative = %s/%s", got.XRelative, got.YRelative) }
}

func TestPlacementFromAnchorPreservesBehindText(t *testing.T) {
	got, ok := PlacementFromAnchor(docbinary.FloatingAnchor{Right: 1440, Bottom: 1440, Wrap: 3, BelowText: true})
	if !ok { t.Fatal("PlacementFromAnchor returned false") }
	if got.WrapType != "none" || !got.BehindDoc { t.Fatalf("placement = %+v", got) }
}

func TestWrapSide(t *testing.T) {
	cases := []struct{ side uint8; want string }{{0,"both"},{1,"left"},{2,"right"},{3,"largest"}}
	for _, tc := range cases {
		if got := WrapSide(docbinary.FloatingAnchor{WrapSide: tc.side}); got != tc.want { t.Fatalf("side %d = %q, want %q", tc.side, got, tc.want) }
	}
}
