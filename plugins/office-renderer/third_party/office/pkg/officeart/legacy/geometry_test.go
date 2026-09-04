package legacy

import "testing"

func TestShapeTypeToPresetSharedAcrossLegacyFormats(t *testing.T) {
	cases := map[uint16]string{
		1:   "rect",
		5:   "triangle",
		34:  "straightConnector1",
		70:  "leftRightUpArrow",
		113: "smileyFace",
		129: "bracePair",
	}
	for id, want := range cases {
		if got := ShapeTypeToPreset(id); got != want {
			t.Fatalf("ShapeTypeToPreset(%d) = %q, want %q", id, got, want)
		}
	}
}
