package drawingcss

import (
	"fmt"

	"office/pkg/ooxml/drawing"
)

// LineStyle returns CSS properties for a DrawingML line style.
func LineStyle(ls drawing.LineStyle) map[string]string {
	props := make(map[string]string)

	// Stroke color
	if ls.Alpha < 1.0 {
		props["stroke"] = fmt.Sprintf("rgba(%s, %s, %s, %.2f)",
			hexToRGB(ls.Color, 0),
			hexToRGB(ls.Color, 1),
			hexToRGB(ls.Color, 2),
			ls.Alpha)
	} else {
		props["stroke"] = "#" + ls.Color
	}

	// Stroke width
	props["stroke-width"] = fmt.Sprintf("%.1fpt", ls.Width)

	// Line cap
	switch ls.Cap {
	case drawing.LineCapRound:
		props["stroke-linecap"] = "round"
	case drawing.LineCapSquare:
		props["stroke-linecap"] = "square"
	default:
		props["stroke-linecap"] = "butt" // CSS equivalent of flat
	}

	// Line join
	switch ls.Join {
	case drawing.LineJoinRound:
		props["stroke-linejoin"] = "round"
	case drawing.LineJoinBevel:
		props["stroke-linejoin"] = "bevel"
	default:
		props["stroke-linejoin"] = "miter"
	}

	// Miter limit
	if ls.Join == drawing.LineJoinMiter {
		props["stroke-miterlimit"] = fmt.Sprintf("%.1f", ls.MiterLimit)
	}

	// Dash pattern
	dashArray := ls.DashArray()
	if dashArray != "" {
		props["stroke-dasharray"] = dashArray
	}

	return props
}
