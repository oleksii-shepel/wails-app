package drawing

import "testing"

func TestClass13FlowchartSpecializationsDoNotReuseApproximatePresets(t *testing.T) {
	punched, _ := (&PresetGeometry{Name: "flowChartPunchedTape"}).GetPath(21600, 10800)
	doc, _ := (&PresetGeometry{Name: "flowChartDocument"}).GetPath(21600, 10800)
	if pathsEqual(punched, doc) {
		t.Fatal("punched tape must not reuse document geometry")
	}
	merge, _ := (&PresetGeometry{Name: "flowChartMerge"}).GetPath(21600, 10800)
	tri, _ := (&PresetGeometry{Name: "triangle"}).GetPath(21600, 10800)
	if pathsEqual(merge, tri) {
		t.Fatal("merge must not reuse upward triangle geometry")
	}
}

func TestClass13MathDivideHasBarAndTwoDots(t *testing.T) {
	p, _ := (&PresetGeometry{Name: "mathDivide"}).GetPath(21600, 10800)
	moves, closes, arcs := 0, 0, 0
	for _, c := range p.Commands {
		switch c.Type {
		case "M":
			moves++
		case "Z":
			closes++
		case "A":
			arcs++
		}
	}
	if moves != 3 || closes != 3 {
		t.Fatalf("mathDivide subpaths moves=%d closes=%d, want 3/3", moves, closes)
	}
	if arcs != 8 {
		t.Fatalf("mathDivide dot arcs=%d, want 8", arcs)
	}
}

func TestClass13SpecialSymbolsRemainValidOnPortraitBounds(t *testing.T) {
	for _, name := range []string{"flowChartPunchedTape", "flowChartMerge", "mathDivide"} {
		p, err := (&PresetGeometry{Name: name}).GetPath(10800, 21600)
		if err != nil || len(p.Commands) == 0 {
			t.Fatalf("%s invalid path: %v", name, err)
		}
	}
}

func pathsEqual(a, b Path) bool {
	if len(a.Commands) != len(b.Commands) {
		return false
	}
	for i := range a.Commands {
		if a.Commands[i].Type != b.Commands[i].Type || len(a.Commands[i].Points) != len(b.Commands[i].Points) {
			return false
		}
		for j := range a.Commands[i].Points {
			if a.Commands[i].Points[j] != b.Commands[i].Points[j] {
				return false
			}
		}
	}
	return true
}
