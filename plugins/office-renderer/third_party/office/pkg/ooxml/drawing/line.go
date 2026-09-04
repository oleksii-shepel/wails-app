package drawing

import (
	"fmt"
	"math"
)

// LineCap represents the line cap style.
type LineCap string

const (
	LineCapFlat   LineCap = "flat"
	LineCapRound  LineCap = "round"
	LineCapSquare LineCap = "square"
)

// LineJoin represents the line join style.
type LineJoin string

const (
	LineJoinMiter LineJoin = "miter"
	LineJoinRound LineJoin = "round"
	LineJoinBevel LineJoin = "bevel"
)

// DashPattern represents a line dash pattern.
type DashPattern string

const (
	DashPatternSolid            DashPattern = "solid"
	DashPatternDash             DashPattern = "dash"
	DashPatternDot              DashPattern = "dot"
	DashPatternDashDot          DashPattern = "dashDot"
	DashPatternLongDash         DashPattern = "longDash"
	DashPatternLongDashDot      DashPattern = "longDashDot"
	DashPatternLongDashDotDot   DashPattern = "longDashDotDot"
	DashPatternSystemDash       DashPattern = "sysDash"
	DashPatternSystemDot        DashPattern = "sysDot"
	DashPatternSystemDashDot    DashPattern = "sysDashDot"
	DashPatternSystemDashDotDot DashPattern = "sysDashDotDot"
)

// LineStyle represents DrawingML line properties.
type LineStyle struct {
	Width      float64     // Line width in points
	Color      string      // Hex RGB color
	Fill       Fill        // Optional gradient/pattern fill for the line stroke
	Alpha      float64     // Transparency (0.0 = transparent, 1.0 = opaque)
	ThemeColor string      // Theme color reference (e.g. "accent1")
	ThemeShade string      // Theme shade modifier (hex string)
	ThemeTint  string      // Theme tint modifier (hex string)
	LumMod     float64     // Luminance modulation (0.0-1.0)
	LumOff     float64     // Luminance offset (-1.0 to 1.0)
	Cap        LineCap     // Line cap style
	Join       LineJoin    // Line join style
	MiterLimit float64     // Miter limit for miter joins
	Dash       DashPattern // Dash pattern
	Compound   bool        // Whether this is a compound line
	// CompoundType stores the DrawingML cmpd attribute value:
	// "sng" (single, default), "dbl" (double), "thickThin", "thinThick", "tri" (triple).
	CompoundType string
	// For compound lines: widths of sub-lines (0.0 to 1.0)
	CompoundWidths []float64
	// HeadEnd is the optional arrowhead at the start of the line.
	HeadEnd *Arrowhead
	// TailEnd is the optional arrowhead at the end of the line.
	TailEnd *Arrowhead
}

// DefaultLineStyle returns a default solid black line.
func DefaultLineStyle() LineStyle {
	return LineStyle{
		Width:      1.0,
		Color:      "000000",
		Alpha:      1.0,
		Cap:        LineCapFlat,
		Join:       LineJoinMiter,
		MiterLimit: 10.0,
		Dash:       DashPatternSolid,
	}
}

// dashArray returns the CSS stroke-dasharray value for the dash pattern.
func (ls LineStyle) DashArray() string {
	switch ls.Dash {
	case DashPatternSolid:
		return "none"
	case DashPatternDash:
		return "3, 3"
	case DashPatternDot:
		return "1, 3"
	case DashPatternDashDot:
		return "3, 3, 1, 3"
	case DashPatternLongDash:
		return "5, 3"
	case DashPatternLongDashDot:
		return "5, 3, 1, 3"
	case DashPatternLongDashDotDot:
		return "5, 3, 1, 3, 1, 3"
	case DashPatternSystemDash:
		return "3, 3"
	case DashPatternSystemDot:
		return "1, 3"
	case DashPatternSystemDashDot:
		return "3, 3, 1, 3"
	case DashPatternSystemDashDotDot:
		return "3, 3, 1, 3, 1, 3"
	default:
		return "none"
	}
}

// ArrowheadType represents arrowhead styles.
type ArrowheadType string

const (
	ArrowheadNone     ArrowheadType = "none"
	ArrowheadTriangle ArrowheadType = "triangle"
	ArrowheadArrow    ArrowheadType = "arrow"
	ArrowheadStealth  ArrowheadType = "stealth"
	ArrowheadDiamond  ArrowheadType = "diamond"
	ArrowheadOval     ArrowheadType = "oval"
)

// ArrowheadSize represents arrowhead size.
type ArrowheadSize string

const (
	ArrowheadSizeSmall  ArrowheadSize = "sm"
	ArrowheadSizeMedium ArrowheadSize = "med"
	ArrowheadSizeLarge  ArrowheadSize = "lg"
)

// Arrowhead represents an arrowhead decoration.
type Arrowhead struct {
	Type   ArrowheadType
	Size   ArrowheadSize
	Length ArrowheadSize
}

// ArrowheadWidth returns the arrowhead width in points.
func (a Arrowhead) Width() float64 {
	switch a.Size {
	case ArrowheadSizeSmall:
		return 4.0
	case ArrowheadSizeMedium:
		return 6.0
	case ArrowheadSizeLarge:
		return 8.0
	default:
		return 6.0
	}
}

// ArrowheadLength returns the arrowhead length in points.
func (a Arrowhead) LengthPoints() float64 {
	switch a.Length {
	case ArrowheadSizeSmall:
		return 6.0
	case ArrowheadSizeMedium:
		return 9.0
	case ArrowheadSizeLarge:
		return 12.0
	default:
		return 9.0
	}
}

