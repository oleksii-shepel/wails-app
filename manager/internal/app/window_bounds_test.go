package app

import "testing"

func TestFitBoundsToDisplayClampsOversizedWindow(t *testing.T) {
	x, y, width, height := fitBoundsToDisplay(
		displayRect{left: 0, top: 0, right: 1200, bottom: 700},
		10, 20, 1600, 900, true,
	)

	if width != 1200 || height != 700 {
		t.Fatalf("expected size to clamp to work area, got %dx%d", width, height)
	}
	if x != 0 || y != 0 {
		t.Fatalf("expected position to be clamped into work area, got (%d,%d)", x, y)
	}
}

func TestFitBoundsToDisplayPreservesUnpositionedWindow(t *testing.T) {
	x, y, width, height := fitBoundsToDisplay(
		displayRect{left: 0, top: 0, right: 1200, bottom: 700},
		0, 0, 1400, 900, false,
	)

	if width != 1200 || height != 700 {
		t.Fatalf("expected size to clamp to work area, got %dx%d", width, height)
	}
	if x != 0 || y != 0 {
		t.Fatalf("expected position to stay unchanged without persisted coordinates, got (%d,%d)", x, y)
	}
}
