package drawing

import "strings"

// VectorPrimitiveKind identifies the rendered geometry primitive type.
type VectorPrimitiveKind int

const (
	PrimitivePath VectorPrimitiveKind = iota
	PrimitivePolyline
	PrimitiveEllipse
)

// VectorStyle carries the fill and stroke styling for a vector primitive.
type VectorStyle struct {
	Fill            Fill
	Stroke          *LineStyle
	FillColor       string
	StrokeColor     string
	StrokeWidth     float64
	StrokeFillDefID string // Optional SVG def ID for a gradient/pattern stroke
}

// Ellipse describes a rendered ellipse primitive.
type Ellipse struct {
	Center Point
	RX     float64
	RY     float64
}

// VectorPrimitive is the renderer-facing vector contract shared across outputs.
type VectorPrimitive struct {
	Kind    VectorPrimitiveKind
	Path    *Path
	Points  []Point
	Ellipse *Ellipse
	Style   VectorStyle
	Closed  bool
}

// ShapeGeometrySpec describes the render-facing geometry and styles for a shape.
type ShapeGeometrySpec struct {
	Geometry    *Geometry
	Preset      *PresetGeometry
	Width       float64
	Height      float64
	Fill        Fill
	FillColor   string
	Stroke      *LineStyle
	StrokeColor string
	StrokeWidth float64
}

// ConnectorSpec describes a connector body plus optional endpoint decorations.
type ConnectorSpec struct {
	Type            string
	BoundsX         float64
	BoundsY         float64
	BoundsW         float64
	BoundsH         float64
	Adjustments     []int
	FlipH           bool
	FlipV           bool
	LineStyle       *LineStyle
	LineColor       string
	LineWidthPt     float64
	ArrowHead       string
	ArrowHeadWidth  string
	ArrowHeadLength string
	ArrowTail       string
	ArrowTailWidth  string
	ArrowTailLength string
}

// BuildConnectorPrimitives returns the shared vector primitives for a connector.
func BuildConnectorPrimitives(spec ConnectorSpec) []VectorPrimitive {
	color := spec.LineColor
	if spec.LineStyle != nil {
		if color == "" && spec.LineStyle.Color != "" {
			color = spec.LineStyle.Color
		}
		if spec.LineWidthPt <= 0 && spec.LineStyle.Width > 0 {
			spec.LineWidthPt = spec.LineStyle.Width
		}
	}
	if color == "" {
		color = "000000"
	}
	if spec.LineWidthPt < 0.5 {
		spec.LineWidthPt = 0.5
	}

	origPoints := ConnectorWaypoints(spec.Type, spec.BoundsX, spec.BoundsY, spec.BoundsW, spec.BoundsH, spec.Adjustments, spec.FlipH, spec.FlipV)

	headTrim, tailTrim := 0.0, 0.0
	if spec.ArrowHead != "" {
		headTrim = ArrowTrimDistance(spec.ArrowHead, spec.ArrowHeadLength)
	}
	if spec.ArrowTail != "" {
		tailTrim = ArrowTrimDistance(spec.ArrowTail, spec.ArrowTailLength)
	}
	drawPoints := ShortenWaypointsForArrows(origPoints, headTrim, tailTrim)

	var primitives []VectorPrimitive

	// Check if this is a curved connector that should use Bézier curves
	if strings.HasPrefix(spec.Type, "curvedConnector") {
		// For curved connectors, create a Path with cubic Bézier commands
		path := createCubicBezierPath(drawPoints)
		primitives = []VectorPrimitive{{
			Kind: PrimitivePath,
			Path: &path,
			Style: VectorStyle{
				Stroke:      spec.LineStyle,
				StrokeColor: color,
				StrokeWidth: spec.LineWidthPt,
			},
		}}
	} else {
		// For straight and bent connectors, use a polyline
		primitives = []VectorPrimitive{{
			Kind:   PrimitivePolyline,
			Points: append([]Point(nil), drawPoints...),
			Style: VectorStyle{
				Stroke:      spec.LineStyle,
				StrokeColor: color,
				StrokeWidth: spec.LineWidthPt,
			},
		}}
	}

	// Expand compound lines (dbl, thickThin, thinThick, tri) into layered gap strokes.
	// Connectors have no fill, so use white as the gap color (works on light slides).
	if spec.LineStyle != nil && spec.LineStyle.Compound {
		if gaps := compoundLineGapPrimitives(primitives[:1], spec.LineStyle.CompoundType, spec.LineWidthPt, "FFFFFF"); len(gaps) > 0 {
			primitives = append(primitives, gaps...)
		}
	}

	if prim, ok := arrowPrimitive(spec.ArrowHead, spec.ArrowHeadWidth, spec.ArrowHeadLength, color, origPoints[0], origPoints[1]); ok {
		primitives = append(primitives, prim)
	}
	if last := len(origPoints) - 1; last > 0 {
		if prim, ok := arrowPrimitive(spec.ArrowTail, spec.ArrowTailWidth, spec.ArrowTailLength, color, origPoints[last], origPoints[last-1]); ok {
			primitives = append(primitives, prim)
		}
	}

	return primitives
}