// PathForArrowhead returns the SVG path for the arrowhead.
func (a Arrowhead) PathForArrowhead(lineWidth float64) string {
	if a.Type == ArrowheadNone {
		return ""
	}

	width := a.Width()
	length := a.LengthPoints()

	switch a.Type {
	case ArrowheadTriangle:
		return fmt.Sprintf("M 0,0 L %.1f,%.1f L %.1f,-%.1f Z",
			length, width/2, length, width/2)
	case ArrowheadArrow:
		return fmt.Sprintf("M 0,0 L %.1f,%.1f L %.1f,0 L %.1f,-%.1f Z",
			length*0.7, width/2, length, length*0.7, width/2)
	case ArrowheadStealth:
		return fmt.Sprintf("M 0,0 L %.1f,%.1f L %.1f,0 L %.1f,-%.1f Z",
			length*0.8, width/2, length, length*0.8, width/2)
	case ArrowheadDiamond:
		return fmt.Sprintf("M 0,0 L %.1f,%.1f L %.1f,0 L %.1f,-%.1f Z",
			length/2, width/2, length, length/2, width/2)
	case ArrowheadOval:
		rx := length / 2
		ry := width / 2
		return fmt.Sprintf("M 0,0 A %.1f,%.1f 0 0,1 0,%.1f A %.1f,%.1f 0 0,1 0,0 Z",
			rx, ry, width, rx, ry)
	default:
		return ""
	}
}

// ConnectorType represents connector line types.
type ConnectorType string

const (
	ConnectorTypeStraight ConnectorType = "straight"
	ConnectorTypeElbow    ConnectorType = "elbow"
	ConnectorTypeCurved   ConnectorType = "curved"
)

// ConnectionSite represents a connection point on a shape.
type ConnectionSite struct {
	// Position on shape perimeter (0.0 to 1.0)
	Position float64
	// Angle in degrees (0 = right, 90 = up, 180 = left, 270 = down)
	Angle float64
}

// CalculateConnectionSites calculates connection sites for a rectangle.
func CalculateConnectionSites(width, height float64) []ConnectionSite {
	sites := make([]ConnectionSite, 0, 8)

	// Add corner points
	sites = append(sites,
		ConnectionSite{Position: 0.0, Angle: 0},    // Right middle
		ConnectionSite{Position: 0.25, Angle: 90},  // Top middle
		ConnectionSite{Position: 0.5, Angle: 180},  // Left middle
		ConnectionSite{Position: 0.75, Angle: 270}, // Bottom middle
	)

	// Add edge midpoints
	sites = append(sites,
		ConnectionSite{Position: 0.125, Angle: 45},  // Top-right corner
		ConnectionSite{Position: 0.375, Angle: 135}, // Top-left corner
		ConnectionSite{Position: 0.625, Angle: 225}, // Bottom-left corner
		ConnectionSite{Position: 0.875, Angle: 315}, // Bottom-right corner
	)

	return sites
}

// PointForConnectionSite calculates the actual point for a connection site.
func PointForConnectionSite(site ConnectionSite, rect Rect) Point {
	// Calculate position along perimeter
	perimeter := 2 * (rect.Width + rect.Height)
	pos := site.Position * perimeter

	// Determine which edge the point is on
	if pos < rect.Width {
		// Right edge
		return Point{X: rect.X + pos, Y: rect.Y}
	} else if pos < rect.Width+rect.Height {
		// Bottom edge
		return Point{X: rect.X + rect.Width, Y: rect.Y + (pos - rect.Width)}
	} else if pos < 2*rect.Width+rect.Height {
		// Left edge
		return Point{X: rect.X + rect.Width - (pos - rect.Width - rect.Height), Y: rect.Y + rect.Height}
	} else {
		// Top edge
		return Point{X: rect.X, Y: rect.Y + rect.Height - (pos - 2*rect.Width - rect.Height)}
	}
}

// CalculateElbowPath calculates the path for an elbow connector.
func CalculateElbowPath(start, end Point, startAngle, endAngle float64) Path {
	// Simple elbow connector: horizontal then vertical
	midX := (start.X + end.X) / 2

	return Path{
		Commands: []PathCommand{
			{Type: "M", Points: []Point{start}},
			{Type: "L", Points: []Point{{X: midX, Y: start.Y}}},
			{Type: "L", Points: []Point{{X: midX, Y: end.Y}}},
			{Type: "L", Points: []Point{end}},
		},
		FillRule: "nonzero",
	}
}

// CalculateCurvedPath calculates the path for a curved connector.
func CalculateCurvedPath(start, end Point, startAngle, endAngle float64) Path {
	// Simple curved connector: quadratic Bézier curve
	ctrlX := (start.X + end.X) / 2
	ctrlY := (start.Y + end.Y) / 2

	// Adjust control point based on angles
	angleDiff := math.Mod(endAngle-startAngle+360, 360)
	if angleDiff > 180 {
		// Curve in the opposite direction
		ctrlX += (end.Y - start.Y) * 0.5
		ctrlY -= (end.X - start.X) * 0.5
	} else {
		ctrlX -= (end.Y - start.Y) * 0.5
		ctrlY += (end.X - start.X) * 0.5
	}

	return Path{
		Commands: []PathCommand{
			{Type: "M", Points: []Point{start}},
			{Type: "Q", Points: []Point{{X: ctrlX, Y: ctrlY}, end}},
		},
		FillRule: "nonzero",
	}
}
