package layout

import "testing"

func TestLegacyWordArtSlantDirectionsUsePowerPointScreenCoordinates(t *testing.T) {
	if got := resolveWordArtPathData("textSlantDown", false); got != "M 5 78 L 95 22" {
		t.Fatalf("textSlantDown path = %q, want down-left legacy baseline", got)
	}
	if got := resolveWordArtPathData("textSlantUp", false); got != "M 5 22 L 95 78" {
		t.Fatalf("textSlantUp path = %q, want up-left legacy baseline", got)
	}
}
