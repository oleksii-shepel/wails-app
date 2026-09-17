package drawing

import "testing"

// These presets are emitted by the shared legacy OfficeArt adapter. They used
// to be accepted by PresetGeometry only through the rectangle default, which
// made Class 13 appear supported while silently losing shape semantics.
func TestClass13LegacyFlowchartPresetsDoNotFallBackToRectangle(t *testing.T) {
	names := []string{
		"flowChartMultidocument", "flowChartSummingJunction", "flowChartOr",
		"flowChartCollate", "flowChartOfflineStorage", "flowChartOnlineStorage",
		"flowChartMagneticDrum",
	}
	rect, _ := (&PresetGeometry{Name: "rect"}).GetPath(21600, 10800)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got, err := (&PresetGeometry{Name: name}).GetPath(21600, 10800)
			if err != nil {
				t.Fatal(err)
			}
			if pathsEqual(got, rect) {
				t.Fatalf("%s still uses rectangle fallback", name)
			}
			portrait, err := (&PresetGeometry{Name: name}).GetPath(10800, 21600)
			if err != nil {
				t.Fatal(err)
			}
			if len(portrait.Commands) == 0 {
				t.Fatalf("%s produced empty portrait geometry", name)
			}
		})
	}
}