func arrowPrimitive(arrowType, widthSize, lengthSize, color string, tip, neighbor Point) (VectorPrimitive, bool) {
	ag := ComputeArrowGeometry(arrowType, widthSize, lengthSize, tip, neighbor)
	if ag == nil {
		return VectorPrimitive{}, false
	}

	switch ag.Type {
	case "triangle", "stealth", "diamond":
		return VectorPrimitive{
			Kind:   PrimitivePolyline,
			Points: append([]Point(nil), ag.PolyPts...),
			Closed: true,
			Style: VectorStyle{
				FillColor: color,
			},
		}, true
	case "arrow":
		return VectorPrimitive{
			Kind:   PrimitivePolyline,
			Points: []Point{ag.Left, ag.Tip, ag.Right},
			Style: VectorStyle{
				Stroke:      &LineStyle{Cap: LineCapRound, Join: LineJoinRound},
				StrokeColor: color,
				StrokeWidth: ag.StrokeWidth,
			},
		}, true
	case "oval":
		return VectorPrimitive{
			Kind: PrimitiveEllipse,
			Ellipse: &Ellipse{
				Center: ag.Center,
				RX:     ag.RX,
				RY:     ag.RY,
			},
			Style: VectorStyle{
				FillColor: color,
			},
		}, true
	default:
		return VectorPrimitive{}, false
	}
}

// BuildShapePrimitives returns the shared vector primitives for a preset shape.
func BuildShapePrimitives(spec ShapeGeometrySpec) ([]VectorPrimitive, error) {
	if spec.Width <= 0 || spec.Height <= 0 {
		return nil, nil
	}

	style := VectorStyle{
		Fill:        spec.Fill,
		FillColor:   spec.FillColor,
		Stroke:      spec.Stroke,
		StrokeColor: spec.StrokeColor,
		StrokeWidth: spec.StrokeWidth,
	}
	if style.Stroke != nil {
		if style.StrokeColor == "" && style.Stroke.Color != "" {
			style.StrokeColor = style.Stroke.Color
		}
		if style.StrokeWidth <= 0 && style.Stroke.Width > 0 {
			style.StrokeWidth = style.Stroke.Width
		}
	}

	if spec.Geometry != nil {
		switch spec.Geometry.Type {
		case GeometryTypeCustom:
			if len(spec.Geometry.Paths) == 0 {
				return nil, nil
			}
			primitives := make([]VectorPrimitive, 0, len(spec.Geometry.Paths))
			for _, path := range spec.Geometry.Paths {
				normalized := NormalizePath(path, spec.Width, spec.Height)
				primitives = append(primitives, VectorPrimitive{
					Kind:  PrimitivePath,
					Path:  &normalized,
					Style: style,
				})
			}
			return primitives, nil
		case GeometryTypePreset:
			if spec.Preset == nil {
				spec.Preset = &PresetGeometry{Name: spec.Geometry.Name}
			}
		}
	}

	if spec.Preset == nil {
		return nil, nil
	}

	if spec.Preset.Name == "ellipse" {
		return []VectorPrimitive{{
			Kind: PrimitiveEllipse,
			Ellipse: &Ellipse{
				Center: Point{X: spec.Width / 2, Y: spec.Height / 2},
				RX:     spec.Width / 2,
				RY:     spec.Height / 2,
			},
			Style: style,
		}}, nil
	}

	path, _ := spec.Preset.GetPath(spec.Width, spec.Height)
	primitives := []VectorPrimitive{{
		Kind:  PrimitivePath,
		Path:  &path,
		Style: style,
	}}
	// Expand compound lines (dbl, thickThin, thinThick, tri) into layered gap strokes.
	// For shapes, the gap color shows through the fill; fall back to white when there is none.
	if style.Stroke != nil && style.Stroke.Compound {
		gapColor := style.FillColor
		if gapColor == "" {
			gapColor = "FFFFFF"
		}
		if gaps := compoundLineGapPrimitives(primitives, style.Stroke.CompoundType, style.StrokeWidth, gapColor); len(gaps) > 0 {
			primitives = append(primitives, gaps...)
		}
	}
	return primitives, nil
}

