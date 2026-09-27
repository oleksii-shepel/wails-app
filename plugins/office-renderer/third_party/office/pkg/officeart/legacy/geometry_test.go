package legacy

import "testing"

func TestShapeTypeToPresetUsesMSODRAWMSOSPTValues(t *testing.T) {
	cases := map[uint16]string{
		1:   "rect",
		20:  "line",
		22:  "can",
		23:  "donut",
		32:  "straightConnector1",
		40:  "curvedConnector5",
		41:  "callout1",
		56:  "pentagon",
		57:  "noSmoking",
		58:  "star8",
		66:  "leftArrow",
		69:  "leftRightArrow",
		70:  "upDownArrow",
		75:  "rect", // msosptPictureFrame
		76:  "quadArrow",
		96:  "smileyFace",
		99:  "circularArrow",
		106: "cloudCallout",
		109: "flowChartProcess",
		135: "flowChartDelay",
		184: "moon",
	}
	for id, want := range cases {
		if got := ShapeTypeToPreset(id); got != want {
			t.Fatalf("ShapeTypeToPreset(%d) = %q, want %q", id, got, want)
		}
	}
	for _, id := range []uint16{0, 14, 24, 31, 100, 136, 175, 999} {
		if got := ShapeTypeToPreset(id); got != "" {
			t.Fatalf("ShapeTypeToPreset(%d) = %q, want unmapped geometry", id, got)
		}
	}
}
