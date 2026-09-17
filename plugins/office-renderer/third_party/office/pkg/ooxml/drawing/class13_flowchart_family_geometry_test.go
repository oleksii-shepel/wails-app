package drawing

import "testing"

func TestClass13FlowchartPresetsOwnDistinctGeometry(t *testing.T) {
	names := []string{"flowChartInputOutput", "flowChartPredefinedProcess", "flowChartInternalStorage", "flowChartManualOperation", "flowChartPunchedCard", "flowChartSort", "flowChartExtract", "flowChartStoredData", "flowChartDelay", "flowChartMagneticTape", "flowChartMagneticDisk", "flowChartDisplay"}
	rect, _ := (&PresetGeometry{Name: "rect"}).GetPath(21600, 10800)
	rsig := pathCommandSignature(rect)
	seen := map[string]string{}
	for _, name := range names {
		p, err := (&PresetGeometry{Name: name}).GetPath(21600, 10800)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		sig := pathCommandSignature(p)
		if sig == rsig {
			t.Errorf("%s still falls back to rectangle", name)
		}
		if prev, ok := seen[sig]; ok {
			t.Errorf("%s reuses %s geometry", name, prev)
		}
		seen[sig] = name
		portrait, err := (&PresetGeometry{Name: name}).GetPath(10800, 21600)
		if err != nil || len(portrait.Commands) == 0 {
			t.Errorf("%s portrait geometry invalid: %v", name, err)
		}
	}
}
