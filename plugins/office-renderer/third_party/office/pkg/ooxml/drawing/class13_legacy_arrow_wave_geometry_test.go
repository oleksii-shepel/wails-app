package drawing

import (
	"reflect"
	"testing"
)

func TestClass13LegacyArrowAndWavePresetsDoNotFallBackToRectangle(t *testing.T) {
	rect, _ := (&PresetGeometry{Name: "rect"}).GetPath(21600, 10800)
	for _, name := range []string{"leftUpArrow", "upDownArrowCallout", "circularArrow", "wave"} {
		p, err := (&PresetGeometry{Name: name}).GetPath(21600, 10800)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if reflect.DeepEqual(p, rect) {
			t.Fatalf("%s still falls back to rectangle", name)
		}
		if len(p.Commands) < 5 {
			t.Fatalf("%s has insufficient topology: %d commands", name, len(p.Commands))
		}
	}
}

func TestClass13LegacyArrowAndWavePresetsSurvivePortraitBounds(t *testing.T) {
	for _, name := range []string{"leftUpArrow", "upDownArrowCallout", "circularArrow", "wave"} {
		p, err := (&PresetGeometry{Name: name, Adjustments: []float64{.18, .28}}).GetPath(10800, 21600)
		if err != nil || len(p.Commands) == 0 {
			t.Fatalf("%s portrait geometry invalid: %v", name, err)
		}
	}
}

func TestClass13WaveAndCircularArrowAdjustmentsOwnGeometry(t *testing.T) {
	for _, name := range []string{"wave", "circularArrow", "leftUpArrow"} {
		a, _ := (&PresetGeometry{Name: name, Adjustments: []float64{.12, .22}}).GetPath(21600, 10800)
		b, _ := (&PresetGeometry{Name: name, Adjustments: []float64{.34, .40}}).GetPath(21600, 10800)
		if pathsEqual(a, b) {
			t.Fatalf("%s ignores adjustments", name)
		}
	}
}
