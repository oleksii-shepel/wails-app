package legacy

import (
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

func TestLegacyOfficeArtPropertyReferencesUseSharedEscherConstants(t *testing.T) {
	tests := []struct {
		name string
		got  uint16
		want uint16
	}{
		{"line opacity", officeartescher.PropLineOpacity, 0x01C7},
		{"line dashing", officeartescher.PropLineDashing, 0x01CE},
		{"line start arrow", officeartescher.PropLineStartArrow, 0x01D0},
		{"line end arrow", officeartescher.PropLineEndArrow, 0x01D1},
		{"line start arrow width", officeartescher.PropLineStartArrowWidth, 0x01D2},
		{"line start arrow length", officeartescher.PropLineStartArrowLength, 0x01D3},
		{"line end arrow width", officeartescher.PropLineEndArrowWidth, 0x01D4},
		{"line end arrow length", officeartescher.PropLineEndArrowLength, 0x01D5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("property id = %#04x, want %#04x", tc.got, tc.want)
			}
		})
	}
}
