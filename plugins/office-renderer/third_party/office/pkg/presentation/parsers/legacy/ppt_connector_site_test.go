package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestLegacyPptConnectionSitePointUsesOfficeCounterclockwiseOrderFromTop(t *testing.T) {
	bounds := presentationir.Rect{X: 100, Y: 200, W: 80, H: 40}
	want := []presentationir.Point{
		{X: 140, Y: 200}, // top
		{X: 100, Y: 220}, // left
		{X: 140, Y: 240}, // bottom
		{X: 180, Y: 220}, // right
	}
	for site, expected := range want {
		got := legacyPptConnectionSitePoint(bounds, uint32(site))
		if got == nil || *got != expected {
			t.Fatalf("site %d = %#v, want %+v", site, got, expected)
		}
	}
}
