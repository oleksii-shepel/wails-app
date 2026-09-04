package render

import (
	"fmt"
	"math"
)

// styleDebugHSL returns a pastel HSL colour for the given paragraph index.
// Adjacent indices are maximally distinct thanks to the golden angle (137.508°).
func styleDebugHSL(idx int) string {
	hue := uint32(math.Round(math.Mod(float64(idx)*137.508, 360)))
	return fmt.Sprintf("hsl(%d,55%%,85%%)", hue)
}

// styleDebugBgColor wraps styleDebugHSL as a CSS background declaration
// suitable for appending into an HTML inline style= attribute.
func styleDebugBgColor(idx int) string {
	return "background:" + styleDebugHSL(idx)
}