// compoundLineGapPrimitives returns extra "gap" primitives needed to render a compound
// (double/triple) line. For each source primitive in prims the caller should first write
// the original primitive (full-width stroke with line color), then write all gap primitives
// returned here on top. The gap primitives use a narrower stroke in gapColor to visually
// cut the center out of the wider stroke, creating the appearance of parallel lines.
//
// compoundType is the DrawingML cmpd value ("dbl", "thickThin", "thinThick", "tri").
// baseWidth is the total compound stroke width in points.
// gapColor is the sRGB hex color used for the gap strokes (typically the fill or background color).
func compoundLineGapPrimitives(prims []VectorPrimitive, compoundType string, baseWidth float64, gapColor string) []VectorPrimitive {
	if compoundType == "" || compoundType == "sng" || gapColor == "" {
		return nil
	}
	if baseWidth <= 0 {
		return nil
	}

	// gapRatio is the width of the gap stroke as a fraction of baseWidth.
	// Additional gap ratios cover triple-line variants.
	var gapRatios []float64
	switch compoundType {
	case "dbl":
		// Two equal-weight lines with a centered gap (gap ≈ 38 % of total width).
		gapRatios = []float64{0.38}
	case "thickThin", "thinThick":
		// Outer thick + inner thin (or vice versa); the gap is narrower so more of
		// the total width shows as line color.
		gapRatios = []float64{0.22}
	case "tri":
		// Thin / thick / thin triple line with two gaps.
		gapRatios = []float64{0.42, 0.14}
	default:
		return nil
	}

	gapStyle := LineStyle{Cap: LineCapFlat, Join: LineJoinMiter}

	var extra []VectorPrimitive
	for _, ratio := range gapRatios {
		gapWidth := baseWidth * ratio
		for _, p := range prims {
			gap := VectorPrimitive{
				Kind:    p.Kind,
				Path:    p.Path,
				Points:  p.Points,
				Ellipse: p.Ellipse,
				Closed:  p.Closed,
				Style: VectorStyle{
					Stroke:      &gapStyle,
					StrokeColor: gapColor,
					StrokeWidth: gapWidth,
				},
			}
			extra = append(extra, gap)
		}
	}
	return extra
}

// createCubicBezierPath creates a Path with cubic Bézier commands from waypoints.
// For curved connectors, the waypoints include control points.
func createCubicBezierPath(points []Point) Path {
	if len(points) < 4 {
		// Not enough points for a cubic Bézier curve
		// Fall back to a simple line
		path := Path{}
		if len(points) >= 2 {
			path.Commands = append(path.Commands, PathCommand{
				Type:   "M",
				Points: []Point{points[0]},
			})
			for i := 1; i < len(points); i++ {
				path.Commands = append(path.Commands, PathCommand{
					Type:   "L",
					Points: []Point{points[i]},
				})
			}
		}
		return path
	}

	// For curved connectors, points should be in the format:
	// [start, control1, control2, end] for curvedConnector2
	// [start, control1, control2, control3, end] for curvedConnector3
	path := Path{}

	// Move to start point
	path.Commands = append(path.Commands, PathCommand{
		Type:   "M",
		Points: []Point{points[0]},
	})

	// Create cubic Bézier curve
	if len(points) == 4 {
		// curvedConnector2: start, control1, control2, end
		path.Commands = append(path.Commands, PathCommand{
			Type:   "C",
			Points: []Point{points[1], points[2], points[3]},
		})
	} else if len(points) >= 5 {
		// curvedConnector3: start, control1, control2, control3, end
		// We'll create a composite curve with multiple Bézier segments
		// For simplicity, we'll use the first 4 points for the first segment
		// and the last 4 points for the last segment
		path.Commands = append(path.Commands, PathCommand{
			Type:   "C",
			Points: []Point{points[1], points[2], points[3]},
		})
		// If there are more points, add more segments
		if len(points) > 5 {
			for i := 3; i < len(points)-3; i += 3 {
				if i+3 < len(points) {
					path.Commands = append(path.Commands, PathCommand{
						Type:   "C",
						Points: []Point{points[i+1], points[i+2], points[i+3]},
					})
				}
			}
		}
	}

	return path
}
