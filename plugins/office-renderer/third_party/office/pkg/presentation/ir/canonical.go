package ir

import "math"

// CanonicalRotationDeg normalizes clockwise rotation to the canonical
// presentation interval [-180, 180). Equivalent authored rotations such as
// 270 and -90 therefore compare identically across legacy PPT and DrawingML.
func CanonicalRotationDeg(deg float64) float64 {
	if deg == 0 {
		return 0
	}
	deg = math.Mod(deg, 360)
	if deg >= 180 {
		deg -= 360
	} else if deg < -180 {
		deg += 360
	}
	// PPT uses signed 16.16 degrees while DrawingML stores 1/60000 degree.
	// Round to a common 1e-3 degree precision so equivalent authored angles do
	// not diverge in canonical IR because of source quantization.
	deg = math.Round(deg*1000) / 1000
	// Avoid negative zero leaking into serialized/debug IR.
	if math.Abs(deg) < 1e-12 {
		return 0
	}
	return deg
}

// CanonicalArrowEnd collapses equivalent no-arrow representations and applies
// the DrawingML/OfficeArt medium-size defaults when an arrow actually exists.
func CanonicalArrowEnd(kind, width, length string) (string, string, string) {
	if kind == "" || kind == "none" {
		return "", "", ""
	}
	if width == "" {
		width = "med"
	}
	if length == "" {
		length = "med"
	}
	return kind, width, length
}
