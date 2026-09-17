package legacy

import (
	"testing"

	presentationir "office/pkg/presentation/ir"
)

func TestLegacyPptConnectorRuleSitesTranslateLegacyClockwiseOrdering(t *testing.T) {
	shape := &presentationir.Shape{Geometry: "rect", Bounds: presentationir.Rect{X: 100, Y: 200, W: 400, H: 200}}
	want := []presentationir.Point{
		{X: 500, Y: 300}, // legacy cpti 0: right
		{X: 300, Y: 400}, // legacy cpti 1: bottom
		{X: 100, Y: 300}, // legacy cpti 2: left
		{X: 300, Y: 200}, // legacy cpti 3: top
	}
	for site, expected := range want {
		got := legacyPptConnectorRuleSitePointForShape(shape, uint32(site))
		if got == nil || *got != expected {
			t.Fatalf("legacy rule site %d = %#v, want %#v", site, got, expected)
		}
	}
}

func TestLegacyPptConnectorRuleSitesPreserveShapeTransform(t *testing.T) {
	shape := &presentationir.Shape{
		Geometry:    "rect",
		Bounds:      presentationir.Rect{X: 100, Y: 200, W: 400, H: 200},
		RotationDeg: 90,
	}
	// Legacy cpti 0 is the authored right site; a clockwise 90-degree rotation
	// moves it to the bottom of the transformed shape.
	got := legacyPptConnectorRuleSitePointForShape(shape, 0)
	want := presentationir.Point{X: 300, Y: 500}
	if got == nil || *got != want {
		t.Fatalf("rotated legacy rule site = %#v, want %#v", got, want)
	}
}

func TestLegacyPptConnectorRuleSitesUsePresetSiteCount(t *testing.T) {
	shape := &presentationir.Shape{Geometry: "triangle", Bounds: presentationir.Rect{X: 100, Y: 200, W: 300, H: 400}}
	first := legacyPptConnectorRuleSitePointForShape(shape, 0)
	wrapped := legacyPptConnectorRuleSitePointForShape(shape, 3)
	if first == nil || wrapped == nil || *first != *wrapped {
		t.Fatalf("triangle rule site count did not wrap at 3: first=%#v wrapped=%#v", first, wrapped)
	}
}
